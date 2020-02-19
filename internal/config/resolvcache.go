// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package config

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/luids-io/common/util"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// ResolvCacheCfg stores repository settings
type ResolvCacheCfg struct {
	ExpireSecs int
	DumpLog    string
	DumpSecs   int
	Limits     resolvcache.Limits
}

// SetPFlags setups posix flags for commandline configuration
func (cfg *ResolvCacheCfg) SetPFlags(short bool, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	pflag.IntVar(&cfg.ExpireSecs, aprefix+"expire", cfg.ExpireSecs, "Expire time in seconds.")
	pflag.StringVar(&cfg.DumpLog, aprefix+"dumplog", cfg.DumpLog, "Cache dump file for debug.")
	pflag.IntVar(&cfg.DumpSecs, aprefix+"dumpsecs", cfg.DumpSecs, "Dump interval time in seconds.")

	pflag.IntVar(&cfg.Limits.BlockSize, aprefix+"limit.blocksize", cfg.Limits.BlockSize, "Limit Blocksize.")
	pflag.IntVar(&cfg.Limits.MaxBlocksClient, aprefix+"limit.maxblocksclient", cfg.Limits.MaxBlocksClient, "Limit max blocks per client.")
	pflag.IntVar(&cfg.Limits.MaxNamesNode, aprefix+"limit.maxnamesnode", cfg.Limits.MaxNamesNode, "Limit max names per node.")
}

// BindViper setups posix flags for commandline configuration and bind to viper
func (cfg *ResolvCacheCfg) BindViper(v *viper.Viper, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	util.BindViper(v, aprefix+"expire")
	util.BindViper(v, aprefix+"dumplog")
	util.BindViper(v, aprefix+"dumpsecs")

	util.BindViper(v, aprefix+"limit.blocksize")
	util.BindViper(v, aprefix+"limit.maxblocksclient")
	util.BindViper(v, aprefix+"limit.maxnamesnode")
}

// FromViper fill values from viper
func (cfg *ResolvCacheCfg) FromViper(v *viper.Viper, prefix string) {
	aprefix := ""
	if prefix != "" {
		aprefix = prefix + "."
	}
	cfg.ExpireSecs = v.GetInt(aprefix + "expire")
	cfg.DumpLog = v.GetString(aprefix + "dumplog")
	cfg.DumpSecs = v.GetInt(aprefix + "dumpsecs")

	cfg.Limits.BlockSize = v.GetInt(aprefix + "limit.blocksize")
	cfg.Limits.MaxBlocksClient = v.GetInt(aprefix + "limit.maxblocksclient")
	cfg.Limits.MaxNamesNode = v.GetInt(aprefix + "limit.maxnamesnode")
}

// Empty returns true if configuration is empty
func (cfg ResolvCacheCfg) Empty() bool {
	if cfg.ExpireSecs > 0 {
		return false
	}
	if cfg.DumpLog != "" {
		return false
	}
	return true
}

// Validate checks that configuration is ok
func (cfg ResolvCacheCfg) Validate() error {
	return nil
}

// Dump configuration
func (cfg ResolvCacheCfg) Dump() string {
	return fmt.Sprintf("%+v", cfg)
}
