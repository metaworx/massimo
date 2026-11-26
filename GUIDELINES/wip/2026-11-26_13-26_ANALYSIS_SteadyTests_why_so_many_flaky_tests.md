# ANALYSIS SteadyTests: Why so many flaky tests, and what would end them

Measured on `origin/main` at 2026-11-26 13:26
(`merge(AccessEnds/Block-3)`, #463).

## TL;DR

The suite is large and strict, and it runs against a live system
that moves on its own.
Most flakes this month were one of three things:

- the server's own traffic took back what a test had set;
- a test waited for the wrong thing;
- the fake Cube did less than a real one.

Helpers fix one test at a time.
What ends a class of flakes is one way of doing each thing,
held by a guard test that fails a new test doing it another way.

## 1. Why there are so many

### The suite is large, and a flaky test fails the run

- 764 UI tests in 75 files (35,700 lines),
  run in four cases: about 3,000 browser tests on every PR.
  Beside them, the Go suite under `-race` and on armv6 QEMU,
  and the Android emulator.
- A failed test runs once more.
  The run fails even when the retry passes
  (`tools/ui-test/run.sh:320`, "they are flaky").
  This is the right rule, and it means every flake is seen:
  at one failure in 3,000 test runs, nearly every PR run is red once.
- 73 of the 75 UI files were added since 2 November,
  by many sessions, each with its own way of setting data and waiting.

### The system under test moves on its own

Each xdist worker has one server and one fake Cube for all its tests
(`run.sh`, `start_set`).
While a test runs, things happen that it did not cause:

- the server asks the Cube for its device list every 10 s
  (`DefaultPollInterval`), and pushes the device events that follow;
- after a write, the server re-reads, and pushes the config event;
- the page reloads its data on those events, and the header's clock ticks;
- an earlier test's write can still arrive at the next test.
  Which test comes next depends on how xdist shares the tests out,
  so a leak shows only in some runs.

A test that put its data into one place,
and checked it before or after one of these, failed sometimes.

### Three places to put data, and no rule for which

A test can put its data into the fake Cube, the server, or the page.
In the UI tests:

| What | Uses | Files |
|------|-----:|------:|
| `state.` written or read through `page.evaluate` | 667 | 57 |
| `page.evaluate` in all | 1,814 | 73 |
| `Reads` (the read that keeps what a test told it, #439, #448) | 13 | 6 |

Data put into the page alone lasts until the next read or event.
ToldData moved the tests it found at risk; the rest are unclear.

### Waits are hand-written

| What | Uses | Files |
|------|-----:|------:|
| `wait_until(page, "<JavaScript>", s)` | 1,238 | 74 |
| `wait_for_timeout` (a fixed pause) | 125 | 36 |
| plain `page.route` | 175 | 45 |
| `safe_route` (no race when a request is in flight) | 55 | 11 |
| Playwright's `expect(...)`, which retries by itself | 0 | 0 |

Each `wait_until` is JavaScript written for that test.
Whether it waits for the right moment is up to its writer.
The plain `page.route` handlers' race is roadmap item 12.

### The fake Cube does less than a Cube

The fake ignores some commands the server sends,
for example 0x22, being fixed now as AP FakeGroup.
Until #448 it also lost a paired device at a reconnect.
The server then sees a different Cube than the test expects.
Nothing fails when a new command has no handling in the fake.

## 2. What the flakes of the last two days were

| Flake | Cause | Class |
|-------|-------|-------|
| batteries offered again (#439) | the server's budget reconnect | own traffic |
| unchanged C line twice (#437) | a re-read applied twice | own traffic |
| 14 room, battery and mark tests (#448, Blocks 1 to 4) | data put into the page alone | own traffic |
| settings test, "one read after it (2)" (#448, Block 8) | the server's own change events | own traffic |
| pairing test, `KeyError: 'copied'` (#448, Block 6) | the fake lost a paired device | fake Cube |
| restore test on armv6 (#448, Block 7) | read the log before its line | wrong wait |
| times-ago, "4 sec. ago" (#448, Block 9) | a write in place, not redrawn | wrong wait |
| test_new_batteries_sent | a button replaced by a redraw | wrong wait |
| room mismatch after a reconnect (FakeGroup) | the fake ignores 0x22 | fake Cube |

The plans that went into the tests' stability this month:
TestRaces, UiParallel, TestInfrastructure, TestSpeed, LogCursor, Flakes,
ServerTraffic, RereadOnce and ToldData.
The Go side has had a kit since TestInfrastructure (`internal/testkit`);
two of the nine flakes above were Go tests.
The UI side has `helpers.py` (1,354 lines), which grew one fix at a time.

## 3. What would end them

Each item ends a class of flakes, not one flake,
and each comes with a guard that keeps it so.

1. **One way to set the page's data:**
   through the fake Cube or through `Reads`, never into `state` alone.
   Guard: a test refuses a test file writing `state.<field>`,
   outside a list of existing files that only shrinks.
2. **One way to wait for the page:**
   Python helpers per region, such as "this tile shows 21°",
   built on Playwright's `expect`, which retries until it holds,
   instead of 1,238 JavaScript strings.
   Guard: no new `wait_for_timeout` and no plain `page.route`,
   outside a list that only shrinks.
3. **A test begins from a known Cube:**
   the fake goes back to the recorded greeting between test files,
   so no earlier test's write can arrive.
   A UI run without the 10 s poll removes most of the server's own events.
   That needs a server change: today a poll of 0 means the default,
   and the settings' minimum is 10 s (`settings_api.go:105`),
   where `watch -poll 0` already means never.
   The few tests that need the poll would turn it on.
4. **The fake handles every command the server builds:**
   a Go test lists each builder of `internal/protocol/cube`,
   and fails where the fake neither applies it
   nor names it as ignored, with the reason.
5. **Flakes found on main, not in a feature's PR:**
   a nightly UI run with each test three times in a shuffled order,
   whose flaky tests are listed in the run's summary.

Items 1 and 2 touch most test files.
They can move a region at a time, with the guard first.
Items 3 to 5 are small, and 3 removes the most flakes per line changed.
