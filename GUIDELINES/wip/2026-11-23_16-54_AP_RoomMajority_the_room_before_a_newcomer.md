# AP RoomMajority v1.0: The room before a newcomer

## TL;DR

A thermostat put into a room takes the room's state and configuration,
read before it is linked, instead of auto and the first other thermostat's;
the room checks' re-send then keeps that state.
4.5 °C shows as OFF, and the tile marks the eco and comfort temperatures.

- copy's 0x40: auto, temperature 0 → the members' state before the change (mode,
  setpoint, until), still direct, without a Target
- re-send order (RadioHealth.D188): + the state before the change, + the
  member changed last on a tie (D3-A); Massimo's own command first (D4-A)
- copy's configuration: first other thermostat → per radiogram group, what
  most members share (D5-A)
- 4.5 °C → "OFF"; the log "OFF (4.5 °C)" (D6-A)
- tile and device rows: a moon at the eco temperature, a sun at comfort

The room of 2026-11-23 then keeps its manual 5 °C.

### Blocks

1. Order: the re-send order, the tie rule, the held reference (`drift.go`)
2. Copy: the reference sent in the copy; the majority configuration
3. Marks: OFF, and the eco and comfort marks (UI, log page, API)

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

The owner, 2026-11-23:

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
  Else the wall thermostat's, as now.
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
  4. the wall thermostat's state;
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
    The plan proposes the tie rule (D9).
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

Cited, not edited: AP DeviceUnPairingFix,
whose copy of a room's settings (its Blocks 1 and 2, `MoveCommands`,
`CopyCommands`) this changes, and AP RadioHealth,
whose D188 order and Block 7 report (D162-A, third tier) it extends.

### Open decisions

This plan's own decisions start at D7, after the owner's D3 to D6.

- **D7.
  "Changed last" for a member in auto (D3-A).**
  - **A (recommended):**
    for a member in auto, its mode's time only,
    and its setpoint's where it does not follow its programme (a temperature set
    by hand): a switch point moves the setpoint, and nobody wished it.
  - **B:**
    the mode's or the setpoint's time, whichever is later, always:
    a member's switch point at 06:00 beats a hand change at 05:30.
- **D8.
  The copy's reference order.**
  - **A (recommended):**
    the re-send's order over the members before the change,
    Massimo's own command first: one function,
    and the copy and a later re-send cannot send different states.
  - **B:**
    the members' state alone, without Massimo's own command:
    a room command Massimo sent minutes before, shown by one member of three,
    loses in the copy and wins in the re-send.
- **D9.
  The configuration's tie rule (D5-A).**
  - **A (recommended):**
    per group, what most of the other members share,
    on a tie the addressee's (`cubeio.Addressee`), then the first member's:
    the rule the report already uses (`roomReferenceKey`).
    The report then compares the programme per day, as the copy decides it,
    so a device fresh from its copy is never reported.
  - **B:**
    per group, what more than half share,
    else nothing for that group (the repair's rule, RadioHealth D163-A):
    on a tie the device keeps its own, a factory-fresh one its factory values.
  - **C:**
    on a tie the first other thermostat's, as today: arbitrary,
    by the Cube's order.
- **D10.
  The SET label beside the marks.**
  - **A (recommended):**
    the mark beside the setpoint value; SET and its icon stay:
    the label says what the number is, the mark which preset it equals.
  - **B:**
    the mark replaces SET's icon: one icon less,
    but the tile's two values look alike without it.
- **D11.
  Marks in the device rows.**
  - **A (recommended):**
    yes, by each device's own eco and comfort temperatures:
    a member whose configuration differs then shows it.
  - **B:**
    the tile only.
- **D12.
  Comfort equal to eco.**
  - **A (recommended):**
    both marks, sun then moon: the setpoint is both,
    and each mark's label says so.
  - **B:**
    neither: a setpoint that is both looks like neither.
  - **C:**
    the sun alone: arbitrary.
- **D13.
  Where the page gets eco and comfort.**
  - **A (recommended):**
    a thermostat's entry in the device and room lists carries `comfort_temp`
    and `eco_temp` from its C line: two numbers each, no extra request,
    and they follow a configuration change.
  - **B:**
    the tile reads each room thermostat's detail,
    as More's Eco and Comfort do now: a request per room at every load,
    the marks missing until it answers.
- **D14.
  Where OFF shows.**
  - **A (recommended):**
    every text that shows a setpoint: the tile's SET and its stepped value,
    the device rows, More's Eco and Comfort, the tile's "shows …" of a write,
    the House tile's result, the week programme's segments, and the log page,
    as "OFF (4.5 °C)".
    Number fields keep 4.5 (the holiday form, the programme's fields,
    the configuration), as do the API,
    the process log's key-values and the command line.
  - **B:**
    the three places named, the tile,
    the device rows and More's Eco and Comfort, and the log page.
  - **C:**
    A, and the process log's `setpoint` as "OFF (4.5 °C)":
    a text where tools read a number.
- **D15.
  The repair Place.**
  It puts a device into the room its list names (`buildPlace`) and copies
  nothing.
  - **A (recommended):**
    it holds the room's reference too, for the re-send:
    a placed device brings its state as an added one does.
  - **B:**
    `PUT …/devices/{rf}` only.
- **D16.
  A copy whose 0x40 is discarded** (`mode_not_set`; in a move *unverified*,
  docs/cube-protocol.md).
  - **A (recommended):**
    the copy then sends the reference once as a room command,
    as the re-send does, which a moved thermostat took within 11 s on hardware;
    with no reference, nothing, as now.
  - **B:**
    as now: the answer says so,
    and the room checks re-send only where the members still differ,
    not where the newcomer's state reached them all.

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
  The strictly newest known time wins; none, or two equal,
  fall to the wall thermostat's.
- **A tie** is no state shown by more than half, a split three ways included.
- **The held reference:**
  the request computes the reference over the room's members before the change,
  by the re-send's order (D8),
  and keeps it for the room in the room checks' state, with its time.
  The re-send's second step takes it in place of the all-members majority.
  It is dropped at the room's first check after the change that finds the
  members agreeing, on any state; 24 h after the request (`resendGap`);
  and by a later structure change of the room, which holds its own.
  The window it covers: the read-back,
  then 10 minutes to the Warn and 10 to the re-send.
  A person's change by hand at a device in that window that does not reach its
  partners is overridden once.
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
  D9-A takes the report's rule into `cubeio` per group and per day,
  and the report calls it; the repair stays as it is.
  Only radiator thermostats store all of 0x12 and the window-open minutes,
  so those come from the radiator thermostats among the others, else from a
  wall thermostat's boost settings and `RoomRadiator`'s defaults, as now.
- **The copy's 0x11 to every member** stays, the original software's step:
  with D9-A's values,
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
- **The UI** is AP UiParts':
  the tile and the device rows are parts (`src/rooms/Tile.tsx`,
  `src/rooms/Devices.tsx`),
  the week programme editor is plain (`week-program.js`).

## Implementation Plan

A sub-agent on Opus builds each block without a commit,
a second reader reads it, and the session verifies it and makes the commit;
the three commits are one pull request.
Tests run against the fake Cube and check the exact lines sent;
identifiers are placeholders (`KEQ0000000`, `0a0101`).
Nothing goes to a real Cube without the owner's word.

### Block 1: Order

- `internal/server/drift.go`:
  - `referenceState(members, intent, now, cubeNow, clock)`:
    the re-send's order over the given members, Massimo's intent,
    what more than half show, the newest change (D3-A, D7),
    the wall thermostat's, none; `intended()` calls it.
  - The tie rule from `snap.Times`, a time equal to `snap.Since()` none;
    a member in auto by D7.
  - The held reference per room: `holdReference(room, state, at)`,
    taken by the second step in place of the all-members majority,
    dropped as the Analysis says.
  - `from` names the rule: `massimo`, `majority`, `newest` (new),
    `wall_thermostat`; `before_change: true` marks a held reference.
- The log page names the new `from` and `before_change` (`src/log/text.ts`);
  its texts in all six languages (FR/IT/NL/PL machine-translated and marked).
- Tests (`drift_test.go`), each with the re-send's exact line:
  a held manual 5 °C over a room of two that differ;
  two that tie, the newest change wins; a switch point is no change (D7-A);
  times unknown, the wall thermostat's; Massimo's command still first;
  the reference dropped at agreement and after 24 h.
- Docs: `docs/api-v1.md`, the room checks' numbered list (RadioHealth.D188
  as changed) and the log's `from`; the OpenAPI file's `disagree` summary.
- A `Fixed` CHANGELOG fragment.

**Verification:**
each test fails with its rule taken out; `go test ./internal/server`;
`go test -race -count=1 ./internal/server`; `gofmt -l .`,
`go vet` for the three systems; `make ui-check`.

### Block 2: Copy

- `internal/server/config_api.go`, `updateDevice`: after the structure lock,
  before `addDevice` or `moveDevice`,
  the target room's reference (Block 1's `referenceState` over the room's scan);
  passed to the copy and held for the room checks.
  The repair Place holds it too (D15).
- `internal/cubeio/pairing.go`:
  - `CopyCommands` takes the state: the 0x40 carries the reference's mode,
    setpoint and until, to the device directly, without a Target; auto
    with 0 where the reference is auto on the programme, or there is none.
  - `roomReference` → `RoomConfig` (D9): per group, the temperatures,
    the valve settings, each day;
    window-open minutes and 0x12 from the radiator thermostats as the Analysis
    says; each device's own offset.
  - D16: after a discarded 0x40, the reference once as a room command,
    through the queue, after the programme.
- `internal/server/congruence.go`:
  the report takes `RoomConfig`'s rule, the programme per day (D9-A).
- Tests (`config_api_test.go`, `cubeio_test.go`), with the exact lines:
  the 2026-11-23 case,
  a room of one radiator thermostat in manual 5 °C and an added wall thermostat:
  its 0x40 is manual 5 °C, direct;
  the fake Cube then shows the radiator in auto and the re-send is manual 5 °C
  as a room command, `before_change`;
  the reference read before the links (the fake Cube changes the old member as
  it takes the link half, the copy still sends the earlier state);
  auto on the programme, no other thermostat, a holiday;
  the configuration per group and per day, a tie to the addressee's;
  a discarded 0x40 (D16); the report after a copy finds nothing.
- Docs: `docs/api-v1.md`, "PUT /api/v1/cubes/{sn}/devices/{rf}" (the copy
  paragraph) and the report's room reference; `docs/cube-protocol.md`,
  "Pairing a device, as the original software does it" (Massimo's copy,
  where it differs from the original's now).
- A `Fixed` CHANGELOG fragment.

**Verification:**
each test fails on a fault planted for it (auto in the 0x40,
the reference read after the links, the first other thermostat's configuration,
a Target on the 0x40); `go test ./internal/cubeio ./internal/server`;
`-race` for `internal/server`; `gofmt`, `go vet` for the three systems.

### Block 3: Marks

- The API (D13): a thermostat's entry carries `comfort_temp` and `eco_temp` from
  its C line (`internal/view`); `docs/api-v1.md`, the OpenAPI file,
  `api.d.ts` (`make api-types`); a test of the entry.
- `src/shared/`:
  `fmtSetpoint`, "OFF" for 4.5 °C, and the preset mark,
  each with its row in `docs/ui.md` ("How the page is built");
  `app.js` the same formatter for the plain scripts.
- OFF where D14 says; the log page "OFF (4.5 °C)".
- `index.html`:
  `icon-eco`, a crescent moon, and `icon-comfort`, a sun with rays,
  each mark with its label.
- The tile: the mark beside the shown value, the stepped one included, by
  the room thermostat's eco and comfort, as More's are (D10, D12); the device
  rows by each device's own (D11).
- Texts in all six languages (FR/IT/NL/PL machine-translated and marked);
  "OFF" as the displays show it.
- Tests: UI tests of the tile and a device row at OFF, at eco, at comfort,
  at comfort equal to eco, at 1100 and 390 px, in light and dark; the log
  page's "OFF (4.5 °C)".
- Docs: `docs/ui.md`, the tile's SET and the device rows.
- An `Added` CHANGELOG fragment.

**Verification:**
`make ui-check`; the changed UI test files, as `make ui-test` runs them;
`go test ./internal/server ./internal/view`; `TestOpenAPICoversRoutes`.

### The pull request

The three commits on one branch, `main` merged in before it;
the guidelines check over the range; the pull request's run green.
The owner merges it.

## Proposed commit messages

- The plan: `chore(aps): +RoomMajority v1.0`
- Block 1: `fix(roomcheck): A re-send keeps the room's state from before a newcomer`
- Block 2:
  `fix(copy): A thermostat put into a room takes the room's state and configuration`
- Block 3:
  `feat(ui/rooms): OFF for 4.5 °C, and marks for the eco and comfort temperatures`

## Change History

- v1.0 (2026-11-23): first revision, from the owner's account of the
  2026-11-23 case and decisions D3-A to D6-A; D7 to D16 open.
