// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/core/event"
	"github.com/luids-io/core/event/codes"
	"github.com/luids-io/core/xlist"
	"github.com/luids-io/core/xlist/check"
	"github.com/luids-io/core/xlist/reason"
)

// Plugin is the main struct of the plugin
type Plugin struct {
	Next plugin.Handler
	Fall fall.F

	cfg     Config
	logger  yalogi.Logger
	metrics *fwmetrics
	policy  RuleSet
	client  *check.Client
	started bool
}

// New returns a new Plugin
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
	}
	return p, nil
}

// RegisterMetrics register metrics in controller
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
	err := p.client.Ping()
	return err == nil
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
	// create client
	p.client = check.NewClient(dial, []xlist.Resource{},
		check.SetLogger(p.logger),
		check.SetCache(p.cfg.CacheTTL, p.cfg.CacheTTL))
	p.started = true
	return nil
}

// Shutdown plugin
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.client.Close()
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
	//get domain from request
	domain := req.Name()
	if dns.IsFqdn(domain) {
		domain = strings.TrimSuffix(domain, ".")
	}
	//check in xlist
	resp, err := p.client.Check(ctx, domain, xlist.Domain)
	if err != nil {
		//on-error
		p.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		p.logger.Warnf("error checking %s: %v", domain, err)
		return p.dispatchAction(ctx, &req, p.policy.OnError, 0)
	}
	// process response
	action, ttl, err := p.processResponse(ctx, &req, domain, resp)
	if err != nil {
		p.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		p.logger.Warnf("processing response %s: %v", domain, err)
		return p.dispatchAction(ctx, &req, p.policy.OnError, 0)
	}
	// dispatch rule action
	return p.dispatchAction(ctx, &req, action, ttl)
}

func (p *Plugin) newResponseChecker(ctx context.Context, req *request.Request) *responseChecker {
	return &responseChecker{
		ResponseWriter: req.W,
		ctx:            ctx,
		req:            req,
		fw:             p,
	}
}

func (p *Plugin) processResponse(ctx context.Context, req *request.Request, domain string, resp xlist.Response) (ActionInfo, int, error) {
	// get applied rule
	var code event.Code
	var rule Rule
	if resp.Result {
		code = codes.DNSListedDomain
		rule = p.policy.Domain.Listed
		if p.policy.Domain.Merge {
			err := rule.Merge(resp.Reason)
			if err != nil {
				return rule.Action, 0, fmt.Errorf("processing reason '%s': %v", resp.Reason, err)
			}
		}
		p.metrics.listedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	} else {
		code = codes.DNSUnlistedDomain
		rule = p.policy.Domain.Unlisted
		p.metrics.unlistedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	}
	// apply rule
	if rule.Log {
		p.logger.Infof("%s check '%s' response: %v '%s'", req.RemoteAddr(),
			domain, resp.Result, reason.Clean(resp.Reason))
	}
	if rule.Event.Raise {
		e := event.New(event.Security, code, rule.Event.Level)
		e.Set("remote", req.RemoteAddr())
		e.Set("query", domain)
		e.Set("listed", domain)
		e.Set("reason", reason.Clean(resp.Reason))
		event.Notify(e)
	}
	return rule.Action, resp.TTL, nil
}

func (p *Plugin) dispatchAction(ctx context.Context, req *request.Request, a ActionInfo, ttl int) (int, error) {
	// dispatch rule action
	switch a.Type {
	case ReturnValue:
		return plugin.NextOrFailure(p.Name(), p.Next, ctx, req.W, req.Req)
	case CheckIP:
		respChecker := p.newResponseChecker(ctx, req)
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
