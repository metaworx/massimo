# Icons from Tabler Icons

The web UI's sprite (`internal/server/ui/index.html`) draws five of its
icons from Tabler Icons 3.24.0 (`@tabler/icons`, MIT, see `LICENSE`),
copied as paths into symbols of the page:

- `icon-leaf`: `leaf`, the eco button;
- `icon-shuffle`: `arrows-shuffle`, a room whose thermostats are in
  different modes;
- `icon-room`: the outline of `home`, a thermometer of Massimo's own
  drawn inside it, the room's virtual wall thermostat;
- `icon-tool`: `tool`, the Maintenance page;
- `icon-dice`: `dice-5` (`icons/outline/dice-5.svg`, SHA-256
  `f17aa29f77a1717da7de1e96fc33d0794480f0c27f58667905a5fb777bfb9eab`),
  the dice beside a new password that suggests one; its five dots are
  circles of radius 0.5 filled with the icon's colour and stroked as the
  rest, as Tabler draws them, where the sprite's other symbols are
  stroked paths.

The rest of the sprite is Massimo's own, drawn in the same 24-unit box and
stroke.

System info's credits (`make credits`, `tools/credits/README.md`) take the
package and its version from this table:

| Directory | Package | Version | Licence |
|-----------|---------|---------|---------|
| `.` | `@tabler/icons` | 3.24.0 | MIT, `LICENSE` |
