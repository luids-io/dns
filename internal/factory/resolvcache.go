// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"fmt"
	"time"

	"github.com/luids-io/core/utils/yalogi"
	"github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// ResolvCache is a factory for a resolv cache service
func ResolvCache(cfg *config.ResolvCacheCfg, clogger resolvcache.CollectLogger, logger yalogi.Logger) (*resolvcache.Service, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid resolv cache config: %v", err)
	}
	svc := resolvcache.NewService(
		resolvcache.NewCache(time.Duration(cfg.ExpireSecs)*time.Second, cfg.Limits),
		resolvcache.DumpCache(time.Duration(cfg.DumpSecs)*time.Second, cfg.DumpLog),
		resolvcache.SetCollectLogger(clogger),
		resolvcache.SetLogger(logger),
	)
	return svc, nil
}
