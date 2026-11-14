# AP UiPolish v1.0: menus, header and copies

## TL;DR

The web UI's fixes from the owner's look at the merged CubeBadges and
CopyFacts: menus that stay in the window, the certificate's state that
follows, the version never hidden, the pill always in its place, and
facts to copy where they were missing.

- every menu → kept in the window, its height capped with a scroll of
  its own, the page not scrolled with it; a badge wraps its age
- the certificate → "being read…", asked again, "could not be read";
  its address copyable, a link
- the version, the sign and the time → a row of their own; the time
  kept in the compacted header
- a Cube's pill → top-right always, its badges a line under it; a green
  ✓ where nothing is wrong, the header's too
- the callback URL, both serials of a Cube proxy → copyable; help for
  Sign-in providers and Integrations; Maintenance in the owner's order

### Blocks

1. MenuFit: every menu kept in the window and capped in height; tests
2. CertPending: the certificate's state followed, its address a copyable
   link; tests
3. HeaderFacts: the facts' row; the time in the compacted header; tests
4. PillPlace: the pill top-right, the OK sign; tests
5. SettingsCopy: the callback URL, the proxies' serials, the help texts,
   Maintenance's order; docs

The five blocks are one pull request.

## Discussion

The owner's look at the UI on 2026-11-14 night, weighed in three EVAL
rounds; `PLAN++` on 2026-11-15:

- the pill's list: scrollable within the window's height, without
  scrolling the page; its badge overflows; never right-aligned out of
  the window (D113-A: every menu of the page);
- the certificate "being read" reads as final and does not follow the
  poll (D112-A: "being read…", asked again every 2 s up to 30 s, then
  "could not be read"); its address copyable and a link to a new window;
- the version hidden between the logo and the Cube choice, and cut again
  on a narrow phone (D115-A: a row of its own); the time gone from the
  compacted header (D119-A: kept on every width, the version goes);
- a Cube's pill top-right always, as Pianterreno's head, its badges a
  line under it; a green OK sign where nothing is wrong, the header's
  pill too (D118-A: a ✓ in the pill's frame, "Nothing wrong");
- Sign-in providers' callback URL copyable, a help text in its head and
  in Integrations'; Cube proxies with the serial they answer discovery
  with, both copyable; Maintenance: Announce on the LAN (bold as the
  others), Settings file, Restart, Reboot;
- D114-A, D120-A: two plans, this one first; AP PageMenu, the pages,
  after it. The plans stay open until the UI is fixed (the owner).

The backups the owner misses after a start are investigated by a session
of their own; a fix follows its findings.

## Analysis

**Menus.** `dropdown()` (`components.js`) makes a `details.menu` a menu;
its list is `.menu-items`, absolutely placed, `right: 0` under its
summary (`style.css`), the header's problem list too
(`.cube-problems .problems-list`). Nothing keeps it in the window or
caps its height. A badge (`.flag`, `inline-flex`) holds its text and its
age (`.age`, `nowrap`), so a long one is wider than the list's 360 px.

**The certificate** (`update.js` `fillCertificate`): `sysinfo.proxyPending`
while the server has not read the proxy's certificate yet
(`proxied.go`, a handshake of at most 5 s after the first request);
`loadSystemInfo` runs only when System settings opens.

**The header** (`style.css` `.top-line`): one grid row, "left cube nav";
`.top-facts` (the version, the server's sign, the time) sits in "left"
under the logo, which the Cube choice and the page buttons narrow to the
logo's width; below 600 px "left nav" / "cube cube", the facts beside
the page buttons. Compacting (`body.scrolled`) hides `#refresh-btn`.

**A Cube's head** (`cubes.js` `cubePanel`, `fillCubeHead`): the summary
wraps the sign, the name, the serial, the address and the budget, then
`.cube-pill` and `.panel-badges` (the source), whichever does not fit
going to the next line. `problemsEl` gives null where nothing is wrong,
and the pill is then empty; the header's (`renderCubeProblems`) too.

**Copies** (AP CopyFacts' `copyEl`, `copyIcon`): `#providers-callback`
(`partials/providers.html`) is plain text. `cube_proxies`
(`runtimes.go` `proxyStatus`) has the Cube's serial and the address, not
the serial the proxy answers discovery with (`ProxyListener` /
`proxy.Options.DiscoverySerial`, the Cube's own where none is set).
**Help** slots are `data-help` spans in a card's head (`app.js`
`initHelpSlots`, `help.<key>` in i18n). **Maintenance**
(`partials/maintenance.html`): Restart, Announce, Reboot, Settings file.

## Implementation Plan

### Block 1: MenuFit

- `components.js` `dropdown()`: on open, `placeMenu(list)`: right-aligned
  to its summary where that fits, else left-aligned, else the window's
  width less 16 px a side; `max-height` the room below it less 16 px,
  `overflow-y: auto`, `overscroll-behavior: contain`; again on resize
  while open.
- `style.css`: a problem list up to the window's width less 32 px; a
  badge wraps, its age under its text where they do not fit side by side.
- Tests at 1100 and 390 px: the header pill's list inside the window, a
  long list scrolls in itself and the page does not move, its badges
  inside it; the Cube choice and ☰ inside the window.

**Verification:** `make ui-test` (`test_problem_list.py`, `test_header.py`,
the new tests).

### Block 2: CertPending

- `update.js`: while the certificate is pending and System settings is
  shown, `loadSystemInfo` again every 2 s for up to 30 s; after the last,
  "could not be read" (`sysinfo.proxyUnread`); "being read…"
  (`sysinfo.proxyPending`), six languages.
- The address under the certificate: `copyEl` and a link to it in a new
  window (`linkEl`).
- Tests with a stubbed `GET /api/v1/system`: pending, then read; pending
  to the end, "could not be read"; the address copies and links.

**Verification:** `make ui-test` (`test_system_info*.py`, `test_tls.py`).

### Block 3: HeaderFacts

- `style.css` `.top-line`: the facts a row of their own, "facts" across
  the header under the logo and the Cube choice, on a phone under the
  logo and the page buttons; where the row is too narrow, the version a
  line of its own, the sign and the time the next (`.top-facts`
  wrapping); the version cut only where it is wider than the header.
- Compacted: the sign and the time kept, the version hidden, on every
  width.
- Tests at 1100, 800, 600, 430 and 390 px with a build's version: not
  cut; compacted, `#updated` shown; `test_scroll.py` green.

**Verification:** `make ui-test` (`test_header.py`, `test_scroll.py`).

### Block 4: PillPlace

- `cubes.js`, `style.css`: a Cube's head a grid, the pill in the first
  line's last column whatever wraps, the head's badges a line of their
  own under the name.
- `components.js` `okPill(label)`: a green ✓ in the pill's frame, its
  label and tooltip "Nothing wrong", no list; `fillCubeHead` and
  `renderCubeProblems` use it where `problemsEl` gives null; the found
  Cubes' sign beside it in the header.
- Tests: the pill top-right at 1100 and 390 px with a long head; the ✓
  where nothing is wrong, in a head and in the header.

**Verification:** `make ui-test` (`test_cubes.py`, `test_cube_problems.py`).

### Block 5: SettingsCopy

- `partials/providers.html`, `providers.js`: the callback URL `copyEl`
  with its icon; a help slot in the head (`help.providers`).
- `partials/integrations.html`: a help slot (`help.integrations`).
- `runtimes.go` `proxyStatus`: `discovery_serial`, the serial the proxy
  answers discovery with; `docs/api-v1.md`, the OpenAPI file; a test.
- `update.js` `fillProxies`: the Cube's serial and the discovery serial,
  both copyable, with the address.
- `partials/maintenance.html`: Announce on the LAN (its title as the
  others'), Settings file, Restart, Reboot.
- i18n, six languages; UI tests: the copies, the order, the help; `docs/ui.md`;
  fragments.

**Verification:** `make ui-test` whole, `go test ./...`, `make race`,
`make cross`, `gofmt -l .`, the three `go vet`,
`GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with AP PageMenu, a commit of its own on main.

Block 1, `fix(ui): Every menu stays in the window and scrolls in itself`;
Block 2, `fix(ui): The proxy's certificate is asked for again until read`;
Block 3, `fix(ui): The version, the sign and the time in a row of their own`;
Block 4, `feat(ui): A Cube's pill top-right, a green sign where nothing is wrong`;
Block 5, `feat(ui): The callback and the proxies' serials to copy, Maintenance reordered`.

## Change History

- v1.0 (2026-11-15): the first revision, from the owner's look at the
  merged CubeBadges and CopyFacts in three EVAL rounds (D112-A, D113-A,
  D115-A, D118-A, D119-A, D120-A) and `PLAN++`.
