// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlisthole

import (
	"net"
	"os"

	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

func isIPv4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	if ip.To4() == nil {
		return false
	}
	return true
}

func fileExists(file string) bool {
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		return true
	}
	return false
}

func getMsgReplyNXDomain(req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(req.Req, dns.RcodeNameError)
	return m
}

func getMsgReplyRefused(req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(req.Req, dns.RcodeRefused)
	return m
}

func getMsgReplyIP(ip string, ttl int, req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req.Req)
	m.Authoritative, m.RecursionAvailable = false, true
	if req.QType() == dns.TypeA {
		m.Answer = a(req.Name(), []net.IP{net.ParseIP(ip)}, ttl)
	} else if req.QType() == dns.TypeAAAA {
		m.Answer = aaaa(req.Name(), []net.IP{net.ParseIP(ip)}, ttl)
	}
	return m
}

func a(zone string, ips []net.IP, ttl int) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: uint32(ttl)}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}

func aaaa(zone string, ips []net.IP, ttl int) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.AAAA)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeAAAA,
			Class: dns.ClassINET, Ttl: uint32(ttl)}
		r.AAAA = ip
		answers = append(answers, r)
	}
	return answers
}
