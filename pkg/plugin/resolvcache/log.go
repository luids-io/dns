// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvcache

import (
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

// wrapped log to implement yalogi.Logger interface
type wlog struct {
	clog.P
}

func (w wlog) Warnf(format string, args ...interface{}) {
	w.P.Warningf(format, args...)
}
