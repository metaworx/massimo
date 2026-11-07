# AP PasswordCost v1.1: password hashing on small boxes

## TL;DR

Blocks 1 to 5 made the server's hash cheap and bounded (pull request 87).
Blocks 6 to 11 take it off the server:
`auth_version` 2 signs in by OPAQUE (RFC 9807),
the browser runs argon2id, and the server keeps a record
that never holds the password and that no guess can be tested against offline.

- server per sign-in: argon2id ≈ 2 s, 19 MiB → ≈ 5 ristretto255
  multiplications, ≈ 50–80 ms, kilobytes (estimated; Block 6 measures)
- `POST /api/v1/login` → `login/start` + `login/finish`;
  a registration likewise in two calls
- `password_hash` → an OPAQUE record of 192 bytes and its argon2 parameters
- unknown name → a fake record, the same shape as a real one
- lockout: counted at the start, since a start is a guess;
  cleared by a finished sign-in
- v1 users → upgraded at their next sign-in in the browser;
  then a setting turns v1 off
- `bytemare/opaque` v0.18.0 (Go), `@serenity-kit/opaque` 1.1.0 vendored;
  CSP `'wasm-unsafe-eval'`

### Blocks

1. Parameters: the new defaults, the ceiling, the benchmarks, a memory test (done, #87)
2. Gate: hashes at once bounded in the store, `503` when the queue is full (done, #87)
3. OneAtATime: one sign-in per address, the dialog's single submit (done, #87)
4. Rehash: a stale hash replaced at a successful sign-in (done, #87)
5. CheckFirst: user add and set password refuse before hashing (done, #87)
6. OpaqueServer: the key file, migration 0006, sign-in and registration routes, fake records, the lockout at the start
7. OpaqueClient: the vendored module, the CSP, the sign-in dialog over OPAQUE, the browser against the Go server
8. Register: every password set from the browser registers a record; the CLI where Go's records open in the browser
9. Upgrade: a v1 sign-in in the browser registers at once and drops the hash
10. V1Off: a setting that refuses the password sign-in
11. Docs: the key scheme on the export key, `docs/store.md` sections 5, 6 and 10

## Discussion

The owner measured on a Raspberry Pi 1 (ARMv6, one core, 426 MB RAM,
about 313 MB available) with v0.10.0-alpha.1 on 2026-11-07,
from the audit log's `detail.ms`:
`POST /api/v1/users` with a password 4585 ms,
the same name again (409) 4727 ms, `POST /api/v1/login` 6138 and 6378 ms.
The owner's decisions at the TL;DR gate of 2026-11-07 (`PLAN++ D5-A`,
the others as recommended):

- **D1-A.** m=19 MiB t=2 p=1 on every box, not parameters chosen by the
  box's memory: a hash verifies at the same cost wherever it is restored or
  synced, and the dummy check costs what a real one does.
- **D2-A.** min(GOMAXPROCS, 4) hashes running, 8 waiting, then `503`:
  19 MiB at most on a Pi 1, 76 MiB on four cores.
- **D3-A.** One sign-in per address at a time, a second `429`:
  the lockout bites only if attempts arrive one after another.
- **D4-A.** The rehash writes `password_hash` alone, as `last_login_at` is
  written, guarded by the old value; `version` and `updated_at` stay,
  since it is not an edit.
- **D5-A.** This plan now, server side;
  `auth_version` 2 (the client derives the key) and a PAKE are the next
  revision of this plan, planned after this run (`PLAN D5-C`).

The owner's decisions at the TL;DR gate of v1.1 (`PLAN++` of 2026-11-07,
all as recommended):

- **D6-A.** OPAQUE, with `github.com/bytemare/opaque` on the server and
  `@serenity-kit/opaque` in the browser:
  the RFC of 2025, no salt sent ahead of the proof, so nothing to
  precompute guesses against; not SRP-6a, not a key derived in the browser
  and sent, which a copy could replay.
- **D7-A.** The browser's argon2id at m=19 MiB t=2 p=1,
  the parameters kept with each record so that they can rise later.
- **D8-A.** The server's OPAQUE keys in `opaque.key`, 0600, in the data
  directory beside `node.key`: a copy of `massimo.db` alone allows no
  offline guessing (docs/server.md, "What version 1 prepares", item 8).
- **D9-A.** The v1 sign-in stays for scripts until Block 10's setting turns
  it off; each user is upgraded at the next sign-in in the browser.
- **D10-A.** This revision, v1.1, Blocks 6 to 11 after the finished 1 to 5.
- **D11-A.** A new branch from main once #87 is merged.
  #87 was open and green, not merged, when the `PLAN++` came,
  so the branch starts from #87's head and merges main once #87 is in;
  #87 itself is left as it is.

## Analysis

### Blocks 1 to 5

- **What costs.** `internal/store/password.go` hashed with
  `argon2.IDKey(password, salt, 1, 64*1024, 4, 32)`.
  Measured on one core (`GOMAXPROCS=1`, amd64, `x/crypto` v0.58.0):
  48 ms and 64 MiB a hash; four at once peak at 256 MiB of heap.
  OWASP's m=19 MiB t=2 p=1: 29 ms and 19 MiB, four at once 76 MiB;
  m=7 MiB t=5 p=1: 18 ms and 7 MiB.
  On the Pi 1 the hash is the 4.5 s of a user add;
  the new parameters cost about 0.6 of it by the same ratio.
  The other 1.5 s of a login is not the hash
  (the session, its touch and the last login are three writes);
  it is not in this plan.
- **What bounds concurrent attempts: nothing.**
  `authorize` checks the address's lockout, `login` the name's,
  and both count a failure after `CheckPassword` returns.
  Attempts sent together all pass the checks before the first is counted,
  and each allocates 64 MiB: five at once exceed what a Pi 1 has free.
  An unknown name costs the same through `DummyVerify`.
- **User add hashes first.** `AddUser` hashes before it opens the
  transaction that finds the name taken or the role missing;
  `SetPassword` hashes before its `UPDATE` finds no user.
- **Old hashes stay.** The PHC string carries m, t and p, so a hash of
  the old parameters verifies; without a rehash it costs 64 MiB at every
  sign-in until the password changes.
- **What the stored string may ask for.** `VerifyPassword` takes m up to
  2³²−1 KiB from the row: a corrupt or foreign row would allocate that much.
- **The dialog.** `login.js` posts on every submit;
  Enter pressed twice sends two sign-ins, which Block 3 would answer `429`.
- **Bodies.** The login's body is read through the audit's read-ahead,
  at most 1 MiB with a store; nothing to change.

### Blocks 6 to 11: `auth_version` 2 by OPAQUE

- **What version 1 prepared.** `users.auth_version` (migration 0002,
  docs/server.md, "What version 1 prepares", item 6) is read at login,
  so the key scheme arrives as a change at the next sign-in,
  not as a reset of every password.
  Its comment says 2 is "the client derives the key and the server hashes
  that"; OPAQUE does better, the server holds no hash of anything the client
  sends, and the comment is brought up to date.
- **The protocol** (RFC 9807, suite ristretto255-SHA512, 3DH, argon2id as
  the key stretching function, run by the client only).
  Registration: the client blinds the password (32 bytes),
  the server evaluates it under a key of its own for that user and adds its
  public key (64 bytes), the client finishes and uploads the record:
  its public key (32), a masking key (64) and an envelope (96),
  192 bytes; nothing is kept on the server between the two calls.
  Sign-in: KE1 (96 bytes) → KE2 (320 bytes), the server keeping the state
  of the exchange; KE3 (64 bytes) → the session.
  Both sides end with a session key, and the client with an export key that
  the server never sees.
- **What the server pays.** Per start an OPRF evaluation, a key pair, and
  3DH: about five scalar multiplications on ristretto255; per finish a MAC.
  Not measured with the library: the auto-mode classifier refused to fetch
  it during the TL;DR gate.
  The standard library's X25519 takes 55.1 µs on amd64 and 548.3 µs on
  a 32-bit build, and argon2 ran about 20 times slower on the Pi 1 than on
  that build, so about 50 to 80 ms a sign-in there, with kilobytes,
  against about 2 s and 19 MiB now.
  Block 6's benchmark measures it on both builds.
- **The browser's cost.** The argon2id of D7-A in WebAssembly,
  on the phone or laptop that signs in, at registration and at every
  sign-in; the server sends the record's parameters with KE2.
- **The libraries.**
  `github.com/bytemare/opaque` v0.18.0: Go, MIT, by one of the RFC's
  authors, RFC 9807's test vectors, a fake record for unknown users;
  not audited.
  `@serenity-kit/opaque` 1.1.0 (2026-02-01): MIT, `opaque-ke` 4 (audited by
  NCC Group) compiled to WebAssembly and inlined into its `esm/index.js`,
  7 files and 889,725 bytes unpacked; key stretching presets and
  `argon2id-custom`.
  The KSF runs on the client alone, at registration and at sign-in,
  so the two libraries meet only in the protocol messages.
  They would meet in the KSF too if a record made with Go's client had to
  open in the browser (Block 8's CLI):
  `opaque-ke` salts its argon2id with 16 zero bytes, a choice RFC 9807
  leaves open, so Block 8 tests it before relying on it.
- **The page's rules.** No build step and no framework stay:
  the module is one vendored file, loaded by `import()` from `login.js`
  only when a sign-in or a registration needs it.
  WebAssembly needs `script-src 'self' 'wasm-unsafe-eval'` in the CSP of
  `securityHeaders`; nothing else changes there.
  `GET /ui/{file}` serves no subdirectory, so `ui/vendor/` gets a route of its
  own and a place in the `go:embed` list.
  `docs/store.md` open decision 17 named this bend of the rule;
  it is taken with this plan.
- **Who exists.** `DummyVerify` keeps an unknown name as slow as a known
  one today; under v2 an unknown name gets a fake record,
  so its KE2 has the size and shape of a real one.
  The OPRF key of a user is derived from the server's seed and a credential
  identifier: the user's `uuid` for a real user, the normalised name for an
  unknown one, so that the same name meets the same key every time and two
  tries cannot tell a fake from a real record.
- **A start is a guess.** The client learns from KE2 alone whether its
  password was right, since only the right one opens the envelope;
  KE3 merely proves it to the server.
  So the lockout counts a failure against the address and the name at the
  start, as soon as KE2 is sent, and a finished sign-in clears both as a
  password sign-in does now.
  The TL;DR gate said "counted at finish"; that would let a client guess
  without ever being counted.
- **The pending starts.** The state of a start (the expected MAC, the session
  key) is kept in memory under a random login id for 60 s, at most
  `maxTracked` of them; the address claim of Block 3 is held from the start
  to the finish or the expiry, so an address still runs one sign-in at a
  time.
  A restart loses them, and the client starts again.
- **Version 1 beside version 2.** The browser cannot know a user's version,
  and the server must not say it per user, or the start would tell names
  with a v1 hash from the rest.
  So the start always answers a KE2, a fake one for a v1 user too,
  and says once for the whole box whether the v1 sign-in is still on.
  A browser whose finish fails falls back to `POST /api/v1/login` while it
  is, passing the login id, so the attempt counts once;
  a v1 user then signs in and is upgraded (Block 9).
  The cost while v1 is on: a mistyped password of a v2 user crosses once
  more, over TLS, and is refused.
  Once Block 10's setting turns v1 off, no password crosses at all.
- **The password paths.** Today a password is set by:
  the first owner (`POST /api/v1/users` on an open server, from the local
  network), a user add with a password, a password set by someone with
  `users` or `guests` (`PUT /api/v1/users/{name}`), one's own change
  (`current_password`), and the CLI's `user add` and `user passwd`.
  Under v2 the browser registers in each of them;
  one's own change proves the current password with a sign-in whose finish
  answers a one-time proof, valid 5 minutes, instead of a session.
- **The CLI.** It opens the store and the data directory,
  so it can run both halves of a registration in one process with
  `bytemare/opaque`'s client, if a record made that way opens in the
  browser (the KSF salt above).
  If it does, the CLI registers v2 and Block 10 can turn v1 off everywhere;
  if not, the CLI keeps v1 and the setting says the CLI's passwords then need
  one sign-in in the browser before it is turned off.
- **The keys.** `opaque.key` holds the OPRF seed (64 bytes) and the server's
  key pair, made at the first start as `node.key` is, mode 0600.
  Lost, every v2 record is useless and its users need a password set
  again; the backup in `docs/store.md` stage (e) carries the database only,
  so `docs/update-windows.md` and the backup's documentation say to keep
  the data directory, as they do for `node.key`.
- **The export key.** It never leaves the browser and is the same at every
  sign-in with the same password: the source of the wrapping key of
  docs/server.md's key scheme once end to end comes, replacing the
  argon2id-and-HKDF split there. Nothing uses it yet.
- **The schema.** Migration 0006 adds `users.opaque_record` (BLOB, NULL when
  there is none) and `users.opaque_ksf` (the PHC-style parameters,
  `m=19456,t=2,p=1`). A v2 user has `password_hash` empty and
  `auth_version` 2. Additive: a binary of before reads a v2 user as one
  without a password, keys only, which refuses rather than lets in.
  `has_password` counts a record as a password.
- **What "never sent" means then.** No password, hash or replayable key
  crosses at a v2 sign-in or registration.
  Each existing password crosses once more, over TLS, at its upgrade;
  a mistyped one while v1 is on; the session after a sign-in is still a
  bearer token, so TLS stays; and a server that serves altered JavaScript
  can still read what is typed (docs/server.md, "Trusting the code").
- **WebCrypto.** Argon2 in WebCrypto is a WICG draft and a Chrome experiment
  of June 2026; PBKDF2 is there, but OPAQUE's libraries do not take it.
  Not usable yet.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing runs on the owner's Pi or reaches a Cube.
Each block is one functional commit citing `AP PasswordCost v1.0, Block <n>`
for Blocks 1 to 5 and `AP PasswordCost v1.1, Block <n>` for 6 to 11,
with its CHANGELOG bullet; one pull request for each run,
`origin/main` merged into the branch first.

### Block 1: Parameters

Done in #87.

1. `internal/store/password.go`:
   `argonTime = 2`, `argonMemory = 19 * 1024`, `argonThreads = 1`,
   with OWASP named in the comment;
   the ceiling `maxArgonMemory = 256 * 1024`, `maxArgonTime = 16`,
   `maxArgonThreads = 16`;
   the parsing of `VerifyPassword` moved into `parseHash(hash) (phc, bool)`
   (m, t, p, salt, key), which refuses a string over the ceiling,
   for Block 4 to reuse.
2. Tests (`password_test.go`): the new prefix in `TestPasswordHash`;
   a hash of the old parameters still verifies;
   one over the ceiling does not, without allocating;
   `TestPasswordMemory`: one `HashPassword` allocates between 18 and 21 MiB
   (`runtime.MemStats.TotalAlloc`);
   `BenchmarkPassword` with the current and the old parameters,
   `b.ReportAllocs`.
3. `docs/store.md` section 6, "Login in the web UI":
   the parameters, why, and what they cost.
4. CHANGELOG, Changed.

**Verification:** the test gate; `go test -run '^$' -bench Password
-benchmem ./internal/store` shows about 19 MiB a hash against 64 MiB.

### Block 2: Gate

Done in #87.

1. `internal/store/password.go`: `hashGate` with `slots chan struct{}`
   and a count of waiters; `enter(ctx) (leave func(), err error)`
   takes a free slot at once, waits while fewer than the queue wait,
   else `ErrBusy`; the wait ends with the context.
   `Store.hashes` made in `newStore` with min(GOMAXPROCS, 4) slots
   and 8 waiting; `argonKey = argon2.IDKey` as the one call, for the tests.
2. `CheckPassword` (with `DummyVerify`), `AddUser` and `SetPassword`
   hash inside `enter`.
3. `internal/server`: `httpError.retryAfter`, set as `Retry-After` by the
   wrapper; `store.ErrBusy` and a context ended while waiting answer
   `503` with `Retry-After: 5` from the login, the user add and the
   password change (`userFail`).
4. Tests: the gate's slots, queue, `ErrBusy` and context;
   eight concurrent `CheckPassword` with two slots never have more than two
   hashes in flight and all succeed;
   `BenchmarkPasswordChecksAtOnce` reports the peak heap of eight at once;
   the server's mapping of `ErrBusy` to `503` with `Retry-After`.
5. `docs/store.md` section 6 ("The lockout"), `docs/api-v1.md` (the login,
   the users), the OpenAPI file (`Busy`, `503` on the three routes).
6. CHANGELOG, Security.

**Verification:** the test gate with `-race` for `internal/store` too.

### Block 3: OneAtATime

Done in #87.

1. `internal/server/login_api.go`: `signIns`, a set of address keys under a
   mutex, bounded by `maxTracked`; the login claims the address after the
   name's lockout check and before the password check, releases it when done;
   a claimed address answers `429` "a sign-in from this address is already
   running" with `Retry-After: 1`, not counted as a failure;
   a full set answers `503` as Block 2's busy.
2. `internal/server/ui/login.js`: the dialog ignores a submit while its
   sign-in runs.
3. Tests: `TestLoginOneAtATime` (a claimed address answers `429`,
   the claim released lets the next one in, no failure counted);
   `tools/ui-test/test_login.py`: two submits send one sign-in.
4. `docs/store.md` section 6 ("The lockout"), `docs/api-v1.md` (the login),
   the OpenAPI file (the login's `429`).
5. CHANGELOG, Security, amending Block 2's bullet where it is the same area.

**Verification:** the test gate and `make ui-test`.

### Block 4: Rehash

Done in #87.

1. `CheckPassword`: after a match, a hash whose m, t, p or key length
   differ from the current ones is hashed again inside the same slot and
   written with `UPDATE users SET password_hash = ? WHERE id = ? AND
   password_hash = ?`; a read-only store or a failed write leaves the old
   hash, logged, and the sign-in succeeds.
2. Tests: a user with a hash of the old parameters signs in and has one
   of the new; a wrong password changes nothing; a password changed
   meanwhile is not overwritten; `version` and `updated_at` stay.
3. `docs/store.md` section 6, "Login in the web UI".
4. CHANGELOG, Changed, amending Block 1's bullet.

**Verification:** the test gate.

### Block 5: CheckFirst

Done in #87.

1. `AddUser`: the role and a live user of the name are looked up before
   the hash (`ErrNoRole`, `ErrUserExists`); the transaction keeps its own
   checks for a race.
   `SetPassword`: a live user of the name is looked up before the hash
   (`ErrNoUser`).
2. Tests, counting `argonKey`'s calls: a taken name, a missing role and a
   missing user hash nothing; the API answers `409` and `404` as before.
3. `docs/store.md` section 6.
4. CHANGELOG, Changed.

**Verification:** the test gate.

### Block 6: OpaqueServer

1. `go.mod`: `github.com/bytemare/opaque` v0.18.0 and what it pulls in;
   each new module's licence noted in the commit, `govulncheck ./...` clean.
2. `internal/store/opaque.go`: `opaque.key` in the data directory
   (`OpaqueKeyPath`, the pattern of `node.key`: made with `O_EXCL` and
   mode 0600 at the first open of a writable store, read at every open):
   the OPRF seed and the server's key pair;
   the server configuration, suite ristretto255-SHA512 with the context the
   browser's library uses, kept in one place.
3. `internal/store/migrations/0006_opaque.sql`:
   `users.opaque_record BLOB` and `users.opaque_ksf TEXT NOT NULL DEFAULT ''`;
   the comment of `auth_version` in 0002 is history and stays,
   the new file says what 2 now means.
4. Store: `OpaqueRegistration(ctx, name, request)` (the response, under the
   user's credential identifier), `SetOpaqueRecord(ctx, name, record, ksf,
   by)` (record and parameters written, `password_hash` emptied,
   `auth_version` 2, sessions ended where `by` is someone else, as
   `SetPassword`), `opaqueLogin(ctx, name)` (the record, or a fake one for a
   name with none, with the parameters to send); `User.HasPassword` true for
   a record too.
5. `internal/server/opaque_api.go`:
   `POST /api/v1/login/start` `{user, ke1}` → `{login, ke2, ksf, v1}`;
   `POST /api/v1/login/finish` `{login, ke3, label}` → the answer of
   `POST /api/v1/login` today;
   `POST /api/v1/users/{name}/register/start` `{request}` → `{response}` and
   `.../register/finish` `{record, ksf, proof}`, allowed as a password set
   is today (`allowed` in `auth.go`), `proof` for one's own change.
   The start: the name's lockout checked, the address claimed (Block 3),
   one failure counted against the address and the name, the state kept
   under `login` for 60 s, at most `maxTracked`, the claim held until the
   finish or the expiry.
   The finish: a wrong or late KE3 `401` "wrong user or password" and
   nothing more counted; a right one the session, both counters cleared,
   `last_login_at`; with `"purpose": "proof"` at the start, a one-time
   proof for 5 minutes in place of a session.
   Unknown, disabled and expired users answer as `POST /api/v1/login` does,
   after the finish, never at the start.
6. `internal/server/audit.go`: the four routes;
   `docs/api-v1.md` and `docs/api-v1-openapi.yaml` (`TestOpenAPICoversRoutes`).
7. Tests with `bytemare/opaque`'s client in Go: a v2 user registers and signs
   in; a wrong password is `401` and counted once, at the start; an unknown
   name's KE2 has the length of a real one and the same name's evaluation is
   the same twice; a v1 user's start answers a fake KE2; a late finish
   `401`; a second start from the address `429`; a full set of pending
   starts `503`; a malformed KE1 or record `400`; the key file made once,
   0600, and read back; the proof one-time and per user.
   `BenchmarkOpaqueLogin`: the server's start and finish, time and bytes,
   on amd64 and with `GOARCH=386`, scaled to the Pi 1 by Block 1's ratio.
8. CHANGELOG, Added.

**Verification:** the test gate; the benchmark's figures in the run's NOTE,
against the 50 to 80 ms estimated here.

### Block 7: OpaqueClient

1. `internal/server/ui/vendor/`: `@serenity-kit/opaque` 1.1.0's
   `esm/index.js` as `opaque.js`, its `LICENSE`, and a `README.md` naming the
   version, the npm integrity of the tarball and the SHA-256 of the file;
   `TestVendoredOpaque` checks that SHA-256, so that a change is a change on
   purpose.
   `ui.go`: the file in the `go:embed` list, `GET /ui/vendor/{file}`.
2. `securityHeaders`: `script-src 'self' 'wasm-unsafe-eval'`; the test that
   reads the CSP updated.
3. `login.js`: `opaqueModule()` imports `ui/vendor/opaque.js` once and
   awaits its `ready`; `signIn(user, password)` runs `startLogin`,
   `login/start`, `finishLogin` with the `ksf` of the answer
   (`argon2id-custom`), `login/finish`; the dialog shows that it is working
   while argon2 runs, its text through `t()` in all six languages.
   The v1 fallback is Block 9's.
4. `tools/ui-test/test_opaque.py`: in the page, with the vendored module,
   a user registered through the two registration routes, then signed in
   through the dialog; the session works, and no request body holds the
   password (the requests recorded); at 1100 and 390 px.
5. `docs/ui.md`: the sign-in over OPAQUE, the module and why it is vendored.
6. CHANGELOG, Added, beside Block 6's bullet.

**Verification:** the test gate and `make ui-test`; the module's load and
one sign-in timed in the test's browser and noted.

### Block 8: Register

1. `login.js`, `users.js`: `register(name, password, proof)` runs
   `startRegistration`, `register/start`, `finishRegistration` with D7-A's
   parameters, `register/finish`.
   The first owner: `POST /api/v1/users` without a password, then the
   registration, then the sign-in.
   A user add with a password: the user added without one, then registered.
   A password set for someone else: registered.
   One's own: a sign-in with `"purpose": "proof"` for the current password,
   then the registration with the proof.
   A registration that fails after the add leaves a user without a password,
   which the panel shows as now and the next try sets.
2. The CLI's `user add` and `user passwd`: both halves of the registration in
   one process with `bytemare/opaque`'s client and D7-A's parameters,
   if the record opens in the browser: `tools/ui-test` signs in a user the
   CLI registered.
   If it does not, the CLI keeps v1 and the plan's NOTE says so.
3. Tests: Go, each password path of the API ending at `auth_version` 2 with
   an empty `password_hash`; the UI test of each path in the page.
4. `docs/store.md` section 6 ("The commands"), `docs/ui.md`.
5. CHANGELOG, Changed.

**Verification:** the test gate and `make ui-test`.

### Block 9: Upgrade

1. `POST /api/v1/login` takes `login`, a start that failed in the browser:
   the start's pending state is taken and nothing more counted;
   for a user still on v1 the answer adds `upgrade`, a one-time proof for
   5 minutes.
2. `login.js`: a finish that fails while the start said `v1` falls back to
   `POST /api/v1/login` with `login`; an answer with `upgrade` registers the
   same password at once, with that proof, and the user is on v2.
   A failed registration leaves the user on v1, logged; the next sign-in
   tries again.
3. Tests: Go (a fallback counts once; the proof is one-time and per user;
   a v2 user's password sign-in is refused as a wrong one);
   `tools/ui-test`: a v1 user signs in through the dialog and ends on v2 with
   an empty hash, and signs in again over OPAQUE.
4. `docs/store.md` section 6, `docs/api-v1.md`, the OpenAPI file.
5. CHANGELOG, Changed.

**Verification:** the test gate and `make ui-test`.

### Block 10: V1Off

1. The setting `auth.password_login` (true by default), under
   `PUT /api/v1/settings` with `settings`; off, `POST /api/v1/login`
   answers `403` "the password sign-in is off; sign in from the web UI",
   the start says `v1` false, and an API key is not affected.
   Where the CLI kept v1 (Block 8), its `user add` and `user passwd` say that
   the user signs in once in the browser before the setting goes off.
2. The settings panel (`users` partial): the switch, and how many users are
   still on v1 (`auth_version` in `GET /api/v1/users`), its text in all six
   languages.
3. Tests: Go (off refuses, keys pass, the start's `v1`); `tools/ui-test`:
   the switch, and a sign-in with it off.
4. `docs/store.md` section 6, `docs/api-v1.md`, the OpenAPI file,
   `docs/ui.md`.
5. CHANGELOG, Added.

**Verification:** the test gate and `make ui-test`.

### Block 11: Docs

1. `docs/server.md`, "The key scheme": the wrapping key from OPAQUE's export
   key by HKDF, in place of the argon2id-and-HKDF split and the hash of an
   authentication key; "What version 1 prepares", item 6, done;
   "Trusting the code" names the module as part of what the page loads.
2. `docs/store.md`: section 5 (migration 0006), section 6 (the sign-in, the
   lockout at the start, the key file), section 10 items 15 and 17 taken.
3. `README.md` where the CLI's password commands are described;
   the backup's documentation: `opaque.key` with the data directory.
4. No CHANGELOG bullet: no shipped path.

**Verification:** `GUIDELINES/shared/tools/check`; every route of Blocks 6
to 10 in `docs/api-v1.md` and the OpenAPI file.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +PasswordCost v1.0

Password hashing on small boxes (cheaper argon2id parameters, a bound on the hashes at once, one sign-in per address, a rehash at sign-in, no hash where the answer is known) gets its plan registered before the first project write.

Key changes:
- GUIDELINES/wip/2026-11-07_14-38_AP_PasswordCost_password_hashing_on_small_boxes.md: the plan, five blocks, with the owner's measurements on a Raspberry Pi 1 and decisions D1 to D5 of 2026-11-07.

File-revision-v1: GUIDELINES/wip/2026-11-07_14-38_AP_PasswordCost_password_hashing_on_small_boxes.md::v1.0::2026-11-07::All::the first revision
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `perf(store): argon2id at 19 MiB and two passes`
2. `fix(security:store): bound the password hashes that run at once`
3. `fix(security:server): one sign-in at a time from an address`
4. `feat(store): rehash a password at sign-in when its parameters are old`
5. `perf(store): look up the name before hashing in user add and set password`

The revision v1.1, a commit of its own before Block 6:

```
chore(aps): +PasswordCost v1.1

The plan gains auth_version 2 by OPAQUE, Blocks 6 to 11, after Blocks 1 to 5 of pull request 87.

Key changes:
- GUIDELINES/wip/2026-11-07_14-38_AP_PasswordCost_password_hashing_on_small_boxes.md: the sign-in and registration by OPAQUE (RFC 9807), the libraries, the lockout counted at the start, the v1 path and its end, the owner's decisions D6 to D11 of 2026-11-07.

File-revision-v1: GUIDELINES/wip/2026-11-07_14-38_AP_PasswordCost_password_hashing_on_small_boxes.md::v1.1::2026-11-07::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::auth_version 2 by OPAQUE, Blocks 6 to 11
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

6. `feat(server): sign-in and registration by OPAQUE`
7. `feat(ui): the sign-in dialog over OPAQUE`
8. `feat(ui): every password set from the browser registers OPAQUE`
9. `feat(server): a password sign-in in the browser upgrades to OPAQUE`
10. `feat(server): a setting that turns the password sign-in off`
11. `docs(server): the key scheme on OPAQUE's export key`

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate's findings and
  the owner's decisions D1 to D5 of the same day.
- v1.1 (2026-11-07): Blocks 6 to 11, `auth_version` 2 by OPAQUE, from the
  TL;DR gate of `PLAN D5-C` and the owner's `PLAN++` with D6 to D11 as
  recommended; Blocks 1 to 5 marked done in #87; the lockout counted at the
  start, not at the finish as that gate had it.
