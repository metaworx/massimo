# AP FuzzDeadline v1.0: Go's fuzzing deadline passes

## TL;DR

`make fuzz` no longer fails a target on Go 1.26's own `-fuzztime` deadline,
the race of golang/go#75804, which failed the Fuzz job twice on 2026-11-09.
The parsers stay as they are: none of them hangs.

- `make fuzz`'s loop in the Makefile → `tools/fuzz/run.sh`,
  called with `FUZZTIME` and `FUZZPKGS`
- a target whose only failure message is `context deadline exceeded`,
  no input written → passes, with a warning naming the issue
- a toolchain whose `internal/fuzz` has Go 1.27's fix → nothing passes that way:
  the workaround ends by itself
- `GUIDELINES/project/CI.md`, sections 2 and 3; a `### Fixed` bullet

### Blocks

1. `tools/fuzz/run.sh`, the Makefile's `fuzz` calling it,
   `GUIDELINES/project/CI.md` and `CHANGELOG.md`

## Discussion

Asked by the owner on 2026-11-09,
after the Fuzz job failed on FuzzConfig with `context deadline exceeded`
and no crash input:
Go's harness, or a hang in the parser?

**The owner's decisions** (`PLAN++`, the TL;DR gate's recommendations,
2026-11-10):

- **D1-A:**
  accept exactly this failure, as Go 1.27 does.
  Not B, a retry of the target,
  which costs `FUZZTIME` a hit and proves nothing more; not C,
  the Fuzz job on Go 1.27, which leaves local runs failing; not D,
  `-fuzztime` as a count, whose time varies fourfold between targets; not E,
  waiting for Go 1.26.11.
- **D2-A:**
  the script reads the toolchain's own `internal/fuzz/fuzz.go` for the fixed condition.
  Not B, a version check, wrong if the backport slips past 1.26.11;
  not C, always on, which needs someone to remove it.

## Analysis

The evidence is in
`GUIDELINES/wip/2026-11-10_00-04_ANALYSIS_FuzzDeadline_context_deadline_exceeded.md`;
in short:

- `DecodeConfig` and `DecodeWeekProgram` run in time bounded by their input,
  and the failed run's executions grew to its end;
- Go 1.26's `CoordinateFuzzing` drops the deadline's error only when it equals
  `fuzzCtx.Err()`,
  and `context` closes the outer context's `Done` before it cancels `fuzzCtx`,
  so the main loop can see the deadline first and keep it;
- Go 1.27.0 also compares with `ctx.Err()` (CL 774140, CL 804900);
  the 1.26 backport, golang/go#82362, is approved for 1.26.11 and not merged;
- 1 of 24 local runs failed so, on 4 cores, with no new input to minimise;
  2 of the last 31 Fuzz jobs did, on FuzzLine and on FuzzConfig.

### What the script accepts

The output of the target's `go test` run,
from its `--- FAIL: <target> (` line on, carries exactly one indented line,
`    context deadline exceeded`.
A crash adds its message and `Failing input written to …`,
a hung worker `fuzzing process hung or terminated unexpectedly`,
and another target's failure has another name:
none of them matches.
The fixed toolchain returns no error in that case,
so passing it is what Go 1.27 does.

### How it ends

`harness_fixed` looks for `err == ctx.Err() || err == fuzzCtx.Err()`
in `$(go env GOROOT)/src/internal/fuzz/fuzz.go`.
Every Go distribution carries its source there.
Should the line change upstream, the script goes on accepting the one
output a fixed harness no longer produces, which costs nothing.

## Implementation Plan

### Block 1: `make fuzz` passes Go's own deadline

1. `tools/fuzz/run.sh`:
   the loop of the Makefile's `fuzz`,
   each package's `Fuzz*` targets one after the other for `FUZZTIME`,
   stopping at the first that fails;
   the target list read before it is looped over,
   so that a package that does not build fails the run rather than fuzzing nothing;
   each run's output through `tee` into a temporary file; on a failure,
   `harness_fixed` or `deadline_only` decide,
   and an accepted one prints a line naming golang/go#75804,
   as a `::warning::` under GitHub Actions.
2. `Makefile`, `fuzz`: `FUZZTIME=$(FUZZTIME) tools/fuzz/run.sh $(FUZZPKGS)`;
   its comment names the script.
3. `GUIDELINES/project/CI.md` v1.21.0: section 2's Fuzz job,
   section 3's `make fuzz` bullet.
4. `CHANGELOG.md`, `### Fixed`: a bullet.

**Verification:**

- `deadline_only` against the two failed jobs' output,
  FuzzLine's and FuzzConfig's: accepted;
  against a crash, a second message, a pass and another target's name:
  refused;
- the script in a scratch module:
  a target that panics fails the run and leaves its input;
  one that passes passes;
  one whose `go test` output is the deadline's alone passes with the warning on Go 1.26,
  and fails where `harness_fixed` answers yes;
- `bash -n`, `shellcheck`;
  `make fuzz` in full;
  `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `GUIDELINES/shared/tools/check`'s changelog, reflow and version checks.

## Proposed commit message

```
fix(fuzz): Go's own fuzzing deadline no longer fails make fuzz
…
```

## Change History

- v1.0 (2026-11-10): the first revision, by `PLAN++`: D1-A, D2-A.
