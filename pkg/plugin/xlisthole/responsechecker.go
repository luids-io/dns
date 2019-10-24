// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"context"

	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/luids-io/core/event"
	"github.com/luids-io/core/event/codes"
	"github.com/luids-io/core/xlist"
	"github.com/luids-io/core/xlist/check/parallel"
	"github.com/luids-io/core/xlist/reason"
)

// ResponseChecker implements dns.ResponseWriter, it is used internally by
// XListHole to check ips returned by other plugins
type ResponseChecker struct {
	dns.ResponseWriter
	ctx context.Context
	req *request.Request
	fw  *XListHole
}

// WriteMsg implements dns.ResponseWriter interface. In this method, the returned IP
// validation is done.
func (r *ResponseChecker) WriteMsg(q *dns.Msg) error {
	if q.Rcode != dns.RcodeSuccess || len(q.Answer) == 0 {
		return r.ResponseWriter.WriteMsg(q)
	}
	// prepare parallel queries
	queries := r.prepareQueries(q.Answer)
	// do check
	responses, hasErrors, err := parallel.Check(r.ctx, []xlist.Checker{r.fw.client}, queries)
	if err != nil {
		//on-error
		r.fw.metrics.errors.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
		r.fw.logger.Warnf("error in request: %v", err)
		return r.dispatchAction(q, r.fw.policy.OnError, 0)
	}
	if hasErrors {
		//on-error in some of the responses
		for _, resp := range responses {
			if resp.Err != nil {
				r.fw.metrics.errors.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
				r.fw.logger.Warnf("error checking %s: %v", resp.Request.Name, resp.Err)
			}
		}
		return r.dispatchAction(q, r.fw.policy.OnError, 0)
	}
	//check ok, process responses
	action, ttl, err := r.processResponses(responses)
	if err != nil {
		r.fw.metrics.errors.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
		r.fw.logger.Warnf("processing responses: %v", err.Error())
		return r.dispatchAction(q, r.fw.policy.OnError, 0)
	}
	// dispatch final computed action
	return r.dispatchAction(q, action, ttl)
}

// prepare parallel queries from an answer
func (r *ResponseChecker) prepareQueries(answer []dns.RR) []parallel.Request {
	queries := make([]parallel.Request, 0, len(answer))
	for _, a := range answer {
		//check ip4 returned
		if rsp, ok := a.(*dns.A); ok {
			queries = append(queries, parallel.Request{
				Name:     rsp.A.String(),
				Resource: xlist.IPv4,
			})
		}
		//check ip6 returned
		if rsp, ok := a.(*dns.AAAA); ok {
			queries = append(queries, parallel.Request{
				Name:     rsp.AAAA.String(),
				Resource: xlist.IPv6,
			})
		}
	}
	return queries
}

func (r *ResponseChecker) processResponses(responses []parallel.Response) (ActionInfo, int, error) {
	// default rule
	applyRule := r.fw.policy.IP.Unlisted
	applyTTL := 0
	// iterate results
	for _, resp := range responses {
		var code event.Code
		var rule Rule
		if resp.Response.Result {
			//if it's on the list
			rule = r.fw.policy.IP.Listed
			if r.fw.policy.IP.Merge {
				err := rule.Merge(resp.Response.Reason)
				if err != nil {
					return r.fw.policy.OnError, 0, err
				}
			}
			applyRule = rule
			applyTTL = resp.Response.TTL
			code = codes.DNSListedIP
			r.fw.metrics.listedIPs.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
		} else {
			//if it's not on the list
			rule = r.fw.policy.IP.Unlisted
			code = codes.DNSUnlistedIP
			r.fw.metrics.unlistedIPs.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
		}
		//now, apply policy for this IP check
		if rule.Log {
			r.fw.logger.Infof("%s check '%s' response: %v '%s'", r.req.RemoteAddr(),
				resp.Request.Name, resp.Response.Result, resp.Response.Reason)
		}
		if rule.Event.Raise {
			e := event.New(event.Security, code, rule.Event.Level)
			e.Set("remote", r.req.RemoteAddr())
			e.Set("query", r.req.Name())
			e.Set("listed", resp.Request.Name)
			e.Set("reason", reason.Clean(resp.Response.Reason))
			event.Notify(e)
		}
	}
	return applyRule.Action, applyTTL, nil
}

func (r *ResponseChecker) dispatchAction(q *dns.Msg, a ActionInfo, ttl int) error {
	// dispatch final computed action
	switch a.Type {
	case SendNXDomain:
		m := getMsgReplyNXDomain(r.req)
		return r.ResponseWriter.WriteMsg(m)
	case SendRefused:
		m := getMsgReplyRefused(r.req)
		return r.ResponseWriter.WriteMsg(m)
	case SendFixedIP4:
		m := getMsgReplyIP(a.Data, ttl, r.req)
		return r.ResponseWriter.WriteMsg(m)
	default:
		// returns original response
		return r.ResponseWriter.WriteMsg(q)
	}
}
