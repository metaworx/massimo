# NOTE UnsavedMarks v1.0: how block 4 went

Where Block 4, RoomConfig, went differently from the plan,
the choices taken in the run, and what verified it.
It was built with AP UiParts' Block 8, whose NOTE says how that block went.

## What it made

- **Four tabs of a room's configuration are settings forms:**
  Room, Temperatures, Week programme, and on the Devices tab
  the radiator settings and a form per device.
  Their keys are `cfgFormKey(serial, id, form)`, `rooms:<serial>:<id>:cfg:<form>`,
  beside the virtual thermostat's `:vwt` under the room's key,
  the form `room`, `temps`, `program`, `valves` or `device:<radio address>`.
- **What is typed stays** from one tab to another and when the configuration
  closes; opened again it shows the tab it was on.
- **Save by its state, Reset beside it,** and Cancel, which resets and closes;
  the week programme keeps its own resets,
  and its draft is the programme kept in its form's place.
- **The disk** in the room's own entry of the tile's pill and under the Cube,
  "Living room, Temperatures: not saved";
  its click, from any page, opens the tab with the focus in the form
  (`openForm`, by `openRoom` of `store.ts`, which `openSettings` uses too).
- **Forgetting** as Block 2 forgets the virtual thermostat's,
  and a device's form also when the room no longer has the device.
- **`tools/ui-test/test_unsaved.py`:** Block 1's test extended to a tab,
  and the cases of the decisions below.

## Decisions

Taken in the run under the owner's `EXEC++`, each the owner's to change:

- **D21.**
  The Copy tab is a command, not a settings form:
  no marks, no disk, no question on leaving.
  So the plan's "A command is no setting" has it,
  against its Block 4's "each tab".
  What it copies stays chosen across a close.
- **D22.**
  After a copy that went through the rooms chosen are cleared,
  and what is copied stays as it was set.
- **D23.**
  A write the Cube discarded, or that failed in part, is a failed save and
  stays red: only what was taken counts as stored (from UiParts.D189-A).
  The plain code took the values sent as stored either way.

## Different from the plan

1. **Who did it.**
   A sub-agent on Opus 5.5, without a commit,
   and a rework of eleven points after a second reader;
   the commits were made from recorded trees.
2. **One commit,** after UiParts' Block 8's two.
3. **The room's own entry of the tile's pill** lists the configuration's forms
   (`cfgFormKey(serial, id)`), so the virtual thermostat's are listed once,
   under its own entry.
4. **The Copy tab is no form** (D21), where the plan made every tab one.
5. **An edit in the week programme** is a change of its form,
   so the minute starts again and an unsaved form is changed again (D11-A).
6. **The session was interrupted** on 2026-11-18 in the middle of the rework
   and went on on 2026-11-20.

## For Block 5

- A configuration's forms are keyed under the room's,
  so a section's prefix works for a room as it does for a Cube.
- The worse state of several forms is still not in the forms' list.
- A command, as the Copy tab and the House panel, registers no form.

## What verified it

- **The new tests on the page before the change:**
  they failed, also the four written in the rework on the tree before it;
  they pass after it.
- **Before the merge of main:**
  the checks of UiParts' Block 8 pass;
  twelve UI test files in all four cases: 506 passed and 58 skipped, none failed.
- **After the merge of main:**
  the same checks and nineteen UI test files in all four cases:
  662 passed and 2 skipped, none failed.
- **Not run:**
  the whole UI suite, which is CI's; `make race`; `make lan-test`.
