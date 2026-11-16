# AP UnsavedMarks v1.0: marks for unsaved settings

## TL;DR

A settings form keeps what is changed in it until it is saved or reset,
and the page says where changes are unsaved.
It is built once, in the shared parts,
and each region gets it as it becomes a part.

- a draft → kept by a key, outlives its form;
  forgotten at sign-out and when what it belongs to is gone
- Save: clean, plain and off → changed, accent → unsaved,
  amber sign → saving → failed, red sign, or saved, the check
- unsaved → on leaving the form, or 60 s after the last change
- Save paired with Reset, Cancel or both
- the nearest pill → a disk naming the part; a click opens it
- settings without a pill → one in their section; the menu → a sign
- leaving the page with unsaved forms → asked first, with their list
- plain forms → unchanged

### Blocks

1. Mechanics: the drafts' store, the forms' registry, Save's states,
   Reset, the pill's item, the icon; first user Cube settings
2. Rooms: the virtual thermostat's settings, the row's and the tile's pill
3. Leaving: the question on leaving the page and at sign-out, with the list
4. RoomConfig: a room's configuration, with AP UiParts' Block 8
5. Sections: a section's pill and the menu's sign,
   with AP UiParts' Blocks 10 and 13

## Discussion

### What is decided

The owner, 2026-11-16, at the gate of AP UiParts' Block 7,
where the proposal was made (D184 there);
the numbers are that plan's and are cited so:

- **UiParts.D184-A.**
  Block 7 went out as it was; this plan keeps what is typed, for every form.
- **What is typed survives its form's closing,**
  until it is saved, reset or the page is left.
- **UiParts.D186-A.**
  Amber comes when the form is left with unsaved changes -
  closed, or the focus gone elsewhere - and 60 s after the last change.
- **UiParts.D187-A.**
  The parts only, each region as it becomes one:
  Cube settings and the virtual thermostat first,
  a room's configuration with AP UiParts' Block 8.
- **UiParts.D188.**
  Settings without a pill near them, as System settings and My settings,
  get a pill in their section, and a sign in the header's menu as well.
- **UiParts.D189-A.**
  Any failed save turns the sign red, also a value the server refused.
  A save that went through, a Reset or a Cancel clears it.
- **After a save** the sign is the OK check.
- **Save is paired** with a Reset, back to the stored values, or a Cancel,
  back to the stored values and the settings closed where that applies, or both;
  the form decides.
- **The signs stand before the button's text.**

The owner, later that day, on the findings:

- **Save, clean:**
  off, no sign, no accent colour.
  **Save, changed:** no sign, the accent colour.
- **Leaving:**
  while a form is unsaved and the user closes or leaves the page,
  a dialog asks for confirmation, with a list of the unsaved forms,
  each of which can be clicked.
- **The other UI plans are on hold** and wait for AP UiParts' Block 7 and for
  this plan.
  So the change to the shared problem list is this plan's to make.
- **`PLAN++`.**
  No gate had stated a scope for it, so it runs this plan's blocks only:
  no block's branch is pushed, no pull request opened and nothing retired
  without the owner's word.

### Decided in the writing

Each as recommended, and each small enough to change afterwards:

- **D1-A.
  When a draft is forgotten** (the owner's question).
  When what it belongs to is gone from the server: its Cube removed,
  its room or its device deleted.
  And at sign-out.
  - Not when the Cube is disconnected: it is still there,
    and a save then fails, red, until it is back.
  - Not when another Cube is chosen: the drafts stay, each with its Cube,
    and that Cube's entry in the header's list shows the disk.
- **D2-A.
  How long the check stays.**
  It fades with the result line, after 5 s, and Save is clean again.
  A form opened an hour later then shows no check from before.
- **D3-A.
  A click on the pill's item** opens the part it names and puts the focus into
  it.
- **D4-A.
  How the signs are drawn.**
  On Save: the page's warning mark, amber; its cross, red; its check.
  In a pill: the disk as a sign with the corner mark,
  as the cog and the log have theirs,
  so that the state reads without its colour.
- **D5-A.
  A refused value** shows as the red sign and the message line.
  Its field is not outlined: the API names no field.
  That would be a plan of its own.
- **D6-A.
  Settings saved at once,** as the preferences and the switches of Maintenance,
  have no Save and get nothing.
- **D7-A.
  Saving while the server is not reached** is a failed save, red.
  The page when the server is gone (`docs/roadmap.md`,
  item 5) is not part of this plan.

### Still open

Both are Block 3's and are gated before it:

- **D8.
  What "asked when the page is left" can be.**
  When a tab is closed, reloaded or left,
  a browser shows a prompt of its own and nothing of the page's:
  no list, no text, nothing to click.
  - A (recommended):
    the browser's prompt while a form is unsaved;
    where the user stays, the page then shows its dialog with the list.
    At sign-out, where the page itself drops the drafts,
    its own dialog with the list asks first.
  - B: the browser's prompt alone, and the dialog at sign-out.
  - C: no prompt on leaving; the dialog at sign-out alone.
- **D9.
  Moving inside the page,** to another of its pages or another Cube.
  Nothing is lost there: the drafts are kept.
  - A (recommended): no question; the pill,
    the section and the menu's sign say it.
  - B: the dialog asks there too.

### What stays as it is

- The plain forms, until their region is a part.
- A command is no setting: the House panel, the holiday form
  and the tile's stepped setpoint keep their own ways.
- The "changed meanwhile" notice of a draft.

## Analysis

The findings this plan rests on are
`wip/2026-11-16_18-16_ANALYSIS_UnsavedMarks_findings.md`:
every settings form with its buttons and its nearest pill,
what each shared part lacks, the pills and sections,
how a save fails, the icon, the tests, and the cut.
What bears on the blocks:

- **A draft is the component's state today** (`useDraft`,
  `ui/src/shared/field.tsx`) and goes with its form.
  A Cube's panel keeps its typing only while it is folded,
  not from the list to the page.
- **`FormFooter` has one look for Save** and no Reset.
  The turning ring of a running request comes after the text.
- **An entry of the problem list cannot be clicked,**
  and it has no kind for "unsaved".
  Block 6's NOTE of AP UiParts named the same change for the header.
- **The guard types and then moves the focus out.**
  That turns a form amber:
  it has to take the sign as a child of the same button,
  and clear the drafts between its views.
- **A section's badge box is emptied at every call** of `sectionBadges`;
  a part's pill needs a place of its own beside it.
  The header's menu is plain and has one mark, the log's.
- **Block 8 of AP UiParts** removes the tile's waiting for the focus,
  the event amber uses, and moves the forms with the most to lose:
  a room's configuration drops its edit when it closes.

## Implementation Plan

As a block of AP UiParts is run:
a sub-agent on Opus 5.5 builds it in the session's worktree, without a commit;
a second reader checks it for reuse against the later regions' plain code;
the commits are made from recorded trees.
The reuse rule of the contract holds:
what is not about a region is a shared part, a region writes no copy of one,
and nothing is written without a user.
Each block is one pull request, pushed and opened at the owner's word.
Block 1 is cut from Block 7's branch of AP UiParts while that is not on main.

### Block 1: Mechanics

- **`ui/src/shared/forms.ts`,** knowing no region:
  - the drafts' store: a place by a form's key and a field,
    holding the draft and the stored value it started from;
    forgetting by a key's beginning;
  - the registry of forms with unsaved changes, as a signal:
    key, the part's name as a text key with its arguments, state,
    the error, and how to open it;
  - a form's hook: its state, the leaving of the focus,
    the 60 s after the last change, `save`, `reset` and `cancel`.
- **`useDraft`** takes a place and behaves as today without one.
- **`FormFooter`:**
  Save by its state - clean, plain and off; changed, accent; unsaved,
  the amber sign; saving, the ring; failed, the red sign; saved, the check,
  for 5 s - the sign a child before the text; Reset beside Cancel.
- **The problem list:**
  an entry with an action, drawn as a button,
  and the kind `unsaved` with its disk.
- **The icon:**
  the disk as a symbol and as a sign with the corner marks.
- **The texts,** in all six languages.
- **Cube settings,** the first user: its drafts by the Cube's serial,
  kept from the list to the page; Reset; the item in the panel head's pill
  and in the header's entry of the Cube, which opens the panel.
- **Forgetting:**
  at sign-out and when a Cube is removed.
- **The guard** takes the sign and clears the drafts between its views.
- **Tests,** `tools/ui-test/test_unsaved.py`:
  the states one after another, the 60 s shortened,
  a save not reached and a save refused, Reset,
  the pill's item and its click, the typing kept from the list to the page.
- **Documents:**
  `docs/ui.md`, "How the page is built" and "Cube settings";
  a CHANGELOG fragment.

**Verification:**
`make ui`, gofmt, `go vet` for the three platforms, `go test ./...`,
`make ui-check`, the guidelines' checks;
the touched UI test files in all four cases, the guard among them;
the new test fails on the page before the change.

### Block 2: Rooms

- **The virtual thermostat's two settings:**
  their drafts by the device, kept when the device list closes;
  Save off while nothing is changed; Reset.
- **The item** in the device row's pill and in the tile's;
  its click opens the device list at the row.
- **Forgetting:**
  when the room or the device is gone.
- **Tests:**
  the settings typed, the list closed and opened; the tile's pill and its click.
- **Documents:**
  `docs/ui.md`, "Rooms"; a CHANGELOG fragment.

**Verification:**
as for Block 1, with the rooms' test files.

### Block 3: Leaving

Waits for D8 and D9.

- **On closing, reloading or leaving the page** while a form is unsaved:
  as D8 decides.
- **A dialog with the list** of the unsaved forms, a shared part,
  each item opening its form.
- **Sign-out** asks first, with the list.
- **Tests:**
  the prompt asked for and not asked for; the list; sign-out.
- **Documents:**
  `docs/ui.md`; a CHANGELOG fragment.

**Verification:**
as for Block 1.

### Block 4: RoomConfig

Inside Block 8 of AP UiParts, or directly after it.

- **Each tab of a room's configuration** is a form with a key of its own;
  closing the configuration keeps its edit.
- **Cancel** resets and closes; the programme keeps its own resets.
- **The item** in the tile's pill opens the tab it names.

**Verification:**
as for a region of AP UiParts, with Block 1's test extended to a tab.

### Block 5: Sections

With Blocks 10 and 13 of AP UiParts, as their forms become parts.

- **A place for a part's pill** in each section's head of System settings,
  and in the cards of My settings, which have none.
- **The header's menu:**
  the unsaved sign, joined with the log's mark as the worse of the two,
  and a mark on the menu's item that leads there.

**Verification:**
as for a region of AP UiParts.

## Proposed commit messages

- The plan: `chore(aps): +UnsavedMarks v1.0, +ANALYSIS UnsavedMarks`
- Block 1:
  `feat(ui/shared): A settings form keeps its changes and marks them unsaved`,
  `feat(ui/cubes): Cube settings keep what is typed and say that it is unsaved`
- Block 2:
  `feat(ui/rooms): The virtual thermostat's settings keep what is typed and say that it is unsaved`
- Block 3: `feat(ui): Leaving the page with unsaved settings asks first`
- Block 4:
  `feat(ui/rooms): A room's configuration keeps its edit and marks it unsaved`
- Block 5:
  `feat(ui): A section and the menu say where settings are unsaved`

## Change History

- v1.0 (2026-11-16): first revision, by `PLAN++`,
  from the owner's proposal at D184 of AP UiParts' Block 7,
  with UiParts.D186-A, D187-A, D188 and D189-A as the owner chose,
  the owner's three points on the findings,
  D1-A to D7-A as recommended, and D8 and D9 open.
