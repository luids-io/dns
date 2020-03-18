// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package luidsapi

import (
	"errors"
	"fmt"

	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/luisguillenc/yalogi"

	"github.com/luids-io/core/apiservice"
)

//Plugin is the main struct of the plugin
type Plugin struct {
	logger yalogi.Logger
	cfg    Config

	apisvc  *apiservice.Autoloader
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
		logger: wlog{P: clog.NewWithPlugin("luidsapi")},
	}
	return p, nil
}

// Start plugin
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

// Name implements plugin interface
func (p Plugin) Name() string { return "luidsapi" }

// Health implements plugin health interface
func (p Plugin) Health() bool {
	if !p.started {
		return false
	}
	return p.apisvc.Ping() == nil
}

// Shutdown plugin
func (p *Plugin) Shutdown() error {
	if !p.started {
		return nil
	}
	p.started = false
	return p.apisvc.CloseAll()
}

// GetDiscover returns discover created
func (p *Plugin) GetDiscover() apiservice.Discover {
	return p.apisvc
}

// singleton instance
var discover apiservice.Discover

// SetDiscover main instance
func SetDiscover(d apiservice.Discover) {
	discover = d
}

// GetService returns service using main instance
func GetService(id string) (apiservice.Service, bool) {
	if discover == nil {
		return nil, false
	}
	return discover.GetService(id)
}
