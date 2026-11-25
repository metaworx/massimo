# AP StartConfiguration v1.2: The settings file and the start

## TL;DR

The settings file, the reload, the start and `massimo install` disagree
in small ways that cost a lockout, a lost setting, a looping service or
a leaked key.
This plan gives them one parser and one set of rules, and has install
refuse what the next start refuses.

- `install -settings`, no file → refused before any write
- a value the start refuses → install refuses it; a kept file's → a warning
- a kept file install cannot read → its https drop-ins kept
- Windows, a name in another case → its secret hidden; reload, config see it
- capability tables → AP CapabilityTable (D55)
- postinst, a byte order mark → skipped
- the plain Cube's log lines → its serial, once learned
- a hostname against the row's IP → tied, a lookup within 2 s
- fakecube's `Close` → no panic; Windows test binaries → swept

Blocks 1 to 13 are built and merged (#425, #427, #430, #440, #443);
v1.2 adds Blocks 14 to 22, in five pull requests from `main`.

### Blocks

1. Lines: one parser, the first line wins; byte order mark, UTF-16, `=x`, `MASSIMO_ENV_FILE` (merged, #425)
2. Notes: a note on the file on stderr, at a reload, in config and `settings_read` (merged, #425)
3. Install: `massimo install` reads the file with env's parser (merged, #425)
4. ApiReload: a failed API reload rotates and checkpoints; the SIGHUP test waits for the reload's end (merged, #427)
5. Settle: the newest `settings_read` warning settles the older ones (merged, #427)
6. RestartRead: `GET /api/v1/restart` with `restart` or `update` (merged, #427)
7. Card: the Maintenance card's badge and questions while the next start refuses (merged, #427)
8. Names: the start's own errors name their setting (merged, #430)
9. Helper: the Windows update helper's listen blocks updates, not the start (merged, #430)
10. Windows: the guard and `PerCubeValue` on Windows, in CI's Windows job (merged, #430)
11. PlainProxy: a plain Cube's proxy listens for it beside the store's Cubes (merged, #430)
12. Include: `update` and `reboot` include `restart` (merged, #443)
13. SettingsRefused: `install -settings` refuses a file serve refuses (merged, #440)
14. SettingsThere: `install -settings` refuses a path that is not there, before any write
15. SettingsValues: install runs the start's value checks; `-tls-http` names itself
16. KeptUnread: a kept file install cannot read keeps its TLS drop-ins
17. WindowsNames: a name in another case: its secret hidden, then seen by the reload and config
18. Tables: carried to AP CapabilityTable
19. PostinstBom: postinst skips a byte order mark before it compares
20. PlainLogs: the plain Cube's log lines read its serial on each line
21. PlainResolve: a hostname and an IP of one Cube tie the plain lines to its row
22. TestKit: fakecube's `Close` before it served; the Windows test binaries swept

## Discussion

### What the owner asked for

On 2026-11-24, after AP LogEdges and AP ReloadChecks retired:
`PLAN fixes for the open points`.
The analysis this plan rests on followed, its twelve points with D1 to D11,
D20 to D27 and D30.
At its gate and two EVALs, the owner chose one plan for the points of the
start and its settings, D30-C, and named it StartConfiguration.
On 2026-11-25: `PLAN++`, for this plan written and committed onto main,
the update's question passed to AP UpdateHistory's session, and three
stacked pull requests built, nothing merged.
On 2026-11-25, at the run's completion gate: `EXEC merge`, for the three
pull requests merged in order (#425, #427, #430), with D36-A, D37 and
D38-B.
At the next gate: `PLAN++`, for this revision with D39-A and D40-A,
committed onto main, and its two blocks built as two pull requests from
`main`, nothing merged.
On 2026-11-25, at v1.1's completion gate:
`D41A, D42B, D43: AutoUpdate just retired. what's the issue?, D44: PLAN`.
The analysis v1.2 rests on followed, its items 1 to 13 with D45 to D60.
At its TL;DR gate: `D55: check with session "store.md vs api-v1.md"`,
and `PLAN++`, for this revision with every other decision's option A,
committed onto main, and its blocks built as pull requests from `main`,
nothing merged.
That session's answer carried D55 to its plan, AP CapabilityTable.

### What is decided

The owner, 2026-11-24:

- **D1-A. A name given twice:** the first line wins, at the start and at a
  reload, an empty one included.
- **D2-A. What config says of it:** its source names both lines, "file,
  lines 3 and 4: the first wins".
- **D3-B. Where a note on the file is told:** on stderr at each command's
  start, as a Warn at a reload that finds one, in config, and as a
  `settings_read` warning.
  The text keeps the two kinds apart: "the next start refuses: …" for a
  refusal, "the settings file: …" for a note.
  The card's badge (D9) and its notice stay with refusals only.
- **D33-A. The start's notes:** the start writes a `settings_read` warning
  after its `server_started`, which settles every earlier one, so that a
  restart does not clear the sign while the note stands.
- **D4-A. A line without a name:** ignored, with a note.
- **D5-A. A byte order mark:** a UTF-8 one at the file's start is skipped;
  a UTF-16 file is refused as "UTF-16 text, save it as UTF-8".
- **D6-A. A `MASSIMO_ENV_FILE` line:** ignored, with a note; the file the
  start read stays the one file read.
- **D7-A. Install's reader:** `internal/install` uses env's parser.
- **D8-A. A failed reload through the API:** rotate and checkpoint whatever
  the read found, then answer 500, as SIGHUP does.
- **D9-A. The card's head while the next start refuses:** a second badge,
  a warning, with a fixed text; the pending count stays.
- **D10-C. The questions while the next start refuses:** a sentence added to
  the restart's question, the reboot's and the update's.
- **D35-A. The update's question:** passed to AP UpdateHistory's session,
  whose v1.1 works in the Software card (`Software.tsx`);
  this plan keeps the restart's and the reboot's, and provides the read the
  update's question needs (D34).
- **D34-A. Who may read `GET /api/v1/restart`:** a holder of `restart` or of
  `update`, since every update restarts the server.
  Built in Block 6; D37 goes past it, and Block 12 takes the route back to
  `restart` alone.
- **D11-A. `settings_read` warnings in a row:** the newest settles the older
  ones (`any: true`).
- **D20-A. The start's own errors:** `MASSIMO_LISTEN`, `MASSIMO_LOG`,
  `MASSIMO_DATA` and `-tls-http`, each as `-flag or VAR: …`.
- **D21-A. The Windows update helper's listen:** an address
  `install.HealthURL` cannot read blocks updates from the API, with the
  reason, instead of stopping the start, as under brew.
- **D22-A. The guard on Windows:** it runs, and its files ask for CI's
  Windows job.
- **D23-A. `PerCubeValue` on Windows:** it returns the name as it is set,
  and the Windows job runs `./internal/env`.
- **D24-B, as the owner refined it.** Where both plain lines,
  `MASSIMO_CUBE_ADDRESS` and `MASSIMO_CUBE_PROXY`, are in the file, the plain
  proxy listens for the plain Cube at once, which then takes the serial it
  reports.
  A plain `MASSIMO_CUBE_PROXY` without the plain address keeps today's
  behaviour, with no warning: `serve` ignores it unless one Cube has lines
  of its own, and the other commands connect to it.
- **D30-C, D32-A:** one plan, in three pull requests stacked on each other:
  the parser (Blocks 1–3), the reload and its signs (4–7), the start and
  Windows (8–11).
- **D25-A to D27-A, D31-A:** the three points outside the start had no plan,
  and are merged (#413).

The owner, 2026-11-25, at the run's completion gate, on the points the run
left open:

- **D36-A. The plain proxy of a Cube the store has:** the store's rule, as
  built in Block 11: the plain lines are that Cube's, compared with its
  row; the plain proxy is a difference with its button.
- **D37. Who may restart:** `reboot` and `update` each include `restart`,
  in the owner's words.
  Whether `reboot` also includes `update` was left open: an update may bring
  new or incompatible features, while a reboot is mainly for maintenance.
- **D38-B. `install -settings` with a file serve refuses** (a line that is
  not `KEY=VALUE`): refused before any write, as UTF-16 is, in a pull
  request of its own.

And at the gate that offered this revision, with `PLAN++`:

- **D39-A. How `include` is built:** at the check.
  A holder of `update` or of `reboot` passes every check of `restart`;
  no role's stored set changes, and no migration runs.
- **D40-A. `reboot` does not include `update`,** for the owner's reason.

The owner, 2026-11-25, at v1.1's completion gate:

- **D41-A. A `-settings` path that is not there:** refused before any
  write, as a file serve cannot read is.
- **D42-B. Install and the start's value checks:** install runs them on
  the `-settings` file, before any write.
- **D43. AutoUpdate's text:** answered.
  It described D34-A's answer to a holder of `update` without `restart`,
  `available` false, which Block 12 removed; it went with AutoUpdate's
  retirement (`chore(aps): -AutoUpdate, -UpdateHistory`).
  The comment over `readRefused` in
  `internal/server/ui/src/software/store.ts` still says it; it is fixed
  in Block 19's pull request, since Block 18 is carried to
  AP CapabilityTable (D55).
- **D44. The run's other findings:** `PLAN`, answered by the analysis
  this revision rests on.

And at the analysis's TL;DR gate, with `PLAN++`, each the analysis's
option A, D55 aside:

- **D45-A. A missing `-settings` path beside a kept file:** refused too.
  One rule: a wrong path is a mistake either way.
- **D46-A. How install gets the start's checks:** `cmd/massimo` hands the
  table's run to install in `Options`.
  One table, a small change, no drift.
- **D47-A. The Cubes check:** `FileCubes`, `Serials` and `PerCubeValue`
  take the environment as a list; the start and the reload pass
  `os.Environ()`, install the file's lines.
  So the file's own values are checked, and install's environment does
  not leak in.
- **D48-A. The checks that read the box, the TLS pair and the DNS-01
  hook:** refused as the start refuses them, as root sees the box, with
  `-tls`'s own files checked at their sources.
  The start would refuse the same file.
- **D49-A. A kept file with a value the start refuses:** a warning, and
  the install goes on.
  An upgrade never fails.
- **D50-A. Windows' kept file:** read and warned of as on Linux, before
  the service is stopped.
  The same rule on every system.
- **D51-A. A kept file install cannot read:** its TLS-derived files kept
  as they are, with a warning.
  Install does not know the TLS state, so it changes nothing of it.
- **D52-A. How many refusals install names:** every one, as the reload
  does.
  The file is fixed in one pass.
- **D53-A. A known name in another case:** Windows takes it as
  `MASSIMO_X` everywhere, in the reload, the config listings and the
  secrets; Linux, whose start ignores it, warns "ignored: names are upper
  case".
  Today Linux drops it silently and Windows applies it unseen.
- **D54-A. The spelling after a case change on Windows:** names compared
  by key, Windows' spelling kept.
  Spelling it again means unset then set, and a `Getenv` in between would
  see it unset.
- **D55. The capability tables:** carried over to AP CapabilityTable.
  The owner asked to check with the session "store.md vs api-v1.md"
  first; its plan, from the owner's `PLAN++ D1-A, D2-A` of 2026-11-25,
  takes all of the analysis's item 7 and more:
  `docs/api-v1.md`'s table the only one with routes, `docs/store.md` §6
  with no capability table, keeping only the default roles and their
  history, and `TestCapabilityTableCoversRoutes` holding the table to
  the registered routes.
  Its docs fix on main,
  `docs(api): The capability tables name every route the server guards`,
  already closes every gap item 7 lists, in both tables.
  The reason: that plan holds the whole scope, and its part on main
  already fixes the drift; Block 18 builds nothing here.
- **D56-A. postinst's byte order mark:** a block of this plan.
  It is D5-A's rule, and AP Packaging has no block in that file.
- **D57-A. A hostname against an IP:** a lookup bounded to 2 s at the
  start, where nothing is tied yet.
  Simple, and needed only until the tie is saved.
- **D58-A. `massimo config` and the tie:** the same lookup, so that it
  agrees with serve.
- **D59-A. The test kit, items 11 and 12:** one block of this plan.
  Block 10 put `builtBinary` on Windows, and AP CleanRuns, retired,
  covered make targets and suite scripts only.
- **D60-A. The pull requests:** five, each from `main`: Block 17 first,
  alone, since it carries the security fix; Blocks 14 to 16, install;
  Block 19, with D43's comment; Blocks 20 and 21, the plain Cube;
  Block 22, the test kit.
  The blocks of one pull request share code and tests.

### Where it meets other plans

- **AP Packaging** owns `packaging/env.template`; Block 1 adds one comment
  line to it.
  Block 19 changes `packaging/deb/postinst`, which none of its blocks
  touches (D56-A).
- **AP CapabilityTable** takes the capability tables of `docs/api-v1.md`
  and `docs/store.md` §6, item 7 of v1.2's analysis (D55); Block 18 is
  carried to it.
- **AP LogAlerts** owns the settle table, which Block 5 changes;
  AP RadioHealth names `logalerts.go`.
- **AP UiParts** owns the parts' rules; Block 7 follows them in
  `src/maintenance/`.
- **AP UpdateHistory** takes the update's question (D35-A), and reads
  `GET /api/v1/restart` as Block 6 allows, and after Block 12 as `update`'s
  holder, who holds `restart` through it.
  It has retired, with AP AutoUpdate.
- **AP IntegrationUsers** owns the integrations' ceiling and maximum, which
  Block 12 reads; AP UiParts the role editor's part (`src/admin/`).
- Whichever plan lands second in a shared file carries the other's change.

## Analysis

The analysis this plan rests on is
`wip/2026-11-24_14-40_ANALYSIS_StartConfiguration_findings_of_logedges_and_reloadchecks.md`,
its points P1 to P10, read on main at 2121837d and checked again on
3cdb42c1.
What bears on the blocks:

- **The parser** (`internal/env/dotenv.go`): `LoadFile` takes a name's first
  non-empty line, `ReloadFile` its first line, an empty one too;
  `readFile` accepts `=x`, and keeps a byte order mark as part of the first
  name; a UTF-16 file stops at a line past its end.
- **`MASSIMO_ENV_FILE`** (`env.File()`): a line of it sets the variable, and
  every later `File()` names the other file, while the start read the
  default one; a reload then revokes the owner's key (P10).
- **Install's `readEnv`** (`internal/install/install.go:752`) takes a name's
  last line and does not know the byte order mark.
- **`apiReload`** (`cmd/massimo/reload.go`) returns its error before
  `after(ctx)`, the rotation and the checkpoint (P2).
- **The settle rule** (`internal/server/logalerts.go:100`):
  `settings_read` is settled by `settings_read` and `server_started`, but
  with `any` unset, so two warnings in a row both stay alerts (P7).
- **`GET /api/v1/restart`** takes the `restart` capability
  (`internal/server/server.go:1529`); `POST /api/v1/update` takes `update`.
- **The card** counts pending settings only in its head badge
  (`maintenance/store.ts`, `Maintenance.tsx`), and `restart.confirm` and
  `reboot.confirm*` promise a server back in a few seconds (P6).
- **The start's own errors** (`internal/serve/serve.go:331`) name no
  setting (P4); on Windows the update helper refuses an address
  `HealthURL` cannot read before the listen (P5).
- **The guard** (`cmd/massimo/startchecks_test.go`) skips Windows, as the
  built binary is no `.exe`; with D21-A and Block 8 it passes there in all
  33 runs.
- **`PerCubeValue`** (`internal/env/env.go:139`) returns the upper-case name
  on Windows, where `os.Getenv` ignores the case (P8);
  CI's Windows job (`.github/workflows/platforms.yml:83`) does not run
  `./internal/env`, and `ci.yml`'s Windows paths do not name the guard's
  files.
- **The plain Cube** (`internal/serve/setup.go`): a target without a known
  serial gets a listener with no serial, which only a single Cube may have;
  beside a Cube of the store the start stops with "several cubes
  configured" (P9), and a service loops.
- **The SIGHUP test's race** (`cmd/massimo/reload_test.go:809`): it waits for
  the 401 and then reads the pending settings, but `reload()` sets the token
  before `DropKeepOpen` and `SetPendingRestart`; it failed #413's first run.

For v1.1, read on main at 0b42e072:

- **A principal's capabilities** (`internal/server/auth.go`): the role's
  effective set (`store.Effective`: the role's rows less the capabilities
  disabled for every role), then for an integration's user the ceiling and
  the integration's maximum (`intersect`), and once more when an
  integration acts for a user (`onbehalf.go`); `principal.has` reads the
  set, and `GET /api/v1/me` lists it (`integrationCaps`).
  An include taken before those bounds would be cut by them, so it is
  taken on the set they leave.
- **What `restart` admits** (`docs/store.md` §6): `GET` and
  `POST /api/v1/restart`, `POST /api/v1/reload` and
  `POST /api/v1/cubes/{sn}/refresh`.
  Through D37 a holder of `update` or `reboot` gains the reload and a Cube's
  refresh too.
- **The seeded roles:** `update` is the role integration's alone, which
  holds `restart` already, and `reboot` is seeded for no role, so no
  seeded role's set changes; a role the owner made with `update` or
  `reboot` and without `restart` gains it.
- **Block 6's pair rule** (`capOr`, `routeCapabilities`, the two-capability
  cases of `routeCapabilityError` and `capabilityRequired`) has one route,
  `GET /api/v1/restart`; with the include it has none.
- **The role editor** (`src/admin/Roles.tsx`) and the integration's maximum
  (`src/integrations/`) show a switch per capability from the stored set;
  the web UI's `can()` (`shared/me.ts`) reads `GET /api/v1/me`.
- **`install -settings`** (`internal/install/install.go`,
  `checkSettingsFile`): it reads the file leniently and refuses UTF-16
  alone; serve's start refuses also a line that is not `KEY=VALUE` and a
  line too long to read (`env.LoadFile`), so a service installed with such
  a file does not start.

For v1.2, read on main at
`chore(aps): +NOTE CleanRuns v1.0, how the run went`.
The findings, their probes and the options are in
`wip/2026-11-25_18-42_ANALYSIS_StartConfiguration_v1_2_findings.md`.
What bears on the blocks:

- **A `-settings` path that is not there** (item 1): the check exempts
  `os.ErrNotExist` on purpose, and `TestInstallSettingsNotThere` pins it.
  The copy then fails after the binary, the user and the directories are
  written; on Windows after the service is stopped.
- **The start's value checks** (item 2): one table,
  `cmd/massimo/startchecks.go`, which the start runs and the reload runs
  on the changed values; install runs none of it, so `MASSIMO_HSTS=year`
  is copied and the service restarted.
  Two checks read the box, the TLS pair and the DNS-01 hook; the Cubes
  check reads the process environment, not the getter it is handed.
- **A kept file install cannot read** (item 3): `settingsVars` returns
  nil, so TLS counts as off: the health drop-in falls to http, the listen
  drop-in loses `CAP_NET_BIND_SERVICE`, macOS's helper URL becomes http.
- **`install -tls-http :80x`** (item 4) names `-listen`, through
  `splitListen`.
- **A name in another case on Windows** (items 5 and 6): the start
  applies it through `Getenv`; the reloader's `settings()`, the config
  listings and `env.Serials` match the name exactly, and a reload that
  changes value and case keeps the old spelling.
  `SecretVariable` is case-sensitive, so `GET /api/v1/config` lists the
  value of `MASSIMO_API_Token`, the owner's key, to a holder of
  `settings`.
- **The capability tables** (item 7) had drifted from the registered
  routes in both documents; AP CapabilityTable takes them (D55).
- **postinst** (item 8) compares the settings with the template's with a
  UTF-8 byte order mark kept, so a box still on the template is started.
- **The plain Cube's loggers** (item 9) are made while its serial is
  still "", so its client's, queue's and proxy's lines carry an empty
  `cube=` for the whole first run.
- **A hostname against an IP** (item 10): `PlainRow` compares as
  written; a plain `localhost:P` beside a row at `127.0.0.1:P` raced in
  the probe, and one run in three kept a dead second Cube.
- **The test kit** (items 11 and 12): `fakecube.Server.Close` panics on a
  fake that never served; on Windows TestMain removes the test binary's
  directory once and ignores the error.
- **Item 13**, the Software card reading `refused`, was done (#445); only
  D43's comment is left.

## Implementation Plan

A sub-agent on Opus builds each block without a commit, an independent
second reader reads it, and the session verifies it and makes the commit.
The three pull requests are stacked: the second's branch starts from the
first's, the third's from the second's, each with `main` merged in before
it is opened.
The owner merges them, in order.

### Block 1: Lines

- `internal/env`: one parser, which `LoadFile` and `ReloadFile` share.
  - The first line of a name wins, an empty one included (D1-A); a later
    line is kept as a note, with both line numbers.
  - A UTF-8 byte order mark at the start is skipped; a UTF-16 file, either
    byte order, is refused as "<file>: UTF-16 text, save it as UTF-8"
    (D5-A).
  - A line without a name is ignored, with a note (D4-A).
  - A `MASSIMO_ENV_FILE` line is ignored, with a note, and sets nothing
    (D6-A).
  - The notes are returned beside the values; Block 2 tells them.
- `env.Source` of a name given twice: "file, lines 3 and 4: the first wins"
  (D2-A).
- Tests: each rule at a start and at a reload, the reload agreeing with the
  start; a file whose first line is a comment after a byte order mark;
  P10's case, a reload no longer reading the other file.
- Docs: the README's settings-file paragraph; one comment line in
  `packaging/env.template`; a `Fixed` CHANGELOG fragment.

**Verification:** each test fails on a fault planted for it (the reload's
first-line rule as today, the byte order mark kept, `MASSIMO_ENV_FILE`
set); `go test ./internal/env ./cmd/massimo`.

### Block 2: Notes

- The notes of Block 1 told, the same text everywhere (D3-B):
  on stderr at each command's start, as a Warn at a reload that finds one,
  and in `massimo config`.
- `settings_read` at warn carries them as "the settings file: …", after
  "the next start refuses: …" where both stand; one line holds all that the
  read found, so that the newest says everything (Block 5).
- The start writes a `settings_read` warning of its notes after its
  `server_started` (D33-A).
- The card's badge and notice read the refusals only, not the notes
  (`refused` in the API stays refusals).
- Tests: the start's line after `server_started`; a reload with a note and a
  refusal; a reload without either settles it; no secret value in a note.
- Docs: `docs/api-v1.md` (`settings_read`), the README; the fragment amended.

**Verification:** the tests fail without the start's line and with the
two kinds in one text; `go test ./cmd/massimo ./internal/server`.

### Block 3: Install

- `internal/install` reads the settings file with env's parser (D7-A):
  `checkSettings`, `tlsOn` and `MASSIMO_TLS_HTTP` see what `serve` sees;
  `install -settings` refuses a UTF-16 file before it copies it.
- Tests: install's reader on a duplicate, quotes, the byte order mark and
  UTF-16; the existing install tests unchanged.
- The fragment amended.

**Verification:** the tests fail with install's old reader;
`go test ./internal/install`; the Windows build (`GOOS=windows go vet`).

### Block 4: ApiReload

- `apiReload`: a failed read rotates and checkpoints too, then answers 500,
  as SIGHUP does (D8-A).
- `TestServeAppliesTheTokenOnSIGHUP` waits for the reload's end, not for the
  401 alone.
- Tests: the API's failed reload rotates the log; the SIGHUP test under
  `-race` at scale 2, 20 runs.
- Docs: the README's reload paragraph where it names the API; a `Fixed`
  fragment.

**Verification:** the new test fails with today's `apiReload`; the SIGHUP
test fails with a delay planted between `SetToken` and `SetPendingRestart`
before its fix, and passes after; `go test ./cmd/massimo`.

### Block 5: Settle

- `logalerts.go`: `settings_read` with `any: true`, the newest warning
  settling the older ones (D11-A).
- `TestUnsettled` and the alerts tests as the analysis's probe found.
- Docs: `docs/api-v1.md` where it names the alerts' rules; the fragment.

**Verification:** a test of two warnings in a row fails without `any`;
`go test ./internal/server`.

### Block 6: RestartRead

- `GET /api/v1/restart` for a holder of `restart` or of `update` (D34-A):
  the route helper takes either capability, where a route names two.
- Tests: a role with `update` alone reads it, one with neither gets 403;
  `POST /api/v1/restart` still takes `restart` alone.
- Docs: `docs/api-v1.md`, the OpenAPI file, `api.d.ts` (`make api-types`)
  where it names capabilities; a `Changed` fragment.

**Verification:** the test fails with the route on `restart` alone;
`TestOpenAPICoversRoutes`; `go test ./internal/server`.

### Block 7: Card

- The Maintenance card's head: a second badge, a warning, with a fixed text
  ("The next start fails"), shown while `refused` is not empty; the
  pending count stays (D9-A).
- The restart's question and the reboot's gain a sentence while `refused`
  is not empty: the server does not come back until the settings file is
  fixed (D10-C, D35-A).
- Texts in all six `ui/i18n/<lang>.json` (FR/IT/NL/PL machine-translated
  and marked); `docs/ui.md`.
- Tests: the UI test of the card in the four cases, the badge and both
  questions with and without a refusal.
- A `Changed` fragment.

**Verification:** the changed UI test files in the four cases;
`make ui-check`; `go test ./internal/server`.

### Block 8: Names

- The start's own errors name their setting as `-flag or VAR: …`:
  `MASSIMO_LISTEN`, `MASSIMO_LOG`, `MASSIMO_DATA` and `-tls-http`, with the
  "serve: " prefix (D20-A).
- The guard's `says` fragments follow.
- Tests: each error's text; the guard.
- Docs: the README where it quotes one; a `Fixed` fragment.

**Verification:** the guard fails on a text left as it was;
`go test ./cmd/massimo ./internal/serve`.

### Block 9: Helper

- Windows: with `-update-request`, an address `install.HealthURL` cannot
  read no longer stops the start; updates from the API are blocked with the
  reason, as under brew (D21-A).
- Tests: the start goes on with such an address; `POST /api/v1/update`
  answers the reason; the helper's own tests.
- Docs: `docs/update-windows.md`, `docs/api-v1.md` where it names why an
  update is blocked; the fragment.

**Verification:** the tests through interop, with `ProgramData` in a temp
dir and the mtime of the service's `service.log` the same before and after;
`GOOS=windows go vet ./...`.

### Block 10: Windows

- The guard runs on Windows (D22-A): the built binary's `.exe` name, and the
  not-executable case left out there.
- `PerCubeValue` returns the name as it is set: the exact name first, then
  the first match whatever its case (D23-A).
- CI: the Windows job runs `./internal/env/...`; `ci.yml`'s Windows paths
  name the guard's files and `internal/env/`, so that a pull request
  touching them asks for the job.
- Tests: `TestPerCubeValue` passes on both systems; the guard through
  interop.
- A `Fixed` fragment; `make actionlint`.

**Verification:** the guard and `./internal/env` through interop, as in
Block 9; `make actionlint`; the pull request's Windows job runs.

### Block 11: PlainProxy

- Where both `MASSIMO_CUBE_ADDRESS` and `MASSIMO_CUBE_PROXY` are in the
  file, the plain proxy is tied to the plain Cube at once, by that Cube and
  not by a serial, and the start goes on beside the store's Cubes;
  the server takes the serial the Cube reports (D24-B).
- A plain `MASSIMO_CUBE_PROXY` without the plain address: as today, no
  warning.
- Tests, with the fake Cube and a temp store: the plain Cube beside a stored
  one, the proxy serving the plain Cube before and after its serial is
  known; the plain proxy alone as today; a Cube added in the UI after a
  first start that did not reach the plain Cube.
- Docs: the README's single-Cube setup; a `Fixed` fragment.

**Verification:** the tests fail with today's `setup.go`;
`go test ./internal/serve ./internal/server ./cmd/massimo`;
`make lan-test` where the change reaches the proxy's discovery answers.

### Block 12: Include

- `update` and `reboot` each include `restart` (D37, D39-A), and `reboot`
  does not include `update` (D40-A):
  - one table of what a capability includes, beside `capabilities` in
    `auth.go`, taken on a principal's set after every bound (the role's
    effective set, an integration's ceiling and maximum, the acting for a
    user), so that `has`, every route's check and `GET /api/v1/me` agree;
  - a capability disabled for every role stays disabled: `restart` off
    is off for a holder of `update` too;
  - nothing stored changes: no migration, a role's rows as they are.
- `GET /api/v1/restart` is registered with `restart` again; `capOr` and its
  two-capability cases go, since no route has two.
- The role editor and the integration's maximum show `restart` on and
  locked where `update` or `reboot` is on, with a hint that says why;
  what is saved stays the stored set.
- Tests: a role with `update` alone, and one with `reboot` alone, restarts,
  reloads and refreshes a Cube, and reads `GET /api/v1/restart` with
  `available` as for `restart`'s holder; `reboot` alone is refused
  `POST /api/v1/update`; `restart` disabled refuses both; an integration
  whose maximum holds `update` passes `restart`'s check; `GET /api/v1/me`
  lists `restart`; the role editor's switch in the four cases.
- Docs: `docs/store.md` §6's table and a sentence on the include,
  `docs/api-v1.md` where it names `restart`'s holders and the two
  capabilities, the OpenAPI file and `api.d.ts` (`make api-types`),
  `docs/ui.md`, the texts in all six `ui/i18n/<lang>.json`;
  a `Changed` fragment.

**Verification:** the tests fail with the include left out, and with it
taken before the integration's bounds; `TestOpenAPICoversRoutes`;
`go test ./internal/server ./internal/store`; `make ui-check`; the
changed UI test files in the four cases.

### Block 13: SettingsRefused

- `install -settings <file>` reads the file as serve's start does
  (`env.ReadFile`), and refuses what the start refuses, with the parser's
  error, before any write (D38-B): a line that is not `KEY=VALUE`, a line
  too long to read, and UTF-16 as before.
- A settings file a re-install keeps, without `-settings`, is read
  leniently as now, so that an upgrade never fails on it.
- Tests: `install -settings` with each refused file writes nothing, on
  each system, Windows through interop; a kept file with such a line
  still installs, with its warning.
- Docs: the README's `install` paragraph and `packaging/README.md` where
  they say what `-settings` refuses; a `Fixed` fragment.

**Verification:** the tests fail with today's `checkSettingsFile`;
`go test ./internal/install ./cmd/massimo`; `GOOS=windows go vet ./...`.

### Block 14: SettingsThere

- `internal/install`: `checkSettingsFile` no longer lets
  `os.ErrNotExist` through.
  A `-settings` path that is not there is refused with the system's
  error before any write (D41-A), on every entry point: a hand install,
  a packaged box, macOS and Windows, fresh, in a dry run and over a kept
  file (D45-A).
- The comments that say a missing path passes go, in
  `internal/install/install.go` and `internal/env/dotenv.go`.
- Tests:
  - a "not there" case in `refusedFiles()`, so that
    `TestInstallRefusesSettingsServeCannotRead` covers each entry point,
    and the same case in the Windows test;
  - `TestInstallSettingsNotThere` replaced by them;
  - planted fault: the exemption restored.
- Docs: the README's `install` paragraph and `packaging/README.md` where
  they say what `-settings` refuses.
- Fragment `CHANGELOG.d/Fixed/claude-install-settings-there.md`:

  ```text
  - `massimo install -settings` refuses a path that is not there before it
    writes anything, where it first wrote the binary, or stopped the
    service on Windows, and then failed.
  ```

**Verification:** `make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/install ./internal/env`;
`gofmt -l .` empty; `go vet ./...` for linux, windows and darwin;
`internal/install`'s Windows test binary through interop, `ProgramData`
in a temp dir; the tests fail on the exemption, planted through
`-overlay`.

### Block 15: SettingsValues

- `cmd/massimo` hands the run of the start's checks
  (`startchecks.go`'s table) to install in `Options` (D46-A); install
  runs it on the `-settings` file (D42-B).
- The getter is the file's values alone, read with `env.ReadFile`:
  - `-tls` and its flags applied first; in files mode the pair loaded at
    the `-tls-cert` and `-tls-key` sources, so that a mismatched pair is
    refused, where `tls.go` checks only that they exist;
  - the DNS-01 hook checked as root sees it (D48-A);
  - on macOS the plist's fixed `-log-process-format none` applied.
- The Cubes check: `serve.FileCubes`, `env.Serials` and
  `env.PerCubeValue` take the environment as a list (D47-A); the start
  and the reload pass `os.Environ()`, install the file's lines.
- A refusal names every value refused (D52-A), then "nothing was
  written", in a dry run too, before any write on every entry point.
- A kept file, without `-settings`: the same checks as warnings, "the
  next start refuses: …", and the install goes on (D49-A); on Windows it
  is read before the service is stopped (D50-A).
- What install cannot check stays the start's and the reload's refusal:
  whether `_massimo` can read the key or run the hook, and a relative
  path, since the service runs from `/`.
- `install -tls-http :80x` (item 4): `readListen` with
  `-tls-http %q: %w`, so that it says `-tls-http ":80x": bad port`.
- Files: `cmd/massimo/{install,startchecks,reload}.go`,
  `internal/install/{install,tls,windows,darwin}.go`,
  `internal/serve/setup.go`, `internal/env/env.go`.
- Tests, each with its planted fault:
  - each entry point, dry and over a kept file, refuses with nothing
    written; fault: the check run after the writes, or skipped on one
    system;
  - a bad `MASSIMO_HSTS` and a file's bad `_IGNORE` are refused, and the
    process environment is ignored; fault: `FileCubes` reading
    `os.Environ`;
  - the macOS log format passes; fault: the fixed flag ignored;
  - a mismatched `-tls` pair is refused; fault: an existence check only;
  - `-tls off` over a file's `MASSIMO_TLS_HTTP` passes; fault: `-tls` not
    applied;
  - a kept bad file installs with its warning; fault: strict on a kept
    file;
  - two bad values are both named; fault: the first only;
  - the exact `-tls-http ":80x": bad port`; fault: `splitListen` back.
- Docs: the README's `install` paragraph and `packaging/README.md`:
  install refuses what the next start refuses, and what it cannot check.
- Fragment `CHANGELOG.d/Added/claude-install-settings-values.md`:

  ```text
  - `massimo install -settings` refuses every value the next start would
    refuse, before it writes anything, and warns of them in a kept file;
    `-tls-http` names itself in its error, where it named `-listen`.
  ```

**Verification:** `make ui`; `MASSIMO_ENV_FILE=/dev/null go test
./cmd/massimo ./internal/install ./internal/serve ./internal/env`;
`gofmt -l .` empty; `go vet ./...` for linux, windows and darwin; the
Windows test binaries of `internal/install` and `cmd/massimo` through
interop, `ProgramData` in a temp dir; each test fails on its fault,
planted through `-overlay`.

### Block 16: KeptUnread

- Where a kept settings file cannot be read, UTF-16 or a line too long,
  `settingsVars` no longer answers nil as if TLS were off.
  Install keeps the TLS-derived files as they are, and warns that the
  next start refuses the file (D51-A):
  - the health drop-in stays https;
  - the listen drop-in keeps `CAP_NET_BIND_SERVICE` for `-tls-http :80`;
  - on macOS, the helper plist's URL stays https.
- Windows has no drop-ins and is unchanged.
- Files: `internal/install/{tls,listen,darwin}.go`.
- Tests: a hand install, a packaged box and macOS, each over earlier
  https drop-ins and a kept file it cannot read: the files as they were,
  the warning printed.
  Planted fault: the nil fallback.
- Docs: `packaging/README.md` where it says what a re-install keeps.
- Fragment `CHANGELOG.d/Fixed/claude-install-kept-unread.md`:

  ```text
  - `massimo install` over a kept settings file it cannot read keeps the
    https health check and the right to bind port 80, with a warning,
    where it set them back as if TLS were off.
  ```

**Verification:** `make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/install`; `gofmt -l .`
empty; `go vet ./...` for linux, windows and darwin; the tests fail on
the nil fallback, planted through `-overlay`.

### Block 17: WindowsNames

- First, a commit of its own, `fix(security:api)`:
  `server.SecretVariable` checks the name in upper case, so that a
  secret named in another case, `MASSIMO_API_Token`, shows no value in
  `GET /api/v1/config`, on every system.
  It lands first: the listings of the next commit name more variables,
  and would otherwise show more secrets.
  - Tests: `SecretVariable("MASSIMO_API_Token")` is true on every system;
    `GET /api/v1/config` over such a line shows no value.
    Planted fault: the case-sensitive check.
  - Fragment `CHANGELOG.d/Security/claude-secret-name-case.md`:

    ```text
    - Windows: `GET /api/v1/config` hides a secret's value whatever the case
      of its name, where `MASSIMO_API_Token` in the settings file showed the
      owner's key to a holder of `settings`.
    ```

- Then, `fix(server)`: a name is keyed by `env.NameKey`, `nameKey`
  exported, the identity on Linux and the upper case on Windows (D53-A):
  - the reloader's `settings()` (`cmd/massimo/reload.go`) and both
    config listings (`cmd/massimo/config.go`) key names by it, and
    `env.Known` is checked on that key;
  - `env.Serials` reads a prefix in any case on Windows;
  - a reload compares by key and keeps Windows' spelling (D54-A), so that
    a changed value is pending, or refused, whatever its case;
  - on Linux, a known name in another case, which the start ignores, is
    a note on the file, "ignored: names are upper case", told as Block 2's
    notes are.
  - Tests:
    - `cmd/massimo/reload_windows_test.go`, which CI's Windows job runs
      with `./cmd/...`: a change in lower case is pending, a refused value
      is refused, and the probe's case, `massimo_listen=:1` then
      `MASSIMO_LISTEN=:3`, is a change; fault: the exact prefix restored;
    - Windows: `massimo config` lists `massimo_listen` as known; fault:
      `Known` on the name as written;
    - Linux: the note for `massimo_listen`; fault: no note.
  - Docs: the README's settings-file paragraph, where it says names are
    upper case.
  - Fragment `CHANGELOG.d/Fixed/claude-settings-name-case.md`:

    ```text
    - Windows: a setting named in another case, which the start applies, is
      seen by a reload and by `massimo config`; on Linux, where the start
      ignores it, a note says so, where both kept silent.
    ```

**Verification:** for each commit: `make ui`; `MASSIMO_ENV_FILE=/dev/null
go test ./internal/server ./internal/env ./cmd/massimo`; `gofmt -l .`
empty; `go vet ./...` for linux, windows and darwin; the Windows test
binaries of `internal/env`, `internal/server` and `cmd/massimo` through
interop; each test fails on its fault, planted through `-overlay`.

### Block 18: Tables

Carried to AP CapabilityTable (D55), not built here.

### Block 19: PostinstBom

- `packaging/deb/postinst`: `settings()` removes a UTF-8 byte order mark
  at the start of the first line before it compares the file with the
  template's (D56-A), as serve skips it (D5-A).
  A box still on the template's settings then stays stopped until it has
  an owner.
- Test: a case in `packaging/deb/test.sh`, "a template with a BOM: not
  started".
  Planted fault: the strip removed.
- Docs: none; the comment over `settings()` says why.
- Fragment `CHANGELOG.d/Fixed/claude-postinst-bom.md`:

  ```text
  - deb: a settings file still the template's, saved with a UTF-8 byte
    order mark, no longer starts the service at an upgrade before the
    box has an owner.
  ```

- In its pull request, a commit of its own for D43: the comment over
  `readRefused` in `internal/server/ui/src/software/store.ts` says that
  `update` and `reboot` include `restart`, and that the 403 is for a user
  with none of the three.
  No fragment: a comment, which no user sees; its Key changes say so.

**Verification:** `make deb-test`, which fails on the strip removed,
planted in a copy of postinst; for the comment, `make ui` and
`make ui-check`.

### Block 20: PlainLogs

- `internal/slogx`: a handler that reads a Cube's serial on each record,
  from a function, where today the value is taken when the logger is
  made.
- `internal/server/runtimes.go`: the client's and the queue's loggers,
  and the proxy's at its listener's start, read the runtime's
  `cubeSerial()`; a stored Cube's lines carry `cube` once.
- Tests, with the fake Cube:
  - a unit test of the handler; fault: the value taken at construction;
  - after the connection, the plain Cube's client's, queue's and proxy's
    lines carry `cube=KEQ0000100`; fault: today's `partLog(rt.serial)`;
  - a stored Cube's lines carry `cube` exactly once; fault: the key
    twice.
- Docs: none; the README's "`cube=` where it concerns one" holds again.
- Fragment `CHANGELOG.d/Fixed/claude-plain-cube-log-serial.md`:

  ```text
  - The log's lines of the plain Cube's connection, queue and proxy carry
    the serial it reports, where they kept an empty `cube=` until the
    next start.
  ```

**Verification:** `make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/slogx ./internal/server`,
and `-race` for `internal/server`; `gofmt -l .` empty; `go vet ./...` for
linux, windows and darwin; each test fails on its fault, planted through
`-overlay`.

### Block 21: PlainResolve

- `internal/serve/setup.go`: where the plain lines are not tied and no
  row's address matches as written, `PlainRow` resolves both hosts once,
  within 2 s in all, and ties them where a resolved IP and port match
  (D57-A); the tie is saved, `seed.plain_serial`, as today.
- The resolver is injected; an error or a resolver that blocks neither
  ties nor fails the start, which then goes on as today.
- `massimo config` uses the same lookup (D58-A), so that it agrees with
  serve.
- Tests, with a resolver injected and no network, each with its planted
  fault (the port ignored; no timeout; always resolving; resolution
  removed):
  - a name matching the row's IP ties, and so does the reverse;
  - one address of several matching ties;
  - another port does not tie;
  - an error or a blocking resolver neither ties nor fails the start, and
    returns within the bound;
  - the resolver is not called where the Cube is tied or matches as
    written;
  - `setupCubes` returns one Cube, not two;
  - `massimo config` shows the tie.
- Docs: the README's single-Cube setup, where it says when the plain
  lines are a stored Cube's.
- Fragment: Block 11's `CHANGELOG.d/Fixed/claude-plain-proxy-beside-store.md`
  amended:

  ```text
  - serve: the plain `MASSIMO_CUBE_ADDRESS` and `MASSIMO_CUBE_PROXY` beside
    a Cube of the web UI no longer stop the start; at that Cube's address,
    by name or IP, the plain lines are that Cube's, compared with its row.
  ```

**Verification:** `make ui`;
`MASSIMO_ENV_FILE=/dev/null go test ./internal/serve ./cmd/massimo`, and
`-race` for `internal/serve`; `gofmt -l .` empty; `go vet ./...` for
linux, windows and darwin; each test fails on its fault, planted through
`-overlay`.

### Block 22: TestKit

- First, `fix(fakecube)`: `fakecube.Server.Close` on a fake that never
  served, paused or not, returns without a panic: a nil guard over the
  listener.
  - Test: the two cases; planted fault: the guard removed.
  - Fragment `CHANGELOG.d/Fixed/claude-fakecube-close.md`, since
    `massimo fakecube` is shipped:

    ```text
    - The fake Cube no longer panics when it is closed before it served,
      paused or not.
    ```

- Then, `test(cmd)`, for the test binary's directory on Windows:
  - TestMain (`cmd/massimo/update_store_test.go`) removes it with retries,
    and names a directory it could not remove;
  - `builtBinary` (`cmd/massimo/serve_stop_test.go`) first removes the
    `massimo-test-bin-*` directories in the temp directory that are older
    than a day, which only a killed or timed-out run leaves, and keeps a
    younger one, which may be another run's.
  - Tests: an old directory is removed and a fresh one kept; fault: the
    age check dropped. On Windows, a file held open for 300 ms is removed
    after it is closed; fault: a single `RemoveAll`.
  - No fragment: tests alone.

**Verification:** `make ui`; `MASSIMO_ENV_FILE=/dev/null go test
./internal/cubelan/fakecube ./cmd/massimo`; `gofmt -l .` empty; `go vet
./...` for linux, windows and darwin; `cmd/massimo`'s Windows test
binary through interop; each test fails on its fault, planted through
`-overlay`.

### The pull requests

- **First** (Blocks 1–3), **second** (4–7), **third** (8–11), each
  `main` merged in, the guidelines check over its range, its run green.
- The third asks for the Windows job by its files.
- v1.1: **Block 12** and **Block 13**, each a pull request of its own from
  `main`, not stacked; the owner merges them, in either order.
- v1.2: five pull requests, each from `main`, not stacked (D60-A); the
  owner merges them:
  - **Block 17** first, alone, since it carries the security fix:
    `merge(StartConfiguration/Block-17): A name in another case: its secret hidden, then seen by the reload`;
  - **Blocks 14 to 16**, install:
    `merge(StartConfiguration/Blocks-14-16): install refuses what the next start refuses, before any write`;
  - **Block 19**, with D43's comment; Block 18 is carried to
    AP CapabilityTable:
    `merge(StartConfiguration/Block-19): postinst skips a byte order mark before it compares the settings`;
  - **Blocks 20 and 21**, the plain Cube:
    `merge(StartConfiguration/Blocks-20-21): The plain Cube: its serial in its lines, a hostname tied to its row`;
  - **Block 22**, the test kit:
    `merge(StartConfiguration/Block-22): The fake Cube closes before it served; old Windows test binaries swept`.
- Before each: `make ui`, `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `gofmt`, `go vet` for the three systems, `make cross`.

## Proposed commit messages

- The plan: `chore(aps): +StartConfiguration v1.0, +ANALYSIS StartConfiguration`
- Block 1: `fix(env): The first line of a name wins, and the settings file's odd lines are ignored`
- Block 2: `feat(serve): A note on the settings file is told at the start, at a reload and in the log`
- Block 3: `fix(install): massimo install reads the settings file as serve does`
- Block 4: `fix(serve): A failed reload through the API rotates and checkpoints`
- Block 5: `fix(server): The newest settings_read warning settles the older ones`
- Block 6: `fix(api): GET /api/v1/restart is readable with update too`
- Block 7: `feat(ui/maintenance): The card's badge and questions say when the next start fails`
- Block 8: `fix(serve): The start's own errors name their setting`
- Block 9: `fix(update): The Windows update helper's listen blocks updates, not the start`
- Block 10: `fix(env): PerCubeValue names a variable as it is set, and the start's guard runs on Windows`
- Block 11: `fix(serve): A plain Cube's proxy listens beside the store's Cubes`
- v1.1: `chore(aps): +StartConfiguration v1.1, -NOTE StartConfiguration v1.0`
- Block 12: `feat(api): The update and reboot capabilities include restart`
- Block 13: `fix(install): install -settings refuses a file serve refuses, before any write`
- v1.2: `chore(aps): +StartConfiguration v1.2, +ANALYSIS StartConfiguration v1_2_findings`
- Block 14: `fix(install): install -settings refuses a path that is not there, before any write`
- Block 15: `feat(install): install refuses a settings value the next start refuses, each one named`
- Block 16: `fix(install): A kept settings file install cannot read keeps its TLS drop-ins`
- Block 17: `fix(security:api): A secret setting named in another case shows no value in GET /api/v1/config`
- Block 17: `fix(server): Windows: a setting named in another case is seen by the reload and massimo config`
- Block 19: `fix(packaging): postinst skips a byte order mark before it compares the settings with the template`
- Block 19's pull request: `docs(ui/software): The Software card's comment says that update and reboot include restart`
- Block 20: `fix(server): The plain Cube's log lines carry the serial it reports`
- Block 21: `fix(serve): A hostname and an IP of one Cube tie the plain lines to its row`
- Block 22: `fix(fakecube): A fake Cube closed before it served does not panic`
- Block 22: `test(cmd): The Windows test binary's directory is removed with retries, and old ones are swept`

## Change History

- v1.0 (2026-11-25): first revision, by `PLAN++`, from the analysis's points
  P1 to P10 and the SIGHUP test's race, with D1-A, D2-A, D3-B, D33-A,
  D4-A to D9-A, D10-C, D11-A, D20-A to D23-A, D24-B as refined, D30-C,
  D32-A, D34-A and D35-A.
- v1.1 (2026-11-25): Blocks 12 and 13, by `PLAN++`, from D37 with D39-A and
  D40-A, and D38-B; D36-A recorded as built; folds
  "NOTE StartConfiguration v1.0: owner decisions d36 to d38", its file
  removed. Blocks 1 to 11 are built and merged and stand as they are.
- v1.2 (2026-11-25): Blocks 14 to 22, by `PLAN++`, from the owner's
  answers at v1.1's completion gate, "D41A, D42B, D43: AutoUpdate just
  retired. what's the issue?, D44: PLAN", and at the TL;DR gate,
  "D55: check with session "store.md vs api-v1.md"" and `PLAN++`;
  with D41-A, D42-B, D45-A to D54-A and D56-A to D60-A; D43 and D44
  answered; D55 carried to AP CapabilityTable, Block 18 with it, and
  D43's comment moved to Block 19's pull request.
  It rests on `wip/2026-11-25_18-42_ANALYSIS_StartConfiguration_v1_2_findings.md`.
  Blocks 1 to 13 are built and merged and stand as they are.
