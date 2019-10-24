// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"context"
	"fmt"
	"strings"

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

// XListHole is the main struct of the plugin
type XListHole struct {
	Next plugin.Handler
	Fall fall.F

	logger  yalogi.Logger
	metrics *fwmetrics
	policy  RuleSet
	client  *check.Client
}

// New returns a new XListHole
func New(cfg Config) (*XListHole, error) {
	// dial grpc connection
	dial, err := grpctls.Dial(cfg.Endpoint, cfg.Client)
	if err != nil {
		return nil, fmt.Errorf("cannot dial with %s: %v", cfg.Endpoint, err)
	}
	// wrapped logger
	logger := wlog{P: clog.NewWithPlugin("xlisthole")}
	// create client
	client := check.NewClient(
		dial, []xlist.Resource{},
		check.SetLogger(logger),
		check.SetCache(cfg.CacheTTL, cfg.CacheTTL))
	// create and set values
	return &XListHole{
		logger:  logger,
		metrics: newMetrics(),
		policy:  cfg.Policy,
		client:  client,
	}, nil
}

// ServeDNS implements the plugin.Handle interface. In this method, the blackhole does
// the checking process of domain names.
func (f *XListHole) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	//create request
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, writer, query)
	}
	//get domain from request
	domain := req.Name()
	if dns.IsFqdn(domain) {
		domain = strings.TrimSuffix(domain, ".")
	}
	//check in xlist
	resp, err := f.client.Check(ctx, domain, xlist.Domain)
	if err != nil {
		//on-error
		f.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		f.logger.Warnf("error checking %s: %v", domain, err)
		return f.dispatchAction(ctx, &req, f.policy.OnError, 0)
	}
	// process response
	action, ttl, err := f.processResponse(ctx, &req, domain, resp)
	if err != nil {
		f.metrics.errors.WithLabelValues(metrics.WithServer(ctx)).Inc()
		f.logger.Warnf("processing response %s: %v", domain, err)
		return f.dispatchAction(ctx, &req, f.policy.OnError, 0)
	}
	// dispatch rule action
	return f.dispatchAction(ctx, &req, action, ttl)
}

// Name implements the plugin.Handle interface.
func (f XListHole) Name() string { return "xlisthole" }

// Health implements the health.Healther interface.
func (f XListHole) Health() bool {
	err := f.client.Ping()
	return err == nil
}

// Close xlisthole
func (f *XListHole) Close() error {
	return f.client.Close()
}

func (f *XListHole) newResponseChecker(ctx context.Context, req *request.Request) *ResponseChecker {
	return &ResponseChecker{
		ResponseWriter: req.W,
		ctx:            ctx,
		req:            req,
		fw:             f,
	}
}

func (f *XListHole) processResponse(ctx context.Context, req *request.Request, domain string, resp xlist.Response) (ActionInfo, int, error) {
	// get applied rule
	var code event.Code
	var rule Rule
	if resp.Result {
		code = codes.DNSListedDomain
		rule = f.policy.Domain.Listed
		if f.policy.Domain.Merge {
			err := rule.Merge(resp.Reason)
			if err != nil {
				return rule.Action, 0, fmt.Errorf("processing reason '%s': %v", resp.Reason, err)
			}
		}
		f.metrics.listedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	} else {
		code = codes.DNSUnlistedDomain
		rule = f.policy.Domain.Unlisted
		f.metrics.unlistedDomains.WithLabelValues(metrics.WithServer(ctx)).Inc()
	}
	// apply rule
	if rule.Log {
		f.logger.Infof("%s check '%s' response: %v '%s'", req.RemoteAddr(),
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

func (f *XListHole) dispatchAction(ctx context.Context, req *request.Request, a ActionInfo, ttl int) (int, error) {
	// dispatch rule action
	switch a.Type {
	case ReturnValue:
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, req.W, req.Req)
	case CheckIP:
		respChecker := f.newResponseChecker(ctx, req)
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, respChecker, req.Req)
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
