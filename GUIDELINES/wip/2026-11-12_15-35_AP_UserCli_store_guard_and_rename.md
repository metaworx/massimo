# AP UserCli v1.0: store guard and rename

## TL;DR

`massimo user` no longer writes a store nobody serves, says which store it wrote,
and learns `rename`;
a removed user's row gives its name up, so a new user under the name is a new row.

- `user …` without `-data`/`MASSIMO_DATA`, no store in the current directory
  → refused, naming the service's command; was: a new store, silently
- every write → `… (store <path>)`; another store than the service's → a warning
- `user rename <current> <new> [-no-password | -password-stdin]`
  → the new password asked (OPAQUE keys on the name), sessions ended, keys kept
- delete → the row renamed `<name>-del-YYYYMMDD-HHNN` (UTC);
  `add` under a removed name → a new row, no longer the old one's;
  a migration renames the rows removed before

### Blocks

1. Store guard: the refusal, the store line, the warning; tests; README, docs/store.md
2. Rename and removed names: `RenameUser`, `user rename`, delete's rename,
   migration 0015, `add` always a new row; tests; README, docs/store.md, docs/api-v1.md

## Discussion

On 2026-11-12 the owner set a password with `massimo user passwd` and made an API key
with `massimo user token` on the Pi 5 at breno, and neither signed in.
The commands had run without `sudo -u _massimo`:
run by anyone but the service user, without `-data` or `MASSIMO_DATA`,
the data directory is the current one,
and `user` creates a store there, with keys of its own, without saying so.
The service's store never saw the change.

The owner then asked for a `user rename`,
and for a removed user's row to be renamed `<user>-del-<YYYYMMDD_HHNN>`.

**The owner's decisions**, 2026-11-12, `PLAN++ UserCli` on the evaluation's gates:

- **F1-F3**, the store guard, as evaluated.
- **D9-C.**
  `user rename <current> <new>` asks for the new password,
  `-password-stdin` reads it, `-no-password` leaves the user with API keys only.
  The OPAQUE record is bound to the login name (the credential identifier
  from which the server derives the user's OPRF key, `internal/store/opaque.go`),
  so it cannot survive a rename; a new one is made under the new name.
- **D10-A.**
  The command line only; the store's function is there for the API later.
- **D13-A.**
  A removed user's row is renamed at the removal,
  and the rows removed before by a migration.
  `_` is not allowed in a login name (`^[a-z0-9][a-z0-9._@-]{0,63}$`),
  so the form is `<name>-del-YYYYMMDD-HHNN`.

**What D13-A changes on purpose:**
`docs/api-v1.md` says today that the name of a removed user can be used again,
"the new user takes over the old one's row, so the audit's history of the name
reads on".
That gives the new person the old person's id and uuid,
and so the old person's audit and log history.
After this plan a new user under the name is a new row;
the removed row keeps its history under its new name,
and the log's `user=` filter finds it by that name.

**Names chosen here**, routine and not decisions of the owner's:
the branch `claude/user-cli`;
the timestamp in UTC, as every time the store keeps;
`-<id>` added where the renamed name is taken;
the store's `RenameUser`; the migration `0015_removed_user_names.sql`.

**Not in this plan:**
a rename through the API or the web UI (D10-A);
the signed-out page and the sign-in card (AP SignIn);
sign-in through OpenID Connect (AP Oidc).

**Who runs it:**
this session, under the owner's `PLAN++ UserCli` of 2026-11-12:
the plan, its registration on main, both blocks on `claude/user-cli`,
the push and the pull request; no merge, retirement or release.

## Analysis

**The data directory** (`cmd/massimo/options.go`, `dataDir`):
`-data` (set as `MASSIMO_DATA`), else `MASSIMO_DATA` from the environment or
the settings file, else a Windows service's own directory,
else the service's for the service user (`serviceDefaults`), else `.`.
Only the last is chosen without anybody naming it.
`userCmd.open` and `store migrate` call `store.Open`,
which creates `massimo.db`, `node.key` and `opaque.key` where there are none.

**The guard** refuses that last case where no `massimo.db` is there yet,
before any password is asked.
A store already in the current directory, as a developer's checkout has,
is used as before.
`store migrate` exists to create a store, and does, where a directory was named.

**The service's store** is `install.StateDir` (`serviceStateDir`) on Linux and
macOS, `%ProgramData%\Massimo` on Windows.
It counts as installed where its directory exists;
for another user that is a `stat` of `/var/lib/massimo`,
which works though the directory is `0750`.

**Rename:**
`users.name` is `UNIQUE` over every row, removed ones too.
Sessions and keys, preferences and push subscriptions hang on the user's id,
and stay.
The audit's `target` is text (`user:<name>`) and keeps the old name, as written then.
Lockout rows keyed by the old name expire on their own.

**Removal:**
`DeleteUser` sets `deleted_at`;
`addUser` finds a removed row by name and revives it (the takeover above).
After the rename at removal, `addUser` finds no removed row under a name,
so the revival goes; its clearing of preferences and push subscriptions with it.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `cmd/massimo/servicedefaults.go` | 1 | `implicitDataDir`, `serviceStore`, `checkStoreDir`, `warnOtherStore` |
| `cmd/massimo/user.go` | 1, 2 | `open` guarded and warning; the store line; `rename` |
| `cmd/massimo/store.go` | 1 | `migrate` guarded |
| `cmd/massimo/user_test.go`, `store_test.go` | 1, 2 | the guard, the line, the warning; rename |
| `internal/store/users.go` | 2 | `RenameUser`; `DeleteUser` renames; `addUser` always inserts |
| `internal/store/migrations/0015_removed_user_names.sql`, `store.sql` | 2 | the rows removed before |
| `internal/store/users_test.go`, `migrate_test.go` | 2 | rename, removal, the migration |
| `internal/server/logsources_test.go` | 2 | a removed user's events by its new name |
| `README.md`, `docs/store.md` §6 "The commands" | 1, 2 | the data directory, the store line, `rename` |
| `docs/api-v1.md` | 2 | a removed user's name; the log's `user=` |
| `CHANGELOG.d/` | 1, 2 | a fragment per bullet |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over `internal/store` and `internal/server`,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube or a box.
One functional commit per block, citing `AP UserCli v1.0, Block <n>`.

### Block 1: Store guard

1. `cmd/massimo/servicedefaults.go`:
   - `implicitDataDir()`: no `MASSIMO_DATA`, no Windows service directory,
     not `serviceDefaults`;
   - `serviceStore()`: the service's data directory where it exists on this host,
     else empty;
   - `checkStoreDir(args ...string)`: with an implicit data directory and no
     `massimo.db` in it, an error naming the directory and, where the service
     is installed, `serviceExample(args...)`, else "give -data";
   - `warnOtherStore(args ...string)`: where the service is installed,
     this run is not the service user's, and the data directory is not the
     service's, a warning on standard error with the same command.
2. `cmd/massimo/user.go`: `open` runs both before `store.Open`;
   each write prints `(store <file>)` after its line:
   on standard output for `add`, `passwd`, `disable`, `enable`, `revoke`,
   on standard error beside the key for `token`.
3. `cmd/massimo/store.go`: `migrate` runs both.
4. Tests, run first on the old code where they must fail:
   an empty current directory without `MASSIMO_DATA` refuses `user add`
   and creates nothing;
   with a service directory set (`serviceStateDir`) the error names
   `sudo -u _massimo`, and a run on another directory warns;
   the store line names the file.
5. `README.md` and `docs/store.md` §6 "The commands":
   the refusal, the store line, the warning.
6. `CHANGELOG.d/Fixed/claude-user-cli-store-dir.md`:

   ```
   - `massimo user` no longer creates a store of its own in the current
     directory: it asks for `-data` or names the service's command, says
     which store it wrote, and warns of one that is not the service's.
   ```

**Verification:**
the test gate; on jaguar, `massimo user list` in an empty directory refuses
and leaves it empty, and with `-data` on a scratch store prints the store line.

### Block 2: Rename and removed names

1. `internal/store/users.go`:
   - `RenameUser(ctx, current, new, password string, by int64) (User, error)`:
     the new name checked as `add` checks it, `ErrUserExists` where any row has it,
     the record for the new name made before the transaction
     (`opaqueRecordFor`), name and credential written in one;
     an empty password leaves no password;
   - `DeleteUser`: the row's name becomes `<name>-del-YYYYMMDD-HHNN` of
     `deleted_at` in UTC, `-<id>` added where that is taken;
   - `addUser`: a name found is a user that exists; the revival goes.
2. `internal/store/migrations/0015_removed_user_names.sql`, `min_reader: 1`:
   every removed row renamed the same way; `store.sql` regenerated as the
   migrations' tests require.
3. `cmd/massimo/user.go`: `rename <current> <new> [-no-password | -password-stdin]`:
   the password asked twice on the terminal unless one of the flags;
   the sessions ended; the audit `user.rename` with `from`; the log line;
   `rename` in the usage.
4. Tests:
   `RenameUser` (signs in by OPAQUE under the new name and not the old,
   keys and preferences kept, sessions gone, a taken name refused,
   `-no-password` leaves none);
   `DeleteUser` then `AddUser` under the name gives a new id and uuid;
   the migration renames a removed row of a store at 0014;
   `user rename` through `runUserWith`;
   `logsources_test.go`: a removed user's events under the new name.
5. `README.md`, `docs/store.md` §6 "The commands": `rename`.
   `docs/api-v1.md`: the paragraph on a removed user's name; the log's `user=`.
6. `CHANGELOG.d/Added/claude-user-cli-rename.md`:

   ```
   - `massimo user rename <current> <new>` renames a user, asking for the
     password under the new name; its API keys and settings stay.
   ```

   `CHANGELOG.d/Changed/claude-user-cli-removed-names.md`:

   ```
   - A removed user's row is renamed `<name>-del-<date>`, so a new user
     under the name starts with a row of its own rather than the old one's
     history.
   ```

**Verification:**
the test gate; `store info` of a scratch store before and after migration 0015;
`massimo user rename` on a scratch store, then a sign-in through a scratch
`serve` in the browser under the new name.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +UserCli v1.0

The owner's user commands on the breno Pi wrote a store of their own in the current directory, which the service never read; the owner asked for a guard against that, a rename command, and removed users that give their name up. PLAN++ UserCli of 2026-11-12, with F1-F3, D9-C, D10-A and D13-A.

Key changes:
- GUIDELINES/wip/2026-11-12_15-35_AP_UserCli_store_guard_and_rename.md: the plan, two blocks, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_15-35_AP_UserCli_store_guard_and_rename.md::v1.0::2026-11-12::All::the first revision, by PLAN++ UserCli
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `fix(cli): massimo user creates no store of its own and says which one it wrote`;
Block 2, `feat(store): massimo user rename, and a removed user gives its name up`.

## Change History

- v1.0 (2026-11-12): the first revision,
  from the evaluation of the breno login failure and the owner's
  F1-F3, D9-C, D10-A and D13-A of the same day.
