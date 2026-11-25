# AP CapabilityTable v1.0: One table, held to the routes

## TL;DR

The table of capabilities and the routes each guards is kept once, in
`docs/api-v1.md`, and a Go test holds it to the routes the server
registers. The two rules both documents state, the include and the rank
rule, become one fragment that `sync-docs.sh` writes into both.

- capability table: `docs/store.md` §6 and `docs/api-v1.md` → `docs/api-v1.md` only; store.md points to it
- Routes column: loose prose ("every `GET` of a Cube") → one `METHOD path` per route, `…` a wildcard
- new test: every capability has one row, in `auth.go`'s order; every registered route is in its own capability's row; every route named exists
- include rule and the rank rule's opening → `docs/fragments/_capability-rules.md`, inlined into both
- OpenAPI `x-capability` stays as it is, tested as before

### Blocks

1. The include and rank rules as one fragment in both documents
2. One capability table, held to the registered routes by a test

## Discussion

The owner's decisions of 2026-11-25, at `PLAN++ D1-A, D2-A`,
on the EVAL of the same day (asked after the docs fix
`docs(api): The capability tables name every route the server guards`):

- **D1-A:** one table, in `docs/api-v1.md` ("Authentication"),
  plus a Go test against the registered routes.
  `docs/store.md` section 6 keeps what is the store's:
  the capability keys as rows of the roles, the default roles and their table,
  the history of the seeds, and it points to the table.
  Not chosen: D1-B (a fragment in both documents),
  D1-C (the column generated from the OpenAPI file),
  D1-D (no Routes column, the OpenAPI file the only list of routes).
- **D2-A:** the rules both documents state go into a fragment,
  `docs/fragments/_capability-rules.md`, written into both by
  `GUIDELINES/shared/tools/sync-docs.sh`, which the guidelines `check` runs
  with `--check`.

## Analysis

**The three copies of "which route needs which capability":**

1. The registrations: `api("METHOD /path", "capability", handler)` in
   `internal/server`, read by `registeredRoutes` in
   `internal/server/openapi_test.go`.
   Two values are not capabilities: `none` (no token) and `any`
   (any signed-in user).
2. The OpenAPI file's `x-capability` per operation, held to the
   registrations both ways by `TestOpenAPICoversRoutes`.
   It stays as it is: clients and tools read one operation at a time.
3. The markdown tables, `| Capability | Allows | Routes |`, in
   `docs/store.md` section 6 and `docs/api-v1.md` "Authentication".
   Nothing tested them, and both had drifted: routes missing in both,
   no `integrations` row in the API's, a `read` row claiming the backups.
   The docs fix of 2026-11-25 reconciled them by hand.

**What a registration does not say**, and the table states in prose:
the checks in handlers (`config_api.go`: `rename` for a name and
`structure` for `room_id` in `PUT …/devices/{rf}`, `configure` for the copy
of a room's settings and for a battery mark; `repair_api.go`: `structure`
beside `repair` for "unlink all and re-link"), `allowed()` in `auth.go`
(the users routes for `guests`, `passwords` and one's own name; a device
move for `structure`), and the includes (`update` and `reboot` include
`restart`).
The test cannot tie those to routes from the source text,
so it lets a row name a route registered under another capability,
and checks only that the route exists.

**The Routes column today** mixes route text with prose:
"every `GET` of a Cube, its rooms, devices, …",
"its `approve`, `refuse` and `accept-instance`",
"`/api/v1/me/push`" without a method, "`GET …/backups/{id}` and its `diff`",
"the users routes, limited to guests".
The test needs each route named, so the column is rewritten in one form:

- A route is a code span `` `METHOD path` ``;
  methods before it, each a code span joined by `/`, share its path:
  `` `GET`/`POST /api/v1/users` ``.
- A path starts with `/api/v1/` or with `…`;
  `…` stands for any characters, anywhere in the path
  (`…/devices/{rf}`, `…/repair/…`, `…/identities…`),
  and a path matches a route's whole path.
- Text around the spans stays free prose ("with `name`",
  "for any signed-in user"), so a row can still say what limits a route.
  A row may name no route at all (`guests`, which has no route of its own).

**The two rules both documents state** are not word for word the same,
which the EVAL said they were:

- The include rule: the API's paragraph says more
  (`GET /api/v1/me` lists `restart`, the example of an integration's call,
  `GET /api/v1/roles` showing the set as saved,
  "nor `update` `reboot`");
  store.md's adds the decision (AP StartConfiguration, D37, D40-A) and
  "no role's rows and no migration".
- The rank rule ("A user changes only users whose role has no capability
  its own lacks, …"): its first four lines are the same;
  then the API's goes on with the users routes' `may`
  (longer since AccessEnds' Block 2),
  store.md's with "The rule holds for `users`, `guests` and `passwords`
  alike".

So the fragment holds the include rule in the API's wording, with
store.md's "no migration" folded in, and the rank rule's four common lines;
each document keeps its own sentences after it.
The decisions' names stay in store.md, outside the fragment.
In store.md the rank rule moves up, from the roles' prose to beside the
include rule, where the fragment sits.

**Fragments:** `docs/fragments/_X.md` opens with the line
`> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.`
and a heading; a document takes it between
`<!-- BEGIN GENERATED PARTIAL docs/fragments/_X.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->`
and its `END` marker (README.md has five).

**Other references to the table:**
`internal/server/auth.go` says `capabilities` "lists every capability of
docs/store.md section 6 in its order"; it names `docs/api-v1.md` instead.
That comment is a shipped path (`internal/**`), so Block 2 carries a
CHANGELOG bullet. `docs/store.md`'s paragraph "The mapping is a table in
`internal/server/auth.go` …" says "the table lists both", and is reworded.
Other documents naming "docs/store.md section 6" name its users, roles,
OPAQUE and lockout, not the table, and stay.

**Another plan in the same text:** AP AccessEnds (open, another session)
changes `docs/api-v1.md` "Authentication" and "Integrations".
The branch merges `origin/main` before its pull request; a later change of
AccessEnds to the text the fragment takes goes into the fragment.

## Implementation Plan

Branch `claude/capability-table` from `origin/main`, after this plan's
commit on main. Every commit cites `AP CapabilityTable v1.0, Block <n>`.

### Block 1: The include and rank rules as one fragment

1. Write `docs/fragments/_capability-rules.md`: the include rule in the
   API's wording with "no migration" from store.md, then the rank rule's
   four common lines, as one or two paragraphs that read in both documents.
2. In `docs/api-v1.md` "Authentication", replace the include paragraph and
   the rank rule's first four lines with the PARTIAL block; the `may`
   paragraph follows it.
3. In `docs/store.md` section 6, replace the include paragraph with the
   PARTIAL block, followed by its decisions (2026-11-25,
   AP StartConfiguration, D37, D40-A) and "The rule holds for `users`,
   `guests` and `passwords` alike (docs/api-v1.md, "Authentication")";
   remove the rank rule's four lines from the roles' prose.
4. Run `GUIDELINES/shared/tools/sync-docs.sh`.

**Verification:** `GUIDELINES/shared/tools/sync-docs.sh --check` exits 0;
each document holds the fragment's text once, and neither keeps an old
copy of it (grep "has no capability its own lacks" and
"each include `restart`"); `GUIDELINES/shared/tools/check` exits 0.

### Block 2: One capability table, held to the routes by a test

1. In `docs/api-v1.md`, merge store.md's Allows texts into the table where
   they say more (the `read` row's list, `guests`' invite and end date),
   leaving the migrations to store.md;
   rewrite the Routes column in the form of the Analysis,
   every registered route of a capability in its row.
2. In `docs/store.md` section 6, replace the table with a sentence that
   names the table of `docs/api-v1.md` ("Authentication") and the test;
   reword "the table lists both" in the paragraph on the mapping.
3. In `internal/server/auth.go`, the comment of `capabilities` names
   `docs/api-v1.md`, "Authentication".
4. Add `TestCapabilityTableCoversRoutes` beside `TestOpenAPICoversRoutes`
   in `internal/server/openapi_test.go`, using `registeredRoutes`:
   - the table of `docs/api-v1.md` with the header
     `| Capability | Allows | Routes |` is found, and no other file under
     `docs/` carries one;
   - its rows are `capabilities`, each once, in that order;
   - every route named in a row exists, its method with it;
   - every registered route of a capability is named in that
     capability's row (`none` and `any` are in no row of their own);
   - a failure names the row and the route.
5. A CHANGELOG fragment `CHANGELOG.d/Changed/claude-capability-table.md`,
   for the API's readers: the table names every route, and a test keeps
   it so.

**Verification:** `make ui`, then
`MASSIMO_ENV_FILE=/dev/null go test ./internal/server -run 'TestOpenAPICoversRoutes|TestCapabilityTableCoversRoutes'`
passes; it fails, naming the row and the route, with a route taken out of
its row, with a route that does not exist added, and with two rows
swapped (each tried once, then undone); `gofmt -l .` prints nothing;
`go vet ./internal/server` for linux, windows and darwin;
`GUIDELINES/shared/tools/check` exits 0. The whole suite is left to CI.

## Proposed commit message

For Block 1:

```text
docs(api): The include and rank rules are one text in both documents

AP CapabilityTable v1.0, Block 1: docs/store.md and docs/api-v1.md each
stated that update and reboot include restart, and who may change whom,
in wordings that had grown apart; both now take one fragment.

Key changes:
- docs/fragments/_capability-rules.md holds the include rule and the
  rank rule's opening
- docs/api-v1.md and docs/store.md take it through sync-docs.sh; each
  keeps its own sentences after it
- No CHANGELOG bullet: documents only

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```text
test(api): One capability table, held to the server's routes by a test

AP CapabilityTable v1.0, Block 2: the table of capabilities and their
routes was kept in docs/store.md and docs/api-v1.md by hand, and both
had drifted from the routes; it is now in docs/api-v1.md alone, each
route named, and a test fails where a route is missing from its row.

Key changes:
- docs/api-v1.md's table names every registered route in its
  capability's row; docs/store.md points to it
- TestCapabilityTableCoversRoutes checks the rows against auth.go's
  capabilities and the registered routes
- auth.go's comment names the table's new place

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- **v1.0** (2026-11-25): the plan, on the owner's D1-A and D2-A of the
  EVAL after the docs fix of the capability tables.
