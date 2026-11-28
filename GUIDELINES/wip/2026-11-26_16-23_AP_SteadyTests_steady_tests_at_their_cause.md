# AP SteadyTests v1.2: Steady tests at their cause

## TL;DR

Flaky tests fixed at their cause, a pattern at a time:
each fix is a kit helper, documented, which every test with the pattern uses,
and a guard keeps the pattern closed
(`wip/2026-11-26_14-29_ANALYSIS_SteadyTests_steady_tests.md`,
`wip/2026-11-27_13-12_ANALYSIS_SteadyTests_eval_and_soak.md`).
v1.0's nine blocks carried AccessEnds' nine flakes;
v1.1 added five for what the run, the soak and other sessions found;
v1.2 adds seven for the flakes of 2026-11-28 and the page defects behind them.

- fixed windows → the fake's apply held, `UI_TEST_SCALE`, a write made due
  by the test, timed UI states held
- waits on the wrong moment → one wait for the page's reads, several boxes
  read at once, waits for the request itself
- the server's own traffic and earlier tests' leftovers → a known Cube and a
  server left as found, per test file; no poll in UI runs
- a pattern found → a helper in the kit, with its page in the tests' front page
- flakes → found by a nightly soak; Gradle's 429 → a cache
- a room check skipped → the rig waits for the server's own reasons
- the host's network change → the guard's, not the page's
- a refused change → taken back (settings, #498; schemes); two page races

### Blocks

1. WrongWaits: items 4, 5 and 6 wait for what they check
2. HoldApply: the fake Cube holds its apply; items 1, 2, 3; three tests of the same race
3. UiScale: `UI_TEST_SCALE` in the kit's waits; the page's start (items 7, 8)
4. PageReads: one wait for a page whose reads all answered; the idioms' guard
5. KnownCube: the fake reset between test files; `serve -poll 0`; UI runs without the poll
6. StateGuard: no test file writes `state.<field>` outside a shrinking list
7. FakeCommands: the fake applies or names every command the server builds
8. NightlySoak: a nightly UI run, each test three times, shuffled
9. GradleCache: a Gradle cache in the Android and emulator jobs
10. FrontPage: one page for every suite; the authors' rule; the kit by pattern
11. ServerLeftovers: a test file leaves the server as it found it (the soak's A, B, D)
12. GoWindows: the Go tests' windows set by the test, the fake's silent lines
13. UiMoments: timed UI states held, several boxes read at once, waits for the request
14. AgedCell: a reading's age does not widen its cell; the test pins it
15. RoomChecksReady: the room checks' reasons to skip, one function the rig waits on
16. AppPageKit: `ConnectPageTest` takes `Page.kt`'s waits
17. DiscoveryReconnect: the moved Cube's wait can hold, as long as a loaded host needs
18. HostNoise: the guard tells the host's network change from the page's errors
19. RefusedSchemes: a refused colour scheme goes back, as a refused setting does
20. TicketSwitch: a Cube switch drops the ticket asked for the other Cube
21. LatePairingRead: a pairing read older than a join's event keeps the joined device

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

The owner, 2026-11-27, `PLAN+` on the checkpoint gate of the same day,
every decision not answered taking its recommended option:

- **The authors' rule** (the owner's words): local individual fixes do not
  go on where there is a pattern; a test's author evaluates using a helper
  and documents it, so that other tests reuse it.
  Each block of v1.1 begins with a sweep for its pattern's other places;
  its fix is a kit helper, documented, which every place found takes;
  a local fix stays only where the sweep finds the pattern once,
  and its commit says so.
- **D6-A.** Block 10: `docs/testing.md` becomes the front page of every suite,
  and `docs/ui-testing.md` opens with its kit, the runner comparison moved
  to its end.
- **D7-A.** The findings outside v1.0's blocks become Blocks 10 to 14.
- **D8-A.** Block 11 goes into #473 before its merge,
  so that the soak's first night says something.
- **D9-A.** Blocks 12 to 14 go into `maint/0.17` and are merged upward:
  the release line runs the same tests, and Block 14 fixes a defect of 0.17.

Open, for the plan gate of v1.1:

- **D10.** Where the authors' rule is written:
  - **A (recommended).** A sentence in the contract's "Tests"
    (`GUIDELINES/project/_CONTRACT.md`, inlined into `AGENTS.md`),
    every agent's first reading, and the rule with the kit's helpers by
    pattern in `docs/testing.md`.
  - **B.** In `docs/testing.md` only.
- **D11.** What Block 11's check does with a file that leaves the server
  otherwise than it found it:
  - **A (recommended).** It fails that file, naming what it left,
    as Block 5's check of pending writes does;
    what no API can undo (log lines) is listed with its reason.
  - **B.** It warns in the log and fails nothing.

The owner, 2026-11-27 and 2026-11-28, recorded in the NOTEs
`wip/2026-11-27_15-15_NOTE_SteadyTests_v1.1_the_gates_decisions.md` and
`wip/2026-11-28_10-40_NOTE_SteadyTests_v1.1_the_refused_change_and_block_7.md`:
D10-A, D11-A, D14-A (#498, merged into `maint/0.17`), D15-A (Block 7 on #473),
and D16 for planning.

The owner, 2026-11-28, `PLAN++` for v1.2 with D19-B and D20-A,
D17 and D18 taking their recommended options:

- **D16-A.** The flakes from AP MobileApp's and AP ErrorCodes' runs go into
  v1.2 by pattern: Blocks 15 to 18. The emulator's slow first boot is
  recorded; the app's process killed with Google Play services' is a choice
  in the app for AP MobileApp.
- **D17-A.** The room checks' reasons to skip a room become one server
  function, which the room checks and the test rig both use,
  so that the rig cannot fall behind the code.
- **D18-A.** The discovery reconnect and the guard's host noise are
  Blocks 17 and 18.
- **D19-B.** #498 stays as it was; the same defect for colour schemes
  is Block 19.
- **D20-A.** The two page races found in passing, the ticket across a Cube
  switch and the pairing read handled after a join, are Blocks 20 and 21,
  in one product-fix pull request into `maint/0.17` after #498,
  with Block 19.

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
- **v1.1's pull requests:** Block 11 onto `claude/steady-kit` (#473);
  Blocks 12, 13 and 14 in one pull request into `maint/0.17`,
  merged upward after it; Block 10 against `main` once #473 has merged,
  since it documents #473's kit too.
- **v1.1's plan commits** go onto `maint/0.17`, where Blocks 1 to 3 and 9
  shipped, and are merged upward (the owner's rule after the freeze).
- **v1.2's pull requests:** Blocks 15 to 18 in one against `main`
  (`claude/steady-flakes`): the tests they fix are on `main` only, or the kit
  is the same on both lines and `main` runs it first;
  Blocks 19 to 21 in one into `maint/0.17` (`claude/steady-page-races`),
  merged upward after it: they fix defects of 0.17.
  Builders: one for the Go and UI kit blocks (15, 17, 18),
  one for the app's (16), one for the page's (19 to 21).
- **v1.2's plan commit** goes onto `maint/0.17` and is merged upward,
  with #498.

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

### For Blocks 10 to 14: the patterns

`wip/2026-11-27_13-12_ANALYSIS_SteadyTests_eval_and_soak.md` holds the
causes and proofs of the owner's eight, the pairing test and the soak's
first run; the NOTE of v1.0's run lists the rest
(`wip/2026-11-26_22-05_NOTE_SteadyTests_v1.0_how_the_run_went.md`,
"Found outside the plan").
Grouped by pattern, not by test:

- **A window the test should set** (Block 12, 13):
  a fixed product timer or a rig's window that a slow runner overruns.
  The resend window of `TestUnconfirmedRoomCommandNotResent`,
  the drift rig's poll overtaking `setRoom2`'s "pending",
  `TestConfigWriteConfirmed`'s apply, the 250 ms client timeout of
  `TestDownloadOutlastsTheClientTimeout`, the retries of
  `TestProxyRetriedUntilTheAddressIsThere`;
  in the UI, the 1400 ms "copied" fade.
  The kit already has `HoldApply` and the scales; the new helpers let a
  test make a write due (`dueNow`), wait for a try made after a change,
  and hold a timed UI state.
- **A wait on the wrong moment** (Block 13):
  two reads where one moment is checked (the pairing test's two boxes;
  `bounding_box()` is called in 16 test files),
  a check before the request it checks (`test_automatic_updates`;
  the two sign-in tests' settings sync, to be confirmed).
- **An earlier test's leftover on the server** (Block 11):
  the login warning four tests leave puts a mark on ☰ for every later page;
  `test_register_and_link` leaves a disabled integration its next run joins;
  `test_moving_inside_asks_nothing`'s 401 and
  `test_a_joined_device_keeps_what_is_typed`, to be confirmed.
  Block 5 resets the fake Cube per file; the server's users, integrations,
  settings and unseen warnings are not.
- **The fake doing less than a Cube** (Block 12):
  `TestPollSkipsABusyLine`: the fake records one line at a time,
  so an `f:` after a poll's `l:` is missed while it is silent.
- **Text whose length grows with the run** (Block 14):
  a reading's age reaches two digits in a run over 10 minutes,
  and the holiday tile overflows in German at 1100 px; a real server shows
  two-digit ages all day, so the page is wrong, not the test.

### For Blocks 15 to 21: the flakes of 2026-11-28

AP MobileApp's session sent four flakes from its run (#496, #497),
AP ErrorCodes' session two; Block 11's builder and the builders of #473's
CI fix and of #498 found the page defects. Each passed on a rerun.

- **A room check skipped** (Block 15): `TestRepairPlaceHoldsTheRoomsState`
  on armv6, "the re-send: []". `resendRoom` waits for the queue's answer,
  so a re-send made is recorded: none was made. `roomChecks` skips a room
  while a structure change is sent, the queue holds a command for it,
  a room confirmation is open or the room is watched;
  the rig's `settled()` waits for the last of these only.
  A first check that skips the room starts its ten minutes late,
  so `disagreeAndResend`'s third check comes too early.
  It passed 300 of 300 locally, on one CPU at scale 4; QEMU is slower.
  The rig's checks are called in 76 places
  (`drift_test.go`, `resend_test.go`, `config_api_test.go`).
- **A private copy of the kit's wait** (Block 16):
  `ConnectPageTest.theBridgeAnswersTheConnectPage` timed out on its own
  5 s `eval` while the WebView's first start stalled the main thread 4 s.
  `Page.kt`'s `waitFor` allows a slow `eval` per poll within 30 s and wraps
  the same `ActivityScenario`; `ConnectPageTest`'s are the only private copies.
- **A wait that may never hold** (Block 17):
  `TestCubeConfiguredBySerialFollowsItsAddress` timed out once after 5 s
  waiting for the moved Cube (`discovery_test.go:642`). Either the reconnect's
  backoff outlasts an unscaled 5 s on a loaded host, or
  `second.Connections() == 1` cannot hold once the client has connected twice.
- **The host's noise as the page's** (Block 18):
  `test_reboot_waits_and_is_taken_back[390-en]` failed on Chromium's
  `net::ERR_NETWORK_CHANGED`, a change of the host's network (another
  session's Docker network, for one); the guard fails a test on any console
  error.
- **A refused change kept by the browser** (Block 19):
  #498 takes a refused setting back; a refused colour scheme is still kept,
  and a sign-in hands it to a server without the user's schemes.
- **A late answer over a newer event** (Blocks 20, 21):
  `connectEvents` reads `state.serial` again once the ticket comes, so a Cube
  switch meanwhile opens one Cube's stream with the other's ticket
  (Block 11 planted it: a console 401);
  `cubes/flow.ts`' `apply()` lets a pairing read answered before a join,
  and handled after the join's event, replace the list and drop the device
  until the pairing ends (#473's CI fix found it).
- **Recorded only:** the emulator's first boot past `APP_EMU_BOOT_TIMEOUT`
  once in CI; the app's process killed when Google Play services' died,
  likely appcompat's emoji font from its font provider, unconfirmed,
  for AP MobileApp.

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

### Block 10: FrontPage

- **Goal:** one page says which suites exist, how each runs, how a steady
  test is written, and which helper serves which pattern.
- **Changes:**
  - `docs/testing.md`: the front page: the suites (Go, the UI suite, the
    soak, the LAN test, the emulator, the hardware plans) with their
    commands; the rules of a steady test; the authors' rule; the kit's
    helpers by pattern, Go and UI, each with when to use it and where it
    is described; the Go waits that are there today stay below it.
  - `docs/ui-testing.md`: opens with its kit (today's section 9 and the
    guard), the runner comparison moved to its end.
  - D10-A: the contract's "Tests" gets the authors' rule in a sentence,
    `AGENTS.md` and `GUIDELINES/README.md` regenerated from it.
  - `README.md`'s documents list names the front page.
- **Tests:** none: documents only; `GUIDELINES/shared/tools/check`.
- **CHANGELOG:** none: documents only.
- **Commits:** `docs(testing): …`; the contract's sentence in a commit of
  its own.
- **Verification:** `check`; every helper the page names exists
  (a grep in the commit's proof).

### Block 11: ServerLeftovers

- **Goal:** a UI test file leaves the server as it found it, and the soak's
  failures of causes A, B and D end.
- **Changes:**
  - The sweep: every test that leaves a user, an integration, a setting,
    a preference, a session or a warning behind, in a whole seeded run.
  - `tools/ui-test/conftest.py`: `clear_preferences` marks every warning
    present as seen; after each file's last test, beside Block 5's check of
    pending writes, a check that the server's users, integrations and
    settings are as at the file's start (D11-A), failing that file and
    naming what it left; what no API undoes is listed with its reason.
  - `helpers.py`: a helper for an id of a test's own per run
    (`test_register_and_link`'s installation and subject), and whatever the
    sweep finds that several files undo alike.
  - The tests the sweep names undo what they set; `test_update.py`'s
    automatic updates wait for the request (cause D), with the helper
    Block 13 makes if it fits, else the kit's request wait.
  - `test_moving_inside_asks_nothing` and
    `test_a_joined_device_keeps_what_is_typed`: their cause confirmed;
    each goes to the block of its pattern.
  - `tools/ui-test/soak_summary.py`: each failure names its worker,
    and the retry's outcome.
  - `docs/ui-testing.md` section 9.
- **Tests:** the soak's seed 45895569993 in 390-en, three repeats:
  the old tree fails 11, the new none; the check fails a planted file that
  leaves a user; the whole suite once in 1100-en, shuffled.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commits:** the warnings; the check; each pattern's helper with the tests
  that take it.
- **Verification:** the plants; the four cases of the touched files;
  the soak on #473 green, or each failure left named with its cause.

### Block 12: GoWindows

- **Goal:** a Go test sets the moments it checks instead of racing a window.
- **Changes:**
  - The sweep: the Go tests that wait out or race a rig's window or a
    product timer, from the NOTE's list and a grep.
  - The kit: a rig helper that makes a room's open writes due now
    (`dueNow`), for the resend and confirmation tests
    (`TestUnconfirmedRoomCommandNotResent`, `resend_test.go:170`, `:313`);
    a wait for a try made after a change, for the proxy's retries
    (`TestProxyRetriedUntilTheAddressIsThere`);
    `setRoom2` and `TestConfigWriteConfirmed` hold the fake's apply
    (`HoldApply`); client timeouts against a local fake scaled
    (`TestDownloadOutlastsTheClientTimeout`).
  - The fake Cube records the lines it reads while silent
    (`TestPollSkipsABusyLine`).
  - `docs/testing.md`: each helper.
- **Tests:** a planted delay per pattern: the old test fails, the new
  passes, three runs each; `-race` at scale 2, `-count=5`.
- **CHANGELOG:** a fragment where a non-test file of `internal/` changes.
- **Commits:** a commit per helper with the tests that take it.
- **Verification:** `go test` of the touched packages, `make race` for them;
  `gofmt`; `go vet` for linux, windows and darwin; Windows through interop for
  `internal/update`.

### Block 13: UiMoments

- **Goal:** a UI test reads one moment once, and holds what fades.
- **Changes:**
  - The sweep: the tests that read two boxes for one check
    (`bounding_box()` in 16 files), that check a request's effect before
    the request, or that race a timed state (fades, toasts, marks).
  - `helpers.py`: a read of several boxes at once; a hold of a timed state
    (the "copied" fade first); the request wait the sweep shows is needed.
  - The places found take them: `test_pairing.py`'s, `test_cubes.py`'s and
    `test_repair.py`'s two-box checks, `test_system_info`'s fade,
    the sign-in tests' settings sync once confirmed.
  - `docs/ui-testing.md` section 9; Block 4's guard counts a new
    two-box read, if the sweep shows it can be counted.
- **Tests:** a planted redraw between two reads, a planted 1.6 s before
  the fade's check, a planted late request: old fails, new passes;
  the touched files in the four cases; the whole suite once in 1100-en.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commits:** a commit per helper with the tests that take it.
- **Verification:** the plants; the four cases.

### Block 14: AgedCell

- **Goal:** a reading's age never widens its cell, and the tests do not
  depend on how long a run has lasted.
- **Changes:**
  - `internal/server/ui/style.css` (the aged cell's grid, around line 1448):
    the age's track does not widen the cell.
  - The sweep: the tests whose page shows a text that grows with the run
    (ages, durations); they pin it with the kit's `pause_clock` or a told
    time.
  - A UI test at 1100 px in German with a two-digit age.
- **Tests:** the old page overflows with "≥ 17 Min.", the new does not,
  in the four cases.
- **CHANGELOG:** a fragment under Fixed: the page is shipped.
- **Commit:** `fix(ui): A reading's age does not widen its cell`
- **Verification:** `make ui-check`; the four cases of the touched files;
  the screenshots at 1100 and 390 px.

### Block 15: RoomChecksReady

- **Goal:** the drift rig's checks run when the room checks would check
  the room, so that a test's ten minutes start when it says.
- **Changes:**
  - `internal/server/drift.go`: the reasons `roomChecks` skips a room
    (a structure change sent, the queue holding a command for the room,
    an open room confirmation, a watched room) become one function,
    which `roomChecks` uses (D17-A); no behaviour changes.
  - The rig (`drift_test.go`, `config_api_test.go`): before each check, a
    wait until no reason holds for the rooms the test checks, through that
    function; `settled` and `disagreeAndResend` take it.
  - The sweep: every test that calls the rig's checks or `roomChecks`
    itself takes the wait.
  - `docs/testing.md`: the helper.
- **Tests:** a command held for the room across the first check
  (`HoldApply`): the old `TestRepairPlaceHoldsTheRoomsState` fails with
  "the re-send: []", the new passes, three runs each;
  `-race` at scale 2, `-count=5`; `make test-armv6` where it runs.
- **CHANGELOG:** as the project does for a change of `internal/` without
  a change a user sees.
- **Commits:** the function; the wait with the tests that take it.
- **Verification:** `go test ./internal/server/...`, `make race` for it;
  `gofmt`; `go vet` for linux, windows and darwin.

### Block 16: AppPageKit

- **Goal:** `ConnectPageTest` waits as every app test does.
- **Changes:**
  - `app/android/app/src/androidTest/…/ConnectPageTest.kt`:
    `Page(sc).eval` and `waitFor` in place of its private copies.
  - The sweep: the app tests' other private waits (none found yet);
    the app kit's waits named where the app tests are documented.
- **Tests:** a main-thread stall of 6 s planted before the first `eval`:
  the old class fails, the new passes; `make app-emulator-test`.
- **CHANGELOG:** none: `androidTest` is not shipped.
- **Commit:** `test(app): …`
- **Verification:** the plant; `make app-emulator-test` once whole.

### Block 17: DiscoveryReconnect

- **Goal:** the moved Cube's wait holds once the Cube is there,
  however loaded the host.
- **Changes:**
  - First the cause: `Connections()` and the reconnect's tries at the
    timeout, with a planted second connect and a planted slow reconnect.
  - `internal/server/discovery_test.go`: the condition counts the
    connections since the move, or waits for the reconnect's own event,
    within the kit's scaled wait; the sweep: the discovery tests with the
    same wait take it.
- **Tests:** the plants: old fails, new passes, three runs each;
  `-race` at scale 2, `-count=5`.
- **CHANGELOG:** none where only tests change.
- **Commit:** `test(server): …`
- **Verification:** `go test ./internal/server/...`; `make race` for it.

### Block 18: HostNoise

- **Goal:** the UI guard fails a test on the page's errors,
  not on a change of the host's network.
- **Changes:**
  - First whether a Chromium launch option keeps it from seeing the host's
    network changes; else `tools/ui-test/conftest.py`'s guard lists
    `net::ERR_NETWORK_CHANGED` as the host's, with its reason.
  - The sweep: CI's and the soak's failures on console lines of the host
    rather than the page.
  - `docs/ui-testing.md` section 9.
- **Tests:** a planted `net::ERR_NETWORK_CHANGED` line: the old guard fails,
  the new passes; a planted other console error still fails.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): …`
- **Verification:** the plants; the whole UI suite once in one case.

### Block 19: RefusedSchemes

- **Goal:** a colour scheme change whose save met a 401 goes back,
  as a setting's does since #498, and no sign-in sends it later (D19-B).
- **Changes:**
  - `internal/server/ui/login.js`, `theme.js`: the schemes that hold, kept
    beside the settings' (`prefsSync.held`); a refused schemes push taken
    back and drawn.
  - The kit: whether the server holds the user's schemes, beside
    `server_holds_settings`, one helper if it fits both.
  - `docs/ui.md`.
- **Tests:** a refused scheme with the server holding schemes and holding
  none: the old page sends it at the sign-in, the new does not.
- **CHANGELOG:** a fragment under Fixed.
- **Commit:** `fix(ui): …`
- **Verification:** the four cases of the touched files.

### Block 20: TicketSwitch

- **Goal:** a stream is opened only with a ticket asked for its own Cube.
- **Changes:** `internal/server/ui/app.js` `connectEvents`: the ticket is
  used for the Cube it was asked for; a switch meanwhile drops it and asks
  for the shown Cube's.
- **Tests:** a ticket held across a Cube switch: the old page's console
  shows the 401, the new opens the shown Cube's stream.
- **CHANGELOG:** a fragment under Fixed.
- **Commit:** `fix(ui): …`
- **Verification:** the four cases of the touched files.

### Block 21: LatePairingRead

- **Goal:** a pairing read older than a join's event does not drop the
  joined device.
- **Changes:** `internal/server/ui/src/cubes/flow.ts` `apply()`: an answer
  asked before the last pairing event does not replace what the event
  brought.
- **Tests:** a pairing read whose body the page reads after the join's
  event: the old page drops the device, the new keeps it.
- **CHANGELOG:** a fragment under Fixed.
- **Commit:** `fix(ui): …`
- **Verification:** `make ui-check`; the four cases of the touched files.

### Before each pull request

The branch merges `origin/main`;
`GUIDELINES/shared/tools/check` passes;
the pull request's text in commit form, a block's proofs in it.

## Proposed commit message

The plan: `chore(aps): +SteadyTests v1.0, +ANALYSIS SteadyTests`;
its revision: `chore(aps): +SteadyTests v1.1, +ANALYSIS SteadyTests eval_and_soak`,
onto `maint/0.17`, then `merge: maint/0.17 into main, with SteadyTests v1.1`;
v1.2: `chore(aps): +SteadyTests v1.2`, onto `maint/0.17`,
then `merge: maint/0.17 into main, with SteadyTests v1.2 and #498`.

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
- v1.1 (2026-11-27): from the owner's `PLAN+` on the checkpoint gate of
  2026-11-27, with the authors' rule: Blocks 10 to 14, a pattern each,
  from the run's NOTE, the soak's first run and the owner's eight;
  D6-A, D7-A, D8-A, D9-A; D10 and D11 open; v1.0's PR shape after the freeze
  (#466, #467 into `maint/0.17`, #473 against `main`) recorded.
- v1.2 (2026-11-28): from the owner's `PLAN++` with D19-B and D20-A,
  D17-A and D18-A as recommended: Blocks 15 to 21 for the flakes of
  2026-11-28 (AP MobileApp's, AP ErrorCodes') and the page defects found by
  Block 11, #473's CI fix and #498; the NOTEs' D10-A, D11-A, D14-A, D15-A
  named; two pull requests, against `main` and into `maint/0.17`.
