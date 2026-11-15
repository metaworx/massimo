# NOTE RoomConfirm v1.0: how the run went

2026-11-15, the run of `PLAN++ RoomConfirm` after `EXEC D190-A`
(branch `claude/room-confirm`, Blocks 1 to 3, not pushed).
Where the work went differently from the plan, and what verified it.

## Different from the plan

1. **AP LegacyLogs was not retired** in the register commit (D190-A):
   AP DeviceUnPairingFix still names one of its NOTEs by its path.
   Its six analyses stay under their names (D191-A).
2. **D175 is two decisions.**
   AP UiParts' NOTE of its Block 2 numbered the TypeScript baseline D175
   at the same time.
   The register commit says so; this plan's next revision renames its own.
3. **A merge of main before Block 3**, with RadioHealth's repair and drift
   (#312, #313), whose UI changed the same files.
   Both sides were kept in the four conflicts.
4. **An answer of `confirmed`** besides `pending`:
   each device shows the write already at the Cube's answer,
   as with an unchanged value, or a configuration the writer's own re-read
   already shows.
5. **A configuration write is one write per device.**
   A room's configuration or programme has one warning for each device
   that did not take it, with the device in `device`.
   The L entry's writes keep one warning per write.
6. **Replacement:**
   - an L entry's write replaces the open write of its kind to the device
     (setpoint, lock, display);
   - a configuration write replaces the values it sets, so a newer eco
     leaves an older boost open.
7. **The values' names** are the configuration's API names.
   A day is `program.<day>`, and a long day's second radiogram
   `program.<day>.2`, since a LAN proxy client sends the two as two lines.
8. **A device's setpoint** goes with the room flag, and every member takes
   it (T135), but only the device it names is expected to show it,
   as the plan has it.
9. **Not in the plan:**
   - the warning settles by a mark alone, as the structure line does;
   - the header's problem list does not carry the mark (D179-A names the
     tile);
   - the tile's mark comes from the devices' confirmations, so a state
     event clears it at once;
   - the page reloads the rooms at each unconfirmed line.
10. **A wall thermostat** is compared by what its C line holds:
    the presets, the offset, the window-open temperature and the boost.
    That the line takes 0x11 and 0x12 is the original software's and FHEM's
    reading, not seen on hardware; a false "not confirmed" would show it.
11. **The proxy's follow-up test** now writes what the C line will show:
    it changed eco alone and expected "taken over".
12. **For AP RadioHealth:** its drift (#313) sends a room's state again
    once the members disagree for ten minutes.
    It overlaps its Block 8, an unconfirmed room command sent once more,
    which builds on Block 1.

## The checks

- The Go gate, `make cross`, `go vet` for linux, windows and darwin,
  `gofmt`, after each block.
- `make race` after each block and after the merge.
- `make ui-test`: `test_confirm.py` (8), and `test_rooms.py`,
  `test_log.py`, `test_problem_list.py`, `test_header.py` and
  `test_room_more.py` (228), at 1100 and 390 px in English and German.
- `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`:
  12 checks, all well.
- Nothing was sent to a Cube; nothing of it was seen on hardware.
