# AP Flakes v1.0: Flaky tests at their cause

## TL;DR

Flaky tests seen on PR #410's run and in a survey of the open PRs' CI,
each fixed where it waits for the wrong thing, a commit each, one pull request
(`wip/2026-11-24_23-11_ANALYSIS_Flakes_three_tests_and_open_prs.md`).

- `TestSameConfigAtDebug` → `pumpSync` before its second mark:
  a re-read is applied twice, two config lines
- emulator suite → a test takes the app's tasks out of recents:
  Android 14 kills the next test 1 s after the orchestrator's `pm clear`
- emulator reporting → a killed instrumentation named in the job log,
  with the device's lines
- `test_unconfirmed_marks` → waits for what it reads
- `TestLogAfterReadsABurstOnce/store` → AP LooseEnds' Block 5 or here (D8);
  `docs/testing.md` on `-race` by hand
- `TestServeAppliesTheTokenOnSIGHUP` → AP StartConfiguration's Block 4
  or here (D9)

### Blocks

1. SameConfig: `TestSameConfigAtDebug` waits for the pump before its second mark
2. EmulatorTasks: each emulator test takes the app's tasks out of recents
3. EmulatorReport: the tally and `run.sh` report a killed instrumentation
4. UnconfirmedMarks: `test_unconfirmed_marks` waits for what it reads
5. RaceScale: `docs/testing.md` on `-race` by hand; the burst test as D8 says
6. Sighup: the SIGHUP test, only where D9 brings it here

## Discussion

### What is decided

The owner, 2026-11-25, `PLAN++` on the TL;DR gate of 2026-11-24:

- **D1-A.** `TestSameConfigAtDebug` is fixed in the test, with `pumpSync`.
- **D2-A.** The re-read's double apply in the product
  (`cubeio.Writer.Reread`, the proxy's `c:`) is a plan of its own,
  after this one.
- **D3-A.** The burst test: n=1000 and a failure that says how far the reader
  got - now open again as D8.
- **D4-A.** `docs/testing.md` says how a `-race` run by hand takes
  `make race`'s scale and flags.
- **D5-A.** The emulator tests take their tasks out of recents
  (`finishAndRemoveTask`).
- **D6-A.** The tally prints a failure's first lines;
  `run.sh` prints the device's kill and crash lines of a failed or short run,
  and warns, without failing, where a passing run still shows the app's
  "Destroy timeout of remove-task".
- **D7-B.** The other flakes come into this plan,
  `TestServeAppliesTheTokenOnSIGHUP` and `test_unconfirmed_marks`;
  the open PRs' real failures go to the sessions that wrote them
  (#415's `READS`, #414's Windows and armv6 tests): sent on 2026-11-25.

### Decided in the writing

- **The order:** the blocks no other plan holds come first,
  the two that another open plan already names last (D8, D9),
  so that the run can stop before them without holding the others up.
- **The pull request** is opened after Block 4,
  so that CI's emulator job proves Block 2 while D8 and D9 are open;
  Blocks 5 and 6 join it.

### Open

- **D8. `TestLogAfterReadsABurstOnce/store` is AP LooseEnds' too.**
  Its Block 5, item 4, makes the one wait over the burst a wait for progress:
  the test fails where no page reads a new line for `testkit.Wait`, scaled.
  That fixes the same timeout another way, on the same lines.
  - **A (recommended):** the test stays AP LooseEnds'.
    Its session gets this plan's measurements
    (`RACEGCFLAGS` halves the burst; n=1000 keeps what the test checks),
    and Block 5 here is the `docs/testing.md` paragraph alone.
    One change to the lines, by the plan that named them first;
    a wait for progress holds under a plain `-race` too.
  - **B:** this plan does n=1000 and the failure's message now,
    and AP LooseEnds' session drops its item 4 in a revision.
    Costs another plan's revision; two changes of the lines where it does not.
- **D9. `TestServeAppliesTheTokenOnSIGHUP` is AP StartConfiguration's.**
  Its Block 4 makes the test wait for the reload's end,
  the race of the failure "GET /api/v1/restart: 200" without `"pending"`.
  - **A (recommended):** it stays AP StartConfiguration's,
    and its session gets the second failure text, which its analysis does not
    name: `reload_test.go:246: has not come: the store checkpointed`
    (run 44906936277, attempt 1). Block 6 falls away.
  - **B:** this plan fixes the test now, and AP StartConfiguration's Block 4
    keeps only `apiReload`. Costs that plan's revision,
    and its Block 4 changes `reload()` beside a test fixed here.

## Analysis

The causes, the measurements and the survey are in
`wip/2026-11-24_23-11_ANALYSIS_Flakes_three_tests_and_open_prs.md`.
What each block rests on:

### For Block 1: the second config line

`cubelan`'s `handle` puts a C reply into the snapshot and emits `EventConfig`
before `Send` returns it;
`cubeio.Writer.Reread` (`internal/cubeio/cubeio.go:104-109`) applies it again
(`ApplyLocal`), a second event.
The test's first `logAfter` takes the first line;
the pump may write the second after the second `logMark`,
from the snapshot before the second re-read's reply, at debug, unchanged,
and the second `logAfter` takes it.
`pumpSync` (docs/testing.md, "The pump") is the wait for it.
Widened (100 ms in the pump, the fake's `c:` 300 ms late): 30 of 30 failed,
0 of 30 with `pumpSync`.
Beside two `-race` runs of the package: 22 of 1100 failed, 0 of 1100.

### For Block 2: the kill after `pm clear`

The orchestrator clears the app's data between tests
(`clearPackageData`, `app/android/app/build.gradle.kts:45`).
Android 14 removes the app's tasks from recents with a kill;
where the last test's activity is still closing, the kill waits 1 s
(`KILL_TASK_PROCESSES_TIMEOUT_MS`) and then takes every process of the package,
the next test's instrumentation among them where it has not attached yet.
2 of 11 suite runs lost a test this way; the timer fired in 6 of 10.
`ActivityManager.getAppTasks()` returns the tasks the clear would remove,
and `AppTask.finishAndRemoveTask()` removes one without a kill:
with none left, the clear starts no timer.

### For Block 3: what the job log said

Nothing: the tally wrote "not among the tests the sources declare" for the
orchestrator's own record `android-test#testRunFailed`,
whose text ("Test instrumentation process crashed") it did not print,
and the decisive lines were in a 12 MB `logcat.txt` of the artifact.
A filter over the app's kill, crash and attach lines gives under ten lines
on each of the three logs read (two failures, one pass).

### For Block 4: the devices tab

`test_confirm.py::test_unconfirmed_marks[390-en]` (`:101`) failed once in
14 days, on #416's run 45486067658:
`Locator.inner_text: Timeout 5000ms exceeded`,
waiting for a `.cfg-device`'s `.device-unconfirmed` after the room's devices
tab is opened; 2.8 s on the in-job retry.
The block finds what the test reads before the tab has drawn it.

### For Block 5: `-race` by hand

The burst's wait takes the scale already (`For(4*testkit.Wait)`).
A plain `go test -race` differs from `make race` twice:
scale 1 instead of 2, and `checkptr` on for `modernc.org/sqlite`
(`RACEGCFLAGS`), which doubles a store-heavy test.
`GUIDELINES/shared/lang/go/TESTING.md` and `docs/store.md` name a plain
`go test -race ./...`.

## Implementation Plan

Each block is a commit on `claude/flaky-three`; Blocks 1 to 4 then go to one
pull request, and Blocks 5 and 6 join it.
Locally only the touched packages and test files run; CI runs the rest.
Every `go test` runs with `MASSIMO_ENV_FILE=/dev/null`.

### Block 1: SameConfig

- **Goal:** `TestSameConfigAtDebug` holds under load.
- **Changes:** `internal/server/logdetail_test.go`:
  `pumpSync(t, s)` after `fc.SetConfig`, before the second `logMark`,
  with a comment on the second config line.
- **Tests:** the widened overlay (a sleep in the pump, a late `c:`)
  fails the old test and passes the new one, 20 runs each;
  under load, `-count=20` turns beside two `-race` runs of the package.
- **Documents:** none; docs/testing.md's "The pump" already says it.
- **CHANGELOG:** none: only a test changes, said in `Key changes`.
- **Commit:** `test(server): The unchanged C line's test waits for the pump before the second re-read`
- **Verification:** the widened run and a load run on the committed test,
  each by its exit code; `go test ./internal/server`;
  `gofmt -l`, `go vet` for linux, windows and darwin.

### Block 2: EmulatorTasks

- **Goal:** no emulator test is killed by the previous test's clear.
- **Changes** (`app/android/app/src/androidTest/`):
  1. `Rig.kt`: `removeTasks()`, every task of `ActivityManager.appTasks`
     finished and removed, a failure said, not thrown.
  2. `EmulatorTest.kt`: its `after` closes the scenario, then removes the tasks.
  3. `ConnectPageTest.kt`: an `@After` that removes the tasks.
- **Tests:** the androidTest sources compile; CI's emulator job runs the suite,
  and its `logcat.txt` has no "Destroy timeout of remove-task" for the app's
  task (6 of 10 runs before).
- **Documents:** `tools/app-emulator/README.md`, in Block 3.
- **CHANGELOG:** none: only tests change.
- **Commit:** `test(app): An emulator test takes the app's tasks out of recents after it`
- **Verification:** the androidTest compile; the emulator job on the pull
  request, and its artifact's `logcat.txt`.

### Block 3: EmulatorReport

- **Goal:** a killed instrumentation explains itself in the job log.
- **Changes:**
  1. `tools/app-emulator/tally/main.go`: a failure's first lines under its
     case; `android-test#testRunFailed` named as the run's own record.
  2. `tools/app-emulator/run.sh`: on a failed or short run, the device log's
     kill, crash and attach lines of the app; a warning where a passing run
     shows the app's "Destroy timeout of remove-task";
     the crash buffer saved beside the log.
  3. `tools/app-emulator/README.md`, "The results": the device's lines,
     the warning, and the race.
- **Tests:** `tally/main_test.go` cases for a failure's lines and
  `testRunFailed`; the filter run over the three saved logs.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(app-emulator): A killed instrumentation is named in the job log, with the device's lines`
- **Verification:** `go test ./tools/app-emulator/...`; shellcheck of
  `run.sh`; `GUIDELINES/shared/tools/check`.

### Block 4: UnconfirmedMarks

- **Goal:** `test_unconfirmed_marks` waits for the devices tab's rows.
- **Changes:** `tools/ui-test/test_confirm.py`, the wait found to be missing.
- **Tests:** a delay planted where the cause is fails the old test and
  passes the new one; `test_confirm.py` in the four cases.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): The unconfirmed marks test waits for the devices tab`
- **Verification:** `make ui-test` over `test_confirm.py`, with ports of its
  own and the main checkout's venv; `GUIDELINES/shared/tools/check`.

### Block 5: RaceScale

- **Goal:** a `-race` run by hand is as `make race` sizes the waits.
- **Changes:** `docs/testing.md`, "The scale": the race's waits are sized for
  `make race`, and how a run by hand takes its scale and `RACEGCFLAGS`.
  With D8-B also `internal/server/logafter_test.go`: n=1000,
  and a timeout that names how far the reader got.
- **Tests:** with D8-B, the store subtest under a plain `-race` in the package.
- **CHANGELOG:** none.
- **Commit:** `docs(testing): A run of -race by hand takes make race's scale and flags`
- **Verification:** `GUIDELINES/shared/tools/check`; with D8-B,
  `go test -race -count=1 ./internal/server`.

### Block 6: Sighup

Only with D9-B: `cmd/massimo/reload_test.go` waits for the reload's end,
each failure text's race seen failing with a planted delay and passing after.
Commit: `test(cli): The SIGHUP test waits for the reload's end`.

## Proposed commit message

The plan: `chore(aps): +Flakes v1.0, +ANALYSIS Flakes`

Block 1:

```
test(server): The unchanged C line's test waits for the pump before the second re-read

For AP Flakes v1.0, Block 1 (D1-A): TestSameConfigAtDebug failed under
load because a re-read gives two config lines, Send's reply and
Reread's own apply; the pump could write the second after the test's
second mark, from the old snapshot, and the test took it for the
change's. It now waits for the pump before that mark.

Key changes:
- internal/server/logdetail_test.go: pumpSync before the second mark.
- No CHANGELOG bullet: only a test changes, and nothing shipped
  behaves differently.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `PLAN++` on the TL;DR
  gate of 2026-11-24: D1-A to D6-A, D7-B; D8 and D9 open.
