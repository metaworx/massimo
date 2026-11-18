# ANALYSIS TestSpeed: why the tests are slow

For the owner, who asked on 2026-11-18 at the end of AP TestInfrastructure
"why the tests have become so slow (long-running) and if there's anything that can be
done to cut the overall run time", and why so many tests are skipped.
It is the analysis AP TestSpeed v1.0 names for its next revision.
Nothing tracked was changed.
The scripts, the raw `go test -json` and pytest outputs and the CI data are in
`GUIDELINES/temp/testinfra/speed/`, which is scratch and may go at any time.

## TL;DR

What is slow: a full CI run takes **13 to 15 min**, 19 when runs queue;
it took 7 on 2026-11-14.
Its longest job is **UI test (1100 px)** at 12.6 min,
ahead of Race, Vet and test and armv6 at about 7 each.
Locally `go test ./...` takes about 110 s,
and `make race` 10 to 23 min on this shared machine.

Why:

- **Growth, not the plan:**
  `internal/server` 203 → 370 tests, the UI suite 386 → 736 items,
  the store's migrations 58 → 126 KB of SQL, between 2026-11-12 and 2026-11-17.
  Race went 2.6 → 7.1 min **before** #354, and 7.1 → 7.2 after it.
- **AP TestInfrastructure made nothing slower:**
  locally it is 5 to 9 % faster (`internal/server` 106.5 → 97.3 s),
  and in CI the same per package.
  Its scale costs nothing in a passing run.
- **Under `-race`, SQLite is the cost:**
  the 166 server tests that open a fresh store take 79 % of the package's race
  time.
  A fresh store under `-race` costs four times what it did on 11-12 (1.7 s at a low load),
  and half of that is `checkptr` inside `modernc.org/sqlite`.
- **No package runs its tests in parallel:**
  the module has no `t.Parallel()`.
  `internal/server` uses 40 % of one core natively,
  and every Go job waits for it.
- **25 server tests take over 1 s each,** most of them waiting out product timers:
  together half of the package's native time.
- **UI: 44 % of a test is set-up and tear-down**
  (a new context, a page load, the video),
  and the screenshots run after the suite in the same job.
- **CI compiles everything from scratch:**
  the Go cache it restores holds 13 MB.
  A run is 16 jobs, and GitHub runs 20 at a time, so parallel sessions queue.

The three biggest savings:

1. **`make race` without `checkptr` for `modernc.org/...`:**
   one flag.
   Store-heavy race tests take 45 to 50 % less time,
   so Race goes from about 7.2 to 5 min, and `make race` here about 40 % less.
2. **The UI jobs, one per case,** with the screenshots in a job of their own
   and no video on the first pass:
   the critical path 12.6 → about 6 min.
3. **Parallel Go tests, `internal/server` first:**
   Race → about 3 min, Vet and test −1 min,
   `go test ./...` here 110 → about 40 s.

Together a full run takes about 7 min.

The skips: one group hides tests that should run.

- **Go, Vet and test:**
  5 skips. 3 need root and **run in no job**;
  1 is Windows-only and runs in the Windows job;
  1 is a measurement behind an environment variable.
- **Go, armv6:**
  19 skips: those 5, and 14 that build or copy the binary under `-short`.
- **UI:**
  30 a job, all in `test_parts_guard.py`. 28 are the guard in the case where it
  does not run, by design (`GUARD_CASES`),
  and each loads a page before it skips; 2 are regions on its list.

The decisions are D6 to D14, after the plan's D5;
Blocks 2 to 9 follow its Block 1.

## 1. What was measured

- **CI:**
  the 520 finished runs of `ci.yml` from 2026-11-12 to 2026-11-18,
  with each run's jobs (`gh api …/runs/<id>/jobs`),
  and 96 job logs for the per-package `ok … <s>` lines.
  These are the medians of the jobs that passed.
  A run counts as after a block once its commit holds the block's merge,
  or once it is the block's own branch.
  Runners vary by ±30 %, and the periods after a block have 1 to 6 runs.
  #367's run (Block 9) is listed apart.
- **Locally:**
  this machine has 16 cores and is shared.
  The load was 2 to 40 during the runs, and each run's load is in its `.meta`.
  Three trees, each a `git archive` with its own `make ui`:
  now (`claude/testkit-ci`, main with Block 9),
  before #354 (`90f7c572`), and 2026-11-12 (`1665cd90`).
  The packages are those of the brief, `-json -count=1`.
- **UI:**
  one case (1100-en, 4 workers) with `--durations=0`,
  the guard's file in 1100-de,
  and `test_room_updates.py` with and without video,
  each with 1 worker and twice.

## 2. CI over time

Job medians in minutes (number of runs):

| Job | 11-12 | 11-14 | 11-15 | 11-17, before #354 | after, #354 to #361 | #367 (B9) |
|-----|------:|------:|------:|------:|------:|------:|
| Vet and test | 5.2 (28) | 5.4 (51) | 5.5 (76) | 6.3 (17) | 7.3 (14) | 7.7 |
| Race | 2.6 (28) | 3.5 (52) | 3.7 (76) | 7.1 (17) | 7.2 (14) | 6.4 |
| Test (armv6, QEMU) | 2.8 (28) | 3.4 (52) | 3.9 (73) | 6.3 (17) | 7.1 (14) | 7.7 |
| UI test (1100 px, screenshots) | - | 5.7 (9) | 7.8 (68) | 11.6 (13) | 12.6 (14) | 11.5 |
| UI test (390 px) | - | 4.1 (6) | 5.4 (59) | 8.2 (14) | 8.8 (13) | 11.5 |
| macOS LaunchDaemon | 6.4 (26) | 4.2 (52) | 4.6 (76) | 6.7 (13) | 6.9 (14) | 7.6 |
| Windows service | 2.6 (28) | 2.7 (52) | 2.7 (76) | 2.9 (17) | 2.9 (14) | 3.6 |
| Fuzz (push only) | 3.1 (16) | | 3.0 (37) | 3.2 (6) | 3.2 (11) | - |
| LAN against avahi | 1.8 (17) | | 4.0 (58) | 4.2 (6) | 4.2 (7) | 4.3 |
| **A full run, start to All tests passed** | | 7.2 | 9.1 | 12.9 | 13.0 to 18.6 | 14.8 |

Block by block the medians move within the runners' spread (Race 6.0, 7.5, 9.5,
7.8, 6.2, 7.4 for Blocks 3 to 8; UI 1100 px 12.3, 12.3, 11.2, 15.4, 12.6, 12.7).

What one test step costs, in seconds,
`internal/server`'s own `ok` line (two runs each):

| Job | 11-12 | 11-13 | 11-14 | 11-15 | 11-16 | 11-17 | after, B2 to B8 | #367 |
|-----|------:|------:|------:|------:|------:|------:|------:|------:|
| Vet and test | 34, 38 | 45, 50 | 56, 57 | 66, 69 | 91, 101 | 90, 89 | 86 to 101 | 91 |
| Race | 96, 115 | 149, 124 | 155, 139 | 228, 173 | 398, 373 | 343, 482 | 299 to 480 | 317 |
| armv6 | 90, 96 | 116, 144 | 131, 169 | 209, 170 | 362, 301 | 265, 265 | 231 to 368 | 370 |
| Race, `internal/store` | 74, 91 | 103, 80 | 88, 75 | 125, 78 | 230, 212 | 188, 283 | 162 to 284 | 170 |

What grew (on `main` at the day's end; the UI's items from that day's runs):

| | 11-12 | 11-13 | 11-14 | 11-15 | 11-16 | 11-17 | 11-18 |
|-|------:|------:|------:|------:|------:|------:|------:|
| `internal/server` tests | 203 | 244 | 267 | 333 | 368 | 369 | 370 |
| `internal/store` tests | 87 | 93 | 100 | 101 | 119 | 119 | 119 |
| store migrations (files) | 17 | 21 | 22 | 23 | 26 | 26 | 26 |
| UI items a job (2 cases) | | | 386 | 578 | 650 | 728 | 736 |

- **Inside a job:**
  Race's step is `internal/server`'s run (about 50 s to compile,
  then 300 to 480 s of tests; `internal/store` runs beside it).
  Vet and test is `go vet` 95 s, `go test` 115 s and the cross build 204 s (77,
  52 and 162 s on 11-12).
  UI 1100 px is the suite 640 s,
  the screenshots 66 s and the container 27 s. macOS's step "Install, run,
  update and remove the LaunchDaemons" went 38 → 157 s (11-15 → after),
  outside the tests; AP TestSpeed's Block 1 gates that job.
- **UI per item:**
  0.78 → 0.86 s an item in the 1100 px job, 0.62 → 0.65 in the 390 px one,
  before against after.
  The job grew with its items.
- **The cache:**
  every Go job restores `setup-go-Linux-x64-ubuntu26-go-1.26.11-1995a6…`, 13 MB,
  since go.sum changed on 2026-11-15.
  It was 134 MB before.
  The first job to end saves it, and by its size that is a light one,
  so the heavy jobs compile from scratch.
  The repository's caches stand at 10.7 GB, above GitHub's 10 GB:
  CodeQL's overlay databases take 9.6 GB (334 entries, one set a commit),
  setup-go's 0.55 GB, and the least recently used entries are evicted.
- **The queue:**
  a full run is 16 jobs and 71 runner-minutes (43 on 11-14).
  The Free plan runs 20 jobs at a time.
  On 11-16 and 11-17 a full run's last test job started a median 5 min after What to test,
  p90 12 to 16 min.
  #367's jobs waited 3 min.

## 3. Locally, per test

`go test -json -count=1` of the brief's packages, in seconds,
a package's own time:

| Package | now (2 runs) | before #354 (2 runs) | 11-12 |
|---------|------------:|-------------------:|------:|
| `internal/server` | 100.0, 102.7 (370 tests) | 108.7, 122.9 (369) | 42.1 (203) |
| `cmd/massimo` | 32.4, 42.6 | 32.7, 33.2 | 33.2 |
| `internal/store` | 24.7, 31.4 | 24.5, 24.8 | 14.2 |
| `internal/cubelan` | 19.8, 19.8 | 22.6, 22.5 | 17.4 |
| `internal/cubelan/fakecube` | 8.1, 9.4 | 7.6, 7.9 | 1.2 |
| `internal/serve` | 6.1, 6.1 | 5.9, 6.5 | 5.0 |
| whole run, wall | 101 s, 104 s | 109 s, 124 s | 43 s |

- **The plan:**
  the 699 tests in both trees, each at its faster run,
  take 186.7 s now and 197.7 s before; `internal/server` 97.3 against 106.5 s.
  The starts became faster with Blocks 2 and 5:
  many tests of 0.28 to 0.31 s now take 0.11 to 0.16 s.
- **Since 2026-11-12:**
  `internal/server`'s 167 new tests add 50.7 s;
  its 203 old ones went 42.0 → 49.1 s (`TestAuditOfEveryWrite`,
  `TestRolesAgainstRoutes` grew with the routes).
- **Where `internal/server`'s 97 s go:**
  62 tests without a start take 4 s;
  142 with a start and no store take 53 s (median 0.07 s);
  159 with a start and a store take 40 s (median 0.14 s).
  The 25 tests over 1 s take 46.7 s.
- **CPU:**
  the server's test binary used 46 s of CPU in 115 s (40 % of one core).
- **Under `-race`** (`MASSIMO_TEST_SCALE=2`, load 15 to 40): 1356 s,
  1053 s of CPU (77 %).
  The 166 tests that open a store take 1070 s (median 4.7 s);
  the other 204 take 285 s (median 0.13 s).
  Race against plain is a median 26×,
  and 80 to 110× for the API's store-heavy tests (`TestPreferencesRoutes`,
  `TestOIDCSignIn`).
- **A fresh store under `-race`** (`TestOpenMigratesAnEmptyFile`, alternating,
  same load): 1.0 s in the 11-12 tree, 4.0 s now and before #354.
  At load 2 to 10 it is 1.7 s now; the migrations' SQL went 58 → 126 KB.
  The profile: 50 % `runtime.checkptr*` inside `modernc.org/sqlite`,
  40 % its SQL parser.
- **`-gcflags=modernc.org/...=-d=checkptr=0`**, two runs each at load 2 to 10:
  a fresh store 1.68 → 0.86 s;
  seven store-heavy server tests 40.9 and 37.9 → 22.4 and 18.9 s (−45 to −50 %).
  Whole packages, `internal/server` and `internal/store` side by side:
  without `checkptr` 321 and 142 s (load 2 to 10);
  with it, `internal/store` took 615 s at load 15 to 20,
  which the load confounds, so the alternating runs above are the measure.

The slowest tests (faster of two runs, s):

| Test | Package | now | before #354 | What it waits for |
|------|---------|----:|------------:|-------------------|
| `TestNetconfigBroadcastRecovery` | `cmd/massimo` | 10.00 | 10.00 | netconfig's waits for an answer, run against a fake that falls silent |
| `TestUnconfirmedRoomCommandNotResent` | server | 5.96 | 6.27 | the resend windows (0.3 to 2 s), five subtests |
| `TestRestoreRefusals` | server | 3.53 | 3.53 | a queue gap of 500 ms between steps |
| `TestSearchMarksTheAnswers` | cubelan | 3.17 | 3.16 | `askWait` + `lateWait` |
| `TestDiscoveryDHCPFlag` | server | 3.17 | 3.16 | a Cube that never answers `c`: the search's waits |
| `TestOneSerialFromACubeAndAProxy` | server | 3.16 | 3.16 | the same |
| `TestOpenDiscoveryListen` | serve | 3.06 | 3.06 | serve's default discovery wait, 3 s |
| `TestNetConfigRoundTrip` | cubelan | 2.99 | 2.99 | `setSettle`, `rereadWait` |
| `TestReleasedStoresDownAgain` | store | 2.46 | 2.03 | downgrades of whole stores |
| `TestUnconfirmedRoomCommandResent` | server | 2.22 | 2.28 | the resend window |
| `TestRolesAgainstRoutes` | server | 2.21 | 2.18 | every route as every role |
| `TestDowngradeRefusesAnOpenStore` | store | 2.15 | 2.12 | |
| `TestACallDuringARunGetsTheNext` | server | 2.12 | 2.10 | `DiscoveryWait: 1 s`, twice |
| `TestUnconfirmedRoomCommandResentBehindTheHold` | server | 2.11 | 2.18 | the resend window |
| `TestALostCubeIsSearchedForWhileLost` | server | 2.07 | 2.07 | fixed sleeps of 0.6 and 0.5 s against the backoff |
| `TestReleaseAndRestartFreeTheCube` | fakecube | 2.05 | 2.04 | |
| `TestServeAnswersRequestInFlightAtStop` | `cmd/massimo` | 2.05 | 2.16 | a built serve's stop |
| `TestWaitHealthyOverTLS` | `cmd/massimo` | 2.01 | 2.01 | the health poll |
| `TestNetconfigAgainstTheFakeCube` | `cmd/massimo` | 2.00 | 2.00 | netconfig's waits |
| `TestAuditOfEveryWrite` | server | 1.88 | 1.82 | every write route |

## 4. The UI suite

1100-en alone, 4 workers, load 17 to 37: 649 s; 365 passed, 2 skipped,
and 1 flaky (`test_stream_ticket_waits_for_the_sign_in`,
which passed on the retry).

- **The time of a test:**
  2524 s in all: set-up 755 (30 %), the test 1414, tear-down 355 (14 %).
  A median test spends 2.0 s in set-up (a new context, `goto`, the rooms,
  the stream) and 0.9 s in tear-down (the video, the context,
  the store's check).
- **The files:**
  `test_parts_guard.py` 387 s on one worker (15 % of the case);
  `test_unsaved.py` 199; `test_log.py` 142; `test_cubes.py` 117;
  `test_header.py` 92.
  `--dist loadfile` still balances: 649 s against an even share of 631 s.
- **The slowest tests:**
  the guard's regions rooms 34.9 s, roles 34.8, cubes-list 30.8,
  appearance 20.6; `test_language_redraw` 25.3; `test_register_and_link` 23.1;
  `test_rules_kept_on_the_server` 21.7; `test_sign_out_asks_first` 20.8;
  `test_passwords_registered_in_the_browser` 20.8;
  `test_to_top_on_every_page` 20.6.
- **The guard where it does not run** (1100-de): its 28 skips take 42 s,
  each a full page load first.
  conftest's autouse `guard` asks for `page` before the file's `guard_case` skips.
  A scratch pytest shows the same order.
- **Video:**
  `test_room_updates.py` (23 tests, 1 worker) took 54.4 and 44.0 s with video,
  and 39.7 and 40.3 s without.
  Tear-down went 12 and 10 → 5 and 5 s: about −0.3 s a test,
  more for short tests.
- **Fixed waits:**
  91 `wait_for_timeout`s and `time.sleep`s, 34 s a case, about 17 s a job.

## 5. What costs the time

- **No `t.Parallel()`** in any package:
  a package's time is the sum of its tests.
  `go test -p` runs packages side by side,
  so each Go job lasts as long as `internal/server`'s serial run.
  On CI's 4 CPUs, Race runs `internal/server` and `internal/store` beside each other,
  each on about one core.
- **SQLite under `-race`:**
  `modernc.org/sqlite` is C translated to Go.
  Under `-race` every memory access is instrumented, and `checkptr`,
  which `-race` turns on,
  checks every `unsafe.Pointer` step of it. 166 server tests and most store
  tests migrate a new file through 26 migrations (491 statements,
  the 0024 raise with its down part among them).
- **Product timers the tests wait out:**
  cubelan's `askWait`, `lateWait`, `setSettle` and `rereadWait` (1 to 3 s);
  the queue's gap where a test sets 500 ms; the resend and confirm windows;
  netconfig's waits in `cmd/massimo`.
  Most are already variables "so that tests can shorten them",
  but not from `internal/server`.
- **The start** (`startServer`): about 0.07 s, and 0.14 s with a store,
  natively.
  The greeting is read once.
  The connect, `pumpSync` and the flush return the moment they hold.
- **The kit:**
  a 10 ms tick, so a wait returns on average 5 ms after its condition.
  The scale (2 for `make race`,
  4 for armv6) lengthens only a wait that would fail.
  No passing wait takes the scale: no `Within` is used as "nothing came",
  and `Never` ends on its end condition.
- **Sleeps left:**
  11 in `internal/server` (0.05 to 0.6 s), 11 in `internal/cubelan`,
  7 in `cmd/massimo`, together under 5 s.
  `Never`: 10 call sites, each with an end that comes.
- **The build in CI:**
  13 MB of cache, so `go vet` takes 95 s, the cross build 204 s,
  and Race and armv6 each about 50 s before their first test.
  Block 9's `-count=1` changes nothing there: no CI log shows a `(cached)` test.
- **UI:**
  4 workers on the runner's 4 CPUs.
  Each test gets a new context and a page load, the video recorded throughout,
  for both cases of the job.
  The screenshots (66 s) run after the suite in the same job;
  the second pass runs only after a failure.
- **The queue:**
  GitHub Free's 20 jobs.
  Since Block 9 the UI jobs also run for a change to any `internal/server/*.go`,
  `_test.go` files included.

## 6. Why it got slower

- **What AP TestInfrastructure made slower:**
  nothing measurable.
  Locally it is 5 to 9 % faster.
  In CI `internal/server`'s step is 89 to 90 s before and 86 to 101 s after,
  and under race 343 to 482 s before and 299 to 480 s after.
  The UI's time an item is +5 to +10 %,
  within the runners' spread and the new tests.
  Block 9 adds the three race packages (5 to 8 s, beside the server's run),
  and the UI jobs for pull requests that change a server Go file.
  `pumpSync`, the `Never`s and the scale cost milliseconds in a passing run.
- **What was already slow:**
  the serial packages; the 25 tests that wait out a timer;
  the UI's set-up of a page for every test; SQLite under `-race`;
  the near-empty CI cache (since 11-15).
- **What grew with new tests** (most of it, between 11-12 and 11-17):
  - `internal/server` ×1.8 in tests, so ×2.4 natively and ×3.5 under race.
  - The UI suite ×1.9 in items, so the 1100 px job ×2.2.
  - The store's migrations ×2.2 in SQL, so a fresh store under `-race` ×4,
    paid by every test that opens one.
  - That jump came with AP StoreRaise's blocks on 11-16:
    Race's `internal/server` went 173 to 228 → 373 to 398 s that day,
    and `internal/store` 78 to 125 → 212 to 230 s.

## 7. The skips

| Where | Skipped | Why | Hides something? |
|-------|--------:|-----|------------------|
| Vet and test (Linux) | 5 | `TestOwnerKeptAsRoot`, `TestRunsAsTheStoresOwner` and `TestRollbackStoreRunsAsTheStoresOwner` need root; `TestNoServiceUserOnWindows` is Windows-only; `TestRaiseTimes` measures only with `MASSIMO_STORE_RAISE_ROWS` | **the three root tests run nowhere:** no job runs Go tests as root, though the runner has `sudo`; they test what an update does as root (the store's owner, a file's owner). The Windows job runs the Windows one; the measurement is by hand (docs/store.md) |
| Race | 1 | `TestRaiseTimes`, the only one of the five in a race package | no |
| Test (armv6, QEMU) | 19 | the 5, and 14 under `-short`: tests that build the binary or run copies of the test binary (`launchBuiltServe`, `massimoBinaries`, `keptBinary`), which QEMU cannot | no: Vet and test runs them natively |
| Windows, macOS | not reported | subsets by `-run`, no `-json` | unknown; docs/testing.md says so |
| UI 1100 px | 30 of 736 | 28: `test_parts_guard.py` in 1100-de, where the guard does not run (`GUARD_CASES`: 1100-en and 390-de only); 2: `room-config` on its not-moved list, and `sign-in`, which the fake Cube's fixture cannot show | no; but the 28 are no skips in substance, and they load a page each |
| UI 390 px | 30 of 736 | the same, with 390-en and 390-de | the same |

`test_home.py`'s test and the guard's `home-notice` region skip without
`run.sh`'s second server.
`run.sh` always starts it, so they run in CI and in `make ui-test`.
Deselecting the guard's 28 tests outside its cases would leave "2 skipped",
both of them real notes.
The UI jobs went from 33 to 36 skips (11-15,
11-16) down to 30 as the regions left the list.

## 8. What can be done

Ranked by saving for the effort.
The savings are estimated from the numbers above,
for a CI run unless "locally" says otherwise.

| # | What | Saving | Effort | Risk |
|---|------|--------|--------|------|
| 1 | `make race` builds `modernc.org/...` with `-d=checkptr=0`; the module's own packages keep it | Race 7.2 → about 5 min; `make race` here −40 % | one flag | low: checkptr stays on for our code; SQLite's own tests are upstream's |
| 2 | UI jobs: one per case (four), the screenshots a job of their own | the critical path 12.6 → about 6.5 min | small: the matrix | 3 jobs more a run, so more queueing; each case pays the container (27 s) |
| 3 | No video on the first UI pass; the retry records it with the trace | about −10 % of a UI job | one flag | the first failure has a screenshot and no video; the retry has both |
| 4 | The guard deselected outside its two cases | −40 s on the guard's worker; "30 skipped" → 2 | small | none |
| 5 | The UI jobs not for `internal/server/*_test.go` alone | 21 runner-minutes a test-only pull request | one pattern | none: a test file does not change the binary |
| 6 | A Go build cache that holds the build: a key per heavy job, saved by main's runs; CodeQL's old overlay entries pruned | Vet and test −2 to −3 min; Race and armv6 −40 s | small | the quota: about 1 to 2 GB of Go caches beside CodeQL's |
| 7 | One migrated store per test binary, copied for each test not about migrating | under `-race` perhaps a further −30 % of the store-heavy tests; locally −20 s | medium | a test that should meet the migrations; measure first |
| 8 | `t.Parallel()` in `internal/server`, then `internal/store`, `cmd/massimo`, `internal/cubelan`, `internal/serve`, with a list of the serial ones | Race → about 3 min with 1; Vet's tests 115 → about 60 s; armv6 −2 min; locally `go test ./...` 110 → about 40 s | large: 370 + 290 tests, a checking test | medium: hidden shared state, which the race detector shows; timing under contention, which the kit now absorbs |
| 9 | The tests over 1 s get a shorter product timer through an option, where the length is not what they check | about −30 s of serial time natively; after 8 only the longest matter | medium | a timing that becomes too tight on a loaded runner |
| 10 | Shard `internal/server`'s race run across 2 to 3 jobs | Race → about 3 min | small | jobs more, queueing; unneeded with 8 |
| 11 | A smaller scale | none: a passing wait does not take it | | slower runners fail |
| 12 | armv6 on main only | 7 runner-minutes a pull request | small | a 32-bit fault found after the merge; not recommended |
| 13 | `-short` for slow tests in Vet and test | none on the critical path | | less covered |
| 14 | The three root tests run as root in Vet and test | none; about +10 s | small | none: they skip today |

With 1 to 6, a full run's longest job is about 6.5 min (UI), then Race about 5,
so a run takes about 7 min instead of 13 to 15.
With 8 the Go jobs drop to 3 to 4 min,
and locally `go test ./...` takes about 40 s and `make race` a few minutes.

AP TestSpeed's Block 1 (the macOS and Windows jobs gated by path) is not in the table:
it saves those jobs' 10 runner-minutes on a pull request that does not touch them,
and so some of the queue, not a run's length.

## 9. Decisions for the owner

Numbered after AP TestSpeed v1.0's D1 to D5,
since the plan's next revision rests on this.

- **D6.
  `checkptr` for SQLite under `-race`.**
  - A (recommended): `make race` passes
    `-gcflags=modernc.org/...=-d=checkptr=0`;
    the race detector stays on everywhere,
    and `checkptr` for the module's packages.
  - B: as now.
- **D7.
  The UI jobs.**
  - A (recommended): one job per case, and the screenshots a job of their own,
    on every run as today.
  - B: one job per case, and the screenshots only on a push to main.
  - C: as now.
- **D8.
  The video of the first UI pass.**
  - A (recommended): off; `run.sh`'s retry records the video and the trace.
  - B: as now.
- **D9.
  The guard outside its cases, and the UI trigger.**
  - A (recommended): the guard's tests are deselected at collection in 1100-de and 390-en,
    and the UI jobs do not run for `internal/server/*_test.go` alone.
  - B: only the deselection.
  - C: as now.
- **D10.
  The Go build cache.**
  - A (recommended): a cache of the build per heavy job (Vet and test, Race,
    armv6), saved by main's runs and restored by pull requests,
    with `setup-go`'s own cache off where it saves an empty one,
    and CodeQL's overlay entries of older commits pruned.
  - B: as now.
- **D11.
  Parallel Go tests.**
  - A (recommended): `t.Parallel()` in `internal/server`, `internal/store`,
    `cmd/massimo`, `internal/cubelan` and `internal/serve`,
    every test but those on a list with a reason (a package hook swapped,
    a fixed loopback address or port, a wall-clock bound),
    and a test that holds every test to it.
  - B: `internal/server` alone.
  - C: none; shard the server's race run across jobs instead (option 10).
- **D12.
  Fresh stores in tests.**
  - A (recommended): measured after D6; if still worth it,
    one migrated store per test binary,
    copied for every test that is not about migrating.
  - B: as now.
- **D13.
  Product timers in tests.**
  - A (recommended): after D11, the tests that then set a package's time
    get a shorter timer through an option, unless the length is what they check.
  - B: a fake clock through those timers.
  - C: as now.
- **D14.
  The tests that need root.**
  - A (recommended): Vet and test runs the three of them once more as root (`sudo`),
    by name, about 10 s.
  - B: as now, run by hand only.

## 10. Blocks for AP TestSpeed v1.1

After Block 1 (Gates).
Each block shows its saving against this analysis's medians in its pull request's run.

2. **Times:**
   the Go jobs' summary lists the 20 slowest tests from `-json`,
   and `run.sh` passes `--durations=20`,
   so that every later block shows its saving.
3. **Race:**
   `checkptr` off for `modernc.org/...` in `make race` (D6); `docs/testing.md`,
   CI.md.
4. **Ui:**
   a job per case and the screenshots beside them (D7),
   no video on the first pass (D8),
   the guard deselected and the trigger without `_test.go` (D9);
   `docs/ui-testing.md`, `tools/ui-test/README.md`, CI.md.
5. **Cache:**
   the build cache per heavy job (D10), and the root tests (D14); CI.md.
6. **Parallel server:**
   `internal/server` (D11), with the serial list and its test,
   under `-race -count=20` beside a busy machine; `docs/testing.md`.
7. **Parallel rest:**
   `internal/store`, `cmd/massimo`,
   `internal/cubelan` and `internal/serve` (D11).
8. **Stores:**
   the migrated template (D12), if Block 3's numbers leave it worth it.
9. **Timers:**
   the tests that set a package's time (D13).
