// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlistrbl

import (
	"errors"
	"net"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin.
type Config struct {
	Service  string
	Zones    []string
	ReturnIP string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Service:  "xlistrbl",
		ReturnIP: "127.0.0.69",
		Zones:    make([]string, 0),
	}
}

// Validate configuration.
func (cfg Config) Validate() error {
	if cfg.Service == "" {
		return errors.New("service empty")
	}
	ip := net.ParseIP(cfg.ReturnIP)
	if ip == nil {
		return errors.New("invalid returnip")
	}
	return nil
}

// Load configuration from controller.
func (cfg *Config) Load(c *caddy.Controller) error {
	//parse configuration
	i := 0
	for c.Next() {
		if i > 0 {
			return plugin.ErrOnce
		}
		i++
		// parse zones
		args := c.RemainingArgs()
		cfg.Zones = make([]string, len(c.ServerBlockKeys))
		copy(cfg.Zones, c.ServerBlockKeys)
		if len(args) > 0 {
			cfg.Zones = args
		}
		for i := range cfg.Zones {
			cfg.Zones[i] = plugin.Host(cfg.Zones[i]).Normalize()
		}
		if c.NextBlock() {
			for {
				apply, ok := mapConfig[c.Val()]
				if ok {
					apply(c, cfg)
				} else {
					if c.Val() != "}" {
						return c.Errf("unknown property '%s'", c.Val())
					}
				}
				if !c.Next() {
					break
				}
			}
		}
	}
	return nil
}

type loadCfgFn func(c *caddy.Controller, cfg *Config) error

// main configuration parse map
var mapConfig = map[string]loadCfgFn{
	"service": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Service = c.Val()
		return nil
	},
	"returnip": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		value := c.Val()
		if !isIPv4(value) {
			return c.Errf("invalid returnip value '%s'", value)
		}
		cfg.ReturnIP = value
		return nil
	},
}
