# NOTE UnsavedMarks v1.0: how block 5 began

What of Block 5, Sections, is done, the choices taken in the run,
and what is left.
It was built with AP UiParts' Block 10, whose NOTE says how that block went.

## What is done and what is left

- **Done, with UiParts' Block 10:**
  the pills of System settings' sections and the header's menu,
  in the step "fix(ui): The settings lists keep a panel's draft".
- **Left, with UiParts' Block 13:**
  the cards of Personal settings, My profile, My settings and Notifications,
  which have no place for a pill yet, and the menu's item that leads there.

## How the marks are placed

- **A region of its own, `ui/src/chrome/`,**
  drawn once as `parts.js` loads,
  since the chrome is plain until UiParts' Block 14 and the log's mark until Block 11.
- **Places the plain chrome leaves alone,** each a `part-slot`:
  - a section's pill in its `span.section-pill`, beside the plain badges,
    which `sectionBadges` empties without touching the pill's place;
  - a menu item's disk in its `span.menu-item-mark`;
  - ☰'s mark in `#nav-menu-mark`.
- **`data-forms`** names the beginnings of the keys of the forms a place shows:
  a section's its list's (`users`, `roles`, `providers`, `integrations`),
  with its name's text key in `data-title`;
  System settings' item `users roles providers integrations`;
  Cube settings' item `cube` (D24).
- **`unsavedTone`** of `shared/forms.ts` gives the worse state of the forms
  under those keys: `bad` after a failed save, else `warn`, else nothing.
- **☰'s mark** is the worse of the log's and the forms' of every menu item:
  `renderLogSign` of `log.js` hands the log's count and tone to `MassimoParts.chrome.log`;
  ☰'s name and tooltip say both.
- **An item of a pill** names the form and opens it with the focus in it,
  from any page, its card unfolded first where it is folded.

## Which forms have marks

- **With marks,** each a settings form with Save, Reset and the disk:
  a user's, an identity's change (under its user's key), a role's, a provider's,
  an integration's name and an integration's maximum.
- **Without, and why:**
  - the forms that add a user, a role or a provider are commands (D26);
  - an integration user's Acts as is a command, though it keeps its draft (D25);
  - the switches saved at once have no Save, as D6-A has it:
    the password sign-in, a capability for every role,
    whether integrations may ask, an integration's Enabled;
  - Backup and restore holds commands alone, so its section has no pill,
    and neither has Capabilities for every role,
    whose switches are saved at once.

## Decisions

Taken in the run under the owner's `EXEC++`, each the owner's to change:

- **D24.**
  ☰ and the menu's Cube settings item also mark Cube settings' unsaved forms,
  since D9-A has the pill, the section and the menu's sign say it.
  The item gets a disk of its own with `data-forms="cube"`,
  and ☰ joins it as it joins the others.
  A room's form on the Rooms page has no item in ☰, Rooms being a header button,
  and gets no mark there.
- **D25.**
  An integration user's Acts as keeps its draft and gets no marks:
  Link also takes a proposal the select already shows,
  so it cannot be off while nothing is changed; it is a command.
- **D26.**
  The new item's forms, a user's, a role's and a provider's, are commands:
  no marks and no question on leaving, as D21 has the Copy tab.

## For Block 13

- **The pills' places for the cards of Personal settings:**
  a `span.section-pill.part-slot` in each card's head with its `data-forms`
  and `data-title`.
- **The menu's item for Personal settings,** `#menu-me`,
  gets a `span.menu-item-mark.part-slot` with the beginnings of its forms' keys,
  and ☰ joins it as it joins the others.
- **A room's forms have no item in ☰,**
  so the Rooms page's unsaved forms are marked in the tile and the Cube's entry alone,
  as D24 leaves them.
- **What is shown or typed once,** a new password, one's current one,
  a key just made, is a part's own state, never a draft (UiParts' NOTE).

## What verified it

The tests of the marks are in `test_admin_updates.py`:
the pill, ☰ and System settings' disk, failed and joined with the log's warning;
Cube settings' disk on ☰ (D24); a pill's item unfolding its card.
The counts are in AP UiParts' NOTE,
`GUIDELINES/wip/2026-11-21_01-10_NOTE_UiParts_v1.0_how_block_10_went.md`.
