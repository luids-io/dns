// Copyright 2019 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package xlistrbl

import (
	"net"
	"strings"

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

func reverse(name string) (rev string) {
	s := strings.Split(name, ".")
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	rev = strings.Join(s, ".")
	return
}

func getMsgReplyIP(ip string, ttl int, req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req.Req)
	m.Authoritative, m.RecursionAvailable = true, false
	if req.QType() == dns.TypeA {
		m.Answer = a(req.Name(), []net.IP{net.ParseIP(ip)}, ttl)
	} else if req.QType() == dns.TypeAAAA {
		m.Answer = aaaa(req.Name(), []net.IP{net.ParseIP(ip)}, ttl)
	}
	return m
}

func getMsgReplyNXDomain(req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetRcode(req.Req, dns.RcodeNameError)
	m.Authoritative, m.RecursionAvailable = true, false
	return m
}

func getMsgReplyTxt(text string, ttl int, req *request.Request) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req.Req)
	m.Authoritative, m.RecursionAvailable = true, false
	if req.QType() == dns.TypeTXT {
		m.Answer = txt(req.Name(), []string{text}, ttl)
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

func txt(zone string, txts []string, ttl int) []dns.RR {
	r := new(dns.TXT)
	r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeTXT,
		Class: dns.ClassINET, Ttl: uint32(ttl)}
	r.Txt = txts
	return []dns.RR{r}
}
