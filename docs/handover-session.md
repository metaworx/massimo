# Handover from the session of 2026-11-04

For the next session, local or cloud. Develop on `main` (pull requests
go there); `alpha` (called `phase-0` until 2026-11-05) is the release
branch and update channel of the alpha releases, promoted from `main`
with `make release-alpha` (`BUMP=minor` for a feature: v0.2.0-alpha.1,
v0.3.0-alpha.1, …; without it for a fix: v0.2.0-alpha.2); README.md,
"Versions". Read README.md, then docs/roadmap.md, then this file.

Done on 2026-11-05 after the UI rounds: docs/design.md and
docs/background.md (the proposal, the evaluation, the sources), the
code review docs/review-2026-11-05.md with its first tier fixed
(constant-time token check, security headers on the UI, one state
constructor `backup.StateOf`, one metadata mirror
`Client.ApplyMetadataWrite`, comments on six helpers, the OpenAPI
coverage test, MANU without a guessed setpoint, pending radiograms
against the Cube's observed slot capacity), release engineering on the
roadmap, the ufw ports in docs/proxy.md.

The review's second tier, also on 2026-11-05: three extractions,
internal/view (the read model: hidden set, visible rooms and devices,
the room-ready rule `Silent`, the pending count), internal/cubeio (writes
that keep the snapshot in step, used by the API, the proxy and the
restore planner) and internal/env (every variable lookup, the .env
loader, the address resolution); "hidden" as the one term
(`-show-hidden`, older flags kept); the proxy refuses `t:` for hidden
devices; `serve -refresh` reconnects to each Cube every six hours; the
README's Cube variables as a table; a summary at the top of
docs/portal-protocol.md. Each item in the review says where it went.
Security 2 and 3 followed: a per-address lockout after ten wrong tokens
in a minute, and single-use stream tickets (`POST …/events/ticket`,
`?ticket=`); `?token=` on the stream is deprecated and still accepted.
The web UI opens the stream with a ticket per connection since branch
claude/ui-value-ages, so `?token=` can go in a later release. Roadmap
item 9: every device with state carries `received` (per value, when
the server saw it appear), `last_seen` and `stale`; rooms carry
`stale`, the Cube `observed_since` (docs/api-v1.md says what the times
can and cannot mean); the UI shows them (below). Still open in the
review: logic 3 (documented); logic 6 and 7, duplication 6 and the
Playwright smoke test were done in the UI follow-ups.

Release engineering (roadmap 8): the repository is public; CI and the
release workflow are in .github/workflows; a tag publishes signed
releases (the key is the `signing` environment's `MASSIMO_RELEASE_KEY`,
only usable by `v*` tags; the public half is
internal/update/release-key.pem); `make release-alpha` fast-forwards
`alpha` to `main` and tags it with the channel, pushed together; `massimo
update` and the web UI's Settings → Software install releases through
the root helper units in packaging/systemd (packaging/README.md,
"Updating"). On Windows, `massimo install` makes a Windows service and
`massimo update -restart massimo` updates it from an administrator
prompt (packaging/README.md, "Windows"); CI tests that on a Windows
runner. On macOS `massimo install` makes two LaunchDaemons, the server as
the hidden user `_massimo` and the root update helper started by
launchd's WatchPaths on the request file, with the Linux paths
(packaging/README.md, "macOS"); `massimo update -restart
ch.metaworx.massimo` restarts it with `launchctl kickstart -k`. Tested on
a Mac (install, -listen, the web UI update to v0.3.0-alpha.1, rollback
by hand and automatic, remove -purge) and in a CI job on macos-latest.
The release check asks GitHub's API without a token, limited to 60
requests an hour per address; MASSIMO_GITHUB_TOKEN lifts that, and CI
passes the job's token (the shared macOS runners hit the limit).
CI also runs govulncheck with the toolchain pinned in the workflows
(Go 1.26, newest patch; go.mod keeps the minimum, 1.24).

Done later on 2026-11-05 (branch claude/ui-feedback-9b6fa2, four ui
commits): the owner's UI feedback. The week programme editor selects the
days with the same programme, has the "Only {day}" switch, a copy
section and Reset day / Reset all; the Copy tab has a Rooms and a
Settings section with switches; a help button next to every setting
explains it; all texts go through the translation table in
internal/server/ui/i18n.js, English and German, with a Language setting
(Settings → Software included). Verified with Playwright (Python,
Chromium) against the fake Cube at 1100 and 390 px, in English and
German: no console errors, no horizontal scroll, the new controls
reachable and operable by keyboard. Not yet seen on real hardware.

Follow-ups the same day (branch claude/ui-followups): Forget token
fixed, "mixed" for rooms whose thermostats differ in mode, a note on
the paused auto-reload in the settings, shared UI helpers in app.js,
`make ui-test`, help texts checked against the eQ-3 manuals, French,
Italian, Dutch and Polish, and docs/ui.md.

Then (branch claude/ui-value-ages): the ages of the values on the
tiles and in the device list, greyed values and "Not seen for N hours"
for stale devices, "Open" in the tile's head of a room with an open
window (the owner's wish after the write test), and the event stream
with a ticket per connection, reopened with a fresh one after an
error. `make ui-test` runs the server with a token and covers all of
it. Not yet seen on real hardware; a value received within 5 s of
`observed_since` counts as already there ("≥"), since the first
device list arrives just after it, not at that exact time.

## State

- `massimo serve` keeps both Cubes connected, serves the JSON API
  (docs/api-v1.md, docs/api-v1-openapi.yaml), answers as a Cube on the
  LAN for other clients (docs/proxy.md; the Cube serves one client at a
  time, so everything goes through serve) and serves the web UI at `/`.
- Configuration in `.env` on the Mint box (192.168.1.44, second address
  192.168.1.47 for the second Cube's proxy): per Cube `_ADDRESS`, `_KEY`,
  `_IV`, `_NAME`, `_PROXY`, `_PROXY_SERIAL`; `MASSIMO_API_TOKEN`,
  `MASSIMO_LOG`. Room 255 on the JEQ Cube holds the three ghost contacts
  (docs/portal-protocol.md explains why); it is hidden everywhere.
- All hardware findings are in docs/portal-protocol.md. The firmware
  dump plan is docs/firmware-extraction.md (parts ordered or to order:
  FT232H, 2x10 header, Pomona 5250 clip; pin 1 of ST3 still to be
  measured).
- Not yet verified on real thermostats: the configuration endpoints and
  the UI panels on them. Test plan: docs/handover-write-test.md. Run it
  before trusting those panels.

## Conventions

- Commits end with the Co-Authored-By and Claude-Session lines used
  throughout the log; no model names in code or commits.
- No keys, IVs, tokens, real device names or serials in fixtures, docs
  or commits; `.env` and backups are gitignored.
- UI: plain HTML/CSS/JS in internal/server/ui/, go:embed, no framework,
  no inline handlers, textContent for API text, CSS variables for light
  and dark, phone first (390 px), verified with Playwright against the
  fake Cube; `make ui-test` (tools/ui-test) runs the smoke test, and
  its greeting.txt is the fixture with device states.
- Bigger features were done as parallel forks in git worktrees and
  merged; the same works locally with worktrees or branches.

## Open UI points

- Owner's earlier remark: opening device list or configuration is per
  tile; a global switch could come later.
- Help texts were checked against eQ-3's thermostat manuals; the
  manuals do not describe maximum valve and valve offset (the offset
  text follows FHEM's MAX module). The Dutch manual gives descaling at
  Saturday 11:00, the English one and our devices 12:00; the texts say
  12:00.
- Translations: all six languages are in i18n.js; FR/IT/NL/PL are
  translations with the manuals' terms, not yet read by a native
  speaker. A de-CH variant would only need the words with "ß". Error
  texts from the server reach the UI in English.
- The Copy tab copies what is stored for the room, not unsent edits of
  the other tabs, and says so; if the owner expects the edits, it can
  take c.prog.days and c.edit instead.
- Playwright set-up on the Mint box (no node, no ensurepip): see
  tools/ui-test/README.md.

## Next steps in order

1. The write test (docs/handover-write-test.md) on the JEQ Cube:
   steps 1 to 6 passed (docs/write-test-2026-11-05.md); its findings 1
   and 2 are addressed (re-reads and a budget reconnect after writes,
   accepted commands in the log); step 7, moving a device, is still to
   run.
2. The owner's review of the UI rounds (PR 3, PR 5 and the ages,
   Open and ticket branch claude/ui-value-ages) on the boxes, and the
   open UI points above.
3. Pairing new devices (roadmap item 4): the server side is in (API
   `POST/GET/DELETE …/pairing`, `paired` events, `PUT …/devices/{rf}`
   with `room_id` for a device in no room, pairing through the proxy);
   next the test at the Cube (the assumptions are listed as unverified
   in docs/portal-protocol.md, pairing section) and the UI panel from
   docs/api-v1.md.
4. Finding Cubes: serve runs discovery at the start, every 30 minutes
   and on `POST /api/v1/cubes/discover`, lists Cubes that are not
   configured in `GET /api/v1/cubes` (`"configured": false`) and serves
   Cubes configured by serial only at the address they answer from; the
   Cube list's refresh in the UI builds on it. To check on the Pi: a
   second Cube on the LAN shows up after a refresh, serve's own proxy
   answers do not.
5. Firmware dump when the adapter is in (docs/firmware-extraction.md).
6. Users and a database (roadmap item 6).
