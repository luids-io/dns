// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"errors"
	"fmt"
	"time"

	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
	"github.com/luids-io/dns/pkg/resolvcache/tracelog"
)

// TraceLogFile is a factory for a cache logfile
func TraceLogFile(cfg *config.ResolvCacheCfg, logger yalogi.Logger) (*tracelog.File, error) {
	if cfg.TraceFile != "" {
		return nil, errors.New("invalid resolv cache config: log file empty")
	}
	return tracelog.NewFile(cfg.TraceFile)
}

// ResolvCache is a factory for a resolv cache service
func ResolvCache(cfg *config.ResolvCacheCfg, clog resolvcache.TraceLogger, logger yalogi.Logger) (*resolvcache.Service, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid resolv cache config: %v", err)
	}
	svc := resolvcache.NewService(
		resolvcache.NewCache(time.Duration(cfg.ExpireSecs)*time.Second, cfg.Limits),
		resolvcache.DumpCache(time.Duration(cfg.DumpSecs)*time.Second, cfg.DumpFile),
		resolvcache.SetTraceLogger(clog),
		resolvcache.SetLogger(logger),
	)
	return svc, nil
}
