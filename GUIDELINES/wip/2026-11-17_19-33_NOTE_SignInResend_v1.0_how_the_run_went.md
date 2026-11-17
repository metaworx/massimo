# NOTE SignInResend v1.0: how the run went

Where the plan's one block went differently from the plan,
the choices made in the run, and what verified it.

## What it made

- **A write that met an expired session is not sent again.**
  It fails at once and says that it was not sent,
  the sign-in opens, and the user does it again afterwards.
  A read is sent again, as before.
- **A stepped setpoint that waited to be sent** is dropped when the sign-in
  opens, and its tile says so.
- **Another user's sign-in after an expired session** does to the page
  what a sign-out does: nothing of the first user is left, shown or sent.
- **`tools/ui-test/test_sign_in_resend.py`:**
  eleven tests.

## Decisions

Made in the run, and small enough to change afterwards:

- **D6-A.**
  A change of the user's own settings whose push met the 401 is lost
  from the browser after the sign-in, as any other write:
  the page reads the server's settings then, and the text says to do it again.
- **D7.**
  Where another user signs in,
  the lines that say a write was not sent are taken back.
  They are addressed to someone who never did it.
- **D8.**
  The rule is about what goes out, not about one function:
  nothing a user did before the sign-in is sent after it under whoever signs in.
  So the stepped setpoint and the pushes that wait on a timer fall under it,
  which the plan did not name.
- **D9.**
  For the same user, the settings pushes that waited are held while the
  sign-in is open and go on after it: they are that user's own.
  A stepped setpoint is dropped for the same user too.

## Different from the plan

1. **Who did it.**
   A sub-agent on Opus 5.5, without a commit, and a rework of six points
   after a second reader; the commit was made from a recorded tree.
2. **The second reader found the same defect behind three timers:**
   a stepped setpoint, held two seconds before it is sent and planned
   again while a write is under way; the push of the user's settings;
   the save of the notification rules.
   The first build closed the page's two request functions alone.
3. **And a write of the page's own, older than the plan:**
   where the server has no settings for a user,
   the page pushes those kept in the browser.
   After an expired session they were the first user's,
   and went into the second user's account.
   A sign-out prevented that; the expired session's path did not.
4. **One path for "this user is gone":**
   what a sign-out does to the page is one function now,
   which another user's sign-in after a 401 calls too.
   Besides what it did before, it takes back the lines that say a write
   was not sent and closes the House panel.
5. **One request is still sent again:**
   the ticket of the event stream.
   It changes nothing, and a ticket is bound to whoever asks for it.
   Were it to fail at once,
   the stream's retries would go on while the sign-in is open,
   each a failed check counted against the address.
6. **The text** reads right before and after the sign-in:
   "not sent, since you were signed out: do it again once signed in".
7. **After the sign-in the message is said again** on the page's line
   only where the reload left that line empty.
8. **A result line that shows a failure is remembered,**
   so that the page can take it back for a user who is gone.

## Defects found outside the block

Not fixed:

- A request sent with the old key whose 401 arrives after the sign-in is
  done opens the sign-in a second time, though the page is signed in.
  A write so answered fails and is not sent again.

## What verified it

- **The new tests on the page before the change:**
  ten of the eleven failed, in the builder's run.
  The eleventh, the stream's ticket, tests what did not change;
  it fails where the exception for the ticket is taken out.
- **On the block as first built** the tests of the stepped setpoint and
  of another user's sign-in failed: they catch what the second reader found.
- **A second reader,** on every way the page sends a request;
  six points reworked.
- **Before the merge of main:**
  `make ui`, gofmt, `go vet` for linux, windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make ui-check`,
  the types' `-check` and the guidelines' twelve checks pass;
  UI tests in all four cases: twelve test files in the builder's run,
  484 passed and 60 skipped; fourteen here, 536 passed and 60 skipped.
- **Main was merged twice,** since it moved again before the push;
  the second merge brought the two blocks this branch was cut on,
  and nothing under the web UI or its tests.
  After each: the same, and `make cross`.
- **UI tests after the first merge:**
  eleven test files in all four cases, 424 passed, none failed.
- **Not run:**
  the whole UI suite, which is CI's; `make race`; `make lan-test`.
