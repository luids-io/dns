// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"errors"

	collectapi "github.com/luids-io/api/dnsutil/grpc/resolvcollect"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// ResolvCollectAPI creates grpc service
func ResolvCollectAPI(cfg *config.ResolvCollectAPICfg, csvc *resolvcache.Service, logger yalogi.Logger) (*collectapi.Service, error) {
	if !cfg.Enable {
		return nil, errors.New("dnsutil resolvcollect service disabled")
	}
	if !cfg.Log {
		logger = yalogi.LogNull
	}
	gsvc := collectapi.NewService(csvc, collectapi.SetServiceLogger(logger))
	return gsvc, nil
}
