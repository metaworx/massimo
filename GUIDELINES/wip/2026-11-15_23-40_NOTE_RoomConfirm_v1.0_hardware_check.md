# NOTE RoomConfirm v1.0: hardware check

2026-11-15, D192-A on the owner's `EXEC D192-A`: the confirmations on the
owner's Cubes, Cube firmware 1.1.3, with the branch of pull request #316.

## How

- **Beside breno's serve:** the branch's build ran on jaguar as `serve -addr`
  to one Cube's LAN proxy on breno, with no store, so that no automatic reset
  could go out, and with `-proxy off -announce=off -discover-interval 0 -refresh 0`.
- **The room checks off:** a build overlay, never committed, kept them from
  sending a room's state again.
- **Four radio writes**, each undone by the next:
  - .25, room 1, three radiator thermostats in auto at 16 °C: manual at 16 °C,
    then auto again;
  - .22, room 2: its wall thermostat's eco from 5.5 to 6.0 °C, then back.

## What it showed

| Cube | Write | Answer | Shown after | Warning |
|------|-------|--------|-------------|---------|
| .25 | room 1, manual 16 °C | `pending`, mode open | 25 s, all three | none |
| .25 | room 1, auto | `pending`, mode open | 20 s, all three | none |
| .22 | wall thermostat, eco 6.0 °C | `pending`, `eco_temp` open | 15 s, the first re-read | none, "taken over" |
| .22 | wall thermostat, eco 5.5 °C | `pending`, `eco_temp` open | 16 s | none, "taken over" |

- **A wall thermostat's C line takes a 0x11** (note 10 of the run's NOTE):
  in `docs/cube-protocol.md` with the date.
- **The setpoint at 16 °C** was already shown, so manual waited for the mode
  alone, as it should.
- **Through a Massimo proxy that shows its virtual wall thermostats,** a room's
  members include the proxy's 0000xx devices, which take the room's state as
  the proxy sets them; they confirmed with the others.
  Only a serve behind another serve's proxy sees them.
- **The duty cycle** was 3 % before and 6 % after on .25, and 3 % on .22.
