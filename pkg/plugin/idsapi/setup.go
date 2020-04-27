// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsapi

import (
	"fmt"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"

	"github.com/luids-io/common/util"
	"github.com/luids-io/core/apiservice"
)

func init() {
	caddy.RegisterPlugin("idsapi", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

// setup function creates a new instance and register to controller
func setup(c *caddy.Controller) error {
	p, err := createPlugin(c)
	if err != nil {
		return plugin.Error("idsapi", err)
	}
	c.OnStartup(func() error {
		err := p.Start()
		if err != nil {
			return err
		}
		//sets as singleton discover
		SetDiscover(p.GetDiscover())
		return nil
	})
	c.OnShutdown(func() error {
		return p.Shutdown()
	})
	return nil
}

// creates a plugin from a controller
func createPlugin(c *caddy.Controller) (*Plugin, error) {
	config := DefaultConfig()
	err := config.Load(c)
	if err != nil {
		return nil, err
	}
	//create archiver plugin
	p, err := New(config)
	if err != nil {
		return nil, c.Err(err.Error())
	}
	return p, nil
}

func getServiceDefs(cfg Config) ([]apiservice.ServiceDef, error) {
	dbFiles, err := util.GetFilesDB("json", cfg.ConfigFiles, cfg.ConfigDirs)
	if err != nil {
		return nil, err
	}
	loadedDB := make([]apiservice.ServiceDef, 0)
	for _, file := range dbFiles {
		entries, err := apiservice.DefsFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("loading file '%s': %v", file, err)
		}
		loadedDB = append(loadedDB, entries...)
	}
	return loadedDB, nil
}
