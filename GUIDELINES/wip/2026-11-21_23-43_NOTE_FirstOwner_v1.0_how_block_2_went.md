# NOTE FirstOwner v1.0: how block 2 went

Where Block 2, Card, went differently from the plan,
the choices taken in the run, and what verified it.
The block puts the first owner's form into the sign-in card:
a server without users asks whoever opens its page for its owner.

## What it made

- **The card opens with the form that creates the owner**
  at the page's start and after each read of who is in
  that finds an open server for a caller on the local network,
  not kept open and not skipped in this load.
  Before, the form was a tile among the rooms, which asked nobody.
- **The start waits for the card:** behind it only the version and the update's state
  are read, not the Cubes, the rooms, the stream or the sign-in providers.
- **Skip for now,** under the form: the page as an open server's for this load;
  the next load asks again. Nothing is stored.
- **Create the owner** as before: the password registered, the owner created, signed in,
  in the app the login kept once.
  Then who is in is read before the card closes;
  the button turns until then, Skip for now and the app's Cancel off meanwhile.
  A toast says "Owner {name} created and signed in.",
  and in the app that the login by fingerprint was not kept, where it was not.
- **An owner made elsewhere meanwhile,** a 401 or the server's 409:
  the card turns to the sign-in for that server, "The owner exists now: sign in."
- **No card** on a server kept open, without a store, or with users.
- **The form's fields are not controlled,** as the card's own (UiParts D210).
- **Shared:** `TextField` gains a third way, not controlled, read through a ref;
  `UserNameField` and `NewPassword` take a value with its setter or a ref;
  Add a user and My profile's Change password are unchanged;
  `shared/me.ts` gains `firstOwnerOf`.
- **`login.js`:** `askLogin("owner")` opens the card on the form, saying nothing;
  `loadMe` asks for it, `loadMeAndPreferences` waits for it;
  `finishSignIn` reads who is in before the card closes on the owner's path.
- **Texts and documents:** `setup.skip` and `setup.exists` in six languages;
  `docs/ui.md`, `packaging/README.md`, the first start's text with its two copies,
  in `README.md` and `packaging/site/index.html`.
- **Tests:** `test_first_owner.py`, new; five other test files changed;
  `test_login.py` and `test_connect.py`, the app's acceptance test, unchanged.
- **One commit,** as the plan names it:
  "feat(ui): A server without users asks for its owner in the sign-in card".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **A design turn before any code,** answered by the coordinating session.
   Built in an exported copy of main with Block 1 in it,
   beside AP UiParts' Block 15, which held the session's worktree:
   one step a recorded tree, the commit made from it afterwards.
2. **D21-A.** Skip for now in a row of its own under the form,
   which Block 3's "Keep it open…" shares.
   The other way: in the form's row of actions.
3. **D22-A.** A toast after the owner is created, since the card has closed.
   The other ways: a toast only where the app kept no login; nothing.
4. **D23-A.** The owner's state has a height of its own:
   at 390 px about 702 px in a browser, 745 to 792 px in the app, by language.
   The sign-in keeps its height and place, 470 px at the top 215, unchanged.
   The other way: one place of one height for all three forms,
   which adds about 180 px of empty space to every sign-in.
5. **D24-A.** No focus placed after Skip, as after a sign-in.
   The other way: the banner focused, so that a screen reader reads it.
6. **D25-A.** The other tests' stubs of an open server without `setup` stay as they are,
   and get no card.
   The other way: `setup` and `kept_open` added to each.
7. **D26-A.** In the app the login is kept by one try, as before;
   a failure is said in the toast.
   The other way: the sign-in's box with Try again.
8. **D27-A.** The condition is checked in `loadMe`, at every read of who is in;
   the start and a sign-out wait for the card.
   The other way: checked at the start and at a sign-out alone.
9. **D42-A,** from the first reading.
   A server seen open that has a store opens a later sign-in on the password's way,
   also after Skip; one seen without a store on the API key's way, as before.
   Before, after Skip it was the key's way, from the owner's form the password's.
   The other way: the key's way after Skip, as it was.
10. **D43-A,** from the first reading.
    The owner's password fields are emptied at an opening alone, never at the closing,
    as the sign-in card's own (UiParts D210):
    a password manager takes the values when the form goes.
    So the typed password stays in the unseen fields
    until the page is loaded again or the card opens again;
    before this block the form went with who is in.
    The other way: emptied once the card has closed after Create.
11. **Closed on the way:** AP MobileApp's session had found on main
    that Create the owner's busy state ended before the read of who is in had answered,
    a window in which a second tap was told that the name is taken.
    The button now turns until the card has closed.
12. **The next free number** is FirstOwner D44;
    D28 to D41 are the designs of Blocks 3 and 4, answered while this block was read.

## What the readings found

A sub-agent on Opus 5.5 designed and built it.
Two independent readers on Opus read the recorded tree through git
and ran tests of their own in exports:
one `login.js`, the card's states and its store, the session and the app's side;
the other the form and the shared parts, the texts, the look, the tests,
the documents, the tree and its message.

Both confirmed:
a refused request's sign-in unchanged statement by statement;
no card flashing on a server kept open or without a store;
one read of who is in from the click to the page;
no password in a signal, a store, an attribute, a log line or a toast;
the app's bridge, its calls and their order as they were.

Three musts:

1. **Skip for now stayed live while Create the owner was busy,**
   found by both readers, each with a test.
   Tapped then, the start went on as an open page,
   and the creation's late end started the page a second time:
   a second poll, a second ticket of the stream.
   In a worse sequence, Skip, then a refused request's sign-in, then the late end,
   that sign-in's promise never answered and the reads waiting on it hung.
   Now Skip and Cancel are off while it is busy,
   and an ask whose card was closed or opened anew is ended no more.
   The same check serves the sign-in's own three ways,
   which had the hole in principle, though no user could close that card mid-sign-in.
2. **The site's page was stale:** it inlines the first start's text too,
   and only the README had been changed by hand; CI's guidelines check would have failed.
3. **A sentence of `docs/ui.md` claimed too much:**
   "the page behind the card reads nothing" holds at a start alone,
   and the version and the update's state are read even then.

Twelve shoulds, done, among them:
the sign-in's notice over plain HTTP hidden on the owner's form;
tests for the passwords' mismatch, the name's check while typing,
the toast of a login not kept, a 390 by 640 screen in the app,
each button's text on one line;
the Dutch and German texts; `packaging/README.md` saying "at each load".

The second reading: both readers verified the rework and found it settled;
the one that had found the hole of Skip during Create ran its own two tests again:
they pass. Two small points were done:
a comment that still said "seen open" where it now means "without a store",
and a count in a new test read too early to catch what it counts.

## For AP MobileApp's session

- **Unchanged:** the bridge's calls and their order:
  `login.status()` once per opening of the card, `login.keep()` once after the sign-in,
  `connect()` from Cancel and from ☰'s Select server;
  every `setup-…` id `test_connect.py` reaches; `fake_bridge.js`;
  the dialog not modal, the form a real form;
  `test_connect.py` and `test_login.py` unchanged and passing.
- **New:** the owner's form is inside the sign-in dialog (`#setup` in `#login-dialog`),
  where it was a tile among the rooms;
  Skip for now (`#setup-skip`) under it, beside the app's Cancel:
  Skip shows this server's page without an owner, Cancel goes back to the app's servers;
  `#setup-tls`, the self-signed line; a login not kept is said in a toast.
- **The phone as its own server** (D18-A) is asked at every start
  until it is kept open or has an owner.

## For Blocks 3 and 4

- **Block 3:** "Keep it open…" goes into `#setup-more` beside Skip;
  its confirmation into `#setup` in the form's place; the store gains the step.
- **Block 4:** the one opening is `askLogin("owner")`; `firstOwnerOf` serves ☰ and Users.
- **An opening on a running page** must mind, as the first reading showed,
  that once the owner exists and before the page holds its session,
  every request of the open page is refused:
  the page behind the card stands still, the poll stopped and the stream closed;
  the card does not turn while its own creation is under way;
  and a turn there runs what a refused request's opening runs.

## Seen and left

- An integration approved on an open server makes it a server with users and no owner:
  the card turns to a sign-in nobody can pass (D17-A, left to AP IntegrationUsers).
- The typed password in the unseen fields (D43-A).

## What verified it

- **The builder, on the last tree,** each exit 0:
  `make ui`, `gofmt -l .` empty, `make ui-check`, `go run ./tools/apitypes -check`,
  `go vet` for linux, windows and darwin, the Go tests of `internal/server` and `tools`;
  `sync-docs.sh --check`, the README and the site's page equal to their fragment;
  `reflow.sh --check` on every changed Markdown file; `commit-msg.sh`.
  UI: the whole suite in 1100-en and 390-de, 1231 passed;
  `test_first_owner.py` in all four cases, 76 passed.
- **The readers' own runs** in exports of the new tree:
  eight files of the sign-in in two cases, 188 passed,
  two of a reader's own stricter probes failing as they should;
  the step's ten other files in 1100-de and 390-en, 168 passed;
  the guard fails on a planted replacement of the form.
- **Shown failing:** on main without the block
  the new file's tests fail at "the card opens",
  and the six of "no card where not asked" pass, as forward guards;
  the builder's two new tests of Skip during Create fail on the tree before the rework.
- **The coordinating session:** the recorded tree and the builder's copy equal;
  the message; the changelog's check, strict;
  `GUIDELINES/shared/tools/check` on the commit in a scratch clone.
- **Not run locally:** the whole UI suite in 1100-de and 390-en, which CI runs.
