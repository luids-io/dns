// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsapi

import (
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

// I wrapped log to implement yalogi.Logger interface
var log = wlog{P: clog.NewWithPlugin("idsapi")}

type wlog struct {
	clog.P
}

func (w wlog) Warnf(format string, args ...interface{}) {
	w.P.Warningf(format, args...)
}
