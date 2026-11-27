# ANALYSIS SteadyTests: The owner's eight, the pairing test and the soak's first run

For AP SteadyTests v1.0, after its run (`EXEC eval`, 2026-11-27).
Two investigators found the causes; each fix below was proven in a run:
the old code fails with a planted delay or event, and the new code passes.
No project file was changed.

## TL;DR

- Of the owner's eight, four are covered:
  - `test_copy` by #472;
  - three by #467. The builder that saw them ran on a branch without #467.
- Three are not covered, and each has a small test fix:
  - a product fade outlasted by the retry;
  - a 400 ms resend window;
  - a confirmation the drift rig's poll overtakes.
- The pairing test that turned main red read two moments.
  #473's run without the poll removes what set it off;
  the two reads still need to become one.
- The soak's 33 failures on #473 have four causes, and none comes from #473.
  - A login warning that four tests leave on the server.
    It puts an amber mark on ☰ for every later test, and causes 21 of the 33.
  - An integration test that does not undo itself across repeats: 8 of the 33.
  - A real layout defect in German at 1100 px once a reading's age has two
    digits: 3 of the 33.
  - A wait on the wrong thing: 1 of the 33.

## 1. The owner's eight and the pairing test

| # | Test | Covered | Cause, if not | Fix |
|---|------|---------|---------------|-----|
| 1 | `test_password_suggest.py::test_copy` | #472 | | |
| 2 | `test_system_info_keeps_its_folds_a_copy_and_a_selection` | no | "copied" fades on a fixed 1400 ms product timer (`components.js:340`); copy, menu clicks, re-read and check (`test_maint_updates.py:1157-1163`) must fit in it | a kit `hold_fade`: an observer clears the icon's timer once "copied" is set |
| 3 | `test_stream_ticket_waits_for_the_sign_in` | #467 (Block 1) | the builder's branch, AccessEnds v1.2, did not have it | |
| 4 | `TestProxyFailingInTheAPI` | #467 (Block 2) | same branch | |
| 5 | `TestProgramTag` | #467 (Block 2) | same branch | |
| 6 | `TestUnconfirmedRoomCommandNotResent/superseded` | no | `confirmWithin` 400 ms, unscaled (`resend_test.go:269`); a late device PUT lets the resend go first (`resend.go:162-176`) | a rig helper `dueNow(room)`: the test sets the moment the write falls due |
| 7 | `TestHeldReferenceLifetime` | no | the drift rig polls every 50 ms and the fake applies before `S:`, so a slow handler meets a poll that already shows the value: "confirmed", not "pending" (`drift_test.go:529, 1115, 1125`) | `setRoom2` holds the fake's apply over the PUT (`HoldApply`) |
| 8 | `test_put_into_a_room_from_devices_without_a_room[1100-de]` | partly, #473 | the server's 10 s poll drew the device's values above the form between the check's two reads (`test_pairing.py:129`) | read both rects in one call; a kit helper for the four places that do the same (`test_cubes.py`, `test_pairing.py`, `test_repair.py`) |

The proofs:

| # | Planted | Old | New |
|---|---------|-----|-----|
| 2 | 1.6 s before the second `show_view` | failed 2 of 2 | passed 2 of 2 |
| 6 | 500 ms between the PUTs | failed 3 of 3 | passed 3 of 3, and `-race` 5 of 5 |
| 7 | 200 ms before `setpointConfirm` | failed 3 of 3 with the reported message | passed 3 of 3, and `-race` 5 of 5 |
| 8 | a wait for the poll's values between the reads | failed 2 of 2 within 3 px of CI's numbers | passed 2 of 2 |

`resend_test.go:170` (1.5 s) and `:313` (2 s) hold the same kind of window as
item 6; neither has been seen failing.

## 2. The soak's first run on #473

Run 45895569993: 33 failures on the first pass across the four jobs,
from four causes.

| Cause | Failures | Class | Where | Fix |
|-------|---------:|-------|-------|-----|
| A. A `login_failed` warning that `test_oidc.py:219`, `:426` and `test_opaque.py:86`, `:201` leave stays for the server's life (`logalerts.go:107`); `clear_preferences` (`conftest.py:516`) deletes `log.seen`, so every later page shows it on ☰ (`Menu.tsx:62`), and the mark's 5 px widen the header | 21: the two menu tests (`test_personal_updates.py:108, 336`) and `test_header_folds_step_by_step` | an earlier test's leftover | the kit | `clear_preferences` marks every warning present as seen (`PUT me/preferences/log.seen`) |
| B. `test_register_and_link`'s cleanup only disables the integration (`store/integrations.go:514`); its next repeat, with the same installation id, joins the old one | 8 | a test that does not undo itself | the test | an installation id and subject of its own per run |
| C. The holiday tile overflows in German at 1100 px once a reading's age has two digits ("≥ 17 Min."), which a run longer than 10 minutes reaches (`style.css:1448-1453`) | 3 | a real layout defect | the UI | the age's grid track does not widen its cell; the test pins a two-digit age |
| D. `test_automatic_updates` checks the settings writes once its window shows, before the write (`test_update.py:316`) | 1 | a wait on the wrong thing | the test | wait for the request, as `:333` does |

- **The size of the fix:** cause A's fix took a seeded reproduction from 11
  failures to 2, and the log tests passed with it, 118 of 118.
- **More at risk:** three tests of `test_admin_updates.py` (`:612`, `:873`,
  `:965`) fail the same way in a seeded run.
- **#473 caused none of it.** On main's tree, test_oidc, then test_header,
  then test_personal_updates on one server fail the same three tests.
- **Cause C is a product bug:** a real server shows two-digit ages all day.
  It is in 0.17.
- **What "passed on retry" means in the soak:** the retry lands on whichever
  server is free, so it can mean only "a server without the leftover".
  The soak's summary should name each failure's worker.

## 3. What the follow-ups would be

| Block | What | Target |
|-------|------|--------|
| 10 | the tests' front page (D6) | `main` |
| 11 | the soak's causes A, B and D, and the summary names the worker | #473, before its merge |
| 12 | cause C: the aged cell does not widen, with a test pinning a two-digit age | `maint/0.17`, then upward |
| 13 | items 2, 6, 7 and 8: `hold_fade`, `dueNow`, `setRoom2`'s hold, one read of several rects | `maint/0.17`, then upward |
| 14 | the NOTE's other findings, investigated and fixed: `TestPollSkipsABusyLine`, `TestConfigWriteConfirmed`, `TestProxyRetriedUntilTheAddressIsThere`, `TestDownloadOutlastsTheClientTimeout`, the two sign-in tests, `test_a_joined_device_keeps_what_is_typed`, `test_moving_inside_asks_nothing` | `maint/0.17`, then upward |
