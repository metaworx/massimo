# AP PasswordCost v1.0: password hashing on small boxes

## TL;DR

On a one-core box with 426 MB, a password check should be cheap,
limited in how many run at once,
and skipped where the answer is known without one.

- argon2id m=64 MiB t=1 p=4 → m=19 MiB t=2 p=1 (OWASP):
  memory per hash −70 %, time −40 %
  (one core: 48 → 29 ms; a Pi 1 ≈ 4.5 → 2.7 s, scaled)
- hashes at once: unbounded → min(cores, 4) running, 8 waiting,
  then `503` with `Retry-After`
- sign-ins per address: unbounded → one at a time, a second gets `429`;
  the sign-in dialog sends one at a time
- old hashes: verified as stored → rehashed at the next successful sign-in
- user add of a taken name, set password of a missing user:
  hash then the error → the error without a hash
- parameters accepted when verifying: any → m ≤ 256 MiB, t ≤ 16, p ≤ 16
- measured by benchmarks and tests (bytes per hash, hashes in flight,
  hashes counted), not on hardware; `docs/store.md` section 6 with the code

### Blocks

1. Parameters: the new defaults, the ceiling, the benchmarks, a memory test
2. Gate: hashes at once bounded in the store, `503` when the queue is full
3. OneAtATime: one sign-in per address, the dialog's single submit
4. Rehash: a stale hash replaced at a successful sign-in
5. CheckFirst: user add and set password refuse before hashing

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

## Analysis

- **What costs.** `internal/store/password.go` hashes with
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

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing runs on the owner's Pi or reaches a Cube.
Each block is one functional commit citing `AP PasswordCost v1.0, Block <n>`,
with its CHANGELOG bullet; one pull request for the run,
`origin/main` merged into the branch first.

### Block 1: Parameters

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

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate's findings and
  the owner's decisions D1 to D5 of the same day.
