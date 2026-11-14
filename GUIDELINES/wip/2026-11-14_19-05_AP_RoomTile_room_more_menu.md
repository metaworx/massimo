# AP RoomTile v1.0: a room's More menu, and a transmit error that settles

## TL;DR

A room tile's **Holiday** button becomes **More**, a menu with Eco,
Comfort and Holiday, and a holiday shows its end. A transmit error the
device has reported since is reset, so a red error means a device not
heard from.

- Holiday button → **More ▾**: Eco, Comfort (the room's eco or comfort
  temperature, the mode kept), a **Hold (Manual)** switch while the room
  is in Auto, Holiday… (its form, as now)
- a room on holiday → More shows "Holiday until 24.12., 18:00", the
  room's pill too
- a device in error that reports again → Massimo sends `r:` for it,
  logged at Info (D102-B, after the hardware investigation)

### Blocks

1. MoreMenu: the menu, Eco and Comfort in the room's mode, Hold, Holiday;
   tests
2. HolidayShown: the holiday's end on More and in the room's pill; tests
3. ErrorReset: `r:` for a device in error that reports again; tests;
   `docs/cube-protocol.md` (runs once D102 is settled)
4. Docs: `docs/ui.md`, the six languages

Blocks 1, 2 and 4 are one pull request; Block 3 another, once the
investigation's evaluation settles D102.

## Discussion

The owner's points of 2026-11-14 evening:

- "in the room tile, make Holiday a dropdown More, showing Holiday, Eco,
  Comfort";
- D99: Eco and Comfort set the current setpoint to the room's eco or
  comfort temperature and do not change the mode, so in Auto the next
  switch point undoes it; rather than four entries, keep the mode and
  offer a switch to Manual while the room is in Auto; a room on holiday
  shows the holiday with its end date;
- "When exactly is there a transmit error?": answered in the evaluation;
  a hardware test on 2026-11-14 showed a device's reports never clear
  the flag, and `r:` does at no radio cost; D102-B recommended, a session
  of its own investigating further on the hardware.

`PLAN+ as recommended` on 2026-11-14.

## Analysis

**The tile's modes** (`tiles.js`): Auto, Manu and Holiday buttons in the
tile's foot, Holiday opening the room's holiday form (`roomUI(room)
.vacation`). The house card's Eco and Comfort (`houseMode`, `sendHouse`)
read each room thermostat's `eco_temp` and `comfort_temp` from
`deviceDetail` and send the room's setpoint.

**The API** takes a temperature in Auto: `{"mode":"auto","temp":…}` holds
it until the next switch point (`docs/api-v1.md`, the modes' table);
Manual holds it; holiday takes `until`.

**The transmit error.** The queue resets it before a command to the
device (`internal/engine/queue.go`). A device that reports again shows a
changed entry in the device list; the server sees it as a state event
(`cubelan.EventState`) with the flag still set. One `r:` per such report
costs no memory slot (one sample, 2026-11-14).

## Implementation Plan

### Block 1: MoreMenu

- `tiles.js`: the Holiday button becomes a `dropdown()` "More": Eco,
  Comfort (the room thermostat's temperature from `deviceDetail`, sent
  in the room's current mode: Auto with `temp`, Manual with `temp`),
  **Hold (Manual)** a switch shown while the room is in Auto (on, Eco and
  Comfort send Manual), Holiday… (the form, as now).
- Keyboard: the menu's items by the arrow keys, Escape closes.
- Tests: Eco in Auto sends `{"mode":"auto","temp":<eco>}`, with Hold
  `{"mode":"manual",…}`; in Manual Eco keeps Manual; Holiday… opens the
  form.

**Verification:** `make ui-test` (`test_rooms.py`, the send tests).

### Block 2: HolidayShown

- A room on holiday: More's label "Holiday until <date, time>", marked;
  the room's pill (AP Signs) lists the holiday with its end.
- Tests at 1100 and 390 px.

**Verification:** `make ui-test`.

### Block 3: ErrorReset (after D102 is settled)

- `internal/server`: on a state event of a device whose flag is set and
  whose entry changed in anything else, a queued `r:` for it, at most
  once per report; Info "transmit error reset, the device has reported".
- Tests against the fake Cube: the reset sent once, not for a device
  that has not reported; the line exact (`cube.CmdResetError`).
- `docs/cube-protocol.md`: Massimo's behaviour.

**Verification:** `go test ./...`, `make race`.

### Block 4: Docs

- `docs/ui.md` "Rooms"; i18n, six languages; fragments.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(ui): A room's More menu with Eco, Comfort and Holiday`;
Block 2, `feat(ui): A room on holiday shows until when`;
Block 3, `feat(server): A device in error that reports again is reset`;
Block 4, `docs(ui): A room's More menu`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.
