# NOTE LinkedRemoval v1.1: how block 2 went

Where Block 2, Installation, went differently from the plan,
the choices taken in the run, what the reading changed,
among them a stream ticket that still opened after its caller was let out,
and what verified it.

## What it made

Accepting an integration's new installation ends the old one's event streams,
as the owner decided at the completion gate of Block 1 (D4-A):

- **An event stream** opened through an integration for one of its users keeps
  the installation id the call came with (`instance` on the principal,
  set by `integrationPrincipal` once the chain lets the call in),
  and ends at its next keep-alive once another id is accepted,
  by `POST …/accept-instance` or by an attach,
  where it went on carrying what was sent to the user it acts as while the old
  installation's calls were refused (`403` `instance_pending`).
- **A ticket's stream** keeps the id too,
  since the ticket holds the principal of the call that asked for it.
- **The keep-alive only reads:**
  `stillActsFor` compares the stream's id with the integration's accepted one
  and records nothing pending,
  so a stream ends exactly where a call of its installation would be refused:
  the old installation's call that records its id again leaves the new
  installation's stream going,
  and the old one accepted again ends the new one's.
- **A stream ticket** opens the stream only while its caller is still let in:
  redeeming one asks `stillAdmitted` first,
  and a caller no longer let in (an installation no longer accepted,
  a key revoked since) is refused `401` as a token that ended is,
  counting toward no lockout, nothing sent.
- **What goes on:**
  the integration's own stream without the header,
  which carries no installation id, the owner's, a plain key's,
  and the accepted installation's.
- **One commit,** on `maint/0.17`:
  "fix(security:server):
  Accepting an integration's new installation ends the old one's streams",
  on the plan's v1.1 commit, which the builder built on
  and which is still `maint/0.17`'s head:
  nothing to carry, and the commit's tree is the builder's last.

## Different from the plan

Against the plan's block, item by item; each the owner's to change:

1. **A ticket's redemption asks whether its caller is still let in**
   (from the server's reader's should; not in the plan,
   which named only the stream's keep-alive), the coordinating session's choice.
   A ticket the old installation asked for before the acceptance and redeemed
   after it opened the stream (`200`) and sent the whole current state,
   twelve `state` events in the reader's probe,
   before its first keep-alive ended it:
   a window of up to the ticket's minute and one keep-alive,
   while the old installation's calls were refused.
   A ticket of a key revoked since did the same, older than the block.
   `authorize` now asks `stillAdmitted` for the ticket's principal
   before the stream sends anything,
   and refuses one no longer let in as a token that ended,
   `401` "missing or wrong token", uncounted; that closes both.
   The builder chose the `401` at the open over a `200` and an end (Choices).
   `docs/api-v1.md` (the stream's `?ticket=`,
   under "POST /api/v1/cubes/{sn}/events/ticket")
   and the OpenAPI ticket route say so,
   and a second Security bullet:
   "A stream ticket no longer opens a stream for a caller
   that is no longer let in, such as a revoked key."
2. **An attach ends the old installation's streams too**
   (from both readers; not in the plan):
   approving a request with `attach` writes the integration's `instance_id`,
   as `AcceptInstance` does,
   so the same comparison ends the streams of the installation accepted before.
   Tested (`TestStreamEndsAtAnAttach`) and documented:
   `docs/api-v1.md` beside `attach`, in the accept-instance paragraph
   and in both lists of when a stream ends,
   and the OpenAPI `approve`.
   The integration's own stream goes on until the key's collection revokes the
   old key (the server's reader's probe).
3. **Tests beyond the plan's:**
   in `TestStreamEndsWithItsInstallation`,
   the owner's and a plain key's streams going on;
   nothing written by the keep-alives after the old stream's end
   (the accepted id the new one, none pending);
   the old installation's call recording itself pending again ending nothing,
   a call of the new installation `200`,
   and the old installation's ticket from before the acceptance
   refused after that;
   the old installation accepted again ending the new one's stream.
   `TestStreamEndsAtAnAttach` (2);
   `TestTicketOfACallerLetOut` (1): eleven tickets of a key revoked after they
   were issued, one more than the lockout takes,
   each refused and none counted,
   and a ticket of a caller still let in opening the stream.
4. **Helpers, not copies,** in test files not the block's own:
   `asFrom`, which `as` calls (`onbehalf_test.go`);
   `ticketForWith`, which `ticketFor` calls (`auth_test.go`);
   `openStreamAt`, which `openStreamWith` calls,
   and `openTicketStream`, the new tests' own helper over it
   (`stream_admit_test.go`).
5. **Documents beyond the plan's:**
   those of 1 and 2, beside the plan's three places in `docs/api-v1.md`
   and the OpenAPI `accept-instance`.

## Choices the plan left open

- **The id set after the chain lets the call in**
  (`integrationPrincipal`, once `actingFor` returns),
  carried by the principal and never written.
  The other way: a parameter of `actingFor`,
  which would have lengthened its signature
  for a value the stream's re-check discards.
- **A ticket of a caller let out refused `401` at the open**
  (within 1, the coordinating session's choice):
  the page reacts to it as to a stream ended at a keep-alive,
  an error and then a new ticket, whose request asks it to sign in;
  and a plain `EventSource` does not retry a `401`,
  where after a `200` and an end it would reconnect with the spent ticket,
  which counts as a failed check.
  The other way: a `200` and an end at once, no state sent.

## What the readings found

One builder built the block in an exported copy of `maint/0.17`,
the plan's v1.1 at its head;
two readers read the first tree, one the server and its security,
the other the tests, the documents and the message.

Confirmed in the reading:

- the id is set once, after the chain lets the call in,
  from the header's value `parseOnBehalf` checked (32 lower-case hex digits,
  the store's pattern), carried by a ticket's pointer,
  and empty for every principal that is no call for an integration's user;
  each place a principal is made or copied was read (`principalFor`,
  the sign-in's, `q := *p` for the integration's own key, `actingFor`,
  the API wrapper, the stream, the ticket);
- a ticket redeemed with the header is refused `not_integration`, as before;
- every stream that should stay goes on:
  the owner's, a plain key's and the integration's own act for no one
  and skip `stillActsFor`; the accepted installation's matches;
- only `AcceptInstance` and an attach write `instance_id`;
  a pending id changes none;
- `stillActsFor` only reads;
- a request and a stream decide by the same comparison,
  apart from up to one keep-alive's lag and the late ticket (1 above);
- the test fails on the base for the reason it names,
  waits on signals, never a sleep, and passes under `-race`;
  the moved helpers send the same requests, byte for byte;
- the three places in `docs/api-v1.md` and the OpenAPI description
  match the code, and the fragment is the plan's.

No must.
Seven shoulds, done in one rework:

- the late ticket (1 above), from the server's reader;
- a keep-alive's write was not tested:
  both readers added a `NotePendingInstance` inside `stillActsFor`,
  and the test still passed;
  it now asserts, after the old stream's end,
  that the accepted id is the new one and none is pending;
- the late ticket was redeemed before the old installation recorded itself
  pending again (the tests' reader),
  so a re-check that kept a stream whose id is pending passed;
  it is redeemed after that now, and refused;
- the attach (2 above), from both;
- the keep-alive list's words,
  whose inner "or another installation id accepted" read as the outer list's
  last item (the tests' reader), now
  "…that acts as another user now
  or that came from another installation than the one accepted or attached";
- the test's comment, broken mid-sentence, rewrapped;
- the message, which named `openTicketStream`
  among the helpers that an existing one calls:
  now the new test's own over `openStreamAt`,
  and the message names 1, its test and its bullet, and the attach.

Kept for the completion gate, not changed:
the key's collection after an attach (1 there).
No reader read the rework.

## Seen and left

- **The page's text for a pending installation**
  ("until then its calls are refused", in six languages) does not name streams:
  not wrong, left by the builder (For the completion gate, 3);
  the next change of that text's.
- **Up to one keep-alive** (30 s) between an acceptance and a stream's end,
  as for every end at a keep-alive;
  and an acceptance turned back within one leaves the old stream open,
  as its calls are let in again at that moment:
  the server's reader's word, not a gap.
- **A stream opened with the old header after the acceptance** is refused `403`
  by the request path, which the block left unchanged and did not test again.
  Nothing to do.
- **An installation id empty on both sides** cannot arise:
  `parseOnBehalf` and `AddIntegrationRequest` each require 32 hex digits.
  Nothing to do.
- **The key's collection after an attach** is not tested by the block:
  it answers `410` where the old key was used (For the completion gate, 1).
- **A store that cannot be read when a ticket is redeemed**
  now refuses the ticket `401` without a line in the log,
  where the ticket opened the stream before;
  it fails closed as the keep-alive does, which logs no store error either
  (the drafter's reading of the commit, the coordinating session's word).
  A line for both, at Error, is for a later change of `stillAdmitted`.
- **The new fragment's one `reflow.sh` warning,** the 90-column "would fit" of a
  73-column wrap, which Block 1's fragment and others carry.
  Nothing to do.

## For the completion gate

1. **Collecting the key after an attach answers `410` where the old key,
   still live, was ever used** (`CollectIntegrationKey`):
   the tests' reader saw it; older than the block, and not judged.
2. **D3-A:**
   a push subscription made through an integration for a linked user
   goes onto the roadmap at this plan's retirement, for the next version,
   since it needs a store migration.
3. **The page's text for a pending installation**
   ("until then its calls are refused") does not name streams:
   left, not wrong.
4. **What a user notices:**
   accepting a new installation,
   or attaching a request with another installation id,
   ends the old installation's streams for the integration's users
   within a keep-alive;
   a stream ticket whose caller was let out since it was issued,
   a revoked key's among them, is refused `401`,
   and the page's next ticket asks it to sign in.

## What verified it

- **The builder, on the base:**
  a probe, not in the tree:
  a stream for a user through the integration,
  a call from another installation (`403` `instance_pending`),
  `POST …/accept-instance`, and five keep-alives later the stream still open
  and carrying a notification sent to the integration's user;
  on the tree it ends.
- **The builder, on the first tree:**
  `make ui`, `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin, `go run ./tools/apitypes -check`,
  `MASSIMO_ENV_FILE=/dev/null go test -count=1` of `internal/server`,
  `cmd/massimo`, `internal/serve` and `tools/identifiers`, each exit 0,
  and after the last wording edit of the documents the tests that read them,
  0; the race run of `internal/server` at `MASSIMO_TEST_SCALE=2`, alone,
  exit 0 (322 s), before that edit; `make cross`, exit 0;
  `reflow.sh --check`, 0 errors on `docs/api-v1.md`,
  its 187 warnings as on the base, and on the fragment;
  `commit-msg.sh` in a scratch clone at the base, 0 errors and 0 warnings;
  the whole UI suite in 1100-en, 878 passed in 16:44.
  The test seen failing:
  on the base, at the end of the old installation's stream;
  with the order swapped, at the ticket's;
  with the check cut and the late ticket first, at the late ticket's;
  without the id set, every stream through an integration ending at once.
- **The server's reader:**
  probes of the late ticket (`200`, twelve `state` events,
  then up to one keep-alive),
  of the attach (passing on the tree, failing on the base)
  and of a write at the keep-alive (not caught);
  the test on the tree with the base's `onbehalf.go`, failing for its reason;
  under `-race -count=5` with `TestStreamEndsWithTheLink`, passing;
  `go vet` and `gofmt` clean; `make ui` in the export.
- **The tests' reader:**
  the tree's four test files on the base,
  failing at
  "timed out after 5s waiting for the end of the old installation's stream";
  four mutations of the code, two caught
  (the id not set, any pending id ending the stream)
  and two not (a write at the keep-alive, a pending id's stream kept),
  which became shoulds; the test 3 times,
  and under `-race` at scale 2 `-count=10` with `TestStreamEndsWithTheLink`,
  `TestStreamEndsWhenItsCallerIsNotLetIn` and `TestEventStreamTicket`;
  the whole of `internal/server` once (71 s), all passing;
  `gofmt` and `go vet` (linux) clean; `reflow.sh --check` on `docs/api-v1.md`,
  187 warnings on base and tree, the same set, none on the new lines;
  `TestOpenAPICoversRoutes`; `commit-msg.sh` in a scratch clone at the base,
  0 errors and 0 warnings.
- **The builder, after the rework:**
  each new or changed test seen failing without its change:
  with the base's `auth.go` and `onbehalf.go`,
  the ticket test (the revoked key's first ticket `200`),
  the attach test and the installation test (timing out on the end);
  without the redemption's check,
  the ticket test and the installation test (the late ticket `200`);
  with a write at the keep-alive,
  at the assertion that nothing was written (the old id pending);
  with a re-check that keeps a pending id's stream,
  at the late ticket (`200`).
  `make ui`, `gofmt -l .` empty, `go vet` for three platforms,
  `apitypes -check`, `go test -count=1` of `internal/server`, `cmd/massimo`,
  `internal/serve` and `tools/identifiers`,
  the race run of `internal/server` at scale 2, alone (302 s), `make cross`,
  each exit 0;
  `reflow.sh --check`, 0 errors on `docs/api-v1.md` (187 warnings,
  the base's set) and on both fragments,
  the new one with one warning (Seen and left);
  `commit-msg.sh` in a scratch clone at the base, 0;
  the whole UI suite in 1100-en, 878 passed in 12:38.
- **The coordinating session:**
  in a clone with the commit, `commit-msg.sh`, 0 errors and 0 warnings,
  `changelog.sh check --staged --strict`, the change carrying its bullet,
  and `GUIDELINES/shared/tools/check` over the range, twelve checks all well;
  the commit's parent is the builder's base, so its tree is the builder's last,
  and the builder's last runs are the committed tree's.
- **Not run here:**
  the whole `go test ./...` on the last tree, which is the pull request's CI;
  `GUIDELINES/shared/tools/check` and `changelog.sh` by the builder,
  in a copy without git.
