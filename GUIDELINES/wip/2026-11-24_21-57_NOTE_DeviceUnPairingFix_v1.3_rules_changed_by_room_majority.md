# NOTE DeviceUnPairingFix v1.3: rules changed by room majority

AP RoomMajority (merged with #410, retired on 2026-11-24, `chore(aps): -RoomMajority`)
changed two of this plan's rules.
The code, `docs/api-v1.md` and `docs/cube-protocol.md` follow the rules below;
this plan's text, where it states the old ones, is to be read with this NOTE.

## D167 and D169-A, the addressee of a room radiogram

The one function D169-A asked for is now `roomorder.Of`,
and `cubeio.Addressee` takes its first (RoomMajority.D9):

1. a thermostat the Cube holds no valid state for is passed over;
2. the room's master, where it is a thermostat of the room;
3. the room's wall thermostat;
4. its first radiator thermostat, then each other one, in the Cube's order;
5. where every thermostat of the room is dead, the same order without step 1,
   since a radiogram with the room flag must name one.

The moving or leaving device is never named.
This holds for room commands, the room checks' re-send, the links of a move,
a removal and a placement, the re-links, "unlink all and re-link",
and the master repair's candidate.

## Blocks 1 and 2, the copy of a room's settings (`CopyCommands`)

- **The state:** the room's state is read under the structure lock,
  before the device is linked, by the room checks' order:
  Massimo's own command, the majority, the newest change, then the order.
  The copy's 0x40, still sent to the device directly without a Target,
  carries its mode, setpoint and until in place of auto with temperature 0.
  Auto with 0 stays where the room is on its programme,
  or has no state or no other thermostat.
  A newer room setpoint of Massimo's, sent while the device was linked,
  takes the read state's place.
- **A discarded 0x40:** the state goes once more as a room command after the
  programme, recorded as Massimo's own command.
- **The configuration:** each group (the temperatures, the valve settings) and
  each day of the programme is what more than half of the room's other
  thermostats with a valid state share, else the first of the order's.
  The valve settings and the window-open minutes come from the radiator
  thermostats alone.
  Each device keeps its own offset.
- **The copy's 0x11** goes to the device and to the room's thermostats with a
  valid state, no longer to every member; the 0x12 and the differing days go to
  the device only.
- **The repair's Place** holds the room's state for the re-send too,
  and still copies nothing.
- **The answer's `copied`** names the state sent (`state`, `state_from`),
  why no configuration went (`config_skipped`), and `room_mode`.
