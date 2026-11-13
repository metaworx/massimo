# The mark, and every file made from it

`make_brand.py` draws Massimo's mark - the bold M with a clock in its V and a
thermometer in each leg - and writes every file made from it: SVG, PNG, the
favicon, Home Assistant's brand images, and the animations, in which the
minute hand goes round and the mercury follows.

The drawing is one set of parameters; its **flavours** differ in the hands,
the hub and a gray M behind, and each is drawn for light and for dark pages.
The SVGs are plain filled paths - the M's stroke outlined, the cut-outs
subtracted - so that every renderer draws them alike.

## Setting up

```bash
python3 -m venv tools/brand/.venv
tools/brand/.venv/bin/pip install -r tools/brand/requirements.txt
```

The logo's letters are Lato Regular, in `fonts/` with its licence, the SIL
Open Font License (`fonts/OFL.txt`); `--font` takes another.

## Building

```bash
tools/brand/.venv/bin/python tools/brand/make_brand.py --list
tools/brand/.venv/bin/python tools/brand/make_brand.py --flavour grayorange --dry-run
tools/brand/.venv/bin/python tools/brand/make_brand.py --flavour grayorange --yes
```

A bare call prints its usage: a build prunes its target and may run for
minutes. `--help` has every option.

- **`--flavour NAME`**, repeated, or `all`: every flavour writes the same set.
- **`--scope`**, comma-separated, builds part of the set (below); default all.
- **`--theme light|dark`** builds for one kind of page only.
- **`--i`** draws the logo's "i": `dot` (its dot in orange, the default),
  `plain`, `orange`, `bang` (the MAX! exclamation mark), `bang-up` (upside
  down).
- **`--out DIR`** is the target, by default
  `<guidelines root>/temp/brand/<basename>`, the root as
  `GUIDELINES/config.ini` names it; **`--name`** the files' first name part,
  by default the flavour. Both take one flavour.
- The target is **pruned first**: on a terminal the tool asks; without one it
  refuses unless given **`--yes`**. It prunes only a folder it made itself
  (the file `.make_brand` says so) or an empty one, and refuses any other,
  `--yes` or not.
- **`--dry-run`** lists what would be pruned and written; **`--jobs N`**
  limits the rendering processes (default: every core); `-q`, `-v`.

Exit codes: 0 done; 1 a prune declined at the question; 2 could not run (an
option, a target it will not prune); 3 a requirement missing (a Python
package, the font).

## Flavours

| Name | Description |
|------|-------------|
| `classic` | the first blue mark: hands and thermometers carved, cut-outs transparent |
| `bang` | the minute hand the MAX! exclamation mark in orange, the hour hand carved, the hub open |
| `twin` | both hands orange exclamation bars, the square on the closed hub |
| `metaworx` | `twin` in metaworx's orange, the letters in its gray |
| `graycarved` | a gray M behind, so that cut-outs show gray on any page; the minute hand the orange exclamation bar, the hour hand carved |
| `graycapsule` | `graycarved`, the hour hand a carved capsule clear of the turning square |
| **`grayorange`** | `graycapsule`, the capsule in orange - **the chosen one**, with `--i dot` |

## Scopes and files

Files are named `<basename>-<form>-<theme>[-<size> | -turn-<speed> | -tick-1s | -tight]`,
form `icon`, `logo` or `small` (the icon without its scale, for 48 px and
below), theme the page it is for.

A drawing has a margin: the icon its 512 box, the logo 6 % of the M's
height on every side.
A tight one (`-tight.svg`) has none, for a page that places it exactly:
its height is the M's, from its top to its foot, its bottom edge the
letters' baseline, and its sides the M's left and the last letter's right.

| Scope | Writes |
|-------|--------|
| `svg` | `svg/`: the icon, the logo and the small icon, each also tight (`-tight.svg`) |
| `png` | `png/`: the icon at 256 and 512 px, the logo 128 and 256 px high (`-h128`) |
| `ladder` | `png/`: the icon at 64, 128, 180, 192, 1024 px, the small one at 16, 24, 32, 48 px |
| `tile` | `svg/` and `png/`: the mark in white on a blue square, for an app, at 180, 192, 512 px |
| `favicon` | `favicon.ico`, 16 to 256 px, the small icon up to 48 |
| `homeassistant` | `homeassistant-brand/`: `icon.png`, `icon@2x.png`, `logo.png`, `logo@2x.png` and their `dark_` ones, trimmed to what is drawn as Home Assistant's brand rules ask - the icon a square on the mark's wider side |
| `turn-fast` | `anim/`: the turn at 40, 80 and 160 ms a frame - 80 frames of 4.5°, a turn in 3.2, 6.4 or 12.8 s - as GIF and WebP |
| `turn-slow` | `anim/`: the turn at 1 s and 3 s a frame - 800 frames of 0.45°, a turn in 13 min 20 s or 40 min - as GIF |
| `tick` | `anim/`: 60 frames of a second, 6° each, a turn a minute, as GIF |
| `frames` | `frames/`: every frame drawn for the animations, as SVG |
| `ui` | the web UI's files, flat, by fixed names; `make ui-brand` writes them (below) |

Every turn is a seamless loop, without a pause. A GIF knows transparency only
on or off: its frames are drawn at four times their size, scaled down, and
every pixel is then one of the drawing's own colours or transparent - no
background at the edges on any page, at the price of hard edges. The WebPs
keep the soft ones.

A full build of every flavour is about 14,000 frames: ten minutes on sixteen
cores, and about 200 MB.

## The web UI's files

`make ui-brand` builds the `ui` scope of the chosen flavour, `grayorange`
with the i's dot in orange, into `internal/server/ui/brand/`, which the
server embeds; it makes `tools/brand/.venv` first where there is none,
which needs the network once.
The folder is the tool's: its marker `.make_brand` is committed with the
files, and the next run replaces them all.
Whoever changes the mark runs it and commits the files with the change;
the Go build needs no Python.

| File | What |
|------|------|
| `brand.svg` | a sprite of symbols without colours, both drawings tight: `logo-cover`, `logo-back`, `logo-m`, `logo-hour`, `logo-minute`, `logo-letters`, `logo-dot`, and the small mark's `mark-cover`, `mark-back`, `mark-m`, `mark-hour`, `mark-minute` |
| `brand.css` | the mark's colours for light and dark pages as `--brand-back`, `--brand-m`, `--brand-hands`, in `style.css`'s pattern; the classes `brand-cover`, `brand-back`, `brand-m`, `brand-hands`, `brand-letters` that colour the sprite's parts; the logo's and the mark's proportions, `--brand-logo-ratio` and `--brand-mark-ratio` |
| `favicon.svg` | the small mark, the light colours, and the dark ones under the browser's dark mode |
| `favicon.ico` | as the `favicon` scope makes it |
| `tile-192.png`, `tile-512.png` | the tile, for the manifest's `any` icons and the notifications |
| `maskable-192.png`, `maskable-512.png` | the tile's blue as a full square, the mark at 0.62, within the circle a maskable icon keeps |
| `apple-touch-180.png` | the same square, for iOS, which rounds the corners itself |

A page draws a part with `<use href="ui/brand/brand.svg#logo-m" class="brand-m"/>`;
the cover, first, takes `--brand-cover`, else the page's `--bg`, so that the
word under the drawing shows until the sprite has come
(`docs/ui.md`, "Header").
The minute hand with the hub's square is a part of its own, for a turn the
page might draw one day.
