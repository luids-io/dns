// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"fmt"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"

	apicheck "github.com/luids-io/api/dnsutil/grpc/resolvcheck"
	apicollect "github.com/luids-io/api/dnsutil/grpc/resolvcollect"
	cconfig "github.com/luids-io/common/config"
	cfactory "github.com/luids-io/common/factory"
	"github.com/luids-io/core/serverd"
	"github.com/luids-io/core/yalogi"
	iconfig "github.com/luids-io/dns/internal/config"
	ifactory "github.com/luids-io/dns/internal/factory"
	"github.com/luids-io/dns/pkg/resolvcache"
	"github.com/luids-io/dns/pkg/resolvcache/tracelog"
)

func createLogger(debug bool) (yalogi.Logger, error) {
	cfgLog := cfg.Data("log").(*cconfig.LoggerCfg)
	return cfactory.Logger(cfgLog, debug)
}

func createHealthSrv(msrv *serverd.Manager, logger yalogi.Logger) error {
	cfgHealth := cfg.Data("health").(*cconfig.HealthCfg)
	if !cfgHealth.Empty() {
		hlis, health, err := cfactory.Health(cfgHealth, msrv, logger)
		if err != nil {
			logger.Fatalf("creating health server: %v", err)
		}
		msrv.Register(serverd.Service{
			Name:     fmt.Sprintf("health.[%s]", cfgHealth.ListenURI),
			Start:    func() error { go health.Serve(hlis); return nil },
			Shutdown: func() { health.Close() },
		})
	}
	return nil
}

func createTraceLogger(msrv *serverd.Manager, logger yalogi.Logger) (resolvcache.TraceLogger, error) {
	cfgRCache := cfg.Data("resolvcache").(*iconfig.ResolvCacheCfg)
	var trace resolvcache.TraceLogger
	if cfgRCache.TraceFile != "" {
		cfile, err := tracelog.NewFile(cfgRCache.TraceFile)
		if err != nil {
			return nil, err
		}
		msrv.Register(serverd.Service{
			Name:     "tracelog",
			Shutdown: func() { cfile.Close() },
		})
		trace = cfile
	}
	return trace, nil
}

func createResolvCache(trace resolvcache.TraceLogger, msrv *serverd.Manager, logger yalogi.Logger) (*resolvcache.Service, error) {
	cfgRCache := cfg.Data("resolvcache").(*iconfig.ResolvCacheCfg)
	cache, err := ifactory.ResolvCache(cfgRCache, trace, logger)
	if err != nil {
		return nil, err
	}
	msrv.Register(serverd.Service{
		Name:     "resolvcache",
		Start:    cache.Start,
		Shutdown: cache.Shutdown,
	})
	return cache, nil
}

func createCollectAPI(gsrv *grpc.Server, csvc *resolvcache.Service, msrv *serverd.Manager, logger yalogi.Logger) error {
	cfgAPI := cfg.Data("service.dnsutil.resolvcollect").(*iconfig.ResolvCollectAPICfg)
	if cfgAPI.Enable {
		gsvc, err := ifactory.ResolvCollectAPI(cfgAPI, csvc, logger)
		if err != nil {
			return err
		}
		apicollect.RegisterServer(gsrv, gsvc)
		msrv.Register(serverd.Service{Name: "service.dnsutil.resolvcollect"})
	}
	return nil
}

func createCheckAPI(gsrv *grpc.Server, csvc *resolvcache.Service, msrv *serverd.Manager, logger yalogi.Logger) error {
	cfgAPI := cfg.Data("service.dnsutil.resolvcheck").(*iconfig.ResolvCheckAPICfg)
	if cfgAPI.Enable {
		gsvc, err := ifactory.ResolvCheckAPI(cfgAPI, csvc, logger)
		if err != nil {
			return err
		}
		apicheck.RegisterServer(gsrv, gsvc)
		msrv.Register(serverd.Service{Name: "service.dnsutil.resolvcheck"})
	}
	return nil
}

func createServer(msrv *serverd.Manager) (*grpc.Server, error) {
	cfgServer := cfg.Data("server").(*cconfig.ServerCfg)
	glis, gsrv, err := cfactory.Server(cfgServer)
	if err == cfactory.ErrURIServerExists {
		return gsrv, nil
	}
	if err != nil {
		return nil, err
	}
	if cfgServer.Metrics {
		grpc_prometheus.Register(gsrv)
	}
	msrv.Register(serverd.Service{
		Name:     fmt.Sprintf("server.[%s]", cfgServer.ListenURI),
		Start:    func() error { go gsrv.Serve(glis); return nil },
		Shutdown: gsrv.GracefulStop,
		Stop:     gsrv.Stop,
	})
	return gsrv, nil
}

func createCollectSrv(msrv *serverd.Manager) (*grpc.Server, error) {
	cfgServer := cfg.Data("server.collect").(*cconfig.ServerCfg)
	if cfgServer.Empty() {
		cfgServer = cfg.Data("server").(*cconfig.ServerCfg)
	}
	glis, gsrv, err := cfactory.Server(cfgServer)
	if err == cfactory.ErrURIServerExists {
		return gsrv, nil
	}
	if err != nil {
		return nil, err
	}
	if cfgServer.Metrics {
		grpc_prometheus.Register(gsrv)
	}
	msrv.Register(serverd.Service{
		Name:     fmt.Sprintf("server.collect.[%s]", cfgServer.ListenURI),
		Start:    func() error { go gsrv.Serve(glis); return nil },
		Shutdown: gsrv.GracefulStop,
		Stop:     gsrv.Stop,
	})
	return gsrv, nil
}
