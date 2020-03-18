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
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/dnsutil"
	"github.com/luids-io/dns/pkg/plugin/luidsapi"
)

//Plugin is the main struct of the plugin
type Plugin struct {
	Next plugin.Handler
	Fall fall.F
	//internal
	logger   yalogi.Logger
	cfg      Config
	svc      apiservice.Service
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
	var ok bool
	p.svc, ok = luidsapi.GetService(p.cfg.Service)
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
	// fill data with query
	data := dnsutil.ResolvData{
		Timestamp:        time.Now(),
		Server:           net.ParseIP(req.LocalIP()),
		Client:           net.ParseIP(req.IP()),
		QID:              query.Id,
		Name:             strings.TrimSuffix(req.Name(), "."),
		CheckingDisabled: query.CheckingDisabled,
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
		data.AuthenticatedData = rrw.Msg.AuthenticatedData
		if len(rrw.Msg.Answer) > 0 {
			data.Resolved = make([]net.IP, 0, len(rrw.Msg.Answer))
			for _, a := range rrw.Msg.Answer {
				if rsp, ok := a.(*dns.A); ok {
					data.Resolved = append(data.Resolved, rsp.A)
				} else if rsp, ok := a.(*dns.AAAA); ok {
					data.Resolved = append(data.Resolved, rsp.AAAA)
				}
			}
		}
	}
	p.archiver.SaveResolv(data)
	return rc, err
}

// Name implements plugin interface
func (p Plugin) Name() string { return "resolvarchive" }

// Health implements plugin health interface
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.svc.Ping() == nil
}

// Shutdown plugin
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.archiver.Close()
}
