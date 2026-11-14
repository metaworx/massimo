# AP IntegrationUsers v1.0: Integrations act for users

## TL;DR

A registered integration, Home Assistant first, acts for its own users. Each call names the user
and the call's level, and Massimo allows only what every link in the chain allows.

- **Registration:** a request without a key → a short code on both sides → the owner approves and
  sets the maximum → the key, issued once. Zeroconf instances are listed as hints.
- **Fixed role `integration`:** like `owner`, unchangeable.
  - The ceiling: no owner capabilities, `passwords`, `users`, `roles`, `roles.disable`,
    `capabilities` or `auth`; `guests` is allowed.
  - Today's holders of that role → a new role "integration (before 1.0)", as they are.
- **Identities:** `user_identities` → one table for OIDC providers and integrations.
  - A breaking change for v0.15.0-alpha* stores; `min_reader` stays 1.
- **A call's capabilities:** the integration user's role ∧ `integration` ∧ the linked Massimo user
  ∧ the key's maximum ∧ the call's level.
- **Header:** `X-MassimoHA-On-Behalf-Of: id=…; level=…; name*=…; instance=…`.
  - Another installation's id → refused until the owner accepts it.
- **Linking:** proposed by name, confirmed by the owner; several Home Assistant users → one
  Massimo user.
- **`_massimo._tcp`:** gains `url`, `loc`, `reg` and `api=1`. **`features`:** `integration: 1.0`.

### Blocks

1. Identities: `user_identities` rebuilt, `integrations`, the fixed role, D11's move (breaking)
2. Registration API: requests, short code, approval, key once, revoke, installation check
3. On behalf of: the header, users made on first sight, the capability chain, audit, `features`
4. Linking: proposals by name, confirm, unlink; the users list's fields
5. mDNS: `_home-assistant._tcp` browsed; `url`, `loc`, `reg`, `api` in `_massimo._tcp`
6. Web UI: the Integrations section, the users list's markers and links
7. Docs, and the note to the integration session

## Discussion

**The request** came from the session "HA Integration 1" at the owner's word on 2026-11-14.
- Every Home Assistant user writes to Massimo under the integration's one API key, so Massimo
  can't tell who did what.
- The integration knows: each service call's context names the Home Assistant user, and Home
  Assistant checks the call's level per entity (`admin`, `control`, `view`).
- Its side, D55-B and D56: a call without a user names `system`. The integration builds its part
  only once Massimo's is on `main`, exactly as specified here.

**What Home Assistant gives** (the integration session's answers, from Home Assistant 2026.10's
source):
- **A stable key:** `user.id`, `uuid4().hex`, made once, never changed or reused. Names aren't
  unique.
- **What changes at any time:** the display name, the sign-in name, and membership in admin/
  users/read-only. So the level travels with each call and is never stored.
- **One user, several sign-ins:** one user may hold several, and stays one id. One person may
  still have two users; only Massimo can join them.
- **Calls without a user:** automations carry none (`system`). A script keeps the user who
  started it. System-generated users (Supervisor, Cloud, …) have real ids.
- **The installation id** (`core.uuid`): public on the LAN through zeroconf, and copied by a
  backup restore. It changes only by the "duplicate instance id" repair. A label, never proof.

**Decided** by the owner (the D numbers of the evaluation of 2026-11-14):

- **D1-C:** Massimo proposes a link by name, and the owner confirms it.
  - Each integration identity carries `user_id`, the Massimo user it acts as.
  - Several Home Assistant users may point to the same Massimo user.
- **D2-A:** Home Assistant's system-generated users come with their real ids, marked
  `system_generated`, and appear in the users list like any other.
- **D3-A:** one header, `X-MassimoHA-On-Behalf-Of`. Its parameters: `id` (32 hex, or `system`),
  `level`, `name*` (RFC 8187) and `instance`.
- **D4-B:** a call whose `instance` differs from the one recorded is refused until the owner
  accepts the new id (a restore brings the old id back, so nothing breaks).
- **D5-C:** Home Assistant asks and the owner approves with a short code; the instances Massimo
  sees by zeroconf are listed as hints.
- **D6-A:** a new integration user has the role `integration`, which the owner may narrow per
  user, never widen.
- **D7-A, with the owner's ceiling:**
  - the levels: `view` reads; `control` adds `setpoint`, `lock` and `configure` (Home Assistant
    asks control for `set_program`); `admin` adds nothing of its own;
  - an integration never holds the owner's capabilities, `passwords` or `users`;
  - `guests` is allowed: guest management belongs in the integration.
- **D8-A:** `_massimo._tcp` gains `url`, `loc`, `reg`, and `api=1` (the full `features` map
  stays in `/healthz`, a TXT entry being 255 bytes at most).
- **D9-A:** the ceiling also leaves out `roles`, `roles.disable`, `capabilities` and `auth`, so an
  integration can never widen its own users.
- **D10-C, without raising `min_reader`:** one identities table for OIDC providers and
  integrations, rebuilt. The owner marks it a breaking change: it concerns only stores of
  v0.15.0-alpha*, the releases since OIDC, which can't open a store again after the migration.
- **The fixed role `integration`** (the owner's design of 2026-11-14): like `owner`, it can't be
  edited, disabled or deleted. Its capabilities are the ceiling, and it is the ceiling of every
  call through an integration (A2).
- **D11-A:** the users holding the role `integration` today (every API key made for an
  integration so far) move to a new, ordinary role "integration (before 1.0)" with exactly their
  capabilities. Nothing widens.

## Analysis

### A1. The model

- **`integrations`**, one row per registered integration:
  - `uuid`, `kind` (`homeassistant`) and `name` (Home Assistant's `location_name`);
  - `instance_id`: the installation id, as recorded and as accepted by the owner;
  - `pending_instance`: an id seen and not yet accepted;
  - `url` and `version`: from the request or zeroconf, for display;
  - `max_caps`: a JSON list, the key's maximum, a subset of the role `integration`;
  - `user_id`: the integration's own user, which holds its API key, with the role `integration`.
    It also stands for calls without a header (the coordinator's polls and stream);
  - `system_user_id`: the user `system` of this integration;
  - `state`: `pending`, `active` or `disabled`;
  - for a pending request: `request_code` (the short code), `request_expires` and
    `request_remote`;
  - `version`, `deleted_at` and `origin`, as the owned tables have.
- **`user_identities`**, rebuilt (D10-C):
  - `provider_id` becomes nullable, beside a new `integration_id`; exactly one is set (a CHECK).
  - `subject` is the OIDC `sub`, or the Home Assistant `user.id`, or `system` for the
    integration's `system` user.
  - `user_id` is the Massimo user the identity acts as (D1-C).
  - `own_user_id` (new) is the integration-only user made at the first call. It stays when the
    identity is linked, marked linked, so its history stays readable.
  - `system_generated` (new) and `last_level` (new, for display only).
  - The OIDC columns (`email`, `name`, `username`, `groups`, `bound_at`, `last_used_at`) stay.
  - Unique: (`provider_id`, `subject`) where `subject` is set, as today; (`integration_id`,
    `subject`).
- **Integration-only users** are `users` rows:
  - without a password, so no sign-in;
  - named `<integration key>.<first 8 hex of the id>`, the name being unique and lower case;
  - `display_name` is Home Assistant's name, updated when a call brings a new one.

### A2. A call's capabilities

For a request with an integration's key and the header:

1. **The identity:** (`integration`, `id`). Where none exists, it and an integration-only user
   with the role `integration` are made. An `id` of `system` is the integration's `system` user.
2. **The user's role:** its capabilities, kept only where the role `integration` has them too.
   The owner may give an integration user a smaller role, never a larger one.
3. **The link:** where the identity is linked to a Massimo user, only the capabilities both have
   are kept.
4. **The key's maximum:** only those in `max_caps` are kept.
5. **The level:** `view` keeps `read` (and `preferences`); `control` adds `setpoint`, `lock` and
   `configure`; `admin` keeps all.
6. **The rest** is what the request may do. The audit and the log name the user acted as, with
   the integration it came through.

Refused:

- a header with any other key: `403`;
- a malformed header: `400`;
- an `instance` other than the recorded one: `403`, with the new id noted as `pending_instance`
  for the owner (D4-B);
- a disabled integration, or a disabled integration user: `403`.

A request with the key and **without** the header acts as the integration's own user (its role
`integration` ∧ `max_caps`).

### A3. Registration (D5-C)

1. **Home Assistant asks:** `POST /api/v1/integrations/requests` without a key, with `kind`,
   `name`, `instance_id`, `url` and `version`.
   - Massimo answers with a request id and a six-character code, which Home Assistant shows.
   - Pending requests live 10 minutes; at most 5 are pending, and the route is rate-limited per
     address as the sign-in is.
2. **The owner approves:** the request appears in the web UI's Integrations section with its
   code, its address, and whether a zeroconf record of that installation confirms it. The owner
   compares the codes, sets `max_caps` (default: all of the ceiling) and approves; or refuses.
3. **Home Assistant collects:** it polls `GET /api/v1/integrations/requests/{id}` with the code.
   - Once approved, the first such call answers with the API key (`kind: api`, made for the
     integration's own user) and never again.
   - Refused or expired, it answers `410`.
- **Revoking** an integration deletes its key and disables its users; its rows stay.

### A4. Breaking change

The migration rebuilds `user_identities` with `foreign_keys: off`, keeping every row:
`provider_id` becomes nullable, and the new columns get their defaults. It keeps `min_reader` 1
(the owner, D10-C).

- **Who is affected:** stores of v0.15.0-alpha*, the releases with OIDC. Opened by such a release
  after the migration, the identities' changed constraints may refuse OIDC writes.
- **Where it is said:** the commit is `feat(store)!` with a `BREAKING CHANGE:` trailer, and the
  changelog has an entry under "Changed".
- **Older releases** never had the table.

### A5. `_massimo._tcp` (D8-A)

The TXT record gains:
- `url`: `MASSIMO_PUBLIC_URL`, or the API's own;
- `loc`: the installation's name, the settings' server name, else the host name;
- `reg=1` while registration requests are accepted (a setting, on by default);
- `api=1`: `/healthz` has `features`.

Massimo's mDNS browser also asks for `_home-assistant._tcp`, every 30 minutes with the Cube
discovery and on request. The `uuid`, `location_name`, `internal_url`, `base_url` and `version`
it finds give the hints, and confirm a request's installation id.

### A6. The code each block touches

- **`internal/store`:**
  - `identities.go`, `identity.go` (OIDC, the rows by provider);
  - `providers.go`, `roles.go` (`ErrImmutable` for `owner`), `tokens.go`, `users.go`;
  - the migrations.
- **`internal/server`:**
  - `auth.go`: the principal, `caps` from the role;
  - `oidc.go`;
  - `users_api.go`;
  - `audit.go`: `by`;
  - `features.go`.
- **`internal/announce`:** `announce.go` (the TXT record) and `browser.go` (the browser).
- **`internal/server/ui/`:** the users page; a new partial for the Integrations section.

## Implementation Plan

Every block brings its tests, `docs/api-v1.md` and the OpenAPI file for each route, `docs/ui.md`
for the UI, `docs/store.md` for the store, and a `CHANGELOG.d` fragment.

### Block 1: Identities

1. The migration:
   - `user_identities` rebuilt with `integration_id`, `own_user_id`, `system_generated` and
     `last_level`, `provider_id` nullable;
   - `integrations` created;
   - the role `integration`'s set becomes the ceiling;
   - its current holders move to "integration (before 1.0)" with their current set (D11-A).
2. `roles.go`: `integration` is immutable like `owner`; it can't be assigned to a regular user.
3. The store's accessors for integrations and their identities. OIDC's keep their behaviour,
   tested as before.
4. `feat(store)!` with `BREAKING CHANGE:`.

**Verification:** `go test ./...`, the store's migration tests (a v0.15.0-alpha.1 store with OIDC
identities and an integration-role key migrated), `make race`, `make cross`, lint.

### Block 2: Registration

Routes:
- `POST /api/v1/integrations/requests` (no key, rate-limited);
- `GET /api/v1/integrations/requests/{id}?code=`;
- `GET /api/v1/integrations`;
- `POST /api/v1/integrations/{uuid}/approve`, `…/refuse`, `…/accept-instance`;
- `PUT` and `DELETE /api/v1/integrations/{uuid}`.

The managing routes need a new capability, `integrations`, held by owner and manager; the
ceiling leaves it out.

**Verification:** route tests for each step and each refusal; the key is issued once.

### Block 3: On behalf of

1. Parse the header (RFC 8187 `name*`; `id` 32 hex or `system`; `level` one of three;
   `instance` 32 hex).
2. Resolve or make the identity and the user (A1); the capability chain (A2).
3. The audit's `by` and the log's user, with the integration; `features`: `integration: 1.0`.

**Verification:** a table test of the chain, every step narrowing; route tests with the fake
Cube (a `view` call refused a setpoint, a `control` call sending the exact line); the refusals
of A2.

### Block 4: Linking

1. `GET /api/v1/integrations/{uuid}/users` with link proposals by display name.
2. `PUT …/users/{subject}` with `{"user": "<name>"}` to link, `{"user": null}` to unlink.
3. The users list (`GET /api/v1/users`) marks integration users and linked identities.

**Verification:** route tests; two Home Assistant users linked to one Massimo user; the chain
after a link.

### Block 5: mDNS

1. The browser asks for `_home-assistant._tcp` too.
2. The TXT record gains `url`, `loc`, `reg` and `api`.
3. The setting for `reg`.

**Verification:** unit tests of both records; `make lan-test` (the announcement changes).

### Block 6: Web UI

1. System settings gain an Integrations section: pending requests with their code and the
   zeroconf confirmation; approve with the maximum (switches under the ceiling); refuse; each
   integration's users with their links; accept a new installation id; revoke.
2. The users list marks integration users and their integration; a Massimo user shows its
   linked Home Assistant users.
3. Six languages; `docs/ui.md`.

**Verification:** `make ui-test` at 1100 and 390 px, with new tests for the section.

### Block 7: Docs and the integration session

1. `docs/api-v1.md` ("Integrations"), `docs/store.md` (section 6), `docs/proxy.md` (the
   discovery record).
2. A note to the session "HA Integration 1" with the header, the routes and the version that has
   them.

**Verification:** the guidelines check; the line breaks.

## Proposed commit message

For Block 1, for example:

```
feat(store)!: One identities table for providers and integrations

AP IntegrationUsers v1.0, Block 1.

Key changes:
- user_identities names an OIDC provider or an integration; integrations is new.
- The role integration is fixed at the ceiling; its holders move to
  "integration (before 1.0)".

BREAKING CHANGE: a store of v0.15.0-alpha* cannot be opened by that release
again after this migration.
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- **v1.0** (2026-11-14): first revision. It rests on the briefing of the session "HA Integration
  1", its two answers about Home Assistant's users and installation id, and the owner's D1 to
  D11.
