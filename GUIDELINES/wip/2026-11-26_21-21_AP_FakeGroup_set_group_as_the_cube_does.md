# AP FakeGroup v1.0: Set group as the Cube does

## TL;DR

What the test Cube showed of set group (0x22), remove group (0x23) and the
wakeups goes into the protocol notes, the fake Cube, and the server's own
moves; a commit per block, one pull request, on `main` (0.18)
(`wip/2026-11-26_19-13_ANALYSIS_FakeGroup_hardware_check.md`).

- 0x22 → sets the C line's room byte, a new room's too
- 0x23 naming the device's room → byte 0; naming another room →
  refused, byte unchanged, device flagged (failed radiogram 0x23)
- fake Cube: takes neither → takes both, as the Cube did (design (c),
  the flag included)
- a room's wakeup reaches its members only → a device in no group,
  or not yet in the room, needs its own wakeup (`z:1e,D,<rf>`)
- server's move: 0x23 names the list's room → names the C line's room,
  none where it names no room; the device woken itself where a room's
  wakeup cannot reach it
- `z:1e` → awake 45 to 70 s, not 30 s; recorded, not changed here

### Blocks

1. HardwareDay: `docs/cube-protocol.md`, the findings of 2026-11-26
2. FakeGroup: the fake Cube takes 0x22 and 0x23 as the Cube did
3. PairRoom: a room setpoint reaches a device `test_pairing.py` put into the room
4. ServerMove: a move's 0x23 names the C line's room, the device woken itself

## Discussion

### What is decided

The owner, 2026-11-25 and 26, on the TL;DR gates and the hardware check:

- **D1-B.** The design waited for a check on the test Cube.
  It ran on 2026-11-26 and showed design (c).
- **D3-A.** Block 3 checks in the UI that a room setpoint reaches a placed
  device.
- **D4-A.** One pull request, a commit per block, each fix proven against
  the code before it; locally only the touched packages and test files;
  the whole suite is CI's.
- **D13-A.** The fake's 0x23 naming another room leaves the byte and sets
  the device's error flag with failed radiogram 0x23, as the Cube did.
- **D14-B.** The server's own move is a block of this plan (Block 4).
- **D15-A.** A second device in room 1, to see whether a room command
  reaches a member its address does not name, is a short check of its
  own, later, once the duty cycle allows; not a block here.
- **D16-A.** The plan goes to `main` (0.18), not `maint/0.17`.
- **D17-A**, run: the `z:` series measured the window of `z:1e`;
  `z:05` and `z:3c` were not reached (the run's 60 % stop).

### Decided in the writing

- **The order:** the protocol notes first (Block 1, documents alone),
  then the fake (Block 2), so that Block 4's tests run against a fake that
  refuses a 0x23 as the Cube does; Block 3 between them,
  since it rests on Block 2 alone.
- **The wakeup's unit** stays as it is in `cube.CmdWakeup`:
  the session "Massimo Fixer 1" holds it for its AP ClockWakeup,
  and has the measurements. Block 1 records them in the notes.
- **The roadmap:** "create a room, and move a device into a new room,
  in the page" for v0.18, as the owner asked, goes onto
  `docs/roadmap.md` with this plan's registration.
- **The branch** is `claude/fake-group`, from `main`.

## Analysis

The designs, the server's sequences and the tests the fake's change reaches
are in `wip/2026-11-25_18-25_ANALYSIS_FakeGroup_fake_cube_set_group.md`;
the check on the test Cube in
`wip/2026-11-26_19-13_ANALYSIS_FakeGroup_hardware_check.md`;
the wakeup's window in
`wip/2026-11-26_21-17_ANALYSIS_FakeGroup_z_wakeup_series_v2.md`.

### For Block 1: what goes into the notes

From the test Cube, firmware 1.1.3, 2026-11-26:

- 0x23 naming the device's own room set the C line's room byte to 0
  within 45 s; naming another room, the device refused it
  (error flag, failed radiogram 0x23) and the byte stayed.
- 0x22 set the byte, into a room the list did not have yet,
  once the device heard it.
- A room's wakeup did not wake a device in no group;
  `z:1e,D,<rf>` did, and is accepted with `A:`.
- The original software's move of a device in no group failed for that
  reason, its sequence logged line by line;
  it explains the failed move of 2026-11-16.
- A room setpoint addressed through a device in no group reached it,
  at 17 % of the duty cycle.
- `z:1e` kept a device awake 45 to 70 s; the notes read it as 30 s.
- A radiogram to a sleeping device cost 9 % alone, 59 % with an `r:` for
  the device every 2 s while it was pending.
- `r:` takes the address in base64; a hex address is refused.

### For Block 2: the fake's model

The fake keeps C lines in `s.configs` (`editConfigLocked`) and carries out
radiograms in `applyNow`, which has no case for 0x22 or 0x23.
Its device state knows the error flag (`control.go`, byte 5 bit 0x08),
and byte 4 is the failed radiogram's type.
0x22 writes the C line's room byte (payload byte 5);
0x23 writes 0 where its group is the byte, and else sets the flag and
byte 4 to 0x23.
The investigator's overlay of design (b) passed every test;
`startNewcomer` (`internal/server/config_api_test.go`) stands in for the
missing 0x22 with a hook, which then goes.

### For Block 4: the server's move

`cubeio.MoveCommands` builds the out half with `CmdSetGroup(rf, oldRoom,
true)`, `oldRoom` the room list's room, after a wakeup of that room;
the in half wakes the new room and ends with the 0x22.
On the Cube:

- where the C line names another room than the list, or none,
  the 0x23 is refused and flags the device;
- a room's wakeup reaches only devices whose C line names that room,
  so the device in a move's in half, and a device in no group in its out
  half, hear the radiograms only through a burst, or not at all.

So the 0x23 names the C line's room and is left out where it names none,
and the device is woken itself (`z:1e,D,<rf>`) where a room's wakeup would
miss it. Placement repair (`buildPlace`) and a restore whose room differs
(`backup/plan.go`) build the same halves and follow.

## Implementation Plan

Every commit is on `claude/fake-group`, one pull request against `main`.
Locally only the touched packages and test files run; CI runs the rest.
Every run takes `MASSIMO_ENV_FILE=/dev/null`.
Nothing is sent to a real Cube but Block 4's check, on the owner's go.

### Block 1: HardwareDay

- **Goal:** the notes say what the test Cube showed.
- **Changes:** `docs/cube-protocol.md`: the findings above, dated, with the
  firmware, beside the notes of 2026-11-04 and 2026-11-16 and in
  "Lessons from the hardware days"; the reading of `z:1e` as 30 s corrected
  to the measured window, its unit left to AP ClockWakeup.
- **Tests:** none: documents alone.
- **CHANGELOG:** none: `docs/` is not shipped.
- **Commit:** `docs(protocol): The test Cube's set group, remove group and wakeups`
- **Verification:** `GUIDELINES/shared/tools/check`.

### Block 2: FakeGroup

- **Goal:** the fake Cube takes 0x22 and 0x23 as the Cube did.
- **Changes:**
  1. `internal/cubelan/fakecube/apply.go`: 0x22 and 0x23 cases,
     the room byte a named constant; 0x23 naming another room flags the
     device (D13-A); the file's comment lists them.
  2. `internal/server/config_api_test.go`: `startNewcomer`'s hook goes.
  3. `internal/protocol/cube/commands.go`: `CmdSetGroup`'s comment says
     what the hardware confirmed of both.
  4. `cmd/massimo/fakecube.go`: `-apply`'s usage names the group.
- **Tests:** fakecube tests of each case: `c:` and a new greeting after
  0x22, after 0x23 of the device's room, after 0x23 of another room
  (byte unchanged, flag set), and of a joined device;
  server tests that a move and an add leave the C line in the room with no
  room finding. Each fails against the fake before the block.
- **Documents:** `README.md`, the fake Cube's text.
- **CHANGELOG:** a fragment under `CHANGELOG.d/Changed/`.
- **Commit:** `feat(fakecube): The fake Cube takes set group and remove group as the Cube does`
- **Verification:** `make ui`; `go test` of `internal/cubelan/...`,
  `internal/server`, `internal/cubeio`, `internal/proxy`, `internal/backup`,
  `cmd/massimo`; `gofmt -l .`; `go vet` for linux, windows and darwin;
  `GUIDELINES/shared/tools/check`.

### Block 3: PairRoom

- **Goal:** the UI suite shows that a placed device takes its room's commands.
- **Changes:** `tools/ui-test/test_pairing.py`:
  `test_put_into_a_room_from_devices_without_a_room` sets room 1's
  setpoint after the put and waits for the device's state to show it;
  room 1's state is put back at the test's end.
- **Tests:** the test fails against the fake before Block 2 and passes
  after; `test_pairing.py` in the four cases.
- **CHANGELOG:** none: `tools/` is not shipped.
- **Commit:** `test(ui-test): A device put into a room takes the room's setpoint`
- **Verification:** the four cases, with ports of their own and the main
  checkout's venv; `GUIDELINES/shared/tools/check`.

### Block 4: ServerMove

- **Goal:** a move, a placement repair and a restore send what the Cube takes.
- **Changes:** `internal/cubeio/move.go` and its callers
  (`internal/server/config_api.go`, `repair_api.go`,
  `internal/backup/plan.go`): the 0x23 names the C line's room and is left
  out where it names none; the device's own wakeup before a half whose
  room's wakeup would miss it.
- **Tests:** against Block 2's fake, a move whose C line names another room
  than the list, and a move of a device in no group, each ending with the
  C line in the new room and no flag; they fail before the block;
  the existing move, repair and restore tests pass.
- **Documents:** `docs/cube-protocol.md`, Massimo's move sequence.
- **CHANGELOG:** a fragment under `CHANGELOG.d/Fixed/`.
- **Commit:** `fix(cubeio): A move names the room the device is in and wakes the device itself`
- **Verification:** as Block 2; then, on the owner's go and as its gate
  says, one move on the test Cube through the proxy of the owner's service,
  the C line read after each half.

### At the retirement

`docs/roadmap.md`, item 12: the entry on the fake Cube's 0x22 and 0x23 is
pruned.

## Proposed commit message

The plan: `chore(aps): +FakeGroup v1.0, +ANALYSIS FakeGroup ×3`,
with the roadmap's entry for a new room in the page.

Block 1:

```
docs(protocol): The test Cube's set group, remove group and wakeups

For AP FakeGroup v1.0, Block 1 (D1-B): the test Cube, firmware 1.1.3,
showed on 2026-11-26 that 0x22 sets a device's C-line room byte, that
0x23 clears it only where it names the device's room and is refused
otherwise, that a room's wakeup reaches its members only, and that
z:1e keeps a device awake 45 to 70 s.

Key changes:
- docs/cube-protocol.md: the findings, dated, with the firmware.
- No CHANGELOG bullet: docs/ is not shipped.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-26): first revision, from the owner's `PLAN+ FakeGroup`
  after the TL;DR gates and the hardware check: D1-B, D3-A, D4-A,
  D13-A, D14-B, D15-A, D16-A, D17-A.
