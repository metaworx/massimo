# NOTE TestInfrastructure v1.0: what the runs showed

Block 1, Runs: the runs of the analysis's section 9, what each showed,
where the run differed from the plan, and the blocks each bears on.
The commands, logs and overlays were scratch under `GUIDELINES/temp/testinfra/`;
every overlay was a build overlay, and no tracked file was changed for a run.

## What the runs showed

| # | Suspicion | Outcome |
|---|-----------|---------|
| 1 | The exact comparisons of the Cube's lines fail when a poll falls in the window | **Confirmed** |
| 2 | The starts without the greeting wait fail when the pump is slow | **Not seen**; four other tests fail every time |
| 3 | The log's UI tests meet the server's own lines | **Not seen** |
| 4 | Discovery tests skip under port load | **Not seen** |
| 5 | The `cubelan` rebinds fail under load | **Not seen** |
| 6 | `TestEventLinesMatchTheRows` passes with a doubled line | **Confirmed** |
| 7 | armv6 runs near the 5 s waits | **Not settled**: the package ran into Go's 10-minute timeout first |
| 8 | Races in the three packages `make race` leaves out | **None found** |

### 1. The exact comparisons

`go test -count=20 -run 'TestRemove|TestCopy|TestRepair' ./internal/server`,
30 tests, 600 runs each way:

- **As built:** all passed.
- **With `DefaultPollInterval` at 20 ms:** 60 failures in 6 tests,
  every one an `l:` inside the compared lines.
  - `TestRemoveRefused` and `TestCopyNeedsConfigure`, which check that nothing
    was sent: 20 of 20.
  - `TestRemoveListedDevice` 7, `TestRemoveResetsFlaggedMembers` 6,
    `TestRemoveLeavesHiddenDevicesAlone` 6, `TestRemoveDeviceInARoomByItsConfig` 1.
- **One place the review did not name:**
  `TestCopyNeedsConfigure` compares at `capabilities_test.go:260`,
  outside `sameLines`.

### 2. The slow pump

`internal/server` with the pump sleeping 100 ms before each event,
four passes (one, then `-count=3`):

- **As built:** one pass, 123 s, all passed.
- **Slowed:** 476 s a pass at load 7 to 12; 1433 s for three at load 12 to 25.
- **The starts the review suspected never failed:**
  none of the 49 tests on `startStored`, nor any on `startRooms`;
  of the about 200 tests whose start does not wait for the greeting,
  only the four below.
- **Four tests failed in all four passes,** and none other ever:

  | Test | Start | Fails with |
  |------|-------|------------|
  | `TestRemoveListedDevice` | `startCopyCube` | "timed out waiting for the events" at 5 s |
  | `TestProgramConfigEvents` | `startRoomsOpts` | no config event within its wait |
  | `TestFollowUpAfterAWrite` | inline | "no config event for the device" at 0.2 s |
  | `TestPolledStateChangesReachTheLog` | inline | no changes at 1.7 s |

  Each reads the log, or waits on events the pump hands on,
  at once or within a fixed time:
  the class of Block 4's barrier.

### 3. The log's UI tests

`test_log.py`, all 29 tests in the four cases, one worker,
the server's poll at 1 s:

- **1 s poll:** 116 passed, twice (load 6 to 8).
- **As built:** 116 passed, once (load 12).
- **Why:** the Cube client sends a state event only where a device's state
  changed (`internal/cubelan/snapshot.go:280`, `sameState`),
  and the fake Cube's states stay as they are,
  so a poll of the server brings no line of its own.
- **What does replace what a test put in** is the page's own reads:
  `loadCubes()` replaces `state.found` (the flaky test fixed by pull request #348).

### 4. The discovery skips

- **As the review wrote it,** `-race -count=20 -v -run Discovery`
  beside the race packages run over and over: 180 runs, no skip.
  It reaches 9 tests and few of the skip sites,
  which are mostly in tests without "Discovery" in their names.
- **The whole package** under `-race`, three times, beside the same load,
  was stopped after 21 minutes at load 20, no skip in 601 results:
  each test with a skip site had run about twice.
- **Aimed:** the 18 tests that reach a `t.Skip`, `-race -count=20` beside the
  same load (4 to 22): 360 runs in 756 s, all passed, no skip.

### 5. The rebinds

`-race -count=50` of `TestReconnectWhileDisconnected`, `TestALostCubeIsReportedOnce`,
`TestLostFromALossToTheNextGreeting` and `TestSetAddressCutsTheBackoffShort`
beside the race packages: 200 runs, all passed, 143 s at load 7.

### 6. The doubled line

With every event line written twice (`eventline.go`, `eventLine`),
`TestEventLinesMatchTheRows` passes, as it does as built.

### 7. armv6

`make test-armv6`'s command with `-v`, in the image `massimo-armv6-test`
with 4 CPUs (`docker run --cpus=4`), at load 2 to 14:

- **`internal/server` failed at Go's default 10-minute timeout,**
  no test hung: its tests add up to about 21 minutes.
  Every other package passed; 1172 results passed, 21 skipped.
- **No test failed on a wait.**
  A test's time is the sum of its steps,
  so the times do not show how near one wait came to its 5 s.
  The slowest: `TestNoRealIdentifiers` 34 s, `TestReleasedStoresDownAgain` 26 s,
  `TestUnconfirmedRoomCommandNotResent` 21 s, `TestRolesAgainstRoutes` 20 s.
- **In CI** the armv6 job's `internal/server` took 245 to 367 s
  over its last six runs, up to 61 % of the limit;
  `internal/store` 100 to 175 s.

### 8. The races left out

`go test -race -count=5 ./internal/engine ./internal/cubeio ./internal/certs`:
no race, 13 s in all.

### Seen on the way

- **`internal/server` takes longer than 10 minutes under `-race` here.**
  The race packages, run three times as load beside runs 4 and 5,
  all passed; `internal/server` took 711, 988 and 723 s,
  `internal/store` 432 to 513 s, at load 4 to 22.
  They passed because the runs gave `-timeout 60m`:
  `make race` gives none, and Go's default 10 minutes would have failed each.

## Different from the plan

1. **Not on an idle machine.**
   Other sessions run tests at any time (the owner, 2026-11-17),
   so each run's load is recorded instead; it went from 1 to 25.
   Runs 1, 2 and 6 provoke their fault through an overlay,
   and runs 4 and 5 want load beside them.
2. **The flaky `test_found_cube_with_removed_rows` was fixed first**
   (D11-A, D12-A): pull request #348.
   With the page's poll at 300 ms it failed eight times of eight;
   with the fix the file's 28 tests passed fourteen runs of fourteen.
3. **Run 3** named a UI test, `test_log_lines`, that does not exist:
   the whole of `test_log.py` ran instead.
4. **Run 4** was aimed at the 18 tests that reach a skip, as above.
5. **Run 2** had four passes, not ten: a pass took eight minutes, and the
   outcome was the same in every one.
6. **Run 7** ran in Docker with 4 CPUs, as a CI runner has, with `-v` added.
7. **The session ran the runs itself;** a run builds nothing,
   so no builder and no second reader.

## What it changes in the blocks

- **Block 3** also takes `capabilities_test.go:260`, the comparison outside
  `sameLines`.
- **Block 4** also takes the four tests of run 2.
- **Block 5** loses most of the gain the review gave it:
  of the about 200 tests on starts without the greeting wait,
  four failed with a slow pump, and none of the 49 on `startStored`.
  It keeps its own: one start in place of about twenty.
- **Block 7:** a quiet server set does not quiet the log's tests,
  whose foreign lines a poll does not bring.
  What a test injected is replaced by the page's own reads.
- **Block 9:** `make race` and `make test-armv6` both need a `-timeout`
  well above 10 minutes: each went past it here under load.
