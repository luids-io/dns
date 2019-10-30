// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

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
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/core/dnsutil"
)

//Plugin is the main struct of the plugin
type Plugin struct {
	Next plugin.Handler
	Fall fall.F

	logger   yalogi.Logger
	cfg      Config
	archiver *Archiver
	started  bool
}

// New returns a new Plugin
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:    cfg,
		logger: wlog{P: clog.NewWithPlugin("resolvarchive")},
	}
	return p, nil
}

// Start plugin
func (p *Plugin) Start() error {
	if p.started {
		return errors.New("plugin started")
	}
	dial, err := grpctls.Dial(p.cfg.Endpoint, p.cfg.Client)
	if err != nil {
		return fmt.Errorf("cannot dial with %s: %v", p.cfg.Endpoint, err)
	}
	p.archiver = NewArchiver(dial, p.cfg.Buffer, p.logger)
	p.started = true
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
	// gets IPs from answer
	var resolved []net.IP
	if rrw.Msg != nil && len(rrw.Msg.Answer) > 0 {
		resolved = make([]net.IP, 0, len(rrw.Msg.Answer))
		for _, a := range rrw.Msg.Answer {
			if rsp, ok := a.(*dns.A); ok {
				resolved = append(resolved, rsp.A)
			} else if rsp, ok := a.(*dns.AAAA); ok {
				resolved = append(resolved, rsp.AAAA)
			}
		}
	}
	if len(resolved) > 0 {
		// prepare data
		name := req.Name()
		if dns.IsFqdn(name) {
			name = strings.TrimSuffix(name, ".")
		}
		local := req.LocalIP()
		server := net.ParseIP(local)
		if server == nil {
			p.logger.Warnf("parsing local '%s'", local)
			return rc, err
		}
		remote, _, _ := net.SplitHostPort(req.RemoteAddr())
		client := net.ParseIP(remote)
		if client == nil {
			p.logger.Warnf("parsing remote '%s'", req.RemoteAddr())
			return rc, err
		}
		// save data
		data := dnsutil.ResolvData{
			Timestamp: time.Now(),
			Server:    server,
			Client:    client,
			Name:      name,
			Resolved:  resolved,
		}
		p.archiver.SaveResolv(data)
	}
	return rc, err
}

// Name implements plugin interface
func (p Plugin) Name() string { return "resolvarchive" }

// Health implements plugin health interface
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	err := p.archiver.Ping()
	return err == nil
}

// Shutdown plugin
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.archiver.Close()
}