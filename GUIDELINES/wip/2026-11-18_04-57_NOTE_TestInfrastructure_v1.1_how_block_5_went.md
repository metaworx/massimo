# NOTE TestInfrastructure v1.1: how block 5 went

Where Block 5, Start, went differently from the plan,
what it left and for which block, and what verified it.

## What it made

- **`startServer(t, opts...)`** in the server's tests,
  and the rig it returns: the fake Cube and its watched lines, the server,
  its API, the runtime, the store, a log where asked, and `stop`.
  It returns once the Cube is connected, the pump has handled the greeting
  (`pumpSync`) and the store's writer has written the start's lines.
- **Its options:** the server's options, quiet, a log, a store, a greeting or
  a change to it, the fake Cube, other Cubes, a restart over an earlier start;
  and bundles of them in place of the helpers' particular starts
  (`storedRooms`, `confirming`, `repairing`, `discovering`, …).
- **The recorded greeting read once** for the server's tests;
  each test takes a copy of its own (`recorded(t)`).
- **`testkit.Clock`:** one clock a test moves on, its timers run in their
  order as it does.
- **`docs/testing.md`:** the start, its options and what it waits for;
  the clock.

## Different from the plan

1. **Who did it.**
   One builder on Opus, then one second reader;
   the session made the reader's small fixes and the commits.
2. **Two commits, one pull request:**
   the clock with `notify`'s tests on it, then the start and the server's tests.
3. **About twenty start helpers are gone** (`start`, `startWith`,
   `startStored`, `startRooms`, `startDiscovering`, …, about 300 call sites),
   and 18 starts written out by hand use the start.
   The drift, slots, mismatch and moving Cubes' rigs and `fixedCube` keep their
   names and build on it (D15-A).
   `TestNothingStartsAfterRun` keeps its own: it tests `Run`'s start and end.
4. **The greeting's loads:** the analysis counted 90 across the repository.
   The server's 51 are one; the other packages' 37 calls of
   `fakecube.LoadGreeting` stay, since they are not the server's starts.
   A loader they shared would need a test-only home outside the fake Cube.
5. **Five clocks, two:**
   the server's three and `notify`'s are `testkit.Clock`;
   the drift tests keep `checkClock`,
   real time with an offset, so that the Cube's time moves on by itself.
6. **The start's cleanup waits for `Run` to end,**
   and four restarts now stop the first `Run` before the second start
   (`TestBootstrapThroughServer`, `TestLockoutSurvivesRestart`,
   and after the second reader `TestFoundCubeSurvivesRestart`
   and `TestChangedWhileServerDown`, which ran two servers on one store).
7. **`quiet()` polls once an hour,** where one helper polled every 2 minutes:
   no test runs that long.
8. **The slow pump's run takes longer** (454 to 772 s):
   every start waits for the greeting through it.
   A plain run of the server's package takes as long as before (about 103 s).

## Seen, left for later blocks

For Block 8 (Holes):

- `TestRefusedRequestsAuditedPerMinute`: "at most one" row within the minute,
  where rows are written as the minute ends, so an early row passes.
- `TestFollowUpGivesUpAndRespectsTheRefreshSwitch`:
  a 300 ms sleep, then exactly three re-reads and one connection.
- `TestLogSurvivesRestart`: the bound `len(all) < 4` is weak.

## Verification

- **The builder:** `go test ./...` (109 s);
  under `-race` at scale 2, `internal/server` three times (1356 s);
  the slow pump over the server's package, every test passed (772 s);
  at load 0.3 to 1.9.
- **The second reader** found nothing that must be fixed:
  414 test functions before and after, no store, poll, greeting or wait lost
  in a move.
  The session fixed its points: the two restarts, `again` making a Cube a row
  only of a store the first start made its own, the start's fake closed even
  where the start fails, and three lines of `docs/testing.md`.
- **The session, before the commits:**
  `gofmt`, and `go vet` for linux, windows and darwin;
  `go test ./...` (109 s at load 0.9 to 1.3);
  the slow pump over the server's package once (776 s);
  under `-race` at scale 2, beside it: the kit and `notify` 10 runs,
  the moved restart, reset, confirm, slots, drift, discovery, reboot,
  restore, audit and auth tests 2, `internal/server` once, all passed;
  the first commit's tree vetted on its own, and its tests run.
- **After the merge of `main`,** which brought only Block 4's NOTE:
  the guidelines check (12 of 12).
- **Left to CI:** `make test-armv6` at scale 4, `make race`,
  the Windows and macOS jobs.
- **Pull request #358.**
