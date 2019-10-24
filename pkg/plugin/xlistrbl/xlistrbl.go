// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlistrbl

import (
	"context"
	"fmt"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/core/xlist"
	"github.com/luids-io/core/xlist/check"
)

//XListRBL is the main struct of the plugin
type XListRBL struct {
	Next  plugin.Handler
	Fall  fall.F
	Zones []string

	logger   yalogi.Logger
	returnIP string
	client   *check.Client
}

// New returns a new XListRBL
func New(cfg Config) (*XListRBL, error) {
	// dial grpc connection
	dial, err := grpctls.Dial(cfg.Endpoint, cfg.Client)
	if err != nil {
		return nil, fmt.Errorf("cannot dial with %s: %v", cfg.Endpoint, err)
	}
	//wrapped logger
	logger := wlog{P: clog.NewWithPlugin("xlistrbl")}
	// create xlist client
	client := check.NewClient(
		dial, []xlist.Resource{},
		check.SetLogger(logger),
		check.SetCache(cfg.CacheTTL, cfg.CacheTTL))
	//create rbl
	var d = &XListRBL{
		logger:   logger,
		Zones:    cfg.Zones,
		returnIP: cfg.ReturnIP,
		client:   client,
	}
	return d, nil
}

// Name implements plugin interface
func (r XListRBL) Name() string { return "xlistrbl" }

// Health implements plugin health interface
func (r XListRBL) Health() bool {
	err := r.client.Ping()
	return err == nil
}

// ServeDNS implements the plugin.Handle interface.
func (r XListRBL) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	// only respond to A and TXT queries
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeTXT {
		return plugin.NextOrFailure(r.Name(), r.Next, ctx, writer, query)
	}
	//check if request name matches with zones managed
	qname := req.Name()
	zone := plugin.Zones(r.Zones).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(r.Name(), r.Next, ctx, writer, query)
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
		r.logger.Debugf("error getting resource type %s: %v", qname, err)
		return dns.RcodeRefused, err
	}
	//TODO(luisguillenc): implement ipv6 support
	if resource == xlist.IPv6 {
		r.logger.Debugf("got ipv6 request %s: %v", qname, err)
		return dns.RcodeRefused, err
	}
	//check in blacklist
	response, err := r.client.Check(ctx, checkres, resource)
	if err == xlist.ErrResourceNotSupported {
		r.logger.Debugf("unsupported resource %v for query %s: %v", resource, qname, err)
		return dns.RcodeRefused, err
	}
	if err != nil {
		r.logger.Warnf("error checking %s: %v", checkres, err)
		return dns.RcodeServerFailure, err
	}
	// return result
	if response.Result {
		switch req.QType() {
		case dns.TypeA:
			m := getMsgReplyIP(r.returnIP, response.TTL, &req)
			return dns.RcodeSuccess, req.W.WriteMsg(m)
		case dns.TypeTXT:
			m := getMsgReplyTxt(response.Reason, response.TTL, &req)
			return dns.RcodeSuccess, req.W.WriteMsg(m)
		}
	}
	m := getMsgReplyNXDomain(&req)
	return dns.RcodeNameError, req.W.WriteMsg(m)
}

// Close rbl
func (r *XListRBL) Close() error {
	return r.client.Close()
}
