# AP WebBrand v1.0: The mark in the web UI

## TL;DR

The brand tool writes the web UI's files with a scope of its own,
`make ui-brand` runs it, and the web UI takes them:
the logo over the word "Massimo", the mark alone on a narrow screen,
a favicon that follows light and dark, and the installable app's icons.

- `make_brand.py --scope ui` → `internal/server/ui/brand/`, fixed names:
  `brand.svg` (a sprite, no colours), `brand.css` (the brand's colours),
  `favicon.svg`, `favicon.ico`, tiles, full-bleed squares
- the sprite's parts coloured by `brand.css` → light, dark and the chosen
  mode as the page; the letters `var(--fg)`
- `<h1>` keeps the word; the drawing over it, its first layer the background:
  the word until the sprite has come, or for good where it never does
- under 600 px the mark alone, over the word's "M"
- manifest `any` → the tile; `maskable` and Apple → a full blue square,
  the mark within the safe zone
- `/favicon.ico` and `favicon.svg` (its own dark rule); the house icons go

### Blocks

1. BrandScope: the tool's `ui` scope, the hands split, the sprite,
   `brand.css`, `favicon.svg`, the full square; its README; `make ui-brand`;
   the files it writes
2. UiFiles: the server serves `ui/brand/` and `/favicon.ico`,
   SVG files with a policy of their own; the page's head, the manifest,
   the notifications' icon; the house icons gone; Go tests
3. Logo: the header, the sign-in dialog,
   the first owner's form and the connect page; `style.css`; `docs/ui.md`;
   UI tests; the screenshots

The three blocks are one pull request.

## Discussion

The owner asked, on metaworx/massimo#215 (AP Brand, merged and retired),
how Massimo's new mark comes into the web UI, as an `EVAL`;
the evaluation was answered on 2026-11-13 with D25 to D31.
The owner then asked:

- D27: "could the logo mask the word, so the word is shown as long as the
  graphic is not yet loaded? should the logo become the icon on a small width?"
- D28: "can you adopt the tools/brand/make_brand.py to create the required
  files, possible under an additional `scope`? launched by `make ui-brand`"

and, after the answers, `PLAN++ as recommended` (2026-11-13).

**The owner's decisions**, all as recommended:

- **D25-A:**
  the favicon as `favicon.ico` at `/favicon.ico` and as `favicon.svg`,
  the small mark with a dark rule of its own.
- **D26-A:**
  the manifest's `any` icons the tile;
  `maskable` and the Apple touch icon a full blue square,
  the mark within the safe zone; the notifications the tile.
- **D27-A, with the owner's two points:**
  the logo in the header, the sign-in dialog,
  the first owner's form and the connect page;
  the word under it until the drawing has come; under 600 px the mark alone.
- **D28, the owner's way:**
  a `ui` scope of `make_brand.py`, run by `make ui-brand`;
  the page draws the sprite's parts and `style.css` and `brand.css` colour them.
- **D29-A:**
  the files committed; the build needs no Python.
- **D30-A:**
  no animation now; the minute hand a part of its own in the sprite,
  so that one can come later.
- **D31-A:**
  this plan, its own.
- **D32-A:**
  the brand's colours in `brand.css`,
  written by the tool from its own constants.

**Names chosen here**, routine and not the owner's decisions:

- the folder `internal/server/ui/brand/`, the tool's own (its marker
  `.make_brand` committed beside the files, which the embed leaves out)
- the files `brand.svg`, `brand.css`, `favicon.svg`, `favicon.ico`,
  `tile-192.png`, `tile-512.png`, `maskable-192.png`, `maskable-512.png`,
  `apple-touch-180.png`
- the sprite's symbols: `logo-cover`, `logo-back`, `logo-m`, `logo-hour`,
  `logo-minute`, `logo-letters`, `logo-dot`,
  and for the small mark `mark-cover`, `mark-back`, `mark-m`, `mark-hour`,
  `mark-minute`
- the classes `brand-cover`, `brand-back`, `brand-m`, `brand-hands`,
  `brand-letters`; the tokens `--brand-back`, `--brand-m`, `--brand-hands`
- the flavour and the i as the owner chose them in AP Brand: `grayorange`,
  `--i dot`; the branch `claude/web-brand`

**Not in this plan:**
the apps' launcher icons and how the apps bundle `ui/brand/` beside
`connect.html` (the MobileApp work; the word stays where they do not);
the site's favicon; an animation; the tool's other scopes.

**Who runs it:**
this session, under the owner's `PLAN++ as recommended` of 2026-11-13.
The pull request is pushed and opened; merging it is the owner's.

## Analysis

### The tool (Block 1)

`tools/brand/make_brand.py` (v1.0.0) builds a flavour's files by scope
(`SCOPES`, `plan()`), into `--out`, which it prunes first,
and only where its marker `.make_brand` says it made the folder.
The mark is three paths (`mark()`): the gray M behind,
the blue M with its cut-outs (`outline()`, `fill-rule="evenodd"`),
and the hands in orange (`finger_d()`: the hub's square and the minute bar,
which turn together, and the hour capsule).
The logo (`logo_svg()`) adds the letters "assimo" in Lato as one path and the
i in its parts, the dot in the accent.
The small mark is the mark without the thermometers' scale (`scale=False`).
The tile (`tile_svg()`) is the mark in white at 0.72 on a blue square with
corners of 112 of 512.

The `ui` scope writes, flat into `--out`:

- **`brand.svg`**, a sprite of symbols without fills:
  for the logo (its viewBox) a cover rectangle, the M behind, the M,
  the hour hand, the minute hand with the hub's square, the letters,
  the i's dot; for the small mark (0 0 512 512) the same but the letters.
  `finger_d()` is split into its two hands, the same path data joined as before,
  so the other scopes' files do not change.
- **`brand.css`**, with the one-line licence comment the contract asks of CSS:
  the tokens `--brand-back`, `--brand-m`, `--brand-hands` for light and dark,
  in `style.css`'s pattern (the media query with `:root:not([data-theme="light"])`,
  and `:root[data-theme="dark"]`), from the tool's constants;
  the classes' fills, the letters `var(--fg)`;
  the logo's proportions as `aspect-ratio`, so that the page needs no
  numbers of the drawing.
- **`favicon.svg`**:
  the small mark with the light fills as attributes and the dark ones in a
  `@media (prefers-color-scheme: dark)` rule, on classes.
  It follows the browser's own light or dark, as the tab strip does.
- **`favicon.ico`**:
  as the `favicon` scope makes it, 16 to 256 px.
- **`tile-192.png`, `tile-512.png`**:
  the tile.
- **`maskable-192.png`, `maskable-512.png`, `apple-touch-180.png`**:
  the tile's blue as a full square, the mark in white at 0.62.
  Android's safe zone is a circle of 40 % of the side;
  the mark's farthest corner from the centre is 306 of 512 at full size,
  so 0.62 keeps it within (190 of 205);
  iOS rounds the corners itself and fills transparency black.

`make ui-brand` makes `tools/brand/.venv` where it is missing
(`python3 -m venv`, `pip install -r tools/brand/requirements.txt`,
which needs the network once) and runs
`make_brand.py --flavour grayorange --i dot --scope ui --out internal/server/ui/brand --yes`.

### Serving the files (Block 2)

`ui.go` embeds `ui/icons/*.png` and serves `/ui/icons/{file}`;
every file goes out with `no-cache` and an ETag of the version and the name,
and with the page's policy, `default-src 'self'`.
Under that policy an SVG's own `<style>` is refused where the browser applies
the policy to the image, so the SVGs under `/ui/brand/` get one of their own:
`default-src 'none'; style-src 'unsafe-inline'` (an SVG image runs no script).
`/favicon.ico` needs a route at the root, and a `Content-Type` of its own:
Go's table has no `.ico`.
The embed takes `ui/brand/*.svg`, `*.png`, `*.ico` and `*.css`,
which leaves the marker out.
The page's head: `favicon.ico` with `sizes="32x32"` and the SVG after it,
which Chrome then prefers; the Apple touch icon; `brand.css`.
The manifest and `sw.js` take the new names;
`ui/icons/` and its route go, and the tests that name them change.
Browsers keep favicons and installed apps their icons by address,
so new names make them take the new ones.

### The logo (Block 3)

The header's `.brand` holds `<h1>Massimo</h1>` and the version;
`connect.html` the same `<h1>`; the sign-in dialog a heading "Sign in";
the first owner's form "No users yet".
`<use>` of a symbol in another file draws nothing until that file has come.
So the `<h1>` keeps "Massimo" as its text, and an `<svg>` lies over it,
`aria-hidden`, of the logo's size: its first part, `logo-cover`,
is a rectangle in the background of the place (`--bg` in the header,
`--card` in a dialog), so the word shows until the sprite is there,
and stays where it never comes.
Screen readers read the word; nothing moves when the drawing comes,
the `<h1>`'s box being the logo's.
Under 600 px, where the header's first line also holds the time of the last
update and ↻, a second `<svg>` with the small mark stands over the word's "M",
the rest of the word visually hidden;
at about 24 px the full mark's scale would blur.
The parts take their colours from `brand.css`, so the chosen mode and the
schemes apply as to the rest of the page.

### Risks

- A browser that does not draw `<use>` of another file's symbol shows the word:
  the fallback is the design.
- The screenshots change on every page;
  they are retaken (`make ui-screenshots`).

## Implementation Plan

### Block 1: BrandScope

1. `make_brand.py`:
   `finger_d()` split into the minute hand with the hub's square and the hour
   hand, joined as before; `mark()` from the parts.
2. The `ui` scope: `sprite_svg()`, `brand_css()`, `favicon_svg()`,
   `square_svg()`, the PNGs and the ICO, flat into `--out`;
   `SCOPES` and `--list`; `VERSION` 1.1.0.
3. A check that the other scopes write what they wrote before:
   `svg`, `tile` and `favicon` of `grayorange` compared with v1.0.0's.
4. `tools/brand/README.md`:
   the scope and `make ui-brand`.
5. `Makefile`:
   `ui-brand`.
6. `make ui-brand`:
   the files into `internal/server/ui/brand/`.
7. `CHANGELOG.d/Added/claude-web-brand.md`.

**Verification:**
the comparison of step 3; the files open in a browser, light and dark;
`make ui-brand` a second time changes nothing.

### Block 2: UiFiles

1. `ui.go`:
   the embed, `/ui/brand/{file}` with the SVG policy, `/favicon.ico`;
   `ui/icons/` and its route gone.
2. `index.html` and `connect.html`: the head's links;
   `manifest.webmanifest`; `sw.js`.
3. `ui_test.go`:
   every file the head, the manifest and `sw.js` name is served, with its type;
   `/favicon.ico`; the SVG policy.
4. `docs/ui.md`.

**Verification:**
the Go gate, `make cross`.

### Block 3: Logo

1. The header's `<h1>`: the word, the logo over it, the small mark under 600 px;
   the sign-in dialog and the first owner's form; `connect.html`'s `<h1>`.
2. `style.css`:
   the places' sizes and covers.
3. `docs/ui.md`.
4. UI tests: the word with the sprite refused;
   the logo's colours in light and dark; the mark alone at 390 px;
   the accessible name; no horizontal scroll.
5. `make ui-screenshots`.

**Verification:**
`make ui-test`, the Go gate.
Branch `claude/web-brand` from `main`, one pull request.

## Proposed commit message

```
feat(brand): The brand tool writes the web UI's files

For AP WebBrand v1.0, Block 1: …

Key changes:
- tools/brand/make_brand.py: the ui scope …
- Makefile: ui-brand
- internal/server/ui/brand/: the files it writes
- CHANGELOG.d/Added/claude-web-brand.md: the bullet

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 and 3 in the same form:
`feat(ui): The web UI's favicon and app icons are Massimo's mark`,
`feat(ui): The web UI shows Massimo's logo`.

## Change History

- v1.0 (2026-11-13): the first revision, by `PLAN++ as recommended`,
  with D25-A, D26-A, D27-A and D28 as the owner put them, D29-A, D30-A,
  D31-A and D32-A.
