# Settings layout mock

A static mock of **System settings**, **My settings**, **My profile**
and the header's **👤** menu and Cube choice in a proposed layout,
for the owner to decide on before any of it is built (plan name
`SettingsLayout`).
Nothing here ships: `docs/` is outside `project.shipped`,
and the page is not served by `massimo serve`.

- `index.html` is the page.
  It loads the UI's own `internal/server/ui/style.css` unchanged,
  then `proposed.css`, the rules that would join `style.css`,
  and keeps the mock's own rules (its bar, the decision tags,
  what the browser draws itself) in a `style` element.
- `mock.js` sets the state from the address and does what the
  proposal does: folding, the table of contents, back to top,
  and the header and the page's row kept at the top while scrolling,
  with the sections menu marking the section in view.
- `shoot.py` takes the screenshots into `screenshots/`.

The orange tags, **D1** to **D13**, name the decisions the owner
took (2026-11-08); "expert: S/N" marks a device's serial in expert mode.
Every name, serial and address is a placeholder
(`Holiday Home`, `KEQ0000100`, `MEQ0000103`, `0a0103`, `192.0.2.10`).

## Opening it

Open `index.html` in a browser straight from the checkout:
it needs no server, only the relative path to `style.css`.
The bar at its top switches between the views; the address takes:

| Parameter | Values | What it does |
|-----------|--------|--------------|
| `page` | `system` (default), `prefs`, `profile`, `names` | the page shown; `names` is the Cube names proposal (D5) |
| `theme` | `light`, `dark` | as My settings → Mode sets it |
| `menu` | `user`, `cube`, `sections` | the 👤 menu, the Cube choice or the sections menu open |
| `scroll` | a section's id, such as `sec-users` | the page scrolled to it, the header compacted |
| `cube` | `2` | the Cube without a name chosen, which is not connected |
| `tip` | `user` | the 👤 button's tooltip, drawn |
| `as` | `manager` | System settings → Users as a manager sees it (D7) |
| `fold` | `all`, or section names: `software,maint,cubes,users,roles,caps,backup` | sections folded at the start |
| `open` | `store-info`, `config-info` | Maintenance's details open (D4) |
| `shot` | `1` | without the mock's bar |

## Screenshots

`screenshots/<view>-<width>-<theme>.png`, at 1100 and 390 px,
light and dark, retaken with the UI test's Playwright:

```bash
tools/ui-test/.venv/bin/python docs/mock/settings-layout/shoot.py
```

A filter takes only the matching ones, `shoot.py prefs-390`.
A tooltip and a native select's open list are drawn by the browser
and cannot be caught; the mock draws them, marked as drawn.
