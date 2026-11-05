# Roadmap

The frame for this list is docs/design.md: target environments
(standalone, edge, server), the architecture, the drivers, Loxone
ownership, multi-user and the public server, and the phases with
their status. This file is the near-term order within that frame.

Owner's priorities as of 2026-11-04 evening, after a day on real
hardware. The items are in order; the first ones are the next session.

Done the same night (phase-0 up to the merge of UI round five): the
configuration API (names, moves, device and room configuration, week
programmes), the UI panels for them with the week programme editor and
copy-to-rooms, the log view, Cube names from `.env`, and the UI details
of item 3. What remains of items 1 to 3 is hardware verification of
each new write path and the feedback that follows.

1. **Room and device configuration in the API and the UI.** The
   original UI's Configuration menu per room: Temperatures (eco,
   comfort, window-open temperature, maximum), Device Settings (boost
   duration and valve opening, descaling day and time, temperature
   offset per device), Structure (room names, device names, delete
   rooms, delete devices, move devices), Holiday mode, and Copy
   configuration to other rooms. The protocol builders (0x11, 0x12,
   metadata) and the restore planner already do all of this; what is
   missing is API endpoints (PUT per room or device, metadata writes
   for names) and inline panels in the UI. Device name and offset first.
2. **Week programme per room**: edit one day, all days, or a selection
   of days, with a copy-day function, in the UI and the API (0x10 per
   day, builders and planner exist).
3. **UI details**: IS and SET temperatures at the same size in the tile
   ("IS" is the measured temperature: always from a wall thermostat,
   only occasionally from a radiator thermostat, which reports it with
   some status messages), the stepper buttons on one side as up/down
   chevrons, a HOLIDAY button next to AUTO and MANU with the end date on
   hover (suitcase as in the original), a manual refresh button, the
   "House…" button labelled MANU, expert details with firmware, IP
   address and radio address of the Cube, the time of the last poll
   with seconds, the Expert switch out of the header (it lives in the
   settings panel).
4. **Pairing** new devices through the server and the UI (`n:`/`x:`,
   the sequence in portal-protocol.md), including through the proxy.
5. **Firmware dump** of Cube 1 once the FT232H is in (docs/
   firmware-extraction.md), then the DataFlash, then the portal reply
   format and the update encryption from the code.
6. **Users and a database**: owner arbitration per Cube, settings such
   as the hidden devices editable in the UI, SQLite; internal/env is the
   one place the configuration is read today, to be replaced by it.
7. **Translations** of the UI: done for the languages of the original
   software (English, German, French, Italian, Dutch, Polish) in
   `internal/server/ui/i18n.js`; open: a review of FR/IT/NL/PL by
   native speakers, and the server's error texts, which still reach the
   UI in English.
8. **Release engineering**: done: CI (gofmt, vet, tests, cross build) on
   every push and pull request; on a `v*` tag, binaries for Linux (amd64,
   arm64, armv7, armv6), macOS (amd64, arm64) and Windows, `SHA256SUMS`
   and an Ed25519-signed manifest published as a GitHub release, with the
   tag's branch as its update channel; `massimo update` (`-check`,
   `-rollback`, health check with automatic rollback), and the web UI's
   update button through a root helper unit (packaging/README.md);
   `massimo install` sets serve up as a systemd service, with the update
   helper. Open: install on macOS (a LaunchDaemon) and Windows (a Windows
   service, which Windows updates need too: rename the running .exe,
   restart through the service manager), and the future `edge` mode.
9. **Timestamps per value**: keep the time each value was last received
   from the Cube, per device and field, in the snapshot, the API
   (`state` fields with their `*_at` times or a `received` map) and the
   UI (age next to the value, greyed when stale). It matters most for
   radiator thermostats, which report their measured temperature only
   occasionally, so an old IS value should not pass as current; also
   for valve position, window state and battery. Add a last-contact time
   per device and a "not seen for N hours" warning. Server side done
   2026-11-05: `received` per value, `last_seen` and `stale` (thermostats
   unchanged for 24 hours) on devices and rooms, `observed_since` on the
   Cube (docs/api-v1.md); the UI part, ages next to the values and greyed
   stale ones, is open.
10. Later: Loxone, a radio-stick driver without a Cube.

Open UI feedback from 2026-11-04 is listed in handover-session.md.
