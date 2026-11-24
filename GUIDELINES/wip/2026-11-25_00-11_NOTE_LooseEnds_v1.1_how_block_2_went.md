# NOTE LooseEnds v1.1: how block 2 went

Where Block 2, Card, went differently from the plan,
the choices taken in the run, and what verified it.

## What it made

- **A wrong password** is said in the page's language: "Wrong name or password."
  Before, the server's English words stood inside the page's sentence.
- **The first owner's form** clears its old line at each submit and gives the
  focus to the field at fault: Again after a mismatch, Name for a bad name.
- **Escape** leaves a card the user opened from the running page (the warning,
  ☰, Users), from its form and from the keep-open confirmation,
  and gives the focus back to what opened it (D3-A).
  With the card's own ☰ open, the first Escape closes ☰ alone.
  It does nothing in the start's card, in a sign-in the server asks for,
  or while a request of the card runs.
- **The API key's form** signs in once however often Enter is pressed;
  Sign in turns meanwhile.
- **The card's secrets go after a success**
  (D13-A): the password, Again and key fields of both cards are emptied,
  and an eye that showed one is closed again.
  A refusal, Skip for now, Back to the page, Escape, Cancel, Keep it open
  and the turn to the sign-in empty nothing.
- **Two texts:**
  the German sentence of the owner's form,
  and the switch in Users reads "Keep this server open without an owner" in both
  positions.
- **Two commits:**
  "fix(ui): The sign-in card's refusals,
  Escape and the key's second Enter" and "feat(ui):
  The sign-in card empties its secrets after a success".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **D41-A is taken at once.**
   The plan emptied the fields in the drawing after the close,
   and kept D41-A for a check by hand that failed.
   The first reading showed the order:
   the fields were empty before the first request that completes after the form is hidden,
   which is the moment a browser's password manager and the Android app's autofill
   take a sign-in for done.
   So the emptying waits for the page's first read after the success to settle:
   who is in for a sign-in, the settings for a new owner.
   No timer; a read that fails still ends in it;
   a card opened again meanwhile is not emptied late.
   The other way: at the close, as the plan had it.
   The owner's check by hand stays.
2. **One mark stands for every success.**
   Where a second sign-in closes the card before the first one's reads have settled,
   the first's settle empties the second's fields before a read of its own.
   It is rare, nothing typed is lost, and it is accepted;
   the code's comment says so.
   The other way: a mark per opening.
3. **Every 401 at the password's sign-in is worded as a wrong name or password,**
   also the sign-in that took longer than the server keeps its start:
   rare, and a new try passes.
   The other way: the server's words for that one,
   told by a code the refusal would carry.
4. **The key's form waits for the token to be set** before it ends the sign-in.
   In a browser that is at once;
   in the Android app it is the bridge's answer,
   so Sign in turns for as long as the app takes to look at the server's sessions.
   The password's way waits likewise where the app keeps the login.
5. **A missing password is the browser's to refuse.**
   The fields are required, so the form's handler never sees an empty one;
   the branch the plan had for it is cut,
   and a test holds the browser's refusal.
6. **The changelog names the wrong password alone.**
   The key's double sign-in came with the parts' card, after the last release,
   and a defect brought and cured within one release has no bullet of its own.
   The plan's bullet had both.
7. **The Escape tests are in `test_ways.py`,** where the three ways' helpers are;
   `test_users.py` is unchanged, since the switch shows on an open server alone
   and `test_keep_open.py` holds its label.
8. **A test of AP UiParts was rewritten:**
   the one that held "the fields are emptied at an opening alone"
   (UiParts.D210), which D13 reverses.
   The second reader compared the two line by line: only that assertion changed.

## What the readings found

One builder built it in an exported copy of main with Block 4.
Two readers read the recorded trees through git and ran tests in their own exports:
one the card's code and what a user can lose,
the other the tests, the texts, the documents and the messages.

Neither found a defect in the card's code.
Confirmed: the order of the two Escape listeners in every way the card opens;
the card is no modal dialog, so Escape does not close it natively;
no other Escape listener of the page is changed by the menus' handler marking
the event; nothing is emptied on a refusal or a way out;
both tests that pass on the tree before,
Escape in the start's card and Escape while Create runs,
fail on a planted fault.

Two musts, done: the changelog's bullet (6 above) and the emptying's moment (1 above).
Seven shoulds, done, among them:
the eye's check waits for the eye;
Skip, Back and Escape are tested to empty nothing;
a comment that promised the server's words for every other refusal says what is so.

The second reading found nothing left that must change:
every success reaches the emptying once,
a card open and being typed in is never emptied late,
and on every path a request completes with the fields still full before they are emptied.

## For the owner's check by hand

No test can show that a password manager still offers to save.
After the merge:

1. **Chrome:**
   clear the saved password for the server;
   sign in and wait for the offer to save; sign out, load the page again,
   and see that it fills.
2. **Chrome, a new owner:**
   on a fresh store, fill the three fields,
   press Create the owner and watch for the offer.
3. **Firefox:**
   the same two, watching for the key in the address bar.
4. **The Android app:**
   sign in by password and watch for the system's prompt to save;
   again with a new owner, and once with the login by fingerprint switched on.

Where one of them no longer offers to save: D41-B, the emptying taken back.

## For AP PasswordRules' blocks, which change the same forms

- `NewPassword` and `PasswordInput` take a number, `reset`,
  that rises at an opening and at a success settled;
  a field's eye closes with it.
- The first owner's form hands the focus to Name or Again in its submit's handler;
  a rule's line added there clears with the form's own at each submit.

## Seen and left

- **In the Android app the key's form waits for the bridge** (4 above):
  no test holds how long Sign in may turn where the server does not answer.
- **Two reads without a session behind the start's card** (the walk's finding 4)
  are Block 3's.

## What verified it

- **The builder, on the last tree:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for linux, windows and darwin,
  the Go tests of the server and the command line, each exit 0;
  the sign-in's UI test files in all four cases, 430 passed.
- **Seen failing first:**
  each new test on the tree before its step;
  the emptying's tests on the tree that emptied at the close.
- **The readers' own runs** in their exports, in two cases:
  24 and 26 passed; the planted faults seen failing.
- **The coordinating session:**
  the two trees and their fragments;
  both commits made after the strict changelog check of each;
  the merge with main, in which seven files met and merged by themselves;
  on the merge, `GUIDELINES/shared/tools/check`, twelve checks all well,
  `make ui` and the page's Go tests.
