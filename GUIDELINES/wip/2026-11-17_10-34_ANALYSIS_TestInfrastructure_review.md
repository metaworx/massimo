# ANALYSIS TestInfrastructure: review of the test infrastructure (2026-11-17)

Read from a plain export of `main` (no `.git`);
history from `origin/main` in the worktree, read-only.
Nothing was run.
Paths and line numbers are those of the export.
Effort: S small, M medium, L large.
Gain: what it removes, as far as reading shows.

## 1. Summary

| # | Finding | Effort | Gain |
|---|---------|--------|------|
| 1 | No shared waiting code. 10 named poll helpers (`eventually`, `waitFor`, `within`, …) and about 70 hand-written deadline loops, 5 s fixed everywhere, no scale for QEMU or load. | M | ~420 call sites onto one helper; one knob for slow runners |
| 2 | The top flake class (6 fixes in the history): a test reads the API (the client's snapshot) and acts before the server's pump has handled the event. Each fix is a local wait. No pump barrier exists. | M | the class, not one test at a time |
| 3 | `greetingPublished` is waited for by `startWith` only. `startStored` (49 call sites), `startRooms` and about 20 other start helpers, plus 27 inline `go s.Run(ctx)`, do not. Two later fixes (vwt, log sources) were this gap again. | S | a known latent flake in ~80 starts |
| 4 | The owner's LogWatcher idea fits. There are 7 log-capture types, 3 SSE readers and ~11 fake-Cube line counters; none can say "from now on, skip the rest, wait for X". `builtServe.waitFor` comes closest but scans from line 0. | M | most "met another item first" failures |
| 5 | `sameLines(fc.Received()[sent:], …)` in `config_api_test.go` compares the Cube's lines exactly, with no filter for polls (`l:`) or follow-up re-reads (`c:`). The repair and restore copies filter `l:` (and `f:`). | S | 13 sites that can fail on a 10 s poll |
| 6 | UI `test_log.py` reads `.first` / `.nth(i)` of a kind after dispatching its own events, while the test server's stream keeps sending (10 s poll, automatic backups). The fix that holds `appendLogEvent` lives inside one test only. | S | ~15 tests of the most-fixed file |
| 7 | The UI server sets run the default 10 s poll and automatic backups; most UI log fixes were their events. `run.sh` could start them quiet. | S | the source of 5 of the UI fixes |
| 8 | Close-then-rebind ports: 6 `free*` helpers return a port they just closed; 5 tests close a fake Cube and start another on the same address. Two were fixed, the cubelan ones left as they were by an earlier decision. | S | a known flake class under `make race` |
| 9 | CI: a job re-run green counts as passed (`code-result.sh`, CI.md 198); the Tested status then skips every test job for that code. A flake that passes on a re-run leaves no trace. | S | visibility only |
| 10 | Tests that cannot fail as named: `TestEventLinesMatchTheRows` never checks the "no second line" its comment promises; 12 discovery tests and one elsewhere `t.Skip` on any bind error, so a port clash turns them green. | S | 2 holes |

## 2. Patterns of failure seen in the history

About 110 commits match the grep; 50 of them changed a test to fix a failure.
Grouped:

| Pattern | Commits (summary line) | Worked around by |
|---------|------------------------|------------------|
| **A. The test outran the pump** (API shows the snapshot, the pump handles the event later) | "TestWallReset waits for the pump's state events, not the API"; "The failed radiogram's test waits for the pump's line before it changes the list again"; "TestDisagreeResendsMassimosSetpoint waits for the pump to confirm the room"; "A started server has published its greeting's events"; "The virtual view's test counts only the mismatches it sends"; "The log's sources test waits for the Cube's connect" | a stream subscription in one test; `caughtUp` in one test; `driftRig.confirmed`; `greetingPublished` in `startWith` only; a `finding: test` marker; a second wait condition |
| **B. A foreign item came first, or was counted** | "TestLogWithoutStore counts only the events it had read"; "The log file test waits for the SIGHUP's own rotation"; "Let a proxy client's write skip a pushed device list"; "startStored returns once the server's start is written"; "Discovery's tests wait for a run that began after their call"; UI: "The log's answers test keeps the stream's own events out", its follow-up "…keeps the server's events out at appendLogEvent", "The log's live-event test counts only the kinds it sends", "The log's last 20 are the newest, whatever the stream brings", "The log's device names test reads the state lines alone" | filter by id; wait for an attribute (`reason=sighup`); skip pushed `L`; flush the writer; post again until `ran_at` is after the call; filter by kind; hold `appendLogEvent`; read by id |
| **C. Event before its log line** | "TestALostCubeIsReportedOnce waits for the line it reads"; "TestAnUnreachableCubeIsReportedOnce waits for the line it reads"; code side: "A Cube's loss and return are logged before their events" | a second wait for the line; then the order fixed in the client |
| **D. A fixed deadline or order too tight on a slow runner** (QEMU, load) | "The DNS-01 tests end their time on an event, not after a fixed time" (500 ms and 300 ms); "The observer test holds the first command until the next two are queued"; "The boost test waits for each device's boost to end"; "A page by command finds its rows under -short too"; the failed-radiogram commit notes 5 s `waitFor` running out at load 40 | end on an event; a gated recorder; wait for both; smaller stride; left as is |
| **E. A port closed and taken again** | "The two tests that bring a Cube up late hold its port"; "The proxy discovery test takes two distinct free ports"; "The single-Cube test dials again when another listener drops it"; "The busy-Cube test starts once the fake holds the other client" | `fakecube.Serve(ln)`; hold both listeners; re-dial until 20 s; wait `Connections() == 1` |
| **F. Background traffic of the server** (polls, re-reads) | "Keep polls and re-reads away from the room tests' writes"; code side: "A command waits for the one in flight instead of failing busy" | slower polls in tests; then fixed in the client |
| **G. Files written in two steps, or left open** | "The settings watch test writes each file whole, renamed into place"; "TestMigration12 closes the store it reopens" | rename into place; `openAt` registers the close |
| **H. Lock held across `Fatalf`, map order** | "a refusal window test that does not hang or depend on map order" | read under lock, then assert; step the clock |
| **I. UI: smooth scroll and header compaction** | "Start the slow scroll once the page stands still"; "The slow-scroll test lets the header settle before its steps"; "…waits for a quiet header, not a still page"; "The page bar test waits for the menu's scroll to rest"; "The scroll tests wait for a still header, a pending notice included"; "The Reset all tests wait for the page to rest after the scroll"; "A jump down fails the scroll test only where the page goes back"; "A card of the personal page is used once it is in view"; "The dropped Cube's rooms are measured in one evaluation" | `scroll_settled` (10 frames), `quiet_header` (0.8 s), measure in one `evaluate`, assert only a move back |
| **J. UI: shared server state and file order** | "Tests that open pages without the token forget the failed checks"; "The Cube panel's pill test starts from a Cube without problems" (window open > 5 min); the scroll tests' pending radiogram; "The header tests set the version whose fit they test"; "No trace on the first pass…" (cube_found once) | a teardown that forgets failed checks and asserts none; stub the problems; drop a wait; set the version |
| **K. UI: page and request timing** | "The log's device names test waits for the rooms request"; "the pairing test waits for the countdown's text"; "Guard the proxy test's route against a late request" | `wait_held`; wait for text; `try/except` around `route.fulfill` |
| **L. Shell** | "A log check reads the whole log" (`grep -q` and `pipefail`) | `has()` |

Leads checked: the pump lead (A), the 500 ms order deadline (D),
"the next" event or line (B), the `EventSource` listener order (B,
the appendLogEvent commit), `-race` under load (D, the 5 s note;
`make race` has no `-timeout`), the smooth scroll (I): all confirmed.
"A flaky marker with reruns hid real failures":
no rerun plugin or marker exists in the code or its history;
the only retry is `run.sh`'s second pass (`tools/ui-test/run.sh:227`),
which fails the run anyway (`:232`).
What can hide a failure is CI's re-run of a job (section 8).

## 3. The waiting, as it is

### Go: named poll helpers

| Helper | Defined | Call sites | Body |
|--------|---------|-----------:|------|
| `eventually` | `internal/server/followup_test.go:144` | 112 | 5 s, 10 ms poll |
| `waitFor` | `internal/server/pairing_api_test.go:52` | 62 | same body as `eventually`, same package |
| `within` (bool) | `internal/server/backups_api_test.go:45` | 4 | d, 10 ms |
| `waitConnected` | `internal/server/cubes_api_test.go:64` | 15 | 5 s, API poll |
| `eventually` | `internal/cubelan/client_test.go:679` | 64 | 5 s, 5 ms |
| `waitEvent` | `internal/cubelan/client_test.go:40` | 19 | client channel, skips other kinds, 5 s |
| `waitFor` | `internal/proxy/responder_test.go:117` | 15 | 5 s, 2 ms |
| `within` | `internal/cubelan/fakecube/oneclient_test.go:44` | 8 | 5 s, 10 ms |
| `waitFor` (line type) | `internal/engine/queue_test.go:29` | 2 | fake Cube's received lines |
| `waitForCube` | `internal/serve/serve_test.go:71` | few | 10 s |
| `builtServe.waitFor` | `cmd/massimo/serve_stop_test.go:113` | 10 | subprocess stderr, notified, 10 s, **scans from line 0** |
| `builtServe.waitForAttr` | `cmd/massimo/logging_test.go:280` | 2 | polls the same lines, 10 s |
| special waits | `driftRig.waitSetting/confirmed/waitOutcome` (drift, resend), `waitUnconfirmed` (confirm:86), `waitWatch` (errorwatch:85), `slotsCube.waitReadings/waitSlotsEvents` (slots:100,149), `mismatchCube.waitChecks` (congruence:202), `settleState` (errorwatch:56), `reportAndSettle` (autoreset:162), `postDiscovery` (discovery) | — | each on `eventually`/`waitFor` or a loop |

### Go: hand-written waiting

| Kind | Count | Where most |
|------|------:|------------|
| `for deadline := time.Now().Add(…)` loops | ~69 (48 × 5 s, 14 × 10 s) | server (start helpers, cubes, commands, pairing, reboot), store events, serve setup, cmd/massimo |
| `time.After(…)` in a select | 59 (32 × 5 s) | cubelan client, cmd/massimo reload/service, stream_admit, resume |
| `time.Sleep` | 103 | 36 of them a fixed wait before an assertion (below) |
| fixed sleep then "nothing came" / "no more" | ~25 | `cubelan/client_test.go:251`, `send_turn_test.go:78,138,166`, `hold_test.go:63,115`, `fakecube/apply_test.go:206,299`, `server/confirm_test.go:197`, `followup_test.go:94,130,279`, `backups_api_test.go:69,289,307`, `discovery_test.go:781,783,834`, `engine/queue_test.go:359`, `queue_moved_test.go:66`, `cubeio/pairing_test.go:47,60`, `proxy/proxy_test.go:659`, `responder_test.go:209,292`, `vwt_test.go` (300 ms select) |
| fixed sleep then a positive check | ~8 | `cubelan/client_test.go:117` (≥2 polls in 200 ms), `:497,522` (1.3 s "two failed dials"), `server/eventline_test.go:94`, `cmd/massimo/serve_stop_test.go:203,206` |

Log capture types: `syncBuffer` three times (`server/followup_test.go:155`,
`engine/queue_test.go:208`, `proxy/responder_test.go:230`),
`syncLog` (`serve/setup_test.go:485`),
`logLines` with level/msg matching (`cubelan/client_test.go:618`),
`lockedWriter` (`cmd/massimo/sighup_unix_test.go:20`),
`builtServe.lines`. 49 test loggers write text that tests search with
`strings.Contains` or `strings.Count` (76 sites); none parses records.

SSE readers: `readStream/readStreamFrom` with `next`, `nextTest`,
`burst` (`server/resume_test.go:26-130`),
`openStream` raw lines (`stream_admit_test.go`),
`notificationEvents` (`push_api_test.go:266`).

Fake Cube line readers: `countType` (server, proxy), `received` (engine,
cubelan `send_turn`), `countRadio`, `sLines`, `sentSince` (filters `l`),
`linesAfter` (filters `l`, `f`), `lastRadio`, `pollsReceived`, `resets`.
All copy `fc.Received()` and index it.
The fake has no notification (`fakecube.go:370 Received`, `:377 Connections`).

### UI (`tools/ui-test/`)

| Item | Count / place |
|------|---------------|
| `wait_until(page, expr, s)` (`helpers.py:35`, 200 ms poll, returns bool) | 410 calls; 2 not asserted: `test_notifications.py:77`, `test_cubes.py:79` |
| `scroll_settled` (`helpers.py:46`, 10 frames, returns bool) | result ignored at `helpers.py:241`, `test_header.py:464,479,483,486,490`, `test_to_top.py:22` |
| `wait_held` (`helpers.py:141`) | polls 100 ms |
| `wait_for_timeout(n)` fixed | 94; 18 × 1600 (the page drops the stream's first 1.5 s from the log), 24 × 300, 14 × 200, 10 × 500; 18 in `test_log.py` |
| `try/except Exception: pass` around a route's fulfil | ~15 copies (`test_cubes.py:276,322,332,396`, `test_integrations.py:156,238,262,299`, `test_users.py:88`, `test_parts_guard.py:180,285`, …) |
| console and page-error guard | `conftest.py` `on_console`, `Guard.verify` (as part of the call) |
| stream watching | `Watch.streams` holds request URLs only; no event of the stream is recorded |
| reruns, markers | `pytest.ini`: one marker (`used_ticket`), no rerun plugin; `run.sh:227` second pass with trace, exit 1 either way |
| per-test isolation | one server set per worker, shared by its files and the four cases; `clear_preferences` before each page; `failed_checks_forgotten` asserts no address failures after each test |

## 4. Where a test trips over what comes first

Ranked by how likely they fail; "how sure" is my reading.

| Rank | Place | What can come first | How sure |
|-----:|-------|---------------------|----------|
| 1 | UI `test_log.py:41` (`test_log_changes`), `:63` (`test_log_radio_faults`, exact 4 newest of `kind-state`), `:187` | a real state line from the 10 s poll after the dispatch | high: the same cause failed three sibling tests |
| 2 | UI `test_log.py:383-384` (`test_log_server_lines`): waits for "any `kind-backup` line", which the server's own automatic backup lines already satisfy, then reads `.first` | a real automatic backup (the very event of the appendLogEvent commit) | high |
| 3 | UI `test_log.py:354-358`, `:295-314`, `:406`, `:432`: `.first` of `connected`, `metadata`, `config`, `change` | the server's own lines of those kinds | medium (rarer kinds) |
| 4 | `internal/server/config_api_test.go:297` `sameLines`, 13 sites (`:360`, `:380`, `TestRemoveListedDevice` `:588`, …) | an `l:` poll (10 s default, `startCopyCube` sets none) or a follow-up `c:` | medium on armv6/QEMU, low on amd64 |
| 5 | `internal/server/resume_test.go:113` `burst`: the first N events must be states; `TestStreamResumes` (`:177-187`) fails on any `state` after the resume | a pump state event from a poll or a late greeting event | low-medium |
| 6 | `startStored`/`startRooms` users that count the stream or the log (`vwt_test.go:188` burst sizes 10/7/9 then `b[9]` virtual; `logsources_test.go`) | the greeting's own events, published after the start returns (`greetingPublished` missing) | medium: fixed twice already, case by case |
| 7 | `internal/server/autoreset_test.go:162` `reportAndSettle`: returns once any state line ever had `key → to` | an older line with the same change makes it return before the new one | medium (passes too early; see 7 below) |
| 8 | `internal/server/config_api_test.go:542` `logKinds(t, ts, n)`: "from the n-th line on", on a page of `limit=500` | more than 500 lines shift the index; a foreign line of the same kind and device counts | low |
| 9 | `cmd/massimo/serve_stop_test.go:113` `builtServe.waitFor`: first match since the process start | a line of the same message before the action (the SIGHUP fix was this) | low now, structural |
| 10 | `tools/lan-test/run.sh:305-306, 324-327`: `holder_log \| has …` reads the whole container log | a line of an earlier step | low |

"Nothing came" after a fixed sleep (section 3) does not fail falsely;
it passes falsely when the runner is slow.
On QEMU those tests check less than on amd64.

## 5. Helpers to build

Order: 1, 2, 3 first (they remove the classes A, B, D), then 4-7.

### 5.1 One poll helper with a scale (S, ~420 sites)

For every `eventually`/`waitFor`/`within` and deadline loop.

```go
// package testkit (internal/testkit, imported only by _test files)
func Scale(d time.Duration) time.Duration          // × MASSIMO_TEST_SCALE (CI armv6: 4, race: 2)
func Eventually(t testing.TB, what string, cond func() bool, opts ...Opt)
func Within(d time.Duration, cond func() bool) bool
func Never(t testing.TB, what string, cond func() bool, until func() bool) // "nothing came" until a barrier, not a sleep
```

Replaces the 10 named helpers and most loops.
Migrate package by package: server (174 calls), cubelan (64), proxy (15),
fakecube (8), then the loops.
The package-wide 5 s becomes `Scale(5*time.Second)`.

### 5.2 The watcher (owner's idea) (M)

One generic type, several feeders.
Started before the action, it keeps everything; a wait reads from a cursor,
skips what does not match, and on timeout prints what it did see.

```go
type Watch[T any] struct{ /* mu, items []T, notify chan, show func(T) string */ }
func (w *Watch[T]) Mark() Cursor                                   // "from now on"
func (w *Watch[T]) Await(t testing.TB, from Cursor, what string, match func(T) bool) T
func (w *Watch[T]) AwaitAll(t testing.TB, from Cursor, what string, match ...func(T) bool) []T // any order
func (w *Watch[T]) Count(from Cursor, match func(T) bool) int
// on timeout: "no <what> within 5s; seen since the mark: 12 items: state 0a010d …"

func LogWatch(level slog.Level) (*slog.Logger, *Watch[Record])     // a slog.Handler; Record{Level, Msg, Attrs map[string]any}
func StreamWatch(t testing.TB, url string) *Watch[SSE]              // SSE{ID, Kind, Data, Event map}
func CubeWatch(fc *fakecube.Server) *Watch[cube.Message]            // needs fakecube.OnReceive(func(cube.Message))
func ClientWatch(c *cubelan.Client) *Watch[cubelan.Event]
func ProcWatch(r io.Reader) *Watch[Record]                         // cmd/massimo's subprocess stderr
```

One generic type with four feeders serves best: the waits are the same,
the items differ, and a test often needs two (log and stream).
Closest today: `builtServe.waitFor` (notified, but no cursor),
`cubelan.waitEvent` (skips other kinds, no record of them),
`nextTest` (skips by kind), `waitAll` in `vwt_test.go:220` (any order).
Replaces: the 7 log buffers, 3 SSE readers, ~11 line counters,
`next`/`nextTest`/`burst`,
`sameLines` on slices (as `CubeWatch` with a filter for `l:`),
and about 60 waits that read the log API only to find one new line.
Where: `internal/testkit`;
`fakecube` gets a small `OnReceive` hook (it is test code already).
Migrate: cubelan `logLines` and `waitEvent` first (few, simple),
then server's stream tests (`resume`, `vwt`, `wallreset`, `push`),
then the log-API waits, then `cmd/massimo`.

### 5.3 A pump barrier (S-M, the class A)

```go
// in package server, test-only file
func (rt *runtime) pumpSync(t testing.TB) // waits until the pump has handled every event the client emitted before the call
```

Needs a counter of emitted events in the client and of handled ones in the pump
(or a test hook after each event).
Replaces `caughtUp` (failedradiogram), `driftRig.confirmed`,
the stream wait in `wallreset_test.go`,
the NTP-enqueue trick in `settleState` and `reportAndSettle`,
and is what `greetingPublished` approximates.

### 5.4 One server start (M, ~80 starts)

`startWith`, `startStored`, `startQuiet`, `startRooms(Opts)`,
`startStructureCube`, `startRepairCube`, `startConfirmCube`,
`startPairingCube`, `startCopyCube`, `startResets`, `startRestoreCube`,
`startDrift`, `startSlotsCube`, `startMismatchCube`, `startWatchCube`,
`startTwoCubes`, `startDiscovering`, … and 27 inline `go s.Run(ctx)`.

```go
func startServer(t testing.TB, o ...StartOpt) *rig // WithStore, WithGreeting, WithFake(mod), WithOptions(mod), Quiet (no poll, no refresh)
// rig{fc, ts, s, st, rt, log *Watch[Record], stream func() *Watch[SSE]}; returns once connected, greetingPublished and the writer flushed
```

Same waits for every test; `Quiet` sets polls off where a test counts.

### 5.5 Ports that stay held (S)

`testkit.Listen(t)` returns an open listener to hand on (`fakecube.Serve(ln)`
exists);
a `fakecube.Pause()/Resume()` that drops clients and refuses without closing the
socket, for "the Cube goes and comes back".
Replaces `freeAddr` (cubelan, serve, cmd/massimo), `freeTCP`, `freeUDP`,
`freeUDPPort`, and the rebinds at `cubelan/client_test.go:524,704,784,955`,
`proxy/proxy_test.go:317`.

### 5.6 UI: a stream watcher and a log isolator (S)

```python
# helpers.py
def watch_stream(page)            # init script: wraps EventSource listeners, records (kind, id, data, t) in window.__events
def wait_event(page, kind, pred_js, after=None, seconds=5)  # from a mark, skips the rest
def isolate_log(page)             # test_log.py:709's sendTestEvent + appendLogEvent gate, shared
def wait_log_live(page)           # a page flag for "the first 1.5 s are over", in place of 18 × wait_for_timeout(1600)
def route_safely(page, pattern, handler)  # the ~15 try/except copies
```

`isolate_log` first: all of `test_log.py`'s dispatching tests (17
dispatches) then read only their own lines.

### 5.7 A quiet UI server set (S)

`run.sh:116-121` starts `serve` with the defaults:
a 10 s poll and automatic backups.
A long `-poll` and backups off for the set (or a second "busy" set for the tests
that need polls) remove the source of the UI log fixes.
Which tests need a poll is to be checked (`test_cube_intervals.py`,
the scroll tests' notices).

## 6. Redundancy

| Written more than once | Copies | Fold into |
|------------------------|-------:|-----------|
| poll helpers (`eventually` ×2, `waitFor` ×3, `within` ×2, …) | 10 + ~70 loops | 5.1 |
| `eventually` and `waitFor` in the same package `server`, same body | 2 | 5.1 |
| log capture buffers | 7 | 5.2 `LogWatch` |
| SSE readers | 3 | 5.2 `StreamWatch` |
| fake Cube line counters, three with different filters (`sameLines` none, `sentSince` `l`, `linesAfter` `l`,`f`) | ~11 | 5.2 `CubeWatch` |
| server start helpers | ~20 + 27 inline | 5.4 |
| greeting load + fake start (`LoadGreeting` 90 times, `const fixture` in 8 packages) | 90 | a `fakecube.StartFixture(t, mods...)` |
| free-port helpers | 6 | 5.5 |
| fake clocks in `internal/server` (`checkClock`, `fakeClock`, `testClock`, `stepClock`) and `notify.fakeClock` | 5 | one stepping clock in testkit |
| `settleState` / `reportAndSettle` (near-copies, different matching) | 2 | 5.3 + one |
| `waitFor` / `waitForAttr` on `builtServe` | 2 | 5.2 `ProcWatch` |
| log-API readers in server tests (`logOf`, `rawLog`, `allLog`, `logKinds`, `stateLines`, `watchLines`, `unconfirmedEvents`, `driftRig.events`, `slotsCube.events`, `mismatchCube.events`, `foundEvents`) | 11 | one `logQuery(t, ts, q) []event` with an id cursor |
| UI route `try/except` | ~15 | 5.6 `route_safely` |
| UI `wait_for_timeout(1600)` | 18 | 5.6 `wait_log_live` |
| UI fixes that filter the server's own events per test | 5 tests | 5.6, 5.7 |

## 7. Tests that look inaccurate, or lock in what looks wrong

| Place | Why | How sure |
|-------|-----|----------|
| `internal/server/eventline_test.go:73` `TestEventLinesMatchTheRows` | the comment says "each once, with no second line"; the loop at `:108-116` only checks each row has a line; leftover lines are never asserted. A doubled line passes. | sure |
| `internal/server/discovery_test.go:32` `freeUDP`, and `:357`, `:388`, `:608-625`, `:716-730`, `:807`, `:871-886`; `found_elsewhere_test.go:27` | `t.Skip` on any bind error, also on Linux where 127.0.0.2/3 exist: a port clash under `make race` skips, and the job is green | sure of the code; how often unknown |
| `internal/server/autoreset_test.go:162` `reportAndSettle` | returns at any line that ever had `key → to`, not a new one; the twin `settleState` (`errorwatch_test.go:56`) counts lines before. A second report of the same change does not wait. | likely |
| `internal/cubelan/client_test.go:491` `TestSetAddressCutsTheBackoffShort` | a sleep of 1.3 s stands for "two failed dials", and "connected within 700 ms" is a wall-clock bound; slow runners can fail it, fast ones may not have built the backoff it says it cuts | suspicion |
| `internal/cubelan/client_test.go:512` `TestReconnectWhileDisconnected` | `freeAddr` closed, 1.3 s later the fake starts on it: the class E, left as it was by an earlier decision | sure of the gap |
| `internal/cubelan/client_test.go:245` `TestClientWaitsForItsAddress` | "no connection without an address" after 100 ms only | weak, not wrong |
| `internal/server/backups_api_test.go:69,289,307` | "no backup without a change" after 300 ms with a 60 ms window: on a slow runner the check may end before a wrong backup would come | weak, not wrong |
| `internal/server/vwt_test.go:188` | burst sizes 10/7/9 are hard-coded `next` calls; a change to a view's burst would silently move the window over live events | suspicion |
| `internal/announce/announce_test.go:129,159,180` | `ask` returns nil on timeout; the positive asks then dereference it (a panic, not a message) | sure, cosmetic |
| UI `test_log.py:383` | the wait (`kind-backup` count > 0) is met by lines the test did not send, so it waits for nothing | sure |
| UI `test_notifications.py:77`, `test_cubes.py:79`; `scroll_settled` at 7 places | a wait whose result is dropped: a timeout goes on as if it held | sure |
| UI route handlers with `except Exception: pass` | also swallow an assertion or a typo inside the handler | sure of the code |
| UI cleanups with `except Exception: pass` (`test_oidc.py:157`, `test_integrations.py:99`, `test_users.py:155`) | a failed cleanup leaves the shared server changed for later files on the worker, which then fail elsewhere (class J); only failed token checks are asserted at teardown | likely |

Nothing found that locks in plainly wrong behaviour.
Two worth a look: `TestDisagreeResendsMassimosSetpoint`'s fix says `roomChecks`
"drops a disagreement not yet reported" while a write confirms,
and that is kept as intended;
and `test_a_jump_down_stays_where_it_went` now accepts any move forward,
which also accepts a forward jump of a bug's making (the commit says so).

## 8. CI

| Item | Effect | Effort |
|------|--------|--------|
| A failed job re-run green counts (`packaging/code-result.sh`, `GUIDELINES/project/CI.md:198`); the Tested status (fingerprint) then skips every test job for that code on the merge | a flake that passes the re-run leaves no record; the next run does not test it again (CI.md:137 says so) | S: write each run attempt's failures to the summary or a file kept per fingerprint |
| `run.sh`'s second pass passes → "flaky" message, run fails | does not hide; but only the trace of a second failure is kept, so a flake's first failure has video only | — |
| PRs run the UI tests only for changes under `internal/server/ui/`, `tools/ui-test/`, … (`ci.yml` "What to test") | a server or API change that breaks the page shows on `main` only | S: add `internal/server/[^/]+\.go$` and `docs/api-v1-openapi.yaml` |
| `make race` (`Makefile:217`): no `-timeout`, no `-count=1`; seven packages, `internal/engine` (the queue), `internal/cubeio`, `internal/certs` (hooks, goroutines) not among them | a hang costs the default 10 min per package; races in engine go unseen | S |
| `make test-armv6` (`Makefile:227`): `-short` and QEMU, no scale for deadlines | the 5 s waits are the same as on amd64; the history's D-class failures came from here | S with 5.1's `Scale` |
| `go test ./...` in "Vet and test" and the Race job run the same packages again without race | time; the Race job could run all packages with `-race` and the plain job skip those | M, check the minutes first |
| No job-level `timeout-minutes` on the test jobs | a hung test binary is bounded by Go's 10 min per package only | S |
| `t.Skip` on bind errors (section 7) | green with tests skipped; nothing reports skip counts | S: `go test -json` and fail on skips of named tests in CI, or `t.Fatal` on Linux |
| LAN job: `has` over the whole log | see 4, rank 10 | S |

## 9. Not verified by reading

| Suspicion | Run that settles it |
|-----------|---------------------|
| `sameLines` sites fail when a poll falls in the window | a build overlay with `DefaultPollInterval = 20 * time.Millisecond` in `internal/server/settings_api.go`, then `MASSIMO_ENV_FILE=/dev/null go test -overlay overlay.json -count=20 -run 'TestRemove|TestCopy|TestRepair' ./internal/server` |
| `startStored`/`startRooms` tests that count events fail when the pump is slow | an overlay that sleeps 100 ms in `pump` before each event, then `MASSIMO_ENV_FILE=/dev/null go test -overlay overlay.json -count=10 ./internal/server` and compare with `startWith`'s tests |
| UI log tests fail on a real state or backup line | `run.sh` with `-poll 1s` on the set (local edit), then `UI_TEST_WORKERS=1 PYTEST_ADDOPTS='-k "test_log_changes or test_log_radio_faults or test_log_server_lines or test_log_lines"' make ui-test` |
| discovery tests skip under port load | `MASSIMO_ENV_FILE=/dev/null go test -race -count=20 -v -run Discovery ./internal/server 2>&1 \| grep -c -- '--- SKIP'` beside `make race` |
| cubelan rebinds fail under load | `MASSIMO_ENV_FILE=/dev/null go test -race -count=50 -run 'TestReconnectWhileDisconnected|TestALostCubeIsReportedOnce|TestLostFromALossToTheNextGreeting|TestSetAddressCutsTheBackoffShort' ./internal/cubelan` while `make race` runs beside it |
| `TestEventLinesMatchTheRows` passes with a doubled line | add a second `s.log.Info("device state", …)` in an overlay and run `-run TestEventLinesMatchTheRows ./internal/server` |
| armv6 durations near the 5 s waits | `make test-armv6 2>&1 \| grep -E '^(ok\|---)'` with `-v` added locally, sorted by time |
| engine races | `MASSIMO_ENV_FILE=/dev/null go test -race -count=5 ./internal/engine ./internal/cubeio ./internal/certs` |

## 10. Covered and sampled

- **In depth:**
  the history (all test-fix commits' messages, the workarounds named there);
  `internal/server` start helpers (`server_test.go`, `cubes_api_test.go`),
  the stream helpers (`resume_test.go`), `vwt_test.go` streams,
  `followup_test.go`, `errorwatch_test.go`/`autoreset_test.go` settles,
  `config_api_test.go` `sameLines`/`logKinds`, `eventline_test.go`,
  `backups_api_test.go` waits, `discovery_test.go` skips;
  `internal/cubelan/client_test.go` helpers and lost-Cube tests;
  `internal/proxy/proxy_test.go` setup and `readLine`; the fake Cube's API;
  `cmd/massimo` `builtServe`; `tools/ui-test/conftest.py`, `helpers.py`,
  `pytest.ini`, `run.sh`; the `test_log.py` tests that dispatch;
  `ci.yml` jobs and "What to test"; `Makefile` test targets;
  `tools/fuzz/run.sh`.
- **Sampled (grep and a few reads):**
  every `time.Sleep`, `time.After`, deadline loop, `t.Skip`,
  log handler and helper definition across the 221 Go files;
  `internal/store/events_test.go` waits and prune tests; `internal/announce`,
  `internal/certs/dns01_test.go`, `internal/serve/setup_test.go`;
  UI `wait_until`, `wait_for_timeout`, `except` and `.first` sites;
  `tools/lan-test/run.sh`; `GUIDELINES/project/CI.md` on re-runs.
- **Left out:**
  `internal/notify` beyond its clock;
  `internal/engine` and `internal/cubeio` beyond the sleeps; `internal/update`,
  `internal/install`, the Windows and macOS jobs;
  the UI tests other than `test_log.py`,
  `conftest.py` and `helpers.py` in depth (`test_parts_guard.py`,
  `test_scroll.py`, `test_header.py` only by grep);
  `docs/ui-testing.md` beyond a grep; the fake Cube's `apply.go` logic.
