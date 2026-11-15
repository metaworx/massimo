# T135: one room command and the device its header names

2026-11-15, 13:00–13:16 UTC, for AP LegacyLogs v1.0, Block 6 (G12), on the owner's `EXEC D156: B`.
- **Where:** .25's room 1, with one wall thermostat and three radiator thermostats; Cube firmware 1.1.3.
- **How it was sent:** through serve's proxy (.35), no wakeup before any command, as neither the original software nor Massimo sends one before 0x40.
- **Who did what:** the owner checked every device's display after each command, and serve's store on breno was read, read-only.
- **Labels:** dev1 is the wall thermostat; dev2 to dev4 are the radiator thermostats in metadata order. dev2 is the room's master, the room address in the Cube's metadata.

No names, serials or addresses below.

## TL;DR

- **One set-temperature command with the room flag sets the whole room, whichever member its header names:** the master, the wall thermostat, or any radiator thermostat. There is no "lead".
- **The room number decides which room.**
  - A command with room 1's number, its header naming a radiator thermostat of room 2, set all four devices of room 1.
  - It also set the named device in room 2. That device's wall thermostat took nothing, and put it back to its own setpoint within two minutes.
- **Every command was accepted and not discarded.**
  - Each set all four devices within 7 to 17 s in serve's store, and on their displays, as the owner confirmed each time.
  - Each took one memory slot, back within the minute, and 3 % of the duty cycle: 7 commands, 16 % → 31 %.
- **For Massimo (Block 6):**
  - a room command can be addressed to any member of the room;
  - the fallback for a room whose master is missing only has to find a member, the first radiator thermostat or else the wall thermostat;
  - it must never name a device of another room, which would take the command as well.

## The commands (UTC)

| Time | Header names | Setpoint | Cube's answer | Room 1 in serve's store | Owner's displays |
|---|---|---|---|---|---|
| 13:00:02 | dev2, the master | manual 10.0 °C | not discarded, 49 free, 16 % | all four at 13:00:09 | all at 10.0 |
| 13:08:17 | dev1, the wall thermostat | 11.0 °C | not discarded, 48 free, 16 % | all four at 13:08:28 | all at 11.0 |
| 13:10:12 | dev2 | 12.0 °C | not discarded, 48 free, 19 % | all four at 13:10:28 | all at 12.0 |
| 13:11:21 | dev3 | 13.0 °C | not discarded, 48 free, 22 % | all four at 13:11:38 | all at 13.0 |
| 13:12:12 | dev4 | 14.0 °C | not discarded, 48 free, 25 % | all four at 13:12:28 | all at 14.0 |
| 13:13:12 | a radiator thermostat of room 2, with room 1's number | 19.0 °C | not discarded, 48 free, 28 % | all four at 13:13:28; **and the named device in room 2, 5.5 → 19.0 °C**, its wall thermostat unchanged at 5.5 °C | room 1 all at 19.0; room 2's radiator thermostat at 19.0 |
| 13:14:49 | dev2 | auto (temperature 0) | not discarded, 48 free, 31 % | all four back to auto 21.0 °C at 13:14:58 | — |
| 13:15:18 | — | — | — | room 2's radiator thermostat back to 5.5 °C by itself | room 2 back to 5.5 |

Before the first command: duty 16 %, 50 slots free, every member valid and none flagged; room 2's radiator thermostat valid, manual 5.5 °C.

## What it shows

- **The room flag and number reach the room's group.** The device in the header need not be the master: any member works.
- **The named device takes the command too, even from another room.**
  - In room 2, only the named radiator thermostat changed, not its room's wall thermostat. So a Cube's command is not passed on between devices the way a wheel turn is.
  - Its wall thermostat put it back to the room's setpoint about two minutes later, presumably with its next regular transmission.
  - That fits T134 too, where a radiator thermostat set alone went back to the room's setpoint at once, twice.
- **What the Cube sees:** one slot per room command, as in N5 of the analysis of 2026-11-14, unlike a room-flagged link radiogram, which took about one slot per member (T134).

## For the plan

- **Block 6 (G12),** `internal/server/server.go`, the room setpoint and mode:
  - keep the metadata's master where it names a device of the room;
  - else the room's first radiator thermostat, else its wall thermostat;
  - and refuse rather than name a device that is in another room.

  The test checks the line sent to the fake Cube.
- **docs/cube-protocol.md,** in "Pairing a device, as the original software does it", the set-temperature addressing paragraph:

```markdown
A set-temperature command (0x40) with the room flag sets every member of the
room whose number it carries, whichever member its header names: the room's
master, its wall thermostat or any of its radiator thermostats (2026-11-15,
Cube firmware 1.1.3, one of the owner's rooms with a wall thermostat and three
radiator thermostats, through serve's proxy, no wakeup; each command set all
four within 7 to 17 s, one memory slot and 3 % of the duty cycle each).
A command whose header names a device of another room sets that device too,
besides the room it names; that device's own wall thermostat put it back
to its room's setpoint about two minutes later.
```

## How it was run

All in `GUIDELINES/temp/legacy/`:
- `t135_room.py` against the proxy, one command per run. Its lines were checked against Massimo's `CmdSetTemperature` byte for byte.
- The real wall thermostat was read from serve's store (`breno_realwall.py`), because the proxy shows serve's virtual one in its place.
- `t134_watch.py` and `breno_room_now.py` read serve's store, read-only.

Nothing else was sent to any Cube.
