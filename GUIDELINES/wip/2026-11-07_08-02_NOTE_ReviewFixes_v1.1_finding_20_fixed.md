# NOTE ReviewFixes v1.1: finding 20 fixed

Corrects item 1 of [the run's
NOTE](2026-11-07_03-06_NOTE_ReviewFixes_v1.1_how_the_go_stack_went.md),
which left finding 20 of [the
analysis](2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md)
as not reproduced:
at its completion gate on 2026-11-07 the owner chose D7-A,
fix it on Block 2's branch,
and pointed at the Windows side of the WSL host for its tests.

## Reproduced

`TestServiceStopWaitsForServe` (`cmd/massimo/service_windows_test.go`)
hands the service's handler a Stop and expects serve's context ended,
the handler waiting until serve returned,
a stop pending with a wait hint,
and the stop's lines in `service.log`.
Go is not installed on the Windows side,
so the package's test binary was cross-compiled with `GOOS=windows`
and run there through interop:

- On Block 2's branch before the fix,
  with only the names the test calls added, without their behaviour:
  `--- FAIL: TestServiceStopWaitsForServe`,
  "the handler returned on the stop without ending serve's context".
- After it: passes five runs in a row,
  and so do the rest of `cmd/massimo`'s tests with `-test.short`,
  on both branches;
  the two that build the binary with `go` skip there,
  and CI's Windows job runs them.

## Fixed

On #73's branch, merged into #75's:
the stop ends serve's context,
reports stop pending with a wait hint of 20 seconds,
answers the manager's interrogations,
and returns once serve did,
which wrote the log's pending events and closed the store;
`main` does not end the process before the manager heard the service stopped.
A serve that has not returned within the 20 seconds is ended all the same,
and a serve that fails by itself still ends the process unreported,
so that the manager's recovery actions restart it.

## What went differently

1. **The service log of the box**:
   the first runs of the test,
   before it pointed `ProgramData` at a temporary directory,
   appended 14 lines (`running`, `stop requested`, `serve stopped`)
   to `C:\ProgramData\Massimo\service.log` of the service installed there,
   timed 07:57:02 to 07:57:41 on 2026-11-07.
   Nothing else of that service was touched,
   and the lines are left where they are.
