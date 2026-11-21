# NOTE UnsavedMarks v1.0: how block 5 ended

What of Block 5, Sections, was left and is now done,
the choices taken in the run, and where the plan stands.
It was built with AP UiParts' Block 13, whose NOTE says how that block went,
and ends what
`GUIDELINES/wip/2026-11-21_01-10_NOTE_UnsavedMarks_v1.0_how_block_5_began.md`
began.

## What was left and is now done

- **Left after UiParts' Block 10:**
  the cards of Personal settings, which had no place for a pill,
  and the menu's item that leads there.
- **Done, with UiParts' Block 13,**
  in the steps "fix(ui): My profile keeps what is typed"
  and "fix(ui): The appearance and the scheme editor keep an edit".

## How the marks are placed on Personal settings

- **The pills,** each a `span.section-pill.part-slot` in a card's head:
  - My profile's, with `data-forms="profile"`;
  - My settings', `#prefs`, with `data-forms="scheme"`,
    the scheme editor being in its Appearance card.
- **The menu's item,** `#menu-me`,
  has a `span.menu-item-mark.part-slot` with `data-forms="profile scheme"`;
  ☰ joins it with the other items' marks and the log's, as it joins the others.
- **A pill's item** opens its form in sight from any page:
  the card unfolded, the focus in the form.

## Which forms have marks

- **With marks,** each a settings form with Save, Reset and the disk:
  the display name, the own password, the scheme editor.
- **Without, and why:**
  - My settings' rows and the Appearance choices are saved at once (D6-A);
  - the notification rules are saved a second after the last change
    and have no Save (D27);
  - the link's confirmation in My profile is a command.

## Decisions

Taken in the run under the owner's `EXEC++`, each the owner's to change:

- **D27.**
  A notification rule has no Save and gets no marks, as D6-A has it,
  so the Notifications card has no pill.
  The block's brief had named it among the forms.
  The other way: a rule as a form with marks,
  which would need a Save the card does not have.
- **D28.**
  A new password typed in My profile is a changed form:
  the marks, and the question on leaving,
  as System settings' user form has them.
  The current password alone is not,
  since a password manager may fill it in.
  The forms' store holds only that it changed and a way to empty it,
  never the text.
  The other way: any field of the password's form,
  the current password's too, counting as a change.

## What is not marked, across the plan

- **A room's form on the Rooms page** has no item in ☰,
  Rooms being a header button,
  so it is marked in the tile and the Cube's entry alone,
  as D24 leaves it.
- **Settings saved at once,** wherever they are,
  have no Save and get nothing (D6-A).
- **Commands,** the forms that add a user, a role or a provider,
  an integration user's Acts as, and the link's confirmation,
  get no marks (D25, D26).

## All five blocks are built

With this, every section of the plan's Block 5 is built:
System settings and Cube settings with UiParts' Block 10 (D24 to D26),
Personal settings here.
Every settings form with a Save on the page has marks.
The plan's five blocks are now built;
this block's last part reaches main with UiParts' Block 13.
Its retirement is the owner's word, and is not claimed here.

## What verified it

The tests of the marks are in `test_personal_updates.py`:
My profile's pill, ☰ and Personal settings' disk,
the pill's item unfolding the card;
a new password marking its form and Reset emptying it,
a current one alone marking nothing;
the scheme editor's colour left unsaved
marking the pill, ☰ and Personal settings,
the pill's item bringing the editor back from a folded card,
Reset clearing them;
Save as and Enter saving only a change.
The counts are in AP UiParts' NOTE,
`GUIDELINES/wip/2026-11-21_08-03_NOTE_UiParts_v1.0_how_block_13_went.md`.
