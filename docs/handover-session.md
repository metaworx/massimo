# Handover, 2026-11-06

For the next session, on any machine.
Read README.md, AGENTS.md (the contract and the commit form), docs/roadmap.md,
then this file.
This file says where things are and what is still owed;
what was done is in CHANGELOG.md, one section per release, and in the commits.

Branches and releases: develop on `main` through pull requests the owner merges;
`alpha` is the release branch and the update channel,
promoted from `main` with `make release-alpha` (`BUMP=minor` for a feature).
Newest release: v0.7.0-alpha.1 (2026-11-06), the first with the store.
CI runs the Go checks, the service installs on Windows and macOS runners,
the web UI smoke test with the screenshot set (the Screenshots job),
and the guidelines' checks (`GUIDELINES/shared/tools/check`) on every push
and pull request.
A ruleset on `main` requires those seven checks and auto-merge is on,
so `gh pr merge --auto` waits for them; the owner's own pushes bypass it.

## Where things run

- **The house Cube**
  (JEQ0192422) is on the M7 network at .118 by DHCP, not reserved yet.
  Only the Raspberry Pi reaches it.
- **The Raspberry Pi 1**
  (Raspberry Pi OS Lite 32-bit) runs `massimo serve` as a systemd service on the
  alpha channel: **.200** is the API and web UI on port 80,
  **.201** the proxy for the house Cube.
  Settings in `/etc/massimo/env`;
  its DHCP address is not reserved either (the M7 range is 20–199).
  The first start after the update to v0.7.0-alpha.1 creates
  `/var/lib/massimo/massimo.db` and seeds it from the env file;
  nobody has watched that yet.
- **The spare Cube**
  (KEQ0537249) is on jaguar's network at .46 by DHCP,
  served by a Massimo Windows service on jaguar (.45: proxy on 62910,
  web UI on 8099).
  It is the Cube for hardware tests from jaguar and for the firmware dump.
  Its stored fallback address is .46 static since the netconfig test.
- **jaguar** is the development machine, in WSL 2 in NAT mode:
  no LAN broadcast and no inbound connections,
  so `massimo discover` and a proxy from WSL do not work;
  outbound to the Pi and to the Windows service does.
  Windows binaries built with `GOOS=windows` run from WSL through interop
  and use Windows networking;
  elevation goes through `gsudo` with the owner's UAC confirmation.
  The checkout's `.env` selects the spare Cube through the service's proxy;
  the house Cube is reached with `-sn JEQ0192422` through .201.
- **The Mac** (192.168.1.25) had the LaunchDaemon test install, purged;
  `/etc/sudoers.d/massimo-test` is still to delete.
- **The Mint box** is retired.
  Its `.env` copies and the release key in its session scratchpad are to delete,
  and its SSH key is to remove from the Pi.

Every command that talks to a Cube goes through the proxy of the `serve` that holds it;
a direct connection takes the Cube's only slot (`-no-direct` refuses the fallback,
`-direct` forces it).

## The checkout on jaguar

Go 1.26, `gh`, commit and tag signing through the repository's git config
and KeePassXC's SSH agent (run `timeout 60 git commit …`:
a locked agent hangs instead of failing),
the Playwright venv for `make ui-test` in tools/ui-test/.venv.
The conventions are AGENTS.md and GUIDELINES/project/_CONTRACT.md;
docs/review-2026-11-05.md has the coding standard they came from.

## Decisions waiting for the owner

- The built-in colour schemes: Massimo,
  High contrast and Graphite are the ones shipped;
  the owner picks the final set.
- Real Cube serials and room names in the documents, the test reports and the
  screenshot set under docs/screenshots (in the public history already);
  a fixture with placeholder names would take them out of the set.
- The items of docs/store.md section 10 marked open, and docs/server.md's,
  once the server mode starts.

## Hardware checks still owed

1. The store's first start on the Pi:
   the log of the update to v0.7.0-alpha.1 (`store created`,
   `cube added from the settings file`), `massimo store info`,
   and Settings → Cubes.
   On jaguar's Windows service it went as designed on 2026-11-06:
   the file created and migrated to version 1, the node key written,
   the Cube seeded from the env file (`source: env`), the node id in the API,
   `store info` and `config` reading the file beside the running service.
2. Pairing (docs/api-v1.md, "Pairing"): the six steps of pull request #10,
   the assumptions marked unverified in docs/portal-protocol.md.
3. Write test step 1 repeated,
   to watch the follow-up re-reads and the budget reconnect in the log and the stream.
4. The UI on the boxes: real ages and a stale thermostat, a window opening live,
   a stream reconnect while `serve` restarts, the log texts with real changes.
5. Windows on a real PC: docs/testing-windows.md step 7b (the Update button),
   now that v0.7.0-alpha.1 has it.
6. macOS: a reboot with nobody logged in, the firewall rule across an update,
   an Intel Mac, a real Cube or the proxy on a Mac.
7. netconfig: a broadcast `C` to a Cube on a wrong subnet;
   `discover` on a Cube with a static address.
8. Discovery on the Pi with the spare Cube on the same network.

## Code follow-ups

- Pairing does not copy the room's settings to a new device;
  removing a device has no API;
  the UI has no pairing panel (a session brief is ready).
- The backup and restore panel in the UI (the API is in),
  and the notice when the server is gone (briefs ready).
- Settings → Server in the UI (intervals, the stale threshold); the API exists.
- The UI could ignore the short `connected: false` during the six-hourly refresh
  and the budget reconnects.
- Server error texts reach the UI in English; FR, IT,
  NL and PL await a native speaker.
- `serve -log` writes to the file and to stderr,
  twice under a supervisor that keeps stderr.
- The web UI is built from components.js,
  partials under internal/server/ui/partials/ and one translation file per
  language (docs/ui.md, "How the page is built"); the pairing,
  backup and Users panels are written from them.
  `detailsPanel` and `stepper` are written and have no caller yet.
- The UI smoke test runs in CI's Screenshots job since 2026-11-06;
  docs/ui-testing.md recommends moving it to pytest and keeping Playwright,
  not Cypress.
- On the Pi: delete `~/massimo-alpha12.*`,
  `/usr/local/bin/massimo.alpha9.bak` and, once the M7 setup is settled,
  `/etc/massimo/env.before-m7`; keep `massimo.prev`.

The lessons of the hardware days are in docs/portal-protocol.md ("Lessons from
the hardware days") and packaging/README.md ("Learnings").

## Sessions in flight

- **Store stage (b)** (users, login, the capability model, the audit,
  `commands`; docs/store.md section 9) runs in a session of its own on the
  branch `claude/store-b`; its login form in the UI waits for the components,
  which are in.
- **TLS for `serve`** is being planned in a session of its own from an
  evaluation of 2026-11-06: certificate files by flags with reload,
  self-signed at first start, the reverse-proxy support,
  autocert with TLS-ALPN-01; a DNS-01 hook and UPnP later.
  Its reverse-proxy block touches auth.go after stage (b) lands.

## Next steps in order

1. The owner's items above: the secrets and the key on the Mint box,
   the M7 reservations, the Mac's sudoers file, the Mint key on the Pi.
2. The store's first start on the Pi (Settings → Software),
   then the hardware checks, pairing first.
3. The UI sessions from the briefs: backup and restore, the pairing panel,
   the notice when the server is gone, written from the components.
4. The two sessions in flight to their pull requests: store stage (b), TLS.
5. Firmware dump when the adapter is in (docs/firmware-extraction.md).
6. The server mode (docs/server.md), Loxone.
