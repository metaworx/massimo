# AP Signs v1.0: one sign system for the web UI

## TL;DR

The web UI's icons, badges and colours become one sign system: a state
is a colour and a mark, so that it reads without colour too, and what
needs attention blinks. The server gets a sign of its own with four
states.

- amber icons #D05911, amber text #B34D0F, red #991B1B (light); dark
  orange #FFA94D, red #FF453A; Manual keeps its own amber fill
- marks: none / triangle / cross on the network, radio, window and cog
  icons; radio → the Wi-Fi fan with the mark at its bottom right, the
  window → a paned window with the mark there (the original app's)
- badges → bold, a triangle or a cross before amber or red text
- blinking → amber triangles, red crosses, the server down; personal
  setting System / On / At first (5 s) / Off
- problem list → one entry format at every level, sub-entries
  indented
- window → closed green, open blue, after 5 min amber, after 10 min
  red; the two limits are server settings
- server sign → connected, update out, new version, unreachable

### Blocks

1. Colours: the new token `warn-mark`, the new values, all three schemes
   and the editor, a 3:1 rule for icons; tests
2. Marks: the sprite's marked icons, the Wi-Fi radio, the paned window;
   badges with a mark, bold
3. Blinking: the marks that blink, the personal setting
4. ProblemList: `problemsEl`'s entry format, sub-entries, the marks in
   the list
5. Window: the three states, the two limits as server settings
6. ServerSign: the four states before the update time, the poll's
   failure noticed
7. Docs: `docs/ui.md`, the six languages, the UI tests at 1100 and
   390 px

The seven blocks are one pull request. AP CubeBadges builds on it.

## Discussion

The owner's evaluation of the status signs, in mock rounds 5 to 8 on
2026-11-14 (the page "Cube Choice Signs"), settled:

- D70-C2: amber in two strengths, icons #D05911 and text #B34D0F (the
  clock's #ED6E21 no longer fills a badge); outlined badges stay
  outlined, "filled is different from not-filled";
- Manual alone keeps the filled amber #8A5F00 (dark #E3B341);
- the red darker and the text bold; a bright red would come closer to
  the orange (green-blind ΔE 1 against the text);
- D84-A, dark: a lighter orange #FFA94D and a strong red #FF453A
  (ΔE 29, brightness 1.8, green-blind ΔE 12, against today's 18, 1.3
  and 13);
- D79-A: the marks by state, none, a triangle, a bigger cross; the
  network's triangle clear of its bar; battery, proxy and file amber
  only, without a mark;
- D80-B, D81-A: amber triangles blink, every red cross blinks, the
  server's down sign blinks;
- D82-A: the personal setting "Blinking signs", System (follows the
  system's reduced motion), On, At first (5 s), Off;
- D83-A: badges bold with a triangle or a cross before amber or red
  text, neutral badges (a mode) regular; in a problem list the mark
  replaces the kind's icon;
- the list: one entry format at every level, an icon, the name, a grey
  detail, the badges under the name; sub-entries indented to the name,
  a guide line down from the parent's icon;
- the window (D71-A): closed filled green, open filled blue (D90-A),
  after 5 minutes an amber outline with a triangle, after 10 minutes
  filled red with a cross; the two limits are server settings, set in
  the settings file and the API (D89-A), not yet in the UI (roadmap
  item 6);
- the server's sign, in a frame before the update time: connected (a
  green plug), update out (a blue "i", for those with the settings
  right, D88-A; a click opens Software), new version running (an amber
  triangle, blinking; a click reloads), unreachable (a filled red
  square with the open plug, blinking, the other signs greyed); the
  mark replaces the plug, so that the frame keeps its width (D87-A);
- on 2026-11-14, while the plans were written, the owner pointed at the
  original app's icons: the radio a Wi-Fi fan with a triangle at its
  bottom right, the window a paned window with its mark in the same
  corner; the Wi-Fi fan replaces the tower from the thermostat's LCD;
- D86-A: this plan holds the shared signs; AP CubeBadges builds on it,
  AP TileBadges' work is merged.

`PLAN++ as recommended` on 2026-11-14.

## Analysis

**The colours** (`theme.js`, `style.css`). `THEME_TOKENS` holds 19
tokens; `BUILTIN_SCHEMES` has massimo (the CSS's blocks, checked by
`TestUIDefaultSchemeMatchesCSS`), contrast and graphite; a saved user
scheme lacking a token takes the default's (`hexColour(obj[k]) ||
def[k]`). `CONTRAST_PAIRS` are the text pairs the editor rates; the
test requires every pair, the muted text and ok/warn/bad on a tile at
4.5. Icons are drawn in `warn` today, and so share the text's amber.
A 20th token `warn-mark` takes the icons' amber, rated at 3:1 against
the card (non-text contrast); `warn` becomes the text's deeper amber.
`bad` and `fill-bad` take the new red. Today `fill-warn` is the
Manual mode's fill (`tiles.js` `MODE_TONE`) and nothing else: it keeps
that role and its values.

Computed for the massimo scheme: #D05911 on white 4.1, on the dark
card #FFA94D 8.6; #FF453A on #202020 4.8, #151514 on #FF453A 5.4;
#991B1B on white 8.3. The contrast and graphite schemes get values of
their own, at the same rules.

**The icons** (`index.html`'s sprite). `icon-radio` is the LCD's
tower; `icon-window` a cross-barred frame, `icon-window-open` a sash;
`icon-battery`, `icon-cog`, `icon-plug-on`, `icon-plug-off`. The marks
are drawn as their own `<symbol>`s placed with the icon (an `svg`
holding two `use`s, or a symbol per marked icon), so that the mark's
paths carry the class that blinks. `PROBLEM_KINDS` (`tiles.js`) and
`problemsEl` (`components.js`) pick an icon per kind.

**The badges** (`components.js` `badgeEl(text, kind)`, `span.flag`
with a kind class). The kinds that warn or fail get the mark before
their text and the bold weight; the `mode` kind stays regular.

**The problem list** (`components.js` `problemsEl(entries, label)`):
a `details.menu.problems`, its summary one icon per kind present, its
list `problem-entry` rows of `who` and `flags`. The general format
adds an entry's `sub` (the grey detail) and `subs` (entries under it),
and puts the badges under the name.

**The window** (`tiles.js` `deviceProblems`, line 659's badges). The
server gives `received.window_open`, when it saw the current value: an
open window's age. `settings_api.go` holds the server's duration
settings (`stale_after` with its range, default, `applies: now`). The
two limits join it as `window_warn_after` (5m) and
`window_alert_after` (10m), 1m to 24h, alert not under warn. A reader
without the settings right cannot read `GET /api/v1/settings`; the
limits reach the page with the other facts a reader may see, where
Block 5 finds the place (`GET /api/v1/system` is read on the System
info page).

**The server's sign** (`app.js`). `checkVersion` reads `healthz` on
every poll and returns quietly on an error; a new version sets
`state.newVersion`, shows `#reload-btn` and reloads when nothing is
open. `poll`'s catch swallows the error: nothing tells the page that
the server is gone. `update.js` loads `GET /api/v1/update` for those
with the settings right and badges "update available". The sign
replaces `#reload-btn`.

**Blinking.** One class on `<html>` (`blink-on`, `blink-first`) from
the setting and `prefers-reduced-motion`; a CSS animation on the
marks' class. "At first" blinks five times, then stays.

## Implementation Plan

### Block 1: Colours

- `theme.js`: `warn-mark` the 20th token; its values in the three
  schemes, light and dark; massimo's `warn`, `bad`, `fill-bad` and dark
  `fill-fg` as settled; the editor's rows; an icon pair rated at 3:1.
- `style.css`: the token blocks (light, dark, the media query), the
  icons' amber in `--warn-mark`.
- `ui_test.go`: `TestUIDefaultSchemeMatchesCSS` at 20 tokens; the 3:1
  rule for `warn-mark`, `ok` and `bad` against the card.
- i18n: the token's name in the editor, six languages.

**Verification:** `go test ./internal/server/`, the scheme editor in a
browser, light and dark.

### Block 2: Marks

- The sprite: the network (top node, stem, bus; the triangle above the
  bus, clear of it; a bigger cross), the radio as a Wi-Fi fan, the
  window as a paned window, each with a mark at its bottom right; the
  cog; the marks `mark-warn` and `mark-x` with a class that blinks.
- `components.js`: `signEl(kind, state)` builds an icon with its mark
  and colour; `badgeEl` puts the mark before a warning or failing
  badge's text, bold; neutral badges regular.
- `tiles.js`: `PROBLEM_KINDS` and the device heads use `signEl`.

**Verification:** `make ui-test` at 1100 and 390 px; screenshots
looked at, light and dark, with the colour-blind filters of the mock.

### Block 3: Blinking

- `style.css`: the animation on the marks' class, under `blink-on` and
  `blink-first` (five times).
- `app.js`: the personal setting `blink`: `system` (the default;
  `prefers-reduced-motion: reduce` → off, otherwise on), `on`, `first`,
  `off`; the class on `<html>`, again on a change of the media query.
- `partials/appearance.html`: the setting's field in My settings.
- i18n, six languages.

**Verification:** a UI test for the class per value; reduced motion
emulated.

### Block 4: ProblemList

- `components.js` `problemsEl`: an entry is an icon, the name, a grey
  detail and its badges under the name; `subs` are entries under it,
  indented to the name, a guide line from the parent's icon; in the
  list a warning or failure shows its mark in place of the kind's
  icon.
- `style.css`: the entry's grid, phone width first.
- `tiles.js`: the tiles' entries in the format.

**Verification:** `make ui-test`; the list at 390 px with long names.

### Block 5: Window

- `internal/store`: the keys `window_warn_after`, `window_alert_after`;
  `settings_api.go`: their range, default and check; the settings file
  takes them as it takes `stale_after`.
- The page reads the limits where a reader may (found in the block,
  documented in `docs/api-v1.md` and the OpenAPI file).
- `tiles.js`: the window's state from `received.window_open` and the
  limits: closed `fill-ok`, open filled blue (`accent`), past the
  first limit an amber outline with a triangle, past the second
  `fill-bad` with a cross; the tile head's window sign the same; the
  state moves on with the page's clock between polls.
- Tests: the settings API's range and check; the UI test of each state
  with a faked age.

**Verification:** `go test ./...`, `make ui-test`.

### Block 6: ServerSign

- `index.html`: the sign in a frame before the update time, in the
  refresh button's row; `#reload-btn` goes into it.
- `app.js`: `serverState` from `checkVersion` and `poll`: `up`; `info`
  where `update.js` knows of an update and the user has the settings
  right; `new` where the server runs another version (a click
  reloads); `down` after a failed poll, back on the next success; the
  other signs greyed while down, the update time the last success.
- `update.js`: tells `app.js` of an update out.
- i18n: the four states' labels and tooltips, six languages.
- Tests: each state, by a faked `healthz` and update answer and a
  stopped server.

**Verification:** `make ui-test`; the four states looked at, 1100 and
390 px.

### Block 7: Docs

- `docs/ui.md`: the sign system (colours, marks, blinking, the list,
  the window, the server's sign); `docs/api-v1.md` for the two
  settings.
- `tools/ui-test`: the screenshots.
- CHANGELOG fragments, one per block that a user sees.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

The registration, with the other plans of the same evaluation, a commit
of its own on main.

The blocks:
Block 1, `feat(ui): Amber icons and text in two strengths, a darker red`;
Block 2, `feat(ui): A state's sign is its colour and a mark`;
Block 3, `feat(ui): Signs that need attention blink, as each person chooses`;
Block 4, `feat(ui): One entry format in every problem list`;
Block 5, `feat(ui): An open window turns amber, then red, after two limits`;
Block 6, `feat(ui): The server's sign says connected, update, new version or gone`;
Block 7, `docs(ui): The sign system`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation
  in mock rounds 5 to 8 and `PLAN++ as recommended`, with the original
  app's radio and window icons the owner pointed at.
