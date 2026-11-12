# AP Oidc v1.0: OpenID Connect sign-in

## TL;DR

Users sign in through an OpenID Connect provider (Nextcloud, Google, Microsoft, Authentik,
Keycloak) beside the password:
providers kept in the store, a user's identity matched by the provider's `iss` and `sub`,
an e-mail the owner assigns bound at the first sign-in with it,
users created on their own only where a provider allows it.

- sign-in → name + password or API key → also "Sign in with <provider>" on the card
- flow → authorization code + PKCE, confidential client, `state` and `nonce`;
  `id_token` from the token endpoint over TLS (OIDC Core 3.1.3.7), standard library only
- identity → `(provider, sub)`; owner's e-mail = pre-link, bound once at `email_verified`
- auto-create → off by default; on: a role without administrative capabilities
  (default `viewer`) and the allowed e-mail domains
- providers → new capability `auth`, held by `owner` alone by default;
  a user's identities → `users` (`guests` for a guest)
- client secret → in the store, never in an answer (`has_secret`)
- redirect URI → the page's own address + `api/v1/login/oidc/callback`, registered at the provider

### Blocks

1. Store: migration 0016, `auth_providers` and `user_identities`, their functions, tests; docs/store.md
2. Sign-in: discovery, the flow, the four sign-in routes, tests against a fake provider; OpenAPI, docs/api-v1.md
3. Management: capability `auth`, the providers' and identities' routes, audit; OpenAPI, docs/api-v1.md
4. UI: the card's provider buttons, System settings → Sign-in providers, a user's identities; i18n; ui-test; docs/ui.md
5. Guide: setting up a provider, Nextcloud's as the example; README.md

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

## Change History

- v1.0 (2026-11-12): the first revision,
  from the owner's request and the evaluation of the same day, with D1-A to D7-A.
