# Changelog

All notable changes to Massimo are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions
below 1.0.0 are pre-releases, tagged `vX.Y.Z-alpha.N` by
`make release-alpha` (`README.md`, "Versions"). A new feature starts
the next minor, a fix counts the suffix up.

## [Unreleased]

### Added

- `massimo serve -tls files -tls-cert … -tls-key …`: TLS from certificate
  files, re-read when they change, with HSTS for a year (`-hsts <days>`);
  `massimo install -tls-cert … -tls-key …` installs them (README.md).

- CI runs the guidelines' checks on every push and pull request
  (generated documents, languages, revision records, headings, line
  breaks, the changelog against its tags, commit messages on a PR).

- `make ui-screenshots` takes the web UI's screenshot set; CI's
  Screenshots job runs the UI smoke test and uploads the set.

### Changed

- The colour-scheme editor takes typed or pasted hex values next to each
  colour, and its contrast badges stand before the colours, which now
  line up (docs/ui.md, "Appearance").

- The web UI loads the texts of the chosen language only
  (`ui/i18n/<lang>.json`), and the browser revalidates the UI's files
  by their ETag instead of downloading them again.

- The web UI's settings and room configuration share one row layout:
  the label at the left and the control at the right, a text field or
  a long choice on a line of its own (docs/screenshots).

- The web UI asks in the page, translated, before it removes a Cube or
  installs an update, instead of in a browser dialog; Settings → Cubes
  lists a found Cube with its firmware and age, as the header does.

- The release workflow reads the update channel from the tag's name
  (`v0.7.0-alpha.1` is on `alpha`) before the tag message's `channel:`
  line, so a message rewritten with the changelog keeps its channel.

## [0.7.0-alpha.1] - 2026-11-06

### Added

- The store: `massimo serve` keeps the Cubes and its settings in
  `massimo.db` (SQLite) in the data directory (`-data`,
  `MASSIMO_DATA`), created at the first start, migrated with a
  `min_reader` rule that keeps a rollback of the binary safe, seeded
  from the settings file, with the Cubes found by discovery surviving
  a restart and a node identity (`node.id`, an Ed25519 key).

- `POST`, `PUT` and `DELETE /api/v1/cubes`, `GET` and
  `PUT /api/v1/settings`, and Settings → Cubes in the web UI: add,
  change and remove Cubes and the server's settings while it runs;
  a changed address reconnects, a changed proxy address restarts that
  listener, a hidden device applies at once.

- `massimo store backup|migrate|info` for the store, and
  `massimo config` says which values come from the store.

- `massimo netconfig` reads and writes a Cube's network configuration
  over UDP (static address or DHCP), with a broadcast recovery for a
  Cube on a wrong address; the protocol is confirmed on hardware.

- `massimo discover` shows whether a Cube takes its address by DHCP.

- `-direct` and `-no-direct` on every command: skip the configured
  proxy, or refuse the fallback to the Cube when the proxy does not
  answer; the fallback warns that it takes the Cube's only connection.

- Updates from the web UI on Windows: the service starts the update
  helper itself as a detached process; the CI Windows job exercises
  the API path and a rollback.

- `massimo legacy` shows the original software's Cube search lines.

- The designs of the store (`docs/store.md`), of what comes after it
  (`docs/server.md`) and of the Windows update path.

### Changed

- The first device list of a connection carries the greeting's time
  as `observed_since`.

- `serve -addr` runs without a store; the Cube and settings routes
  answer 503 then.

- The stale threshold is a server setting (`stale_after`) rather than
  a constant.

- A discovery answer no longer rewrites a configured Cube's row; only
  the discovery columns change, and only when they differ.

- `.gitattributes` keeps the embedded unit files, plists and scripts
  LF on every checkout; the protocol fixture keeps its CRLF.

- `update` removes `massimo.exe.failed` once a newer binary is in
  place, and a rollback tolerates a missing target.

- `massimo config` knows the update variables as names.

## [0.6.0-alpha.1] - 2026-11-05

### Added

- Colour schemes in the web UI: light, dark or system mode, three
  built-in schemes, a custom scheme with a contrast check, export and
  import as `massimo-scheme/1`.

- A restore API: a plan from an uploaded backup, the send checked
  against the plan's hash, progress on the event stream, guard rails
  for the duty cycle and the Cube's serial.

- An "Installing" section in the README per operating system, with
  the SmartScreen dialog as the Windows test saw it.

- The report of the Windows service tested on a real PC.

### Changed

- Both buttons of Settings → Software are greyed out while an update
  runs, also after a reload of the page.

- `.env.*` copies and `*.log` files are ignored by git.

## [0.5.0-alpha.1] - 2026-11-05

### Added

- State events say what changed and who changed it (the server,
  another program, someone at the device); the log view shows it as
  "window opened", "set 18.0 → 21.0", "reports again".

- `started_at` on `/healthz`; the web UI shows it in expert mode.

### Changed

- The web UI's Cube search uses the server's discovery API.

- Blue is the accent colour, red only for alerts.

- Device type and radio address show in expert mode only.

## [0.4.0-alpha.1] - 2026-11-05

### Added

- Discovery from `serve`: at the start, every 30 minutes and on
  `POST /api/v1/cubes/discover`; unconfigured Cubes in
  `GET /api/v1/cubes`, `cube_found` events, Cubes configured by serial
  follow their discovered address; "Search for new Cubes…" in the UI.

- Pairing: `POST`, `GET` and `DELETE …/pairing`, `paired` events,
  adding a paired device to a room, pairing through the proxy,
  inclusion mode in the fake Cube.

- After a write the server re-reads the device's configuration and
  reconnects once for a fresh budget when radiograms were pending;
  accepted commands and wakeups in the log.

- macOS LaunchDaemons for `install` and `update`, tested on a Mac and
  in CI; `MASSIMO_GITHUB_TOKEN` for the release check.

- Ages and stale warnings in the UI, the "Open" window badge on the
  tile, the event stream opened with tickets.

### Fixed

- The window contact's move and its link error, confirmed in the
  write test's step 7.

## [0.3.0-alpha.1] - 2026-11-05

### Added

- Timestamps per value: `received`, `last_seen`, `stale` and
  `observed_since` in the API; ages and greyed values in the UI.

- A lockout after ten wrong tokens a minute, and single-use tickets
  for the event stream (`?token=` on the stream deprecated).

- French, Italian, Dutch and Polish in the web UI, machine-translated
  and marked; `docs/ui.md`, using the web UI.

- `serve -refresh`: every Cube's snapshot re-read in full every six
  hours.

- `make ui-test`, a Playwright smoke test of the web UI.

- The Windows service: `massimo install` and `massimo update` on
  Windows, tested on a CI runner; govulncheck in CI.

- `internal/view`, `internal/cubeio` and `internal/env`: one read
  model, one place for writes that keep the snapshot in step, one
  reader of the command line's configuration.

### Changed

- "Hidden devices" is the one term; `-listen` names the listener on
  every command (the old flags are kept); the proxy refuses to delete
  hidden devices.

- `connected` means the greeting is in, and goes false on a loss.

### Fixed

- A reconnect right after `Ready` waits for the new greeting.

- A single Cube without a serial gets its proxy again.

## [0.2.0-alpha.1] - 2026-11-05

### Changed

- `install -listen` moves the API and web UI and is kept across
  installs.

- Releases: a new feature starts the next minor, a fix counts the
  suffix up.

## [0.1.0-alpha.12] - 2026-11-05

### Changed

- Development happens on `main`; `make release-alpha` promotes it to
  the branch `alpha`, the update channel of the alpha releases.

- Pre-releases are numbered past `alpha.9`.

## [0.1.0-alpha.11] - 2026-11-05

### Fixed

- `update` runs its smoke test in an empty directory.

## [0.1.0-alpha.10] - 2026-11-05

### Added

- `massimo install` sets `serve` up as a systemd service, with a root
  update helper; `massimo update` installs signed releases, with a
  health check and a rollback; an Update button in Settings.

- CI tests every push and publishes signed releases from `v*` tags;
  macOS and `linux-armv6` targets; the update channel; `make release`.

- Documentation for preparing a Raspberry Pi 1, per-Cube proxy
  addresses, moving networks, and measured resource use.

## [0.1.0-alpha.9] - 2026-11-05

### Changed

- MANU needs a known setpoint; pending radiograms count against the
  Cube's observed capacity.

- One state constructor and one metadata mirror for the API, `status`
  and backups.

- A constant-time token check and security headers on the web UI; a
  test that the OpenAPI description covers every route.

## [0.1.0-alpha.8] - 2026-11-05

### Added

- The design proposal, the background evaluation, the code review
  with the coding standard, and the session handover.

## [0.1.0-alpha.7] - 2026-11-04

### Changed

- The web UI: coloured badges, IS and SET icons, the info and cog
  buttons on the tile, icon pages in the header, accessibility.

- `massimo config` redacts the API token like the keys and IVs.

## [0.1.0-alpha.6] - 2026-11-04

### Added

- The room configuration area and the log view in the web UI; Cube
  names from `MASSIMO_CUBE_<SN>_NAME`; `GET …/log` with the last
  events.

## [0.1.0-alpha.5] - 2026-11-04

### Added

- Room and device configuration in the API: names, moves, device and
  room configuration, week programmes.

- IS and SET tiles with a chevron stepper, HOLIDAY, a refresh button.

## [0.1.0-alpha.4] - 2026-11-04

### Added

- The first web UI at `/`: room tiles with a stepper, the House tile,
  the device list, expert mode, settings with polling and the clock
  zone, a reload on a new server version.

- `serve -log` appends the log to a file.

- Documentation of the firmware extraction over JTAG.

## [0.1.0-alpha.3] - 2026-11-04

### Added

- Room 255 as the ghost room, `-show-ghosts` and `-no-ignore`,
  `DELETE …/rooms/{id}`, hidden parking rooms; the proxy hides ignored
  devices from clients and puts them back into metadata writes.

- `status` and the API count the radiograms the Cube still waits for.

- `serve -sn` serves one Cube; the proxy settings come from `.env`;
  the commands try the proxy first; `make tag`.

## [0.1.0-alpha.2] - 2026-11-04

### Added

- The server: one connection and one command queue per Cube, the JSON
  API, server-sent events, and the legacy launcher with the LAN lines
  derived from the original software's gateway calls.

- The LAN proxy: `serve` answers as a Cube on port 62910 with UDP
  discovery, so other programs and the original software connect to
  the server.

- The ignore list, the key lock (`PUT …/devices/{rf}/lock`), the
  `status` command with reported and ready flags.

- Backups record each device's live state; `restore -state` re-applies
  mode and setpoint; week programmes by weekday with HH:MM times; wall
  thermostats' display mode, offset, window-open and boost.

- Cube discovery, per-Cube variables (`MASSIMO_CUBE_<SN>_KEY`), the
  IV exchange over the LAN, `massimo config`, and the portal's probe
  mode.

- Documentation of the portal protocol, the PC software's portal
  client and the Cube's UDP protocol.

## [0.1.0-alpha.1] - 2026-11-03

### Added

- The Cube LAN protocol package, `massimo decode`, the portal crypto
  package with `portal-crypto`, the fake portal server and a LAN
  client command.

- Secrets from a `.env` file, `--version`, a Makefile with cross
  builds.

[Unreleased]: https://github.com/metaworx/massimo/compare/v0.7.0-alpha.1...HEAD
[0.7.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.6.0-alpha.1...v0.7.0-alpha.1
[0.6.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.5.0-alpha.1...v0.6.0-alpha.1
[0.5.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.4.0-alpha.1...v0.5.0-alpha.1
[0.4.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.3.0-alpha.1...v0.4.0-alpha.1
[0.3.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.2.0-alpha.1...v0.3.0-alpha.1
[0.2.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.12...v0.2.0-alpha.1
[0.1.0-alpha.12]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.11...v0.1.0-alpha.12
[0.1.0-alpha.11]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.10...v0.1.0-alpha.11
[0.1.0-alpha.10]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.9...v0.1.0-alpha.10
[0.1.0-alpha.9]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.8...v0.1.0-alpha.9
[0.1.0-alpha.8]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.7...v0.1.0-alpha.8
[0.1.0-alpha.7]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.6...v0.1.0-alpha.7
[0.1.0-alpha.6]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.5...v0.1.0-alpha.6
[0.1.0-alpha.5]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.4...v0.1.0-alpha.5
[0.1.0-alpha.4]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.3...v0.1.0-alpha.4
[0.1.0-alpha.3]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.2...v0.1.0-alpha.3
[0.1.0-alpha.2]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.1...v0.1.0-alpha.2
[0.1.0-alpha.1]: https://github.com/metaworx/massimo/releases/tag/v0.1.0-alpha.1
