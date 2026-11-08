# AP SettingsLayout v1.0: Settings pages, new layout

## TL;DR

Roadmap item 15 as the owner decided it on the mock
(`docs/mock/settings-layout/`, D1 to D13, 2026-11-08):
System settings full width in foldable cards, the header and the page's
row kept at the top, the Cube choice a menu with plugs, My settings with a
column of "only on this device", and four server changes.
Every piece that recurs is a component in `components.js` (owner, 2026-11-08).

- panel heads: `ownerowner` → name, key, badges at the right, `you` first
- System settings: 520 px column → cards in a grid, Users, Roles and
  Capabilities flowing in two columns; contents row, sections menu, ↑
- Cube choice: native select + "connected" → menu, plug, serial and address
- server: `POST /api/v1/reload`, store and config views,
  rank rule with a `passwords` capability, a keys-only user's first password
- components: `detailsPanel`, `dropdown` get callers; new `sectionCards`,
  `pageContents`, `stickyHeader`, `menuItem`, `pickerMenu`, `connIcon`,
  `deviceIds`, `actionRow`, `kvList`, `passwordFields`, `fieldRow`'s `deviceOnly`

### Blocks

1. Panel heads through `detailsPanel`: name and key apart, badges right, `you` first
2. System settings in section cards: order, grid and flow, contents row, folding, back to top
3. The header and the page's row at the top, the sections menu, the phone's compact header
4. Menu items with icons; the 👤 tooltip with the user's name
5. The Cube choice as a menu, its connection a plug
6. A device's serial beside its radio address, in the four places
7. My settings: the "only on this device" column, the reading width
8. Maintenance: the restart moved there, `POST /api/v1/reload`
9. The store and the settings in effect: two read-only routes and their view
10. The rank rule and the `passwords` capability
11. "Set a password" for a keys-only user
12. Documents and screenshots as built; the mock removed

## Discussion

The owner listed fifteen points from the v0.11.0-alpha.1 screenshots
(2026-11-08), kept as roadmap item 15 by the coordinating session,
and asked for a static mock with the real CSS before any code (15.9).
The mock is on this branch ("docs(ui): Mock the settings pages in a new
layout"); its screenshots are retaken with its `shoot.py`.
The owner decided on it, at the TL;DR gate and two EVALs, then `EXEC+` and
`PLAN+`:

- **D1-A**, with the owner's change: section cards in a grid, Software
  update beside Maintenance, Cubes and Backup full width; Users, Roles and
  Capabilities flow freely in two columns, none split.
- **D2-A**, extended: a row of section buttons under the title, each
  section folding from its heading, open at the start, the fold kept per
  browser, ↑ once the row is out of sight; and the page's row kept at the
  top with a sections menu at its right end, marking the section in view.
- **D3-A**: "Reload now" in Maintenance, a route doing what SIGHUP does,
  for `restart`, with the time the file was last read.
- **D4-A**: the store and the settings in effect, folded in Maintenance,
  for `settings`; secrets never leave the server.
- **D5-B**: a Cube's name where it is chosen or spoken of, its serial and
  address where one Cube is told from another; "Holiday Home" in the
  documents (the owner's; "Living room" is a room's name).
- **D6-A**: the Cube choice a menu of the page, as 👤 is.
- **D7-A**: a capability "set users' passwords", seeded for the manager,
  and a rank rule for it and for `users`.
- **D8-A**: "Set a password" in My profile for a keys-only user,
  without a current one.
- **D9-A**, with the owner's change: the switches in a column of their own,
  each row's line running through it.
- **D10-A**: My settings and My profile 640 px wide.
- **D11-A**: a device's identifiers labelled, "S/N MEQ0000103 · RF 0a0103";
  a Cube's not ("KEQ0000100 · 192.0.2.10").
- **D12-A**: the sections menu a bulleted-list icon, apart from the
  header's ☰.
- **D13-B**: on a phone, scrolled, the Cube choice takes the brand's place.
- The plug: joined and green for connected, apart and red for not,
  in the Cube choice and its list, in place of the "connected" label
  (owner, D12's EVAL).
- **Reusable components** (owner, during `PLAN+`): every piece that recurs
  is a function of `components.js` or a template of a partial,
  built generic with its first caller; each block names its callers.

Nobody else works on these files: the session first named for this area
runs AP LogAddress, now retired; `docs/roadmap.md` is the coordinating
session's, which marks item 15 when the plan is done.

The plan's pull requests: Blocks 1 to 7, the page and its components;
Blocks 8 to 11, the server's changes; Block 12 with the last of them.
The owner may merge in other groups.

## Analysis

- **Panel heads.** The user and role templates (`partials/users-panel.html`,
  `roles-panel.html`) put name, key and badges into a `summary` that is not
  flex, so the three run together ("ownerowner"); the Cube panel's is flex.
  `userBadges` (`users.js:182`) puts `you` last.
  `detailsPanel` (`components.js:385`) builds exactly that summary and has no
  caller (roadmap 12).
- **System settings** is one `.settings` tile, at most 520 px
  (`style.css:300`), its parts the partials of `uiPartials`'s `system`
  marker in the order cubes, backup, users, roles, software (`ui.go:40`).
  The parts are headed by `.cubes-head`, a muted label.
  The page is in the address (`#system`), so the contents use buttons,
  not `#…` links, which would change the page.
- **Keeping the header at the top** works with `position: sticky`:
  `body`'s `overflow-x: hidden` passes to the viewport and does not make
  `body` a scroll container (tried in the mock).
  A sticky row holds only within its parent, so the page's row is a child
  of the page's `section`, not of a header inside it.
  The lines under the header's first line (found Cubes, certificate,
  expert details) scroll away; the reload button for a new version stays.
- **The free flow** is CSS columns over the three cards with
  `break-inside: avoid`. The break falls after Users when Users is taller
  than Capabilities, after Roles otherwise; opening a role never moves it,
  opening a user's panel may move Roles (tried in the mock).
- **Menus.** `dropdown` (`components.js:434`) makes a `details.menu` one
  open at a time, closed by a choice, Escape or a click outside; 👤, ☰ and
  the rooms' ⋯ use it. The Cube choice is a native `select`
  (`renderCubeSelect`, `app.js:552`), whose closed and open texts are the
  same, so it cannot show the name alone closed and serial and address open.
  The connection is a pill (`#connection`, `app.js:510`).
- **A device's serial**: the API sends it (`internal/view/view.go:140`,
  `omitempty`); the UI shows the radio address alone in four places:
  the pairing row (`cubes.js:431`), the hidden devices (`cubes.js:153`),
  the room's device list (`tiles.js`, `.device-rf`) and the room
  configuration (`config.js:397`) (15.15).
- **My settings**: each "only on this device" is a `label.switch.device-only`
  under its setting (`index.html`, `partials/appearance.html`), shown with
  `needs-sync`; `settings.thisDeviceFor` ("{setting}: only on this device")
  is already in the six files.
- **Reload**: `serve` reads its settings file every five seconds and on
  SIGHUP through `settingsReloader.reload(ctx, why)` (`cmd/massimo/reload.go:129`);
  `restart_api.go` takes the pending names from `serve` through
  `SetPendingRestart`. A reload route takes the reloader the same way.
- **Store and config**: `store.Info` (`internal/store/backup.go:36`) has
  what `store info` prints; `massimo config` gathers its facts in
  `cmd/massimo/config.go`, a `main` package the server cannot import, and
  redacts secrets (`redact`). `serve` hands the server a function giving
  those facts, secrets as "set" or "not set".
- **The rank rule**: `mayManage` and `mayAssign` (`users_api.go:154`)
  let a holder of `users` change anyone and give any role; `mayRegister`
  (`opaque_api.go:343`) follows them. The rule: one changes only a user
  whose role has no capability one lacks, and gives only such roles; the
  owner has all, so only an owner changes an owner. `passwords` sets the
  password of such a user and nothing else. A migration (0008) adds it and
  seeds it for the manager.
- **A keys-only user's password**: `putUser` asks a user setting its own
  password for the current one or a proof (`users_api.go:345`), which a
  user without one cannot give; `login.js:151` hides the form then.
  A user without a password may set its first one as itself.
  A key could then set a password that outlives the key; it can already
  create keys that do, so nothing new is exposed.

## Implementation Plan

On `claude/agitated-hawking-7b4d6f`, after the mock's commit.
Every block: its texts in all six `ui/i18n/*.json` (FR, IT, NL, PL
machine-translated and marked), no `innerHTML`, its CHANGELOG bullet, and a
new component documented where it is defined, in the style of
`components.js`, and in `docs/ui.md`, "How the page is built".
The mock's `proposed.css` is the source of the rules; they move into
`style.css` under generic class names (`.section-card`, not `#sec-users`),
an id only for a grid area.

**Verification**, after every block:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`make cross`, `gofmt -l .` empty, `go vet` on linux, windows and darwin,
`make ui-test` with the block's new test at 1100 and 390 px
(`UI_TEST_PYTHON` the main checkout's venv, free `UI_TEST_*_PORT`).

### Block 1: Panel heads through `detailsPanel`

- `users.js`, `roles.js`, `cubes.js`: the panels built with `detailsPanel`,
  the form cloned from its template as the body; the templates lose their
  `details` and `summary`.
- `userBadges`: `you` first, then the role, the state, "keys only".
- `style.css`: `.panel > summary` flex, a chevron, the key muted and apart,
  `.panel-badges` at the right edge (from `proposed.css`).
- Components: `detailsPanel` gets its three callers.
- `tools/ui-test`: a user's and a role's summary have a gap between name and
  key, their badges end at the panel's right edge, `you` is the first badge.

### Block 2: System settings in section cards

- Partials: each part a `section.tile.section-card` with
  `h3 > button.section-fold` (`aria-expanded`, `aria-controls`), its
  badges and its help; `uiPartials` in the order software, maintenance
  (Block 8 fills it), cubes, users, roles, capabilities, backup;
  Capabilities a card of its own (`partials/capabilities.html`).
- `style.css`: the page full width (`.settings.settings-page`), the grid,
  the free flow of Users, Roles and Capabilities, the contents row, ↑.
- Components:
  - `sectionCards(root)`: folding, the fold kept per browser
    (`localStorage`, every access in try/catch), a folded card keeps its
    badges; any page's cards, System settings' the first.
  - `pageContents(page, sections)`: the row of section buttons, each
    scrolling to its card and opening it; the sections menu of Block 3
    takes the same list.
  - `backToTop(page)`: shown once the row is out of sight.
  - `sectionBadges(card, badges)`: what a card holds at a glance
    (up to date, 2 served, 5 users, 3 kept).
- `tools/ui-test`: the order, a card folds and stays folded after a reload,
  a contents button opens a folded card, ↑ appears and returns.

### Block 3: The header and the page's row at the top

- `index.html`, `style.css`: the header sticky on every page; scrolled, it
  drops the time of the last update and the clock; under 600 px, scrolled,
  the Cube choice takes the brand's line (D13-B); `#version-short` does not
  wrap.
- System settings' row (`.page-bar`): the title and, at its right end, the
  sections menu (`icon-list`, tooltip "Sections") marking the section in
  view; the cards' `scroll-margin-top` the two rows' height.
- Components: `stickyHeader()` (the `scrolled` class, the header's height
  as `--top-h`), for every page; `pageBar(page, title, sections)` with the
  sections menu through `dropdown` and `menuItem` (Block 4), for any page
  with sections.
- `tools/ui-test`: scrolled, the header and the row stay at the top at both
  widths, the menu marks the section in view and scrolls to the one chosen.

### Block 4: Menu items with icons

- `index.html`: the 👤 menu's items with icons (`icon-id`, `icon-sliders`,
  `icon-signout`; Forget key `icon-key`), the ☰ menu's with the page icons;
  the 👤 button's `title` and `aria-label` with the user's display name
  and login (`login.js`, where `#me-line` is filled).
- Components: `menuItem(symbol, label, onClick)`, used by 👤, ☰, the
  sections menu and the Cube choice's search entry.
- `tools/ui-test`: each item has its icon; the 👤 title names the user.

### Block 5: The Cube choice as a menu, its connection a plug

- `index.html`, `app.js`: `#cube-select` and `#connection` give way to a
  `details.menu` whose summary shows the plug and the chosen Cube's name
  (its serial without one), its title the serial, address and connection;
  the list: per Cube the plug, the name, under it serial and address;
  "Found, not configured" without a plug, off; "Search for new Cubes…".
- The Rooms notice of a disconnected Cube names it by its name and gives the
  serial in its detail (D5-B).
- Components: `pickerMenu({ items, value, onPick })`, a `dropdown` whose
  summary shows the chosen item, items of two lines, `aria-current` on the
  chosen, the arrow keys moving between items; `connIcon(connected)`, the
  plug with its `aria-label` (`icon-plug-on`, `icon-plug-off`), in the
  summary and in each item.
- `tools/ui-test`: choosing a Cube through the menu, the search entry, a
  disconnected Cube's plug, the list wider than the control at 1100 px.

### Block 6: A device's serial beside its radio address

- `cubes.js` (pairing row, hidden devices), `tiles.js` (`.device-rf`),
  `config.js:397`: "S/N MEQ0000103 · RF 0a0103", the RF part in expert mode
  only, the serial when the API has one.
- Components: `deviceIds(serial, rf)`, its four callers.
- `tools/ui-test`: expert mode shows both in the device list; without it
  the serial alone.

### Block 7: My settings, the device column and the width

- `index.html` (`#prefs`), `partials/appearance.html`: each setting's switch
  in the column, headed "only on this device", its title
  `settings.thisDeviceFor`; each row's line through the column; the column
  gone where settings do not follow the user (`no-sync`).
- `style.css`: My settings and My profile at most 640 px.
- Components: `fieldRow(…, { deviceOnly: key })` puts the switch into the
  column; `deviceTable(rows)` with its head.
- `tools/ui-test`: a switch in the column keeps its setting on this device,
  its tooltip names the setting; no column for a guest.

### Block 8: Maintenance and `POST /api/v1/reload`

- `cmd/massimo/serve.go`: the reloader handed to the server
  (`srv.SetReloader`), as the pending names are.
- `internal/server`: `POST /api/v1/reload` (capability `restart`) reads the
  settings file at once, as SIGHUP does, checkpoints the store, writes an
  audit row and answers what changed and what waits for a restart;
  `GET /api/v1/restart` adds the file and when it was last read.
  Tests: the route with and without `restart`, an audit row, a changed
  setting listed; `TestOpenAPICoversRoutes`.
- `partials/maintenance.html`, `update.js`: the restart moved there from
  Software, "Reload now", the last read, the pending names.
- Components: `actionRow(title, text, button)`, also Software update's check.
- `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, README.md "Server".

### Block 9: The store and the settings in effect

- `cmd/massimo/config.go`: the facts `massimo config` prints gathered in a
  function that both the command and `serve` use, secrets as "set";
  `serve` hands it to the server.
- `internal/server`: `GET /api/v1/store` (`store.Info`: file, schema,
  node, tenant, rows) and `GET /api/v1/config`, capability `settings`.
  Tests: no secret in the answer (a key and the token set), 403 without
  `settings`; `TestOpenAPICoversRoutes`.
- Maintenance: "Store" and "Settings in effect", folded.
- Components: `kvList(pairs)` for both views, `countList(counts)` for the
  store's rows.
- `docs/api-v1.md`, the OpenAPI file.

### Block 10: The rank rule and `passwords`

- `internal/store/migrations/0008_passwords_capability.sql`: the capability
  `passwords`, seeded for the manager; `store/roles.go` its name.
- `users_api.go`: `mayManage` and `mayAssign` hold the rank rule;
  `passwords` lets one set the password of a user one outranks, nothing
  else; `opaque_api.go` `mayRegister` the same.
- `users.js`: a panel shows what the viewer may change, "Only an owner
  changes an owner." on an owner's panel for the others.
- Tests: a manager sets an operator's password, not an owner's, gives no
  owner role; a created role with `users` but without `restore` cannot change
  a manager; the migration on a store of version 7.
- `docs/store.md` section 6 (the table), `docs/api-v1.md` "Authentication",
  `cap.passwords` in the six files.

### Block 11: "Set a password" for a keys-only user

- `users_api.go` `putUser`, `opaque_api.go`: a user without a password sets
  its first one as itself, without a current one; audited.
- `login.js`, `partials/profile.html`: "Set a password" where `has_password`
  is false, "Change password" otherwise.
- Components: `passwordFields({ current })`, the new password and its
  repetition, with the current one where asked: My profile's two forms and
  the first owner's form.
- Tests: the first password set by the user itself, a second refused without
  the current one; `tools/ui-test` the profile of a keys-only user.

### Block 12: Documents and screenshots

- `docs/ui.md`: the header, System settings, My settings, My profile and
  the components as built.
- `tools/ui-test/screenshots.py`: the shots of the new pages;
  `docs/screenshots/` retaken.
- `docs/mock/settings-layout/` removed: what it proposed is built.

**Verification:** as every block; the screenshots looked at.

## Proposed commit message

- `chore(aps): +SettingsLayout v1.0`
- `fix(ui): A panel's head shows its name and key apart, its badges at the right edge`
- `feat(ui): System settings in foldable section cards, with contents and back to top`
- `feat(ui): The header and the page's row stay at the top while scrolling`
- `feat(ui): Icons in the menus, the user's name in the 👤 tooltip`
- `feat(ui): The Cube choice is a menu with each Cube's serial, address and connection`
- `feat(ui): Expert mode shows a device's serial beside its radio address`
- `feat(ui): My settings keeps "only on this device" in a column of its own`
- `feat(api): POST /api/v1/reload, and Maintenance in System settings`
- `feat(api): The store and the settings in effect, in System settings`
- `fix(security:users): A user changes only users who rank no higher`
- `feat(users): A user with keys only sets its first password in My profile`
- `docs(ui): System settings and My settings as built, the mock removed`

## Change History

- v1.0 (2026-11-08): the first revision, from the mock's TL;DR gate and the
  owner's two EVALs (D1 to D13), `EXEC+` committing the mock, then `PLAN+`
  with the owner's reusable components.
