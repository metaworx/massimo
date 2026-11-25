# NOTE CapabilityTable v1.0: how the run went

Where the run of `PLAN++ D1-A, D2-A` went differently from the plan,
what it found on the way, what verified it, and what is left.
Both blocks are built on the branch `claude/capability-table`,
one signed commit each; the branch is not pushed and has no pull request.

## What it made

- **Block 1,** "docs(api): The include and rank rules are one text in both
  documents": `docs/fragments/_capability-rules.md` holds the include rule
  in the API's wording and the rank rule's four common lines;
  `docs/api-v1.md` and `docs/store.md` take it through `sync-docs.sh`.
- **Block 2,** "test(api): One capability table, held to the server's
  routes by a test": the table lives in `docs/api-v1.md` alone, each route
  a code span `METHOD path`; `docs/store.md` section 6 points to it;
  `TestCapabilityTableCoversRoutes` in `internal/server/openapi_test.go`
  holds it to the registrations; the comment of `capabilities` in
  `internal/server/auth.go` names the table; a CHANGELOG fragment in
  `CHANGELOG.d/Changed/`.

## Where it went differently

1. **The two rules were not word for word the same,**
   as the EVAL before the plan said; the plan's Analysis records it.
   The fragment takes the API's fuller include rule, with store.md's
   "no migration" folded in as "Nothing stored changes, and no migration
   runs".
2. **store.md's order after the fragment:** "The rule holds for `users`,
   `guests` and `passwords` alike" comes first, then the decisions
   (AP StartConfiguration, D37, D40-A), so that "The rule" follows the
   rank rule it means. The plan had the decisions first.
3. **A third store.md line changed:** the schema comment of
   `role_capabilities` said "a key from the table in section 6";
   it says "a capability's key (section 6)".
4. **store.md keeps the migrations** the removed Allows texts carried,
   in one line beside the pointer: `integrations` came with 0021,
   `repair` with 0022.
5. **Three checks beside a route's own capability were not in the table,**
   and are now:
   - `passwords` reads one user, `GET …/users/{name}` (`mayView`);
   - `structure` beside `repair` on `…/repair/unlink-relink`
     (`repair_api.go`), named in the `structure` row as well;
   - `guests` on the users routes, named as routes instead of
     "the users routes, limited to guests".
6. **The test checks two things beyond the plan:**
   a code span left in a Routes cell that is a bare method, or a path
   without a method, fails it; and so does a path that starts with
   neither `/api/v1/` nor `…`.

## What verified it

- `make ui`, then
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server -run 'TestOpenAPICoversRoutes|TestCapabilityTableCoversRoutes' -count=1`:
  passes, run by the builder and once again here.
- The test fails, naming the row and the route, with each of these tried
  once and undone: a route taken out of its row; a route that does not
  exist added; two rows swapped; a row deleted; a path written without a
  method; a second file under `docs/` carrying the table's header.
- `gofmt -l .` prints nothing; `go vet ./internal/server` for linux,
  windows and darwin exits 0.
- `sync-docs.sh --check`, `changelog.sh check --staged` and `commit-msg.sh`
  pass for each commit; `GUIDELINES/shared/tools/check` passes on the
  branch.
- Not run here, and left to CI: `go test ./...`, `make race`,
  `make cross-check`, the UI suite (no UI file changed).

## Seen and left

- **`docs/ui.md`** says, in its Users bullet, that "a user changes only
  users whose role has no capability its own lacks": a summary of the
  UI, not a copy of the rule, left as it is.
- **`audit`** widens what some `read` routes show (another user's command,
  the audit's entries in the log and the stream); that is content, not
  access to a route, and stays out of the table.
- **The `any` routes** `POST /api/v1/login/key`, `POST /api/v1/logout`
  and `DELETE /api/v1/me/token` are in no row; `GET /api/v1/me` and one's
  own preferences are named in `read`'s row "for any signed-in user".
- **AP StartConfiguration** had planned the same table and test as its
  Block 18; its session asked on the owner's word, and its v1.2 carries
  the block to this plan (its D55).

## What the completion gate needs

- The branch's push and its pull request: `PLAN++` was sent where no gate
  stated a push, so it ran the plan's blocks only.
  The branch merges `origin/main` first; main had gained only plan files
  by the end of the run.
- The retirement, after the merge, at the owner's word.
