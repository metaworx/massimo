# NOTE AccessEnds v1.2: how block 9 went

Where Block 9, Managers, went differently from the plan,
the choices taken in the run, what the two readings changed,
among them an older hole closed, and what verified it.

## What it made

Who manages an integration's users,
as the owner answered at the gate of 2026-11-26, "D41: A4+A5":

- **A holder of `users`** manages them by the rank rule, as before,
  a role beyond the maximum taken and capped (D6-A).
- **A holder of `guests` that comes through no integration**
  reaches regular guests alone, and lists no integration's user (D45-A).
- **A call through an integration**
  (its key at any level, for a user linked to a Massimo user,
  or a key one of its users holds from before)
  reaches its own integration's users stored as guest alone,
  where its chain holds `guests`:
  no other integration's user, no regular user, no new user,
  and no password registered for a new name.
- **A holder of `integrations`**
  (D41-A5, D46-A to D48-A) reaches every integration's user but one stored as
  owner, which stays an owner's alone (D47-A as revised,
  For the completion gate, 6): it renames, disables,
  gives an end date to and removes it,
  revokes the keys and removes the identities it holds from before,
  and gives it a role it outranks within its integration's maximum,
  `403` beyond (D48-A).
  It sets no password of one and removes none held from before.
  The seeded manager, which does not outrank `integration`,
  moves such a user off it, and an owner gives it back.
- **The own name is never a call's through an integration:**
  a call for a user linked to a Massimo user,
  or a key one of an integration's users holds from before,
  has none of the rights of that user's own name on the 16 users routes;
  the user keeps them signed in itself.
- **An integration's own user,** `<uuid>:`, which holds its key,
  **and its system user** are removed by nobody through the users routes:
  the integration's own removal keeps both.
- **`may.beyond`** says whether a user's stored role lies beyond its
  integration's maximum, since `may.roles` now leaves out roles within it
  that the caller does not outrank.
- **The page**
  (D46-A): a holder of `integrations` sees Users,
  every integration's user but one stored as owner and itself,
  each panel following the server's `may`;
  Add a user is drawn for a holder of `users` or `guests` alone;
  Remove the password follows `may.password`;
  the line under a stored role beyond the maximum follows `may.beyond`;
  the integration's own and system user offer no Remove;
  the Integrations card's Link stays for holders of `users`,
  `guests` or `passwords`.
- **Two commits,** on `maint/0.17` (Different from the plan, 1):
  "fix(security:api):
  Another integration's key or a guests holder no longer manages an
  integration's users" and "feat(ui):
  A holder of integrations sees Users and manages an integration's users there".

## Different from the plan

Against the plan's block, item by item; each the owner's to change:

1. **Into 0.17:**
   since the version freeze of 2026-11-26 the block goes into 0.17.
   The coordinating session carried both steps from the plan's v1.2 commit,
   which `maint/0.17` holds, onto that branch's head,
   "merge(AccessEnds/Block-10): The dice alternates words and characters;
   a tap copies the suggestion (#472)",
   by a three-way merge of trees without conflict.
   The pull request's base is `maint/0.17`,
   and this NOTE goes there too (AP MaintBranch, D5-A),
   reaching `main` by the upward merge.
2. **The own name refused through an integration** (item 5, `allowed`,
   and `access.self`; from the first reading).
   The plan's table kept the linked call's "own name and password as before";
   that rule let a call through an integration at level `view`,
   for a Home Assistant user linked to a Massimo user,
   make that user a key holding every capability of its role,
   whatever the ceiling, the maximum and the level,
   and set its first password, on the base as on the first tree.
   The owner's A3, which A4 takes,
   is that a call through an integration manages its own integration's users
   alone, which the linked local user is not;
   so `selfOf` is never true for such a call, nor, from the second reading,
   for a key an integration's user holds from before,
   which the documents count as one.
   It reaches the 16 routes under `/api/v1/users/{name}`: `GET`,
   `PUT` and `DELETE` of the user; its tokens, `GET`, `POST` and `DELETE`;
   `register`; its identities, `GET`, `POST`, `PUT` and `DELETE`;
   the start of a link; and the pending link, `GET`, `POST` and `DELETE`.
3. **The integration's own and system user stay** (not in the plan;
   from the first reading):
   the manager, or a holder of `integrations` alone, could remove `<uuid>:`,
   `403` on the base, and the owner could already;
   its key went, and a request attached to the integration again could not
   collect one ("FOREIGN KEY constraint failed").
   Now `DELETE` of either answers `403` to everyone, the owner too,
   with words that point to `DELETE /api/v1/integrations/{uuid}`,
   which keeps the integration to be attached again.
4. **`may.beyond`** (not in the plan; from the first reading):
   the page read "not in `may.roles`" as "beyond the maximum",
   which the rank rule on `may.roles` made false;
   the field is in the OpenAPI file, `api.d.ts` and `docs/api-v1.md`.
5. **`mayRegister`** (`opaque_api.go`, not in the plan)
   registers a record for a new name only for a caller that may add a user,
   so a call through an integration is refused there too.
6. **A change is asked again in the write:**
   `putUser` asks `mayManage` again of the user as `UpdateUser` reads it,
   so that an owner's making it an owner meanwhile is not overwritten
   by a holder of `integrations` (from the first reading).
7. **The refusals' words:**
   the plan's three texts as given;
   besides, the integration's own users' refusal (3 above),
   and a holder of `users` and `guests` that does not outrank guest gets the
   rank rule's words, not "a holder of integrations or of users changes it".
8. **Remove the password follows `may.password`** (`Users.tsx`,
   not in the plan): following `may.manage`,
   it would have been offered to a holder of `integrations`,
   which the server refuses.
9. **Add a user** is drawn for a holder of `users` or `guests`,
   as the plan says, so it is gone for a holder of `passwords` alone too,
   whom the server refused anyway.
10. **The Integrations card's Link** (`integrations/store.ts`,
    not in the plan's files):
    with `canManageUsersOf` taking `integrations`, the first tree offered Link
    to a holder of `integrations` alone; the coordinating session kept Link
    for holders of `users`, `guests` or `passwords`, as before the block
    (For the completion gate, 7).
11. **The CHANGELOG's one Security bullet is two:**
    "An integration's users are changed by holders of users or integrations
    and by their own integration alone."
    and "A call through an integration no longer changes regular guests,
    nor makes a key for or sets the password of the user it is linked to.";
    the Added bullet as the plan gives it.
12. **`TestWhoManagesAnIntegrationsUsers`:**
    ten holders, the plan's nine and a holder of `read` and `integrations`
    alone, against users an integration's first call makes for each,
    stored as `integration`, `guest` and `owner`,
    the second integration's guest and a regular guest,
    where the plan named the integrations' system users; for each,
    the list with `may.manage`, `may.roles` and `may.beyond`, each change,
    a new guest and the regular guest's password and key; besides,
    `may.beyond` of an integration's user stored as operator (true) and as
    viewer (false), the held key's own name, the refusals' words,
    a registration of a new name,
    an empty password (`403` from a holder of `integrations`,
    `200` from the manager),
    and a held key and identity a holder of `integrations` revokes and removes,
    the key answering `401` after.
13. **Tests the plan did not name:**
    `TestNoOwnNameThroughAnIntegration` (every route of the own name through a
    linked call at `view`, and at `admin` for a user linked to a manager;
    the user itself keeps them),
    `TestIntegrationsOwnUsersStay` (the owner, the manager and a holder of
    `integrations` each `403` on both users,
    then a request attached again collects a key that works);
    `test_remove_the_password_follows_may_password`,
    `test_add_a_user_for_users_or_guests_alone`;
    `test_an_integrations_holder_manages_its_users` also holds no line under a
    user stored as `integration`, for the holder and for the manager,
    no Remove for the integration's own users, and no Link.
    `TestAssignableRolesWithinTheCeiling`: the manager offered `[viewer]`,
    the empty list held now by a holder of `read` and `integrations`.
14. **Two tests not the block's changed** for what the block changed:
    `TestLastRegularUserRemovedWarns` (AP LooseEnds') removed `<uuid>:system` on
    an open server, now refused:
    it removes one of the integration's ordinary users,
    what it checks unchanged;
    `test_an_integrations_role_choice_stays_within_its_maximum`'s stubbed owner
    says `beyond: true`, as the server sends now.

## Choices the plan left open

- **The integration's refusal words** go to a guest that `guests` reached
  before, the call's or the holder's;
  every other refusal keeps the rank rule's or the capability's words.
  The other way: the new words for every refusal of an integration's user.
- **A call through an integration making a user** gets the same words as
  changing one,
  "a call through an integration changes its own integration's users alone".
  The other way: words of its own.
- **A key held from before counts under its user's integration,**
  as the plan's item 1 has it for `guests`, and for the own name too.
  The other way: such a key keeps its own name,
  which the documents already said it did not.
- **`may.beyond` speaks of the stored role;**
  a role chosen and not saved, where the list read again offers it no more,
  still goes by `may.roles` (D4).
  The other way: the page reckons the maximum itself,
  from roles' sets it does not hold.
- **The own users' refusal is for everyone,** the owner too.
  The other way: the owner keeps the removal, as on the base,
  and the integration is lost with it.
- **No test holds the window of 6 above;**
  the server's reader held it open with a seam in a scratch copy alone,
  and saw the second ask close it.
  The other way: a seam in the server for a test.

## What the readings found

One builder built the block in an exported copy of the plan's v1.2 commit;
two readers read the two recorded trees,
one the server and its security, the other the page and the tests.

Confirmed in the first reading:
row by row the access rules match the plan's table;
an integration's user stored as owner stays an owner's alone on every users
route, `PUT`, `DELETE`, tokens, identities and `register`;
a role given is at most the holder's set, the ceiling and the maximum together,
and `integration` is given by rank alone; Home Assistant's own flows, its `/me`,
the collection of its key and its calls for its users, are unchanged;
a held key counts under its user's integration and is capped;
a revoked or disabled integration's users are still managed;
users an older store moved to `integration-v0` are regular users,
which keep regular guests and lose an integration's;
the new tests fail on the base for the reasons they name;
the capability rules' fragment and its two copies are the same.

Four musts, done in the first rework:

- **A call through an integration for a linked local user could make that
  user a key and set its password** (2 above), older than the block.
- **The integration's own users could be removed,** and the integration then
  not attached again (3 above).
- **"Beyond its integration's maximum" was said where it is not so:**
  the manager saw the line under every integration's user stored as
  `integration`, which always lies within; now from `may.beyond` (4 above).
- **The Integrations card's Link for a holder of `integrations` alone**
  came up blank where the server proposed a user the holder cannot read,
  its hint naming that user, and Link sent it and got `403` (10 above).

Shoulds, done: the rank words of 7 above; the window of 6 above;
`docs/api-v1.md` says that removing a linked integration's user does not stop
its calls, "disable it, or unlink it";
the tests of 12 and 13 above that the plan did not name;
`docs/ui.md`'s paragraph on a holder of `integrations` moved and qualified,
and the head of the Users bullet given the exception;
the two Security bullets of one clause each,
and message 01's sentence on a password held from before.
Kept for the completion gate, not changed:
a holder of `integrations` can tell whether a login name exists (5 there).

The second reading found no must.
The server's reader's probes:
all 17 own-name calls through an integration (16 routes,
`PUT` twice) answer `403` at `admin` through a chain that holds `guests`,
the owner signed in directly keeping its rights;
the own users stay with the new words, disabling the system user still `200`,
and an integration attached again collects its key;
Home Assistant's flows work over HTTP;
`may.beyond` is the same for every holder; the window of 6 above,
held open by a seam in a scratch copy,
gives `403` on the rework and `200` on the first tree.
The page's reader's probe, signed in as the manager:
no line under a user stored as `integration`,
the line under a user stored as operator, no Remove for the own users,
and no Link for a holder of `integrations` alone.

Three small points, done in the second rework:

- **No Go test held `may.beyond` for a role but owner:**
  cut down to owner alone, it passed every test;
  now the operator and the viewer of 12 above.
- **A key an integration's user holds from before kept its own name:**
  it read its user, renamed itself and listed its tokens, `200`,
  where `docs/api-v1.md` counts it as a call through an integration;
  `selfOf` now asks that its user be no integration's,
  and the test holds the three `403`.
- **`docs/ui.md`'s words:**
  the Users bullet's new clause given a verb and "but one stored as owner";
  D47-A no longer cited for the own users' missing Remove in `docs/ui.md`,
  in `Users.tsx` and in `users_api.go`, since that refusal is the rework's own.

Kept for the completion gate from the second reading:
the integration's own key, called without the header,
no longer reads its own user or its tokens (3 there).
No reader read the second rework separately:
its points were the readers' own,
and rest on the builder's mutations of `beyond()` and `selfOf`.

## Seen and left

- **Two test comments still cite D47-A for the own users staying:**
  `TestIntegrationsOwnUsersStay`'s and the one in
  `TestLastRegularUserRemovedWarns`,
  where the second rework took that citation out of the documents and the code;
  seen in the commits by this NOTE's writer.
  The next change of those tests'.
- **The window of 6 above** has no test in the suite:
  the next change of `putUser`'s.
- **The unsteady tests the runs met,** none the block's:
  `TestProxyFailingInTheAPI`,
  in the builder's whole Go run of step 01 and in its race run after the first
  rework, passing alone (AP SteadyTests names it);
  in the server's reader's first race run, at a load of about 67, that test,
  `TestProgramTag` (AP SteadyTests'),
  `TestUnconfirmedRoomCommandNotResent/superseded`, a resend timing test,
  and `TestParallelOrListed`, which failed on the reader's own probe file,
  each passing alone; `test_system_info_keeps_its_folds_a_copy_and_a_selection`,
  twice in the builder's first whole UI run on a copy icon's fade,
  passing alone (AP SteadyTests' run names its fade among the next candidates).
- **The builder's first whole UI run** ended at the 1700 s timeout at a load of
  about 42,
  and the page's reader's first run reached 200 of 206 tests before it:
  each made again or covered by later runs (What verified it).
- **The external Home Assistant integration** was read by no reader:
  that its code reads neither route of For the completion gate, 3,
  is the coordinating session's word.

## For the completion gate

1. **An older hole closed:**
   a call through an integration for a linked local user could make that
   user a key and set its password, through the own-name rule;
   now refused on all 16 own-name routes,
   and for a key an integration's user held from before too.
   The user keeps them signed in itself.
2. **The integration's own users can no longer be removed through the users
   routes,** by anyone, the owner included;
   `DELETE /api/v1/integrations/{uuid}` removes the integration
   and keeps it to be attached again.
3. **The integration's own key, called without the header,**
   can no longer read `GET /api/v1/users/<uuid>:` or its tokens,
   `200` before; its `/me`, the collection of its key, its calls and
   `DELETE /me/token` are unchanged.
   Home Assistant's integration code reads neither route.
4. **Removing a linked integration's user does not stop its calls,**
   which go on as the linked user: older than the block,
   now in `docs/api-v1.md`: disable it, or unlink it.
5. **A holder of `integrations` can tell whether a login name exists:**
   `GET /api/v1/users/{name}` and `register` answer `404` for a missing name
   and `403` for one it may not see.
   Nothing new: on the base `PUT /api/v1/integrations/{uuid}/users/{id}` told it
   already.
6. **D47-A as revised by the coordinating session:**
   an integration's user stored as owner stays an owner's alone,
   so that "nobody but an owner changes an owner" holds without exception;
   the manager and a holder of `integrations` get `403`.
   The plan's D47-A, taken under PLAN++, carries option C's owner rule;
   its option A alone said "whatever its role".
7. **Link stays for holders of `users`, `guests` or `passwords`:**
   the coordinating session's choice;
   a holder of `integrations` alone sees who each user acts as, without Link.
8. **What a user notices:**
   a holder of `integrations`, the manager among them,
   sees and changes an integration's users in System settings, Users;
   a user the manager moves off `integration` waits for an owner to give it
   back; Add a user shows for holders of `users` or `guests` alone;
   another integration's key and a holder of `guests` of no integration no
   longer reach an integration's users;
   an integration's calls no longer reach regular guests or make a user.

## What verified it

- **The builder, on the first trees:**
  `make ui`, `gofmt -l .` printing nothing, `go vet` for linux,
  windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test -count=1 ./...` (38 packages),
  the race run of `internal/server` with `MASSIMO_TEST_SCALE=2` (974 s),
  `make cross-check`, `make cross`,
  `make ui-check` and `go run ./tools/apitypes -check`, each exit 0;
  the strict changelog check on both trees, `reflow.sh --check`, 0 errors,
  and `sync-docs.sh --check` in a scratch repository for the fragment's two
  copies.
  UI: the four files of the plan in all four cases, 306 passed;
  the whole suite in 1100-en, 867 passed and 3 failed, two of which passed on
  the suite's retry (Seen and left).
  Seen failing: `TestWhoManagesAnIntegrationsUsers` and
  `TestAssignableRolesWithinTheCeiling` on the base,
  exactly on the plan's "Today" column;
  `test_an_integrations_holder_manages_its_users` on step 01's tree.
  No script or test in `packaging/`, `tools/`, `app/` or `.github/` manages
  users through an integration's key or a `guests` holder;
  there is no Home Assistant test setup in the repository.
- **The server's reader, first:**
  `go test -count=1` of `internal/server` and `internal/store`,
  `gofmt`, `go vet` of `internal/server` and `apitypes -check`, each exit 0;
  the race run of `internal/server` at scale 2, exit 1 on four tests,
  each passing alone (Seen and left);
  probes of the own name on the base and on step 01, of the own users'
  removal and the attach after it, of a linked user's removal,
  and of a login name told through the integrations route on the base.
- **The page's reader, first:**
  `make ui-check`, exit 0;
  `reflow.sh --check` of `docs/ui.md` and the Added bullet, 0 errors;
  the new and changed Go tests on step 01's tree pass,
  and on the base with step 01's test file fail on the plan's "Today" column,
  the held key's revoke included; `test_users.py`,
  `test_integrations.py` and `test_parts_guard.py` in all four cases,
  200 of 206 passed and none failed before the timeout;
  the new UI test 4 of 4 alone; a scratch probe of the line and of Link, 4 of 4;
  `commit-msg.sh`, exit 0 on both messages.
- **The builder, after the first rework:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for three platforms,
  `apitypes -check` and `make cross`, each exit 0;
  `go test -count=1` of `internal/store` and `cmd/massimo` passing,
  of `internal/server` passing once `TestLastRegularUserRemovedWarns` was
  moved (14 above), run again in full;
  the race run at scale 2, exit 1 on `TestProxyFailingInTheAPI` alone,
  passing alone under `-race`;
  `reflow.sh --check`, 0 errors on `docs/api-v1.md`, `docs/store.md`,
  `docs/ui.md` and the fragment; the strict changelog check on both trees.
  UI: the four files in all four cases, 318 passed;
  the whole suite in 1100-en, 872 passed.
  Seen failing: eight mutations of the server, one at a time,
  each failing the test that holds it (the own name, the own users,
  `beyond`, the rank words, the registration's refusal, the empty password,
  the roles within the maximum, `integrations` let into the routes);
  five changes of the page undone in turn, each failing its own assertion
  (Remove on the own users, the line, Link, Remove the password, Add a user).
- **The server's reader, second:**
  the probes of the second reading; `make ui`,
  `go test -count=1` of `internal/server`, `internal/store` and `cmd/massimo`,
  and the race run of `internal/server` at scale 2 (608 s, no failure),
  each exit 0.
- **The page's reader, second:**
  the probe as the manager in all four cases; `test_users.py` in all four cases,
  104 passed; `test_integrations.py` and `test_parts_guard.py`, 110 passed;
  `make ui-check`, exit 0;
  `reflow.sh --check` of `docs/ui.md` and both Security bullets, 0 errors.
- **The builder, after the second rework:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for three platforms,
  `go test -count=1 ./internal/server`,
  `reflow.sh --check docs/ui.md docs/api-v1.md` (0 errors) and the strict
  changelog check on both trees, each exit 0;
  `test_users.py` and `test_integrations.py` in all four cases, 148 passed;
  seen failing: `beyond()` cut down to owner alone,
  and `selfOf` without its new condition, each failing the test.
- **The coordinating session:**
  both steps carried onto `maint/0.17`'s head without conflict;
  in a clone with each commit `commit-msg.sh`, 0 errors and 0 warnings,
  `changelog.sh check --staged --strict`, 0,
  and `GUIDELINES/shared/tools/check` over the range,
  twelve checks all well.
  The Go tests of `internal/server`,
  `internal/store` and `cmd/massimo` and the whole UI suite in 1100-en,
  on the committed tree, after make ui: go test -count=1 of internal/server,
  internal/store and cmd/massimo, ok, and the whole UI suite in 1100-en,
  875 passed, exit 0.
- **Not run here:**
  `GUIDELINES/shared/tools/check` by the builder, in a copy;
  the whole `go test ./...` on the last tree, which is the pull request's CI;
  the external Home Assistant integration.
