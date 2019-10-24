// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"sync"

	"github.com/caddyserver/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

//once is used for register metrics
var once sync.Once

type fwmetrics struct {
	listedDomains   *prometheus.CounterVec
	unlistedDomains *prometheus.CounterVec
	listedIPs       *prometheus.CounterVec
	unlistedIPs     *prometheus.CounterVec
	errors          *prometheus.CounterVec
}

func newMetrics() *fwmetrics {
	return &fwmetrics{
		listedDomains: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "xlisthole",
			Name:      "listed_domains_count_total",
			Help:      "Counter of positive listed domains.",
		}, []string{"server"}),
		unlistedDomains: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "xlisthole",
			Name:      "unlisted_domains_count_total",
			Help:      "Counter of negative listed domains.",
		}, []string{"server"}),
		listedIPs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "xlisthole",
			Name:      "listed_ips_count_total",
			Help:      "Counter of positive listed IPs.",
		}, []string{"server"}),
		unlistedIPs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "xlisthole",
			Name:      "unlisted_ips_count_total",
			Help:      "Counter of negative listed IPs.",
		}, []string{"server"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: plugin.Namespace,
			Subsystem: "xlisthole",
			Name:      "check_errors",
			Help:      "Counter of check errors.",
		}, []string{"server"}),
	}
}

func (m fwmetrics) register(c *caddy.Controller) {
	once.Do(func() {
		metrics.MustRegister(c, m.listedDomains)
		metrics.MustRegister(c, m.unlistedDomains)
		metrics.MustRegister(c, m.listedIPs)
		metrics.MustRegister(c, m.unlistedIPs)
		metrics.MustRegister(c, m.errors)
	})
}
