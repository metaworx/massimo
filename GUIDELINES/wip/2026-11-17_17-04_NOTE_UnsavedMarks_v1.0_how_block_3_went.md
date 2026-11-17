# NOTE UnsavedMarks v1.0: how Block 3 went

Where Block 3, Leaving, went differently from the plan,
the choices made in the run, and what verified it.

## What it made

- **The browser's prompt** on closing, reloading or leaving the page
  while a settings form has changes not saved.
- **The page's dialog,** where the user stays:
  the forms with changes, each a button that opens its form.
- **The same dialog at a sign-out,** which drops the drafts:
  Stay, or the button that signs out.
- **In the shared parts:**
  `FormsDialog`; the forms with changes as items;
  every changed form counted as left.
- **A region of its own,** `ui/src/leaving/`, for the dialog.

## Decisions

Made in the run, and small enough to change afterwards:

- **D15-A.**
  Closing the dialog puts the focus back where it was.
- **D16-A.**
  A click on a form in the sign-out's dialog opens the form and does not sign
  out.
- **D17.**
  The browser asks for every form with changes:
  the one still being typed in and the one being saved too,
  not only those marked unsaved, as D8-A is worded.
  Each of them would be lost.
- **D18.**
  The dialog comes a second after the browser's prompt.
  A browser does not tell the page how its prompt was answered,
  and a page that is left runs on until the next one comes:
  shown at once, the dialog appeared on a page that was leaving.
  Where the next page takes longer than a second it still does, briefly.
- **D19.**
  Where the last form is saved while the sign-out's question is open,
  the sign-out goes on.
- **D20.**
  After a session has expired, the sign-in keeps the drafts for the same
  user and forgets them all where another user signs in.

## Different from the plan

1. **Who did it.**
   A sub-agent on Opus 5.5, without a commit, and a rework of eight points
   after a second reader; the commits were made from recorded trees.
2. **Two commits:**
   the shared dialog and what the forms' list gained; the region and its users.
3. **Who mounts the dialog.**
   A region of its own with a place of its own at the end of the page:
   the page's chrome is plain until AP UiParts' Block 14,
   and no region imports another.
   It belongs with the chrome once that moves.
4. **The page's own reload for a newer server waits** while a form has changes.
   It would have raised the browser's prompt with nobody having done anything.
   The server's sign shows the newer version meanwhile,
   and the reload follows at the first poll after the form is saved or reset.
5. **The prompt's listeners are there only while a form has changes.**
   A page that always listens can lose the browser's back and forward cache.
   They are added by an effect of the list itself, not of a part:
   a part's effect came a frame late,
   and a reload right after typing was missed.
6. **The page hidden takes a stay's dialog back,**
   so that a page kept for the back button comes back without it.
7. **"Forget key"** asks as Sign out does, under its own name.

## For the blocks that use it

- **`FormsDialog` is the forms' list with a title,** not the page's modal.
  A later modal takes its shell out:
  opening on a property, the focus, the closing.
- **Block 5** needs the worse state of several forms, failed over unsaved,
  from the forms' list; it is not there yet.
- **A test of the browser's prompt** first gives the page a click:
  without one a browser shows no prompt whatever the page does.
  The first test written for "nothing asks" could not fail for that reason.
- **A test waits until the form is listed** before it leaves the page:
  a form is listed after its next drawing.

## Defects found outside the block

Not fixed:

- A request that met an expired session is sent again after the sign-in,
  whoever signs in.
  A save typed by one user can so be sent under another's key.
  It was so before this plan.
- A sign-in through a provider leaves the page,
  so the browser's prompt asks there too;
  staying then shows the dialog above the sign-in,
  and its forms cannot be shown while signed out.

## What verified it

- **The new tests on the page before the change:**
  four of the first five failed, and five of the ten after the rework
  failed on the block as first built, in the builder's runs.
- **What a headless browser cannot show:**
  the prompt as a person sees it.
  The tests answer the browser's dialog event.
- **Reuse:**
  a second reader; eight points reworked.
- **Main was merged before the block was built,** and had not moved when it was
  done.
  On that tree: `make ui`, gofmt, `go vet` for linux, windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`, `make ui-check`,
  the types' `-check` and the guidelines' twelve checks pass.
- **UI tests in all four cases:**
  eleven test files in the builder's run, 536 passed and 60 skipped;
  thirteen here, 576 passed and 60 skipped.
  None failed.
- **Not run:**
  the whole UI suite, which is CI's; `make race`; `make lan-test`.
