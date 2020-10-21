// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin.
type Config struct {
	Service string
	Policy  RuleSet
	Exclude IPSet
	Views   []View
}

// View stores view configuration
type View struct {
	Service string
	Include IPSet
}

// DefaultConfig returns a Config with default values.
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

// Validate configuration.
func (cfg Config) Validate() error {
	if cfg.Service == "" {
		return errors.New("service empty")
	}
	err := cfg.Policy.Validate()
	if err != nil {
		return fmt.Errorf("invalid policy config: %v", err)
	}
	if len(cfg.Views) > 0 {
		names := make(map[string]bool)
		for _, view := range cfg.Views {
			if view.Service == "" {
				return errors.New("service-view service name is required")
			}
			_, duplicated := names[view.Service]
			if duplicated {
				return fmt.Errorf("service-view %s: is duplicated", view.Service)
			}
			names[view.Service] = true
			if view.Include.Empty() {
				return fmt.Errorf("service-view %s: ips or networks are required", view.Service)
			}
		}
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
	"service-view": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) < 2 {
			return c.ArgErr()
		}
		view := View{}
		view.Service = args[0]
		args = args[1:]
		for _, arg := range args {
			_, cidr, err := net.ParseCIDR(arg)
			if err == nil {
				view.Include.CIDRs = append(view.Include.CIDRs, cidr)
				continue
			}
			ip := net.ParseIP(arg)
			if ip != nil {
				view.Include.IPs = append(view.Include.IPs, ip)
				continue
			}
			return c.SyntaxErr("must be an ip or cidr")
		}
		cfg.Views = append(cfg.Views, view)
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
