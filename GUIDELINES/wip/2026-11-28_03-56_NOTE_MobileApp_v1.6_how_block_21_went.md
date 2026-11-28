# NOTE MobileApp v1.6: how Block 21 went

The owner's `PLAN++` of 2026-11-28 on v1.6 built Block 21 on
`claude/mobile-sessions-held`, from `main` at `chore(aps): +MobileApp v1.6`,
in five commits, the server's two first (D113-A), the fifth from the
re-review of #496:

1. `fix(server): A cancelled request is not a password check that waited`
2. `feat(api): POST /api/v1/tokens/revoke ends a token by its revoke code`
3. `fix(app): A session the app held is ended after its process, and no logout
   the server refused`
4. `feat(app): A token the app forgets is ended by its code at the next contact`
5. `fix(app): A session sealed before a restart keeps the key it was made from`

The coordinating session pushed them, opened #496 and merged it on
2026-11-28 once its CI was green. The Android emulator job's first run
failed in the emulator's first setup, before any test ran (SystemUI's
view of the SIM, past `APP_EMU_BOOT_TIMEOUT`); its rerun passed.

## Where it went differently from the plan

- **The migration is 0100** (`0100_token_revoke.sql` and its down part):
  `origin/main`'s last is `0026_backup_pending.sql`, and none of AP KeyLife's
  #481 and #482 nor AP ServerBackup's #492 had merged when the branch was
  finished. Whichever of them merges first, Block 21's renumbers at its merge
  of `main`, as `loadMigrations` refuses a gap.
- **Two store tests took the schema's version for the number of migrations**:
  `TestOpenMigratesAnEmptyFile` counted `schema_version` rows against
  `SchemaVersion()`, and `TestBackupPendingMigration` migrated to the newest
  file and expected 26. With the first number of `main`'s block both broke;
  the first counts the migrations now, the second migrates up to 26. KeyLife's
  and ServerBackup's 0100 meet the same.
- **D106-A**: `userFail` answers a context that ended `503`, its own words
  and no `Retry-After`; the store's `ErrBusy` keeps `passwordBusy`'s answer
  there, since only a password check or hash raises it. `passwordFail` is the
  helper of the password routes: a user added with a password, a password
  set, the current password checked, the OPAQUE registration.
  Its route test drives `deleteMyToken` through the wrapper's own steps
  (`beginAudit`, the handler, the error's answer, `endAudit`) with the
  request's context ended, since no real request can be cancelled between
  the authentication and the store's call: every step before reads the
  store with the same context.
- **The route's refused requests**: a locked-out address (`429`) is counted
  too, as a code that matches nothing and a malformed body are, not a row of
  its own; the plan named the other three. Their rows per address and minute
  are kept apart from the address's other refused requests
  (`codeRefusals`), so that neither hides in the other's row, where the plan
  had them counted "as refused requests are".
- **`RevokeByCode` reads first**: a plain read looks the code up, and only a
  match begins the write transaction, read again inside it, so that a code
  anyone may post takes no write.
- **`RevokeByCode`** shares `revokeToken`, `RevokeToken`'s statements, in one
  transaction, and revokes a token past its end too (the KeyLife session's
  note to the coordinator: "live" is "not revoked"). The revoke hash is
  written by `addToken` (through `addTokenHash`'s new `revokeHash`
  argument), `AddKeySession` and `TouchToken`; KeyLife's `AddKey`,
  `AddKeySessionAs` and `RekeyToken` adapt whichever lands second.
- **The servers' store moved into `MassimoApp`** beside the keeper and the
  checks, so that one store for the process writes the `held` records; two
  stores, the old activity's and the new one's, could have written over each
  other. Version 1's legacy switch is read there too.
- **The code is written at an unseal by `AppKey.Step.open`**, which holds the
  store, not by `LoginStore.open`, a function of the companion without one.
- **An entry left is equal to another by its token alone** (`KeyLeft` by
  user and id, `SessionLeft` and `HeldSession` by user, id and time), its
  code, whether it was taken and when it was left aside, so that the notes
  that end or match an entry find it whatever its code says.
- **Remove looks for the server's address once**, before its turn-off, so
  that the codes go to the same address.
- **What waits is the call's own**: the answers' `pending` of `signOut`,
  `forget` and `forgetAll` say whether the session or key that call left
  carries its code (`Sessions.signOutWith`, `Revoker.Off.pending`,
  `Summary.pending`), not whether the server has any code waiting; Remove's
  only where `gone.json` was written.
- **A session is left before it is ended**: Sign out and Sign out of all
  note the session in `unended` first and take it away once the server ended
  it, as a session replaced was already, so that an app that ends meanwhile
  leaves it recorded; a sealed session no check of this process has seen is
  left by its `held` record. The record is written and cleared under the
  lock of what was checked, and leaving a session clears its own record
  alone, so that a check of the next one keeps its.
- **The record keeps the key a session was made from** (the re-review):
  after a restart a sealed session made from the kept key, at a server
  asked after a logout, is not logged out where the server ended it itself,
  replaced by the key's next session or revoked with the key by Turn off;
  D100-A holds after a restart too.
- **A probe still running keeps the last state**: the grey badge and the
  session's line of a server down follow what the last probe found while the
  next one runs, so that neither flickers and a plain-HTTP row is not drawn
  again; the first probe shows no server down.
- **Exit is Block 19's**: `MassimoApp.exit` does what Exit needs (the memory
  dropped, the records moved into `unended`), with no button yet;
  `docs/app.md` says Block 19's Exit is to use it.
- **The emulator suite cannot end the app's process**: the instrumentation
  runs in it, so the plan's `am kill` after Home would end the test. Its
  stand-in is `MassimoApp.exit()` and a relaunch, the same path as a new
  process's start (the memory gone, the records left), the relaunch opening
  the first server added at start, which posts its codes. A process the
  system truly ends is the owner's phone check, step 1.
  Nothing stops the host's `serve` either: a server that does not answer is
  one whose address the app holds closed
  (`https://127.0.0.1:9/`), given back for the server's start; Remove's case
  writes the real address into `gone.json` for the server found again.
- **No refused logout** is read from every audit row, those refused before
  anyone was known among them, which are written once their minute is over:
  `HeldTest` waits 65 s for it.
- **The night mode** is `cmd uimode night yes`, the activity made anew, and
  the page signed in after it; the language is the phone check's alone.

## Findings

- The Kotlin `Sessions` took its logger as a trailing lambda in one test;
  with the new `checks` last it is passed by name there.
- In `turnOffWhileTheServerIsDownLeavesCodesTheNextContactTakes` the key's code
  goes first and ends the key's session with it, so the session's own code
  then matches nothing and writes no row: the case checks the session ended,
  not a row of its own.

- The first whole emulator run had 19 of 22 green; the three failures were the
  block's own: `ConnectPageTest` still wanted version 5; `HeldTest`'s second
  login by the key awaited `open`, whose page takes the list's place; and the
  relaunch waited for the list, where the first server added opens at start
  (D43-A), which posts its codes as well. Fixed, the three classes ran green
  (10 of 10); `HeldTest`'s fix is folded into the third commit.

## Tests

After the review's fixes, on the final tree:

- `make ui`, `go test ./...` (39 packages), `go test -race -count=1` of
  `internal/server` and `internal/store`, `make cross-check`, `gofmt -l .`,
  `go vet` for linux, windows and darwin: all green.
- The JVM tests: 171 after the fifth commit, none failing (144 before the
  block); `lintFossDebug` clean.
- `test_connect.py` in the four cases: 112 passed;
  `test_server_page_revoked_at_once` seen failing with the toast's call undone.
- The whole UI suite once, 1100-en, on the committed tree before the review:
  880 passed.
- The emulator suite, whole: 22 tests, none failing. Before the review its
  first run had 19 of 22 green, the three failures the block's own tests',
  fixed and folded in.
- `GUIDELINES/shared/tools/check`: all well.

## Left for the owner's phone check

The plan's Verification, each step after what it needs, the server's audit
read after each:

1. at A, asked at each start: a password sign-in; the app swiped from the
   recent apps and started again: the first session ended within seconds, by
   its code (F47);
2. at A, asked at each start, signed in: the phone's dark mode turned on,
   then its language changed: still signed in each time, no prompt;
3. at A, asked at each return: a fingerprint tap; within seconds A's `serve`
   stopped, then Turn off; A started, ↻: the key and the session ended, no
   sign-in needed (F43);
4. at B: two fingerprint logins one after the other, then Turn off while
   signed in: no refused `logout` in B's audit (F41);
5. A frozen while a session is held: the list's "signed in as …" grey; the
   settings' session line saying Sign out ends it the next time the app
   reaches A; Sign out; A thawed, ↻: the session ended, its line gone;
6. A frozen, a Revoke under My profile given up after the limit; A thawed:
   its audit says the client went away, not "too many password checks" (F46);
7. A stopped, Remove A with its login kept; A started, the app started again:
   the key and the session ended, A no longer in the list.

Steps 1, 3, 5 and 7 are what the emulator suite could only stand in for: a
process truly ended by the system, and a `serve` truly stopped and frozen.
