# ANALYSIS Flakes: three flaky tests, and the open PRs' failures

2026-11-24, session "Fix three flaky tests seen on PR #410",
branch `claude/flaky-three` at origin/main 3cdb42c1.
Investigation only, by three Opus sub-agents of that session:
the probes ran through `go test -overlay` from the session's scratch,
the Android one from CI's logs and artifacts and AOSP 14's sources.

## 1. TestSameConfigAtDebug (internal/server/logdetail_test.go:255)

**Cause: the product reports every re-read twice; the test's second mark
can fall between the two.**

- `cubelan` `handle` (client.go:572-629) puts a C reply into the snapshot
  and emits `EventConfig` before `Send` returns it.
- `cubeio.Writer.Reread` (cubeio.go:104-109) then calls
  `Client.ApplyLocal(reply)`: `handle` again, a second `EventConfig`.
- So one `Reread` gives two events and two config lines (probe: ids 18, 19,
  both debug).
- The first `logAfter` (:264) takes the first line.
  The pump may write the second after the second `logMark` (:282),
  before the second re-read's reply reaches the snapshot:
  it reads the old snapshot (server.go:1175), logs "no change" at debug,
  and `logAfter` (:284) takes that line. The check at :286 fails.
- Failure text, all 22 failures:
  `the change: {Kind:config Device:0a010d … Level:debug … Changes:map[] …}`.

**Measured** (turns of `-count=20`, main's test and the fixed one alternated):

| Beside it | Load average | main | fixed |
|---|---|---|---|
| 2× `-race` whole-package runs | 3.5–45.5 | 22/1100 | 0/1100 |
| 2× plain whole-package runs | 7.3–13.8 | 0/120 | 0/120 |
| quiet | 3–7 | 0/940 | 0/940 |
| widened (100 ms in the pump, `c:` 300 ms late) | – | 30/30 | 0/30 |

**Fix (test):** `pumpSync(t, s)` after `fc.SetConfig`, before the second
`logMark` - the documented wait (docs/testing.md, "The pump").

**Product finding (not this flake's fix):** the double apply is also in the
proxy's `c:` path (proxy.go:440-446). Each re-read runs `checkMismatches`,
`publishVWTs`, `checkConfig` and `bk.touch` twice and sends two stream events.
By reading the code only: the second, local C line could be taken as the
reply of another request's pending `c:` (client.go:580-590).
Dropping the second apply: main's test 0/20 widened;
`internal/cubeio`, `proxy`, `cubelan`, `fakecube` pass; `internal/server` not run.

## 2. TestLogAfterReadsABurstOnce/store (internal/server/logafter_test.go:110)

**The wait already takes the scale**:
`testkit.Eventually(…, testkit.For(4*testkit.Wait), …)` (:185/:205),
20 s at scale 1, 40 s at scale 2.
It is a deadline on about 20 s of work: 4 writers × 3000 lines through SQLite.

**Measured** (store subtest, one or two runs each):

| Run | Burst | Deadline | Result |
|---|---|---|---|
| plain `-race`, scale 1, package | 28.8 s | 20 s | FAIL |
| plain `-race`, scale 1, alone | 18.9–20.9 s | 20 s | FAIL |
| `-race` + RACEGCFLAGS, alone | 6.9–10.8 s | 20 s | pass |
| plain `-race`, scale 2, package | 35.1 s | 40 s | pass, thin |
| `make race` form, package | 17.6 s | 40 s | pass |
| n=1000, plain `-race`, scale 1, package | 9.9 s | 20 s | pass |

- RACEGCFLAGS (checkptr off for modernc.org) halves the work;
  the scale only doubles the deadline.
  Plain `-race` at scale 1 is over 20 s even alone.
- Profile: half the CPU in checkptr; the burst is serialised through the store.
- n=1000 still gives 3000 lines in the fake Cube's log (~15 pages),
  4000 in every Cube's (~20 pages), ≥ 40 store batches.
  The commit-between-walks property is tested directly by the store's
  `TestEventsForwardWhileWriting` (internal/store/events_test.go:364).

**Fix (test):** n=1000, and a failure that says how far the reader got
(slow runner vs stopped reader).

**Docs:** docs/testing.md "The scale" gets a paragraph: the race waits are
sized for `make race` (scale 2 and RACEGCFLAGS); a `-race` run by hand takes
both, and how. The shared baseline and docs/store.md:1725 name a plain
`go test -race ./...`.

**Seen in passing:** `TestProgramTag` failed once in a scale-1 `-race` load run
at load 45 (`etag_test.go:92`): the fake's `ApplyAfter` of 600 ms is not scaled.

## 3. Android emulator: `testRunFailed`, a test not run

**Cause: an Android 14 race between the orchestrator's `pm clear` and a
postponed task kill; no test fails.**

Attempt 1 of run 45391014885 (logcat of its artifact):

```
12:17:12.921 ActivityManager: Force stopping ch.metaworx.massimo … clear data
12:17:13.896 ActivityManager: Start proc 7793:ch.metaworx.massimo … (next test)
12:17:13.931 ActivityTaskManager: Destroy timeout of remove-task, attempt to kill Task{… A=10196:ch.metaworx.massimo}
12:17:13.932 ActivityManager: Killing 7793:ch.metaworx.massimo/u0a196 (adj -10000): remove task
12:17:23.896 ActivityManager: Process … 7793 … failed to attach
```

- `pm clear` (`clearPackageData=true`, app/android/app/build.gradle.kts:45)
  removes the app's tasks from recents.
  Where the last test's activity is still closing, AOSP 14 postpones the kill
  by `KILL_TASK_PROCESSES_TIMEOUT_MS` = 1 s, then kills every process of the
  package - the next test's instrumentation, if it has not attached yet.
- The next test dies 21–36 ms after its start; the orchestrator reports
  "Test instrumentation process crashed", "Expected 14 tests, received 13".
  Gradle says BUILD SUCCESSFUL; only the tally's count fails the job.
- The test lost is random (KeepTest in #410's run, TapTest in #415's).
- 2 failures in 11 suite runs since the job began (2026-11-24 02:30).
  The 1 s timeout fired for the app's task 9 times in 6 of 10 runs with
  artifacts; it killed only where the next process was < 50 ms old.
- Ruled out: boot, the test servers, certificates, the prompt, the tally.

**Fix (tests):** each test takes the app's tasks out of recents after closing
its activity (`ActivityManager.appTasks` → `finishAndRemoveTask()`, no kill):
`Rig.removeTasks()`, called from `EmulatorTest`'s `after` and an `@After` in
`ConnectPageTest`. The next `pm clear` then finds no task and starts no timer.
Not run on an emulator here; CI shows it.

**Reporting:**
- `tools/app-emulator/tally`: a failure's first lines under its case;
  `android-test#testRunFailed` named as the run's own record
  ("the instrumentation ended before the run did"),
  not "not among the tests the sources declare".
- `tools/app-emulator/run.sh`: on a failed or short run, the device log's
  kill and crash lines of the app (filter tried on three logs: < 10 lines each,
  the two failures show the decisive lines);
  a warning, not a failure, where a passing run still shows the app's
  "Destroy timeout of remove-task".
- `tools/app-emulator/README.md`, "The results".

## 4. Open PRs (checked 22:30)

| PR | Red because of | Class |
|---|---|---|
| #416 | UI test (390-en): `test_confirm.py::test_unconfirmed_marks` 5 s timeout, passed on the job's retry | flake only |
| #415 | `test_shell_updates` `READS` lacks its new `backups/unserved` read (all 4 UI cases); emulator race (TapTest) | real + flake 3 |
| #414 | Windows: `file:C:\…` parsed as a port; armv6: `TestPackagedUpdateSource`'s armhf index overwrites alpha | real |
| #412 | Debian bookworm: `sum.golang.org` stream error, twice | infra only |
| #369 | needs compileSdk 37 (#416) | real |

- Flakes 1 and 2 hit no open PR and failed in no CI run in 14 days:
  they show on this host's load (1) and outside `make race` (2).
- #412's error: `NFPM ?= go run github.com/goreleaser/nfpm/v2/cmd/nfpm@…`
  (Makefile:44) runs outside go.mod, so its modules are checked against
  sum.golang.org on every run.

Other flakes, last 14 days, all branches, each passed on a re-run:

- Go: `TestServeAppliesTheTokenOnSIGHUP` (2, two texts), `TestStreamResyncs` (2),
  `TestStreamResumes` (2), `TestPairingAndAddingToARoom`, `TestLogAlertsRoute`,
  `TestLogSourceFilter`, `TestApplyBoost`; armv6 `TestConfigWriteConfirmed`,
  `TestProxyRetryStops` (1 each).
- UI (passed on the in-job retry): `test_scroll` 34 (last 11-22),
  `test_send` 12 (11-15), `test_password_eye` 6 (11-23), `test_header` 6 (11-18),
  `test_log` 3, `test_unconfirmed_marks` 1 (#416).
