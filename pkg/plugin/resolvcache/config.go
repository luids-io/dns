// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"errors"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin.
type Config struct {
	Service string
	Policy  RuleSet
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Service: "resolvcache",
		Policy: RuleSet{
			MaxClientRequests: Rule{Log: true},
			MaxNamesResolved:  Rule{Log: true},
		},
	}
}

// Validate configuration.
func (cfg Config) Validate() error {
	if cfg.Service == "" {
		return errors.New("service empty")
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
	"on-maxclient": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		s := strings.Join(args, " ")
		rule, err := ToRule(s)
		if err != nil {
			return c.Errf("invalid on-maxclient: %v", err)
		}
		cfg.Policy.MaxClientRequests = rule
		return nil
	},
	"on-maxnames": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		s := strings.Join(args, " ")
		rule, err := ToRule(s)
		if err != nil {
			return c.Errf("invalid on-maxnames: %v", err)
		}
		cfg.Policy.MaxNamesResolved = rule
		return nil
	},
}
