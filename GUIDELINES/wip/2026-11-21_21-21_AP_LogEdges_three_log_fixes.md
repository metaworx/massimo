# AP LogEdges v1.0: Three log fixes

## TL;DR

Three of the five findings that AP LogCursor left need fixing:
the log can give out an id twice, a typo in the settings file is taken without
a word, and one test can pass when it should fail.
Each fix is a few lines; the three are one pull request, a commit each.

- the prune that empties the log → ids again from 1, alerts hidden for months
  → the prune keeps the log's newest row, no id given out twice, no migration
- `MASSIMO_LOG_LEVEL=deubg` in a settings file read again → taken silently,
  the next start fails → a Warn line at the reload
- `TestNewerWriteReplaces`' fixed 300 ms → reads on to the re-send's report,
  then finds no second warning
- a Cube's log after a resync → fixed by UiParts Block 11 (#396), not here
- a Pi without a clock → harmless; a sentence in `docs/store.md`

### Blocks

1. Store: the prune keeps the log's newest row, and the clock's sentence
2. Reload: a log level or format that does not parse is warned of
3. Test: `TestNewerWriteReplaces` reads on to the re-send's report

## Discussion

### What the owner asked for

On 2026-11-21, after AP LogCursor's last pull request merged:
`PLAN the five open findings` (LogCursor's NOTE, "Found, not changed");
then, at the TL;DR gate, `get feedback from "UI refactor"`, whose answer
is in the analysis's second revision; then `PLAN++`.

### What is decided

The owner, 2026-11-21, `PLAN++` on the gate offering every option as recommended:

- **D1-A. A Cube's log after a stream resync:** left to UiParts Block 11,
  which fixed it on main (#396, UiParts.D208) before this plan was written.
- **D2-A. `store.ParseLevel` and the API's `level=`:** no change;
  the API already answers 400 to an unknown level.
- **D3-A. A log level or format in the settings file read again that does not
  parse:** a Warn line at the reload; the value still waits for the next start.
- **D4-A. `TestNewerWriteReplaces`:** the log cursor, read to the re-send's report
  (`resend_not_confirmed`), then no second `not_confirmed`.
- **D5-A. Ids reused after a prune to empty:** the prune keeps the log's newest
  row; it reverses LogCursor.D10-A, which left the reuse documented.
- **D6-A. The unit and the clock:** no unit change;
  a sentence in `docs/store.md`.
- **D7-A. An epoch per run for log cursors:** none.
- **D8-A. Where the work goes:** this plan, its three blocks in one pull request,
  a commit each.

## Analysis

The analysis this plan rests on is
`wip/2026-11-21_16-45_ANALYSIS_LogEdges_five_log_findings_v2.md`.
What bears on the blocks:

- **The prune** (`internal/store/events.go:491`) is the only thing that deletes
  log rows, per Cube and then the server's, in chunks:
  `DELETE FROM log WHERE id IN (SELECT id FROM log WHERE <cube> AND time < ? LIMIT ?)`.
  `log.id` is `INTEGER PRIMARY KEY` without AUTOINCREMENT,
  so SQLite gives out max(id)+1, and a log emptied gives out 1 again.
  It happens where the newest line is older than `log.keep_days`:
  at the start after the box was off that long (the start prune runs before the
  first batch), or on a server that logged nothing that long.
- **What a reuse does:** `log.seen.all` keeps the old high id, so the alerts at or
  below it stay hidden; the room-master mismatch counts a reused id as seen and
  warns again every minute; the log's part keys a line by its Cube's serial and
  its id.
- **The 409** (`store.ErrBeyondNewest`) loses one case with Block 1,
  the log emptied by the prune;
  it keeps a store restored from an older copy, a Cube that gains its row,
  and a server without a store restarted with its clock set back.
  The log's part relies on the 409 in three places and needs no change.
- **The reload** (`cmd/massimo/reload.go:130-158`) takes a changed
  `MASSIMO_LOG_LEVEL` as waiting for the next start, with an Info line,
  `at_next_restart=MASSIMO_LOG_LEVEL`; the start refuses a level or a log format
  that does not parse (`cmd/massimo/logging.go:30-42`, `:115-123`).
- **The test** (`internal/server/confirm_test.go:169-190`) sleeps 300 ms and
  expects one `not_confirmed` line; a re-send's report mislabelled
  `not_confirmed` comes about 600 ms after the first and passes it.
- **UiParts** changes the log's part (`internal/server/ui/src/log/`);
  its Block 15, not started, removes `log.js` and touches `src/log/`:
  whoever lands second carries the other's change.

## Implementation Plan

A sub-agent on Opus builds each block without a commit,
a second reader reads it, and the session verifies it and makes the commit;
the three commits are one pull request (D8-A).

### Block 1: Store

- `internal/store/events.go`: the prune's walk keeps the log's newest row,
  `AND id < (SELECT max(id) FROM log)`, so max(id) never falls (D5-A);
  its comment, and the one on the ids' reuse.
- Comments: `internal/server/log.go` on the ids;
  `internal/server/ui/src/log/store.ts`, `logGone`'s cases of the 409
  lose "emptied by the prune".
- Tests, beside `TestEventsPruned`: a prune of every line keeps the newest and
  the next line's id is above it; a later prune removes it once a newer line is
  there; a start after `keep_days` gives no id twice.
- Docs: `docs/store.md` on the log's ids (LogCursor.D10-A reversed) and on the
  prune, and the ring's ids behind a clock set back (D6-A);
  `docs/api-v1.md` on the 409's cases; a `Fixed` CHANGELOG fragment.

**Verification:**
`go test ./internal/store ./internal/server` and under `-race`;
each new test fails with the old walk planted back;
`make ui-check` for `store.ts`; `gofmt`, `go vet` for the three systems.

### Block 2: Reload

- `cmd/massimo/reload.go`: where a reload finds `MASSIMO_LOG_LEVEL`
  or a log format that does not parse, a Warn line naming the setting and why,
  beside the Info line; the value waits for the next start as before (D3-A).
- A test in `cmd/massimo/reload_test.go` with a captured log.
- Docs: the README's paragraph on reading the settings file again;
  a `Fixed` CHANGELOG fragment.

**Verification:**
`go test ./cmd/massimo`; the test fails without the warning;
`gofmt`, `go vet` for the three systems.

### Block 3: Test

- `internal/server/confirm_test.go`, `TestNewerWriteReplaces`:
  the fixed sleep goes; it reads on to the re-send's report,
  then the log holds no second `not_confirmed` (D4-A).
- No docs; no CHANGELOG fragment, only a test changes.

**Verification:**
the test fails on each of three planted faults (the re-send's report as
`not_confirmed`, the replacement off, the late flag off);
20 runs under `-race` at `MASSIMO_TEST_SCALE=2`.

### The pull request

The three commits on one branch, `main` merged in before it;
the guidelines check over the range; the pull request's run green.
The owner merges it.

## Proposed commit messages

- The plan: `chore(aps): +LogEdges v1.0, +ANALYSIS LogEdges`
- Block 1: `fix(store): The log's prune keeps its newest line, so no id is given out twice`
- Block 2: `fix(serve): A settings file read again warns of a log level that does not parse`
- Block 3: `test(server): The newer write's test reads on to the re-send's report`

## Change History

- v1.0 (2026-11-21): first revision, by `PLAN++`, from the five findings of
  AP LogCursor's NOTE, with D1-A to D8-A as the analysis recommends.
