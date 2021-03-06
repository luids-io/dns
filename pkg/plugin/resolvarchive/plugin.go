// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

// Package resolvarchive implements a CoreDNS plugin that integrates with
// luIDS dnsutil.Archive api.
package resolvarchive

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/google/uuid"
	"github.com/miekg/dns"

	"github.com/luids-io/api/dnsutil"
	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/pkg/plugin/idsapi"
)

//Plugin is the main struct of the plugin.
type Plugin struct {
	Next plugin.Handler
	Fall fall.F
	//internal
	logger   yalogi.Logger
	cfg      Config
	svc      apiservice.Service
	serverIP net.IP
	exclude  IPSet
	ignoreRC []int
	archiver *Archiver
	started  bool
}

// New returns a new Plugin.
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:      cfg,
		exclude:  cfg.Exclude,
		ignoreRC: cfg.IgnoreRC,
		logger:   wlog{P: clog.NewWithPlugin("resolvarchive")},
	}
	return p, nil
}

// Start plugin.
func (p *Plugin) Start() error {
	if p.started {
		return errors.New("plugin started")
	}
	var err error
	p.serverIP = p.cfg.ServerIP
	if p.serverIP == nil {
		p.serverIP, err = externalIP4()
		if err != nil {
			return err
		}
		p.logger.Infof("server-ip not defined, using ip address: %v", p.serverIP)
	}
	if len(p.ignoreRC) > 0 {
		p.logger.Infof("ignoring return codes: %v", p.ignoreRC)
	}
	var ok bool
	p.svc, ok = idsapi.GetService(p.cfg.Service)
	if !ok {
		return fmt.Errorf("cannot find service '%s'", p.cfg.Service)
	}
	iarchive, ok := p.svc.(dnsutil.Archiver)
	if !ok {
		return fmt.Errorf("service '%s' is not an dnsutil archiver api", p.cfg.Service)
	}
	p.archiver = NewArchiver(iarchive, p.cfg.Buffer, p.logger)
	p.started = true
	return nil
}

// ServeDNS implements the plugin.Handle interface.
func (p Plugin) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	if !p.started {
		return dns.RcodeServerFailure, errors.New("plugin not started")
	}
	// only if A or AAAA then archive response
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	//check if client ip is excluded by config
	clientIP := net.ParseIP(req.IP())
	if p.exclude.Contains(clientIP) {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	// get request id
	rid := idsapi.GetRequestID(ctx)
	if rid == uuid.Nil {
		p.logger.Errorf("can't get request id")
		return dns.RcodeServerFailure, errors.New("can't get request id")
	}
	// fill data with query
	data := &dnsutil.ResolvData{
		ID:        rid,
		Timestamp: time.Now(),
		Server:    p.serverIP,
		Client:    clientIP,
		QID:       query.Id,
		Name:      strings.TrimSuffix(req.Name(), "."),
	}
	if req.QType() == dns.TypeAAAA {
		data.IsIPv6 = true
	}
	data.QueryFlags.AuthenticatedData = query.AuthenticatedData
	data.QueryFlags.CheckingDisabled = query.CheckingDisabled
	if query.IsEdns0() != nil {
		data.QueryFlags.Do = query.IsEdns0().Do()
	}

	// do resolv in next plugin
	rrw := dnstest.NewRecorder(writer)
	rc, err := plugin.NextOrFailure(p.Name(), p.Next, ctx, rrw, query)
	if err != nil {
		return rc, err
	}
	// fill data with response
	data.Duration = time.Since(data.Timestamp)
	data.ReturnCode = rc
	if rrw.Msg != nil {
		data.ReturnCode = rrw.Msg.Rcode
		// check if return code must be ignored
		if len(p.ignoreRC) > 0 {
			for _, irc := range p.ignoreRC {
				if data.ReturnCode == irc {
					return rc, nil
				}
			}
		}
		data.ResponseFlags.AuthenticatedData = rrw.Msg.AuthenticatedData
		if len(rrw.Msg.Answer) > 0 {
			data.ResolvedIPs = make([]net.IP, 0, len(rrw.Msg.Answer))
			data.ResolvedCNAMEs = make([]string, 0, len(rrw.Msg.Answer))
			for _, a := range rrw.Msg.Answer {
				if rsp, ok := a.(*dns.A); ok {
					data.ResolvedIPs = append(data.ResolvedIPs, rsp.A)
				} else if rsp, ok := a.(*dns.AAAA); ok {
					data.ResolvedIPs = append(data.ResolvedIPs, rsp.AAAA)
				} else if rsp, ok := a.(*dns.CNAME); ok {
					target := rsp.Target
					if dns.IsFqdn(target) {
						target = strings.TrimSuffix(target, ".")
					}
					data.ResolvedCNAMEs = append(data.ResolvedCNAMEs, target)
				}
			}
		}
	}
	p.archiver.SaveResolv(data)
	return rc, err
}

// Name implements plugin interface.
func (p Plugin) Name() string { return "resolvarchive" }

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
	return p.archiver.Close()
}
