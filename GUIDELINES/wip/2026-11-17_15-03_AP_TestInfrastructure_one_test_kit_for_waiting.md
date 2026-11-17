# AP TestInfrastructure v1.0: one test kit for waiting

## TL;DR

The tests wait in a hundred ways and trip over what comes first.
One small test kit replaces them, block by block,
each block moving every call site of what it replaces.
The product gains a few lines of test hooks and nothing in its behaviour.

- what the review could only read → settled by runs, first
- 10 poll helpers, about 70 loops, each a fixed 5 s → one wait with a scale
- "the next line or event" → a watcher: told what to wait for,
  started before the action, skips the rest, shows what it saw
- a test ahead of the server's event pump, fixed six times → one barrier
- about 20 start helpers, 27 starts written out → one start, the same waits
- ports closed and taken again → listeners that stay held
- UI tests meeting the server's own lines → a quiet server set, a log isolator
- tests that cannot fail or skip silently → they check what they say
- CI: a failure that passes on a re-run → leaves a record

### Blocks

1. Runs: the runs that settle the review's suspicions
2. Kit: the scaled wait, in place of the poll helpers and the loops
3. Watcher: one watcher fed by the log, the event stream,
   the fake Cube's lines and the client's events
4. Barrier: the wait on the server's event pump
5. Start: one server start for the server's tests
6. Ports: listeners that stay held, a fake Cube that can pause
7. Ui: the quiet server set, the log isolator, the stream watcher
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
`wip/2026-11-17_10-34_ANALYSIS_TestInfrastructure_review.md`.
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

## Implementation Plan

A session of its own runs it (D1-A), as the blocks of the other plans are run:
a sub-agent on Opus builds a block without a commit,
a second reader checks it, the commits are made from recorded trees,
main is merged, and the block is pushed with its pull request.
A block shows that it removed what it set out to remove:
the tests it moved pass many times over under load,
as `-race -count=20` beside a busy machine, where they failed before.

### Block 1: Runs

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
- The ten named helpers go.
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
- The exact comparison of the Cube's lines becomes a watch with a filter.

**Verification:**
as Block 2; the review's run with the fast poll passes.

### Block 4: Barrier

- A wait until the pump has handled every event the client had sent out before
  the call.
- It replaces the six local fixes the review names.

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

**Verification:**
as Block 2.

### Block 6: Ports

- A listener that stays open and is handed on;
  a fake Cube that drops its clients and refuses without closing its port.
- The six free-port helpers and the five rebinds go.

**Verification:**
as Block 2; the review's run of the rebinds passes.

### Block 7: Ui

- The UI server sets quiet: a long poll and no automatic backups,
  or a second set for the tests that need a poll.
- In `tools/ui-test/`: a log that holds a test's own lines alone;
  a watcher of the page's event stream; a wait for the log to be live,
  in place of 18 fixed waits; one safe route, in place of 15 copies.
- A wait whose result is dropped fails where it times out;
  a cleanup says what it could not undo.
- `docs/ui-testing.md` and `tools/ui-test/README.md`.

**Verification:**
the whole UI suite, twice; the review's run with the 1 s poll passes.

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
  a timeout, `-count=1`, and the three packages left out.
- A time limit on each test job.
- A pull request runs the UI tests for a change to the server's Go files
  and to the OpenAPI file too.
- Skips of named tests are counted and reported.
- `GUIDELINES/project/CI.md`.

**Verification:**
`make actionlint`; a run of the workflow on its own pull request.

## Proposed commit messages

- The plan: `chore(aps): +TestInfrastructure v1.0, +ANALYSIS TestInfrastructure`
- Block 1: `chore(aps): +NOTE TestInfrastructure v1.0, what the runs showed`
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
