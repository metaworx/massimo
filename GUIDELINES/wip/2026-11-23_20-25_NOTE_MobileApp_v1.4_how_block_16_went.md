# NOTE MobileApp v1.4: how block 16 went

Block 16, sessions and keys (F32, F35 to F39; D65-A to D70-A, D80-B, D82-A),
run on 2026-11-23 as one pull request,
metaworx/massimo#408 (`claude/mobile-sessions-keys`),
of six commits and a merge of `main`:
"test(ui-test): A read refused for an earlier sign-in, the card closed,
opens a second card" (step 1),
"fix(ui): A 401 of an earlier sign-in opens no second card, and a session a
sign-in replaces is ended" (steps 2 and 3, the page's half),
"fix(ui): A request the server does not answer fails after its time limit,
so that no button turns for good" (step 8),
"feat(api): DELETE /api/v1/me/token revokes the token that calls,
as the app's kept key now revokes itself" (steps 9 and 10),
"fix(app): Remove signs out first, and a session forgotten without its server
is ended at the next sign-in" (steps 3 to 5, the app's half),
"feat(ui): My profile shows every key's id and in the app this device's key,
read after the sign-in's revoke" (steps 6 and 7),
and "merge: main, with three plans' files, into Block 16's sessions and keys",
which brought plan files alone.
The tests, the documents and the fragments (steps 11 to 13) went with each.
It was built by a sub-agent from a brief and read by two second readers,
one for the page and one for the server and the app,
each reading the rework twice;
what they found is marked below.
The session "UI refactor" was told before the block began.

## How it ran

- **Step 1 failed on `main`, as it had to.**
  `test_late_refusal_of_an_earlier_sign_in` failed in the four cases,
  each with "no second card",
  which confirms F36's cause,
  likely and not proven when D68-A was taken.
  One adjustment to the step:
  the 401 is held back until `login/finish` has answered and the card closed,
  since the bug shows only once the card's promise is gone.
- **Checked here**, each exit 0:
  `make ui`; `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `go test -race -count=1 -timeout 30m ./internal/server/`;
  `gofmt -l .` and `go vet` for linux, windows and darwin; `make cross`;
  `make ui-check`; `go test ./tools/identifiers/...`;
  the Gradle tasks `:app:assembleFossDebug :app:testFossDebugUnitTest
  :app:lintFossDebug :app:lintNightlyDebug` (135 unit tests);
  the touched UI test files in the four cases, at 1100 and 390 px (244 passed);
  `GUIDELINES/shared/tools/check`; `commit-msg.sh` on each commit.
  Every new test was seen to fail with its fix undone;
  a four-thread test of the session swap, on the old code,
  failed in three runs of three.
- **The whole UI suite and the race job's other packages** were left to CI.
- **CI's first run** failed one UI test in the four cases,
  `test_personal_updates.py::test_a_read_that_finds_a_change_waiting_keeps_it`:
  its `pause()` gave Playwright's clock milliseconds where it takes seconds,
  so the clock jumped far ahead and fired every pending timer,
  since this block the held read's own time limit among them.
  "test(ui-test): The personal page's tests pause Playwright's clock at the
  page's now, in seconds" fixed the test; the second run passed every job.
  Three other test files make the same call and pass; a task of its own.
  Merged as "merge: MobileApp, Block 16: No second sign-in card,
  sessions ended, every key's id (#408)".

## Departures from the steps

- **A late 401 is told by the token the request carried** (step 2),
  not by the session's number of AP UiParts' Block 15,
  since a sign-in with nobody signed in before raises none.
- **A replaced session's end** (step 3):
  in a browser only where `/me` was read with it (`meToken`),
  its kind being `session`:
  a key would answer `400`, and a token not read yet is left alone.
  In the app the bridge ends it after its answer,
  so that a stalled server cannot hold the card (a reader's).
  A dead session's 401 no longer wipes a token another tab stored since,
  as `dropToken` did (a reader's).
- **The bridge's `setToken` checks every new session** before it answers,
  one more `GET /api/v1/me` per sign-in:
  it learns the session's id for D70-A and serves D67-A's `{revoked}`;
  the app's sessions keep the token's kind.
- **Sessions left** (step 5):
  a replaced session is noted in `unended` before the bridge answers
  and removed once ended,
  so that an app killed meanwhile loses none (a reader's).
  A session left is revoked only where the listed one has its id
  and the same non-empty `created_at` (`GET /api/v1/me`'s `token.created_at`),
  since a store rolled back may give the id again (a reader's);
  one without a time is dropped unasked.
- **`KeysLeft` revokes keys and sessions from one list**
  and returns the ids revoked.
- **The time limits** (step 8) are set by path in one place, `waitOf`,
  each above the server's own.
  The first ones were below it,
  Cube writes having 5 minutes at the server and a repair 10 (a reader's).
  As merged: 20 s by default;
  a write to a Cube and adding a Cube 330 s; a repair 630 s;
  the search for Cubes 90 s;
  the look for an update 75 s, its 30 s under the server's lock coming
  after another look's;
  the end of a provider's sign-in and the integrations found 45 s;
  the stream's ticket the default.
  `apiRaw` (a backup's download, a restore) has none,
  which `docs/ui.md` says.
- **`DELETE /api/v1/me/token`** (step 10) is refused
  for the settings file's `MASSIMO_API_TOKEN`, `409`:
  it could revoke itself, and the store's bootstrap could not bring it back;
  and on a user's behalf (`X-MassimoHA-On-Behalf-Of`), `400`:
  it would revoke the integration's shared key,
  and the audit would name the acting user.
  Both are a reader's.
  A key's own revoke no longer needs its id,
  so a login kept before the contract's version 3 revokes itself
  at a server that has the route.
  `src/api.d.ts` needed no change, `make api-types` writing it as it was.
- **"This device's key"** shows in whichever user's panel lists the key,
  not in My profile alone (step 6).
- **The contract's version 5** is raised here (step 12);
  Block 15 adds to it.
- **The READMEs** beyond the steps:
  `tools/ui-test/README.md`, with an entry for `test_sign_in_resend.py`,
  which had none, and `app/README.md`.
- **The CHANGELOG** (step 13):
  new Fixed fragments for the late 401 and the time limit,
  the defects being in the released web UI;
  an Added one for the route;
  and the app's bullets not yet released amended (COMMIT.md 5.2):
  `claude-mobile-check-app-sessions.md`,
  `claude-mobile-check-fixes-app-keys-left.md`,
  `claude-mobile-check-this-device.md`.

## Not done here

- **The owner's phone checks** of Block 16,
  gathered with Blocks 17, 15 and 18's at the end of the run:
  F36's path again;
  a login turned on at a password sign-in,
  the tag, the ids and the key's id in the server's settings;
  Show in My profile after a sign-in that revoked that key;
  Remove without a kept login;
  Turn off or Sign out with the server down, then a sign-in;
  a Revoke while the link hangs;
  a key revoking itself with the new route;
  a sign-in replacing a held session.
- **`ConnectPageTest.kt`**, the instrumented test,
  is updated to version 5 but not run: it needs a device (Block 18).
- **Left as they are**:
  the old route and the UI can still revoke the settings file's key,
  after which the store's bootstrap logs an error at every start,
  older than this block and a follow-up;
  `Bridge.keep` still reads the session on the main thread,
  the page awaiting `setToken` avoiding the race;
  `POST /api/v1/update` behind a running look could pass 75 s,
  only in a rare double wait.
