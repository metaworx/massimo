# AP PageMenu v1.0: the ⚙ menu and its pages

## TL;DR

⚙ becomes a menu of three pages: the chosen Cube's settings, System
settings, and System Info, which every signed-in user may read; a
back-to-top button follows every page.

- ⚙ → a menu: Cube settings, System settings, System info, each for the
  roles that may open it; on a phone under ☰'s Log
- System info → a page of its own for every role with `read`, out of
  System settings
- Cube settings → the chosen Cube's panel, open, its head the page's
  title row
- back to top → filled in the accent blue on every page; a second in
  the compacted header, left of ⚙ or ☰

### Blocks

1. GearMenu: ⚙ a menu, ☰ on a phone; tests
2. SystemInfoPage: System Info a page of its own, for every role; tests
3. CubePage: the Cube settings page; tests
4. ToTop: both back-to-top buttons; i18n, `docs/ui.md`

The four blocks are one pull request, after AP UiPolish's.

## Discussion

The owner's look at the UI on 2026-11-14 night, weighed in three EVAL
rounds; `PLAN++` on 2026-11-15:

- the back-to-top button filled in the accent colour, and another left
  of ☰, since the pages keep their scroll position (D108-A: shown while
  the header is compacted; D109-A: on every page, a screen down);
- a "Cube Settings" page, a copy of a Cube's settings card for the
  current Cube (D111-A: open, its head the title row, drawn only where
  shown);
- ⚙ a dropdown, as 👤 was, with Cube settings, System settings and
  System info, the last only the info pane, perhaps for everyone
  (D116-A: every signed-in role; D117-A: its own page only);
- D114-A, D120-A: this plan after AP UiPolish, stacked on its branch.
  The plans stay open until the UI is fixed (the owner).

## Analysis

**The nav** (`index.html` `nav.nav`): Rooms, 👤, Log, ⚙ buttons; below
600 px Log and ⚙ hide and ☰ (`#nav-menu`, Log and System settings)
shows. `showView` (`log.js`) shows a view, keeps its scroll
(`viewScroll`), sets the address (`VIEW_HASH`); `mayShow` refuses a page
a role may not open.

**System Info** (`partials/system-info.html`, `update.js`
`loadSystemInfo`): a card of System settings for roles with one of its
capabilities (`needs-system`); `GET /api/v1/system` needs `read` only.
Store and Settings in effect at its end are `needs-settings`. Its rows
have ids, so it lives in one place.

**A Cube's panel** (`cubes.js` `cubePanel`): ids made from the serial
(`cube-<serial>-<field>`, `pair-seconds-<serial>`,
`cube-devices-<serial>`), so it is drawn in one place at a time; its
drafts and pairing state are kept per serial (`cubesUI`).

**Back to top** (`components.js` `backToTop`): a fixed round button on
System settings and the personal page, shown once its first card's head
is out of sight; the header compacts past 96 px (`stickyHeader`).

## Implementation Plan

### Block 1: GearMenu

- `index.html`: ⚙ a `details.menu` (`dropdown()`, AP UiPolish's
  placement) with Cube settings (`needs-cubes-or-pair`), System settings
  (`needs-system`), System info (every role); ☰ on a phone the same
  three under Log; ⚙ hidden where none of them is open to the role.
- `log.js` `showView`, `VIEW_HASH`, `mayShow`: the views `cube` and
  `info`, their addresses (`#cube`, `#info`), the ⚙'s active state for
  the three.
- Tests at 1100 and 390 px: the entries by role; each opens its page;
  the address and back.

**Verification:** `make ui-test` (`test_header.py`, the new tests).

### Block 2: SystemInfoPage

- The System Info card moves to a page of its own (`#info`), its rows as
  now; System settings without it, its sections menu too; Software
  update alone in its row.
- `loadSystemInfo` on the page's view; Store and Settings in effect
  `needs-settings` as now.
- Tests: a viewer opens System info and sees the version, the time and
  the certificate, not Store; System settings has no System Info.

**Verification:** `make ui-test` (`test_system_info*.py`,
`test_system_layout.py`, `test_restart.py`).

### Block 3: CubePage

- `index.html`, `cubes.js`: the page `#cube` with the chosen Cube's
  `cubePanel`, open, its head the page's title row, no fold; drawn when
  shown and removed from System settings' list meanwhile, and the other
  way round; following the Cube choice and every poll, as the card's.
- Tests: the page shows the chosen Cube's facts, settings and actions; a
  change saved there shows in System settings; another Cube chosen, the
  page follows; a role with `pair` alone sees the pairing.

**Verification:** `make ui-test` (`test_cubes.py`, `test_pairing.py`).

### Block 4: ToTop

- `style.css`: the floating button filled in the accent colour.
- `components.js` `backToTop`: every page (Rooms, Log, 👤, System
  settings, System info, Cube settings), shown a screen down; a second
  in the header left of the last page button (⚙ wide, ☰ on a phone),
  shown while the header is compacted.
- i18n, six languages; `docs/ui.md`; fragments.
- Tests at 1100 and 390 px: both appear and take the page to its top.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with AP UiPolish, a commit of its own on main.

Block 1, `feat(ui): ⚙ is a menu of the Cube's, the system's settings and System info`;
Block 2, `feat(ui): System info is a page of its own for every user`;
Block 3, `feat(ui): The chosen Cube's settings on a page of their own`;
Block 4, `feat(ui): Back to top on every page and in the compacted header`.

## Change History

- v1.0 (2026-11-15): the first revision, from the owner's look at the
  UI in three EVAL rounds (D108-A, D109-A, D111-A, D114-A, D116-A,
  D117-A, D120-A, and ⚙ as a menu) and `PLAN++`.
