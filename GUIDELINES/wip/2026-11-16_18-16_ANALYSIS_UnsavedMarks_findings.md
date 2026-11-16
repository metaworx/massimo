# ANALYSIS UnsavedMarks: findings

Findings of the look into the owner's proposal of 2026-11-16,
made at D184 of AP UiParts' Block 7.
A sub-agent read the code on `claude/ui-rooms` after the merge of main;
nothing was built or run.
Four of its findings were checked again by hand and are marked so.
Paths are under `internal/server/ui/` unless they say otherwise.

## What the owner decided already

- What is typed survives its form's closing, for every form,
  until it is saved, reset or the page is left.
- D186-A: amber when the form is left with unsaved changes,
  and 60 s after the last change.
- D187-A: the parts only, each region as it becomes one.
- D188: settings without a pill near them get a pill in their section,
  and a sign in the header's menu.
- D189-A: any failed save turns red, also a value the server refused.
- Save is paired with a Reset or a Cancel or both; the form decides.
- The signs stand before the button's text; after a save the OK check.

## 1. The forms

| Form | Where | Part or plain, block | Buttons today | Closing it | Knows it changed | Nearest pill or section |
|---|---|---|---|---|---|---|
| Cube settings | `src/cubes/Form.tsx:127-177` | part, 5 | Save | the panel folds and the form stays; going from the list to the page builds it again and loses the typing | `useDraft`; Save off while nothing is to send | the panel head's pill, the header's entry of the Cube |
| Virtual thermostat | `src/rooms/Devices.tsx:140-169` | part, 7 | Save | the device list closes and the drafts go | `useDraft`, but Save is never off | the device row's pill, the tile's |
| A room's configuration: room, temperatures, devices, device, programme, copy | `config.js:209,295,368,450,559,671` | plain, 8 | Cancel, which closes, and Send; the device tab Send and Remove; the programme also its resets | `closeConfig` drops the edit | `sendWhenChanged` | the tile |
| User, display name, identity | `users.js:365,423`, `providers.js:237` | plain, 10 and 13 | Save | a panel | `sendWhenChanged` | section Users; My profile has none |
| Role | `roles.js:126` | plain, 10 | Save | a panel | `sendWhenChanged` | section Roles |
| Provider | `providers.js:134` | plain, 10 | Cancel and Save | yes | nothing | section Providers |
| An integration's limits | `integrations.js:384-387` | plain, 10 | Save | a panel | nothing | section Integrations |
| Saved at once, without a Save | `prefs.js`, `notify.js:268`, `maintenance.js:382,494`, `update.js:208` and others | plain, 12 and 13 | none | | | only a failure applies |

The House panel, the holiday form and the tile's stepped setpoint are commands,
not settings, and are left out.

## 2. The shared parts, and what each lacks

- **`useDraft`** (`shared/field.tsx:45`):
  its state is the component's and goes with it.
  It cannot be given a place to keep it, cannot reset several fields together,
  and says "changed" to nobody outside the form.
- **`Meanwhile`:**
  enough as it is, once its base is kept with the draft.
- **`FormFooter`:**
  no Reset, no state, no icon before the text.
- **`busyProps`:**
  the turning ring comes after the text.
  A sign before the text has to be a child of the button,
  so that the button stays the element it was (the guard checks that).
- **`actionRunner`** (`shared/action.ts`): `run` answers yes or no and
  drops the error's status, so "not reached" and "refused" look alike.
- **`Problems`** (`shared/problems.tsx`): an entry cannot be clicked,
  and there is no kind for "unsaved".
- **`liveMap`:**
  usable as the store; it forgets all keys or none.

**The smallest design:**
one new file `shared/forms.ts`, knowing no region.

- **Drafts** are kept in a store by a key the region chooses,
  such as `cube:<serial>`; `useDraft` takes the place where given
  and behaves as today without one.
  They are forgotten at sign-out and when their Cube goes.
- **A registry** of the forms with unsaved changes:
  key, what part it is, state, error, and how to open it.
  The pill, the section and the menu read it.
- **Save's states:**
  clean (off, no sign);
  changed (no sign);
  unsaved (amber) on leaving the form, on its closing while changed,
  or 60 s after the last change;
  saving (the ring);
  failed (red) until a save goes through, Reset or Cancel;
  saved (the check), then clean.
- **Reset** takes the stored values into every draft;
  **Cancel** is Reset and the region's close.
- **A region writes:**
  the key, the part's name, how to open it, the pill's item among its own,
  and Reset or Cancel in its footer.

## 3. The pills

- **A room's tile:**
  `src/rooms/Tile.tsx`, its entries from `rooms/facts.ts`.
  It shows only while an item has a kind; an unsaved item makes it appear.
- **A Cube:**
  the panel head (`src/cubes/Panel.tsx:135-147`) and the header's list of every
  Cube's problems (`src/header/CubeProblems.tsx`).
- **An item that opens the part it names** needs an action on an entry,
  drawn as a button, and a kind `unsaved` with its text in six languages.
- **A section's pill:**
  `sectionBadges` (`components.js:817`) empties the section's badge box at every
  call (checked).
  A part's pill needs a place of its own beside it, in each section's partial.
  The cards of My settings have no badge box at all.
- **The header's menu** is plain (`partials/header.html`).
  Its one mark today is the log's (`log.js:1415-1430`).
  The unsaved sign has to join it as the worse of the two.

## 4. Failures

- **`api()`** (`app.js:260-295`): not reached is a `TypeError` without a status;
  refused is an error with a status and the answer's data.
  Behind a reverse proxy a dead server answers 502 or 504, with a status.
- **The runner** makes one red line of both.
- **A refused value on its field:**
  the server names no field (`{"error": message}` alone).
  Fields are outlined only by checks in the page (`aria-invalid`, checked:
  `components.js:648`, `users.js:110`).

## 5. The icon

- A `<symbol>` in `index.html`, and a kind in `SIGNS`
  (`components.js:188`) with the corner marks, as the cog and the log have.
- The icon choice holds the window and the radio only: no variants needed.
- No Go test checks that a symbol exists.

## 6. Tests

- **Save today:**
  Cube settings in `test_cubes.py:173-183` and `test_cube_page.py:77`.
  Nothing saves the virtual thermostat's settings.
- **The 60 s:**
  Playwright's clock (used once, in `screenshots.py`, checked),
  or a setting of the parts for tests.
  Whether the fixture can set the clock before the page's scripts run
  could not be told by reading.
- **Not reached:**
  a request aborted by a route, as the guard does once.
- **The guard** types and then moves the focus out.
  That will turn things amber:
  it has to accept the sign as a child of the same button,
  and clear the drafts between its views.

## 7. Collisions

The owner, 2026-11-16:
the other UI plans are on hold and wait for Block 7 and for this plan.
So this plan goes first after Block 7,
and the change to `Problems` is its own to make.

- **The same change to `Problems`** is named in Block 6's NOTE for the
  coming change to the header: an action as an item.
  That change then finds it made.
- **Block 8** removes the tile's focus-out handling for the configuration,
  the same event amber would use.
- **AP LogAlerts** owns the menu's mark.
- **Blocks 10, 13 and 14** move the forms of System settings and My settings,
  the section badges and the menu.
- **`docs/roadmap.md` item 5,** the UI when the server is gone (checked:
  `state.serverDown` exists): it overlaps "not reached".

## 8. A cut

| | Pull request | When |
|---|---|---|
| U1 | the store and the registry, `useDraft` with a place, `FormFooter`'s states and Reset, the runner telling reached from refused, the entry's action and the kind `unsaved`, the icon, the texts, `docs/ui.md`; first user Cube settings | after Block 7 is on main, before Block 8 |
| U2 | the rooms: the virtual thermostat kept across the close, Save off when unchanged, the row's and the tile's pill | after U1, before Block 8 |
| U3 | a room's configuration | inside Block 8, or right after it |
| U4 | a section's place and the menu's sign, with the log's mark | with Block 10 for System settings, Block 13 for My settings |

**Risks:**

- the guard, as above;
- the draft's base must be kept with it, or "changed meanwhile" breaks
  after the form is built again;
- a Cube's panel shown in two places shares one draft: wanted, but a change;
- the 60 s under the test suite;
- forgetting drafts at a Cube's change and at sign-out.
