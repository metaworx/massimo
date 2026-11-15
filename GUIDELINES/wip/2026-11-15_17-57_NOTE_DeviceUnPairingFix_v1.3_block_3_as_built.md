# NOTE DeviceUnPairingFix v1.3: block 3 as built

How Block 3 of AP DeviceUnPairingFix v1.3
(`wip/2026-11-15_13-10_AP_DeviceUnPairingFix_removal_and_moves_as_the_original.md`),
"Done, read back from the Cube", was built on 2026-11-15 under the owner's
`EXEC DUPF 3`, where it differs from the plan's words.

- **The checks** run in the follow-up loop (`internal/server/followup.go`),
  which already re-reads C lines and reconnects for the budget after
  writes: at 2, 6 and 12 minutes after the change
  (`Options.StructureChecks`, `DefaultStructureChecks`), the plan's
  "within 15 minutes". Done is a budget the Cube reported after the change
  with the free slots back at their count before it. At the last check,
  only where the Cube reported nothing since the change, the server
  reconnects once for a fresh H line, and only with `RefreshInterval` set,
  as the follow-up's own budget refresh.
- **Item 2** compares the error flag and the link error of the members of
  the rooms concerned (by their C lines) and of the device, before the
  change and at its outcome; the outcome comes after one wakeup period
  of about ten minutes, so "past the next wakeup" is the outcome's time.
- **Item 3**: the API answers `"confirmation": "pending"` for a removal,
  a placement and a move; "done" and "not confirmed" are the Cube's log
  event of the new kind `structure` (at `info`, or at `warn` with the
  slots still held, `no_reading`, and the members `flagged` or with a
  `link_error`), which the log page shows as **device change** and which,
  at `warn`, is an alert until marked. The command's stored result is not
  rewritten afterwards. The UI's messages after a move, a removal and a
  placement add that the room's devices take it at their next wakeup and
  that the log says when it is done.
- **The fake Cube** gained `SetFreeSlots`, for its S replies and the H line
  of its greeting, so that a test holds and frees slots.
- **A Block 1 gap closed**: the OpenAPI file's removal description still
  gave the old sequence (0x23, 0x21 both ways); it now gives Block 1's.
