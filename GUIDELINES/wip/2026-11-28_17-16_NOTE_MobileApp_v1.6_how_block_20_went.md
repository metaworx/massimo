# NOTE MobileApp v1.6: how Block 20 went

The owner's `EXEC as recommended` of 2026-11-28 (D115-D, D117-A) built
Block 20 on `claude/mobile-menu-cards`, from `main` at
`chore(aps): +NOTE MobileApp v1.6, the copy and the codes`, in two commits:

1. `feat(ui): The menu's app entries last, the version's copy at the foot,
   one eye for a new password`
2. `feat(app): A removed server's revoke codes follow its node to a server
   of the list`

It merged `main` once, with AP KeyLife's Blocks 4 and 5 and the documents'
line breaks; the coordinating session pushed it, opened #506 and merged it
on 2026-11-28 once its CI was green.

## Where it went differently from the plan

- **D115-D, step 4.** AP ErrorCodes' Block 6 (#489) had already turned
  `copied` into `showBubble(text, el)` and taken `data-copied`, its CSS
  and `#copy-status` away, so nothing of D97-A's removals was left to do.
  What D115-D adds: the version rows of the server's ☰ and of the app's
  list's ☰ say "Copied" at the window's foot, `copied(null, ok)`, through
  a `foot` option of `copyable` and `copyIcon` (`connect.js`) and of the
  parts' `Copy`, `CopyIcon` and `CopyPair`. Every other copy keeps the
  bubble above its control, and a failed copy its own words, as #489 has
  them; `test_settings_copy.py` needed no change.
- **Step 3.** The server's version row had no copy icon on `main`, though
  the plan says it "keeps" one: it gets one (`Copy` with `icon`). Its copy
  closes ☰ through a new `after` option of `Copy`, told of the control in
  the click's handler once the clipboard was asked, which closes the
  menu and gives the focus to its summary; no timer.
- **Step 1.** The rule after Personal settings is left out too where
  nothing follows it (an open server without Sign out nor Create the
  owner), so that no rule stands at ☰'s end; the rules have ids
  (`menu-app-sep`, `menu-out-sep`).
- **Step 9.** Where the app's `connect()` fails, Sign out shows the
  sign-in card as in a browser; the list is asked for once the bridge's
  `setToken(null)` has answered.
- **Step 8.** The repetition's red line shows from the first key that
  differs, as D98-A has it; its words go as the two match, but its place
  stays, as tall as its text, as the rule's line keeps its own: the whole
  1100-en suite found `test_the_shown_line_stays_while_one_types` moving
  13 px, since a suggestion fills both fields and emptying the first made
  the line come, the centred card moving. The submit still says
  `setup.mismatch` on the form's line, as before. One eye for the pair is
  a `shown` state the caller may hold for `PasswordInput` (`NewPassword`'s),
  with `eye={false}` on the second field; `PasswordField` gains `problem`
  and `room`.
- **Step 11, D107-A.** Nothing to trim: the card's password way and the
  first owner's form trim the name already, the key way takes none, and
  `POST /api/v1/login`, OPAQUE's start and a registration take it through
  `store.NormalName`. `TestSignInTrimsTheName` and `test_opaque.py`'s case
  show each path with a trailing space; the Go test fails with the trim
  taken out of `NormalName`.
- **The fragments.** Block 15's bullets that Block 20 changes are on
  `maint/0.17` and ship in 0.17, so Block 20's changes have bullets of
  their own (`CHANGELOG.d/Changed/claude-mobile-menu-cards*.md`, five).
  D117-A changes Block 21's revoke codes, on `main` alone and unreleased:
  it amends that bullet (`claude-mobile-sessions-held-codes.md`), as
  COMMIT.md 5.2 asks, instead of a fragment of this branch's name.
- **D117-A.** A removed server's codes are dropped by its id, not by its
  node (`GoneStore.dropAll`), so that a Remove of the same node between
  the read and the drop keeps its own. A code in a `gone.json` written
  before this change has no user nor id (debug builds alone: no release
  carries Block 21): it cannot become an entry, stays in `gone.json`, and
  is posted by the node rule. `Bridge.save` is the only path that saves a
  server, so it alone takes codes back; its answer is read again after,
  so that the page sees them `pending`.

## The review of the branch

A code review before the push found no functional defect; its points are
folded into the two commits:

- Texts the new order and the dropped "…" left behind: the Cube choice's
  entry and `toServerSettings`'s row in `docs/ui.md`, `shared/app.ts`,
  `NewPassword`'s comment, and the comments of `Choice.tsx`, `connect.js`,
  `src/plain/login.d.ts` and `EmulatorTest.kt`; `tools/ui-test/README.md`,
  `test_password_eye.py`'s and `test_connect.py`'s descriptions; and
  `tools/app-emulator/README.md`'s table, which now lists Block 19's,
  Block 21's and Block 20's cases.
- The pin a removed server's code is posted with is checked where it is
  posted: `Codes` takes `revoke`, `ServerApi.revokeCode` by default, a
  seam the JVM test records the pin through, since the fake is plain HTTP
  and `ServerApi` holds it to no pin there. The test fails with the
  removed server's pin.
- A take-back whose write fails leaves the codes in `gone.json`
  (`aTakeBackWhoseWriteFailsLeavesTheCodesInGone`, `servers.json.new` a
  directory); it fails where `gone.json` drops them first.
- `Bridge.save`'s `adopt` is checked in the emulator:
  `CodesTest.aServerAddedAgainWithItsNodeTakesItsCodesBack`.

## Findings

- The ⓘ's facts at 390 px: the fingerprint wraps over about eight short
  lines in its column, its copy icon beside the middle, as before Block
  20 without the icon; it reads, but the column is narrow.
- The connect page's sprite still has no `mark-warn` nor `mark-x`
  (Block 19's finding), untouched here.

## Tests

On the committed tree, unless said otherwise; the box ran two other
sessions' UI suites and an emulator beside these, its load near 37:

- `make ui`, `go test ./...` (40 packages), `gofmt -l .` empty, `go vet`
  for linux, windows and darwin, `make cross-check`, `make ui-check`: all
  green; `go test -race -count=1 ./internal/server`: green (850 s).
- The changed UI files in the four cases: 631 passed, one setup error,
  `test_profile_offers_a_first_password[390-de]`, a 5 s read of the test's
  own `clear_preferences` timing out, which passed at its retry. After the
  repetition's fix, the files of the password pair in the four cases:
  684 passed.
- The whole UI suite, 1100-en: first 902 passed and
  `test_the_shown_line_stays_while_one_types` failed twice (the 13 px
  above, fixed); on the final tree 903 passed.
- The JVM tests: 177, none failing (173 before, four new for D117-A);
  `lintFossDebug` clean; `assembleFossDebugAndroidTest` built.
- The emulator suite: the first run stopped before any test, the
  emulator's first setup not ended within 420 s; the second, with
  `APP_EMU_BOOT_TIMEOUT=900`, had 28 of 29 green, the new
  `TapTest.signOutOnAServersPageShowsTheList` among them, and
  `ConnectPageTest.theConnectPageMayNotAskForAToken` without an answer to
  its script while the app skipped frames at the WebView's first start,
  as Block 19's run met it; its class run again: 2 of 2.
- `GUIDELINES/shared/tools/check`: all well.

After the review's fixes, folded in by fixup commits and an autosquash:
`make ui-check` and `go test ./internal/server/...` green; the touched UI
files (`test_password_eye.py`, `test_connect.py`) in the four cases: 184
passed; the JVM tests 178, none failing, lint clean, the androidTest
build made; the emulator's `CodesTest` 7 of 7 (its new case among them)
and `TapTest` 6 of 6; `GUIDELINES/shared/tools/check`: all well.

## For the owner at the completion gate

Two points the review raised, left as built for the owner to decide:

1. A mismatch at the submit is said twice: on the repetition's line
   (D98-A) and on the form's message line, as before Block 20.
2. D104-A's toast takes every failure My profile's line says: a client
   check that sends nothing (a key without a label) and a failed read of
   My profile as well as a refused request.

## Left for the owner's phone check

Block 20's Verification, each step after what it needs:

1. A's ☰: the version row, Personal settings, a rule, App settings, This
   server's settings, Select server, a rule, Sign out; no "…"; a tap on
   the version row or its icon copies the version and closes ☰; the
   list's version row the same; at 390 px on Android 13 or later the word
   "Copied" at the screen's foot beside the system's clipboard preview,
   both readable;
2. A's sign-in card: the server's line above the title, the address and
   ⓘ on one line; ⓘ's two facts copied by a tap and by their icons, the
   bubble above each;
3. C's first owner's card: the command on a line of its own, copied by a
   tap and by its icon;
4. C's password pair: one eye showing and hiding both; the repetition red
   while it differs, the line under it; the owner made;
5. A's card, the password manager's autofill filling the name with its
   space: the sign-in passes;
6. Sign out in A's page: the app's list;
7. A frozen, a Revoke under My profile: after the limit the failure in
   the toast, in red, and on the line once the toast has gone;
8. a browser at A: ☰ with one rule where the app's entries are not;
9. D117-A: A stopped, Remove A with its login kept, A started, A added
   again: its settings list the key and the session left, waiting, and
   A's audit ends them at the first contact.
