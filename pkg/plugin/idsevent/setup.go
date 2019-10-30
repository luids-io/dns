// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

func init() {
	caddy.RegisterPlugin("idsevent", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

// setup function creates a new instance and register to controller
func setup(c *caddy.Controller) error {
	p, err := createPlugin(c)
	if err != nil {
		return plugin.Error("idsevent", err)
	}
	c.OnStartup(func() error {
		return p.Start()
	})
	c.OnShutdown(func() error {
		return p.Shutdown()
	})
	return nil
}

// creates a plugin from a controller
func createPlugin(c *caddy.Controller) (*Plugin, error) {
	config := DefaultConfig()
	err := config.Load(c)
	if err != nil {
		return nil, err
	}
	//create archiver plugin
	p, err := New(config)
	if err != nil {
		return nil, c.Err(err.Error())
	}
	return p, nil
}
