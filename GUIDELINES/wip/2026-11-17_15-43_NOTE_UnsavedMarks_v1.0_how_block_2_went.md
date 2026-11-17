# NOTE UnsavedMarks v1.0: how Block 2 went

Where Block 2, Rooms, went differently from the plan,
the choices made in the run, and what verified it.

## What it made

- **The virtual thermostat's two settings** keep what is typed:
  when the device list closes and opens, when the rooms load again,
  when a device reports and while another Cube is chosen.
  Save shows the form's state, and Reset stands beside it.
- **The disk** while they are unsaved or a save failed:
  in the device row's pill, in the tile's,
  and under the Cube in the header's list and in its panel's head.
  Its click, from any page, chooses the Cube, shows the rooms,
  opens the device list and puts the focus into the settings.
- **In the shared parts:**
  a place says what stored value its draft holds;
  `hasChanges` says whether a form has changes; a field can be off.

## Decisions

Made in the run, and small enough to change afterwards:

- **D12.**
  A room form's disk shows under its Cube, on every page,
  and opens the form from anywhere.
  The builder had left it on the rooms page alone.
  It follows from D9-A: moving to another page or Cube asks nothing
  because the pills say where the unsaved settings are.
- **D13.**
  A form whose stored values load later, and that has its drafts already,
  draws from them at once; one that has none yet keeps its fields off
  until the values have come, and says so where they cannot be read.
  As first built, what was typed before the values came was dropped,
  and a list opened again showed other values for a moment.
- **D14-A.**
  Outside expert mode the disk shows all the same,
  and the virtual thermostat's row shows while its settings have changes;
  saved or reset, it is an expert's row again.
  Nothing switches expert mode on.

## Different from the plan

1. **Who did it.**
   A sub-agent on Opus 5.5, without a commit, and a rework of eight points
   after a second reader; the commits were made from recorded trees.
2. **Two commits:**
   what the shared parts gained; the settings.
3. **The drafts are kept by the room,** where the plan said by the device:
   a save that changes the radio address keeps the form.
   The key is `rooms:<serial>:<id>:vwt`;
   a room's configuration takes keys beside it, under the room's.
4. **A room's forms are not under the Cube's key.**
   The Cube's entry lists them beside its own;
   a Cube that is no longer served forgets both.
5. **Forgetting has a call of its own,** after the rooms' load,
   and only for a Cube that is connected with its greeting:
   before its first greeting the server answers the rooms' load with nothing,
   which the page reads as no rooms.
   Not connected, a Cube forgets no draft and keeps the rooms last seen.
6. **A room hidden in Cube settings** is no longer in the load's answer,
   and its forms are forgotten.
7. **After a save the form shows what was sent,**
   also where the stored values cannot be fetched again.
8. **The row's key is the room's,** not the radio address:
   saving a new address builds nothing again.
9. **No drawing for expert mode was needed** once D14-A was taken;
   a plain function written for it went again.

## For the blocks that use it

- **A room's forms in the tile's pill** are listed through the virtual
  thermostat's device entry today.
  A configuration tab needs an entry of the room, and one list of the
  room's forms that does not name the virtual thermostat twice.
- **A tab's open** calls the configuration's own opening with its tab;
  this form's open closes the configuration.
- **A form's open has to work from anywhere:**
  the header calls it, and the list on leaving the page will.
  What it needs after a load is kept as a request and carried out by the
  call after the load.
- **A disconnect in a test** is the Cube's `connected` set to false in
  the page with an empty answer for its rooms; no real one is made.

## What verified it

- **The new tests on the page before the change:**
  all nine failed, in the builder's run; they pass after it.
- **That drawing settles:**
  a test counts the drawings of the form and its pills while the form is
  unsaved and both disks show.
  With Block 1's fault put back on purpose the page froze and the test hung,
  in the builder's run.
- **Reuse:**
  a second reader against the configuration's tabs; eight points reworked.
- **Before the merge of main:**
  `make ui`, gofmt, `go vet` for linux, windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make ui-check`,
  the types' `-check` and the guidelines' twelve checks pass;
  UI tests in all four cases, in the builder's run and here:
  thirteen test files there, 532 passed and 60 skipped; fourteen here,
  540 passed and 60 skipped.
- **After the merge of main:**
  the same, and `make cross`.
- **UI tests after the merge of main:**
  fourteen test files in all four cases, the guard among them:
  556 passed and 60 skipped, none failed.
- **Not run:**
  the whole UI suite, which is CI's; `make race`; `make lan-test`.
