// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package collectfile

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/peer"
)

// Log implements an asyncronous cache.CollectLogger using a file for storage
type Log struct {
	fname   string
	file    *os.File
	started bool
	resolvs chan *logData
	close   chan struct{}
	closed  chan struct{}
}

// BufferSize for the logger
const BufferSize = 512

type logData struct {
	peer     *peer.Peer
	ts       time.Time
	client   net.IP
	name     string
	resolved []net.IP
}

// New creates a new collector logger
func New(fname string) *Log {
	return &Log{fname: fname}
}

// Write implements cache.CollectLogger
func (f *Log) Write(peer *peer.Peer, ts time.Time, client net.IP, name string, resolved []net.IP) error {
	if !f.started {
		return errors.New("collect log not started")
	}
	f.resolvs <- &logData{
		peer:     peer,
		ts:       ts,
		client:   client,
		name:     name,
		resolved: resolved,
	}
	return nil
}

// Start collector logger
func (f *Log) Start() error {
	if f.started {
		return nil
	}
	var err error
	f.file, err = os.Create(f.fname)
	if err != nil {
		return err
	}
	f.resolvs = make(chan *logData, BufferSize)
	f.close = make(chan struct{})
	f.closed = make(chan struct{})
	f.started = true
	go f.run()
	return nil
}

// Stop collector logger
func (f *Log) Stop() {
	if !f.started {
		return
	}
	f.started = false
	close(f.close)
	<-f.closed
	close(f.resolvs)
	f.file.Sync()
	f.file.Close()
}

func (f *Log) run() {
PROCESSLOOP:
	for {
		select {
		case resolv := <-f.resolvs:
			s := getDataString(resolv)
			_, err := f.file.WriteString(s)
			if err != nil {
				break PROCESSLOOP
			}
		case <-f.close:
			for {
				//send buffered
				select {
				case resolv := <-f.resolvs:
					s := getDataString(resolv)
					_, err := f.file.WriteString(s)
					if err != nil {
						break PROCESSLOOP
					}
				default:
					break PROCESSLOOP
				}
			}
		}
	}
	close(f.closed)
}

func getDataString(data *logData) string {
	resolved := make([]string, 0, len(data.resolved))
	for _, r := range data.resolved {
		resolved = append(resolved, r.String())
	}
	peerinfo := ""
	if data.peer != nil {
		peerinfo = data.peer.Addr.String()
	}
	tstamp := data.ts.Format("20060102150405")
	return fmt.Sprintf("%s,%s,%s,%s,%s\n", peerinfo, tstamp, data.client.String(), data.name, strings.Join(resolved, ","))
}
