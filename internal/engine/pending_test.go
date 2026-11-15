// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package engine

import (
	"errors"
	"testing"
)

// Pending finds a waiting command by what match accepts, and no longer
// once it is withdrawn.
func TestPending(t *testing.T) {
	q := NewQueue(nil, Options{})
	byKey := func(key string) func(Command) bool { return func(c Command) bool { return c.Key == key } }
	if q.Pending(byKey("setpoint:room:2")) {
		t.Fatal("an empty queue has a command")
	}
	tk := q.Enqueue(Command{Key: "setpoint:room:2", Priority: PriorityHigh})
	q.Enqueue(Command{Key: "refresh", Reconnect: true})
	if !q.Pending(byKey("setpoint:room:2")) || !q.Pending(byKey("refresh")) {
		t.Fatal("a waiting command not found")
	}
	if q.Pending(byKey("setpoint:room:3")) {
		t.Fatal("a command of another room found")
	}
	q.Withdraw(tk, errors.New("withdrawn"))
	if q.Pending(byKey("setpoint:room:2")) {
		t.Fatal("a withdrawn command still found")
	}
}
