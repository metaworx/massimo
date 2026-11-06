# AP StorePrefs v1.0: Preferences per user

## TL;DR

Store stage (c) of docs/store.md:
the page's settings and the user's colour schemes follow the user across browsers.
The store keeps them per user,
the page reads them at sign-in and writes them on change,
and a switch beside each setting keeps a value for this device instead;
a role without the capability `preferences`, a guest,
keeps everything in the browser as today.

- `preferences` table since 0001 → no table change;
  one additive migration seeds the new capability.
- Keys `ui` (poll, expert, clock, language, theme,
  the chosen scheme) and `schemes` (the user's `massimo-scheme/1` documents) →
  `GET`/`PUT`/`DELETE /api/v1/me/preferences[/{key}]`, own only.
- `PUT` and `DELETE` need `preferences`: manager, operator and viewer have it,
  guest and integration not (D5).
- Precedence on the page: device override → user default (server,
  cached) → the browser's values from before → built-in;
  the first sign-in seeds the server from the browser.
- A write is an audit row, not a command (D3);
  the theme mode is a user default with a device override like the rest (D2
  folded into D5).
- The smoke block lands on the pytest suite once pull request 56 is merged (D4).
- Branch `claude/store-c` from `origin/main`;
  one commit per block with its tests and bullet.

### Blocks

1. Migration 0003 and the capability `preferences`: the seed for manager,
   operator and viewer, the server's and the page's capability lists,
   the translations, the tests' tables, section 6's tables.
2. Store: `preferences.go` (list, get, set, delete to a tombstone,
   the key and size rules); tests.
3. Server: `me_api.go` with the four routes, the audit actions, no command row;
   docs/api-v1.md and the OpenAPI file; tests.
4. UI: the scopes, the switch per setting, the sync at sign-in and on change,
   the seeding, the schemes; docs/ui.md.
5. Smoke test on the pytest suite: a scheme set, a fresh context, sign in,
   see it; a guest sees no switches.
   Waits for pull request 56.
6. Documents: docs/store.md stage status, README "Server";
   CHANGELOG bullets with each commit.

## Discussion

The decisions of the TL;DR and EVAL gates (2026-11-06):

- **D1-A, the keys `ui` and `schemes`.**
  Section 9 (c) names `scheme` as one `massimo-scheme/1` document;
  the editor keeps several schemes under names,
  so the user's own schemes are a list under `schemes`,
  and the chosen scheme's id is a field of `ui`.
- **D3-A, an audit row and no command row** for a preference write:
  nothing goes to a Cube.
- **D4-A, the smoke block on the pytest suite** of pull request 56;
  Blocks 1 to 4 and 6 do not touch `tools/ui-test/`.
- **D5-A with the capability**, the owner's addition:
  every setting is a user default on the server,
  a switch beside it keeps a device value in the browser instead,
  and the user default is kept on the server only for a role with `preferences`.
  Guests must not have it: their settings stay on their devices,
  where the built-in defaults apply until they change them there.
  D2 (the theme mode) is the same mechanism:
  a user default with a device override, so a wall tablet can stay dark.

Why a capability and not a rule by role key: the roles are editable,
and a created role decides for itself;
the switch per capability lets the owner turn it off for everyone but owner.

## Analysis

### What exists

- `preferences` (migration 0001): `uuid`, `user_id`, `key`, `value` (JSON,
  a content column: no index, no `WHERE`), `updated_at`, `version`,
  `deleted_at`, `origin`, primary key (`user_id`, `key`).
- The page keeps `massimo.poll`, `massimo.expert`, `massimo.clock`,
  `massimo.lang`, `massimo.theme`, `massimo.scheme` (the chosen id,
  `custom:<name>` for an own scheme) and `massimo.schemes` (the own schemes,
  `[{name, light, dark}]`); `loadSettings` reads them at start,
  `theme.js` applies the scheme before the page is drawn.
- `state.me` (login.js) carries the capabilities;
  `hasCap` and the `no-<capability>` body classes hide what a role may not do;
  `afterLogin` runs `loadMe`, then `refresh` and `connectEvents`.
- The capability list lives in `auth.go` (`capabilities`),
  `login.js` (`CAPABILITIES`), the tests' tables (`allCaps` in `roles_test.go`,
  `roleSets` in `capabilities_test.go`, `TestSeededRoles`),
  docs/store.md section 6 and docs/api-v1.md "Authentication";
  the roles grid labels each as `cap.<key>`.
- The audit's `actions` table names every non-GET route;
  the wrapper writes a command row for every audited action but `login` and `logout`.

### The capability

`preferences`:
keep the settings on the server, so that they follow the user across browsers.
Guards `PUT` and `DELETE /api/v1/me/preferences/{key}`; the `GET`s take `any`,
since a user without rows reads nothing.
Seeded for manager, operator and viewer by migration
`0003_preferences_capability.sql` (`min_reader` 1, `INSERT OR IGNORE`,
so a fresh file and an updated one end the same);
owner has every capability by code, guest and integration none.
A created role takes it from the roles editor;
the capabilities switch can turn it off for everyone but owner.

### The store

`Preference{UserID, Key, Value json.RawMessage, UpdatedAt, Version, Origin}`;
`Preferences(ctx, userID)` (not tombstoned, by key),
`Preference(ctx, userID, key)`,
`SetPreference(ctx, userID, key, value)` (an upsert that clears a tombstone
and bumps the version), `DeletePreference(ctx, userID, key)` (a tombstone).
A key is `^[a-z][a-z0-9_.-]{0,31}$`; a value is valid JSON of at most 64 KiB.
Rows of a removed user stay with the user's tombstone.

### The routes

- `GET /api/v1/me/preferences` (`any`): every key of the signed-in user,
  `{"ui": {…}, "schemes": {…}}`; 409 without a signed-in user (open,
  token and none modes), which tells the page to keep to the browser.
- `GET /api/v1/me/preferences/{key}` (`any`): the value, 404 when unset.
- `PUT /api/v1/me/preferences/{key}` (`preferences`): the body is the value;
  400 for a bad key or no JSON, 413 over the limit;
  answers `{"key", "value", "updated_at", "version"}`.
- `DELETE /api/v1/me/preferences/{key}` (`preferences`): a tombstone;
  `{"ok": true}` also when it was unset.
- Audit actions `preference.set` and `preference.delete`;
  the wrapper's exclusion from the command rows becomes a set of actions,
  `login`, `logout` and these two.

### The page

- `ui` on the server:
  `{"poll": 10, "expert": false, "clock": "browser", "lang": "auto", "theme": "system", "scheme": "massimo"}`; `schemes`: `{"format": "massimo-schemes/1", "schemes": [{"format": "massimo-scheme/1", "name", "light", "dark"}, …]}`.
- Storage in the browser: `massimo.ui`, the cache of the server's `ui`;
  `massimo.device`, the overrides, `{"poll": 5}`,
  written only through the switch; `massimo.schemes` as today,
  the cache of `schemes`;
  the keys from before (`massimo.poll` and the others) are read while there is
  no `massimo.ui`,
  seeded into the server at the first sign-in with the capability when it has no `ui`,
  and removed then.
- Precedence in `loadSettings`: the device override, else the cached `ui`,
  else the key from before, else the built-in default.
- The switch "this device only" beside each of the six settings (`needs-preferences`,
  so a guest and an open server see none): on,
  the row's value goes to `massimo.device` and stays; off,
  the override goes and the user default applies.
- A change of a setting: with an override, `massimo.device`;
  else `state.settings` into `massimo.ui` and, signed in with the capability,
  a `PUT` of `ui` debounced by a second.
  The own schemes: `storeCustomSchemes` writes the cache and,
  with the capability, a `PUT` of `schemes`.
- At sign-in (`afterLogin`, before `refresh`): `GET /api/v1/me/preferences`;
  `ui` present → applied where not overridden, cached;
  absent with the capability → seeded from the settings in hand;
  `schemes` likewise.
  A 409 or a 403 leaves the browser's.
- Log out keeps the caches; the next sign-in overwrites them.
- docs/ui.md: the switch per setting, which settings follow the user,
  the capability.

### Tests

- Store: a round trip of both keys, the tombstone and the version,
  the key and size rules, another user's rows apart.
- Server: the four routes as a user with the capability;
  `PUT` refused for a guest (403) and answered 409 on an open server;
  a key too large (413); the audit rows; no command row;
  `TestRolesAgainstRoutes` picks the new routes up;
  `TestOpenAPICoversRoutes` their `x-capability`.
- The seeded sets: `TestSeededRoles`, `roleSets`, `allCaps` gain `preferences`.
- The page (Block 5, pytest): the owner sets a scheme and the poll,
  a fresh context signs in as the owner and has both;
  the operator the runner creates sees the switches,
  a guest the runner creates sees none.
- The gate after every block: `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race` for `internal/server`, `gofmt -l .` empty, `go vet ./...` for linux,
  windows and darwin, `make cross`;
  `make ui-test` after Block 4 on the suite in `main` then;
  `GUIDELINES/shared/tools/check` before each commit.

## Implementation Plan

### Block 1: the capability

1. `internal/store/migrations/0003_preferences_capability.sql`,
   `-- min_reader: 1`: `INSERT OR IGNORE INTO role_capabilities` for manager,
   operator and viewer.
2. `auth.go` `capabilities`, `login.js` `CAPABILITIES`;
   `cap.preferences` in the six translation files; `allCaps`,
   `roleSets` and `TestSeededRoles` updated;
   `TestMigratesAVersionOneFile` reaches version 3.
3. docs/store.md section 6 (the two tables),
   docs/api-v1.md "Authentication" (the table and the roles paragraph).

**Verification:**
the store and server tests;
`store info` on a fresh directory prints schema version 3.

### Block 2: the store

1. `internal/store/preferences.go` as the Analysis has it;
   `preferences_test.go`.

**Verification:**
the store tests, `TestNoFilterOnContent` among them.

### Block 3: the routes

1. `internal/server/me_api.go`:
   the four routes, `userStore`, the limits, `auditTarget` (`preference:<key>`);
   `audit.go` actions and the no-command set; the registrations in `server.go`.
2. `me_api_test.go` as the Analysis lists;
   docs/api-v1.md "Preferences" and the OpenAPI paths and schema.

**Verification:**
`go test -race ./internal/server`.

### Block 4: the page

1. `app.js`:
   `loadSettings` with the precedence,
   `saveSetting(key, value)` that routes to the device or the user default
   and the debounced `PUT`, the seeding; `login.js`:
   `loadPreferences` in `afterLogin` and at `init` after `loadMe`; `theme.js`:
   the schemes' `PUT`; `index.html` and `appearance.html`: the switch per row;
   `style.css`; `i18n`: `settings.thisDevice`, `help.thisDevice`.
2. docs/ui.md "Settings".

**Verification:**
`go test ./internal/server -run TestUI`; `make ui-test` on the suite `main` has;
the page at 1100 and 390 px.

### Block 5: the smoke test (waits for pull request 56)

1. The pytest module for the preferences as the Tests section lists;
   the runner creates a guest beside the operator.

**Verification:**
`make ui-test`.

### Block 6: the documents

1. docs/store.md: stage (c) implemented, the capability in section 6,
   item 11's status; README "Server": the settings follow the user.

**Verification:**
`GUIDELINES/shared/tools/check`; the full gate; `make cross`.

## Proposed commit message

```
chore(aps): +StorePrefs v1.0

Register the plan for store stage (c): the settings and the colour schemes per user, with a capability that keeps a guest's in the browser.

File-revision-v1: GUIDELINES/wip/2026-11-06_19-33_AP_StorePrefs_preferences_per_user.md::v1.0::2026-11-06::All::the first revision
Assisted-by: claude-code:claude-fable-5-1
```

## Change History

- v1.0 (2026-11-06): first revision,
  from the TL;DR and EVAL gates of the same day; D1-A, D3-A, D4-A,
  D5-A with the owner's capability rule.
