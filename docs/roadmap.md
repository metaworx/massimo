# Roadmap

The frame for this list is docs/design.md: target environments (standalone,
edge, server), the architecture, the drivers, Loxone ownership,
multi-user and the public server, and the phases with their status.
This file is the near-term order within that frame;
docs/handover-session.md says where the last session stopped and what the next
one does first.

Status as of 2026-11-05 night (v0.5.0-alpha.1 plus the pull requests
#22 to #29 of the first session on jaguar, docs/handover-session.md).
The items keep their numbers from the owner's list of 2026-11-04;
done items stay for the record, the open part of each is marked.
The plan behind items 12 to 15 and the storage work is docs/proposal-2026-11-05.md.

1. **Room and device configuration in the API and the UI.**
   Done: temperatures, device settings, names, moves, holiday mode,
   copy to other rooms, with inline panels and help buttons.
   Verified on hardware on 2026-11-05 (docs/write-test-2026-11-05.md,
   steps 1 to 7).
   Open: deleting rooms and devices has no API (`t:` is only passed through the proxy).
2. **Week programme per room.**
   Done: the editor selects the days with the same programme, "Only {day}",
   a copy section, resets; verified on hardware (step 4 of the write test).
3. **UI details.**
   Done, plus the owner's later wishes: ages and stale warnings,
   the "Open" badge, the blue accent, device identifiers in expert mode only,
   the log that says what changed.
   Open: the owner's look at all of it on the boxes (handover,
   "Hardware checks").
4. **Pairing** new devices through the server and the UI (`n:`/`x:`,
   the sequence in portal-protocol.md), including through the proxy.
   Server side done 2026-11-05 against the fake Cube (API `…/pairing`,
   adding a paired device to a room with `PUT …/devices/{rf}`,
   pairing through the proxy).
   Open, in order: the six-step test at the Cube, the pairing panel in the UI,
   copying the room's settings to the new device as the original software does,
   and removing a device.
5. **Firmware dump** of the spare Cube once the FT232H is in
   (docs/firmware-extraction.md), then the DataFlash, then the portal
   reply format and the update encryption from the code.
6. **Users and a database**:
   owner arbitration per Cube,
   settings such as the hidden devices editable in the UI, SQLite;
   internal/env is the one place the configuration is read today,
   to be replaced by it.
   Next big piece.
   A design document comes first: which settings move from `.env`,
   the permission model (design.md, section 6),
   what stays configurable by file for the Pi, migration of an existing `.env`;
   then the owner's decisions, then code.
   The design document is written: docs/store.md (pull request #27),
   SQLite through `modernc.org/sqlite` behind `internal/store`, the schema,
   the roles, the migration of an installed box and the stages (a) to (e),
   with eleven decisions for the owner in its section 10.
   Code after the decisions.
7. **Translations** of the UI:
   done for the six languages of the original software in `internal/server/ui/i18n.js`;
   open: a native speaker's review of FR, IT, NL and PL,
   and the server's error texts, which still reach the UI in English.
8. **Release engineering**:
   done: CI with gofmt, vet, tests, cross builds and govulncheck;
   signed releases from `v*` tags for Linux (amd64, arm64, armv7, armv6),
   macOS and Windows; `massimo update` with health check and rollback;
   `massimo install` as a systemd service,
   a Windows service or macOS LaunchDaemons,
   with the update helper for the web UI's button on Linux and macOS;
   `make release-alpha`;
   the Windows service tested on a real PC (docs/windows-test-2026-11-05.md,
   all steps passed);
   an Installing section per operating system in the README (#26).
   Open: the web UI starting updates on Windows;
   the follow-ups of the Windows report (quiet install steps,
   `service.log` duplicating `serve.log`,
   `massimo.exe.failed` never cleaned up); macOS notarization,
   `.pkg` or Homebrew, log rotation through SIGHUP; the future `edge` mode.
9. **Timestamps per value**:
   done, server and UI (`received`, `last_seen`, `stale`, `observed_since`;
   ages, greyed values, "Not seen for N hours").
   Open: the stale threshold once real data shows what fits.
10. **Finding Cubes**:
    done 2026-11-05: `serve` discovers at start,
    every 30 minutes and on `POST /api/v1/cubes/discover`,
    lists unconfigured Cubes, follows the address of Cubes configured by serial,
    and the UI has "Search for new Cubes…".
    Open: the check on the Pi with the spare Cube;
    the DHCP flag in `massimo discover`.
11. **After writes**:
    done: re-reads of the device configuration and a budget reconnect (kept by
    the owner's decision), accepted commands in the log.
    Open: an on-demand budget refresh in the API and the UI;
    a follow-up for writes from proxy clients;
    no direct fallback to the Cube in the command line when a proxy is configured.
12. **Colour schemes**:
    done in the UI (#29): light, dark or system mode,
    three built-in schemes (Massimo, High contrast, Graphite),
    a custom scheme with a contrast check,
    export and import as `massimo-scheme/1`, kept in the browser.
    Open: the owner's choice of the built-in set;
    the per-user copy in the store (item 6, stage c);
    a phrase in the README's UI paragraph.
13. **Backup and restore in the web UI**:
    the server side is done (#28): a plan from an uploaded backup with its cost,
    the send checked against the plan's hash, progress on the stream,
    the guard rails (one restore at a time, 80 % duty cycle, 1 MB,
    the Cube's own serial).
    Open: the Settings panel (download, upload, plan, confirm, progress;
    a session brief is ready),
    then the backup history once the store exists (item 6, stage e).
14. **The UI when the server is gone**:
    a prominent notice that the connection to the server is lost (as opposed to
    the Cube being disconnected), edits blocked, polling until it is back.
    Open; a session brief is ready.
15. **Software section**:
    both buttons greyed out while an update runs, also after a reload (#25).
    Open: starting updates from the web UI on Windows (item 8).
16. Later: Loxone (design.md, section 4),
    a radio-stick driver without a Cube (phase 3),
    the edge agent and the public server (phase 4).

The small code follow-ups and the hardware checks still owed are
listed in docs/handover-session.md.
