# AP GlobalMenu v1.5: one menu, four settings pages

## TL;DR

The header keeps Rooms and ☰ only, at every width. ☰ holds every page,
and the four settings pages link to each other in their title rows.
Backups are drawn per Cube, the header shows how long ago the chosen
Cube was heard from, and the Cube choice's list shows each Cube's
problems.

- header → Rooms, ☰ (and ↑ while compacted), everywhere; 👤, Log and ⚙
  leave it (D131-C)
- ☰ → Log │ Cube settings, System settings, Maintenance, System info │
  Personal settings, Sign out (D130-A); the log's mark on ☰
- settings pages → a title row each; the other settings pages as buttons
  left of the sections menu, icons only on a phone (D132-A, D135-A)
- the buttons → in the title's place, all four in ☰'s order, the current
  one filled; on a phone the current one with its name (D157-A, D158-A);
  one page alone → its title, no buttons
- the Cube search's line beside the Cube choice → a toast at the page's
  foot, shared with notifications, Show for found Cubes (D170-A); ☰'s
  Sign out in the accent (D171-A); ☰'s mark on a disc (D172-A)
- Maintenance → its own page, `#maintenance`: Software update and
  Maintenance, out of System settings
- backups → one component per Cube; Cube settings shows its Cube's,
  System settings one fold per Cube (D133-A)
- "updated 20:51:00" → "8 s": the chosen Cube's last message, on the
  server's clock; its tooltip "updated 20:51:00, click to reload" (D134-A)
- Cube choice's list → each Cube's problem icons after its name (spec v10)
- the server's sign → in the header's pile, always a list, "Refresh
  now" first; the version → ☰'s first row, after the icon, a rule under
  it (D177-A, D185-A)
- the pile's times → two columns, the times right-aligned: "Cube read
  14 sec ago", "Server replied 5 sec ago", "Current time 09:27:08";
  the age above the pile → gone, both reads' times in expert mode's
  radio budget line; one clock for all (D180-A, D181-A)
- the header's row → the logo at the left; the pile, the Cube choice,
  ↑ ⌂ ☰ at the right (D182-A, D194-A); where it is short it folds,
  measured, step by step: the logo → the M, the Cube choice → a cube
  button that names the Cube in its tooltip, the pile → the next row
  (D195-B); the M always there
- the pile's list → as wide as its content, up to the page's (D184-A)
- a Cube named → the cube before its name, in the text's colour, its
  link sign after it; in a panel's pill the link sign leads, in the
  tick's place, and its list says the link (D190-A, D191-A, D196-A,
  D197-A)
- the House tile → named after the Cube, "All rooms of …" its tooltip
  (D193-A)
- a tile's notices → entries of its pill (D179-A); the tile's line only
  while the room's commands are refused

Blocks 1–13 are merged (#289, #293, #301, #350, #351, #355). Blocks
11–15 are built on the Preact parts of AP UiParts.

### Blocks

1. CubePageBar: Cube settings' visible title row; the panel's head
   scrolls under it
2. PageSwitch: the other settings pages as buttons in each title row
3. GlobalMenu: the header Rooms and ☰; ☰ every page and Sign out, by
   role, with the log's mark
4. MaintenancePage: Software update and Maintenance on `#maintenance`
5. Backups: one component per Cube, on Cube settings and by Cube in
   System settings
6. UpdatedAge: the chosen Cube's age in the header, and its tooltip
7. CubeListSigns: the problem icons in the Cube choice's list
8. Docs: `docs/ui.md` whole, the screenshot set, the slideshow
9. PageTabs: the settings pages' buttons in the title's place, in one
   order, the current one filled
10. SearchToast: the search's outcome in a toast; ☰'s Sign out in the
    accent and its mark on a disc
11. PileParts: `Problems` with a leading icon and a list where nothing
    is wrong, an item that leaves its menu open, `Dropdown` told of its
    closing
12. HeaderPile: the server's sign and the age in the pile, Refresh now,
    the version in ☰, the row's order, ↑ beside the Cube choice, the
    header's shots
13. TileNotices: a tile's notices as entries of its pill, one line in
    the tile
14. HeaderRow: the logo left, the pile and the Cube choice right, the
    row folding step by step, ↑ ⌂ ☰; the reads' times relative, in the
    list and in expert mode; the list's width; ☰'s version row
15. TileLine: a tile's line only while its commands are refused; the
    notices' tests on the page's own reads
16. CubeSign: the cube before a Cube's name, its link sign after it or
    leading its pill, the link's row in the pill's list; the House tile
    named after the Cube

## Discussion

The owner's look at the pages of AP PageMenu (#281), 2026-11-15, and
the answers to the TL;DR gate that followed:

- Cube settings gets the page title "Cube settings", and its Cube's
  Backup and restore card.
- The three settings pages get buttons for the other two,
  right-aligned left of the "hamburger", folded into their icons when
  the page is narrow. D132-A: the hamburger is the page's own sections
  menu, in its title row.
  - The owner asked whether more buttons should come now that the pages
    are split further. D135-A (this plan's answer): the switch holds
    every settings page, Maintenance included, so each page shows the
    other three. Personal settings and Log are not settings of the
    system and stay in ☰ only.
- System settings → Backup and restore uses the same component, the
  backups grouped by Cube in folds as in the Cubes card (D133-A).
- The header's "updated" time:
  - loses its "updated" prefix;
  - is the time the chosen Cube was last polled or heard from;
  - shows the time passed since then (D134-A: seconds under a minute,
    then minutes, hours, days);
  - its tooltip says "updated <time>, click to reload".
- ☰ comes back as the global menu, shown at every width, holding Log,
  Cube settings, System settings, Maintenance, System info, Personal
  settings and Sign out. Maintenance is a new page with the Software
  update and Maintenance cards of System settings.
  - D130-A, its order: Log first, the most used and carrying the mark;
    the settings from one Cube to the system, its upkeep, then what is
    only read; the person last, Sign out at the very end.
  - D131-C, everywhere: the header holds Rooms and ☰ alone, a phone's
    too. ☰ takes the log's warning and error mark.
- The Cube choice's open list gets the problem icons the spec foresaw
  (`cube_choice_signs_v10`, "The open Cube list"): AP CubeBadges drew
  each Cube's network sign there, not the icons after its name.

`PLAN++` on 2026-11-15: this session writes the plan and runs all eight
blocks into a pull request stacked on #281, then the completion gate. No
merge and no retirement.

The owner's look at the merged pages, 2026-11-15: the buttons move from
page to page, since the current page is left out and only System settings
and Maintenance have a sections menu at the row's end.

- D157-A: the buttons take the title's place, left-aligned, all four
  always in ☰'s order, the current one filled in accent; the title stays
  for a screen reader. Option B, the title kept and the current one
  disabled, was not taken: on a phone the row wraps on every page, the
  name shows twice and a disabled control does nothing.
- D158-A: on a phone the current one keeps its name, cut with "…" where
  the row is too narrow, the others their icon; without it the page's
  name shows nowhere on a phone.
- The owner's addition: where the role may open one settings page alone,
  its title shows and no button is drawn.

`PLAN++` on 2026-11-15 for this revision: Block 9 on a new branch from
main, to a pull request and the completion gate. No merge and no
retirement.

The owner's look after #293, 2026-11-15, at the header in German and
dark: "10 Cubes gefunden, nicht eingerichtet", the search's outcome beside
the Cube choice, should show as a toast; ☰'s Sign out should be in the
accent; and whether this needs a plan.

- D170-A: one toast for the page, the one notifications use, made a
  shared component; the search's outcome there, a found Cube with Show
  (System settings → Cubes at the found Cubes); the line beside the Cube
  choice goes.
- D171-A: ☰'s Sign out (and Forget key) in the accent and bold, as the
  personal page's button.
- D172-A: ☰'s mark on a small disc of the card's colour, so that it
  reads on the filled ☰; in the owner's screenshot it hardly shows.
- D173-A: this plan's Block 10, not a plan of its own, nor none.

`PLAN++` on 2026-11-15 for this revision: Block 10 on a new branch from
main, to a pull request and the completion gate. No merge and no
retirement.

The owner's look at the header on a phone, 2026-11-16: the server's sign
into the header's pile (the problem list beside the Cube choice), the
pile on the row's baseline, the age centred above it; in the pile's list
"Refresh now" first, with the time the chosen Cube was read, the time the
server last replied and the time now, in the browser's time zone; the
version as a small first row in ☰; the logo kept above 330 px; the pile
before the Cube choice, so that only the Cube choice wraps; ↑ right of
the Cube choice on a wide page; the age to the pile's right where the
pile has a line of its own (under about 260 px); shots of the header at
each width. Then, looking at two tiles: should a tile's notices be rows
of its pill, the tile keeping one line that opens it?

- D175-A: this plan's Blocks 11–13, not a plan of its own.
- D176-A: the M stays in the compacted phone header; under 331 px the
  logo goes, compacted or not.
- D177-A: the server's sign is the pile's first icon, in its state's
  colour, without a frame; its click's other action (Software update,
  the reload) the list's second item while it has one; unreachable
  fills the pile red.
- D178-A: the age above the pile, to its right where the pile has a
  line of its own.
- D179-A: a tile's notices (the room not ready, the stale values, the
  mismatches) as entries of its pill with their "since"; the tile keeps
  one line that opens the pill, which says so while room commands are
  discarded.

The header and the rooms became Preact parts meanwhile (AP UiParts,
Blocks 6 and 7). Asked on the owner's word, the session of AP UiParts
moved them unchanged and sent a briefing on 2026-11-17 for building the
change on the parts; what the change removes (the facts row, the version,
the refresh button) it left plain. The owner sent `PLAN++` with all as
recommended, stacking onto #347 (AP UnsavedMarks, Block 3) rather than
starting from main: one branch and pull request per block, Block 11 on
#347's branch, Blocks 12 and 13 on Block 11's. No merge and no
retirement.

The owner's look at the merged header on a phone and at a tile,
2026-11-18: the times should be aligned under each other, leading the
rows or right-aligned; relative, "{n} {unit} ago"; "Current time" for
"Now"; the pile and the Cube choice at the page's right, next to ⌂; the
open list wider, up to the screen's or `main`'s width; the logo goes
below 600 px already, not as agreed (the image is the `logo` or the
`icon`, as the brand's files are named); the age above the pile does not
agree with the list's times; the age out of the pile, into expert mode's
line of the Cube's radio budget; a rule after ☰'s version and a small M
before it. Of the tiles: the notices are in the pill now, but the tile's
line should go but where it says that the room's commands are refused.

The age and the list's times use the same clock and the same skew, but
tick on two timers: the age on its own, at its value's whole seconds,
the list each second from its opening; each cuts to the second, so the
two can differ by a second and change at different moments.

- D180-A: two columns, the label and its time, the times right-aligned;
  the two reads relative, their exact time the tooltip; one clock that
  ticks each second for the list and the expert line, one formatter.
- D181-A: the age above the pile goes; in expert mode, the radio budget
  line has both reads' times, "Cube read" and "Server replied".
- D182-A: from 600 px, the logo at the left, then at the right the pile,
  the Cube choice, ↑ while compacted, ⌂ and ☰.
- D183-A: below 600 px, the first row the whole logo (the `logo`, not
  the `icon`) with ⌂ and ☰, the second the pile and the Cube choice, at
  the right; under 331 px the logo goes. This replaces D176-A, the M on
  a phone, which predated the owner's word and was not what was meant.
- D184-A: the pile's list as wide as its content, up to `main`'s width,
  under the pile at its right edge, kept in the window.
- D185-A: ☰'s version row: the small `icon` before the version, a rule
  after it.
- D186-A: this plan's Block 14, not a plan of its own.
- D187-B: the notices' tests, moved to the page's own reads (AP
  TestInfrastructure, Block 7), ride with this revision's blocks, not a
  pull request of their own.

The tile's line is the owner's own addition, Block 15. `PLAN++` on
2026-11-18 for this revision: Blocks 14 and 15 on one branch from main,
one pull request, a second reader before it; no merge and no
retirement.

The owner's second look, the same day, before the pull request of
Blocks 14 and 15: on a phone the Cube choice on the second row should
take the room left and push the pill to the far left, and the pill take
the logo's place where the logo goes; the cube's symbol in front of a
Cube's name and its network sign after it, always, in its state's
colour. The evaluation that followed (D188–D192) was overtaken by the
owner's rethinking: the House tile named after the Cube; ↑ left of ⌂;
the Cube choice a button where it would wrap, its tooltip the Cube; the
logo the M, there always; the pill to the next row where nothing else
gives; which folds first; the link sign in a pill where the Cube has one,
in the tick's place, and a row of its own in the pill's list.

- D188 and D189 (the phone's second row, the pill in the logo's place):
  withdrawn, the folding of D195 takes their place.
- D190-A: the cube before a Cube's name wherever a Cube is named with
  its sign; its link sign after the name, the first of its icons there,
  always, green, amber or red; where the Cube has a pill (its panel's
  head), the link sign leads the pill instead.
- D191-A: the cube in the text's colour, grey in a menu's list as ☰'s
  icons; a blue cube is a Cube found and not set up.
- D192: replaced by D198.
- D193-A: the House tile's title the chosen Cube's name with the cube
  before it, its tooltip and its name for a screen reader "All rooms of
  …".
- D194-A: ↑ ⌂ ☰ at every width, one ↑.
- D195-B: where the row is short, first the logo becomes the M, then the
  Cube choice a cube button, its tooltip the chosen Cube, then the pile
  goes to the next row, at the left; measured, as if ↑ were there, so
  that scrolling never changes the fold.
- D196-A: in a Cube's own pill the green link sign stands for the tick;
  the header's pile keeps its tick, its lead being the server's sign.
- D197-A: the panel's pill always a list, the Cube's link in words its
  first row, "connected" too; a broken link's badge is that row.
- D198-A: this revision rebuilds Block 14, not pushed, and adds Block
  16; Blocks 14 to 16 in one pull request.

`PLAN++` on 2026-11-18 for this revision, as recommended: Block 14
rebuilt, Block 15 as it is, Block 16, one pull request after a second
reader; no merge and no retirement.

Out of scope: the redraws that close an open pill (the evaluation of
2026-11-15, D118–D121, waits for the chip session's fresh look).

## Analysis

- **The header** (`index.html` `nav.nav`): Rooms `#view-tiles`, 👤
  `#view-me`, Log `#view-log`, ↑ `#top-btn`, ⚙ `#gear-menu` and ☰
  `#nav-menu`, ☰ shown below 600 px only. `log.js` `VIEW_BUTTON`,
  `GEAR_VIEWS`, `showView` and `renderLogSign` know each by id; the
  log's mark on ☰ exists already (`.mark-sign`, a phone's).
- **Sign out** is the personal page's `#logout-btn` (`login.js`): Sign
  out for a user, Forget key for a token, none on an open server. ☰'s
  entry does the same, by the same label.
- **The pages' rows** (`components.js` `pageBar`): System settings and
  the personal page have a `.page-bar` with `.sec-menu`. System info has
  a row with its badges and no menu. Cube settings has a visually
  hidden heading, its panel's head sticky in its place (PageMenu NOTE 5).
- **System settings** (`ui.go` `uiPartials`, marker `system`):
  `software.html`, `maintenance.html`, `cubes.html`, then the flow, then
  `backup.html`. `update.js` loads the update, the restart, the reboot
  and the announcement on `onView("system")`.
  - `GET /api/v1/update` needs `read`. Software update shows to every
    role that has System settings today (`hasSystem`), so Maintenance's
    page shows to the same roles.
  - System settings shows without the capabilities only Maintenance
    uses (`update`, `restart`, `reboot`), so that no role gets an empty
    page.
- **Backups** (`backup.js`): one card with fixed ids (`#backup-take`,
  `#backups-list`, `#restore-box`, …) for the header's Cube
  (`base()`), its state in one `backupUI`. Every route is per Cube
  (`/api/v1/cubes/{sn}/backups…`), so a component per serial needs no
  API change.
  - `restoreEvent` finds the run by its id.
  - The ids of a box carry its place (`cube-` or `sys-`), as the same
    Cube can be drawn on both pages.
- **The age:** `GET /api/v1/cubes` gives each Cube's `updated`, the time
  the server last took a message from it (`cubelan` `Snapshot.apply`):
  "last polled or heard from".
  - It is the server's time. The page measures it against the server's
    clock: the offset from the poll's `Date` header, as System info
    measures its own (`sysClock.skew`).
  - `renderUpdated` writes `header.updated` from the browser's last
    poll; `fitFacts` refits the facts when their text changes (#272's
    fix), so a ticking age keeps them fitted. Tabular digits keep its
    width from jumping.
- **The Cube choice** (`app.js` `renderCubeSelect`, `components.js`
  `pickerMenu`): each item has an icon, a label and a sub line.
  `problemsEl` draws the kinds of a set of entries as icons, the worst
  tone each. The list takes the same per Cube, the network kind left out
  (the sign before the name is it). The proxy and the settings file go
  under the cog for the settings right, as in the header's pill
  (D85-C).
- **The icon of Maintenance:** the sprite has none. Tabler Icons' `tool`
  (MIT, `third_party/tabler-icons/`) is drawn in the same box and stroke.
- **The switch** (after Block 4, `components.js` `pageSwitch`): built
  once per page at the start (`log.js` `initLog`) in a `.page-tools`
  group, right-aligned, with the sections menu; a role's pages hidden by
  their `needs-*` class, as ☰'s entries are. Cube settings and System info
  have no sections menu; System info has its badges in the row
  (`update.js` `fillProxies`, at most one).
  - Which pages a role may open is `mayShow`, applied in `login.js`
    `applyCaps` whenever the signed-in user changes: the one place where
    the count of a role's settings pages is known.
  - On a phone, the current one's name makes it wider, so the buttons
    after it shift with the page shown; their order, and those before it,
    stay. At full width every button keeps its place.
  - With the current page's name shown on its button, the title (`h2`)
    stays in the row, visually hidden, still the page's name for a screen
    reader (each page's `aria-labelledby`).
- **The search** (`app.js` `discoverCubes`): the Cube choice's entry
  starts it and reads "Searching…" while it runs, disabled; `showSearch`
  writes `#cube-search` beside the Cube choice: "Searching…", then the
  outcome for 8 s, a failure in red for 15 s. `showFound` opens System
  settings → Cubes with the found Cubes' fold open.
- **The toast** (`index.html` `#notify-toast`, `notify.js` `notifyToast`):
  a title and a text at the page's foot, × closes it, 10 s; hidden while
  signed out. Its ids are the notifications'; `test_notifications.py`
  reads them.
- **☰'s mark** (`log.js` `renderLogSign`, `markEl`): the amber triangle
  or red cross at ☰'s corner, in `--warn-mark` or `--bad`, blinking as the
  person chooses. On the filled ☰ (`--accent`, a light blue in the dark
  theme) amber has little contrast.
- **☰'s Sign out** (`#menu-logout`, `login.js`): a menu entry like the
  others; the personal page's `.logout-btn` is in the accent, bold.
- **Tests:** `helpers.py` `show_view` and `open_personal_page` open
  every page through ☰ after Block 3. The tests of the cards that move
  open `maint` after Block 4, and `test_backup.py` follows the component
  in Block 5.

For Blocks 11–13, on #347's branch:

- **The header's parts** (`ui/src/header/`): `index.tsx` mounts the Cube
  choice with every Cube's problem list into `#header-cube`, the age into
  `#updated` inside the plain `#refresh-btn`, the server's sign
  (`ServerSign.tsx`, an `IconButton`) into `#server-sign-slot`; `store.ts`
  holds the region's counter `load`, `send.ts` `pollNow`, which turns
  `#refresh-btn` while it polls. Plain: the facts row (`.top-left`,
  `.top-facts`, `fitFacts`, `watchFacts`), `#version-short` and
  `#refresh-btn` in `header.js` and `partials/header.html`; ☰ and
  `#top-btn` until UiParts' Block 14.
- **The shared list** (`ui/src/shared/problems.tsx`): `Problems` draws
  `OkPill`, a pill without a list, where nothing is wrong, and takes no
  element before the kinds' icons. An `Item` has `action` (its badge a
  button) and `at` (an age), no "since". `Dropdown` (`menu.tsx`) tells
  its opening (`onOpen`), not its closing; every menu goes through the
  plain `dropdown()` of `components.js`, which closes it on a click on
  any button of its list. `docs/ui.md` names "an item whose action
  leaves the list open" as a gap for "Refresh now".
- **The server's answers:** `api()` (`app.js`) passes each to
  `noteServerClock`; nothing keeps when the last one came.
- **The tile** (`ui/src/rooms/Tile.tsx`): `NotReady`, the stale
  `Notice` and `Mismatches` under the head, each a notice with its
  "since" (`Notice`'s `since`); the pill's entries are `roomEntries`
  (`rooms/facts.ts`), per device, where a silent device already has its
  "not reported" badge.
- **The rules:** a shared part knows no region and gets a property, with
  its row in `docs/ui.md`, from the block that first needs it; a part
  looks up no id it draws with `$()`; no `data-i18n` in a part; `make ui`
  before Go builds, `make ui-check` for the parts. The parts' guard
  (`test_parts_guard.py`) has the view `header_problems`; its `main-menu`
  view names `.top-facts`, which Block 12 removes.

For Blocks 14–15, on main:

- **The header's row** (`style.css` `.top-line`): a grid on its lower
  edge, `brand pile cube up . nav`; below 600 px `brand pile . nav` and
  the Cube choice on a second row, or the pile on a line of its own,
  which `Pile.tsx` measures with a `ResizeObserver` (`ownLine`); under
  331 px no logo. Below 600 px `.top .brand-name` shows the `icon` (the
  sprite's `mark-*`) in place of the `logo` (`logo-*`), a rule of AP
  WebBrand; `brandMarks` (`components.js`) draws both into every
  `[data-brand]`, and the CSS shows one. The logo is 36 px high and 5.33
  times as wide, 192 px.
- **The pile** (`ui/src/header/Pile.tsx`): `Updated` (the age, its own
  timer) above the pill; `RefreshNow`'s times one string of three lines,
  `MenuItem`'s `sub`, which takes a string alone; its own timer while
  the list is open. Its list opens to the right (`.cube-problems
  .problems-list`), `min-width` 240 px, as wide as the window less 32 px.
- **The expert line** (`#cube-stats`, plain, `header.js` `renderHeader`):
  the Cube's name and its radio budget, each value set by its id.
- **The width of `main`:** the body is at most 1100 px with 16 px
  gutters, so the page's content is `min(100vw, 1100px) - 32px`.
- **The tile** (`ui/src/rooms/Tile.tsx` `NoticeLine`): the line counts
  the notices (`roomNotices` of `facts.ts`, `room.notices` in the six
  languages), or says that the room's commands are refused.
- **The notices' tests** (`test_rooms.py`) set rooms and devices in the
  page; the page's next read of the rooms takes them back
  (`docs/ui-testing.md` §9). The fixture `reads` answers the read of the
  rooms (`**/rooms?devices=all`) with what a test puts in.

For Block 16, on main:

- **A Cube's sign** is the network sign in its link's state (`CubeSign`
  of `model/cube.tsx`, `cubeSignEl` of `header.js` for the plain
  regions), before the Cube's name in the Cube choice and its list
  (`header/Choice.tsx`, a `Picker`), in each Cube's heading of the
  pile's list (`cubeEntry`, its `sign`), in a Cube's panel head
  (`cubes/Panel.tsx`) and in a Cube's backup fold (`backup.js`).
- **The panel's pill** is a `Problems` with `ok`: the green check where
  nothing is wrong, the Cube's heading in its list with the link's badge
  only where it is not connected; its summary's icons count the link's
  kind, `net`.
- **A blue cube** (`icon-cube3d` in the accent) is the pile's sign of
  the Cubes found and not served.
- **The House tile's title** is plain markup (`partials/house-tile.html`,
  `house.title`), the tile's part drawn into `#house-slot` under it.

## Implementation Plan

Each block carries its own texts in the six languages, `docs/ui.md`, its
tests and its CHANGELOG fragment. Where a fragment of #281 says what
this plan changes (⚙'s menu, Cube settings' head, the back-to-top
placement), the fragment is corrected in place: none is released.

### Block 1: CubePageBar

- `index.html`: `#cube-page` gets a `.page-bar` with the visible `h2`
  "Cube settings", like the other pages.
- `style.css`: the panel's head is no longer sticky; it stays the
  panel's first row, open and unfoldable.
- `docs/ui.md` "Cube settings"; the fragment `claude-cube-page.md`.
- Tests: `test_cube_page.py`, the title row shown and kept under the
  header, the panel's head scrolling.

**Verification:** `make ui-test` with `test_cube_page.py` and
`test_gear_menu.py`, 1100 and 390 px.

### Block 2: PageSwitch

- `components.js` `pageSwitch(page)`: in a page's row, right-aligned
  before its sections menu, a button for every other settings page the
  role may open (`mayShow`), its icon and name; below 600 px the icon
  alone, labelled.
- The settings pages, in ☰'s order: `cube`, `system`, `info`; Block 4
  adds `maint`.
- System info's and Cube settings' rows get the same structure.
- `docs/ui.md`; a fragment.
- Tests: `test_page_switch.py`, the buttons by role on every settings
  page, the click, the icons only at 390 px.

**Verification:** `make ui-test` with the new file and the pages' tests.

### Block 3: GlobalMenu

- `index.html`: the header's nav is Rooms, ↑ and ☰; 👤, Log and ⚙ go;
  ☰ is shown at every width.
- ☰'s entries, by role (`needs-*`):
  - Log
  - Cube settings, System settings, System info
  - Personal settings, Sign out
  - with a separator between the groups.
- Sign out does what the personal page's button does, by its label, and
  is absent where that button is.
- `log.js`: `VIEW_BUTTON` keeps Rooms; `MENU_VIEWS` takes `GEAR_VIEWS`'
  place with every page ☰ opens; ☰ is active, and its entry current,
  while one of its pages is shown; `renderLogSign` draws the mark on ☰
  and the sign on its Log entry.
- `style.css`: the rules of ⚙ and the per-width hiding go.
- i18n: ☰'s name "menu"; the gear's keys go.
- `docs/ui.md`, the header; the fragments `claude-gear-menu.md`,
  `claude-to-top.md`, `claude-system-info-page.md` and
  `claude-cube-page.md` corrected for ☰.
- Tests: `helpers.py` opens pages through ☰; `test_gear_menu.py`
  becomes `test_main_menu.py` (the order, the groups, the roles, the
  current entry, Sign out, the mark); `test_header.py`,
  `test_log_alerts.py`, `test_menu_fit.py`, `test_settings.py` and
  `test_to_top.py` follow.

**Verification:** `make ui-test` whole, 1100 and 390 px.

### Block 4: MaintenancePage

- `ui.go`: a marker `maint` with `software.html` and `maintenance.html`,
  out of `system`.
- `index.html`: `#maint-page` with its row (title, switch, sections
  menu); ☰'s entry Maintenance between System settings and System info.
- `log.js`: the view `maint` at `#maintenance`.
  - `mayShow`: `maint` with `hasSystem()`; `system` with
    `hasSettingsPage()`, the same without `update`, `restart` and
    `reboot`.
  - `login.js` sets the body's `no-maint` and `no-system` by them.
- `update.js`: its loads on `onView("maint")`.
- The sprite: `icon-tool`, Tabler's `tool`; `third_party/tabler-icons/MASSIMO.md`.
- `docs/ui.md` "Maintenance"; a fragment.
- Tests: the cards' tests open `maint`; `test_system_layout.py`'s cards;
  the switch and ☰ with Maintenance.

**Verification:** `make ui-test` whole; `go test ./internal/server/`
(the partials).

### Block 5: Backups

- `backup.js` becomes `backupBox(serial, place)`, built from a
  template, its state per serial (`backupUI.of[serial]`):
  - the bar (format, download, take, restore from file), the pending
    line, the list, the restore and the outcome;
  - its ids carrying its place;
  - `restoreEvent` finds the run among them.
- Cube settings: a "Backup and restore" section card under the panel,
  for the chosen Cube, with `backup`.
- System settings: the card holds one fold per Cube, its sign, its
  name, its serial and the count kept, like the Cubes card; a single
  Cube's fold open.
- `docs/ui.md`; the fragment.
- Tests: `test_backup.py` on both pages; two Cubes in System settings.

**Verification:** `make ui-test` whole.

### Block 6: UpdatedAge

- `app.js`:
  - `renderUpdated` shows the chosen Cube's `updated` as an age: seconds
    under a minute, then minutes, hours, days;
  - it ticks each second while under a minute, each minute after;
  - it measures against the server's clock, the offset from the poll's
    `Date` header;
  - nothing shows where the Cube has no `updated`.
- The refresh button's tooltip and name: "updated 20:51:00, click to
  reload".
- `style.css`: tabular digits.
- i18n: `header.updated` goes; the tooltip key reworded.
- `docs/ui.md`; a fragment.
- Tests: `test_header.py`, the age from a Cube's `updated` set in the
  page, its steps, the tooltip; the tests that read `#updated`.

**Verification:** `make ui-test` whole.

### Block 7: CubeListSigns

- `components.js` `pickerMenu`: an item's `icons` after its label.
- `app.js` `renderCubeSelect`: each Cube's problem kinds as icons, the
  worst tone each, without the network's; the proxy and the settings
  file under the cog for the settings right.
- `docs/ui.md`; a fragment.
- Tests: a Cube with a device problem and one without, in the open list.

**Verification:** `make ui-test` with `test_header.py` and the Cube
choice's tests.

### Block 8: Docs

- `docs/ui.md`: the header, ☰ and the pages read as one.
- The screenshot set, carrying the session's earlier change to it (the
  new pages' shots, System info's machine fixed):
  - ☰ open in place of ⚙;
  - Cube settings, Maintenance and System info, at 1100 and 390 px;
  - the slideshow.
- `tools/ui-test/README.md`.

**Verification:** `make ui-screenshots` and `slideshow.sh`; each new
shot looked at; `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt`, the three `go vet`, `GUIDELINES/shared/tools/check`.

### Block 9: PageTabs

- `components.js` `pageSwitch`: every settings page, in ☰'s order, at the
  row's start; the current one filled in accent, `aria-current="page"`, a
  click on it opening nothing; the title visually hidden while the
  buttons show.
- `style.css`: below 600 px the others their icon alone, the current one
  its icon and its name, cut with "…" where the row is too narrow; the
  sections menu and System info's badges at the row's right end;
  `.page-tools` and Block 4's wrapping go.
- `log.js`: where the role may open one settings page alone (`mayShow`),
  the rows show their title and no button, set from `login.js`
  `applyCaps`.
- `docs/ui.md`: the header's list, the pages, the components; the
  fragment `claude-page-switch.md` corrected in place (not released).
- `screenshots.py`: the pointer moved off the page before each shot; the
  four settings pages taken again at 1100 and 390 px, and the slideshow.
- Tests: `test_page_switch.py`, every page's four buttons in order, the
  current one filled and marked, each button's place at full width and
  before the current one on a phone, the current one's name on a phone,
  the sections menu on the same line, no horizontal scroll; a viewer's
  System info with its title and no button; `test_cube_page.py`'s row.

**Verification:** `make ui-test` whole, 1100 and 390 px; `go test ./...`,
`make cross`, `gofmt`, the three `go vet`,
`GUIDELINES/shared/tools/check`; each new shot looked at.

### Block 10: SearchToast

- `components.js` `showToast(text, {title, ms, tone, action})`: the
  page's one toast, a newer in the place of the one shown; `bad` a red
  edge; an action a button that runs it and closes the toast.
  - `index.html`: `#toast` in place of `#notify-toast`, with the action's
    button.
  - `notify.js` `notifyToast` shows through it.
- `app.js` `discoverCubes`: the outcome as a toast, 8 s; a found Cube
  with Show (`showFound`) for a role that may open System settings; a
  failure in red, 15 s. `showSearch` and `#cube-search` go; the Cube
  choice's entry says "Searching…" as before.
- i18n: `discover.show` in the six languages.
- `style.css`: ☰'s Sign out in the accent and bold; ☰'s mark on a disc
  of the card's colour; the toast's `bad` edge.
- `docs/ui.md`: the search, the toast, ☰'s Sign out and mark; a fragment
  for the toast; the fragment of ☰ corrected in place (not released).
- Tests: `test_discover.py` (the outcome in the toast, Show, a refused
  search in red); `test_notifications.py` (the toast's ids);
  `test_main_menu.py` (Sign out in the accent, the mark's disc).
- The screenshot set taken again where it changes.

**Verification:** `make ui-test` whole; `go test ./...`, `make cross`,
`gofmt`, the three `go vet`, `GUIDELINES/shared/tools/check`.

Blocks 11–13 are verified as D174 has it: locally the UI test files a
block touches, in all four cases, `make ui`, `make ui-check`, the Go
tests of `internal/server`, `gofmt`, the three `go vet` and
`GUIDELINES/shared/tools/check`; the whole UI suite, `make race` and
`make cross` are CI's. Before each pull request a second reader, who has
not seen this plan's reasoning, checks the block for reuse against the
shared parts and the code still plain.

### Block 11: PileParts

The shared properties Block 12 needs, each in `ui/src/shared/` and its
row in `docs/ui.md`:

- `Problems`: `lead`, an element before the kinds' icons in the
  summary, shown also where nothing is wrong; `always`, the list there
  where nothing is wrong too, its summary `lead` and the green check, its
  list `ok` as a line; `head`, what the list shows before its entries;
  `onOpen` and `onClose`, passed to its `Dropdown`.
- `Dropdown`: `onClose`, told each time it closes.
- `MenuItem`: `keepOpen`, a click on it leaves its menu open:
  `dropdown()` of `components.js` closes a menu on a click on any button
  of its list but one marked `data-keep-open`; `docs/ui.md`'s gap goes.

**Verification:** `make ui-check`; the Go tests of `internal/server`
(the parts' rules); `test_problem_list.py`, `test_header.py` and
`test_parts_guard.py` unchanged and passing (the properties change
nothing where they are not given); their behaviour is tested by Block 12.

### Block 12: HeaderPile

- `partials/header.html`: the row is the logo, `#header-pile`, the Cube
  choice in `#header-cube`, ↑ beside it (shown from 600 px while
  compacted), and the pages; the facts row, `#refresh-btn` and
  `#server-sign-slot` go; the version a first row of ☰'s list, and a
  copy beside the logo shown only while signed out.
- `ui/src/header/Pile.tsx`, in `#header-pile`: the age above the pile;
  the pile a `Problems` with `lead` the server's sign (its icon in its
  state's colour; unreachable fills the pill red and blinks), `always`,
  `head` "Refresh now" (`keepOpen`, turning while it polls) with its
  three times, and while the sign offers one, the list's second item:
  Software update, or the reload for the new version. While the list is
  open the times move on each second, and stop when it closes. The pile's
  label names the server's state and the problems. Where the logo, the
  pile and the pages do not fit one line, measured in the part's effect
  with a `ResizeObserver`, the pile takes a line of its own and the age
  moves to its right. `ServerSign.tsx` and `CubeProblems.tsx` go into it;
  `Updated.tsx` stays the age.
- `app.js` `api()`: the time of the server's last answer, kept in
  `state`; `send.ts` `pollNow` turns the Refresh now item.
- `header.js`: `fitFacts`, `watchFacts`, the refresh button's tooltip
  and `renderVersionTitle`'s fitting go; `checkVersion` fills both
  version elements; `prefs.js`'s wiring of `#refresh-btn` goes.
- `style.css`: the row a grid, its items on its lower edge (the logo's
  baseline, the buttons' lower edge); below 600 px the Cube choice alone
  on the second row, compacted too; under 331 px no logo; the age over
  the pile, beside it on a line of its own; ↑ after the Cube choice from
  600 px, between ⌂ and ☰ below; the facts row's rules go.
- i18n: the three times' words, the list's items, in the six languages.
- `docs/ui.md` "Header"; a fragment; `claude-gear-menu.md` or the
  header's fragment corrected where not released.
- Tests: `test_header.py`, `test_header_updates.py`,
  `test_server_sign.py`, `test_problem_list.py`, `test_menu_fit.py`,
  `test_main_menu.py`, `test_to_top.py`, `test_login.py` and the guard's
  views (`main-menu` without `.top-facts`; the pile's list open while it
  polls): the order, the lower edge, the age's place, the list open with
  Refresh now and its times moving, the sign's states and their item, the
  version in ☰ and signed out, ↑'s two places, the logo by width.
- `screenshots.py`: the header alone at 1100, 1100 compacted, 700, 390
  and 320 px compacted.

**Verification:** as above; each new shot looked at.

### Block 13: TileNotices

- `ui/src/shared/problems.tsx`: an `Item`'s `since`, an API time, drawn
  after its text as `Notice` draws it; its row in `docs/ui.md`.
- `rooms/facts.ts` `roomEntries`: a mismatch an item of its device's
  entry (the room's own entry where it names no device of the room); the
  stale values an item of their device; the room not ready an item of the
  room, its silent devices keeping their own badges; each with its
  "since" and a kind of its own for the summary's icons.
- `rooms/Tile.tsx`: `NotReady`, the stale `Notice` and `Mismatches` go;
  one line under the head while there is any of them, which opens the
  pill, and says that room commands are discarded while the room is not
  ready.
- i18n: the line and the new kinds' names, in the six languages.
- `docs/ui.md` "Rooms"; a fragment.
- Tests: the tiles' tests of the notices (`test_room_confirm.py`,
  `test_mismatch*.py` or where they are) read the pill and the line.

**Verification:** as above.

### Block 14: HeaderRow

- `partials/header.html` and `style.css`: the row a flex row on its
  lower edge, the logo at the left, the pile, the Cube choice and ↑ ⌂ ☰
  at the right; one ↑, before ⌂, while compacted, at every width.
- `ui/src/header/fold.ts` (new): the row's fold, measured whenever the
  row, the logo, the pile or the Cube choice change size, as
  `data-fold` on the row: 0 whole; 1 the logo the M; 2 the Cube choice
  a cube button, its name, link and chevron hidden, its tooltip the
  chosen Cube; 3 the pile on the next row, at the left. Measured as if
  ↑ were there; the M always shown.
- `ui/src/header/clock.ts` (new): one clock, a signal that ticks each
  whole second and at each render, and `fmtAgo` ("14 sec ago",
  `Intl.RelativeTimeFormat` in the page's language), shared by the list
  and the expert line.
- `Pile.tsx`: the pill alone; `RefreshNow`'s times a grid of two
  columns, "Cube read" and "Server replied" relative with their exact
  times as tooltips, "Current time" a clock; `ownLine` and its observer
  go. `Updated.tsx` gives way to the expert line's times.
- `shared/menu.tsx`: `MenuItem`'s `sub` takes elements too; its row in
  `docs/ui.md`.
- `partials/header.html`: in `#cube-stats` a slot for the reads' times;
  ☰'s version row with the `icon` before the version and a rule after
  it.
- `style.css`: the pile's list as wide as its content, up to `main`'s
  width, at the pill's right edge; the times' grid; ☰'s version row.
- i18n: "Cube read", "Server replied", "Current time" without the
  time, in the six languages.
- `docs/ui.md` "Header"; the header's fragment corrected (not released).
- Tests: `test_header.py` (the fold's steps in their order, nothing
  outside the window, the fold the same compacted, the cube button),
  `test_to_top.py`, `test_server_sign.py`, `test_main_menu.py`,
  `test_brand.py`, `test_system_layout.py` where they read the row, the
  age or the times.
- `screenshots.py`: the header's shots and the pile's list open; the set
  and the slideshow taken again.

**Verification:** as above; each header shot looked at, at 1100, 700,
390 and 320 px.

### Block 15: TileLine

- `rooms/Tile.tsx`: the line only while the room's commands are
  refused, its Show opening the pill; `roomNotices` and the texts of
  the count go.
- `test_rooms.py`: the notices' tests put their rooms into the page's
  own reads (`reads`, `ROOMS`); the stale and mismatch tests open the
  pill by its summary.
- `docs/ui.md` "Rooms"; the tiles' fragment corrected (not released).

**Verification:** as above.

### Block 16: CubeSign

- `model/cube.tsx`: `CubeIcon`, the cube in the text's colour;
  `cubeEntry` with the cube before the name and the link sign after it,
  the link's badge where it is not connected, and for the Cube's own
  pill the link's row always, "connected" too.
- `shared/problems.tsx`: an entry's `tail`, drawn after its name, and
  `Problems`' `skip`, kinds its summary leaves out (the link the lead
  shows); their rows in `docs/ui.md`.
- `header/Choice.tsx`: the cube before the name, the link sign the
  first of the icons after it, in the summary and in each row.
- `cubes/Panel.tsx`: the cube before the name; the pill led by the link
  sign, in the tick's place, its list the link's row first.
- `backup.js`: a Cube's backup fold, the cube before the name and the
  link sign after it.
- `partials/house-tile.html` and `rooms/House.tsx`: the House tile's
  title the chosen Cube's name with the cube, "All rooms of …" its
  tooltip and its name for a screen reader, in the six languages.
- `docs/ui.md` "Header", "System settings", "Rooms"; a fragment.
- Tests: the Cube choice, the pile's list, the Cubes' panels, the
  backups' folds and the House tile where they read a Cube's sign or the
  title.

**Verification:** as above; the shots taken once, after Block 16.

## Proposed commit messages

- Block 1: `feat(ui): Cube settings has its title row`
- Block 2: `feat(ui): The settings pages link to each other in their title rows`
- Block 3: `feat(ui): ☰ holds every page at every width, the header Rooms and ☰`
- Block 4: `feat(ui): Maintenance is a page of its own`
- Block 5: `feat(ui): Backup and restore per Cube, in Cube settings and System settings`
- Block 6: `feat(ui): The header shows how long ago the chosen Cube was heard from`
- Block 7: `feat(ui): The Cube choice's list shows each Cube's problems`
- Block 8: `docs(ui): The menu and the pages in docs/ui.md and the screenshot set`
- Block 9: `feat(ui): The page buttons take the title's place, the current one filled`
- Block 10: `feat(ui): The search's outcome in a toast, ☰'s Sign out and mark clearer`
- Block 11: `feat(ui/shared): A problem list leads with an icon, lists when all is well`
- Block 12: `feat(ui/header): The server's sign and the age in the pile, the version in ☰`
- Block 13: `feat(ui/rooms): A tile's notices are entries of its pill`
- Block 14: `feat(ui/header): A header that folds, the reads' times relative`
- Block 15: `feat(ui/rooms): A tile's line only while its commands are refused`
- Block 16: `feat(ui): A Cube's cube before its name, its link after it`

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN++`, with D130-A,
  D131-C (everywhere), D132-A, D133-A and D134-A as the owner chose;
  D135-A this plan's answer to the owner's question on more buttons.
- v1.1 (2026-11-15): Block 9, PageTabs, by `PLAN++`, with D157-A and
  D158-A as the owner chose and the owner's rule for a role with one
  settings page; the TL;DR names the merge of Blocks 1–8 (#289).
- v1.2 (2026-11-15): Block 10, SearchToast, by `PLAN++`, with D170-A,
  D171-A, D172-A and D173-A as recommended; the TL;DR names the merge of
  Block 9 (#293).
- v1.3 (2026-11-17): Blocks 11–13, the header's pile and a tile's
  notices on the Preact parts, by `PLAN++` with D175-A to D179-A as
  recommended, stacked on #347; the TL;DR names the merge of Block 10
  (#301).
- v1.4 (2026-11-18): Blocks 14 and 15, the header's row and times and
  the tile's line, by `PLAN++` with D180-A to D186-A and D187-B, the
  times right-aligned and both reads in the expert line as the owner
  added; D183-A replaces D176-A; the TL;DR names the merge of Blocks
  11–13 (#350, #351, #355).
- v1.5 (2026-11-18): Block 14 rebuilt as the folding header with ↑ ⌂ ☰,
  and Block 16, a Cube's sign and the House tile's title, by `PLAN++`
  with D190-A as the owner refined it and D191-A, D193-A to D198-A as
  recommended; D188 and D189 withdrawn, D192 replaced by D198.
