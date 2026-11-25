# NOTE AccessEnds v1.0: how block 1 went

Where Block 1, Late, went differently from the plan,
the choices taken in the run, what the readings changed, and what verified it.

## What it made

Every way of the sign-in card now checks its opening after its answer,
as the key's way has since LooseEnds' Block 6 (LooseEnds.D44-A, D1-A, D2-A).

- **A late answer changes nothing:**
  an answer of the password's way, the key's,
  the fingerprint's in the app or a provider's start that comes once its opening
  has ended (another way signed in, the card closed or opened anew,
  a sign-out) writes no line, takes no focus and sets no token;
  so does one that comes once another way of the same opening has set its token.
- **A late password's session is signed out at the server**
  (`endSession` of `login.js`, `POST /api/v1/logout` with it),
  and the page keeps the token it holds.
- **A late provider's start leaves the page where it is.**
- **The login by fingerprint keeps the card working** from its press until the
  card has closed, as the first owner's creation does: the other ways' Sign in,
  both Cancels, **Select server…** and the providers' buttons are off,
  and it does not start while the password's or the key's sign-in of the opening
  runs.
  One that still answers late is signed out at the server,
  and the app keeps none of it where the page holds no token.
- **Sign in and a provider's button turn for their own opening:**
  the password's and the key's Sign in and the providers' buttons are ready
  again in a card opened anew (`useBusy`'s epoch, `src/shared/question.tsx`).
- **The UI tests' fake bridge** ends at the server the session its `setToken`
  and its `login.fingerprint()` replace, as the app does.
- **The key's two tests** get what LooseEnds' Block 6 NOTE left to this
  file's next change.
- **One commit:**
  "fix(ui): A sign-in's answer after another way signed in changes nothing".

## Every way and its late answer

| Way | Checked after its answer | A late answer | Held by |
|-----|--------------------------|---------------|---------|
| the password's, refused | `still()` before the line and the focus | says nothing, takes no focus | `test_a_late_password_refusal_writes_nothing` |
| the password's, a session | `still()`, then `take()`, before `setToken` | its session signed out at the server, no token | `test_a_late_password_answer_makes_no_token`, `test_a_password_answer_after_a_sign_out_does_nothing` |
| the key's | `still()` on both branches, `take()` before `setToken` | no token, no line, its field as it is | `test_a_late_key_answer_makes_no_token`, `test_a_late_key_refusal_writes_nothing`, `test_an_answer_after_another_way_set_its_token_makes_none` |
| the fingerprint's, refused | `still()` before the line; the app's status read again in any case | says nothing | `test_a_late_fingerprint_makes_no_token` |
| the fingerprint's, a session | `still()`, then `take()`, before `setToken` | signed out at the server, the bridge left with none where the page holds none | `test_a_late_fingerprint_makes_no_token`, `test_the_fingerprint_keeps_the_card_working` |
| a provider's start | `still()` before `location.assign` and before the refusal; it takes no opening | no navigation, says nothing | `test_a_late_provider_start_stays_on_the_page`, `test_a_provider_page_that_keeps_the_page_leaves_the_card_open` |

Left as they were, each closed already:
the first owner's form (D1-A); keep it open, which checks that its ask is open
and sets no token; `finishSignIn`, which checks `ask.closed` after each await;
the password's keep step, whose choice is dropped when the opening ends;
and the provider's return, read in `init` before any card exists.

A fingerprint can still answer late only by a script:
after a sign-out behind the open card,
its session is ended and neither the page nor the bridge holds one;
after a script's close or new opening of the card, its session is ended,
and where the page still holds a session,
the app has ended that one already as it kept the fingerprint's,
and keeps the fingerprint's, ended.
No user path reaches either.

## Different from the plan

Against the plan's block, item by item:

1. **`store.ts`:**
   `opening()` and `still()` as the plan has them, and `take()` beside them:
   the opening taken by the way that sets its token,
   after which `still()` is false for every other way of it (What the reading
   found).
   `submitKey` takes it, its behaviour the same.
2. **`submitPassword`** as the plan has it,
   and it does nothing while the card is working on the fingerprint.
3. **The fingerprint: D2-A carried out otherwise.**
   The plan's hand-back, `setToken(state.token)`,
   would have signed the user out in the real app.
   The app keeps the fingerprint's session as `setToken` would (`Bridge.kt`'s
   `openKept`, `Sessions.put`), which ends the session it replaces,
   the password's that the page had just set;
   the hand-back then made the app end the fingerprint's too, both dead,
   and the next request opened the card again.
   With no token, the hand-back `setToken(null)` forgot the fingerprint's
   session on the phone without ending it.
   The fake bridge ended nothing, so the plan's test passed.
   The coordinating session's settlement, after it read the app's code:
   the window is closed as D1-A closes the first owner's form,
   the card working from the fingerprint's press until it closes;
   an answer whose opening ended meanwhile is signed out at the server
   (`endSession(token, true)`), and there is no hand-back.
   The plan's test, the password signing in while the finger is held,
   describes a state the card no longer reaches:
   `test_a_late_fingerprint_makes_no_token` now holds a refusal in a later
   opening and a session after a sign-out,
   and `test_the_fingerprint_keeps_the_card_working` (new) the working card.
4. **The providers' buttons** as the plan has them,
   `startProvider` answering the address for the card to open;
   they are off while the card is working,
   and turn for their own opening.
5. **`login.js`:**
   `endSession(token, kept)` does nothing for no token,
   which would otherwise send the page's own,
   and with `kept` leaves the bridge with none where the page holds no token;
   `setToken` uses it for its own logout.
   `login.d.ts` declares it and `startProvider`'s new type.
6. **The first owner's form:**
   unchanged (D1-A); `docs/ui.md` and `Card.tsx`'s head say why.
7. **`Card.tsx`'s head** says it of every way,
   with the fingerprint's working card and the one path only a script reaches.
8. **Beyond the plan's files:**
   `src/shared/question.tsx` (`useBusy` and `BusyButton` take an epoch,
   0 unless given, so that their other callers are unchanged;
   the card passes its `opened`), `tools/ui-test/fake_bridge.js`,
   `docs/app.md` (what the fake bridge does on a server's page),
   and `tools/ui-test/README.md`
   (the clauses of `test_login.py` and `test_sign_in_updates.py`).
   `src/plain/app.d.ts` took `state.token` in the first build,
   for the hand-back, and gave it up again.
9. **Tests beyond the plan's:**
   `test_a_password_answer_after_a_sign_out_does_nothing`,
   `test_an_answer_after_another_way_set_its_token_makes_none`,
   `test_the_fingerprint_keeps_the_card_working`
   and `test_a_provider_page_that_keeps_the_page_leaves_the_card_open`;
   the provider's test also holds a start in a card opened anew and after a
   sign-out;
   the late password's test also holds that a request with its session's token
   answers 401,
   and the late refusal's test that Sign in is ready in the card opened anew.
10. **The CHANGELOG fragment** in the plan's words.
11. **The verification:**
    the plan's eight UI files and the three others that use the fake bridge;
    `make app-emulator-test` not run, KVM not being open to the user;
    the whole UI suite on the first rework's tree, not on the last;
    `GUIDELINES/shared/tools/check` by the coordinating session on the commit,
    the exported copy having no git of its own.

## Choices the plan left open

- **`take()` right before every `setToken`:**
  the other way, `docs/ui.md` naming the window of one read of who is in,
  in which a second way of the same opening set its token over the first's.
- **The fingerprint does not start while the password's or the key's
  request of the opening runs:**
  the working card alone left a password still turning when the finger was
  tapped.
  The other way: the working card alone.
- **The fingerprint's busy stays the card's,**
  one system prompt at a time.
  The other way: per opening, as the other buttons.
- **Busy for the opening in the shared part,**
  an epoch of `useBusy` that the card raises with each opening.
  The other way: left as it was, older than the block,
  an old opening's buttons turning and off until its answer came.
- **`endSession` in `login.js`, with `kept` for the bridge:**
  the other way, the first build's hand-back from the part,
  which put `state.token` into the parts' types.
- **`setToken` ends what it replaces through `endSession`:**
  the other way, two copies of the same request.
- **A provider's start takes no opening:**
  the other way, `taken` released where the page stays or comes back.
- **The navigation stopped by `route.abort("aborted")`** in the test of a page
  left standing, as Stop or Esc stops it,
  not by the provider's page answering 204:
  with the 204, Playwright waited for that navigation,
  and the first fill timed out without reaching the code.
- **No navigation, by the Navigation API's `navigate` event,**
  which Chromium fires at once at `location.assign`,
  also to another origin:
  the other way, the first build's wait of one second for none.
- **The late password's logout** awaited with `expect_response` around the
  let-go, its timeout turned into a named assertion: the other way,
  the first build's 25 polls of 200 ms.
- **The key's late refusal's focus on the key's way button,**
  the key's field shown beside it, so that a late focus request would move it:
  the other way, the focus in the key's field itself,
  where a late request would change nothing to see.

## What the reading found

One builder built the block in an exported copy of main with the plan.
Two readers read the first tree:
one the sign-in's code and its security,
with the fake bridge against `docs/app.md` and the Android bridge;
the other the tests, the fake bridge and the documents.

Confirmed: the password's both answers, the key's,
the provider's and the fingerprint's refusal are checked as the plan asks;
the first owner's form, keep it open and `finishSignIn` hold;
`endSession` ends only its bearer's session and leaks no token;
no reachable path drops an answer of an opening still open;
the four new tests failed on the base for the reasons they name,
and each reader's own faults planted in `Card.tsx` were caught.

One must, both readers':
**in the real app a late fingerprint signed the user out** (Different from the
plan, 3), settled by the coordinating session.
The second reader rated the null hand-back a must of its own,
the first reader a should.

The shoulds, done in the first rework:

- **One way's token and another way of the same opening:**
  between a way's `setToken` and its `finishSignIn` closing the opening,
  a second way could set its token over the first,
  whose session a browser did not end; `take()` now.
- **`endSession` with an empty token** fell back to the page's own token
  and would have signed out the session in use; it returns now.
- **Two polls became signals:**
  the late password's logout, and the provider's wait for no navigation.
- **What no test held:**
  the session's half of `still()` for the password,
  the fingerprint and a provider;
  the app's status read again after a late refusal of the fingerprint;
  a late provider's start in a card opened anew;
  the fingerprint answering after the session was forgotten; a test each.
- **Words:**
  a docstring named a review's tree git does not hold,
  and names the reason in the code's terms now;
  the message said the README said why the first owner's form needs no check,
  which it did not;
  `docs/ui.md`'s fingerprint lines, the message and the fragment,
  each true of the reworked tree.
- **Older than the block:**
  the busy buttons lived per card (Choices, the epoch).
- **`docs/app.md`'s list** of what the fake bridge does now says it ends what it
  replaces, and `state.token` is out of the parts' types again.

The first reader read the rework and found it holds:
no way can act after a `take()` or after its opening ended;
the working card cannot be left set by any user path;
a late fingerprint in the real app can no longer end the page's session;
the fake bridge matches `Sessions.put`;
and `useBusy`'s epoch keeps the old meaning for its other callers.
One must, done in a second rework:
**a provider's `take()` killed its opening where the page stayed**
(Stop or Esc while the provider is slow, a 204, a download):
every way of it then did nothing until a reload,
the password's session ended unseen and the card unable to be left,
shown in a scratch test of the reader's.
A provider's start takes no opening now, and a test holds it.
One should, done: two sentences said more than holds for the path only a script
reaches; a clause each now.

The coordinating session read the second rework's diff of `Card.tsx`
and found nothing to change.
The second reader did not read the reworks, and there was no third reading.

**Planted faults:**
the builder's seven on the first tree, each of the key's success and focus,
the password's logout and focus, the fingerprint's hand-back and refusal,
and the provider's refusal taken out; the second reader's four;
the builder's 13 in five rounds after the first rework;
and the first rework's `take()` put back into the provider's button after the
second.
Each failed its test at the assertion it names.

## Seen and left

- **No emulator test presses the card's fingerprint**
  (For the completion gate, 3).
  The app's tests' next change, the owner's to ask for.
- **The one path only a script reaches:**
  a script closing the card on a page that holds a session while the fingerprint
  runs; the bridge then keeps the fingerprint's session, ended,
  and the page's next request opens the card.
  `Card.tsx`'s head and `docs/ui.md` say so; as it is.
- **A fingerprint's prompt that never settles keeps the card working;**
  the app's prompt settles at a dismissal and when its activity has gone
  (`AppKey`'s `dropAll`).
  The app's, as it is.
- **A late provider's start leaves a sign-in pending at the server,**
  which expires.
  The server's, as it is.
- **`test_oidc.py` has no entry in `tools/ui-test/README.md`,**
  as 39 test files have none, older than the block.
  The README's, nobody's yet.
- **`tools/ui-test/README.md` has four line-break warnings more,**
  0 errors; `docs/ui.md` and `docs/app.md` as before.
  As it is.
- **One UI test failed once under load** in the first rework's run,
  `test_sign_in_resend.py`'s `test_stream_ticket_waits_for_the_sign_in` in
  1100-de, the stream not open while other sessions' suites ran;
  it passed on its retry and alone in all four cases.
  Not this block's file; nobody's yet.
- **No test holds the providers' buttons off while the fingerprint runs:**
  `test_the_fingerprint_keeps_the_card_working` checks both Sign in buttons,
  both Cancels and Select server; the code turns the providers' buttons off
  too (`off={card.working}`).
  The next change of that test's.

## For the completion gate

1. **What a user notices:**
   an answer of any way of the sign-in card that comes once another way has
   signed in, the card was closed or opened anew, or the page signed out,
   writes nothing and sets no token;
   a late password's session is signed out at the server;
   a late provider's start no longer leaves the page for the provider's;
   Sign in and a provider's button are ready again in a card opened anew.
   In the app, while the fingerprint's prompt is open, the other ways' Sign in,
   Cancel, **Select server…** and the providers' buttons are off,
   and the fingerprint is off while the password or the key signs in.
2. **D2-A for the fingerprint is carried out otherwise,**
   by the coordinating session's settlement, the owner's to change
   (Different from the plan, 3):
   the window closed as D1-A closes the first owner's form,
   a late fingerprint's session signed out at the server, no hand-back.
   What D2-A asks still holds: nothing live is left that nobody holds.
   The other ways, as the first reader named them:
   the page takes a late fingerprint's session where its own token was a
   session and reads who is in again;
   or the app ends the session it replaces only once the page hands it on,
   a change of the app's contract (`docs/app.md`).
3. **CI's Android emulator job presses no fingerprint in the card:**
   it signs in by password with the login kept,
   and `TapTest`'s `aTapAsksForTheFingerprintAndItsCancelOpensTheCard` checks
   only that the card shows the fingerprint's button.
   The working card and a late fingerprint are held by the UI tests and the
   fake bridge alone,
   the bridge's ending of a replaced session by the JVM test
   `aTokenPutEndsTheSessionItReplaces`.
   `make app-emulator-test` did not run here, and nobody tried it on a phone.
4. **A sign-in way's answer after a sign-out or another way's token now does
   nothing in every way of the card:**
   before the block the key's way alone checked its opening,
   and a second way of the same opening could set its token over the first
   in the window of one read of who is in, older than the block.
5. **A provider's start takes no opening:**
   a navigation that leaves the page standing,
   stopped or answered without a new page,
   leaves the card to the other ways.
6. **A shared part changed:**
   `useBusy` and `BusyButton` take an epoch,
   and their other callers keep their old meaning.
7. **The fake bridge now ends what it replaces,**
   as `docs/app.md` says the app does;
   no assertion of the other tests that use it changed.

## What verified it

- **The builder, on the first tree,** each by its exit code, 0:
  `make ui`, `make ui-check`, `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin;
  `go test -count=1` of `internal/server/...`, `tools/identifiers/...`
  and `cmd/massimo/...`;
  `reflow.sh --check` on `docs/ui.md` and `tools/ui-test/README.md`,
  0 errors; `commit-msg.sh`.
  The plan's eight UI files and `test_connect.py`, `test_tls.py` and
  `test_personal_updates.py` in all four cases, 750 passed;
  the whole UI suite in 1100-en, 818 passed.
  The four new tests on the base, 16 of 16 failing in the four cases,
  each for the reason it names; seven planted faults, each caught.
  `make cross` and `go run ./tools/apitypes -check` not run:
  no Go code and no API type changed.
  The grep for scripts that sign in found the app's Kotlin code
  and `EmulatorTest.kt`, which signs in by password;
  no rule for signing in changed.
- **The first reader, on the first tree:**
  `make ui`, `make ui-check` and
  `go test -run 'TestUI|TestState|TestShared|TestOpenAPI' ./internal/server/`,
  each ok;
  `Bridge.kt` and `Sessions.kt` read against `docs/app.md`.
  It ran no UI test.
- **The second reader, on the first tree:**
  the base page with the tree's three test files and fake bridge in the four
  cases, 16 failed and 8 passed, each new test at its first named assertion,
  the key's two passing with LooseEnds' Block 6 there; four planted faults,
  each caught; `test_login.py`, `test_sign_in_updates.py`, `test_oidc.py`,
  `test_connect.py`,
  `test_password_eye.py` and `test_parts_guard.py` in all four cases,
  350 passed, no retry;
  the Navigation API's `navigate` at `location.assign` checked in the suite's
  Chromium; `commit-msg.sh` 0, `reflow.sh --check` 0 errors.
- **The builder, after the first rework:**
  the base page with the new and changed tests in the four cases, 28 failed,
  all seven tests each for its named reason, the key's two passing;
  13 planted faults in five rounds, each caught; `make ui`, `make ui-check`,
  `gofmt -l .` printing nothing,
  `go test -count=1 ./internal/server/...` and `commit-msg.sh`, each exit 0;
  `reflow.sh --check` on `docs/ui.md`,
  `docs/app.md` and `tools/ui-test/README.md`, 0 errors;
  the eleven UI files in all four cases,
  761 passed and 1 failed (Seen and left),
  which passed on its retry and alone in all four cases;
  the whole UI suite in 1100-en, 821 passed.
- **The first reader, after the first rework:**
  `make ui` and `make ui-check`, exit 0; its scratch test of a provider's page
  answering 204, then the password.
- **The builder, after the second rework:**
  the new provider's test failing in 1100-en with the first rework's `take()`
  put back; `make ui`, `make ui-check`,
  `reflow.sh --check docs/ui.md` (0 errors) and `commit-msg.sh`, each exit 0;
  `test_oidc.py`, `test_login.py`,
  `test_sign_in_updates.py` and `test_parts_guard.py` in all four cases,
  238 passed, no retry.
- **The coordinating session:**
  the app's code read before the settlement;
  the second rework's diff of `Card.tsx`, read;
  the step carried from the plan's commit, "chore(aps): +AccessEnds v1.0",
  onto main's head, "chore(aps): -ToldData, -ANALYSIS ToldData",
  by a three-way merge of trees without conflict;
  in a clone with the commit:
  `commit-msg.sh`, 0 errors and 0 warnings;
  `changelog.sh check --staged --strict`, 0;
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well
  (line breaks 0 errors).
- **Not run here:**
  `make app-emulator-test` (no KVM);
  the whole UI suite on the last tree;
  the whole `go test ./...`, which is the pull request's CI.
