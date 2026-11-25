# AP ServerTraffic v1.0: UI tests without the server's own traffic

## TL;DR

The UI suite's flakes that come from the server's own traffic,
fixed at their source and in the tests, a commit each, one pull request
(`wip/2026-11-25_10-42_ANALYSIS_ServerTraffic_ui_flakes_the_servers_own_traffic.md`).

- fake Cube's S reply `0x31` → one radiogram pending → a budget reconnect
  2 min after any write → `massimo fakecube -free-slots N`,
  `run.sh` every slot free
- `reads.keep(rf)` → the rooms read again after the server's own `state`
  and `config` events of `rf`; `test_confirm.py`'s local copy goes
- `mark_of` → one look; the last check waits for the eco mark
- `test_new_batteries.py`'s told device → kept
- a failed UI run → the servers' logs in its results
- the reboot wait test → its "Route is already handled" race found and fixed

### Blocks

1. FreeSlots: `massimo fakecube -free-slots`, the UI suite's fake Cube with every slot free
2. Keep: `reads.keep(rf)` in `helpers.py`, `test_confirm.py`'s copy replaced
3. Marks: `test_room_marks.py`'s `mark_of` in one look, the last check waiting
4. Batteries: `test_new_batteries.py`'s told device kept
5. Logs: a failed UI run keeps the servers' logs in its results
6. RebootRoute: the reboot wait test's route race

## Discussion

### What is decided

The owner, 2026-11-25, `PLAN++ both, D1-A` on the TL;DR gate of the same day,
every decision not answered taking its recommended option:

- **D1-A.** The budget reconnect goes from the UI suite at its source:
  `massimo fakecube -free-slots N`, and `run.sh` gives the suite's fake Cube
  every slot free.
  Not D1-B, measuring the walk again after a link event.
- **D2-A.** `mark_of` reads the marks in one look,
  the last check waits for the eco mark,
  and the thermostat is kept with the helper.
- **D3-A.** The helper `reads.keep(rf)` on `helpers.Reads`.
- **D4-A.** A failed UI run keeps the servers' logs in its results.
- **D5-A.** `test_maint_updates.py::test_a_reboot_event_while_this_pages_reboot_waits`
  is this plan's last block.

### Decided in the writing

- **The order:** the source first, then the helper before its two users,
  then the logs, and the reboot race last, which needs finding first.
- **The branch** is `claude/room-marks-race`;
  the pull request opens after Block 5, and Block 6 joins it.
- **Not here:** a re-read's double apply, which is AP RereadOnce's.

## Analysis

The three flakes, their reproductions and the helper's design are in
`wip/2026-11-25_10-42_ANALYSIS_ServerTraffic_ui_flakes_the_servers_own_traffic.md`.
It was written before the reconnect's source was known;
what it says of the sources, the settings watch and the walk's fix is
taken over below.

### For Block 1: the budget reconnect

Found by the investigator of the session "AP TestInfrastructure",
2026-11-25, and checked against the code here:

- `afterWrite` (`internal/server/followup.go`) sets the budget's next look
  2 min after a radio write.
- `followUpStep` then reconnects to the Cube once the follow-ups are done,
  at most every 15 min (`FollowUpReconnectGap`),
  only with the periodic refresh on,
  and only where `Budget().Pending()` is above 0.
  `tools/ui-test/run.sh` passes no `-refresh`, so the default 6 h holds.
- The fake Cube's S reply says `0x31` free slots
  (`internal/cubelan/fakecube/fakecube.go`, `freeSlots`),
  which `Pending()` (`internal/cubelan/budget.go`) counts as one radiogram
  of 50 pending.
  So every write in the UI suite arms a reconnect.
- `Server.SetFreeSlots(n)` already sets the S replies and the greeting's H line.
  With 50 free, `Pending()` is 0, and no budget reconnect comes.

The evidence:

- a full local suite showed the reconnects 2:00 after writes,
  and one of them broke `test_sign_in_resend.py::test_role_save_says_not_sent`;
- a write timed so that the reconnect falls in mid-walk failed
  `test_pages_read_at_each_showing_what_they_read` with CI's text,
  on main and before #425;
- the walk alone, with no write before it, passed 60 of 60.

#425 is cleared: the UI suite's servers have no settings file,
so its watch never reloads.
The failures came with it only because the set of tests changed,
which moves the test that falls 2 min after a worker's last write.

What changes for the tests:

- The Go tests build their fake Cube with `fakecube.New` and are untouched.
- The UI suite's server makes no budget reconnect by itself;
  a test that wants one plants a `POST …/refresh`.
- The UI tests that show a budget tell it through `reads`
  (`helpers.py`'s `REPAIR_BUDGET`, `test_cubes.py`'s `reads.cube`),
  so the fake's numbers do not reach them; the block checks it.
- The structure checks' reconnect (12 min after a change)
  and the 6-hourly refresh stay possible, and fall outside one test's time.

### For Blocks 2 to 4: the told data taken back

A test that tells a device's data through `reads` loses it at the server's
next event of that device, which carries the server's own view:

- **The marks:** the reconnect's `config` burst took back the told eco value,
  and `mark_of` counted the marks twice across the redraw.
  Block 1 removes the reconnect,
  but a follow-up's lone `config` event can do the same,
  so the count and the wait stay worth fixing.
- **The batteries:** a lone `state` event after an earlier write
  dropped the button for good, since no read of the rooms follows it.

`reads.keep(rf)` reads the rooms again after each server `state` or `config`
event of `rf`.
It works on the page kit's wrapper of the EventSource (`PAGE_KIT`),
which tells a server event from a test's dispatched one by `isTrusted`,
so a test's own events start no read,
and `test_room_marks.py`'s check that no rooms are read without `changed`
still holds.
It follows a reopened stream too, and `reads.clear` forgets it.

### For Block 5: the logs

`run.sh` keeps each set's `serve.log` and `fakecube.log` in a `mktemp -d`
directory that its cleanup removes.
CI uploads `tools/ui-test/results/` of a failed case,
which held the screenshots alone,
so neither session could see which reconnect it was.
The logs hold only the suite's test data,
and the server redacts keys and tokens in its log.

### For Block 6: the route race

`test_a_reboot_event_while_this_pages_reboot_waits` failed with
"Route.fulfill: Route is already handled!" in CI run 45517834691 (390-en),
and once in the investigator's local run.
The race is inside the test or its routes; the block finds it first.

## Implementation Plan

Each block is a commit, or two where it says so, on `claude/room-marks-race`.
Locally only the touched packages and test files run; CI runs the rest.
Every run takes `MASSIMO_ENV_FILE=/dev/null`;
the UI runs take the main checkout's venv
(`UI_TEST_PYTHON=/home/mdr/projects/massimo/tools/ui-test/.venv/bin/python`)
and ports of their own, checked free with `ss -ltnu`.
Each fix's proof is cited in the pull request.

### Block 1: FreeSlots

- **Goal:** the UI suite's server makes no budget reconnect by itself.
- **Changes:**
  1. `cmd/massimo/fakecube.go`: `-free-slots N`,
     which calls `SetFreeSlots(n)` where given;
     without it the fake says what it says today.
  2. `tools/ui-test/run.sh`: the fake Cube with `-free-slots 50`,
     and a comment on why.
- **Tests:** the flag's test where `cmd/massimo` tests the fake Cube;
  a scratch UI test that writes, waits 2:10 and counts the server's
  `connected` events, one with the old `run.sh` and none with the new;
  `test_shell_updates.py` in the four cases.
  The UI tests that read the budget are checked to take it from `reads`.
- **Documents:** `README.md`, the fake Cube's options;
  `docs/ui-testing.md`, that the suite's fake Cube has every slot free
  and that a test wanting a reconnect plants one.
- **CHANGELOG:** a fragment, `CHANGELOG.d/Added/claude-room-marks-race.md`,
  since `cmd/` is shipped.
- **Commits:**
  `feat(fakecube): The fake Cube reports the free memory slots it is given`,
  then `test(ui-test): The UI suite's fake Cube has every memory slot free`.
- **Verification:** `make ui`; `go test ./cmd/massimo ./internal/cubelan/fakecube`;
  the scratch test old and new; `gofmt -l .`;
  `go vet` for linux, windows and darwin; `GUIDELINES/shared/tools/check`.

### Block 2: Keep

- **Goal:** a test that tells a device through `reads` keeps what it told.
- **Changes:**
  1. `tools/ui-test/helpers.py`: `Reads.keep(rf)`,
     the rooms read again after each server `state` or `config` event of `rf`,
     on every stream of the page; `Reads.clear` forgets it.
  2. `tools/ui-test/test_confirm.py`: `READ_AFTER_ITS_EVENTS` replaced by it.
- **Tests:** `test_confirm.py` in the four cases.
- **Documents:** `docs/ui-testing.md`, "The page's own reads and its stream":
  what the reads keep, and when a test takes `keep`.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): A told device is read again after the server's own events of it`
- **Verification:** the four cases; `GUIDELINES/shared/tools/check`.

### Block 3: Marks

- **Goal:** `test_a_config_event_brings_new_presets` holds through a redraw.
- **Changes:** `tools/ui-test/test_room_marks.py`:
  1. `mark_of` reads the marks in one `evaluate`;
  2. the last check waits for the row's eco mark,
     its Set cell found by its label at each look;
  3. the told thermostat is kept (`reads.keep`).
- **Tests:** the old test against the new with the planted reconnect
  and the rooms reads held (`PLANT=burst`), three runs each, 390-en;
  `test_room_marks.py` in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The room marks test reads a mark in one look and waits for the eco mark`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 4: Batteries

- **Goal:** `test_new_batteries_sent` keeps its told device.
- **Changes:** `tools/ui-test/test_new_batteries.py`: `Device.told` keeps the
  device (`reads.keep`).
- **Tests:** the old test against the new with the planted write
  (`PLANT=write`), three runs each, 390-de;
  `test_new_batteries.py` in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The new batteries test keeps its told device`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 5: Logs

- **Goal:** a failed UI run says what the servers did.
- **Changes:** `tools/ui-test/run.sh`: where the tests failed,
  each set's `serve.log` and `fakecube.log` are copied into
  `results/logs/set<n>/` before the cleanup.
  CI's upload already takes `results/`; `.github/` is not touched.
- **Tests:** a run with a failing scratch test leaves the logs;
  a passing run leaves none.
- **Documents:** `tools/ui-test/README.md`, what `results/` holds.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): A failed run keeps the servers' logs in its results`
- **Verification:** the two runs; shellcheck of `run.sh` where installed;
  `GUIDELINES/shared/tools/check`.

### Block 6: RebootRoute

- **Goal:** `test_a_reboot_event_while_this_pages_reboot_waits` fulfils
  each route once.
- **Changes:** `tools/ui-test/test_maint_updates.py`, or the helper whose
  route it is, at the race found.
- **Tests:** a delay planted where the race is fails the old test
  and passes the new one; `test_maint_updates.py` in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): …`, named once the cause is known.
- **Verification:** the planted runs and the four cases by their exit codes.

### Before the pull request, and after Block 6

On the committed tree: the whole UI suite once in one case
(`PYTEST_ADDOPTS='-k 1100-en'`), since Block 1 changes every test's fake Cube;
`GUIDELINES/shared/tools/check`.
Then CI is watched, and nothing is merged or retired before the owner's word.

### At the retirement

`docs/roadmap.md`, item 12, "Small ones": the helper's entry is pruned.

## Proposed commit message

The plan: `chore(aps): +ServerTraffic v1.0, +ANALYSIS ServerTraffic`

Block 1, its first commit:

```
feat(fakecube): The fake Cube reports the free memory slots it is given

For AP ServerTraffic v1.0, Block 1 (D1-A): massimo fakecube -free-slots N
makes the fake Cube's S replies and its greeting's H line say N free
memory slots. Without it, the fake says 0x31, one radiogram pending,
which makes a server reconnect two minutes after every write; the UI
suite takes every slot free in the next commit.

Key changes:
- cmd/massimo/fakecube.go: -free-slots, through SetFreeSlots.
- README.md: the option.
- CHANGELOG.d/Added/claude-room-marks-race.md: the option.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `PLAN++ both, D1-A`
  on the TL;DR gate of 2026-11-25: D1-A to D5-A.
