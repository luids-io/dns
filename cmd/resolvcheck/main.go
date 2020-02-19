// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/luids-io/dns/cmd/resolvcheck/config"
)

//Variables for version output
var (
	Program  = "resolvcheck"
	Build    = "unknown"
	Version  = "unknown"
	Revision = "unknown"
)

//Variables for configuration
var (
	cfg = config.Default(Program)
	//behaviour
	configFile = ""
	version    = false
	help       = false
	debug      = false
	//input
	inStdin = false
	inFile  = ""
)

func init() {
	//config mapped params
	cfg.PFlags()
	//behaviour params
	pflag.StringVar(&configFile, "config", configFile, "Use explicit config file.")
	pflag.BoolVar(&version, "version", version, "Show version.")
	pflag.BoolVarP(&help, "help", "h", help, "Show this help.")
	pflag.BoolVar(&debug, "debug", debug, "Enable debug.")
	//input params
	pflag.BoolVar(&inStdin, "stdin", inStdin, "From stdin.")
	pflag.StringVarP(&inFile, "file", "f", inFile, "File for input.")
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
	// check args
	if len(pflag.Args()) == 0 && !inStdin && inFile == "" {
		fmt.Fprintln(os.Stderr, "required query data")
		os.Exit(1)
	}

	// creates logger
	logger, err := createLogger(debug)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// create grpc client
	client, err := createClient(logger)
	if err != nil {
		logger.Fatalf("couldn't create client: %v", err)
	}
	defer client.Close()

	// process args
	if !inStdin && inFile == "" {
		for _, arg := range pflag.Args() {
			data, err := getValues(arg)
			if err != nil {
				logger.Fatalf("%v", err)
			}
			startc := time.Now()
			resp, err := client.Check(context.Background(), data.client, data.resolved, data.name)
			if err != nil {
				logger.Fatalf("Check '%s' returned error: %v", arg, err)
			}
			fmt.Fprintf(os.Stdout, "%v,%v,%s: %v,%v,%v (%v)\n", data.client, data.resolved, data.name, resp.Result, resp.Last, resp.Store, time.Since(startc))
		}
		return
	}

	// read from file or stdin
	reader := os.Stdin
	if inFile != "" {
		file, err := os.Open(inFile)
		if err != nil {
			logger.Fatalf("opening file: %v", err)
		}
		defer file.Close()
		reader = file
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		data, err := getValues(line)
		if err != nil {
			logger.Fatalf("%v", err)
		}
		startc := time.Now()
		resp, err := client.Check(context.Background(), data.client, data.resolved, data.name)
		if err != nil {
			logger.Fatalf("Check '%s' returned error: %v", line, err)
		}
		fmt.Fprintf(os.Stdout, "%v,%v,%s: %v,%v,%v (%v)\n", data.client, data.resolved, data.name, resp.Result, resp.Last, resp.Store, time.Since(startc))
	}
	if err := scanner.Err(); err != nil {
		logger.Fatalf("%v", err)
	}
}

type recordData struct {
	client   net.IP
	resolved net.IP
	name     string
}

func getValues(arg string) (recordData, error) {
	data := recordData{}
	values := strings.Split(arg, ",")
	if len(values) < 2 {
		return data, fmt.Errorf("invalid values '%s'", arg)
	}
	clientIP := net.ParseIP(values[0])
	if clientIP == nil {
		return data, fmt.Errorf("invalid clientip '%v'", values[0])
	}
	resolvedIP := net.ParseIP(values[1])
	if resolvedIP == nil {
		return data, fmt.Errorf("invalid clientip '%v'", values[1])
	}
	name := ""
	if len(values) == 3 {
		name = values[2]
	}
	data.client = clientIP
	data.resolved = resolvedIP
	data.name = name
	return data, nil
}
