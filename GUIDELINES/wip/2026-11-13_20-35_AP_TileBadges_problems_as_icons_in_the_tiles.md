# AP TileBadges v1.0: problems as icons in the tiles

## TL;DR

A room's problems show as icons in its tile's head, and a tap lists them
by device. One component in `components.js` does this for every place
that has problems to show.

- `problemsEl(entries, label)` → one icon per kind present, each in its
  colour; a dropdown of the entries, each with its badges
- room tile head: radio fault, battery low, window open, mixed modes →
  icons; "Open" and the "mixed" note → into it
- device head: a type icon before the name; its problems as icons with
  their own dropdown; the mode stays a word
- the virtual thermostat → the house with a thermometer, no list entry
  (its flags are the wall thermostat's)
- the values' times: the age under the name, the time under the value
- "Temperature from" → serials
- a wall thermostat's 0 °C → "not reported"
- icons: Tabler's leaf and shuffle, ours after the thermostat's display

### Blocks

1. Problems: `problemsEl`, the icons, the room tile's and the device's
   heads, the type icons; Tabler's notice; tests; `docs/ui.md`
2. Values: the times' layout, "Temperature from" by serial; tests
3. WallZero: a wall thermostat's 0 is not reported; the test;
   `docs/cube-protocol.md`

The three blocks are one pull request, after AP HeaderPages'.

## Discussion

The owner's `EVAL` of the room tile on 2026-11-13, with screenshots of
a room whose thermostats were in different modes, a wall thermostat with
a transmit error, and its virtual thermostat:

- "mixed" a badge, and in the tile's head; an icon in the same dropdown
  (D41: the shuffle);
- the transmit error, the battery and the window in the tile's head, as
  icons, any of them opening a dropdown like the Cube choice's (D42-C),
  one entry per device with its badges (D46-A);
- the same in a device's head, where long names leave no room for words
  (D48-B);
- an icon before every device name, the virtual one a room's (D51-A),
  in place of a "Virtual" badge;
- the times in the device's values under the wrong value: the age under
  the name, the date and time under the value, on one line;
- "Temperature from" names the radio address, the serial is wanted;
- the room's 0.0 °C: the wall thermostat had new batteries and reported
  0 until it measured (D49-A);
- the icon set (D50): Tabler where it fits, our own drawings where the
  thermostat's display has a symbol (the open window, the radio, the
  battery), the picks as recommended;
- the dropdown a general component, placed in a card's or a section's
  head, its icons optionally coloured (D52);
- all of it in this plan, the Windhager Webserver session (AP RoomApi,
  whose Block 5 built the virtual thermostat's view) told which
  functions change before the first commit (D44-C).

`PLAN++` with the recommended picks on 2026-11-13.
The icons were compared on a page for the owner, its drafts the ones
taken here.

## Analysis

**The room tile** (`partials/room.html`, `tiles.js`).
Its head holds the name, `.window-open` (`renderWindowOpen`: "Open"
with the latest opening's age, the contacts in its tooltip) and the ⋯
menu. `renderSetpoint` marks mixed modes on the mode buttons (dashed,
`aria-pressed="mixed"`) and fills `.mode-mixed` with "mixed" and the
devices' modes in its tooltip. The devices' warnings are in the ⓘ panel
only: `deviceBadges` lists them for `renderDevice` (not reported, stale,
battery low, transmit error, link error, locked, then the window state
or the mode as a filled badge).

**The virtual thermostat** (`internal/view/vwt.go`) copies the real wall
thermostat's flags (battery, error, link error, locked): listing it would
show the same problem twice. `renderVirtual` draws it as a device in
expert mode and adds "Temperature from" with `sources.actual_temp`,
radio addresses.

**The values** (`deviceValues`, `valueCell`): a cell is the name, the
number, the unit, an icon and the value's age (`ageEl`: the relative age,
and in expert mode the time, `.age-exact`), the age spanning the cell's
columns after the name, each part on its own line.

**Menus**: `dropdown(details)` makes a `details.menu` one of a kind
(closed by a choice, Escape and a click outside); the ⋯ menu and the Cube
choice are such menus. `sectionBadges` fills a section card's head.

**A wall thermostat's temperature** (`devicelist.go`): a radiator
thermostat's 0 is "not reported"; a wall thermostat's is read as 0.0 °C,
and the virtual thermostat takes the wall thermostat's temperature alone
where there is one, so the room showed 0.0 °C on 2026-11-13 until the
newly powered wall thermostat had measured.

**The icons** are SVG symbols in `index.html`, drawn in 1.8 strokes
(`.ni`, `.ti`, `.di`). Tabler's are MIT; their notice belongs in
`third_party/`, as `bytemare-ksf` has its own.

## Implementation Plan

### Block 1: Problems

- `index.html`'s sprite: `icon-radiator`, `icon-wall`, `icon-window`,
  `icon-window-open`, `icon-room`, `icon-radio`, `icon-battery`
  (ours), `icon-leaf`, `icon-shuffle` (Tabler); the licence comment;
  `third_party/tabler-icons/LICENSE`.
- `components.js`: `problemsEl(entries, label)`: `null` without a
  problem; else a `details.menu.problems` (`dropdown`), its summary one
  icon per kind in the entries' order, each in its tone's colour, labelled
  for a screen reader with the kinds; its items the entries, each its
  type icon, its name and a mono line, then its badges, each led by its
  icon, with its age.
- `tiles.js`:
  - `roomProblems(room)`: the real devices' radio fault, battery low and
    window open, and the thermostats' modes where they differ, as
    entries; the room tile's head shows `problemsEl` in place of
    `.window-open` and `.mode-mixed`;
  - `renderDevice`: the type icon before the name (`deviceIcon`), the
    problems as `problemsEl` of the one device, the mode or window
    badge after it;
  - `renderVirtual`: the room's icon, no badge of its own.
- `style.css`: `.problems`, the icons' tones; `.window-open` and
  `.mode-mixed` go.
- i18n: the kinds' names, six languages.
- Tests: `test_rooms.py` (the window's icon and its age, the list, the
  mixed modes, the virtual thermostat not listed), a test of the device
  head; screenshots.
- `docs/ui.md` "Rooms"; a fragment.

**Verification:** `make ui-test` for the rooms and the devices, 1100 and
390 px; the icons looked at, light and dark.

### Block 2: Values

- `valueCell`, `style.css`: the age under the name, the time under the
  value, on one line; outside expert mode the age alone.
- `renderVirtual`: "Temperature from" names each source by its serial
  (`deviceIds`' mono), its radio address where the device is unknown.
- Tests: the cells' places at 390 px; the serial in "Temperature from".

**Verification:** the tests named; `docs/ui.md` where it describes the
values.

### Block 3: WallZero

- `internal/protocol/cube/devicelist.go`: a wall thermostat's
  temperature of 0 is not reported (no `ActualTemp`), as a radiator
  thermostat's; the comment says why.
- A test with an L entry of a wall thermostat whose temperature bytes
  are 0.
- `docs/cube-protocol.md`: the finding, dated 2026-11-13, Cube firmware
  1.1.3, a wall thermostat after new batteries.
- A fragment.

**Verification:** `go test ./...`, `make race`, `make cross`,
`gofmt -l .`, the three `go vet`.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work
branch:

```
chore(aps): +TileBadges v1.0

The owner's evaluation of the room tile on 2026-11-13: its problems as icons in its head that list them by device, the same in each device's head through one component, a type icon before every device name, the values' times under their own value, the sources by serial, and a wall thermostat's 0 °C as not reported. PLAN++ with the recommended picks, D41, D42-C, D44-C, D46-A, D48-B, D49-A to D52.

Key changes:
- GUIDELINES/wip/2026-11-13_20-35_AP_TileBadges_problems_as_icons_in_the_tiles.md: the plan, three blocks

File-revision-v1: GUIDELINES/wip/2026-11-13_20-35_AP_TileBadges_problems_as_icons_in_the_tiles.md::v1.0::2026-11-13::All::the first revision, by PLAN++ as recommended
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(ui): A room's problems as icons in its head, listed by device`;
Block 2, `fix(ui): A value's time under that value, the sources by serial`;
Block 3, `fix(cube): A wall thermostat's 0 °C is not a measurement`.

## Change History

- v1.0 (2026-11-13): the first revision, from the owner's evaluation
  and `PLAN++` with the recommended picks.
