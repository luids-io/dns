// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"

	"github.com/luids-io/common/util"
)

// Config stores configuration for the plugin
type Config struct {
	ConfigDirs  []string
	ConfigFiles []string
	CertsDir    string
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		ConfigFiles: []string{"/etc/luids/services.json"},
	}
}

// Validate configuration
func (cfg Config) Validate() error {
	empty := true
	for _, file := range cfg.ConfigFiles {
		if !util.FileExists(file) {
			return fmt.Errorf("config file '%s' doesn't exists", file)
		}
		if !strings.HasSuffix(file, ".json") {
			return fmt.Errorf("config file '%s' without .json extension", file)
		}
		empty = false
	}
	for _, dir := range cfg.ConfigDirs {
		if !util.DirExists(dir) {
			return fmt.Errorf("config dir '%s' doesn't exists", dir)
		}
		empty = false
	}
	if empty {
		return errors.New("config required")
	}
	if cfg.CertsDir != "" && !util.DirExists(cfg.CertsDir) {
		return fmt.Errorf("certificates dir '%v' doesn't exists", cfg.CertsDir)
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
	"files": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		cfg.ConfigFiles = make([]string, len(args), len(args))
		copy(cfg.ConfigFiles, args)
		return nil
	},
	"dirs": func(c *caddy.Controller, cfg *Config) error {
		args := c.RemainingArgs()
		if len(args) == 0 {
			return c.ArgErr()
		}
		cfg.ConfigDirs = make([]string, len(args), len(args))
		copy(cfg.ConfigDirs, args)
		return nil
	},
	"certsdir": func(c *caddy.Controller, cfg *Config) error {
		if !c.NextArg() {
			return c.ArgErr()
		}
		cfg.CertsDir = c.Val()
		return nil
	},
}
