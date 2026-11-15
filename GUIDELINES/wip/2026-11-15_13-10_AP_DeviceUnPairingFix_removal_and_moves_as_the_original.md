# AP DeviceUnPairingFix v1.3: Removal and moves that work

## TL;DR

Massimo removes, moves and places devices with the original MAX! software's
sequences, resets the room's flagged members first, and re-pairs a device
whose links do not come right. A sequence counts by its results.

- **Removal:** 0x23, 0x21 to and from each partner → `r:` for every
  flagged member, **one** room-flagged 0x21, `t:`, `m:`
- **Move:** its own out-part → the removal's unlink half with a 0x23
  and the device's own side, then the placement's link half
- **Placement:** 0x20 both ways per partner → one room-flagged 0x20,
  one per window contact, one to the device per partner, 0x22 (D170-A)
- **Addressee:** the master in the room, else the wall, else the first
  radiator thermostat; room commands too
- **Done:** "accepted" → slots back, no flag, the device working
- **Re-pair:** a guided action that keeps name, room and configuration

### Blocks

1. The removal, and each line's answer logged
2. Shared halves and one addressee for every room radiogram
3. "Done" read back from the Cube
4. A guided re-pair that keeps the device's settings
5. On hardware, with the owner: a removal and a move

## Discussion

Massimo's removal of a radiator thermostat from room 1 of the Cube behind
.35 was discarded by the Cube twice on 2026-11-15 (04:28 and 04:41),
before its `t:`, and left the room's five members flagged with byte 4 =
0x21. The owner then removed it with the original software, whose log
shows the sequence (Analysis).

The owner, 2026-11-15: "156 A -> making the move.go using the same path as
removeDevice followed by addDevice …", "PLAN+ RadioHealth +
DeviceUnPairingFix"; on what counts: "We don't need to know the internals
of the radio signals. We need to know the working commands we need to send
to the CUBE in order to get reliable results", and "take the legacy
protocol as a baseline but be vigilant to logical errors also there".
At the gate of v1.1: "159 A (B remains as a manual route by first removing
and then paring) … 167 A with the metadata's master tried first, as long
as it is in the same room - warned if not (and no unread warning) - , not
the moving device, falling back to the WT (if available) then the first TS
in the room", then "D168A, D169A". At the stop of Block 2, before its
commit: "D170-A, revise the AP to v1.3 accordingly and commit it with
the changes to block 2, push+PR both blocks with the revised AP. EXEC".

What goes over the air cannot be observed here. That a room-flagged
radiogram reaches each member as a radiogram of its own is an assumption
the slot counts fit (one slot per member); this plan does not rest on it.

**Settled** (D159 to D169 at the gate of v1.1, NOTE DeviceUnPairingFix
v1.1: owner decisions, folded in here):

- **D156-A:** a plan of its own for removal and moves.
- **D158-A** (the owner, through AP LegacyLogs,
  `wip/2026-11-15_13-55_NOTE_LegacyLogs_v1.0_block_5_dropped.md`):
  `linkTarget` and the link half of moves and removals are this plan's;
  LegacyLogs' 0x23-first change (#286) is on `main`.
- **D159-A, with B by hand:** a move is the removal's unlink half and the
  placement's link half, the device staying paired; removing and pairing
  anew stays available as D168's guided action.
- **D160-A:** 0x23 first in a move, as the original's
  `DisassociateDeviceFromRoom` and T134's out-step; none in a removal, as
  captured: `t:` resets the device.
- **D161-B:** before a removal or a move, one `r:01` for every flagged
  member of the room, the leaving device included; free, and it removes a
  known cause of discards, though the original resets the leaving device
  alone.
- **D167, the owner's order:** a room radiogram names the metadata's
  master where it is a device of the room, else the room's wall
  thermostat, else its first radiator thermostat; never the moving
  device, never a device of another room. A master outside its room is
  warned of (AP RadioHealth, Block 7), only while no unread warning for
  it exists.
- **D168-A:** the manual re-pair is a guided action in the device's
  panel, under `structure` and `pair`: it keeps the device's name, room
  and configuration, removes the device (its factory reset), opens the
  pairing and asks for the device's button, places it in its room and
  writes its configuration back.
- **D170-A** (at the stop of Block 2, before its commit): a move's and
  a placement's link halves follow the original software's captures of
  2026-11-04 (Analysis): the room's one radiogram with the room flag to
  the addressee, one to each window contact among the partners, and one
  to the moving or placed device for each partner, so that its own
  partners follow it; 0x23 first out, 0x22 last in. v1.2 had taken
  T134's in- and out-step, a test of one radiogram reaching every
  member, for the original's sequence; with it the device would keep
  its old room's partners and learn none in the new one, and T134's wall
  thermostat showed a link error three minutes after its out-step.
  A removal is not concerned: the original's of 2026-11-15 sent nothing
  to the leaving device, which `t:` resets.
- **D169-A:** D167's order is one function that moves, removals and room
  commands share; AP LegacyLogs' Block 6 (the room-command fallback, now
  master, then first radiator thermostat, then wall thermostat) changes to
  call it, its session told on the owner's word.

## Analysis

**The original software's removal** (its log,
`%USERPROFILE%\.MAX\logs\eq3.2026-11-15.log`, 12:30:41–43, the Cube
behind .35, firmware 1.1.3, four of the room's members flagged with byte 4
= 0x21 by Massimo's two attempts):

1. `wakeUpRoom(1)`, a `z:` line;
2. `r:` for the leaving device (flagged); duty and slots as before;
3. one `s:` 0x21: flags 0x04, room 1, the recipient the room's first
   radiator thermostat, the payload the leaving device's address and type;
   accepted, 49 → 45 free, back to 49 within three minutes;
4. `t:01,1,…`; 5. the `m:` without the device.

No 0x21 to the leaving device, no 0x23. The Cube listed one device fewer;
the others reported as before, and a room command three minutes later
(with four `r:01` first) set them all.

**The original software's placement and move** (its captures of
2026-11-04, `docs/cube-protocol.md`, "Pairing a device, as the original
software does it"): a new wall thermostat into a room of one radiator
thermostat and two window contacts got, after the room's wakeup, 0x20
to the radiator thermostat with the room flag, 0x20 to each contact
directly, and 0x20 to the new device for each partner, then 0x22.
A move sent the old room's wakeup, 0x23, 0x21 for every device of the
old room in both directions with the same addressing, then the new
room's wakeup, 0x20 for every device of the new room in both
directions, 0x22.

**T134** (AP LegacyLogs, 2026-11-15, the same room, through the proxy):
out: `z:`, 0x23 to the wall thermostat, one room-flagged 0x21 to the first
radiator thermostat naming the wall thermostat (50 → 47 → 45 free);
in: `z:`, one room-flagged 0x20 to the same radiator thermostat, 0x22 to
the wall thermostat (48 → 44 → 43). Only the addressed member's slot came
back at once; the others' at the Cube's next wakeup, about 10 minutes
later. The wall thermostat showed a link error three minutes after the out
step, cleared after the in step.

**T135** (the same room): a room-flagged 0x40 set every member of the
numbered room whichever member its header named; one naming a device of
another room set that device too.

**Massimo today** (`main`):

- `internal/cubeio/move.go`, `MoveCommands`: the out-part a room wakeup,
  0x23, then for each partner 0x21 to the partner (radiator thermostats
  room-flagged, `linkTarget`) and to the moving device; the in-part 0x20
  in both directions per partner and 0x22. With four partners nine radio
  lines each way.
- `internal/cubeio/remove.go`, `RemoveCommands`: `MoveCommands`' out-part,
  then `t:`; `RemoveDevice` writes the metadata without the device.
- `internal/server/config_api.go`: `moveDevice` (`PUT …/devices/{rf}` with
  a room) runs the out- and in-part, and with its copy option writes the
  device's configuration; a device in no room, after a pairing, is placed
  the same way, its out-part then only a 0x23. `removeDevice`
  (`DELETE …/devices/{rf}`). Pairing: `internal/cubeio/pairing.go`,
  `internal/server/pairing_api.go`.
- `internal/server/server.go`: AP LegacyLogs' Block 6 picks the member a
  room command names.
- `internal/engine/queue.go`: wakes the room, resets the command's target
  where flagged, retries once after a discard; it logs a step's result,
  not each line's answer, so which line the Cube discarded at 04:28 and
  04:41 is not known.
- The exact-line tests: `internal/cubeio/cubeio_test.go`,
  `internal/server/config_api_test.go`.

## Implementation Plan

### Block 1: The removal

1. `internal/cubeio/remove.go`: a removal's unlink of its own: the room
   wakeup, one `r:01` per flagged member of the room (D161-B), one
   room-flagged 0x21 naming the leaving device, addressed as D167; no
   0x23, no 0x21 to the leaving device; then `t:` and `m:`. A device alone
   in its room sends no 0x21.
2. Each line's answer recorded, so a discard names its line: as built,
   in the step's stored result (`internal/server/commands.go`:
   `answers`, `discarded_line`), since the queue already logs each line's
   answer and a step the server observes logs at Debug, which is how the
   answers of 04:28 and 04:41 were lost.
3. Tests: the exact lines from a room of five, of two, of one, with
   flagged members.
4. `docs/cube-protocol.md` (Massimo's delete), `docs/api-v1.md`;
   `CHANGELOG.d/Fixed/`.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`,
  `make cross`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: Shared halves and one addressee

1. `internal/cubeio`: the addressee of a room radiogram (D167, D169-A):
   the master where it is a device of the room, else the wall thermostat,
   else the first radiator thermostat, never a given device to leave out;
   `linkTarget` becomes it.
2. `unlinkFromRoom` (Block 1's; for a move with a 0x23 first, D160-A,
   and the device's own side, D170-A) and `linkIntoRoom` (one
   room-flagged 0x20 naming the device, to the addressee, one to each
   window contact among its partners, one to the device for each
   partner, then 0x22 to the device), as the original's captures of
   2026-11-04.
3. A move is `unlinkFromRoom` of the old room, then `linkIntoRoom` of the
   new one (D159-A); a placement after a pairing is `linkIntoRoom` alone;
   a removal `unlinkFromRoom` without 0x23, then `t:` and `m:`. Each half
   carries its own room's number (T135).
4. Room commands name the same addressee (`internal/server/server.go`,
   AP LegacyLogs' Block 6), in this block where that block is merged,
   else in that block. As run: Block 6 reached `main` with #295 while
   this block ran, so its `roomAddress` gave way to
   `cubeio.RoomAddressee` here, the same order; the backup restore's
   step labels count partners (`cubeio.PartnerCount`), no longer half
   the lines.
5. Tests: the exact lines of a move, a placement and a removal, with and
   without a radiator thermostat or a wall thermostat in the room, a
   wall thermostat moving, and a master in another room.
6. `docs/cube-protocol.md`, `docs/api-v1.md`; `CHANGELOG.d/Fixed/`.

- **Verification:** as Block 1.

### Block 3: "Done", read back from the Cube

1. A removal, placement or move reports done once its lines were accepted
   and the Cube's free slots are back at their count before it, read at
   the next greeting or `S:` reply, within 15 minutes (a wakeup period and
   more); until then "pending", after it a Warn naming the slots still held.
2. Afterwards no member flagged by it and no link error lasting past the
   next wakeup: else a Warn naming the device.
3. The API's result and the UI say which ("done", "pending", "not
   confirmed").
4. Tests against the fake Cube, its slots held and freed.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 4: A guided re-pair (D168-A)

1. A route and a panel action under `structure` and `pair`: the device's
   name, room and configuration (temperatures, week programme, valve
   settings) are kept; the device is removed (Block 1, its factory reset);
   the pairing opens and the UI asks for the device's button; the paired
   device, matched by its serial, is placed in its room (Block 2) and its
   configuration written back.
2. A device that does not pair within the pairing time: the kept settings
   stay for a retry, and the UI says so.
3. Six languages; `docs/ui.md`, `docs/api-v1.md`.
4. Tests against the fake Cube, its pairing answered and not.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 5: On hardware, with the owner

Under a gate of its own, each radio command said before it is sent:

1. A move of one device into another room and back through Massimo; read
   back: lines accepted, slots back, no flag, no lasting link error, and
   the device working in its room: a room setpoint reaches it, a wheel
   turn on a partner reaches it, for a window contact the window-open
   check of "Lessons from the hardware days".
2. A removal of a device the owner chooses, through Massimo; read back:
   gone from the Cube's lists, the room's members as before.
3. Where the owner has one at hand, a re-pair (Block 4), the owner at the
   device.
4. Cost: about 8 % duty per move, a few radiograms per removal.
5. A NOTE of how it went.

- **Verification:** each read-back as listed; nothing discarded.

## Proposed commit message

For Block 1:

```
fix(cubeio/remove): Remove a device with one room-flagged 0x21, its room's flags reset first

A removal resets every flagged member of the room and sends one 0x21 to the room's addressee naming the leaving device, without the 0x23 and the 0x21 to the leaving device, as the original software removed one on 2026-11-15 (AP DeviceUnPairingFix v1.3, Block 1).

Key changes:
- internal/cubeio/remove.go: the removal's own unlink
- internal/engine/queue.go: each line's answer of a structure command at Debug
- the exact-line tests; docs/cube-protocol.md, docs/api-v1.md

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
fix(cubeio/move): A move is the removal's unlink and the placement's link

Removal, placement and move share two halves, each with one room-flagged link radiogram carrying its room's number, and for a move or a placement one to each window contact and one to the device for each partner, as the original software's captures of 2026-11-04 (AP DeviceUnPairingFix v1.3, Block 2).

Key changes:
- internal/cubeio: the addressee; unlinkFromRoom and linkIntoRoom; MoveCommands and RemoveCommands built on them
- the exact-line tests of a move, a placement and a removal

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 3 to 5: `feat(cubeio/done)`, `feat(api/repair-pairing)`, and a NOTE.

## Change History

- v1.0 (2026-11-15): first revision; D156-A settled; D158, D159, D160,
  D161 open.
- v1.1 (2026-11-15): D158-A settled; T134 and T135 in the analysis; a
  sequence judged by its results, the original a baseline; D161 to B;
  D167 added; Block 2 no longer waits for T134; Block 3 reads "done"
  back from the Cube; the hardware run is Block 4.
- v1.2 (2026-11-15): folds NOTE DeviceUnPairingFix v1.1: owner decisions;
  D159-A with B by hand, D160-A, D161-B, D167 in the owner's order,
  D168-A, D169-A settled; Block 2 makes the addressee one function, room
  commands included; Block 4 the guided re-pair; the hardware run is
  Block 5.
- v1.3 (2026-11-15): D170-A settled at the stop of Block 2: a move's and
  a placement's link halves as the original's captures of 2026-11-04,
  the device's own side and the window contacts included; the captures
  in the Analysis; Block 1's line answers as built, in the step's result;
  Block 2 item 4 as run, room commands on the shared addressee once
  LegacyLogs' Block 6 was on `main`.
