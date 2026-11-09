# NOTE ServerAnnouncements v1.0: block 3 after the gate

What changed in Block 3 after its completion gate, run on 2026-11-09
at the owner's `EXEC`s, before its pull request.
Item 1 corrects item 10 of the NOTE `block_3_rules`.

## Decisions

1. **D11-A: `internal/notify` is under `make race`.**
   It is in `RACEPKGS` and in the contract's test gate (v1.3.3),
   and in `CI.md`'s Race job (v1.13.4),
   amended into Block 3's commit;
   the NOTE `block_3_rules` says, in item 10, that it is not.
2. **D12-A: `TestAnUnreachableCubeIsReportedOnce` waits for the line it reads.**
   The client emits `connected` before it logs the re-established
   connection, and the test read the log at the event,
   as its sibling `TestALostCubeIsReportedOnce` did before D9-A.
3. **D13-A: a push subscription's refused or failed removal names it.**
   `pathTarget` took every `{id}` but a token's for a room,
   so a `DELETE /api/v1/me/push/{id}` refused or failed before its
   handler named the subscription was audited as `room:<id>`;
   it is `push:<id>`.
4. **D14-A: the two tests that bring a Cube up late hold its port.**
   `TestSerialLearnedWhileRead` and `TestLearnedSerialServedAlready`,
   `main`'s, closed a free port and started the fake Cube on it later;
   a race run failed with `address already in use`,
   another connection having been given the port meanwhile.
   The fake Cube gained `Serve(ln)`,
   and both tests keep their listener and serve on it,
   the server's first dial waiting for the greeting.
   AP UiParallel does not touch it:
   its ports are the UI suite's and UDP 23272.
5. **The subscriptions' rows are deleted, as before.**
   The owner asked whether deleting them breaks a removed user's name in
   the log: it does not.
   Only `push_subscriptions` rows are deleted;
   users and tokens are still marked,
   and a temporary test (not committed) found a removed user's event
   naming its author.

## Where the work went differently from the plan

1. **The push routes are no commands.**
   The plan and `docs/api-v1.md` say so,
   but the actions were missing from `noCommand`:
   each subscription and removal made a command row and took an
   `Idempotency-Key`.
   `TestEveryActionHasALine`, which #124 brought, found them missing from
   `changeLines` too; they write no line, as a preference does.
2. **Two merges of `main`.**
   The first brought #124 and #126, the second #125, #128, #129 and #130.
   `CI.md` came to v1.13.4 here and to v1.14.0 on `main`:
   the merge is v1.14.1, with both.
