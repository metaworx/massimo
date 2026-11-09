// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package announce

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// testNode is a node_id of the form the store makes, a UUIDv7.
const testNode = "0190a5d2-0000-7000-8000-000000000001"

// The addresses the tests' responder gives as the host's.
var testAddrs = []net.IP{net.ParseIP("192.0.2.10").To4(), net.ParseIP("2001:db8::10")}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// freeUDPPort is a UDP port nobody holds on the loopback just now.
func freeUDPPort(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	return pc.LocalAddr().(*net.UDPAddr).Port
}

// sink is a socket on the loopback the responder announces to instead
// of the groups.
func sink(t *testing.T) net.PacketConn {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	return pc
}

// startTest starts a responder for svc on a port of its own, announcing
// to the sink.
func startTest(t *testing.T, svc Service, log *slog.Logger) (*Responder, int, net.PacketConn) {
	t.Helper()
	port, s := freeUDPPort(t), sink(t)
	r, err := start(context.Background(), svc, log, options{port: port, sendTo: []net.Addr{s.LocalAddr()}, hostAddrs: func() []net.IP { return testAddrs }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Stop)
	return r, port, s
}

func testService() Service {
	return Service{Instance: "box.example.lan", Port: 8099, Node: testNode, Version: "v0.12.0"}
}

// query is a legacy unicast query for one name and type.
func query(t *testing.T, id uint16, name string, typ dnsmessage.Type) []byte {
	t.Helper()
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id})
	b.StartQuestions()
	if err := b.Question(dnsmessage.Question{Name: dnsmessage.MustNewName(name), Type: typ, Class: dnsmessage.ClassINET}); err != nil {
		t.Fatal(err)
	}
	msg, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// ask sends msg from a port of its own to the responder on the loopback
// and returns the answer, or nil when none comes within wait.
func ask(t *testing.T, port int, msg []byte, wait time.Duration) *dnsmessage.Message {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	if _, err := pc.WriteTo(msg, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}); err != nil {
		t.Fatal(err)
	}
	pc.SetReadDeadline(time.Now().Add(wait))
	buf := make([]byte, 9000)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		return nil
	}
	var m dnsmessage.Message
	if err := m.Unpack(buf[:n]); err != nil {
		t.Fatal(err)
	}
	return &m
}

// byType are a message's records of one type, the answers' and the
// additional ones.
func byType(m *dnsmessage.Message, typ dnsmessage.Type) []dnsmessage.Resource {
	var out []dnsmessage.Resource
	for _, r := range append(append([]dnsmessage.Resource(nil), m.Answers...), m.Additionals...) {
		if r.Header.Type == typ {
			out = append(out, r)
		}
	}
	return out
}

// A legacy unicast PTR question for the service type is answered to its
// port, with its ID and question, the PTR, the SRV, the TXT and the
// addresses, at a TTL of ten seconds at most and without the cache-flush
// bit; a question for the host name gets its addresses, and one about
// another service nothing.
func TestAnnounceAnswersQuery(t *testing.T) {
	_, port, _ := startTest(t, testService(), quietLog())
	m := ask(t, port, query(t, 4711, "_massimo._tcp.local.", dnsmessage.TypePTR), 5*time.Second)
	if m == nil {
		t.Fatal("no answer")
	}
	if m.ID != 4711 || !m.Response || !m.Authoritative || len(m.Questions) != 1 || m.Questions[0].Name.String() != "_massimo._tcp.local." {
		t.Fatalf("header or question: %+v %+v", m.Header, m.Questions)
	}
	ptr := byType(m, dnsmessage.TypePTR)
	if len(m.Answers) != 1 || len(ptr) != 1 || ptr[0].Body.(*dnsmessage.PTRResource).PTR.String() != "box._massimo._tcp.local." {
		t.Fatalf("answers %+v", m.Answers)
	}
	srv := byType(m, dnsmessage.TypeSRV)
	if len(srv) != 1 || srv[0].Body.(*dnsmessage.SRVResource).Port != 8099 || srv[0].Body.(*dnsmessage.SRVResource).Target.String() != "massimo-00000001.local." {
		t.Fatalf("srv %+v", srv)
	}
	txt := byType(m, dnsmessage.TypeTXT)
	if len(txt) != 1 || !slices.Equal(txt[0].Body.(*dnsmessage.TXTResource).TXT, []string{"node=" + testNode, "v=v0.12.0", "tls=0", "name=box"}) {
		t.Fatalf("txt %+v", txt)
	}
	a, aaaa := byType(m, dnsmessage.TypeA), byType(m, dnsmessage.TypeAAAA)
	if len(a) != 1 || a[0].Body.(*dnsmessage.AResource).A != [4]byte{192, 0, 2, 10} || len(aaaa) != 1 || !net.IP(aaaa[0].Body.(*dnsmessage.AAAAResource).AAAA[:]).Equal(testAddrs[1]) {
		t.Fatalf("addresses %+v %+v", a, aaaa)
	}
	for _, r := range append(m.Answers, m.Additionals...) {
		if r.Header.TTL > ttlLegacy || r.Header.Class != dnsmessage.ClassINET {
			t.Errorf("%v: ttl %d class %#x", r.Header.Type, r.Header.TTL, uint16(r.Header.Class))
		}
	}

	host := ask(t, port, query(t, 1, "MASSIMO-00000001.local.", dnsmessage.TypeA), 5*time.Second)
	if host == nil || len(host.Answers) != 1 || host.Answers[0].Header.Type != dnsmessage.TypeA {
		t.Fatalf("host name: %+v", host)
	}
	if other := ask(t, port, query(t, 2, "_http._tcp.local.", dnsmessage.TypePTR), 300*time.Millisecond); other != nil {
		t.Fatalf("answered for another service: %+v", other)
	}
}

// With TLS the TXT record says so and carries the certificate's
// fingerprint as it is at the answer.
func TestAnnounceTLSFingerprint(t *testing.T) {
	svc := testService()
	fp := "AA:BB"
	var mu sync.Mutex
	svc.Fingerprint = func() string { mu.Lock(); defer mu.Unlock(); return fp }
	_, port, _ := startTest(t, svc, quietLog())
	for _, want := range []string{"AA:BB", "CC:DD"} {
		mu.Lock()
		fp = want
		mu.Unlock()
		m := ask(t, port, query(t, 3, "box._massimo._tcp.local.", dnsmessage.TypeTXT), 5*time.Second)
		if m == nil || len(m.Answers) != 1 {
			t.Fatalf("txt: %+v", m)
		}
		if got := m.Answers[0].Body.(*dnsmessage.TXTResource).TXT; !slices.Contains(got, "tls=1") || !slices.Contains(got, "fp="+want) {
			t.Fatalf("txt %v, want tls=1 and fp=%s", got, want)
		}
	}
}

// The responder announces itself at the start, with the cache-flush bit
// on its unique records, and says goodbye with TTL 0 at Stop.
func TestAnnounceGoodbye(t *testing.T) {
	r, _, s := startTest(t, testService(), quietLog())
	read := func() *dnsmessage.Message {
		s.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 9000)
		n, _, err := s.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		var m dnsmessage.Message
		if err := m.Unpack(buf[:n]); err != nil {
			t.Fatal(err)
		}
		return &m
	}
	first := read()
	if first.ID != 0 || !first.Response || len(first.Questions) != 0 || len(byType(first, dnsmessage.TypePTR)) != 1 {
		t.Fatalf("announcement %+v", first)
	}
	for _, rr := range first.Answers {
		unique := rr.Header.Type != dnsmessage.TypePTR
		if rr.Header.TTL == 0 || (rr.Header.Class&cacheFlush != 0) != unique {
			t.Errorf("announced %v: ttl %d class %#x", rr.Header.Type, rr.Header.TTL, uint16(rr.Header.Class))
		}
	}
	r.Stop()
	r.Stop() // a second Stop does nothing
	for {
		m := read()
		if ptr := byType(m, dnsmessage.TypePTR); len(ptr) == 1 && ptr[0].Header.TTL == 0 {
			for _, rr := range m.Answers {
				if rr.Header.TTL != 0 {
					t.Errorf("goodbye %v: ttl %d", rr.Header.Type, rr.Header.TTL)
				}
			}
			return
		}
	}
}

// Browse finds the responder by a unicast query, with what its records
// say.
func TestBrowseFindsResponder(t *testing.T) {
	svc := testService()
	svc.Fingerprint = func() string { return "AA:BB" }
	_, port, _ := startTest(t, svc, quietLog())
	found, err := browse(context.Background(), time.Second, []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1), Port: port}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("found %+v", found)
	}
	f := found[0]
	if f.Name != "box" || f.Host != "massimo-00000001.local." || f.Port != 8099 || f.Node != testNode || f.Version != "v0.12.0" || !f.TLS || f.Fingerprint != "AA:BB" ||
		!f.From.Equal(net.IPv4(127, 0, 0, 1)) || f.Address() != "127.0.0.1:8099" || !f.Has(testAddrs[0]) || !f.Has(testAddrs[1]) || f.Has(net.ParseIP("192.0.2.11")) {
		t.Fatalf("found %+v", f)
	}
}

// A family whose socket cannot be opened is left out with a line in the
// log; with neither, Start fails and names the cause.
func TestAnnounceWithoutMulticastWarns(t *testing.T) {
	refuse := errors.New("refused for the test")
	var log bytes.Buffer
	port, s := freeUDPPort(t), sink(t)
	o := options{port: port, sendTo: []net.Addr{s.LocalAddr()}, hostAddrs: func() []net.IP { return testAddrs },
		listen: func(network string, port int) (net.PacketConn, error) {
			if network == "udp6" {
				return nil, refuse
			}
			return listenShared(network, port)
		}}
	r, err := start(context.Background(), testService(), slog.New(slog.NewTextHandler(&log, nil)), o)
	if err != nil {
		t.Fatal(err)
	}
	r.Stop()
	if !strings.Contains(log.String(), "one address family not announced") || !strings.Contains(log.String(), "refused for the test") {
		t.Fatalf("log: %s", log.String())
	}
	o.listen = func(string, int) (net.PacketConn, error) { return nil, refuse }
	if _, err := start(context.Background(), testService(), quietLog(), o); err == nil || !errors.Is(err, refuse) || !strings.Contains(err.Error(), "no mDNS socket") {
		t.Fatalf("no socket: %v", err)
	}
}

// The instance name is the host name's first label, and the SRV target
// the node's last eight hex digits.
func TestNames(t *testing.T) {
	for in, want := range map[string]string{"box.example.lan": "box", "box": "box", "": "massimo", " ": "massimo", strings.Repeat("x", 70): strings.Repeat("x", 63)} {
		if got := InstanceName(in); got != want {
			t.Errorf("InstanceName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := hostLabel(testNode); got != "massimo-00000001" {
		t.Errorf("hostLabel %q", got)
	}
}
