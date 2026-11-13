// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The Cube routes of the store (docs/store.md section 9 (a)): add a
// Cube, change its configuration and remove it while the server runs.
// Each writes the row first and then applies it to the runtime: a new
// runtime starts, a changed address reconnects, a changed proxy address
// restarts that listener, a changed hidden list applies at once.

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/protocol/cube"
	"go.metaworx.ch/massimo/internal/store"
)

// maxCubeName is the longest Cube name accepted, in bytes.
const maxCubeName = 64

var serialPattern = regexp.MustCompile(`^[A-Z0-9]{1,16}$`)

// cubeRequest is the body of PUT /api/v1/cubes/{sn} and POST
// /api/v1/cubes; absent fields stay as they are.
type cubeRequest struct {
	Serial          string    `json:"serial,omitempty"` // POST only
	Name            *string   `json:"name"`
	Address         *string   `json:"address"`
	FollowDiscovery *bool     `json:"follow_discovery"`
	ProxyAddress    *string   `json:"proxy_address"`
	ProxySerial     *string   `json:"proxy_serial"`
	Hidden          *[]string `json:"hidden"`
	// ProxyVirtual is kept apart from the Cube's row (proxy_virtual.go).
	ProxyVirtual *bool `json:"proxy_virtual"`
}

// validAddress checks a host[:port] address; "" is allowed.
func validAddress(field, a string) error {
	if a == "" {
		return nil
	}
	host, port, err := net.SplitHostPort(withCubePort(a))
	if err != nil || host == "" && field == "address" || strings.ContainsAny(a, " /\\") {
		return fail(http.StatusBadRequest, "%s %q: want host or host:port", field, a)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fail(http.StatusBadRequest, "%s %q: bad port", field, a)
	}
	return nil
}

// apply checks the request's fields and writes them into a row.
func (req cubeRequest) apply(c *store.Cube) error {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if len(name) > maxCubeName || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return fail(http.StatusBadRequest, "name: at most %d bytes of text", maxCubeName)
		}
		c.Name = name
	}
	if req.Address != nil {
		a := strings.TrimSpace(*req.Address)
		if err := validAddress("address", a); err != nil {
			return err
		}
		c.Address = a
	}
	if req.FollowDiscovery != nil {
		c.FollowDiscovery = *req.FollowDiscovery
	}
	if c.Address == "" {
		c.FollowDiscovery = true // nothing else to connect to
	}
	if req.ProxyAddress != nil {
		a := strings.TrimSpace(*req.ProxyAddress)
		if err := validAddress("proxy_address", a); err != nil {
			return err
		}
		c.ProxyAddress = a
	}
	if req.ProxySerial != nil {
		ps := strings.ToUpper(strings.TrimSpace(*req.ProxySerial))
		if ps != "" && !serialPattern.MatchString(ps) {
			return fail(http.StatusBadRequest, "proxy_serial %q: letters and digits, at most 16", ps)
		}
		c.ProxySerial = ps
	}
	if req.Hidden != nil {
		list := []string{}
		for _, a := range *req.Hidden {
			rf, err := cube.ParseRFAddress(strings.TrimSpace(a))
			if err != nil {
				return fail(http.StatusBadRequest, "hidden: %v", err)
			}
			list = append(list, rf.String())
		}
		c.Hidden = list
	}
	c.Source = store.SourceUI
	return nil
}

// cubeStore is the store, or the error of a server without one.
func (s *Server) cubeStore() (*store.Store, error) {
	if s.opts.Store == nil {
		return nil, fail(http.StatusServiceUnavailable, "this server has no store")
	}
	return s.opts.Store, nil
}

// decodeCube reads a cube request; unknown fields are refused, so that a
// typo does not pass as "no change".
func decodeCube(r *http.Request) (cubeRequest, error) {
	var req cubeRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fail(http.StatusBadRequest, "bad body: %v", err)
	}
	return req, nil
}

// storeFail maps a store error to a status.
func storeFail(err error) error {
	var he *httpError
	switch {
	case errors.As(err, &he):
		return err
	case errors.Is(err, store.ErrNotFound):
		return fail(http.StatusNotFound, "%v", err)
	case errors.Is(err, store.ErrExists):
		return fail(http.StatusConflict, "%v", err)
	}
	return fail(http.StatusInternalServerError, "%v", err)
}

// postCube handles POST /api/v1/cubes: serve a Cube discovery found, or
// one given by serial, from now on. The body is {"serial": …} with any
// of the fields of PUT; without an address the Cube follows discovery.
func (s *Server) postCube(w http.ResponseWriter, r *http.Request) error {
	s.cubeMu.Lock() // one add, change or remove at a time: check, write and start together
	defer s.cubeMu.Unlock()
	st, err := s.cubeStore()
	if err != nil {
		return err
	}
	req, err := decodeCube(r)
	if err != nil {
		return err
	}
	serial := store.NormalSerial(req.Serial)
	if !serialPattern.MatchString(serial) {
		return fail(http.StatusBadRequest, "serial %q: letters and digits, at most 16", req.Serial)
	}
	if s.configured(serial) != nil {
		return fail(http.StatusConflict, "cube %s is served already", serial)
	}
	auditTarget(r, "cube:"+serial)
	row, known, err := st.Cube(r.Context(), serial)
	if err != nil {
		return storeFail(err)
	}
	change := func(c *store.Cube) error {
		if err := req.apply(c); err != nil {
			return err
		}
		c.Configured = true
		return nil
	}
	if known {
		row, err = st.UpdateCube(r.Context(), serial, change)
	} else {
		row = store.Cube{Serial: serial}
		if err = change(&row); err == nil {
			row, err = st.AddCube(r.Context(), row)
		}
	}
	if err != nil {
		return storeFail(err)
	}
	s.noteRow(row)
	rt, err := s.addCube(row)
	if err != nil {
		return fail(http.StatusConflict, "%v", err)
	}
	s.log.Info("cube added", "serial", serial, "address", row.Address, "follow_discovery", row.FollowDiscovery, "remote", s.clientIP(r))
	writeJSON(w, http.StatusCreated, s.summary(rt))
	return nil
}

// storedCube returns the runtime of the path's Cube and its row, or the
// error of a Cube that has none: one served for this run only.
func (s *Server) storedCube(r *http.Request) (*runtime, *store.Store, error) {
	rt, err := s.cube(r)
	if err != nil {
		return nil, nil, err
	}
	st, err := s.cubeStore()
	if err != nil {
		return nil, nil, err
	}
	if rt.cubeSerial() == "" {
		return nil, nil, fail(http.StatusConflict, "the cube has not reported its serial yet")
	}
	row, ok, err := st.Cube(r.Context(), rt.cubeSerial())
	if err != nil {
		return nil, nil, storeFail(err)
	}
	if !ok || !row.Configured || rt.config().Source == store.SourceFlag {
		return nil, nil, fail(http.StatusConflict, "cube %s is served for this run only (-addr or -sn), not from the store", rt.cubeSerial())
	}
	return rt, st, nil
}

// putCube handles PUT /api/v1/cubes/{sn}: any of name, address,
// follow_discovery, proxy_address, proxy_serial and hidden.
func (s *Server) putCube(w http.ResponseWriter, r *http.Request) error {
	s.cubeMu.Lock() // one add, change or remove at a time: check, write and start together
	defer s.cubeMu.Unlock()
	rt, st, err := s.storedCube(r)
	if err != nil {
		return err
	}
	req, err := decodeCube(r)
	if err != nil {
		return err
	}
	if req.Serial != "" {
		return fail(http.StatusBadRequest, "the serial is the path's")
	}
	before, _, _ := st.Cube(r.Context(), rt.cubeSerial())
	row, err := st.UpdateCube(r.Context(), rt.cubeSerial(), req.apply)
	if err != nil {
		return storeFail(err)
	}
	auditCubeChanges(r, before, row)
	if req.ProxyVirtual != nil {
		if err := s.setProxyVirtual(r, rt, row.ID, *req.ProxyVirtual); err != nil {
			return err
		}
	}
	s.noteRow(row)
	if err := s.applyCube(rt, row); err != nil {
		return fail(http.StatusInternalServerError, "%v", err)
	}
	s.log.Info("cube changed", "serial", rt.cubeSerial(), "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, s.summary(rt))
	return nil
}

// auditCubeChanges tells the change line what a Cube's change changed:
// its name, its address, whether it follows discovery, its proxy and its
// hidden devices, from and to.
func auditCubeChanges(r *http.Request, before, after store.Cube) {
	for _, c := range []struct {
		name     string
		from, to any
	}{
		{"name", before.Name, after.Name},
		{"address", before.Address, after.Address},
		{"follow_discovery", before.FollowDiscovery, after.FollowDiscovery},
		{"proxy_address", before.ProxyAddress, after.ProxyAddress},
		{"proxy_serial", before.ProxySerial, after.ProxySerial},
		{"hidden", strings.Join(before.Hidden, ","), strings.Join(after.Hidden, ",")},
	} {
		if c.from != c.to {
			auditChange(r, c.name, c.from, c.to)
		}
	}
}

// applyCube brings a running Cube in line with its changed row.
func (s *Server) applyCube(rt *runtime, row store.Cube) error {
	cfg, err := CubeFromRow(row)
	if err != nil {
		return err
	}
	old := rt.config()
	rt.setConfig(func(c *CubeConfig) { *c = cfg })
	addr := cfg.Addr
	if addr == "" {
		if f, ok := s.foundInfo(rt.cubeSerial()); ok && f.Address != "" {
			addr = net.JoinHostPort(f.Address, strconv.Itoa(s.cubePort()))
		}
	}
	had := rt.client.Address() != ""
	if addr != "" && rt.client.SetAddress(addr) {
		s.log.Info("cube address changed", "serial", rt.cubeSerial(), "address", addr)
		if had {
			s.mu.RLock()
			ctx := s.ctx
			s.mu.RUnlock()
			if ctx != nil {
				go rt.queue.Enqueue(engine.Command{Key: "refresh", Priority: engine.PriorityNormal, Reconnect: true, Moved: true}).Wait(ctx)
			}
		}
	}
	if addr == "" && !had {
		s.wakeDiscovery()
	}
	if !s.opts.ProxyFixed && (old.ProxyAddress != cfg.ProxyAddress || old.ProxySerial != cfg.ProxySerial) {
		s.stopProxies(rt)
		s.startCubeProxy(rt)
	}
	// The hidden list: the views and the proxy read it at every use.
	return nil
}

// deleteCube handles DELETE /api/v1/cubes/{sn}: the server stops serving
// the Cube; its row stays with configured = 0 (and its history with it),
// and it is listed as found once discovery sees it.
func (s *Server) deleteCube(w http.ResponseWriter, r *http.Request) error {
	s.cubeMu.Lock() // one add, change or remove at a time: check, write and start together
	defer s.cubeMu.Unlock()
	rt, st, err := s.storedCube(r)
	if err != nil {
		return err
	}
	row, err := st.UpdateCube(r.Context(), rt.cubeSerial(), func(c *store.Cube) error {
		c.Configured, c.Source = false, store.SourceUI
		return nil
	})
	if err != nil {
		return storeFail(err)
	}
	s.noteRow(row)
	s.removeCube(rt)
	s.log.Info("cube removed", "serial", rt.cubeSerial(), "remote", s.clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "serial": rt.cubeSerial()})
	return nil
}
