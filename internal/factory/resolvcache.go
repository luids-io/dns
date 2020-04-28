// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"errors"
	"fmt"
	"time"

	"github.com/luids-io/core/utils/yalogi"
	"github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
	"github.com/luids-io/dns/pkg/resolvcache/cachelog"
)

// CacheLogFile is a factory for a cache logfile
func CacheLogFile(cfg *config.ResolvCacheCfg, logger yalogi.Logger) (*cachelog.LogFile, error) {
	if cfg.LogFile != "" {
		return nil, errors.New("invalid resolv cache config: log file empty")
	}
	return cachelog.NewFile(cfg.LogFile)
}

// ResolvCache is a factory for a resolv cache service
func ResolvCache(cfg *config.ResolvCacheCfg, clog resolvcache.CollectLogger, qlog resolvcache.QueryLogger, logger yalogi.Logger) (*resolvcache.Service, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid resolv cache config: %v", err)
	}
	svc := resolvcache.NewService(
		resolvcache.NewCache(time.Duration(cfg.ExpireSecs)*time.Second, cfg.Limits),
		resolvcache.DumpCache(time.Duration(cfg.DumpSecs)*time.Second, cfg.DumpFile),
		resolvcache.SetCollectLogger(clog), resolvcache.SetQueryLogger(qlog),
		resolvcache.SetLogger(logger),
	)
	return svc, nil
}
