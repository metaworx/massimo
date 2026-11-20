# NOTE TestSpeed v1.1: how blocks 3 to 10 went

Where Blocks 3 to 10 went differently from the plan, what verified them,
and what the run found beside them.

## The result

A pull request's run with every job, before the plan (a run of 2026-11-18)
and after it (Block 10's, #381):

| Job | Before | After |
|-----|-------:|------:|
| Race | 9.4 min | 2.7 min |
| Vet and test | 7.3 min | 6.4 min |
| armv6 | 7.7 min | 2.8 min |
| UI | 16.2 and 12.0 min | five jobs of 2.1 to 4.0 min |
| macOS, Windows | 7.0 and 2.9 min | 5.7 and 2.6 min, where their code changed |
| The whole run | 16.6 min | 6.9 min |

The merge's run on `main`, with the build and the release, took 7 min 39 s.

Locally, after Block 10, `go test ./...` takes 13 s where it took 110,
and `make race` 62 s where it took 507;
its slowest package is the store, 60 s, most of it the migration tests,
which open new stores on purpose.

## Block 3, Times (pull request #374)

1. **Beyond the plan, the 10 slowest packages:**
   the jobs wait for their slowest package, and the parallel tests of Blocks 7
   and 8 cut a package's time while the sum of its tests' stays the same.
2. **A package from go test's cache** is left out of the tables and counted,
   since its times are an earlier run's.

## Block 4, Race (pull request #375)

1. **Measured by alternated runs at the same load:**
   whole packages beside each other were confounded by the load.
   The store-heavy server tests 43 % less, `internal/store` 52 % less,
   `make race` 289 s where it took 507.
2. **Still caught:**
   a data race planted in `internal/store` and in SQLite's packages,
   and a pointer misuse in the module and in another dependency.

## Block 5, Ui (pull request #376)

1. **The five jobs keep the one id `screenshots`,**
   which All tests passed needs as one, and the Tested record's `ui=` with it.
2. **Their time limits are estimates** until they have runs of their own.

## Block 6, Cache (pull request #377)

1. **The cache key is weekly,** per Go version and `go.sum`,
   saved only by a push to `main` whose every step passed,
   so that a red run never saves a cache the next one trusts.
2. **Beyond the plan, `caches.yml`:**
   CodeQL's overlay databases held 10.0 of the repository's 10.8 GB of Actions
   caches, and past 10 GB GitHub evicts the least recently used,
   the Go jobs' among them;
   every hour all but the three newest commits' are deleted.
   The first prune deleted about 335 entries, 9.7 GB.
3. **`-count=1` in every Go job that gates:**
   with a cache, go test would otherwise take a pass from an earlier run.

## Block 7, Parallel server (pull request #378)

1. **Shared state found by the runs:**
   the commands' stores, the follow-up tests' configuration,
   a Cube that went while a poll waited (`awayWait`),
   found under `-race -count=20` at load and by `-cpu 1`; all fixed.
2. **One product line moved:**
   the reboot's check reads its length before its timer starts,
   which changes nothing the server does.

## Block 8, Parallel rest (pull request #379)

1. **The guard moved into the kit** (`ParallelOrListed`, `Swapping`),
   reading every system's test files,
   so the Windows-only tests are held to it on Linux.
2. **Found by the guard:**
   a serve test left `MASSIMO_CUBE_ADDRESS` set for the tests after it;
   the fake Cube's `Pause` kept a dropped client's slot.
3. **The command line and serve stay largely serial:**
   they keep options in package variables and read settings from the
   environment; only a product change would free them.

## Block 9, Stores (pull request #380)

1. **Built, as D12-A allowed:**
   Block 4's and Block 8's numbers left it worth it;
   the store ran no faster above four cores,
   a third of its time SQLite's allocator lock.
2. **Two halves:**
   the template's file handling in `internal/testkit` (`FileTemplate`,
   `WriteNew`), which imports no package of the module,
   `storekit` for the other packages, and the store's own tests on the kit,
   since they cannot import `storekit`.
3. **Measured:**
   a copy 3 ms plain and 30 ms under race, a new store 107 and 693 ms;
   the server's tests under race 26 s where they took 188.

## Block 10, Timers (pull request #381)

1. **The options:**
   `cubelan.NetWaits` for the network configuration's reads and writes,
   `SearchOptions.LateWait`, and the unexported `Options.discoveryLate`,
   `Options.rebootStuck` and serve's `Config.discoveryWait`;
   the variables tests swapped are constants,
   and the product sets none of the options.
2. **The second reader found a check lost on CI:**
   with the short re-read scaled and the fake's restart not,
   `TestNetConfigRoundTrip` no longer told the restart's long re-read from the
   short one at scale 2 and 4, the Race and armv6 jobs'.
   A window waited out where nothing answers now never takes the scale,
   the fake's restart is tied to it, and the plant fails at scale 1, 2 and 4.
   Its other points went in too: a test of the zero `NetWaits`,
   which no test covered; scaled deadlines for the calls the fake answers
   and short unscaled windows for those it does not; two wider margins.
3. **Left alone:**
   the tests whose timer is their own option and needed
   (`TestACallDuringARunGetsTheNext`, `TestRestoreRefusals`, the resend tests,
   `TestProgramTag`), the late second `TestSearchWaitsForALateAnswer` checks,
   and those that do not set their package's time:
   `internal/server`'s parallel phase is bound by its slots,
   so one timer there saves about a sixteenth of its length.
4. **Measured, plain, each package alone:**
   the command line 17.2 → 6.8 s, the Cube client 6.2 → 1.6 s,
   serve 3.7 → 1.0 s, the server 9.2 → 8.0 s;
   the two serial netconfig tests 12 → 1.4 s.

## Flakes seen, outside the blocks

- `test_rooms.py::test_out_of_date_device` (#377)
  and `test_rooms.py::test_radio_fault_badges[390-de]` (#378):
  passed on a re-run (D10-A).
- `TestProxyRetryStops`:
  once, possibly a reused port.
- The pile list's width test: handed to session "Web UI updates".
- At `-race -cpu 1`, on the trees before and after Block 10 alike:
  the confirm tests, the server's 200 ms discovery runs ("found []"),
  `TestAFixedCubeFoundElsewhere`, the held link, the proxy retry,
  `TestProgramTag`, the Cube client's `TestPollSkipsABusyLine`.
  CI runs at four CPUs, where none failed.

## Found in the product, not changed

1. The Cube client notices a lost Cube only after its 10 s reply timeout where a
   poll is pending.
2. Three server tests bind the host's ports 62910 and 23272, and stay serial.
3. A new store leaves `massimo.db.before-24` beside it:
   migration 0024 raises `min_reader` in the same open that created the file,
   so the store makes its rollback copy even on a new install.
4. `waitHealthy` (`cmd/massimo/update.go`) sleeps a whole `healthEvery` (2 s)
   past its deadline: a 1 s timeout returns after about 2 s.
5. `GUIDELINES/wip/HANDOVER.md` still names the UI job's old artifact.

## Verification

Every block: a builder,
an independent second reader whose points were fixed before the commit,
the guidelines check, the pull request's run green before the merge;
Blocks 7 to 10 also `-race` at scale 2 with `-count` 2 or 3, `-cpu 1,4`,
and faults planted for the tests they changed.
