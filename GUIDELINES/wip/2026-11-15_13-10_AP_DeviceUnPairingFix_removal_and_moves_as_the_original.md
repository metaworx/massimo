# AP DeviceUnPairingFix v1.0: Removal and moves as the original

## TL;DR

Massimo removes a device from its room the way the original MAX! software
1.4.6 removed one on 2026-11-15, and builds a move from the same two halves
as a removal and a placement, as the owner asked (D156-A).

- **Removal:** 0x23, then 0x21 to and from each partner → `r:` for the
  leaving device where flagged, **one** room-flagged 0x21 to the room's
  anchor, then `t:` and `m:`, as captured
- **Move:** its own out-part → the removal's unlink half (with the 0x23 a
  device that stays needs), then the placement's link half
- **Placement** (a device into a room): one room-flagged 0x20 to the anchor,
  as the original's `AssociateNewDeviceToRoom`, once AP LegacyLogs' T134
  shows it reaches every member
- **`linkTarget`** and the link half: taken over from AP LegacyLogs, Block 5 (D158)
- **On hardware**, with the owner: one removal and one move through Massimo

### Blocks

1. The removal as the original sends it
2. Unlink and link halves shared by removal, placement and move
3. On hardware: a removal and a move through Massimo

## Discussion

Massimo's removal of a radiator thermostat from room 1 of the Cube behind
.35 was discarded by the Cube twice on 2026-11-15 (04:28 and 04:41), before
its `t:`, and left the room's five members flagged with byte 4 = 0x21.
The owner then removed it with the original software, whose log shows the
sequence (Analysis). The owner, 2026-11-15:
"156 A -> making the move.go using the same path as removeDevice followed
by addDevice …", then "PLAN+ RadioHealth + DeviceUnPairingFix".

AP LegacyLogs, Block 3, already sends a move-out's 0x23 first (#286, on
`main`). Its Block 4 is T134, a hardware test of one room-flagged link to
an anchor (0x21, then 0x20 and 0x22) in the same room, waiting for the
owner's `EXEC t134`; its Block 5, the anchor links in `linkTarget`, is not
started and waits for T134. The log-mining session recommends that Block 5
moves here, with T134's result as its input, and that LegacyLogs keeps the
test; it touches nothing of `move.go`, `remove.go` or `linkTarget` until
the owner decides (D158).

**Open, each with its recommendation:**

- **D158. Who owns `linkTarget` and the link half**
  - **A (recommended).** This plan: AP LegacyLogs' Block 5 moves here;
    LegacyLogs keeps T134, whose result this plan's Block 2 waits for.
    One plan changes the links of removal, placement and move together,
    the same change everywhere.
  - **B.** LegacyLogs keeps Block 5, and this plan waits for it before
    Block 2: two plans on the same three functions in turn.
- **D159. What "removeDevice followed by addDevice" means for a move**
  - **A (recommended).** Shared code, the device staying paired:
    a move's out-part is the removal's unlink half (without `t:` and the
    metadata's deletion), its in-part the placement of a device that is in
    no room, which is what follows a pairing today.
    No button press, the device keeps its settings.
  - **B.** Literally: delete the device (`t:`, a factory reset), then pair
    it again into the new room: someone at the device for every move,
    and its week programme and offsets lost.
- **D160. The 0x23 of a move's out-part**
  - **A (recommended).** A move sends 0x23 first, as the original's
    `DisassociateDeviceFromRoom` (captured 2026-11-04, AP LegacyLogs
    Block 3): the device stays, so its group has to go.
    A removal sends none, as the original's `RemoveDevices` (captured
    2026-11-15): `t:` resets the device anyway.
  - **B.** 0x23 in both: one line more in a removal, which the original
    does not send.
- **D161. Resets before a removal**
  - **A (recommended).** As the original: one `r:` for the leaving device
    where its 0x08 is set; the room-flagged 0x21 was accepted with four
    other members flagged. AP RadioHealth's Block 2 (room commands) does
    not apply: a removal is no room command.
  - **B.** Reset every flagged member of the room first: lines the
    original does not send, though free.

## Analysis

**The original software's removal** of a radiator thermostat that answers
(its log, `%USERPROFILE%\.MAX\logs\eq3.2026-11-15.log`, 12:30:41–43,
the Cube behind .35, firmware 1.1.3, its five room members flagged
with byte 4 = 0x21 by Massimo's two attempts):

1. `removeDevices([<address>], true)` → `wakeUpRoom(1)`, a `z:` line;
2. `r:` for the leaving device (its 0x08 set): 3 % duty, 49 free, as before;
3. one `s:` 0x21: flags 0x04, room 1, the recipient the room's first
   radiator thermostat, the payload the leaving device's address and type
   (`00 04 21 000000 <anchor> 01 <leaving> 01`); answered not discarded,
   6 % duty, 49 → 45 free, back to 49 within three minutes;
4. `deleteDevices([<address>], true)`, `t:01,1,…`;
5. `sendMetaData`, the `m:` without the device.

No 0x21 to the leaving device, no 0x23. Afterwards the Cube listed one
device fewer, and the four others reported as before.

**Massimo today** (`main`, after AP LegacyLogs Block 3):

- `internal/cubeio/move.go`, `MoveCommands`: the out-part is a room wakeup,
  0x23 for the moving device, then for each partner 0x21 to the partner
  (radiator thermostats room-flagged, through `linkTarget`) and 0x21 to
  the moving device; the in-part 0x20 in both directions per partner and
  0x22. With four partners that is nine radio lines out and nine in.
- `internal/cubeio/remove.go`, `RemoveCommands`: the unlink is
  `MoveCommands`' out-part, then `t:` with the force flag; `RemoveDevice`
  writes the metadata without the device.
- `internal/server/config_api.go`: `moveDevice` (`PUT …/devices/{rf}` with
  a room), `removeDevice` (`DELETE …/devices/{rf}`); a device in no room,
  after a pairing, is placed by the same `moveDevice`, its out-part then
  only a 0x23.
- The queue (`internal/engine/queue.go`) wakes the room, resets the
  command's target where flagged and retries once after a discard.
- The exact-line tests: `internal/cubeio/cubeio_test.go`,
  `internal/server/config_api_test.go`.

**Which line the Cube discarded at 04:28 and 04:41** is not known: the
queue logs a step, not its lines' answers. Nothing was flagged before the
first attempt, so it was not the flag.

**The original's placement and move:** `AssociateNewDeviceToRoom` sends one
room-flagged link to an anchor, the room flag reaching the others;
`DisassociateDeviceFromRoom` sends 0x23 first (AP LegacyLogs' analysis,
`wip/2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md`, G3 and G4).
Whether one room-flagged 0x20 reaches every member is T134's question;
for 0x21 the removal above is strong evidence (one slot per member).

## Implementation Plan

### Block 1: The removal as the original sends it

1. `internal/cubeio/remove.go`: the unlink of a removal is its own:
   a room wakeup, `r:` for the leaving device where its 0x08 is set
   (D161-A), one room-flagged 0x21 to the room's anchor (its first radiator
   thermostat, else its wall thermostat) naming the leaving device;
   no 0x23 (D160-A), no 0x21 to the leaving device. Then `t:` and `m:`
   as today.
2. A room with no partner left (the device alone in it) sends no 0x21.
3. The queue logs each line's answer of a structure command at Debug,
   so a discard names its line.
4. Tests: the exact lines of a removal from a room of five, from a room of
   two, of a device alone, of a flagged leaving device.
5. `docs/cube-protocol.md` (Massimo's delete), `docs/api-v1.md`;
   `CHANGELOG.d/Fixed/`.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`,
  `make cross`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: Shared unlink and link halves (after T134)

1. `internal/cubeio`: two halves, `unlinkFromRoom` (Block 1's, with the
   0x23 for a move, D160-A) and `linkIntoRoom` (one room-flagged 0x20 to
   the anchor and the 0x20s the moving device needs, then 0x22, as T134
   shows them); `linkTarget` as T134 shows it (D158-A).
2. A move is `unlinkFromRoom` of the old room followed by `linkIntoRoom`
   of the new one (D159-A); the placement after a pairing is
   `linkIntoRoom` alone; a removal is `unlinkFromRoom` without 0x23,
   then `t:` and `m:`.
3. Tests: the exact lines of a move, a placement and a removal, for rooms
   with and without a radiator thermostat, and with a wall thermostat
   moving.
4. `docs/cube-protocol.md`, `docs/api-v1.md`; `CHANGELOG.d/Fixed/`.

- **Verification:** as Block 1, and the move's exact lines against
  T134's captured ones.

### Block 3: On hardware, with the owner

Under a gate of its own, each radio command said before it is sent:

1. A move of one device into another room of a Cube and back, through
   Massimo (`PUT …/devices/{rf}`); the store and the Cube's greetings
   show the members' flags, the slots and the device's room.
2. A removal of a device the owner chooses (a spare, or one being
   replaced), through Massimo (`DELETE …/devices/{rf}`).
3. Cost: about 8 % duty per move and a removal's few radiograms.
4. A NOTE of how it went.

- **Verification:** each step's flags and slots read back; nothing
  discarded.

## Proposed commit message

For Block 1:

```
fix(cubeio/remove): Remove a device with one room-flagged 0x21, as the original

A removal resets the leaving device where flagged and sends one 0x21 to the room's anchor, without the 0x23 and without the 0x21 to the leaving device, as the original software removed one on 2026-11-15 (AP DeviceUnPairingFix v1.0, Block 1).

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
fix(cubeio/move): A move is the removal's unlink and the placement's link

Removal, placement and move share two halves, each with one room-flagged link to the room's anchor as the original software sends it (AP DeviceUnPairingFix v1.0, Block 2).

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-15): first revision; D156-A settled; D158, D159, D160,
  D161 open.
