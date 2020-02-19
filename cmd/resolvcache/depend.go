// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/luisguillenc/serverd"
	"github.com/luisguillenc/yalogi"

	"github.com/luids-io/api/dnsutil/resolvcheck"
	"github.com/luids-io/api/dnsutil/resolvcollect"
	cconfig "github.com/luids-io/common/config"
	cfactory "github.com/luids-io/common/factory"
	iconfig "github.com/luids-io/dns/internal/config"
	ifactory "github.com/luids-io/dns/internal/factory"
	"github.com/luids-io/dns/pkg/resolvcache"
	"github.com/luids-io/dns/pkg/resolvcache/collectfile"
)

func createLogger(debug bool) (yalogi.Logger, error) {
	cfgLog := cfg.Data("log").(*cconfig.LoggerCfg)
	return cfactory.Logger(cfgLog, debug)
}

func createCollectLogger(srv *serverd.Manager, logger yalogi.Logger) (resolvcache.CollectLogger, error) {
	cfgCollectLog := cfg.Data("collectlog").(*iconfig.CollectLogCfg)
	//creates collection logger if defined
	var clogger resolvcache.CollectLogger
	if cfgCollectLog.File != "" {
		cfile := collectfile.New(cfgCollectLog.File)
		srv.Register(serverd.Service{
			Name:     "collectlog.service",
			Start:    cfile.Start,
			Shutdown: cfile.Stop,
		})
		clogger = cfile
	}
	return clogger, nil
}

func createResolvCache(clogger resolvcache.CollectLogger, srv *serverd.Manager, logger yalogi.Logger) (*resolvcache.Service, error) {
	cfgRCache := cfg.Data("cache").(*iconfig.ResolvCacheCfg)
	//creates resolv cache
	cache, err := ifactory.ResolvCache(cfgRCache, clogger, logger)
	if err != nil {
		return nil, err
	}
	srv.Register(serverd.Service{
		Name:     "resolvcache.service",
		Start:    cache.Start,
		Shutdown: cache.Shutdown,
	})
	return cache, nil
}

func createCollectSrv(cache *resolvcache.Service, srv *serverd.Manager) error {
	cfgCollect := cfg.Data("grpc-collect").(*cconfig.ServerCfg)
	cglis, cgsrv, err := cfactory.Server(cfgCollect)
	if err != nil {
		return err
	}
	// register service
	resolvcollect.RegisterServer(cgsrv, resolvcollect.NewService(cache))
	if cfgCollect.Metrics {
		grpc_prometheus.Register(cgsrv)
	}
	srv.Register(serverd.Service{
		Name:     "grpc-collect.server",
		Start:    func() error { go cgsrv.Serve(cglis); return nil },
		Shutdown: cgsrv.GracefulStop,
		Stop:     cgsrv.Stop,
	})
	return nil
}

func createCheckSrv(srv *serverd.Manager, resolvCache *resolvcache.Service) error {
	cfgCheck := cfg.Data("grpc-check").(*cconfig.ServerCfg)
	fglis, fgsrv, err := cfactory.Server(cfgCheck)
	if err != nil {
		return err
	}
	// register service
	resolvcheck.RegisterServer(fgsrv, resolvcheck.NewService(resolvCache))
	if cfgCheck.Metrics {
		grpc_prometheus.Register(fgsrv)
	}
	srv.Register(serverd.Service{
		Name:     "grpc-check.server",
		Start:    func() error { go fgsrv.Serve(fglis); return nil },
		Shutdown: fgsrv.GracefulStop,
		Stop:     fgsrv.Stop,
	})
	return nil
}

func createHealthSrv(srv *serverd.Manager, logger yalogi.Logger) error {
	cfgHealth := cfg.Data("health").(*cconfig.HealthCfg)
	if !cfgHealth.Empty() {
		hlis, health, err := cfactory.Health(cfgHealth, srv, logger)
		if err != nil {
			logger.Fatalf("creating health server: %v", err)
		}
		srv.Register(serverd.Service{
			Name:     "health.server",
			Start:    func() error { go health.Serve(hlis); return nil },
			Shutdown: func() { health.Close() },
		})
	}
	return nil
}
