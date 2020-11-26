// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/luids-io/dns/cmd/resolvcollect/config"
)

//Variables for version output
var (
	Program  = "resolvcollect"
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
	if len(pflag.Args()) == 0 && !inStdin && inFile == "" {
		fmt.Fprintln(os.Stderr, "required collect data")
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

	// collect from args
	if !inStdin && inFile == "" {
		for _, arg := range pflag.Args() {
			record, err := getValue(arg)
			if err != nil {
				logger.Fatalf("%v", err)
			}
			startc := time.Now()
			err = client.Collect(context.Background(), record.client, record.name, record.resolved, record.cnames)
			if err != nil {
				logger.Fatalf("collect '%s' returned error: %v", arg, err)
			}
			fmt.Fprintf(os.Stdout, "%s,%s,%s (%v)\n", record.client, record.name, record.resolved, time.Since(startc))
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
		record, err := getValue(line)
		if err != nil {
			logger.Fatalf("%v", err)
		}
		startc := time.Now()
		err = client.Collect(context.Background(), record.client, record.name, record.resolved, record.cnames)
		if err != nil {
			logger.Fatalf("collect '%s' returned error: %v", line, err)
		}
		fmt.Fprintf(os.Stdout, "%s,%s,%s (%v)\n", record.client, record.name, record.resolved, time.Since(startc))
	}
	if err := scanner.Err(); err != nil {
		logger.Fatalf("%v", err)
	}
}

type recordData struct {
	client   net.IP
	name     string
	resolved []net.IP
	cnames   []string
}

func getValue(arg string) (recordData, error) {
	data := recordData{}
	values := strings.Split(arg, ",")
	if len(values) < 3 {
		return data, fmt.Errorf("invalid values '%s'", arg)
	}
	// get client ip
	clientIP := net.ParseIP(values[0])
	if clientIP == nil {
		return data, fmt.Errorf("invalid clientip '%v'", values[0])
	}
	// get domain name
	name := values[1]
	if !isDomain(name) {
		return data, fmt.Errorf("invalid domain '%v'", name)
	}
	// get resolved ips
	resolved := values[2:]
	resolvedIP := make([]net.IP, 0, len(resolved))
	resolvedCNAME := make([]string, 0, len(resolved))
	for _, value := range resolved {
		ip := net.ParseIP(value)
		if ip == nil {
			resolvedCNAME = append(resolvedCNAME, value)
			continue
		}
		resolvedIP = append(resolvedIP, ip)
	}
	// set data
	data.client = clientIP
	data.name = name
	data.resolved = resolvedIP
	data.cnames = resolvedCNAME
	return data, nil
}

// note: we precompute for performance reasons
var validDomainRegexp, _ = regexp.Compile(`^(([a-zA-Z0-9]|[a-zA-Z0-9_][a-zA-Z0-9\-_]*[a-zA-Z0-9_])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

func isDomain(s string) bool {
	return validDomainRegexp.MatchString(s)
}
