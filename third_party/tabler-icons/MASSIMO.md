# Icons from Tabler Icons

The web UI's sprite (`internal/server/ui/index.html`) draws four of its
icons from Tabler Icons 3.24.0 (`@tabler/icons`, MIT, see `LICENSE`),
copied as paths into symbols of the page:

- `icon-leaf`: `leaf`, the eco button;
- `icon-shuffle`: `arrows-shuffle`, a room whose thermostats are in
  different modes;
- `icon-room`: the outline of `home`, a thermometer of Massimo's own
  drawn inside it, the room's virtual wall thermostat;
- `icon-tool`: `tool`, the Maintenance page.

The rest of the sprite is Massimo's own, drawn in the same 24-unit box and
stroke.

System info's credits (`make credits`, `tools/credits/README.md`) take the
package and its version from this table:

| Directory | Package | Version | Licence |
|-----------|---------|---------|---------|
| `.` | `@tabler/icons` | 3.24.0 | MIT, `LICENSE` |
