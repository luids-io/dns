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
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/api/xlist/check"
	"github.com/luids-io/core/xlist"
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
	client   *check.Client
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
	// dial grpc connection
	dial, err := grpctls.Dial(p.cfg.Endpoint, p.cfg.Client)
	if err != nil {
		return fmt.Errorf("cannot dial with %s: %v", p.cfg.Endpoint, err)
	}
	// create xlist client
	p.client = check.NewClient(dial, []xlist.Resource{},
		check.SetLogger(p.logger),
		check.SetCache(p.cfg.CacheTTL, p.cfg.CacheTTL))
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
	err := p.client.Ping()
	return err == nil
}

// Shutdown plugin
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.client.Close()
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
	response, err := p.client.Check(ctx, checkres, resource)
	if err == xlist.ErrResourceNotSupported {
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
