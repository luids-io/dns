// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"errors"
	"fmt"

	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/luisguillenc/grpctls"
	"github.com/luisguillenc/yalogi"

	"github.com/luids-io/api/event/notify"
	"github.com/luids-io/core/event"
	"github.com/luids-io/core/event/buffer"
)

//Plugin is the main struct of the plugin
type Plugin struct {
	logger  yalogi.Logger
	cfg     Config
	buffer  *buffer.Buffer
	client  *notify.Client
	started bool
}

// New returns a new Plugin
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

// Start plugin
func (p *Plugin) Start() error {
	if p.started {
		return errors.New("plugin started")
	}
	//create dial
	dial, err := grpctls.Dial(p.cfg.Endpoint, p.cfg.Client)
	if err != nil {
		return fmt.Errorf("cannot dial with %s: %v", p.cfg.Endpoint, err)
	}
	//create client and buffer for async event writes
	p.client = notify.NewClient(dial, notify.SetLogger(p.logger))
	p.buffer = buffer.New(p.client, p.cfg.Buffer, buffer.SetLogger(p.logger))
	//register buffer as default event buffer
	event.SetBuffer(p.buffer)
	p.started = true
	return nil
}

// Name implements plugin interface
func (p Plugin) Name() string { return "idsevent" }

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
	p.buffer.Close()
	return p.client.Close()
}
