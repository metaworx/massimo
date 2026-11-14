// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package announce

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Found is a server that announced itself.
type Found struct {
	// Name is the instance name, the server's host name.
	Name string `json:"name"`
	// Host is the SRV record's target, Port the API's port.
	Host string `json:"host"`
	Port int    `json:"port"`
	// From is the address the answer came from; Addrs are the target's
	// A and AAAA records.
	From  net.IP   `json:"from"`
	Addrs []net.IP `json:"addrs,omitempty"`
	// From the TXT record.
	Node        string `json:"node,omitempty"`
	Version     string `json:"version,omitempty"`
	TLS         bool   `json:"tls"`
	Fingerprint string `json:"fingerprint,omitempty"`
	// TXT is every entry of the TXT record, the keys lower case.
	TXT map[string]string `json:"-"`
}

// Address is where the server's API is reached, host:port: the address
// the answer came from.
func (f Found) Address() string { return net.JoinHostPort(f.From.String(), fmt.Sprint(f.Port)) }

// Has says whether ip is one of the server's addresses.
func (f Found) Has(ip net.IP) bool {
	return f.From.Equal(ip) || slices.ContainsFunc(f.Addrs, ip.Equal)
}

// Browse asks for the servers on the LAN and collects the answers that
// come within wait: by a query to the mDNS groups on every interface, or,
// with unicast addresses given, to those instead (for another subnet,
// or NAT setups such as WSL 2). Either is a legacy unicast query, which
// a responder answers to the asking port.
func Browse(ctx context.Context, wait time.Duration, unicast ...net.IP) ([]Found, error) {
	var targets []*net.UDPAddr
	for _, ip := range unicast {
		targets = append(targets, &net.UDPAddr{IP: ip, Port: Port})
	}
	return browse(ctx, wait, ServiceType, targets, len(targets) == 0)
}

// HomeAssistantType is the DNS-SD service type of Home Assistant.
const HomeAssistantType = "_home-assistant._tcp"

// HomeAssistant is a Home Assistant installation that announced itself
// (its zeroconf record): the hint of an integration that may ask to be
// registered, and the confirmation of a request's installation id.
type HomeAssistant struct {
	Name        string `json:"name"`         // the instance name
	From        net.IP `json:"from"`         // the address the answer came from
	Port        int    `json:"port"`         // its port
	UUID        string `json:"uuid"`         // the installation id, core.uuid
	Location    string `json:"location"`     // location_name, the home's name
	Version     string `json:"version"`      // its version
	InternalURL string `json:"internal_url"` // internal_url, or base_url
	ExternalURL string `json:"external_url,omitempty"`
}

// BrowseHomeAssistant asks for the Home Assistant installations on the
// LAN, as Browse asks for the servers.
func BrowseHomeAssistant(ctx context.Context, wait time.Duration, unicast ...net.IP) ([]HomeAssistant, error) {
	var targets []*net.UDPAddr
	for _, ip := range unicast {
		targets = append(targets, &net.UDPAddr{IP: ip, Port: Port})
	}
	found, err := browse(ctx, wait, HomeAssistantType, targets, len(targets) == 0)
	if err != nil {
		return nil, err
	}
	return homeAssistants(found), nil
}

// homeAssistants reads Home Assistant's TXT entries.
func homeAssistants(found []Found) []HomeAssistant {
	out := []HomeAssistant{}
	for _, f := range found {
		out = append(out, HomeAssistant{Name: f.Name, From: f.From, Port: f.Port, UUID: strings.ToLower(f.TXT["uuid"]), Location: f.TXT["location_name"],
			Version: f.TXT["version"], InternalURL: cmp.Or(f.TXT["internal_url"], f.TXT["base_url"]), ExternalURL: f.TXT["external_url"]})
	}
	return out
}

// browse sends the query for a service type to targets, and to the
// groups with multicast.
func browse(ctx context.Context, wait time.Duration, svcType string, targets []*net.UDPAddr, multicast bool) ([]Found, error) {
	query, err := browseQuery(svcType)
	if err != nil {
		return nil, err
	}
	col := collector{svcType: svcType}
	var wg sync.WaitGroup
	deadline := time.Now().Add(wait)
	opened := 0
	var errs []error
	for _, network := range []string{"udp4", "udp6"} {
		v6 := network == "udp6"
		var mine []*net.UDPAddr
		for _, t := range targets {
			if (t.IP.To4() == nil) == v6 {
				mine = append(mine, t)
			}
		}
		if !multicast && len(mine) == 0 {
			continue
		}
		pc, err := net.ListenPacket(network, ":0")
		if err != nil {
			errs = append(errs, err)
			continue
		}
		opened++
		for _, t := range mine {
			pc.WriteTo(query, t)
		}
		if multicast {
			sendToGroup(pc, v6, query)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer pc.Close()
			stop := context.AfterFunc(ctx, func() { pc.SetReadDeadline(time.Now()) })
			defer stop()
			pc.SetReadDeadline(deadline)
			buf := make([]byte, 9000)
			for {
				n, from, err := pc.ReadFrom(buf)
				if errors.Is(err, net.ErrClosed) || isTimeout(err) {
					return
				}
				if err != nil {
					continue
				}
				if src, ok := from.(*net.UDPAddr); ok {
					col.add(buf[:n], src.IP)
				}
			}
		}()
	}
	if opened == 0 {
		return nil, fmt.Errorf("announce: no socket to browse from: %w", errors.Join(errs...))
	}
	wg.Wait()
	return col.found(), nil
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// browseQuery is the PTR question for a service type.
func browseQuery(svcType string) ([]byte, error) {
	name, err := dnsmessage.NewName(svcType + ".local.")
	if err != nil {
		return nil, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: uint16(rand.UintN(1 << 16))})
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	if err := b.Question(dnsmessage.Question{Name: name, Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}); err != nil {
		return nil, err
	}
	return b.Finish()
}

// sendToGroup sends the query to the family's group on every interface
// that takes multicast, or, where none does, the way the routes say.
func sendToGroup(pc net.PacketConn, v6 bool, query []byte) {
	group := &net.UDPAddr{IP: groupV4, Port: Port}
	if v6 {
		group = &net.UDPAddr{IP: groupV6, Port: Port}
	}
	ifs, _ := net.Interfaces()
	sent := 0
	for _, ifi := range ifs {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 || !hasFamily(ifi, v6) {
			continue
		}
		var err error
		if v6 {
			err = ipv6.NewPacketConn(pc).SetMulticastInterface(&ifi)
		} else {
			err = ipv4.NewPacketConn(pc).SetMulticastInterface(&ifi)
		}
		if err == nil {
			if _, err := pc.WriteTo(query, group); err == nil {
				sent++
			}
		}
	}
	if sent == 0 {
		pc.WriteTo(query, group)
	}
}

// collector gathers the records of the answers, per address they came
// from.
type collector struct {
	svcType   string // asked for, such as ServiceType
	mu        sync.Mutex
	instances map[string]*Found   // from + name of the instance, lower case
	hosts     map[string][]net.IP // from + host name, lower case
	order     []string            // the instances' keys as they came
}

// add takes one answer's records.
func (c *collector) add(msg []byte, from net.IP) {
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil || !h.Response {
		return
	}
	if err := p.SkipAllQuestions(); err != nil {
		return
	}
	var rs []dnsmessage.Resource
	answers, err := p.AllAnswers()
	if err != nil {
		return
	}
	rs = append(rs, answers...)
	if err := p.SkipAllAuthorities(); err == nil {
		if extra, err := p.AllAdditionals(); err == nil {
			rs = append(rs, extra...)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.instances == nil {
		c.instances, c.hosts = map[string]*Found{}, map[string][]net.IP{}
	}
	serviceSuffix := "." + c.svcType + ".local."
	key := func(name string) string { return from.String() + "|" + strings.ToLower(name) }
	instance := func(name string) *Found {
		k := key(name)
		f, ok := c.instances[k]
		if !ok {
			f = &Found{Name: name[:len(name)-len(serviceSuffix)], From: from}
			c.instances[k] = f
			c.order = append(c.order, k)
		}
		return f
	}
	isInstance := func(name string) bool {
		return len(name) > len(serviceSuffix) && strings.EqualFold(name[len(name)-len(serviceSuffix):], serviceSuffix)
	}
	for _, r := range rs {
		name := r.Header.Name.String()
		switch body := r.Body.(type) {
		case *dnsmessage.PTRResource:
			if strings.EqualFold(name, c.svcType+".local.") && isInstance(body.PTR.String()) && r.Header.TTL > 0 {
				instance(body.PTR.String())
			}
		case *dnsmessage.SRVResource:
			if isInstance(name) && r.Header.TTL > 0 {
				f := instance(name)
				f.Host, f.Port = body.Target.String(), int(body.Port)
			}
		case *dnsmessage.TXTResource:
			if isInstance(name) && r.Header.TTL > 0 {
				f := instance(name)
				f.TXT = map[string]string{}
				for _, kv := range body.TXT {
					k, v, _ := strings.Cut(kv, "=")
					f.TXT[strings.ToLower(k)] = v
					switch strings.ToLower(k) {
					case "node":
						f.Node = v
					case "v":
						f.Version = v
					case "tls":
						f.TLS = v == "1"
					case "fp":
						f.Fingerprint = v
					}
				}
			}
		case *dnsmessage.AResource:
			c.hosts[key(name)] = appendIP(c.hosts[key(name)], net.IP(body.A[:]))
		case *dnsmessage.AAAAResource:
			c.hosts[key(name)] = appendIP(c.hosts[key(name)], net.IP(body.AAAA[:]))
		}
	}
}

func appendIP(list []net.IP, ip net.IP) []net.IP {
	if slices.ContainsFunc(list, ip.Equal) {
		return list
	}
	return append(list, append(net.IP(nil), ip...))
}

// found are the instances with an SRV record, one per node (a Home
// Assistant's uuid): a server that answered from two addresses (IPv4 and
// IPv6) is listed once, by the first answer, an IPv4 one where there is
// one.
func (c *collector) found() []Found {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Found
	byNode := map[string]int{}
	for _, k := range c.order {
		f := *c.instances[k]
		if f.Host == "" {
			continue
		}
		f.Addrs = c.hosts[f.From.String()+"|"+strings.ToLower(f.Host)]
		if id := cmp.Or(f.Node, f.TXT["uuid"]); id != "" {
			if i, ok := byNode[id]; ok {
				if out[i].From.To4() == nil && f.From.To4() != nil {
					out[i] = f
				}
				continue
			}
			byNode[id] = len(out)
		}
		out = append(out, f)
	}
	slices.SortStableFunc(out, func(a, b Found) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return bytes.Compare(a.From.To16(), b.From.To16())
	})
	return out
}
