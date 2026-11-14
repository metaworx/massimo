# AP IconChoice v1.0: each person's window and radio icons

## TL;DR

Each person chooses how the window and the radio are drawn, between the
two versions the evaluation had, with their marks.

- **Window:** the opening sash of before AP Signs (the default), or the
  original software's paned window; a closed window's type icon
  follows
- **Radio:** the Wi-Fi fan (the default), or the tower from the
  thermostat's display
- My settings → Appearance → "Window icon" and "Radio icon", kept like the
  other settings, for the person or this device

### Blocks

1. Drawings: both sets in the sprite with their marks; `SIGNS` by the
   setting; tests
2. Setting: the two choices in My settings, their defaults; i18n; UI
   tests; `docs/ui.md`

The two blocks are one pull request.

## Discussion

The owner's points of 2026-11-14 evening:

- "can we make user preference for the following two icons: Window: the
  two versions we had; Radio error: also the two versions";
- D106-A: a plan of its own, run after AP RoomTile, as both touch the
  tiles' icons;
- D107: the defaults are the Wi-Fi fan and the opening sash of before
  AP Signs.

`PLAN+ as recommended` on 2026-11-14.

## Analysis

**The drawings.** AP Signs put the paned window (`icon-window`,
`icon-window-open`, `icon-window-open-small`) and the Wi-Fi fan
(`icon-radio`, `icon-radio-small`) in the sprite, with the corner marks.
Before it, the sprite had the cross-barred frame (closed), the sash
(open), and the tower; mock v8 drew their marked forms (the sash with a
mark inside, the tower moved aside). `components.js` `SIGNS` names the
drawings of each kind; `tiles.js` `deviceIcon` picks a device's type
icon.

**The setting** joins `SETTING_KEYS` in `app.js` as AP Signs' "Blinking
signs" did, with its check, storage and device switch.

## Implementation Plan

### Block 1: Drawings

- The sprite: the sash set (closed frame, open sash, its small form) and
  the tower (its small form), beside the paned window and the Wi-Fi fan.
- `SIGNS` and `deviceIcon` read the setting; the marks as before.
- Tests: each choice's symbols in a tile's head, in a problem list and as
  a type icon.

**Verification:** `make ui-test` (`test_rooms.py`, `test_problem_list.py`).

### Block 2: Setting

- `app.js`: `iconWindow` (`sash`, `paned`) and `iconRadio` (`wifi`,
  `tower`) in `SETTING_KEYS`, their checks, defaults `sash` and `wifi`.
- `partials/appearance.html`: two rows with their device switches.
- i18n, six languages; UI tests: the choices kept, the page redrawn.
- `docs/ui.md` "My settings"; a fragment.

**Verification:** `make ui-test` whole, `go test ./internal/server/`,
`GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(ui): Both drawings of the window and the radio, with their marks`;
Block 2, `feat(ui): Each person chooses the window and radio icons`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.
