# NOTE IntegrationUsers v1.0: how the run went

Plan: `wip/2026-11-14_11-45_AP_IntegrationUsers_integrations_act_for_users.md`.
Run under the owner's `PLAN++ D11-A`, then `EXEC D12-A reword`.

## The pull requests

Each block is its own pull request, stacked on the one before:

| Block | Pull request | Summary |
|-------|--------------|---------|
| 1 | #235 | `feat(store)!: One identities table for providers and integrations` |
| 2 | #236 | `feat(api/integration): Register integrations with a short code` |
| 3 | #238 | `feat(api/integration): Let an integration act for its users` |
| 4 | #239 | `feat(api/integration): Link an integration's users to Massimo users` |
| 5 | #240 | `feat(announce/integration): Announce to integrations and look for Home Assistant`, and D12-A: `feat(announce/integration): Announce the address the reverse proxy serves` |
| 6 | #243 | `feat(ui/integration): Approve integrations and link their users` |
| 7 | #244 | `docs(store/integration): Describe the integrations in the store` |

None is merged; #235 goes first.

## Where the run went differently from the plan

1. **A new capability, `integrations`.**
   The plan's routes needed a capability for the owner's side of the
   registration.
   `integrations` is outside the ceiling, so an integration never manages
   integrations.
   The owner has it, and migration 0021 gives it to `manager`.
2. **The ceiling.**
   "No owner capabilities" (D7-A) is read as no capability that manages
   people, sign-in or the integrations:
   `users`, `passwords`, `roles`, `roles.disable`, `capabilities`, `auth`
   and `integrations`.
   `update`, `restart`, `reboot`, `cubes`, `cubes.reboot`, `settings` and
   `audit` stay in it, as does `guests` (D9-A).
   Each integration's maximum narrows the ceiling further.
3. **`integration-v0` is made only where needed.**
   Migration 0021 creates the role only where the role `integration` had
   users, and moves them with exactly the set they had (D11-A).
   A store without such users never sees the role.
4. **The refusals carry a `code`.**
   The codes are `bad_header`, `not_integration`,
   `integration_disabled`, `instance_pending`, `user_disabled` and
   `forbidden`.
   HA Integration 1's notes 1 to 5 are built in, among them
   1 (`/me` answers `integration` and `acts_for`), 4 (these codes) and
   5 (name collisions, `system` without a name).
5. **Block 4: where the users are listed.**
   The plan's `GET /api/v1/integrations/{uuid}/users` clashes in Go's mux
   with `GET /api/v1/integrations/requests/{id}`, since neither pattern is
   more specific.
   The users come with `GET /api/v1/integrations/{uuid}` instead,
   a route the API lacked anyway.
   The link stays at `PUT …/{uuid}/users/{id}`.
6. **Block 5: the setting.**
   `integrations.requests` was a string in Block 2 (`"off"` refused).
   It is now a switch in `GET` and `PUT /api/v1/settings`, true by
   default.
   No release had the string form.
7. **Block 5: the announcement.**
   `MASSIMO_PUBLIC_URL` is new (`internal/env`).
   `loc` is the host name in full, since there is no setting for the
   installation's name.
   An entry longer than a TXT string's 255 bytes is left out.
8. **D12-A, decided by the owner mid-run.**
   Where `MASSIMO_PUBLIC_URL` is not set, `url` is the address a trusted
   proxy serves the server at, as the last authorized request through it
   said, kept in the setting `server.proxied_url`.
   The scheme's own port is left out, in `url` and in System Info.
9. **The commit summaries were reworded at the owner's word.**
   Blocks 2 to 5 were reworded to `api/integration` and
   `announce/integration`; Block 1 keeps `feat(store)!`, because its
   breaking change touches OIDC too.
   `commit-msg.sh` refuses `api:integration`: the colon is a role's.
   The branches were force-pushed with lease, and the trees did not change.
10. **Block 6: a name clash.**
    `integrations.js` declared `renderFound`, which `app.js` already has
    for the found Cubes.
    Loaded later, it replaced that function, and the found Cubes' link
    disappeared.
    The UI suite caught it (`test_discover.py`), and the function was
    renamed.
    A guard test is proposed as a separate task.

## What was checked

- **Every block:**
  - `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  - `go vet` for linux, windows and darwin, and `gofmt`;
  - `make cross`;
  - `GUIDELINES/shared/tools/check`: 12 of 12.
- **Block 5:**
  - `-race` on `announce` and `serve`;
  - `make lan-test`: 14 of 14, avahi resolving the extended record.
- **D12-A and Block 6:** `make ui-test`, with 680 and then 684 passing.
- **The HA side:** HA Integration 1 has the header, the routes and the
  refusal codes.
  A real Home Assistant has not called the server yet.

## Left for the owner

- **Merge the stack**, #235 first.
- **On the Pi:**
  - with `MASSIMO_ANNOUNCE=on`, and after one signed-in page through the
    proxy, `avahi-browse -r _massimo._tcp` should show
    `url=https://massimo.breno.caleon.ch`;
  - System settings → Integrations should list the Home Assistant
    installation on the LAN.
- **The retirement**, once the HA side has registered against the merged
  build.
