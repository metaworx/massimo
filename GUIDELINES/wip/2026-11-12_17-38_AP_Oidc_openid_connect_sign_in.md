# AP Oidc v1.2: OpenID Connect sign-in

## TL;DR

Users sign in through an OpenID Connect provider (Nextcloud, Google, Microsoft, Authentik,
Keycloak) beside the password; Blocks 1-5 are in v0.15.0-alpha.1.
v1.1 fixes what the first real sign-in through breno's Nextcloud showed,
lets users link their own accounts, restricts a provider to its groups,
and seals the client secret (since sealed with a key derived from `opaque.key`, AP SealedSecrets).
v1.2 asks before a link is stored, and explains My profile's controls.

- claims → `id_token` only → also the userinfo endpoint (Nextcloud's OIDC app ≥ 2.0)
- a pre-link → e-mail only → e-mail or the user id at the provider (`sub`), bound at once
- issuer → checked at the sign-in → checked when saved, the provider's own form kept
- a refusal → "unknown account", no reason logged → its reason logged and shown
  under the provider buttons
- linking → the manager's, by e-mail → also the user's own, "Link" in My Profile;
  details (name, user name, groups, last sign-in) on both
- groups → not read → a claim per provider (`groups`), allowed groups; none: any
- client secret → plain in the store → sealed (AES-GCM) with `secret.key` beside it
- a link → stored as it comes back → shown first (account, name, groups, the link it replaces),
  stored at **Link**, dropped at **Cancel**
- My profile → its buttons unexplained → an ⓘ per group: display name, password,
  sessions and keys, linked accounts

### Blocks

1. Store: migration 0016, `auth_providers` and `user_identities`, their functions, tests; docs/store.md
2. Sign-in: discovery, the flow, the four sign-in routes, tests against a fake provider; OpenAPI, docs/api-v1.md
3. Management: capability `auth`, the providers' and identities' routes, audit; OpenAPI, docs/api-v1.md
4. UI: the card's provider buttons, System settings → Sign-in providers, a user's identities; i18n; ui-test; docs/ui.md
5. Guide: setting up a provider, Nextcloud's as the example; README.md
6. The sign-in fixed: userinfo, a pre-link by user id, the issuer at saving, the reasons; tests; docs
7. Linked accounts: migration 0019, the details kept, linking from My Profile, a waiting link changed; UI, tests, docs
8. Groups: migration 0020, the claim and the allowed groups, the refusal; UI, tests, docs
9. The sealed secret: `secret.key`, sealing on write and at the start, a lost key; tests, docs
10. The link confirmed: the pending link, its routes, My profile's confirmation; tests, docs
11. Help in My profile: an ⓘ per group of controls, six languages; ui-test, docs/ui.md

## Discussion

The owner asked on 2026-11-12 for sign-in through OAuth:
a new capability `auth`, one or more providers with a client id and a secret,
remote users (by e-mail) assigned to a user,
and per provider whether users are created on their own.
The evaluation of the same day answered it as OpenID Connect:
OAuth 2 grants access, it does not say who signed in;
OIDC adds the `id_token` and discovery,
which breno's Nextcloud, Google, Microsoft, Authentik and Keycloak speak.

**The owner's decisions**, 2026-11-12, `PLAN++ Oidc` on the evaluation's gate ("as recommended"):

- **D1-A.** OIDC only, a provider found by its issuer's discovery document.
- **D2-A.** A new capability `auth` for the providers, `owner` alone by default;
  a user's identities under `users`, and under `guests` for a guest.
- **D3-A.** Identities by `iss` and `sub`;
  an assigned e-mail is a pre-link, bound at the first sign-in with it and `email_verified`.
- **D4-A.** Auto-create off by default;
  on, with a role the provider names (default `viewer`) and allowed e-mail domains.
- **D5-A.** Several providers from the start.
- **D6-A.** The client secret in the store, never in an API answer.
- **D7-A.** The app (Android, iOS) later, in AP MobileApp:
  providers refuse a sign-in in a WebView, so it needs the system browser and an app link.

**Refined here, routine:**

- **The redirect URI** is the page's own address, which the page sends when it starts a sign-in,
  plus `api/v1/login/oidc/callback`.
  The evaluation proposed a setting `server.public_url`;
  behind a reverse proxy `serve` cannot know its address, but the page does,
  and the provider accepts only the redirect URIs registered with it,
  so a forged address never comes back.
  System settings shows the URI to register.
- **"Never above operator"** (D4-A) is a role holding none of the administrative capabilities
  (`guests`, `users`, `passwords`, `roles`, `roles.disable`, `capabilities`, `cubes`,
  `settings`, `update`, `restart`, `reboot`, `audit`, `auth`), and not `owner`,
  rather than a comparison with `operator`'s set, which the owner may change.
- **The standard library**, no `golang.org/x/oauth2`:
  the flow is a URL, one form POST and a JSON decode.
- **No signature check of the `id_token`**:
  it comes straight from the token endpoint over TLS, which OIDC Core 3.1.3.7 allows in its place;
  so an issuer must be `https`, but on loopback for the tests.
  `iss`, `aud`, `exp` and `nonce` are checked.

**Names chosen here**, routine and not decisions of the owner's:
the tables `auth_providers` and `user_identities`;
the routes below; the actions `provider.add`, `provider.update`, `provider.remove`,
`identity.add`, `identity.remove`;
the scopes `openid email profile`;
a new user's name from `preferred_username`, else the e-mail's local part,
`-2`, `-3` added where taken;
the branch `claude/oidc`.

**Not in this plan:**
plain OAuth 2 providers (GitHub without OIDC, D1-B);
the app's sign-in (D7-A);
a signature check through JWKS;
signing out at the provider (the session is Massimo's).

**Who runs it:**
this session, under the owner's `PLAN++ Oidc` of 2026-11-12:
the plan, its registration on main, and its blocks committed on `claude/oidc`.
No gate stated a push for this plan,
so the completion gate offers the push and the pull request.
A real sign-in through breno's Nextcloud needs a client registered there, the owner's.

### v1.1: after the first real sign-in

Blocks 1-5 were merged (#193) and released in v0.15.0-alpha.1.
The owner registered a client at breno's Nextcloud on 2026-11-13 and found:

- the issuer `https://cloud.casaleone.ch/`, as typed, was refused at the sign-in,
  the provider calling itself without the slash; the check belongs to the saving;
- the refusal showed under the password form, not under the provider's button;
- with the issuer fixed and the owner's e-mail assigned, the sign-in was refused as an unknown account:
  the log has `login failed … user=nextcloud:mdr`, Nextcloud's subject,
  so the code was traded and the `id_token` checked, but no verified e-mail matched.
  Nextcloud's OIDC app puts the e-mail and the profile into the `id_token` since its 2.0
  only where the client asks for them by the `claims` parameter, or an admin setting says so;
  its userinfo endpoint (`/apps/oidc/userinfo`) answers them.
  Which of the two, the e-mail missing or not verified, was not seen: nothing logged it.

The owner asked, beside them, whether the client secret is encrypted in the store (it is not),
for linking by the users themselves in My Profile and by the managers on the user's card,
with the linked account's details shown on both,
and whether the providers' groups could restrict who signs in.

**The owner's decisions**, 2026-11-13, `PLAN++ Oidc` on the evaluation's gate,
which recommended each option taken:

- **D20-A.** The client secret sealed with a key of its own,
  `secret.key` (0600) beside the store's file;
  a backup or a copy of the store's file alone does not hold it;
  a lost key means the secret is entered again.
- **D21-A.** A user links an account in My Profile ("Link" per provider, a sign-in there);
  linking again at a provider replaces the link there;
  removing the last way in (no password, no other linked account) is refused to the user.
  A manager assigns by e-mail or user id, changes a waiting one, removes any.
  Both show the provider, the e-mail, the name and user name there, the groups,
  when it was linked and last used, kept current at each sign-in.
- **D22-A.** Per provider a groups claim (`groups` by default) and allowed groups:
  a sign-in in none of them is refused, a linked user's too; none named: no restriction.
- **D25-A** (D23-A of the first evaluation).
  The userinfo endpoint read after the token exchange, where the discovery names one;
  its `sub` must be the `id_token`'s; it fills what the `id_token` lacks.
- **D26-A.** A pre-link by e-mail or by the user id at the provider (`sub`);
  one by user id is bound from the start.
- **D27-A.** The issuer checked when an enabled provider is saved:
  the discovery document must be reached and name the same issuer,
  but for a trailing slash, where the document's form is kept.
- **D28-A.** A refusal's reason logged (no e-mail, e-mail not verified, no user,
  not in the groups, the account another user's),
  and the page's message for it under the provider buttons.
- **D29-A.** This revision, the fixes first (Block 6), so that they can go out on their own.

**Refined here, routine:**

- **"The last way in"** is a password or another linked account;
  API keys, which sign in on the card too, are not counted,
  being meant for programs.
  A manager may remove any link: the manager can set a password.
- **A changed waiting link**: a manager changes the e-mail or the user id of a link
  not yet used for a sign-in; a used one is removed and assigned again.
- **The subject is answered** (`subject`) to the user and the managers:
  the user id at the provider is no secret, and a manager assigns by it.
- **The groups' values** are read as a list of strings or one string;
  Microsoft Entra sends group ids, which the allowed groups then name.
  `groups` is not added to the default scopes: Google refuses a scope it does not know.
  The guide names it for Nextcloud.
- **The key file** is made at the first secret written, not at every start,
  by `internal/safefile`; a read-only store never makes it.
  A secret is sealed as `v1:` and base64 of the nonce and the AES-256-GCM box,
  the provider's uuid as the additional data,
  so that a sealed secret copied to another row does not open.
  A plain secret left from v0.15.0-alpha.1 is sealed at the next start of a writable store.

**Names chosen here**, routine:
the reasons `noemail`, `unverified`, `refused`, `group`, `taken`;
the routes `POST /api/v1/users/{name}/identities/link/{key}` (the user's own, a link started)
and `PUT /api/v1/users/{name}/identities/{id}`;
the columns `user_identities.name`, `.username`, `.groups`,
`auth_providers.groups_claim`, `.allowed_groups`;
the fragment `#oidc-linked=<key>`;
the branch `claude/oidc-links`.

**Not in this plan:**
a role given or kept in step by the groups (D22-B, D22-C);
a provider trusting e-mails it does not mark verified;
the key in `.env` (D20-B).

**Who runs it:**
this session, under the owner's `PLAN++ Oidc` of 2026-11-13,
whose gate stated the blocks committed on a branch and no push:
this revision committed first on `claude/oidc-links`, then Blocks 6-9.
The completion gate offers the push and the pull request.
The real sign-in through breno's Nextcloud is the owner's.

### v1.2: a link confirmed, and help in My profile

The owner's real sign-in through breno's Nextcloud worked on 2026-11-13
with the e-mail assigned; a first attempt had looked refused because the browser,
still signed in at Nextcloud, never showed its page.
The owner then asked for a confirmation once a link comes back,
since an admin with an admin and a personal account at Nextcloud
could link the wrong one, and for help icons in My profile.

**The owner's decisions**, 2026-11-13, `PLAN++ Oidc` on the evaluation's gate:

- **D38-A.** A link is shown before it is stored:
  the provider, the account's e-mail, name, user name and groups,
  and the link it replaces, if any; **Link** stores it, **Cancel** drops it.
  A wrong account then never replaces a good link.
- **D37-C.** An ⓘ, the page's help button, behind the controls of My profile.
- **D39-A.** This revision, Blocks 10 and 11.

**Refined here, routine:**

- **The pending link** is held in memory like a sign-in's one-time code,
  ten minutes, once, for the user who started the link and from its address;
  a restart drops it.
  The account another user has (`taken`) and one outside the provider's groups (`group`)
  are refused at the callback, before anything is shown.
- **One ⓘ per group of controls**, not per button:
  the display name, the password, the sessions and keys, the linked accounts;
  each explains what its buttons do,
  Link among them (a sign-in at the provider, which then signs the user in here).

**Names chosen here**, routine:
the fragment `#oidc-link-confirm=<code>`;
the routes `GET`, `POST` and `DELETE /api/v1/users/{name}/identities/link/pending/{code}`;
the actions `identity.link` for the confirmation, the change itself,
and `identity.link.start` for the start;
the help keys `help.profileName`, `help.profilePassword`, `help.profileTokens`,
`help.profileLinked`; the branch `claude/oidc-confirm`.

**Who runs it:**
this session, under the owner's `PLAN++ Oidc` of 2026-11-13,
whose gate stated both blocks run on a branch, without a push:
this revision committed first on `claude/oidc-confirm`, then Blocks 10 and 11.

## Analysis

**The flow:**

1. The card lists the enabled providers (`GET /api/v1/login/providers`, no authentication).
2. A click: `POST /api/v1/login/oidc/{key}` with `{"redirect": "<the page's address>"}`.
   The server reads the provider's discovery document (cached for an hour),
   keeps `state`, `nonce`, the PKCE verifier and the redirect for ten minutes, in memory,
   and answers the authorization URL; the page goes there.
3. The provider sends the browser to `GET /api/v1/login/oidc/callback?code=…&state=…`.
   The server takes the state, posts the code with the verifier to the token endpoint
   (`client_secret_basic`, or `client_secret_post` where the provider names only that),
   checks the `id_token`, and finds the user:
   - an identity of the provider with that `sub`;
   - else a pre-link of the provider with that e-mail, bound now, where `email_verified`;
   - else, where the provider creates users and the e-mail's domain is allowed,
     a new user with the provider's role and an identity bound to it;
   - else refused.
   A disabled or expired user is refused.
   It answers `303` to the page with `#oidc=<code>`, a one-time code good for a minute,
   from the same address; a refusal with `#oidc-error=<reason>`.
4. The page posts the code (`POST /api/v1/login/oidc/finish`) and receives the session,
   as from `POST /api/v1/login` (`openSession`), so the token never sits in a URL.

**Abuse:** a start is held to the address's lockout as a password start is,
and a refused callback counts as a failed sign-in of the address and is logged (`login_failed`).

**The store:**
`auth_providers` with `uuid`, `key` (unique among the live ones), `name`, `issuer`,
`client_id`, `client_secret`, `scopes`, `auto_create`, `auto_role_id`, `auto_domains`,
`enabled`, and the versioned columns of `users`;
`user_identities` with `uuid`, `user_id`, `provider_id`, `subject` (NULL until bound),
`email`, `created_at`, `created_by`, `bound_at`, `last_used_at`;
unique `(provider_id, subject)`, and unique `(provider_id, lower(email))` among the unbound.
A removed provider is a tombstone whose identities are deleted;
a removed user's identities go with it.

**The routes:**

| Route | Capability | Action |
|-------|------------|--------|
| `GET /api/v1/login/providers` | none | |
| `POST /api/v1/login/oidc/{key}` | none | `login.start` |
| `GET /api/v1/login/oidc/callback` | none | (its own log line) |
| `POST /api/v1/login/oidc/finish` | none | `login` |
| `GET`/`POST /api/v1/providers` | `auth` | `provider.add` |
| `PUT`/`DELETE /api/v1/providers/{key}` | `auth` | `provider.update`, `provider.remove` |
| `GET`/`POST /api/v1/users/{name}/identities` | `users` | `identity.add` |
| `DELETE /api/v1/users/{name}/identities/{id}` | `users` | `identity.remove` |

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/store/migrations/0016_oidc.sql`, `store.sql` | 1 | the two tables |
| `internal/store/providers.go`, `identities.go`, tests | 1 | the rows and their rules |
| `internal/server/oidc.go`, `oidc_test.go` | 2 | discovery, flow, matching, the sign-in routes |
| `internal/server/providers_api.go`, `auth.go`, `audit.go`, tests | 3 | `auth`, the management routes, actions |
| `docs/api-v1-openapi.yaml`, `docs/api-v1.md` | 2, 3 | every route |
| `internal/server/ui/…` (`login.js`, `index.html`, `partials/providers.html`, `users.js`, `users-panel.html`, `i18n/*.json`) | 3, 4 | the actions' and the capability's texts; the buttons, the panel, the identities |
| `tools/ui-test/test_oidc.py` | 4 | the card's buttons, the panel, the identities |
| `docs/store.md`, `docs/ui.md`, `docs/oidc.md`, `README.md` | 1, 4, 5 | as the code |
| `CHANGELOG.d/` | 1-4 | a fragment per bullet |

### v1.1

**The callback**, after the token exchange:
the userinfo endpoint (`userinfo_endpoint` of the discovery) with the access token;
an answer whose `sub` is not the `id_token`'s fails the sign-in;
one that fails otherwise is logged and the `id_token`'s claims go on alone.
Then, for a sign-in:

1. an identity with that `sub` → its user;
2. else a waiting e-mail, where the e-mail is verified → bound;
3. else a user created, where the provider creates users;
4. else refused, the reason the first that holds:
   `noemail` (no e-mail sent), `unverified` (an e-mail not marked verified), `refused`;
5. the groups (Block 8), where the provider names allowed groups: none of them → `group`.

For a link (Block 7), started by a signed-in user for itself:
the `sub` another user's → `taken`;
else the user's other identities at the provider removed,
this one added bound, or the user's waiting e-mail bound,
and the browser sent back with `#oidc-linked=<key>`, no session made.
Each sign-in and link writes the identity's details:
the e-mail, `name`, `preferred_username` and the groups.

**Saving a provider**, enabled, with its issuer changed or newly enabled:
the discovery read without the cache;
an issuer that differs only by a trailing slash takes the document's;
any other difference, or no answer, is a 400 naming what the document says, or why it failed.

**The secret**: `store.Open` reads `secret.key` where it exists;
`AddProvider` and `UpdateProvider` seal, making the key where none is;
`Provider` and `Providers` open, and a secret that does not open
(the key lost or another's) reads as none, with `SecretLost` set and a warning at the start.

**The routes added or changed:**

| Route | Capability | Action |
|-------|------------|--------|
| `GET /api/v1/users/{name}/identities` | `users`, or the user's own | |
| `POST /api/v1/users/{name}/identities` | `users` | `identity.add`, `email` or `subject` |
| `PUT /api/v1/users/{name}/identities/{id}` | `users` | `identity.update` |
| `DELETE /api/v1/users/{name}/identities/{id}` | `users`, or the user's own but the last way in | `identity.remove` |
| `POST /api/v1/users/{name}/identities/link/{key}` | the user's own | `identity.link` |

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/oidc.go`, `oidc_test.go` | 6-8 | userinfo, reasons, links, groups |
| `internal/server/providers_api.go`, tests | 6-8 | the issuer at saving, the identities' routes |
| `internal/store/identities.go`, `providers.go`, tests | 6-9 | by subject, details, a waiting link changed, groups, sealing |
| `internal/store/migrations/0019_identity_details.sql`, `0020_provider_groups.sql`, `store.sql` | 7, 8 | the columns |
| `internal/store/secret.go`, tests | 9 | the key and the box |
| `internal/server/ui/login.js`, `providers.js`, `users.js`, partials, `i18n/*.json` | 6-8 | the reasons, linking, the details, the groups |
| `tools/ui-test/test_oidc.py` | 6-8 | as the UI |
| `docs/api-v1.md`, OpenAPI, `docs/store.md`, `docs/oidc.md`, `docs/ui.md` | 6-9 | as the code |
| `CHANGELOG.d/` | 6-9 | a fragment per block |

### v1.2

**The callback of a link**, after the token exchange and the groups' check:
the account another user's is `taken`, as before;
else the pending link is kept (the user, the provider, the subject, the details,
the address, ten minutes) under a new code,
and the browser goes back with `#oidc-link-confirm=<code>`.
My profile reads it (`GET …/link/pending/{code}`):
the provider's key and name, the account's details,
and `replaces`, the user's identities at that provider a link would remove.
`POST` stores it (`LinkIdentity`, as Block 7 did at the callback),
`DELETE` drops it; a code unknown, spent, too old, another user's
or from another address is `404`.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/oidc.go`, `oidc_test.go` | 10 | the pending link, its three routes |
| `internal/server/server.go`, `audit.go` | 10 | the routes, `identity.link.start` |
| `internal/server/ui/login.js`, `providers.js`, `users.js`, `partials/users-panel.html`, i18n | 10 | the confirmation in My profile |
| `internal/server/ui/users.js`, `login.js`, `partials/profile.html`, i18n | 11 | the ⓘ per group |
| `tools/ui-test/test_oidc.py`, a test of My profile's help | 10, 11 | as the page |
| `docs/api-v1.md`, OpenAPI, `docs/ui.md`, `docs/oidc.md` | 10, 11 | as the code |
| `CHANGELOG.d/Added/` | 10, 11 | a fragment each |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over `internal/store` and `internal/server`,
`GUIDELINES/shared/tools/check`,
and for Block 4 `make ui-test` (1100 and 390 px).
Nothing reaches a Cube, a box or a real provider; the fakes answer.
One functional commit per block, citing `AP Oidc v1.0, Block <n>`.

### Block 1: Store

1. `0016_oidc.sql` (`min_reader: 1`, additive), `store.sql` regenerated.
2. `providers.go`: `AuthProvider`; `Providers`, `Provider`, `AddProvider`,
   `UpdateProvider`, `DeleteProvider` (a tombstone, its identities deleted);
   the key checked as a role's; the issuer an absolute `https` URL, `http` only on loopback.
3. `identities.go`: `Identity`; `Identities(user)`, `AddIdentity` (a pre-link by e-mail),
   `DeleteIdentity`, `UserBySubject`, `BindIdentity` (a pre-link by e-mail, verified),
   `AddBoundIdentity`, `TouchIdentity`.
4. Tests: the rules above, a provider's removal, a user's removal, the uniqueness.
5. `docs/store.md`: the tables in section 5, the identities in section 6.
6. `CHANGELOG.d/Added/claude-oidc-store.md`.

**Verification:** the test gate.

### Block 2: Sign-in

1. `oidc.go`: discovery (cached an hour, its `issuer` checked);
   the pending starts and the one-time codes in memory, at most `maxTracked`;
   the authorization URL with PKCE S256; the token exchange;
   the `id_token`'s checks; the matching and the auto-create;
   the four sign-in routes.
2. `oidc_test.go` against an `httptest` provider:
   a bound identity signs in; a pre-link binds and then matches by `sub` only;
   an unverified e-mail does not bind; auto-create off refuses, on creates with the role,
   a foreign domain refuses; a disabled user refuses;
   a wrong `iss`, `aud` or `nonce`, an expired `id_token`, an unknown or used state,
   a used code and a code from another address fail;
   the session is `openSession`'s.
3. OpenAPI and `docs/api-v1.md`, "Sign-in through OpenID Connect".
4. `CHANGELOG.d/Added/claude-oidc-sign-in.md`.

**Verification:** the test gate.

### Block 3: Management

1. `auth.go`: `auth` in the capabilities; `login.js`'s `CAPABILITIES` and `cap.auth` in six languages.
2. `providers_api.go`: the providers' routes (the secret written, answered as `has_secret`;
   the auto-create role checked), the identities' routes with the users' access rules.
3. `audit.go`: the actions, their lines; `log.action.*` in six languages.
4. Tests: each route's answers and refusals, the secret never answered,
   a guests holder on a guest's identities only; `TestOpenAPICoversRoutes`.
5. OpenAPI, `docs/api-v1.md` (the routes, the capability), `docs/store.md` section 6's table.
6. `CHANGELOG.d/Added/claude-oidc-providers.md`.

**Verification:** the test gate.

### Block 4: UI

1. The card: a button per enabled provider above the password form,
   the start, `#oidc=` and `#oidc-error=` at load.
2. System settings → Sign-in providers (`needs-auth`), `partials/providers.html`:
   the list, a form to add and change one (key, name, issuer, client id,
   secret written only, enabled, auto-create with role and domains),
   the redirect URI to register; remove with a confirmation.
3. System settings → Users: a user's identities, add (provider, e-mail), remove.
4. i18n in six languages; `docs/ui.md`.
5. `tools/ui-test/test_oidc.py`: the panel adds a provider against a fake provider
   the test serves, the card shows its button, a pre-link signs in through it,
   the identity shows as bound; at 1100 and 390 px.
6. `CHANGELOG.d/Added/claude-oidc-ui.md`.

**Verification:** the test gate with `make ui-test`.

### Block 5: Guide

1. `docs/oidc.md`: what to register at a provider (the redirect URI, the scopes),
   Nextcloud's OIDC app as the example, Google's and Microsoft's in a line each,
   the pre-link and the auto-create, what is not done (signing out at the provider, the app).
2. `README.md`: a paragraph pointing there.

**Verification:** `reflow.sh --check` shows no new warning. No CHANGELOG bullet: documents only.

Blocks 6-9 end with the same test gate, `make ui-test` where the UI changes,
and cite `AP Oidc v1.1, Block <n>`.

### Block 6: The sign-in fixed

1. `oidc.go`: the userinfo read and merged; the reasons `noemail`, `unverified`, `refused`,
   logged at Warn with the provider and the reason.
2. `providers_api.go`: the issuer checked at saving (D27-A);
   `POST …/identities` with `subject` instead of `email`, bound from the start;
   `subject` in the identities' answer.
3. `identities.go`: `AddSubjectIdentity`.
4. The page: the reasons' messages under the provider buttons (`login.js`, `index.html`),
   the manager's form with a choice of e-mail or user id (`providers.js`, `users-panel.html`);
   i18n in six languages.
5. Tests: userinfo filling the e-mail, a `sub` mismatch failing, userinfo down;
   each reason; the issuer with and without the slash, unreachable;
   a pre-link by subject signing in; `test_oidc.py` with the message's place.
6. `docs/api-v1.md`, OpenAPI, `docs/oidc.md` (the issuer, the claims, Nextcloud's setting),
   `docs/ui.md`; `CHANGELOG.d/Fixed/claude-oidc-links-signin.md`.

**Verification:** the test gate with `make ui-test`.

### Block 7: Linked accounts

1. `0019_identity_details.sql`: `user_identities.name`, `.username`, `.groups` (JSON);
   `store.sql`.
2. `identities.go`: the details written at each sign-in and link (`TouchIdentity`),
   `LinkIdentity` (replacing the user's others at the provider, `ErrIdentityTaken`),
   `UpdateIdentity` (a waiting one's e-mail or subject).
3. `oidc.go`: the link's start and its callback; `providers_api.go`: the routes, the user's own,
   the last way in; `audit.go`: `identity.link`, `identity.update`.
4. The page: My Profile → "Linked accounts" (the list with details, Link per provider, Remove),
   the card's list with the details and a change of a waiting one;
   `#oidc-linked=` at load; i18n.
5. Tests: a link, linking again, a `sub` another user's, the last way in, a waiting one changed,
   a used one not; `test_oidc.py` linking from My Profile at 1100 and 390 px.
6. Docs; `CHANGELOG.d/Added/claude-oidc-links-linking.md`.

**Verification:** the test gate with `make ui-test`.

### Block 8: Groups

1. `0020_provider_groups.sql`: `auth_providers.groups_claim` (default `groups`),
   `.allowed_groups`; `store.sql`; `providers.go`.
2. `oidc.go`: the claim read from the `id_token` and the userinfo; the refusal `group`.
3. The page: the provider's form, two fields; the reason's message; i18n.
4. Tests: in a group, in none, a list and a string, no restriction; `test_oidc.py` the fields.
5. Docs; `CHANGELOG.d/Added/claude-oidc-links-groups.md`.

**Verification:** the test gate with `make ui-test`.

### Block 9: The sealed secret

1. `secret.go`: the key (`secret.key`, 0600, `safefile`), `seal` and `open` with the provider's uuid.
2. `providers.go`: sealed on write, opened on read, `SecretLost`;
   `store.Open` seals plain secrets left from v0.15.0-alpha.1.
3. `providers_api.go`: `secret_lost` in the answer; the page says so on the provider.
4. Tests: sealed in the file, opened in the row; a store without the key; a plain secret sealed at the open;
   a sealed secret copied to another row does not open; read-only makes no key.
5. `docs/store.md` (the key beside the store, a backup without it), `docs/oidc.md`;
   `CHANGELOG.d/Changed/claude-oidc-links-secret.md`.

**Verification:** the test gate; `make ui-test` for the page's line.

Blocks 10 and 11 end with the same test gate and `make ui-test`,
and cite `AP Oidc v1.2, Block <n>`.

### Block 10: The link confirmed

1. `oidc.go`: `oidcPendingLink` in `oidcFlows` (at most `maxTracked`);
   the callback keeps it and answers `#oidc-link-confirm=<code>`;
   `GET`, `POST` and `DELETE …/identities/link/pending/{code}`, the user's own.
2. `server.go`, `audit.go`: the routes; `identity.link` the confirmation,
   a change line; `identity.link.start` the start, none.
3. The page: `#oidc-link-confirm=` opens My profile with the confirmation
   at the top of Linked accounts: the details, "This replaces …", **Link** and **Cancel**;
   i18n in six languages.
4. Tests: confirmed, cancelled, twice, too old, another user's, another address,
   a replacement listed; `test_oidc.py`: the confirmation, then linked, and a cancel.
5. `docs/api-v1.md`, OpenAPI, `docs/ui.md`, `docs/oidc.md`;
   `CHANGELOG.d/Added/claude-oidc-confirm-link.md`.

**Verification:** the test gate with `make ui-test`.

### Block 11: Help in My profile

1. `helpParts` (app.js) behind the display name's Save, the password's title,
   the sessions and keys, and the linked accounts; the explanations in six languages.
2. Tests: each group's ⓘ labelled, opening its text; `named_buttons` on My profile.
3. `docs/ui.md`; `CHANGELOG.d/Added/claude-oidc-confirm-help.md`.

**Verification:** the test gate with `make ui-test`.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +Oidc v1.0

The owner asked for sign-in through OAuth with a capability of its own, several providers, remote users assigned by e-mail and auto-created users per provider; the evaluation of 2026-11-12 answered it as OpenID Connect. PLAN++ Oidc of the same day, with D1-A to D7-A.

Key changes:
- GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md: the plan, five blocks, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md::v1.0::2026-11-12::All::the first revision, by PLAN++ Oidc
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(store): OpenID Connect providers and users' identities`;
Block 2, `feat(server): Sign-in through an OpenID Connect provider`;
Block 3, `feat(server): The providers and users' identities through the API, under the capability auth`;
Block 4, `feat(ui): Sign in with a provider, and System settings for providers and identities`;
Block 5, `docs(oidc): Setting up an OpenID Connect provider`.

v1.1, on `claude/oidc-links` before its blocks:

```
chore(aps): Oidc v1.1

The first real sign-in through breno's Nextcloud was refused, and the owner asked for linking in My Profile, groups and a sealed client secret. PLAN++ Oidc of 2026-11-13, with D20-A, D21-A, D22-A and D25-A to D29-A.

Key changes:
- GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md: v1.1, Blocks 6 to 9, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md::v1.1::2026-11-13::All::Blocks 6 to 9, by PLAN++ Oidc
Assisted-by: claude-code:claude-opus-5-5
```

Block 6, `fix(server): An OpenID Connect sign-in reads the userinfo and says why it is refused`;
Block 7, `feat(server): Users link their own accounts at the providers`;
Block 8, `feat(server): A provider admits only the groups it names`;
Block 9, `feat(store): The providers' client secrets are sealed with a key beside the store`.

v1.2, on `claude/oidc-confirm` before its blocks:

```
chore(aps): Oidc v1.2

The owner's real sign-in through breno's Nextcloud worked; the owner asked that a link be shown before it is stored, so that a wrong account cannot replace a good link, and for help in My profile. PLAN++ Oidc of 2026-11-13, with D37-C, D38-A and D39-A.

Key changes:
- GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md: v1.2, Blocks 10 and 11, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_17-38_AP_Oidc_openid_connect_sign_in.md::v1.2::2026-11-13::All::Blocks 10 and 11, by PLAN++ Oidc
Assisted-by: claude-code:claude-opus-5-5
```

Block 10, `feat(server): A link is shown before it is stored`;
Block 11, `feat(ui): My profile explains its controls`.

## Change History

- v1.0 (2026-11-12): the first revision,
  from the owner's request and the evaluation of the same day, with D1-A to D7-A.
- v1.1 (2026-11-13): Blocks 6 to 9, from the first real sign-in through breno's Nextcloud
  and the owner's questions, with D20-A, D21-A, D22-A and D25-A to D29-A.
- v1.2 (2026-11-13): Blocks 10 and 11, a link confirmed and help in My profile,
  with D37-C, D38-A and D39-A.
