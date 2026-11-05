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
roadmap, the ufw ports in docs/proxy.md. The review's second tier is
still open.

Release engineering (roadmap 8): the repository is public; CI and the
release workflow are in .github/workflows; a tag publishes signed
releases (the key is the `signing` environment's `MASSIMO_RELEASE_KEY`,
only usable by `v*` tags; the public half is
internal/update/release-key.pem); `make release-alpha` fast-forwards
`alpha` to `main` and tags it with the channel, pushed together; `massimo
update` and the web UI's Settings → Software install releases through
the root helper units in packaging/systemd (packaging/README.md,
"Updating").

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
  fake Cube (`internal/cubelan/fakecube` with the fixture greeting; a
  greeting with device states lives only in the old session's scratch
  space, make a new one by editing the fixture's L line).
- Bigger features were done as parallel forks in git worktrees and
  merged; the same works locally with worktrees or branches.

## Open UI points

- Owner's earlier remark: opening device list or configuration is per
  tile; a global switch could come later.
- The help texts follow what the original MAX! manual says each
  setting does, from memory of it, not from the document; check them
  against the manual, above all valve offset and the open-window
  duration, and adjust the English and German entries in i18n.js.
- Translations: French, Italian, Dutch and Polish have empty tables in
  i18n.js (roadmap item 7). German uses "ß"; a de-CH variant would only
  need the few words that have one. Error texts from the server reach
  the UI in English.
- The Copy tab copies what is stored for the room, not unsent edits of
  the other tabs, and says so; if the owner expects the edits, it can
  take c.prog.days and c.edit instead.
- Found in passing, not fixed: the "Forget token" handler in app.js
  calls `panel.classList` on an undefined `panel`, so the click throws
  after the token is cleared (refresh and reconnect do not run).
- Playwright set-up on the Mint box without node or ensurepip:
  `python3 -m venv --without-pip venv`, get-pip.py from
  bootstrap.pypa.io run with the venv's python, then `pip install
  playwright` and `python -m playwright install chromium`. The page's
  CSP forbids eval, so `page.wait_for_function` with a string fails;
  poll from Python instead.

## Next steps in order

1. The write test (docs/handover-write-test.md) on the JEQ Cube.
2. The owner's review of the UI feedback round (branch
   claude/ui-feedback-9b6fa2), and the open UI points above.
3. Pairing new devices (roadmap item 4).
4. Firmware dump when the adapter is in (docs/firmware-extraction.md).
5. Users and a database (roadmap item 6).
