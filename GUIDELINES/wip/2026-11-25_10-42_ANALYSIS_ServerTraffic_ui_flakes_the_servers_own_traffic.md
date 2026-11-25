# ANALYSIS ServerTraffic: UI flakes from the server's own traffic

## TL;DR

Two UI flakes on main come from the server's own traffic: a Cube reconnect it makes by itself mid-test.
A third, test_new_batteries_sent, comes from a lone server event of the device.
Planted, that traffic reproduces all three failures; the fixes are in the tests and in one shared helper.

- `test_pages_read_at_each_showing_what_they_read` → a Cube reconnect in mid-walk → the page's full load counted as the page's reads
- `test_a_config_event_brings_new_presets` → the reconnect's `config` burst takes back the told eco data → the mark gone until the rooms are read
- `test_new_batteries_sent` → a lone server `state` event of the device → its button gone for good
- helper `reads.keep(rf)` → the rooms read again after each server `state` or `config` event of `rf`
- CI → the servers' logs with a failed case's results, so the next reconnect names its source

## 1. The walk's extra reads (test_shell_updates.py)

**CI:** run 45583045225, the UI test (1100-de) job, attempt 1.
The maint showing read `cubes 2, cubes/KEQ0000100/rooms 3, cubes/KEQ0000100/devices 3, log/alerts 1` on top of what READS expects.
The test passed on the job's own retry.
The other session says #430's run 45564396008 shows the same set at the "me" showing, and so does `claude/auto-update`'s run 45577068673.

**Reproduced:** a `POST /api/v1/cubes/KEQ0000100/refresh` planted at the walk's first maint showing, 1100-de, the test as on main.
It failed 2 of 2, each time with exactly CI's extra reads; unplanted, it passed.
The reads come from the reconnect's burst as the page handles it (internal/server/ui/app.js):

- `metadata` → `loadRooms`;
- `config` events, one per device, with the server's view of the device;
- `connected` → `refresh()` → `loadCubes`, `loadRooms`, `loadAlerts`.

**Where a reconnect comes from by itself.** I know of two sources, and CI cannot say which it was: the UI job uploads `tools/ui-test/results/` alone, not `serve.log`.

- **The budget refresh after writes** (internal/server/followup.go):
  - at least 2 min after the last radio write, at most every 15 min;
  - only where the Cube last said radiograms are pending;
  - only with the periodic refresh on, which the UI tests' serve has by default (tools/ui-test/run.sh passes no `-refresh`).
  - Any earlier test's write starts one.
- **The structure checks' last reconnect** (internal/server/structure.go): after a removal, placement or move.

**The settings watch is unlikely.** It came with #425, and the session "AP TestInfrastructure" suspects it, since the failure is new since #425.

- `tools/ui-test/run.sh` starts serve under `env -i` without `MASSIMO_ENV_FILE`, so its settings file is `./.env` in the set's directory.
- As far as I see, no test writes that file.
- `reload()` (cmd/massimo/reload.go) does not reconnect a Cube.

That session's sub-agent is measuring the rates before and after #425.

**What the reads cannot be.** The helper of D3 below does not exist yet, so it cannot be the cause; and this test tells nothing through `reads`.

## 2. The vanishing mark (test_room_marks.py)

**CI:** run 45581057290, the UI test (390-en) job.
The failure was "one mark", `assert 0 == 1`, at test_room_marks.py:184, called from :591.

**What happened:**

- `mark_of` counts the marks twice:
  - `if marks.count() == 0: return None`;
  - `assert marks.count() == 1`.
- The first count found the mark, and the second found none.
- The reconnect's burst brings a `config` event of the thermostat with the server's own eco (16), not the test's told 18.
- `updateDevice` draws that, and the eco mark goes until the next rooms read brings the told data back.
- That read comes from `metadata` or `connected`.

**Reproduced:** the same reconnect, planted after "the rooms were read", with the page's rooms reads held 1.5 s.
The old test failed 3 of 3: `mark_of` found no mark at all, and the check then raised a TypeError on None.
With nothing held, the burst's own `metadata` read brings the mark back within milliseconds, which is the window CI's two counts fell into.

## 3. The batteries button (test_new_batteries.py)

**CI:** #422's run 45573284085, 390-de.
At line 190, after `dev.told("weak", False)`, the button was found, visible and stable, then detached, and no button came back within the click's 5 s.

**Cause:** a lone server `state` (or `config`) event of the device.

- Such an event comes from the server's poll after an earlier test's write, or from a follow-up re-read of a C line.
- It carries the server's battery, ok, so the page drops the button.
- No read of the rooms follows it, so nothing brings the told "weak" back.
- Unlike the reconnect, a lone event starts no read.

**Reproduced:** a planted `PUT …/devices/{rf}/setpoint` with `{"mode": "auto"}` before the second click, 390-de.
It brought the server's `change` and two `state` events of the device, and no read.
The old test's click timed out, 2 of 2, as CI's did; unplanted, it passed.

## 4. The helper the roadmap names

docs/roadmap.md, item 12: "a helper of tools/ui-test/ for a test that tells a device's data through `reads`, reading the rooms again after the device's own `state` and `config` events".

**Today:** `test_confirm.py` has a local version, `READ_AFTER_ITS_EVENTS` (AP Flakes, Block 4). It reacts to every event of the device, the test's own included.

**Proposed:** `reads.keep(rf)` on `helpers.Reads`.

- It works on the page kit's wrapper of the EventSource (`PAGE_KIT`, helpers.py).
- That wrapper already knows a server event from a dispatched one, by `isTrusted`.
- After each server `state` or `config` event of `rf`, it calls `loadRooms()` on every stream of the page, a reopened one too.
- The test's own dispatched events start no read, so `test_room_marks`' check "no read of the rooms without changed" still holds.
- `reads.clear` forgets it.

**Users:**

- `test_confirm.py` (its local copy goes);
- `test_room_marks.py`'s told thermostat;
- `test_new_batteries.py`'s `Device.told`.

docs/ui-testing.md, "What the reads do not keep yet", then says what the helper keeps.

**Not for the shell walk.** A reconnect reads the rooms anyway, and the walk counts reads: there the fix is to measure again.

## 5. Fix options

**The walk:** measure it again where a server link event came during it.

- A link event is `disconnected`, `hello` or `connected` from the server; the page kit's events show them.
- The walk is measured again, up to three walks.
- A walk whose reads differ without such an event still fails at once, so the test stays strict.
- The other way, a test-only serve option without budget reconnects, is a product change, and it would hide the reconnect from every other UI test.

**The marks:**

- `mark_of` reads the marks in one `evaluate`, so it can no longer count twice.
- The last check waits for the row's eco mark, with the Set cell found by its label at each look, not by an index taken earlier.

**The batteries button:** `Device.told` keeps the device (`reads.keep`).

**CI:** the UI job copies each set's `serve.log` and `fakecube.log` into `tools/ui-test/results/` on a failure (run.sh's cleanup), so the next such failure names its reconnect.

**Proof for each:**

- old against new with the planted reconnect, or, for the batteries button, with the planted write;
- the touched files in the four cases.
