// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
)

// Config stores configuration for the plugin.
type Config struct {
	Service  string
	Buffer   int
	Instance string
	WaitDups int
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Service: "idsevent",
		Buffer:  1024,
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
	"instance": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.Instance = c.Val()
		return nil
	},
	"buffer": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		var err error
		cfg.Buffer, err = strconv.Atoi(c.Val())
		if err != nil {
			return c.SyntaxErr("buffer must be an integer")
		}
		if cfg.Buffer < 0 {
			return c.SyntaxErr("buffer must be greater than zero")
		}
		return nil
	},
	"waitdups": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		var err error
		cfg.WaitDups, err = strconv.Atoi(c.Val())
		if err != nil {
			return c.SyntaxErr("waitdups must be an integer")
		}
		if cfg.WaitDups < 0 {
			return c.SyntaxErr("waitdups must be greater than zero")
		}
		return nil
	},
}
