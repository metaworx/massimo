# AP RoomsPage v1.0: Tiles, House and the jump

## TL;DR

The owner's six requests for the rooms page, handed over by the session
"UI refactor" on 2026-11-24, and the page's jump to the top that the
owner sees at every poll: seven blocks, the jump's fix first, each a
pull request of its own.

- page at its end on a phone → no longer moves up at each poll
- a tile in holiday → ⓘ and ⚙ at the top, the device rows' values at the border
- setpoint row → a grid: chevrons at the tile's right border, never over it;
  the moon or sun a superscript after the °
- IS and SET → short labels; a tap on the icon shows the full term
- room entry → `min_setpoint`, `max_setpoint`
- SET → a list from OFF to the room's highest, and a field for `.` or `,`
- House tile → Holiday in its one form; the date hidden while Permanently;
  the lowest and highest actual and set temperatures

### Blocks

1. Jump: the header's measure clipped, a phone's page stays at its end
2. Holiday row: the tile's tools top-aligned, the device values at the border
3. Setpoint row: a grid, the chevrons fixed, the mark after the °
4. Labels: short labels under IS and SET, a shared tip with the full term
5. Range: `min_setpoint` and `max_setpoint` on the room entry
6. Dropdown: SET opens a field and the list from OFF to the highest
7. House: Holiday in the one form, the date hidden, lowest and highest

## Discussion

### What is decided

The owner, 2026-11-25, with `PLAN++ all A`
on this session's TL;DR gate and its amendment:

- **D1-A. Order and pull requests:**
  the blocks in this order, one pull request each,
  pushed as each is done; none merged by the session.
- **D2-A. The short labels,** at most five characters:
  en IS/SET, de IST/SOLL, fr ACT./CONS., it ATT./IMP., nl IS/DOEL, pl RZ./ZAD.
- **D3-A. The full term on a tap:**
  a small tip built once in `src/shared/`:
  one open at a time, closed by Escape or a click outside,
  keyboard operable.
- **D4-A. The dropdown's list:**
  OFF, then every 0.5 °C from the room's lowest setpoint to its highest,
  without ON.
- **D5-A. A typed value:**
  rounded to 0.5 °C, 4.5 °C counting as OFF;
  a value outside the room's range is marked on the field and not sent.
- **D6-A. Holiday's default end:**
  a week from today at 06:00, shown and editable before Send.
- **D7-A. One form** (the owner: "I want one single form,
  with different settings that determine the available options and
  defaults"): the House tile keeps its one mode form,
  and Holiday is a fifth choice beside ECO, AUTO, COMFORT and MANU.
  The choice sets what the form offers and its defaults:

  | Choice | Temperature | Duration |
  |--------|-------------|----------|
  | ECO, COMFORT | none, the room's own | Permanently or until a date |
  | AUTO | none | none |
  | MANU | the field, the first room's setpoint | Permanently or until a date |
  | Holiday | the field, OFF | until a date only, a week ahead at 06:00 |

  The Holiday button in the modes' row opens the form with Holiday chosen.
- **D8-A. The House's date** is hidden while Permanently is chosen.
- **D9-A. A tap on a lowest or highest value** does nothing:
  the rows name no room.
- **D10-A. The lowest and highest** are computed in the page
  from the rooms' virtual wall thermostats (`room.vwt`); no server change.
- **D11-A. The jump** is Block 1, a pull request of its own,
  so that it can merge while the rest is built.

The owner's answers on the jump, 2026-11-25:
the Android app on the phone; all the way to the top; every single time;
after nothing but scrolling down and waiting for the poll, in expert mode;
the owner's server on devel (v0.16.0-alpha.1-612 for the phone,
-716 for Firefox), no update in between; also in Firefox on the desktop
at 490 × 1130 px, there only at the very end of the page and by a part,
which grows with the window's height. A screen recording showed the
moves at 10.5 s, 20.5 s and 30.5 s, each the moment the server's reply
came, each with the page at its end.

### Decided in the writing

- **W1. Branches.**
  Blocks 1 and 2 branch off main, each on its own.
  Blocks 3 to 7 touch the same lines of `Tile.tsx` and `style.css`,
  and Block 6 needs Block 5's fields:
  they are one stack, each block's branch on the one before.
  Every pull request is against `main`;
  one of the stack names in its body the pull request to merge first.
  Branch names: `claude/rooms-jump`, `claude/rooms-holiday-row`,
  `claude/rooms-setpoint-row`, `claude/rooms-labels`,
  `claude/rooms-range`, `claude/rooms-dropdown`, `claude/rooms-house`.
- **W2. Who builds.**
  An Opus sub-agent builds each block in this session's worktree,
  one at a time; a second Opus sub-agent reads its diff
  before the pull request; this session commits, signs, pushes
  and opens the pull request.
- **W3. Verification.**
  Locally the UI test files a block touches, in all four cases,
  `make ui-check` for a change under `src/`,
  `go test` of the packages a block touches, `gofmt`, `go vet` for linux,
  windows and darwin, and `GUIDELINES/shared/tools/check`;
  the whole Go and UI suites are CI's.
- **W4. Main moves meanwhile.**
  The session "UI refactor" is changing the six `ui/i18n/<lang>.json`,
  `components.js` (the menus' Escape), `index.html` (the icons' sprite)
  and the UI test kit (`helpers.py`, `conftest.py`):
  each branch merges `origin/main` before its pull request.
- **W5. Names.**
  No real name of the owner's Cubes, rooms or devices,
  and no real serial, in any file, test or message:
  the screenshots and the recording show them.

## Analysis

### The jump

The header's row folds where it is short (`src/header/fold.ts`,
AP GlobalMenu): at each fit it sets `data-fold="measure"`,
lays the row out with every item at its whole width, reads the widths,
and sets the step that fits, all in one task.
`watchFold` fits again at each of the header's renders,
and the header renders at each poll.

On a phone's page (`width=device-width`) the measured row is wider than
the screen: 564 px on a 412 px page with the fake Cube.
For that layout the page's scroll width is 580 px,
and a mobile browser zooms out to show it: the window's height
becomes 1289 px instead of 915.
Where the page is scrolled nearer its end than those 374 px,
the browser cuts the scroll to fit the taller window,
and nothing moves it back once the row is folded again.
At the page's end the page moves up by the zoom's share of the
window's height (here 41 %), and to the top where the page is short:
the owner's recording, and Firefox's responsive mode,
whose jump grows with the window's height.

Measured with the fake Cube in Chromium as a phone (412 × 915,
`is_mobile`): the page at its end, scrollY 945; after the first poll 571,
and so after each poll that finds it at its end.
With `.top-line[data-fold="measure"] { overflow-x: clip; }`
the measure leaves the page 412 px wide, reads the row's 564 px
as before, and scrollY stays 945 through three polls.
The clip applies to the measure alone,
so no open list or focus ring of the row is ever clipped.

The search of "UI refactor" did not find it:
its pages were desktop pages, where nothing zooms,
and its tile was in the middle of the page.
`test_scroll.py`'s `test_scroll_to_the_bottom_stays_there`
runs without `is_mobile` and does not see it.

### The other requests

- **The holiday row** (`src/rooms/Tile.tsx`, `.mode-row`, `.tile-tools`;
  `.device-vals` in `style.css`):
  in holiday the tile's chip "Holiday until …" wraps onto a second line,
  and the tools centre on both lines;
  the device rows' values end short of the row's right edge.
- **The setpoint row** (`Tile.tsx`, `div.setpoint-row`):
  two `.temp-block`s and `div.stepper` in a flex row;
  in Italian at 346 px the labels go, and at 956 px in two columns
  the chevrons overflow into the next tile.
  `PresetMark` is a badge at the top right of SET's value.
- **The labels:** `room.is` and `room.set` are today's labels,
  from IS/SET to ATTUALE/IMPOSTATA.
- **The range:** `setRoomOrder` (`internal/server/vwt.go`) gives the room
  its comfort and eco temperatures from `roomConfig`,
  whose `Temps` also hold `Max` and `Min` (`cubeio.Temperatures`).
- **The House tile** (`src/rooms/House.tsx`):
  `MODES` ECO, AUTO, COMFORT, MANU; `DURATIONS` permanent and until;
  a non-AUTO mode with until is sent as `vacation`, else as `manual`;
  the date field is disabled, not hidden, while permanent.

## Implementation Plan

Each block moves its documents with its code, ends with the gate of
`GUIDELINES/shared/GATE_WORKFLOW.md`, and is a pull request of its own
(D1-A, W1). Every commit of the run cites `AP RoomsPage v1.0, Block <n>`.

### Block 1: Jump

- **Goal:**
  a phone's page scrolled to its end stays there through a poll.
- **Changes:**
  1. `internal/server/ui/style.css`:
     `.top-line[data-fold="measure"] { overflow-x: clip; }`,
     its reason in the header's comment.
  2. `internal/server/ui/src/header/fold.ts`:
     the file comment says why the measure is clipped.
- **Tests:**
  `tools/ui-test/test_scroll.py`: a page as a phone
  (`has_touch`, `is_mobile` at 390 px, as `test_keep_open.py` starts one),
  long enough with a room's devices open, scrolled to its end,
  through the header's render that a poll brings:
  its scrollY unchanged. Seen failing before the change.
- **Documents:**
  `docs/ui.md`, the header's row: measured within its width,
  so that the page never widens for the measure.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-rooms-jump.md`.
- **Commit:**
  `fix(ui/header): A phone's page stays at its end through a poll`
- **Verification:**
  `make ui-check` exit 0; `test_scroll.py` in all four cases exit 0;
  `GUIDELINES/shared/tools/check` exit 0.

### Block 2: Holiday row

- **Goal:**
  a tile in holiday on a phone has no empty space
  above its tools or right of its device rows' values.
- **Changes:**
  1. `style.css`: the tile's tools aligned to the top of the modes' row
     when it wraps; the device rows' values right-aligned to the row's border.
  2. `src/rooms/Tile.tsx` and `src/rooms/Devices.tsx` only where the CSS
     cannot do it alone.
- **Tests:**
  a test of `test_room_marks.py` or `test_rooms.py` with a room in holiday:
  the tools' top equals the first mode button's top;
  the last value's right edge within 2 px of its row's.
- **Documents:**
  `docs/ui.md`, the tile and the device rows.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-rooms-holiday-row.md`.
- **Commit:**
  `fix(ui/rooms): A tile in holiday keeps its tools up, its values at the edge`
- **Verification:**
  as Block 1's, with the touched test files.

### Block 3: Setpoint row

- **Goal:**
  the setpoint row fits its tile at every width and in every language.
- **Changes:**
  1. `style.css`, `.setpoint-row`: a grid of the IS block, the SET block
     and the chevrons; the chevrons' column fixed at the tile's right
     border; the value blocks `minmax(0, 1fr)`; the numbers' size clamped.
  2. `Tile.tsx`, `PresetMark`: a small superscript right after the °,
     inside the value's box.
- **Tests:**
  the row checked at 346 px, 390 px and two columns at 956 px,
  in all six languages: the chevrons inside the tile,
  both labels shown, the mark after the °.
- **Documents:**
  `docs/ui.md`, the tile's setpoint row.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-rooms-setpoint-row.md`.
- **Commit:**
  `fix(ui/rooms): The setpoint row fits its tile, the mark after the degree`
- **Verification:**
  as Block 1's, with the touched test files.

### Block 4: Labels

- **Goal:**
  short labels under the IS and SET icons; a tap on the icon
  shows the full term (D2-A, D3-A).
- **Changes:**
  1. The six `ui/i18n/<lang>.json`: `room.is` and `room.set` the short
     labels of D2-A; `room.isFull` and `room.setFull` the full terms
     (FR, IT, NL, PL machine-translated and marked).
  2. `src/shared/` (new): the tip, a labelled button that opens a small
     tip: one open at a time, Escape and a click outside close it,
     keyboard operable; its row in `docs/ui.md` ("How the page is built").
  3. `Tile.tsx`: the IS and SET icons the tip's buttons.
- **Tests:**
  a tap on each icon shows its full term; Escape and a click outside
  close it; a second tip closes the first; keyboard opens it.
- **Documents:**
  `docs/ui.md`, the tile and the shared part.
- **CHANGELOG:**
  `CHANGELOG.d/Changed/claude-rooms-labels.md`.
- **Commit:**
  `feat(ui/rooms): Short labels under IS and SET, the full term on a tap`
- **Verification:**
  as Block 1's, with the touched test files.

### Block 5: Range

- **Goal:**
  a room entry gives the room's lowest and highest setpoint.
- **Changes:**
  1. `internal/view/view.go`, `Room`: `MinSetpoint`, `MaxSetpoint`
     (`min_setpoint`, `max_setpoint`, omitted for a room without a
     thermostat), commented as `ComfortTemp` is.
  2. `internal/server/vwt.go`, `setRoomOrder`: both from `roomConfig`'s
     `Temps.Min` and `Temps.Max`.
- **Tests:**
  `internal/server`: a room entry carries both, from the fake Cube's
  configuration; a room without a thermostat carries neither.
- **Documents:**
  `docs/api-v1.md` (the room entry), `docs/api-v1-openapi.yaml`,
  `src/api.d.ts` by `make api-types`.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-rooms-range.md`.
- **Commit:**
  `feat(api): A room entry gives its lowest and highest setpoint`
- **Verification:**
  `make ui`, then `go test ./internal/server/ ./internal/view/` exit 0;
  `gofmt -l .` prints nothing; `go vet ./...` for linux, windows and darwin
  exit 0; `go run ./tools/apitypes -check` exit 0;
  `GUIDELINES/shared/tools/check` exit 0.

### Block 6: Dropdown

- **Goal:**
  SET opens a list of the room's temperatures and a field (D4-A, D5-A).
- **Changes:**
  1. `Tile.tsx`: SET's value a button with a small ▾ exactly under the °,
     opening a popover of the shared menu (one open at a time,
     Escape and a click outside close it).
  2. The popover: a number field on top (`inputmode="decimal"`,
     `.` and `,` alike, Enter sends; D5-A), and under it the list,
     OFF and then every 0.5 °C from `min_setpoint` to `max_setpoint`,
     each with its eco or comfort mark, scrolled to the current value;
     a choice sends the room setpoint the stepper sends.
  3. The six `ui/i18n/<lang>.json`: the popover's texts.
- **Tests:**
  the list's first and last entries; a choice and a typed `21,5`
  each send their line to the fake Cube;
  a value outside the range is marked and not sent; Escape closes.
- **Documents:**
  `docs/ui.md`, the tile's setpoint.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-rooms-dropdown.md`.
- **Commit:**
  `feat(ui/rooms): SET opens a list of temperatures and a field`
- **Verification:**
  as Block 1's, with the touched test files.

### Block 7: House

- **Goal:**
  the House tile's one form takes Holiday,
  hides the date while Permanently,
  and the tile shows the house's lowest and highest temperatures
  (D6-A to D10-A).
- **Changes:**
  1. `src/rooms/House.tsx`: Holiday a fifth choice of the form and a button
     with the case icon in the modes' row; each choice's options and
     defaults as D7-A's table; Holiday sends `vacation` with its
     temperature and end, one room command per room,
     shown as the House's writes are.
  2. The date field hidden while Permanently (D8-A).
  3. Under the modes, two rows: the thermometer with Block 4's short label,
     the lowest and highest actual temperature;
     the gauge with its short label, the lowest and highest set one,
     OFF where it is; no room names, the values aligned,
     on one line where there is room (D9-A, D10-A).
  4. The six `ui/i18n/<lang>.json`: Holiday's texts.
- **Tests:**
  Holiday's defaults (OFF, a week ahead at 06:00, no Permanently),
  its send to every ticked room;
  the date hidden while Permanently and shown with until;
  the two rows' values from the fake Cube's rooms.
- **Documents:**
  `docs/ui.md`, the House tile.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-rooms-house.md`.
- **Commit:**
  `feat(ui/rooms): Holiday in the House's form, its lowest and highest`
- **Verification:**
  as Block 1's, with the touched test files.

## Proposed commit message

The plan: `chore(aps): +RoomsPage v1.0`.

Block 1:

```
fix(ui/header): A phone's page stays at its end through a poll

AP RoomsPage v1.0, Block 1: the header's row is measured within its own
width. Laid out at its whole width for the measure, it made a phone's
page wider than the screen for that moment; the browser zoomed out and
cut the scroll of a page near its end, which moved up at every poll.

Key changes:
- internal/server/ui/style.css: the measure clipped to the row's width
- internal/server/ui/src/header/fold.ts: why, in its comment
- tools/ui-test/test_scroll.py: a phone's page at its end through a poll
- docs/ui.md: the header's row measured within its width
- CHANGELOG.d/Fixed/claude-rooms-jump.md: the bullet

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 to 7: the summaries of their blocks above.

## Change History

- v1.0 (2026-11-25): first revision, written under `PLAN++ all A`.
