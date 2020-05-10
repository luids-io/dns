// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	checkapi "github.com/luids-io/api/dnsutil/grpc/resolvcheck"
	collectapi "github.com/luids-io/api/dnsutil/grpc/resolvcollect"
	"github.com/luids-io/core/yalogi"
	"github.com/luids-io/dns/pkg/resolvcache"
)

// ResolvCheckAPI creates grpc service
func ResolvCheckAPI(csvc *resolvcache.Service, logger yalogi.Logger) (*checkapi.Service, error) {
	gsvc := checkapi.NewService(csvc)
	return gsvc, nil
}

// ResolvCollectAPI creates grpc service
func ResolvCollectAPI(csvc *resolvcache.Service, logger yalogi.Logger) (*collectapi.Service, error) {
	gsvc := collectapi.NewService(csvc)
	return gsvc, nil
}
