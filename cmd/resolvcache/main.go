// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/luids-io/core/serverd"
	"github.com/luids-io/dns/cmd/resolvcache/config"
)

//Variables for version output
var (
	Program  = "resolvcache"
	Build    = "unknown"
	Version  = "unknown"
	Revision = "unknown"
)

var (
	cfg = config.Default(Program)
	//behaviour
	configFile = ""
	version    = false
	help       = false
	debug      = false
	dryRun     = false
)

func init() {
	//config mapped params
	cfg.PFlags()
	//behaviour params
	pflag.StringVar(&configFile, "config", configFile, "Use explicit config file.")
	pflag.BoolVar(&version, "version", version, "Show version.")
	pflag.BoolVarP(&help, "help", "h", help, "Show this help.")
	pflag.BoolVar(&debug, "debug", debug, "Enable debug.")
	pflag.BoolVar(&dryRun, "dry-run", dryRun, "Checks and construct list but not start service.")
	pflag.Parse()
}

func main() {
	if version {
		fmt.Printf("version: %s\nrevision: %s\nbuild: %s\n", Version, Revision, Build)
		os.Exit(0)
	}
	if help {
		pflag.Usage()
		os.Exit(0)
	}
	// load configuration
	err := cfg.LoadIfFile(configFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// creates logger
	logger, err := createLogger(debug)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// echo version and config
	logger.Infof("%s (version: %s build: %s)", Program, Version, Build)
	if debug {
		logger.Debugf("configuration dump:\n%v", cfg.Dump())
	}

	// creates main server manager instance
	msrv := serverd.New(Program, serverd.SetLogger(logger))

	// create cache logger
	trace, err := createTraceLogger(msrv, logger)
	if err != nil {
		logger.Fatalf("creating cache logger: %v", err)
	}
	// create resolv cache
	cache, err := createResolvCache(trace, msrv, logger)
	if err != nil {
		logger.Fatalf("creating resolv cache: %v", err)
	}

	if dryRun {
		fmt.Println("configuration seems ok")
		os.Exit(0)
	}

	// create checker server
	fgsrv, err := createServer(msrv)
	if err != nil {
		logger.Fatalf("creating check server: %v", err)
	}
	err = createCheckAPI(fgsrv, cache, msrv, logger)
	if err != nil {
		logger.Fatalf("creating check api: %v", err)
	}

	// create collector server
	cgsrv, err := createCollectSrv(msrv)
	if err != nil {
		logger.Fatalf("creating collect server: %v", err)
	}
	err = createCollectAPI(cgsrv, cache, msrv, logger)
	if err != nil {
		logger.Fatalf("creating collect api: %v", err)
	}

	// creates health server
	err = createHealthSrv(msrv, logger)
	if err != nil {
		logger.Fatalf("creating health server: %v", err)
	}

	// run server
	err = msrv.Run()
	if err != nil {
		logger.Errorf("running server: %v", err)
	}
	logger.Infof("%s finished", Program)
}
