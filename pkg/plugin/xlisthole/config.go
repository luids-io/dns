// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"strconv"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/luisguillenc/grpctls"
)

// Config stores configuration for the plugin
type Config struct {
	Endpoint string
	Client   grpctls.ClientCfg
	CacheTTL int
	Policy   RuleSet
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		Endpoint: "tcp://127.0.0.1:5801",
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
	err := cfg.Client.Validate()
	if err != nil {
		return c.Errf("invalid client config: %v", err)
	}
	err = cfg.Policy.Validate()
	if err != nil {
		return c.Errf("invalid policy config: %v", err)
	}
	return nil
}

type loadCfgFn func(c *caddy.Controller, cfg *Config) error

// main configuration parse map
var mapConfig = map[string]loadCfgFn{
	"endpoint": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		value := c.Val()
		_, _, err := grpctls.ParseURI(value)
		if err != nil {
			return c.Errf("invalid endpoint '%s'", value)
		}
		cfg.Endpoint = value
		return nil
	},
	"cache": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		value, err := strconv.Atoi(c.Val())
		if err != nil {
			return c.Errf("invalid cache value '%s'", c.Val())
		}
		cfg.CacheTTL = value
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
	//Client options
	"clientcert": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Client.CertFile = c.Val()
		return nil
	},
	"clientkey": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Client.KeyFile = c.Val()
		return nil
	},
	"servercert": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Client.ServerCert = c.Val()
		return nil
	},
	"servername": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Client.ServerName = c.Val()
		return nil
	},
	"cacert": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Client.CACert = c.Val()
		return nil
	},
	"systemca": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		switch strings.ToLower(c.Val()) {
		case "true":
			cfg.Client.UseSystemCAs = true
		case "false":
			cfg.Client.UseSystemCAs = false
		default:
			return c.Err("invalid systemca value")
		}
		return nil
	},
}
