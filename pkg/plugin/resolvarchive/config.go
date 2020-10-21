// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvarchive

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin.
type Config struct {
	Service string
	Buffer  int
	//server ip used for storage info
	ServerIP net.IP
	Exclude  IPSet
	IgnoreRC []int
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Service: "resolvarchive",
		Buffer:  100,
	}
}

// Validate configuration.
func (cfg Config) Validate() error {
	if cfg.Service == "" {
		return errors.New("service empty")
	}
	if cfg.Buffer < 1 {
		return fmt.Errorf("invalid buffer value: %v", cfg.Buffer)
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
		if c.NextBlock() {
			for {
				apply, ok := mapConfig[c.Val()]
				if ok {
					err := apply(c, cfg)
					if err != nil {
						return err
					}
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
	"server-ip": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		ip := net.ParseIP(c.Val())
		if ip == nil {
			return c.Err("invalid server-ip")
		}
		cfg.ServerIP = ip
		return nil
	},
	"exclude": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		for _, arg := range args {
			_, cidr, err := net.ParseCIDR(arg)
			if err == nil {
				cfg.Exclude.CIDRs = append(cfg.Exclude.CIDRs, cidr)
				continue
			}
			ip := net.ParseIP(arg)
			if ip != nil {
				cfg.Exclude.IPs = append(cfg.Exclude.IPs, ip)
				continue
			}
			return c.SyntaxErr("must be an ip or cidr")
		}
		return nil
	},
	"ignore-rc": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		for _, arg := range args {
			rc, err := strconv.Atoi(arg)
			if err != nil {
				return c.SyntaxErr("must be an integer")
			}
			cfg.IgnoreRC = append(cfg.IgnoreRC, rc)
		}
		return nil
	},
}
