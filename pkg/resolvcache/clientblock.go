// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	"net"
	"sync"
	"time"

	"github.com/luids-io/api/dnsutil"
)

// this struct is per dns client ip
type clientBlock struct {
	cache *Cache
	mu    sync.RWMutex
	// blocks stores blocks
	blocks []*resolvBlock
}

func (c *clientBlock) doQuery(resolved net.IP, name string) (bool, time.Time) {
	var blocks []*resolvBlock
	//gets a copy of block pointers for iterate without lock the full client
	c.mu.RLock()
	if c.blocks == nil {
		blocks = nil
	} else {
		blocks = make([]*resolvBlock, len(c.blocks))
		copy(blocks, c.blocks)
	}
	c.mu.RUnlock()
	//iterate blocks
	for i := len(blocks) - 1; i >= 0; i-- {
		result, last := blocks[i].doQuery(resolved, name)
		if result {
			return result, last
		}
	}
	return false, time.Time{}
}

func (c *clientBlock) insert(resolved net.IP, name string, ts time.Time) error {
	//gets current block
	block := c.currentBlock()
	ok, err := block.insert(resolved, name, ts)
	if err != nil {
		return err
	}
	// !ok -> block is full
	for !ok {
		//call nextfreeblock, it's safe concurrency
		block, err = c.nextFreeBlock()
		if err != nil {
			return err
		}
		ok, err = block.insert(resolved, name, ts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *clientBlock) currentBlock() *resolvBlock {
	//use read lock for fatest path
	c.mu.RLock()
	idx := len(c.blocks) - 1
	if idx >= 0 {
		ret := c.blocks[idx]
		c.mu.RUnlock()
		return ret
	}
	c.mu.RUnlock()
	//creates a new block
	c.mu.Lock()
	defer c.mu.Unlock()
	idx = len(c.blocks) - 1
	if idx >= 0 {
		return c.blocks[idx]
	}
	return c.newResolvBlock()
}

func (c *clientBlock) nextFreeBlock() (*resolvBlock, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	//if empty returns a new block
	idx := len(c.blocks) - 1
	if idx < 0 {
		return c.newResolvBlock(), nil
	}
	//gets current block and check if it's full
	block := c.blocks[idx]
	if block.next < c.cache.limits.BlockSize {
		return block, nil
	}
	//checks limits
	if len(c.blocks) > c.cache.limits.MaxBlocksClient {
		return nil, dnsutil.ErrLimitDNSClientQueries
	}
	//returns a new block
	return c.newResolvBlock(), nil
}

func (c *clientBlock) newResolvBlock() *resolvBlock {
	bs := c.cache.limits.BlockSize
	newblock := &resolvBlock{
		cache: c.cache,
		last:  time.Now(),
		index: make(map[string]int, bs),
		nodes: make([]node, bs, bs),
	}
	c.blocks = append(c.blocks, newblock)
	return newblock
}

func (c *clientBlock) clean(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// iterate and append all updated
	newblocks := make([]*resolvBlock, 0)
	for _, block := range c.blocks {
		if time.Since(block.last) <= d {
			block.clean(d)
			newblocks = append(newblocks, block)
		}
	}
	c.blocks = newblocks
}

func getIPKey(ip net.IP) string {
	//return string(ip.To16())
	return ip.String()
}
