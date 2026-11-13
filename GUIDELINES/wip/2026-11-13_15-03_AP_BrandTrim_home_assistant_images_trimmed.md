# AP BrandTrim v1.0: Home Assistant's images trimmed

## TL;DR

The brand tool's Home Assistant images are trimmed to the mark, as Home
Assistant's brand rules ask, so that the integration can carry them.

- `--scope homeassistant` → the icon on the 512 box, the logo with a 6 % margin
  → both cropped to what is drawn, the icon squared on its wider side
- the other scopes → unchanged, byte for byte
- the tool → v1.2.0

### Blocks

1. The trimmed images: the `homeassistant` scope, the README, the version

## Discussion

**Where it comes from.**
The Home Assistant integration (metaworx/massimo-homeassistant, its AP
Integration v1.4, Block 6) carries the mark's brand images in
`custom_components/massimo/brand/`; its owner decided there that they are
trimmed by this tool (D27-A, 2026-11-13), and that the change is built on
AP WebBrand's branch `claude/web-brand`, which has changed the tool, its pull
request stacked on that branch.

## Analysis

Home Assistant's brand rules ask for an icon of 1:1 at 256 px (512 at `@2x`)
and a logo whose short side is 128 to 256 px, transparent and trimmed:
no margin around what is drawn.
The tool's images keep one: at 256 px the icon has 13 px left and right and
22 px above and below, the logo about 6 px all round.
Cropping the rendered image to its drawn pixels is exact for every flavour,
whatever its hands or letters reach; rendering at four times the size first
keeps the edges as clean as the other scopes'.

## Implementation Plan

### Block 1: The trimmed images

1. `trimmed()`: render at four times the size, crop to the drawn pixels,
   square the icon on its wider side, scale to the size asked.
2. The `homeassistant` scope writes its icons and logos through it.
3. `VERSION` 1.2.0; `tools/brand/README.md` says that scope's images are
   trimmed.

**Verification:**
`--flavour grayorange --scope homeassistant`: the icons 256 and 512 px square,
the logos 128 and 256 px high, the drawn pixels touching the edges (the icon's
wider side); the other scopes' files byte for byte as before;
`check-plans.sh`, `check-citations.sh`, `changelog.sh check --staged`.

## Proposed commit message

```
feat(brand): Home Assistant's images are trimmed to the mark

AP BrandTrim v1.0, Block 1: the homeassistant scope crops its icons and logos to what is drawn, the icon squared on its wider side, as Home Assistant's brand rules ask; every other scope's files are as before.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-13): the plan, from the Home Assistant integration's
  AP Integration v1.4, D27-A.
