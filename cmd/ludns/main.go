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

	//luids coredns plugins
	_ "github.com/luids-io/dns/pkg/plugin/idsapi"
	_ "github.com/luids-io/dns/pkg/plugin/idsevent"
	_ "github.com/luids-io/dns/pkg/plugin/resolvarchive"
	_ "github.com/luids-io/dns/pkg/plugin/resolvcache"
	_ "github.com/luids-io/dns/pkg/plugin/xlisthole"
	_ "github.com/luids-io/dns/pkg/plugin/xlistrbl"

	//luids api services
	_ "github.com/luids-io/api/dnsutil/grpc/archive"
	_ "github.com/luids-io/api/dnsutil/grpc/resolvcollect"
	_ "github.com/luids-io/api/event/grpc/notify"
	_ "github.com/luids-io/api/xlist/grpc/check"
)

var directives = []string{
	"idsapi",
	"idsevent",
	"prometheus",
	"health",
	"resolvarchive",
	"resolvcache",
	"xlisthole",
	"cache",
	"forward",
	"xlistrbl",
	"whoami",
	"startup",
	"shutdown",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
