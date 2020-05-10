// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsevent

import (
	"github.com/luids-io/api/event"
)

// Registered event codes
const (
	//DNS Blackhole
	DNSListedDomain   event.Code = 10020
	DNSUnlistedDomain event.Code = 10021
	DNSListedIP       event.Code = 10022
	DNSUnlistedIP     event.Code = 10023

	//DNS Collect
	DNSMaxClientRequests  event.Code = 10024
	DNSMaxNamesResolvedIP event.Code = 10025
)
