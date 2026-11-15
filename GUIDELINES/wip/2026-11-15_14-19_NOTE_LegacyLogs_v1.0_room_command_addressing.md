# NOTE LegacyLogs v1.0: room command addressing

2026-11-15, a measured fact for Block 6 (G12), from the hardware test T135 of the same day (`wip/2026-11-15_14-16_ANALYSIS_LegacyLogs_room_setpoint_test.md`), run on the owner's `EXEC D156: B`.

## The fact

- **Any member works.** One set-temperature command (0x40) with the room flag sets every member of the room whose number it carries, whichever member its header names: the room's master (the room address in the Cube's metadata), its wall thermostat, or any radiator thermostat. Five commands to one room of a wall thermostat and three radiator thermostats each set all four within 7 to 17 s.
- **The room number decides the room.**
- **The named device takes the command too, even from another room.** One command with that room's number, its header naming a radiator thermostat of another room, set the four and the named device. That device's own wall thermostat put it back to its room's setpoint about two minutes later.

## What changes in Block 6

- The plan's fallback stands: the metadata's master, else the room's first radiator thermostat, else its wall thermostat.
- **Two refinements:**
  - the master counts only where it names a device that is in the room: by the metadata and by its own C line;
  - a room whose candidates are all elsewhere gets a refusal (`fail(409, …)`), never a command that names a device of another room.
- **The test:**
  - a room whose master is missing, or names a device of another room, sends the line with the first radiator thermostat;
  - a room with only a wall thermostat sends it with the wall thermostat;
  - each line checked exactly against the fake Cube.

No new block: Block 6 keeps its place and its commit.
