# ANALYSIS StoreLog: the review of store stage (d)

A review of `origin/main...claude/store-d-3` (AP StoreLog v1.0,
Blocks 1 to 3) by a subagent that saw the code,
the plan and the project's rules, not the run.
Each finding below was checked again in the code by the session that ran the
plan; the column "Rechecked" says how.

| # | Severity | Where | Finding | Rechecked |
|---|----------|-------|---------|-----------|
| 1 | high | `internal/store/events.go` `Events` | The page query reads and sorts all of a Cube's rows: `(? = 0 OR id < ?)` keeps SQLite from walking by id, so it takes `events_cube_time (cube_id=?)` and a temporary B-tree for `ORDER BY id DESC`. | Probe rerun: 1.3 to 1.4 s a page on 90,000 rows; 1.4 ms with a `(cube_id)` index and the WHERE built from the parameters given. |
| 2 | medium | `events.go` `RunEvents`, `FlushEvents`, `PruneEvents` | A prune runs inside the writer's loop, so a flush waits for the whole prune: the first select of `FlushEvents` has no timeout, and the prune ignores shutdown (`WithoutCancel`). | Code read: confirmed. Reviewer's probe: a flush returned after 4.3 s while 300,000 rows were pruned. |
| 3 | medium-low | `ui/log.js` `loadOlder`, `loadLog`; `app.js` `selectCube` | A late answer lands on the wrong state: Clear, a select change or a Cube switch does not invalidate a request in flight, so a Cube can show another Cube's lines and cursor. | Code read: confirmed, no generation check. |
| 4 | medium-low | `log.js` `fillLogSelects`, `app.js` `selectCube` | A Cube switch keeps the old Cube's device filter and device list: the first page of the new Cube asks for a device it does not have. | Code read: confirmed. |
| 5 | low | `log.js` `appendLogEvent` | Past 2000 lines a live event trims the oldest, which are the pages just fetched with Older; the cursor moves forward, and the next Older fetches the same page. Once only live lines are left, the cursor becomes a time. | Code read: confirmed. |
| 6 | low | `log.js` `logLineMatches`, `loadLog` | The "cannot load" line has kind `error`, so a chosen kind or device hides it and the log is silently empty. | Code read: confirmed. |
| 7 | low | `CHANGELOG.md` [Unreleased] Changed | Two incompatible changes of `GET …/log` are not in the bullet: `limit` above 1000 answers 400, and `state.received` is no longer in the log's events. | Diff read: confirmed; `docs/api-v1.md` states both. |
| 8 | low | `events.go` `RunEvents`, `server.go` `Run` | The writer ends on the same context as the producers, so events emitted while they wind down (a last `disconnected`, a `restore_done` of a restore stopped by the shutdown) stay in the queue. | Plausible, from the code. |
| 9 | low | `server/log.go` `getLog` | A runtime that learns its row mid-run drops the ring's earlier events from its log; a device hidden later keeps its events in the log for `log.keep_days`. | Code read: confirmed; to document. |
| 10 | low | tests | `TestLogWithoutStore` leaves the ring's `device` and time cursor untested; `test_log.py` assumes the shared server keeps under 200 events; the coalescing part of `TestEventsPruned` asserts nothing. | Code read: confirmed. |

## Where the code and the plan disagree

- D7 says a page by id walks the table from its end; it does not (finding 1).
- The risks say a prune chunk holds the connection for milliseconds;
  a log read waits for the whole prune (finding 2).
- The NOTE says `FlushEvents` gives up after ten seconds;
  that holds only once the writer has taken the request (finding 2).
- `EventStats` is for the tests and `store info`; only the tests call it.

## Found sound

Paging by id (strict `<`, no loop), `before` and `limit` parsing,
Cube isolation, parameterised SQL, the capabilities (`read` for the log,
`settings` for `log.keep_days`), the bounded queue, a second writer refused,
a read-only store, serve waiting for `Run`, the prune's cutoff and cascade,
the stored JSON decoding back, `storedEvent` stripping a copy only,
no `innerHTML` and no inline handlers, the eleven keys in six languages,
Older ignoring a double click,
the API document and the OpenAPI file against the code, the measured size.
`go test -race` of `internal/store` and `internal/server`,
ten times for the log and event tests, passed.
