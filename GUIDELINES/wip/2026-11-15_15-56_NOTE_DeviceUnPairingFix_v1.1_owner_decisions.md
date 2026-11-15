# NOTE DeviceUnPairingFix v1.1: owner decisions

The owner's answers to the plan gate of AP DeviceUnPairingFix v1.1
(`wip/2026-11-15_13-10_AP_DeviceUnPairingFix_removal_and_moves_as_the_original.md`),
2026-11-15.

## Settled

- **D159-A**, with B as a manual route: a move is the removal's unlink
  half and the placement's link half, the device staying paired; removing
  and pairing anew stays available by hand (D168).
- **D160-A:** 0x23 first in a move, none in a removal.
- **D161-B:** before a removal or a move, one `r:01` for every flagged
  member of the room, the leaving device included.
- **D167, in the owner's order:** a room-flagged link radiogram names
  the metadata's master where it is a device of the room, else the room's
  wall thermostat, else its first radiator thermostat; never the moving
  device. A master outside its room is warned of (AP RadioHealth, Block 7),
  only while no unread warning for it exists.
- **D168-A:** the manual re-pair is a guided action in the device's
  panel, under `structure` and `pair`: it keeps the device's name, room
  and configuration, removes the device (its factory reset), opens the
  pairing and asks for the device's button, places it in its room and
  writes its configuration back. The route for a device whose links do
  not come right.
- **D169-A:** one order for the member a room radiogram names, D167's,
  in one function that moves and room commands share; AP LegacyLogs'
  Block 6 (the room-command fallback, master, then first radiator
  thermostat, then wall thermostat) changes to call it, which its session
  is told on the owner's word.
