// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlistrbl

import (
	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() {
	caddy.RegisterPlugin("xlistrbl", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	rbl, err := createRBL(c)
	if err != nil {
		return plugin.Error("xlistrbl", err)
	}
	c.OnShutdown(func() error {
		return rbl.Close()
	})
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		rbl.Next = next
		return rbl
	})
	return nil
}

func createRBL(c *caddy.Controller) (*XListRBL, error) {
	cfg := DefaultConfig()
	err := cfg.Load(c)
	if err != nil {
		return nil, err
	}
	rbl, err := New(cfg)
	if err != nil {
		return nil, c.Err(err.Error())
	}
	return rbl, nil
}
