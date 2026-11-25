# AP ToldData v1.0: Tests tell their data through reads

## TL;DR

UI tests that lose what they wrote into the page alone, and four flakes of the
server's own traffic or the test kit, fixed in the tests and in the fake Cube,
a commit each, one pull request
(`wip/2026-11-25_14-36_ANALYSIS_ToldData_told_state_and_open_flakes.md`).

- 20 tests at risk, 4 that keep nothing → told through the rooms' read, kept,
  their checks waiting
- tests 4, 5, 6 of the "UI refactor" list → fixed among them
- test 8 → the fake Cube keeps a paired device and a written room list
- folds → helpers in `helpers.py`; 2 places in `test_password_eye.py`
- restore test on armv6 → waits for its log line
- settings test, "one read after it (2)" → waits for the server's own events
- times-ago test, "'4 sec. ago'" → its cause found, then fixed

### Blocks

1. Helpers: `Reads.device`, `Reads.room`, the fold helpers; docs/ui-testing.md
2. RoomMore: `test_room_more.py` and `test_battery_words.py`'s list (tests 4, 5)
3. RoomMarks: `test_room_marks.py`, the 7 at risk and the 7 unclear (test 6)
4. Rooms: `test_rooms.py`, `test_icon_choice.py`, `test_problem_list.py`, `test_repair.py`
5. Folds: `test_password_eye.py`'s two fold places
6. FakePairing: the fake Cube keeps a paired device and a written room list (test 8)
7. RestoreLine: `TestRestoreFailsWhenTheCubeDiscards` waits for its log line
8. SettingsEvents: the settings test waits for the server's own change events
9. TimesAgo: `test_reads_times_ago`, its cause found and fixed

## Discussion

### What is decided

The owner, 2026-11-25, `PLAN++ D9a` on the TL;DR gate of the same day,
every ToldData decision not answered taking its recommended option:

- **D1-A.** Every test at risk, the 4 that keep nothing, and the 7 unclear.
- **D2-A.** One shared `Reads.device(rf, …)` and `Reads.room(rid, …)`.
- **D3-A.** The fake Cube keeps the joined device and an `m:` across
  reconnects, and the test names a reconnect in its failure.
- **D4-A.** The fold helpers move into `helpers.py`, with `close_fold`,
  and `test_password_eye.py`'s two places take them.
- **D5-A.** The restore test is Block 7.
- **D6-A.** One pull request, a commit per block, each fix proven old against
  new with a planted event or read; the touched files in the four cases;
  the whole suite is CI's.
- **D7-A.** The settings test and the times-ago test are Blocks 8 and 9.

Outside the plan, from the same gate:

- **D9-A.** The credits' lasting fix (`internal/credits/credits.json` records
  each module's version, so every `go.mod` bump fails
  `TestGeneratedFileCurrent`) goes to AP PasswordRules' session as a proposal.
- **D8** was not signalled: #441, Dependabot's bump of `golang.org/x/net`,
  is left as it is.

### Decided in the writing

- **Two builders:** the UI blocks (1 to 5, 8, 9) in this session's worktree,
  the Go blocks (6, 7) in a worktree of their own,
  their commits taken onto the plan's branch at the end;
  the commits' order may differ from the blocks' numbers.
- **The branch** is `claude/told-data`.

## Analysis

The causes, the reproductions and the survey are in
`wip/2026-11-25_14-36_ANALYSIS_ToldData_told_state_and_open_flakes.md`.
What it does not hold, and what each block rests on:

### For Blocks 1 to 4: the told data

After #439 the server makes no reconnect by itself,
but an earlier test's write still brings the server's poll's `state` event of
that device some seconds later, into the next file's first tests,
and no read of the rooms follows it.
Data written into the page alone (`updateDevice`, a field of `state`,
a dispatched event) goes at that event or at the page's next read.
Telling it through the rooms' read and keeping the device
(`Reads.keep`, #439) brings it back after both;
a check after it waits, since the told data come back a moment later.

### For Block 6: the paired device

The fake Cube's greeting never carries the device that joins with `-pair`,
and it answers an `m:` with `A:` and keeps nothing,
so a reconnect forgets the device and its room.
A real Cube lists a paired device that is in no room.

### For Block 7: the restore's end

The test waits until the restore's state is no longer "running" and reads the
log once; on QEMU the pump had not yet written `restore_done`
(main's CI run 45617523557, Test (armv6, QEMU)).

### For Block 8: the settings test's second read

`test_maint_updates.py::test_another_setting_changed_during_the_own_put_is_read_after_it`
failed in CI run 45620724300, 1100-de: "one read after it (2)".
The server's log of that set shows its own `change` event for each of the
three writes within 430 ms: the test's reset, the "other browser's"
`announce`, and the page's own `poll_interval`.
The test dispatches its own copy of the second beside it,
so the page can read twice, as the server's events fall.

### For Block 9: the times-ago test

`test_header.py::test_reads_times_ago` failed in CI run 45622423266, 390-en:
"5 s ago: '4 sec. ago'".
`SET_READ` sets the server's clock and the Cube's time, then reads the drawing
once; the page moves its clock only by more than 1.5 s,
so that is unlikely the cause. The block finds it first.
It differs from the earlier "125 s ago: '5 sec. ago'",
which the old reconnect most likely made.

## Implementation Plan

Every commit is on `claude/told-data`, one pull request.
Locally only the touched packages and test files run; CI runs the rest.
Every run takes `MASSIMO_ENV_FILE=/dev/null`;
the UI runs take the main checkout's venv and ports of their own,
checked free with `ss -ltnu`.
Each fix's proof is cited in the pull request.

### Block 1: Helpers

- **Goal:** one way to tell a device or a room, and to open a fold.
- **Changes:** `tools/ui-test/helpers.py`:
  1. `Reads.device(rf, **fields)`: the device's fields, its `state` among them,
     in the rooms' read from now on, the device kept;
  2. `Reads.room(rid, **fields)`: a room entry's fields, its presets and its
     virtual wall thermostat's sources, the room's thermostats kept;
  3. `FOLD_TOGGLE` with a timeout, `toggle_fold`, `open_fold` and `close_fold`,
     taken from `test_password_suggest.py`, which imports them.
- **Tests:** `test_password_suggest.py` in the four cases;
  the helpers' first users prove them in Block 2.
- **Documents:** `docs/ui-testing.md`, section 9: what `device` and `room` tell
  and keep, and the folds' helpers.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commits:** `test(ui-test): A test tells a device or a room through the rooms' read`,
  then `test(ui-test): The fold helpers serve every UI test`.
- **Verification:** the four cases; `GUIDELINES/shared/tools/check`.

### Block 2: RoomMore

- **Goal:** the holiday, the modes and the battery words hold through an event.
- **Changes:** `tools/ui-test/test_room_more.py`'s three tests at risk,
  told through `Reads.device`;
  `tools/ui-test/test_battery_words.py`'s `assert_badge` reopens the list
  until its badge shows, and measures it in the same look.
- **Tests:** test 5 and test 4's list with a planted `state` event after the
  click, old against new, three runs each; both files in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The room's more tests and the battery words tell their device through the read`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 3: RoomMarks

- **Goal:** `test_room_marks.py` holds through an event and a rooms read.
- **Changes:** `tools/ui-test/test_room_marks.py`:
  1. the 7 at risk told through `Reads.device` and `Reads.room`;
  2. the two that count rooms reads leave keep's reads out of their count;
  3. the five whose in-page state is the event under test get it in the page
     and in the read, after a look at the tile's wait (`settleAwaiting`).
- **Tests:** test 6 with a planted reconnect and a planted read, old against
  new, three runs each; a sample of the others with a planted event;
  the file in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The room marks tests tell the room and its devices through the read`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 4: Rooms

- **Goal:** the rooms', icons', problem list's and repair's tests keep their data.
- **Changes:** `tools/ui-test/test_rooms.py` (8 tests),
  `test_icon_choice.py` (3), `test_problem_list.py::test_one_entry_format`
  (its Cubes through `reads.change` and `loadCubes()`),
  `test_repair.py` (2), each as the analysis's survey says.
- **Tests:** one test of each file with a planted event, old against new,
  three runs each; the four files in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The rooms' tests tell their data through the read`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 5: Folds

- **Goal:** `test_my_profiles_password_eyes` presses inside an open fold only.
- **Changes:** `tools/ui-test/test_password_eye.py`: `open_fold` and
  `close_fold` in place of the two clicks.
- **Tests:** a planted late `toggle` fails the old test and passes the new one;
  the file in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The password eyes' test waits for its fold`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 6: FakePairing

- **Goal:** a reconnect keeps a paired device and a written room list.
- **Changes:**
  1. `internal/cubelan/fakecube`: the joined device's C line and L entry stay
     in the greeting until a `t:` deletes it;
     an acknowledged `m:` replaces the greeting's M lines;
  2. `tools/ui-test/test_pairing.py`: a `cube reconnecting` line of the
     server's log named in the failure message.
- **Tests:** a fakecube test: pair, reconnect, the greeting holds the device;
  `m:`, reconnect, the greeting holds the new list; it fails on main.
  `test_pair_put_into_a_room_and_remove` with a refresh planted before the
  submit, old against new; `test_pairing.py` in the four cases.
- **Documents:** the fake Cube's lines in `README.md` where they describe it.
- **CHANGELOG:** a fragment under `CHANGELOG.d/`, since `internal/` is shipped.
- **Commit:** `fix(fakecube): The fake Cube keeps a paired device and a written room list`
- **Verification:** `go test ./internal/cubelan/fakecube ./cmd/massimo`;
  the planted runs; `gofmt -l .`; `go vet` for linux, windows and darwin.

### Block 7: RestoreLine

- **Goal:** `TestRestoreFailsWhenTheCubeDiscards` holds on a slow machine.
- **Changes:** `internal/server/restore_api_test.go`: the test waits for the
  `restore_done` line (`pumpSync`, or a wait for the kind).
- **Tests:** a delay planted in the pump (an overlay) fails the old test
  and passes the new one.
- **CHANGELOG:** none: only a test changes.
- **Commit:** `test(server): The restore test waits for its end in the log`
- **Verification:** the overlay runs; `go test ./internal/server -run TestRestore`;
  `gofmt -l .`; `go vet` for linux, windows and darwin.

### Block 8: SettingsEvents

- **Goal:** the settings test counts its reads after the server's own events.
- **Changes:** `tools/ui-test/test_maint_updates.py`: the test waits for the
  server's own `change` events of its writes instead of dispatching a copy,
  and counts after them; any test of the same pattern in the file likewise.
- **Tests:** a planted late server event fails the old test and passes the
  new one; the file in the four cases.
- **CHANGELOG:** none.
- **Commit:** `test(ui-test): The settings test counts its reads after the server's own events`
- **Verification:** the planted runs and the four cases by their exit codes.

### Block 9: TimesAgo

- **Goal:** `test_reads_times_ago` holds.
- **Changes:** `tools/ui-test/test_header.py`, or the page where the cause is,
  once found.
  Where the cause is not found with reasonable effort, the block stops and the
  run says what was seen.
- **Tests:** the cause planted fails the old test and passes the new one;
  the file in the four cases.
- **CHANGELOG:** none where only a test changes.
- **Commit:** named once the cause is known.
- **Verification:** the planted runs and the four cases by their exit codes.

### Before the pull request

The touched files in the four cases and the touched Go packages, each by its
exit code; `GUIDELINES/shared/tools/check` on the committed tree.
Then CI is watched, and nothing is merged or retired before the owner's word.

## Proposed commit message

The plan: `chore(aps): +ToldData v1.0, +ANALYSIS ToldData`

Block 1, its first commit:

```
test(ui-test): A test tells a device or a room through the rooms' read

For AP ToldData v1.0, Block 1 (D2-A): Reads.device and Reads.room put
a device's or a room's fields into the rooms' read and keep them, so
that what a test tells comes back after the server's own events of it
and after the page's next read.

Key changes:
- tools/ui-test/helpers.py: Reads.device, Reads.room.
- docs/ui-testing.md: section 9, what they tell and keep.
- No CHANGELOG bullet: tools/ is not shipped.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `PLAN++ D9a`
  on the TL;DR gate of 2026-11-25: D1-A to D7-A.
