# AP StartConfiguration v1.0: The settings file and the start

## TL;DR

The settings file, the reload and the start disagree in small ways that
cost a lockout, a silent loss of a setting or a looping service.
This plan gives them one parser and one set of rules, shows what the next
start refuses before a restart, and makes the start's refusals the same on
every system.

- a name given twice → the first line wins everywhere, config names both lines
- `=x`, a `MASSIMO_ENV_FILE` line → ignored, with a note
- UTF-8 byte order mark → skipped; UTF-16 → refused, "save it as UTF-8"
- a note on the file → stderr, the reload's Warn, config and `settings_read`
- install's own reader → env's parser
- a failed reload through the API → rotates and checkpoints, as SIGHUP does
- the Maintenance card → a badge and questions that say the next start fails
- `GET /api/v1/restart` → `restart` or `update`
- the start's own errors → name their setting; the guard runs on Windows
- a plain Cube's address and proxy beside the store's Cubes → served,
  not a stopped start

### Blocks

1. Lines: one parser, the first line wins; byte order mark, UTF-16, `=x`, `MASSIMO_ENV_FILE`
2. Notes: a note on the file on stderr, at a reload, in config and `settings_read`
3. Install: `massimo install` reads the file with env's parser
4. ApiReload: a failed API reload rotates and checkpoints; the SIGHUP test waits for the reload's end
5. Settle: the newest `settings_read` warning settles the older ones
6. RestartRead: `GET /api/v1/restart` with `restart` or `update`
7. Card: the Maintenance card's badge and questions while the next start refuses
8. Names: the start's own errors name their setting
9. Helper: the Windows update helper's listen blocks updates, not the start
10. Windows: the guard and `PerCubeValue` on Windows, in CI's Windows job
11. PlainProxy: a plain Cube's proxy listens for it beside the store's Cubes

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

### Where it meets other plans

- **AP Packaging** owns `packaging/env.template`; Block 1 adds one comment
  line to it.
- **AP LogAlerts** owns the settle table, which Block 5 changes;
  AP RadioHealth names `logalerts.go`.
- **AP UiParts** owns the parts' rules; Block 7 follows them in
  `src/maintenance/`.
- **AP UpdateHistory** takes the update's question (D35-A), and reads
  `GET /api/v1/restart` as Block 6 allows.
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

### The pull requests

- **First** (Blocks 1–3), **second** (4–7), **third** (8–11), each
  `main` merged in, the guidelines check over its range, its run green.
- The third asks for the Windows job by its files.
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

## Change History

- v1.0 (2026-11-25): first revision, by `PLAN++`, from the analysis's points
  P1 to P10 and the SIGHUP test's race, with D1-A, D2-A, D3-B, D33-A,
  D4-A to D9-A, D10-C, D11-A, D20-A to D23-A, D24-B as refined, D30-C,
  D32-A, D34-A and D35-A.
