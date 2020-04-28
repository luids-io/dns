// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package cachelog

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/luids-io/core/dnsutil"
	"google.golang.org/grpc/peer"
)

// LogFile implements an asyncronous cache.CollectLogger and cache.QueryLogger using a file for storage
type LogFile struct {
	fname    string
	file     *os.File
	closed   bool
	data     chan *logData
	closeSig chan struct{}
	waitSig  chan struct{}
}

// BufferSize for the logger
const BufferSize = 512

type opType int

const (
	opCollect = iota
	opCheck
)

func (op opType) String() string {
	switch op {
	case opCollect:
		return "collect"
	case opCheck:
		return "check"
	}
	return ""
}

type logData struct {
	op       opType
	peer     *peer.Peer
	ts       time.Time
	client   net.IP
	name     string
	resolved []net.IP
	response dnsutil.ResolvResponse
}

func (data *logData) String() string {
	client := data.client.String()
	peerinfo := ""
	if data.peer != nil {
		peerinfo = data.peer.Addr.String()
	}
	tstamp := data.ts.Format("20060102150405")
	switch data.op {
	case opCollect:
		resolved := make([]string, 0, len(data.resolved))
		for _, r := range data.resolved {
			resolved = append(resolved, r.String())
		}
		return fmt.Sprintf("%s,collect,%s,%s,%s,%s\n", tstamp, peerinfo, client, data.name, strings.Join(resolved, ","))
	case opCheck:
		resolved := ""
		if len(data.resolved) > 0 {
			resolved = data.resolved[0].String()
		}
		return fmt.Sprintf("%s,check,%s,%s,%s,%s,%v\n", tstamp, peerinfo, client, data.name, resolved, data.response.Result)
	}
	return fmt.Sprintf("%s,unknown,%s\n", tstamp, peerinfo)
}

// NewFile creates a new logger
func NewFile(fname string) (*LogFile, error) {
	file := &LogFile{fname: fname}
	if err := file.init(); err != nil {
		return nil, err
	}
	return file, nil
}

// WriteCollect implements cache.CollectLogger
func (f *LogFile) WriteCollect(peer *peer.Peer, ts time.Time, client net.IP, name string, resolved []net.IP) error {
	if f.closed {
		return errors.New("log is closed")
	}
	f.data <- &logData{
		op:       opCollect,
		peer:     peer,
		ts:       ts,
		client:   client,
		name:     name,
		resolved: resolved,
	}
	return nil
}

// WriteCheck implements cache.QueryLogger
func (f *LogFile) WriteCheck(peer *peer.Peer, ts time.Time, client, resolved net.IP, name string, resp dnsutil.ResolvResponse) error {
	if f.closed {
		return errors.New("log is closed")
	}
	f.data <- &logData{
		op:       opCheck,
		peer:     peer,
		ts:       ts,
		client:   client,
		resolved: []net.IP{resolved},
		name:     name,
		response: resp,
	}
	return nil
}

// init  logger
func (f *LogFile) init() error {
	var err error
	f.file, err = os.Create(f.fname)
	if err != nil {
		return err
	}
	f.data = make(chan *logData, BufferSize)
	f.closeSig = make(chan struct{})
	f.waitSig = make(chan struct{})
	go f.run()
	return nil
}

// Close logger
func (f *LogFile) Close() error {
	if f.closed {
		return errors.New("log is closed")
	}
	f.closed = true
	close(f.closeSig)
	<-f.waitSig
	close(f.data)
	f.file.Sync()
	return f.file.Close()
}

func (f *LogFile) run() {
PROCESSLOOP:
	for {
		select {
		case resolv := <-f.data:
			_, err := f.file.WriteString(resolv.String())
			if err != nil {
				break PROCESSLOOP
			}
		case <-f.closeSig:
			for {
				//send buffered
				select {
				case resolv := <-f.data:
					_, err := f.file.WriteString(resolv.String())
					if err != nil {
						break PROCESSLOOP
					}
				default:
					break PROCESSLOOP
				}
			}
		}
	}
	close(f.waitSig)
}