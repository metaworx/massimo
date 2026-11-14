# AP CubeBadges v1.0: every Cube's signs and problems

## TL;DR

Every Cube gets a network sign by its link, and its problems become one
pill: in the header beside the Cube choice, for all Cubes, and in System
settings → Cubes, for each. Unsaved edits there survive the live
redraws. It builds on AP Signs.

- API → each Cube's link reason, the address discovery found it at,
  its devices' problems and its configuration warnings
- header → the Cube choice with its network sign; beside it the
  problems pill: links, devices, and a cog group for the rest (D85-C)
- the pill's list → grouped by Cube, the Cube always its heading,
  devices indented under it, the cog group last
- System settings → Cubes → closed: sign, name, serial · host, one
  pill with everything; the pill opens its list, not the Cube; open:
  its badges on top; the green "connected" badge goes
- drafts → kept across redraws; a stored value that changed meanwhile
  warns, with "Take theirs" (D59-A)

### Blocks

1. LinkApi: each Cube's link state with its reason, `found_address`;
   the OpenAPI file, tests
2. ProblemsApi: each Cube's devices' problems and configuration
   warnings in `GET /api/v1/cubes`; tests
3. HeaderPill: the Cube choice's sign, the problems pill and its
   grouped list, the cog group, the choice list's icons
4. CubeCards: System settings → Cubes, closed and open
5. Drafts: edits kept across redraws, the conflict warning
6. Docs: `docs/ui.md`, `docs/api-v1.md`, the six languages, tests

The six blocks are one pull request, stacked on AP Signs' until that
one is merged.

## Discussion

The owner's evaluation of 2026-11-13/14, in mock rounds 1 to 8 (the page
"Cube Choice Signs"), settled:

- live badges in System settings → Cubes, the cards redrawn as the
  poll goes, and unsaved edits kept, with a warning and "Take theirs"
  where the stored value changed meanwhile (D59-A);
- `found_address`: the address discovery last found a Cube at, beside
  the one it connects to;
- a network sign before each Cube's name, by its link: connected (no
  mark), held or reconnecting (a triangle), lost, stale or busy (a
  cross);
- beside the Cube choice the same problems pill as a room tile's, its
  list grouped by Cube: the Cube always the heading (with a green sign
  when only its devices fail), its devices indented under it;
- D85-C: a cog group in the header's pill for everything that is
  neither a link nor a device: a proxy that does not listen, a settings
  file that differs, Cubes found and not added; for those with the
  settings right;
- the Cube choice's own list keeps each Cube's sign;
- System settings → Cubes: a closed Cube shows its sign, its name,
  serial and host, its counts, and one pill holding every problem (the
  link, the proxy, the settings file, the devices) beside the neutral
  source badge; a click on the pill opens its list, not the Cube; an
  open Cube shows its own badges as its top line; the green "connected"
  badge goes;
- D73-A: the shuffle icon is for mixed modes only;
- D86-A: one plan of its own, on AP Signs' signs, list and colours.

`PLAN++ as recommended` on 2026-11-14.

## Analysis

**The Cubes' API** (`server.go` `cubeView`, `listCubes`). Each Cube has
`connected`, `link` (`cubelan.Link`: `state` held, stale or busy, with
`since` and `until`, from AP LinkOutage), `proxy_failing`,
`file_differs`, `mac`, `last_seen`; Cubes discovery found and the
server does not serve come in the same array with `configured: false`.
Nothing gives the address discovery found a Cube at (the follow and
the fixed address both show `address`). Nothing gives a Cube's
devices' problems: the page loads rooms for the chosen Cube only
(`loadRooms`), so the header cannot know another Cube's.

**The problems** are those of `tiles.js` `deviceProblems` (radio
error and link error, low battery, the window by Signs' limits), by
device: its radio address, serial, name and room, its problem kinds and
their level, and for the window when it opened. The server builds them
from `view.Devices` with the hidden list and the stale threshold, the
same that `push_api.go` uses for notifications.

**The header** (`index.html`'s Cube choice, `app.js`
`renderCubeSelect`, `renderHeader`). The choice is a `select` today;
its list cannot hold icons. A `dropdown()` (`components.js`) holds them.
The pill is `problemsEl` with Signs' entry format and `subs`.

**System settings → Cubes** (`cubes.js` `renderCubes`, `cubePanel`,
`cubesPolled`; `partials/cube-panel.html`). The section's badges: served,
disconnected, proxy failing, found; each Cube's: connected or
disconnected, source, differs, proxy failing. `cubesPolled` redraws on
each poll: a redraw while a field is being edited would take the
edit, which is why the cards are not live today.

## Implementation Plan

### Block 1: LinkApi

- `cubeView`: `link_reason`, one of `connected`, `held`, `stale`,
  `busy`, `refused`, `unreachable`, `not_found`, from `connected`, the
  link and the client's last error; `found_address`, the address and
  port discovery last found the Cube at, absent where none.
- `docs/api-v1.md`, the OpenAPI file.
- Tests: each reason from the fake Cube's states; `found_address` after
  a discovery answer.

**Verification:** `go test ./internal/server/ ./internal/cubelan/`,
`make race`.

### Block 2: ProblemsApi

- `cubeView`: `problems`, a list of the Cube's devices with a problem:
  `rf_address`, `serial`, `name`, `room`, and per kind (`radio`,
  `battery`, `window`) its level (`warn`, `bad`) and since when;
  `warnings`, the configuration's (`proxy`, `file`).
- The window's level from Signs' limits.
- Tests: each kind from captured lines; a hidden device not listed.

**Verification:** `go test ./...`.

### Block 3: HeaderPill

- `index.html`, `app.js`: the Cube choice a `dropdown()` with each
  Cube's sign and name; the button shows the chosen Cube's sign.
- Beside it the pill: `problemsEl` of every Cube with a problem, the
  Cube as the heading (its sign; green when only its devices fail), its
  devices as `subs`; the cog group last, for the settings right (proxy,
  file, found Cubes).
- Tests: the list's order and grouping, the cog group's right, the
  choice by keyboard.

**Verification:** `make ui-test` at 1100 and 390 px.

### Block 4: CubeCards

- `cubes.js` `cubePanel`: the closed head with the sign, name, serial ·
  host, counts, the pill and the source badge; the pill's click opens
  its list without opening the card; the open card's top line its
  badges; the "connected" badge out; `renderCubes`' section badges in
  the same signs.
- Tests: the pill in a closed card, the click, the top line.

**Verification:** `make ui-test`.

### Block 5: Drafts

- `cubes.js`: an edited field keeps its value across `cubesPolled`; a
  stored value that changed since the edit began shows a warning under
  the field with the new value and "Take theirs"; Save sends the draft.
- Tests: an edit survives a poll; a change by another client warns;
  "Take theirs" takes it.

**Verification:** `make ui-test`.

### Block 6: Docs

- `docs/ui.md` "Header" and "System settings → Cubes"; `docs/api-v1.md`.
- i18n: every new text, six languages.
- CHANGELOG fragments.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

The registration, with the other plans of the same evaluation, a commit
of its own on main.

The blocks:
Block 1, `feat(api): A Cube's link reason and the address it was found at`;
Block 2, `feat(api): Each Cube's problems in the Cubes' list`;
Block 3, `feat(ui): The Cube choice with its sign, all Cubes' problems beside it`;
Block 4, `feat(ui): A Cube's problems in one pill in System settings`;
Block 5, `feat(ui): An edit of a Cube survives the live redraw`;
Block 6, `docs(ui): The Cubes' signs and problems`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation of
  2026-11-13/14 and `PLAN++ as recommended`.
