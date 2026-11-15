# AP DeviceUnPairingFix v1.1: Removal and moves that work

## TL;DR

Massimo removes a device from its room with the sequence the original MAX!
software 1.4.6 used on 2026-11-15, resets the room's flagged members first,
and builds a move from the same two halves as a removal and a placement
(D156-A). A sequence counts by its results, read back from the Cube and the
devices; the original is a baseline.

- **Removal:** 0x23, then 0x21 to and from each partner → `r:` for every
  flagged member, **one** room-flagged 0x21 to a member of the room,
  `t:`, `m:`
- **Move:** its own out-part → the removal's unlink half (with a 0x23),
  then the placement's link half
- **Placement:** 0x20 in both directions per partner, 0x22 → one
  room-flagged 0x20, 0x22, as T134 sent them
- **Done:** "accepted" → accepted, the slots back after the next wakeup,
  no flag, the device working in its room
- **`linkTarget`** and the link half: this plan's (D158-A)

### Blocks

1. The removal, and each line's answer logged
2. Unlink and link halves shared by removal, placement and move
3. "Done" read back from the Cube
4. On hardware, with the owner: a removal and a move

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
protocol as a baseline but be vigilant to logical errors also there and so
maybe even improve things by our own logic, discoveries and tests".
Then "PLAN+ both".

What goes over the air cannot be observed here. That a room-flagged
radiogram reaches each member as a radiogram of its own is an assumption
the slot counts fit (one slot per member); this plan does not rest on it.

**Settled:**

- **D156-A:** a plan of its own for removal and moves.
- **D158-A** (the owner, through AP LegacyLogs,
  `wip/2026-11-15_13-55_NOTE_LegacyLogs_v1.0_block_5_dropped.md`):
  `linkTarget` and the link half of moves and removals are this plan's;
  LegacyLogs' Block 5 is dropped, and its 0x23-first change (#286) is on
  `main`.

**Open, written here as recommended:**

- **D159. What "removeDevice followed by addDevice" means for a move**
  - **A (recommended).** Shared code, the device staying paired: a move's
    out-part is the removal's unlink half (without `t:` and the metadata's
    deletion), its in-part the placement of a device in no room.
  - **B.** Delete the device (`t:`, a factory reset) and pair it again:
    someone at the device for every move, its settings lost.
- **D160. The 0x23**
  - **A (recommended).** A move sends 0x23 first, as the original's
    `DisassociateDeviceFromRoom` and T134's out-step: the device stays, its
    group has to go. A removal sends none, as captured: `t:` resets the
    device.
  - **B.** 0x23 in both: a line the original's removal does not send.
- **D161. Resets before a removal or a move**
  - **B (recommended).** One `r:01` for every flagged member of the room,
    the leaving device included: free, and it removes a known cause of
    discards; the original resets only the leaving device, and the
    original is no proof.
  - **A.** The leaving device only, as the original.
- **D167. The member a room-flagged link radiogram names**
  - **A (recommended).** The room's first radiator thermostat, else its
    wall thermostat, never the moving device and never a device of
    another room, as AP LegacyLogs' Block 6 names one for room commands;
    T135: any member works, the room number decides, and a device of
    another room named in the header takes it too.
  - **B.** The metadata's master: it may be no device of the room
    (AP RadioHealth checks it, Block 7).

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
  a room) runs the out- and in-part; a device in no room, after a pairing,
  is placed the same way, its out-part then only a 0x23. `removeDevice`
  (`DELETE …/devices/{rf}`).
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
   room-flagged 0x21 naming the leaving device, addressed to the member of
   D167-A; no 0x23, no 0x21 to the leaving device; then `t:` and `m:`.
   A device alone in its room sends no 0x21.
2. `internal/engine/queue.go`: each line's answer of a structure command
   logged at Debug, so a discard names its line.
3. Tests: the exact lines from a room of five, of two, of one, with
   flagged members.
4. `docs/cube-protocol.md` (Massimo's delete), `docs/api-v1.md`;
   `CHANGELOG.d/Fixed/`.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`,
  `make cross`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: Shared halves

1. `internal/cubeio`: `unlinkFromRoom` (Block 1's, with a 0x23 first for
   a move, D160-A) and `linkIntoRoom` (one room-flagged 0x20 naming the
   device, addressed to the member of D167-A, then 0x22 to the device, as
   T134's in-step); `linkTarget` as D167-A.
2. A move is `unlinkFromRoom` of the old room, then `linkIntoRoom` of the
   new one (D159-A); a placement after a pairing is `linkIntoRoom` alone;
   a removal `unlinkFromRoom` without 0x23, then `t:` and `m:`.
3. Each half carries its own room's number: the old room's out, the new
   room's in (T135).
4. Tests: the exact lines of a move, a placement and a removal, with and
   without a radiator thermostat in the room, and a wall thermostat moving.
5. `docs/cube-protocol.md`, `docs/api-v1.md`; `CHANGELOG.d/Fixed/`.

- **Verification:** as Block 1.

### Block 3: "Done", read back from the Cube

1. A removal, placement or move reports done once its lines were
   accepted and the Cube's free slots are back at their count before it,
   read at the next greeting or `S:` reply, within 15 minutes (one wakeup
   period and more); until then "pending", after it a Warn naming the
   slots still held.
2. Afterwards no member flagged by it and no link error lasting past the
   next wakeup: else a Warn naming the device.
3. The API's result and the UI say which ("done", "pending", "not
   confirmed").
4. Tests against the fake Cube, its slots held and freed.

- **Verification:** the Go gate, `make ui-test`, `GUIDELINES/shared/tools/check`.

### Block 4: On hardware, with the owner

Under a gate of its own, each radio command said before it is sent:

1. A move of one device into another room and back through Massimo; read
   back: lines accepted, slots back, no flag, no lasting link error, and
   the device working in its room: a room setpoint reaches it, a wheel
   turn on a partner reaches it, for a window contact the window-open
   check of "Lessons from the hardware days".
2. A removal of a device the owner chooses, through Massimo; read back:
   gone from the Cube's lists, the room's members as before.
3. Cost: about 8 % duty per move, a few radiograms per removal.
4. A NOTE of how it went.

- **Verification:** each read-back as listed; nothing discarded.

## Proposed commit message

For Block 1:

```
fix(cubeio/remove): Remove a device with one room-flagged 0x21, its room's flags reset first

A removal resets every flagged member of the room and sends one 0x21 to a member of the room naming the leaving device, without the 0x23 and the 0x21 to the leaving device, as the original software removed one on 2026-11-15 (AP DeviceUnPairingFix v1.1, Block 1).

Key changes:
- internal/cubeio/remove.go: the removal's own unlink
- internal/engine/queue.go: each line's answer of a structure command at Debug
- the exact-line tests; docs/cube-protocol.md, docs/api-v1.md

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
fix(cubeio/move): A move is the removal's unlink and the placement's link

Removal, placement and move share two halves, each with one room-flagged link radiogram carrying its room's number (AP DeviceUnPairingFix v1.1, Block 2).

Key changes:
- internal/cubeio: unlinkFromRoom and linkIntoRoom; MoveCommands and RemoveCommands built on them; linkTarget
- the exact-line tests of a move, a placement and a removal

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 3 and 4: `feat(cubeio/done)`, and a NOTE.

## Change History

- v1.0 (2026-11-15): first revision; D156-A settled; D158, D159, D160,
  D161 open.
- v1.1 (2026-11-15): D158-A settled; T134 and T135 in the analysis; a
  sequence judged by its results, the original a baseline; D161 to B;
  D167 added; Block 2 no longer waits for T134; Block 3 reads "done"
  back from the Cube; the hardware run is Block 4.
