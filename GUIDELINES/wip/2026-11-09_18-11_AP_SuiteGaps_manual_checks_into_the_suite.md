# AP SuiteGaps v1.0: Manual checks into the suite

## TL;DR

Four checks that AP Logging's last runs made by hand become part of the suite
and of CI, in three blocks, a pull request each.

- 32-bit ARM → built, never tested → the whole suite under QEMU as the Pi 1's
  core, on every pull request (D1-A, D2-A, D3-A)
- the race this found in `TestEventsPageQuery` → fixed
- migrations → tested on files the tests build → also on stores written by
  each of the six releases with a store, as text dumps (D4-A, D5-B)
- the Pi timing harness and the PC's batch timing → scratch code →
  benchmarks in `internal/store`, from a box's copy when given one (D6-A),
  run once in CI (D7-A)

### Blocks

1. ArmTests: `make test-armv6` and its CI job, the race fixed, CI.md
2. StoreFixtures: `tools/store-fixture`, the dumps of v0.7.0 to v0.12.0,
   `TestMigratesReleasedStores`, the rule in `docs/store.md`
3. StoreBench: the benchmarks, `make store-bench`, the recipe for a box,
   the run in CI

## Discussion

**The owner's question** of 2026-11-09, after AP Logging retired:
which manual tests of the session belong in the suite, in docker or in CI.
The answer named four (T1 to T4); the owner's `PLAN T1 to T4` had them weighed,
and `PLAN++ D5-B` took every recommendation but D5:

- **D1-A.** Every package runs under QEMU, with `-short`.
- **D2-A.** The job runs on every pull request and every push to `main`,
  beside Vet and test.
- **D3-A.** Ubuntu's `qemu-user` from apt on the runner,
  the tests run through `qemu-arm -cpu arm1176`: no new action,
  and the core is the Pi 1's.
- **D4-A.** A fixture is a text dump (`sqlite3 .dump`), not the file.
- **D5-B.** A dump of each of the six releases that have a store:
  v0.7.0, v0.8.0, v0.9.0, v0.10.0, v0.11.0 and v0.12.0 (each `-alpha.1`),
  at schemas 1, 2, 3, 4, 7 and 8;
  rather than only the installed schemas 7 and 8 (D5-A)
  or only from the next release (D5-C).
- **D6-A.** One set of benchmarks serves the PC and a box's copy.
- **D7-A.** CI runs each benchmark once, to see that it still runs.

**Not in this plan:**
making the new job a required check, a setting of the ruleset and the owner's;
`benchstat` (golang.org/x/perf), an outside tool not in the module;
timing in CI, which says nothing of an SD card;
running anything on the owner's boxes.

**Names chosen here**, routine: the blocks' names; `make test-armv6` and
`make store-bench`; the job `armv6`, named "Test (armv6, QEMU)";
`tools/store-fixture/make.sh`; `internal/store/testdata/releases/<tag>.sql`;
`TestMigratesReleasedStores`; `BenchmarkWriteEvents` and
`BenchmarkEventsPage`; `MASSIMO_STORE_BENCH_COPY` and
`MASSIMO_STORE_BENCH_ROWS`; the branches `claude/arm-tests`,
`claude/store-fixtures` and `claude/store-bench`.

**Who runs it:** this session.
The plan's own commits go straight onto `main`;
each block's branch starts from `origin/main` after them
and becomes a pull request with its commit.

## Analysis

Read on `main` at 0fe7e35.

### T1: the suite on 32-bit ARM

The `armhf` package ships the armv6 binary (`Makefile`, `PACKAGES`),
and CI builds it (`make cross` in Vet and test) but runs no test on 32-bit ARM.
The one armv6 test run so far was AP Logging's Pi timing, a single test.

Tried on jaguar on 2026-11-09: every package's tests built with
`GOARCH=arm GOARM=6` (26 s), run with `-short` by Debian 13's `qemu-arm`
10.0.13 with `-cpu arm1176`, in a container without a network,
each from its package's directory:

| Package | Result | Time |
|---------|--------|-----:|
| `internal/server` | pass | 135 s |
| `internal/store` | fail, below | 59 s |
| `cmd/massimo` | pass | 22 s |
| `internal/cubelan`, `internal/serve` | pass | 12 s each |
| `internal/update`, `internal/certs` | pass once `/tmp` allowed execution (Docker's `--tmpfs` is `noexec`) | 5 s, 2 s |
| the other 15 | pass | 0 to 3 s each |
| all, one after another | | 264 s |

`go test -exec` runs the packages side by side,
so the job should take about as long as `internal/server` on the runner.
No 32-bit fault turned up.

**The race it found:** `TestEventsPageQuery` failed under `-short` on
"the server's warnings", a shape of AP Logging v1.2, Block 8 (#137).
`eventsStore` starts the writer, whose first prune runs in its goroutine;
under QEMU it ran after the test's fill and deleted the rows filled at
`start`, exactly 30 days ago, `log.keep_days`' default,
among them the server's one warning of 100 rows.
The same can happen on amd64, where the prune nearly always comes first.
The fill moves inside the retention window: rows from 29 days ago to one day
ago, so that the prune finds none to delete.

**In CI:** `sudo apt-get install qemu-user`, then the make target.
`ci.yml` notes that apt took up to 11 minutes for Playwright's libraries;
one package with its few dependencies should take far less,
to be seen in the pull request.

### T2: stores written by the released binaries

`docs/store.md` ("Migrations", the rules' "Tests") promises
"a fixture database per released version"; `internal/store` has none.
`TestMigratesAVersionOneFile` and `TestMigration9` build their older files
from the migrations and from rows the tests write,
so a row an older binary really wrote is not checked:
its JSON in `events.data`, a backup's body, a setting's value, an audit row.
The owner's two copies, written by v0.12.0-alpha.1, were the first real files
to go through 0009 to 0011.

| Release | Schema | `user` commands | Backups by hand |
|---------|-------:|-----------------|-----------------|
| v0.7.0-alpha.1 | 1 | no | no |
| v0.8.0-alpha.1 | 2 | yes | no |
| v0.9.0-alpha.1 | 3 | yes | no |
| v0.10.0-alpha.1 | 4 | yes | yes |
| v0.11.0-alpha.1 | 7 | yes | yes |
| v0.12.0-alpha.1 | 8 | yes | yes |

Every one has `massimo fakecube`, `massimo store backup` and the setpoint
routes; each tag's own `tools/ui-test/run.sh` shows how its `serve` meets the
fake Cube (the bootstrap token, `MASSIMO_CUBE_<serial>_ADDRESS`, `-proxy off`).
The script takes the flags a version knows from its `serve -h`,
and the greeting and serials from today's `tools/ui-test/greeting.txt`,
whose identifiers are placeholders (`KEQ0000100`, `MEQ0000101` …);
an older tag's own `run.sh` may name another serial, which stays out.

**A dump** is reviewed as a diff of text and is read by `tools/identifiers`,
which reads every tracked file; the file itself would be neither.
`sqlite3 .dump` writes each table's statement as the file keeps it,
so an altered table loads as the release left it.
The store's driver runs a dump in one `Exec`, as it runs a migration.

**What the test checks**, for each dump, after `Open` has migrated it:
the schema version is the binary's;
`integrity_check` says ok and `foreign_key_check` finds nothing;
every table keeps at least its rows, `events` exactly its own;
no row of `events` is left without a source (0009 and the writer's fill);
a page of the log with the server's rows, the users and the backups read.

### T3 and T4: the timing

The Pi harness was rebuilt for both of AP Logging's runs: it opened a copy,
grew it to 90,000 events from the copy's own, and timed 30 batches and
10 pages at each level.
The PC's scratch test timed a batch with a third warnings.
One set of benchmarks does both (D6-A):

- **The store:** made once per run, from a copy when
  `MASSIMO_STORE_BENCH_COPY` names one, grown to `MASSIMO_STORE_BENCH_ROWS`
  (90,000) with the newest 1000 events of its busiest Cube, cycled;
  else from rows of its own, as `TestEventsPageQuery` fills them,
  3000 under `-short`.
  No writer runs, so no prune either.
- **`BenchmarkWriteEvents`:** one batch of 100 through `writeEvents`,
  the writer's own; of its own rows a third are warnings.
- **`BenchmarkEventsPage`:** pages of 200 of the busiest Cube:
  `all`, `warnings`, `errors`, `server` (with the server's rows).
- **On a box:** `make store-bench` cross-builds the test binary for arm64 and
  armv6 into `dist/`; the box runs it with `-test.bench . -test.count 10`.
  Go's lines say the time per batch and per page.
- **In CI (D7-A):** `-bench . -benchtime 1x -short` on `internal/store`,
  seconds, the password benchmarks included; no time is compared.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `internal/store/events_test.go` | 1 | the fill inside the retention window |
| `Makefile` | 1, 3 | `test-armv6`; `store-bench` |
| `.github/workflows/ci.yml` | 1, 3 | the job `armv6`; the benchmarks once in Vet and test |
| `GUIDELINES/project/CI.md` | 1, 3 | section 2, the jobs |
| `tools/store-fixture/make.sh` (new) | 2 | the dump of a release's store |
| `internal/store/testdata/releases/*.sql` (new) | 2 | the six dumps |
| `internal/store/releases_test.go` (new) | 2 | `TestMigratesReleasedStores` |
| `internal/store/events_bench_test.go` (new) | 3 | the benchmarks |
| `docs/store.md` | 2, 3 | "Migrations", the rule of the dumps; the recipe for a box |
| `CHANGELOG.md` | 1–3 | a bullet each under Unreleased, Added |

### Risks

- **The emulation's slowness** can bring out other timing in tests,
  as it did the prune: each such test is fixed in Block 1 or named at its
  gate, never skipped under QEMU without the owner's word.
- **apt on the runner** may be slow: the job's time is read in the pull request.
- **An older tag's `serve`** may want flags or variables the script does not
  give: each release is built and run once, and the script says which
  request a release did not take.
- **Blocks 1 and 3** both change `ci.yml`, CI.md section 2 and
  `CHANGELOG.md`: the second to merge merges `origin/main` first.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`, `make race`,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from the
main checkout.
No UI file changes, so no `make ui-test`.
Nothing reaches a real Cube, a box or the network: the fake Cube answers.
One functional commit per block, citing `AP SuiteGaps v1.0, Block <n>`,
with its CHANGELOG bullet in the wrapped style of its neighbours.

### Block 1: ArmTests

On the branch `claude/arm-tests`.

1. `internal/store/events_test.go`, `TestEventsPageQuery`: the fill from
   29 days ago to one day ago, `mid` in between; the comment says why
   (the writer's first prune).
   Every other user of `fill` checked for the same edge.
2. `Makefile`: `test-armv6`:
   `GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 MASSIMO_ENV_FILE=/dev/null`
   `go test -short -exec 'qemu-arm -cpu arm1176' ./...`,
   QEMU taken from the system (`qemu-user`), with its comment.
3. `.github/workflows/ci.yml`: the job `armv6`, "Test (armv6, QEMU)",
   beside Vet and test on every pull request and push to `main`:
   checkout, setup-go, `qemu-user` from apt, `make test-armv6`.
4. `GUIDELINES/project/CI.md`: section 2, the job and why; its version and
   history line.
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - CI runs the tests on 32-bit ARM under QEMU, as the Raspberry Pi 1's
     core, and `make test-armv6` does the same where `qemu-arm` is
     installed (`GUIDELINES/project/CI.md`).
   ```

**Verification:** the test gate;
`make test-armv6` in a container with Debian's `qemu-user` and this machine's
Go, the tests without a network;
`TestEventsPageQuery` passing under QEMU;
the job green on the pull request, its time noted at the gate.

### Block 2: StoreFixtures

On the branch `claude/store-fixtures`.

1. `tools/store-fixture/make.sh <tag>`:
   the tag built in a temporary worktree;
   its `fakecube` with today's `tools/ui-test/greeting.txt`,
   its `serve` in a scratch directory with the flags its `serve -h` knows;
   once the Cube is connected: a setpoint on a device and on a room,
   a room renamed, the settings the release takes;
   from v0.8.0 a user and a key (`massimo user add`, `user token`) and a
   request with the key; from v0.10.0 a backup by hand;
   then `serve` stopped, `store backup` by the same binary, and
   `sqlite3 .dump` of the copy under a header naming the tag, its commit and
   the date; the requests a release did not take are listed.
   Placeholders only, made-up passwords and keys.
2. `internal/store/testdata/releases/v0.7.0-alpha.1.sql` to
   `v0.12.0-alpha.1.sql`, made by it (D5-B).
3. `internal/store/releases_test.go`: `TestMigratesReleasedStores`,
   the checks of the Analysis, one subtest per dump.
4. `docs/store.md`, "Migrations", the rules' "Tests": the dumps, the script,
   and the rule that a pull request with a migration adds the newest
   release's dump when it is missing.
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The store's tests migrate a store written by each release since
     v0.7.0-alpha.1, kept as text dumps that `tools/store-fixture` makes
     (`docs/store.md`, "Migrations").
   ```

**Verification:** the test gate;
the test failing when 0009's fill of the source is left out on purpose;
`tools/identifiers` passing with the dumps;
each release's run of the script without an error.

### Block 3: StoreBench

On the branch `claude/store-bench`.

1. `internal/store/events_bench_test.go`: the store of the Analysis, made once
   per run and removed after it; `BenchmarkWriteEvents` and
   `BenchmarkEventsPage` with its four pages;
   the store's size and levels logged once.
2. `Makefile`: `store-bench`: `dist/store-bench-linux-arm64` and
   `dist/store-bench-linux-armv6` (`go test -c`), with its comment.
3. `docs/store.md`, section 5 after the measurements: measuring on a box,
   the owner's copy command, the run, two binaries compared, the cleanup;
   only the times leave the box.
4. `.github/workflows/ci.yml`, Vet and test: the benchmarks once;
   `GUIDELINES/project/CI.md` section 2 with it.
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - Benchmarks of the log's writer and pages in `internal/store`, also on
     a copy of a box's store, and `make store-bench` builds them for the
     Raspberry Pis (`docs/store.md`).
   ```

**Verification:** the test gate;
the benchmarks on the PC at 90,000 events, near `docs/store.md`'s figures;
a run from a copy, the stand-in store of AP Logging's dry run as the copy;
`make store-bench` building both binaries.

## Proposed commit message

The plan's commit, on `main`, before the run's first project write:

```
chore(aps): +SuiteGaps v1.0

The owner's question after AP Logging retired, which manual tests of the session belong in the suite, weighed as T1 to T4 and taken with PLAN++ D5-B: the suite under QEMU as the Pi 1's core on every pull request, stores written by each of the six releases with a store as text dumps, and one set of benchmarks for the PC and a box's copy.

Key changes:
- GUIDELINES/wip/2026-11-09_18-11_AP_SuiteGaps_manual_checks_into_the_suite.md: three blocks, ArmTests, StoreFixtures and StoreBench; the QEMU trial on jaguar and the race it found in TestEventsPageQuery; D1-A to D7-A, but D5-B.

File-revision-v1: GUIDELINES/wip/2026-11-09_18-11_AP_SuiteGaps_manual_checks_into_the_suite.md::v1.0::2026-11-09::All::the first revision
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `ci(armv6): The tests run on 32-bit ARM under QEMU, as the Pi 1's core`
2. `test(store): Stores written by each release migrate to the current schema`
3. `test(store): Benchmarks of the log's writer and pages, a box's copy too`

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's question after AP
  Logging retired and the QEMU trial on jaguar; the owner's `PLAN++ D5-B`.
