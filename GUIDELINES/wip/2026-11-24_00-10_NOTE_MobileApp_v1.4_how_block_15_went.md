# NOTE MobileApp v1.4: how block 15 went

Block 15, the list and the card (F31, F33, F34, F40; D62-A, D72-B, D73-A,
D74-A, and D84-A with the switch of the gate's NOTE),
run from 2026-11-23 into 2026-11-24 as one pull request,
metaworx/massimo#409 (`claude/mobile-list-card`),
of five commits, a merge of `main` and two fixes CI's runs called for:
"fix(security:app): A tap or Back into a server asked at each return asks for
the fingerprint first" (steps 1 to 3),
"feat(ui): In the app the sign-in card names the server it signs in to,
with an ⓘ for its id and fingerprint" (step 4), "feat(app):
A server's ☰ in the app has This server's settings… and App settings…,
on the app's own page" (step 5),
"fix(app): Show in My profile off while the server does not answer,
Remove server on a line of its own" (steps 6 and 7),
"feat(ui): Every password field has an eye showing the password,
and auth.password_reveal turns it off" (step 8),
and "merge: main, with RefreshBatteries, into Block 15's list and card",
which brought RefreshBatteries' Blocks 1 to 5 without a conflict.
The texts, the contract's version 5, the tests, the documents and the fragments
(steps 9 to 13) went with each.
It was built by a sub-agent from a brief and read by two second readers,
one for the page and the server and one for the app,
each reading the rework twice;
what they found is below.
The session "UI refactor" was told before the block began.

## How it ran

- **Checked here**, each exit 0:
  `make ui`; `make ui-check`; `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `go test -race -count=1 -timeout 30m ./internal/server/`;
  `gofmt -l .` and `go vet` for linux, windows and darwin; `make cross`;
  `go test ./tools/identifiers/...`;
  the Gradle tasks (144 unit tests);
  the touched and related UI test files in the four cases,
  at 1100 and 390 px (1304 passed, and 648 after the last fixes);
  `GUIDELINES/shared/tools/check`;
  `commit-msg.sh` and the changelog check on each commit.
  Every new test was seen to fail with its fix undone.
- **On the merged tree**, after the merge of `main`:
  `make ui`, `make ui-check`, `go test ./...` and `gofmt` passed.
- **The whole UI suite and the race job's other packages** were left to CI.
- **CI's first two runs** failed one test,
  `test_password_eye.py::test_my_profiles_password_eyes`,
  in the four cases and then in two, each passing on its retry:
  the page hid a shown password at the fold's `toggle` event,
  which Chrome fires in a task after the click that opens the fold,
  and a close and an opening close together come as one event;
  under the runners' load a read, or a paint, came first,
  so a shown password could be seen in clear for a frame.
  "fix(ui): A password shown is hidden at any toggle of its fold,
  also a close and an opening taken as one" covered the merged event,
  and "fix(ui): A password shown is hidden in the same task its fold opens,
  before any paint" the rest:
  a `MutationObserver` on the `open` of every `<details>` around the field
  hides it in the task that opens the fold, never while it is closed.
  A check that reads the field in that task failed in the four cases
  before it, and the eye's, Users' and the personal page's tests passed
  five times with four workers after it.
  The third run passed every job;
  merged as "merge: MobileApp, Block 15: A tap asks the fingerprint,
  the card names its server, the password's eye (#409)".
  The first builder's context ran out before the second fix,
  which a second one wrote from its findings.

## What the readers found

Each was fixed in the commit of its step.

- **Back from the list reloaded a server's page with its session and no prompt**,
  blocking and a security hole:
  the WebView's history held the list, the server and the list again.
  The list taking a page's place now withholds the opening of the server it left
  (`Opened.leaving`),
  and the list once loaded clears the WebView's history,
  so that Back from it leaves the app.
  Hence the first commit's `fix(security:app)`;
  its CHANGELOG amends the login's Added bullet (COMMIT.md 5.2),
  `claude-mobile-login.md`,
  and adds no Security fragment, the feature not yet being released.
- **A withheld page's `setToken(null)`**
  now leaves the held session alone (`Opened.takes`).
- **A server's page navigating to the app's own page** now loads it without
  its fragment (`Origins.byPage`),
  and anything but `GET` on the app's host answers `405` (`Origins.appServes`),
  a form posted there escaping `shouldOverrideUrlLoading`,
  so that what the list opens at its start is the bridge's handover alone.
- **A shown password stayed shown after its fold closed.**
  The first fix hid it as the fold closed,
  and Chrome then drew nothing of the input when the `<details>` opened again,
  its type having turned while it closed.
  The final rule: the type never turns inside a closed `<details>`,
  and a hide waits for the fold's next opening.
  The test's reopening of the fold was restored,
  and was seen failing in the four cases before the final rule.
- **The browser's own eye** is hidden on the whole page,
  the API key's field and a provider's client secret too,
  as the switch's texts say.
- **The test of the switch turned off** holds `/healthz`,
  so that it proves no eye shows before its answer.
- **Switching the eye off and on again** no longer brings a shown password back.
- **No Warn** where a request's context ended before the setting's read (`PasswordReveal`,
  and `PasswordLogin` the same).
- **Found by the builder**:
  `autocorrect="off"` rendered as "on" in Chrome through Preact's property path;
  it is now set as an attribute, and a test asserts it.

## Departures from the steps

- **A tap over plain HTTP outside the home network** opens the page at its
  card (steps 1 and 2).
- **Any failed tap where a login is kept** reads the list again,
  not a refused key's alone (step 3);
  and the list makes one opening at a time.
- **The tap's prompt** is titled with the existing `login_title`
  rather than a new one (steps 2 and 9).
- **The withheld opening survives the activity being made anew**
  (`KEY_WITHHELD`),
  and the cover leaves a page at its card alone (step 2).
- **`{login: true}`** stays unchanged, though the list no longer uses it.
- **App settings' fragment** is the existing `#app-settings`,
  not `#app` (step 5).
- **Show in My profile** is turned on and off in place as the probe's state changes,
  not drawn again (step 6).
- **Remove server's rule** is a CSS border,
  and its line is one of two texts, with a kept login and without (step 7).
- **The card's password row** is a `.login-field` with a `<label for>` (step 8).
- **The eye is on password fields alone** (step 8, D84-A):
  an API key's field and a provider's client secret have none of ours.
- **D84-A's switch** is the setting `auth.password_reveal`, on by default,
  carried by `/healthz` as `password_reveal`,
  always present and true without a store,
  since no public route carried settings before sign-in;
  the page shows no eye until `/healthz` has answered.
  Its switch, "Show password button", is in System settings → Users,
  for holders of `settings`.
- **The CHANGELOG** (step 13): three new Added fragments,
  `claude-mobile-list-card-server.md`, `claude-mobile-list-card-menu.md` and
  `claude-mobile-list-card-eye.md`;
  and the app's bullets not yet released amended (COMMIT.md 5.2):
  `claude-mobile-login.md` and `claude-mobile-check-fixes-app-keys-left.md`.

## Not done here

- **The owner's phone checks** of Block 15,
  gathered with Blocks 17, 16 and 18's at the end of the run:
  the tap's prompt, "Log in to" with the server's name,
  and its cancel into the card;
  the sign-in icon;
  a server asked at each return opened twice,
  and Back from the list:
  the app closes, and a server's page reached without a tap shows its card;
  the card's line and its ⓘ against `massimo tls show`;
  ☰'s two entries;
  Show in My profile off with the server down;
  Remove server's line;
  D83-A's look (D60-A) and the autofill;
  the eye: the focus and the keyboard at its tap,
  the keyboard not learning a shown password, the autofill after showing,
  a fold hiding it at its reopening, and the switch off.
- **The WebView's history and Back, and `ConnectPageTest.kt`**,
  are Block 18's: they need a device.
