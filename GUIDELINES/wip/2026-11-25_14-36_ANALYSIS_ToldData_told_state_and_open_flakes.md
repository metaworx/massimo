# ANALYSIS ToldData: Tests that tell the page their data, and the open flakes

2026-11-25, session "Fix the racy mark_of in test_room_marks.py", on the owner's
`PLAN A - also see if there are other tests that should use reads`.
The list of unsteady tests came from the session "UI refactor".
Two investigators reproduced the failures with planted traffic;
nothing tracked was written.

## TL;DR

The page takes back data a test wrote into it alone.
#439 removed the server's budget reconnect,
but one lone device event per earlier write is left,
and 20 more tests are exposed to it the way tests 4 to 6 were.

- test 4, battery words → fixed by #439's `keep`;
  its list still closes at an event after the click
- test 5, holiday end → `updateDevice` alone → lost at the lone event;
  not covered by #439
- test 6, wall thermostat presets → lost at a rooms read
  (the old reconnect's); #439 removes the source, not the exposure
- survey of 97 tests → 20 at risk, 70 safe, 7 unclear;
  4 more tell through `reads` but keep nothing
- test 8, pairing `KeyError: 'copied'` → a reconnect forgets a paired device
  the fake Cube never repeats → a gap in the fake
- folds → helpers into `helpers.py`; 2 places at risk, 188 safe
- `TestRestoreFailsWhenTheCubeDiscards` (armv6, main) → reads the log
  before the pump wrote the restore's end

## The server's own traffic after #439

The servers' logs of CI run 45620724300 (1100-de), the first kept by #439:

- no budget refresh, no reconnect and no unconfirmed write came by itself;
- one lone device event is left: a room-1 write `{"mode": "auto"}`
  (`test_log.py::test_log_command_detail`, the file's last test)
  brings the server's poll's `state` of radiator 0a010a 6 to 7 s later,
  which falls into the next file's first tests on that worker,
  and the room's virtual wall thermostat's event with it;
- no read of the rooms follows that event,
  so `keep` is what brings a told device back;
- the stream's own connect burst ended at least 20 ms before each test's
  first look, over 60 fixtures: it is not a cause.

## Tests 4 to 6

All three put a device's or a room's data into the page,
and the server's own traffic took it away before the test acted.
Two kinds of that traffic matter here.

- **A lone `state` event of Wohnzimmer's radiator (0a010a).**
  Some tests let a write to room 1 reach the fake Cube.
  `test_log_command_detail`, the last test of `test_log.py`,
  sends `PUT …/rooms/1/setpoint {"mode": "auto"}`.
  The fake applies Auto as the week programme gives it at that minute,
  so the setpoint changes unless the programme says 19 then.
  The server's next poll then sends a `state` event of 0a010a,
  and with it one of the room's virtual wall thermostat.
  After `test_log_command_detail` that event comes
  in the first tests of the next file its worker runs.
  It carries the server's view of the device,
  and nothing reads the rooms after it.
  This is still there after #439.
  CI run 45620724300 (1100-de, after #439) shows it in sets 2 and 3:
  `device state … Radiator Wohnzimmer … setpoint=12 changed=setpoint`,
  6 to 7 s after a room-1 write `set=map[mode:auto]`.
  The same logs have no `unconfirmed`, no budget refresh and no reconnect.
- **The budget reconnect, until #439.**
  It came two minutes after a write while the fake said one radiogram pending.
  Its burst was `metadata`, then a `config` event per device, then `connected`.
  `metadata` and `connected` read the rooms again.
  All three failure screenshots show "1 pending, states may still change".
  The two of 390-en also show Wohnzimmer at SET 12.0°,
  so a room-1 write had reached that worker's fake Cube earlier.

Ruled out: the stream's connect burst arriving after the page fixture.
In 60 page fixtures (390-en, locally) the burst of `state` events
took at most 73 ms, and it ended at least 20 ms before the test's first look.
No burst event came after the page was handed over.

The plants are scratch copies of the tests with one call put in,
run alone in one case, three times each unless said otherwise (`PLANT=…`):

- `write` writes the device (`PUT …/devices/0a010a/setpoint {"mode": "auto"}`)
  and goes on once the server's `state` event of it has come;
- `state` does the same,
  and holds the page's rooms reads for 1.5 s after the event;
- `plain` is a reconnect (`POST …/refresh`), the budget reconnect's burst;
- `burst` is that reconnect with the page's rooms reads held 1.5 s;
- `read` is a read of the rooms by the page itself,
  started by a dispatched `unconfirmed` event, as `metadata`,
  `unconfirmed` or `mismatch` start one.

### Test 4: `test_battery_words_in_the_problem_lists[1100-en]`

CI run 45519040230, job 138886564697:
the tile's `.problems-sum` was "not stable", then "detached".

- **Cause:** the test tells the level through `reads` (`Device.told`).
  At that commit `Device` did not keep the device.
  A lone `state` or `config` event of 0a010a drew the server's battery, ok.
  The tile's only problem went, and with it its `details.problems`.
  Nothing read the rooms again,
  since the poll does not read them while the stream is open,
  so the click timed out.
  The budget reconnect cannot have done it:
  its `metadata` and `connected` reads bring the told level back.
  The screenshot does not name the write:
  its Wohnzimmer is at SET 19.0°, the setpoint the fake starts with.
- **Evidence:** plants before the tile's list.
  - The test with that commit's `Device`, `state`:
    failed 3 of 3, its click timing out as CI's did.
  - The same, `plain` and `burst` (a reconnect, the reads held 1.5 s):
    passed 2 of 2 each.
  - The test as on main, `state`: passed 3 of 3.
- **#439:** covers the cause.
  `Device.told` keeps the device (Block 4).
- **What is left:** `assert_badge` clicks once and then waits for the badge.
  An event after the click still draws the server's level,
  and keep's read draws the told one again in a new list, which is closed.
  A `state` plant right after the click failed the test as on main
  3 of 3, at `flag.wait_for`.
- **Fix:** `assert_badge` opens the list again until the badge shows,
  and measures it in the same look (`FIT` also says whether the list is open).
  A copy with this fix passed that plant 3 of 3.

### Test 5: `test_a_holiday_shows_its_end[390-en]`

CI run 45529542678, job 138921209593:
the tile's `.problems-sum` was detached during `summary.click()`.

- **Cause:** `SET_MODE` puts the holiday into the page alone,
  by `updateDevice`, on room 1's thermostat 0a010a.
  Either of the server's own traffic takes it back:
  - the lone `state` event of 0a010a;
  - before #439, the reconnect: its `config` events and its reads.
  The holiday pill is the tile's only problem, so its list goes.
  Nothing tells the holiday again.
  The failure screenshot shows Wohnzimmer in AUTO, without a holiday.
- **Evidence:** plants before the click.
  - The test as on main, `write`: failed 3 of 3 (`Locator.click: Timeout`).
  - The same, `plain` (the reconnect): failed 2 of 2, the same way.
  - A copy told through `reads`, below: passed 3 of 3 with `write`,
    and 3 of 3 with `state`, both planted right after the click.
- **#439:** does not cover it.
  It removes the reconnect, but the lone `state` event is still there.
- **Fix:**
  - The room's modes go into the rooms' read: a helper of the file that
    sets each thermostat's `state.mode` and `until`,
    and adds the twin to the room's devices where it is told.
  - It keeps the thermostats and the room's virtual wall thermostat,
    and reads the rooms at once.
  - Each check waits: More's text and class,
    and the pill's suitcase and label, in one look.
  - The list is opened again until each thermostat's flag shows.
  - The same helper serves `test_eco_and_comfort_keep_the_mode`
    and `test_a_holiday_keeps_the_tools_up_and_the_values_at_the_edge`.

### Test 6: `test_the_rooms_presets_over_a_wall_thermostats[390-en]`

PR #434, job 139139737698: the room's comfort was sent as 22, not 21.

- **Cause:** the test puts four things into the page alone:
  - the wall thermostat (`WALL`);
  - the thermostat's setpoint and presets (`SET`);
  - the room's eco and comfort (`PRESETS`, `r.eco_temp`, `r.comfort_temp`);
  - the virtual wall thermostat's `sources.setpoint`.
  More sends the room's comfort, its entry's `comfort_temp`.
  Only a read of the rooms replaces the entry,
  with the server's eco 15 and comfort 22 for room 1.
  A device's `state` event leaves the entry as it was.
  Before #439 the budget reconnect's `metadata` and `connected` read the rooms,
  two minutes after a write; the screenshot's SET 12.0° shows
  that a room-1 write had reached that worker's fake Cube.
  So this failure is most likely the reconnect's, not the lone event's.
  The More menu stays open through the read (one element per key),
  and its Comfort then holds 22.
- **Evidence:** plants before the Comfort click.
  - The test as on main:
    - `plain` (the reconnect): failed 3 of 3,
      "the room's comfort sent: [{'mode': 'auto', 'temp': 22}]", as in CI;
    - `read` (a read of the page's own): failed 2 of 2, the same way;
    - `write` (a lone `state` event): passed 2 of 2;
    - unplanted: passed 1 of 1.
  - A copy told through `reads`, below:
    passed 3 of 3 with `plain`, 2 of 2 with `read`, 2 of 2 with `write`.
- **#439:** removes this failure's source, the budget reconnect.
  The test still loses its data at any read of the rooms by the page:
  `unconfirmed`, `mismatch`, `metadata`, a reconnect.
  Its SET and members' line checks also lose theirs
  at the lone `state` events of 0a010a
  and of the room's virtual wall thermostat.
- **Fix:**
  - The rooms' read carries all four (one change function):
    the wall thermostat among the room's devices;
    `thermostat` naming it;
    the virtual wall thermostat's `sources.setpoint`;
    the thermostat's setpoint and presets;
    the room's `eco_temp` and `comfort_temp`.
  - The room's thermostat and its virtual wall thermostat are kept.
  - The wall thermostat's 16 is told and read again.
  - The checks wait: SET 16 with no mark in one look,
    and More's two temperatures in one look.

## Tests that should tell through reads

The grep took every `test_*.py` for these:

- `updateDevice(`;
- an assignment to a `state.` field, a room's field or a device's field
  in `page.evaluate`;
- a dispatched `state`, `config` or `metadata` event;
- `STATE_EVENTS_JS` and `state_events`.

That found 97 tests, sorted by what their data must outlast
and whether they act on it after that can come.
Since #439 the only device data the suite's server takes back by itself
are those of Wohnzimmer's radiator and room 1's virtual wall thermostat:
the lone `state` events above.
Anything else is taken back only by a read of the page's own.
In run 45620724300 the server started no such read by itself.
The (a) list says which applies.

| | Tests |
|---|---|
| (a) at risk | 20 |
| (b) safe | 70 |
| (c) unclear | 7 |
| All | 97 |

Beside them, four tests already tell room 1 through `reads` but keep nothing.
The lone `state` event takes back what they told (below).

### (a) At risk, with the fix

Room 1's radiator or virtual wall thermostat, which the lone events reach:

- `test_room_more.py`:
  - `test_eco_and_comfort_keep_the_mode`:
    Manual and the holiday through test 5's helper;
    wait for More's state before each send.
  - `test_a_holiday_shows_its_end`: test 5, above.
  - `test_a_holiday_keeps_the_tools_up_and_the_values_at_the_edge`:
    the holiday through test 5's helper;
    wait for "Holiday until" on More before measuring.
- `test_room_marks.py`:
  - `test_room_thermostat_follows_the_entry`:
    tell through the rooms' read `SET`, the twin, the entry's `thermostat`,
    `WALL` and the virtual wall thermostat's sources;
    keep the room's thermostats and virtual wall thermostat;
    `VWT_SETPOINT` stays a dispatched event.
  - `test_the_rooms_presets_over_a_wall_thermostats`: test 6, above.
  - `test_marks_and_off`: `SET` and `PRESETS` through the read, kept;
    read the marks after a wait for each case's SET and mark.
  - `test_more_shows_off_from_the_entry`: `SET` and `PRESETS` through the read,
    kept; wait for More's temperatures.
  - `test_a_boost_waits_for_nothing`:
    `SET` and the twin through the read, kept.
  - `test_a_pending_write_or_an_open_window_hides_the_line`:
    `SET`, the twin and `WINDOW` through the read, kept.
  - `test_shows_off_of_a_write_not_confirmed`:
    `SET` with its confirmation through the read, kept;
    open the list again until its flag shows.
- `test_rooms.py`:
  - `test_radio_fault_badges`: `RADIO_FAULTS` through the read, kept;
    `badges` opens the list again until the entry shows.
  - `test_problems_in_the_heads`: the error, the level and the modes through
    the read, kept; wait for the head's icons.
  - `test_battery_levels`: `test_new_batteries.py`'s `Device`, told and kept,
    with test 4's list fix.
  - `test_out_of_date_device`: `out_of_date` and `last_known` through the read,
    kept; wait for the badge.
  - `test_stale_and_expert`: already through `reads`;
    keep room 1's devices and its virtual wall thermostat.

Other devices, rooms' fields or the Cubes,
lost only at a read of the page's own:

- `test_rooms.py::test_open_window`: `WINDOW_TIMES` through the read,
  the contact kept; wait for the head's icons.
- `test_icon_choice.py`: `test_the_defaults`, `test_the_other_drawings`,
  `test_the_choice_in_my_settings`:
  `PROBLEMS` through the read, Kind 1's contact and thermostat kept;
  `check` waits for the head's icons.
- `test_problem_list.py::test_one_entry_format`:
  the three Cubes through `reads.change(CUBES, …)` and `loadCubes()`,
  not `state.cubes`, as `docs/ui-testing.md` section 9 asks;
  `held_poll` is then not needed.

Told through `reads` already, not kept, all on room 1:

- `test_rooms.py`: `test_not_ready_line` and `test_mismatch_marks`:
  keep the told device.
- `test_repair.py`: `test_listing_names_the_device` and
  `test_listing_without_a_room_name` (`rename_room_devices`):
  keep room 1's devices; wait for the name offered.

### (c) Unclear

- `test_room_marks.py`, five tests:
  `test_members_line`, `test_auto_into_auto_waits_for_the_setpoints`,
  `test_the_house_holds_the_line`, `test_a_write_not_taken_leaves_the_wait`,
  `test_members_line_after_the_deadline`.
  - Their in-page `SET` is the state event under test.
    `updateDevice` settles the tile's wait with `event` true;
    a read does it with `event` false,
    and never ends a wait for Auto (`settleAwaiting`, `rooms.js`).
  - Telling through `reads` alone would change what they test.
    `SET` into the page and into the read, with the devices kept,
    is likely right.
  - Whether keep's read in the middle of the tile's wait changes the outcome
    needs a look.
- `test_room_marks.py`, two tests:
  `test_a_config_event_brings_new_presets` (its first half) and
  `test_set_follows_the_virtual_wall_thermostats_events`.
  - They count the rooms' reads ("no read").
  - Keep's reads after a server event would be counted.
  - The count would have to leave them out (`window.__uiTest.kept`).

### (b) Safe

70 tests, each for one of these reasons:

- **They put the server's own data:**
  - `test_room_updates.py`, 19 (`state_event` applies the device as it is);
  - `test_room_more.py::test_the_menu_by_keyboard`
    (Auto, as the server has it);
  - `test_server_sign.py::test_server_gone`
    (the window open, as the server has it);
  - `test_data_updates.py`, `test_parts_guard.py` 2,
    `test_repair_updates.py`, `test_stream.py`:
    the events are triggers, with the server's data.
- **The test answers the reads itself:**
  `test_room_marks.py`'s `test_a_burst_of_reloads_reads_the_rooms_twice`
  and `test_a_cube_change_reads_its_rooms_at_once`.
- **They check at once, after the put or after the wait that shows it:**
  - `test_room_marks.py`: `test_the_setpoint_row_fits_in_every_language`,
    `test_a_thermostat_gained_or_lost_reads_the_rooms`;
  - `test_rooms.py::test_typing_in_the_configuration_survives_a_state_event`;
  - `test_log.py::test_log_names_room_and_device`.
- **The data are no rooms' or Cubes' data:**
  - settings, which the page reads only at its start and at a sign-in:
    `test_unsaved.py` 19, `test_header.py` 2, `test_scroll.py`,
    and `test_rooms.py`'s `test_eco_button` and `test_virtual_wall_thermostat`
    (one of `test_unsaved.py` also applies a virtual wall thermostat as it is,
    and one of `test_header.py` sets a Cube's `updated`
    and reads it in the same look);
  - `state.me`, read only at a sign-in:
    `test_admin_updates.py`, `test_discover.py`;
  - the log's lines: `test_log.py`'s other 11,
    `test_battery_words.py::test_battery_words_in_the_log`.

## Test 8

`test_pairing.py::test_pair_put_into_a_room_and_remove` failed twice,
each passing on its retry:

- #417, run 45492439943, 390-de: `KeyError: 'copied'`,
  the form showing "no device TEQ0000001";
- run 45518863169, 1100-de: "the device is in the room",
  after a PUT that did answer `copied`.

**Cause:** a reconnect of the server's own between the join and the PUT
(390-de), or right after it (1100-de), forgets the joined device.

- `'copied'` is read from the answer to the form's own
  `PUT …/devices/TEQ0000001`, against the real server;
  the answer without it was the 404 "no device TEQ0000001".
- The joined device lives only in the server's snapshot,
  its C line from the re-read that the pairing starts.
- The fake Cube's greeting never carries it:
  it adds no C line for the `-pair` device,
  and it answers an `m:` with `A:` and keeps nothing.
- At a new connection the snapshot drops every configuration the greeting
  does not repeat, so the device goes, and after the PUT its room too.
- A real Cube lists a paired device that is in no room
  (`docs/cube-protocol.md`): this is a gap of the fake.

**The trigger** was the budget reconnect that #439 removed,
two minutes after an earlier test's write in the same worker.

**Reproduced:**

- a refresh planted before the submit, 390-de: `KeyError: 'copied'`,
  the 404 and a screenshot as CI's;
- a refresh planted after the PUT, 1100-de: "the device is in the room";
- the fake before #439 with a setpoint written at the test's start:
  `cube reconnecting` 120 s after the write, 13 s after the pairing,
  and `KeyError: 'copied'`; the retry passed, held back by the 15 min gap;
- the same with #439's fake: no reconnect, and the test passed.

**Still possible:** the structure checks' reconnect, 12 min after a change,
or any other reconnect.

**Fix options:**

- **The fake keeps what a Cube keeps** (recommended):
  the joined device's C line and L entry stay in its greeting until a `t:`
  deletes it, and an acknowledged `m:` replaces the greeting's M lines;
  a Go test pairs, reconnects and finds the device,
  writes `m:`, reconnects and finds the new list.
  `internal/` is shipped, so it carries a CHANGELOG fragment.
- **Beside it:** the test names a `cube reconnecting` line of `serve_log()`
  in its failure message; it explains a failure, nothing more.
- **Not recommended:** `serve -refresh 0` in `run.sh`,
  which would change what the refresh tests test.

## The fold helpers

`test_password_suggest.py` has `toggle_fold` and `open_fold`,
which take a `<details>` fold's `toggle` event before a test acts inside it.

**Proposed** in `helpers.py`, beside `frames`:

- `FOLD_TOGGLE`, its script, with a timeout,
  so that a click that toggles nothing fails at once;
- `toggle_fold(page, sel, seconds=3)`, `open_fold` and a new `close_fold`,
  `sel` being the `<details>`;
- `test_password_suggest.py` imports them,
  and `test_password_eye.py`'s own `toggle()` can use `toggle_fold`.

**Which folds do something at their toggle:**
`#me-password` and `.user-panel` start their password fields anew,
which drops a shown password, a suggestion and a refusal mark;
the others read or move something the tests wait for, or nothing.

**The tests:** 190 places open a fold and then act inside it.

- **At risk, 2,** both in `test_password_eye.py::test_my_profiles_password_eyes`:
  its click on `#me-password-title` before `check_eye` presses the eye,
  and the close and reopen by clicks before the press at its line 214.
  Fix: `open_fold` and `close_fold` in place of the clicks.
- **Safe, 188:** 90 checked one by one
  (they type into controlled fields, wait for what the toggle reads,
  or act on what is drawn without it), 98 by their fold's kind.
- **Unclear:** none.

## TestRestoreFailsWhenTheCubeDiscards

Main's CI run 45617523557 failed in Test (armv6, QEMU) alone:
`restore_api_test.go:316`, "done event <nil>".
The test waits until the restore's state is no longer "running",
then reads the log once;
on QEMU the pump had not yet written the `restore_done` line,
which the server's log shows in that very moment.
The fix is in the test: wait for the line, as `docs/testing.md`,
"The pump", asks (`pumpSync`, or a wait for the kind).
Nothing merged today reaches it: a restore re-reads nothing.

## What a plan would do

- **A shared helper of the rooms' read,** in `helpers.py`:
  `Reads.device(rf, **fields)` puts a device's fields, its `state` among them,
  into the rooms' read and keeps it,
  and `Reads.room(rid, **fields)` does the same for a room's entry,
  its presets and its virtual wall thermostat's sources;
  the checks after it wait.
  The investigator's per-file helper for test 5 is its first user.
- **The 20 tests at risk, and the 4 that keep nothing,** told through it,
  file by file: `test_room_more.py` (3), `test_room_marks.py` (7),
  `test_rooms.py` (6), `test_icon_choice.py` (3), `test_problem_list.py` (1),
  `test_repair.py` (2), with `test_battery_words.py`'s list reopened (test 4).
- **The 7 unclear** of `test_room_marks.py`:
  the two that count rooms reads leave keep's reads out of their count;
  the five whose in-page state is the event under test get it in the page
  and in the read, after a look at the tile's wait.
- **The fold helpers** into `helpers.py`, and `test_password_eye.py`'s two places.
- **The fake Cube** keeps a paired device and a written room list (test 8).
- **The restore test** waits for its line.

Not in it: the header's times and the burst's third read
(tests 1, 2 and 7 of the list), which #439's Block 1 most likely ended;
CI on main shows whether they come back.
