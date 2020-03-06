// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"fmt"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/luisguillenc/serverd"
	"github.com/luisguillenc/yalogi"
	"google.golang.org/grpc"

	apicheck "github.com/luids-io/api/dnsutil/resolvcheck"
	apicollect "github.com/luids-io/api/dnsutil/resolvcollect"
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

func createCollectLogger(msrv *serverd.Manager, logger yalogi.Logger) (resolvcache.CollectLogger, error) {
	cfgCollectLog := cfg.Data("collectlog").(*iconfig.CollectLogCfg)
	//creates collection logger if defined
	var clogger resolvcache.CollectLogger
	if cfgCollectLog.File != "" {
		cfile := collectfile.New(cfgCollectLog.File)
		msrv.Register(serverd.Service{
			Name:     "collectlog.service",
			Start:    cfile.Start,
			Shutdown: cfile.Stop,
		})
		clogger = cfile
	}
	return clogger, nil
}

func createResolvCache(clogger resolvcache.CollectLogger, msrv *serverd.Manager, logger yalogi.Logger) (*resolvcache.Service, error) {
	cfgRCache := cfg.Data("resolvcache").(*iconfig.ResolvCacheCfg)
	cache, err := ifactory.ResolvCache(cfgRCache, clogger, logger)
	if err != nil {
		return nil, err
	}
	msrv.Register(serverd.Service{
		Name:     "resolvcache.service",
		Start:    cache.Start,
		Shutdown: cache.Shutdown,
	})
	return cache, nil
}

func createCollectAPI(gsrv *grpc.Server, csvc *resolvcache.Service, logger yalogi.Logger) error {
	gsvc, err := ifactory.ResolvCollectAPI(csvc, logger)
	if err != nil {
		return fmt.Errorf("creating collectapi service: %v", err)
	}
	apicollect.RegisterServer(gsrv, gsvc)
	return nil
}

func createCollectSrv(msrv *serverd.Manager) (*grpc.Server, error) {
	cfgServer := cfg.Data("server-collect").(*cconfig.ServerCfg)
	glis, gsrv, err := cfactory.Server(cfgServer)
	if err != nil {
		return nil, err
	}
	if cfgServer.Metrics {
		grpc_prometheus.Register(gsrv)
	}
	msrv.Register(serverd.Service{
		Name:     "resolvcache-collect.server",
		Start:    func() error { go gsrv.Serve(glis); return nil },
		Shutdown: gsrv.GracefulStop,
		Stop:     gsrv.Stop,
	})
	return gsrv, nil
}

func createCheckAPI(gsrv *grpc.Server, csvc *resolvcache.Service, logger yalogi.Logger) error {
	gsvc, err := ifactory.ResolvCheckAPI(csvc, logger)
	if err != nil {
		return fmt.Errorf("creating checkapi service: %v", err)
	}
	apicheck.RegisterServer(gsrv, gsvc)
	return nil
}

func createCheckSrv(msrv *serverd.Manager) (*grpc.Server, error) {
	cfgServer := cfg.Data("server-check").(*cconfig.ServerCfg)
	glis, gsrv, err := cfactory.Server(cfgServer)
	if err != nil {
		return nil, err
	}
	if cfgServer.Metrics {
		grpc_prometheus.Register(gsrv)
	}
	msrv.Register(serverd.Service{
		Name:     "resolvcache-check.server",
		Start:    func() error { go gsrv.Serve(glis); return nil },
		Shutdown: gsrv.GracefulStop,
		Stop:     gsrv.Stop,
	})
	return gsrv, nil
}
