# AP Brand v1.0: The mark's generator

## TL;DR

Massimo's new mark, the bold M with a clock and two thermometers, and the
script that draws it and every file made from it, come into Massimo as
`tools/brand/`, with a command line to build one flavour, or part of one,
into a folder of its own.

- the mark → a script in the Home Assistant integration's session → `tools/brand/make_brand.py`
- one run for every design → `--flavour` (one or `all`), `--scope`, `--theme`, `--i`
- output → `<guidelines root>/temp/brand/<basename>`, pruned first, after asking or `--yes`
- the logo's letters → the system's Lato → Lato Regular vendored beside the tool (SIL OFL)
- bare call → usage; `--list`, `--dry-run`, `--jobs`, `--font`

### Blocks

1. The tool: `tools/brand/` with the script, its command line, the font,
   its requirements and README; the contract names it

## Discussion

**Where it comes from.**
The mark was designed in the Home Assistant integration's session
(metaworx/massimo-homeassistant, its AP Integration, NOTE "the icon", decision
D13-Z4 and its later rounds, 2026-11-13).
Its owner chose the flavour `grayorange`, the logo with the orange dot on the
"i", and asked for the generator to live in Massimo, where the mark belongs,
under `tools/brand/`, with a command line (that session's gate, D22-B,
D23-A, D24-A).

**Decided there, binding here:**

- **D22-B.** This plan, in a worktree of Massimo's own, and a pull request.
- **D23-A.** Lato Regular vendored beside the tool, with its licence;
  `--font` overrides it.
- **D24-A.** A bare call prints usage: building needs `--flavour`.
- **The pruning option is `--yes`**: the tool prunes its target first, asking
  on a terminal, refusing without one unless `--yes`, and only a folder it
  made itself (a marker file) or an empty one.

**Not in this plan:** Massimo's own icons in the web UI, the apps and the
site keep the house for now; replacing them is a plan of its own, which this
tool's output serves.

## Analysis

**The drawing** is one parameterised mark: the M as one thick stroke with
round joins, the cut-outs subtracted (skia-pathops), so that the SVGs are
plain filled paths every renderer draws alike (cairosvg and rsvg-convert give
the same pixels).
Its flavours differ in the hands, the hub and a gray M behind; each is drawn
for light and for dark pages.

**The animations** turn the minute hand once, the mercury following, as
seamless loops: 800 frames, of which every tenth for 40, 80 and 160 ms a
frame (GIF and WebP), all for 1 s and 3 s (GIF), and a tick turn of 60 frames
of a second. Every frame is kept as an SVG.
A full build of every flavour is about 14,000 frames, ten minutes on 16 cores
and about 200 MB.

**Python** is ignored by `detect-languages.sh` here (`[languages] ignore`),
and `tools/**` is not shipped: no changelog bullet.
The tool's environment, as `tools/ui-test/`'s, is a `.venv` beside it,
ignored by git.

**The command line** follows the invocation contract of the shared
`lang/shell/CLI.md`, which carries to Python: `--help`/`-h`,
`--version`/`-V`, exits 0 (done), 2 (could not run), 3 (an unmet
requirement), `-q`/`-v`; a bare call prints usage, as a build prunes and
writes for minutes.

## Implementation Plan

### Block 1: The tool

1. `tools/brand/make_brand.py`: the drawing as it stands, and the command
   line:
   `--flavour NAME` (repeatable, or `all`), `--scope` (`svg`, `png`,
   `ladder`, `tile`, `favicon`, `homeassistant`, `turn-fast`, `turn-slow`,
   `tick`, `frames`; default all), `--theme light|dark`,
   `--i dot|plain|orange|bang|bang-up` (default `dot`), `--out DIR` (default
   `<guidelines root>/temp/brand/<basename>`, the root read from
   `config.ini`), `--name BASENAME` (default the flavour; refused with more
   than one), `--yes`/`-y`, `--dry-run`, `--jobs N`, `--font TTF`, `--list`,
   `-q`/`-v`, `--help`, `--version`.
   Every flavour writes the same set of files.
2. `tools/brand/fonts/Lato-Regular.ttf` and its `OFL.txt`.
3. `tools/brand/requirements.txt`, `tools/brand/README.md` (set-up, the
   names, the flavours, the scopes); `/tools/brand/.venv/` in `.gitignore`.
4. The contract's source directories name `tools/brand/`;
   `tools/sync-docs.sh` regenerates `AGENTS.md` and `GUIDELINES/README.md`.

**Verification:**
`--help`, `--version`, a bare call (usage, exit 2), `--list`,
`--dry-run --flavour all`; a build of one flavour at one theme with every
scope, its files checked against the set; pruning: a marked folder pruned
with `--yes`, an unmarked one refused, no terminal and no `--yes` refused;
`check-plans.sh`, `check-citations.sh`, `changelog.sh check`.

## Proposed commit message

```
feat(brand): The mark's generator, with a command line to build a flavour

AP Brand v1.0, Block 1: tools/brand/make_brand.py draws Massimo's new mark - the bold M with a clock and two thermometers - and writes every file made from it, for one flavour or all, into a folder of its own.

Key changes:
- tools/brand/make_brand.py: the drawing, and --flavour, --scope, --theme, --i, --out, --name, --yes, --dry-run, --jobs, --font, --list
- tools/brand/fonts/: Lato Regular and its licence, for the logo's letters
- tools/brand/README.md and requirements.txt; its .venv ignored
- the contract names tools/brand/

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-13): the plan, from the Home Assistant integration's
  session, where the mark was designed.
