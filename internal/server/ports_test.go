// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

import (
	"net"
	"sync"
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// heldPorts hands the server's proxies listeners the test holds
// (Options.proxyListen, given by listenOn), so that no other program
// takes a proxy's port between the test's choice of it and the proxy's
// listen, as one can where the test closes a listener for its port and
// the proxy opens another. A port held with hold goes to the first
// listen at its address; one held with taken only once the test gives
// it, the listens before failing as at a port another program holds,
// since the test holds it. Any other address listens as net.Listen does.
// The listens at each address are counted (listens), so that a test can
// see that a proxy tries no more.
type heldPorts struct {
	mu    sync.Mutex
	held  map[string]*heldPort
	tries map[string]int
}

// heldPort is a listener a test holds for a proxy, and whether the test
// gave it to the proxy yet.
type heldPort struct {
	ln    net.Listener
	given bool
}

// listenOn makes the server's proxies listen through ports.
func listenOn(ports *heldPorts) startOpt {
	return editOptions(func(o *Options) { o.proxyListen = ports.listen })
}

// hold holds a port of the loopback for a proxy, and returns its
// address.
func (h *heldPorts) hold(t *testing.T) string {
	t.Helper()
	addr, give := h.taken(t)
	give()
	return addr
}

// taken holds a port of the loopback that a proxy gets once give is
// called, and returns its address.
func (h *heldPorts) taken(t *testing.T) (addr string, give func()) {
	t.Helper()
	ln := testkit.Listen(t)
	addr = ln.Addr().String()
	p := &heldPort{ln: ln}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.held == nil {
		h.held = map[string]*heldPort{}
	}
	h.held[addr] = p
	return addr, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		p.given = true
	}
}

// listens is how many listens came at addr so far.
func (h *heldPorts) listens(addr string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tries[addr]
}

// listen is the proxies' listen (Options.proxyListen): the listener held
// at addr once it is given, once; net.Listen's otherwise.
func (h *heldPorts) listen(addr string) (net.Listener, error) {
	h.mu.Lock()
	if h.tries == nil {
		h.tries = map[string]int{}
	}
	h.tries[addr]++
	p := h.held[addr]
	if p != nil && p.given {
		delete(h.held, addr)
		h.mu.Unlock()
		return p.ln, nil
	}
	h.mu.Unlock()
	return net.Listen("tcp", addr)
}
