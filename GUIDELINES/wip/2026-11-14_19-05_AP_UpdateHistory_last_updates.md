# AP UpdateHistory v1.1: the last updates in Software update

## TL;DR

Software update lists the last three updates: when, the version and its
channel, and who started it, or "outside Massimo" where apt, brew or a
person on the box did. v1.1 puts the date first, adds the channel, and
ends an entry as soon as the helper reports its outcome.

- the store → one settings row, `update.history`, a JSON list of the
  last ten (Blocks 1 to 3, merged as #261)
- an entry → `channel`: the one asked for, or the box's at the start
  that found an update from outside (D98-A)
- a line → "24.11.2026, 15:53 · v0.16.0-alpha.1-596-g8f468fbb (devel) ·
  by martin" (D97-A)
- an update the helper ended without a restart ("apt has nothing newer
  yet") → its outcome at the server's next read, not "under way" until
  the next start

### Blocks

1. HistoryStore: the row, written at the start of an update and at the
   next start; tests (done)
2. HistoryApi: `history` in `GET /api/v1/update`; the OpenAPI file;
   tests (done)
3. HistoryUi: the last three in Software update; i18n; UI test;
   `docs/ui.md` (done)
4. HistoryChannel: the entry's channel, the date first, the helper's
   outcome read at once; tests, docs

Block 4 is one pull request.

## Discussion

The owner's points of 2026-11-14 evening:

- Software update shows the last three updates: what version, when, by
  whom or "auto";
- D95, the owner's form: a JSON list in one row of the settings, filled
  when an update is started and at the start after it;
- D96-A: an update Massimo did not start is "outside Massimo", with the
  package manager where known; Massimo installs no update by itself.

`PLAN+ as recommended` on 2026-11-14.

The owner's points of 2026-11-15 and 2026-11-24, with #261 on a box:

- "move the date before the version"; "can we add the channel used?";
- D97-A (asked as D153): the channel in brackets after the version;
- D98-A (asked as D154): an update from outside carries the channel the
  box follows at the start that found it: apt's suite, brew's channel,
  else the setting or the build's own;
- the defect: after "nothing to install: apt has nothing newer yet" the
  entry stayed "under way", and turned "not reached" only when the next
  update was started; the helper restarted nothing, so no start ended it.

`PLAN++` on 2026-11-24, with AP AutoUpdate v1.0.

## Analysis

**Today.** `logStarted` (`internal/server/systemlog.go`) writes
`server_started` with the version, keeps the version in the setting
`log.server_version`, and adds `before` where it changed;
`logUpdateResult` writes the helper's outcome. `POST /api/v1/update`
(`update_api.go`, audit action `update.start`) starts the helper; the
audit row names the user. The log keeps 30 days by default, so an update
older than that is gone from it.

**The row.** `internal/store` keeps JSON settings (`SetSetting`,
`Setting`). `update.history` is a list of
`{at, by, from, to, reached, reached_at, manager}`: `by` a user's name, or
"" for outside Massimo; `to` the target where Massimo started it;
`reached` the version the next start ran. Ten entries are kept.

**At an update's start** (`update_api.go`, where the helper is started):
an entry with `at`, `by`, `from` (the running version), `to`.

**At a start** (`logStarted`), where the version differs from
`log.server_version`: the newest entry without `reached` gets it, where
its `to` matches or is unknown; otherwise a new entry with `from`,
`reached`, `reached_at` and `manager` (`s.opts.Update`'s manager: apt,
brew or none), `by` empty.

**v1.1.** `noteVersionStart` ends a waiting entry with the helper's
outcome only at a start (`helperEnded`). The Debian helper answers
`up-to-date` without a restart where apt has nothing newer, which happens
while CI's GitHub release is out before the apt index reaches Pages
(AP AutoUpdate's D5 makes the check honest). `updateRelease` reads the
helper's status at every `GET /api/v1/update`; ending the newest waiting
entry there, under `historyMu`, with the same `helperEnded`, gives the
outcome within one read. `startUpdate` knows the channel it asks for
(`v.Channel`); `noteVersionStart` takes `s.updateChannel(ctx)`. The page
is `internal/server/ui/src/software/Software.tsx` since AP UiParts
(`historyLine`).

## Implementation Plan

### Block 1: HistoryStore

- `internal/server/update_history.go`: the entry type, `noteUpdateStart`
  and `noteVersionStart`, the row read and written whole; ten kept.
- `update_api.go` calls `noteUpdateStart`; `logStarted` calls
  `noteVersionStart`.
- Tests: a started update completed by the next start; a start with a
  new version and no entry, "outside" with the manager; an unchanged
  start adds nothing; eleven entries keep ten.

**Verification:** `go test ./internal/server/`, `make race`.

### Block 2: HistoryApi

- `GET /api/v1/update`: `history`, newest first.
- `docs/api-v1.md`, the OpenAPI file; a test.

**Verification:** `go test ./internal/server/` (`TestOpenAPICoversRoutes`).

### Block 3: HistoryUi

- `update.js` `renderUpdate`: under the version lines, "Last updates":
  three lines, the version reached (or the target, still running), when,
  by whom or "outside Massimo (apt)".
- i18n, six languages; a UI test with a stubbed answer.
- `docs/ui.md` "Software update"; a fragment.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

### Block 4: HistoryChannel

- `update_history.go`: `channel` in the entry; `noteUpdateStart` takes
  the channel asked for, `noteVersionStart` the box's for an entry from
  outside; `noteHelperOutcome`, called from `updateRelease`, ends the
  newest waiting entry with the helper's final state newer than it.
- `docs/api-v1.md`, the OpenAPI file: `channel`; the outcome at the next
  read.
- `Software.tsx` `historyLine`: the date first, the version with its
  channel in brackets, then who; i18n where a text is new.
- Tests: the channel both ways; an entry ended by a read after
  `up-to-date`, without a start; the UI line's order and channel.
- `docs/ui.md` "Software update"; the fragment.

**Verification:** `make ui`, `go test ./internal/server/`, `gofmt -l .`,
the three `go vet`, the UI test files touched in their four cases,
`GUIDELINES/shared/tools/check`; the whole suites in CI.

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(server): The server keeps its last updates`;
Block 2, `feat(api): The update's answer lists the last updates`;
Block 3, `feat(ui): Software update shows the last three updates`;
Block 4, `feat(ui/update-history): The last updates show the date first and the channel`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.
- v1.1 (2026-11-24): Block 4, the date first and the channel (D97-A,
  D98-A), and an entry ended at the helper's outcome without a start;
  from the owner's points on #261 on a box.
