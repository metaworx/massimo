// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package fakecube

import "sync"

// applyHold holds the fake's apply (HoldApply): while holds is above 0
// the commands that fall due wait in held.
type applyHold struct {
	mu    sync.Mutex // taken before Server.mu; orders the applies
	holds int        // HoldApply calls not yet released
	held  [][]byte   // the commands due meanwhile, in order
}

// HoldApply makes the fake apply nothing until the release it returns,
// as devices asleep until their wakeup: each command that falls due
// meanwhile, at its acknowledgement or ApplyAfter later, is applied at
// the release, in the order they fell due, before the release returns.
// A test holds the apply to see a write on its way for as long as it
// needs, and releases it for the confirmation. Holds nest: the last
// release applies; a second call of one release does nothing.
func (s *Server) HoldApply() (release func()) {
	h := &s.held
	h.mu.Lock()
	h.holds++
	h.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.holds--; h.holds > 0 {
				return
			}
			for _, raw := range h.held {
				s.applyNow(raw)
			}
			h.held = nil
		})
	}
}

// applyOrHold applies a command that fell due, or keeps it for the
// release while the apply is held.
func (s *Server) applyOrHold(raw []byte) {
	h := &s.held
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.holds > 0 {
		h.held = append(h.held, raw)
		return
	}
	s.applyNow(raw)
}
