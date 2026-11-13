# AP SealedSecrets v1.0: one key seals the secrets

## TL;DR

The secrets in `massimo.db` are sealed with a key derived from `opaque.key`,
not with a `secret.key` of their own, and the Web Push key is sealed too,
before any release carries `secret.key`.

- sealing key → `secret.key`, a file of its own → HKDF-SHA256 over `opaque.key`, no new file
- a sealed value → `v1:` → `v2:`; a `v1:` one opened once with a `secret.key` found,
  sealed again, the file renamed `secret.key.retired`
- `push.vapid` (the VAPID key pair) → plain JSON in `settings` → sealed, the setting's key
  its additional data; a plain one sealed at the next start
- a store without `opaque.key`'s bytes → the secrets lost, the push key made anew,
  the browsers subscribing again at their next visit

### Blocks

1. The derived key: `secret.go`, `v2:`, a `secret.key` found; tests; the documents
2. The push key sealed: a sealed setting in the store, `push.vapid` through it; tests; the documents

## Discussion

AP Oidc v1.1, Block 9 (#213) sealed the providers' client secrets with `secret.key`,
a key file of its own beside `massimo.db`.
The owner asked on 2026-11-13 what else it should seal,
and whether `opaque.key` and `secret.key` should not be one key, or one derived from the other.
The evaluation answered: derived, since `opaque.key` cannot change without resetting every password,
both files sit in the same directory with the same owner and mode,
and no release nor rolling build carries `secret.key` yet
(`release/devel` and `release/nightly` were both behind #213 on 2026-11-13);
and that the one other secret in the store is the Web Push key, `push.vapid`.

**The owner's decisions**, 2026-11-13:

- **D30-A.** The VAPID key sealed (on the evaluation of what `secret.key` may seal).
- **D32-A.** The sealing key derived from `opaque.key`, `secret.key` dropped.
- **D35-A.** A `secret.key` found, made by a build of main between #213 and this plan:
  its secrets opened once, sealed again with the derived key,
  the file renamed `secret.key.retired` and the renaming logged, for the owner to delete.
- **D36-A.** The push key lost (only with `opaque.key` lost, which resets every password anyway):
  a new key pair made; the page already unsubscribes a subscription made for another key
  and subscribes again.

D35-A and D36-A were the recommendations of the gate on which the owner sent
`PLAN++ SealedSecrets`.

**Refined here, routine:**

- **The derivation**: HKDF-SHA256 (`crypto/hkdf`) over the 96 bytes of `opaque.key`
  (the AKE private key and the OPRF seed), no salt,
  the info `massimo sealed secrets v2`, 32 bytes:
  the two uses stay apart by the label.
- **Read-only stores** read `opaque.key` where they can, as for OPAQUE today,
  and open the secrets with it; where they cannot, the secrets read as lost.
- **The sealed setting** keeps the JSON string `"v2:…"` in `settings.value`,
  the setting's key as the additional data,
  so that a box moved to another setting does not open.
  `Setting` of a sealed one answers the box; `SecretSetting` opens it.
- **The release dumps** in `internal/store/testdata/releases/` hold no provider
  and may hold a plain `push.vapid`: the open seals it, as on a box.

**Names chosen here**, routine:
`SecretSetting`, `SetSecretSetting`, `sealedSettings` (the list of settings kept sealed);
`secret.key.retired`; the branch `claude/sealed-secrets`.

**Not in this plan:**
the Cube keys in the store (D31-A: dropped with the portal code);
the push subscriptions' own keys (D30-B):
a push service refuses a message without the VAPID signature.

**Who runs it:**
this session, under the owner's `PLAN++ SealedSecrets` of 2026-11-13,
whose gate stated the blocks run on a branch, without a push:
the plan registered on main, both blocks committed on `claude/sealed-secrets`.

## Analysis

**Today**, on main: `internal/store/secret.go` reads or makes `secret.key` (PEM, 32 bytes)
at the first secret sealed (`secretKey`), seals `"v1:"` + base64 of the nonce and the
AES-256-GCM box with the provider's uuid as additional data (`sealSecret`),
opens it (`openSecret`, a plain value as it is),
and seals plain client secrets at the open of a writable store (`sealPlainSecrets`).
`internal/store/opaque.go` (`loadOpaque`) reads or makes `opaque.key` at every open,
and keeps only the OPAQUE server built from it.
`internal/server/push_api.go` (`vapidKey`) reads and writes `push.vapid` with
`Setting` and `SetSetting`; the settings API never answers it.

**After:** `loadOpaque` keeps the derived key (`s.secret`) beside the OPAQUE server;
`sealSecret` seals `v2:` with it; `openSecret` opens `v2:` with it,
`v1:` with `secret.key` where one is found, a plain value as it is.
At the open of a writable store, `sealPlainSecrets` seals every plain or `v1:` value as `v2:`,
the providers' secrets and the settings of `sealedSettings`;
where every `v1:` value opened, `secret.key` becomes `secret.key.retired`.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/store/secret.go`, `opaque.go`, `store.go`, tests | 1, 2 | the derived key, `v2:`, `v1:` and `secret.key`; the sealed settings |
| `internal/store/settings.go` | 2 | `SecretSetting`, `SetSecretSetting` |
| `internal/server/push_api.go`, tests | 2 | `push.vapid` through them |
| `internal/store/providers.go`, `internal/server/providers_api.go` | 1 | the comments that name `secret.key` |
| `docs/store.md`, `docs/oidc.md`, `docs/ui.md`, `README.md`, `packaging/README.md`, `docs/api-v1.md` | 1, 2 | the key, the push key |
| `CHANGELOG.d/Changed/claude-oidc-links-secret.md` | 1 | the unreleased bullet, rewritten |
| `CHANGELOG.d/Changed/claude-sealed-secrets-push.md` | 2 | the fragment |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over `internal/store` and `internal/server`,
and `GUIDELINES/shared/tools/check`; `make ui-test` for the page's text in Block 1.
Nothing reaches a Cube, a box or GitHub but the plan's registration.
One functional commit per block, citing `AP SealedSecrets v1.0, Block <n>`.

### Block 1: The derived key

1. `opaque.go`: `loadOpaque` keeps the derived key, read or made with `opaque.key`.
2. `secret.go`: `v2:` with the derived key; `v1:` opened with a `secret.key` found;
   `sealPlainSecrets` seals plain and `v1:` values as `v2:`,
   then renames `secret.key` to `secret.key.retired` where every `v1:` value opened, and logs it.
3. The comments of `providers.go` and `providers_api.go`;
   `providers.secretLost` in six languages names `opaque.key`.
4. Tests: `v2:` in the file and no `secret.key` made;
   a `v1:` secret and its `secret.key` sealed again, the file retired;
   a `v1:` secret without its key lost, the file left;
   another `opaque.key` loses the secrets; a read-only store opens them.
5. The documents; the unreleased bullet rewritten:

   ```
   - The sign-in providers' client secrets are sealed with a key derived
     from `opaque.key`, so that a copy of `massimo.db` alone does not hold
     them; those kept plain are sealed at the next start.
   ```

**Verification:** the test gate with `make ui-test`.

### Block 2: The push key sealed

1. `settings.go`: `SecretSetting` and `SetSecretSetting`; `sealedSettings` holds `push.vapid`.
2. `secret.go`: `sealPlainSecrets` seals those settings too.
3. `push_api.go`: `vapidKey` through them; a box that does not open is logged
   and a new key pair made (D36-A).
4. Tests: `push.vapid` sealed in the file, the same public key after a restart,
   a plain one sealed at the open, a box moved to another setting lost,
   a lost one replaced by a new pair.
5. `docs/store.md` (`push.vapid`), `docs/api-v1.md` ("Notifications");
   `CHANGELOG.d/Changed/claude-sealed-secrets-push.md`:

   ```
   - The Web Push key is kept sealed in the store, as the sign-in
     providers' client secrets are.
   ```

**Verification:** the test gate.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +SealedSecrets v1.0

The client secrets are sealed with secret.key, a key file of its own, which no release carries yet; the owner decided on 2026-11-13 to derive the sealing key from opaque.key instead and to seal the Web Push key too. PLAN++ SealedSecrets, with D30-A, D32-A, D35-A and D36-A.

Key changes:
- GUIDELINES/wip/2026-11-13_13-56_AP_SealedSecrets_one_key_seals_the_secrets.md: the plan, two blocks, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-13_13-56_AP_SealedSecrets_one_key_seals_the_secrets.md::v1.0::2026-11-13::All::the first revision, by PLAN++ SealedSecrets
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(store): The secrets are sealed with a key derived from opaque.key`;
Block 2, `feat(store): The Web Push key is kept sealed`.

## Change History

- v1.0 (2026-11-13): the first revision,
  from the owner's D30-A and D32-A and the gate's D35-A and D36-A.
