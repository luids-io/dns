// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvarchive

import (
	"context"
	"errors"

	"github.com/luisguillenc/yalogi"
	"google.golang.org/grpc"

	"github.com/luids-io/api/dnsutil/archive"
	"github.com/luids-io/core/dnsutil"
)

// Archiver is an archiver with an channel buffer
type Archiver struct {
	logger yalogi.Logger
	//internal buffered data channel
	dataCh chan dnsutil.ResolvData
	//client used for archive
	client *archive.Client
	//control state
	closed   bool
	sigclose chan struct{}
}

// NewArchiver returns a new instance
func NewArchiver(dial *grpc.ClientConn, bufsize int, logger yalogi.Logger) *Archiver {
	a := &Archiver{
		logger:   logger,
		client:   archive.NewClient(dial, archive.SetLogger(logger)),
		dataCh:   make(chan dnsutil.ResolvData, bufsize),
		sigclose: make(chan struct{}),
	}
	go a.doProcess()
	return a
}

// SaveResolv data in an asyncronous mode
func (a *Archiver) SaveResolv(data dnsutil.ResolvData) {
	if !a.closed {
		a.dataCh <- data
	}
}

// Ping archiver
func (a *Archiver) Ping() error {
	if a.closed {
		return errors.New("archiver closed")
	}
	return a.client.Ping()
}

// Close archiver
func (a *Archiver) Close() error {
	if a.closed {
		return nil
	}
	a.closed = true
	close(a.dataCh)
	<-a.sigclose
	return a.client.Close()
}

func (a *Archiver) doProcess() {
	for data := range a.dataCh {
		_, err := a.client.SaveResolv(context.Background(), data)
		if err != nil {
			a.logger.Warnf("%v", err)
		}
	}
	close(a.sigclose)
}
