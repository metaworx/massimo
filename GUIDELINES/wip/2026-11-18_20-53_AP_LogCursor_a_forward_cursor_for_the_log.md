# AP LogCursor v1.0: a forward cursor for the log

## TL;DR

The log's API pages back only, and `since=` repeats and skips lines;
the log kept without a store has no ids at all.
This plan gives the log a forward cursor, `after=<id>`,
and ids to every line, then moves the tests and the UI's poll onto it.

- `before=` back, `since=` by the millisecond → `after=<id>`, exclusive,
  the oldest `limit` lines after it, oldest first, with every filter
- the ring's lines without ids → ids from one counter per server,
  above the store's and growing across a restart
- a cursor the log never gave out → 409, the client reloads
- 45 test waits polling for one new line → `logMark`, `logAfter`, `logAfterN`
- the UI's All Cubes poll on `since=` → `after=`, no repeats, no gaps

### Blocks

1. Product: `after=` on both log routes, the ring's ids, the 409, its docs
2. Tests: the cursor helpers, the 45 waits moved onto them
3. Ui: the All Cubes poll on `after=`

## Discussion

### What the owner asked for

At AP TestInfrastructure's gate after Block 8, 2026-11-18:
D22, "a forward cursor for the log's API", asked to be explained;
then `PLAN D22B`: "Add `after=<id>` to the log API in a plan of its own,
then move those waits onto it".
Then `PLAN+ LogCursor all A`.

### What is decided

The owner, 2026-11-18, `PLAN+ LogCursor all A`, each as the analysis recommends:

- **D1-A. Ids for the ring.**
  One counter per server, starting at the server's start time in microseconds,
  under one lock; a server line takes one id for every ring.
- **D2-A. What `after` means.**
  Exclusive; the first `limit` lines after it, oldest first;
  with every filter, `before` and `since`; `after=0` is the start.
- **D3-A. A cursor the log never gave out** (beyond its newest id):
  409, with an error saying to reload; the UI reloads, and the test helper fails.
- **D4-A. A cursor older than what the log keeps**
  (the ring's 500 lines, the prune): no signal;
  documented that the page starts at the oldest line kept.
- **D5-A. `since=`** stays, documented as a range's start and not a cursor.
- **D6-A. The UI's poll** moves to `after=<the newest id>`; a 409 reloads the log.
- **D7-A. The stream carrying the log's id:** not in this plan.
- **D8-A. The test helper:** `logMark`, `logAfter` and `logAfterN`,
  an id cursor, polling in the test's own goroutine.
- **D9-A. The 12 waits right after `startServer`** become direct reads,
  on the start's barrier.
- **D10-A. The log's id column:** no migration; D3's 409 catches a cursor beyond
  the newest id.
- **D11-A. The feature flag:** `log_after` "1.0" in `/healthz`.

### Decided in the writing

- **D12-A. Docs go with their block,** the owner's rule for plans:
  the analysis's block of docs alone is folded into Blocks 1 to 3.

## Analysis

The analysis this plan rests on is
`wip/2026-11-18_13-07_ANALYSIS_LogCursor_log_forward_cursor.md`.
What bears on the blocks:

- **Two log routes:** `GET /api/v1/cubes/{sn}/log` and `GET /api/v1/log`
  (`log.go`), `parseLogQuery` with `before`, `since`, `limit`, `source`, `level`,
  `command`, `device`, `kind` and `user`; unknown parameters are ignored, so an
  older server ignores `after=` (D11).
  `GET /api/v1/log/alerts?after=` is the precedent for the name.
- **The store** walks each Cube `ORDER BY id DESC LIMIT ?`;
  one writer gives out ids in commit order and every read flushes it first,
  so its ids already work as a gap-free forward cursor;
  every index carries the rowid, so a forward walk needs no new index;
  `TestEventsPageQuery` checks that no page shape sorts.
- **The ring** (a Cube without a row, a server without a store) keeps 500 lines
  per Cube without ids, answers `before=<id>` with `[]`,
  and merges every Cube's log by time.
  Its ids and its appends must happen under one server-wide lock (D1),
  and `before=<id>` must work on it in the same block, or Older breaks.
- **`since=`** repeats lines of the same millisecond and skips a line stamped
  before the poll's newest but written after it, and its page is the newest
  `limit` lines.
- **The stream's ids** (`<epoch>-<seq>`) are unrelated and stay (D7).
- **Clients:** the web UI's `log.js` (`pollLog` every 10 s on `since=`,
  Older on `before=`); Home Assistant and the Android app do not read the log.
- **The tests:** 45 waits poll the log's API for one new line,
  six of them inside helpers with about 73 callers
  (`settleState`, `waitWatch`, `waitOutcome`, `waitUnconfirmed`,
  `waitSlotsEvents`, `structureEvent`); counting waits stop counting at their
  page's `limit`; 12 come right after `startServer`.

## Implementation Plan

A sub-agent on Opus builds a block without a commit, a second reader reads it,
and the session verifies it and makes the commits; each block is a pull request.

### Block 1: Product

- The store's forward walk (`AfterID`, `id > ?`, `ORDER BY id ASC`), and the
  forward shapes in `TestEventsPageQuery`.
- `after` on both log routes (D2), and the check against the newest id that
  answers 409 (D3, D10).
- The ring's ids from one counter per server under one lock (D1),
  `before=<id>` on the ring, `ringLog` merged by id.
- `log_after` in `/healthz`'s features (D11).
- Docs: `docs/api-v1.md` (both routes, the ring's paragraph, `since` as a range's
  start, the features table), the OpenAPI file (`after`, `id`, the 409),
  `docs/store.md`; a CHANGELOG fragment.
- Tests: forward paging reads every line exactly once through a burst bigger
  than `limit`, in the store and in the ring, for one Cube and for every Cube;
  `after` with filters and with `before`; `after=0`, the 400s, the 409s;
  the ring's ids growing across a restart without a store;
  the two ring tests of `log_api_test.go` rewritten.

**Verification:**
`go test ./...`; the package's tests under `-race`; the new tests each failing on
a fault planted for it (a lock per runtime, the newest `limit` in place of the
oldest).

### Block 2: Tests

- `logcursor_test.go`: `logMark`, `logAfter`, `logAfterN` (D8);
  the 33 waits for a new line and the six helpers moved onto them;
  the 12 right after the start read directly (D9);
  the offset reads (`newLines`, `logKinds`' `n`) as cursors.
- `docs/testing.md`: the log's cursor, beside the watcher.

**Verification:**
the moved tests under `-race` at `MASSIMO_TEST_SCALE=2` 20 times;
no `testkit.Eventually` around a read of the log's API is left.

### Block 3: Ui

- `pollLog` asks `after=<the newest id>`; a 409 calls `loadLog` (D6).
- `tools/ui-test/test_log.py` asserts `after=`; a new test covers the 409;
  `docs/ui.md`.

**Verification:**
the changed UI test files in the four cases.

## Proposed commit messages

- The plan: `chore(aps): +LogCursor v1.0, +ANALYSIS LogCursor`
- Block 1: `feat(api): The log pages forward from a line, after=<id>`
- Block 2: `test(server): A test reads the log forward from a mark`
- Block 3: `feat(ui/log): The log of every Cube polls forward from its newest line`

## Change History

- v1.0 (2026-11-18): first revision, by `PLAN+ LogCursor all A`,
  from the analysis of the owner's D22-B in AP TestInfrastructure,
  with D1-A to D11-A as the analysis recommends and D12-A decided in the writing.
