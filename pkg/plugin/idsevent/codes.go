// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"github.com/luids-io/core/event"
)

// Registered event codes
const (
	//DNS Blackhole
	DNSListedDomain   event.Code = 10001
	DNSUnlistedDomain event.Code = 10002
	DNSListedIP       event.Code = 10003
	DNSUnlistedIP     event.Code = 10004

	//DNS Collect
	DNSMaxClientRequests  event.Code = 10005
	DNSMaxNamesResolvedIP event.Code = 10006
)
