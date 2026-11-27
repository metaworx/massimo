# NOTE LinkedRemoval v1.0: how block 1 went

Where Block 1, Removal, went differently from the plan,
the choices taken in the run, what the two readings changed,
among them an open event stream that outlived the link,
and what verified it.

## What it made

Removing an integration's user ends its link to a Massimo user,
as the owner decided at the completion gate of AP AccessEnds (D54-A),
in a store of this version and in an older one (D1-A, D2-A):

- **The removal** of an integration's user removes its identity too,
  linked or not, as a tombstone of this node one version on (`DeleteUser`,
  by `real_user_id` beside `effective_user_id`).
  Its next call makes it anew, unlinked,
  with the role `integration` within the integration's maximum,
  where its calls went on acting as the linked user (`200` on the base).
- **An identity whose own user was removed before,**
  as an older store holds one,
  acts for nobody: no list shows it, no link takes it,
  and the next call removes it and makes it anew,
  logged once, without a migration (D1-A).
- **The linked Massimo user removed,** the integration's next call for that user
  takes its own user again, unlinked,
  with the role and state the owner left it (D2-A),
  where every call failed with a `500` on the name the live user still held,
  until an owner removed it.
- **An event stream** opened through the integration for one of its users
  ends at its next keep-alive once the link ends,
  where it went on carrying what was sent to the linked user.
- **One commit,** on `maint/0.17`:
  "fix(security:store):
  Removing an integration's user ends its link to a Massimo user".

## Different from the plan

Against the plan's block, item by item; each the owner's to change:

1. **Carried onto `maint/0.17`'s head:**
   the builder built on the plan's v1.0 commit;
   the coordinating session carried the step onto "merge:
   A Cube's problems and the log's alerts go by the battery level (#479)" by a
   three-way merge of trees without conflict,
   across ErrorCodes' Block 1 in `auth.go`, `onbehalf.go` and `docs/api-v1.md`,
   and SteadyTests' Blocks 12 to 14 in `docs/testing.md`.
   The pull request's base is `maint/0.17`,
   and this NOTE goes there too (AP MaintBranch, D5-A).
2. **`onbehalf.go` changed** (item 3 said nothing,
   unless it must refuse a removed own user): it need not,
   since `IntegrationUser` returns only an identity whose two users are live,
   read in its one transaction;
   a removal landing after that read is a request already in flight,
   as a key revoked mid-request is.
   It changed for the stream instead (3).
3. **An open event stream ends with the link** (from the first reading's must;
   not in the plan): `stillAdmitted` asks `stillActsFor`,
   at each keep-alive of a stream through an integration for one of its users,
   whether the key's integration is still the stream's and active,
   the identity still live (`IntegrationIdentityByID`, which writes nothing),
   acting as the same user for the same own user,
   and the chain at the stream's level still holding `read` (`actingFor`,
   the tail of `integrationPrincipal` split out, which writes nothing).
   The coordinating session's choice:
   the same check ends the stream at an unlink, a new link and a disabling too,
   where the plan named only the removal;
   `docs/api-v1.md` says so in "Acting for an integration's users" and in the
   stream's keep-alive paragraph.
4. **`withinIntegration`,** one helper for links 2 and 4 of the chain,
   the ceiling and the maximum (the coordinating session's choice,
   from the second reading): a request's principal,
   an integration's user's key (`principalFor`) and the stream's re-check each
   wrote them out,
   so that a later change to the chain could have reached requests but
   not streams; behaviour unchanged.
5. **An identity whose linked user was removed** is treated as D1-A's too,
   as a guard (the plan named the own user alone):
   no version left one;
   only a link racing the removal of its target could, which 6 closes.
6. **A link to a removed user** (from the first reading; not in the plan):
   `LinkIntegrationUser` reads the live identity and its target in the
   transaction that writes the link and refuses a removed or missing target,
   `ErrNoUser`, which the route answers `404` "no user …";
   on the first tree the route answered `200` `"linked": true`
   for a link the store then hid.
7. **What a client sees of an older store's identity** before its next call
   (not in the plan's two functions):
   `IntegrationIdentities`, `AllIntegrationIdentities` and
   `LinkIntegrationUser` leave out an identity whose own or linked user was
   removed (`liveIntegrationIdentity`),
   so the Integrations card lists it no more,
   the linked user's links in the users list leave it out,
   no user is shown acting for it, and the link route answers `404`.
8. **The empty id refused,** `ErrBadName` where it was `ErrUserExists`:
   without it the reuse of D2-A could have taken `<uuid>:`,
   the integration's own user that holds its key;
   `parseOnBehalf` already answers `400` to an empty id.
   **The integration's own and system user are kept out of the reuse by id**
   (from the first reading):
   on the first tree a store call with the subject `SYSTEM`,
   the system identity removed, took `<uuid>:system`;
   only a call to the store reaches that, since `parseOnBehalf` lower-cases.
9. **Documents beyond the plan's two:**
   the OpenAPI file's `DELETE /api/v1/users/{name}` says the link ends;
   `docs/api-v1.md` no longer says a removed user's row is renamed
   `<name>-del-…`, which migration 0024 ended (`docs/store.md` said so already);
   `docs/testing.md` names `storekit.Raw`.
10. **A Fixed bullet beside the plan's Security one** (from the second reader):
    "An integration's user whose linked Massimo user was removed acts as itself
    again, where its calls failed."; the Security bullet as the plan gives it.
11. **Helpers, not copies** (from the second reader): `storekit.Raw`,
    a second connection past the store, in the test kit,
    which the server's four raw opens and serve's one now call; `sendFromWith`,
    which `meCaps` calls; `openStreamWith`, which `openStream` calls.
    So the block touches tests not its own: `commands_test.go`,
    `integration_user_test.go` and serve's `setup_test.go`.
12. **Tests the plan did not name:**
    `TestLinkToARemovedUser`; `TestStreamEndsWithTheLink` (removed,
    unlinked and disabled, another user's stream going on);
    in `TestRemovalEndsTheLink`'s older store,
    what a client sees before the next call; in the store's tests,
    the log line once, the tombstone's fields from another node's row,
    and the system user by an upper-case id.
    `TestLinkedUserRemovedActsAsItsOwn` makes the owner's changes through
    `PUT /api/v1/users/{name}`.

## Choices the plan left open

- **The user taken back as the owner left it**
  (D2-A): its role, disabled flag, end date, display name and version unchanged;
  a stored role beyond the maximum acts within it, the chain unchanged,
  and `may.beyond` stays true; a disabled one stays refused, `user_disabled`.
  The other way: its role set back to `integration` when it is taken back.
- **What the reuse takes:**
  only a live user of that name of the same integration that no live identity
  has as its own; never one of no integration or of another,
  a removed one, or one another identity holds: the name is held,
  `ErrUserExists`, as before.
  The other way: a user of that name of the same integration whatever holds it.
- **The orphan logged once, at Info:**
  "its call took it as a first call",
  words that hold where D2-A takes the user back as where it is made anew.
  The other way: no line, or a Warn.
- **The first read and the last one in `IntegrationUser` filter on
  `deleted_at` alone,**
  since the UPDATE before them removed every other identity of the subject
  in the same transaction, said in a comment;
  no test could tell the fuller filter apart (the second reader's mutation).
  The other way: `liveIntegrationIdentity` there too.
- **The stream asks at the level it was opened at,**
  which the identity's last level holds at the stream's opening,
  and writes no time of use or level at a keep-alive.
  The other way: the level the identity holds now,
  which a later call at another level changes.

## What the readings found

One builder built the block in an exported copy of the plan's v1.0 commit;
two readers read the first tree,
one the store, the server and its security, the other the tests,
the documents and the message.

Confirmed in the first reading:
the removal's SQL is right in every state tried (linked, unlinked,
either user removed, either disabled or past its end,
revoked and attached again, two subjects, two integrations);
sixteen concurrent first calls made one identity and no error,
and twenty rounds of a removal racing a call no error, the next call unlinked;
the UPDATE of an orphan at every call through an integration adds no cost
or race inside the transaction that already holds the write lock;
provider identities have no `real_user_id`,
so a regular user's removal takes nothing more;
the empty id and the guard of 5 above are right for every caller;
each new test fails on the base for the reason it names,
`200` as the linked user and `500` on the name;
12 of 14 mutations of the code were caught; the `-del-` correction is right,
and the documents agree with each other;
no page text or translation speaks of a linked user's removal.

One must, done in the rework:

- **An open event stream outlived the end of the link,**
  older than the block, and the same at an unlink and a disabling:
  the server's reader's probe opened a stream for a linked integration's user,
  removed that user, and five keep-alives later the stream was still open
  and carried a notification sent to the linked user (3 above).

Shoulds, done in the rework:
the reuse kept out the integration's own users by id (8 above);
a link to a removed user refused (6 above);
where an orphan comes from, in the comment of `IntegrationUser`,
`docs/store.md` and the store's test comments:
only the own user's case is an older store's;
what a client sees of an orphan, tested through the users list,
the Integrations card and the link route;
the first read's filter (Choices);
the log line tested once, through the test kit's log watch;
the tombstone's origin checked from another node's row,
where every row already carried this node's;
the helpers of 11 above; the owner's changes through the users route;
the upper-case case's comment, that only the store reaches it;
the Fixed bullet; and the message, which names the route,
says what a client sees and covers the stream.
Kept for the completion gate, not changed:
a push subscription made through the integration (1 there),
and a remade user's role (2 there).

The second reading, the server's reader's of the rework, found no must;
the stream, the reuse by id and the link held:

- `integrationPrincipal` refuses as before,
  with the same codes in the same order; `actingFor` is its old tail,
  reading only; the stream's capabilities are rebuilt by the same expression,
  and the re-check returns only yes or no, so it never widens a stream.
- The stream ends at the integration's user's removal, disabling or unlink,
  a link of an unlinked user, the linked user removed, disabled or past its end,
  its role disabled, a maximum without `read`,
  the integration disabled and a revoke; a ticket's stream at the removal too.
  The owner's stream, a plain key's and the integration's own without the header
  go on.
- `TestStreamEndsWithTheLink` fails in all three cases with the re-check cut
  out, timing out on the stream's end.

Two shoulds: links 2 and 4 written out three times,
made one helper in a last rework (4 above); and an older point,
not about the link, kept for the completion gate (3 there).
The tests' reader did not read the rework,
and no reader read the last rework, whose behaviour is unchanged.

## Seen and left

- **The route's `404` for a target removed since its lookup** has a store test
  and no route test: only a race reaches it (For the completion gate, 4).
  The next change of `putIntegrationUser`'s.
- **An older store's orphan cannot show in `Namer.Current`:**
  its own user is removed and so not listed; the filter there has no test.
  Nothing to do.
- **A role narrowed but still holding `read`** keeps the audit flag the
  stream opened with, as `stillAdmitted` does for every stream:
  the same for all streams, the server's reader's word, not a gap.
- **The unsteady tests the runs met,** none the block's:
  `TestProxyRetriedUntilTheAddressIsThere`,
  in the builder's first race run beside `make cross`,
  passing alone 3 of 3 under `-race`; AP SteadyTests names it, and its fix,
  "test(server):
  The proxy's retry test waits for tries made after its error changed",
  is on `maint/0.17`'s head, under the carried commit.
  `test_the_session_end_forgets_system_info`,
  `test_leaving_with_a_changed_form_asks`
  and `test_users_on_an_open_server_list_an_integrations_users` (on a stubbed
  users list),
  in the builder's whole UI run after the rework at a load of about 60,
  which the 1700 s timeout cut at about 58 %,
  each passing alone and in the whole run again; no plan names them:
  AP SteadyTests' if they come back.

## For the completion gate

1. **A push subscription made through the integration for the linked user stays
   deliverable after the link ends,** at a removal, an unlink or a disabling:
   older than the block;
   the linked user sees it in its own list and can remove it.
   Keeping it from outliving the link needs the subscription to record the
   identity it was made for: a migration, the owner's word.
2. **A remade user may hold more than the linked user's role allowed,** up to
   `integration` within the maximum, where that role was narrower:
   D54-A as decided.
   To stop its calls, the owner disables the user.
3. **Accepting a new installation id leaves the old installation's open stream
   running:**
   its calls get `403` `instance_pending`, but its stream for a user goes on
   and carries what is sent to that user;
   the re-check does not compare the stream's installation.
   Older than the block, and not about the link.
4. **The route's `404` for a target removed between its lookup and the write**
   is held by a store test (`TestLinkToARemovedUser`), not a route test:
   only a race reaches it.
5. **What a user notices:**
   removing a linked integration's user ends its link,
   and its next call is a first call's, unlinked;
   removing the linked Massimo user lets the integration's user act as itself
   again, where its calls failed;
   an identity an older store left behind is listed no more,
   and its next call logs one Info line;
   a stream through the integration ends with the link, at a removal, an unlink,
   a new link or a disabling.

## What verified it

- **The builder, on the first tree:**
  what each case did on the base, by the new tests before the change:
  a linked integration's user removed, its next call `200` as the linked user;
  the same in an older store, the identity still listed and linkable;
  the linked user removed, `500` "store: user exists" for good,
  the integration's user disabled too;
  an unlinked one made anew, unchanged.
  `make ui`, `gofmt -l .` printing nothing, `go vet` for linux,
  windows and darwin,
  `MASSIMO_ENV_FILE=/dev/null go test -count=1` of `internal/store`,
  `internal/server`, `cmd/massimo`, `internal/push`, `internal/serve`,
  `internal/testkit/storekit` and `tools/identifiers`, each exit 0;
  the race run of `internal/store` and `internal/server` at
  `MASSIMO_TEST_SCALE=2`,
  exit 1 on the proxy's retry test alone (Seen and left), the store ok (501 s),
  and the server's race run again without load, exit 0 (483 s); `make cross`,
  exit 0; `reflow.sh --check` on both documents, 0 errors;
  the whole UI suite in 1100-en, 875 passed.
  The five new tests seen failing on the base, each for its reason,
  the unlinked cases passing as guards.
  No script or test in `packaging/`, `tools/`,
  `app/` or `.github/` removes an integration's user or relies on a linked one
  outliving its removal;
  in 0.17 there is no sync of rows between nodes and no restore of the server's
  store,
  and a store taken back to 23 loses the removed identities (`docs/store.md`,
  L2).
- **The server's reader, first:**
  probes on the base and the first tree (the stream, the push subscription,
  the system user by an upper-case id, a link to a removed user),
  sixteen concurrent first calls and twenty rounds of a removal racing a call;
  `go test` of `internal/store` and the block's server tests pass,
  `gofmt` and `go vet` clean on both packages.
- **The tests' reader:**
  each new test copied onto the base, failing for the reason it names,
  the unlinked cases passing; on the tree all five pass,
  and again under `-race -count=5`;
  14 mutations of the code, 12 caught, 11 by the store's tests and 3 of them
  by the server's too, the log line and the first read's filter not;
  a probe of an older store's identity through three routes;
  `reflow.sh --check`, 0 errors on `docs/api-v1.md` and `docs/store.md`,
  their warnings as on the base (187 and 80), and on the fragment;
  `commit-msg.sh` in a scratch clone at the base, 0 errors;
  `gofmt`, `go vet` for three platforms on both packages,
  and `tools/identifiers` ok.
- **The builder, after the rework:**
  each new or changed test seen failing without its change (the stream's test,
  timing out in all three cases; the reuse's ids; the link to a removed user;
  what a client sees, with each of three filters dropped; the log line;
  the origin); `make ui`, `gofmt`, `go vet` for three platforms,
  `go test -count=1` of 8 packages (the first tree's and `internal/testkit`),
  the race run at scale 2, run alone,
  of `internal/store` (645 s) and `internal/server` (1093 s), `make cross`,
  each exit 0; `reflow.sh --check` on `docs/api-v1.md`,
  `docs/store.md` and `docs/testing.md`, 0 errors and no new warnings;
  `commit-msg.sh` in a scratch clone at the base, 0.
  UI: the whole suite in 1100-en,
  cut by the timeout with three failures (Seen and left),
  the three alone 4 passed, and the whole suite again, 875 passed in 29 minutes.
- **The server's reader, second:**
  probes of the stream's end and of a new installation id;
  `make ui`, `gofmt`, `go vet` of `internal/store`, `internal/server`,
  `internal/serve` and `internal/testkit/...`,
  `go test -count=1` of `internal/store` and `internal/testkit/...`,
  of `internal/server` for the tests of integrations, on-behalf calls,
  streams, removals, links, tickets and keep-alives,
  and of serve's `TestServePlainSeedCutShort`, each exit 0.
- **The builder, after the last rework** (`withinIntegration`):
  `go test -count=1` of `internal/store`, `internal/server` and `cmd/massimo`,
  the race run of `internal/server` at scale 2 (586 s) and `make cross`,
  each exit 0, and so its other checks; no UI suite.
- **The coordinating session:**
  the step carried onto `maint/0.17`'s head without conflict;
  in a clone with the commit `commit-msg.sh`, 0 errors and 0 warnings,
  `changelog.sh check --staged --strict`, 0,
  and `GUIDELINES/shared/tools/check` over the range, twelve checks all well;
  on the committed tree, after `make ui`, `gofmt -l .` printing nothing,
  `go vet` printing nothing, and `go test -count=1` of `internal/store`,
  `internal/server` and `cmd/massimo`, ok, exit 0.
- **Not run here:**
  the whole `go test ./...` on the last tree, which is the pull request's CI;
  `GUIDELINES/shared/tools/check` by the builder, in a copy.
