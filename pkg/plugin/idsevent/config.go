// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"fmt"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/luisguillenc/grpctls"
)

// Config stores configuration for the plugin
type Config struct {
	Endpoint string
	Client   grpctls.ClientCfg
	Buffer   int
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		Endpoint: "tcp://127.0.0.1:5851",
		Buffer:   100,
	}
}

// Validate configuration
func (cfg Config) Validate() error {
	_, _, err := grpctls.ParseURI(cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpint: %v", err)
	}
	err = cfg.Client.Validate()
	if err != nil {
		return fmt.Errorf("invalid client config: %v", err)
	}
	if cfg.Buffer < 1 {
		return fmt.Errorf("invalid buffer value: %v", cfg.Buffer)
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
