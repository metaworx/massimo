# AP LinkedRemoval v1.0: Removal ends the link

## TL;DR

Removing an integration's user that is linked to a Massimo user ends the link,
where its calls went on acting as that user (AccessEnds.D54-A); for 0.17,
on `maint/0.17`.

- removal → the integration user's identity goes with it;
  its next call makes it anew, unlinked, role `integration`, within the maximum
- an identity whose own user was removed before (an older store) →
  made anew at its next call, not acted on
- the linked Massimo user removed → the integration's next call finds its own
  user again, unlinked, where it failed on the name
- documents → `docs/api-v1.md`, `docs/store.md`

### Blocks

1. Removal: an integration user's removal ends its link, in every store

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

### What stays as it is

- Disabling an integration's user stops its calls, a linked one's too.
- The integration's own users (`<uuid>:`, `<uuid>:system`) cannot be removed
  through the users routes (AccessEnds' Block 9).
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

## Implementation Plan

### Block 1: Removal

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

## Proposed commit message

The plan: `chore(aps): +LinkedRemoval v1.0`

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

## Change History

- v1.0 (2026-11-27): first revision, from the owner's "PLAN++ D54-A"
  at the completion gate of AP AccessEnds; D1-A and D2-A taken with PLAN++.
