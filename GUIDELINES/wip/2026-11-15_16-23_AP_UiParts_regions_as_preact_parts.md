# AP UiParts v1.0: regions as Preact parts

## TL;DR

The web UI stops rebuilding what the user holds. Its regions become
Preact parts, built once and updated in place, one region at a time and
by several sessions side by side, beside the plain code still there.
Before any of that, the scripts that hold several regions are cut into
one script per region, by moves alone.

- scripts with several regions → one script per region, moved by a
  tool, nothing else changed; an open branch follows with one recipe
- no build step → one: esbuild as a Go tool, TSX in, one script out
- a region's drawing → parts; its actions and facts stay plain until
  its turn; plain pieces inside a part stay where they are
- open, focused, typed → kept by the part; the code that saved and put
  back goes
- a guard at every data source: no element replaced, no typed text lost
- first region → the Cubes, where the problem list closed at each poll
- the security policy, the style sheet, the texts → as they are

### Blocks

1. Regions: the scripts and `index.html` cut along the page's regions,
   by moves alone
2. Toolchain: esbuild from Go, Preact vendored, the bundle, the type
   check, the contract's rule
3. Guard: the generic checks, with the list of regions not yet moved
4. ApiTypes: the API's shapes as TypeScript, from the OpenAPI file
5. Cubes: the Cubes list and a Cube's panel as parts, with the shared
   small parts; the reported fault and two defects
6. Header: the facts, the Cube choice, the problem list, the notices
7. Rooms: the tiles, the device rows, the House tile, the devices
   without a room
8. RoomConfig: a tile's configuration
9. Pairing: the pairing form and its countdown
10. Admin: users, roles, providers, integrations, backups
11. Log: the log page and its sign
12. Maintenance: software update, maintenance, system info
13. Personal: my settings, appearance, notifications, my profile
14. Shell: the menu and the pages, the page's chrome, the sign-in
15. End: the data in signals, the plain builders nobody calls, the
    guard's list, the split tool

## Discussion

### What is decided

The owner, on 2026-11-15, after two analyses and a spike
(`wip/2026-11-15_11-44_ANALYSIS_UiParts_ui_redraw_fresh_look.md`,
`wip/2026-11-15_15-30_ANALYSIS_UiParts_room_tile_both_ways.md`):

- **D122-D. The library:** Preact with signals, written in TSX, bundled
  by esbuild run from Go.
- **D170-A. The guard also types:** it types into every field of a
  part, lets data come in and reads the field again.
- **D171-A. The unit of the move:** a region, from its root inward;
  plain pieces inside it stay until their turn. The Cubes first.

As recommended in the first analysis, the owner naming no other:

- **D123-A. The store:** one place writes each collection. Under D122-D
  that is a signal per collection, at the end (Block 15); until then the
  data stays the page's plain objects, with a counter per item that
  says "draw this one again".
- **D124-A. The guard:** generic checks and a test per part. The check
  that build and update agree is not needed: a part has one description.
- **D125-A. The scope:** the whole page.
- **D126-A. The two defects in Cube settings:** with the Cubes (Block 5).
- **D127-A. The rule:** in the contract's "The web UI", the detail in
  `docs/ui.md`.

D170 and D171 were D130 and D131 in the session's messages; those
numbers are AP GlobalMenu's for other decisions.

### The owner's question: can the first block cut the regions out, unchanged?

Yes, and it is Block 1, ahead of the toolchain and apart from it.

- **Why first:** it needs nothing of what follows, and from then on a
  region's move replaces that region's own files. Two sessions on two
  regions no longer meet in `app.js` or `tiles.js`.
- **Why apart:** a commit that holds nothing but moves can be made
  again by a tool on any branch. That is what makes the merge easy:
  an open branch makes the same moves on its own code and then takes
  main's as already done (Block 1, "An open branch").
- **Classic scripts, not modules:** the cut pieces stay scripts with
  globals, in the place of the script they came from. `import` and
  `export` would change how every caller and every test reaches a
  function, which is not "unchanged". A region gets modules when it
  becomes a part.
- **What is not a pure move:** three `init…` functions wire two regions
  each. They are cut in place first, in commits of their own.

### Still open

Written with the recommended option of each; another choice changes the
block named.

- **D172. Where the bundle lives** (Block 2): built by `make` and never
  committed (A, written here), or committed with a check that it is
  current (B).
- **D173. How far the first cut goes** (Block 1): the six scripts that
  hold several regions (A, written here), or every script that holds
  two (B).

### What stays as it is

- The page's security policy; no inline handler, no inline style
  attribute.
- `style.css`, the classes and the ids: a part's markup is its
  partial's.
- The texts: `t()`, `tn()` and the six `i18n/<lang>.json`.
- `week-program.js`, which the Home Assistant card loads by itself;
  `connect.js` and `connect.html`; `sw.js`, `notifytext.js`, `i18n.js`.

## Analysis

### What is lost today, and why

The page builds a part again when data comes in, when the user acts in
it, or never while it is shown. The first analysis found about thirty
places that save and put back what the user had, in seven families, and
22 of 31 probed cases lost all the same. The reported one: a Cube's
problem list, opened with the panel closed, closes at each poll, since
`cubesPolled` builds the whole list again.

### What the spike measured

The room tile as a Preact part, in the page as it is, the rest plain:

- 20 of 20 probed cases kept (today 4);
- 222 of the suite's 224 tests pass unchanged; the two others assert
  that the tile was built again once the focus left;
- 586 lines of code for the drawing against today's 699;
- bundle 40 kB, built in 0.1 s; type check 0.4 s; the security policy
  holds.

### How a part and the plain code meet

Tried in the spike, and the pattern for every region:

- **The page calls the part** through a few functions on
  `window.MassimoParts.<region>`: the collection was loaded, an item
  changed in place, show a result. The plain `render…` names stay, as
  calls of these.
- **Per-place state becomes live:** `state.ui[id]` and its like are
  objects whose properties are signals underneath. The plain actions
  set `ui.pending` as before, and the part follows.
- **The part calls the page's globals**, declared to TypeScript in a
  file per plain script.
- **A plain piece inside a part** is filled by its plain code into an
  element the part leaves alone, as the tile's configuration was.

### Four rules the spike gave

Each was a fault seen in the browser.

1. **A field that shows stored data keeps a draft.** `value={stored}`
   alone is set back at every draw of its part.
2. **A part that reads a signal reads all it depends on as signals.**
   It is not drawn again with its parent while its properties are the
   same; the language and the data's counter are read by the part
   itself.
3. **No `data-i18n` on a part's elements.** `applyI18n(document)` would
   write over text the part owns; a part calls `t()`.
4. **A plain piece that was always given a new element** may leave
   content behind; its place is emptied when it closes.

### Where the regions are today

Lines of `main` at `39340b8`.

| Script | Lines | Regions in it |
|---|---|---|
| `app.js` | 1623 | the core (state, API, helpers, loading, polling, events, `init`); the header (584–855, 1084–1305); the home notice; discovery; my settings' wiring |
| `tiles.js` | 1417 | a room's facts and actions; the tile (244–829); the device rows (830–1022); the devices without a room (148–243); the House tile (1241–1384) |
| `cubes.js` | 1116 | the list and the found Cubes; a Cube's head and panel (74–346, 731–939); pairing (347–730) |
| `update.js` | 1132 | software update (17–275); maintenance; system info (513–569, 643–927) |
| `log.js` | 1490 | the log; its sign; the pages and the menu (1263–1442) |
| `theme.js` | 688 | the theme's logic; the appearance card and the scheme editor (385–686) |
| `components.js` | 1302 | the shared builders; the menus; the page's chrome (724–1034) |
| `login.js`, `users.js`, `providers.js`, `backup.js`, `notify.js`, `roles.js` | 221–694 | two regions each |

`config.js`, `integrations.js` and `week-program.js` are one region
each.
Of `index.html`'s 533 lines, the header, the home notice, the first
owner's form, my settings' rows, the Cube page, the House tile, the
devices without a room, the log view, the sign-in dialog and the toast
are not partials yet.

### What names a script

- `ui.go`'s `go:embed` line, every script by name;
- `index.html`'s script tags, in an order that matters in three places:
  `config.js` after `week-program.js`, `theme.js` after `app.js`, and
  the `DOMContentLoaded` listeners in their order;
- `ui_test.go`: a list of 18 script addresses; `TestUIScriptGlobals`
  (no top-level name in two scripts, by the page's tags), which holds a
  cut by itself; `TestLogKinds` and `TestUIDefaultSchemeMatchesCSS`,
  each tied to one file's name;
- comments in `index.html`, `style.css`, the partials and the scripts,
  about a hundred; `docs/ui.md`, "How the page is built".

Files are served by name from `/ui/{file}`; a name with a `/` is
refused. `sw.js` lists no script.

### Where Go is compiled

A bundle that is embedded and not committed must exist wherever the Go
code is compiled:

- the Makefile's `build`, `test`, `vet`, `race`, `cross`, `packages`
  and what rests on them;
- eleven of CI's jobs, about a dozen steps that call `go` directly;
  `build.yml` through `make build-release`;
- `tools/ui-test/run.sh`, `tools/lan-test/run.sh`,
  `tools/store-fixture/make.sh`;
- the Homebrew formula, which builds on the user's Mac from GitHub's
  archive of the tag or the commit.

## Implementation Plan

Each block carries its tests, its documents and its CHANGELOG fragment.

**The order:** 1, then 2; 3 and 4 side by side; then 5, which brings
the shared small parts and is the pattern; 6 to 14 in any order, each
by a session of its own; 15 last.

**How a region moves** (Blocks 5 to 14):

1. Where its script still holds another region, the script is cut
   first, with Block 1's tool. Its parts go under
   `internal/server/ui/src/<region>/`, with the markup, classes and ids
   of its partial, and `t()` for every text.
2. Its root is mounted in the container `index.html` has for it. The
   plain code hands it its data through `MassimoParts.<region>`; the
   plain `render…` names other scripts call stay, as calls of that.
3. What it kept per place (an `open` set, a `…UI` object, a copy of a
   running request) becomes live state. The code that saved and put
   back, skipped or deferred a redraw goes.
4. A plain piece inside it stays plain, in an element the part leaves
   alone, until its own block.
5. The four rules of the Analysis hold, and the region's fields that
   show stored data keep the "changed meanwhile" notice they have.
6. The region leaves the guard's list (Block 3). A test that asserts a
   rebuild is reworded to what it protects.
7. A partial nothing clones any more is removed; `docs/ui.md` says what
   the region now keeps.

**Verification of a region:** `make ui-test`, the whole suite at 1100
and 390 px in both languages; the guard with the region off its list;
`make ui-check`; the first analysis's cases for the region, as tests.

### Block 1: Regions

- **In place first**, a commit each:
  - `initTiles`, `initUpdate` and `initLog` become one function per
    region, called in the same order;
  - `ui_test.go`'s list of 18 addresses gives way to "every script
    embedded is on the page, or named as not on it", so that a move
    changes no test.
- **`tools/ui-split/`:** `split.py` and `map.txt`, which names for each
  new script the top-level declarations it takes. The tool moves a
  declaration with the comment above it, byte for byte and in the
  order it had; it stops on a name it does not find or finds twice;
  it also puts the new scripts into `ui.go`'s embed line and into the
  page's script tags, in the place of the script cut.
  `--check <commit>` makes a move again on the commit's parent and
  compares. Its `README.md` holds the recipe below.
- **The moves**, one commit per script, each the tool's output and
  nothing beside it:

  | From | To |
  |---|---|
  | `app.js` | `app.js` (the core), `header.js`, `home.js` (the notice, discovery), `prefs.js` |
  | `tiles.js` | `rooms.js` (facts, actions, `renderRooms`, `updateDevice`), `tiles.js`, `devices.js`, `unlisted.js`, `house.js` |
  | `cubes.js` | `cubes.js` (the list, the found Cubes, the page), `cube-panel.js`, `pairing.js` |
  | `update.js` | `update.js`, `maintenance.js`, `system-info.js` |
  | `log.js` | `log.js` (with its sign), `pages.js` |
  | `theme.js` | `theme.js` (the logic, `BUILTIN_SCHEMES`), `appearance.js` |

- **`index.html`:** the regions that are not partials yet become
  partials, through `uiPartials`; the page served is the same text.
- **After the moves**, one commit: the comments that name a moved
  declaration's script name the new one, by the same map.
- `docs/ui.md`, "How the page is built": the scripts by region.
- **An open branch** follows each move so:
  1. merge main at the commit before the move;
  2. run the tool with main's map, commit;
  3. merge the move itself with `-s ours`: the branch has made it;
  4. go on with main.

**Verification:** `go test ./...`; `split.py --check` on every move
commit; the page served before and after the partials is byte for byte
the same; the whole suite unchanged and green in the four cases.

### Block 2: Toolchain

- **`tools/uibundle/`**, a module of its own as `tools/actionlint` is:
  a Go program over esbuild's API, pinned, with its Dependabot entry.
  TSX in from `internal/server/ui/src/`, one classic script out,
  `internal/server/ui/parts.js`, licence comments kept.
- **`third_party/preact/`:** `preact`, its hooks and JSX runtime,
  `@preact/signals` and its core, as published, with their types and
  licences; a `README.md` with versions, npm integrity and SHA-256, and
  `TestVendoredPreact`, as `TestVendoredOpaque` holds `opaque.js`.
- **The bundle** (D172-A) is ignored by git, embedded by name and
  loaded after the plain scripts. `make ui` builds it; the Makefile's
  targets that compile Go depend on it; the three scripts, CI's jobs
  (one shared step) and the Homebrew formula get the same line. Without
  it the Go code does not compile, and the error names the file.
- **`src/index.ts`** registers `window.MassimoParts`, empty.
- **The type check:** `make ui-check`, `tsc --noEmit` from
  `tools/ui-check/` (TypeScript alone, locked); a CI job, named in
  "All tests passed"; its Dependabot entry.
- **The rule** (D127-A), in `GUIDELINES/project/_CONTRACT.md` and
  generated from there:
  - "The web UI": one bundler and one library; a part is built once per
    key and updated in place, and data never replaces an element the
    user can hold; the four rules; plain scripts and partials for what
    has not moved.
  - the facts table: `make ui` before the test gate, `make ui-check`
    for a change under `ui/src/`.
- `docs/ui.md` "How the page is built"; `GUIDELINES/project/CI.md`;
  `packaging/README.md` "Homebrew"; `README.md` where it says how to
  build; `tools/ui-test/README.md`.

**Verification:** from a fresh clone, `go build ./cmd/massimo` fails
naming `parts.js` and `make build` works; the bundle built twice is the
same file; `make cross`; `make actionlint`; `make ui-test`;
`make ui-check` names a planted mistake; the policy header's test as it
is; the formula rendered and built once on the Mac with the owner.

### Block 3: Guard

- **`tools/ui-test/test_parts_guard.py`** (D124-A, D170-A). For every
  region not on its list, with the region's panels shown, and after
  each of `poll(false)`, `loadCubes()`, `loadRooms()` and a state event
  with the data as it was:
  1. every element a user can hold (a button, a field, a link, a
     summary) is the element it was;
  2. every open `details` is open;
  3. every field takes a typed value and still holds it;
  4. the focus is where it was.
- **The list** starts with every region, each with what it loses today,
  and loses a line per block.
- `docs/ui-testing.md` and `tools/ui-test/README.md`.

**Verification:** the guard passes with every region listed; with the
Rooms taken off the list it fails on today's page, shown once in the
pull request.

### Block 4: ApiTypes

- **`tools/apitypes/`:** a Go program that writes
  `internal/server/ui/src/api.d.ts` from the schemas of
  `docs/api-v1-openapi.yaml`. The file is committed, since it changes
  only with the API, and a Go test holds it current, beside
  `TestOpenAPICoversRoutes`.
- `docs/api-v1.md`, where it names the OpenAPI file's uses.

**Verification:** `go test ./tools/apitypes`; `make ui-check` over a
sample that reads a Cube and a device.

### Block 5: Cubes

- **`src/shared/`:** icon, sign and mark (with their blink), badge, a
  value's age, the problem list with entries under entries, the field
  row, the question box; the live-object helper; the language as a
  signal, raised by `applyLanguage`; `src/plain/<script>.d.ts`.
- **`cubes.js` and `cube-panel.js`:** the list, each Cube's head with
  its signs and problem list, the panel with its facts and actions,
  the found Cubes. `cubesUI`, `cubesPolled`'s branches, `fillCubeHead`
  and `fillCubeFacts` go.
- **The reported fault:** a Cube's problem list stays open through a
  poll, its panel open or closed.
- **D126:** Reboot's question names the radiograms pending now; a file
  difference that comes while the panel is shown gets its row.
- Pairing and the Cube's backups stay plain inside.
- `docs/ui.md` "Cube settings"; a fragment under Fixed.

**Verification:** as for a region; tests for the fault and the two
defects, which fail on `main`.

### Block 6: Header

- `header.js`: the facts and their fitting, the Cube choice with each
  Cube's icons, the problem list, the age, the server's sign, the TLS
  notice; `home.js`: the home notice and the found Cubes' hint.
- The problem list is no longer left as it is while open: it follows.
- `docs/ui.md` "Header".

**Verification:** as for a region.

### Block 7: Rooms

- `tiles.js`, `devices.js`, `unlisted.js`, `house.js`, and the drawing
  left in `rooms.js`; the spike's tile is the start, where its scratch
  is still at hand.
- `roomUI`'s object live; `markSending`, the saved holiday draft, the
  kept tile of `renderRooms`, the House panel's `wasChecked` go.
- The configuration stays plain inside the tile.
- The two tests that assert a rebuilt tile are reworded.
- `docs/ui.md` "Rooms".

**Verification:** as for a region; the spike's twenty cases as tests.

### Block 8: RoomConfig

- `config.js`: the tabs, the room, the temperatures, the devices, the
  week programme's frame, the copy. `MassimoWeekProgram` stays the
  element it is, inside.
- `configFocused`, `deferred` and the focus-out timer go; the "changed
  meanwhile" notice stays.
- `docs/ui.md` "Rooms", the configuration.

**Verification:** as for a region.

### Block 9: Pairing

- `pairing.js`: the pairing form, the countdown, the joined devices,
  `pairDeviceForm` as the Cube panel and the devices without a room
  use it.
- `docs/ui.md` "Cube settings", pairing.

**Verification:** as for a region; `test_pairing.py` whole.

### Block 10: Admin

- `users.js`, `roles.js`, `providers.js`, `integrations.js`,
  `backup.js`: each list with its panels; a draft in one panel survives
  an action in another.
- `docs/ui.md` "System settings".

**Verification:** as for a region.

### Block 11: Log

- `log.js`: the lines, the filters and their choices, a command's fold,
  the sign on ☰.
- `docs/ui.md` "Log".

**Verification:** as for a region; `TestLogKinds` still reads `log.js`
or follows the constant.

### Block 12: Maintenance

- `update.js`, `maintenance.js`, `system-info.js`.
- `docs/ui.md` "Maintenance" and "System info".

**Verification:** as for a region.

### Block 13: Personal

- `prefs.js`, `appearance.js` with the scheme editor, the notifications
  card of `notify.js`, my profile in `users.js` and `providers.js`.
- `docs/ui.md` "Personal settings".

**Verification:** as for a region.

### Block 14: Shell

- `pages.js`: ☰, the pages and their addresses; the page's chrome in
  `components.js` (section cards, title rows, the compacting header,
  back to top); `login.js`: the dialog, the first owner's form, the
  open banner.
- `docs/ui.md` "Header" and "How the page is built".

**Verification:** as for a region; `test_scroll.py` and the header's
tests whole.

### Block 15: End

- **The data in signals** (D123-A): the loaders write `state.cubes`,
  `state.rooms` and their like as signals; the counters go.
- The builders of `components.js` nobody calls go; `data-i18n` is left
  in `index.html`'s skeleton alone.
- The guard's list is empty and removed; `tools/ui-split/` is removed.
- The tests that still reach a page global get it from one handle the
  bundle keeps for the suite.
- The contract's "The web UI" loses its words on the plain half;
  `docs/ui.md` "How the page is built", whole.

**Verification:** the whole suite; the guard over every region;
`go test ./...`; `make cross`.

## Proposed commit messages

The plan's own, when a signal first runs it:

    chore(aps): +UiParts v1.0, +ANALYSIS UiParts

    AP UiParts v1.0: the web UI's regions become Preact parts, built once and updated in place, one region at a time; the scripts are cut along the regions first

    Key changes:
    - GUIDELINES/wip/2026-11-15_16-23_AP_UiParts_regions_as_preact_parts.md: the plan, fifteen blocks
    - GUIDELINES/wip/2026-11-15_11-44_ANALYSIS_UiParts_ui_redraw_fresh_look.md: what is lost today and how it is kept, with the guard
    - GUIDELINES/wip/2026-11-15_15-30_ANALYSIS_UiParts_room_tile_both_ways.md: the room tile as a Preact part and as a Svelte part, measured

    File-revision-v1: GUIDELINES/wip/2026-11-15_16-23_AP_UiParts_regions_as_preact_parts.md::v1.0::2026-11-15::All::first revision, by PLAN+
    File-revision-v1: GUIDELINES/wip/2026-11-15_11-44_ANALYSIS_UiParts_ui_redraw_fresh_look.md::v1::2026-11-15::All::what is lost today and how it is kept
    File-revision-v1: GUIDELINES/wip/2026-11-15_15-30_ANALYSIS_UiParts_room_tile_both_ways.md::v1::2026-11-15::All::the room tile both ways, measured
    Assisted-by: claude-code:claude-fable-5-2

The blocks', by summary line:

- Block 1: `refactor(ui): The init functions wire one region each`,
  `build(ui-split): A tool moves a script's regions into scripts of their own`,
  `refactor(ui): <script> is cut along its regions` for each of the six,
  `refactor(ui): The page's regions are partials`,
  `docs(ui): The comments name the scripts as they are now`
- Block 2: `build(third_party): Preact and signals, vendored`,
  `build(ui): The page's parts are bundled by esbuild, from Go`,
  `ci(ui): The bundle is built before Go is compiled, and its types are checked`,
  `docs(contract): The web UI has one bundler and one library`
- Block 3:
  `test(ui-test): A guard: data replaces no element a user holds, and no typed text`
- Block 4: `build(ui): The API's shapes as TypeScript, from the OpenAPI file`
- Block 5: `feat(ui): The shared small elements as parts`,
  `fix(ui): A Cube's problem list stays open when data comes in`,
  `fix(ui): Reboot's question and a late file difference follow the Cube`
- Block 6: `fix(ui): The header keeps what is open in it when data comes in`
- Block 7: `fix(ui): A room tile keeps what is open, focused and typed`
- Block 8: `fix(ui): A tile's configuration is no longer built again around the user`
- Block 9: `fix(ui): The pairing form keeps what is typed`
- Block 10: `fix(ui): The settings lists keep a panel's draft`
- Block 11: `fix(ui): The log keeps the focus and a command's fold`
- Block 12: `fix(ui): Maintenance and System info follow the server while shown`
- Block 13: `fix(ui): Personal settings keep what is typed`
- Block 14: `refactor(ui): The menu, the pages and the sign-in are parts`
- Block 15: `refactor(ui): The page's data are signals`,
  `refactor(ui): The plain builders nobody calls are gone`

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN+`, with D122-D, D170-A
  and D171-A as the owner chose (D130 and D131 in the session's
  messages), D123-A to D127-A as recommended; written with D172-A and
  D173-A, which are open; Block 1 from the owner's question on cutting
  the regions out first.
