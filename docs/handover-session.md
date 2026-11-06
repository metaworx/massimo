# Handover, 2026-11-06

For the next session, on any machine.
Read README.md, AGENTS.md (the contract and the commit form), docs/roadmap.md,
then this file.
This file says where things are and what is still owed;
what was done is in CHANGELOG.md, one section per release, and in the commits.

Branches and releases: develop on `main` through pull requests the owner merges;
`alpha` is the release branch and the update channel,
promoted from `main` with `make release-alpha` (`BUMP=minor` for a feature,
`DRY=1` to see what it would cut, tag and push),
which cuts CHANGELOG.md's `[Unreleased]` into the release's section and tags
with it (README.md, "Versions").
Newest release: v0.8.0-alpha.1 (2026-11-06), the second with the store,
the first with users and login.
CI runs once per commit (a push to `main` or a pull request, never both;
a release branch runs nothing, being a fast-forward of a tested commit):
the Go checks, govulncheck, the service installs on Windows and macOS runners,
the web UI's pytest suite and the screenshot set in Microsoft's Playwright
image (the Screenshots job, two to three minutes),
and the guidelines' checks (`GUIDELINES/shared/tools/check`);
GUIDELINES/project/CI.md says what runs and why.
A ruleset on `main` requires those seven checks,
auto-merge and delete-on-merge are on,
so `gh pr merge --auto` waits for them and the branch goes with the merge;
the owner's own pushes bypass the ruleset.
The `gh` token has no `workflow` scope,
so `gh pr merge` refuses a pull request that changes `.github/workflows/`;
one (#55) was merged as a local signed `merge:` commit pushed to `main`;
`gh auth refresh -h github.com -s workflow` once, by the owner, lifts that.

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
  Its next update, to v0.8.0-alpha.1, creates `/var/lib/massimo/massimo.db`,
  seeds it from the env file, migrates it to version 2 and creates the user
  `owner` from the API token; nobody has watched that yet.
- **The spare Cube**
  (KEQ0537249) is on jaguar's network at .46 by DHCP,
  served by a Massimo Windows service on jaguar (.45: proxy on 62910,
  web UI on 8099).
  It is the Cube for hardware tests from jaguar and for the firmware dump.
  Its stored fallback address is .46 static since the netconfig test.
  The service ran the store's first start (version 1) on 2026-11-06;
  the update to v0.8.0-alpha.1 (version 2,
  the user `owner`) is not watched either.
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
A client of the API needs a token since v0.8.0-alpha.1:
`MASSIMO_API_TOKEN` is the owner's key, every other user has one of their own
(docs/api-v1.md, "Authentication"); without one the API answers 401.

## The checkout on jaguar

Go 1.26, `gh`, commit and tag signing through the repository's git config
and KeePassXC's SSH agent (run `timeout 60 git commit …`:
a locked agent hangs instead of failing),
the Playwright venv for `make ui-test` in tools/ui-test/.venv.
The conventions are AGENTS.md and GUIDELINES/project/_CONTRACT.md;
docs/review-2026-11-05.md has the coding standard they came from.
The web UI's tests are the pytest suite in tools/ui-test/ (`make ui-test`;
one test with `PYTEST_ADDOPTS='-k "<name> and 390-de"'`,
tools/ui-test/README.md); a UI change brings its test there,
and CI keeps a failed test's trace,
screenshot and video as the artifact `ui-test-results`.
The branches `claude/tls*` and `backup/claude/tls*` (ten) and
`claude/ci-update-race` are merged and to delete;
the worktree `.claude/worktrees/xenodochial-pasteur-c0e0c8` (store stage (b),
detached) can go.

## Decisions waiting for the owner

- The built-in colour schemes: Massimo,
  High contrast and Graphite are the ones shipped;
  the owner picks the final set.
- Real Cube serials and room names in the documents, the test reports and the
  screenshot set under docs/screenshots (in the public history already);
  a fixture with placeholder names would take them out of the set.
- The items of docs/store.md section 10 marked open, and docs/server.md's,
  once the server mode starts.
- The packaging plan's steps for the owner (its NOTE, "The owner's steps left"):
  the two Debian package checks added to the ruleset,
  the apt key and its passphrase in the `signing` environment with Pages
  deploying from Actions,
  the tap repository `metaworx/homebrew-massimo` with its deploy key in the secrets,
  and the Pi moved to the package once a release carries it.
- The `workflow` scope for `gh` (above).

## Hardware checks still owed

1. The store's first start on the Pi and the update on the Windows service:
   the log of the update to v0.8.0-alpha.1 (`store created`,
   `cube added from the settings file`, `owner created from MASSIMO_API_TOKEN`),
   `massimo store info`, Settings → Cubes and Settings → Users,
   every client still in with its token.
2. TLS, the owner's day (the TLS plan's NOTE, item 10):
   a certificate from Let's Encrypt's staging directory on the Pi with a name
   and the router's port-443 forward
   (`serve -tls acme -tls-names <name> -acme-directory <staging URL>`,
   or `install` with the same flags);
   the same through the Infomaniak hook
   (`-acme-dns-hook packaging/acme-dns-hooks/infomaniak.sh`,
   the token in `/etc/massimo/infomaniak.token` readable by the service user,
   `curl` and `jq` installed;
   the hook ran against a fake of Infomaniak's API only);
   the self-signed notice on a phone.
   HSTS is on for a year once TLS serves:
   a box that goes back to `http://` runs once with `-hsts 0` first.
3. Pairing (docs/api-v1.md, "Pairing"): the six steps of pull request #10,
   the assumptions marked unverified in docs/portal-protocol.md.
4. Write test step 1 repeated,
   to watch the follow-up re-reads and the budget reconnect in the log and the stream,
   once from the original software through the proxy.
5. The UI on the boxes: real ages and a stale thermostat, a window opening live,
   a stream reconnect while `serve` restarts, the log texts with real changes.
6. Windows on a real PC: docs/testing-windows.md step 7b (the Update button).
7. macOS: a reboot with nobody logged in, the firewall rule across an update,
   an Intel Mac, a real Cube or the proxy on a Mac.
8. netconfig: a broadcast `C` to a Cube on a wrong subnet;
   `discover` on a Cube with a static address.
9. Discovery on the Pi with the spare Cube on the same network,
   now with the DHCP flag in the found row.
10. The Debian package on the Pi, once pull request #63 is in:
    the service's start, which the container test cannot prove.

## Code follow-ups

- Pairing does not copy the room's settings to a new device;
  removing a device has no API (the protocol line `t:` is built and tested,
  sent by nobody);
  the UI has no pairing panel (a session brief is ready).
- The backup and restore panel in the UI (the API is in),
  and the notice when the server is gone (briefs ready).
- Settings → Server in the UI (intervals, the stale threshold,
  the certificate's details, which sit in the header's expert list for now);
  the API exists.
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
- CI's macOS job takes its rollback branch at the update from the API until
  an alpha release knows `-tls` (CI.md section 7); the next release ends that.
  No test reaches Let's Encrypt; a Pebble job in CI would be a plan of its own,
  as would UPnP and a local CA.
- docs/reverse-proxy.md's Synology and QNAP paragraphs come from their documentation,
  not from a NAS.
- The armv6 binary is 14.5 MB since autocert (704 KiB more);
  no release carries it yet, so the Pi 1 has not run it.
- On the Pi: delete `~/massimo-alpha12.*`,
  `/usr/local/bin/massimo.alpha9.bak` and, once the M7 setup is settled,
  `/etc/massimo/env.before-m7`; keep `massimo.prev`.

The lessons of the hardware days are in docs/portal-protocol.md ("Lessons from
the hardware days") and packaging/README.md ("Learnings").

## Plans open on main

- **AP Tls v1.1**
  (GUIDELINES/wip/, with its analysis and its NOTE):
  its five blocks are merged (#53, #55, #57, #58, #59);
  it retires with `chore(aps): -Tls` after the owner's TLS day,
  on the owner's `EXEC retire`, never earlier.

## Sessions in flight

- **Packaging** runs in a session of its own on the branch
  `claude/heuristic-noether-90a440` (pull request #63): AP Packaging v1.0,
  a Debian package by nfpm with a marker that makes `massimo update` defer to apt,
  an apt repository on GitHub Pages rebuilt at each release,
  a Homebrew tap rendered by a workflow; its plan,
  analyses and NOTE are on that branch.
  Windows signing, rpm, MSI, pkg, a container and LoxBerry wait (its D5, D8).

The store stage (b) session and the TLS session are done and deleted;
the session that fixed CI's Windows update race (#62) too.

## Next steps in order

1. The owner's items above: the secrets and the key on the Mint box,
   the M7 reservations, the Mac's sudoers file, the Mint key on the Pi,
   the `gh` scope.
2. The update to v0.8.0-alpha.1 on the Pi and the Windows service (Settings → Software),
   watched as hardware check 1 says; then the TLS day, then pairing.
3. Pull request #63 (packaging) to `main`, the owner's steps of its NOTE,
   a release with TLS and the packages (`BUMP=minor`).
4. The UI sessions from the briefs: backup and restore, the pairing panel,
   the notice when the server is gone, written from the components.
5. Firmware dump when the adapter is in (docs/firmware-extraction.md).
6. The server mode (docs/server.md), Loxone.
