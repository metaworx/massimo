# NOTE FirstOwner v1.0: how block 1 went

Where Block 1, Server, went differently from the plan,
the choices taken in the run, and what verified it.
The block is the server's side of the card to come:
the page of Block 2 will ask a server without users for its owner.

## What it made

- **The setting `auth.keep_open`,** a switch among the server's settings, off by default;
  on an open server anyone on the local network may write it, as every setting there.
- **`GET /api/v1/me` says `kept_open: true`** on an open server while the setting is on;
  absent otherwise:
  a server with users, a server without a store, with or without a token.
- **It changes nothing about who is let in:**
  a caller beyond the local network is refused as before,
  the setting's own PUT included;
  the sign-in on an open server answers as before.
- **The first owner is created once:**
  the store counts the users again inside the transaction that writes the first owner.
  A second request that also saw no users is answered `409`,
  "the first owner exists already: sign in", by a password and by OPAQUE alike;
  one that comes after the first has been written is `401` as before.
- **Two commits,** as the plan names them:
  "feat(server): An open server can be kept open by a setting"
  and "fix(server): The first owner is created once",
  with a CHANGELOG fragment each, under Added and under Fixed.
- **Documents:** `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/store.md`,
  `internal/server/ui/src/api.d.ts` by `make api-types`;
  in the README and `packaging/README.md` no sentence became untrue.

## Different from the plan

1. **D19-A, where the setting goes,** taken in the run, the owner's to change.
   The plan (D12) names three places:
   the API's creation of the first owner, the server's start, a reload.
   Built: the store removes it in every transaction of its own that makes a user row:
   the API's first owner, `massimo user add`, the owner from `MASSIMO_API_TOKEN`,
   a sign-in provider's user, an integration approved, an integration user's first call.
   No path is forgotten, and no window is left between a user and the setting.
   The server also sweeps at its start and at every reload of the settings file,
   for what a process of an older version or a store put in by hand can leave.
   Each removal is an Info line, `keep-open setting removed`;
   no change line of its own, the removal being part of the user's audited creation.
   The other way: the plan's three places alone,
   which leaves `massimo user add` beside a running server to the next reload.
2. **D20-A, the setting on a server with users,** taken in the run, the owner's to change.
   A PUT of true there is refused with `409`
   ("auth.keep_open counts only on a server without users: this one has users");
   false and null are taken; `GET /api/v1/settings` shows it not in effect there.
   The write counts the users in its own transaction,
   so a first owner and a PUT of true cannot leave the setting on beside users.
   The other way: taken and removed at once, or stored and ignored.
3. **D17-A, as the gate took it:** integration users count as users,
   so an integration approved on an open server takes the setting with it;
   left to AP IntegrationUsers.
4. **A server that loses all its users asks again** (D12) on every path of this version:
   a test makes a user and removes it, finds the setting gone,
   and keeps the server open again by a new PUT.
   One window is left, named in `docs/store.md`:
   a process of an older version adds a user while the setting is on,
   and every user is removed again before the next start or reload.
5. **In a PUT of several keys `auth.keep_open` is written first,**
   so that its refusal comes before any other key is written or audited;
   both readers found the keys written in sorted order.
6. **Built in an exported copy of main,** not in the session's worktree,
   which AP UiParts' Block 15 held:
   each step a recorded tree, the two commits made from the trees afterwards.
   No design turn: the block is small.
7. **The next free number** is FirstOwner D21.

## What the readings found

A sub-agent on Opus 5.5 built it.
Two independent readers on Opus read the two recorded trees through git:
one the store, the transactions, the race and what "open" lets in;
the other the API, the settings' model, the documents, the tests and the trees.
Both confirmed the block.

One reader could not break the first owner once:
the store opens its database with one connection,
and every transaction takes the write lock at its start, also against a second process.
The race tests ran twenty times under the race detector;
the reader's own harder test, two stores on one file as two processes,
first owners and PUTs of true mixed, thirty rounds:
always one owner and the setting off.
With the count taken out of the transaction the tests fail, with two owners.

Two musts:
a store assertion on the second owner by OPAQUE could not fail,
passing a nil record, refused whatever the count said;
two comments said that `massimo user add` can leave the setting beside users,
which this version cannot.

Seven shoulds, all done.
The one that mattered: the server's race test rested on timing,
a second request after the first one's commit being `401`, not `409`;
a test now holds the second request after its mode was read, by a body that blocks,
until the first owner exists, and must then see `409`, by a password and by OPAQUE;
the concurrent rounds stay, and a round that did not race is no failure.
The others: `auth.keep_open` written first in a PUT;
the removal's log line naming an integration under `integration`, not `user`;
the window's sentence in `docs/store.md`;
`/me` on a server with a token and no store;
a message's list; forward guards named as such.
The coordinating session read the rework itself.

## For Blocks 2 to 4

- **The page knows at its first read of who is in whether to ask:**
  `mode: "open"`, `setup`, and `kept_open`, absent unless on.
- **`POST /api/v1/users` for the first owner:** `201`;
  `409` "the first owner exists already: sign in"
  where another got there first while this request was under way;
  `401` once the server has users.
  In both the card should end as the sign-in's card for that server.
- **The switch that undoes keep open** (Block 3) writes `auth.keep_open` false or null;
  it is shown only while the server has no users,
  and a PUT of true on a server with users is `409`.

## Seen and left

- `TestLogAfterReadsABurstOnce/store`, not this block's, timed out once after 20 s
  under the race detector while another builder's UI suites loaded the machine;
  it passes alone.

## What verified it

- **The builder, on each tree,** each exit 0:
  `make ui`, `gofmt -l .` empty, `go vet` for linux, windows and darwin,
  the Go tests of the store, the server and `cmd`;
  on the last tree also `go test -count=1 ./...`,
  `go test -race -count=20 -run 'FirstOwner|KeepOpen'` over the three packages,
  `go run ./tools/apitypes -check`, `make ui-check` and `make cross`.
  Each new test shown failing with its change taken out;
  the checks from beyond the local network are forward guards.
- **The readers:** both trees built and tested alone; the generated types current;
  the OpenAPI file loads and names the new answers at the right routes;
  `reflow.sh --check` on the two changed documents, 0 errors.
- **The coordinating session:** what each tree changes, and that the copy is the last;
  both messages by `commit-msg.sh`; the changelog check, strict, per commit;
  `GUIDELINES/shared/tools/check` on the two commits in a scratch clone,
  12 checks, all well.
- **Not run locally:** the UI suite, which CI runs;
  no page changes, only the generated `api.d.ts` gains a field.
