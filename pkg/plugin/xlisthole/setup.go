// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() {
	caddy.RegisterPlugin("xlisthole", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	f, err := createXListHole(c)
	if err != nil {
		return plugin.Error("xlisthole", err)
	}
	c.OnStartup(func() error {
		f.metrics.register(c)
		return nil
	})
	c.OnShutdown(func() error {
		return f.Close()
	})
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		f.Next = next
		return f
	})

	return nil
}

func createXListHole(c *caddy.Controller) (*XListHole, error) {
	cfg := DefaultConfig()
	err := cfg.Load(c)
	if err != nil {
		return nil, err
	}
	xhole, err := New(cfg)
	if err != nil {
		return nil, c.Err(err.Error())
	}
	return xhole, nil
}
