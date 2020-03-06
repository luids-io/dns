// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package factory

import (
	"github.com/luisguillenc/yalogi"

	checkapi "github.com/luids-io/api/dnsutil/resolvcheck"
	collectapi "github.com/luids-io/api/dnsutil/resolvcollect"
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
