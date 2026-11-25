# ANALYSIS StartConfiguration: D41-A, D42-B and the runs' findings

Read on `main` at `chore(aps): +NOTE CleanRuns v1.0, how the run went`,
2026-11-25, by three read-only investigations, each finding checked again
in the code here. The probes ran through `go test -overlay`. Nothing in
the repository changed, and nothing reached real hardware.

The owner's answers to the completion gate:
- D41-A: a `-settings` path that is not there is refused before any write.
- D42-B: install runs the start's value checks.
- D43: AutoUpdate has retired. What was the issue?
- D44: PLAN.

No open plan and no roadmap item takes any of the items below, and none
was fixed since, except item 13.

## D43: what the issue was

AP AutoUpdate's Analysis described `GET /api/v1/restart` answering
`available: false` to a holder of `update` without `restart`. That was
StartConfiguration.D34-A, which Block 12 (#443) removed.

AutoUpdate retired with `chore(aps): -AutoUpdate, -UpdateHistory`, so its
text is gone. One trace is left: the comment over `readRefused` in
`internal/server/ui/src/software/store.ts:278-286` still speaks of
"a holder of update without restart" and of "a 403 for a user with neither
update nor restart". It should say that `update` and `reboot` include
`restart`, and the 403 is for a user with none of the three. This is a
comment-only fix; it goes in passing into Block 18.

## Install

### 1. A `-settings` path that is not there (D41-A)

- `internal/install/install.go:774` exempts `os.ErrNotExist` on purpose,
  and `TestInstallSettingsNotThere` (`settings_test.go:366-388`) pins it.
- **Hand install on Linux:** the check passes. Install then writes the
  binary, the user, the state, log and settings directories
  (install.go:242-253, 654-663), and only then does the copy fail
  (680 → 845). If the old account `massimo` is renamed, `user()` stops a
  running service first (484-487), and it stays stopped.
- **macOS:** the binary and the user are written first (darwin.go:172,
  175), and the copy fails at 178.
- **Windows:** install runs `stopService` (windows.go:85) and `winBinary`
  (93) first, then the copy fails (96 → 227). The service is left stopped.
- **Packaged box:** `packagedSettings` fails at 423, before any write.
- **Dry run:** it fails in the same place, after its "would …" lines.
- **Where a settings file is kept:** Linux and macOS print "exists; kept it
  and did not copy X" (670) without saying X is not there. Windows prints
  only "exists; kept it" (windows.go:220).
- **Fix:** drop the exemption. That is one line plus its comments
  (install.go:99-104, 759-769; env/dotenv.go:151-156, 274-280). All four
  entry points already call the check before any write.
- **Tests:**
  - A "not there" case in `refusedFiles()`, so that
    `TestInstallRefusesSettingsServeCannotRead` covers the hand install, a
    packaged box and macOS, each fresh, in a dry run and over a kept file.
  - The same case in the Windows test.
  - `TestInstallSettingsNotThere` is replaced.
  - Planted fault: the exemption restored.
- **Docs:** README.md:1116-1119, packaging/README.md:152-155, and a Fixed
  fragment. docs/update-windows.md does not mention `-settings`.
- No script under packaging/ passes `-settings`.

### 2. The start's value checks in install (D42-B)

A probe installs `MASSIMO_HSTS=year`: install copies it, restarts the
service, and gives no warning.

- **One table holds the checks:** `cmd/massimo/startchecks.go:54-113`. The
  start runs it at serve.go:114. The reload runs it on the changed values
  only (reload.go:298-317).
- **Pure functions of the values:**
  - the log formats, the rotation and the level;
  - HSTS and the trusted proxies;
  - restart, SIGHUP, reboot and the announcement;
  - the TLS mode, its flags' pairing and the ACME names.
- **Checks that read the box:**
  - TLS files mode loads the certificate and key pair (certs/files.go:41).
  - DNS-01 checks that its hook exists and is executable
    (certs/dns01.go:139-143).
- **The Cubes check reads the process environment.** `serve.FileCubes`
  (setup.go:272) reads `env.Serials` and `PerCubeValue` (env.go:141), not
  the getter it is handed. So a file's bad `_IGNORE` would be missed, and
  install's own environment would leak in. Its only refusal is a bad
  `_IGNORE` list.
- **What the table leaves out:** the listen address, the log file, the
  data directory and the store, the `-tls-http` port, and making a
  self-signed certificate. These are the box's state; the service
  definitions pass `-listen`, `-log` and `-data` anyway.
- **macOS:** the plist fixes `-log-process-format none`, so the start
  never reads that line of the file there.
- **Fix:**
  - Install builds a getter from the `-settings` file's values alone.
  - It applies `-tls` first. In files mode, it checks the pair at the
    `-tls-cert` and `-tls-key` sources, which also catches a mismatched
    pair; today tls.go:62-66 only checks that they exist.
  - On macOS it applies the fixed log format.
  - It refuses with every refusal named and "nothing was written", in a
    dry run too.
  - A kept file gets the same checks, as warnings ("the next start
    refuses: …"), and the install goes on.
- **What install cannot check:** whether `_massimo` can read the key or
  run the hook (install runs as root), and relative paths (the service
  runs from `/`). These stay the start's and the reload's refusals.
- **Size:** about 150-250 lines across `cmd/massimo/{install,startchecks,
  reload}.go`, `internal/install/{install,tls,windows,darwin}.go`,
  `serve/setup.go` and `env/env.go`, and about 200 lines of tests. All
  three systems.
- **Tests, each with its planted fault:**

  | Test | Planted fault |
  |------|---------------|
  | each entry point, dry and kept, refuses with nothing written | the check run after the writes, or skipped on one system |
  | a bad HSTS and a file's bad `_IGNORE` are refused, and the process environment is ignored | `FileCubes` reading `os.Environ` |
  | the macOS log format passes | the fixed flag ignored |
  | a mismatched `-tls` pair is refused | an existence check only |
  | `-tls off` over a file's `MASSIMO_TLS_HTTP` passes | `-tls` not applied |
  | a kept bad file installs with its warning | strict on a kept file |

### 3. A kept file install cannot read: its TLS drop-ins fall to http

- `checkSettings` prints "cannot check" (install.go:707-709), and
  `settingsVars` returns nil without a word (tls.go:272-275). So TLS
  counts as off:
  - the health drop-in is rewritten from https to http;
  - the listen drop-in loses `CAP_NET_BIND_SERVICE` for `-tls-http :80`;
  - on macOS, the helper plist's URL becomes http.
  - Windows has no drop-ins and is not affected.
- The start refuses that file anyway: the strict read stops every command
  (main.go:28-31).
- **The harm comes later,** once the owner fixes the file without running
  install again:
  - The update helper's http check never gets JSON from an https server,
    so every update rolls back (update.go:384-425).
  - `:80` is denied, so the start fails.
- **Fix:** where the file cannot be read, keep the TLS-derived files as
  they are and warn that the next start refuses the file. About 50 lines
  in tls.go, listen.go and darwin.go.
- **Tests:** a hand install, a packaged box and macOS, each with earlier
  https drop-ins. Planted fault: the nil fallback.

### 4. `install -tls-http :80x` names `-listen`

- tls.go:86 calls `splitListen`, which adds `-listen %q:` itself
  (listen.go:28). Install says `-tls-http: -listen ":80x": bad port`.
- **Fix:** `readListen` with `-tls-http %q: %w`, which gives
  `-tls-http ":80x": bad port`. Two lines, and a test row that asserts the
  exact text. Planted fault: `splitListen` back.
- serve's own message (serve.go:401) is already right.

## Windows names

### 5. A name in another case: one the start applies, the reload does not see

A probe of `internal/env` ran on Windows through interop.

- `LoadFile` sets `massimo_listen`, and `Getenv("MASSIMO_LISTEN")` returns
  its value. So **the start applies it**, and the start's checks see it.
- The reloader's `settings()` (reload.go:122-130) takes only an exact
  `MASSIMO_` prefix and an exact `Known` name. It does not see the
  setting. A changed value, even one the next start refuses, shows as
  neither pending nor refused, and the service fails at its next start.
- `config.go:61,111` hide it from `massimo config` and
  `GET /api/v1/config`. A name in mixed case is listed but marked unknown.
- `env.Serials` (env.go:189) misses a prefix in lower case.
- **Security:** `server.SecretVariable` (serverinfo_api.go:55-57) is
  case-sensitive. On Windows, `MASSIMO_API_Token=…` in the settings file
  is the owner's key: the start reads it through `Getenv`. Yet
  `GET /api/v1/config` lists its value, since `_Token` is not `_TOKEN`. It
  shows it to any holder of `settings`, and an integration can hold that.
  Checked here in the code.
- **Fix:**
  - `settings()` and both config listings key names by `nameKey`
    (exported from `env`).
  - `Known` is checked on that key, and `SecretVariable` on the upper case.
  - The secret fix lands first, in a `fix(security…)` commit: listing more
    names would otherwise list secrets in lower case too.
  - About 40 lines. Linux is unchanged, since `nameKey` is the identity
    there.
- **Tests:**
  - `cmd/massimo/reload_windows_test.go` (CI's Windows job runs
    `./cmd/...`): a change in lower case is pending, and a refused value
    is refused. Planted fault: the exact prefix restored.
  - On every system: `SecretVariable("MASSIMO_API_Token")` is true.

### 6. A reload that changes value and case keeps the old spelling

In the probe, a start with `massimo_listen=:1` and then a reload with
`MASSIMO_LISTEN=:3` reports a change. Windows keeps the spelling
`massimo_listen`, and `settings()` stays blind until the next start. The
other way round is harmless. Item 5's fix, comparing by key, settles
this, and the probe's case becomes a Windows test.

## Docs and packaging

### 7. The capability tables have drifted

The truth is the routes the server registers: the `api(` calls in
server.go:1501-1608, config_api.go:38-47 and repair_api.go:52-67.

- **`docs/api-v1.md:237-262`:**
  - It has no `integrations` row, although `GET /api/v1/capabilities` is
    said to follow this table.
  - Its `read` row names `GET /api/v1/me`, which needs no capability, and
    "backup state", which needs `backup`.
  - Its `read` row leaves out `/log`, `/log/alerts`, `/system`,
    `/push/vapid` and `/commands/{id}`.
  - Its `users` row leaves out the identities routes.
- **`docs/store.md:1197-1225`:**
  - `update` lacks `PUT /api/v1/update/answer`.
  - `settings` lacks `GET /api/v1/store` and `/config`.
  - `preferences` lacks `/me/push`.
  - `configure` lacks the copy in `PUT …/devices/{rf}`.
  - `backup` names only stage (e)'s list.
  - `read` has the same gaps as api-v1.md's.
- **Both tables** leave out `DELETE …/backups/{id}` (`restore`).
- **Fix:** api-v1.md's table becomes the only one with routes, and a test
  keeps it in step. The test reuses `registeredRoutes` of openapi_test.go
  and checks three things:
  - the rows equal `capabilities`, in order;
  - every route a row names is registered with that capability;
  - every registered route is in its row.

  store.md §6 keeps its Allows column and the roles, and points to
  api-v1.md for the routes. That means about 120 lines of test, and the
  prose cells written as routes.
- **Planted faults:** `update/answer` dropped; a route in the wrong row;
  the `integrations` row removed.

### 8. postinst's `settings()` and a byte order mark

- `packaging/deb/postinst:20-22` drops blank and `#` lines and compares
  the rest with the template's (:136).
- A UTF-8 BOM turns the first comment into a "setting". With a BOM
  prepended to `packaging/env.template`, the comparison says "differs".
- So a box still on the template's settings, with no owner yet, is
  started. That is what the check exists to prevent: until there is an
  owner, any device on the LAN may use it.
- serve itself skips the BOM (D5-A).
- **Fix:** remove a BOM at the start of the first line. One line, and a
  case in `packaging/deb/test.sh` ("a template with a BOM: not started"),
  run by `make deb-test`. Planted fault: the strip removed.
- AP Packaging is open, but none of its blocks touches postinst.

## Cubes

### 9. The plain Cube's loggers carry no serial learned later

- The client's and the queue's loggers are fixed when the runtime is made
  (runtimes.go:123, 134), and the proxy's when the listener starts (307).
  At those times the serial is still "".
- The serial is learned later, in `pump` (server.go:996, 1056). Event lines
  read `rt.cubeSerial()` for each line and are right.
- **Probe:** after `KEQ0000100` was learned, a pause and resume logged
  "cube connection attempt failed", "re-established" and "greeting" with
  an empty `cube=`. In the default log, the gaps are "cube reconnecting",
  "cube still unreachable", "poll failed", and the queue's and the proxy's
  lines.
- This breaks README.md:722-723 ("`cube=` where it concerns one"). It
  lasts for the first run, until the next start ties the row.
- **Fix:** a `slogx` handler that reads the serial on each record. About
  40 lines.
- **Tests:**
  - A unit test. Planted fault: the value taken at construction.
  - A server test: after the connection, the client's, queue's and proxy's
    lines carry `cube=KEQ0000100`. Planted fault: today's
    `partLog(rt.serial)`.
  - A stored Cube's lines carry `cube` exactly once. Planted fault: the
    key twice.

### 10. A hostname against an IP is not tied at the start

- `PlainRow` and `cubeEndpoint` (setup.go:222-241) compare strings: the
  host in lower case, and the port.
- Tying means the plain lines become that row's Cube: `seed.plain_serial`
  is set and one runtime serves it (setup.go:66-81).
- **Probe:** one single-client fake, the row at `127.0.0.1:P`, the plain
  address `localhost:P`. It went two ways in three runs:
  - **The stored runtime won (1 of 3):** a second Cube is listed by its
    address, with no serial and never connected. It redials for the whole
    run, and a plain proxy serves that dead Cube. The setting stays empty,
    so every start races again.
  - **The plain runtime won (2 of 3):** it learns the serial, finds it
    served and stops (server.go:1062-1074). The setting is written, so
    the next start heals.
  - No Cube lockout follows, because the loser closes cleanly.
- **It is realistic:** a Cube added from discovery carries its IP, while a
  settings file often carries a name.
- **Fix:** where the plain lines are not tied and no address matches as
  written, resolve both hosts once, within 2 s in all. They tie where a
  resolved IP and port match. About 60 lines.
- **The DNS risks are small:**
  - a broken resolver delays the start by at most 2 s, then falls into
    today's race;
  - `.local` names may not resolve in the static build;
  - a wrong tie needs the same IP and port at that moment, which is the
    same Cube;
  - the tie is saved, so the lookup is rare.
- **Tests,** with a resolver injected and no network:
  - a name matching the row's IP ties, and so does the reverse;
  - one address of several matching ties;
  - another port does not tie;
  - an error or a blocking resolver neither ties nor fails the start, and
    returns within the bound;
  - the resolver is not called when the Cube is tied or matches as
    written;
  - `setupCubes` returns one Cube, not two.

  Planted faults: the port ignored; no timeout; always resolving;
  resolution removed.

## Test kit

### 11. `fakecube.Server.Close` panics on a fake that never served

- fakecube.go:566 calls `ln.Close()` on a nil `s.ln`. Only `Serve` sets
  it.
- Reproduced: `&Server{}` then `Close()` panics, and so do Pause then
  Close.
- **Fix:** a nil guard, three lines and a two-case test. Planted fault:
  the guard removed. It needs a fragment, since `massimo fakecube` is
  shipped.

### 12. Windows: the test binary's directory is sometimes left behind

- AP CleanRuns covers make targets and suite scripts only, not Go test
  directories.
- TestMain (`cmd/massimo/update_store_test.go:39-40`) removes
  `%TEMP%\massimo-test-bin-*` once and ignores the error. A killed or
  timed-out run never gets there. Block 10 made that binary build on
  Windows.
- `internal/update/store.go:84-104` also ignores its removal errors.
- Not reproduced: the directory was removable 40 times out of 40.
- **Fix:**
  - TestMain removes with retries and names a directory it could not
    remove.
  - `builtBinary` first clears `massimo-test-bin-*` directories older
    than a day, as CleanRuns D3-A does for the UI suite.
  - About 40 lines.
- **Tests:**
  - An old directory is cleared and a fresh one is kept. Planted fault:
    the age check dropped.
  - On Windows, a file held open for 300 ms. Planted fault: a single
    RemoveAll.

## 13. Already done: the Software card reads `refused`

`software/store.ts:278-297` decides on `refused`, since AP AutoUpdate's
Block 7 (#445). Only its comment is stale (see D43 above). Drop this
item.

## Proposed blocks of v1.2

| Block | Name | Items | Commit |
|-------|------|-------|--------|
| 14 | SettingsThere | 1 | `fix(install)` |
| 15 | SettingsValues | 2, 4 | `feat(install)` |
| 16 | KeptUnread | 3 | `fix(install)` |
| 17 | WindowsNames | 5, 6 | `fix(security:api)`, then `fix(server)` |
| 18 | Tables | 7, the comment of D43 | `docs(api)` with its test |
| 19 | PostinstBom | 8 | `fix(packaging)` |
| 20 | PlainLogs | 9 | `fix(server)` |
| 21 | PlainResolve | 10 | `fix(serve)` |
| 22 | TestKit | 11, 12 | `fix(fakecube)`, `test(cmd)` |

## Decisions, numbered on from D44

- **D45. A missing `-settings` path while a settings file is kept.**
  - A (recommended): refuse it too, as #440 does for a file it cannot
    read. One rule; a wrong path is a mistake either way.
  - B: pass, saying it is not there.
- **D46. How install gets the start's checks.**
  - A (recommended): `cmd/massimo` hands the table's run to install in
    `Options`. One table, a small change, no drift.
  - B: move the table and its parsers into an internal package.
- **D47. The Cubes check.**
  - A (recommended): `FileCubes`, `Serials` and `PerCubeValue` take the
    environment as a list. The start and the reload pass `os.Environ()`;
    install passes the file's lines.
  - B: install leaves the Cubes check out, documented.
- **D48. The checks that read the box: the TLS pair and the DNS-01 hook.**
  - A (recommended): refuse as the start does, as root sees the box, with
    `-tls`'s own files checked at their sources. The start would refuse
    the same file.
  - B: warn only.
- **D49. A kept file with a value the start refuses.**
  - A (recommended): warn and go on. An upgrade never fails.
  - B: warn and do not restart.
- **D50. Windows' kept file.**
  - A (recommended): read it and warn as Linux does, before the service
    is stopped.
  - B: leave it unread, as today.
- **D51. A kept file install cannot read.**
  - A (recommended): keep the TLS-derived files as they are, with a
    warning. Install does not know the TLS state, so it does not change
    them.
  - B: decode UTF-16 or skip the long line for install's view alone.
  - C: leave it as today.
- **D52. How many refusals install names.**
  - A (recommended): every one, as the reload does, so that the file is
    fixed in one pass.
  - B: the first, as the start does.
- **D53. A known name in another case.**
  - A (recommended): Windows takes it as `MASSIMO_X` everywhere: the
    reload, the config listings, the secrets. Linux, where the start
    ignores it, warns "ignored: names are upper case". Today Linux drops
    it silently and Windows applies it unseen.
  - B: the Windows fix only.
  - C: refuse it at the start, which breaks Windows boxes that work
    today.
- **D54. The spelling after a case change on Windows.**
  - A (recommended): compare by key, and keep Windows' spelling.
    Spelling it again means unset then set, and a `Getenv` in between
    would see it unset.
  - B: spell it again in `ReloadFile`.
- **D55. The capability tables.**
  - A (recommended): api-v1.md's table is the one with routes, kept by a
    test; store.md §6 points there. The OpenAPI file already carries each
    route's capability, so a third copy kept by hand drifts.
  - B: fix both by hand, with no test.
  - C: fix both, and test both.
- **D56. postinst's byte order mark.**
  - A (recommended): a block of v1.2. It is this plan's D5-A, and AP
    Packaging has no live block in that file.
  - B: a revision of AP Packaging.
- **D57. A hostname against an IP.**
  - A (recommended): a lookup bounded to 2 s at the start, where nothing
    is tied yet. Simple, and needed only until the tie is saved.
  - B: tie when the plain runtime dials, by the remote address it
    reaches. Exact, with no lookup, but medium-sized, and it touches
    `pump`.
  - C: the plain runtime dials first, and the stored ones wait. No DNS,
    but it fixes only the start.
  - D: a warning and docs only.
- **D58. `massimo config` and the tie.**
  - A (recommended): it uses the same lookup, so that it agrees with
    serve.
  - B: it compares as written.
- **D59. The test kit (items 11 and 12).**
  - A (recommended): one block of v1.2. Block 10 put `builtBinary` on
    Windows.
  - B: AP CleanRuns v1.1.
  - C: the roadmap.
- **D60. The pull requests.**
  - A (recommended): five, each from main:
    - Block 17 first, alone, since it carries the security fix;
    - Blocks 14-16, install;
    - 18-19, docs and packaging;
    - 20-21, the plain Cube;
    - 22, the test kit.

    The blocks of one pull request share code and tests.
  - B: one per block.
  - C: one for all.
