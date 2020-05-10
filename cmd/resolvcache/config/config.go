// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package config

import (
	cconfig "github.com/luids-io/common/config"
	"github.com/luids-io/core/goconfig"
	iconfig "github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// Default returns the default configuration
func Default(program string) *goconfig.Config {
	cfg, err := goconfig.New(program,
		goconfig.Section{
			Name:     "resolvcache",
			Required: true,
			Data: &iconfig.ResolvCacheCfg{
				DumpSecs:   60,
				Limits:     resolvcache.DefaultLimits(),
				ExpireSecs: 3600,
			},
		},
		goconfig.Section{
			Name:     "server",
			Required: true,
			Short:    true,
			Data: &cconfig.ServerCfg{
				ListenURI: "tcp://127.0.0.1:5891",
			},
		},
		goconfig.Section{
			Name:     "server.collect",
			Required: false,
			Data:     &cconfig.ServerCfg{},
		},
		goconfig.Section{
			Name:     "server.check",
			Required: false,
			Data:     &cconfig.ServerCfg{},
		},
		goconfig.Section{
			Name:     "log",
			Required: true,
			Data: &cconfig.LoggerCfg{
				Level: "info",
			},
		},
		goconfig.Section{
			Name:     "health",
			Required: false,
			Data:     &cconfig.HealthCfg{},
		},
	)
	if err != nil {
		panic(err)
	}
	return cfg
}
