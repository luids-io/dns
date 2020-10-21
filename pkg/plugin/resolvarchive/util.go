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

// IPSet contains ips and cidrs.
type IPSet struct {
	IPs   []net.IP
	CIDRs []*net.IPNet
}

// Contains returns true if ip exists in the set.
func (f *IPSet) Contains(ip net.IP) bool {
	if len(f.IPs) == 0 && len(f.CIDRs) == 0 {
		return false
	}
	if len(f.CIDRs) > 0 {
		for _, lcidr := range f.CIDRs {
			if lcidr.Contains(ip) {
				return true
			}
		}
	}
	if len(f.IPs) > 0 {
		for _, lip := range f.IPs {
			if lip.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// Empty returns true if ipset is empty
func (f *IPSet) Empty() bool {
	if len(f.CIDRs) == 0 && len(f.IPs) == 0 {
		return true
	}
	return false
}
