# AP TestInfrastructure v1.1: one test kit for waiting

## TL;DR

The tests wait in a hundred ways and trip over what comes first.
One small test kit replaces them, block by block,
each block moving every call site of what it replaces.
The product gains a few test hooks and nothing in its behaviour.

- the review's suspicions → settled by runs (done): 2 confirmed, 5 not seen
- 10 poll helpers, about 70 loops, a fixed 5 s → one wait with a scale
- "the next line or event" → a watcher, started before the action,
  that skips the rest and shows what it saw
- a test ahead of the event pump → one barrier
- about 20 start helpers, 27 starts by hand → one start
- ports closed and taken again → listeners that stay held
- UI data the page's own reads replace → one shared answer, a log isolator
- tests that cannot fail or skip silently → they check what they say
- CI: a re-run that passes → a record; `make race`, armv6 → 30-minute timeout

### Blocks

1. Runs: the runs that settle the review's suspicions (done)
2. Kit: the scaled wait, in place of the poll helpers and the loops
3. Watcher: one watcher fed by the log, the event stream,
   the fake Cube's lines and the client's events
4. Barrier: the wait on the server's event pump
5. Start: one server start for the server's tests
6. Ports: listeners that stay held, a fake Cube that can pause
7. Ui: the page's own reads answered, the log isolator, the stream watcher
8. Holes: the tests that cannot fail or skip silently
9. Ci: a re-run leaves a record, timeouts, the missing race packages

## Discussion

### What the owner asked for

On 2026-11-17, in these words:
"review the testing infrastructure.
main focus: test robustness, reliability and infrastructure redundancy.
for example, often a test failed because it has been looking for a specific
event or log line to appear and tripped over another coming first.
So maybe a LogWatcher helper might be something,
where a test can say what it's waiting for and only notified if that happens.
There may be tons of individual workarounds.
along the way, also note test inaccuracies or possible locks of wrong logic."

The review is this plan's analysis.

### What is decided

The owner, 2026-11-17, `PLAN+ all A`:

- **D1-A.**
  A session of its own runs the plan.
- **D2-A.**
  All the blocks, in this order.
- **D3-A.**
  Each block moves every call site of what it replaces, package by package,
  and the old helpers go.
  The price is large diffs in test files other sessions also touch.
- **D4-A.**
  The runs that settle the review's suspicions come first,
  on an idle machine, before anything is built.
  The machine was not idle: other sessions run tests at any time
  (the owner, 2026-11-17), so each run's load was recorded instead.

The owner, 2026-11-17, at Block 1's gates:

- **D11-A.**
  The flaky `test_discover.py::test_found_cube_with_removed_rows` is fixed
  first, as one pull request, ahead of Block 1 (pull request #348).
- **D12-A.**
  This session fixes it: the test is of Block 7's class.

The owner, 2026-11-17, `PLAN++` at the gate after Block 1:

- **D13-A.**
  Block 7 answers the page's own reads with what a test put in,
  as the found Cubes' fixture of pull request #348 does,
  and turns the UI server sets' automatic backups off;
  their poll stays.
  Run 3 showed that the server's poll brings no line of its own
  into the log's tests.
- **D14-A.**
  `make race` and `make test-armv6` run with `-timeout 30m`:
  both went past Go's 10-minute default under load in Block 1.

### Decided in the writing

Each as recommended, and each small enough to change afterwards:

- **D5-A.
  Where the kit lives.**
  `internal/testkit`, imported by test files alone;
  a test holds it to that.
- **D6-A.
  The scale.**
  An environment variable, `MASSIMO_TEST_SCALE`, 1 where it is not set;
  `make race` sets 2 and `make test-armv6` 4, and CI's jobs through them.
  Measuring the machine at the start was the other way:
  it makes a run's waits differ from the run before.
- **D7-A.
  What the product gains.**
  Hooks without behaviour, and no build tag:
  a count of the events the Cube client has sent out and of those the
  server's pump has handled; in the fake Cube, which is test code already,
  a call at each line received, and a pause that keeps its port.
- **D8-A.
  Pull requests.**
  One for a block; a block whose move is large is cut by package,
  each pull request passing on its own.
  The old helper goes in the block's last pull request,
  so that a test written meanwhile against it stops compiling and is moved,
  not missed.
- **D9-A.
  A test that skips where its platform has what it needs** fails there instead;
  it skips only where the platform lacks it.

Decided in writing v1.1, each as recommended:

- **D15-A.
  A helper that names what it waits for keeps its name**
  (`waitConnected`, `waitForCube`, the drift rig's and the slots Cube's waits),
  and waits through the kit;
  the generic poll helpers go.
- **D16-A.
  The waits on a channel or on lines** - the Cube client's `waitEvent`,
  the queue tests' `waitFor` on the fake Cube's lines,
  `builtServe.waitFor` - move with Block 3's watcher;
  Block 2 gives their deadlines the scale.
- **D17-A.
  A fixed sleep before "nothing came" becomes a `Never`** only where the test
  has an end to wait for;
  the others move with Blocks 3 and 4, which bring one.

### Still open

Gated before Block 9:

- **D10.
  What a failure that passes on a re-run does.**
  Today the re-run counts,
  and the tests are then skipped for that code at the merge.
  - A (recommended):
    it still counts, and it leaves a record:
    the job that needed a re-run is named in the pull request's report
    and in a line of the run's summary.
  - B: a re-run does not count; a job that failed once blocks the merge
    until a new commit.

### What stays as it is

- The fake Cube with its recorded greeting;
  no test needs hardware or the network.
- The test commands of the contract, and the four cases of the UI tests.
- The second pass of the UI tests' runner, which fails the run either way.
- What a test asserts, but in Block 8.

## Analysis

The review this plan rests on is
`wip/2026-11-17_10-34_ANALYSIS_TestInfrastructure_review.md`;
Block 1's runs are
`wip/2026-11-17_18-57_NOTE_TestInfrastructure_v1.0_what_the_runs_showed.md`.
It read an export of `main` and ran nothing;
its section 9 names the run that settles each thing it only suspects.
Four of its findings were checked against the code again when it came in:
the two poll helpers with one body in package `server`,
the one start that waits for the greeting among the starts,
the exact comparison of the Cube's lines,
and the UI server sets started with the server's defaults.
What bears on the blocks:

- **No shared waiting code.**
  Ten named poll helpers and about 70 deadline loops, about 420 call sites,
  every wait a fixed 5 s whatever the machine.
- **The commonest flake,** fixed six times, each time where it showed:
  a test reads the API, which is the Cube client's snapshot,
  and acts before the server's pump has handled the event.
- **One start of about twenty waits until the greeting is published;**
  the most used one does not, at 48 call sites,
  and 27 tests start the server by hand.
- **Nothing can say "from now on, skip the rest, wait for this":**
  seven log captures, three readers of the event stream
  and about eleven counters of the fake Cube's lines come close,
  each in its own way.
- **Ports are closed and then taken again:**
  six helpers return a port they have just closed,
  and five tests start a second fake Cube on the address of the first.
- **The UI server sets run with a 10 s poll and automatic backups,**
  whose lines most fixes to the log's UI tests had to step around.
- **Holes:**
  a test that never checks the "no second line" its comment promises;
  discovery tests that skip on any bind error; waits whose result is dropped;
  cleanups that swallow their errors.
- **CI:**
  a job that passes on a re-run counts, and leaves no record;
  `make race` has no timeout and leaves three packages out;
  a pull request runs the UI tests only for a change to the UI.
- **Nothing found that locks in plainly wrong behaviour;**
  two tests are named as worth a second look.

What Block 1's runs showed, as the NOTE has it:

- **Confirmed:**
  with a 20 ms poll the exact comparisons of the Cube's lines failed
  60 times in 600 runs, two tests every time,
  one of them at `capabilities_test.go:260`, outside `sameLines`;
  a doubled event line passes `TestEventLinesMatchTheRows`.
- **Not seen:**
  with the pump slowed, no failure the greeting's wait would have prevented,
  but four tests that fail every time;
  in the log's UI tests, no line of the server's poll,
  which brings a state event only where a device changed;
  no discovery skip in 360 runs of the 18 tests that can skip;
  no failed rebind in 200 runs; no race in the three packages left out.
- **Seen on the way:**
  `internal/server` took 711 to 988 s under `-race` here,
  and on armv6 with 4 CPUs it ran into Go's 10-minute default;
  in CI's armv6 job it takes up to 61 % of it.

## Implementation Plan

A session of its own runs it (D1-A), as the blocks of the other plans are run:
a sub-agent on Opus builds a block without a commit,
a second reader checks it, the commits are made from recorded trees,
main is merged, and the block is pushed with its pull request.
A block shows that it removed what it set out to remove:
the tests it moved pass many times over under load,
as `-race -count=20` beside a busy machine, where they failed before.

### Block 1: Runs

Done: `wip/2026-11-17_18-57_NOTE_TestInfrastructure_v1.0_what_the_runs_showed.md`.

- The runs of the review's section 9, each as written there, on an idle machine:
  the exact comparison of the Cube's lines with a fast poll;
  the starts with a slow pump; the log's UI tests with a 1 s poll;
  the discovery tests' skips under `make race`; the rebinds under load;
  the test of the doubled line; the armv6 run's durations;
  the races of the three packages left out.
- Their outcome as a NOTE.
  Where one changes a later block, the plan's next revision says so.

**Verification:**
each run's command and result in the NOTE.

### Block 2: Kit

- `internal/testkit`:
  the scale; a wait for a condition; "nothing came",
  held until a barrier and not for a sleep.
- Every poll helper and loop moved: `internal/server`, `internal/cubelan`,
  `internal/proxy`, the fake Cube, then the loops written out.
- The generic poll helpers go;
  a helper that names what it waits for waits through the kit (D15-A).
- Every fixed deadline of a wait takes the scale,
  those on a channel too (D16-A);
  a sleep before "nothing came" becomes `Never` where it has an end (D17-A).
- `make race` and `make test-armv6` set the scale.
- A test that no file but a test imports the kit.
- `docs/testing.md`, new: how a test waits; named from `CONTRIBUTING.md`.

**Verification:**
`go test ./...`, `make race`, `make test-armv6`;
no named poll helper and no deadline loop left in the packages moved.

### Block 3: Watcher

- One watcher: a mark, a wait for a match from a mark that skips the rest,
  a wait for several in any order, a count;
  on a timeout it shows what it did see since the mark.
- Its feeders: the log, the event stream, the fake Cube's lines,
  the Cube client's events, a child process's output.
- Moved, in this order: `internal/cubelan`'s log lines and event waits;
  the server's stream tests; the waits that read the log's API for one new line,
  through one query with a cursor; `cmd/massimo`.
- The exact comparison of the Cube's lines becomes a watch with a filter,
  `sameLines` and the one at `capabilities_test.go:260`.

**Verification:**
as Block 2; the review's run with the fast poll passes.

### Block 4: Barrier

- A wait until the pump has handled every event the client had sent out before
  the call.
- It replaces the six local fixes the review names,
  and moves the four tests that failed in every pass with a slow pump:
  `TestRemoveListedDevice`, `TestProgramConfigEvents`,
  `TestFollowUpAfterAWrite` and `TestPolledStateChangesReachTheLog`.

**Verification:**
as Block 2; the review's run with the slow pump passes.

### Block 5: Start

- One start for the server's tests, with what a test needs as options:
  a store, a greeting, a change to the fake Cube, the server's options,
  quiet (no poll, no refresh).
- It returns once connected, the greeting published and the writer flushed,
  the same for every test.
- The start helpers and the starts written out go;
  with them the 90 loads of the greeting, and the five clocks become one.
- Its gain is one start in place of about twenty:
  Block 1 found no failure the greeting's wait would have prevented.

**Verification:**
as Block 2.

### Block 6: Ports

- A listener that stays open and is handed on;
  a fake Cube that drops its clients and refuses without closing its port.
- The six free-port helpers and the five rebinds go.

**Verification:**
as Block 2; the review's run of the rebinds passes.

### Block 7: Ui

- The page's own reads that replace what a test put in
  (its poll, and the reads that events and actions start):
  one shared fixture answers them with the server's data and the test's,
  as the found Cubes' fixture of pull request #348 does (D13-A).
- The UI server sets without automatic backups; their poll stays (D13-A).
- In `tools/ui-test/`: a log that holds a test's own lines alone;
  a watcher of the page's event stream; a wait for the log to be live,
  in place of 18 fixed waits; one safe route, in place of 15 copies.
- A wait whose result is dropped fails where it times out;
  a cleanup says what it could not undo.
- `docs/ui-testing.md` and `tools/ui-test/README.md`.

**Verification:**
the whole UI suite, twice;
the tests moved pass with the page's poll at 300 ms, a build overlay.

### Block 8: Holes

- Each place of the review's section 7:
  the test that never checks its "no second line";
  the skips on a bind error (D9-A);
  the settle that returns on an old line;
  the backoff test's wall-clock bounds;
  an ask that returns nothing and is dereferenced;
  the UI waits that are met by lines the test did not send.
- The two tests named as worth a second look are read with the owner
  before anything in them changes.

**Verification:**
each corrected test fails on a fault planted for it.

### Block 9: Ci

Waits for D10.

- A job that needed a re-run leaves its record (D10).
- `make race`:
  `-timeout 30m`, `-count=1`, and the three packages left out;
  `make test-armv6`: `-timeout 30m` (D14-A).
- A time limit on each test job.
- A pull request runs the UI tests for a change to the server's Go files
  and to the OpenAPI file too.
- Skips of named tests are counted and reported.
- `GUIDELINES/project/CI.md`.

**Verification:**
`make actionlint`; a run of the workflow on its own pull request.

## Proposed commit messages

- The plan: `chore(aps): +TestInfrastructure v1.0, +ANALYSIS TestInfrastructure`
- Block 1: `chore(aps): +NOTE TestInfrastructure v1.0, what the runs showed` (done)
- Block 2: `test(testkit): One wait with a scale, in place of the poll helpers`
- Block 3: `test(testkit): A watcher that is told what a test waits for`
- Block 4: `test(server): A test waits for the event pump`
- Block 5: `test(server): One start for the server's tests`
- Block 6: `test(testkit): A test's port stays held`
- Block 7: `test(ui): A quiet server set and a log of the test's own lines`
- Block 8: `test(server): Tests that could not fail check what they say`
- Block 9: `ci(tests): A failure that passes on a re-run leaves a record`

## Change History

- v1.0 (2026-11-17): first revision, by `PLAN+`,
  from the review the owner asked for that day,
  with D1-A to D4-A as the owner chose,
  D5-A to D9-A as recommended, and D10 open.
- v1.1 (2026-11-17): by `PLAN++`, after Block 1:
  its runs' outcome (NOTE v1.0, what the runs showed) in the Analysis;
  D11-A, D12-A, D13-A and D14-A as the owner chose,
  D15-A to D17-A as recommended;
  Blocks 2, 3, 4, 5, 7 and 9 changed accordingly; D10 still open.
