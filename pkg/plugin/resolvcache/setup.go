// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

// init function registers plugin
func init() {
	caddy.RegisterPlugin("resolvcache", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	p, err := createPlugin(c)
	if err != nil {
		return plugin.Error("resolvcache", err)
	}
	c.OnStartup(func() error {
		return p.Start()
	})
	c.OnShutdown(func() error {
		return p.Shutdown()
	})
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		p.Next = next
		return p
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
