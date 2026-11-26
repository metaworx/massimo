# AP SteadyTests v1.0: Steady tests at their cause

## TL;DR

The nine flakes met while AP AccessEnds was built, fixed at their cause,
each with the kit piece of its class,
and the guards that keep each class closed,
in nine blocks and two pull requests
(`wip/2026-11-26_14-29_ANALYSIS_SteadyTests_steady_tests.md`).

- fixed windows a loaded runner overruns (1, 2, 3, 7, 8) →
  the fake Cube's apply held by the test; `UI_TEST_SCALE` in every UI wait
- waits on a moment that proves nothing (4, 5, 6) →
  waits on what they check; one wait for a page whose reads all answered
- Gradle's 429 (9) → a Gradle cache
- the old idioms → guards with lists that only shrink
- the server's own traffic → a known Cube per test file, no poll in UI runs
- flakes → found by a nightly run, not by a feature's PR

### Blocks

1. WrongWaits: items 4, 5 and 6 wait for what they check
2. HoldApply: the fake Cube holds its apply; items 1, 2 and 3; the same race in three tests
3. UiScale: `UI_TEST_SCALE` in the kit's waits and the fixture; the page's start (items 7, 8)
4. PageReads: one wait for a page whose reads all answered; the idioms' guard
5. KnownCube: the fake reset between test files; `serve -poll 0`; UI runs without the poll
6. StateGuard: no test file writes `state.<field>` outside a shrinking list
7. FakeCommands: the fake applies or names every command the server builds
8. NightlySoak: a nightly UI run, each test three times, shuffled
9. GradleCache: a Gradle cache in the Android and emulator jobs

## Discussion

### What is decided

The owner, 2026-11-26, `PLAN++` on the TL;DR gate of the same day:

- **D1-A.** All nine blocks, the list's fixes first, then the guards.
- **D2-A.** The list of the session "UI refactor" was sorted by class
  before the plan; done.
- **D3-A.** Playwright's `expect` for new waits is settled in a later plan,
  which moves the regions onto `Reads` and `expect`; this plan adds guards.
- **D4-A.** The UI run goes without the server's 10 s poll;
  the tests that need a device list read ask for it.
  `serve -poll 0` means never, a product change with its fragment.
- **D5-A.** Two pull requests: Blocks 1, 2, 3 and 9, then Blocks 4 to 8.
  Block 7 waits for AP FakeGroup's pull request,
  and goes into the second one, or a third, whichever is open then.

Open, asked by the owner during the writing:

- **D6.** A front page for the tests:
  - **A (recommended).** A Block 10: `docs/testing.md` becomes the page for
    every suite (which exist, how each runs, the rules of a steady test,
    a flaky test fixed at its cause), and `docs/ui-testing.md` opens with its
    kit, the runner comparison moved to its end.
  - **B.** Each block updates only the documents it touches.

### Decided in the writing

- **Builders:** Opus sub-agents, each on a branch of its own in an isolation
  worktree, their commits taken onto the pull request's branch.
  The first pull request has three: the UI blocks (1, 3), the Go block (2)
  and the CI block (9).
- **Branches:** `claude/steady-tests` for the first pull request,
  `claude/steady-kit` for the second.
- **A commit per fix,** each proven old against new
  with a planted delay or event, three runs each unless said;
  the touched Go packages and the touched UI files in the four cases locally;
  where `conftest.py` or `helpers.py` changes the whole UI suite once,
  in one case, on the committed tree; the whole suite is CI's.
- **The guards** of Blocks 4 and 6 are Go tests,
  as `tools/identifiers` is, so that `go test ./...` fails a new use.
  A file's listed count may fall and may not rise;
  a fall lowers the list in the same commit.
- **The retry's scale:** `run.sh` raises `UI_TEST_SCALE` for the retry,
  which runs with the trace; 3 unless Block 3's measurements say otherwise.
- **The soak gates nothing:** Block 8's run is a workflow of its own,
  red where a test failed once, read as a report;
  the nightly build does not wait for it.

## Analysis

The causes, the proofs and the sweep are in
`wip/2026-11-26_14-29_ANALYSIS_SteadyTests_steady_tests.md`,
the measures of the suite in
`wip/2026-11-26_13-26_ANALYSIS_SteadyTests_why_so_many_flaky_tests.md`.
What they do not hold, and what each block rests on:

### For Block 2: item 3 seen elsewhere

The session "AP TestInfrastructure" (AP StartConfiguration) met item 3
on its own, 2026-11-26: run alone under `-race`,
22 failures in 460 runs on its branch and 15 in 460 on the code before it;
a whole `-race` run of `internal/server` failed on it twice.
Recorded in its NOTE "how the run went", "Found outside the plan".
Its suggested fix is this plan's.

### For Block 5: who needs the poll

No test was counted yet that needs the 10 s poll.
The block's first step runs the suite with `-poll 0` and lists the tests
that fail; each asks for its read (a refresh through the API, or the fake's
control) instead.

### For Block 7: AP FakeGroup

AP FakeGroup, in another session, makes the fake apply 0x22.
Its handling is the first entry Block 7's test finds;
Block 7 starts once FakeGroup's pull request has merged,
so that the two do not both edit `fakecube/apply.go`.
Block 2's `HoldApply` touches the same file;
the first pull request merges `origin/main` before it is opened, as always.

## Implementation Plan

Each block ends at its Verification; the blocks of a pull request are
taken onto its branch in their order.

### Block 1: WrongWaits

- **Goal:** items 4, 5 and 6 wait for what they check.
- **Changes:**
  - `tools/ui-test/test_maint_updates.py`:
    `test_system_info_reads_again_at_a_resync_while_shown_only` waits on the
    new start's own data (`#info-started .age`'s `data-at`);
    `test_an_event_keeps_a_question_and_takes_back_one_whose_reboot_waits`
    waits for the reboot read (`wait_reads`) before `answered`.
  - `tools/ui-test/test_sign_in_resend.py`:
    `test_stream_ticket_waits_for_the_sign_in` waits for the second ticket
    (`back(page, seen, before + 1)`).
- **Tests:** a planted late answer (an in-page `fetch` wrapper):
  the store's 600 ms (item 4), the settings' 600 ms (item 5),
  the restart's rooms read 1500 ms (item 6); old fails, new passes;
  the two files in the four cases.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): Three tests wait for the moment they check`
- **Verification:** the plants' runs; the two files in the four cases.

### Block 2: HoldApply

- **Goal:** the Go tests of items 1, 2 and 3, and those of the same race,
  depend on no window.
- **Changes:**
  - `internal/cubelan/fakecube`: `Server.HoldApply() (release func())`:
    the commands acknowledged while held are applied at the release,
    as a sleeping device confirms after its wakeup; a test of its own.
  - `internal/server/etag_test.go`, `TestProgramTag`: hold, PUT,
    "pending", 412, release, then the confirmation;
    follow-ups that outlast the release.
  - `internal/server/sources_test.go`, `TestRoomProgramTagOfTheMembers`;
    `internal/server/confirm_test.go`, `TestLockAndDisplayConfirmed` and
    `TestRoomCommandConfirmed`: the same, where they race `ApplyAfter`.
  - `internal/store/events_test.go`, `TestEventSize`: its batch through
    `writeEvents` directly; it measures bytes per row, not the writer.
  - `internal/server/proxy_retry_test.go`, `TestProxyFailingInTheAPI`:
    `tries >= 1`.
  - The sweep's rest: each fixed in a line where `HoldApply` or the kit's
    scale does it; the others recorded in the block's NOTE.
- **Tests:** planted delays by overlay: 1 s before `setRoomProgram`'s answer
  (item 1), 6 ms per row in `writeEvents` (item 2),
  50 ms before `putCube`'s answer (item 3); old fails, new passes;
  `-race -count=5` of each changed test.
- **CHANGELOG:** a fragment for `HoldApply`, `internal/` being shipped,
  under the heading `CHANGELOG.d/`'s rules give it.
- **Commits:** a commit per fix, `test(server): …`, `test(store): …`,
  the fake's first.
- **Verification:** the plants; `go test` of the touched packages,
  `make race` for them; `gofmt -l .`; `go vet` for linux, windows and darwin.

### Block 3: UiScale

- **Goal:** the UI waits stretch on a slow runner and in the retry,
  and the page's start says which read it waited for.
- **Changes:**
  - `tools/ui-test/helpers.py`: `scaled(seconds)`, from `UI_TEST_SCALE`
    (a positive number, refused otherwise), inside `wait_until`, `settled`,
    `Stream.wait`, `wait_held`, `ApiReads.answered`'s ceiling,
    `wait_log_live`, `scroll_settled` and the fold helpers;
    never inside a window that waits for nothing to come.
  - `tools/ui-test/conftest.py`: the page fixture's default timeout,
    its `.room` and its stream wait scaled;
    the page's start one helper with `owner_page`, 30 s, naming on a timeout
    the API reads still open and the message line (item 7).
  - The test files' own loops (`wait_count`, `back`, `wait_reads`) scaled.
  - `tools/ui-test/test_header.py`, `test_the_address_names_the_page`:
    its checks right after a wait wait themselves (item 8).
  - `tools/ui-test/run.sh`: passes the scale on, raised for the retry.
  - `docs/ui-testing.md` section 9 and `tools/ui-test/README.md`: the scale.
- **Tests:** the first rooms read answered 11 s late: the old fixture fails,
  the new passes; a scale of `0` or `abc` refused;
  the touched files in the four cases;
  the whole UI suite once in one case.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): The UI tests' waits take UI_TEST_SCALE`,
  and the page's start in a commit of its own.
- **Verification:** the plant; the four cases; the whole suite in one case.

### Block 4: PageReads

- **Goal:** one wait for a page whose reads all answered,
  and no new copy of the old idioms.
- **Changes:**
  - `tools/ui-test/helpers.py`: `ApiReads.answered` follows a read the page
    starts only after another answered (the burst reader, the page's start);
    the files' own copies that mean the same take it
    (`test_maint_updates.py`'s `answered`, and the others found);
    the helpers defined in several files as one thing move into
    `helpers.py`.
  - A Go test, as `tools/identifiers`: per test file the count of
    `wait_for_timeout`, of plain `page.route` and of local definitions of a
    kit helper's name; a count above its listed one fails.
  - `docs/ui-testing.md` section 9: the wait, the guard and its list.
- **Tests:** a planted chained read: the old `answered` returns before it,
  the new one after; the guard fails on a planted new use and on a count
  left above a fall; the touched files in the four cases;
  the whole UI suite once in one case.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commits:** the wait, then the moved helpers, then the guard.
- **Verification:** the plants; `go test ./tools/...`; the four cases;
  the whole suite in one case.

### Block 5: KnownCube

- **Goal:** a test file begins from the recorded Cube,
  and the server sends nothing on its own between a test's steps.
- **Changes:**
  - `internal/cubelan/fakecube`: the control's `POST /reset`:
    back to the greeting it started with (devices, rooms, M lines,
    no joined device), the connections dropped so that the server reads it.
  - `cmd/massimo/serve.go`, `internal/server`: `-poll 0` means never,
    as `watch -poll 0` does; the setting's range stays.
  - `tools/ui-test/run.sh`: the fake with `-control`, the servers with
    `-poll 0`; `conftest.py`: a reset before each test file,
    waiting until the server has read the Cube again.
  - The tests that need a device list read ask for it.
  - `README.md` (`serve -poll`), `docs/api-v1.md` where the settings'
    `fixed_by` is described, the fake's README part,
    `docs/ui-testing.md` section 9.
- **Tests:** a Go test that `-poll 0` sends no `l:` (`testkit.Never`);
  the reset's Go test; the suite run with `-poll 0` before the change of
  `run.sh`, its failures listed and fixed;
  the whole UI suite once in one case.
- **CHANGELOG:** a fragment for `-poll 0` and one for the fake's reset.
- **Commits:** the fake's reset; `-poll 0`; the UI run's use of both.
- **Verification:** `go test` of the touched packages, `make race` for
  them; `gofmt -l .`; `go vet` for linux, windows and darwin;
  the whole UI suite in one case.

### Block 6: StateGuard

- **Goal:** no new test puts its data into the page alone.
- **Changes:** Block 4's guard counts the writes of `state.<field>`
  in a test file's page scripts, against a list that only shrinks;
  its failure names `Reads` and `docs/ui-testing.md` section 9.
- **Tests:** the guard fails on a planted write and passes on main's files.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): No new test writes the page's state alone`
- **Verification:** `go test ./tools/...`.

### Block 7: FakeCommands

- **Goal:** a command the server builds that the fake does not handle
  fails a test.
- **Changes:** a Go test beside the fake that lists the commands of
  `internal/protocol/cube` and the fake's apply cases, and fails where a
  command is neither applied nor named as ignored, with its reason.
- **Tests:** the test fails with a planted builder, and passes on main
  after AP FakeGroup.
- **CHANGELOG:** none: only a test changes.
- **Commit:** `test(cubelan): The fake Cube handles or names every command`
- **Verification:** `go test ./internal/cubelan/... ./internal/protocol/...`.

### Block 8: NightlySoak

- **Goal:** flakes show on main overnight, not in a feature's PR.
- **Changes:**
  - `tools/ui-test/conftest.py`: `UI_TEST_REPEAT` runs each test so many
    times, `UI_TEST_SHUFFLE` orders them by a seed it prints;
    no new library.
  - `.github/workflows/ui-soak.yml`: nightly and by hand, the UI suite in
    the four cases, three times shuffled, the tests that failed once listed
    in the run's summary; it gates nothing.
  - `GUIDELINES/project/CI.md` and `tools/ui-test/README.md`: the soak.
- **Tests:** a repeat of 2 with a seed on one file runs it twice,
  in the printed order; `make actionlint`.
- **CHANGELOG:** a fragment, `.github/` being shipped.
- **Commit:** `ci(ui-soak): A nightly run repeats the UI tests in a shuffled order`
- **Verification:** `make actionlint`; one run by hand on the branch.

### Block 9: GradleCache

- **Goal:** the Android jobs read Gradle's downloads from a cache,
  not from Maven Central on every run.
- **Changes:** `.github/workflows/ci.yml`: the jobs Android app and
  Android emulator restore and save Gradle's caches and wrapper,
  keyed by the build's Gradle files, as the Go caches are.
- **Tests:** `make actionlint`; the pull request's run of both jobs,
  the second run reading the cache.
- **CHANGELOG:** a fragment, `.github/` being shipped.
- **Commit:** `ci(android): The Android jobs cache Gradle's downloads`
- **Verification:** `make actionlint`; the jobs' logs show the cache.

### Before each pull request

The branch merges `origin/main`;
`GUIDELINES/shared/tools/check` passes;
the pull request's text in commit form, a block's proofs in it.

## Proposed commit message

The plan: `chore(aps): +SteadyTests v1.0, +ANALYSIS SteadyTests`

Block 1:

```
test(ui-test): Three tests wait for the moment they check

For AP SteadyTests v1.0, Block 1 (D1-A): the resync test waits on the
new start's own data, which its old wait never compared as it meant
to; the event test waits for the reboot read queued behind the
settings read; the sign-in test waits for the stream the page's
restart opens, not the one it closes.

Key changes:
- tools/ui-test/test_maint_updates.py: two tests' waits.
- tools/ui-test/test_sign_in_resend.py: the second ticket.
- No CHANGELOG bullet: tools/ is not shipped.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-26): first revision, from the owner's `PLAN++`
  on the TL;DR gate of 2026-11-26: D1-A, D3-A, D4-A, D5-A; D6 open.
