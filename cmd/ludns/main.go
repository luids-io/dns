// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	_ "github.com/coredns/coredns/plugin/cache"
	_ "github.com/coredns/coredns/plugin/forward"
	_ "github.com/coredns/coredns/plugin/health"
	_ "github.com/coredns/coredns/plugin/metrics"
	_ "github.com/coredns/coredns/plugin/whoami"
	_ "github.com/luids-io/dns/pkg/plugin/idsevent"
	_ "github.com/luids-io/dns/pkg/plugin/resolvcache"
	_ "github.com/luids-io/dns/pkg/plugin/xlisthole"
	_ "github.com/luids-io/dns/pkg/plugin/xlistrbl"
)

var directives = []string{
	"idsevent",
	"xlistrbl",
	"xlisthole",
	"prometheus",
	"health",
	"resolvcache",
	"forward",
	"whoami",
	"cache",
	"startup",
	"shutdown",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
