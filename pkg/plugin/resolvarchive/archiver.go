// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvarchive

import (
	"context"

	"github.com/luids-io/api/dnsutil"
	"github.com/luids-io/core/yalogi"
)

// Archiver is an archiver with an channel buffer.
type Archiver struct {
	logger yalogi.Logger
	//internal buffered data channel
	dataCh chan *dnsutil.ResolvData
	//client used for archive
	client dnsutil.Archiver
	//control state
	closed   bool
	sigclose chan struct{}
}

// NewArchiver returns a new instance.
func NewArchiver(client dnsutil.Archiver, bufsize int, logger yalogi.Logger) *Archiver {
	a := &Archiver{
		logger:   logger,
		client:   client,
		dataCh:   make(chan *dnsutil.ResolvData, bufsize),
		sigclose: make(chan struct{}),
	}
	go a.doProcess()
	return a
}

// SaveResolv data in an asyncronous mode.
func (a *Archiver) SaveResolv(data *dnsutil.ResolvData) {
	if !a.closed {
		a.dataCh <- data
	}
}

// Close archiver.
func (a *Archiver) Close() error {
	if a.closed {
		return nil
	}
	a.closed = true
	close(a.dataCh)
	<-a.sigclose
	return nil
}

func (a *Archiver) doProcess() {
	for data := range a.dataCh {
		_, err := a.client.SaveResolv(context.Background(), *data)
		if err != nil {
			a.logger.Warnf("%v", err)
		}
	}
	close(a.sigclose)
}
