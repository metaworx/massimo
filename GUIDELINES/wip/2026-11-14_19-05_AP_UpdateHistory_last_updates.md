# AP UpdateHistory v1.0: the last updates in Software update

## TL;DR

Software update lists the last three updates: the version, when, and who
started it, or "outside Massimo" where apt, brew or a person on the box
did.

- the store → one settings row, `update.history`, a JSON list of the
  last ten
- an update started in Massimo → an entry: who, the version before, the
  target, when
- the next start with another version → the entry completed: the version
  reached and when; no entry waiting → a new one, "outside Massimo",
  with apt or brew where the box has one
- `GET /api/v1/update` → `history`, newest first
- Software update → the last three

### Blocks

1. HistoryStore: the row, written at the start of an update and at the
   next start; tests
2. HistoryApi: `history` in `GET /api/v1/update`; the OpenAPI file;
   tests
3. HistoryUi: the last three in Software update; i18n; UI test;
   `docs/ui.md`

The three blocks are one pull request.

## Discussion

The owner's points of 2026-11-14 evening:

- Software update shows the last three updates: what version, when, by
  whom or "auto";
- D95, the owner's form: a JSON list in one row of the settings, filled
  when an update is started and at the start after it;
- D96-A: an update Massimo did not start is "outside Massimo", with the
  package manager where known; Massimo installs no update by itself.

`PLAN+ as recommended` on 2026-11-14.

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

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(server): The server keeps its last updates`;
Block 2, `feat(api): The update's answer lists the last updates`;
Block 3, `feat(ui): Software update shows the last three updates`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.
