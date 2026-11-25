# NOTE RoomsPage v1.0: how the run went

The run of AP RoomsPage v1.0 under the owner's `PLAN++ all A` of 2026-11-25:
seven blocks, each built by an Opus sub-agent in this session's worktree,
read by a second Opus sub-agent before its pull request,
committed, signed and pushed here; none merged by the session.

## The pull requests

| Block | Pull request | State at this NOTE |
|-------|--------------|--------------------|
| 1 Jump | #435 | merged by the owner |
| 2 Holiday row | #436 | merged by the owner |
| 3 Setpoint row | #438 | merged by the owner |
| 4 Labels | #444 | merged by the owner |
| 5 Range | #446 | merged by the owner |
| 6 Dropdown | #450 | open |
| 7 House | #454 | open, on Block 6's branch |

The owner merged with titles of the form `merge(RoomsPage/Block-<n>): …`;
the pull requests from Block 4 on were titled so.

## Where the run went otherwise than the plan

- **W1, the branches.**
  Blocks 1 and 2 went off main as planned.
  Block 3 went off main, Block 4 on it.
  Block 5 went off main rather than on Block 4: it shares no file with it.
  Block 6 went on Block 4 with Block 5's branch merged in,
  Block 7 on Block 6.
- **Block 1.**
  `docs/ui.md` says what sticks out of the row while it is measured is cut off.
  Safari before 16 ignores `overflow-x: clip`; there the fix does nothing,
  and does no harm.
- **Block 2.**
  The test is in `test_room_more.py`, which already puts a room on holiday,
  not in `test_room_marks.py` or `test_rooms.py`.
  The right column's cells carry the class `end`.
  A device value's label is at least 3.2em wide at every width
  (3.6em above 480 px before).
- **Block 3.**
  The columns are sized by their content and spread across the row
  (`minmax(0, auto)`, `space-between`), not the plan's `minmax(0, 1fr)` halves,
  which gave smaller numbers.
  956 px gives three columns of 297 px, not two;
  the fit test covers two columns at 610 px.
  The fit test runs in one case and sets the widths and languages itself.
  A fixed size precedes the `clamp()` for a browser without container units,
  and a user without the setpoint has two columns.
- **Block 4.**
  No new keys `room.isFull` and `room.setFull`:
  the tip shows `room.measured` and `room.setTemp`, the values' accessible names,
  capitalised by CSS.
  The tip's button holds the icon and the short label,
  named by both and the full term (WCAG 2.5.3).
  Escape on an open tip closes the tip alone.
  The setpoint row is a layer of its own only while a tip in it is open.
- **Block 5.**
  The docs and the field's comment say 4.5 is OFF and 30.5 ON within the range.
- **Block 6.**
  OFF is offered in every room and accepted when typed,
  as the chevrons reach it; whether a thermostat whose lowest setpoint
  is above 4.5 takes OFF by radio is not confirmed on hardware.
  A typed ON is refused (D4-A); the chevrons still reach it.
  Only the ▾ is a button, not SET's whole value.
  The value set already is not sent again
  while no write of the room is open.
  The list opens upward where the room below is short,
  never under the sticky header.
  The popup's mechanics went into a shared part, `src/shared/popup.ts`,
  which the tip now uses too; a sign-out closes an open list or tip.
  The tests check the bodies of the writes, held at their routes,
  not the Cube's lines, since the suite lets no write through.
  A guard view was added to `test_parts_guard.py`.
  After AP ToldData came to main, the list's tests tell their data
  through the rooms' read.
- **Block 7.**
  The rows of the lowest and highest stand in columns under 481 px
  and on one line above.
  A typed temperature and a changed date are kept across the choices;
  an end that has passed is forgotten at the next open and refused at Send.
  A hidden field is disabled, so that it never stops Send.
  A stale measured temperature is left out of the lowest and highest
  while a fresh one is there, as the server does within a room.
  `Stepper` and `DateTime` of `src/shared/field.tsx` gained optional props.
- **Verification**, as W3 said:
  the touched UI test files in four cases and the touched Go packages locally,
  the whole suites by CI.

## Found on the way and left

- The reload of the page after a server update still starts at the top;
  `header.js` `somethingOpen()` counts no open list, tip or menu.
- At 320 and 360 px, and with Italian labels in a desktop's 345 px tiles,
  a device row's values still run a little past the row, less than before.
- The House tile's "tomorrow at 06:00" adds 24 hours,
  a day too many on the evening before the spring's change of the clocks;
  offered to the owner as a task of its own.
- The tool buttons are 1.7 px lower than the mode buttons at the bottom
  where the modes' row has one line.
