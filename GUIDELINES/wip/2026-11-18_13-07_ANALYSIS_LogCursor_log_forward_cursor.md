# ANALYSIS LogCursor: a forward cursor for the log

Owner's decision D22-B (2026-11-18): the log's API gains `after=<id>`,
and the log kept without a store gains ids,
so a client pages forward without gaps or repeats;
then one test helper replaces the server tests that poll `GET …/log` for one new line.
Background: NOTE TestInfrastructure v1.1 (Block 3), item 3.
Read at 8d5ce975 (branch claude/testkit-two-tests), read only,
by a sub-agent of the session running AP TestInfrastructure.

## TL;DR

- **Paging today goes back only.**
  Two log routes, `GET /api/v1/cubes/{sn}/log` and `GET /api/v1/log`.
  - `before=` takes an id (in the store) or a time.
  - `since=` takes a time cut to the millisecond,
    and its page is the newest `limit` lines:
    a burst bigger than `limit` loses its middle,
    a line stamped earlier but written later is skipped,
    and lines in the same millisecond repeat.
  - The ring has no ids and answers `[]` to `before=<id>`.
    The ring is the memory log: a server without a store,
    or a Cube without a row.
- **The store's ids already work as a gap-free forward cursor:**
  one writer gives out ids in commit order,
  and every read flushes the writer first.
  Missing: a forward walk (`id > ? ORDER BY id ASC`) and ids for the ring.
- **The stream's ids are unrelated and stay** (`<epoch>-<seq>` per runtime).
- **Recommended:**
  `after=` exclusive, the oldest `limit` lines after the cursor, oldest first,
  with every filter; the ring's ids from one counter per server,
  seeded with the start time in microseconds, handed out under one lock;
  a cursor the log never gave out gets `409`; `since=` stays,
  as the start of a time range; `/healthz` gains the feature `log_after`;
  the web UI's All Cubes poll moves to `after=`;
  the tests get `logMark` and `logAfter`.
- **45 waits move:**
  44 `testkit.Eventually` and 1 `testkit.Within`, in 17 files;
  six of them sit in helpers with about 73 call sites;
  about 12 come straight after `startServer`, whose barrier already holds.
- **Four blocks:**
  product, docs, the tests' move, the UI.

## 1 The log's API today

**Routes** (server.go:1485-1487)

- `GET /api/v1/cubes/{sn}/log` (read), `getLog` (log.go:199-226):
  the store for a Cube with a row (log.go:216), the ring otherwise (log.go:224).
- `GET /api/v1/log` (read), `getAllLog` (log.go:235-264):
  with a store, the rows of Cubes that have one (:249-254);
  without, `ringLog` (:269-286): the rings merged by time (:284),
  the server's lines taken from the first ring.
- `GET /api/v1/log/alerts?after=` (read), logalerts.go:376-404:
  `after` exclusive (`a.ID > after`, :393), an integer of 0 or more (:378-383).
  The precedent for the name and its meaning.
- No other log route.
  `GET /api/v1/audit?before=` pages its own table: out of scope.

**Parameters**:
`parseLogQuery`, log.go:111-175.

- `before`:
  an id of 1 or more, or an RFC 3339 time (:114-125).
- `since`:
  an RFC 3339 time, not after a `before` time (:126-135).
- `limit`:
  1 to 1000, 200 by default (:136-142, :45-47).
- `source`:
  one of 7 sources (:143-148, :86).
- `level`:
  info by default (:149-155).
- `command` (:156-162); `device`: a serial or a radio address (:163-173);
  `kind`; `user`, resolved to a row (`logUser` :290-305).
- Unknown parameters are ignored: an older server silently ignores `after=`.

**Store** (store/events.go)

- `Events` (:560-595): one walk per Cube plus one for the server's lines (cube_id NULL),
  two per Cube for a user filter.
  - Each walk is `ORDER BY id DESC LIMIT ?` (`eventsQuery` :658-709).
  - `BeforeID` is `id < ?` (:660-662).
  - `Before` and `Since` become id bounds by a subquery,
    `+time` kept as a filter (:663-672).
  - `mergeWalks` merges by id, newest first (:599-619).
- `storedLog` flushes the writer first (log.go:314) and turns the rows round to
  oldest first (:327).
- Table `log` (migrations/store.sql:181-195):
  `id INTEGER PRIMARY KEY`, without AUTOINCREMENT;
  `time` TEXT at the millisecond (store.go:42 `timeLayout` `.000Z`, :293).
- Indexes (:197-213): `log_cube`, `log_cube_kind`, `log_cube_serial`,
  `log_cube_source`, `log_cube_time`, `log_cube_warn` (partial, level >= 4),
  `log_command`, `log_user`, `log_real_user`; every one carries the rowid,
  so a forward walk in id order needs no new index.
- One writer goroutine writes batches (:6-19, :42-51); `AddEvent` never blocks,
  and drops an event when its buffer is full (:194-199).
- The prune deletes the oldest lines by time (:482).
- Nothing runs `DELETE FROM cubes`, so the ON DELETE CASCADE never fires.
- `TestEventsPageQuery` (events_test.go:349-460) checks that every page shape is planned
  without `TEMP B-TREE` or `SCAN log`.

**Why `since=` repeats and skips**

- It repeats: the column is cut to the millisecond while the line's JSON time keeps
  nanoseconds, so `since=<a line's time>` takes that whole millisecond again;
  the UI removes the duplicates.
- It skips:
  1. the page is the newest `limit` lines at or after the time (`DESC LIMIT`);
  2. an event's `Time` is stamped when the event is made,
     its id when it is written:
     a line stamped before the poll's newest time but written after the poll is
     never asked for again;
  3. the ring is merged by time across Cubes,
     so a Cube that lags behind loses lines the same way.

**Ring**

- `runtime.events` (server.go:497-498), held under `rt.logMu`.
- `remember` (server.go:687-698) hands an event to the store for a Cube with a row;
  otherwise it appends to the ring, capped at `logSize` = 500 per Cube (:683).
- `logServer` (:724-738) writes a server line once to the store and once into every
  row-less Cube's ring.
- `filterEvents` (log.go:355-375) answers `beforeID != 0` with `[]` (:357-359)
  and keeps the last `limit` lines (:371-373).
- `event.ID` (server.go:593-595):
  "the stream's events and the ring's have none".
- A Cube that gains its row while running (`learnStoreID`,
  log.go:73-83) switches to the store,
  and its ring's lines leave the log (api-v1.md:3252-3254).

**Stream**

- Ids are `<epoch>-<seq>` (resume.go:15-26, :78-81):
  the epoch set at each runtime's start (runtimes.go:113),
  the sequence counted per runtime (`publishLocked`, resume.go:108-113).
- The stream also counts events the log never keeps:
  virtual wall thermostat events (vwt.go:271, :510; etag.go:283)
  and a user's notifications (push_api.go:422-427).
- The server's own line gets a different number on each Cube's stream.
- The replay ring holds 1024 events (resume.go:29);
  an unknown id gets a `resync` and the state burst (server.go:2126-2130).
- The store gives a line its id at INSERT,
  up to a second after the stream sent it,
  so the stream cannot carry the log id; the docs say so (api-v1.md:3202).
- **No id is shared between the stream and the log.**

## 2 Clients

**Web UI, `internal/server/ui/log.js`**

- The per-Cube view loads a page with `loadLog` (:1019-1052) and adds live
  stream lines (`appendLogEvent` :972-977);
  it drops a live line the page already has by time,
  kind and device (:1046-1048).
- Older pages go back with `before=<the oldest line's id>`,
  or its time where it has no id (`logCursor` :935-939, `loadOlder` :1057-1086).
- The time ranges send `since=<the range's start>` (`logQuery` :951-963).
- Under All Cubes the stream adds nothing (:973);
  `pollLog` (:1100-1118) runs every `LOG_POLL_MS` = 10 s (:39,
  `logPollSet` :1122-1129),
  asks `since=<the newest line's time>` with `limit` = `LOG_PAGE` = 200 (:19),
  and drops a line it already has by id, or without ids by time, kind,
  device and Cube (`logSameLine` :1090-1093).
- The alerts use `GET /api/v1/log/alerts?after=` (:1391);
  the alert mark is looked up by line id (:836).
- `loadLog` is also called from pages.js:181 and app.js:1049.
- UI tests: tools/ui-test/test_log.py:316-319 asserts that the poll asks
  `since=<the newest time>`;
  test_log.py:741 and :766 assert `before=<id>` for Older;
  test_parts_guard.py:531-533 and :662 name `pollLog()`.
- **Catch:**
  once ring lines carry ids, `logCursor` takes the id,
  so `before=<id>` must work on the ring; today it answers `[]`.

**Docs**

- docs/api-v1.md: :3183-3259, a Cube's log;
  it already says that a time cannot page a burst "without gaps or repeats"
  (:3241-3243); the ring described as having no id, `before` taking a time,
  `since` working (:3247-3251). :3261-3282, every Cube's log;
  without a store merged by time, without ids (:3281). :3284,
  the alerts' `after=`. :3549-3577, the stream's ids and resume. :352-372,
  the features table (`event_ids`).
- docs/api-v1-openapi.yaml: :2965-3035 and :3036-3081, the two log routes;
  `id` documented as "absent for a Cube without a row" (:2999, :3070); :3097,
  the alerts' `after` (integer, minimum 0); the log routes document no 409.
- docs/store.md:1753-1790, stage (d); the API bullet :1769-1774.
- `TestOpenAPICoversRoutes` (openapi_test.go:59-134) compares only routes and
  `x-capability`, and that the YAML parses and its references resolve:
  a new query parameter needs no change there.

**Other clients**

- Home Assistant (massimo-homeassistant at d78bfd1) never reads the log;
  it uses the stream with `Last-Event-ID` (api.py:333-342, coordinator.py:189,
  :323-328).
- `cmd/massimo` has no `log` subcommand;
  only serve_stop_windows_test.go:96 polls the log,
  checking `ID != 0` against a store.
- `app/`
  (Android) makes no log calls.

## 3 The tests that would move (internal/server, 45 waits)

- **S**:
  straight after `startServer`; its barrier (start_test.go:216-222:
  the Cube connected, `pumpSync`, the store flushed) already holds,
  so a direct read is enough; to be confirmed per test.
- **A**:
  a new line after an action; moves to `logAfter`.

| Wait | Calls | What it counts | Class |
|---|---|---|---|
| `settleState` errorwatch_test.go:43-55 (wait :50) | 25 | state lines above a count; `stateLines` reads `limit=1000` (:22-33) and **stops counting at 1000** | A |
| `waitWatch` errorwatch_test.go:73-85 (:76) | 10 | `watchLines` up to n, `limit=1000` | A |
| `waitOutcome` resend_test.go:49-60 (:52) | 20 | `r.events` (drift_test.go:103), `limit=200` | A |
| `waitUnconfirmed` confirm_test.go:74-82 (:77) | 7 | `unconfirmedEvents`, `limit=500` (:61-71) | A |
| `waitSlotsEvents` slots_test.go:105-112 (:108) | 8 | `c.events`, `limit=500` (:77) | A |
| `structureEvent` structure_test.go:32-45 (:35) | 3 | `limit=500` | A |

The 39 inline waits: autoreset_test.go:54, :81; battery_test.go:35;
config_api_test.go:615, :637; confirm_test.go:367; congruence_test.go:541;
errorwatch_test.go:132, :281; failedradiogram_test.go:115, :137;
log_api_test.go:52 S, :149, :181 S, :194 S (after a restart), :241 S, :363,
:460, :484, :531 S, :602 S; logdetail_test.go:168 S, :175, :215 S, :223, :243 S,
:280 S, :286, :306; logserial_test.go:52 S, :101, :196; logsources_test.go:65 S;
poll_test.go:45, :73; resend_test.go:183; systemlog_test.go:194;
wallreset_test.go:125; `testkit.Within` at systemlog_test.go:108.
That makes 12 S and 33 A, the A count with the six helpers.

- **Count waits stop working past a page limit:**
  a wait that counts lines stops counting at its page's `limit` (500 or 1000);
  the cursor removes that.
- **Offset reads a cursor would simplify:**
  `newLines(…, len(before))` in logdetail_test.go:172-178,
  and the `n` of `logKinds` in config_api_test.go:533-545.
- **Block 1 changes these tests:**
  `TestLogWithoutStore` (log_api_test.go:238-301) checks that ring lines have no
  id (:245-249) and that `before=5` answers `[]` (:254-256);
  `TestLogOfEveryCubeWithoutStore` (:599-627) checks the merge is by time
  (:610-612).

## 4 Design

**`after=<id>`**

- An event's id, exclusive, an integer of 0 or more, as for `/log/alerts`;
  anything else 400.
- The page is the first `limit` matching lines with id > `after`, oldest first;
  it combines with every filter;
  `before` (an id or a time) and `since` add further bounds;
  `after=0` starts at the oldest line kept.
- A client continues from the last id of a page:
  a full page means more may follow;
  an empty page leaves the cursor where it was.
- In the store: `EventQuery` gains `AfterID` and a forward flag;
  `eventsQuery` adds `id > ?` and `ORDER BY id ASC`;
  `mergeWalks` merges oldest first, and `storedLog` skips the turn-round;
  `TestEventsPageQuery` gains forward shapes (plain, with a kind,
  at warn and above, with the server's lines), none of which may sort.
- In the ring: `filterEvents` keeps id > `after` and takes the first `limit`;
  `before=<id>` keeps id < `before` and takes the last `limit`;
  `ringLog` orders by id.
- **Why there are no gaps:**
  in the store, one writer gives out ids in commit order and each read flushes
  first, so any line missing from a page gets an id higher than the page's last;
  in the ring, the id and the append must happen under **one server-wide lock**,
  and `ringLog` must copy the rings under it: with a lock per runtime,
  `GET /api/v1/log` could see Cube B's id 11 before Cube A's id 10.

**Ids for the ring**

- One counter per `Server`,
  starting at `time.Now().UnixMicro()` when the server is made:
  ids grow across a restart,
  so an old cursor reads the new run from its start without repeats;
  they stay below 2^53, which JavaScript reads exactly, until the year 2255;
  they sit far above the store's ids,
  so a ring id never matches an alert's store id (log.js:836).
- A server line takes one id for every ring.

**Restarts and store changes**

- Without a store, a run's ids are higher than the run before;
  a clock set back (a Pi without a clock chip that starts before NTP) gives lower ids:
  the old cursor is beyond the newest and gets 409.
- With a store, ids survive a restart; a store restored from an older copy,
  a downgrade,
  or a prune that emptied the table after `keep_days` of silence leave a cursor
  beyond `max(id)`, which gets 409 while it stays beyond.
- A Cube that gains its row: its ring cursor is beyond the store's ids,
  so it gets 409 and the client reloads;
  the docs already say the ring's lines leave the log.

**`since=`** stays as the start of a time range:
the UI's ranges use it (log.js:953-955),
and so does the alerts' settle scan inside the server (logalerts.go:263);
the docs say it is not a cursor.

**Docs**

- api-v1.md, both routes: `after`; the ring has ids;
  `before=<id>` works on the ring; the 409; `since` is a range start,
  not a cursor; the features table: `log_after`.
- The OpenAPI: `after` on both routes; the `id` description; a 409 response.
- store.md (d); docs/testing.md: the log's cursor, in the watcher section.
- `TestOpenAPICoversRoutes` needs no change;
  optionally it could also compare the log routes' documented parameters with
  `parseLogQuery`'s keys.

**UI**

- `pollLog` asks `after=<the newest id>`; a 409 calls `loadLog()`;
  `logSameLine` stays as a guard.
- It gains: a burst of more than 200 lines within 10 s no longer loses its
  middle;
  a line stamped before the newest line's time but written after it is no longer
  skipped; lines in the same millisecond no longer repeat;
  the ring's merge by time goes away.
- The per-Cube view stays on the stream: its live lines have no id.

**Test helper**, in the watcher's style:

    // logCursor reads a Cube's log forward from a mark (AP LogCursor).
    type logCursor struct{ t *testing.T; ts *httptest.Server; path, token string; at int64; seen []event }
    func logMark(t *testing.T, ts *httptest.Server) *logCursor   // the newest id now: level=debug&limit=1; the read flushes the store
    func logAfter(t *testing.T, c *logCursor, what string, match func(event) bool, opts ...testkit.Option) event
    func logAfterN(t *testing.T, c *logCursor, what string, n int, match func(event) bool, opts ...testkit.Option) []event

- Each tick reads `?after=<at>&level=debug&limit=1000` and moves `at` past every
  line read, so each line is read once; it keeps those lines,
  and a failure shows the last 20 since the mark,
  as `Watch` does (testkit/watch.go:41-44, :429-461).
- Nothing logged before the mark can match,
  which avoids the trap in docs/testing.md:92-97.
- For example, `settleState` becomes: mark, `SetState`,
  then `logAfter(stateLine(rf, key, to))`.

## 5 Risks

1. **Ordering:**
   the ring needs one server-wide lock around both the id and the append;
   a lock per runtime leaves gaps between Cubes.
2. **`limit` cutting a page:**
   `after` must return the oldest `limit` lines;
   the newest would lose a burst's middle.
3. **Reused store ids:**
   without AUTOINCREMENT,
   a prune that empties the table or a restored store gives ids out again;
   a cursor still beyond the newest id gets 409,
   but a reuse that has already passed the cursor goes unnoticed.
4. **Older servers** ignore `after=` and answer the newest page:
   clients need the feature flag.
5. **Older on a ring Cube:**
   once ring lines carry ids,
   `before=<id>` must work on the ring in the same block, or Older breaks.
6. **Lost lines:**
   a cursor older than the ring's 500 lines or the prune loses lines without a
   signal (D4).
7. **Order change:**
   without a store, the merge of every Cube's log changes from time order to id
   order, which shows slightly differently.

## 6 Decisions (recommended option first)

1. **Ids for the ring.**
   A) One counter per server starting at the start time in microseconds,
   under one lock; a server line gets one id in every ring.
   B) The store gives out ids itself:
   kept in memory from `max(id)` when the store opens, the INSERT naming the id,
   a row-less Cube's ring drawing from the same counter; one id space,
   and the stream could carry the log id; more work in the writer.
   C) A counter from 1 at each start:
   ids collide across restarts and with the store's.
2. **What `after` means.**
   A) Exclusive; the first `limit` lines after it, oldest first;
   with every filter, `before` and `since`; `after=0` is the start.
   B) Like A, but 400 with `before` or `since`.
   C) The newest `limit` lines after it: the gap `since` has today.
3. **A cursor the log never gave out** (beyond its newest id).
   A) 409 with an error saying to reload; the UI reloads and the helper fails.
   B) 200 with a `resync` object first, as the stream does.
   C) An empty page, documented.
4. **A cursor older than what the log keeps** (the ring's 500 lines, the prune).
   A) No signal; documented that the page starts at the oldest line kept.
   B) A response header saying lines were dropped.
   C) 409, as in D3.
5. **`since=`.**
   A) Stays, documented as a range start and not a cursor.
   B) Deprecated.
   C) Removed, which breaks the UI's ranges.
6. **The UI's poll.**
   A) Moves to `after=<the newest id>`, a 409 reloads the log.
   B) Stays on `since`.
7. **The stream carrying the log id.**
   A) Not in this plan.
   B) Yes: it needs D1-B for Cubes with a row,
   and would let a client fill a gap after a `resync`.
8. **The test helper.**
   A) `logMark`, `logAfter` and `logAfterN` with an id cursor,
   polling in the test's own goroutine.
   B) A generic `testkit.PollWatch` feeder plus an `apiLogWatch`;
   it needs a catch-up read at each mark.
   C) Stay on `Eventually`.
9. **The 12 S waits.**
   A) Become direct reads, on startServer's barrier.
   B) Move to `logAfter` from 0.
10. **The log's id column.**
    A) No migration; D3 catches the cursors beyond the newest id.
    B) A migration making `log.id` AUTOINCREMENT,
    as 0013 and 0014 did for backups and commands; it copies the whole log once.
11. **The feature flag.**
    A) `log_after` "1.0" in `/healthz`; no store needed.
    B) None.

## 7 Blocks (one pull request each)

1. **Product.**
   The store's forward walk, and the forward shapes in the query-plan test;
   `after` on both routes, and the check against `max(id)` that answers 409;
   the ring's ids under one lock, `before=<id>` on the ring, `ringLog` by id;
   `log_after` in the features.
   Tests: forward paging reads every line exactly once through a burst bigger
   than `limit`, in the store and the ring, for one Cube and for every Cube;
   `after` with filters and in a window with `before`; `after=0`,
   the 400s and the 409s; the ring's ids grow across a restart without a store.
   The two ring tests in log_api_test.go rewritten;
   a CHANGELOG.d/Added fragment.
2. **Docs.**
   api-v1.md (both routes, the ring paragraph, the features table),
   the OpenAPI (`after`, `id`, the 409), store.md (d);
   optionally the parameter check in openapi_test.go.
3. **The tests' move.**
   A new `logcursor_test.go` with the helper;
   the 33 A waits and the six helpers move to it;
   the 12 S waits become direct reads (D9); the offset reads become cursors;
   docs/testing.md gains the cursor.
   Verified as in Block 3: `-race` at scale 2, the moved tests run 20 times.
4. **UI.**
   `pollLog` on `after=`; a 409 calls `loadLog`;
   test_log.py:316-319 asserts `after=<the newest id>`,
   and a new UI test covers the 409.
