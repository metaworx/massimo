# AP HeaderPages v1.0: the header line and the personal page

## TL;DR

The header becomes one line on a common baseline, its logo as tall as the
page buttons. The 👤 menu becomes a page of its own, built like System
settings.

- brand tool → a tight logo and mark (no margin): the M from top to
  baseline; `-tight.svg` files; the web UI's sprite takes them; v1.3.0
- header → logo, version and "updated" on the M's baseline; the M 36 px
- ↻ in a frame → "updated 17:28:19" and an icon, one frameless button
- the version's tooltip → the full version "(server version)"
- the Cube's clock: header → the Cube's facts, moved on from when it was
  read (`cube_time_at`), its offset noted
- System settings: the row of section buttons goes; ☰ stays
- "This browser's address" → "Browser address"
- 👤 menu → 👤 opens the personal page: My profile, My settings,
  Notifications as section cards; ☰ and Sign out in its bar

### Blocks

1. BrandTight: the tool's tight shapes, `-tight.svg`, the sprite and
   `brand.css`; the README; v1.3.0; `make ui-brand`
2. Header: the baseline row, the 36 px M, the refresh button, the
   version's tooltip; the Cube's clock out of the header, `cube_time_at`
   in the API and the Cube's facts; tests; `docs/ui.md`, `docs/api-v1.md`
3. SystemPage: the section buttons' row out; "Browser address"; tests
4. PersonalPage: one page of three cards, 👤 its button, Sign out and ☰
   in its bar, the icons; the old addresses; tests; `docs/ui.md`

The four blocks are one pull request.

## Discussion

The owner's `EVAL` of the header on 2026-11-13, with screenshots at
390 px, mid width and wide, and the answers over four rounds:

- the baseline of the logo, the version and "updated" one line (D36-A:
  the version right after the logo);
- the second time in the header is the Cube's clock, read from its
  greeting at the last connect and never moved on: out of the header,
  into the Cube's facts, moved on and labelled (D37-A);
- the refresh icon off centre in its frame: the frame goes, the time and
  the icon one button that a finger can hit;
- the M as tall as the buttons to the right;
- the version's tooltip the full version, then " (server version)";
- System settings keeps ☰, not the row of section buttons;
- the personal pages as System settings is built (D38-A: the same grid,
  full width up to 900 px, two columns above);
- the user dropdown goes: 👤 opens the page, a prominent Sign out at its
  top right, left of its ☰ (D39-A), the dropdown's icons in ☰ and in
  the cards' heads;
- "This browser's address", System Info's longest label, shorter
  (D40-A, "Browser address");
- a logo and a mark without a margin, for precise placing, in the tool,
  for other uses too (D43-A: `-tight.svg` beside each, the sprite on
  them);
- two plans, this one first (D45-A); the room tile is AP TileBadges.

`PLAN++` with the recommended picks on 2026-11-13.

## Analysis

**The brand tool** (`tools/brand/make_brand.py`, v1.2.0).
The logo's viewBox pads 6 % of the M's height on every side
(`logo_parts`: `pad = 0.06 * mh`); the mark sits in its 512 box with the
M at `M_BOX` (26, 30, 460, 422), moved down by `SHIFT` (14). The letters
stand on the M's baseline (`baseline = my + mh`), so the M's box, the
logo's height and the letters' baseline are one: a tight viewBox is
`mx my total mh` for the logo, `26 44 460 422` for the mark. The `ui`
scope's sprite (`sprite_svg`) and `brand.css` (`--brand-logo-ratio`,
the `.brand-logo` aspect ratio) take the padded viewBox today. The
`svg` scope writes `svg/<base>-{icon,logo,small}-<theme>.svg`; the
`homeassistant` scope trims its PNGs from the padded drawings, and stays
as it is.

**The header** (`index.html` `.top-left`, `style.css`).
`.top-left` is a wrapping flex row, `align-items: center`; `.brand` the
same, holding `h1.brand-name` (an inline grid: the word's two spans and,
from `brandMarks`, the logo's and the mark's SVGs in one cell) and
`#version-short`. `--brand-h` is 1.6rem; the page buttons are 36 px
high. With the tight logo the SVG's bottom is the letters' baseline.
A flex item without a baseline of its own takes its bottom edge as one:
with the word's spans out of the flow (placed over the drawing's cell,
absolutely), the h1's baseline is the drawing's bottom, and
`align-items: baseline` puts the version and "updated" on it.
`#refresh-btn` is an `.icon-btn` of 30 px with the text glyph ↻, whose
font places it off centre; `#updated` is beside it, outside the button.

**The version** (`app.js` `checkVersion`) sets `#version-short` from
`splitVersion(v).short`; its title is `header.serverVersion` by
`data-i18n-attr`, which a change of language would put back.

**The Cube's clock** (`#clock-plain`, `renderHeader`, `clockText`).
`cube_time` is the H line's date and time (`summary` in `server.go`),
read at each connect; nothing says when, so the page cannot move it on.
The Cube's facts in System settings → Cubes (`fillCubeFacts`,
`cube-panel.html`) already have a "Cube clock" row from `clockText`.
`cubelan.Snapshot` stores the greeting without its time; it is set where
the H line is applied, and the summary can carry it as `cube_time_at`.
The Cube's clock has minutes only: an offset under 90 s says nothing.

**System settings** (`#system`): `nav.toc`, filled by `pageContents`,
under the page bar whose ☰ (`.sec-menu`, `pageBar`) holds the same
buttons. `pageContents` has no other page.

**The personal pages** are three sections (`#profile`, `#prefs`,
`#notify`), each a view of its own (`log.js`: `VIEW_SECTION`,
`VIEW_HASH`, `showView`, `onView`), opened from `#user-menu`
(`#menu-profile`, `#menu-prefs`, `#menu-notify`, `#me-line`,
`#logout-btn`; `login.js` `renderMe`). My profile is for a signed-in user
only (`hasProfile`). Scripts load each on its view: `users.js`
`onView("profile")`, `notify.js` `onView("notify")`, `app.js`
`isShown("notify")`. The OIDC return writes `#profile` into the address
(`login.js`), and `test_oidc.py` expects it there after a link.
The tests that open these pages go through `helpers.show_view`, and
`test_header.py`, `test_login.py`, `test_settings.py`,
`test_preferences.py`, `test_connect.py`, `test_system_layout.py`,
`test_tls.py` and `screenshots.py` name the parts that change.

## Implementation Plan

### Block 1: BrandTight

- `make_brand.py`:
  - `logo_parts(i_style, tight=False)`: the tight viewBox, no margin;
  - `icon_svg` and `logo_svg` take `tight`: the M's box as viewBox;
  - the `svg` scope writes `svg/<base>-<form>-<theme>-tight.svg` beside
    each of its three files;
  - `sprite_svg` and `brand_css` take the tight shapes;
  - `VERSION = "1.3.0"`.
- `tools/brand/README.md`: the tight files, the sprite's geometry.
- `make ui-brand`: `brand.svg` and `brand.css` change, the other files
  stay byte for byte.
- `style.css`: `--brand-logo-ratio`'s fallback, the word's place under the
  tight drawing (the sign-in dialog's `.brand-big` and the connect page
  too).

**Verification:** each scope but `svg` and `ui` writes what v1.2.0 wrote,
byte for byte (`grayorange`, both themes); `svg` adds the six tight
files; a second `make ui-brand` writes the same bytes; the logo and the
mark looked at in Chromium, light and dark.

### Block 2: Header

- `index.html`, `style.css`: `.top-left` and `.brand` on a common
  baseline; the word's spans placed over the drawing's cell, out of the
  flow; `--brand-h: 36px`; under 600 px the mark, 36 px.
- `#refresh-btn`: one frameless button holding `#updated` and an SVG
  icon (`icon-refresh`, new in the sprite), its hit area 44 px by a
  pseudo-element, its focus ring kept; aria-label "Refresh now, updated
  …".
- `checkVersion`: the title `header.versionTitle`, "{version} (server
  version)", set again at a change of language; the `data-i18n-attr`
  goes.
- `#clock-plain` out of the header, `renderHeader` and the signed-out
  rules.
- `cubelan.Snapshot`: the greeting's time (`HelloAt`); `cubeView`:
  `cube_time_at`; a test; `docs/api-v1.md`, the OpenAPI file.
- `clockText`: the Cube's clock moved on from `cube_time_at`; in the
  Cube's facts with its offset where it is 90 s or more ("3 min ahead").
- i18n: the new keys in all six languages.
- Tests: `test_header.py` (the order, the baseline within 1 px, the M's
  height, the button's hit area), `test_login.py`, `test_tls.py`; the
  server test for `cube_time_at`.
- `docs/ui.md` "Header"; a fragment.

**Verification:** `go test ./...`; the UI tests named; the header looked
at, 1100 and 390 px, light and dark.

### Block 3: SystemPage

- `#system`: `nav.toc` out, `pageContents` and its calls and the `.toc`
  rules out; ☰ unchanged.
- `sysinfo.client`: "Browser address", in all six languages.
- `test_system_layout.py` through ☰.
- `docs/ui.md` "System settings"; a fragment.

**Verification:** `test_system_layout.py`, `test_system_info*`, 1100 and
390 px.

### Block 4: PersonalPage

- `index.html`: `#user-menu` goes; `#view-me`, a page button with
  `icon-user`, its tooltip and label naming who is signed in.
- One section `#me` (`settings-page`): its page bar with the title
  ("Personal settings"), `#me-line` under it, `#logout-btn` (icon and
  text) and ☰; a section grid of three section cards, `#profile`,
  `#prefs`, `#notify`, each head with its icon (`icon-id`,
  `icon-sliders`, `icon-bell`); My profile hidden without a user.
- `sectionButtons`: a card's `data-icon` before its name, so ☰ shows the
  same icons.
- `log.js`: the view `me`; `#me` its address; `#profile`, `#prefs` and
  `#notify` open it at their card and stay in the address;
  `showView("me", …, card)`; the scripts' `onView` and `isShown` on `me`.
- `login.js` `renderMe`: the same texts, in the page bar; the button's
  tooltip and label.
- i18n: the page's title and the button's texts, six languages.
- Tests: `helpers.show_view` and `sign_out`; `test_header.py`,
  `test_login.py`, `test_settings.py`, `test_preferences.py`,
  `test_connect.py`, `test_oidc.py` where they name what moved; a test
  of the page (the cards, ☰ with icons, the old addresses, Sign out at
  the top right, no profile on an open server); `screenshots.py`.
- `docs/ui.md` "My profile", "My settings", "Notifications" as one
  section; a fragment.

**Verification:** `make ui-test` whole, 1100 and 390 px;
`go test ./...`, `make cross`, `gofmt -l .`, the three `go vet`.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work
branch:

```
chore(aps): +HeaderPages v1.0

The owner's evaluation of the web UI's header on 2026-11-13: the logo, the version and the time of the last update on one baseline, the M as tall as the page buttons, a frameless refresh, the Cube's clock out of the header, System settings without its row of section buttons, and the 👤 menu a page of its own like System settings. PLAN++ with the recommended picks, D36-A to D40-A, D43-A, D45-A.

Key changes:
- GUIDELINES/wip/2026-11-13_20-35_AP_HeaderPages_header_line_and_personal_page.md: the plan, four blocks

File-revision-v1: GUIDELINES/wip/2026-11-13_20-35_AP_HeaderPages_header_line_and_personal_page.md::v1.0::2026-11-13::All::the first revision, by PLAN++ as recommended
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(brand): The logo and the mark also without a margin`;
Block 2, `feat(ui): The header on one baseline, the Cube's clock in its facts`;
Block 3, `feat(ui): System settings without its row of section buttons`;
Block 4, `feat(ui): The personal settings one page, opened by 👤`.

## Change History

- v1.0 (2026-11-13): the first revision, from the owner's evaluation
  and `PLAN++` with the recommended picks.
