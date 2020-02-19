// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Cache implements a resolv cache in memory
type Cache struct {
	expires time.Duration
	limits  Limits
	mu      sync.RWMutex
	clients map[string]*clientBlock
	//time stamps
	cleaned time.Time
	flushed time.Time
}

// Limits stores max values for cache
type Limits struct {
	BlockSize       int
	MaxBlocksClient int
	MaxNamesNode    int
}

// DefaultLimits returns limits
func DefaultLimits() Limits {
	return Limits{
		BlockSize:       1024,
		MaxBlocksClient: 32,
		MaxNamesNode:    32,
	}
}

// NewCache creates a new Cache
func NewCache(expires time.Duration, limits Limits) *Cache {
	now := time.Now()
	o := &Cache{
		expires: expires,
		limits:  limits,
		clients: make(map[string]*clientBlock),
		flushed: now,
		cleaned: now,
	}
	return o
}

// Set data
func (o *Cache) Set(ts time.Time, client net.IP, name string, resolved []net.IP) error {
	// gets client data
	c := o.getClientBlock(client)
	// insert data into client information
	for _, rip := range resolved {
		err := c.insert(rip, name, ts)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get data
func (o *Cache) Get(client, resolved net.IP, name string) (bool, time.Time) {
	o.mu.RLock()
	c, ok := o.clients[getIPKey(client)]
	o.mu.RUnlock()
	if !ok {
		return false, time.Time{}
	}
	result, last := c.doQuery(resolved, name)
	return result, last
}

// Flushed returns time from last flush
func (o *Cache) Flushed() time.Time {
	return o.flushed
}

// Cleaned returns time from last clean
func (o *Cache) Cleaned() time.Time {
	return o.cleaned
}

// Expires returns expiration time
func (o *Cache) Expires() time.Duration {
	return o.expires
}

// Store returns store time
func (o *Cache) Store() time.Time {
	if time.Since(o.flushed) < o.expires {
		return o.flushed
	}
	return time.Now().Add(-o.expires)
}

// Flush cache
func (o *Cache) Flush() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.clients = make(map[string]*clientBlock)
	//garbage collector hash some work... ;)
	o.flushed = time.Now()
}

// Clean expired items from cache
func (o *Cache) Clean() {
	o.mu.RLock()
	//gets a copy of pointers to clientdata
	clients := make([]*clientBlock, 0, len(o.clients))
	for _, client := range o.clients {
		clients = append(clients, client)
	}
	o.mu.RUnlock()
	//iterate clients and clean
	for _, c := range clients {
		c.clean(o.expires)
	}
	o.cleaned = time.Now()
}

// Dump cache content to writer
func (o *Cache) Dump(out io.Writer) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fmt.Fprintf(out, "dump: %s\n", time.Now())
	fmt.Fprintf(out, "expires: %v\n", o.expires)
	fmt.Fprintf(out, "limits: %+v\n\n", o.limits)
	//for each client
	for key, client := range o.clients {
		client.mu.Lock()
		fmt.Fprintf(out, "- key: %v\n", key)
		//for each block
		for n, b := range client.blocks {
			b.mu.Lock()
			fmt.Fprintf(out, "  - index: %v next: %v last: %s\n", n, b.next, b.last.Format("20060102150405"))
			for k, i := range b.index {
				node := b.nodes[i]
				fmt.Fprintf(out, "    - key: %s index: %v last: %s\n", k, i, node.last.Format("20060102150405"))
				fmt.Fprintf(out, "      name: %s ts: %s\n", node.item.name, node.item.ts.Format("20060102150405"))
				if len(node.others) > 0 {
					for _, item := range node.others {
						fmt.Fprintf(out, "      name: %s ts: %s\n", item.name, item.ts.Format("20060102150405"))
					}
				}
			}
			b.mu.Unlock()
		}
		client.mu.Unlock()
	}
}

func (o *Cache) getClientBlock(ip net.IP) *clientBlock {
	//use read lock for fatest path
	o.mu.RLock()
	c, ok := o.clients[getIPKey(ip)]
	if ok {
		o.mu.RUnlock()
		return c
	}
	o.mu.RUnlock()
	//creates a new block
	o.mu.Lock()
	defer o.mu.Unlock()
	c, ok = o.clients[getIPKey(ip)]
	if ok {
		return c
	}
	return o.newClientBlock(ip)
}

func (o *Cache) newClientBlock(ip net.IP) *clientBlock {
	c := &clientBlock{
		cache:  o,
		blocks: make([]*resolvBlock, 0),
	}
	c.newResolvBlock()
	o.clients[getIPKey(ip)] = c
	return c
}
