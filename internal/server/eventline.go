// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// An event's line in the process log (AP OneLog v1.1, Block 5): every
// event the log keeps is also one line of serve's log, written from the
// event itself, so that the two say the same and no separate slog call
// repeats it. The line carries the event's level, its source and kind,
// the Cube, the device and its serial, the command and who started it,
// then what the event says in short. The process log is then the log's
// rows and the lines the store cannot hold: the proxy's, the
// connection's retries, the listeners, the store's own faults.

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"go.metaworx.ch/massimo/internal/engine"
	"go.metaworx.ch/massimo/internal/store"
	"go.metaworx.ch/massimo/internal/view"
)

// eventMessages are the lines' messages where a kind's words do not say
// it, kept as the separate lines said them before; failedMessages those
// of a server's job that did not happen, which says why in its error.
var (
	eventMessages = map[string]string{
		"state":            "device state",
		"connected":        "cube connected",
		"disconnected":     "cube connection lost",
		"cube_found":       "cube found on the LAN",
		"cube_gone":        "cube gone from the LAN",
		"address_changed":  "cube address from discovery",
		"paired":           "device paired",
		"device_forgotten": "device forgotten",
		"backup":           "backup taken",
		"reboot":           "reboot requested",
		"reboot_cancelled": "reboot taken back",
		"restart":          "restart requested",
		"update":           "update requested",
	}
	failedMessages = map[string]string{
		"backup":       "backup failed",
		"reboot":       "reboot not started",
		"restart":      "restart not started",
		"update":       "update helper not started",
		"restore_done": "restore failed",
		"restore_step": "restore step failed",
	}
)

// eventMessage is an event's line's message: its kind's own, else the
// kind's words.
func eventMessage(ev event) string {
	switch ev.Kind {
	case "command":
		if state, ok := ev.Summary["state"].(string); ok {
			return "command " + state
		}
	case "hold":
		switch ev.Summary["reason"] {
		case engine.HoldDutyCycle:
			return "radio duty cycle exhausted, waiting for the hourly reset"
		case engine.HoldMemorySlots:
			return "few free memory slots on the cube, holding radio commands"
		}
	}
	if ev.Error != "" {
		if m, ok := failedMessages[ev.Kind]; ok {
			return m
		}
	}
	if m, ok := eventMessages[ev.Kind]; ok {
		return m
	}
	return strings.ReplaceAll(ev.Kind, "_", " ")
}

// eventLine writes a settled event's line to log, a logger without a
// source of its own, for the Cube of serial ("" for the server's).
func eventLine(log *slog.Logger, serial string, ev event) {
	level, _ := store.ParseLevel(ev.Level)
	ctx := context.Background()
	if log == nil || !log.Enabled(ctx, slog.Level(level)) {
		return
	}
	attrs := []slog.Attr{slog.String("source", ev.Source), slog.String("kind", ev.Kind)}
	add := func(key string, v any) { attrs = append(attrs, slog.Any(key, v)) }
	if serial != "" {
		add("cube", serial)
	}
	if ev.Device != "" {
		add("device", ev.Device)
	}
	if ev.Serial != "" && ev.Serial != serial {
		add("serial", ev.Serial)
	}
	if ev.Command != 0 {
		add("command", ev.Command)
	}
	switch {
	case ev.By != nil:
		add("by", ev.By.Name)
	case ev.userID != 0:
		add("user_id", ev.userID)
	}
	if ev.Error != "" {
		add("error", ev.Error)
	}
	if dv, ok := ev.State.(view.Device); ok && ev.Kind == "state" {
		// The device's values, as the state line said them before.
		kv := stateAttrs(serial, dv)
		for i := 4; i+1 < len(kv); i += 2 { // after its cube and device
			add(kv[i].(string), kv[i+1])
		}
	}
	if len(ev.Changes) > 0 {
		add("changed", changedNames(ev.Changes))
	}
	if ev.Cube != nil {
		add("ip", ev.Cube.Address)
		if ev.Cube.MAC != "" {
			add("mac", ev.Cube.MAC)
		}
	}
	if ev.Paired != nil {
		add("type", ev.Paired.Type.String())
		if ev.Serial == "" {
			add("serial", ev.Paired.Serial)
		}
	}
	if r := ev.Restore; r != nil {
		add("id", r.ID)
		if r.Step != 0 {
			add("step", r.Step)
		}
		add("steps", r.Steps)
		if r.What != "" {
			add("what", r.What)
		}
		if r.Discarded {
			add("discarded", true)
		}
		if ev.Kind == "restore_done" {
			add("sent", r.Sent)
		}
	}
	keys := make([]string, 0, len(ev.Summary))
	for k := range ev.Summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		add(k, summaryValue(ev.Summary[k]))
	}
	log.LogAttrs(ctx, slog.Level(level), eventMessage(ev), attrs...)
}

// summaryValue is a summary's value as a line prints it: a list or a
// map in short rather than as Go writes it.
func summaryValue(v any) any {
	switch x := v.(type) {
	case []string:
		return strings.Join(x, ",")
	case map[string]any, []any:
		return fmt.Sprint(x)
	}
	return v
}

// line writes the event's line for the Cube.
func (rt *runtime) line(ev event) { eventLine(rt.evlog, rt.cubeSerial(), ev) }
