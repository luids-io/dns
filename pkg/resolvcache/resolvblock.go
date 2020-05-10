// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"net"
	"sync"
	"time"

	"github.com/luids-io/api/dnsutil"
)

// stores resolved ips and names
type resolvBlock struct {
	cache *Cache
	mu    sync.RWMutex
	last  time.Time
	// index map[ip]idx node
	index map[string]int
	nodes []node
	// next stores next free node
	next int
}

// node in block struct
type node struct {
	//last insert date
	last time.Time
	// embedded item
	item
	// others items
	others []item
}

type item struct {
	ts   time.Time
	name string
}

// BlockSize stores the number of nodes in blocks
//const BlockSize = 512

func (b *resolvBlock) doQuery(resolved net.IP, name string) (bool, time.Time) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	//check the index for the resolved ip
	idx, ok := b.index[getIPKey(resolved)]
	if ok {
		return b.nodes[idx].query(name, b.cache.expires)
	}
	return false, time.Time{}
}

// insert returns true if inserted, false if block is full.
// It returns an error if max domains per node has reached
func (b *resolvBlock) insert(resolved net.IP, name string, ts time.Time) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	max := b.cache.limits.MaxNamesNode
	key := getIPKey(resolved)
	// checks if it's a resolved ip
	idx, ok := b.index[key]
	if ok {
		// update block last update
		b.last = ts
		// update node
		err := b.nodes[idx].update(name, ts, max)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	//check if block has space for next node
	if b.next < b.cache.limits.BlockSize {
		// update block last update
		b.last = ts
		//adds node to block
		b.nodes[b.next].update(name, ts, max)
		b.index[key] = b.next
		b.next++
		return true, nil
	}
	return false, nil
}

func (b *resolvBlock) clean(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, n := range b.nodes {
		//clear values of nodes outdated
		if time.Since(n.last) > d {
			b.nodes[i].name = ""
			b.nodes[i].others = nil
		}
	}
}

func (n *node) update(name string, ts time.Time, max int) error {
	// updates node last update
	n.last = ts
	// if embedded item is empty
	if n.name == "" {
		n.ts = ts
		n.name = name
		return nil
	} else if n.name == name {
		n.ts = ts
	} else {
		// if embedded item not empty
		if len(n.others) == 0 {
			n.others = make([]item, 0, max)
			n.others = append(n.others, item{name: name, ts: ts})
			return nil
		}
		// check if name already exists
		for i, o := range n.others {
			if o.name == name {
				n.others[i].ts = ts
				return nil
			}
		}
		// check limits
		if len(n.others) >= max {
			return dnsutil.ErrLimitResolvedNamesIP
		}
		// add new name
		n.others = append(n.others, item{name: name, ts: ts})
	}
	return nil
}

func (n *node) query(name string, expires time.Duration) (bool, time.Time) {
	// check node expired
	if time.Since(n.last) > expires {
		return false, time.Time{}
	}
	// value was cleaned
	if n.name == "" {
		return false, time.Time{}
	}
	// if query without name, returns last update of the node
	if name == "" {
		return true, n.last
	}
	// check name in embedded item
	if name == n.name {
		if time.Since(n.ts) <= expires {
			return true, n.ts
		}
		return false, time.Time{}
	}
	// check in items
	for _, o := range n.others {
		if name == o.name {
			if time.Since(o.ts) <= expires {
				return true, o.ts
			}
			return false, time.Time{}
		}
	}
	return false, time.Time{}
}
