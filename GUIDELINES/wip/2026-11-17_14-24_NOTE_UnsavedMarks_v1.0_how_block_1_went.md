# NOTE UnsavedMarks v1.0: how Block 1 went

Where Block 1, Mechanics, went differently from the plan,
the choices made in the run, and what verified it.

## What it made

- **`ui/src/shared/forms.ts`:**
  the drafts' store by a form's key, the forms with changes not saved,
  and a form's hook with its state, its save and its Reset.
- **`useDraft` with a place,** and `FormFooter` with Save's look and Reset.
- **The problem list:**
  an item with an action, and the kind `unsaved`.
- **The disk,** as a symbol and as a sign with the corner marks.
- **Cube settings,** the first user:
  what is typed is kept, Save shows its state, Reset stands beside it,
  and the Cube's problem lists show the disk, which opens the settings.
- **`tools/ui-test/test_unsaved.py`,** and a marker for a test that moves
  the page's clock.

## Decisions

The owner, 2026-11-17: each block of this plan, when it is done,
is pushed and gets its pull request.

Made in the run, each as its builder recommended,
and small enough to change afterwards:

- **D10-A.**
  After a save the check on the button fades after 5 s;
  the message in the Cubes' message line stays,
  as every message there did before.
  One runner writes that line, and a second one for saves alone
  could wipe the other's text.
- **D11-A.**
  A key typed in a form marked unsaved makes it changed again:
  the amber sign and the disk go until the form is left again or a minute has
  passed.
  A failed save stays red.
  A form with nothing left to send is clean, also after a failed save.

## Different from the plan

1. **Who did it.**
   A sub-agent on Opus 5.5 wrote it in the session's own worktree,
   without a commit,
   and reworked eight points after a second reader had checked it;
   the commits were made from recorded trees.
   It stood still overnight and once more when the session restarted,
   and was woken to finish.
2. **Two commits:**
   the shared parts; Cube settings.
3. **The guard is unchanged.**
   It passes with the new code as it is:
   it needs neither to accept the sign nor to clear the drafts between its
   views.
4. **The 60 s are not a setting.**
   A test moves the page's clock with Playwright's,
   which has to be installed before the page's scripts run:
   the marker `clock` of `conftest.py` does that.
5. **The action is an item's, not an entry's:**
   the disk is one badge inside a Cube's entry.
6. **The list of forms holds the changed ones too,**
   not the unsaved and failed alone, for Block 3:
   a form changed and still focused is not unsaved when the page is closed.
   The pill leaves them out.
7. **The list keeps no error text.**
   The shared runner says only whether a request was taken;
   the error is in the message line.
   Named as a gap.
8. **The time a success stays shown** moved from the rooms' script to
   `components.js`, since a shared part reads it now.
9. **The focus after a click on the disk** asks whether the form is
   shown by a check every browser has; the first one written is missing
   in Safari before 17.4.
10. **The part's name** in the item is the text Cube settings has already;
    no new one was added for it.

## What the second reading found

Three faults that later blocks would have built on, all corrected:

- **A drawing without end, waiting for Block 2.**
  The list was set again at every drawing of an unsaved form,
  because how to open it was compared as a function.
  A pill on the row that holds its form would have drawn the form,
  which would have set the list, which would have drawn the pill.
  The list now carries one function per form for its whole life.
- **A Cancel that resets and closes marked the form unsaved,**
  since Reset left it counted as changed until its next drawing.
- **The question on leaving could not see a form still being typed in.**

## For the blocks that use it

- **One form hook per form element.**
  A form of tabs takes a key per tab under its owner's key;
  that also gives a section its beginning of a key.
- **A form whose stored values load later takes no place before they
  have loaded,** or its draft starts from a placeholder.
  The virtual thermostat's settings are such a form.
- **A part that reads the unsaved forms also reads its region's counter.**
  A part that reads a signal is drawn again only for that signal:
  the Cube panel's head missed a Cube changed in place until it did.
- **An item's action closes the list,** as every button in a menu does.
- **A region forgets its forms after a load that succeeded,**
  never at a drawing: a failed load must forget nothing.
- **`form-reset` and `form-cancel`** are the footer's two classes,
  the same in every form; a test selects by them and the form's place.

## What verified it

- **The new test on the page before the change:**
  all six of its tests failed, in the builder's run; they pass after it.
- **One test cannot fail on the old code yet:**
  the one for a form taken away after a Reset or a save.
  A Cube's form taken away is drawn again at once, in its other place.
  It bites from Block 2 on.
- **Reuse:**
  a second reader against the plain forms of the later blocks;
  eight points reworked.
- **Before the merge of main:**
  `make ui`, gofmt, `go vet` for linux, windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make ui-check`,
  the types' `-check` and the guidelines' twelve checks pass;
  twelve UI test files in all four cases, 448 passed and 60 skipped,
  in the builder's run and here.
- **After the merge of main:**
  the same, and `make cross`.
- **UI tests after the merge of main:**
  fourteen test files in all four cases, the guard among them: 508 passed and 60 skipped,
  none failed.
- **Not run:**
  the whole UI suite, which is CI's; `make race`; `make lan-test`.
