# AP GlobalMenu v1.2: one menu, four settings pages

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

Blocks 1–9 are merged (#289, #293); Block 10 follows on a branch of its own.

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
