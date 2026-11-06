# AP StoreUsers v1.0: Users, login, tokens, audit

## TL;DR

Store stage (b) of docs/store.md:
the server's authentication moves from one token to users,
roles and capabilities in the store, with the audit and `commands` behind it;
one pull request at the end, the web UI block waiting for the UiStructure plan.

- Branch `claude/store-b` from `origin/main`; one functional commit per block,
  each with its tests and its CHANGELOG bullet.
- Migration `0001` already ships every (b) table and the six seeded roles →
  Block 1 is an additive `0002` (`min_reader` 1): `commands`,
  `users.auth_version`, `audit.command_id`.
- `Options.Token` compared in memory → tokens by SHA-256 hash in `tokens`;
  `MASSIMO_API_TOKEN` bootstraps `owner` and its key;
  `serve -addr` (no store) keeps today's check.
- `api(pattern, handler)` → `api(pattern, capability, handler)`;
  effective set = role's capabilities minus the disabled ones,
  nothing for a disabled role, everything for `owner`;
  a route without a capability fails closed;
  the OpenAPI test checks `x-capability` per operation.
- `authLimiter` in memory → `auth_failures` rows by address and by user name,
  surviving a restart.
- One audit row per non-GET request from the `api()` wrapper,
  the target from the handler, secrets stripped.
- Every write request is a `commands` row;
  the engine reports its steps as rows beneath it (D3-B);
  `Idempotency-Key` replays a finished command; `GET /api/v1/commands/{id}`.
- `massimo user …` opens the store beside the server.
- Dependencies: `golang.org/x/crypto` (argon2id, PHC string stored),
  `golang.org/x/term` (the password prompt); `go.mod` stays 1.26.
- Decisions taken at the gate: D1-A, D2-A, D3-B, D4-A, D5-A (Discussion).

### Blocks

1. Migration `0002_commands_auth.sql`: `commands`, `users.auth_version`,
   `audit.command_id`; the rule test learns the content columns;
   `store info` counts commands; a test migrates a version 1 file.
2. Store: `users.go`, `roles.go`, `tokens.go`, `audit.go`, `lockout.go`,
   `commands.go`, `Bootstrap`; tests per file.
3. Server auth from the store: bootstrap at start, token lookup by hash,
   capability per route, effective set, the limiter on `auth_failures`;
   the route-by-role table test and the other auth tests.
4. `POST /api/v1/login`, `POST /api/v1/logout`, `GET /api/v1/me`;
   the first owner while no user exists.
5. Users, roles and capabilities routes; the immutable owner and the last owner.
6. The audit hook in `api()` and `GET /api/v1/audit`;
   the audit row of every write handler.
7. `commands`:
   the engine observer, the rows per request and per step, `Idempotency-Key`,
   `GET /api/v1/commands/{id}`.
8. `massimo user add|list|passwd|disable|enable|token|revoke`.
9. Documents: docs/store.md stage status and item 6, README (the commands,
   what the token means now), docs/api-v1.md "Authentication";
   the route documents ride with Blocks 3 to 7.
10. Web UI, waiting for UiStructure: `login.js`,
    the login form in place of the token dialog, hiding by capability,
    Settings → Users and Roles, the first-owner form, "Log out",
    `auth.*` in six languages, the smoke test as an operator.

## Discussion

The decisions of the TL;DR gate (2026-11-06), with what they settle:

- **D1-A, `structure` with `manager` only.**
  The shipped seed already has it; a move changes the house as a rename does.
  Section 10 item 6 of docs/store.md is confirmed in Block 9.
- **D2-A, the first owner through `POST /api/v1/users`.**
  While no user exists the route is open to a peer on loopback, RFC 1918,
  link-local or a unique local IPv6 address, the role forced to `owner`;
  `X-Forwarded-For` is never read.
  `GET /api/v1/me` then answers `{"user": null, "setup": true|false}` so the
  page knows whether to show the form.
  No route of its own.
- **D3-B, the engine writes the command states.**
  One `commands` row per write request (the parent),
  and one row per `engine.Command` the request enqueues (a step,
  `parent_id` set), written by an observer the server gives the queue:
  `queued` when enqueued, `sent` when the queue begins sending its lines,
  then `acknowledged`, `failed` or `dropped` from its result.
  The parent goes `sent` with its first step and takes its final state from the
  HTTP outcome; a restore's parent is finalised by the run.
  The restore run keeps its own id in its API;
  its command row carries that id in `result`.
- **D4-A, guests without an invitation flow.**
  A `guests` holder creates a guest with a password and an end date;
  the one-time invitation link comes with Block 10, where it is used.
- **D5-A, `golang.org/x/term` for the password prompt**,
  plus `-password-stdin` for scripts.

What Block 10 needs from UiStructure before it starts: the components for forms,
dialogs, lists and switches, where a new Settings panel's template goes,
the per-language JSON layout for an `auth.*` block, and its merge into `main`.
Until then the API works with the bootstrapped owner token and the UI needs no change:
the stored `massimo.token` is the owner's API key.

## Analysis

### What exists

- `internal/store/migrations/0001_initial.sql` (v0.7.0-alpha.1) holds `roles`,
  `role_capabilities`, `capabilities_disabled`, `users`, `tokens`, `audit`,
  `auth_failures` and seeds the six roles with the sets of docs/store.md section 6.
  Missing against docs/server.md "What version 1 prepares": `commands` (item 7),
  `users.auth_version` (item 6), and a link from `audit` to the command.
- `internal/server/auth.go`:
  `tokenMatches` against `Options.Token`, `authLimiter` (per address,
  in memory), `ticketStore` (in memory),
  `authorize` called by the `api()` closure in `server.go` for every route;
  the stream opens with `?ticket=` or, deprecated, `?token=`.
- `internal/server/server.go` `routes()`:
  29 routes through `api(pattern, handler)` plus `configRoutes`;
  `openapi_test.go` extracts them with a regex and compares the paths with
  `docs/api-v1-openapi.yaml`.
- `internal/engine/queue.go`:
  `Command`, `Result`, `Ticket`;
  `Enqueue` coalesces by `Key` and resolves the replaced ticket with `ErrSuperseded`;
  `execute` sends the lines; `drain` resolves the rest with the context's error.
  No hook.
- Enqueue call sites in the server: `config_api.go` (`run`,
  the move's out and in commands, the room configuration), `server.go` (lock,
  setpoint), `restore_api.go` (the NTP read,
  the run's steps through `cubeio.Writer`), `cubeio.Writer` (`Write`, `Reread`);
  the refresh commands of `followup.go`, `discovery.go`,
  `cubes_api.go` and `server.go` belong to no request.
- `POST /api/v1/update` refuses without `Options.Token` (`updateBlocked`).
- `internal/server/ui/app.js` keeps `massimo.token` in `localStorage` and sends
  it as the bearer header; the token dialog opens on 401.
- `tools/ui-test/run.sh` starts the server with `MASSIMO_API_TOKEN=ui-test-token`;
  the smoke test stores the token in `localStorage` before the page loads.

### The three modes of a server

| Mode | When | Who may call | Audit and commands |
|---|---|---|---|
| legacy | no store (`serve -addr`) | as today: `Options.Token`, or open without one; every capability | not written |
| open | store, no user | anyone; every capability; `POST /api/v1/users` creates the first owner from a local peer (D2) | written with `user_id` NULL |
| users | store, at least one user | a session or API token; the user's effective capabilities | written with the user and the token |

`POST /api/v1/update` stays refused in legacy mode without a token and in open mode;
in users mode it needs `update`.

### Principal and capabilities

- `principal` in the request context: the user, the token, the effective set,
  and whether it is anonymous (open mode) or legacy.
  `authorize` resolves the bearer token (`sha256` hex of the header's value)
  through `store.TokenByHash`: not revoked, not expired, the user not disabled,
  not past `expires_at`, the role not disabled;
  else 401 (token) or 403 (user or role).
  The stream also takes a ticket; the ticket remembers the user for the audit.
- The capabilities are constants in `auth.go` with the list `capabilities` for
  validation and `GET /api/v1/capabilities`;
  the route table is the third argument of `api`.
  The effective set: `owner` everything;
  a disabled role nothing (403 "role disabled");
  else the role's rows minus `capabilities_disabled`.
  The users routes are registered with `users` and admit a `guests` holder and a
  user's own name; the handlers narrow (a `guests` holder touches guests only;
  one's own user may change its own password with `current_password`).
  `PUT …/devices/{rf}` is registered with `rename` and requires `structure` when
  the body has `room_id`.
- `openapi_test.go` reads `api("METHOD /path", "capability", …)` and checks that
  every operation of that path in the OpenAPI file carries `x-capability` with
  the same value.

### Lockout

`store.Lockout` keeps the `authLimiter` semantics on `auth_failures`:
`LockedFor(key)`, `Fail(key) bool`, `Succeed(key)`,
keys `addr:<ip>` and `user:<name>`, the window,
the limit and the lockout from `Options`;
rows outside the window and the lockout are pruned on every write.
The server keeps the in-memory limiter for legacy mode behind the same
three-method interface.
A login failure counts against the address and the user name;
a locked name answers 429 to a login as a locked address does to everything.

### Sessions and tokens

Tokens are 32 random bytes, hex, shown once; the store holds the SHA-256.
A session lasts 30 days and is extended on use; `expires_at`,
`last_used_at` and `last_used_from` are written at most once an hour per token,
so a poll every 10 s is not a write every 10 s.
An API key never expires.
Logout revokes the session that made the call; an API key answers 400.
A password set by someone else revokes the user's sessions;
a disabled or removed user keeps its rows and is refused by the user check.

### Bootstrap

`store.Bootstrap(ctx, token)` at `server.New` with a store:
no user and a token → the user `owner` (role `owner`,
no password) and an `api` token with the hash of `MASSIMO_API_TOKEN`,
label `MASSIMO_API_TOKEN`, logged `owner created from MASSIMO_API_TOKEN`;
users and a token whose hash is unknown → added to the first enabled owner with
that label; a token with that label whose hash is not the file's → revoked,
also when the file has none.

### Passwords

argon2id from `golang.org/x/crypto/argon2`, time 1, memory 64 MiB, 4 lanes,
a 16-byte salt, a 32-byte key,
stored as a PHC string (`$argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>`),
verified with the parameters the string carries so they can rise later.
A user without a hash cannot log in with a password.
`users.auth_version` is 1 (the server hashes the password) and is answered by
`GET /api/v1/me` and the login for the client-side derivation of docs/server.md item 6.

### Audit

The `api()` wrapper records every non-GET request but the stream ticket: `time`,
the principal's user and token, the peer's address,
the action from a table by route pattern (`setpoint`, `room.config`,
`device.update`, `cube.add`, `user.add`, `login`, …;
a test fails on a non-GET route without an action), the Cube of `{sn}`,
the target the handler names (`auditTarget(r, "device:1514b6")`),
the request as `detail` (the method, the path,
the JSON body up to 4 KB with `password`, `current_password`, `token` removed,
else its size) with the status, and `result`: 2xx `ok`, 401,
403 and 429 `refused`, else `failed`.
The body is read once into a buffer (the restore's limit of 1 MB applies) and handed on.
Login failures are `login.failed` with `user:<name>`;
`massimo user` writes rows with `remote` `cli` and no user.

### Commands (D3-B)

- `engine.Command` gains `ID` (the parent row's uuid,
  set by the server) and `Seq` (set by the queue);
  `engine.Options.Observer` with `Enqueued(Command)`,
  `Started(Command)` and `Finished(Command, Result)`,
  called for commands with an `ID` only,
  `Finished` also for superseded and drained ones.
- The server's observer per runtime writes the step rows:
  `queued` at `Enqueued`, `sent` at `Started`,
  and from `Finished` `acknowledged` (no error), `dropped` (`ErrSuperseded`,
  `ErrDiscarded`, a cancelled context) or `failed`;
  the parent goes from `queued` to `sent` with its first step.
- The parent row is written by the `api()` wrapper for every non-GET route but login,
  logout and the ticket: the user, the token, the Cube,
  the action and target as in the audit, the request fingerprint, the key.
  Its final state from the answer: 2xx `acknowledged`; 202 `sent`,
  finalised by the restore run (`runRestore` sets `acknowledged` or `failed`
  with the run's id, state and sent count as `result`); 4xx and 503 `dropped`;
  other 5xx `failed`.
  `result` holds the response body.
  At start, rows still `queued` or `sent` become `dropped` with
  `{"error": "server restarted"}`.
- `rt.enqueue(ctx, cmd)` sets `cmd.ID` from the request context and replaces
  `rt.queue.Enqueue(cmd)` at the request call sites;
  `cubeio.Writer` gains an `ID` field for the restore run.
- `Idempotency-Key` (1 to 128 characters) is kept per user;
  the same user and key with a finished row answer its status and body with
  `Idempotent-Replayed: true`;
  a different fingerprint under the same key is 422; a row still running is 409.
  Keys are cleared after 24 hours and rows removed after 30 days,
  both on insert.
- `GET /api/v1/commands/{id}` answers the row by uuid with its steps;
  a user without `audit` sees own commands only.

### Documents and tests

- docs/api-v1.md and docs/api-v1-openapi.yaml with each block that adds
  or changes a route (`TestOpenAPICoversRoutes`);
  README.md "Command line" and "Server" with Block 8 and 9;
  docs/store.md with Block 9;
  CHANGELOG.md with every commit touching `internal/**` or `cmd/**`.
- The test gate after every block: `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race` for `internal/server`, `gofmt -l .` empty, `go vet ./...` for linux,
  windows and darwin, `make cross`;
  `GUIDELINES/shared/tools/check` before each commit.
- `make ui-test` is not run by these blocks:
  the UI does not change until Block 10,
  and the smoke test keeps working with the token the runner sets,
  which Block 3's bootstrap turns into the owner's key.

## Implementation Plan

### Block 1: migration 0002

1. `internal/store/migrations/0002_commands_auth.sql`, `-- min_reader: 1`:
   `ALTER TABLE users ADD COLUMN auth_version INTEGER NOT NULL DEFAULT 1`;
   `CREATE TABLE commands` (`id`, `uuid`, `parent_id`, `cube_id`, `user_id`,
   `token_id`, `idempotency_key`, `action`, `target`, `request`,
   `state` with its CHECK, `status`, `result`, `created_at`, `updated_at`,
   `finished_at`, `version`, `deleted_at`, `origin`) with indexes on (`cube_id`,
   `created_at`), (`parent_id`) and a unique partial index on (`user_id`,
   `idempotency_key`) where the key is not empty;
   `ALTER TABLE audit ADD COLUMN command_id INTEGER REFERENCES commands (id)`.
2. `rule_test.go`:
   `commands` content columns `request` and `result`; `backup.go`:
   `commands` in `infoTables`.
3. `store_test.go`:
   a file migrated with `readerVersion` 1 is opened by the full binary and ends
   at version 2 with `min_reader` 1 and the new columns.

**Verification:**
the store tests, `TestNoIndexOnContent` among them;
`massimo store info` on a fresh directory prints schema version 2.

### Block 2: the store's user model

1. `password.go`:
   `HashPassword`, `VerifyPassword` (argon2id, PHC);
   `go get golang.org/x/crypto`.
2. `users.go`:
   `User`, `Users`, `User`,
   `AddUser` (a tombstoned name is taken over as `AddCube` does), `UpdateUser`,
   `SetPassword`, `DeleteUser`, `CountUsers`,
   the last-owner rule (`ErrLastOwner`), `ErrUserExists`, `ErrNoUser`.
3. `roles.go`:
   `Role` with its set, `Roles`, `Role`, `AddRole`, `UpdateRole` (name, set;
   `owner` refused with `ErrImmutable`), `SetRoleDisabled`,
   `DeleteRole` (builtin or in use refused), `DisabledCapabilities`,
   `SetCapabilityDisabled`, `Effective(user)`.
4. `tokens.go`:
   `Token`, `NewToken` (returns the secret once), `TokenByHash`, `Tokens(user)`,
   `RevokeToken`, `RevokeSessions(user)`, `TouchToken`; `HashToken`.
5. `audit.go`:
   `AuditEntry`, `Audit`, `AuditList(before, limit, user)`.
6. `lockout.go`:
   `Lockout` with `LockedFor`, `Fail`, `Succeed` on `auth_failures`.
7. `commands.go`:
   `Command`, `AddCommand`, `AddStep`, `SetCommandState`,
   `CommandByUUID` (with steps), `CommandByKey(user, key)`, `DropUnfinished`,
   the prunes.
8. `bootstrap.go`:
   `Bootstrap(ctx, token) (BootstrapResult, error)`.
9. Tests: `users_test.go` (add, take over a removed name, the last owner,
   a guest's end date, passwords), `roles_test.go` (seeded sets, a created role,
   rename, disable, the immutable owner, delete rules,
   effective sets with a disabled capability), `tokens_test.go` (hash lookup,
   revoke, touch, sessions revoked on a password set), `audit_test.go`,
   `lockout_test.go` (the window, the lockout, the prune,
   a restart keeps the counters), `commands_test.go` (states, key replay,
   prune), `bootstrap_test.go` (the three cases).

**Verification:**
the store tests; `go vet` per platform.

### Block 3: the server's authentication from the store

1. `auth.go`:
   the capability constants and list, `principal`,
   `authorize` for the three modes,
   the `failureLimiter` interface with the memory and the store implementations,
   the ticket carries the user;
   `api(pattern, capability, handler)` in `server.go` and `config_api.go` with
   the capability of every route;
   `PUT …/devices/{rf}` requires `structure` for `room_id`;
   `updateBlocked` by mode; `localPeer`.
2. `New`:
   `Bootstrap` with `Options.Token` when there is a store;
   the `serve` log line says the mode.
3. `openapi_test.go`:
   `x-capability` per operation;
   the OpenAPI file gets it on every existing operation,
   and the `bearer` scheme's description changes.
4. Tests: the bootstrap cases through the server (the old token keeps full rights,
   a changed one is added, a removed one is revoked);
   each default role against each route, a table test over the route list;
   a created role; a disabled capability; a disabled role;
   a guest past its end date; the lockout across a restart;
   `PUT …/devices/{rf}` with `room_id` as a `manager` and as an `operator`.
5. docs/api-v1.md "Authentication" and docs/api-v1-openapi.yaml.

**Verification:**
`go test -race ./internal/server`;
`make ui-test` not needed (the runner's token becomes the owner's key).

### Block 4: login, logout, me

1. `login_api.go`:
   `POST /api/v1/login`, `POST /api/v1/logout`, `GET /api/v1/me`;
   the first owner through `POST /api/v1/users` (the handler lands in Block 5;
   here the open-mode rule and `setup` in `me`).
2. Tests: login with a password,
   a wrong password counts against the address and the name, a locked name,
   a disabled user, a user without a password, logout revokes,
   `me` in the three modes, the session touched once an hour.
3. The route documents.

**Verification:**
`go test -race ./internal/server`.

### Block 5: users, roles and capabilities routes

1. `users_api.go`:
   `GET`/`POST /api/v1/users`, `GET`/`PUT`/`DELETE /api/v1/users/{name}`,
   `GET`/`POST /api/v1/users/{name}/tokens`,
   `DELETE /api/v1/users/{name}/tokens/{id}`; the `guests` narrowing;
   the own password with `current_password`; the first owner from a local peer.
2. `roles_api.go`:
   `GET`/`POST /api/v1/roles`, `PUT`/`DELETE /api/v1/roles/{key}`,
   `PUT /api/v1/roles/{key}/disabled`, `GET /api/v1/capabilities`,
   `PUT /api/v1/capabilities/{key}`.
3. Tests: the immutable owner, the last owner,
   a `guests` holder refused a `viewer`, a created role used by a user,
   a role deleted while in use, an unknown capability refused,
   the first owner from loopback and refused from a public address.
4. The route documents.

**Verification:**
`go test -race ./internal/server`.

### Block 6: the audit

1. `audit.go` in the server: the action table, `auditTarget`, the body buffer,
   the hook in `api()`; the handlers name their targets.
2. `GET /api/v1/audit?before=&limit=&user=`.
3. Tests: one row per write handler against the fake Cube with the action,
   the target and the result; a refused request is `refused`;
   passwords never land in `detail`; the login rows;
   the listing's paging and filter.
4. The route documents.

**Verification:**
`go test -race ./internal/server`.

### Block 7: commands

1. `internal/engine/queue.go`:
   `Command.ID`, `Command.Seq`, `Options.Observer`, the three calls;
   `queue_test.go`: the observer sees queued, sent and each result,
   superseded and drained included.
2. The server: `rt.enqueue(ctx, cmd)`, the observer per runtime,
   the parent row in `api()`, the final state, `DropUnfinished` at start,
   `Idempotency-Key`, `GET /api/v1/commands/{id}`;
   `cubeio.Writer.ID` for the restore run; `runRestore` finalises its parent.
3. Tests: a setpoint leaves a parent `acknowledged` with one step;
   a move leaves two steps; a superseded setpoint is `dropped`;
   a restore finalises its parent;
   a replayed key sends nothing to the fake Cube;
   a changed body under the key is 422; `GET …/commands/{id}`.
4. The route documents; `GET /api/v1/audit` shows `command_id`.

**Verification:**
`go test -race ./internal/server ./internal/engine`.

### Block 8: massimo user

1. `cmd/massimo/user.go`:
   `runUser` with the subcommands, `-role`, `-display-name`, `-expires`,
   `-no-password`, `-password-stdin`, `-label`;
   the prompt through `golang.org/x/term`; `main.go` dispatch and usage;
   audit rows with `remote` `cli`.
2. `user_test.go`:
   the subcommands against a temporary store with `-password-stdin`.
3. README.md "Command line": the commands.

**Verification:**
`go test ./cmd/massimo`; `make cross`.

### Block 9: documents

1. docs/store.md: the opening paragraph and section 10's status (stage (a) shipped,
   stage (b) implemented but the UI, item 6 confirmed, D3-B),
   section 6's bootstrap paragraph where the code differs.
2. README.md "Server": the authentication paragraph;
   `MASSIMO_API_TOKEN` in the variables table.
3. `CHANGELOG.md`:
   the bullets read as one release's net change.

**Verification:**
`GUIDELINES/shared/tools/check`; the full test gate; `make cross`.

### Block 10: the web UI (waiting)

Not before the coordinating session says so.
`login.js`; app.js: the login form (name and password,
or a token pasted) in place of the token dialog,
hiding by the capabilities of `GET /api/v1/me`, "Log out";
Settings → Users (list, add, invite a guest with an end date, role, disable,
reset password, sessions and keys with revoke) and Settings → Roles (the grid,
rename, create, disable, the switch per capability);
the first-owner form in open mode; `auth.*` in `i18n` for six languages;
docs/ui.md; the smoke test logs in as an operator and sees no Settings → Users;
`make ui-test` at 1100 and 390 px.

## Proposed commit message

```
chore(aps): +StoreUsers v1.0

Register the plan for store stage (b): users, login, tokens, the capability model, the audit and commands, as docs/store.md sections 6 and 9 specify, with the decisions of the TL;DR gate.

File-revision-v1: GUIDELINES/wip/2026-11-06_10-42_AP_StoreUsers_users_login_tokens_audit.md::v1.0::2026-11-06::All::the first revision
Assisted-by: claude-code:claude-fable-5-1
```

## Change History

- v1.0 (2026-11-06): first revision, from the TL;DR gate of the same day; D1-A,
  D2-A, D3-B, D4-A, D5-A taken by the owner with `PLAN++ D3-B`.
