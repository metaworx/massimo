# AP StoreLog v1.1: the log in the store

## TL;DR

Store stage (d): a Cube's events leave the ring of 500 in memory for the
`events` table of `massimo.db`: written in batches by one goroutine,
paged and filtered through `GET …/log`, pruned daily by `log.keep_days`,
paged in the web UI with "Older" and filtered by kind and device.
Blocks 1 to 3 are done; Blocks 4 to 6 fix what the review of 2026-11-06 found.

- `remember` → `Store.AddEvent`, never blocking; a batch one transaction
- `GET …/log?before=&limit=&kind=&device=` → the page before an `id` or a time
- `log.keep_days` → 30 days, 0 keeps all; pruned at the start, daily,
  on a change
- a page → every row of the Cube sorted (1.3 s at 90,000) → the terms given only,
  migration 0004's indexes in id order (1 to 3 ms)
- a prune → blocking every read → yields to flushes and to a shutdown between chunks
- the writer → stopping with the server's goroutines → after them
- UI → late answers dropped, the device choice per Cube,
  pages kept past 2000 lines, the error shown under a choice
- the CHANGELOG's two incompatible changes, two behaviours documented,
  the tests' gaps

The review: [ANALYSIS StoreLog](2026-11-06_22-38_ANALYSIS_StoreLog_review_stage_d.md);
the decisions D1 to D10 in the Discussion.
Run order 4, 5, 6, each on the branch holding its code, merged forward.

### Blocks

1. StoreEvents: `internal/store/events.go`, the batched writer,
   the paging query, the prune, `log.keep_days`, the size test (done)
2. ServerLog: `remember` to the store, `GET …/log` with `before`,
   `kind` and `device`, the fallback ring, the setting in the API,
   the writer started by `Run`, the API documents (done)
3. UiLog: "Older" and the two selects in `log.js`, six translations,
   the pytest tests, docs/ui.md, docs/store.md section 9 (d) (done)
4. StoreFix: `Events` with the terms given, migration 0004,
   the prune between chunks, `FlushEvents` bounded, on `claude/store-d`
5. ServerFix: the writer's own context, the restore tracked,
   the CHANGELOG's Changed bullet, two behaviours in docs/api-v1.md,
   the ring's tests, on `claude/store-d-2`
6. UiFix: late answers, the Cube switch, the paging room,
   the error line, pytest, docs/ui.md and docs/store.md, on `claude/store-d-3`

## Discussion

The findings below are of 2026-11-06, at `main` 716b6cf;
each decision lists the recommended option first, with its reason,
and what the others cost, so that an answer can be "D2-B".

- **D1.
  The cursor of a page.**
  - A (recommended): the log's events carry `id`, the row's id,
    which the stream's events do not have; `before` is an `id` (digits only,
    `id < ?`) or an RFC 3339 time (`time < ?`),
    as the audit's `before` is a time.
    The UI and the pytest page by `id`, so that no event is skipped or
    repeated whatever the clock does: the fourteen state and config events
    of the fake Cube's connect burst carry the same millisecond (measured),
    and Windows' wall clock is coarser than Linux'.
    `before=<time>` stays the form for a person or a script ("the log before midnight").
  - B: `before` a time only, with the events' time kept at nanosecond
    precision in a layout of its own.
    Exact on Linux and macOS, not on a Windows box, where two events of one
    burst can read the same clock; the test "every event once" could fail there.
  - C: a `next` cursor in the answer.
    It changes the answer's shape (an object instead of the array)
    or needs a `Link` header the page's `api()` does not read:
    every reader of the route changes.
- **D2.
  The order of a page.**
  - A (recommended): oldest first within the page, as `GET …/log` answers today;
    the page is the `limit` events before the cursor, in time order.
    `GET …/log?limit=200` without `before` answers exactly what it answers now,
    so the UI's `loadLog` and the eight Go tests that read the log stay as they are.
  - B: newest first, as the audit lists.
    Every reader of the route today flips its loop, for no gain the UI needs:
    it prepends older pages above the lines it has.
- **D3.
  What a stored event holds.**
  - A (recommended): the event object as the stream sent it,
    less `state.received`, the map of per-value times `view.Device`
    carries for the tiles' ages.
    A state event is 787 bytes compact with it and 390 without (measured);
    the log view reads `state.state` and `changes` only,
    and the event's own `time` is the time that matters in a log.
    docs/api-v1.md says the log's copy leaves it out.
  - B: the object exactly as sent, as docs/store.md section 5 words it.
    Twice the file on the SD card for a map nothing in the log reads.
  - A further cut, leaving `state` out of a `state` event that carries `changes`
    (about 150 bytes), is not taken:
    the first line per device and a client replaying the log rely on
    the device's view at that moment.
- **D4.
  The retention.**
  - A (recommended): `log.keep_days` 30, as the owner took decision 5 of
    docs/store.md section 10 (events 30 days);
    `0` keeps everything, as `commands.keep_days` does; 0 to 3650.
    A house of thirty devices writes about 1.5 MB a day at 0.4 KB an event (3000 events,
    the estimate of section 5), 45 MB at the default.
  - B: 90 days, the coordinating session's suggestion:
    three times the file, 135 MB on the Pi's card at that rate,
    against a decision already on record.
  - C: refuse `0`:
    a box whose owner wants every event has no setting for it,
    where every other `keep_days` has one.
- **D5.
  The ring of 500.**
  - A (recommended): the ring stays only as the fallback of a runtime without a
    store or a row: `serve -addr` opens no store at all (`openServeStore`,
    the start line says `store=none (-addr)`),
    a `-sn` Cube of this run has a row only once discovery found it,
    and a server without a store is what most tests build.
    `getLog` applies `before`, `kind`,
    `device` and `limit` to the ring in memory; its events carry no `id`,
    so the UI pages such a Cube by time.
    The code is the one of today behind `rt.storeID == 0`.
  - B: the ring goes entirely.
    A `-addr` run has no log at all,
    and the seven tests that read the log without a store get one each.
- **D6.
  Where `log.keep_days` is set.**
  - A (recommended): the settings API only, documented in docs/api-v1.md
    beside `commands.keep_days`, which has no panel either.
    Settings → Server does not exist in the page yet;
    it is the panel the UiStructure work left for a plan of its own (AP Tls,
    Discussion), which then adds the four durations, the two retentions
    and the read-only identifiers in one go.
  - B: a minimal Settings → Server panel in Block 3 with every setting of
    `GET /api/v1/settings` as a `.field` row.
    A fourth part to the stage, its own pytest test and screenshots,
    and a panel designed twice if its plan has other ideas.
- **D7.
  Migration 0004.**
  - A (recommended): none.
    The `events` table is in `0001_initial.sql` already (`id`, `uuid`,
    `cube_id`, `time`, `kind`, `device`, `data`) with `events_cube_time
    (cube_id, time)` and `events_device_time (cube_id, device, time)`;
    a page by `id` walks the table from its end,
    a device filter uses the second index,
    and the prune deletes per Cube through the first.
    A kind filter scans a Cube's rows by time until the page is full:
    `state` is most of them, and the rarest kind is a scan of a month's rows,
    tens of milliseconds on a Pi 1 for a filter nobody sets twice a minute.
  - B: `0004_events_indexes.sql`, additive, `min_reader` 1,
    with `events_cube_kind_time (cube_id, kind, time)` and `events_time (time)`.
    Two more entries per row, about 50 bytes,
    a schema version for two indexes,
    and a test fixture of the version for `TestMinReader`'s set.
  - Taken in v1.0: A.
    Wrong on its first point, as the review of 2026-11-06 measured:
    `(? = 0 OR id < ?)` keeps SQLite off the id order,
    so a page took `events_cube_time` and sorted every one of the Cube's rows,
    1.3 s a page at 90,000 events,
    holding the one connection. v1.1 adds migration 0004 after all (D10,
    Block 4); `TestMinReader` builds its migrations itself and needs no fixture.

**Measured on 2026-11-06 at 716b6cf**,
`massimo serve` against the fake Cube of `tools/ui-test/greeting.txt` (seven
devices) in a scratch directory: the connect burst is 17 events,
10.6 KB compact; a `state` event is 672 to 811 bytes (787 on average),
390 without `received`, 340 without `received` and `last_seen`;
a `config` event 703 and 351; `hello`,
`metadata` and `connected` 61 to 65 bytes;
the fourteen `state` and `config` events of the burst are stamped within one millisecond.
A `state` event with `changes` adds about 100 bytes to that.

**Not in this plan:**
a cap of the table by size (decision 5's open half;
the measured figures in docs/store.md let the owner judge,
and a NOTE adds a cap if the card asks for it), Settings → Server (D6),
a log across Cubes, the audit's `before` (it stays a time; its rows are rare),
and the pairing and restore events in the page's stream listeners (AP Pairing).

**Branch and pull requests:**
`claude/store-d` from `origin/main`, merged in first;
one pull request per block,
Block 2's and Block 3's branches stacked on the previous block's (`claude/store-d-2`,
`claude/store-d-3`) as AP Tls and AP Pairing did,
`origin/main` merged into each before its pull request, never rebased.
Nothing is pushed, opened or merged without the owner's signal at a gate,
and auto-merge is never armed.
AP Pairing's Block 3 edits `app.js` and the six `i18n/*.json` too;
whichever lands second merges `origin/main` and resolves the key lists.

**Who runs it:**
the implementing session on `claude/store-d`, under the owner's signals;
the coordinating session registers this plan on `main`.

**Names and choices made here**, routine and not the owner's:
the writer flushes at 100 pending events or one second after the first,
whichever comes first, and before every read of the log and at the stop
(docs/store.md section 3 says "every second or every 100 events");
its buffer holds 4096 events, a full buffer drops the event and counts it,
one warning a minute; a failed transaction drops its batch, one error a minute,
and the next batch tries again;
the prune deletes in chunks of 5000 rows per transaction, per Cube,
so that the WAL and the one connection are never held for long;
`limit` is 1 to 1000 as the audit's, default 200 as today;
`CubeConfig.StoreID` carries the row's id into the runtime;
the store's `Events` returns newest first as `AuditList` does and the server
turns the page round.

### Decisions of v1.1, after the review

The review of 2026-11-06,
[ANALYSIS StoreLog](2026-11-06_22-38_ANALYSIS_StoreLog_review_stage_d.md),
found ten defects in Blocks 1 to 3, each rechecked in the code.
The gate after Block 3 put three decisions to the owner, taken as recommended:

- **D8.
  Which findings.**
  - A (taken): 1 to 8 fixed, 9 documented, the gaps of 10 closed.
    Finding 1 alone makes 30 days of log unusable on a Pi.
  - B: 1, 2 and 7 only; the UI's defects ship.
  - C: none; the slow query ships.
- **D9.
  Where the fixes go.**
  - A (taken): each on the branch that holds its code,
    merged forward into the next,
    so that the first pull request is correct by itself.
  - B: one block on `claude/store-d-3`;
    `main` has the slow query until the third merges.
- **D10.
  The page query.**
  - A (taken): the WHERE built from the terms given, every value bound,
    and migration 0004 with indexes in id order within a Cube,
    measured on 90,000 events in a test.
    Fast with several Cubes, at a few bytes per event.
  - B: `+cube_id`, no migration; slow for a quiet Cube beside a busy one.

Measured before the plan (an overlay test, 100,000 events,
nine in ten of one Cube): with `events_cube (cube_id)`,
`events_cube_kind (cube_id, kind)` and `events_cube_device (cube_id, device)`,
every shape (none, `id`, kind, kind and `id`, device, kind and device,
the other Cube) searches one index without a temporary B-tree,
0.7 to 3 ms a page;
a time bound turned into an id bound first (`max(id)` over the covering
`events_cube_time`) takes 28 to 59 ms, reading the Cube's times up to it.

Live events carry no id, and v1.1 gives them none:
an id handed out before the write would be a second source of ids beside
SQLite's, which the replicated rows (docs/store.md,
decision 7) would have to share.
So the UI's cursor falls back to a time only once every fetched line has
been trimmed (Block 6), as on the ring.

## Analysis

### 1. What is there

- **The ring** (`internal/server/server.go`):
  `runtime.events []event` of `logSize` 500 under `logMu`;
  `remember(ev)` appends,
  `emit(ev)` remembers and sends to the stream's subscribers,
  `recent(n)` copies the last n oldest first;
  `getLog` answers `GET /api/v1/cubes/{sn}/log` (capability `read`) with `recent(limit)`,
  `limit` 200 by default,
  at least 1 and without a maximum (the OpenAPI file says 500).
  Events are born in `pump` from `rt.client.Events()` (`internal/cubelan`),
  hidden devices skipped, with `Time: time.Now()`, the device's `view.DeviceOf`
  as `State` and the `view.Changes` since its last state event;
  `emit` serves discovery (`cube_found`, through `broadcast` to every runtime)
  and the restore (`restore_step`, `restore_done`).
  `internal/engine` carries commands, not events: nothing there changes.
- **The event** (`type event struct`): `Kind`, `Device`, `Error`, `State`,
  `Changes`, `Paired`, `Cube`, `Restore`, `Time`;
  twelve kinds: `connected`, `disconnected`, `hello`, `metadata`, `config`,
  `state`, `pairing_started`, `paired`, `pairing_ended` (`cubelan.EventKind`),
  `cube_found`, `restore_step`, `restore_done` (the server's).
- **The store** (`internal/store`): `Open` with WAL, `synchronous=NORMAL`,
  `busy_timeout` 5000, foreign keys, `_txlock=immediate` and one connection
  (`SetMaxOpenConns(1)`); `timeLayout` in milliseconds, fixed width,
  `formatTime`, `parseTime`, `newUUID`, `writable`;
  `Close` closes the database, which `cmd/massimo/serve.go` defers after
  `srv.Run` returns.
  The `events` table of `0001_initial.sql` (D7) is empty on every box;
  `Info` counts it already (`infoTables`).
  `audit.go` is the pattern of a paged table:
  `AuditList(ctx, before, limit, user)`, newest first, `time < ?`,
  `limit` at most 1000; `getAudit` parses `before` with `time.RFC3339Nano`.
  `commands.go` is the pattern of a retention:
  `CommandsKeepDaysKey`, `settingInt(ctx, tx, key)` and `pruneCommands`
  inside the writing transaction, `0` keeps.
  `rule_test.go` holds that no index covers and no query filters `events.data`;
  `kind`, `device` and `time` are structure.
- **The settings** (`settings.go`, `internal/server/settings_api.go`):
  `SettingInt`, `SetSetting`, `DeleteSetting`;
  `intSettings` with `commands.keep_days` (default 0, 0 to 3650, applies now),
  `durationSettings`; `putSettings` checks every value before writing any;
  `TestSettingsAPI` in `cubes_api_test.go` with `startStored`.
- **The Cube's row**:
  `beginAudit` looks the row up by serial per request;
  `CubeConfig` carries no id; `addCube(row)` has the row,
  `rowTarget(row)` in `serve.go` too;
  a `-addr` run opens no store (`openServeStore`) and its Cube is `Source: flag`,
  the `-sn` Cube of this run has a row only when discovery noted it,
  and the plain-file Cube is seeded at its first `connected` (`seedLearned`).
- **The UI** (`internal/server/ui/`): `log.js` with `logState` (`lines`,
  `filter`, `stick`), `LOG_MAX_LINES` 2000,
  `loadLog` (`GET base() + "/log?limit=200"`, the lines replaced),
  `appendLogEvent` for the stream, `renderLog`,
  `logLineMatches` (the text filter),
  `renderLogLine` from `tpl-log-line` (`partials/log.html`), `showView`,
  `initLog`; `index.html`'s `#logview` with `#log-filter`,
  `#log-clear` and `#log-lines`;
  `app.js` appends `state` events after the connect burst's 1.5 s and `hello`,
  `metadata`, `connected`, `disconnected`, `config`, `cube_found`;
  `api()` returns the parsed body; `applyLanguage` redraws after a change;
  `components.js` has `fillOptions`, `selectInput`, `button`, `fieldRow`;
  `.tile-head` is a flex row without wrap, `.log .tile-head` with a 12 px gap;
  605 keys in `en.json` and `de.json`, `log.kind.*` for eight kinds (`state`,
  `hello`, `metadata`, `connected`, `disconnected`, `config`, `error`,
  `cube_found`).
- **The tests**:
  `start`, `startWith`, `startStored` (with a store), `testStore`,
  `eventually` (5 s), `getJSON`, `do` in `internal/server`;
  seven tests read `/log` without a store (`poll_test.go`, `followup_test.go`,
  `pairing_api_test.go`, two in `restore_api_test.go`,
  two in `discovery_test.go`) and `TestFoundCubeSurvivesRestart` with one;
  `open(t, dir)` and `quiet()` in `internal/store`; `tools/ui-test/`:
  24 pytest tests in four cases (1100 and 390 px,
  en and de) against one server with a store and the seven-device fake Cube,
  `test_log.py` with `test_log_opens` and `test_log_changes`,
  `page.route` stubs as `test_update.py` uses them.
- **The documents**:
  docs/api-v1.md "GET /api/v1/cubes/{sn}/log?limit=200" ("the server keeps the last 500"),
  the OpenAPI file's `limit` (maximum 500), "GET and PUT /api/v1/settings",
  docs/ui.md "Log", docs/store.md sections 3,
  5 (the note on `events`) and 9 (d), packaging/README.md "The store",
  README.md line 418 (the settings route's summary).

### 2. What changes, by file

| File | Block | Change |
|------|------:|--------|
| `internal/store/events.go`, `events_test.go` | 1 | new: `Event`, `EventQuery`, `AddEvent`, `RunEvents`, `FlushEvents`, `Events`, `PruneEvents`, `PruneEventsSoon`, `EventStats`, `LogKeepDaysKey`, `DefaultLogKeepDays` |
| `internal/server/server.go`, `runtimes.go` | 2 | `CubeConfig.StoreID`, `rt.storeID`; `remember` hands to the store or the fallback ring; `getLog` with `before`, `limit`, `kind`, `device`; `event.ID`; the writer started by `Run` |
| `internal/server/log_api_test.go` | 2 | new: paging, filters, the restart, the fallback |
| `internal/server/settings_api.go`, `cubes_api_test.go` | 2 | `log.keep_days` in `intSettings`; a change kicks the prune; `TestSettingsAPI` |
| `cmd/massimo/serve.go` | 2 | `rowTarget` sets `StoreID` |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 2 | the log route's parameters, `id`, the copy's `received`, the retention; `log.keep_days` |
| `packaging/README.md` | 2 | "The store": the log is in it, its size, `store backup` compacts |
| `README.md` | 2 | line 418: the settings route holds the retention too |
| `internal/server/ui/log.js`, `index.html`, `style.css` | 3 | "Older", the kind and device selects, the cursor, the filters on live lines |
| `internal/server/ui/i18n/{en,de,fr,it,nl,pl}.json` | 3 | eleven keys |
| `internal/server/ui_test.go` | 3 | `TestLogKinds`: the page's kinds are the server's |
| `tools/ui-test/test_log.py` | 3 | `test_log_filters`, `test_log_older` |
| `docs/ui.md`, `docs/screenshots/log-*` | 3 | "Log"; the shots if the head changed |
| `docs/store.md` | 3 | section 9 (d) as (c) is written; section 5's note; section 10's status |
| `CHANGELOG.md` | 1, 2, 3 | a bullet per block under Unreleased |

**What stays:**
the stream and its events, the ticket, `emit` and `pump`'s shape,
the hidden devices' silence, the audit and the commands,
`LOG_MAX_LINES` as the cap on live lines, the text filter and Clear,
`TestOpenAPICoversRoutes` (no route is added), the schema version (D7),
and `massimo store info`'s count of `events`, which now says something.

### 3. How the pieces fit

- `Store.AddEvent(e)` puts the event on a buffered channel and returns;
  `RunEvents(ctx)`,
  one goroutine the server starts in `Run` and tracks with its `WaitGroup`,
  drains it into a batch and writes the batch in one `BEGIN IMMEDIATE`
  transaction with one prepared `INSERT` per row (`uuid` from `newUUID`,
  `time` by `formatTime`,
  `data` as given) at 100 pending or one second after the first pending,
  at a `FlushEvents` call, and once more when `ctx` ends;
  `serve` closes the store after `Run` returned,
  so nothing writes after `Close`,
  and an `AddEvent` with no writer running is queued,
  up to the buffer (v1.1: as built; v1.0 said dropped).
  The same goroutine prunes: at its start, every 24 hours,
  and when `PruneEventsSoon` is called, so that `PUT /api/v1/settings` with
  `log.keep_days` applies now, as the API says of a whole-number setting.
- `remember` marshals the event (`received` taken out of `State`,
  D3) and calls `AddEvent` with `rt.storeID`,
  set from the row (`StoreID` in `CubeConfig`,
  `addCube`) or at `connected` by serial after `seedLearned`;
  a runtime with no id keeps the ring of today (D5).
  A DB lookup never happens inside `remember`.
- `getLog` parses `before` (digits: an id; else `time.RFC3339Nano`),
  `limit` (1 to 1000, default 200), `kind` and `device` (a radio address,
  `cube.ParseRFAddress`, else 400), calls `FlushEvents`,
  then `Events(ctx, EventQuery{…})`, decodes each `data` into `event`,
  sets `ID`, and answers the page oldest first (D2);
  the fallback ring is filtered in memory the same way.
- `PruneEvents(ctx, now)` reads `log.keep_days` (`DefaultLogKeepDays` when unset,
  nothing when 0) and, per Cube row,
  runs
  `DELETE FROM events WHERE id IN (SELECT id FROM events WHERE cube_id = ? AND time < ? LIMIT 5000)` until it deletes nothing, one transaction per chunk, and logs the rows removed when there were any.
  No `VACUUM` is ever run by the server: the freed pages are reused by
  the next day's rows, so the file stops growing at the window's size,
  and `massimo store backup` writes a compact copy through `VACUUM INTO`,
  which packaging/README.md says.
  `auto_vacuum` is not set: it only takes effect on a file created with it,
  and an existing `massimo.db` would need a `VACUUM` to switch,
  on an SD card, for nothing.
- The UI's `loadLog` asks `?limit=200&kind=&device=` for the selects' values,
  keeps `cursor` (the oldest line's `id`, else its `time` as received),
  and "Older" asks the same with `before=<cursor>`, prepends the page,
  keeps the scroll where it was,
  and disables itself with "no older events" when a page is short;
  a change of either select reloads; Clear empties the lines and the cursor.
  Live events from the stream pass the selects' filters before they are shown.

### 4. Risks

- The one connection: a chunk of the prune or a batch of the writer holds
  it for milliseconds, during which an API read waits;
  the chunk size and the batch size keep that short.
  v1.1: a log read waited for the whole prune, not for a chunk,
  since the writer answered no flush while it pruned (Block 4);
  and `-race` on `internal/store` and `internal/server` covers the channel,
  the timer and the flush.
- A `state` event's size: 0.4 KB with D3-A, measured on seven devices;
  the owner's thirty on the Pi are the real number,
  which the size test and docs/store.md state as measured, not promised.
- The first start after the update: the table is empty, nothing migrates,
  the ring's content of the old binary was never on disk;
  the log begins with that start's `connected`.
- AP Pairing's Block 3 and Block 3 here edit `i18n/*.json`
  and the page's stream listeners; a merge of `origin/main` before each
  pull request resolves it line by line.
- The audit's `before` keeps its time-only form while the log's takes an id too:
  docs/api-v1.md says so at both routes.
- A kind filter without an index (D7-A) is a scan of a Cube's rows:
  measured only by reasoning here;
  if the Pi shows seconds, D7-B is one additive migration later.
  v1.1: the review measured worse, every page sorted (D7, D10).

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`
and `internal/store`), `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, the network, or the owner's Windows service:
every test runs against the fake Cube and a store in a temporary directory;
serials,
addresses and names in tests and documents are placeholders (`KEQ0000000`,
`0abcde`, `Radiator 1`), no key, token or real device name anywhere.
Each block is one or more functional commits citing the revision it ran under,
`AP StoreLog v1.0, Block 1` to `3` and `AP StoreLog v1.1, Block 4` to `6`,
each with its CHANGELOG bullet, signed with `timeout 60 git commit …`,
and one pull request, stacked as the Discussion says.
The commit headline's component carries the part it changes:
`store.log`, `server.log`, `ui.log`, `ui-test`.

### Block 1: StoreEvents

1. `internal/store/events.go`:
   `LogKeepDaysKey = "log.keep_days"`, `DefaultLogKeepDays = 30`;
   `Event{ID, CubeID int64; Time time.Time; Kind, Device string; Data json.RawMessage}`;
   `AddEvent(e Event)`: `Data` must be valid JSON and `CubeID` set,
   else the event is dropped with a debug line;
   a non-blocking send on a channel of 4096, a full channel counts a drop
   and warns once a minute with the count.
2. `RunEvents(ctx context.Context)`:
   the writer's loop as section 3 of the Analysis describes it,
   with `flushAt` 100 and `flushAfter` one second as package variables the
   tests may lower; each flush one transaction, the batch's rows in order,
   `uuid` by `newUUID`, `time` by `formatTime(e.Time)` (now when zero);
   a failed transaction logs an error once a minute and drops the batch;
   `FlushEvents(ctx) error` sends a request and waits for the writer's answer,
   `nil` at once when the writer is not running;
   a last flush when `ctx` ends, then the loop returns.
   `EventStats() (written, dropped, flushes int64)` for the tests
   and `store info` (v1.1: the tests only; `store info` does not show it).
3. `Events(ctx, q EventQuery) ([]Event, error)` with
   `EventQuery{CubeID, BeforeID int64; Before time.Time; Kind, Device string; Limit int}`:
   `WHERE cube_id = ? AND (? = 0 OR id < ?) AND (? = '' OR time < ?)
   AND (? = '' OR kind = ?) AND (? = '' OR device = ?) ORDER BY id DESC LIMIT ?`,
   `Limit` at most 1000, newest first; `TestNoFilterOnContent` holds it.
4. `PruneEvents(ctx, now time.Time) (int, error)` as section 3 describes,
   reading the setting with `SettingInt` (`DefaultLogKeepDays` when unset,
   nothing when 0), chunks of 5000 per Cube, the total returned;
   `PruneEventsSoon()` wakes the loop,
   which also prunes at its start and every 24 hours;
   the log line `events pruned` with `removed` and `keep_days`.
5. Tests (`events_test.go`):
   a burst of 60 events added within a millisecond and flushed lands as one
   transaction (`flushes == 1`) with every row in order;
   250 events need at most three;
   `FlushEvents` returns after the rows are readable;
   a full channel drops and counts;
   a store opened read-only drops with the error logged;
   paging with `Limit` 7 and `BeforeID` over 500 events of two Cubes,
   several sharing one time,
   returns every event of each Cube once and none of the other;
   `Before` as a time returns the events before it; `Kind` and `Device` filter,
   together too;
   the prune over 40 days of events with 30 kept removes the oldest ten days in
   chunks and nothing with 0, twice in a row removes nothing more;
   `TestEventSize`:
   2000 state events of the measured shape written and checkpointed,
   the file's growth per row logged with `t.Logf` and held under 1 KB,
   the figure written into docs/store.md in Block 3;
   `TestNoIndexOnContent` and `TestNoFilterOnContent` run as they are.
6. The CHANGELOG bullet under Added:
   the store keeps the Cubes' events, written in batches of one transaction
   and pruned daily by `log.keep_days` (30 days; 0 keeps them),
   so that the log survives a restart (docs/store.md section 9).

**Verification:**
the test gate with `-race ./internal/store/...`;
`TestEventSize`'s figure noted for Block 3;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 2: ServerLog

1. `internal/server/server.go`, `runtimes.go`, `cmd/massimo/serve.go`:
   `CubeConfig.StoreID int64` (the row's id, 0 without),
   set by `rowTarget` and `addCube`; `rt.storeID` from it,
   and set at `connected` in `pump` by `Store.Cube(ctx, serial)` after
   `seedLearned` when still 0 (the plain-file Cube);
   `Run` starts `Store.RunEvents(ctx)` as a tracked goroutine when there is a store.
2. `remember`:
   with a store and `rt.storeID != 0`,
   the event marshalled compact without `State.received` (a copy of
   `view.Device` with `Received` nil; `Paired`, `Cube`, `Restore`,
   `Changes` and `Error` as they are) and handed to `AddEvent` with `Kind`,
   `Device` and `Time`; else the ring as today.
   `event` gains `ID int64 \`json:"id,omitempty"\``.
3. `getLog`:
   `before` (digits: an id; else RFC 3339 with `time.RFC3339Nano`,
   400 otherwise), `limit` (1 to 1000, default 200, 400 outside),
   `kind` (any string), `device` (a radio address, 400 otherwise); with a store:
   `FlushEvents`, `Events`, each `data` decoded into `event` with its `ID`,
   the page turned oldest first; without:
   `filterEvents(rt.recent(0), before, kind, device, limit)` over the ring,
   the same shape without `id`.
4. `internal/server/settings_api.go`:
   `{key: store.LogKeepDaysKey, def: store.DefaultLogKeepDays, min: 0, max: 3650}`
   in `intSettings`; a `PUT` that sets or clears it calls `PruneEventsSoon`.
5. Tests (`log_api_test.go`, with `startStored`):
   the 17 events of the connect burst in the log with ids, oldest first,
   `limit=5` the first five;
   paging with `before=<first id>` until an empty answer collects every id once;
   `kind=state` answers the seven devices' states and nothing else;
   `device=<rf>` that device's `config` and `state`;
   `before=<time of the connected event>` the events before it; `before=x`,
   `limit=0`, `limit=1001`, `device=zz` answer 400;
   a `state` event's `state` has no `received`, the stream's has;
   a changed device list (`fc.SetDeviceList` as `poll_test.go` does) shows as a
   `state` event with `changes` within `eventually`;
   the server stopped (`cancel`,
   `Run` returned) and started again on the same store answers the old events
   before the new `connected`; `start` without a store answers the ring,
   filtered, without ids (`TestLogWithoutStore`);
   `TestSettingsAPI` gains `log.keep_days` (default 30, `0` accepted,
   3651 refused, `null` back to 30, `applies` now).
6. Documents: docs/api-v1.md:
   the section "GET /api/v1/cubes/{sn}/log" rewritten (the parameters, `id`,
   the page's order, the copy without `received`, the retention,
   the fallback for a Cube of this run,
   an event in the log within a second of the stream),
   "GET and PUT /api/v1/settings" gains `log.keep_days` beside `commands.keep_days`,
   the audit's section says its `before` is a time only; the OpenAPI file:
   `before`, `kind`, `device`, `limit` maximum 1000;
   packaging/README.md "The store": the log is in `massimo.db`,
   about 0.4 KB an event, the window, `store backup` writes a compact copy;
   README.md line 418: the retention among the settings.
7. The CHANGELOG bullets: under Changed,
   `GET …/log` pages with `before` (an event's `id` or a time),
   `kind` and `device`,
   and keeps the log across restarts instead of the last 500 events (docs/api-v1.md);
   under Added, `log.keep_days` in `/api/v1/settings`.

**Verification:**
the test gate with `-race ./internal/server/... ./internal/store/...`;
`TestOpenAPICoversRoutes`;
a `serve` against the fake Cube in a scratch directory,
stopped and started again, answers yesterday's `connected` before today's;
`curl '…/log?kind=state&limit=3'` three state events oldest first;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 3: UiLog

1. `index.html`'s `#logview`:
   the head gains `<select id="log-kind">` and `<select id="log-device">`,
   each with `aria-label` through `data-i18n-attr` (`log.kindAria`,
   `log.deviceAria`), after the text filter;
   `<button type="button" id="log-older" data-i18n="log.older">` above `#log-lines`;
   `.log .tile-head` wraps (`flex-wrap`) so that the five controls fit 390 px,
   the h2 on the first line.
2. `log.js`:
   `LOG_KINDS`, the twelve kinds in the server's order;
   `fillLogSelects()` fills the kind select with `log.allKinds`
   and `log.kind.<kind>` and the device select with `log.allDevices` and the
   devices of `state.rooms` and `state.unlisted` by name (the address when nameless),
   called from `showView("log")` and from `applyLanguage`; `logState.kind`,
   `logState.device`, `logState.cursor`, `logState.more`;
   `loadLog` sends the selects' values and sets the cursor and `more` (a page
   shorter than 200 ends the paging);
   `loadOlder()` on `#log-older` as section 3 describes,
   the scroll kept by the difference of `scrollHeight`;
   the button disabled with `log.noOlder` when `more` is false;
   a change of a select calls `loadLog`; Clear resets the cursor;
   `logLineMatches` also holds a live line to the kind and the device chosen;
   `logLine` keeps `id` and the raw `time`.
3. Texts: `log.allKinds`, `log.kindAria`, `log.allDevices`, `log.deviceAria`,
   `log.older`, `log.noOlder`, `log.kind.pairing_started`, `log.kind.paired`,
   `log.kind.pairing_ended`, `log.kind.restore_step`, `log.kind.restore_done`
   in `en.json` and `de.json`, machine-translated in `fr`, `it`, `nl`, `pl`
   (which carry `i18n.machineNote`); `TestUITranslations` holds them;
   `TestLogKinds` (`ui_test.go`) reads `LOG_KINDS` from `log.js` and fails
   when it differs from the server's list, kept in `server.go` as `eventKinds`.
4. Tests (`tools/ui-test/test_log.py`):
   `test_log_filters(page, case)`: the log opened, `state` chosen in `#log-kind`
   → every `.log-line` has `kind-state` and there are at least seven;
   the first device of `state.rooms[0]` chosen in `#log-device` → every line's
   `.log-rf` is its address and its kinds are `config` or `state`;
   both back to all → the lines of before; `#log-older` disabled with the
   case's "no older events" text, since the fake Cube's log is one page;
   `named_buttons` on the view.
   `test_log_older(page, case)`: `page.route("**/log?**", …)` answers a page
   of three events with ids 11 to 13 to a request without `before`,
   two with ids 1 and 2 to `before=11`, and `[]` otherwise;
   the log opened → three lines; "Older" → five, the oldest first,
   the scroll unchanged; "Older" again → the button disabled;
   `page.unroute` and the view reloaded at the end.
5. Documents: docs/ui.md "Log" (the kinds and devices to choose from, "Older",
   the server's log survives a restart and is kept for the days `log.keep_days` says);
   `make ui-screenshots` for `log-1100-*` if the head changed; docs/store.md:
   section 9 (d) rewritten as (c) is ("Implemented on <date> (AP StoreLog v1.0)"
   with files, API, UI, tests,
   the measured size per event and the decisions D1 to D7 in a line each),
   section 5's note on `events` (the copy without `received`,
   the default 30 taken, the `id` cursor),
   section 10's status line for stage (d),
   and section 1's "today" left as the history it is.
6. The CHANGELOG bullet under Added:
   the web UI's log pages with "Older" and filters by kind and device,
   and shows the server's log from before a restart (docs/ui.md, "Log").

**Verification:**
the test gate; `make ui-test` at 1100 and 390 px in English and German with
the two new tests and the two of today; the shots looked at;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 4: StoreFix

On `claude/store-d`, after the commit of v1.1 there (D9).

1. `internal/store/migrations/0004_events_pages.sql`, `min_reader` 1,
   additive: `events_cube (cube_id)`, `events_cube_kind (cube_id, kind)`,
   and `events_cube_device (cube_id, device)` in place of `events_device_time`.
   SQLite keeps the rowid last in every index,
   so each is in id order within a Cube, a kind or a device,
   and a page reads its rows from the end without a sort;
   `events_cube_time` stays for the prune and the time bound.
2. `Events`:
   the WHERE holds only the terms given, every value bound.
   A time bound becomes an id bound first, the newest id before that time
   (`max(id)` over `events_cube_time`), the time kept as a filter (`+time < ?`),
   so that a page by time is exact and walks by id too.
3. `RunEvents`:
   the prune hands over between chunks: it answers a pending flush,
   writes the queue, and stops once `ctx` is done,
   the loop's last write following.
   A hook for the tests, `pruneStep`, runs after each chunk.
   `PruneEvents(ctx, now)` keeps its form.
   `FlushEvents` gives up after `eventsQuiet` from its call,
   the handover to the writer included.
4. Tests: `TestEventsPageQuery`,
   the plan of every shape without a temporary B-tree,
   and the time of a page on 90,000 events logged (not under `-short`);
   `TestEventsPruneYields`,
   a flush answered and an event written in the middle of a prune,
   and a done `ctx` ending a prune early; `TestEventsPruned`,
   the second `PruneEventsSoon` leaving one request pending;
   `TestEventSize` measured again with the new indexes.
5. The CHANGELOG's Added bullet of the store's events names migration 0004.

**Verification:**
the test gate with `-race ./internal/store/...`;
the page times and `TestEventSize`'s figure noted for Blocks 5 and 6;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 5: ServerFix

On `claude/store-d-2`, after `claude/store-d` is merged into it.

1. `Run`:
   the writer runs on a context of its own,
   cancelled once `s.wg.Wait()` has returned, and `Run` waits for it,
   so that what the server's goroutines emit as they stop is written.
2. `restore_api.go`:
   the restore's goroutine counted through `s.track` while `Run` runs,
   so that `Run` waits for its `restore_done`.
3. The CHANGELOG's Changed bullet of `GET …/log`:
   `limit` above 1000 answers 400,
   and the log's `state` events carry no `received`.
4. docs/api-v1.md, the log:
   a Cube that gains its row while it runs answers from the store from then on,
   the events it kept in memory left out;
   a device hidden later keeps its events in the log until `log.keep_days`
   removes them;
   a page by time first finds the newest id before it. packaging/README.md:
   the size per event Block 4 measured.
5. Tests: `TestLogWithoutStore` with `device` and a time cursor;
   `TestLogLastEvents`, an event a tracked goroutine emits after the end of
   `Run`'s context is in the store after `Run` returned.

**Verification:**
the test gate with `-race ./internal/server/... ./internal/store/...`;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 6: UiFix

On `claude/store-d-3`, after `claude/store-d-2` is merged into it.

1. `log.js`:
   `logState.gen`, raised by `loadLog`, Clear and a Cube switch;
   an answer of an older generation is dropped, in `loadLog` and in `loadOlder`.
2. `logCubeChanged()`, called by `selectCube`:
   the device choice back to all, the lines, the cursor and the room reset,
   the log loaded again when it is open.
   `loadRooms` fills the selects again while the log is open,
   and loads the log again when the device chosen is gone.
3. `logState.room`:
   how many lines live events may grow the log to,
   `LOG_MAX_LINES` after `loadLog` and Clear,
   and the lines plus `LOG_MAX_LINES` after Older,
   so that a live event never trims a page just fetched.
   The cursor stays the oldest line's `id`;
   its time only once every fetched line has been trimmed (the Discussion).
4. `logLineMatches`:
   a line of kind `error` passes the kind and the device chosen.
5. pytest: `test_log_filters` reads Older's state from the page's size,
   not from the shared server's count;
   `test_log_answers`, new, with the page's `api` stubbed:
   a late answer after Clear dropped, the device choice reset by a Cube switch,
   2200 lines kept after a live event, the error line shown with a kind chosen.
6. docs/ui.md "Log": the room and the time cursor;
   docs/store.md: section 5's indexes (0004), section 9 (d) with D8 to D10
   and the size measured again, section 10's line.

**Verification:**
the test gate; `make ui-test` at 1100 and 390 px in English and German;
`GUIDELINES/shared/tools/check --commit-range` clean.

After each run, the completion gate names the NOTE recording how the run went,
the measured size on the owner's Pi being the owner's to add.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +StoreLog v1.0

Store stage (d), the log in the store (the events written in batches to the events table, paged and filtered through GET …/log, pruned by log.keep_days, paged in the web UI), gets its plan registered before the first project write.

Key changes:
- GUIDELINES/wip/2026-11-06_20-55_AP_StoreLog_log_in_the_store.md: the plan, three blocks, the decisions D1 to D7 for the owner in its Discussion with a recommendation each.

File-revision-v1: GUIDELINES/wip/2026-11-06_20-55_AP_StoreLog_log_in_the_store.md::v1.0::2026-11-06::All::the first revision
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits, each written at its gate, with the part as the component,
and `Assisted-by: claude-code:<model>` of the implementing session as the last trailer:

1. `feat(store.log): the Cubes' events in the store, written in batches and pruned daily`
2. `feat(server.log): the log read from the store, paged and filtered, kept for log.keep_days`
3. `feat(ui.log): the log pages with Older and filters by kind and device`,
   `test(ui-test): the log's filters and paging in the pytest suite`,
   `docs(store): stage (d) implemented, and the decisions it took`

v1.1's commit, of its own on `claude/store-d` before Block 4's first write,
`chore(aps): +StoreLog v1.1, +ANALYSIS StoreLog`, with a `File-revision-v1`
trailer for each file; then, each with a merge of the branch below before it:

4. `fix(store.log): the log's pages walk by id, and a prune yields to reads`
5. `fix(server.log): the writer outlives the server's goroutines, the log's changes listed`
6. `fix(ui.log): late answers, the Cube switch and paging past 2000 lines`

## Change History

- v1.0 (2026-11-06): the first revision, from docs/store.md section 9 (d)
  and the code as of main 716b6cf.
- v1.1 (2026-11-06): Blocks 4 to 6 for the review's findings 1 to 10 (ANALYSIS
  StoreLog), with D8-A, D9-A and D10-A as the owner took them; D7's first point,
  the writer's queue,
  the prune's hold on the connection and `EventStats` corrected where the review
  proved v1.0 wrong.
