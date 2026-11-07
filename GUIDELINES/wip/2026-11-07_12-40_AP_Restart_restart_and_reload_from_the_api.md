# AP Restart v1.0: Restart and reload from the API

## TL;DR

Holders of a new capability, `restart` (owner and manager), restart the
server from the API and from Settings → Software, through whatever service
manager runs it.
The server also re-reads its settings file while it runs,
so a new `MASSIMO_API_TOKEN` applies without a restart.

- restart → a clean stop, as SIGTERM gives → exit 75 → systemd, launchd,
  Homebrew start it again; Windows → a detached `massimo restart` stops and
  starts the service through the service manager
- no service manager → not offered, with the reason;
  `-restart auto|exit|off`, `MASSIMO_RESTART` for Docker or a custom unit
- open server → a restart only from the box itself (D3-B), once AP OpenMode
  Block 1 refuses cross-origin writes
- settings file → watched (D4), SIGHUP on Unix (`-sighup auto|reload|stop`,
  `MASSIMO_SIGHUP`); the token applies live, the rest "at the next restart"
- event streams → checked again at each keep-alive, closed once the caller
  would not be let in
- sign-in dialog → its API-key part opens by itself when the page saw the
  server open

### Blocks

Pull request 1, the restart:

1. Capability `restart`: the server's and the UI's lists, migration 0006
   for manager, its label in six languages, the documents
2. The restart in `serve`: service-manager detection, `-restart`, exit 75,
   `massimo restart` and the Windows helper, the systemd unit's exit status
3. `GET` and `POST /api/v1/restart`: the reasons, D3-B, `202`, tests,
   OpenAPI, `docs/api-v1.md`
4. The UI: Settings → Software restarts the server, follows `/healthz`
   until it is back; Playwright at 1100 and 390 px

Pull request 2, the reload:

5. The settings-file reload: the watch (D4), SIGHUP and `-sighup` (D8),
   the token live, the "applies at the next restart" list
6. Event streams checked again at each keep-alive (D5)
7. The UI: the "applies at the next restart" list beside the button, the
   sign-in dialog's key part (D6); Playwright

## Discussion

Settled at the TL;DR gate and the two EVAL answers that followed,
the owner's `PLAN++` taking the recommended options except where he named one.

### D1. How the server restarts itself: A

- **A (chosen).** A clean stop, then on Unix exit 75, which systemd
  (`Restart=on-failure`), launchd (`KeepAlive`, `SuccessfulExit` false) and
  Homebrew (`keep_alive`) answer with a start; the unit gains
  `RestartForceExitStatus=75` and `SuccessExitStatus=75`, so that the journal
  calls it a restart and not a failure (an older unit restarts on 75 anyway).
  On Windows the server starts `massimo restart` as a detached process, as it
  starts the update helper, which stops the service through the service
  manager, so that serve stops as on every stop, and starts it again.
- B. Exit everywhere, Windows by its recovery actions: 10 s and an Event-log
  error per restart.
- C. The root update helper runs `systemctl restart`: absent with Homebrew.

### D2. A run without a service manager: A, with a switch

- **A (chosen).** Not offered, and the API says why.
  `serve -restart auto|exit|off`, default `MASSIMO_RESTART`, else `auto`:
  `auto` detects the Windows service, systemd (`INVOCATION_ID`) and launchd
  (`XPC_SERVICE_NAME`); `exit` exits 75 for whatever runs the server, such as
  Docker's `--restart`; `off` never offers it.
- B. Re-exec itself: the environment would have to be rebuilt without the
  file's values, and on Windows the PID and Ctrl-C change.

### D3. An open server: B

- A. Refused, as updates are.
- **B (chosen).** Allowed from the box itself only: the TCP peer is loopback
  and the request carries no `X-Forwarded-For`, `X-Real-IP` or `Forwarded`,
  so that a reverse proxy on the box does not pass the LAN through.
  It rests on AP OpenMode Block 1: without it, any website the owner's
  browser opens on the box could send the `POST`.
  This plan's pull request 1 is therefore stacked on that plan's.

### D4. The settings file without a restart: open

The owner asked whether the kernel's file notification could do it where
there is one. To be settled at Block 5's gate, in pull request 2.

- **A (recommended).** `stat` every 5 s, applied once size and time held
  for one more check, a parse error or a missing file keeping what runs:
  no dependency, every file system (a network share, WSL's `/mnt`),
  about 17,000 calls a day of microseconds each; SIGHUP on Unix for "now".
- B. The kernel's notification (inotify, kqueue, `ReadDirectoryChangesW`)
  through `github.com/fsnotify/fsnotify`, the `stat` kept where no event
  comes: immediate, but a new dependency, a watch on the directory (editors
  save by renaming a new file over the old), the wait for a settled file
  still needed, and two paths to test.
- C. The same through `golang.org/x/sys` directly, already a dependency:
  no new module, three platform implementations of our own.

### D5. Event streams: A, revised

- **A (chosen).** At each 30 s keep-alive, the stream resolves its caller
  again and closes when that caller would not be let in now: a change from
  open to users, a revoked key, a disabled user or role, an expired guest.
  The page reconnects through a ticket, which asks for a sign-in.
- B. Hooks on each of those events. C. Left as they are.

### D6. The sign-in dialog: A, revised

- **A (chosen).** No new text: the dialog's "Or an API key" part opens by
  itself when the page had seen the server open, since the owner made from
  `MASSIMO_API_TOKEN` has no password.
- B. As it is.

### D7. Pull requests: A, two

Blocks 1 to 4, then 5 to 7.

### D8. The reload signal: A, with a switch

- **A (chosen).** SIGHUP re-reads the settings file at once and, from
  roadmap item 9 on, reopens `-log`. `serve -sighup auto|reload|stop`,
  default `MASSIMO_SIGHUP`, else `auto`: `auto` reloads under a service
  manager and where SIGHUP was ignored at the start (`nohup`), and stops in a
  terminal, where SIGHUP means that the terminal closed.
  The unit gains `ExecReload=/bin/kill -HUP $MAINPID`.
- B. SIGHUP re-reads, SIGUSR1 reopens the log.
  C. SIGHUP rotates, SIGUSR1 re-reads. D. No signal.

Today an unhandled SIGHUP stops `serve`, and systemd counts that as a clean
exit, so `Restart=on-failure` does not start it again.

### D9. The cross-origin hole: A, a plan of its own first

AP OpenMode.

## Analysis

The analysis of the TL;DR gate is
[ANALYSIS Restart](2026-11-07_12-05_ANALYSIS_Restart_restart_and_settings_reload.md):
what happened on jaguar against the code, the platforms' restart table,
which settings can apply live, open mode, the capability.
What the gates added since:

- **The `401` on an open page, verified.** jaguar's v0.7.0-alpha.1 opened a
  token dialog that could not be closed without leaving every later request
  waiting; main asks again on every `401` since v0.8.0-alpha.1.
  Nothing to fix there but D6.
- **SIGHUP, D8.** SIGHUP is the convention for re-reading a daemon's
  configuration (PostgreSQL, sshd, syslog-ng, nginx), and what
  `systemctl reload` and `Type=notify-reload` send. Reopening a log is
  idempotent, and a reload the rotation triggers finds nothing new, since
  the watch applies a change anyway. Log rotation needs no signal yet:
  `packaging/logrotate` uses `copytruncate`, and on macOS launchd holds the
  log file.
- **The next migration is 0006**, not 0005 (`0005_backups_reasons.sql`).
- **The restart's audit.** The `POST` is a change and gets its audit row in
  the API wrapper before the handler returns; the stop waits for the handler
  (`http.Server.Shutdown`) and for the log's writer (`Server.Run`), and the
  store closes after both.

## Implementation Plan

### Block 1: Capability `restart`

1. `internal/server/auth.go`, `capabilities`: `restart` after `update`.
2. `internal/store/migrations/0006_restart_capability.sql`, `min_reader: 1`:
   `restart` for manager, as 0003 did for `preferences`; owner holds every
   capability by code. The store's tests that count migrations or a role's
   set follow.
3. `internal/server/ui/login.js`, `CAPABILITIES`; `cap.restart` in the six
   `ui/i18n/<lang>.json`, FR, IT, NL and PL marked.
4. `docs/api-v1.md` "Authentication", the table and the default roles;
   `docs/store.md` section 6; `docs/api-v1-openapi.yaml` where it lists the
   capabilities.
5. `CHANGELOG.md`: a bullet under `### Added`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`; the
capabilities test that compares the server's list with the documents.

### Block 2: The restart in `serve`

1. `cmd/massimo/restart.go`: `-restart auto|exit|off` (`MASSIMO_RESTART`),
   the detection, and what a restart does: on Unix, and with `exit`, a flag
   and the end of serve's context, then `runServe` returns `errRestart` and
   `main` exits 75; as a Windows service under `auto`, a detached
   `massimo restart` (the update helper's starter, generalised).
2. `massimo restart [-service <name>]`: restarts the installed service, the
   systemd unit, the LaunchDaemon or the Windows service, through the
   update command's `restartService`.
3. `server.Options.Restart`: who restarts the server (`By`), why it cannot
   (`Blocked`), and the function that does it.
4. `packaging/systemd/massimo.service`: `RestartForceExitStatus=75`,
   `SuccessExitStatus=75`; the deb's unit follows from it.
5. Tests: the detection by environment and switch; a serve run whose
   restart ends it with `errRestart` after the store closed; the exit code;
   the Windows helper's arguments (cross-compiled and run through interop,
   the real service untouched).
6. `README.md` (`serve -restart`, `massimo restart`), `packaging/README.md`,
   `docs/update-windows.md`; the CHANGELOG bullet.

**Verification:** as Block 1, with `make cross`, `go vet` for linux,
windows and darwin, and `-race` for `internal/server`.

### Block 3: `GET` and `POST /api/v1/restart`

1. `internal/server/restart_api.go`, both routes with the capability
   `restart`: `GET` says `{"available", "by", "blocked"}`; `POST` answers
   `202 {"ok": true, "by": …}` and restarts once the answer is out,
   `403` on an open server unless from the box itself (D3-B),
   `409` with the reason when it is not available or an update is running.
2. Tests on the fake Cube: each answer, the audit row, the restart done
   once, nothing sent to the Cube; users mode with manager, operator and
   no key; open mode from loopback, from the LAN and through a proxy.
3. `docs/api-v1-openapi.yaml` (`TestOpenAPICoversRoutes`), `docs/api-v1.md`,
   the route list in `server.go`; the CHANGELOG bullet.

**Verification:** as Block 2.

### Block 4: The UI

1. `ui/partials/software.html`, `ui/update.js` or a `restart.js`: the
   button for holders of `restart` while `GET` says available, else the
   reason; a confirmation in place of the buttons, as the update has; after
   the `POST`, `/healthz` every 2 s until `started_at` changes, then the data
   and the stream again and "restarted at …"; after two minutes without it,
   a message.
2. Its texts in the six languages.
3. `tools/ui-test/`: the button, the confirmation, the way back, a hidden
   button without the capability, at 1100 and 390 px.
4. `docs/ui.md`; the CHANGELOG bullet.

**Verification:** `make ui-test`, then as Block 2.

Blocks 5 to 7 are pull request 2; their steps are filled in at the gate
that opens it, once D4 is settled.

### Block 5: The settings-file reload

1. `internal/env`: read the file again into what the process environment
   did not set before it (`env.Source`).
2. `cmd/massimo`: the watch (D4) and, on Unix, SIGHUP with
   `-sighup auto|reload|stop` (`MASSIMO_SIGHUP`, D8); a reload applies
   `MASSIMO_API_TOKEN` unless `-token` was given (`Store.Bootstrap`, or the
   legacy token), and lists by name what else changed.
3. `GET /api/v1/restart` says what applies at the next restart;
   `packaging/systemd/massimo.service` gains `ExecReload`.
4. Tests with a temporary settings file; README, `docs/api-v1.md`,
   `packaging/README.md`; the CHANGELOG bullet.

**Verification:** as Block 2.

### Block 6: Event streams checked again

1. `server.go`, `events`: at each keep-alive, the caller resolved again
   as `authorize` would; the stream ends when it would be refused now.
2. Tests: a stream opened on an open server ends once a user exists; one
   opened with a key ends when the key is revoked.
3. `docs/api-v1.md`, the event stream; the CHANGELOG bullet.

**Verification:** as Block 2.

### Block 7: The UI for the reload

1. Settings → Software lists what applies at the next restart beside the
   button; the sign-in dialog opens its API-key part when the page had
   seen the server open (D6).
2. Texts in six languages; Playwright at 1100 and 390 px; `docs/ui.md`;
   the CHANGELOG bullet.

**Verification:** `make ui-test`, then as Block 2.

## Proposed commit message

Plan registration:

```
chore(aps): +Restart v1.0, +ANALYSIS Restart

The server gets a plan for a restart from the API and the web UI, under a capability of its own, and for re-reading its settings file while it runs, with the analysis it rests on.

File-revision-v1: GUIDELINES/wip/2026-11-07_12-40_AP_Restart_restart_and_reload_from_the_api.md::v1.0::2026-11-07::All::the first revision, D1 to D9 from the TL;DR gate and its two EVAL answers
File-revision-v1: GUIDELINES/wip/2026-11-07_12-05_ANALYSIS_Restart_restart_and_settings_reload.md::v1::2026-11-07::All::the analysis of the TL;DR gate
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' messages follow the form of AP OpenMode's: a `feat` each,
`feat(store)`, `feat(serve)`, `feat(api)`, `feat(ui)`, citing
`AP Restart v1.0, Block <n>`.

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate and its two
  EVAL answers; D3-B and the two switches at the owner's word, D4 open.
