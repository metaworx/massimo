# NOTE LegacyLogs v1.0: block 5 dropped

2026-11-15, the owner's `EXEC D154-A`.

**Block 5 (anchor links) is dropped.** Massimo keeps sending one room-flagged link radiogram (0x20, 0x21) to each radiator thermostat of the room, as `internal/cubeio/move.go` does. The link code of moves and removals is left to AP DeviceUnPairingFix of the session "MAX! transmit error", which was told.

## Why

- **The question was the link radiograms of a move or a removal:** whether one room-flagged radiogram to the room's first radiator thermostat (the original software's form) changes every member's link list.
- **What it would save:** a few radiograms on a rare move. It never touches a setpoint.
- **The owner:** setting the room's temperature is what matters. That question goes to T135 (D156-B): one room-flagged 0x40, addressed to the room's master, to each member in turn and to a device of another room.

## T134, run in .25's room 1 on 2026-11-15, 12:05–12:35 UTC

- **The Cube took one room-flagged 0x21 as one delivery per member,** about one memory slot each. The original software's removal the same day took 4 slots in a room of 4.
- **The addressed radiator thermostat took it at once.** The wall thermostat it named lost that partner 3 minutes later: its link error, the slowly blinking antenna.
- **The other two deliveries stayed pending until the next wakeup,** 10 minutes later. The Cube spent 6 % of the duty cycle on its own retries.
- **The owner's wheel turns came in between,** so every change still reached the wall thermostat through the two members that kept it.
- **Whether every member acts on the radiogram is therefore open.** A decisive run would wait for the Cube's slots before the wheels. It is not run.
- **The room is whole again,** and no repair was sent.

## What changes in the plan

- **Block 5** is not run.
- **Block 4** keeps its docs commit, with the finding above in `docs/cube-protocol.md` ("Pairing a device, as the original software does it"). It is still to come, with the plan's next blocks.
- **D155 (Solaio)** is moot.
