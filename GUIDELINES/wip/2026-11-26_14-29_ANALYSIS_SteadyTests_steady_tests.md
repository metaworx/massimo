# ANALYSIS SteadyTests: The nine flakes of AccessEnds, and the kit they lead to

Measured on `origin/main` at 2026-11-26 14:29
(`chore(aps): +NOTE AccessEnds v1.1, +NOTE SecondFactor v1.0, the gate's decisions`).
Follows `wip/2026-11-26_13-26_ANALYSIS_SteadyTests_why_so_many_flaky_tests.md`.
The causes were found by two investigators.
Each fix below was proven in a run: the old code fails with a planted delay,
the new one passes.
No project file was changed.

## TL;DR

The nine flakes the session "UI refactor" sent fall into two classes,
neither of them yesterday's (the server's own traffic):

- **A fixed window that a slow or loaded runner overruns:** 1, 2, 3, 7 and probably 8.
- **A wait on a moment that proves nothing:** 4, 5 and 6.
- **CI, not a test:** 9.

So the plan fixes the nine first, each with the kit piece of its class.
Then it adds what keeps each class closed: a scale for the UI waits,
one wait for a page that has finished reading, and guards against the old idioms.
After that come yesterday's pieces: a known Cube per file, the guard on `state`,
the fake's command list and the nightly repeat.
Moving every region onto `Reads` and Playwright's `expect` is a plan of its own,
once the guards stand.

## 1. The nine, with their causes

| # | Test | Class | Cause | Fix |
|---|------|-------|-------|-----|
| 1 | `TestProgramTag` (Go, armv6) | window | the fake applies the programme 600 ms after its ack (`fakecube/apply.go:57`); the test's GET (`etag_test.go:89-92`) races it | `fakecube.Server.HoldApply()`: the test holds the apply, checks "pending", then releases it |
| 2 | `TestEventSize` (Go, race) | window | 2000 events through the writer, which `FlushEvents` waits for 10 s, unscaled (`store/events.go:50`) | the test writes its batch with `writeEvents` directly: it measures bytes per row, not the writer |
| 3 | `TestProxyFailingInTheAPI` | window | the proxy's own retry, 5 ms after the PUT's try (`runtimes.go:347-376`), comes before the answer is built | `tries >= 1`, as the same test's System check has |
| 4 | `test_system_info_reads_again_at_a_resync_while_shown_only` | wrong wait | its wait compares `inner_text` with `textContent` (`test_maint_updates.py:1213-1217`), which never match, so it passes at once; the facts read comes after the store's | wait on the start's own `data-at` |
| 5 | `test_an_event_keeps_a_question_and_takes_back_one_whose_reboot_waits` | wrong wait | the reboot read waits behind the settings read in the burst reader (`shared/burst.ts:41-45`); the file's own `answered` (`:323`) does not wait for it | `wait_reads(page, reads, 1)` first |
| 6 | `test_stream_ticket_waits_for_the_sign_in` | wrong wait | `back()` (`test_sign_in_resend.py:84-93`) returns on the first stream after the sign-in, which `afterLogin` closes (`login.js:636-647`) | wait for the second ticket |
| 7 | `test_log_of_every_cube_loads_again_after_409`, setup | window | the page fixture waits 10 s for `.room` (`conftest.py:317`, also `owner_page`), but the page's own reads may take 20 s (`API_WAIT_MS`) | one helper for both, 30 s, naming the reads still open when it gives up |
| 8 | `test_the_address_names_the_page` | probably a window | checked once right after a wait (`scrollY`, `is_visible()`), under load 28 | with the scale (Block 3) |
| 9 | Android emulator, Maven Central 429 | CI | neither the Android nor the emulator job caches Gradle's downloads | a Gradle cache |

Item 4 failed on main's CI once, in the reported run 45631931548,
not in several runs: none in 83 main runs from 2026-11-21 to 24,
none on a branch since 2026-11-24.
The other red main runs of that day were other flakes.

## 2. What the nine show about the kit

- **The UI waits do not scale.**
  Go has `MASSIMO_TEST_SCALE` (race 2, armv6 4); the UI suite has nothing.
  `wait_until` takes literal seconds at about 1,160 places, every kit wait
  defaults to 3 or 5 s, and the fixture's to 5 and 10 s.
  The retry runs with the trace, about 60 times slower per step,
  in the same windows.
  A `scaled(seconds)` in `helpers.py`, read from `UI_TEST_SCALE`,
  inside each kit wait and the fixture, needs no call site changed.
  It must not stretch checks that wait for nothing to come.
- **Files write their own waits.**
  The 75 test files define 499 helpers of their own.
  Several names recur (`loaded` four times; `shown`, `open_panel`, `stub_me`
  three times).
  Item 5's `answered` is one of them:
  a fetch that overtakes the page's own request,
  not the kit's `ApiReads.answered`, which waits for every request in flight.
  Neither follows a read that starts only after another has answered,
  as the burst reader and the page's start do.
- **Go's fake applies on a timer, and tests race it.**
  `HoldApply` gives the tests the moment instead.
  The same race is in `TestRoomProgramTagOfTheMembers` (`sources_test.go:210`),
  `TestLockAndDisplayConfirmed` and `TestRoomCommandConfirmed` (`confirm_test.go`).
  Five more candidates are listed in section 4.

## 3. The blocks, with both gates' items in one order

| Block | What | From |
|-------|------|------|
| 1 | items 4, 5, 6 fixed in their tests | the list |
| 2 | `HoldApply`; items 1, 2, 3; the three tests of the same race | the list, the Go sweep |
| 3 | `UI_TEST_SCALE` in the kit and the fixture, raised for the retry, set per CI job; the page's start as one helper (items 7, 8) | the list; the earlier analysis's item 2 |
| 4 | one wait for a page whose reads have all answered, chained ones too, replacing the files' copies; a guard against new `wait_for_timeout`, plain `page.route` and local copies of a kit helper, outside a list that only shrinks | the earlier item 2 |
| 5 | a known Cube per test file: the fake's control in the UI run, a reset to the recorded greeting between files; `serve -poll 0` meaning never, the UI run without the poll | the earlier item 3 |
| 6 | a guard against a test file writing `state.<field>`, outside a list that only shrinks | the earlier item 1 |
| 7 | a Go test that fails where the fake neither applies a command the server builds nor names it as ignored | the earlier item 4, after AP FakeGroup |
| 8 | a nightly UI run, each test three times in a shuffled order, its flaky tests listed | the earlier item 5 |
| 9 | a Gradle cache in the Android and emulator jobs | the list |

Left to a plan of its own:
moving each region onto `Reads` and region helpers on `expect`,
with D3 settled there.

## 4. The Go sweep, not run

Fixed windows a loaded runner can overrun, besides the three of Block 2:

- `fakecube/apply_test.go:186-190`, `TestApplyBoost`: a 200 ms boost, read right after the send.
- `fakecube/apply_test.go:298-302`, `TestApplyDelayDiscardAndOff`: a 300 ms apply, raced by its "not yet" read.
- `cubelan/search_test.go:148/167`, `TestSearchWaitsForALateAnswer`: an answer 50 ms before the wait ends.
- `confirm_test.go:90/227`: `confirmWithin` 3 s, unscaled, for a confirmation that comes.
- `server/log.go:400`: the log's flush gives up after the same 10 s as item 2, then reads anyway.

Block 2 takes the ones `HoldApply` or the scale fixes in a line;
the rest stay listed in its NOTE.
