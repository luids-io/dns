// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"errors"

	checkapi "github.com/luids-io/api/dnsutil/grpc/resolvcheck"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/internal/config"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// ResolvCheckAPI creates grpc service
func ResolvCheckAPI(cfg *config.ResolvCheckAPICfg, csvc *resolvcache.Service, logger yalogi.Logger) (*checkapi.Service, error) {
	if !cfg.Enable {
		return nil, errors.New("dnsutil resolvcheck service disabled")
	}
	if !cfg.Log {
		logger = yalogi.LogNull
	}
	gsvc := checkapi.NewService(csvc, checkapi.SetServiceLogger(logger))
	return gsvc, nil
}
