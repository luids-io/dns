// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package config

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/luids-io/common/util"
)

// CollectLogCfg stores collect logger settings
type CollectLogCfg struct {
	File string
}

// SetPFlags setups posix flags for commandline configuration
func (cfg *CollectLogCfg) SetPFlags(short bool, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	pflag.StringVar(&cfg.File, aprefix+"file", cfg.File, "Collect log file.")
}

// BindViper setups posix flags for commandline configuration and bind to viper
func (cfg *CollectLogCfg) BindViper(v *viper.Viper, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	util.BindViper(v, aprefix+"file")
}

// FromViper fill values from viper
func (cfg *CollectLogCfg) FromViper(v *viper.Viper, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	cfg.File = v.GetString(aprefix + "file")
}

// Empty returns true if configuration is empty
func (cfg CollectLogCfg) Empty() bool {
	if cfg.File != "" {
		return false
	}
	return true
}

// Validate checks that configuration is ok
func (cfg CollectLogCfg) Validate() error {
	return nil
}

// Dump configuration
func (cfg CollectLogCfg) Dump() string {
	return fmt.Sprintf("%+v", cfg)
}
