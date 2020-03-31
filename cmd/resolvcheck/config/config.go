// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package config

import (
	cconfig "github.com/luids-io/common/config"
	"github.com/luids-io/core/utils/goconfig"
)

// Default returns the default configuration
func Default(program string) *goconfig.Config {
	cfg, err := goconfig.New(program,
		goconfig.Section{
			Name:     "",
			Required: true,
			Data: &cconfig.ClientCfg{
				RemoteURI: "tcp://127.0.0.1:5892",
			},
		},
		goconfig.Section{
			Name:     "log",
			Required: true,
			Data: &cconfig.LoggerCfg{
				Level: "info",
			},
		},
	)
	if err != nil {
		panic(err)
	}
	return cfg
}
