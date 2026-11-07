# AP Navigation v1.0: Header, user menu, pages in the address

## TL;DR

The header says who is signed in and folds on a narrow screen,
Settings splits into what is the user's and what is everyone's,
the open page is in the address, and Home says why it is empty.
No server change.

- log icon → its lines in a page frame (a sheet), no longer a hamburger
- 👤 left of 🏠 → dropdown: name and role, My profile, My settings, Sign out;
  "Log out" / "Forget key" leave Settings
- My profile → display name, password, own sessions and API keys
- My settings → language, appearance, poll, expert, clock, "only on this device";
  ⚙ System settings → Cubes, backups, users, roles, software,
  shown only with one of those capabilities
- address → `#log`, `#profile`, `#prefs`, `#system`; back and forward;
  a reload reopens the page; one the user may not see falls back to Rooms
- below 600 px → `👤 🏠 ☰`, ☰ holding what does not fit
- Home without a connected Cube → a notice: address, last data,
  what to check, a button to System settings → Cubes
- order at full width: D3, open

### Blocks

1. Log icon as a sheet
2. The 👤 menu; Settings loses who is signed in and Log out
3. My profile, My settings, System settings
4. The page in the address; back and forward
5. ☰ on a narrow screen; the tests switch pages through `show_view`
6. The Home notice without a connected Cube
7. Full screenshot set, branch pushed, pull request

## Discussion

From the owner's Pi 1 test of v0.10.0-alpha.1 on 2026-11-07, items 3 to 5,
the owner's EVAL of the same day, and finding 7 of the hardware tests on the
Windows service, passed on by a teammate session:

- **Item 3.** Nobody can see who is signed in;
  Settings → Users only marks the own row "You".
- **Item 4.** The log icon reads as a hamburger.
- **Item 5.** When the header runs out of space, refresh, log and settings
  collapse into a hamburger with a dropdown.
- **The owner's EVAL.**
  The user menu gets My profile (display name, password),
  and Change password leaves Settings;
  moving between pages updates the address,
  so that a reload reopens the page and the browser's history moves between them;
  and maybe a page of the user's settings beside the system's.
- **Finding 7.** With its only Cube disconnected,
  Home is empty under a header reading only "updated 12:06:38".

Settled, by the owner or as recommended at the TL;DR gates of 2026-11-07
(`PLAN++ SendButtons` took the recommendations it did not name):

- **D3, narrow: the owner's.** `👤 🏠 ☰`, the user left of Home.
- **D4-A.** The header folds at a CSS breakpoint below 600 px,
  predictable and testable at 390 and 1100 px; no measuring in JS.
- **D5, the owner's.** The 👤 menu holds who is signed in
  (with the key's label for an API key), My profile and Sign out,
  which only forgets an API key, as "Forget key" does now.
- **D6-A.** The Home notice shows whenever the selected Cube is not connected;
  above the last known rooms, where the server still has them.
- **D7-A.** The suite reaches an unreachable Cube through a second `serve`
  in `run.sh`, its Cube address a closed loopback port;
  page stubs cover no Cube configured and a Cube that dropped after its rooms loaded.
- **D12-A.** The address holds the page only, not a room's open tab:
  a reload drops unsent edits anyway.
- **D13-A.** 👤 → My profile and My settings; ⚙ → System settings.
  On an open server the menu says "open to everyone" and offers My settings.
- **D14-A.** My profile lists the own sessions and API keys,
  with Revoke and Create key:
  "sign out my other browsers" without an administrator.
  The API already allows it (`targetUser(r, st, true)`, the own-name access
  of `auth.go`).

Open, for this plan's gate:

- **D3. The header at full width.**
  - **A.** `👤 🏠 📄 ⚙ ↻` at the right, the left line as now:
    ☰ then folds the right end of the row.
  - **B (recommended), the owner's alternative.**
    The left line reads `Massimo <version> · updated <time> ↻`,
    refresh beside the time it renews;
    the connection pill moves in front of the Cube choice,
    and the Cube choice to the right, beside `👤 🏠 📄 ⚙`.
    ☰ then folds only Log and System settings, and the Cube choice keeps its place.
  - **C.** As B, with the Cube choice centred in the free space;
    it moves whenever the time or the name beside it changes width.

## Analysis

- **The header** (`index.html`): `.top-left` holds the brand and version,
  the Cube choice with the connection pill and the discovery status,
  the Cube's clock and `#updated`, and wraps;
  `.top-right` holds `#refresh-btn` and `nav.nav` with `#view-tiles`,
  `#view-log` (`icon-list`, three lines with dots) and `#view-settings`.
- **The dropdown** pattern is the room tile's `details.menu` with `.menu-items`;
  `tiles.js` closes an open one on a click outside and on Escape,
  one open at a time. The header's menus join that rule.
- **Settings** is one section: poll, expert, clock and language inline,
  then the partials `cubes`, `backup`, `users`, `roles`, `appearance`, `page`,
  `software` (`uiPartials` in `ui.go`);
  `page.html` holds the reload note, `#me-line`, the sync note,
  Change password and `#logout-btn`.
  `go test ./internal/server` checks the i18n keys and the ids the scripts look up.
- **The page switch** is `showView(view)` in `log.js`, no history.
  A hash needs no server route and works under a reverse proxy's path prefix.
- **The tests** click `#view-settings`, `#view-log` and `#refresh-btn`
  about 40 times in 13 files and `screenshots.py`;
  at 390 px two of them sit behind ☰, so the clicks move to a helper.
- **Home without data:** `loadRooms` returns early without a serial,
  and `getRooms` answers `[]` for a Cube that has not connected since the start.
  `GET /api/v1/cubes` has `address`, `connected`, `last_seen`
  (the last answer to discovery) and `updated` (the last data).
- **Screenshots:** every shot shows the header, so Block 7 retakes them all;
  the set is byte-for-byte reproducible.

## Implementation Plan

The gate of every block as AP SendButtons states it,
`make ui-test` at 1100 and 390 px in English and German.
The branch starts from AP SendButtons' and merges `origin/main` before its pull request.
Each block is one functional commit citing `AP Navigation v1.0, Block <n>`,
with its CHANGELOG bullet, `docs/ui.md` moving with it.

### Block 1: LogSheet

1. `index.html`: `icon-list` becomes a sheet:
   a page outline with a folded corner around three short lines.
2. A test that the symbol has its frame (a `path` or `rect` beside the lines).

**Verification:** the gate; the shot of the header looked at in both themes.

### Block 2: UserMenu

1. `index.html`: `details#user-menu.menu` left of `#view-tiles`,
   its summary an icon button (`icon-user`), its items:
   who is signed in (`#me-line` moves here), My profile, My settings, Sign out.
2. `components.js`: `headerMenu(details)` closes on Escape,
   on a click outside and when another menu opens;
   `tiles.js` uses it for the room menus.
3. `login.js`: `renderMe` fills the menu; Sign out calls `logout`;
   for an API key and the token mode it reads "Forget key".
   `page.html` loses `#me-line` and `#logout-btn`.
4. Texts in all six `ui/i18n/<lang>.json`.
5. Tests: the owner's key, the operator signed in and an open server
   show the right line and item; Sign out from the menu signs out.

**Verification:** the gate.

### Block 3: SettingsPages

1. `index.html`: three sections in place of `#settings`:
   `#profile`, `#prefs` and `#system`; `uiPartials` split by page.
2. My profile: the display name and password (Change password moves here),
   the own sessions and keys with Revoke and Create key,
   built from `users.js`' panel parts; Save busy through AP SendButtons' helper.
3. My settings: language, appearance, poll, expert, clock,
   the "only on this device" switches, the sync and reload notes.
4. System settings: Cubes, backups, users, roles, software;
   `#view-settings` hidden without one of their capabilities.
5. `docs/ui.md` "Settings" rewritten as three pages.
6. Tests: the appearance, language and preferences tests open My settings;
   the operator has no ⚙; My profile changes the own display name.

**Verification:** the gate; `go test ./internal/server` (ids and keys).

### Block 4: PageAddress

1. `log.js` `showView(view, {push})`: `location.hash` follows
   (`#log`, `#profile`, `#prefs`, `#system`, empty for Rooms),
   `hashchange` shows the page, `init` opens the page of the hash,
   a page the user may not see becomes Rooms with `replaceState`.
2. Tests: a reload on `#log` opens the log; back returns to Rooms;
   `#system` as the operator opens Rooms.

**Verification:** the gate.

### Block 5: NarrowHeader

1. `index.html`: `details#nav-menu.menu` (☰) with labelled items
   for what folds; `style.css` below 600 px hides the folded buttons
   and shows ☰, highlighted while one of its pages is open.
2. `tools/ui-test/helpers.py`: `show_view(page, view)` clicks the button
   or opens ☰ first; every direct header click in the tests
   and `screenshots.py` goes through it.
3. Tests: at 390 px ☰ is shown and the folded buttons hidden, at 1100 the reverse;
   each item opens its page; no horizontal scroll.

**Verification:** the gate, all four cases.

### Block 6: HomeNotice

1. `index.html`: `#home-notice` above the rooms.
2. `app.js`: without a Cube, "No Cube is configured";
   with one not connected, "Cube <name> is not connected",
   its address, when it last sent data (`updated`, else `last_seen`),
   what to check, and a button to System settings → Cubes
   for those who may open it; above the last rooms where there are some.
3. `run.sh`: a second `serve` whose Cube address is a closed loopback port,
   its own port and data directory.
4. Tests: against the second server the notice and its address;
   stubs for no Cube and a drop after the rooms loaded.

**Verification:** the gate.

### Block 7: Screenshots

1. `screenshots.py` retakes the whole set, a shot of the 👤 menu
   and of ☰ at 390 px added.
2. `origin/main` merged, the gate once more,
   the branch pushed and the pull request opened.

**Verification:** the shots looked at; the pull request's checks run.

## Proposed commit message

Block 2, for the form the others follow:

```text
feat(ui): Say who is signed in, in a user menu in the header

The header's user menu shows the name and role, and holds My profile, My settings and Sign out, which leave Settings (AP Navigation v1.0, Block 2).

Key changes:
- internal/server/ui/index.html, components.js: the user menu, closing as the room menus do.
- internal/server/ui/login.js, partials/page.html: the line and Sign out move to the menu.
- internal/server/ui/i18n: the menu's texts in all six languages.
- tools/ui-test: the owner, the operator and an open server.
- docs/ui.md: the header.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-07): The first revision, from the TL;DR gates of 2026-11-07:
  D3 narrow and D5 the owner's, D4-A, D6-A, D7-A, D12-A, D13-A, D14-A;
  D3 at full width open, with the owner's alternative as B.
