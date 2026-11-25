# NOTE AccessEnds v1.0: how block 2 went

Where Block 2, Users, went differently from the plan,
the choices taken in the run, what the readings changed, and what verified it.

## What it made

An integration's user is offered the roles within its integration's maximum,
and Users on a server without an owner lists an integration's users
(LooseEnds.D45-A with D3-A to D6-A, LooseEnds.D46-A with D7-A to D10-A).

- **The users routes say, per user, the roles the caller may give it**
  (`may.roles`, in the list and in a change's answer),
  and the Role list offers those.
  For an integration's user they are, of the roles the caller may give,
  those whose capabilities lie within the role `integration`'s ceiling
  and its integration's maximum, and `integration` itself;
  never owner, left out by its key.
  `roles` is always sent, an empty list where the caller may give none.
- **`integration` is given back** to an integration's user moved off it,
  by the rank rule as any role, and to no regular user;
  before, one Save away from it was for good (403).
  A role beyond the maximum is still taken (D6-A),
  and every call of such a user is held to the maximum whatever its role.
- **A role beyond the maximum stays chosen,** stored so or chosen and not yet
  saved when the list is read again with fewer roles,
  with the line "Beyond its integration's maximum:
  it acts within that maximum all the same." under it;
  it is offered no more once another is saved (D4-A).
  Add a user no longer offers `integration`, which no new user is given.
- **On an open server the Users head says No owner yet,** as a warning,
  whatever the users, and a count again once an owner exists (D8-A).
  Once the users are read, or their read has failed,
  an integration's users are listed as plain rows,
  their name and badges and nothing to open or press (D7-A),
  under a line that says they act through their integration alone (D9-A),
  with **Open Integrations** under them;
  without them the section is as before.
  This takes back the half of LooseEnds.D42-A that listed none of them;
  the first owner's card keeps its title.
- **Users is read again after every change in Integrations**
  where who is in may manage users (D10-A, widened),
  in the session the change started in alone,
  without the change's outcome waiting for it;
  of two reads that overlap, the newest asked for is shown.
- **Two commits:**
  "fix(api): An integration's user is offered the roles within its maximum"
  and "fix(ui): Users on an open server lists an integration's users,
  no owner yet".

## Different from the plan

Against the plan's block, item by item:

1. **`users_api.go`:**
   `userMay.Roles` is `json:"roles"`, always sent,
   an empty list where the caller may give none,
   where the plan said `omitempty`: left out,
   "may give none" read as a server that sends no list,
   and the page fell back to the old rule,
   without the line (What the reading found).
   The OpenAPI file marks it required.
   New: `assignable`, `allIn`, `mayOf` and `maxOf`;
   changed: `userMay`, `mayAssign` and `assignFail` (both take the user),
   `listUsers` and `putUser`;
   `postUser` only at its two calls, which pass the user,
   Block 3's part of it untouched.
   `putUser` reads the roles and its user's maximum before anything is written
   or a proof used up.
2. **`integrations_api.go`, beyond the plan's files:**
   `integrationMarks` takes the integrations its caller read,
   so that `listUsers` reads them once a request;
   `getIntegration` reads them itself.
3. **`mayAssign`** as the plan has it.
   `assignFail` says why `integration` is refused to an integration's user by
   rank, the rank's or the capability's reason;
   a regular user still hears that it is an integration's and its users' alone.
4. **`roleOptions(current, u, chosen)`,** a third argument beyond the plan's:
   the role chosen and not saved is kept beside the stored one,
   so that a list read again with fewer roles leaves the choice shown,
   and Save sends what the select shows.
   Where `u.may` is there, its `roles` are taken exactly, an empty list as none;
   the old rule is left for a user without `may` and for Add a user.
5. **The line `users.roleAboveMax`** follows the chosen role,
   not the stored one alone, and the select is described by it.
6. **The German text** says "er handelt", not the plan's "es handelt",
   to agree with "der Benutzer"; the Dutch, "hij handelt".
7. **`UsersCard`** as the plan has it;
   `OpenServer` takes `useLook` in place of `useLang`, since it draws badges,
   and Open Integrations has no guard by capability,
   which always holds on an open server.
8. **The open server's line** is drawn once the users are read or their read has
   failed, not before: it said "No users yet" until the read answered,
   then switched to the integrations' line.
   A failed newest read is marked (`users.failed` of `store.ts`,
   cleared at a read that goes through and at a sign-out).
9. **D10-A's reach widened:**
   the plan, after an approval, a refusal and a removal;
   the block, after every change in Integrations,
   since a name, a link and a maximum change what Users shows as well.
   The read is not awaited, so the outcome is said without waiting for it;
   the session's number is taken at the reload's start;
   a failed read is swallowed, the list as it was;
   `loadUsers` numbers its reads, and an answer older than the newest asked for
   writes nothing.
10. **Tests beyond the plan's:**
    - Go: in `TestAssignableRolesOfAnIntegrationsUser`,
      a role holding `users` that outranks viewer alone gets viewer alone and is
      refused `integration`;
      the integration's own key gets guest alone and is refused `integration`;
      owner given to the integration's user is taken and offered no more;
      `integration` given back is checked through a key the integration's own
      user holds, whose call stays within the maximum.
      `TestAssignableRolesWithinTheCeiling` (new): a maximum stored beyond the
      ceiling offers nothing beyond it,
      and the manager managing a guest beyond the maximum gets an empty list.
      `TestUsersRoutes` had to change: `userMay` holds a slice now,
      so it compares field by field,
      decodes the manager's list afresh (a reused element kept the owner's
      `roles`), and holds the manager's guest offered guest alone.
    - `test_users.py`:
      a chosen role kept when the list narrows;
      a role beyond the maximum offered no more once saved.
      In the plan's test the stubbed users' panels have their reads of keys
      stubbed too, which answered 404 with console errors.
    - `test_ways.py`:
      the head a count again once an owner exists;
      `test_users_on_an_open_server_whose_read_fails`.
      `test_create_from_users_shows_the_owner`, not the block's,
      waits for the line, which now comes with the users' read;
      a line that never comes still fails it.
    - `test_integrations.py`:
      the users read again after a refusal and a removal,
      the outcome said while that read is held;
      an older users' answer writing nothing over a newer;
      no users read after a sign-out during the reload.
    - `test_parts_guard.py`:
      the view as the plan has it;
      its comment says it guards Open Integrations and Create the owner…,
      since the rows hold nothing the guard can hold,
      and the region has the users read again as a source (System settings shown
      again), where the plan said the rows.
11. **Documents beyond the plan's:**
    `docs/api-v1.md` says that a change's answer carries `may`,
    and with the OpenAPI file what `may.roles` tells its caller (For the
    completion gate, 3); `docs/ui.md` cites "(AP LooseEnds, D42;
    its list taken back by AP LooseEnds, D46)";
    `admin/index.tsx` names Integrations among the callers of
    `MassimoParts.users.load`.
    `docs/store.md` is unchanged: its "never given to a regular user" holds.
12. **The CHANGELOG fragments and the commits' summaries** in the plan's words.
    The chosen role kept (4) is in the first commit,
    whose code and reach (any read of the users while a draft is held) have it.
13. **The verification:**
    the whole UI suite on the first rework's tree, not on the last;
    `GUIDELINES/shared/tools/check` by the coordinating session on the commits,
    the exported copy having no git of its own.

## Choices the plan left open

- **`roles` empty where the caller may not manage the user;**
  `may` itself is there for every user the caller may see.
  The other way: `roles` left out there.
- **`integrationMarks` given the integrations its caller read:**
  the other way, a second read of them in `listUsers`.
- **`putUser` reads what its answer names before it writes:**
  the other way, after the write,
  where a failed read would answer an error for a change made.
- **The open list draws only users with an integration:**
  a regular user there would mean the server is open no more,
  which the next read of who is in shows.
  The other way: every user listed.
- **No guard by capability on Open Integrations:**
  the other way, `can("integrations")`, which always holds there.
- **Users read after every change in Integrations,
  the outcome not waiting for it:**
  the other ways, D10-A's three changes,
  where a rename, a link or a new maximum left Users stale;
  or the read awaited, as the first build had it,
  each change then waiting for the users', the providers'
  and every open user panel's reads before its outcome.
- **A failed read marked, so that the open server's line still comes:**
  the other way, the line drawn before the read, as before,
  which said "No users yet" above a list about to come.
- **The chosen role kept in the first commit:**
  the other way, in the second, whose D10 reaches it within the page.
- **LooseEnds' text of D42-A left as it is:**
  that plan is not this one's;
  this plan's Discussion and `docs/ui.md` name the taking back.

## What the reading found

One builder built the block in an exported copy of the plan's commit.
Two readers read the first two trees:
one the server and its security, one the page and the tests.

Confirmed: no caller is offered or may set a role it could not set before,
apart from `integration` given back to an integration's own user by the rank
rule, which of the seeded roles only the owner passes;
`principalFor` holds every call to the maximum whatever the role,
so giving `integration` back widens nothing;
`may.roles` never names a role the change refuses,
being `mayManage` and `mayAssign` as the change asks them;
every store state probed is right (a role stored above the maximum kept
and not offered again, a lowered maximum leaving `integration` alone,
a revoked integration still giving its maximum, an integration gone giving none,
so only `integration`, and that user refused at every call);
four changes at once under `-race`, clean;
the new tests failed on the base for the reasons they name;
three planted faults (owner offered, no within check, `integration` to anyone),
each caught; the page in light and dark,
at 390 and 1100 px and in six languages,
its controls labelled and keyboard operable,
and the open list taking neither `#users-list` nor `.user-panel`.

Two musts, the page's reader's, done in the first rework:

- **A sign-out during the Integrations reload sent a users' read nobody asked
  for:**
  after the sign-out, the check that who is in may manage users held,
  nobody being known, so the read went out without a token,
  waited in the sign-in, kept the change's run from settling,
  and was sent again with the next user's token.
  The session's number is taken at the reload's start now, and a test holds it.
- **A role chosen and not saved left the Role list when `may.roles` narrowed:**
  the select showed blank while Save stayed on and would send the hidden role.
  It is kept now (Different from the plan, 4), and a test holds it.

The shoulds, done in the first rework:

- **`roles` left out** looked like a server that sends none (Different from the
  plan, 1), both readers'.
- **What no test held:**
  the ceiling's half of "within",
  a planted fault dropping it passing every test;
  `integration` given back checked through the integration's key with the header
  that acts for its user, which caps the call anyway;
  the users read after a refusal and a removal;
  a role beyond the maximum offered no more once saved;
  the head back to a count once an owner exists.
- **Each change in Integrations waited for the users' reads** before its outcome
  (Different from the plan, 9).
- **Words:**
  the API documents read as if `integration` were always among an integration's
  user's roles, and said nothing of what `may.roles` tells its caller;
  `admin/index.tsx` named My profile alone as `users.load`'s caller;
  `docs/ui.md` cited D42 on the sentence D46 takes back;
  three citations of this plan lacked their comma;
  the German and Dutch line did not agree with the user word.
- **The guard's comment** said more than the guard holds (Different from the
  plan, 10).
- **The open server's line** said "No users yet" until the read answered
  (Different from the plan, 8).

One finding is not the block's to build: any `guests` holder may disable,
rename and remove an integration's user stored as guest, older than the block;
it went to the owner as D41 (For the completion gate, 2).

The page's reader read the first rework:
both musts fixed, the read not awaited unable to write after a sign-out,
`roles` never null, and the new wait in
`test_create_from_users_shows_the_owner` hiding no fault.
Two shoulds, done in a second rework:

- **Two changes in Integrations in a row could leave Users with the older
  list:**
  a refusal's read held, a removal's answered,
  then the older answer let go, and the head counted one user too few.
  `loadUsers` numbers its reads now, and a test holds it.
- **An open server whose users' read failed showed no line:**
  the head, the error line and Create the owner…, and the line never came.
  It comes at a failed read too now, and a test holds it.

Nobody read the second rework but the builder;
its two tests failed on the tree before it, each for the reason it names.

## Seen and left

- **D41,** the owner's (For the completion gate, 2).
- **`listUsers`' three reads are not in one transaction,**
  so `may.roles` can be stale for a moment;
  it is what the page offers, and the change checks again.
  As it is.
- **The builder's first race run after the first rework exited 1,**
  its output not kept, so the failing test is unknown;
  the other race runs of `internal/server` that reached its tests passed,
  five in all, the coordinating session's two on that tree among them.
  Nobody's yet; CI's Race job runs it on the pull request.
- **One UI test errored once in the whole suite after the first rework,**
  `test_log.py`'s `test_log_of_every_cube_loads_again_after_409`,
  a setup timeout waiting for a room;
  it passed on its retry and alone.
  Not this block's file; nobody's yet.
- **`may.roles` on an open server and for a revoked integration's users**
  were run by hand by the server's reader, both right; no test holds them.
  The next change of `users_api_test.go`'s.
- **An Integrations change now also reads the users and the providers again,**
  and every open user panel's sessions, keys and identities,
  after its outcome.
  As it is.
- **The head's badge wraps under the title at 390 px** in every language but
  English, not clipped.
  As it is.
- **`docs/ui.md` has two line-break warnings more, `docs/api-v1.md` one,**
  0 errors.
  As it is.

## For the completion gate

1. **What a user notices:**
   an integration's user's Role list offers only `integration`
   and the roles within its integration's maximum, never owner;
   a role beyond it stays chosen, with a line under it, until another is saved;
   `integration` can be given back; Add a user no longer offers it.
   On a server without an owner, the Users head says No owner yet,
   and an integration's users are listed with nothing to press,
   with Open Integrations under them.
   After a change in Integrations, Users shows its users at once.
2. **D41 is with the owner:**
   any `guests` holder, another integration's key included,
   may disable, rename and remove an integration's user stored as guest,
   older than the block;
   with the plan's maximum the Role list now offers guest as the one role beside
   `integration`, so it steers an owner who narrows such a user toward that.
   The owner has asked what A3 means:
   calls through an integration that manage only that integration's users.
   Nothing of D41 is in this block.
3. **What `may.roles` tells its caller:**
   a holder of `users` without `roles` or `integrations` learns the keys of the
   roles it outranks, unused and disabled ones included, and,
   for each integration's user,
   which of them lie within that integration's maximum, so part of the maximum,
   which `GET /api/v1/roles` and `GET /api/v1/integrations` would refuse it;
   a holder of `guests` learns, per integration's user stored as guest,
   whether guest lies within its maximum.
   D3-A has the server compute it; the API documents now say what it tells.
4. **`may.roles` is always sent,** an empty list where none may be given,
   against the plan's `omitempty` (Different from the plan, 1);
   the API's `may` gains a required field.
5. **LooseEnds.D42-A is taken back in part by D46-A:**
   Users lists an integration's users on an open server,
   where D42-A said it lists none;
   the first owner's card keeps its title, and "No users yet" stays where the
   server has no users at all.
6. **D10's reach is widened** to every change in Integrations,
   its outcome not waiting for the users' read (Different from the plan, 9).
7. **The open mode lets anyone on the local network change an integration's
   users,** the plan's "What stays as it is": the rows offer nothing to press,
   but the server takes each change,
   and giving `integration` back is new there too.

## What verified it

- **The builder, on the first build,** each by its exit code, 0,
  on its last tree: `make ui`,
  `gofmt -l .` printing nothing, `go vet` for linux, windows and darwin,
  `go run ./tools/apitypes -check`, `make ui-check`, `go test -count=1 ./...`,
  `MASSIMO_TEST_SCALE=2 go test -race ./internal/server` and `make cross`;
  on its first tree alone, the three vets,
  `apitypes -check` and the tests of the server, the store and the command line;
  `reflow.sh --check`, 0 errors; `commit-msg.sh` on both messages.
  UI: the plan's six files in all four cases, 434 passed and 4 failed,
  the new test's console errors, which a stub of the keys' reads ended,
  `test_users.py` then 84 passed;
  the first tree's `test_users.py` and `test_integrations.py`, 116 passed;
  the whole suite in 1100-en, 816 passed.
  The new Go test and the four new UI tests failed on the base,
  each for the reason it names.
  The grep for scripts that make users or set roles, in `packaging/`, `tools/`,
  `app/` and `.github/`: none gives the role `integration`,
  and none reads `may`.
- **The server's reader, on the first build's trees:**
  `MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`,
  exit 0 (387 s); `go test` of `internal/store`, `cmd/...`,
  `tools/apitypes` and `tools/identifiers`, exit 0; `go vet ./internal/server`,
  exit 0; `gofmt`, nothing listed;
  two probe tests of its own over the store's states,
  the open server and four changes at once under `-race`; four planted faults,
  three caught and the ceiling's not.
- **The page's reader, on the first build's trees:**
  `make ui` and `make ui-check`, exit 0; the server's tests of the users,
  the rank rule, the OpenAPI file and the UI, ok on both trees; `test_users.py`,
  `test_ways.py`,
  `test_integrations.py` and `test_parts_guard.py` in all four cases,
  282 passed; the four new tests on the base in 1100-en,
  each failing for the reason it names;
  a probe of its own in 1100-en and 390-de for the two musts; the look,
  the six languages, the keyboard and the labels by hand;
  `commit-msg.sh` and `reflow.sh --check`, 0 errors.
- **The builder, after the first rework,** each by its exit code, 0:
  `make ui`, `make ui-check`, `gofmt`, the three vets, `apitypes -check`,
  `go test -count=1` of `internal/server`, `internal/store` and `cmd/massimo`,
  `make cross`, `reflow.sh --check` on `docs/ui.md` and `docs/api-v1.md`,
  `commit-msg.sh` on both;
  the race run of `internal/server` once exit 1 (Seen and left), once exit 0;
  on the first tree alone the three vets, `apitypes -check`,
  the tests of the server, the store and the command line,
  `make ui` and `make ui-check`, each 0,
  and `test_users.py` and `test_integrations.py` in all four cases,
  124 passed.
  UI: the plan's six files in all four cases, 454 passed;
  the whole suite in 1100-en, 819 passed and 1 error (Seen and left).
  Each new test failed without its change, on the tree before,
  on the base or on a planted fault (the ceiling dropped, `roles` left out,
  `principalFor`'s cap removed, the head's badge kept).
- **The coordinating session, on the first rework's tree:**
  `MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server` twice,
  exit 0, 586 s and 446 s, no data race,
  after a first pair that failed at its setup, the export having no UI bundle
  before `make ui`.
- **The page's reader, on the first rework's trees:**
  the four files in all four cases, 298 passed;
  the six new tests on the tree before in 1100-en,
  three failing for their reasons, three passing as they should;
  its probe of the overlapping reads and the failed read in 1100-en and 390-de;
  `make ui`, `make ui-check` and `apitypes -check` on both trees, 0;
  the server's tests as before, ok.
- **The builder, after the second rework,** each by its exit code, 0: `make ui`,
  `make ui-check`, `reflow.sh --check docs/ui.md` and `commit-msg.sh`;
  `test_users.py`, `test_ways.py`,
  `test_integrations.py` and `test_parts_guard.py` in all four cases,
  306 passed; the two new tests failing on the tree before.
  No Go changed, so no vet or race run.
- **The coordinating session, on the commits:**
  the two steps carried from the plan's commit, "chore(aps): +AccessEnds v1.0",
  onto main's head,
  "docs(api): The capability tables name every route the server guards",
  by a three-way merge of trees without conflict;
  main had meanwhile taken AP ToldData,
  which rewrote how the UI tests put data into the page.
  In a clone with each tree: `commit-msg.sh`, 0 errors and 0 warnings;
  `changelog.sh check --staged --strict`, 0 for each step;
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well
  (line breaks 0 errors).
  The whole UI suite in 1100-en on the committed tree, after `make ui`:
  838 passed, exit 0.
- **Not run here:**
  `make app-emulator-test` and `make lan-test`, which nothing here touches;
  the whole `go test ./...` on the committed tree,
  which is the pull request's CI, its result not known as this is written.
