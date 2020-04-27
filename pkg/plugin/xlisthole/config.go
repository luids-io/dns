// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin
type Config struct {
	Service string
	Policy  RuleSet
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		Service: "xlisthole",
		Policy: RuleSet{
			Domain: Rules{
				Listed:   Rule{Action: ActionInfo{Type: SendNXDomain}, Log: true},
				Unlisted: Rule{Action: ActionInfo{Type: ReturnValue}}},
			IP: Rules{
				Listed:   Rule{Action: ActionInfo{Type: SendNXDomain}, Log: true},
				Unlisted: Rule{Action: ActionInfo{Type: ReturnValue}}},
			OnError: ActionInfo{Type: SendRefused},
		},
	}
}

// Validate configuration
func (cfg Config) Validate() error {
	if cfg.Service == "" {
		return errors.New("service empty")
	}
	err := cfg.Policy.Validate()
	if err != nil {
		return fmt.Errorf("invalid policy config: %v", err)
	}
	return nil
}

// Load configuration from controller
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
	//Policy options
	"listed-domain": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		if args[0] == "merge" {
			cfg.Policy.Domain.Merge = true
			args = args[1:]
		}
		if len(args) > 0 {
			s := strings.Join(args, " ")
			rule, err := ToRule(s)
			if err != nil {
				return c.Errf("in listed-domain: %v", err)
			}
			cfg.Policy.Domain.Listed = rule
		}
		return nil
	},
	"unlisted-domain": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		if args[0] == "merge" {
			return c.Err("in unlisted-domain: merge not available")
		}
		s := strings.Join(args, " ")
		rule, err := ToRule(s)
		if err != nil {
			return c.Errf("in unlisted-domain: %v", err)
		}
		cfg.Policy.Domain.Unlisted = rule
		return nil
	},
	"listed-ip": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		if args[0] == "merge" {
			cfg.Policy.IP.Merge = true
			args = args[1:]
		}
		if len(args) > 0 {
			s := strings.Join(args, " ")
			rule, err := ToRule(s)
			if err != nil {
				return c.Errf("in listed-ip: %v", err)
			}
			cfg.Policy.IP.Listed = rule
		}
		return nil
	},
	"unlisted-ip": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		if args[0] == "merge" {
			return c.Err("in unlisted-ip: merge not available")
		}
		s := strings.Join(args, " ")
		rule, err := ToRule(s)
		if err != nil {
			return c.Errf("in unlisted-ip: %v", err)
		}
		cfg.Policy.IP.Unlisted = rule
		return nil
	},
	"on-error": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) != 1 {
			return c.ArgErr()
		}
		action, err := ToActionInfo(args[0])
		if err != nil {
			return c.Errf("in on-error: %v", err)
		}
		cfg.Policy.OnError = action
		return nil
	},
}
