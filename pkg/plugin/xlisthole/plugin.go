// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

// Package xlisthole implements a CoreDNS plugin that integrates with luIDS
// xlist.Check api and adds sinkhole functionality.
package xlisthole

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/luids-io/api/event"
	"github.com/luids-io/api/xlist"
	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/reason"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/pkg/plugin/idsapi"
	"github.com/luids-io/dns/pkg/plugin/idsevent"
)

// Plugin is the main struct of the plugin.
type Plugin struct {
	Next plugin.Handler
	Fall fall.F

	cfg     Config
	logger  yalogi.Logger
	metrics *fwmetrics
	policy  RuleSet
	exclude IPSet
	svc     apiservice.Service
	checker xlist.Checker
	views   []dnsview
	started bool
}

type dnsview struct {
	service string
	include IPSet
	checker xlist.Checker
}

// New returns a new Plugin.
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:     cfg,
		logger:  wlog{P: clog.NewWithPlugin("xlisthole")},
		metrics: newMetrics(),
		policy:  cfg.Policy,
		exclude: cfg.Exclude,
	}
	if len(cfg.Views) > 0 {
		p.views = make([]dnsview, 0, len(cfg.Views))
		for _, v := range cfg.Views {
			p.views = append(p.views, dnsview{service: v.Service, include: v.Include})
		}
	}
	return p, nil
}

// RegisterMetrics register metrics in controller.
func (p *Plugin) RegisterMetrics(c *caddy.Controller) {
	p.metrics.register(c)
}

// Name implements the plugin.Handle interface.
func (p Plugin) Name() string { return "xlisthole" }

// Health implements the health.Healther interface.
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.svc.Ping() == nil
}

// Start plugin.
func (p *Plugin) Start() error {
	if p.started {
		return errors.New("plugin started")
	}
	var ok bool
	p.svc, ok = idsapi.GetService(p.cfg.Service)
	if !ok {
		return fmt.Errorf("cannot find service '%s'", p.cfg.Service)
	}
	p.checker, ok = p.svc.(xlist.Checker)
	if !ok {
		return fmt.Errorf("service '%s' is not a xlist checker api", p.cfg.Service)
	}
	// get service views
	if len(p.views) > 0 {
		for idx, view := range p.views {
			svc, ok := idsapi.GetService(view.service)
			if !ok {
				return fmt.Errorf("cannot find service '%s'", view.service)
			}
			p.views[idx].checker, ok = svc.(xlist.Checker)
			if !ok {
				return fmt.Errorf("service '%s' is not a xlist checker api", view.service)
			}
		}
	}
	p.started = true
	return nil
}

// Shutdown plugin.
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return nil
}

// ServeDNS implements the plugin.Handle interface. In this method, the blackhole does
// the checking process of domain names.
func (p *Plugin) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	if !p.started {
		return dns.RcodeServerFailure, errors.New("plugin not started")
	}
	//create request
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	//check if client ip is excluded by config
	clientIP := net.ParseIP(req.IP())
	if p.exclude.Contains(clientIP) {
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
	}
	//get checker for client ip (if use views)
	checker := p.getChecker(clientIP)
	//get domain from request
	domain := domainFromRequest(&req)
	//check in xlist
	resp, err := checker.Check(ctx, domain, xlist.Domain)
	if err != nil {
		//on-error
		p.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		p.logger.Warnf("error checking %s: %v", domain, err)
		return p.dispatchAction(ctx, &req, checker, p.policy.OnError, 0)
	}
	// process response
	action, ttl, err := p.processResponse(ctx, &req, domain, resp)
	if err != nil {
		p.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		p.logger.Warnf("processing response %s: %v", domain, err)
		return p.dispatchAction(ctx, &req, checker, p.policy.OnError, 0)
	}
	// dispatch rule action
	return p.dispatchAction(ctx, &req, checker, action, ttl)
}

func (p *Plugin) newResponseChecker(ctx context.Context, a ActionType, req *request.Request, c xlist.Checker) *responseChecker {
	r := &responseChecker{
		ResponseWriter: req.W,
		ctx:            ctx,
		req:            req,
		fw:             p,
		checker:        c,
	}
	switch a {
	case CheckIP:
		r.checkIPs = true
	case CheckCNAME:
		r.checkCNAMEs = true
	case CheckAll:
		r.checkIPs = true
		r.checkCNAMEs = true
	}
	return r
}

func (p *Plugin) processResponse(ctx context.Context, req *request.Request, domain string, resp xlist.Response) (ActionInfo, int, error) {
	// get applied rule
	var code event.Code
	var rule Rule
	if resp.Result {
		code = idsevent.DNSListedDomain
		rule = p.policy.Domain.Listed
		if p.policy.Domain.Merge {
			err := rule.Merge(resp.Reason)
			if err != nil {
				return rule.Action, 0, fmt.Errorf("processing reason '%s': %v", resp.Reason, err)
			}
		}
		p.metrics.listedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	} else {
		code = idsevent.DNSUnlistedDomain
		rule = p.policy.Domain.Unlisted
		p.metrics.unlistedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	}
	// apply rule
	if rule.Log {
		p.logger.Infof("%s check '%s' response: %v '%s'", req.RemoteAddr(),
			domain, resp.Result, reason.Clean(resp.Reason))
	}
	if rule.Event.Raise {
		e := event.New(code, rule.Event.Level)
		e.Set("rid", idsapi.GetRequestID(ctx).String())
		e.Set("remote", req.IP())
		e.Set("query", domain)
		e.Set("name", domain)
		if resp.Result {
			e.Set("reason", reason.Clean(resp.Reason))
			score, _, _ := reason.ExtractScore(resp.Reason)
			if score > 0 {
				e.Set("score", score)
			}
		}
		event.Notify(e)
	}
	return rule.Action, resp.TTL, nil
}

func (p *Plugin) dispatchAction(ctx context.Context, req *request.Request, c xlist.Checker, a ActionInfo, ttl int) (int, error) {
	// dispatch rule action
	switch a.Type {
	case ReturnValue:
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, req.W, req.Req)
	case CheckIP, CheckCNAME, CheckAll:
		respChecker := p.newResponseChecker(ctx, a.Type, req, c)
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, respChecker, req.Req)
	case SendFixedIP4:
		m := getMsgReplyIP(a.Data, ttl, req)
		return dns.RcodeSuccess, req.W.WriteMsg(m)
	case SendRefused:
		m := getMsgReplyRefused(req)
		return dns.RcodeRefused, req.W.WriteMsg(m)
	default:
		m := getMsgReplyNXDomain(req)
		return dns.RcodeNameError, req.W.WriteMsg(m)
	}
}

func (p *Plugin) getChecker(ip net.IP) (checker xlist.Checker) {
	checker = p.checker
	if len(p.views) > 0 {
		for _, view := range p.views {
			if view.include.Contains(ip) {
				checker = view.checker
				return
			}
		}
	}
	return
}
