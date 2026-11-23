# AP RoomMajority v1.1: The room before a newcomer

## TL;DR

A thermostat put into a room takes the state and configuration its members
showed before it was linked, and the room checks' re-send keeps that state.
One order speaks for a room, the dead passed over.
4.5 °C shows as OFF; the tile marks eco and comfort.

- copy's 0x40: auto, 0 → the members' state before the change, still direct
- one order (D9): dead passed over → majority → master → wall → radiators
- re-send (RadioHealth.D188): + the state before the change,
  + the newest change on a tie (D3-A, D7-C)
- copy's configuration: first other thermostat → per group and day, by the order
- 4.5 °C → "OFF"; the log "OFF (4.5 °C)"
- tile: moon at eco, sun at comfort, both at both;
  the members' setpoints while they differ

The room of 2026-11-23 then keeps its manual 5 °C.

### Blocks

1. Order: one order for a room's addressee and reference; the re-send (`drift.go`)
2. Copy: the reference state in the copy; the configuration by the order
3. Marks: OFF, the eco and comfort marks, the members' setpoints (UI, log, API)

## Discussion

### What the owner asked for

On 2026-11-23, Cube firmware 1.1.3:
a room held one radiator thermostat in manual 5 °C, set by hand at the device.
A wall thermostat was put into the room (`PUT …/devices/{rf}` with `room_id`).
The copy sent it 0x40 auto with temperature 0,
as the original software does (`internal/cubeio/pairing.go`, `CopyCommands`);
the wall thermostat passed its setpoint to the radiator within a minute;
the room checks reported the disagreement after about 11 minutes and re-sent the
wall thermostat's auto about 10 minutes later,
since RadioHealth.D188 has no majority in a room of two (`intended()`,
`majority()`).
The room went from manual 5 °C to its programme, which nobody wanted.
The owner asked for one plan of three blocks, named RoomMajority.

### What is decided

The owner, 2026-11-23, with the task:

- **The reference state.**
  After an add or a move,
  the state that wins is what most of the room's members show from before the
  change.
  It is read when the request arrives, before the new device is linked,
  so that the new device's factory state (manual 20 °C) cannot reach the others
  first.
- **D3-A.
  A tie.**
  Where those members tie,
  the state of the member whose mode or setpoint changed last:
  a change made by hand at a device is the newest wish.
  Else the wall thermostat's, as now (since v1.1 D9's order).
  Else none.
- **D4-A.
  Massimo's own command.**
  Massimo's own room command from the last 15 minutes stays first in the re-send
  order (`intent`, `intentLead`).
- **The re-send order**
  (RadioHealth.D188) becomes:
  1. Massimo's own command, as D4-A says;
  2. what most of the members show, of those before the structure change
     where one brought a newcomer, otherwise of all members;
  3. the tie rule of D3-A;
  4. the wall thermostat's state (since v1.1 D9's order: the master's,
     the wall thermostat's, the radiator thermostats');
  5. none.
- **Outliers take the others' values.**
  Every member that differs, the new device included, gets the others' values,
  but for its own temperature offset and its display mode (`display_actual`).
  - **State:**
    the new device gets the reference state (mode, setpoint,
    until) in the copy instead of auto;
    auto with temperature 0 only where the reference is auto on the programme,
    or the room has no other thermostat or no reference.
    That 0x40 still goes to the device directly, without a Target:
    the Cube discarded a 0x40 with a Target to a device in a room on 2026-11-04.
    Other members that differ get the reference state through the room checks'
    re-send.
  - **D5-A, configuration:**
    a new device gets the majority configuration, decided per radiogram group:
    the temperatures (0x11),
    the valve settings (0x12) and each day of the week programme (0x10).
    An existing member whose configuration differs is reported,
    and the existing repair re-sends it on a click;
    nothing is re-sent automatically,
    since a programme alone is 7 radiograms per device.
- **D6-A, modified.
  OFF.**
  4.5 °C is shown as "OFF", as the devices' displays show it,
  wherever a setpoint is shown: room tiles, device rows, setpoint choosers.
  The log writes "OFF (4.5 °C)".
  No "ON" for 30.5 °C.
- **Preset marks.**
  In the room tile, a crescent moon where the setpoint is the room's eco
  temperature, a sun with rays where it is the comfort temperature,
  the eQ-3 devices' own symbols.
  Icons are SVG `<symbol>`s in `internal/server/ui/index.html`;
  every icon has a text label, every text is in all six `ui/i18n/<lang>.json`.

The owner, at v1.0's gate (2026-11-23):

- **D7-C.
  "Changed last" (D3-A) and the programme.**
  In auto a setpoint set by hand holds only until the programme's next switch
  point, so a switch point at 06:00 beats a hand change made in auto at 05:30;
  in manual or on holiday the programme changes nothing,
  and the hand setpoint counts.
  So a member that follows its programme counts its setpoint's time
  only against members in auto:
  where any of the tied members is in manual or on holiday,
  such a member counts its mode's time alone.
  A member in auto with a setpoint set by hand counts both times.
- **D8-A.
  The copy's reference order:**
  the re-send's order over the members before the change,
  Massimo's own command first: one function,
  so that the copy and a later re-send cannot send different states.
- **D9, the owner's.
  One order wherever a member stands for its room.**
  It is the same in every place,
  and replaces v1.0's D9 options, D3-A's and RadioHealth.D188's last step
  ("the wall thermostat's"), and the order of DeviceUnPairingFix.D167,
  which DeviceUnPairingFix.D169-A made one function:
  1. a dead member is passed over at every step:
     one the Cube holds no valid state for;
  2. what most of the members share, where there is something to share
     (a state, a group of the configuration, a day of the programme);
  3. the room's master, where it is a thermostat of the room,
     a radiator or a wall thermostat;
  4. the room's wall thermostat;
  5. its first radiator thermostat, then each other one, in the Cube's order.

  For a state, Massimo's own command comes first (D4-A),
  and the newest change (D3-A, D7-C) between steps 2 and 3.
  The addressee has no step 2;
  where every thermostat of the room is dead,
  it is taken by steps 3 to 5 without step 1,
  since a radiogram with the room flag must name one.
  A reference where every member is dead is none.
- **D10-A.
  The SET label beside the marks:**
  the mark beside the setpoint value; SET and its icon stay.
- **D11-A.
  Marks in the device rows,**
  by each device's own eco and comfort temperatures.
- **D12-D.
  Comfort equal to eco:**
  one icon of its own, a moon beside a sun, as the devices' display shows it.
- **D13-A.
  Where the page gets eco and comfort:**
  a thermostat's entry in the device and room lists carries `comfort_temp`
  and `eco_temp` from its C line.
- **D14-A.
  Where OFF shows:**
  every text that shows a setpoint: the tile's SET and its stepped value,
  the device rows, More's Eco and Comfort, the tile's "shows …" of a write,
  the House tile's result, the week programme's segments,
  the members' setpoints (D17-A), and the log page, as "OFF (4.5 °C)".
  Number fields keep 4.5 (the holiday form, the programme's fields,
  the configuration), as do the API,
  the process log's key-values and the command line.
- **D15-A.
  The repair Place**
  (`buildPlace`, which puts a device into the room its list names and copies
  nothing) holds the room's reference too, for the re-send:
  a placed device brings its state as an added one does.
- **D16-A.
  A copy whose 0x40 is discarded** (`mode_not_set`)
  then sends the reference once as a room command, as the re-send does;
  with no reference, nothing, as now.
- **D17-A.
  The members' setpoints in the tile.**
  While the room's thermostats show different setpoints,
  a line under the setpoint row names each with its setpoint and mark,
  and goes once they agree.
  It is not shown while a write to the room waits for its confirmation,
  so that a room command reaching the devices one by one does not flicker.
  A disagreement such as 2026-11-23's then shows at once,
  not after the room checks' 10 minutes.
- **D18, from the owner's three places.
  The mark in a device row:**
  a mark at the row's end would need its room in every row.
  Of before the value, after the Set label, or in place of Set's icon,
  the plan takes the last:
  the mark replaces Set's gauge icon while the setpoint is the device's eco or
  comfort temperature, the gauge otherwise.
  It costs no width in any of the six languages,
  where a mark after the label would move the value in French, Italian and
  Dutch, whose labels already pass the label's width,
  and one before the value would move its digits off Actual's.

Cited, not edited: AP DeviceUnPairingFix,
whose copy of a room's settings (its Blocks 1 and 2, `MoveCommands`,
`CopyCommands`) and addressee (D167, D169-A) this changes, and AP RadioHealth,
whose D188 order, D163-A repair and Block 7 report (D162-A, third tier) it
extends.

## Analysis

- **The re-send** (`internal/server/drift.go`, RadioHealth Block 9):
  `intended()` takes Massimo's intent within `intentLead` while a member shows
  it, then `majority()` (more than half,
  members on their own programmes as one state),
  then the wall thermostat's (`memberResend`).
  In a room of two that differ there is never more than half, so the wall
  thermostat's state wins; on 2026-11-23 that was the newcomer's.
  The room checks are paused while a structure change concerns the room
  (`structureRooms`) and resume after its read-back (2, 6, 12 min).
- **State times** for D3-A are in the snapshot: `snap.Times(rf).Fields`,
  per value when it appeared; a time equal to `snap.Since()` means the value
  was there at the first connection, its age unknown, and counts as no time.
  The strictly newest known time wins, as D7-C counts it;
  none, or two equal, fall to D9's steps 3 to 5.
- **A tie** is no state shown by more than half, a split three ways included.
- **The held reference:**
  the request computes the reference over the room's members before the change,
  by the re-send's order (D8-A),
  and keeps it for the room in the room checks' state, with its time.
  The re-send's second step takes it in place of the all-members majority.
  It is dropped at the room's first check after the change that finds the
  members agreeing, on any state; 24 h after the request (`resendGap`);
  and by a later structure change of the room, which holds its own.
  The window it covers: the read-back,
  then 10 minutes to the Warn and 10 to the re-send.
  A person's change by hand at a device in that window that does not reach its
  partners is overridden once.
- **Dead** (D9): the L entry's valid bit (0x10) is clear:
  the Cube has had nothing from the device since its restart,
  or marks the entry out of date after the device fell silent.
  The room checks' `scanRoom` passes over such members already,
  and `view.Silent` lists them;
  the communication error and the link error do not make a device dead,
  since it answers, and `r:` clears the first.
  The virtual wall thermostat is never one of the Cube's devices,
  so no step can pick it.
- **The order today:**
  `cubeio.Addressee` (DeviceUnPairingFix.D167, D169-A):
  the master where it is a thermostat member, else the wall thermostat,
  else the first radiator thermostat; nothing passes over a dead one.
  Its callers: `RoomAddressee` (the room commands of `server.go` and
  `config_api.go`, the re-send of `drift.go`), `MoveCommands` (moves,
  removals, a placement's link half), `MasterCandidate` (the repair of
  a room's master) and the report's tie (`congruence.go`).
  The re-send's last step is the wall thermostat's state, not the master's.
  The repair's `RoomAgreement` refuses on a tie (`NoMajorityError`) and
  compares whole weeks.
  Since the Cube discards room commands while a room has a member without
  state (`view.Silent`), a dead addressee matters most to the radiograms
  that name one directly: the links of a move, a removal, a placement.
- **The request** (`internal/server/config_api.go`, `updateDevice`): the
  structure lock is taken, then `addDevice` or `moveDevice` links the
  device (`cubeio.MoveCommands`) and writes the metadata; the copy runs
  after the lock (`copySettings` → `cubeio.Writer.CopySettings`).
  The reference is read after the lock and before the links.
  The device is not in the room's C lines yet at that point (an add:
  room 0 or another; a move: its old room),
  so the scan of the room is of the members before the change.
- **The copy** (`CopyCommands`): 0x40 auto with 0 to the device directly;
  after a wakeup,
  0x11 to every thermostat of the room with `roomReference`'s temperatures;
  0x12 and the differing days of 0x10, then 0x11, to the device.
  `roomReference` is the room's first other thermostat,
  a wall thermostat filled up with `RoomRadiator`'s defaults.
- **The configuration's majority** exists twice already:
  `cubeio.RoomAgreement` (repair, more than half or nothing,
  whole weeks) and `roomReferenceKey` (report, most members, then the addressee,
  whole weeks; the programme and the five temperatures,
  not the window-open minutes, not the valve).
  D9 makes one rule of them in `cubeio`, per group and per day,
  which the copy, the report and the repair call.
  The repair then re-sends the days that differ from the room's,
  as the copy does,
  and refuses only where the room has no other thermostat that is not dead.
  Only radiator thermostats store all of 0x12 and the window-open minutes,
  so those come from the radiator thermostats among the others, else from a
  wall thermostat's boost settings and `RoomRadiator`'s defaults, as now.
- **The copy's 0x11 to every member** stays, the original software's step:
  with D9's values,
  an existing member whose temperatures differ takes the room's there.
  The 0x12 and the days go to the new device only.
- **OFF:**
  `cube.TempOff` is 4.5 °C.
  The UI formats temperatures with `fmtTemp` (`app.js`, plain) and `fmtNum`;
  parts and plain scripts both use them.
  The log page writes setpoints in `src/log/text.ts` (`log.set`);
  the process log (`eventline.go`) prints summary values as key-values.
- **The marks:**
  the tile's More menu takes eco and comfort from the room thermostat
  (`roomThermostat`, `presetTemps`, a detail request);
  the page's device entries carry no configuration.
  No moon or sun symbol is in `index.html` yet.
  A device row's cell is a flex of label, value, unit and icon
  (`Cell`, `Devices.tsx`), aligned by minimum widths, not by a grid.
- **The UI** is AP UiParts':
  the tile and the device rows are parts (`src/rooms/Tile.tsx`,
  `src/rooms/Devices.tsx`),
  the week programme editor is plain (`week-program.js`).

## Implementation Plan

A sub-agent on Opus builds each block in this session's worktree without a
commit, a second reader reads it, and the session verifies it and makes the
commit; the three commits are one pull request.
Tests run against the fake Cube and check the exact lines sent;
identifiers are placeholders (`KEQ0000000`, `0a0101`).
Nothing goes to a real Cube without the owner's word.
Locally only the touched packages and UI test files run;
CI runs the whole suite, the race job and the cross builds.

### Block 1: Order

- `internal/cubeio`: D9's order, one function (`RoomOrder`):
  the room's thermostats, the dead passed over,
  in the order master, wall thermostat, radiator thermostats.
  `Addressee` and `RoomAddressee` take its first,
  else, every thermostat dead, its first without step 1;
  `MasterCandidate` takes it with the master left aside.
  Every caller of `Addressee` thereby passes over a dead addressee.
- `internal/server/drift.go`:
  - `referenceState(members, intent, now, cubeNow, clock)`:
    Massimo's intent, what more than half show, the newest change (D3-A,
    D7-C), then D9's steps 3 to 5, the state of the first that has one,
    none; `intended()` calls it.
  - The tie rule from `snap.Times`, a time equal to `snap.Since()` none,
    a member that follows its programme by D7-C.
  - The held reference per room: `holdReference(room, state, at)`,
    taken by the second step in place of the all-members majority,
    dropped as the Analysis says.
  - `from` names the rule: `massimo`, `majority`, `newest` (new),
    `master` (new), `wall_thermostat`, `radiator_thermostat` (new);
    `before_change: true` marks a held reference.
- The log page names the new `from` and `before_change` (`src/log/text.ts`);
  its texts in all six languages (FR/IT/NL/PL machine-translated and marked).
- Tests, each with the exact line sent:
  `cubeio` the order (a dead master, a dead wall thermostat, every thermostat
  dead, the master a window contact);
  a move's link half to a dead wall thermostat's room names the radiator;
  `drift_test.go`: a held manual 5 °C over a room of two that differ;
  two that tie, the newest change wins;
  a switch point beats a hand change in auto, not one in manual (D7-C);
  times unknown, the master's; a dead member passed over;
  Massimo's command still first; the reference dropped at agreement and after
  24 h.
- Docs: `docs/api-v1.md`, the room checks' numbered list (RadioHealth.D188
  as changed), the log's `from`, and every place that names the addressee's
  order; the OpenAPI file's `disagree` summary;
  `docs/cube-protocol.md`, "Set temperature (0x40) addressing", the order.
- A `Fixed` CHANGELOG fragment.

**Verification:**
each test fails with its rule taken out; `go test ./internal/cubeio
./internal/server`; `gofmt -l .`, `go vet` for the three systems;
`make ui-check`.

### Block 2: Copy

- `internal/server/config_api.go`, `updateDevice`: after the structure lock,
  before `addDevice` or `moveDevice`,
  the target room's reference (Block 1's `referenceState` over the room's scan);
  passed to the copy and held for the room checks.
  The repair Place holds it too (D15-A).
- `internal/cubeio/pairing.go`:
  - `CopyCommands` takes the state: the 0x40 carries the reference's mode,
    setpoint and until, to the device directly, without a Target; auto
    with 0 where the reference is auto on the programme, or there is none.
  - `roomReference` → `RoomConfig` (D9): per group, the temperatures,
    the valve settings, each day, what most of the other members not dead
    share, else Block 1's order;
    window-open minutes and 0x12 from the radiator thermostats as the Analysis
    says; each device's own offset.
  - D16-A: after a discarded 0x40, the reference once as a room command,
    through the queue, after the programme.
- `internal/server/congruence.go`:
  the report takes `RoomConfig`'s rule, the programme per day.
- `internal/cubeio/repair.go`: `RoomAgreement` takes `RoomConfig`'s rule;
  `ResendCommands` re-sends the temperatures and the days that differ from
  it; `NoMajorityError` only for a room without another thermostat that is
  not dead; the repair's texts follow (`repair_api.go`, the UI's).
- Tests (`config_api_test.go`, `cubeio_test.go`, the repair's), with the
  exact lines:
  the 2026-11-23 case,
  a room of one radiator thermostat in manual 5 °C and an added wall thermostat:
  its 0x40 is manual 5 °C, direct;
  the fake Cube then shows the radiator in auto and the re-send is manual 5 °C
  as a room command, `before_change`;
  the reference read before the links (the fake Cube changes the old member as
  it takes the link half, the copy still sends the earlier state);
  auto on the programme, no other thermostat, a holiday;
  the configuration per group and per day, a tie to the master's, a dead
  member passed over; a discarded 0x40 (D16-A); a placement (D15-A);
  the report after a copy finds nothing; the repair on a tie.
- Docs: `docs/api-v1.md`, "PUT /api/v1/cubes/{sn}/devices/{rf}" (the copy
  paragraph), the report's room reference and the repair's re-send;
  `docs/cube-protocol.md`,
  "Pairing a device, as the original software does it" (Massimo's copy,
  where it differs from the original's now).
- A `Fixed` CHANGELOG fragment.

**Verification:**
each test fails on a fault planted for it (auto in the 0x40,
the reference read after the links, the first other thermostat's configuration,
a Target on the 0x40, a tie refused); `go test ./internal/cubeio
./internal/server`; `gofmt`, `go vet` for the three systems; `make ui-check`
where the repair's texts changed.

### Block 3: Marks

- The API (D13-A): a thermostat's entry carries `comfort_temp` and `eco_temp`
  from its C line (`internal/view`); `docs/api-v1.md`, the OpenAPI file,
  `api.d.ts` (`make api-types`); a test of the entry.
- `src/shared/`:
  `fmtSetpoint`, "OFF" for 4.5 °C, and the preset mark,
  each with its row in `docs/ui.md` ("How the page is built");
  `app.js` the same formatter for the plain scripts.
- OFF where D14-A says; the log page "OFF (4.5 °C)".
- `index.html`:
  `icon-eco`, a crescent moon, `icon-comfort`, a sun with rays,
  and `icon-eco-comfort`, the two side by side (D12-D),
  each mark with its label.
- The tile: the mark beside the shown value, the stepped one included,
  in the accent colour, by the room thermostat's eco and comfort,
  as More's are (D10-A); the device rows by each device's own,
  in place of Set's gauge (D11-A, D18).
- The members' setpoints (D17-A): a line under the tile's setpoint row,
  each thermostat's name, setpoint and mark, while they differ and no write
  of the tile waits for its confirmation.
- Texts in all six languages (FR/IT/NL/PL machine-translated and marked);
  "OFF" as the displays show it.
- Tests: UI tests of the tile and a device row at OFF, at eco, at comfort,
  at comfort equal to eco; the members' line while they differ, not while a
  write waits, gone once they agree; at 1100 and 390 px,
  in light and dark; the log page's "OFF (4.5 °C)".
- Docs: `docs/ui.md`, the tile's SET, the members' line and the device rows.
- An `Added` CHANGELOG fragment.

**Verification:**
`make ui-check`; the changed UI test files, as `make ui-test` runs them;
`go test ./internal/server ./internal/view`; `TestOpenAPICoversRoutes`.

### The pull request

The three commits on one branch, `main` merged in before it;
the guidelines check over the range; the pull request's run green.
The owner merges it.

## Proposed commit messages

- The plan: `chore(aps): +RoomMajority v1.0`, then `chore(aps): +RoomMajority v1.1`
- Block 1:
  `fix(rooms): One order speaks for a room, and a re-send keeps its state from before a newcomer`
- Block 2:
  `fix(copy): A thermostat put into a room takes the room's state and configuration`
- Block 3:
  `feat(ui/rooms): OFF for 4.5 °C, marks for eco and comfort, and the setpoints of members that differ`

## Change History

- v1.0 (2026-11-23): first revision, from the owner's account of the
  2026-11-23 case and decisions D3-A to D6-A; D7 to D16 open.
- v1.1 (2026-11-23): the owner's answers at v1.0's gate, by PLAN++:
  D7-C, D8-A, D9 as the owner's one order (the dead passed over, then the
  majority, the master, the wall thermostat, the radiator thermostats) in every
  place, the addressee and the repair included, D10-A, D11-A, D12-D, D13-A,
  D14-A, D15-A, D16-A, D17-A (Block 3), and D18 from the owner's three places;
  no decision open.
