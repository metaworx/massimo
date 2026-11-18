# AP TestSpeed v1.1: shorter test runs

## TL;DR

A full CI run takes 13 to 15 minutes, where it took 7 on 2026-11-14:
the tests grew, AP TestInfrastructure made nothing slower.
This plan cuts what runs where it is not needed, then what is slow.

- macOS and Windows jobs on every pull request → only where their code or the
  announcement changed (done, #371); a job no runner took → retried or counted
  as not run, and run before the nightly build
- SQLite under `-race`, 79 % of the server's race time → `checkptr` off for it
- UI test (1100 px), 12.6 min → a job per case, the screenshots apart, no video
  on the first pass
- no `t.Parallel()` anywhere → the server's and four packages' tests in parallel
- a cold Go build in every job → a cache per heavy job
- product timers waited out, a fresh store per test → shorter timers, a template

### Blocks

1. Gates: the macOS and Windows jobs gated by path on a pull request (done)
2. Retries: a job no runner took, on a push, a tag and before the nightly build
3. Times: the slowest tests named in each run
4. Race: `checkptr` off for SQLite
5. Ui: a job per case, the screenshots apart, no first-pass video, the guard deselected
6. Cache: the Go build cache per job, and the tests that need root
7. Parallel server: `internal/server`'s tests in parallel
8. Parallel rest: the store, the command line, the Cube client and `serve`
9. Stores: one migrated store per test binary
10. Timers: shorter product timers where a test waits them out

## Discussion

### What the owner asked for

On 2026-11-18, in these words:
"the macos launch daemon job gets cancelled more often than not due to the lack of
an available runner. could we gate it more specifically, that it runs only when
it's relevant code is changed. should we apply a similar logic to the windows
service then?"
Then: "would the macos service not also announce itself on the network? [...]
I think it should - as well as the linux build.
so I would extend D1-A's path list to include the part of D2A that is
announce-specific".
And, at the end of AP TestInfrastructure:
"please analyse why the tests have become so slow (long-running) and if there's
anything that can be done to cut the overall run time".
Then: "D5-A => any better proposal to spare pushes on main?",
and "D16 - how about B for main, and A for nightly and tags?".

### What is decided

The owner, 2026-11-18, `EXEC as recommended`:

- **D1-A, as the owner extended it.**
  On a pull request the macOS job runs for a change to its own paths
  or to the announcement's paths; it always runs on a tag.
- **D2-A.**
  The Windows job the same, with its own paths, the announcement's,
  and `internal/cubelan` and the server's discovery and search files.
- **D3-A.**
  The announcement's paths are kept once, and used by the LAN, macOS and
  Windows gates.
- **D4-A.**
  This plan holds them, and the tests' run time.

The owner, 2026-11-18, `D5A`:

- **D5-A.**
  A macOS or Windows job that never got a runner on a pull request counts as not
  run, and the push of the merged tree runs it.

The owner, 2026-11-18,
`EXEC D15/D16 as recommended, PLAN+ TestSpeed all A`:

- **D6-A.** `make race` passes `-gcflags=modernc.org/...=-d=checkptr=0`;
  the race detector stays on everywhere, and `checkptr` for the module's packages.
- **D7-A.** The UI tests run as one job per case, and the screenshots as a job
  of their own, on every run.
- **D8-A.** The first UI pass records no video; the retry records the video and
  the trace.
- **D9-A.** The parts guard's tests are deselected at collection in 1100-de and
  390-en, and the UI jobs do not run for `internal/server/*_test.go` alone.
- **D10-A.** A cache of the Go build per heavy job (Vet and test, Race, armv6),
  saved by `main`'s runs and restored by pull requests;
  `setup-go`'s own cache off where it saves an empty one;
  CodeQL's overlay entries of older commits pruned.
- **D11-A.** `t.Parallel()` in `internal/server`, `internal/store`,
  `cmd/massimo`, `internal/cubelan` and `internal/serve`,
  every test but those on a list with a reason
  (a package hook swapped, a fixed loopback address or port, a wall-clock bound),
  and a test that holds every test to it.
- **D12-A.** Measured after D6: if still worth it, one migrated store per test
  binary, copied for every test that is not about migrating.
- **D13-A.** After D11, a test that then waits out a package's timer gets a
  shorter one through an option, unless the length is what it checks.
- **D14-A.** Vet and test runs the three tests that need root once more as root,
  by name.
- **D15-B.** The nightly run first runs the macOS and Windows jobs,
  through a workflow of their own, on `main`'s head where a record says one did
  not run; it ships only once they passed.
- **D16-A, the owner's split.**
  On a push to `main`, a job no runner took counts as not run,
  and the record says so (`mac=false`, `win=false`), so that the devel build goes on;
  before the nightly build and on a tag, a job no runner took is run again,
  at most twice, about 20 minutes apart;
  where no runner came by then, the night is skipped, or the release fails.

## Analysis

The analysis this revision rests on is
`wip/2026-11-18_17-22_ANALYSIS_TestSpeed_why_the_tests_are_slow.md`.
What bears on the blocks:

- **Where the time goes:**
  a full run 13 to 15 minutes, 19 where runs queue;
  UI test (1100 px) 12.6 minutes, Race, Vet and test and armv6 about 7 each;
  locally `go test ./...` about 110 s, `make race` 10 to 23 minutes on this
  shared machine.
- **Growth, not AP TestInfrastructure:**
  `internal/server` 203 → 370 tests, the UI suite 386 → 736 items,
  the store's migrations 58 → 126 KB of SQL, between 2026-11-12 and 11-17;
  Race 2.6 → 7.1 minutes before #354, 7.2 after it;
  locally the plan's waits made the server's tests 5 to 9 % faster.
- **SQLite under `-race`:** the 166 server tests that open a fresh store take
  79 % of the package's race time; a fresh store costs four times what it did on
  11-12, half of it `checkptr` in `modernc.org/sqlite`;
  with `checkptr` off for it, store-heavy race tests took 45 to 50 % less.
- **No parallel tests:** the module has no `t.Parallel()`;
  `internal/server` uses 40 % of one core, and every Go job waits for it;
  25 server tests over 1 s each, most waiting out product timers,
  take half its time.
- **UI:** 44 % of a test is set-up and tear-down (a context, a page load, the video);
  the screenshots run after the suite in the same job;
  each of the guard's 28 skips loads a page first.
- **CI:** the restored Go cache holds 13 MB, so every job compiles from scratch;
  CodeQL's caches fill 9.6 of the 10 GB;
  a run is 16 jobs against GitHub's 20 at a time.
- **The skips:** the UI's 30 a job are the guard outside its two cases and two
  regions on its list; the Go jobs' are legitimate, but three tests that need root
  run in no CI job (D14).
- **The platform jobs (Block 1):** of the last six runs whose macOS job got no
  runner, four were pushes to `main`, where D5-A does not spare them (D16);
  the nightly run tests nothing today (D15).

## Implementation Plan

A sub-agent on Opus builds a block without a commit, a second reader reads it,
and the session verifies it and makes the commits.
Each block's pull request shows its saving against the analysis's medians.

### Block 1: Gates

Done, pull request #371:
the macOS and Windows jobs gated by path on a pull request (D1-A to D3-A),
a job no runner took on a pull request counted as not run (D5-A),
the gates held to the files Go builds per system.

### Block 2: Retries

- A push to `main`: a macOS or Windows job abandoned for want of a runner counts
  as not run, the record says `mac=false` or `win=false`, and the devel build goes
  on (D16-A).
- The nightly run: the two jobs first, through a reusable workflow,
  on `main`'s head where its record says one did not run;
  a job no runner took run again up to twice, about 20 minutes apart;
  the build ships only once they passed, and skips the night otherwise (D15-B).
- A tag: a job no runner took run again up to twice before the release fails.
- `GUIDELINES/project/CI.md`.

**Verification:**
`make actionlint`; the report's and the verdict's cases against `abandoned` on a
push, a tag and the nightly run; a run of the nightly workflow by hand.

### Block 3: Times

- The Go jobs' summary names the 20 slowest tests, from `go test -json`;
  `run.sh` passes `--durations=20`.

**Verification:** the pull request's run shows both.

### Block 4: Race

- `make race` with `checkptr` off for `modernc.org/...` (D6-A);
  `docs/testing.md`, CI.md.

**Verification:** `make race` before and after, alternated, at the same load;
the pull request's Race job against the median.

### Block 5: Ui

- A UI job per case and the screenshots a job of their own (D7-A),
  no video on the first pass (D8-A),
  the guard deselected outside its cases and the UI trigger without `_test.go`
  alone (D9-A);
  `docs/ui-testing.md`, `tools/ui-test/README.md`, CI.md.

**Verification:** the pull request's UI jobs against the median;
a failure planted for the retry shows its video and trace.

### Block 6: Cache

- The Go build cache per heavy job (D10-A); the three root tests as root (D14-A);
  CI.md.

**Verification:** a second run of the pull request restores a full cache;
the root tests run and pass.

### Block 7: Parallel server

- `t.Parallel()` in `internal/server`, the serial list with its reasons,
  and the test that holds every test to it (D11-A); `docs/testing.md`.

**Verification:** `go test` and `-race -count=20` of the package beside a busy
machine; the pull request's Go jobs against the median.

### Block 8: Parallel rest

- The same in `internal/store`, `cmd/massimo`, `internal/cubelan`
  and `internal/serve` (D11-A).

**Verification:** as Block 7.

### Block 9: Stores

- One migrated store per test binary, copied per test (D12-A),
  where Block 4's numbers leave it worth it; otherwise the block is dropped and
  a NOTE says why.

**Verification:** as Block 7.

### Block 10: Timers

- Shorter product timers through options for the tests that wait them out (D13-A).

**Verification:** as Block 7; each changed test still fails on a fault planted
for it.

## Proposed commit messages

- The plan: `chore(aps): +TestSpeed v1.0`;
  `chore(aps): +TestSpeed v1.1, +ANALYSIS TestSpeed`
- Block 1:
  `ci(tests): The macOS and Windows jobs run where their code or the announcement changed`
- Block 2: `ci(tests): A job no runner took is run again before a release`
- Block 3: `ci(tests): Each run names its slowest tests`
- Block 4: `build(make): make race leaves checkptr out of SQLite`
- Block 5: `ci(ui-test): A UI job per case, and the screenshots apart`
- Block 6: `ci(tests): The Go build cached per job, the root tests run as root`
- Block 7: `test(server): The server's tests run in parallel`
- Block 8: `test(store): The store's and the command line's tests run in parallel`
- Block 9: `test(server): One migrated store per test binary`
- Block 10: `test(server): Shorter timers where a test waits them out`

## Change History

- v1.0 (2026-11-18): first revision, by `EXEC as recommended`,
  with D1-A as the owner extended it and D2-A to D4-A;
  one block, the rest of the plan to come from the analysis of why the tests
  are slow.
- v1.1 (2026-11-18): by `EXEC D15/D16 as recommended, PLAN+ TestSpeed all A`:
  the analysis of why the tests are slow in the Analysis;
  D5-A folded in (NOTE TestSpeed v1.0, D5-A never got a runner),
  D6-A to D14-A as recommended, D15-B and D16-A as the owner chose;
  Block 1 done (#371); Blocks 2 to 10 new.
