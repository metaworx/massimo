# AP LinkedRemoval v1.1: Removal ends the link

## TL;DR

Removing an integration's user that is linked to a Massimo user ends the link,
where its calls went on acting as that user (AccessEnds.D54-A);
and an event stream of an integration's old installation ends once a new one is
accepted (D4-A); for 0.17, on `maint/0.17`.

- removal → the integration user's identity goes with it;
  its next call makes it anew, unlinked, role `integration`, within the maximum
- an identity whose own user was removed before (an older store) →
  made anew at its next call, not acted on
- the linked Massimo user removed → the integration's next call finds its own
  user again, unlinked, where it failed on the name
- a stream through an integration → ends with the link (Block 1, merged)
- a new installation id accepted → the old installation's stream ends at its
  next keep-alive, where it went on
- a push subscription made through the integration → next version (D3-A)

### Blocks

1. Removal: an integration user's removal ends its link, in every store (merged,
   #483)
2. Installation: accepting a new installation ends the old one's streams

## Discussion

### What is decided

The owner, 2026-11-27, at the completion gate of AP AccessEnds:
"PLAN++ D54-A", after asking whether it needs a plan of its own.

- **AccessEnds.D54-A.
  Removing an integration's user also removes its identity,
  so that its calls stop acting as the Massimo user it was linked to.**
  The integration makes the user anew at its next call,
  as it does today for one that was not linked:
  so "stops its calls" means that they no longer act as the linked user;
  the integration's user comes back with the role `integration`,
  held to the integration's maximum.
  Not chosen: leaving it as `docs/api-v1.md` says it now ("disable it,
  or unlink it").

### Taken with PLAN++

- **D1-A.
  An identity whose own user was removed before this plan**
  (a store where a linked integration's user was removed):
  - **A (taken).**
    The call that finds it removes it and makes the identity and its user anew,
    as for a first call, so that an older store is right without a migration.
  - **B.**
    A migration that removes such identities once.
  - **C.**
    The call refused (`403`) until an owner acts.
- **D2-A.
  The linked Massimo user removed.**
  Its removal removes the identities that act as it, the integration's ones
  among them (as today); the integration's next call then makes the identity
  anew and its user, whose name `<uuid>:<subject>` is still held by the
  integration's live user.
  - **A (taken).**
    The new identity takes that live user of the same integration again,
    unlinked.
  - **B.**
    The integration's user is removed with the link, and made anew.

### Decided at the completion gate of 2026-11-27

The owner, at the completion gate of Block 1: "PLAN++ D3-A D4-A".
Block 1's own account is the NOTE "how block 1 went";
its reading found both points, each older than the plan.

- **D3-A.
  A push subscription made through the integration for the linked user** still
  gets messages after the link ends (and after an unlink or a disabling);
  the linked user sees it in their own list and can delete it.
  - **A (taken).**
    The next version, as a roadmap item written at this plan's retirement:
    the subscription records the integration's user it was made for,
    so that its removal or unlink takes it.
    It needs a store migration, kept out of 0.17.
  - **B.**
    Left as it is.
- **D4-A.
  Accepting a new installation id leaves the old installation's open stream
  running**, where its calls are refused (`instance_pending`).
  - **A (taken).**
    Block 2 of this plan, for 0.17:
    the stream keeps the installation id it was opened with and ends at its next
    keep-alive once another is accepted; small, no migration.
  - **B.**
    The next version, as a roadmap item.

### What stays as it is

- Disabling an integration's user stops its calls, a linked one's too.
- The integration's own users (`<uuid>:`, `<uuid>:system`) cannot be removed
  through the users routes (AccessEnds' Block 9).
- The integration's own key without the header carries no installation id,
  and its calls and its stream are not held to one (Block 2 neither).
- Any retirement, a release.

## Analysis

### For Block 1: what removal does today

Read on `maint/0.17` at `chore(aps): +ErrorCodes v1.0`, 2026-11-27.

- **The removal** (`internal/store/users.go`, `DeleteUser`):
  the user's row, its keys, its preferences and its push subscriptions go;
  its identities go by `effective_user_id = <the user>`.
  A linked integration user's identity has the linked Massimo user as its
  `effective_user_id` and the integration's user as its `real_user_id`
  (`internal/store/integrations.go`, `IntegrationIdentity`,
  `LinkIntegrationUser`), so it stays.
- **The next call** (`internal/server/onbehalf.go`, the chain;
  `IntegrationUser`):
  the identity is found by integration and subject among those not removed;
  `UserByID` reads the integration's user by its id without asking whether it
  was removed, so the removed row passes the check of its state,
  and the call goes on as the linked user (the AccessEnds reading's probe:
  200 as the owner after the removal).
- **An unlinked one** is removed with its identity (its `effective_user_id` is
  its own), so its next call makes both anew (`integrationUserTx`, `addUserTx`):
  the name of a removed user can be taken again.
- **The linked Massimo user removed:**
  the integration's identities that act as it go with it;
  the next call makes the identity anew and tries to make `<uuid>:<subject>`
  again, which the integration's live user still holds:
  the block finds what that does today and makes it right (D2-A).

### For Block 2: an installation's stream

Read on `maint/0.17` at `merge(LinkedRemoval/Block-1): …`, 2026-11-27.

- **The installation id** comes with every call through an integration for one
  of its users, in the header's `instance` (`parseOnBehalf`);
  a call whose id is not the accepted one is refused, 403 `instance_pending`,
  and the id recorded as `pending_instance` (`integrationPrincipal`,
  `NotePendingInstance`);
  `POST /api/v1/integrations/{uuid}/accept-instance` makes it the accepted one
  (`acceptInstance`, `AcceptInstance`).
- **An open stream** asks at each keep-alive whether its caller would be let in
  now (`stillAdmitted`);
  for a call through an integration for one of its users it asks the chain again
  since Block 1 (`stillActsFor`): the integration, the identity,
  both users and the chain at the stream's level, but not the installation id,
  which the principal does not keep.
  So the old installation's stream goes on after a new one is accepted,
  carrying what is sent to the user it acts as (Block 1's second reading,
  a probe).
- **A ticket's stream** takes the principal of the call that asked for the
  ticket, so it carries that call's installation id too.

## Implementation Plan

### Block 1: Removal

Merged into `maint/0.17` (#483) and merged up into main;
its account is the NOTE "how block 1 went".

- **Goal:**
  an integration's user that is removed no longer acts as the Massimo user it
  was linked to, in a new store and an older one (AccessEnds.D54-A, D1-A,
  D2-A).
- **Changes:**
  1. `internal/store/users.go`, `DeleteUser`:
     the identities whose `real_user_id` is the removed user go too.
  2. `internal/store/integrations.go`, `IntegrationUser`:
     an identity found whose own user was removed is removed and the call
     made as a first call (D1-A);
     a first call whose name `<uuid>:<subject>` is held by the integration's
     live user of the same integration takes that user again (D2-A).
  3. `internal/server/onbehalf.go`:
     nothing, unless reading shows it must refuse a removed own user too;
     the block says what it found.
- **Tests:**
  `internal/store`: a linked integration user removed, then its identity gone
  and the next `IntegrationUser` making it anew, unlinked, role `integration`;
  an identity whose own user was removed before (written as an older store
  has it) made anew at the next call; the linked Massimo user removed, then
  the next call taking the integration's live user again.
  `internal/server`: through the API, a call for a linked user,
  the integration user removed by the owner,
  the next call acting as the integration's user (its role, its maximum),
  no longer as the linked user; the same with the linked Massimo user removed.
  Each seen failing before the change.
- **Documents:**
  `docs/api-v1.md` (the users acting for an integration:
  "removing one ends its link: its next call makes it anew, unlinked"),
  `docs/store.md` (an integration's users and their identities).
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-linkedremoval-removal-ends-the-link.md`:
  "- Removing an integration's user that acts as a Massimo user ends that link,
  where its calls went on as that user."
- **Commit:**
  `fix(security:store): Removing an integration's user ends its link to a Massimo user`
- **Verification,** each by its exit code:
  `gofmt -l .` printing nothing;
  `go vet ./...` for linux, windows and darwin;
  `MASSIMO_ENV_FILE=/dev/null go test -count=1` of `internal/store`,
  `internal/server` and `cmd/massimo`;
  `MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/store ./internal/server`;
  `make cross`;
  `reflow.sh --check` on both documents;
  `changelog.sh check --staged --strict`.
  No page changes, so no UI test is the block's; CI runs the suite.

### Block 2: Installation

- **Goal:**
  an event stream opened through an integration for one of its users from an
  installation id that is no longer the accepted one ends at its next keep-alive
  (D4-A), as its calls are refused.
- **Changes:**
  1. `internal/server/onbehalf.go`:
     the principal of a call through an integration for one of its users keeps
     the installation id the call came with;
     `stillActsFor` ends the stream where it is not the integration's accepted
     id now.
     It writes nothing: no `pending_instance` recorded at a keep-alive.
  2. Nothing for the integration's own key without the header,
     which carries no installation id.
- **Tests:**
  `internal/server`: a stream opened for a user through the integration;
  a call from another installation (its id recorded),
  the owner's `POST …/accept-instance`; the old installation's stream ends,
  waited for as a signal, never a sleep;
  a stream of the newly accepted installation and the integration's own stream
  without the header go on;
  a ticket's stream from the old installation ends too.
  Each seen failing before the change.
- **Documents:**
  `docs/api-v1.md` (the stream's keep-alive, "GET /api/v1/cubes/{sn}/events";
  the installation id, where it says what `POST …/accept-instance` does;
  "Acting for an integration's users", where it says when an event stream ends),
  `docs/api-v1-openapi.yaml` (`POST …/accept-instance`).
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-linkedremoval-old-installation-stream.md`:
  "- Accepting an integration's new installation ends an event stream that
  the old one still held open."
- **Commit:**
  `fix(security:server): Accepting an integration's new installation ends the old one's streams`
- **Verification,** each by its exit code: `gofmt -l .` printing nothing;
  `go vet ./...` for linux, windows and darwin;
  `MASSIMO_ENV_FILE=/dev/null go test -count=1` of `internal/server`
  and `cmd/massimo`;
  `MASSIMO_TEST_SCALE=2 go test -race -count=1 ./internal/server`; `make cross`;
  `reflow.sh --check` on `docs/api-v1.md`;
  `changelog.sh check --staged --strict`;
  the whole UI suite once in `1100-en` (no page changes;
  the stream is the page's too).

## Proposed commit message

The plan: `chore(aps): +LinkedRemoval v1.0`,
then `chore(aps): LinkedRemoval v1.1`

Block 1:

```
fix(security:store): Removing an integration's user ends its link to a Massimo user

AP LinkedRemoval v1.0, Block 1 (AccessEnds.D54-A, D1-A, D2-A): removing an integration's user that is linked to a Massimo user removes its identity too, where its calls went on acting as that user; its next call makes it anew, unlinked, with the role integration and within the integration's maximum. An identity whose own user was removed before is made anew at its next call, so that an older store is right without a migration, and the linked Massimo user removed leaves the integration's next call to take its own user again.

Key changes:
- internal/store: DeleteUser removes the identities whose own user it is; IntegrationUser makes anew an identity whose own user was removed, and takes the integration's live user of that name again
- Tests: the store's removal, an older store's identity and the linked user removed; through the API, the call after each removal
- Docs: docs/api-v1.md, docs/store.md; a Security fragment

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
fix(security:server): Accepting an integration's new installation ends the old one's streams

AP LinkedRemoval v1.1, Block 2 (D4-A): an event stream opened through an integration for one of its users keeps the installation id it was opened with, and ends at its next keep-alive once the owner accepts another, where it went on carrying what was sent to the user it acts as while the old installation's calls were refused.

Key changes:
- internal/server/onbehalf.go: the principal keeps the call's installation id; stillActsFor ends a stream whose id is no longer the accepted one
- Tests: the old installation's stream and its ticket's stream end at the acceptance; the new installation's and the integration's own stream go on
- Docs: docs/api-v1.md, docs/api-v1-openapi.yaml; a Security fragment

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-27): first revision, from the owner's "PLAN++ D54-A"
  at the completion gate of AP AccessEnds; D1-A and D2-A taken with PLAN++.
- v1.1 (2026-11-27): Block 2, an installation's stream (D4-A), and D3-A,
  a push subscription made through an integration, for the next version;
  from the owner's "PLAN++ D3-A D4-A" at the completion gate of Block 1,
  which is merged (#483).
