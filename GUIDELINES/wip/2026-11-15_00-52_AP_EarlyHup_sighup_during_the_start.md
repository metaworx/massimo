# AP EarlyHup v1.0: SIGHUP during the start

## TL;DR

A SIGHUP that comes while serve starts, before it handles SIGHUP as a
reload, ends serve; under systemd that counts as a clean stop, so the
service stays down. serve now catches SIGHUP as soon as it knows it
reloads on it, and logs one that comes during the start at Warn and
drops it.

- SIGHUP in reload mode → caught right after the mode is known (D1-A),
  not after the store is open
- during the start → Warn, dropped: the settings were just read and the
  log file just opened (D2-A)
- once serve runs → a reload as now: the settings, the log's rotation,
  the store's checkpoint
- no new command (D3-A); the README names the ways to ask for a rotation

### Blocks

1. EarlyHup: the catch from the start, its Warn, a unit test and one of
   a built serve; README, CHANGELOG fragment

## Discussion

Found by the fix of `TestServeLogFile` (#268, 2026-11-14), which sent
its SIGHUP before serve handled it. The owner's EVAL: a SIGHUP right
after the start has nothing to do, as the settings were just read and
the log rotated where its format changed, so it is logged at Warn rather
than handled later. `PLAN B`, then `PLAN++` with:

- D1-A: catch SIGHUP right after `sighupAction` has decided the mode, not
  at the top of serve; the few milliseconds before (the flags, the
  settings file, the log file's opening) stay as they are, since `auto`
  must read whether SIGHUP was ignored before catching it;
- D2-A: Warn and drop, no reload or rotation later;
- D3-A: no `massimo logrotate` or `massimo reload`: serve rotates at its
  size, and "Reload now", `systemctl reload massimo` and
  `POST /api/v1/reload` already ask for a rotation; a command could not
  rotate the file itself, which serve holds open.

The Warn is the owner's choice: the contract keeps Warn for what a Cube
refused, but an operator should see that a reload did nothing.

## Analysis

**Now** (`cmd/massimo/serve.go`): `sighupAction` decides the mode early
(after the logger and the log file); with `stop`, SIGHUP joins the
signals of `signal.NotifyContext` soon after, so an early one stops
serve as a later one would. With `reload`, `signal.Notify` for SIGHUP
comes only after `serve.Open` (the store, its migrations, the keys) and
the settings reloader: about 40 ms on CI, seconds with a migration.
Before, SIGHUP has Go's default action and ends serve.

**Who sends it.** The systemd unit's `ExecReload=/bin/kill -HUP
$MAINPID`, for `systemctl reload massimo`; nothing else by design (no
logrotate file since 0.14.0~alpha.2, none for launchd, none on
Windows). The unit is `Type=simple`, active at once, so systemd allows a
reload during the start; `systemd.service(5)` counts an exit by SIGHUP
as clean, so `Restart=on-failure` does not start serve again.

**The change.** A small handler in `sighup.go`: `signal.Notify` on a
channel and a goroutine, started right after the mode is known where it
is `reload`; until serve hands it the reload, a SIGHUP is logged at Warn,
"SIGHUP during the start, ignored", with the reason; afterwards it runs
the reload as the goroutine does now (the settings, the rotation, the
checkpoint), not once the server stops. The handler stops catching when
serve returns.

## Implementation Plan

Every commit cites `AP EarlyHup v1.0, Block 1`.

### Block 1: EarlyHup

- `cmd/massimo/sighup.go`: `hupReload`, its `catch` (Notify, goroutine,
  a stop function), `ready(run)` and `handle`: before `ready`, the Warn.
- `cmd/massimo/serve.go`: the catch right after the mode is known, for
  `reload`; the reload goroutine replaced by `ready`, which skips a
  SIGHUP once the context is done.
- Tests: `handle` before and after `ready` (the Warn line, the run);
  a built serve with `-sighup reload -log`, a SIGHUP once the store is
  created, before the server started: serve keeps running and logs the
  Warn or, where the SIGHUP came late, the rotation for it; checked to
  fail on main's code.
- `README.md`, the `-sighup` paragraph: a SIGHUP during the start is
  logged and ignored; the rotation paragraph names "Reload now" and
  `systemctl reload massimo`.
- CHANGELOG fragment, `Fixed`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt`,
`go vet` for linux, windows and darwin, `make cross`; the new built-serve
test repeated, and on main's code.

## Proposed commit messages

- `chore(aps): +EarlyHup v1.0`, this plan, on main.
- Block 1, `fix(serve): A SIGHUP during the start is logged, not the end of serve`.

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN++` after `PLAN B`.
