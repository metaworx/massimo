# NOTE MobileApp v1.3: how block 14 went

Block 14, the second phone check's findings (F25 to F28, D54 to D57),
run on 2026-11-21 after the plan's v1.3 was registered on `main`,
as one pull request, metaworx/massimo#390 (`claude/mobile-check-fixes`):
"fix(ui): The sign-in card centred, with Cancel and Password, as the second
check found" (steps 1 to 3),
"fix(app): Forget all revokes the keys, and a key left is revoked at the next
session" (steps 4 to 7),
and "merge: main, with System settings' lists as parts, into the second phone
check's fixes".
Each part was built by a sub-agent from a brief and read by two second
readers, the app's part in three rounds, before its commit.
Where the work went differently from the block's steps:

## How it ran

- **The merge of `main`.**
  AP UiParts' Block 10 (#389) had moved My profile's sessions and keys
  from `users.js` into `profile.js`, and System settings → Users into a part;
  `users.js` kept main's form,
  and the marked key (`markProfileKey`, the row's `token-marked`, its scroll
  into view, the mark cleared once another page is shown) went with
  My profile's rows into `profile.js`.
  Nothing of `main` called `keepCardInView`, which this block renamed
  `keepInView`.
- **Checked here**, each exit 0:
  `make ui`; `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`;
  `gofmt -l .` and `go vet` for linux, windows and darwin; `make ui-check`;
  the UI files of the changed areas in the four cases
  (`test_login.py`, `test_connect.py`, `test_users.py`, `test_header.py`'s
  address and session tests, `test_main_menu.py`, and after the merge
  `test_oidc.py`, `test_sign_in_resend.py`, `test_settings.py`,
  `test_profile_help.py`);
  the Gradle tasks `:app:assembleFossDebug :app:testFossDebugUnitTest
  :app:lintFossDebug :app:lintNightlyDebug`;
  `go test ./tools/identifiers/...`; `GUIDELINES/shared/tools/check`.
  The new Kotlin tests were each seen to fail with their fix undone.
- **Left to CI**: the whole Go and UI suites, `make race` and the cross builds.
- **CI's first run** failed one test once, at 390-de,
  `test_system_layout.py`'s `test_header_and_page_bar_stay`,
  and passed it on the retry.
  It was not this block's:
  Integrations' line on the LAN comes with its own answer,
  a part's since UiParts' Block 10,
  which may land after the roles and grows the page by 106 px
  between the test's two readings of its height,
  on `main`'s build as on this branch's.
  "test(ui-test): The page bar's test waits for System settings' reads
  before it measures" fixed the test on the branch;
  the second run passed every job.

## Departures from the steps

- **Remove with a kept login turns it off first**, as Turn off does
  (the session, else one prompt), then removes the server;
  its question says so, and its toast names the key ids still valid there.
  Step 5 asked only that Remove warn of a key left;
  removing a server forgot its login and left the key valid, unwarned.
  The owner decides at the completion gate.
- **Forget all signs out** of each server as Turn off does,
  not only revokes; before, it kept the sessions.
- **The key's id of a login kept before Block 13** comes from
  `GET /api/v1/me`'s `token.key_id` (the session's own row, with the user),
  not from the user's list of tokens;
  a check that finds it keeps it with the login.
- **Keys left from more places**: besides Turn off and Forget all,
  a login forgotten because the phone no longer opens its key
  (a new fingerprint, `KeyInvalidated`) or because a certificate was trusted
  anew records its key, where the node is the same;
  a changed node does not, the key being another server's.
- **What a key left's revoke asks first**: the user's list of sessions and
  keys, one request per check; an id is revoked only where the list has it
  as kind `api`, and never the token in use, the key it was made from or the
  login's own key, ids a store rolled back may give again.
  An id the list does not have as a key is dropped unasked;
  an empty answer drops nothing.
- **An address of its own**: Turn off and Remove, given none
  (the server probing or offline when tapped), look for the address that
  answers, as Forget all does, before a key is left.
- **The answers** (contract version 4): `forget` answers `{revoked, left}`,
  `forgetAll` `{revoked, left, local}` (server ids),
  `remove` `{revoked, keys, local}` (key ids):
  `local` is a key forgotten on the phone alone, its id not known,
  still listed at the server under My profile.
- **The prompt while the app is hidden**:
  androidx.biometric shows no prompt, and calls nothing back,
  once the activity's fragments have their state saved (Home pressed between
  two prompts of Forget all); such a prompt is answered at once as dismissed,
  and that key is left.
  The test is the library's own, `isStateSaved`, not the lifecycle's state,
  which is still CREATED in `onStart`, where the cover asks to unlock.
- **"Select server…"** is one text, shared by the card's ☰,
  the main ☰ and the Cube choice.
- **Cancel** is also on the first owner's form in the app.
- **No shortened German key**: at 390 px the row was measured on one line
  in all six languages, with the fingerprint shown and without it.
- **The CHANGELOG fragments** were folded into the bullets they amend
  (COMMIT.md 5.2): the sign-in card's, the menu's, the sessions' and the
  login by fingerprint's, which takes Block 13's Turn off fragment;
  the keys left have a bullet of their own.

## F28

The code showed no path that signs out before Turn off's revoke:
the connect page calls `forget` alone, and `forget` signs out after it.
It showed two defects, both fixed:
a login kept before Block 13 had no key id, so Turn off went the key's way,
with the prompt, though a session was held;
and on that way the login was forgotten before the sign-out,
so a session kept in memory was forgotten on the phone, not ended.
The sign-out the audit showed fourteen seconds before the revoke came from
outside Turn off; step 12 of Block 13 again on the phone tells where.

## Not checked off the phone

- The Bridge's wiring: the prompts, the answers dropped once the activity
  went (`onIo`), `remove`'s answer, `setToken`'s check after a new session,
  the keys left where a login is forgotten (`forgetLeavingKey`);
  and the race `Sessions.put` guards.
- Small things left:
  `login.keep`'s undo is lost where the activity is recreated during its
  prompt, and its new key stays valid, recorded nowhere;
  the toast covers the open settings' foot at 390 px until it is closed
  (older than this block);
  Show in My profile may find the key revoked already by the sign-in it
  leads to;
  androidx.biometric may cancel a second prompt started while the first
  closes, and that key is then left, not stuck.

## The phone check to propose

1. The sign-in card: centred, its title too; the row
   "fingerprint | Password | API key ☰" on one line;
   Cancel back to the list; the spinner inside the fingerprint's button.
2. Forget all with one server where the app holds a session and one where it
   holds none: one prompt, for the second; the toast; the audit of each.
3. A key left: the rig's serve stopped while Turn off runs;
   the server's line with Show in My profile, the key marked there;
   its revoke at the next sign-in as that user, the line gone; the audit.
4. Remove of a server with a kept login: the question,
   the key revoked or named in the toast; the audit.
5. Block 13's step 12 again, Turn off while signed in:
   the revoke by the session, then the sign-out; the audit read after it.
6. A server asked at each return: the cover's prompt after the app comes back,
   so that the prompt's new test (`isStateSaved`) takes nothing from it.
