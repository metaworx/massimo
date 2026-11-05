# Roadmap

The frame for this list is docs/design.md: target environments
(standalone, edge, server), the architecture, the drivers, Loxone
ownership, multi-user and the public server, and the phases with
their status. This file is the near-term order within that frame;
docs/handover-session.md says where the last session stopped and what
the next one does first.

Status as of 2026-11-05 evening (v0.5.0-alpha.1). The items keep their
numbers from the owner's list of 2026-11-04; done items stay for the
record, the open part of each is marked.

1. **Room and device configuration in the API and the UI.** Done:
   temperatures, device settings, names, moves, holiday mode, copy to
   other rooms, with inline panels and help buttons. Verified on
   hardware on 2026-11-05 (docs/write-test-2026-11-05.md, steps 1 to
   7). Open: deleting rooms and devices has no API (`t:` is only
   passed through the proxy).
2. **Week programme per room.** Done: the editor selects the days with
   the same programme, "Only {day}", a copy section, resets; verified on
   hardware (step 4 of the write test).
3. **UI details.** Done, plus the owner's later wishes: ages and stale
   warnings, the "Open" badge, the blue accent, device identifiers in
   expert mode only, the log that says what changed. Open: the owner's
   look at all of it on the boxes (handover, "Hardware checks").
4. **Pairing** new devices through the server and the UI (`n:`/`x:`,
   the sequence in portal-protocol.md), including through the proxy.
   Server side done 2026-11-05 against the fake Cube (API
   `…/pairing`, adding a paired device to a room with
   `PUT …/devices/{rf}`, pairing through the proxy). Open, in order:
   the six-step test at the Cube, the pairing panel in the UI, copying
   the room's settings to the new device as the original software
   does, and removing a device.
5. **Firmware dump** of the spare Cube once the FT232H is in
   (docs/firmware-extraction.md), then the DataFlash, then the portal
   reply format and the update encryption from the code.
6. **Users and a database**: owner arbitration per Cube, settings such
   as the hidden devices editable in the UI, SQLite; internal/env is
   the one place the configuration is read today, to be replaced by
   it. Next big piece. A design document comes first: which settings
   move from `.env`, the permission model (design.md, section 6), what
   stays configurable by file for the Pi, migration of an existing
   `.env`; then the owner's decisions, then code.
7. **Translations** of the UI: done for the six languages of the
   original software in `internal/server/ui/i18n.js`; open: a native
   speaker's review of FR, IT, NL and PL, and the server's error
   texts, which still reach the UI in English.
8. **Release engineering**: done: CI with gofmt, vet, tests, cross
   builds and govulncheck; signed releases from `v*` tags for Linux
   (amd64, arm64, armv7, armv6), macOS and Windows; `massimo update`
   with health check and rollback; `massimo install` as a systemd
   service, a Windows service or macOS LaunchDaemons, with the update
   helper for the web UI's button on Linux and macOS;
   `make release-alpha`. Open: the web UI starting updates on Windows;
   the Windows service on a real PC (docs/testing-windows.md); macOS
   notarization, `.pkg` or Homebrew, log rotation through SIGHUP; the
   future `edge` mode.
9. **Timestamps per value**: done, server and UI (`received`,
   `last_seen`, `stale`, `observed_since`; ages, greyed values, "Not
   seen for N hours"). Open: the stale threshold once real data shows
   what fits.
10. **Finding Cubes**: done 2026-11-05: `serve` discovers at start,
    every 30 minutes and on `POST /api/v1/cubes/discover`, lists
    unconfigured Cubes, follows the address of Cubes configured by
    serial, and the UI has "Search for new Cubes…". Open: the check on
    the Pi with the spare Cube; the DHCP flag in `massimo discover`.
11. **After writes**: done: re-reads of the device configuration and a
    budget reconnect (kept by the owner's decision), accepted commands
    in the log. Open: an on-demand budget refresh in the API and the
    UI; a follow-up for writes from proxy clients; no direct fallback
    to the Cube in the command line when a proxy is configured.
12. Later: Loxone (design.md, section 4), a radio-stick driver without
    a Cube (phase 3), the edge agent and the public server (phase 4).

The small code follow-ups and the hardware checks still owed are
listed in docs/handover-session.md.
