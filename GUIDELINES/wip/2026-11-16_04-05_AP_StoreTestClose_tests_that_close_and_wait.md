# AP StoreTestClose v1.0: Tests that close and wait

## TL;DR

Three tests fail on main without any change: one on Windows because it
leaves a store open, two under `-race` on a loaded machine because they
outrun the server's pump. Only tests change.

- **TestMigration12:** second store, opened at version 12, never closed →
  Windows refuses `t.TempDir()`'s cleanup; `openAt` registers the close
  (D1-A).
- **TestDisagreeResendsMassimosSetpoint** (`drift_test.go:190`, 2 of 3
  loaded runs), **TestVirtualWallThermostatEvents** (`vwt_test.go:222`):
  suspected the class of TestWallReset (#329) → each confirmed by holding
  the pump back, then fixed to wait for the pump's own work (D2-A).
- **TestFailedRadiogram,** the loaded run's third: fixed by #332 → out.
- **Proof:** Windows run of `internal/store`; `-race -count=50` of the
  two, and `-race -count=3` of `internal/server`, at 16 busy loops.
- **No CHANGELOG bullet:** tests only (COMMIT.md §5.2).

### Blocks

1. TestMigration12 closes its stores: `openAt` registers the close
2. TestDisagreeResendsMassimosSetpoint waits for the pump
3. TestVirtualWallThermostatEvents waits for the pump
4. Proof under load, and the gate

## Discussion

The owner's brief, 2026-11-16: fix TestWallReset under `-race` and
TestMigration12 on Windows, and report any other flaky test a loaded run
of `internal/server` shows. TestWallReset was fixed by #329 before this
session started, so it is out; so is TestFailedRadiogram, which the
loaded run found and #332 fixed while it ran. The TL;DR gate offered D1
and D2; the owner answered `PLAN+ as recommended`.

**Decisions:**

- **D1-A:** `openAt` (`internal/store/events_sources_test.go:19`)
  registers `t.Cleanup(func() { s.Close() })`, so no caller can leave its
  store open. A store a test closes itself is closed once more at the
  cleanup: `sql.DB.Close` is idempotent, and the second close's
  checkpoint error is dropped. (B, a cleanup in TestMigration12 alone,
  left the trap in the helper.)
- **D2-A:** the server tests are Blocks 2 and 3 of this plan, so that
  the loaded run is green at its end; one pull request. (B was a plan of
  their own.)

## Analysis

### TestMigration12 on Windows

Run on 2026-11-16 as the owner's Windows tests run: `GOOS=windows go
test -c` into a copy of the tree under `%TEMP%`, the `.exe` through WSL
interop with `ProgramData` set to a temp directory and
`MASSIMO_ENV_FILE=NUL` (the real `service.log`'s mtime unchanged). The
whole package: TestMigration12 is its only failure, all four subtests,
`unlinkat …\massimo.db: The process cannot access the file because it is
being used by another process`.

`roles_migration_test.go:56` opens the migrated store with
`openAt(t, dir, 12)` and never closes it. The other `openAt` callers
(TestMigration9, 15, 21, 22) close theirs and reopen with `open`, which
registers the close (`store_test.go:34`). With D1-A the explicit
`s11.Close()` stays: the reopen needs the first store closed.

### The loaded run of internal/server

`MASSIMO_ENV_FILE=/dev/null go test -race -count=3 -timeout 45m
./internal/server/`, origin/main at the merge of #330, 16 busy loops on 16
CPUs, load average 27.7 at the start: FAIL after 2363 s.

| Test | Runs failed | Where | Message |
|------|-------------|-------|---------|
| TestDisagreeResendsMassimosSetpoint | 2 of 3 | `drift_test.go:190` | `the report: []` |
| TestFailedRadiogram | 1 of 3 | `failedradiogram_test.go:80` | `timed out waiting for the line that set the flag` |
| TestVirtualWallThermostatEvents | 1 of 3 | `vwt_test.go:222` | `no event` |

No `DATA RACE` in the log. TestFailedRadiogram's cause was found and fixed
by #332, merged while the run went: the test acted on the snapshot before
the pump had written the report's line. The other two are left. The pump builds each state event's view from
the client's snapshot when it handles the event, not when the line came
(#329's finding). A test that waits on the snapshot or the API and then
acts can therefore run ahead of the pump's memos, its log lines and its
events; and the 5 s deadlines of `waitFor` and `next` are short at this
load. Suspected per test, each confirmed in its block before a fix:

- **Disagree:** `set` waits on the client's snapshot, then `check` runs
  `roomChecks`; the report comes from the log (`events`), which the store
  writes after the pump. Either the check reads state the pump has not
  folded in yet, or the report is read before it is written.
- **VWT events:** the member's event came, the room's (000002) did not
  within 5 s; the room's virtual event follows from the pump's handling
  of the member.

### The failure at load 12 on 2026-11-16

Not captured then; one of the loaded run's three is the likeliest, but it
cannot be proven now. Block 4's loaded run is the check.

## Implementation Plan

Branch `claude/storetestclose` from origin/main after the plan's
`chore(aps): +StoreTestClose v1.0` on main. One commit per block, `test:`,
no CHANGELOG bullet (tests only, COMMIT.md §5.2).

### Block 1: TestMigration12 closes its stores

1. `openAt` registers `t.Cleanup(func() { s.Close() })` after the open,
   its comment saying so.
2. Commit `test(store): TestMigration12 closes the store it reopens`.

**Verification:** the Windows run of the whole `internal/store` package
passes (as in the Analysis, `service.log`'s mtime unchanged);
`MASSIMO_ENV_FILE=/dev/null go test -count=1 ./internal/store/` passes on
Linux.

### Block 2: TestDisagreeResendsMassimosSetpoint waits for the pump

1. Confirm: hold the pump back (a temporary sleep before its snapshot
   read, as #329 did) and see the test fail every run; find which step
   runs ahead.
2. Fix the test to wait for the pump's state event, or for the log's
   line, at that step; if the cause is in `roomChecks` or the pump, not
   the test, stop and gate it.
3. Commit `test(server): TestDisagreeResendsMassimosSetpoint waits for the pump`.

**Verification:** with the pump held back, the old test fails and the new
one passes; the temporary sleep removed; `-race -count=20` passes.

### Block 3: TestVirtualWallThermostatEvents waits for the pump

As Block 2, for the room's event at `vwt_test.go:222`.
Commit `test(server): TestVirtualWallThermostatEvents waits for the pump`.

**Verification:** as Block 2.

### Block 4: Proof under load, and the gate

1. 16 busy loops; `-race -count=50 -run
   'TestDisagreeResendsMassimosSetpoint|TestVirtualWallThermostatEvents'
   ./internal/server/`.
2. 16 busy loops; `-race -count=3 -timeout 45m ./internal/server/`; a
   new flaky test found is reported, not fixed, in the completion gate.
3. The gate: `make ui`, `MASSIMO_ENV_FILE=/dev/null go test ./...`,
   `make cross`, `gofmt -l .`, `go vet` for linux, windows, darwin;
   `GUIDELINES/shared/tools/check` before the push.
4. Push the branch, open the pull request for the owner.

**Verification:** both loaded runs pass; the gate is clean; the results
in a NOTE on how the run went.

## Proposed commit message

One per block, as in the Implementation Plan; the first:

    test(store): TestMigration12 closes the store it reopens

    TestMigration12 failed on Windows: it opened the migrated store with
    openAt at version 12 and never closed it, so Windows refused to remove
    t.TempDir()'s massimo.db (AP StoreTestClose v1.0, Block 1). openAt now
    registers the close itself, so no caller can leave its store open; a
    store a test closes itself is closed once more at the cleanup, which
    sql.DB allows.

    Key changes:
    - internal/store/events_sources_test.go: openAt closes its store at
      the test's cleanup
    - Checked: the whole package's test binary on Windows passes, run
      through interop with ProgramData in a temp directory
    - No CHANGELOG bullet: only tests change (COMMIT.md 5.2)

    Assisted-by: claude-code:claude-opus-5-5

## Change History

- **v1.0** (2026-11-16): first revision, from the owner's brief and the
  TL;DR gate's `PLAN+ as recommended` (D1-A, D2-A); TestFailedRadiogram
  left out, fixed by #332.
