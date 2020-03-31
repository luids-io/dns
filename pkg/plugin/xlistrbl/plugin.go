// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlistrbl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/utils/yalogi"
	"github.com/luids-io/core/xlist"
	"github.com/luids-io/dns/pkg/plugin/luidsapi"
)

//Plugin is the main struct of the plugin
type Plugin struct {
	Next  plugin.Handler
	Fall  fall.F
	Zones []string
	//internal
	cfg      Config
	logger   yalogi.Logger
	returnIP string
	svc      apiservice.Service
	checker  xlist.Checker
	started  bool
}

// New returns a new Plugin
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:      cfg,
		logger:   wlog{P: clog.NewWithPlugin("xlistrbl")},
		returnIP: cfg.ReturnIP,
		Zones:    cfg.Zones,
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
	p.checker, ok = p.svc.(xlist.Checker)
	if !ok {
		return fmt.Errorf("service '%s' is not a xlist checker api", p.cfg.Service)
	}
	p.started = true
	return nil
}

// Name implements plugin interface
func (p Plugin) Name() string { return "xlistrbl" }

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
	return nil
}

// ServeDNS implements the plugin.Handle interface.
func (p Plugin) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	if !p.started {
		return dns.RcodeServerFailure, errors.New("plugin not started")
	}
	// only respond to A and TXT queries
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeTXT {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	//check if request name matches with zones managed
	qname := req.Name()
	zone := plugin.Zones(p.Zones).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	//prepare query
	checkres := strings.TrimSuffix(qname, zone)
	if dns.IsFqdn(checkres) {
		checkres = strings.TrimSuffix(checkres, ".")
	}
	checkres = reverse(checkres)
	//get resource type
	resource, err := xlist.ResourceType(checkres,
		[]xlist.Resource{xlist.IPv4, xlist.IPv6, xlist.Domain})
	if err != nil {
		p.logger.Debugf("error getting resource type %s: %v", qname, err)
		return dns.RcodeRefused, err
	}
	//TODO(luisguillenc): implement ipv6 support
	if resource == xlist.IPv6 {
		p.logger.Debugf("got ipv6 request %s: %v", qname, err)
		return dns.RcodeRefused, err
	}
	//check in blacklist
	response, err := p.checker.Check(ctx, checkres, resource)
	if err == xlist.ErrNotImplemented {
		p.logger.Debugf("unsupported resource %v for query %s: %v", resource, qname, err)
		return dns.RcodeRefused, err
	}
	if err != nil {
		p.logger.Warnf("error checking %s: %v", checkres, err)
		return dns.RcodeServerFailure, err
	}
	// return result
	if response.Result {
		switch req.QType() {
		case dns.TypeA:
			m := getMsgReplyIP(p.returnIP, response.TTL, &req)
			return dns.RcodeSuccess, req.W.WriteMsg(m)
		case dns.TypeTXT:
			m := getMsgReplyTxt(response.Reason, response.TTL, &req)
			return dns.RcodeSuccess, req.W.WriteMsg(m)
		}
	}
	m := getMsgReplyNXDomain(&req)
	return dns.RcodeNameError, req.W.WriteMsg(m)
}
