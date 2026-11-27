# AP KeyLife v1.0: The life of API keys

## TL;DR

API keys get a life of their own:
an idle limit the owner sets, an optional end date,
and New secret on a key's row;
first the copy of the key just made and the two gaps AP MobileApp found.

- key shown → the shared copy part and its icon (D1-A)
- a key ended → its session too, wherever a token is let in
- a key revoked → the keys it made, recursively; not at an end (D5-A)
- idle limit → `auth.key_idle_days`, off until set (D2-A);
  a session's use counts as its key's
- exclusion → by who manages the user;
  an integration's key never, the settings file's always (D3-A)
- end date → at Create key and on the row; never by the key itself (D4-A)
- New secret → id, label and end kept;
  the key's session ended unless it is the caller's (MobileApp.D76-A)
- D6 to D8 → A, as recommended, under PLAN++;
  the owner's to change before the block that needs it

### Blocks

1. Copy: the key shown once, drawn with the shared copy part
2. Gap: a key's sessions end with its end; keys made by keys (D5)
3. Idle limit: the setting, a key's exclusion, the check
4. End date: set and moved per key, shown on its row
5. Re-key: the route and "New secret"

## Discussion

### What the owner decided

**In AP MobileApp v1.4** (2026-11-21), on My profile's keys,
recorded there as "AP KeyLife, a plan of its own to come":

- **MobileApp.D75**, the expiry, in the owner's words:
  "BA+B, where a key can be excluded from A. but not integration keys,
  they should also be subject to A, and have an optional B".
  So (A) an idle limit for API keys, a server setting,
  from which a key may be excluded, but an integration's key never;
  and (B) an optional end date per key, an integration's key included.
  The evaluation behind it weighed what an ended key breaks, quietly:
  the app forgets its kept login at the `401`,
  Home Assistant has to register again,
  and the settings file's key, which the bootstrap adds back only where its
  hash is unknown, would no longer let the owner in (D3-A below).
  It also found that a session's use should count as its key's use for the idle limit.
- **MobileApp.D76-A**, the re-key: "New secret" on a key's row;
  the id and the label kept, the new secret shown once,
  the key's sessions ended but the caller's own.
- **MobileApp.D77-A, D78-A**:
  the last use and its address are on every row: built, nothing to do.
- **MobileApp.D79-A**, the renewal: none,
  since use renews a key under the idle limit; weighed again here,
  with the end date, as D4-A.
- **MobileApp.D80-B**:
  `DELETE /api/v1/me/token` is built; as a revoke,
  it takes the key's heirs with D5-A.
- **MobileApp.D81-A**:
  a key is never shown again, the store keeping only its hash,
  and the re-key is the way back: nothing to build.
- Two gaps, for this plan to close first:
  a session made from a key checks only the key's revoke, not its end;
  and any key can make more keys,
  so revoking a leaked key leaves the keys it made.

**At this plan's TL;DR gate** (2026-11-27):

- **D1-A**:
  the copy fix is Block 1, its own small pull request, first:
  the key just made is drawn with the shared `Copy` part and its icon;
  the re-key's new secret (Block 5) is shown in the same box.
- **D2-A**:
  the idle limit is off until the owner sets a number of days,
  so that no key ends because of an update.
- **D3-A**:
  the settings file's key (`MASSIMO_API_TOKEN`,
  `from_settings` in the list) is exempt from the idle limit and takes no end date,
  since an ended one would lock the owner out.
- **D4-A**:
  a key cannot move its own end date;
  the key's user (signed in) or an admin moves it.
  An end date that renews itself would just be the idle limit again.
  "Itself" means that the bearer of the request is that very key;
  a session is not the key.
- **D5-A**, narrowed by the owner:
  revoking a key also revokes the keys it made, recursively,
  on a revoke alone, not when a key ends by its end date or the idle limit.
  In the owner's words: "if I use a key from the app to sign in and then in
  the interface create a new key, this should not be a heir of the phone's key
  that created the session. only if the key itself was used directly."
  So a key is an heir only where the bearer of its making was an API key;
  one made by a session, any session,
  one made from a key by `POST /api/v1/login/key` included,
  is no key's heir.
  One column records the maker, with its migration.

### The owner's, taken as recommended under PLAN++

PLAN++ cleared this plan's gate, so the run takes each A;
the owner may name another before the block that needs it:
D8 before Block 2, D6 and D7 before Block 3.

- **D6.
  What an ended key is.**
  - **A (recommended).**
    A state read at each check, from the key's end date,
    its last use and the limit: nothing is written when a key ends.
    It stays listed, marked Ended with the reason, with Revoke;
    it lives again where its end date is moved later,
    the limit raised or turned off, or the key excluded,
    and its session with it while the session's own 30 days last.
    Its use never brings it back:
    an ended key is refused before its use is written.
  - **B.**
    Final: the first check that finds a key ended writes so (a column `ended_at`),
    and nothing brings it back; it is listed as ended until it is revoked,
    and a date moved, an exclusion or a limit raised counts for live keys alone.
- **D7.
  Who excludes a key from the idle limit.**
  - **A (recommended).**
    Who manages the key's user (`users`, or `guests` for a guest it reaches,
    within the rank rule: the server's `may.manage`),
    at Create key and on the key's row.
    The limit stays the policy of whoever sets it:
    an owner or an administrator excludes their own keys as anyone's,
    and a user who manages nobody keeps the limit on their keys.
  - **B.**
    Also the key's own user, signed in, as D4-A lets them move its end date.
- **D8.
  The keys the settings file's key made, when the file's token changes.**
  - **A (recommended).**
    Revoked with it: the bootstrap revokes the old key when the file's token changes,
    and that is a revoke;
    the file is how the owner ends a token that may have leaked,
    and the log line says how many keys went with it.
  - **B.**
    Kept: the bootstrap's revoke is a rotation, not a leak;
    only a revoke under a user's tokens, `DELETE /api/v1/me/token`
    and a user's removal take heirs.

### Decided in the writing

- **A session's use is its key's,** as MobileApp's evaluation found:
  where `touch` writes a session's last use, at most once an hour,
  it writes its key's too, with that use's address.
  A phone in use so keeps its key,
  and the key's row says when and from where it, or its session,
  was last used.
- **The idle clock** is the key's last use,
  or its making where it was never used, in whole days, at most an hour late.
  The limit applies at once:
  a key unused for longer ends at its next check,
  which the setting's line says before it is saved.
- **A key changes nothing of its own life**, as D4-A says of its end date:
  where the bearer is that very key,
  its end date, its exclusion and its secret are refused (`403`).
  It still revokes itself (MobileApp.D80-B),
  and a session made from it is not the key.
- **An heir is made by** a `POST …/tokens` whose bearer is an API key (D5-A),
  for whichever user;
  never by a session's, nor by the command line, the bootstrap,
  or an integration's collection of its key.
  A revoke follows the makers from key to key in any state,
  so a key ended in between passes its maker's revoke on to its own heirs.
  A user's removal revokes its tokens, so their heirs go too;
  a password set by someone else ends sessions alone, which make no heirs.
  An heir lives past its maker's end, as D5-A narrows it:
  a key with an end date may make one without, and only a revoke reaches it.
  The revoke's answer and its log line name the heirs.
- **The settings file's key is known by its hash**, not by a mark on its row,
  as `from_settings` is (MobileApp.D86-A):
  the store keeps in memory the hash the bootstrap last applied,
  at the start and at each reading of the file,
  and the idle rule passes over that key.
  Its row takes no change (`409`), as it takes no Revoke.
- **An integration's key** takes its end date on its row,
  in its own user's panel under System settings → Users,
  from a holder of `users` or `integrations`; not at the approval.
  It takes no exclusion (`400`, MobileApp.D75) and no New secret (`403`):
  the integration learns its key by its approval's code alone,
  and an ended or revoked one registers again.
- **The re-key** is an API key's alone, live (an ended one `409`),
  never the settings file's (`409`).
  Its end date, exclusion, maker and heirs stay.
  The old secret's hash is kept in a revoked row of its own,
  so that a client still sending it is answered as an ended token
  and not counted as a guess (MobileApp.D64-A).
  A key has one live session at a time (`AddKeySession`),
  so "the key's sessions" is at most one, ended unless it is the caller's.
- **In the app**, the row of this device's key offers no New secret:
  the phone keeps the old secret, and its kept login would fail at its next use.
  A re-key from a browser makes the app forget that login at the `401`,
  as any ended key does.
- **Where the setting is drawn**:
  System settings → Users, under the password sign-in's switches,
  for a holder of `settings`; whole days, 0 for off, at most 3650,
  saved when the field is left.
  It needs `settings`, as the sign-in's switches do,
  so an integration whose maximum holds `settings` may change it,
  as it may change them today (`auth.go:84-87`): left as it is.
- **Builders and pull requests**:
  five pull requests, stacked in the blocks' order, each on the one before;
  Opus sub-agents build a block each;
  the coordinating session reviews, commits signed and opens the pull requests;
  each block's run goes into a NOTE.

**Names chosen here**, routine and not decisions of the owner's:
the column `tokens.made_by` and its migration `0100_token_maker`,
the column `tokens.idle_exempt` and its migration `0101_token_idle`,
each with its down part (main's next numbers: see the Analysis);
the setting `auth.key_idle_days` (`store.KeyIdleDaysKey`);
the routes `PUT /api/v1/users/{name}/tokens/{id}`
and `POST /api/v1/users/{name}/tokens/{id}/rekey`,
audited as `token.update` and `token.rekey`; a token's fields `made_by`,
`idle_exempt`, `idle_ends_at` and `ended`,
the bodies' `idle_exempt` and `expires_at`, the revokes' answer `heirs`;
in the store `AddKey` with `KeyOptions`, `TokenAlive`,
`SetTokenLife` with `TokenLife`, `RekeyToken`,
and a token's `EndsAt` and `Ended`; in the page `setKeyIdleDays`,
`setTokenLife` and `rekeyToken`; the branches `claude/keylife-copy`,
`claude/keylife-gap`, `claude/keylife-idle`,
`claude/keylife-end` and `claude/keylife-rekey`, one per block.

### Not in this plan

- the command line's own flags:
  `massimo user token` makes a key with no end date and no exclusion,
  and `massimo user tokens` prints its `EXPIRES` as today;
  a later plan, where wanted;
- a renewal (MobileApp.D79-A, D4-A);
- the app: nothing to build (Analysis, "For Block 5");
- a count of the keys a new limit ends, before it is saved:
  the setting's line says that such keys end at once.

### Beside other plans

- **AP MobileApp v1.5**, to come (MobileApp.D99-A):
  its Block 20 changes what every copy says (D97-A)
  and puts My profile's failures in the toast (D104-A):
  shared parts that Block 1 and the later blocks use;
  whichever lands second merges the other's,
  and Block 1 takes the `Copy` part as it then is.
- **AP ErrorCodes v1.0** gives every refusal a code, the tokens' in its Block 4:
  a block of this plan that lands after ErrorCodes' Block 1 gives each refusal
  it adds a code and its six texts; one before leaves them to ErrorCodes.
- **AP ServerBackup v1.0**, Block 2, also takes a migration of main's block:
  whichever comes second takes the next number.

## Analysis

Read on main at `merge: maint/0.17 into main, with SteadyTests v1.1`,
2026-11-27, in the code alone;
main has since gained plans alone (`merge: maint/0.17 into main, with +ErrorCodes v1.0`),
its code the same.
The lines are those of that commit.

### For Block 1: the key shown

- **Today:**
  `Tokens` (`internal/server/ui/src/shared/tokens.tsx:100`) keeps the key just
  made in its own state (`shown`, `:118`,
  set by the read that follows `createKey`,
  `src/shared/account.ts:56`) and draws it as
  `<p class="mono user-key">{shown}</p>` in `.user-key-box` (`:171-174`),
  under `users.keyShown`;
  `.user-key` is `word-break: break-all; user-select: all` (`ui/style.css:2244`).
  No click, key or icon copies it.
- **The part:**
  `Copy` (`src/shared/copy.tsx:77`) is a text that copies itself on a click,
  Enter or Space, with `icon` its copy icon after it (`CopyPair`, `:46`).
  Without `name` it names itself "Copy <its text>" (`copy.what`),
  in its `aria-label` and its `title`:
  the key would be read out and shown as a tooltip.
  With `name` that name alone is said (`CopyProps`, `:59`);
  the suggested password does so, quiet as well (`src/shared/password.tsx:676`).
  The key is shown on purpose, so it takes a name and is not quiet.
- **Its other users**, as models:
  the server's facts (`src/signin/Server.tsx:56`),
  the address of the kept-open card (`src/signin/KeepOpen.tsx:87`),
  a provider's callback (`src/admin/Providers.tsx:216`).
- **The tests that read the box:**
  `test_users.py`, `test_new_key_shown_once` (`:381`, the key read at `:410`);
  `test_admin_updates.py`,
  `:186` and `test_a_key_made_is_gone_at_the_next_read` (`:761`, `:775`);
  `test_settings.py`, `test_my_profile_lists_and_creates_own_keys` (`:112`,
  `:123`).
  `.user-key` stays on the copy's span, so their reads hold.
  `test_copy.py:29` shows how a copy is checked:
  the clipboard granted, a click, the icon and Enter.

### For Block 2: how a token is let in, and the two gaps

- **The row:**
  `tokens` (`internal/store/migrations/store.sql:312-326`;
  `docs/store.md:406-420`): the hash, SHA-256 of the secret
  (`internal/store/tokens.go:57`), unique;
  `expires_at`, `NULL` for every key today; `deleted_at`, the revoke, a
  tombstone that keeps its row and its hash (`tokens.go:9-11`);
  `parent_id`, the key a session was made from (migration 0025).
- **The admission** (`internal/server/auth.go:817`, `resolve`):
  `TokenByHash` (`tokens.go:155`) finds a token not revoked,
  its user not removed, and a session made from a key only while the key is
  not revoked (`liveKey`, `:77`); then `tok.Expired` (`auth.go:833`),
  the token's own end; then `principalFor` (`:869`).
  A refused hash goes to `unknownToken` (`:851`) and `TokenEnded` (`tokens.go:176`),
  so that a token issued and ended is not counted toward the lockout (`errEndedToken`,
  `auth.go:795`; MobileApp.D64-A).
- **The first gap, everywhere a token is let in or counted as live:**
  - `TokenByHash`:
    a session whose key is past its end is let in;
  - `TokenEnded`:
    so it is not "ended" either (`:179`, `liveKey` again);
  - `stillAdmitted` (`auth.go:915`, `:930-939`), the event stream's check at
    each keep-alive: the key's revoke alone;
  - `Tokens` (`tokens.go:189`): such a session is listed;
  - `PushSubscriptions` (`internal/store/push.go:102`, `:126`):
    the token's own revoke and end and its user's, nothing of a key,
    so such a session's devices are still sent to;
  - `AddKeySession` (`tokens.go:256`, `:270`):
    makes a session from a key past its end;
    reached only by `loginKey` (`internal/server/login_api.go:393`),
    whose bearer is that key, so after `resolve` has refused it,
    but held to the same rule.
  Nothing sets an end on a key today:
  `postToken` passes none (`internal/server/users_api.go:1022`),
  nor `massimo user token` (`cmd/massimo/user.go:593`),
  nor the integration's collection (`internal/store/integrations.go:425`),
  and the bootstrap clears one (`internal/store/bootstrap.go:186`).
  So the gap is closed before Blocks 3 and 4 make keys end.
- **The second gap:**
  `POST /api/v1/users/{name}/tokens` (`users_api.go:1000`) takes any bearer
  with the rights:
  `targetUser` (`:702`) lets in the user's own name (`selfOf`, `:273`),
  so every key of a user makes keys for that user,
  and a key whose user manages others (`mayManage`, `:311`) makes theirs.
  Nothing records who made a key.
  The bearer is an API key where `p.token.Kind == store.TokenAPI`;
  a session made from a key is of kind `session`, with its `ParentID`.
  The app's key is made by the page's session (`login.keep()`,
  `docs/app.md:242`; Store login the same,
  MobileApp's NOTE of the decisions after the phone check), so it is no heir.
  An integration's key makes none:
  the role's ceiling has no `users` (`auth.go:84`),
  and `noWayIn` (`users_api.go:129`) refuses its users a key.
- **The revokes:**
  `RevokeToken` (`tokens.go:210`) revokes a token and a key's live sessions
  (`revokeSessionsOf`, `:242`) with what they subscribed to push.
  Its callers: `deleteToken` (`users_api.go:1037`),
  `deleteMyToken` (`login_api.go:474`), `logout` (`:418`, a session),
  the bootstrap's revoke of a key that is no longer the file's (`bootstrap.go:107`),
  `RevokeIntegration` (`integrations.go:514`),
  `CollectIntegrationKey` (`:420`) and `massimo user revoke` (`user.go:691`).
  `DeleteUser` (`internal/store/users.go:679`) revokes by its own statement (`:706`);
  `RevokeSessions` (`tokens.go:299`) ends sessions alone.
- **The answers** keep no secret:
  a command row keeps the answer without `token` (`secretFields`,
  `internal/server/audit.go:163`; `withoutSecrets`,
  `internal/server/commands.go:383`); the audit's kinds are mapped by route
  (`audit.go:58`, `:102`, `:104`) and given their lines (`changeLines`,
  `:149`; `TestEveryActionHasALine`).
- **The migration's number:**
  main's last is `0026_backup_pending.sql`; since the freeze of 0.17,
  with `maint/0.17` live, main starts its block at 0100 (`docs/store.md`,
  "Two lines of releases"; `nextBlock`, `internal/store/migrate.go:173`),
  so Block 2's is `0100` and Block 3's `0101`,
  unless another plan's lands first.
  The model is `0025_token_parent.sql` and its down part: additive,
  `min_reader` 24, no foreign key, so that the down part can drop the column.
  Held by `TestMigrationFilesNamed` (`migrate_test.go:155`),
  `TestStoreSQL` (`:114`, `-update` writes `store.sql` again),
  `TestMigratesReleasedStores` (`releases_test.go:37`) and
  `TestDowngradeFromTheStoresDownParts` (`downgrade_test.go:160`).

### For Block 3: use, the setting and the exclusion

- **A token's use** (`auth.go:946`, `touch`):
  written at most once an hour (`touchEvery`, `:57`),
  for the token presented alone,
  with a session's end moved 30 days on (`SessionLifetime`, `:52`).
  The app's key is used at each login by fingerprint (`POST /api/v1/login/key`,
  whose bearer is the key), then its session for as long as the phone uses it:
  without the key touched by its session,
  a phone in daily use would see its key end under a limit shorter than that.
- **The server's settings:**
  whole numbers in `intSettings` (`internal/server/settings_api.go:57`),
  applied at once, `log.keep_days` 0 to 3650 the model;
  read by `SettingInt` (`internal/store/settings.go:242`),
  written by `PUT /api/v1/settings` (`settings_api.go:265`) with `settings`,
  every key alike.
  The sign-in's switches,
  `auth.password_login` (`settings.go:271`) and `auth.password_reveal`,
  are drawn in System settings → Users (`PasswordLogin`,
  `src/admin/Users.tsx:318`, `needs-settings`),
  read by `loadPasswordLogin` (`src/admin/sendUsers.ts:193`) and saved at once
  (`setPasswordLogin`, `:248`);
  a number field is `NumberField` (`src/shared/field.tsx:267`),
  as for the reboot's delay (`src/maintenance/Maintenance.tsx:217`).
- **Who manages a user** (`users_api.go:190`, `access`): `mayManage` (`:311`),
  with `users`, or `guests` for a guest it reaches, within the rank rule;
  with `integrations`, an integration's user (`integrationsOf`, `:294`).
  The tokens' routes are registered with `users` (`server.go:1535-1537`),
  `allowed` (`auth.go:965`) letting in the holders of `guests`, `passwords`
  and `integrations` and the own name, and `targetUser` narrowing.
  The page draws a user's tokens where it manages the user or is that user
  (`Users.tsx:231`),
  and lists an integration's user's without Create key (`listOnly`, `:232`;
  `src/profile/Profile.tsx:213-214`).
- **The settings file's key** is found by the hash of the token the server
  holds from the file (`settingsKey`, `login_api.go:451`).
  The change that made it so weighed a mark on the bootstrap's row,
  "which would need a migration and a rule to move it when the file changes"
  (`fix(api): The settings file's key is not revoked under a user's tokens either`),
  and kept the hash.
  `Bootstrap` (`bootstrap.go:43`) runs at serve's start (`server.go:875`) and
  at each reading of the file (`SetToken`, `auth.go:321`);
  the command line never calls it.
- **An integration's key:**
  collected once by its approval's code (`CollectIntegrationKey`,
  `integrations.go:385`), a used one never again (`:417-419`);
  held by the integration's own user, `<uuid>:` (`:112`),
  listed with Revoke in that user's panel;
  `liveIntegrationKey` (`internal/store/opensetup.go:122`) counts a key live
  by `!t.Expired`, which the idle limit joins.
- **What the page has of a token:**
  `Token` (`src/api.d.ts:684`, generated from the OpenAPI file's schema,
  `docs/api-v1-openapi.yaml:4852`, by `tools/apitypes`);
  `TokenRow` (`tokens.tsx:53`) draws the kind, the id, a badge,
  the label, the key a session came from, the last use and Revoke.
- **The tokens' routes** in the OpenAPI file:
  `/api/v1/users/{name}/tokens` (`:864`) and `…/{id}` (`:918`);
  `TestOpenAPICoversRoutes` (`openapi_test.go:61`) and
  `TestCapabilityTableCoversRoutes` (`:248`), whose `users` row of
  `docs/api-v1.md` (`:266`) names each route.

### For Block 4: the end date

- `expires_at` is a column (`store.sql:320`, "NULL: never (API keys)"),
  checked for the token itself (`Token.Expired`, `tokens.go:54`;
  `TokenEnded`, `:179`; `PushSubscriptions`, `push.go:126`),
  shown by the API (`viewToken`, `login_api.go:145`) and in
  `massimo user tokens` (`user.go:639`), and not drawn by `TokenRow`.
- A user's own end is set in the user's panel with `DateTime` (`field.tsx:366`;
  `Users.tsx:122`, `:134`): the model for a key's.
- `docs/api-v1.md` (`:888`) says that a key "never expires",
  and the package comment of `tokens.go` (`:6-7`) the same.

### For Block 5: the re-key and the app

- The hash is unique (`store.sql:317`), and a revoked row keeps its hash,
  which is how `TokenEnded` knows an ended token:
  a re-key that replaced the hash alone would make the old secret a guess.
- A command row keeps the answer without `token` (`commands.go:383`),
  held by `TestTokenCreationKeepsNoSecret` (`commands_test.go:253`).
- A question before an act is `Question` (`src/shared/question.tsx:118`),
  as the reboot asks (`Maintenance.tsx:229`).
- **The app** keeps an API key for its login by fingerprint;
  a refused `POST /api/v1/login/key` (`401`, `ServerApi.Refused`,
  `app/android/app/src/main/kotlin/ch/metaworx/massimo/ServerApi.kt:172`)
  makes it forget the kept login (`Bridge.kt:454-459`),
  and a session the server refuses is forgotten too (`Sessions.kt:142-145`).
  An ended or re-keyed key so ends the login quietly, as MobileApp.D75 weighed;
  nothing to build in the app.
  The page knows the phone's key (`appKeyId`, `tokens.tsx:114`).
- **Scripts that make or use keys:**
  none under `packaging/` or `.github/`;
  `tools/store-fixture/make.sh:146` runs released binaries' `user token`;
  the UI suite makes and revokes the operator's keys through the API with the
  suite's key (`test_users.py:376-378`), whose shapes stay;
  the app calls `DELETE …/tokens/{id}` (`ServerApi.kt:109`),
  `POST /api/v1/login/key` and `DELETE /api/v1/me/token`,
  whose answers only gain fields.

## Implementation Plan

Each block is a pull request of its own, stacked in the blocks' order,
each branch on the one before; its documents come with it,
and a `CHANGELOG.d/` fragment, its paths being shipped;
its run goes into a NOTE (AGENTS.md §5.4).
Every test is seen failing on its base first.

### Block 1: Copy

- **Goal:**
  the key just made, shown once, copies as every copyable text does (D1-A);
  Block 5's new secret is shown in the same box.
- **Changes:**
  1. `src/shared/tokens.tsx`:
     the box draws the key as
     `<Copy text={shown} name={t("users.keyCopy")} icon cls="mono user-key" />`
     inside its paragraph:
     a click, Enter or Space on the key, or its icon, copies it;
     it is named "Copy the key",
     so that neither its name nor its tooltip repeats the key;
     the file's header says so.
  2. `ui/style.css`:
     `.user-key` keeps `word-break: break-all` and `user-select: all`;
     the icon follows the key's last line at 390 px.
  3. `ui/i18n/*.json`, FR, IT, NL and PL machine-translated and marked:
     `users.keyCopy`, "Copy the key" / "Den Schlüssel kopieren".
- **Documents:**
  `docs/ui.md`: My profile's Sessions and keys, a user's panel,
  and the `Tokens` row of "How the page is built".
- **Tests:**
  - `test_users.py`, `test_new_key_shown_once`:
    the key is a button to the keyboard, named "Copy the key" without the key,
    with its icon; a click, Enter and the icon each copy it,
    the clipboard granted as in `test_copy.py`.
  - `test_settings.py`, `test_my_profile_lists_and_creates_own_keys`:
    the same in My profile.
  - `test_admin_updates.py`:
    its reads of `.user-key` unchanged.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-keylife-copy.md`:
  "- The API key shown once after Create key copies with a click,
  Enter or its copy icon, as every copyable text of the page does."
- **Branch:**
  `claude/keylife-copy`, from `main`.
- **Commit:**
  `fix(ui): The key just made copies with the page's copy part and its icon`
- **Verification,** each by its exit code:
  `make ui-check`;
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./internal/server`;
  `PYTEST_ADDOPTS='test_users.py test_settings.py test_admin_updates.py test_copy.py' make ui-test`
  in the four cases;
  `GUIDELINES/shared/tools/changelog.sh check --staged --strict`;
  `GUIDELINES/shared/tools/check`.

### Block 2: Gap

- **Goal:**
  a session made from a key ends when the key ends,
  wherever a token is let in;
  revoking a key revokes the keys it made, recursively,
  on a revoke alone (D5-A, D8-A).
- **Changes:**
  1. `internal/store/migrations/0100_token_maker.sql`, main's next number:
     `tokens.made_by INTEGER`, the API key that made it,
     `NULL` for every other token;
     `CREATE INDEX tokens_made_by ON tokens (made_by) WHERE made_by IS NOT NULL`;
     no foreign key; `min_reader` 24, additive.
     `0100_token_maker.down.sql` drops both and loses which key made a key,
     and only that; `store.sql` written again.
  2. `internal/store/tokens.go`:
     - `Token.MadeBy`;
       `AddKey(ctx, userID, KeyOptions{Label, MadeBy})` makes an API key,
       `AddToken` staying for sessions and for keys of no maker;
     - the rule `alive` in place of `liveKey`:
       a token not revoked and not past its own end,
       and a session made from a key only while the key is neither;
       `TokenByHash`, `TokenEnded` and `Tokens` hold to it,
       `TokenAlive(ctx, id)` gives a token by id under it,
       and `AddKeySession` refuses a key past its end (`ErrNoToken`);
     - `RevokeToken` revokes, in its transaction, the live heirs,
       followed through `made_by` from key to key in any state,
       with their sessions and what those subscribed to push,
       and returns the heirs' ids; its callers take them.
  3. `internal/store/users.go`, `DeleteUser`:
     the heirs of the user's keys, in its transaction.
  4. `internal/store/push.go`, `PushSubscriptions`:
     a session made from a key is sent to only while the key is alive.
  5. `internal/store/bootstrap.go`:
     the revoke of a key that is no longer the file's takes its heirs (D8-A),
     its log line saying how many.
  6. `internal/server/auth.go`:
     `resolve` takes `TokenByHash`'s answer,
     the token's own end being in the rule;
     `stillAdmitted` asks `TokenAlive` in place of its two reads.
  7. `internal/server/users_api.go`:
     `postToken` makes the key with `AddKey`,
     `MadeBy` the bearer's id where the bearer is an API key,
     none for a session of any kind;
     `deleteToken` answers `{"ok", "id", "heirs"}`,
     the heirs revoked with the key, as `deleteMyToken` does (`login_api.go`);
     both log lines and the change line name them.
  8. `login_api.go`, `tokenView`: `made_by`, absent for none.
  9. `cmd/massimo/user.go`, `revoke`: its count says the heirs.
  10. The page:
      `src/shared/tokens.tsx`, a key made by a key says
      "made by the API key {label}" (`users.madeBy`),
      the label from the list, else `#<id>`;
      `src/shared/account.ts`, `revokeToken`'s message names the heirs
      (`users.revokedWithHeirs`, "Token {id} revoked, with the keys it made:
      {ids}."); `src/api.d.ts` by `make api-types`;
      the six `ui/i18n/*.json`, marked as in Block 1.
- **Documents:**
  `docs/api-v1.md`: "Tokens" (`made_by`, `heirs`, D5-A's rule,
  a key's end ending its session), "POST /api/v1/login, …,
  DELETE /api/v1/me/token" (the heirs), "Users" (a removal's);
  the OpenAPI file (`Token.made_by`, the revokes' answers);
  `docs/store.md`: `tokens.made_by` in the schema, `0100` among the migrations,
  section 6's "A session from a key" with the key's end,
  and a paragraph "Keys made by keys";
  `docs/ui.md`: the row and the message;
  `README.md`: `massimo user revoke`'s count.
- **Tests:**
  - `internal/store`:
    `TestKeySessions`, a session of a key past its end:
    refused by `TokenByHash`, ended for `TokenEnded`, not listed,
    not sent to by `PushSubscriptions`, not made by `AddKeySession`;
    `TestTokenAlive`;
    `TestKeyHeirs`: `made_by` kept, a revoke through three generations,
    a key past its end in between passing it on,
    an heir of another user, a key past its end leaving its heirs live,
    `DeleteUser` and the bootstrap's revoke taking them;
    the migration's tests of the Analysis.
  - `internal/server`:
    `TestLoginKey`, a session of a key past its end `401`, not counted;
    `TestStreamEndsWhenItsCallerIsNotLetIn`,
    its stream ended at the next keep-alive; `TestKeyHeirs`:
    a key made with a key as the bearer names it,
    one made by a password's session and one by a key's session name none,
    `DELETE …/tokens/{id}` and `DELETE /api/v1/me/token` answer the heirs,
    a user's removal takes them;
    `TestEndedTokensAreNotCounted` with such a session.
  - `cmd/massimo`:
    `user revoke`'s count.
  - `tools/ui-test/test_users.py`:
    a stubbed list with an heir draws its line;
    the revoke's message names the heirs.
- **CHANGELOG:**
  `CHANGELOG.d/Security/claude-keylife-gap.md`:
  "- A session made from an API key ends when the key ends, and revoking a key
  also revokes the keys it made, and theirs."
- **Branch:**
  `claude/keylife-gap`, on `claude/keylife-copy`.
- **Commit:**
  `fix(security:auth): A key's session ends with it, and its revoke takes the keys it made`
- **Verification,** each by its exit code:
  `make ui`, then `MASSIMO_ENV_FILE=/dev/null go test ./...`;
  `MASSIMO_ENV_FILE=/dev/null go test -race -count=1 -timeout 30m ./internal/server ./internal/store`;
  `gofmt -l .` prints nothing;
  `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`;
  `make cross-check`; `go run ./tools/apitypes -check`; `make ui-check`;
  `PYTEST_ADDOPTS='test_users.py test_settings.py' make ui-test` in the four cases;
  `make app-emulator-test` where KVM is open, CI's job in any case;
  `GUIDELINES/shared/tools/changelog.sh check --staged --strict`;
  `GUIDELINES/shared/tools/check`.

### Block 3: Idle limit

- **Goal:**
  an API key unused for the days the owner sets ends (MobileApp.D75's A;
  D2-A, D3-A, D6-A, D7-A), and a session's use counts as its key's.
- **Changes:**
  1. `internal/store/migrations/0101_token_idle.sql`:
     `tokens.idle_exempt INTEGER NOT NULL DEFAULT 0`, additive, `min_reader` 24;
     `0101_token_idle.down.sql` drops it, and the exclusions with it;
     `store.sql` written again.
  2. `internal/store/settings.go`:
     `KeyIdleDaysKey = "auth.key_idle_days"`; `internal/server/settings_api.go`,
     `intSettings`: default 0, the limit off (D2-A), 0 to 3650, applied at once.
  3. `internal/store/tokens.go`:
     - the idle part of `alive`: an API key not excluded,
       and not the settings file's, whose last use,
       or its making where it was never used,
       lies the limit's days before now or more, has ended;
       the cutoff read from the setting at each check, none for 0;
       the file's hash the one `Bootstrap` last applied, kept in memory;
     - `Token.IdleExempt`; `KeyOptions.IdleExempt`;
       `SetTokenLife(ctx, id, TokenLife{IdleExempt})`;
     - `Tokens` gives each key `EndsAt`,
       the earlier of its end date and its idle end, and `Ended`,
       by the same rule in Go,
       which a test holds to the SQL on one table of cases.
  4. `internal/store/bootstrap.go`:
     keeps the file's hash for the rule.
  5. `internal/store/opensetup.go`, `liveIntegrationKey`: `Ended` for `Expired`.
  6. `internal/server/auth.go`, `touch`:
     a session made from a key writes its key's last use and address too.
  7. `internal/server/users_api.go`:
     - `POST …/tokens` takes `idle_exempt`;
     - `PUT /api/v1/users/{name}/tokens/{id}` (`putToken`, `server.go`,
       with `users`) takes `{"idle_exempt": true|false}` and answers the token's view;
       a session `400`; an integration's key `400`; the settings file's `409`;
       the bearer that very key `403`;
       a caller that does not manage the user `403` (D7-A);
       another user's token `404`;
     - audited as `token.update` (`audit.go`, its change line from and to).
  8. `login_api.go`, `tokenView`:
     `idle_exempt` (true for the settings file's key),
     `idle_ends_at` (absent where the limit is off or the key excluded),
     `ended`.
  9. The page:
     - `src/admin/Users.tsx`, under the password sign-in's switches:
       "API keys end unused after", a `NumberField` in days, 0 for never,
       saved when left (`setKeyIdleDays` in `src/admin/sendUsers.ts`,
       read with `loadPasswordLogin`), and its line `users.keyIdleHint`:
       a key unused for longer ends at once,
       an integration's key is always held to it, the settings file's never;
     - `src/shared/tokens.tsx`, a key's line:
       "ends {date} unless used", "kept while unused",
       or the badge Ended with "unused since {date}";
       for a caller who manages the user,
       Create key's switch "Keep while unused",
       and on a key's row "Change…", a small form with that switch,
       Save and Cancel; neither on an integration's key,
       the settings file's, nor the key the page is signed in with;
       `setTokenLife` in `src/shared/account.ts`;
     - the six `ui/i18n/*.json`; `help.profileTokens` gains a sentence.
- **Documents:**
  `docs/api-v1.md`: "Tokens" (the limit, `PUT`, the new fields),
  "GET and PUT /api/v1/settings" (`auth.key_idle_days`),
  the capability table's `users` row (`PUT …/users/{name}/tokens/{id}`);
  the OpenAPI file;
  `docs/store.md`: the settings' keys, `tokens.idle_exempt`, `0101`,
  and section 6's "The life of a key";
  `docs/ui.md`: System settings → Users, My profile, the `Tokens` row;
  `docs/app.md`: the app's key kept alive by its logins and its session.
- **Tests:**
  - `internal/store`, `TestKeyIdle`: with the limit off nothing ends; on,
    a key unused past it is refused by `TokenByHash`, ended for `TokenEnded`,
    listed `Ended`, its session with it;
    an excluded key and the settings file's stay; an integration's ends;
    one never used counts from its making;
    the limit raised brings it back (D6-A); `TestKeyEndRule`,
    the SQL and Go forms on one table; the migration's tests.
  - `internal/server`:
    `auth.key_idle_days` taken from 0 to 3650, refused beyond,
    `null` the default; a session's use writes its key's (`touch`);
    `TestPutToken`, each refusal and the change line; `TestOpenAPICoversRoutes`,
    `TestCapabilityTableCoversRoutes`, `TestEveryActionHasALine`;
    `TestEndedTokensAreNotCounted` with an idle key;
    an integration's idle key no longer counts as live for the open setup.
  - `tools/ui-test/test_users.py`:
    the setting saved, read again and put back at the end, its line;
    under a limit, a key's line (the list stubbed);
    Change… and Create key's switch for a manager,
    none in My profile of a user who manages nobody,
    none on an integration's key or the settings file's;
    the switch sends `idle_exempt`.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-keylife-idle.md`:
  "- An API key ends when unused for the days the owner sets under
  System settings → Users, off by default;
  a key may be kept from it, an integration's never."
- **Branch:**
  `claude/keylife-idle`, on `claude/keylife-gap`.
- **Commit:**
  `feat(auth): API keys unused for the days the owner sets end`
- **Verification:**
  as Block 2's, with
  `PYTEST_ADDOPTS='test_users.py test_settings.py test_personal_updates.py' make ui-test`.

### Block 4: End date

- **Goal:**
  a key takes an optional end date at Create key and on its row,
  an integration's key included (MobileApp.D75's B; D3-A, D4-A, D6-A).
- **Changes:**
  1. `internal/store/tokens.go`:
     `KeyOptions.Expires`; `TokenLife.Expires`, a time or none;
     the package comment no longer says that a key never expires.
  2. `internal/server/users_api.go`:
     - `POST …/tokens` takes `expires_at`, RFC 3339, in the future,
       else `400`;
     - `PUT …/tokens/{id}` takes `expires_at`, a time or `null` for none:
       a session `400`; the settings file's `409` (D3-A);
       the bearer that very key `403` (D4-A);
       else the key's user or who manages it, as `targetUser` lets in;
       an integration's key by a holder of `users` or `integrations`.
  3. The page, `src/shared/tokens.tsx`:
     Create key's field "Ends" (`DateTime`, empty for never);
     a key's line "ends {date}", or the badge Ended with "ended {date}";
     Change… gains the end date and "No end",
     for the key's user and who manages it,
     never on the settings file's key nor the key the page is signed in with;
     the six `ui/i18n/*.json`; `help.profileTokens`.
- **Documents:**
  `docs/api-v1.md`: "Authentication" and "Tokens" (no longer "never expires";
  `expires_at` in `POST` and `PUT`); the OpenAPI file; `docs/store.md`:
  `expires_at`'s comment and section 6; `docs/ui.md`; `docs/app.md`:
  an ended key ends the login by fingerprint.
- **Tests:**
  - `internal/store`:
    a key ends at its date, its session with it, its heirs not (D5-A);
    moved later, it lives again (D6-A).
  - `internal/server`:
    `POST` with `expires_at`, one in the past `400`;
    `PUT` moves it and `null` removes it;
    each refusal;
    an integration's key's date set by a holder of `integrations`;
    `TestLoginKey`, a key past its end `401`, not counted.
  - `tools/ui-test/test_users.py`:
    a key made with an end date shows it; Change… moves it and removes it;
    none on the settings file's key or the page's own.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-keylife-end.md`:
  "- An API key may take an end date, at Create key or later on its row,
  an integration's key too; the key itself cannot move it."
- **Branch:**
  `claude/keylife-end`, on `claude/keylife-idle`.
- **Commit:**
  `feat(auth): An API key takes an end date, set at Create key and moved on its row`
- **Verification:**
  as Block 3's.

### Block 5: Re-key

- **Goal:**
  "New secret" on a key's row gives the key a new secret,
  shown once, its id and label kept (MobileApp.D76-A, D81-A).
- **Changes:**
  1. `internal/store/tokens.go`,
     `RekeyToken(ctx, id, keep int64) (Token, string, error)`:
     a live API key's hash replaced in one transaction;
     the old hash kept in a revoked row of its own (kind `api`,
     its user and label, made and revoked now);
     the key's session revoked unless it is `keep`,
     with what it subscribed to push;
     `ErrNoToken` where the key is not a live API key.
  2. `internal/server/users_api.go`, `rekeyToken`:
     `POST /api/v1/users/{name}/tokens/{id}/rekey` (`server.go`, `users`);
     a session `400`; an ended key `409`; the settings file's `409`;
     an integration's key `403`; the bearer that very key `403`;
     another user's token `404`;
     the caller's own session kept where it was made from that key;
     `200` with `{"token", "id", "kind", "label", "created_at"}` as `POST` answers,
     the command keeping it without `token`; audited as `token.rekey`,
     with a change line and an Info line.
  3. The page, `src/shared/tokens.tsx`:
     "New secret" on a live API key's row;
     not on the settings file's key, an integration's,
     the key the page is signed in with, nor, in the app, this device's key;
     it asks first (`Question`, `users.newSecretAsk`: "A new secret for {label}:
     whatever holds the key now is refused.
     The new secret is shown once."),
     then shows the secret in Block 1's box, under "The new secret of key {id},
     shown once" (`users.newSecretShown`);
     `rekeyToken` in `src/shared/account.ts`;
     the six `ui/i18n/*.json`; `help.profileTokens`.
- **Documents:**
  `docs/api-v1.md`: "Tokens" (the re-key) and the capability table's `users` row;
  the OpenAPI file; `docs/store.md`, section 6:
  the re-key and the old hash's row; `docs/ui.md`; `docs/app.md`:
  a key given a new secret elsewhere is refused at the next login by fingerprint,
  and the app forgets the kept login.
- **Tests:**
  - `internal/store`, `TestRekey`:
    the same id, label, end date, exclusion and maker;
    the old secret ended for `TokenEnded`, the new one alive;
    the key's session revoked, `keep`'s kept; its heirs stay.
  - `internal/server`, `TestRekey`:
    the new secret works, the old one is `401` and not counted;
    the caller's session stays, the key's other ends;
    each refusal;
    `TestTokenCreationKeepsNoSecret` for the re-key;
    the OpenAPI, capability and audit tests.
  - `tools/ui-test/test_users.py`:
    New secret asks, sends one `POST`, shows the secret in the copy box,
    and the row keeps its id;
    none on a session, the settings file's key or an integration's;
    none on this device's key with the fake bridge (`fake_bridge.js`).
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-keylife-rekey.md`:
  "- New secret on an API key's row gives the key a new secret, shown once,
  its id and label kept; what held the old one is refused."
- **Branch:**
  `claude/keylife-rekey`, on `claude/keylife-end`.
- **Commit:**
  `feat(auth): New secret gives an API key a new secret, its id and label kept`
- **Verification:**
  as Block 3's.

### Before each pull request

The branch merges the one it stands on, Block 1's `origin/main`;
`GUIDELINES/shared/tools/check` passes;
the whole UI suite once in one case on the committed tree;
the pull request's text in commit form, each block's proofs in it.

## Proposed commit message

The plan:

```
chore(aps): +KeyLife v1.0

AP KeyLife gives an API key a life of its own: an idle limit the owner sets, from which a key may be excluded but an integration's never, an optional end date per key, and New secret on its row, in five blocks and five stacked pull requests. It first closes the two gaps AP MobileApp found: a session made from a key that outlives the key's end, and the keys a leaked key made, which outlive its revoke. Its decisions are AP MobileApp's D75 to D81 and its own D1 to D5, taken at its TL;DR gate; D6-A to D8-A taken as recommended under PLAN++.

Key changes:
- GUIDELINES/wip/2026-11-27_16-53_AP_KeyLife_the_life_of_api_keys.md added, v1.0: D1 to D5 settled, D6-A to D8-A taken as recommended

File-revision-v1: GUIDELINES/wip/2026-11-27_16-53_AP_KeyLife_the_life_of_api_keys.md::v1.0::2026-11-27::All::first revision, from the TL;DR gate's decisions
Assisted-by: claude-code:claude-opus-5-5
```

Block 1:

```
fix(ui): The key just made copies with the page's copy part and its icon

For AP KeyLife v1.0, Block 1 (D1-A): the API key shown once after Create key is drawn with the shared Copy part, so that a click, Enter or Space on it, or its copy icon, copies it, named "Copy the key" without the key itself.

Key changes:
- src/shared/tokens.tsx and style.css: the key a Copy with its icon; users.keyCopy in the six i18n files.
- Tests: test_new_key_shown_once and test_my_profile_lists_and_creates_own_keys copy the key three ways.
- Docs: docs/ui.md; a Fixed fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
fix(security:auth): A key's session ends with it, and its revoke takes the keys it made

For AP KeyLife v1.0, Block 2 (D5-A, D8-A): a session made from an API key counts only while the key is neither revoked nor past its end, wherever a token is let in, counted or sent to. A key made with an API key as the bearer names its maker in tokens.made_by (migration 0100), and a revoke takes the keys it made, recursively; an end takes none, and a key made by a session is no heir.

Key changes:
- internal/store: migration 0100 and its down part; one rule for TokenByHash, TokenEnded, Tokens, TokenAlive, AddKeySession and PushSubscriptions; RevokeToken, DeleteUser and the bootstrap's revoke take the heirs.
- internal/server: postToken records the maker; the revokes answer heirs; stillAdmitted asks TokenAlive.
- The page: an heir's line, the revoke's message naming the heirs.
- Tests: TestKeySessions, TestTokenAlive, TestKeyHeirs (store and server), TestLoginKey, the stream's end, the UI's line.
- Docs: docs/api-v1.md, the OpenAPI file, docs/store.md, docs/ui.md, README.md; a Security fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Block 3:

```
feat(auth): API keys unused for the days the owner sets end

For AP KeyLife v1.0, Block 3 (MobileApp.D75; D2-A, D3-A, D6-A, D7-A): the setting auth.key_idle_days, off by default, ends an API key unused for that many days, its session with it; who manages the user may keep a key from it, never an integration's, and the settings file's key is always kept. A session's use counts as its key's.

Key changes:
- internal/store: migration 0101 (tokens.idle_exempt) and its down part; the idle part of the rule; SetTokenLife; the file's hash from Bootstrap.
- internal/server: the setting; PUT /api/v1/users/{name}/tokens/{id}, audited as token.update; touch writes a session's key; the token's idle_exempt, idle_ends_at and ended.
- The page: the setting under System settings → Users; a key's line, Change… and Create key's switch.
- Tests: TestKeyIdle, TestKeyEndRule, TestPutToken, the setting's bounds, the UI's setting and rows.
- Docs: docs/api-v1.md, the OpenAPI file, docs/store.md, docs/ui.md, docs/app.md; an Added fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Block 4:

```
feat(auth): An API key takes an end date, set at Create key and moved on its row

For AP KeyLife v1.0, Block 4 (MobileApp.D75; D3-A, D4-A, D6-A): POST /api/v1/users/{name}/tokens and PUT …/tokens/{id} take expires_at for an API key, an integration's included; the key itself cannot move it, and the settings file's key takes none.

Key changes:
- internal/store: KeyOptions.Expires, TokenLife.Expires.
- internal/server: expires_at in POST and PUT, with the refusals of D3-A and D4-A.
- The page: Create key's end, a key's line, Change…'s end date.
- Tests: the end and its move in the store, the API and the page.
- Docs: docs/api-v1.md, the OpenAPI file, docs/store.md, docs/ui.md, docs/app.md; an Added fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Block 5:

```
feat(auth): New secret gives an API key a new secret, its id and label kept

For AP KeyLife v1.0, Block 5 (MobileApp.D76-A, D81-A): POST /api/v1/users/{name}/tokens/{id}/rekey gives a live API key a new secret, shown once, and ends the key's session unless it is the caller's; the old secret is answered as an ended token. The settings file's key and an integration's take none.

Key changes:
- internal/store: RekeyToken, the old hash kept in a revoked row.
- internal/server: the route, its refusals, audited as token.rekey; the command keeps no secret.
- The page: New secret, asked first, the secret in the copy box.
- Tests: TestRekey (store and server), TestTokenCreationKeepsNoSecret, the UI's New secret.
- Docs: docs/api-v1.md, the OpenAPI file, docs/store.md, docs/ui.md, docs/app.md; an Added fragment.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-27): first revision;
  from AP MobileApp v1.4's decisions on My profile's keys (MobileApp.D75 to D81)
  and this plan's TL;DR gate (D1-A to D5-A, D5-A narrowed by the owner);
  D6-A to D8-A taken as recommended under PLAN++, the owner's to change.
