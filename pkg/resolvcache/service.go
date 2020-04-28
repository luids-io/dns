// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/peer"

	"github.com/luids-io/core/dnsutil"
	"github.com/luids-io/core/utils/yalogi"
)

// Service implements a dnsutil.ResolvCache service
type Service struct {
	opts    options
	logger  yalogi.Logger
	clogger CollectLogger
	qlogger QueryLogger
	// cache
	cache *Cache
	//control
	started bool
	mu      sync.Mutex
	wg      sync.WaitGroup
	close   chan struct{}
}

// CollectLogger interface defines collection logger interface
type CollectLogger interface {
	WriteCollect(*peer.Peer, time.Time, net.IP, string, []net.IP) error
}

// QueryLogger interface defines query logger interface
type QueryLogger interface {
	WriteCheck(peer *peer.Peer, ts time.Time, client, resolved net.IP, name string, resp dnsutil.ResolvResponse) error
}

// Option is used for component configuration
type Option func(*options)

type options struct {
	logger        yalogi.Logger
	clogger       CollectLogger
	qlogger       QueryLogger
	dumpInterval  time.Duration
	cleanInterval time.Duration
	dumpFile      string
}

var defaultOptions = options{
	dumpInterval:  5 * time.Minute,
	cleanInterval: 1 * time.Minute,
}

// DumpCache sets interval and filename for dump
func DumpCache(d time.Duration, fname string) Option {
	return func(o *options) {
		if d > 0 {
			o.dumpInterval = d
		}
		o.dumpFile = fname
	}
}

// SetCollectLogger sets a collection logger
func SetCollectLogger(l CollectLogger) Option {
	return func(o *options) {
		o.clogger = l
	}
}

// SetQueryLogger sets a query logger
func SetQueryLogger(l QueryLogger) Option {
	return func(o *options) {
		o.qlogger = l
	}
}

// SetLogger option allows set a custom logger
func SetLogger(l yalogi.Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// NewService creates a new Service
func NewService(c *Cache, opt ...Option) *Service {
	opts := defaultOptions
	for _, o := range opt {
		o(&opts)
	}
	s := &Service{
		opts:    opts,
		logger:  opts.logger,
		clogger: opts.clogger,
		qlogger: opts.qlogger,
		cache:   c,
	}
	return s
}

// Collect implements dnsutil.ResolvCollector
func (s *Service) Collect(ctx context.Context, client net.IP, name string, resolved []net.IP) error {
	if !s.started {
		return errors.New("service not started")
	}
	now := time.Now()
	err := s.cache.Set(now, client, name, resolved)
	if err != nil {
		s.logger.Warnf("collecting '%v,%v,%v': %v", client, name, resolved)
	}
	if s.clogger != nil {
		peer, _ := peer.FromContext(ctx)
		err := s.clogger.WriteCollect(peer, now, client, name, resolved)
		if err != nil {
			s.logger.Warnf("writting to collect logger '%v,%v,%v': %v", client, name, resolved)
		}
	}
	return err
}

// Check implements dnsutil.ResolvChecker
func (s *Service) Check(ctx context.Context, client, resolved net.IP, name string) (dnsutil.ResolvResponse, error) {
	if !s.started {
		return dnsutil.ResolvResponse{}, errors.New("service not started")
	}
	now := time.Now()
	resp := dnsutil.ResolvResponse{}
	resp.Result, resp.Last = s.cache.Get(client, resolved, name)
	resp.Store = s.cache.Store()
	if s.qlogger != nil {
		peer, _ := peer.FromContext(ctx)
		err := s.qlogger.WriteCheck(peer, now, client, resolved, name, resp)
		if err != nil {
			s.logger.Warnf("writting to query logger '%v,%v,%v': %v", client, name, resolved)
		}
	}
	return resp, nil
}

// Uptime returns cache information
func (s *Service) Uptime(ctx context.Context) (time.Time, time.Duration, error) {
	if !s.started {
		return time.Time{}, 0, errors.New("service not started")
	}
	return s.cache.Flushed(), s.cache.Expires(), nil
}

// Start service cache
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.logger.Infof("starting cache service")
	// start maintenance goroutines
	s.close = make(chan struct{})
	if s.cache.Expires() > 0 {
		s.wg.Add(1)
		go s.autoClean()
	}
	if s.opts.dumpFile != "" {
		s.wg.Add(1)
		go s.autoDump()
	}
	s.started = true
	return nil
}

// Shutdown service cache
func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return
	}
	s.logger.Infof("shutting down cache service")
	s.started = false
	close(s.close)
	s.wg.Wait()
}

func (s *Service) dump(filename string) error {
	if !s.started {
		return errors.New("service not started")
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	s.cache.Dump(file)
	file.Sync()
	file.Close()
	return nil
}

// cache maintenance go routines
func (s *Service) autoDump() {
	tick := time.NewTicker(s.opts.dumpInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			s.logger.Debugf("dumping cache to %s", s.opts.dumpFile)
			s.dump(s.opts.dumpFile)
		case <-s.close:
			s.wg.Done()
			return
		}
	}
}

func (s *Service) autoClean() {
	tick := time.NewTicker(s.opts.cleanInterval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			s.logger.Debugf("cleaning cache")
			s.cache.Clean()
		case <-s.close:
			s.wg.Done()
			return
		}
	}
}
