// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package resolvarchive

import (
	"errors"
	"net"
)

func externalIP4() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return net.IP{}, err
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		ip = ip.To4()
		if ip == nil {
			continue // not an ipv4 address
		}
		return ip, nil
	}
	return net.IP{}, errors.New("can't locate first non loopback ip address")
}
