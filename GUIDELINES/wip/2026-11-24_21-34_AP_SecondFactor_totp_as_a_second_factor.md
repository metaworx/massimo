# AP SecondFactor v1.0: TOTP as a second factor

## TL;DR

A password sign-in asks for a code from an authenticator app before it gives a session,
for a user who turned it on in My profile or whose role requires it.

- password right → pending step (5 min,
  held to the address) → code or recovery code → session
- covered: OPAQUE, version 1; exempt: providers, keys, the box,
  `MASSIMO_API_TOKEN`
- session marked as passed → only a marked token makes a key, `login.keep()` too
- enrolment in My profile after the password: QR (SVG), secret in fours, link,
  first code, ten recovery codes shown once
- role's flag → enrolment inside the sign-in
- wrong code → lockout, as a wrong password; each code once, ±1 step
- reset: Users' panel, `massimo user totp <name> off`; log and push
- clock: the card warns past 30 s; `/api/v1/system` says if synchronised

### Blocks

1. Store: the factor, the recovery codes, the role's flag, the session's mark
2. Codes: TOTP in Go, the QR code vendored and drawn, the clock's report
3. SecondStep: the pending step, `POST /api/v1/login/second`, the proof,
   the lockout
4. Enrolment: the routes to turn on, off, reset and renew; the command line
5. Keys: a key made only by a token that passed
6. CodeStep: the sign-in card's code step
7. Profile: enrolment in My profile, the reset in Users, the credits' row
8. RoleRequired: the role's flag, enrolment inside the sign-in,
   the roles' editor

## Discussion

### What is decided

The owner, 2026-11-24, on Part 2 of the evaluation
`wip/2026-11-24_19-00_ANALYSIS_SecondFactor_generator_and_second_factor.md`:

- **D20-A.
  TOTP first.**
  Go's standard library covers the codes (HMAC-SHA-1, base32).
  Not chosen: passkeys now (a domain name and a trusted certificate;
  a plan of their own, later), codes by e-mail, nothing.
- **D21-A.
  The QR code by a vendored `rsc.io/qr`**
  (BSD-3-Clause; its pure-Go `coding` and `gf256`,
  without the cgo part) under `third_party/`; the server draws it as an SVG,
  so the page gets no library.
  The secret as text in groups of four and an `otpauth://` link stand beside it
  in any case.
  Credits and licence of included libraries are listed under System info:
  AP PasswordRules' Block 5 makes that list, and this plan adds its row.
- **D22-A.
  A pending step in place of a session.**
  `openSessionFrom` answers a pending step for a password sign-in by a user with a factor;
  `POST /api/v1/login/second` with the pending token and a code or a recovery code answers
  the session.
  The pending token lasts five minutes, is used once and is held to the address.
  The card shows a code step (`autocomplete="one-time-code"`,
  `inputmode="numeric"`) with "Use a recovery code" and Back.
  A proof for one's own password change asks for the code too.
  Not chosen: the code in the same request as the password.
- **D23-A.
  What it covers.**
  Password sign-ins, by OPAQUE and of version 1.
  Exempt: OIDC (the provider's own factor), API keys and an integration's key,
  the command line, `MASSIMO_API_TOKEN`.
  Making a key (`POST …/tokens`,
  the app's `login.keep()`) needs a session that passed the second factor,
  for a user who has one.
  Enrolling offers to end the user's other sessions and keys.
- **D24-A.
  The clock.**
  One step either side; a code taken once (the last step used kept per user);
  enrolment needs a first correct code;
  the code step warns where `serverSkew` is over 30 s;
  `/api/v1/system` says on Linux whether the clock is synchronised (`adjtimex`).
- **D25-A.
  Failures, recovery, reset.**
  A wrong code counts in the lockout as a failed sign-in.
  Ten single-use recovery codes of about 50 bits, kept hashed, shown once.
  Reset by a user with `users` (or the owner) in the user's panel,
  which ends that user's sessions; on the box `massimo user totp <name> off`.
  The user is told in the log and by push where subscribed.
  Turning it off oneself needs a code.
- **D26-B.
  Each user's choice, and the owner may require it per role:**
  a flag of the role; a user of such a role who has none enrols at the next sign-in.
- **D27-A.
  No codes by e-mail:**
  Massimo sends no e-mail, and the guidance rules e-mail out as a factor.
- **D31-A.
  Planned now, built after AP LooseEnds and AP PasswordRules are merged,** which
  change the same sign-in card and forms.

### Decided in the writing

Each as recommended, and each the owner's to change:

- **D60-A.
  The sign-in says its kind.**
  `openSessionFrom` takes how the user signed in: password, provider or key.
  Only a password sign-in by a user with a factor, or whose role requires one,
  is answered a pending step.
  The provider's and the key's paths stay as they are.
- **D61-A.
  A session's row says that it passed** (`tokens.factor`):
  set for a session of the second step and for a provider's session;
  for a key made by a token that has it,
  and for a key made on the box (`massimo user token`, `MASSIMO_API_TOKEN`);
  a key's session takes its key's.
  A session from before the user enrolled has it unset.
- **D62-A.
  A right password clears no count while a second step waits.**
  Today a right password clears the name's and the address's failures
  (`openSession`'s keys, `cleared`).
  Kept so, each right password would wipe the count of the wrong codes before it,
  and the codes could be guessed without end.
  The start's count stays,
  and a right password of version 1 counts once as a start does;
  the second step's success clears them, as `cleared` does today.
- **D63-A.
  "Used once" is one session.**
  A pending token gives one session, or one proof.
  A mistyped code may be entered again within its five minutes,
  three tries in all; the third wrong one spends it.
  Each wrong code counts.
- **D64-A.
  Pending steps live in memory,** as the sign-ins started and the proofs do
  (`opaqueLogins`): a restart ends them, and the card says to sign in again.
  The last step used and the recovery codes are in the store,
  so that a restart replays no code.
- **D65-A.
  A wrong recovery code counts as a wrong code.**
  The codes are kept as HMAC-SHA-256 under a key derived from `opaque.key`,
  as the sealed secrets are:
  50 bits under a plain SHA-256 fall to an offline search of a copied `massimo.db`.
  The card and My profile say how many are left;
  at three or fewer My profile says to renew.
- **D66-A.
  A user of version 1 meets the same step,** through `POST /api/v1/login`;
  the proof to upgrade with (`upgrade`) comes with the session at the second
  step.
  One's own password change by `current_password` takes `code` beside it,
  as a proof is taken only after the code.
- **D67-A.
  Enrolment asks the current password first,** as one's own password change does
  (a proof, or `current_password` for version 1),
  so that someone at an unlocked browser cannot tie the account to another
  phone.
  The OWASP MFA cheat sheet asks to reauthenticate before a factor changes.
- **D68-A.
  The code's terms:**
  a secret of 160 bits (RFC 4226's recommendation), HMAC-SHA-1, six digits,
  30 s;
  the link
  `otpauth://totp/Massimo:<name>@<server's host name>?secret=…&issuer=Massimo`,
  so that two servers stand apart in the app.
  A secret that does not open (`opaque.key` lost) refuses codes and takes recovery codes.
- **D69-A.
  The role's flag** is `roles.second_factor`,
  set by `PUT /api/v1/roles/{key}/second-factor` with `roles`; the owner's role,
  fixed otherwise, takes it from an owner alone;
  the integration's role refuses it, its users signing in by no password.
  It holds at the next password sign-in;
  the running sessions of the role's users stay,
  and Users and My profile say that a factor is required and missing.
- **D70-A.
  The key rule is the caller's:**
  a token makes a key only where it is marked (D61),
  if its user has a factor or its role requires one.
  A holder of `users` making a key for another user is held to the holder's own factor;
  it may reset the other's anyway.
- **D71-A.
  One's own reset is Turn off with a code.**
  The reset route refuses one's own name;
  a lost phone is a recovery code, another holder of `users`, or the box.
- **D72-A.
  The user is told:**
  a log line at Info for a factor turned on, off, reset or renewed,
  and `login_failed` with `"step": "code"` at Warn;
  a notification kind `second_factor` to that user alone, on unless turned off,
  for the same and for a recovery code used.
  A right password with a wrong code tells the user the password is known (OWASP).

### Still open

- **D73.
  How long a session of a factor lasts.**
  Sessions last 30 days from their last use (`SessionLifetime`, `auth.go`),
  so a code at the sign-in guards a month;
  NIST SP 800-63B-4 would have it asked again within 24 hours at AAL2.
  - **A (recommended).
    Leave 30 days.**
    The factor's job here is a stolen password;
    a code a day on a wall tablet is a nuisance a home does not need,
    and sessions can be ended in My profile and by enrolment.
  - **B.**
    A session of a factor lasts 24 hours from its last use.
    Cost: a code a day in every browser;
    the app's fingerprint login is not touched.
  - **C.**
    A setting `auth.factor_session_days`, 30 by default.
    Cost: small; one more setting.
- **D74.
  A binary rolled back would pass the factor by.**
  The migration only adds, so by the store's rule it keeps `min_reader`:
  a binary from before it opens the store and signs every user in with the password alone.
  - **A (recommended).
    The migration raises `min_reader`** to its own version,
    with its down part and the copy `massimo.db.before-<N>`:
    a rollback needs `massimo store downgrade`,
    which drops the factors on the box, where whoever holds it holds everything.
  - **B.
    Additive.**
    Cost: a rollback turns the second factor off without a word.
- **D75.
  Consecutive wrong codes per user.**
  The lockout lets about five codes through in six minutes per name,
  each sign-in's start counting beside its wrong code (D62):
  with three codes valid at a time, about one in three hundred thousand a try,
  a patient attacker who holds the password would pass within months.
  NIST §3.2.2 caps consecutive failures at 100 per authenticator.
  - **A (recommended).
    At 100 wrong codes in a row** the factor takes recovery codes alone until a
    right one or a reset; a right code sets the count to 0;
    the user is told (D72).
  - **B.
    The lockout alone.**
    Cost: as above.

### What stays as it is

- The sign-in through a provider, `POST /api/v1/login/key`, the API keys,
  an integration's key and `MASSIMO_API_TOKEN` (D23).
- The lockout's limits, and its counts surviving a restart.
- The page's sign-in by key in the card, which is a key.
- The app's bridge: its login by fingerprint uses `login/key`.

## Analysis

- **Every session comes from `openSessionFrom`**
  (`internal/server/login_api.go`), through `openSession`:
  from `login` (version 1, and the takeover of an OPAQUE start),
  from `loginFinish` (`opaque_api.go`), from `oidcFinish` (`oidc.go`,
  with `provider` in `extra`) and from `loginKey` (with the key's id).
  It does not know how the user signed in; only `keyID` and `extra` tell.
- **The proof** (`issueProof`, `redeemProof`, `opaque_api.go`) lasts `proofTTL`,
  five minutes, and is spent by its first use; it is held to the user,
  not to the address.
  A sign-in started (`pendingLogin`) holds its name, address,
  lockout keys and purpose, and `login` refuses a takeover from another address.
  The pending second step takes that shape, the address included.
- **The lockout** (`internal/store/lockout.go`,
  `authLimiter` and `storeLimiter` in `auth.go`):
  ten failures in a minute lock a key for five minutes,
  per address and per name.
  `login` counts a wrong password and clears both on success (`openSession`'s
  keys); `loginStart` counts every start at once,
  and `loginFinish` clears it on success (`cleared`,
  which lifts a lockout its own start set).
  Hence D62.
- **Sign-ins started and proofs are in memory** (`opaqueLogins`,
  at most `maxTracked`): "a restart ends them, and the client starts again".
- **Sessions** last `SessionLifetime`, 30 days,
  moved on use at most hourly (`touch`).
  A token's row (`tokens`) holds its kind, its key (`parent_id`, migration 0025)
  and nothing of how it was signed in.
- **Making a key** (`postToken`,
  `users_api.go`) takes any token of the user or of a manager (`targetUser`),
  a key's included, and checks nothing of how it was made.
- **One's own password change** (`putUser`) takes a proof or `current_password`,
  but not from a holder of `users`, who changes its own password with neither;
  that stays as it is, outside this plan.
- **Roles:**
  `Fixed` (owner, integration) makes `UpdateRole` and `SetRoleDisabled` refuse;
  `PUT /api/v1/roles/{key}/disabled` with `roles.disable` is the model for a
  flag of its own.
- **Sealed secrets** (`internal/store/secret.go`): `sealSecret(aad, plain)`,
  AES-256-GCM under a key derived from `opaque.key`;
  `openSecret` reports one that does not open as lost.
- **The store** is at migration 0025;
  AP LooseEnds and AP PasswordRules may take the next numbers.
  An additive migration keeps `min_reader` (`docs/store.md`, "Migrations"): D74.
- **The page:**
  `signIn` in `login.js`, a plain script, runs the OPAQUE start and finish,
  the version-1 takeover with `upgradePassword`,
  and the proof (`currentPassword`).
  The card (`src/signin/Card.tsx`,
  its state in `store.ts`) draws uncontrolled forms in one place of one height
  (UiParts D224).
  `serverSkew` is a plain `let` of `header.js`, which parts already read
  (`header/clock.ts`, `software/store.ts`).
- **No library is needed to show an SVG:**
  the page's policy has `img-src 'self' data:` (`ui.go`),
  so a `data:image/svg+xml` in an `<img>` needs no `innerHTML`.
- **`/api/v1/system`** (`system_api.go`) gives `now` and `time_zone`,
  nothing of the sync; `golang.org/x/sys` is a dependency already.
- **`third_party/`** keeps a module with `LICENSE`,
  `MASSIMO.md` and its `go.mod`, replaced in the root's `go.mod`,
  as `bytemare-ksf` is.
- **Notifications** (`internal/notify`) know Cube and device kinds and `update`;
  none of sign-in.
- **The app** (`Bridge.keep`) makes its key with the page's session through
  `ServerApi.createKey`;
  a refusal other than 401 is shown with the server's words (`err_server`).
  A session of the code step is marked, so `login.keep()` goes on working;
  a refusal reads as the server words it.
  `app/` needs no change.
- **The command line** (`cmd/massimo/user.go`) works on the store directly;
  `user list` shows each user's role, state, password and tokens.

## Implementation Plan

Each block moves its documents with its code,
and ends with the gate of `GUIDELINES/shared/GATE_WORKFLOW.md`.
Block 1 starts once AP LooseEnds and AP PasswordRules are merged (D31).
Blocks 3 to 5 ship routes the page uses from Block 6 on:
no release is cut between Block 3 and Block 7.
Every block's Go gate is the contract's:
`make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`, each exit 0;
`make race` for the touched packages; `gofmt -l .` empty;
`go vet ./...` for linux, windows and darwin, exit 0.
Each new test is seen failing before its change.

### Block 1: Store

- **Goal:**
  the store keeps a user's factor, its recovery codes,
  a role's flag and a token's mark, behind one migration.
- **Changes:**
  - `internal/store/migrations/00NN_second_factor.sql` (the next free number):
    `user_factors` (uuid, `user_id` unique, cascade on delete, `kind` `'totp'`,
    `secret` sealed with the user's uuid as additional data, `last_step`,
    `failures`, `created_at`, `version`, `origin`); `recovery_codes` (`user_id`,
    `mac`, `used_at`); `roles.second_factor` and `tokens.factor`,
    integers defaulting to 0.
    `min_reader` by D74, with `00NN_second_factor.down.sql`.
  - `internal/store/factors.go` (new): `Factor`,
    `SetFactor` (with ten new recovery codes, answered once),
    `DropFactor` (with the user's sessions where asked),
    `UseStep` (true once per step, the step above `last_step`,
    in its transaction), `FactorFailed` and the count of D75, `UseRecovery`,
    `RenewRecovery`, `RecoveryLeft`.
  - `internal/store/secret.go`:
    the recovery codes' MAC key, derived as the seal key is.
  - `roles.go`:
    `Role.SecondFactor`, `SetRoleSecondFactor`, the owner's row allowed,
    the integration's refused.
  - `tokens.go`:
    `Token.Factor`; `AddToken` takes it; `AddKeySession` copies the key's;
    `RevokeTokensExcept` for the enrolment's "end the others".
  - `users.go`:
    `User.HasFactor` read with the user.
- **Tests:**
  `internal/store/factors_test.go`: a factor set, read,
  its secret sealed and lost with another `opaque.key`;
  a step used once and an older one refused; ten codes, each once,
  a wrong one counted; renewed codes replace the old;
  the factor gone with the user.
  `roles_test.go`: the flag on owner, refused on integration.
  `tokens_test.go`: a key's session takes its mark.
  The migration's up and down (`TestRaisingMigrationsHaveDownParts` under
  D74-A).
- **Documents:**
  `docs/store.md` (the schema, "Migrations", section 6 "The second factor").
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-store.md`:
  "- The store keeps each user's second factor, its recovery codes,
  a role's requirement of one and which sessions passed it (migration 00NN)."
  Under D74-A also `Changed`: a binary before it opens the store after
  `massimo store downgrade`.
- **Commit:**
  `feat(store): A user's second factor, its recovery codes and a role's flag`
- **Verification:**
  the Go gate; `make race` for `internal/store`; `make cross`, exit 0.

### Block 2: Codes

- **Goal:**
  TOTP codes in Go, the QR code drawn as an SVG, the clock's report.
- **Changes:**
  - `third_party/rsc-qr/`:
    `coding` and `gf256` of `rsc.io/qr`, its `LICENSE`,
    `MASSIMO.md` (what was taken and why), its `go.mod`;
    the `require` and `replace` in `go.mod`.
    Then `make credits`, so that `go run ./tools/credits -check` and its Go test pass.
  - `internal/totp` (new): `NewSecret` (160 bits), `Code(secret, step)`,
    `Check(secret, code, now, last)` (±1 step, the step it matched),
    `Groups` (the base32 secret in fours), `URI` (D68), `QR(uri)`,
    a `data:image/svg+xml` of `<rect>`s at level M and the smallest version that
    holds it.
  - `internal/server/clock_linux.go`, `clock_other.go`:
    `unix.Adjtimex` with no change, synchronised unless `STA_UNSYNC`;
    `system_api.go`: `clock_synced`, absent off Linux.
- **Tests:**
  `internal/totp/totp_test.go`:
  RFC 6238 Appendix B's SHA-1 rows (eight digits through the digits' parameter),
  six-digit codes, the window's edges, a step not taken twice; `qr_test.go`:
  a known link's module matrix and the SVG's size; `system_api_test.go`:
  `clock_synced` on Linux.
- **Documents:**
  `docs/api-v1.md` and the OpenAPI file (`/api/v1/system`),
  `internal/server/ui/src/api.d.ts` by `make api-types`.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-clock.md`:
  "- `/api/v1/system` says whether the server's clock is synchronised
  (`clock_synced`, Linux)."
- **Commits:**
  `build(third_party): rsc.io/qr's encoder, vendored`,
  `feat(api): The system's answer says whether the clock is synchronised`
- **Verification:**
  the Go gate; `make cross` (the vendored code on armv6 and 386), exit 0;
  `go run ./tools/apitypes -check`, `TestOpenAPICoversRoutes`.

### Block 3: SecondStep

- **Goal:**
  a password sign-in by a user with a factor ends in a pending step,
  and the code gives the session.
- **Changes:**
  - `login_api.go`:
    `openSessionFrom` takes the kind (D60);
    for a password with a factor it keeps a pending step and answers
    `{"second": "totp", "pending", "expires_at", "recovery_left"}`,
    the lockout's counts left standing (D62).
  - `opaque_api.go`:
    `pendingSecond` beside `pendingLogin` (user, address, its keys,
    the start's `locked`, purpose, `extra`, tries);
    `loginFinish` with purpose `proof` answers the pending step too.
  - `second_api.go` (new):
    `POST /api/v1/login/second` with `pending` and `code` or `recovery`:
    the address checked, the step used once,
    a wrong one counted against the name and the address and in D75's count,
    the session marked (D61) and the `upgrade` proof (D66), or the proof;
    then `cleared`.
  - `users_api.go`:
    `current_password` takes `code` for a user with a factor.
  - `oidc.go`:
    the provider's session marked.
  - `systemlog.go` and `internal/notify`: the lines and the kind of D72.
- **Tests:**
  `internal/server/second_test.go`: no step without a factor;
  the step for OPAQUE and for version 1; a right code, the session marked;
  a code taken twice; a step either side; three tries; another address;
  five minutes passed; a recovery code once; a wrong code counted,
  the password's success clearing nothing; the proof through the step;
  `current_password` without a code;
  a provider's and a key's session without a step; the notification sent.
- **Documents:**
  `docs/api-v1.md` ("Authentication", the sign-in routes, the lockout),
  the OpenAPI file, `api.d.ts`, `docs/store.md` (section 6, "The lockout").
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-step.md`:
  "- A password sign-in by a user with a second factor asks for its code or a
  recovery code (`POST /api/v1/login/second`)."
- **Commit:**
  `feat(api): A password sign-in asks for the second factor's code`
- **Verification:**
  the Go gate; `make race` for `internal/server`, exit 0;
  `go run ./tools/apitypes -check`, `TestOpenAPICoversRoutes`.

### Block 4: Enrolment

- **Goal:**
  a user turns the factor on and off; a manager and the box reset it.
- **Changes:**
  - `second_api.go`, under `/api/v1/users/{name}/second-factor`: `GET` (kind,
    since, recovery codes left); `POST`,
    oneself with a proof or `current_password` (D67):
    a secret kept in memory for ten minutes, answered with its groups,
    the link and the QR; `PUT` with that and a first code: the factor stored,
    the ten codes answered once,
    `end_others` ending the user's other sessions and keys,
    the calling session marked; `DELETE`,
    oneself with a code or a recovery code,
    or by a holder of `users` who outranks the user, ending its sessions,
    one's own refused (D71); `POST …/recovery` with a code: ten new codes.
  - `users_api.go`, `login_api.go`:
    `second_factor` in a user's view and in `/me`.
  - `cmd/massimo/user.go`:
    `user totp <name> off`, its sessions ended; `user list` shows who has one.
- **Tests:**
  `second_api_test.go`: each route by self, manager, a lower role,
  the open server; enrolment without the password, with a wrong first code,
  after ten minutes; the codes once; the others ended;
  the reset's push and log line.
  `cmd/massimo/user_test.go`: `totp off` and `list`.
- **Documents:**
  `docs/api-v1.md` (Users, a section "The second factor"), the OpenAPI file,
  `api.d.ts`, `README.md` (the `user` command).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-enrol.md`:
  "- A user turns a second factor on and off through
  `/api/v1/users/{name}/second-factor`; `massimo user totp <name> off`."
- **Commits:**
  `feat(api): A user turns a second factor on and off`,
  `feat(cli): massimo user totp <name> off`
- **Verification:**
  the Go gate; `make race` for `internal/server`; `make cross`, exit 0;
  `go run ./tools/apitypes -check`, `TestOpenAPICoversRoutes`.

### Block 5: Keys

- **Goal:**
  a key is made only by a token that passed the second factor (D23, D70).
- **Changes:**
  - `users_api.go`, `postToken`:
    `403` with "sign in with your second factor to make a key" for an unmarked
    token where its user has a factor or its role requires one;
    the key made marked.
  - `cmd/massimo/user.go` and the owner of `MASSIMO_API_TOKEN` (`auth.go`):
    their keys marked.
- **Tests:**
  `users_api_test.go`: a key by a marked session,
  by a session from before the enrolment, by a key,
  by a manager for another user, by a user without a factor.
  The app: none in `app/`; `docs/app.md` says the refusal.
- **Documents:**
  `docs/api-v1.md` (Tokens), the OpenAPI file, `docs/app.md` (`login.keep()`).
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-second-factor-keys.md`:
  "- A user with a second factor makes an API key only from a session that
  passed it, the app's login by fingerprint included."
- **Commit:**
  `feat(api): A key is made only by a session that passed the second factor`
- **Verification:**
  the Go gate; `make race` for `internal/server`, exit 0.

### Block 6: CodeStep

- **Goal:**
  the sign-in card asks for the code and gives the session.
- **Changes:**
  - `login.js`:
    `signIn` goes on with the pending step, for a session and for a proof;
    the upgrade after it.
  - `src/signin/store.ts`, `Card.tsx`, a part `Code.tsx`:
    the code field (`autocomplete="one-time-code"`, `inputmode="numeric"`,
    six digits), "Use a recovery code" (a text field), Back to the password,
    in the forms' one place; the server's refusals, the tries left,
    five minutes passed; the notice where `serverSkew` passes 30 s.
  - The proof in My profile goes through the same step.
  - The texts in six languages, FR, IT, NL and PL machine-translated and marked.
- **Tests:**
  `tools/ui-test/test_second_factor.py` (new): the step after the password,
  a right code, a wrong one, a recovery code, Back, the skew notice,
  the step for a proof;
  the code computed by the test from the secret (Python's `hmac`, `base64`);
  the guard's region of the card;
  the card's measures at 390 px in six languages.
- **Documents:**
  `docs/ui.md` (the sign-in).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-card.md`:
  "- The sign-in card asks for the second factor's code, or a recovery code."
- **Commit:**
  `feat(ui): The sign-in card asks for the second factor's code`
- **Verification:**
  `make ui-check`, exit 0; `make ui-test` for the new file and `test_login.py`,
  `test_opaque.py`, `test_ways.py`, `test_parts_guard.py`, four cases;
  the whole UI suite in two cases; screenshots at 390 and 1100 px.

### Block 7: Profile

- **Goal:**
  a user turns the factor on and off in My profile;
  a manager resets it in Users.
- **Changes:**
  - `src/profile/` (a part `Factor.tsx`, `send.ts`, `store.ts`): Turn on,
    the password asked; the QR as an `<img>` with its alt text,
    the secret in fours copyable, the link; the first code;
    the ten codes shown once with Copy and "I have kept them";
    "End my other sessions and keys"; Turn off and New recovery codes,
    each with a code; the codes left.
  - `src/admin/Users.tsx`, `sendUsers.ts`: the factor's badge,
    Reset in the panel, which asks first and says the sessions end.
  - System info: `rsc.io/qr` shows among the credits (AP PasswordRules' Block 5).
  - The texts in six languages, marked as in Block 6.
- **Tests:**
  `test_second_factor.py`: enrolment end to end, a wrong first code,
  the codes once, off with a code, renewal, the others ended; `test_users.py`:
  the reset; `test_system_info.py`: the row; the guard's region of My profile.
- **Documents:**
  `docs/ui.md` (My profile, System settings → Users, System info).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-profile.md`:
  "- My profile turns a second factor on with a QR code, and Users resets it."
- **Commit:**
  `feat(ui): My profile turns a second factor on and off, Users resets it`
- **Verification:**
  as Block 6's, with `test_users.py`, `test_system_info.py`,
  `test_personal_updates.py`.

### Block 8: RoleRequired

- **Goal:**
  a role may require a factor, and its users without one enrol at the sign-in.
- **Changes:**
  - `roles_api.go`:
    `PUT /api/v1/roles/{key}/second-factor` (D69);
    the flag in the role's view and in `/me`'s role.
  - `login_api.go`, `second_api.go`:
    a password sign-in by a user of such a role without a factor answers
    `{"second": "enrol", "pending", "secret", "uri", "qr"}`;
    `POST /api/v1/login/second` with the first code stores the factor
    and answers the session with the ten codes.
    No session with rights exists before.
    A provider's user is not asked (D23);
    a user with keys only has no password sign-in.
  - `src/signin/`:
    the enrolment step in the card, the QR, the secret, the link,
    the first code, then the codes and "I have kept them" before the page loads.
  - `src/admin/Roles.tsx`, `sendRoles.ts`: the switch "Requires a second factor"
    in each role's panel, saved at once, on the owner's only for an owner,
    with the role's users still without one.
  - `src/profile/`:
    a notice where the role requires a factor and the user has none.
  - The texts in six languages, marked as in Block 6.
- **Tests:**
  `internal/server/roles_api_test.go` and `second_test.go`:
  the flag by each role, the owner's by an owner alone;
  the enrolment inside the sign-in, a wrong first code, five minutes passed;
  a provider's user; `test_second_factor.py`: the card's enrolment;
  `test_admin_updates.py`: the switch.
- **Documents:**
  `docs/api-v1.md` (Roles, the sign-in), the OpenAPI file, `api.d.ts`,
  `docs/ui.md` (the sign-in, Roles), `docs/store.md` (section 6).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-second-factor-roles.md`:
  "- A role can require a second factor;
  its users without one set it up at their next sign-in."
- **Commits:**
  `feat(api): A role can require a second factor`,
  `feat(ui): A user whose role requires a second factor sets it up at the sign-in`
- **Verification:**
  the Go gate and `make race` for `internal/server`; `make ui-check`;
  as Block 6's UI runs, with `test_admin_updates.py`;
  at the end the whole UI suite in all four cases and `make cross`, each exit 0.

## Proposed commit message

The plan: `chore(aps): +SecondFactor v1.0`

Block 1:

```text
feat(store): A user's second factor, its recovery codes and a role's flag

AP SecondFactor v1.0, Block 1, SecondFactor D25, D61, D65 and D74: the
store had no place for a second factor. Migration 00NN adds a user's
factor, its secret sealed under opaque.key with the user's uuid as
additional data, the last step used and the count of wrong codes; ten
single-use recovery codes, kept as HMAC-SHA-256 under a key derived
from opaque.key; a role's flag that requires a factor; and a token's
mark that it passed one, which a key's session takes from its key.
Nothing reads them yet.

Key changes:
- internal/store/migrations/00NN_second_factor.sql and its down part
- internal/store/factors.go (new): the factor, the step used once, the
  recovery codes
- internal/store/secret.go, roles.go, tokens.go, users.go: the MAC key,
  the role's flag, the token's mark
- internal/store/factors_test.go (new), roles_test.go, tokens_test.go
- docs/store.md: the schema, the migration, "The second factor"
- CHANGELOG.d/Added/claude-second-factor-store.md: the bullet

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-24): first revision.
