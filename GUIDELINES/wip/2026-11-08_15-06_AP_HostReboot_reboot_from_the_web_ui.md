# AP HostReboot v1.0: Reboot from the web UI

## TL;DR

The owner reboots the computer `serve` runs on from System settings →
Maintenance, where the owner's switch allows it and a root helper is
there to do it; the capability `reboot` is the owner's alone.

- switch: `MASSIMO_REBOOT=on` in the settings file, or `serve -allow-reboot`;
  a reload of the file applies it at once
- who reboots: Linux under systemd → root path unit `massimo-reboot.path`;
  macOS → LaunchDaemon `ch.metaworx.massimo-reboot`;
  Windows service → the service itself, `shutdown.exe`
- helper missing where one is needed → the API refuses and says why
- none on Homebrew, in a terminal, in Docker, on Linux without systemd
- delay: 60 s by default, 0 to 3600 in the page; cancel within it
- `massimo install -reboot-helper` / `-no-reboot-helper`; the Debian
  package ships the units disabled
- no sudoers (`NoNewPrivileges`), no scheduled task (LocalSystem)

### Blocks

1. The reboot routes: `GET`/`POST`/`DELETE /api/v1/reboot`, the capability `reboot`
2. `serve`: the switch, who reboots on each system, the request and its answer
3. The helpers and `massimo install -reboot-helper`, the Debian package
4. Maintenance: the reboot row, its delay, the cancel, the way back

## Discussion

The owner asked on 2026-11-08 (EVAL) for a way to reboot the system
"when the env file allows and the user has the capability `reboot`":
what each system needs (sudoers from the install, or a job on Windows
that can be triggered) and which systems would support it.
The evaluation answered per system; the owner decided:

- **D1-A** (as recommended): the helper is opt-in, `massimo install`
  with a flag; the Debian package ships its units disabled.
  The consent that counts on Linux and macOS stays root's.
- **D2**, the owner's form: the switch is `MASSIMO_REBOOT` in the settings
  file or `-allow-reboot` on the command line.
  The API refuses where the switch is on but the system needs a helper
  and none is installed.
- **D3**, the owner's form: the reboot follows 60 s after the request by
  default, and the page can set the delay down to 0.
- **D4-A**: the owner alone. Restarting this service is not rebooting a
  computer that may run other services; `reboot` is seeded for no role,
  and owner has every capability by code.

Then `PLAN++`: the plan committed to `main` first, the work on a branch,
the plan retired on that branch once the work is done, then the pull
request. The merge is the owner's.

## Analysis

### Why no sudoers, and no job on Windows

- **Linux.** `massimo.service` runs as `massimo` with `NoNewPrivileges=yes`:
  `sudo` cannot raise privileges from it, whatever sudoers says, and
  loosening the sandbox for every request the server takes is the worse
  trade. polkit would allow `org.freedesktop.login1.reboot` over D-Bus,
  but Raspberry Pi OS Lite and a minimal Debian have no polkit, and its
  rules changed form between Debian 11 (`.pkla`) and 12 (JavaScript).
  The update helper's pattern needs nothing but systemd: a root path unit
  watches a request file the server writes, and starts a root oneshot.
- **macOS.** The `massimo install` layout already has a root LaunchDaemon
  for updates, started by `WatchPaths`; the reboot gets its own.
- **Windows.** The service runs as LocalSystem, which holds the shutdown
  privilege, and `shutdown.exe` enables it; a scheduled task would only be
  needed for a less privileged service account, which `install` never makes.

### Which systems

| Where `serve` runs | Who reboots | Supported |
|---|---|---|
| Linux, systemd's service (`massimo install`, the Debian package, Raspberry Pi OS) | `massimo-reboot.path` → `massimo-reboot.service`, root | yes, with the helper |
| macOS, `massimo install`'s LaunchDaemon | `ch.metaworx.massimo-reboot`, root | yes, with the helper |
| Windows service | the service, `shutdown.exe /r` | yes |
| Homebrew's service (`brew services`, as the user) | nobody | no |
| a terminal, Docker (`-restart exit`), Linux without systemd | nobody | no |

### The gates, in the order the API reports them

1. **The system**: a service the system runs (`restartBy`'s detection,
   `cmd/massimo/restart.go`), else "only the installed service reboots
   the computer"; Homebrew's job by its label, with its own reason.
2. **The switch**: `MASSIMO_REBOOT` (`on`/`off`, also `true`/`false`,
   `1`/`0`, `yes`/`no`; default off) or `-allow-reboot`.
   A flag given wins over the file, as for every flag (`flagVars`);
   otherwise a reload applies the file's value at once, both ways, and
   switching it off cancels a reboot that waits.
   On Windows only SYSTEM and the Administrators write the settings file,
   so the switch is a real gate there; on Linux and macOS the file belongs
   to the service user, which is why the helper is the second one.
3. **The helper** (Linux, macOS), looked at for every request: on Linux
   the path unit enabled, `/etc/systemd/system/paths.target.wants/massimo-reboot.path`
   (both `systemctl enable` and the package's `deb-systemd-helper` make it);
   on macOS `/Library/LaunchDaemons/ch.metaworx.massimo-reboot.plist`.
   Both live in directories only root writes. Missing: "no reboot helper
   installed: sudo massimo install -reboot-helper".
4. **The moment**: no update running (the helper replaces the binary), no
   restart under way, no reboot already waiting.
5. **An open server** (no users, no token) takes the request only from the
   box itself, as the restart does (`fromTheBox`).
6. **The capability** `reboot`, through the route's registration.

### The request

- **The delay lives in the server**: a timer; at its end the server asks
  (`RebootOptions.Reboot`). A server that stops within the delay reboots
  nothing, and `DELETE /api/v1/reboot` cancels; seconds work everywhere,
  which `shutdown -r +m` (minutes) would not give.
- **Linux and macOS**: `serve` writes `/var/lib/massimo/reboot-request`
  (temporary file and rename) and waits up to 30 s for the helper to take
  it. Still there after that, it removes it and reports "the reboot helper
  did not take the request": a request never lingers to reboot the box at
  some later start. `serve` also removes one it finds at its start.
- **The helper never reads the request**: it removes the file first and
  reboots only if that worked (`ExecStartPre=/bin/rm -f`, then
  `ExecStart=/bin/systemctl reboot`; on macOS a `/bin/sh -c` that tests,
  removes, then `exec /sbin/shutdown -r now`), so that a reboot can never
  start the next one.
- **Windows**: `%SystemRoot%\System32\shutdown.exe /r /t 0 /d p:4:1 /c "…"`
  (a planned reason in the event log), without `/f`, so a user's open
  program is asked rather than killed.
- **The stop**: the system stops the service as on every shutdown
  (SIGTERM; on Windows the service's Shutdown control, already accepted),
  so the store is closed as at every stop.

### The API

- `GET /api/v1/reboot`: `available`, `by` (`systemd`, `launchd`,
  `windows`), `blocked`, `at` while a reboot waits, `error` after one
  that failed.
- `POST /api/v1/reboot` `{"delay": 60}`, seconds 0 to 3600, 60 when left
  out: 202 `{"ok": true, "by": …, "at": …}`; 400 for another delay, 403
  for an open server from elsewhere, 409 with the reason otherwise.
  Audited `server.reboot`, a command (an `Idempotency-Key` replays it).
- `DELETE /api/v1/reboot`: 200 while the reboot waits, 409 once it has
  begun or when none waits. Audited `server.reboot.cancel`.
- The restart says "a reboot is waiting" while one waits.

### The page

Maintenance gets a row under the restart, for a holder of `reboot`:
what a reboot does or why there is none, the delay as a `.field`
(seconds, 60), **Reboot the computer** asking in place; while it waits,
when, and **Cancel the reboot**; then the page follows `/healthz` for up
to five minutes after the reboot's time, as the restart does for two.
The card shows for `restart`, `settings` or `reboot`.

## Implementation Plan

On a branch `claude/host-reboot` from `main` after this plan's commit.
Every block: its CHANGELOG bullet under `[Unreleased]`, its documents
(`docs/api-v1.md` and the OpenAPI file for the routes, `README.md` for the
command line, `packaging/README.md` for the installs, `docs/ui.md` for the
page, `docs/store.md` for the capability), and its texts in all six
`ui/i18n/*.json` (FR, IT, NL, PL machine-translated and marked).

**Verification**, after every block:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet` on linux, windows and darwin, `make cross`;
Block 4 also `make ui-test` at 1100 and 390 px (`UI_TEST_PYTHON` the main
checkout's venv, free `UI_TEST_*_PORT`). Nothing reboots a real computer:
the tests fake the request's writer and `shutdown.exe`.

### Block 1: The reboot routes

- `internal/server/reboot_api.go`: `RebootOptions` (`By`, `Blocked`,
  `Allowed`, `Helper func() string`, `Reboot func() error`), the waiting
  reboot's state and timer, `SetRebootAllowed`, the three handlers.
- `server.go`: the routes with `reboot`; `restart_api.go`: the restart
  blocked while a reboot waits; `audit.go`: `server.reboot`,
  `server.reboot.cancel`; `auth.go`: `reboot` after `restart`.
- `login.js`: `reboot` in `CAPABILITIES`; `cap.reboot` in the six i18n files.
- Documents: `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/store.md`
  section 6 (the capability, seeded for no role).
- Tests: every gate's reason in order, the delay's bounds, the timer
  calling `Reboot` once, the cancel, the open server from the box only,
  the audit rows, `TestOpenAPICoversRoutes`.

### Block 2: `serve`, the switch and who reboots

- `cmd/massimo/reboot.go`: `rebootBy` (the system, Homebrew, the helper's
  marker below a root for tests), the request's writer with its 30 s wait,
  the stale request's removal, Windows' `shutdown.exe` behind a variable.
- `serve.go`: `-allow-reboot`, default `MASSIMO_REBOOT`, an invalid value
  refused at the start; `reload.go`: the variable applied at once, unless
  the flag was given, and never "pending"; `internal/env`: `Known`.
- Documents: `README.md` (serve), `packaging/env.template`.
- Tests: `rebootBy`'s table, the writer (taken, not taken, removed),
  the reload switching it off and on, the Windows command's arguments.

### Block 3: The helpers and `install`

- `packaging/systemd/massimo-reboot.path` and `.service`,
  `packaging/launchd/ch.metaworx.massimo-reboot.plist`, embedded.
- `internal/install`: `-reboot-helper` installs (a hand install writes the
  units; a packaged box enables the package's; macOS writes and loads the
  plist; Windows says it needs none), removes a stale request first, and
  enables the path unit; `-no-reboot-helper` takes it out; neither keeps
  what is there; `-remove` takes it too. `cmd/massimo/install.go`: the flags.
- The Debian package: the units in `/usr/lib/systemd/system/`, not enabled;
  `postinst`, `prerm`, `postrm` as debhelper's for a unit off by default;
  `packaging/deb/test.sh`: shipped, not enabled, `install -reboot-helper
  -dry-run` would enable it, purged.
- Documents: `packaging/README.md` per system, `README.md` (install).
- Tests: `internal/install` with `Root` for Linux, packaged and macOS;
  `systemd-analyze verify` of the units; `make packages deb-test`.

### Block 4: Maintenance

- `partials/maintenance.html`: the reboot row; `update.js`: `loadReboot`,
  `renderReboot`, the question in place with the delay, the cancel, the
  following of `/healthz`; `login.js`: `hasSystem` with `reboot`;
  `style.css`: the card for `restart`, `settings` or `reboot`
  (`needs-maintenance`).
- The six i18n files; `docs/ui.md`, Maintenance.
- `tools/ui-test/test_reboot.py`, the routes stubbed: not offered with the
  reason, the question and its delay, the request's body, the wait with
  its cancel, the way back; the Maintenance screenshots retaken.

## Proposed commit message

- `chore(aps): +HostReboot v1.0`
- `feat(api): The owner reboots the computer from the API, after a delay`
- `feat(serve): -allow-reboot or MASSIMO_REBOOT lets the server reboot its computer`
- `feat(install): -reboot-helper installs the root helper that reboots the computer`
- `feat(ui): Reboot the computer from System settings, Maintenance`
- `chore(aps): -HostReboot`

## Change History

- v1.0 (2026-11-08): the first revision, from the owner's EVAL of a reboot
  from the web UI and the decisions D1-A, D2, D3 and D4-A, `PLAN++`.
