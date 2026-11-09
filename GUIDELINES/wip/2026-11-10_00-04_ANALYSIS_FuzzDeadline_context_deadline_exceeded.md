# ANALYSIS FuzzDeadline: "context deadline exceeded" in the Fuzz job

Main's Fuzz job failed on 3394385 (run 42712716109) with
`--- FAIL: FuzzConfig (10.96s)` / `context deadline exceeded`.

**Verdict:** a race in Go's fuzzing coordinator at the `-fuzztime`
deadline, golang/go#75804, fixed in Go 1.27.0 and not yet in Go 1.26.
The parser does not hang.

## Evidence

**The parser cannot hang.**
FuzzConfig calls `DecodeConfig` and `DecodeWeekProgram`
(`internal/protocol/cube/config.go`).
Both run in time bounded by the input:
one copy of the input, fixed offsets after a length check,
and a loop of at most 7 × 13 slots.
They have no other loop and no recursion.

**The run did not stall.**
Executions rose to the end: 183k, 399k, 626k, then 702k at the deadline
(CI, 4 workers).
A hang would show the count stop growing, as no step of this run did.
No input was written: the artifact `fuzz-crash` holds only the two seeds
already committed (FuzzMetadata, FuzzParseHash).

**The harness has the race (Go 1.26.8 here, 1.26.10 in CI).**
In `internal/fuzz/fuzz.go`, `CoordinateFuzzing` wraps the context
in `WithTimeout(-fuzztime)` and derives `fuzzCtx` from it.
At the deadline its main loop wakes on the outer context's `Done` and
calls `stop(ctx.Err())`.
`stop` drops the error only `if err == fuzzCtx.Err()`.
But `context` closes a parent's `Done` *before* it cancels the parent's
children (`cancelCtx.cancel`).
In that window `fuzzCtx.Err()` is still `nil`,
so `context.DeadlineExceeded` is kept as the run's error.
`testing` then prints it under `--- FAIL`, with no input written.
A 40-line program with the same two contexts sees the window in
35 of 20,000 deadlines when idle,
and in 2,758 of 20,000 while two goroutines create children of `fuzzCtx`
(the workers' minimisation contexts do the same).

**Upstream.**
golang/go#75804, "testing: Fuzz testing with `fuzztime` flag sometimes
fails incorrectly with "context deadline exceeded"", is closed:
CL 774140 on master and CL 804900 for release-branch.go1.27 add
`err == ctx.Err() ||` to that condition.

| Toolchain | The condition in `stop` |
|-----------|--------------------------|
| go1.26.10, release-branch.go1.26 | `err == fuzzCtx.Err() \|\| isInterruptError(err)` |
| go1.27.0, go1.27.3 | `err == ctx.Err() \|\| err == fuzzCtx.Err() \|\| isInterruptError(err)` |

The 1.26 backport, golang/go#82362, is `CherryPickApproved`,
milestone Go1.26.11, and not yet merged.
CI's `go-version: '1.26'` with `check-latest` picks up 1.26.11 once it
ships.

**Reproduced.**
24 runs of
`MASSIMO_ENV_FILE=/dev/null go test -run '^$' -fuzz '^FuzzConfig$' -fuzztime 10s ./internal/protocol/cube/`:
12 on all 16 cores all passed;
of 12 under `taskset -c 0-3` (CI's 4 cores), the first failed
exactly as CI did (`--- FAIL: FuzzConfig (10.10s)` /
`context deadline exceeded`).
That run found no new interesting input (`new interesting: 0`),
so no worker was minimising: minimisation widens the window but is not
needed for it.
No file appeared under `testdata/fuzz/`.

**How often in CI.**
Of the last 40 pushes to `main`, 31 ran the Fuzz job and 2 failed, both
this way, on different targets:
FuzzLine on a5a40f3 (2026-11-09 19:58 UTC) and FuzzConfig on 3394385.
That is about 1 in 15 runs.
`make fuzz` stops at the first failing target (`set -e`),
so the targets after it went unfuzzed in those runs.

## Options

- **A. Accept exactly this failure on a toolchain without the fix.**
  A target whose only failure message is `context deadline exceeded`,
  and that wrote no input, passes with a line naming golang/go#75804.
  That is what Go 1.27 returns in this case: nothing.
  This fixes local runs and CI alike.
  It retires itself once the toolchain carries the fixed condition.
  A prototype of the check accepts the CI log and rejects a crash log,
  a log with a second message, a pass, and another target's name.
- **B. Retry the target once on the same signature**, failing if it comes
  back: the same check, plus 10 s of fuzzing whenever the race hits.
- **C. The Fuzz job on Go 1.27**: fixes CI only; the job then fuzzes with
  another toolchain than the release is built with, and local
  `make fuzz` keeps failing now and then.
- **D. `-fuzztime` as a count** (`500000x`, as one reporter does):
  there is no deadline, so no race.
  But the time per target varies with the target
  (FuzzMetadata about 170k runs in 10 s, FuzzConfig about 700k)
  and with the runner.
- **E. Nothing; wait for Go 1.26.11** and re-run failed jobs by hand.
  Until then about 1 run in 15 fails,
  and the targets after the one that failed go unfuzzed.
