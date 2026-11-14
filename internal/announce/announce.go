// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package announce makes a massimo serve findable on the LAN by
// multicast DNS (RFC 6762) and DNS-SD (RFC 6763), as the service type
// _massimo._tcp, and finds the servers that announce themselves.
//
// The responder answers for its own records only: the service's PTR, the
// instance's SRV and TXT, and the A and AAAA records of its host name,
// massimo-<node>.local, which never contests the operating system's own
// <hostname>.local. It shares UDP port 5353 with the system's responder
// (avahi, mDNSResponder, Windows' own), which every query reaches too; it
// neither probes nor suppresses known answers, since it claims no name
// another host could own. Browse asks by a legacy unicast query (RFC 6762
// section 6.7), which a responder answers to the asking port, so that
// massimo discover needs no port of its own.
package announce

import (
	"cmp"
	"fmt"
	"net"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/dns/dnsmessage"
)

// ServiceType is the DNS-SD service type of a massimo server.
const ServiceType = "_massimo._tcp"

// Port is the mDNS port.
const Port = 5353

// The multicast groups of mDNS.
var (
	groupV4 = net.IPv4(224, 0, 0, 251)
	groupV6 = net.ParseIP("ff02::fb")
)

// The TTLs of the records, in seconds: 120 for the PTR, the SRV and the
// addresses, 4500 for the TXT (RFC 6762 section 10); a legacy unicast
// answer gives ten at most.
const (
	ttlShort  = 120
	ttlLong   = 4500
	ttlLegacy = 10
)

// cacheFlush is the top bit of a resource record's class: the record is
// the whole set of its name and type (RFC 6762 section 10.2).
const cacheFlush = 0x8000

// unicastResponse is the top bit of a question's class: the querier asks
// for a unicast answer (RFC 6762 section 5.4).
const unicastResponse = 0x8000

// Service is what a server announces.
type Service struct {
	// Instance is the instance name, shown by browsers: the box's host
	// name, "massimo" without one.
	Instance string
	// Port is the API's port.
	Port uint16
	// Node is the store's node_id; its last eight hex digits name the
	// host of the SRV record, massimo-<digits>.local.
	Node string
	// Version is the build's version, such as v0.12.0.
	Version string
	// Fingerprint gives the SHA-256 of the certificate where the API
	// serves TLS, read at every answer, since a renewal changes it; nil
	// without TLS.
	Fingerprint func() string
	// URL is the TXT record's url, where a client reaches the API: the
	// public address where the server has one (MASSIMO_PUBLIC_URL); ""
	// gives its own, http(s)://massimo-<digits>.local:<port>.
	URL string
	// Location is the TXT record's loc, the installation's name for a
	// person: the host name in full where Instance is its first label.
	Location string
	// Registration says, read at every answer, whether integrations may
	// ask to be registered: the TXT record's reg, 1 or 0; nil gives 0.
	Registration func() bool
}

// InstanceName is the instance name for a host name: its first label,
// "massimo" for none. It holds no dot, which dnsmessage cannot carry
// inside a label.
func InstanceName(hostname string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(hostname), ".")
	if name == "" {
		return "massimo"
	}
	for len(name) > 63 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

// hostLabel is the label of the SRV target: massimo- and the last eight
// hex digits of the node, which a UUIDv7 holds in its random part.
func hostLabel(node string) string {
	var hex []rune
	for _, r := range strings.ToLower(node) {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' {
			hex = append(hex, r)
		}
	}
	if len(hex) > 8 {
		hex = hex[len(hex)-8:]
	}
	return "massimo-" + string(hex)
}

// names are the DNS names of a service.
type names struct {
	services dnsmessage.Name // _services._dns-sd._udp.local.
	service  dnsmessage.Name // _massimo._tcp.local.
	instance dnsmessage.Name // <instance>._massimo._tcp.local.
	host     dnsmessage.Name // massimo-<node>.local.
}

func namesOf(svc Service) (names, error) {
	var n names
	var err error
	for _, x := range []struct {
		to *dnsmessage.Name
		s  string
	}{{&n.services, "_services._dns-sd._udp.local."}, {&n.service, ServiceType + ".local."},
		{&n.instance, InstanceName(svc.Instance) + "." + ServiceType + ".local."}, {&n.host, hostLabel(svc.Node) + ".local."}} {
		if *x.to, err = dnsmessage.NewName(x.s); err != nil {
			return names{}, fmt.Errorf("announce: name %q: %w", x.s, err)
		}
	}
	return n, nil
}

// txt is the service's TXT record: node, v, tls, name and fp, as since
// the first announcement; url, loc, reg and api=1 (GET /healthz lists the
// features) for the integrations. An entry longer than a TXT string's
// 255 bytes is left out.
func txt(svc Service) []string {
	out := []string{"node=" + svc.Node, "v=" + svc.Version, "tls=0", "name=" + InstanceName(svc.Instance)}
	scheme := "http"
	if svc.Fingerprint != nil {
		out[2], scheme = "tls=1", "https"
		if fp := svc.Fingerprint(); fp != "" {
			out = append(out, "fp="+fp)
		}
	}
	url := svc.URL
	if url == "" {
		url = fmt.Sprintf("%s://%s.local:%d", scheme, hostLabel(svc.Node), svc.Port)
	}
	reg := "reg=0"
	if svc.Registration != nil && svc.Registration() {
		reg = "reg=1"
	}
	out = append(out, "url="+url, "loc="+cmp.Or(strings.TrimSpace(svc.Location), InstanceName(svc.Instance)), reg, "api=1")
	return slices.DeleteFunc(out, func(e string) bool { return len(e) > 255 })
}

// records holds what a response carries: the records it answers and
// those it adds.
type records struct {
	answers, additionals []dnsmessage.Resource
}

// recordSet is every record of a service, with ttl for each (0 is the
// goodbye), and the host's addresses.
type recordSet struct {
	n     names
	svc   Service
	addrs []net.IP
	ttl   func(long bool) uint32
	flush bool // set the cache-flush bit on the unique records
}

func (r recordSet) header(name dnsmessage.Name, typ dnsmessage.Type, unique, long bool) dnsmessage.ResourceHeader {
	class := dnsmessage.ClassINET
	if unique && r.flush {
		class |= cacheFlush
	}
	return dnsmessage.ResourceHeader{Name: name, Type: typ, Class: class, TTL: r.ttl(long)}
}

func (r recordSet) ptr() dnsmessage.Resource {
	return dnsmessage.Resource{Header: r.header(r.n.service, dnsmessage.TypePTR, false, false), Body: &dnsmessage.PTRResource{PTR: r.n.instance}}
}

func (r recordSet) servicesPTR() dnsmessage.Resource {
	return dnsmessage.Resource{Header: r.header(r.n.services, dnsmessage.TypePTR, false, true), Body: &dnsmessage.PTRResource{PTR: r.n.service}}
}

func (r recordSet) srv() dnsmessage.Resource {
	return dnsmessage.Resource{Header: r.header(r.n.instance, dnsmessage.TypeSRV, true, false), Body: &dnsmessage.SRVResource{Port: r.svc.Port, Target: r.n.host}}
}

func (r recordSet) txt() dnsmessage.Resource {
	return dnsmessage.Resource{Header: r.header(r.n.instance, dnsmessage.TypeTXT, true, true), Body: &dnsmessage.TXTResource{TXT: txt(r.svc)}}
}

// address are the host's A records, or its AAAA records with v6.
func (r recordSet) address(v6 bool) []dnsmessage.Resource {
	var out []dnsmessage.Resource
	for _, ip := range r.addrs {
		if ip4 := ip.To4(); ip4 != nil && !v6 {
			out = append(out, dnsmessage.Resource{Header: r.header(r.n.host, dnsmessage.TypeA, true, false), Body: &dnsmessage.AResource{A: [4]byte(ip4)}})
		} else if ip4 == nil && v6 && len(ip) == net.IPv6len {
			out = append(out, dnsmessage.Resource{Header: r.header(r.n.host, dnsmessage.TypeAAAA, true, false), Body: &dnsmessage.AAAAResource{AAAA: [16]byte(ip)}})
		}
	}
	return out
}

// all is every record, as an announcement and a goodbye carry them.
func (r recordSet) all() records {
	return records{answers: append(append([]dnsmessage.Resource{r.ptr(), r.srv(), r.txt()}, r.address(false)...), r.address(true)...)}
}

// answer is what the set answers for one question, nil for a question
// that is not about it.
func (r recordSet) answer(q dnsmessage.Question) records {
	is := func(n dnsmessage.Name) bool { return strings.EqualFold(q.Name.String(), n.String()) }
	typ := q.Type
	every := typ == dnsmessage.TypeALL
	var out records
	switch {
	case is(r.n.service) && (typ == dnsmessage.TypePTR || every):
		out.answers = []dnsmessage.Resource{r.ptr()}
		out.additionals = append(append([]dnsmessage.Resource{r.srv(), r.txt()}, r.address(false)...), r.address(true)...)
	case is(r.n.services) && (typ == dnsmessage.TypePTR || every):
		out.answers = []dnsmessage.Resource{r.servicesPTR()}
	case is(r.n.instance):
		if typ == dnsmessage.TypeSRV || every {
			out.answers = append(out.answers, r.srv())
			out.additionals = append(r.address(false), r.address(true)...)
		}
		if typ == dnsmessage.TypeTXT || every {
			out.answers = append(out.answers, r.txt())
		}
	case is(r.n.host):
		if typ == dnsmessage.TypeA || every {
			out.answers = append(out.answers, r.address(false)...)
		}
		if typ == dnsmessage.TypeAAAA || every {
			out.answers = append(out.answers, r.address(true)...)
		}
	}
	return out
}

// build writes a response: the header, the questions it answers (a
// legacy unicast answer repeats them), the answers and the additional
// records, each once.
func build(id uint16, questions []dnsmessage.Question, rs records) ([]byte, error) {
	b := dnsmessage.NewBuilder(make([]byte, 0, 512), dnsmessage.Header{ID: id, Response: true, Authoritative: true})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	for _, q := range questions {
		if err := b.Question(q); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	key := func(r dnsmessage.Resource) string {
		return r.Header.Name.String() + "/" + r.Header.Type.String() + "/" + r.Body.GoString()
	}
	add := func(list []dnsmessage.Resource) error {
		for _, r := range list {
			if k := key(r); !seen[k] {
				seen[k] = true
				if err := addResource(&b, r); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := b.StartAnswers(); err != nil {
		return nil, err
	}
	if err := add(rs.answers); err != nil {
		return nil, err
	}
	if err := b.StartAdditionals(); err != nil {
		return nil, err
	}
	if err := add(rs.additionals); err != nil {
		return nil, err
	}
	return b.Finish()
}

// addResource adds one record of the kinds a service has.
func addResource(b *dnsmessage.Builder, r dnsmessage.Resource) error {
	switch body := r.Body.(type) {
	case *dnsmessage.PTRResource:
		return b.PTRResource(r.Header, *body)
	case *dnsmessage.SRVResource:
		return b.SRVResource(r.Header, *body)
	case *dnsmessage.TXTResource:
		return b.TXTResource(r.Header, *body)
	case *dnsmessage.AResource:
		return b.AResource(r.Header, *body)
	case *dnsmessage.AAAAResource:
		return b.AAAAResource(r.Header, *body)
	}
	return fmt.Errorf("announce: record type %v", r.Header.Type)
}
