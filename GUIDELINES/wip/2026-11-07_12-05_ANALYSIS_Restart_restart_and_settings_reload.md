# Analysis: restart from the API, settings reload, and the open page on 401

For the TL;DR gate of the plan `Restart` (not written yet; `PLAN+` writes it).
Code read at `origin/main` 84749ec (after v0.10.0-alpha.1).
jaguar runs v0.7.0-alpha.1, 311 commits older; where that matters, it says so.

## 1. What happened on jaguar, against the code

### 1.1 "set MASSIMO_API_TOKEN to start updates from the API"

v0.7.0-alpha.1 decided authentication by `Options.Token` alone
(`updateBlocked`: `s.opts.Token == ""`).
The token is read once, as the default of `serve -token`
(`cmd/massimo/serve.go:52`), after `env.LoadFile` put the file into the
process environment (`cmd/massimo/main.go:24`).
Nothing reads the file again, so an edit waits for a restart.
"Check for updates" only asks GitHub again (`GET /api/v1/update?refresh=1`).

On main the mode is live: with a store, the user count decides
(`internal/server/auth.go:146`, `authMode`), and the message now reads
"set MASSIMO_API_TOKEN or create a user".
Creating the first owner from Settings → Users (open mode, a caller on the
local network) switches to users mode at once, without a restart.
The settings file still does not: `Bootstrap(token)` runs once, in
`server.New` (`internal/server/server.go:420`).

### 1.2 The open page after the restart (verified)

v0.7.0-alpha.1, `app.js` `askToken`: the dialog opened on the first `401`,
but it had no close path. Esc (or any close without submit) left
`tokenPromise` pending, and every later `api()` call awaited it forever,
silently: the poll, "Check for updates", the stream. Only a reload cleared it.
That is the most likely cause of what the owner saw:
the page did ask once, probably from the 10 s poll,
and after that it could not ask again.

On main this is fixed (`login.js` `askLogin`, `dlg.onclose = () => done(false)`,
since 8486c12, 2026-11-06, first released in v0.8.0-alpha.1):

- every `api()` `401` opens the sign-in dialog (`app.js:245`), a dismissed
  one resolves false, and the next `401` asks again;
- the poll (`poll` → `loadCubes`) hits `GET /api/v1/cubes` every poll
  interval, so an open page asks within one interval without a click;
- "Check for updates" goes through `api()` and asks too;
- after a sign-in, `afterLogin` reads `/me` again and restarts data and stream.

What remains:

1. The dialog does not say why it appears now. The owner made from
   `MASSIMO_API_TOKEN` has no password: the way in is pasting the token into
   the key field, which the dialog does not say.
2. An event stream opened in open mode stays connected after the mode
   changes (`server.go:1247`, `events` does not re-check), and a ticket
   issued in open mode carries an open principal for its minute
   (`auth.go:672`). Today only the first owner made from the UI changes the
   mode while running; a live reload of the token would be the second way.

## 2. How the server can restart itself

Every supervisor the installers set up restarts the server when it ends
with an error, and has done so since its first version (2026-11-05).
A clean stop (exit 0, or the Windows service reporting stopped) is not
restarted anywhere.

| Run | What restarts it | Exit that restarts | Delay | Notes |
|-----|------------------|--------------------|-------|-------|
| systemd (`massimo install`, the deb) | `Restart=on-failure`, `RestartSec=10s` | any non-zero | 10 s | journal says "Failed with result 'exit-code'"; `RestartForceExitStatus=75` + `SuccessExitStatus=75` in the unit make it a planned restart; an old unit restarts on 75 anyway |
| launchd (`massimo install` on macOS) | `KeepAlive` `SuccessfulExit=false`, `ThrottleInterval 10` | any non-zero | 0–10 s | |
| Homebrew `brew services` | `keep_alive true` | any | 0–10 s | |
| Windows service | recovery actions: restart after 10 s ×3, reset daily (`internal/install/windows.go:133`); the SCM repeats the last action | process ends without reporting stopped | 10 s | Event log: "terminated unexpectedly"; counts as a failure |
| Windows service, through the SCM | a detached helper calls `install.RestartService` (stop → serve stops cleanly → start), as the update helper already does (`update_helper_windows.go`) | – | ~2 s | LocalSystem may; job-object checks exist (`jobFlags`) |
| terminal, Docker, a custom unit | nothing, or what the owner configured | – | – | exit = stopped |

Detecting a supervisor without changing existing units:
Windows `svc.IsWindowsService` (already `runningAsService`);
systemd sets `INVOCATION_ID` for every service;
launchd sets `XPC_SERVICE_NAME` to the job's label.
For Docker (`--restart`) or a hand-written unit, an override such as
`MASSIMO_RESTART=exit|off` says what to do.

The stop must be the normal one: cancel serve's context, as SIGTERM does,
so that `Run` writes the log's pending events, the command steps and the
refusals, and the store closes (`serve.go:200`); the exit code comes after.
The audit row of the `POST` is written before that.

## 3. Settings-file changes without a restart

What the file holds and what could apply live:

| Variable | Live? |
|----------|-------|
| `MASSIMO_API_TOKEN` | yes: `Bootstrap(token)` again (idempotent: adds the key, revokes the old one); legacy mode (`-addr`) swaps `Options.Token` |
| `MASSIMO_CUBE_<SN>_*` (address, name, proxy, hidden) | seeds only; the store wins; `FileCubes` could be refreshed for the "file differs" hints |
| `MASSIMO_CUBE_<SN>_KEY`, `_IV` | not used by serve |
| `MASSIMO_LISTEN`, `MASSIMO_TLS*`, `MASSIMO_HSTS`, `MASSIMO_ACME_*`, `MASSIMO_TRUSTED_PROXIES`, `MASSIMO_LOG`, `MASSIMO_UPDATE_*` | at restart |

Triggers:

- **A file watch**: `stat` every few seconds, applied once size and time
  have held for one more check, so that an editor's half-written file is not
  read; a parse error or a missing file (an editor's rename) keeps what runs.
  Works on every platform; no signal and no click.
- **SIGHUP** (Unix only): the same reload, at once; roadmap item 9
  ("log rotation through SIGHUP") would reopen `-log` from the same handler.
  Windows has no SIGHUP, so it cannot be the only trigger.
- **On request**, `POST /api/v1/settings/reload`: explicit, but the owner
  must know to click it.

Rules a reload keeps: `-token` on the command line wins over the file, and
a variable already in the process environment wins over the file
(`env.Source` records which); removing the token from the file revokes its
key live, as a restart does today.

Whatever else changed is logged by name (never the value) and listed as
"applies at the next restart": that is what the restart button shows.

## 4. Open mode and the restart

An open API (a store without users, or `-addr` without a token) must not let
anyone on the LAN restart the server. Updates already follow the rule
"never from an unauthenticated API" (`update_api.go`, `startUpdate`).
The restart can follow it: refused in open mode, with the reason
"set MASSIMO_API_TOKEN or create a user". With the live reload (§3), that
costs nothing: setting the token is what lifts the refusal.

## 5. The capability

- `restart` joins `capabilities` (`internal/server/auth.go:66`) and the UI's
  `CAPABILITIES` (`login.js`); owner holds it by code.
- `manager` gets it from a migration, as `0003_preferences_capability.sql`
  did for `preferences`: additive, `min_reader: 1`.
  **The next free migration is 0006**: 0005 is `0005_backups_reasons.sql`.
- `docs/api-v1.md` "Authentication" table and the default-roles sentence,
  `docs/store.md` §6 table, `cap.restart` in all six i18n files,
  `x-capability` in the OpenAPI file.

## 6. Open decisions

- **D1. How the server restarts itself**
  - **A (recommended).** Clean stop, then: Unix exit 75, which systemd,
    launchd and Homebrew restart (unit gets `RestartForceExitStatus=75`
    `SuccessExitStatus=75`); Windows a detached helper restarts the service
    through the SCM. Clean in every log, ~2 s on Windows.
  - **B.** Exit 75 everywhere, Windows by its recovery actions:
    one path, but 10 s and an Event-log error per restart.
  - **C.** The root update helper runs `systemctl restart`:
    needs the helper, absent with Homebrew.
- **D2. A run without a supervisor**
  - **A (recommended).** Not offered: the API says why
    ("no service manager restarts this server"), `MASSIMO_RESTART=exit`
    for Docker or a custom unit.
  - **B.** Re-exec itself: works in a terminal, but the environment must be
    rebuilt without the file's values, and on Windows the PID and Ctrl-C change.
- **D3. Open mode**
  - **A (recommended).** Refused as updates are, with the reason.
  - **B.** Loopback only: behind a reverse proxy on the box everything is loopback.
- **D4. Settings file without a restart**
  - **A (recommended).** File watch + SIGHUP on Unix; the token applies live,
    the rest is listed as "applies at restart".
  - **B.** A "Reload settings" button only: a click the owner must know of.
  - **C.** None; the restart button alone: the owner's requirement unmet.
- **D5. Streams and tickets when the mode changes**
  - **A (recommended).** On a change of mode, or a revoked key, close the
    event streams and drop the tickets; pages reconnect and are asked to sign in.
  - **B.** Leave them until the next restart: an open-mode stream keeps
    sending after the key is required.
- **D6. The sign-in dialog after the switch**
  - **A (recommended).** One line when the page knew the server open:
    "The server now asks for a sign-in; an API key such as MASSIMO_API_TOKEN
    goes into the key field."
  - **B.** As it is.
- **D7. Pull requests**
  - **A (recommended).** Two: the restart (capability, mechanics, API, UI),
    then the reload (watch, token, streams, pending list); each reviewable alone.
  - **B.** One.
