// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"
	"github.com/miekg/dns"

	"github.com/luids-io/core/dnsutil"
	"github.com/luids-io/core/dnsutil/resolvcollect"
	"github.com/luids-io/core/event"
	"github.com/luids-io/core/event/codes"
)

//Collector is the main struct of the plugin
type Collector struct {
	Next   plugin.Handler
	Fall   fall.F
	logger yalogi.Logger

	//policy defined for error management
	policy RuleSet
	//internal buffered data channel
	collectCh chan resolvData
	//client used for collect
	client *resolvcollect.Client
	//control state
	closed   bool
	sigclose chan struct{}
}

// New returns a new Collector
func New(cfg Config) (*Collector, error) {
	dial, err := grpctls.Dial(cfg.Endpoint, cfg.Client)
	if err != nil {
		return nil, fmt.Errorf("cannot dial with %s: %v", cfg.Endpoint, err)
	}
	//creates logger
	logger := wlog{P: clog.NewWithPlugin("resolvcache")}
	//create collector
	client := resolvcollect.NewClient(dial, resolvcollect.SetLogger(logger))
	var d = &Collector{
		logger:    logger,
		client:    client,
		collectCh: make(chan resolvData, cfg.Buffer),
		sigclose:  make(chan struct{}),
		policy:    cfg.Policy,
	}
	go d.doProcess()
	return d, nil
}

// ServeDNS implements the plugin.Handle interface.
func (c Collector) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	req := request.Request{W: writer, Req: query}
	if req.QType() != dns.TypeA && req.QType() != dns.TypeAAAA {
		return plugin.NextOrFailure(c.Name(), c.Next, ctx, writer, query)
	}
	// if A or AAAA gets response
	rrw := dnstest.NewRecorder(writer)
	rc, err := plugin.NextOrFailure(c.Name(), c.Next, ctx, rrw, query)
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
		remote, _, _ := net.SplitHostPort(req.RemoteAddr())
		client := net.ParseIP(remote)
		if client == nil {
			c.logger.Warnf("parsing remote '%s'", req.RemoteAddr())
			return rc, err
		}
		// collect data
		c.Collect(client, name, resolved)
	}
	return rc, err
}

// Name implements plugin interface
func (c Collector) Name() string { return "resolvcache" }

// Health implements plugin health interface
func (c Collector) Health() bool {
	err := c.client.Ping()
	return err == nil
}

// Collect data in an asyncronous mode
func (c *Collector) Collect(client net.IP, name string, resolved []net.IP) {
	if !c.closed {
		c.collectCh <- resolvData{client: client, name: name, resolved: resolved}
	}
}

// Close collector
func (c *Collector) Close() {
	if c.closed {
		return
	}
	c.closed = true
	close(c.collectCh)
	<-c.sigclose
	c.client.Close()
}

type resolvData struct {
	client   net.IP
	name     string
	resolved []net.IP
}

func (c *Collector) doProcess() {
	for data := range c.collectCh {
		err := c.client.Collect(context.Background(), data.client, data.name, data.resolved)
		if err != nil {
			//apply policy management error
			switch err {
			case dnsutil.ErrCollectDNSClientLimit:
				if c.policy.MaxClientRequests.Log {
					c.logger.Infof("%v", err)
				}
				if c.policy.MaxClientRequests.Event.Raise {
					level := c.policy.MaxClientRequests.Event.Level
					e := event.New(event.Security, codes.DNSMaxClientRequests, level)
					e.Set("remote", data.client)
					event.Notify(e)
				}
			case dnsutil.ErrCollectNamesLimit:
				if c.policy.MaxNamesResolved.Log {
					c.logger.Infof("%v", err)
				}
				if c.policy.MaxNamesResolved.Event.Raise {
					level := c.policy.MaxNamesResolved.Event.Level
					e := event.New(event.Security, codes.DNSMaxNamesResolvedIP, level)
					e.Set("remote", data.client)
					e.Set("resolved", data.resolved)
					event.Notify(e)
				}
			default:
				c.logger.Warnf("%v", err)
			}
		}
	}
	close(c.sigclose)
}
