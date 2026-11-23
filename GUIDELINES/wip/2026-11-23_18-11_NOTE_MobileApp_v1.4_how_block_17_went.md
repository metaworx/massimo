# NOTE MobileApp v1.4: how block 17 went

Block 17, one prompt and the lockout (D63-A, D64-A),
run on 2026-11-23 after v1.4 and its gate decisions were committed onto `main`,
as one pull request, metaworx/massimo#405 (`claude/mobile-one-prompt`),
of three commits:
"fix(server): A token that has ended is refused without counting toward the
lockout" (step 1),
"fix(security:server): A guessed Bearer beside any token counts toward the
lockout" (a hole the review found, below),
and "fix(app): Forget all asks for the fingerprint once, one key for the app
sealing every login" (steps 2 to 6);
the documents and the fragments (steps 7 and 8) went with each.
It was built by a sub-agent from a brief and read by two second readers,
one for the server's half and one for the app's,
each reading the rework again before its commit.

## How it ran

- **Checked here**, each exit 0:
  `make ui`;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`;
  `go test -race -count=1 -timeout 30m` for both;
  `gofmt -l .` and `go vet` for linux, windows and darwin; `make cross`;
  `go test ./tools/identifiers/...`;
  the Gradle tasks `:app:assembleFossDebug :app:testFossDebugUnitTest
  :app:lintFossDebug :app:lintNightlyDebug` (122 unit tests);
  `GUIDELINES/shared/tools/check`; `commit-msg.sh` on each commit.
  Every new test was seen to fail with its fix undone,
  the app's by mutations of the code.
- **Left to CI**: the whole Go and UI suites and the race job.
- **CI** passed every job of the pull request's run at the first try;
  merged as "merge: MobileApp, Block 17: Forget all asks once,
  and an ended token does not lock the address out (#405)".

## Departures from the steps

- **The app key's steps are a class of their own**, `AppKey`,
  so that step 6's JVM test over a fake key source can run them.
  `LoginStore` keeps the sealed app key under the reserved name "app"
  and marks each login sealed under it (`byApp`);
  the fake key source moved to a shared `JvmKeys.kt`, with a `FakePrompt`.
- **A run's end** (step 3): `Revoker.ByKey.close()` ends the run's step,
  also where the run throws, the error then thrown on;
  and `MainActivity.onDestroy` calls `bridge.close()`, besides `onIo`.
- **A key the phone takes back** (step 5):
  in a keep, the logins under it are forgotten first;
  when opening, the step answers Gone for every later login under it
  and forgets them at its end, each with its key left by its id (D57-A).
  Only the Keystore's own word counts as taken back:
  a bad tag or a permanently invalidated key (`KeyInvalidated`),
  in the exception or its causes;
  any other failure is Failed and forgets nothing.
  The first build took any security exception for it,
  which would have forgotten every login at a passing error;
  the app's reader found it.
  A login kept before is held to the same narrowing.
- **A dismissed prompt holds** for the rest of Forget all (`refused`):
  the first one dismissed, no later login under the app key asks again,
  so that "once" holds.
- **Forget all's prompt has a title of its own**, `login_forget_all_title`,
  in the six languages, FR, IT, NL and PL machine-translated.
- **`LoginKey.alias` takes a server's id alone** (24 lowercase hex digits),
  so that a page's `forget("app")` reaches no Keystore key.
- **Two keeps at once make one app key**:
  the second is answered Dismissed while the first makes it.
- **The server** (step 1): a removed user's token also counts as ended;
  a store that cannot tell counts the token as a guess
  (`401`, counted, never `500`).
  `docs/store.md`, the OpenAPI description of `POST /api/v1/login/key`
  and `docs/ui.md` stated the old rule and say the new one,
  beside `docs/api-v1.md`;
  `TestLoginKey`'s last part now checks that a revoked key is not counted.
- **The CHANGELOG** (step 8): a new Fixed fragment for the server,
  the lockout's rule having been released;
  a Security fragment for the hole;
  and the app's change amends Block 14's bullet under Added, not yet released
  (`CHANGELOG.d/Added/claude-mobile-check-app-menu.md`, COMMIT.md 5.2).

## The hole beside the lockout

The server's reader found it in the code as it was, before this block.
A wrong Bearer header beside a disabled user's,
an expired guest's or a disabled role's `?token=`
was answered `403` and never counted.
Beside a live `?token=` on the event stream it was answered `200`,
with a `Deprecation` header on a wrong guess alone,
which told a guess from the key,
and the success cleared the address's count.
A guessed Bearer header is now refused `401`
and counted whatever is beside it, the query not read;
a wrong ticket beside a forbidden Bearer token counts before its `403`,
and the forbidden token alone is still no failed check.
A side effect: a guessed Bearer header also hides
a valid stream ticket sent with it.
A browser's EventSource sends no Bearer header, so the web UI is unaffected.

## Not done here

- **The owner's phone check** of Block 17, the plan's three steps
  (Forget all's one prompt over three servers,
  a login kept after the update and one kept before,
  a session logged out sent again on the rig past the limit:
  `401` each time, no `429`),
  gathered with Blocks 16, 15 and 18's at the end of the run.
  The real Keystore and prompt are Block 18's.
- **Left as they are**:
  a used stream ticket that an EventSource sends again
  still counts toward the lockout (outside D64-A);
  `making`, a keep making the app key, is cleared only by its prompt's answer,
  a gap that is theoretical.
