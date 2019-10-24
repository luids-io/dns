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

// setup function creates a new instance and register to controller
func setup(c *caddy.Controller) error {
	col, err := createCollector(c)
	if err != nil {
		return plugin.Error("resolvcache", err)
	}
	c.OnShutdown(func() error {
		col.Close()
		return nil
	})
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		col.Next = next
		return col
	})
	return nil
}

// creates a collector from a controller
func createCollector(c *caddy.Controller) (*Collector, error) {
	config := DefaultConfig()
	err := config.Load(c)
	if err != nil {
		return nil, err
	}
	col, err := New(config)
	if err != nil {
		return nil, c.Err(err.Error())
	}
	return col, nil
}
