// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"fmt"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/luisguillenc/grpctls"

	"github.com/luids-io/core/event"
	"github.com/luids-io/core/event/notify"
)

func init() {
	caddy.RegisterPlugin("dnsevent", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	cfg := DefaultConfig()
	err := cfg.Load(c)
	if err != nil {
		return err
	}
	//creates event notifier
	notifier, err := newNotifier(cfg)
	if err != nil {
		return plugin.Error("dnsevent", c.Err(err.Error()))
	}
	//creates event buffer and uses as default
	buffer := event.NewBuffer(notifier, cfg.Buffer, event.SetLogger(log))
	event.SetBuffer(buffer)

	//register shutdown
	c.OnShutdown(func() error {
		buffer.Close()
		return notifier.Close()
	})
	return nil
}

func newNotifier(cfg Config) (*notify.Client, error) {
	// dial grpc connection
	dial, err := grpctls.Dial(cfg.Endpoint, cfg.Client)
	if err != nil {
		return nil, fmt.Errorf("cannot dial with %s: %v", cfg.Endpoint, err)
	}
	// create client
	return notify.NewClient(dial, notify.SetLogger(log)), nil
}
