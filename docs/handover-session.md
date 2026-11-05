# Handover from the session of 2026-11-04

For the next session, local or cloud. Develop on `main` (pull requests
go there); `alpha` (called `phase-0` until 2026-11-05) is the release
branch and update channel of the alpha releases, promoted from `main`
with `make release-alpha` after each feature; tags up to
`v0.1.0-alpha.N`. Read README.md, then docs/roadmap.md, then this file.

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

## Open UI feedback (not yet implemented), in the owner's words

Week programme editor:
- Selecting a day in the top row also selects every other day with the
  same programme.
- Remove "All days" there; make "Only {day}" a switch: on, edits apply
  to the current day only; off, edits apply to all selected days with
  the same programme at once. The first change that sets the current
  day apart deselects the others, which are then no longer updated.
- "Copy {day} to …" becomes a small section with all days again, each
  toggled by clicking, an "All days" button, the days of the current
  selection greyed out, and a Copy button that applies the current day
  to them; the group then becomes one active group when the switch is
  off.
- "Reset day" and "Reset all" buttons left of Cancel/Send, which move
  to the right.

Copy tab:
- Two sections: Rooms (a list with a switch per room, "All rooms",
  "None") and Settings (Temperatures, Week programme, Radiator
  settings, each a switch).

Everywhere:
- Help icons next to settings that explain what each setting does.
- Internationalisation: all UI texts through a translation table, English
  as the base; translations for the languages the original software
  had (DE, EN, FR, IT, NL, PL) go on the roadmap.
- Owner's earlier remark: opening device list or configuration is per
  tile; a global switch could come later.

## Next steps in order

1. The write test (docs/handover-write-test.md) on the JEQ Cube.
2. The UI feedback above, week programme first.
3. Pairing new devices (roadmap item 4).
4. Firmware dump when the adapter is in (docs/firmware-extraction.md).
5. Users and a database (roadmap item 6).
