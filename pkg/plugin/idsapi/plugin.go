// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

// Package idsapi implements a CoreDNS plugin that loads luIDS api services.
package idsapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/google/uuid"
	"github.com/miekg/dns"

	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/yalogi"
)

//Plugin is the main struct of the plugin.
type Plugin struct {
	Next plugin.Handler
	Fall fall.F
	//internal
	logger  yalogi.Logger
	cfg     Config
	apisvc  *apiservice.Autoloader
	started bool
}

// New returns a new Plugin.
func New(cfg Config) (*Plugin, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		cfg:    cfg,
		logger: wlog{P: clog.NewWithPlugin("idsapi")},
	}
	return p, nil
}

// Start plugin.
func (p *Plugin) Start() error {
	if p.started {
		return errors.New("plugin started")
	}
	defs, err := getServiceDefs(p.cfg)
	if err != nil {
		return fmt.Errorf("loading servicedefs: %v", err)
	}
	p.apisvc = apiservice.NewAutoloader(defs, apiservice.SetLogger(p.logger))
	p.started = true
	return nil
}

// ServeDNS implements the plugin.Handle interface.
func (p Plugin) ServeDNS(ctx context.Context, writer dns.ResponseWriter, query *dns.Msg) (int, error) {
	if !p.started {
		return dns.RcodeServerFailure, errors.New("plugin not started")
	}
	//creates a new uuid and set in context
	newuid, err := uuid.NewRandom()
	if err != nil {
		p.logger.Warnf("can't generate a new uuid: %v", err)
	}
	ctx = SetRequestID(ctx, newuid)
	return plugin.NextOrFailure(p.Name(), p.Next, ctx, writer, query)
}

// Name implements plugin interface.
func (p Plugin) Name() string { return "idsapi" }

// Health implements plugin health interface.
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.apisvc.Ping() == nil
}

// Shutdown plugin.
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.apisvc.CloseAll()
}

// GetDiscover returns discover created.
func (p *Plugin) GetDiscover() apiservice.Discover {
	return p.apisvc
}

// singleton instance
var discover apiservice.Discover

// SetDiscover main instance.
func SetDiscover(d apiservice.Discover) {
	discover = d
}

// GetService returns service using main instance.
func GetService(id string) (apiservice.Service, bool) {
	if discover == nil {
		return nil, false
	}
	return discover.GetService(id)
}
