// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"context"
	"strings"

	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"

	"github.com/luids-io/api/event"
	"github.com/luids-io/api/xlist"
	"github.com/luids-io/api/xlist/parallel"
	"github.com/luids-io/core/reason"
	"github.com/luids-io/dns/pkg/plugin/idsapi"
	"github.com/luids-io/dns/pkg/plugin/idsevent"
)

// responseChecker implements dns.ResponseWriter, it is used internally by
// plugin to check ips returned by other plugins
type responseChecker struct {
	dns.ResponseWriter
	ctx     context.Context
	req     *request.Request
	fw      *Plugin
	checker xlist.Checker
	// must check...
	checkIPs    bool
	checkCNAMEs bool
}

// WriteMsg implements dns.ResponseWriter interface. In this method, the returned IP
// validation is done.
func (r *responseChecker) WriteMsg(q *dns.Msg) error {
	if q.Rcode != dns.RcodeSuccess || len(q.Answer) == 0 {
		return r.ResponseWriter.WriteMsg(q)
	}
	// prepare parallel queries
	queries := r.prepareQueries(q.Answer)
	// do check
	responses, hasErrors, err := parallel.Check(r.ctx, []xlist.Checker{r.checker}, queries)
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
	action, ttl, err := r.processResponses(r.ctx, responses)
	if err != nil {
		r.fw.metrics.errors.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
		r.fw.logger.Warnf("processing responses: %v", err.Error())
		return r.dispatchAction(q, r.fw.policy.OnError, 0)
	}
	// dispatch final computed action
	return r.dispatchAction(q, action, ttl)
}

// prepare parallel queries from an answer
func (r *responseChecker) prepareQueries(answer []dns.RR) []parallel.Request {
	queries := make([]parallel.Request, 0, len(answer))
	for _, a := range answer {
		if r.checkIPs {
			//check ip4 returned
			if rsp, ok := a.(*dns.A); ok {
				queries = append(queries, parallel.Request{
					Name:     rsp.A.String(),
					Resource: xlist.IPv4,
				})
				continue
			}
			//check ip6 returned
			if rsp, ok := a.(*dns.AAAA); ok {
				queries = append(queries, parallel.Request{
					Name:     rsp.AAAA.String(),
					Resource: xlist.IPv6,
				})
				continue
			}
		}
		if r.checkCNAMEs {
			//check cname returned
			if rsp, ok := a.(*dns.CNAME); ok {
				target := rsp.Target
				if dns.IsFqdn(target) {
					target = strings.TrimSuffix(target, ".")
				}
				queries = append(queries, parallel.Request{
					Name:     target,
					Resource: xlist.Domain,
				})
			}
		}
	}
	return queries
}

func (r *responseChecker) processResponses(ctx context.Context, responses []parallel.Response) (ActionInfo, int, error) {
	// apply rule defines final processing dns response
	applyRule := r.fw.policy.CNAME.Unlisted
	if r.checkIPs {
		applyRule = r.fw.policy.IP.Unlisted
	}
	applyTTL := 0
	// iterate results
	for _, resp := range responses {
		var code event.Code
		var rule Rule
		if resp.Request.Resource == xlist.Domain {
			if resp.Response.Result {
				//if it's on the list
				rule = r.fw.policy.CNAME.Listed
				if r.fw.policy.CNAME.Merge {
					err := rule.Merge(resp.Response.Reason)
					if err != nil {
						return r.fw.policy.OnError, 0, err
					}
				}
				applyRule = rule
				applyTTL = resp.Response.TTL
				code = idsevent.DNSListedDomain
				r.fw.metrics.listedDomains.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
			} else {
				//if it's not on the list
				rule = r.fw.policy.CNAME.Unlisted
				code = idsevent.DNSUnlistedDomain
				r.fw.metrics.unlistedDomains.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
			}
		} else {
			//it's an ipv4 or ipv6
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
				code = idsevent.DNSListedIP
				r.fw.metrics.listedIPs.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
			} else {
				//if it's not on the list
				rule = r.fw.policy.IP.Unlisted
				code = idsevent.DNSUnlistedIP
				r.fw.metrics.unlistedIPs.WithLabelValues(metrics.WithServer(r.ctx)).Inc()
			}
		}
		//now, apply policy for this response check
		if rule.Log {
			r.fw.logger.Infof("%s check '%s' response: %v '%s'", r.req.RemoteAddr(),
				resp.Request.Name, resp.Response.Result, reason.Clean(resp.Response.Reason))
		}
		if rule.Event.Raise {
			e := event.New(code, rule.Event.Level)
			e.Set("rid", idsapi.GetRequestID(ctx).String())
			e.Set("remote", r.req.IP())
			e.Set("query", domainFromRequest(r.req))
			e.Set("name", resp.Request.Name)
			if resp.Response.Result {
				e.Set("reason", reason.Clean(resp.Response.Reason))
				score, _, _ := reason.ExtractScore(resp.Response.Reason)
				if score > 0 {
					e.Set("score", score)
				}
			}
			event.Notify(e)
		}
	}
	return applyRule.Action, applyTTL, nil
}

func (r *responseChecker) dispatchAction(q *dns.Msg, a ActionInfo, ttl int) error {
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
