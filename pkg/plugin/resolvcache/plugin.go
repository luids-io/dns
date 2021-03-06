// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

// Package resolvcache implements a CoreDNS plugin that integrates with luIDS
// dnsutil.ResolvCollect api.
package resolvcache

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/luids-io/api/dnsutil"
	"github.com/luids-io/api/event"
	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/pkg/plugin/idsapi"
	"github.com/luids-io/dns/pkg/plugin/idsevent"
)

// Plugin is the main struct of the plugin.
type Plugin struct {
	Next plugin.Handler
	Fall fall.F
	//internal variables
	logger    yalogi.Logger
	cfg       Config
	policy    RuleSet
	svc       apiservice.Service
	collector dnsutil.ResolvCollector
	started   bool
}

// New returns a new Plugin.
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:    cfg,
		logger: wlog{P: clog.NewWithPlugin("resolvcache")},
		policy: cfg.Policy,
	}
	return p, nil
}

// Start plugin.
func (p *Plugin) Start() error {
	var ok bool
	p.svc, ok = idsapi.GetService(p.cfg.Service)
	if !ok {
		return fmt.Errorf("cannot find service '%s'", p.cfg.Service)
	}
	p.collector, ok = p.svc.(dnsutil.ResolvCollector)
	if !ok {
		return fmt.Errorf("service '%s' is not an dnsutil resolvcollect api", p.cfg.Service)
	}
	p.started = true
	return nil
}

// Name implements plugin interface.
func (p Plugin) Name() string { return "resolvcache" }

// Health implements plugin health interface.
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.svc.Ping() == nil
}

// Shutdown plugin.
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return nil
}

// ServeDNS implements the plugin.Handle interface.
func (p Plugin) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	if !p.started {
		return dns.RcodeServerFailure, errors.New("plugin not started")
	}
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	// if A or AAAA gets response
	rrw := dnstest.NewRecorder(writer)
	rc, err := plugin.NextOrFailure(p.Name(), p.Next, ctx, rrw, query)
	if rc != dns.RcodeSuccess || err != nil {
		return rc, err
	}
	// gets IPs and CNAMEs from answer
	var resolved []net.IP
	var cnames []string
	if rrw.Msg != nil && len(rrw.Msg.Answer) > 0 {
		resolved = make([]net.IP, 0, len(rrw.Msg.Answer))
		cnames = make([]string, 0, len(rrw.Msg.Answer))
		for _, a := range rrw.Msg.Answer {
			if rsp, ok := a.(*dns.A); ok {
				resolved = append(resolved, rsp.A)
			} else if rsp, ok := a.(*dns.AAAA); ok {
				resolved = append(resolved, rsp.AAAA)
			} else if rsp, ok := a.(*dns.CNAME); ok {
				target := rsp.Target
				if dns.IsFqdn(target) {
					target = strings.TrimSuffix(target, ".")
				}
				cnames = append(cnames, target)
			}
		}
	}
	if len(resolved) > 0 {
		// prepare data
		name := req.Name()
		if dns.IsFqdn(name) {
			name = strings.TrimSuffix(name, ".")
		}
		remote, _, _ := net.SplitHostPort(req.RemoteAddr())
		client := net.ParseIP(remote)
		if client == nil {
			p.logger.Warnf("parsing remote '%s'", req.RemoteAddr())
			return rc, err
		}
		// collect data
		p.doCollect(ctx, client, name, resolved, cnames)
	}
	return rc, err
}

func (p *Plugin) doCollect(ctx context.Context, client net.IP, name string, resolved []net.IP, cnames []string) {
	err := p.collector.Collect(ctx, client, name, resolved, cnames)
	if err != nil {
		rid := idsapi.GetRequestID(ctx)
		//apply policy management error
		switch err {
		case dnsutil.ErrLimitDNSClientQueries:
			if p.policy.MaxClientRequests.Log {
				p.logger.Infof("%v", err)
			}
			if p.policy.MaxClientRequests.Event.Raise {
				level := p.policy.MaxClientRequests.Event.Level
				e := event.New(idsevent.DNSMaxClientRequests, level)
				e.Set("rid", rid.String())
				e.Set("remote", client)
				event.Notify(e)
			}
		case dnsutil.ErrLimitResolvedNamesIP:
			if p.policy.MaxNamesResolved.Log {
				p.logger.Infof("%v", err)
			}
			if p.policy.MaxNamesResolved.Event.Raise {
				level := p.policy.MaxNamesResolved.Event.Level
				e := event.New(idsevent.DNSMaxNamesResolvedIP, level)
				e.Set("rid", rid.String())
				e.Set("remote", client)
				e.Set("resolved", resolved)
				event.Notify(e)
			}
		default:
			p.logger.Warnf("%v", err)
		}
	}
}
