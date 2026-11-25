# Finding: two server tests lose a race under armv6 emulation

Written 2026-11-20 by the session "UI refactor",
from a read-only investigation by an Opus sub-agent.
Nothing was changed: the tests are not this session's.

## TL;DR

The CI job "Test (armv6,
QEMU)" failed twice in a row on pull request #382 (AP UiParts' Block 8),
each time in a different test of `internal/server`,
and passed on the third attempt.
The branch did not cause it (the sub-agent: 85 to 90 % sure):
both tests are the same on main,
and under Go the branch changes one line of `ui.go`.
Each test has a timing flaw that shows where the emulator is slow and
the server's tests run in parallel.
They will meet other pull requests too.

## What failed

| Attempt | Test | Message |
|---|---|---|
| 1 | `TestConfigWriteConfirmed` | `confirm_test.go:306: timed out after 20s waiting for the C line shows it` |
| 2 | `TestProxyRetryStops` | `proxy_retry_test.go:158: after DELETE, the old address 127.0.0.1:… listens` |
| 3 | none | the job passed |

Every other job passed on the first attempt.
The job is skipped on pushes to main, so it runs almost only on pull requests;
since the server's tests went parallel (AP TestSpeed,
Block 7) it had run five times before #382, all green.

## Why each can fail

- **`TestConfigWriteConfirmed`.**
  It passes fixed times that the test scale does not reach: re-reads at 50,
  50 and 100 ms (`shortFollow`, `confirm_test.go:281`),
  and the fake Cube applies a command 150 ms after it came
  (`fakecube/apply.go:56-57`).
  The last re-read comes about 200 ms after the write: a margin of about 50 ms.
  Where the re-read wins,
  the write is marked late and stays open for `confirmWithin`, a minute,
  so the 20 s wait runs out.
- **`TestProxyRetryStops`.**
  It takes a port from the kernel, closes it, sleeps 100 ms unscaled
  (`proxy_retry_test.go:154-155`) and then asserts that nothing at all
  accepts on that port.
  The server's own slot is ruled out by the code;
  what is left is another parallel test, or another package's test binary,
  taking the freed port in that window.
  `ports_test.go` names this hazard in its own comment.

## Not ruled out

- Whether main itself became a little slower lately;
  five runs are too few for a rate.
- Which process took the port; the logs of parallel tests are mixed.
- Nothing was run locally, under QEMU or otherwise.

## Options

- **A (recommended): fix both tests at the source.**
  - `TestConfigWriteConfirmed`:
    its own follow-up times with the last re-read well behind the fake Cube's
    apply, scaled; or a short `confirmWithin`,
    so that a lost race fails at once and says so.
  - `TestProxyRetryStops`:
    keep the old address held (`heldPorts`) and assert on the server's state,
    in place of dialling a freed port.
- **B: scale the fixed times** these tests pass.
  `docs/testing.md` keeps sleeps and option times unscaled on purpose,
  so that rule would change, and the window only shrinks.
- **C: run the two in series,** or the armv6 job with less parallelism.
  It hides the flaws and slows the job.
- **D: accept a re-run.**
  CI already records a job that passed only on a re-run.
  The two keep coming back.
