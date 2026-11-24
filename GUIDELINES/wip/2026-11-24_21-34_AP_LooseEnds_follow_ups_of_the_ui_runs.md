# AP LooseEnds v1.1: Follow-ups of the UI runs

## TL;DR

What AP UiParts and AP FirstOwner left,
and what a walk on a real open server found
(`wip/2026-11-24_14-31_ANALYSIS_LooseEnds_first_owner_walk.md`), in five blocks,
a pull request each.

- open → no regular user; an integration's users count no more (D1, D2):
  an approval leaves the server open, the owner can still be made
- card → refusal in the page's language, no stale line in the owner's form,
  Escape out of a card the user opened (D3), one sign-in per Enter by key,
  secrets emptied after a success (D13)
- page → no poll behind the start's card;
  Integrations: newest read wins, nothing before the LAN's answer
- clock → `pause_at` in seconds, through one helper
- server tests → four races fixed at the source
  (`wip/2026-11-20_20-32_ANALYSIS_LooseEnds_armv6_two_server_tests.md`)

A Cube's rooms after a change of Cube: fixed on main by AP RoomMajority.

### Blocks

1. Open: one rule in the store for "a regular user"; the trap's regression test
2. Card: the sign-in card's six faults and D13's emptying
3. Page: the poll before the start's card; the Integrations card's two reads
4. Clock: `pause_at` in seconds through a helper, and a guard
5. ServerTests: four tests of `internal/server` fixed at the source

## Discussion

### What is decided

The owner, 2026-11-24, on the walk's findings:

- **D1-A.
  A server is open while it has no regular user:**
  every user that is not an integration's (`users.integration_id IS NULL`),
  a disabled one included, as today.
  An integration's users no longer count:
  they do not close the server, do not block the first owner,
  and do not remove `auth.keep_open`.
  It revises FirstOwner.D17.
  Not chosen: users with a way in (a server whose only owner's key runs out
  would open by itself;
  the owner made from `MASSIMO_API_TOKEN` has a key and no password);
  refusing an integration's approval while there is no owner (no Home Assistant
  on a server kept open).
- **D2-A.
  A server that today holds integration users only becomes open at the
  upgrade,** said in the changelog and as a warning in the log at the start.
  Not chosen: a flag in the store that keeps it closed.
- **D3-A.
  Escape in a sign-in card the user opened from the running page leaves it,** as
  "Back to the page" does; not in a card the server asks for.
- **D13-A.
  The cards' secret fields (password, again,
  API key) are emptied once the sign-in or the creation has succeeded and the
  card has closed,** in both cards; a failure keeps them for the retry.
  It changes FirstOwner.D43 and UiParts.D210,
  which emptied them at the opening alone for the password managers' sake,
  so it needs the owner's check by hand that Chrome,
  Firefox and the Android app's autofill still offer to save:
  no test can show that.
  Not chosen: at send (a failed attempt means typing again); as now.

### Decided in the writing

Each the owner's to change at the completion gate:

- **The focus after Skip** stays as FirstOwner.D24-A chose it.
- **Escape**
  (D3) closes the card's ☰ first where it is open, as everywhere,
  and leaves the card from either step, the form and the confirmation;
  not while a request of the card runs (`card.working`).
- **The German `setup.text`**, second sentence:
  "Lege den Eigentümer hier aus dem lokalen Netz an oder auf der Box mit massimo
  user add."
- **The switch in Users** gets a label that holds in both positions,
  "Keep this server open without an owner", in six languages;
  its hint under it still says what turning it off did.
- **The clock's helper** lives in `helpers.py`, where the tests' kit is,
  not in `conftest.py`: the tests import from `helpers`.

### Taken with PLAN++

These were open in the draft; the run takes them as below,
each the owner's to change, and the completion gate lists them.

- **D40-A.
  An integration's key on an open server.**
  `authorize` answers every request of an open server as the open principal
  before it looks at a key (`auth.go`, `authorize`),
  so once D1 opens a server with an integration,
  Home Assistant's calls are anyone's from the local network:
  not attributed, its `X-MassimoHA-On-Behalf-Of` header not read,
  its users not made, `max_caps` not applied,
  `/api/v1/me` with its key answering `"mode": "open"` and no `integration`;
  an installation beyond the local network is refused (`403`),
  and the restart and the reboot are the box's alone, as on any open server.
  - **A (recommended).**
    So, and said in the documents:
    open means every caller from the local network is anyone,
    and the integration's chain begins with the first owner.
    One rule for an open server; nothing new to test or explain.
  - **B.**
    A request whose Bearer is a live key of an integration is resolved as in
    users mode, its chain and audit as there; every other request open.
    Keeps attribution and a remote Home Assistant,
    but an open server would then have two kinds of caller.
    Not settled by reading: what Home Assistant's integration,
    outside this repository, does when `/me` with its key says `open`;
    the owner's to check with it,
    which the completion gate names as the owner's check with Home Assistant.
- **D41-A.
  Where D13's check by hand finds a password manager no longer offering to
  save.**
  - **A (recommended).**
    The emptying waits for the page's first read after the sign-in
    (`afterLogin`'s `refresh`), a second or so later,
    and the owner checks again.
  - **B.**
    D13 is taken back in the same pull request: emptied at the opening alone,
    as now.
- **D42-A.
  "No users yet" on an open server that has an integration.**
  The card's title (`setup.title`) and Users' line (`users.noOwner`) say so
  while the Integrations card lists the integration with its two users.
  - **A (recommended).**
    As it is: the users the page means are those who sign in,
    and Users lists none of an integration's while the server is open.
  - **B.**
    "No owner yet" in both, in six languages.

### What stays as it is

- The API's open mode: every request from the local network may do everything.
- The first owner's three ways (the banner, ☰, Users) and Skip for now.
- A password's length (walk finding 6): AP PasswordRules'.
- The updater, a second factor (AP SecondFactor) and any retirement:
  not this plan's.

## Analysis

Read on main at `RoomMajority, Blocks 1 to 4 …` (#410) and the four plan commits
after it.

### For Block 1: who counts as a user

- **The one count today** is
  `SELECT count(*) FROM users WHERE deleted_at IS NULL`, written four times:
  `CountUsers` (`internal/store/users.go`), `noUsers` (same file),
  and inside the transactions of `SetKeepOpen` and `DropKeepOpen`
  (`internal/store/settings.go`).
- **Its callers:**
  `authMode` (`internal/server/auth.go`), from which every mode follows:
  `authorize`, `stillAdmitted`, `loginMode`, `meOf`'s `setup` and `kept_open`,
  `AuthMode` for the start's `auth=` (`internal/serve/serve.go`),
  `authenticated` (the binary's replacement from the API),
  the settings' `in_effect` and `409` of `auth.keep_open` (`settings_api.go`),
  the first owner of `addUser` (`users_api.go`) and `mayRegister`
  (`opaque_api.go`); `Bootstrap` (`internal/store/bootstrap.go`),
  at the start and in `SetToken`;
  `noUsers` in `AddFirstOwner` before the hash and in `addUser`'s transaction
  (D15 of FirstOwner).
- **`dropKeepOpen` is called** where a transaction makes a user:
  `addUser` (`users.go`), `AddIdentityUser` (`identities.go`),
  `ApproveIntegration` and `IntegrationUser` (`integrations.go`).
  `DropKeepOpen` runs at the start (`server.go`, after `Bootstrap`)
  and at each reload (`cmd/massimo/reload.go`).
- **Every integration's user carries `integration_id`:**
  `addUserTx` refuses one without (`integrations.go`),
  `integrationUserTx` sets it,
  and migration 0024 set it for the users made before.
  A user of the role `integration` without it is a regular user, by D1's rule.
- **What an approval on an open server does today:**
  `ApproveIntegration` makes `<uuid>:` and `<uuid>:system` and removes
  `auth.keep_open`; the next read finds two users and the mode turns to users:
  the page's next request is `401` and the card turns to a sign-in nobody can
  pass (walk finding 1).
  `MASSIMO_API_TOKEN` at a start does not rescue it:
  `Bootstrap` counts two users, finds no enabled owner for the token,
  and only warns ("MASSIMO_API_TOKEN is set but no enabled owner can hold it").
- **After D1, on a server with integration users only:**
  `authMode` is open; `/api/v1/me` answers `"mode": "open"`,
  `setup` true for a caller on the local network, `kept_open` as the setting is;
  `POST /api/v1/users` makes the first owner (`noUsers` passes);
  `MASSIMO_API_TOKEN` at a start makes the owner `owner` with the token as its key,
  since `Bootstrap` counts no user; an integration's requests are D40's.
- **The page needs no change:**
  `firstOwnerOf` and `keptOpen` (`src/shared/me.ts`,
  `src/shared/owner.ts`) read `/me`'s `mode`, `setup` and `kept_open`,
  which follow the server; Users draws its open section (`OpenServer`,
  `src/admin/Users.tsx`) while `mode` is `open`,
  so it means "no regular user" by itself;
  the Integrations card lists the integration,
  and its panel the integration's users (`GET /api/v1/integrations/{uuid}`);
  the users an integration's user may be linked to are `GET /api/v1/users`
  without an integration's, none (`loadLinkUsers`).
- **Tests that encode today's rule:**
  `internal/store/keepopen_test.go` (`TestKeepOpenGoesWithTheFirstUser`,
  its case "integration" and its integration's user made after every other user
  is gone) and every Go test that approves an integration on a store without a
  regular user and then expects users mode;
  `onbehalf_test.go`'s `TestOnBehalfChain` makes its owner before its first
  call; the others are to be read.
- **Documents that say it:**
  `docs/api-v1.md` ("Authentication"; the setting `auth.keep_open`,
  "the first user removes it, … an integration approved"),
  `docs/store.md` ("Bootstrap and compatibility":
  the keep-open paragraph names an integration's user),
  `README.md` ("while there are no users", in "Server"),
  `docs/ui.md` (the sign-in, Users).

### For Block 2: the sign-in card

Read as AP MobileApp's Blocks 15 and 16 left it.

- **The refusal:**
  `submitPassword` (`src/signin/Card.tsx`) puts any error of `signIn` into
  `login.failed`, "Not signed in: {error}";
  the server's `401` is "wrong user or password" (`login_api.go`,
  `opaque_api.go`), in English in every language (walk finding 2).
  `apiRequest` gives the error its `status`;
  a `429`, `409` or `503` has a text that says more than a key would.
- **The owner's form** (`src/signin/Owner.tsx`, `submit`):
  the name's check returns before `card.ownerSaid` is cleared,
  so a line of an earlier attempt stays beside the name's rule;
  a password's problem is said with `say` alone,
  and the focus stays on Create (walk finding 3).
- **Escape:**
  the dialog is shown not modal (`askLogin`, `login.js`),
  so the browser sends no `cancel` and Escape does nothing (walk finding 5).
  `card.byUser` says the user opened the card (`open`, `src/signin/store.ts`),
  `skip` leaves it and gives the focus back to the control that asked.
  `components.js` closes every open menu on Escape from a listener on `document`,
  registered as the page loads, and does not mark the event.
- **The two texts:**
  `setup.text` in `de.json`; `users.keptOpen`, the switch's label,
  drawn with the switch off too (`OpenServer`, `Users.tsx`; walk finding 7).
- **The key's form:**
  `submitKey` (`Card.tsx`) has no guard and no busy state,
  calls `setToken` without waiting for it and `finishSignIn` again at a second
  Enter; the password's way has `useBusy` (`pw`).
- **The emptying:**
  `Card.tsx` and `Owner.tsx` empty their fields in a layout effect at each
  opening (`card.opened`);
  the dialog's `done` (`askLogin`) calls `MassimoParts.signIn.closed()`,
  which knows nothing of how it ended.
  The header comments of `Card.tsx`, `Owner.tsx`,
  `src/signin/store.ts` and `src/shared/password.tsx` and `docs/ui.md` (the
  sign-in; "How the page is built") say "emptied at each opening alone".

### For Block 3: the page

- **The poll behind the start's card** (walk finding 4):
  `init` (`app.js`) calls `applySettings`, which calls `startPolling`,
  before `loadMeAndPreferences` waits for the card;
  so every `state.settings.poll` seconds (10 by default) `poll` reads the Cubes
  and the log's alerts behind it.
  On an open server they pass;
  once another browser has made the owner they are refused,
  and the refusal is what turned the second browser's card.
  The same poll runs behind the sign-in card at the start of a server with users.
  `docs/ui.md` says the start reads only the version and the update's state
  before the card closes.
  `applySettings` is also called by `applyStoredSettings` and by Personal settings
  (`src/prefs/Settings.tsx`) to apply a new interval.
- **The Integrations card** (`src/integrations/`;
  UiParts' Block 13 NOTE after its CI,
  `wip/2026-11-21_08-41_NOTE_UiParts_v1.0_block_13_after_its_ci.md`,
  and Block 15's):
  - `IntegrationsCard` draws `NotLooking` where `data.loaded && !data.looks`;
    `refresh` sets `loaded` in `loadIntegrations` and reads the LAN's answer
    last (`loadSeen`),
    so until it comes the card says the general "does not look",
    without its reason.
  - `loadIntegrations`, `loadSeen` and `loadLinkUsers` write what they read
    where the session is still the same (`stillSession`) and nothing else is asked:
    where two showings of System settings overlap and the older read answers last,
    the older answer stays.
    The log has a generation of its own for this (`logState.gen`,
    `src/log/store.ts`); `src/shared/` has no such helper.

### For Block 4: Playwright's clock

`page.clock.pause_at(page.evaluate("Date.now()") + 1000)` hands milliseconds to
an API that takes seconds,
so the clock jumps tens of thousands of years ahead and fires every timer of the
page's once:
`test_admin_updates.py`
(`test_a_new_items_form_keeps_the_focus_where_it_is_typed_in`
and `test_an_identitys_change`),
`test_first_owner.py` (`test_the_name_is_checked_as_it_is_typed`),
`test_restart.py` (`test_restart_lists_what_waits`).
`test_personal_updates.py`'s `pause` has the right form, 50 ms on, and says why.
Each of the four pauses so that a frame of the page's comes only when the test
runs it, or a debounce only when the test moves the clock:
neither needs the jump,
and each still shows what it means with the clock 50 ms on.
`tools/` is not a shipped path.

### For Block 5: four server tests

- **`TestConfigWriteConfirmed`** (`confirm_test.go`):
  the fake Cube applies 150 ms after the command (`ApplyAfter`,
  `internal/cubelan/fakecube/apply.go`), the re-reads come at 50,
  100 and 200 ms (`shortFollow`); where every re-read wins,
  the write stays open for `confirmWithin`, a minute,
  and the test's 20 s wait under QEMU runs out (the armv6 analysis).
- **`TestProxyRetryStops`** (`proxy_retry_test.go`): it closes a port it took,
  sleeps `5 * quickRetry[1]`, and asserts that nothing at all accepts there:
  another parallel test or package can take the freed port;
  `ports_test.go` names the hazard.
- **`TestLogOfEveryCubeWithoutStore`** (`log_api_test.go`):
  its failure on 2026-11-18 was "the lines by Cube: map[:2 KEQ0000100:7]",
  the second Cube's line missing.
  That Cube's address refuses (`secondCube`),
  and its only line is the `disconnected` its runtime logs once its dial has
  failed; the test reads every Cube's log once, without waiting for it.
- **`TestLogAfterReadsABurstOnce/store`** (`logafter_test.go`):
  four writers of 3000 lines each and a reader paging two routes,
  all within one `testkit.Eventually` of `4 * testkit.Wait` (20 s,
  40 s under `make race`): a fixed amount of work against a fixed time,
  which a loaded machine misses (FirstOwner's Block 1 NOTE;
  RoomMajority's Block 4 NOTE, unscaled).
- **A CHANGELOG bullet for test files under `internal/`:**
  COMMIT.md §5.1 asks one of every commit touching `project.shipped`,
  which takes `internal/**` with its `_test.go` and exempts none.
  The project's commits that change tests alone, such as "test(server):
  A test reads the log forward from a mark",
  write none and say so in `Key changes` ("No CHANGELOG bullet:
  only a test changes, and nothing shipped behaves differently"):
  §5.2's one reason, the bullets standing being accurate;
  `changelog.sh check` passes, `[Unreleased]` not being empty.
  Block 5 does the same, and changes no file the binary is built from:
  the fake Cube is `massimo fakecube`, so it is not touched.

## Implementation Plan

Each block moves its documents with its code,
ends with the gate of `GUIDELINES/shared/GATE_WORKFLOW.md`,
and is a pull request of its own; none waits for another.

### Block 1: Open

- **Goal:**
  an integration's users neither close an open server,
  nor block its first owner, nor remove `auth.keep_open` (D1),
  and a server that holds only theirs says so at its start (D2).
- **Changes:**
  1. `internal/store/users.go`:
     the rule once, a constant `regularUser`
     (`deleted_at IS NULL AND integration_id IS NULL`)
     and `countUsers(ctx, q queryer)`;
     `CountUsers` counts by it ("the regular users, disabled ones included"),
     `noUsers` too, with its message;
     and `CountIntegrationUsers` for the warning.
  2. `internal/store/settings.go`:
     `SetKeepOpen` and `DropKeepOpen` count by `countUsers` in their
     transactions; `KeepOpenKey`'s,
     `ErrHasUsers`' and `dropKeepOpen`'s comments say a regular user.
  3. `internal/store/integrations.go`:
     `ApproveIntegration` and `IntegrationUser` no longer call `dropKeepOpen`;
     `addUserTx`'s comment says so.
     `addUser` and `AddIdentityUser` keep theirs.
  4. `internal/store/bootstrap.go`:
     the comments of `Bootstrap` and `BootstrapResult.Users` say regular users;
     its code follows `CountUsers`.
  5. `internal/server/server.go`, after `Bootstrap` and `DropKeepOpen`:
     with no regular user and integration users, a Warn line,
     "the server is open: its only users are an integration's;
     create the owner", with `integration_users`;
     `authMode`'s comment (`auth.go`) names the rule.
- **Tests,** each new one seen failing on main first:
  - `internal/store/keepopen_test.go`:
    the case "integration" of `TestKeepOpenGoesWithTheFirstUser` becomes a test
    that an approval and an integration's user made at its first call keep the
    setting and leave `CountUsers` 0,
    and that `SetKeepOpen` takes on beside them;
    `TestDropKeepOpen` with integration users only keeps it;
    `AddFirstOwner` passes beside them, and a second is refused as before.
  - `internal/store/users_test.go`:
    `CountUsers` counts a disabled regular user and no integration's;
    `bootstrap_test.go`:
    `MASSIMO_API_TOKEN` on a store with integration users only makes the owner.
  - `internal/server/keepopen_test.go`,
    the trap as a regression test at the API: an open server kept open;
    `POST /api/v1/integrations/requests`, then its approval without a key;
    then `/api/v1/me` still `open` with `setup` and `kept_open`,
    a read without a token still `200`,
    `POST /api/v1/users` makes the first owner (`201`),
    and the server is then in users mode with the setting gone.
  - The start's warning, by the server's log (`testkit.LogWatch`),
    and none with a regular user.
  - Every test that approves an integration without a regular user and expects
    users mode gets its owner first;
    found by the suite and by a look at the callers of `registerHA`
    and `ApproveIntegration`.
- **Documents:**
  `docs/api-v1.md` ("Authentication": open while no regular user,
  an integration's do not count, and D40's answer; the setting `auth.keep_open`:
  "an integration approved" goes);
  `docs/api-v1-openapi.yaml` where `/me`'s `setup` or the setting describe it
  (its "No users yet" of the `409`s holds);
  `docs/store.md` ("Bootstrap and compatibility", the keep-open paragraph,
  the integrations' paragraph); `README.md` ("Server",
  "while there are no users"); `docs/ui.md` where the sign-in and Users say it.
- **CHANGELOG:**
  a fragment `CHANGELOG.d/Changed/claude-looseends-open.md`:
  "- A server whose only users are an integration's is open until its first
  owner, as one without users: the log warns at the start."
  (two lines); and `CHANGELOG.d/Added/claude-first-owner-keep-open.md` amended,
  "goes with the first user" becoming "goes with the first user who is not an
  integration's".
- **Commit:**
  `fix(server): An integration's users leave an open server open`
- **Verification,** each by its exit code: `make ui`,
  `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `go test -race -count=1` of `internal/store` and `internal/server` with
  `MASSIMO_TEST_SCALE=2`, `gofmt -l .` empty, `go vet ./...` for linux,
  windows and darwin, `go run ./tools/apitypes -check`,
  `GUIDELINES/shared/tools/check`.
  No UI test: no page file changes.
  Not run here: `make cross`, which CI runs.

### Block 2: Card

- **Goal:**
  the sign-in card says a wrong password in the page's language,
  the owner's form says only what is wrong now and points at it,
  Escape leaves a card the user opened (D3), a key signs in once per Enter,
  and the secrets go once the card has closed on a success (D13).
- **Changes:**
  1. **The refusal:**
     `submitPassword` (`Card.tsx`) words a `401` with a new key `login.wrong`,
     "Wrong name or password.", every other error as now;
     the key in six `i18n/*.json`.
  2. **The owner's form** (`Owner.tsx`, `submit`):
     `card.ownerSaid` cleared at the submit's start;
     a mismatch gives the focus to Again, a missing password to Password,
     a missing name to Name, in the submit's handler;
     `newPasswordProblem` (`shared/password.tsx`) unchanged.
  3. **Escape**
     (D3): `components.js`' Escape handler marks the event (`preventDefault`)
     where it closes a menu;
     the card listens for Escape on the document while
     `card.owner && card.byUser`,
     and where the event is not marked and `card.working` is off it calls
     `skip`, which gives the focus back in that handler.
  4. **The texts:**
     `setup.text` in `de.json` as decided;
     `users.keptOpen` "Keep this server open without an owner" in six languages.
  5. **The key's form:**
     `submitKey` runs in a `useBusy` of its own, as the password's way does:
     a second Enter meanwhile does nothing, Sign in turns,
     and it waits for `setToken` before `finishSignIn`.
  6. **D13:**
     the dialog's `done(ok)` (`askLogin`,
     `login.js`) hands `ok` to `MassimoParts.signIn.closed`;
     on a success `src/signin/store.ts` counts it (`card.succeeded`),
     and the layout effects of `Card.tsx` and `Owner.tsx` empty the password,
     again and key fields and hide their eyes in the drawing after the close;
     a failure, Skip and Back change nothing, and the opening empties as before.
     The header comments of `shared/password.tsx`, `Card.tsx`, `Owner.tsx`
     and `src/signin/store.ts` say it.
- **Tests,** in `tools/ui-test/`, each seen failing on main first:
  - `test_login.py`:
    a wrong password says `login.wrong` in the case's language,
    not the server's text;
    two Enters in the key's form send one sign-in's reads (`/api/v1/me` once
    after the key); after a sign-in the closed card's password field is empty,
    after a refused one it is not.
  - `test_first_owner.py`:
    a mismatch, then a bad name, shows the name's rule alone;
    the focus on Again after a mismatch and on Password without one;
    after the owner is made the closed card's two password fields are empty.
  - `test_open_banner.py` or `test_first_owner.py`:
    Escape in the card opened from the banner,
    ☰ and Users shows the page and gives the focus back to what opened it;
    Escape in the start's card does nothing;
    with the card's ☰ open the first Escape closes ☰ alone;
    Escape while Create turns does nothing.
  - `test_keep_open.py` and `test_users.py`: the switch's label, on and off.
  - The guard's views of the card (`test_parts_guard.py`) as they are.
- **Documents:**
  `docs/ui.md` (the sign-in: the refusal, Escape,
  the fields emptied after a success; "How the page is built":
  the card's fields; Users: the switch).
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-looseends-sign-in.md`:
  "- The sign-in card says a wrong password in the page's language,
  and signs in once by a key however often Enter is pressed."
  (two lines); `CHANGELOG.d/Changed/claude-looseends-secrets.md`:
  "- The sign-in card empties its password and key fields once it has closed on
  a sign-in or a new owner."
  (two lines).
  Escape and the owner's form amend nothing:
  the first owner's card is not released yet,
  and `claude-first-owner-card.md` stays accurate (§5.2).
- **Commits:**
  `fix(ui): The sign-in card's refusals, Escape and the key's second Enter`,
  `feat(ui): The sign-in card empties its secrets after a success`.
- **Verification,** each by its exit code: `make ui-check`; `make ui`,
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...` (`TestUIPage`,
  `TestUINoHTMLInjection`, `TestUIFileHeaders`);
  the touched UI test files in all four cases, and `test_parts_guard.py`;
  `gofmt`, `go vet` for three platforms, `GUIDELINES/shared/tools/check`.
  **The owner's check by hand** (D13): Chrome,
  Firefox and the Android app's autofill each still offer to save after a
  sign-in and after the first owner's creation; where one does not, D41.

### Block 3: Page

The brief's part (a), a Cube's rooms after a change of Cube,
is on main (Analysis);
the poll before the start's card moved here from the card's block,
since it is the page's start and `app.js`'s.

- **Goal:**
  nothing of the page is read behind the start's card,
  and the Integrations card shows the newest of its reads and says nothing of
  the LAN before its answer.
- **Changes:**
  1. `app.js`:
     `applySettings` restarts the poll only where it runs (`state.pollTimer`);
     `init` starts it after `loadMeAndPreferences`, beside the first `refresh`;
     `afterLogin`, a dismissal of the card and Log out start it as now.
  2. `src/integrations/store.ts`:
     each of the three reads takes a number of its kind before its request
     and writes its answer only where it is still the newest of that kind and of
     the session; `data.seen`,
     set by `loadSeen`'s answer and cleared by `forgetIntegrations`.
  3. `src/integrations/Integrations.tsx`:
     the LAN's lines and Look again drawn by `data.seen`, not `data.loaded`;
     before it, nothing under "Seen on the LAN".
- **Tests,** each seen failing on main first:
  - `test_first_owner.py` and `test_login.py`, under the clock:
    with the start's card open past two poll intervals,
    no read of the Cubes or the log's alerts is sent,
    and the poll runs once the card has closed.
  - `test_integrations.py`:
    with the LAN's read held, the found section says nothing,
    then the reason once it answers; two showings of System settings,
    the first's read of the integrations held and answered last,
    leave the second's list.
- **Documents:**
  `docs/ui.md` (the start, now as it says; Integrations, the newest read).
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-looseends-start-poll.md`:
  "- The page polls the Cubes only once the sign-in card of its start has
  closed."
  (two lines).
  The Integrations card's two are of its part, not released yet: none (§5.2).
- **Commits:**
  `fix(ui): The page polls once the start's card has closed`,
  `fix(ui): Integrations shows its newest reads and waits for the LAN's answer`.
- **Verification:**
  `make ui-check`; `make ui`
  and `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`;
  `test_first_owner.py`, `test_login.py`,
  `test_integrations.py` and `test_refresh_now.py` in all four cases;
  `GUIDELINES/shared/tools/check`.

### Block 4: Clock

- **Goal:**
  every pause of Playwright's clock is at the page's now in seconds,
  through one helper.
- **Changes:**
  1. `tools/ui-test/helpers.py`:
     `pause_clock(page, ahead_ms=50)`,
     `page.clock.pause_at((page.evaluate("Date.now()") + ahead_ms) / 1000)`,
     its docstring the one of `test_personal_updates.py`'s `pause`,
     which calls it.
  2. The four calls (`test_admin_updates.py` twice, `test_first_owner.py`,
     `test_restart.py`) call it.
  3. The guard, in `conftest.py`: at collection,
     a test module whose source calls `clock.pause_at(` is an error that names
     the helper; `helpers.py` alone may.
- **Tests:**
  each of the four, with a fault planted for it (the focus given in an effect,
  the badge a frame late, the name checked at once),
  fails with the helper and passes without the fault;
  a module planted with a direct call fails collection.
- **Documents:**
  `tools/ui-test/README.md`, where it tells the mark `clock`: the helper,
  and why seconds.
- **CHANGELOG:**
  none: `tools/` is not shipped (`:!tools/**`).
- **Commit:**
  `test(ui-test): Playwright's clock pauses at the page's now, in seconds`
- **Verification:**
  the five files in all four cases, each exit 0;
  the planted faults seen failing; `GUIDELINES/shared/tools/check`.

### Block 5: ServerTests

- **Goal:**
  the four tests hold on a slow or loaded runner because they wait for what they
  check, the armv6 analysis' option A for the first two.
- **Changes** (`internal/server`, test files alone):
  1. `TestConfigWriteConfirmed`:
     `ApplyAfter` 300 ms; its own re-reads, 50 ms,
     then `testkit.Scale(500ms)` and `testkit.Scale(time.Second)`,
     the last two well behind the apply on any runner;
     and `confirmWithin` `testkit.Scale(3*time.Second)`,
     under the wait's 5 s scaled,
     so that a race lost ends in a `not_confirmed` line the test names,
     not in a timeout. docs/testing.md's rule allows it:
     a deadline for an answer that comes takes the scale.
  2. `TestProxyRetryStops`:
     the old address from `heldPorts.taken`, never given,
     so the test holds it throughout;
     `heldPorts` counts the listens at each address,
     and after each step the test asserts with `testkit.Never` that no further
     listen comes there,
     the end a fresh `GET /api/v1/cubes` after `5 * quickRetry[1]`;
     the slot's check as now.
     The dial to a freed port goes.
  3. `TestLogOfEveryCubeWithoutStore`:
     before its reads, `testkit.Eventually` until the fake Cube's ring has its
     six state lines and the second Cube's its `disconnected`.
  4. `TestLogAfterReadsABurstOnce`:
     the one wait over the whole burst becomes a wait for progress:
     the test fails where no page reads a new line of the burst for
     `testkit.Wait`, scaled, before the writers are done,
     and otherwise runs to the burst's end however long it takes;
     `go test`'s `-timeout` bounds it.
- **Tests:**
  each change seen to fix its race:
  its old form made to fail by a planted delay (the re-reads before the apply,
  another listener on the freed port, the second Cube's line held back,
  a slow page read), and the new one passing with the same delay.
- **Documents:**
  `docs/testing.md` where it names a test of these or their pattern (the ports
  held for the proxies; waiting for a pump's line).
- **CHANGELOG:**
  none, said in `Key changes` ("No CHANGELOG bullet: only tests change,
  and nothing shipped behaves differently"),
  as the project's test commits do (Analysis).
- **Commit:**
  `test(server): Four tests wait for what they check, not for a fixed time`
- **Verification,** each by its exit code:
  `MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=20 -timeout 30m -run …` of `./internal/server` over the four tests' names, once alone and once while `make race` runs beside it; `MASSIMO_ENV_FILE=/dev/null go test ./internal/server`; the four under armv6 emulation, `-count=5` with `MASSIMO_TEST_SCALE=4` (the Docker image `massimo-armv6-test`, or `make test-armv6` where QEMU is installed); `gofmt`, `go vet` for three platforms, `GUIDELINES/shared/tools/check`.
  CI's job "Test (armv6, QEMU)" on the pull request.

## Proposed commit message

The plan: `chore(aps): +LooseEnds v1.0`

Block 1:

```
fix(server): An integration's users leave an open server open

For AP LooseEnds v1.0, Block 1 (D1-A, D2-A): a server is open while it
has no regular user, a user that is not an integration's, disabled ones
included; an integration's users no longer close it, block its first
owner or remove auth.keep_open, so that approving Home Assistant on an
open server no longer turns the page into a sign-in nobody can pass,
and MASSIMO_API_TOKEN makes the owner of such a server at its start. A
server that holds an integration's users alone becomes open at the
upgrade, and its log says so at each start.

Key changes:
- internal/store: one rule for a regular user (regularUser,
  countUsers), taken by CountUsers, noUsers, SetKeepOpen and
  DropKeepOpen; CountIntegrationUsers; ApproveIntegration and
  IntegrationUser leave auth.keep_open as it is.
- internal/server: the start's warning where only an integration's users
  are left; authMode's comment.
- Tests: the store's count, keep-open and bootstrap beside an
  integration's users; the trap at the API (a request, its approval on
  an open server kept open, then still open, the first owner made, the
  setting kept until then); the start's warning; the tests that expected
  users mode from an integration alone make their owner first.
- Docs: docs/api-v1.md (Authentication, auth.keep_open), the OpenAPI
  file, docs/store.md (Bootstrap and compatibility), README.md (Server),
  docs/ui.md; a Changed fragment, and the keep-open fragment amended.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-24): first revision, from the owner's decisions D1-A, D2-A,
  D3-A and D13-A on the walk along the first owner's path and on the runs'
  NOTEs; D40 to D42 open.
- v1.1 (2026-11-24): the Analysis' five headings read "For Block <n>",
  since the plans' check takes every heading "Block <n>" for a block of the
  Implementation Plan; nothing else changed, and D40-A, D41-A and D42-A stand
  as taken with PLAN++.
