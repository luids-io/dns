// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

// Package idsevent implements a CoreDNS plugin that integrates with luIDS
// event system.
package idsevent

import (
	"errors"
	"fmt"

	clog "github.com/coredns/coredns/plugin/pkg/log"

	"github.com/luids-io/api/event"
	"github.com/luids-io/api/event/notifybuffer"
	"github.com/luids-io/core/apiservice"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/pkg/plugin/idsapi"
)

//Plugin is the main struct of the plugin.
type Plugin struct {
	logger yalogi.Logger
	cfg    Config

	svc     apiservice.Service
	buffer  *notifybuffer.Buffer
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
		logger: wlog{P: clog.NewWithPlugin("idsevent")},
	}
	return p, nil
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
	notifier, ok := p.svc.(event.Notifier)
	if !ok {
		return fmt.Errorf("service '%s' is not an event notify api", p.cfg.Service)
	}
	//create client and buffer for async event writes
	p.buffer = notifybuffer.New(notifier, p.cfg.Buffer, notifybuffer.SetLogger(p.logger))
	//register buffer as default event buffer
	event.SetBuffer(p.buffer)
	if p.cfg.Instance != "" {
		event.SetDefaultInstance(p.cfg.Instance)
	}
	p.started = true
	return nil
}

// Name implements plugin interface.
func (p Plugin) Name() string { return "idsevent" }

// Health implements plugin health interface.
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.svc.Ping() == nil
}

// Shutdown plugin.
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	p.buffer.Close()
	return nil
}
