# ANALYSIS StartConfiguration: the findings AP LogEdges and AP ReloadChecks left

Read on main at 2121837d, read only, by two Opus sub-agents of the session
that ran both plans; their probes ran through `go test -overlay` from the
session's scratch directory, and on Windows through interop with
`ProgramData` in a temp dir (the service's `service.log` unchanged before
and after each of seven runs). Neither probe is kept.
The points are AP ReloadChecks' NOTE, "Found, not changed", 1 to 9 (9 as two),
and AP LogEdges' NOTE, items 3 and 4.

## TL;DR

Twelve points, two of them worse than noted, one not a problem at all:

- **P10**: a `MASSIMO_ENV_FILE` line in `.env` → a reload reads another file
  → the owner key revoked, every request 401 until a restart
  (a hand-run `serve` without `-env` only)
- **P3**: a byte order mark (PowerShell 5.1) → the first setting lost
  silently, or every command stops at a line that is fine
- **P6**: the restart's question says "offline for a few seconds" while the
  next start refuses the file; a folded card says nothing
- **P9**: a plain Cube's address and proxy beside a Cube of the store → every
  restart stops, and a service loops
- **P11**: `audit.id` is no cursor at all → nothing to fix; but the audit's
  paging loses rows (D26)
- the rest: small, a line to a few each

### Plans proposed (D30-A)

1. **SettingsLines** (the settings file's parser): P1, P3, P10, install's reader
2. **RefusedSigns** (what a refusal shows and does): P6, P7, P2
3. **StartErrors** (the start's refusals on every system): P4, P5, P8
4. **LooseEnds** (three stand-alone fixes): P9, the audit's paging, P12

## The settings file's parser

**P1. A name given twice is never reloaded.** Confirmed, with three rules:
- the start takes the first non-empty line (`internal/env/dotenv.go:112-117`),
  a later one only setting the source to "environment, overrides <file>";
- a reload takes the first line, an empty one too (`:138-144`), its comment
  saying "as at a start", which is wrong;
- `massimo install`'s own reader takes the last line
  (`internal/install/install.go:752-774`), for `checkSettings`, `tlsOn` and
  `MASSIMO_TLS_HTTP`.

Probe: `KEEP=3` then `KEEP=4` gives 3, with the wrong source in config;
a reload of `KEEP=5` alone changes nothing.
`E=` then `E=5` gives 5 at the start, and the first reload of the same file
lists E as pending.
No doc mentions duplicates. Severity low.

**P3. A line `=x`, and a byte order mark.** Confirmed.
- `=x` is accepted, `Setenv("")` fails unseen, and every reload reports an
  empty name changed: the card says "Read again:  changed." Cosmetic.
- A UTF-8 byte order mark before a setting makes it `﻿MASSIMO_…`:
  the real one is missing, and config, which lists only `MASSIMO_` names,
  shows nothing. Before a comment without `=`, as the template's first line
  is, every command stops with `<file>:1: expected KEY=VALUE`.
- UTF-16LE, as PowerShell 5.1's `>` writes it, stops the start at a line past
  the file's end.
- PowerShell 5.1 writes both, and `install -settings` copies the file as it is.
  Severity medium, on Windows.

**P10. A `MASSIMO_ENV_FILE` line redirects the server.** Confirmed, and
harmful. Without `-env` and the variable, the line sets the variable, and
`env.File()` hands the other file to the reloader, `SettingsFile` and config,
while the start read the default one.
Probe: `./.env` naming `other.env`, `serve` without `-env`:
- the first Reload now answered 200, `changed` holding the token;
- the store logged "api token revoked", and every request answered 401 until a
  restart;
- pending and refused were computed against the other file.

Every packaged service passes `-env`, so a service is immune. Severity medium:
rare, but a lockout.

**Install's reader** (found beside P1) disagrees with `serve` on duplicates,
quotes and the byte order mark.

## What a refusal shows and does

**P6. The card's badge and the questions.** Confirmed.
- The head badge counts pending settings only (`store.ts:207-209`,
  `Maintenance.tsx:231-236`), so a refusal of the file, of `MASSIMO_REBOOT`
  or of `MASSIMO_CUBE_SN` adds nothing to a folded card.
- The restart's question (`restart.confirm`) promises "offline for a few
  seconds", which is false while a refusal stands; the reboot's and the
  update's questions say nothing either. The notice under the row does say it.
- Size: about 6 lines in the part, two keys in all six i18n files, two
  sentences in `docs/ui.md`, the UI test in the four cases. Severity medium.

**P7. Two `settings_read` warnings in a row both stay alerts.** Confirmed, and
deliberate in LogEdges (`logalerts_test.go:59-64`).
With `any: true` on the rule only the newest stays, which holds everything the
next start refuses.
Probe: `TestUnsettled` changes as expected, every other alerts and reload
test passes. A seen mark on a superseded line becomes moot, which is harmless.
Severity low.

**P2. A failed reload through the API neither rotates nor checkpoints.**
Confirmed, and older than ReloadChecks: `apiReload` returns before `after`,
while SIGHUP always rotates and checkpoints, and the README says the two do the
same. Probe end to end with `-log`: the API's 500 without "log rotated", SIGHUP
with it.
Windows has no SIGHUP, so Reload now is its only way. About 3 lines.
Severity low.

## The start's refusals on every system

**P4. A `MASSIMO_LISTEN` that does not parse** prints
`error: listen tcp: address 99999: invalid port`, with no "serve: " and no
variable (`internal/serve/serve.go:331-333`).
`MASSIMO_LOG`, `MASSIMO_DATA` and `-tls-http` lack the variable too.
The fix is `serve: -listen or MASSIMO_LISTEN: …`, with the guard's `says`
fragment changed to match; probed on Linux and Windows. Severity low.

**The Windows update helper** (found beside P4): with `-update-request`,
`install.HealthURL` refuses before the listen any address it cannot read,
port 0 and service names included, with a text of its own.

**P5. The guard on Windows.** With the `.exe` and the not-executable case
left out, 32 of 33 runs pass through interop. The run with odd values fails on
the update helper's refusal of `-listen 127.0.0.1:0`. With D21-A and P4's
wrap, all 33 pass, in 2–3 s.
- The first runs were slow, 11–18 s per start, probably the virus scanner on
  new binaries.
- CI's Windows job runs `./cmd/...` without `-short`, so it would run the
  guard, at under a minute more, on main, tags and nightly. On a pull request
  only where What to test answers windows=true: the guard's files are not
  among the Windows paths.

**P8. `TestPerCubeValue` on Windows.** Confirmed through interop.
`os.Getenv` finds the name whatever its case, and `PerCubeValue` returns the
upper-case name where the file says `keq…`.
The value is right. The name is not, where config, the "settings file
differs" warning, the API's file differences and an `_IGNORE` error show it.
The fix: scan the environment, the exact name first, then the first match
whatever its case. Probed on both systems.
CI misses it: the Windows job does not run `./internal/env`.

## Three stand-alone fixes

**P9. A plain Cube's address and proxy beside a Cube of the store** stops the
start: "several cubes configured; name the cube of each proxy listener"
(`internal/serve/setup.go:95-106`, `:160`).
Probed with the fake Cube and a temp store:
- **The docs' path works:** the plain Cube reports its serial first.
- **A plausible path stops every start:** the plain Cube is not reached at
  the first start, then the owner adds a Cube in the UI. A service then
  loops, and the error names a flag the service does not give.

The start should go on, serving the plain Cube without its plain proxy until
its serial is known, with a warning naming `MASSIMO_CUBE_<SN>_PROXY`.
About 5 lines.

**P11. `audit.id` without `AUTOINCREMENT`:** confirmed, but harmless.
`before=` is a time, not an id (`docs/api-v1.md:1348`), and nothing keys on
the ids: no foreign key, no UI, and the app's test harness only within one
test.
**But the audit's paging loses rows:**
- the page filters by time and orders by id (`internal/store/audit.go:212`);
- a row of refused requests carries its minute's first time but is written at
  the minute's end (`internal/server/audit.go:599`);
- so a page ending on one loses the rows written in that minute. In the probe,
  one of three was never listed.

Rows of the same millisecond at a page's end are lost too.
Only an API client that pages the audit sees it.

**P12. The fixed 200 ms in `resend_test.go`:** an end exists. The re-send's
decision comes in the same step as the report, and the follow-up reports
writes in order. A second proxy write's report is therefore the end to wait
for, and the room's day is checked after it.
Probe: the sleep misses a re-send queued 300 ms late (10 of 10), the end
catches it (10 of 10), and the test drops from 0.6 to 0.4 s.

## Where it meets other plans

- **Packaging** owns `packaging/env.template` (P1's comment line).
- **LogAlerts** owns the settle table (P7); **RadioHealth** names
  `logalerts.go`.
- **RefreshBatteries** (merged, not retired) touched `Maintenance.tsx`, and
  **UiParts** owns the parts' rules (P6).
- **RoomMajority** (#410) changes `drift_test.go`, whose rig P12 uses; only
  the merge order matters.
- **MobileApp's** `Host.kt` reads audit ids within one test (P11).

## Open decisions

The numbers are the investigators': D1 to D11, then D20 to D27; D30 is the
grouping.

**D1. Which line of a name given twice wins.**
- **A (recommended):** the first, at the start and at a reload, an empty one
  included; it is what the start does today for every non-empty duplicate.
- **B:** the last, as a shell and systemd do; costs a silent change of value
  at the update for every file with a duplicate.

**D2. What config says of a name given twice.**
- **A (recommended):** its source says "file, lines 3 and 4: the first wins";
  it reaches System Info with no UI change.
- **B:** "file" alone.

**D3. Where a note on the file is warned of** (a duplicate, a line without a
name, a `MASSIMO_ENV_FILE` line).
- **A (recommended):** on stderr at each command's start, as a Warn at a
  reload that finds one, and in config; not in `settings_read`, whose warning
  means "the next start refuses".
- **B:** also `settings_read` at warn and the sign; costs that warning a
  second meaning.
- **C:** in config only.

**D4. A line without a name.**
- **A (recommended):** ignored, with a note (D3); no start that works today is
  refused.
- **B:** a parse error; costs a box with such a line staying down after the
  update.

**D5. A byte order mark and UTF-16.**
- **A (recommended):** a UTF-8 byte order mark skipped at the file's start;
  a UTF-16 file refused as "UTF-16 text, save it as UTF-8".
- **B:** the byte order mark skipped only; UTF-16 keeps its confusing error.

**D6. A `MASSIMO_ENV_FILE` line in the settings file.**
- **A (recommended):** ignored, with a note (D3); the start already reads only
  the default file, which then stays the one file read.
- **B:** `serve` and config keep the file the start read; the line still sets
  the variable for children.
- **C:** refused at the start; costs a box with such a line staying down.

**D7. Install's own reader.**
- **A (recommended):** `internal/install` uses env's parser, so that install and
  `serve` agree; costs install's tests.
- **B:** leave it.

**D8. A failed reload through the API.**
- **A (recommended):** rotate and checkpoint whatever the read found, then
  answer 500, as SIGHUP does and the README says.
- **B:** the docs say the API does both only after a good read.

**D9. The card's head while the next start refuses.**
- **A (recommended):** a second badge, a warning, with a fixed text such as
  "The next start fails"; the pending count stays.
- **B:** the refusals counted in the pending badge.
- **C:** none.

**D10. The questions while the next start refuses.**
- **A (recommended):** one sentence added to the restart's question and the
  reboot's, both of which leave the server down.
- **B:** the restart's only.
- **C:** also the update's; costs the Software card reading the restart's row.

**D11. `settings_read` warnings in a row.**
- **A (recommended):** `any: true`, the newest settling the older ones.
- **B:** as it is.

**D20. How far the start's own errors name their setting.**
- **A (recommended):** all four, `MASSIMO_LISTEN`, `MASSIMO_LOG`,
  `MASSIMO_DATA` and `-tls-http`, as `-flag or VAR: …`.
- **B:** `MASSIMO_LISTEN` alone.

**D21. The Windows update helper's listen.**
- **A (recommended):** an address `HealthURL` cannot read blocks updates from the
  API, with the reason, instead of stopping the start, as under brew.
- **B:** keep the refusal, and give the guard a free port on Windows.

**D22. The guard on Windows.**
- **A (recommended):** run it, the pull request asking for the Windows job.
- **B:** keep the skip.

**D23. `PerCubeValue` on Windows.**
- **A (recommended):** the name as it is set, and the Windows job runs
  `./internal/env`.
- **B:** the test accepts the upper-case name.

**D24. A plain Cube's lines beside the store's Cubes.**
- **A (recommended):** the start goes on, the plain proxy waiting for the serial,
  with a warning.
- **B:** tie the plain listener to the plain Cube at once; costs changes to the
  server's proxy choice.
- **C:** a clearer refusal; the service still loops.

**D25. `audit.id`.**
- **A (recommended):** a line in `docs/store.md` section 11.
- **B:** the log's fix.

**D26. The audit's paging.**
- **A (recommended):** a page ordered by time, then id; same-millisecond ties
  stay.
- **B:** a line in `docs/api-v1.md` only.

**D27. The 200 ms in `resend_test.go`.**
- **A (recommended):** the later report as the end, the room's day checked
  after it.
- **B:** keep it.

**D30. The grouping.**
- **A (recommended):** four plans: SettingsLines, RefusedSigns, StartErrors,
  and LooseEnds. LooseEnds is three stand-alone fixes under one name, an
  exception to "a new subject is a new plan" (AGENTS.md §5.1) that saves
  three gates of one block each.
- **B:** six plans, P9, the audit and P12 each a plan of its own.
- **C:** one plan with a block per point; costs one plan of four subjects.
