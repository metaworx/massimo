# NOTE ServerTraffic v1.0: how the run went

How Blocks 1 to 6 of the plan went:
`wip/2026-11-25_12-26_AP_ServerTraffic_ui_tests_without_the_servers_own_traffic.md`,
on `claude/room-marks-race`,
built by one session for the coordinating one,
under the owner's `PLAN++` (D1-A to D5-A).
Each block is a commit, two for Block 1, as the plan has them,
and Block 4 has a second, for what CI found after the push.
The UI runs took the main checkout's venv, one worker and ports of their own,
checked free with `ss -ltnu`: 8360, 63360 and 24360,
and for the runs beside them 8460, 63460 and 24460.

## Block 1: FreeSlots

- **Built:**
  `massimo fakecube -free-slots N`, 0 to 255, through `SetFreeSlots`;
  without it the fake says what it said before.
  `run.sh` starts each set's fake Cube with `-free-slots 50`.
  `TestFreeSlotsFlag` reads the H line and an S reply without the flag,
  with 33, 50 and 0, and refuses -1, 256 and a word.
  `README.md` has the option,
  `docs/ui-testing.md` v1.25.0 the suite's fake Cube.
- **Proof:**
  a scratch test in 1100-en that writes once through the API,
  waits 2 min 15 s and counts the server's `connected` events.
  With `run.sh` as before: 1, among `hello`, `metadata` and `config` events.
  Its retry, run at once, counted 0,
  since the server reconnects for its budget at most every 15 minutes.
  With `run.sh` now: 0, the write's own `change`, `command` and `state` alone.
  `test_shell_updates.py` passed in the four cases, 22 of 22.
- **The grep:**
  no UI test relies on the fake's own free slots,
  on pending radiograms or on a budget reconnect.
  The tests that show a budget tell it through `reads`:
  `helpers.py`'s `REPAIR_BUDGET`,
  `test_cubes.py` and `test_cube_updates.py` by `reads.cube`,
  and the stubs of `test_refresh_now.py` and `test_admin_updates.py`.
  `test_cubes.py`'s check of the server's own facts asks only that they are not "–".
  `test_confirm.py`'s docstring named the reconnect; Block 2 rewrote it.
- **Deviation:**
  `go vet ./...` fails in this worktree on other sessions' scratch Go files
  under `GUIDELINES/temp/`, which is untracked,
  so vet ran on `go list -e ./...` without them, for linux, windows and darwin:
  the packages a clean checkout has.

## Block 2: Keep

- **Built:**
  `Reads.keep(rf)` in `helpers.py`.
  `PAGE_KIT` adds a listener of `state` and `config` to every stream it wraps,
  a reopened one too.
  After a trusted event of a kept device it calls `loadRooms()`
  once the page's own listeners have drawn the event,
  and counts it in `window.__uiTest.kept`.
  `Reads.clear` of a pattern that the rooms' read matches forgets it (`route_matches`).
  A reload forgets it too, as it forgets a mark.
  `test_confirm.py`'s `READ_AFTER_ITS_EVENTS` gave way to it.
  `docs/ui-testing.md` v1.26.0, "What the reads keep",
  names `test_confirm.py` alone, the one user at that commit.
- **Proof:**
  `test_confirm.py` passed in the four cases, 12 of 12.

## Block 3: Marks

- **Built:**
  `mark_of` reads a place's marks in one `evaluate`.
  The last check waits for the row's eco mark,
  its Set cell found by its label at each look.
  The told thermostat is kept, from the `reads.change` on,
  after the check that no rooms are read for a config event without `changed`.
- **Proof:**
  `PLANT=burst`, 390-en, three runs each.
  The old test failed 3 of 3: `mark_of` found no mark,
  and the check raised a TypeError on None.
  The new one passed 3 of 3.
  The first draft of the wait compared the mark's class, `preset-eco`,
  with `eco`.
  Its planted runs and one unplanted run failed on it,
  and it was fixed before the commit.
  `test_room_marks.py` passed in the four cases, 73 of 73.

## Block 4: Batteries

- **Built:**
  `Device` holds the reads, and `Device.told` keeps the device.
- **Proof:**
  `PLANT=write`, 390-de, three runs each.
  The old test failed 3 of 3, its click timing out with the button gone.
  The new one passed 3 of 3.
  `test_new_batteries.py` passed in the four cases, 12 of 12.
- **Deviation:**
  the scratch plant waited 5 s for the server's state events of the device.
  They come with the server's next poll, every 10 s,
  and with the new fake the 5 s missed them 3 times of 3:
  the old test then passed 3 of 3.
  The plant's copy in `GUIDELINES/temp/servertraffic/` waits up to 12 s,
  and the numbers above are its.

## Block 4 again: the button offered again

- **Found:**
  CI run 45610160341 of the pull request, UI test (390-de),
  failed `test_new_batteries_sent` with "offered again",
  a single `is_visible()` right after `told("weak", False)`;
  the job's retry passed.
  `told` waits only for `state.rooms` to say what was told.
  With `keep`, a server `state` event of the device
  still draws the server's battery first,
  and keep's read brings the told level back only once it is answered:
  the told data come back, but they do not stay,
  and a single look can fall between.
- **Built:**
  every check of the device's drawing after `told` waits for it.
  `Device.shows` waits until the row offers New batteries or not,
  with its badge and its place first among the actions where asked,
  in one look at each try.
  The busy button, the result line with its colour and the button back
  are waited for the same way; `battery_icon` is gone.
  `docs/ui-testing.md` v1.27.0 says that a check of what keep brings back waits for it,
  and names the three tests that take `keep`.
- **The other users:**
  `test_confirm.py` and `test_room_marks.py` stay as they are.
  After `keep` they wait (`wait_until`, `wait_for`,
  a locator's action that waits for its element),
  or check an absence that the server's data draw as well.
- **Proof:**
  `PLANT=state`, 390-de, three runs each:
  a write of the device planted right after the second `told`,
  the test going on as soon as the server's state event of it has come,
  and the page's rooms reads that start within 1.5 s of it held until then.
  The old test failed 3 of 3 with "offered again".
  The new one passed 3 of 3.
  `test_new_batteries.py` passed in the four cases, 12 of 12.

## Block 5: Logs

- **Built:**
  `run.sh`'s `keep_logs` copies each set's `serve.log` and `fakecube.log` into
  `tools/ui-test/results/logs/set<n>/`.
  It runs only where the tests failed, exit 1, after the retry,
  so that they hold both passes.
  `results/logs/` goes before the first pass.
  `tools/ui-test/README.md` says what `results/` holds.
  `.github/` is not touched: CI's upload already takes `results/`.
- **Proof:**
  a failing scratch test in 1100-en left set 0's two logs,
  and with two workers set 1's beside them.
  The same test passing left none.
  shellcheck of `run.sh` is clean.
- **In CI:**
  the 390-de artifact of the pull request's run 45610160341
  held the four sets' `serve.log` and `fakecube.log`,
  and none of the four `serve.log`s has a reconnect,
  which is Block 1 holding in CI.

## Block 6: RebootRoute

- **Found:**
  CI run 45517834691, 390-en, failed
  with "Route.fulfill: Route is already handled!".
  `RebootStub`'s handler raised it (`test_reboot.py`, its GET's fulfill),
  and it came out of the guard's next `page.evaluate` after the test.
  A page that follows a reboot reads GET `/api/v1/reboot` every two seconds.
  The test's cleanup closes the stub,
  whose `page.unroute` takes the page's last route away,
  `counted`'s on the same pattern with it.
  Playwright's driver then removes the page's request interceptor
  and sends every request still in a handler on to the server (`Route.removeHandler`),
  so a handler that answers after that meets a route answered already.
  The driver silences the same error for a fallback of its own,
  but not for a handler's fulfill.
- **Built:**
  `RebootStub` routes through `safe_route`,
  which lets go a request answered already.
  `close` is as it was.
  Its other users, `test_reboot.py` and the other tests of `test_maint_updates.py`,
  take it with it.
- **Proof:**
  `PLANT=slow`, 390-en, three runs each:
  a read of the reboot held in the stub's handler until the stub had closed,
  and the page kept 300 ms after the close, as the guard's checks keep it.
  The old stub failed 3 of 3 with CI's error, in the guard's `scroll_width`,
  its retries too.
  The new one passed 3 of 3, its handler answering after the close in each.
  Unplanted, the new test passed 1 of 1.
  `test_maint_updates.py` and `test_reboot.py` passed in the four cases,
  220 of 220.
- **Not here:**
  other tests' plain `page.route` handlers that fulfill carry the same race
  where a page request is in them as their page's last route goes.
  None has failed on it that this run knows of.

## The checks

- `make ui`, `go test ./cmd/massimo ./internal/cubelan/fakecube`, `gofmt -l .`
  and `go vet` for the three platforms: all exit 0.
- `commit-msg.sh` and `changelog.sh check --staged` passed before every commit.
- `GUIDELINES/shared/tools/check` found one reflow error.
  It was a line of 96 characters in Block 2's text in `docs/ui-testing.md`.
  It found warnings in the new lines too.
  Their line breaks were fixed in fixup commits,
  folded into their blocks' commits before any push,
  so that each block stays one commit.
  `check` then said "all well".
- Block 6 and Block 4's second commit passed `commit-msg.sh`
  and `changelog.sh check --staged`,
  and `check` on the committed tree said "all well".

## Not run locally

The whole UI suite was not run.
The plan asks for it once in one case before the pull request
("the whole UI suite once in one case", `PYTEST_ADDOPTS='-k 1100-en'`),
since Block 1 changes every test's fake Cube.
The owner's brief for this run says "the full suite is CI's":
locally only the touched files ran, in the four cases,
and the whole suite is left to the pull request's CI.

## The pull request's CI

The pull request, #439, ran CI twice.
The first run, on Blocks 1 to 5, failed in UI test (390-de) alone,
the batteries test's "offered again" of the section "Block 4 again",
which passed in the job's own retry.
The second run, on all six blocks and Block 4's second commit,
passed every job (45613812517),
each of the four UI cases at its first pass, with no retry.
