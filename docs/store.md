# Persistent storage for `massimo serve`

The design for the store the server keeps its own data in: what it is for,
the engine, where the file lives, what moves out of `.env`, the schema,
users and roles, the commands, the migration of an installed box,
the stages of the work and the decisions the owner takes before any code is written.
It follows section 4 of the proposal of 2026-11-05 and roadmap item 6;
docs/design.md sections 2, 6 and 7 are the frame.
Stage (a) shipped in v0.7.0-alpha.1,
and stage (b) is implemented but its web UI (section 9);
the owner's decisions are listed at the end. docs/server.md looks further ahead,
at encryption at rest, the public server and sync,
and says what version 1 prepares for them.

## 1. Why a store

Everything the server knows beyond the Cube's own data lives in one of two places today,
and both lose it:

- In `.env` (or `/etc/massimo/env`, `C:\ProgramData\Massimo\env`),
  read once at the start through `internal/env`,
  so every change needs an edit with root and a restart.
  Per Cube, from `env.go`: `MASSIMO_CUBE_<SN>_ADDRESS`, `_KEY`, `_IV`, `_PROXY`,
  `_PROXY_SERIAL`, `_IGNORE` (the hidden devices), `_NAME`,
  and `MASSIMO_CUBE_SN` for the commands.
  For the server: `MASSIMO_LISTEN`, `MASSIMO_API_TOKEN`, `MASSIMO_LOG`,
  `MASSIMO_UPDATE_REQUEST`, `MASSIMO_UPDATE_CHANNEL`, `MASSIMO_GITHUB_TOKEN`,
  and `MASSIMO_ENV_FILE` naming the file.
  `MASSIMO_UPDATE_CHANNEL` and `MASSIMO_GITHUB_TOKEN` are not in `env.Known`,
  so `massimo config` marks them as unknown names;
  that is a small bug to fix on the way.
- In the memory of the `serve` process, in `internal/server`:
  the last 500 events per Cube (`logSize`, `runtime.events`,
  what `GET …/log` and the UI's log view show),
  the Cubes discovery found (`Server.found`,
  the `foundCube` entries `GET /api/v1/cubes` lists with `"configured": false`),
  the devices that joined during the last pairing (`cubelan.InclusionState.Paired`),
  the lockout after wrong tokens (`authLimiter` in auth.go,
  from pull request #7), the stream tickets, the cached release check,
  and a Cube's `observed_since`.
  A restart, including the one every update does, empties all of it.

The UI's own settings (poll interval, expert mode, clock, language,
the chosen Cube, the token) are in the browser's `localStorage`, per browser,
so a phone and a laptop never agree,
and the colour schemes of the proposal's section 2 would join them there.

The features in the queue need something that outlives a restart and is editable
without root: Cube names, addresses and hidden devices edited in the UI;
found Cubes that stay found;
a log that survives an update and holds more than 500 lines;
a history of backups taken automatically;
users with roles instead of one token for everyone; and preferences per user.
The update helper already keeps two small files in `/var/lib/massimo`
and `/var/lib/massimo-update` (the request and `status.json`,
`internal/update/helper.go`); the store is the same idea for everything else,
in one file.

## 2. Engine

Three candidates, with the trade-offs as measured or as they follow from the code.
All numbers are stripped (`-s -w`), `CGO_ENABLED=0`,
`-trimpath` builds with Go 1.26.8, the release build's flags, on 2026-11-05;
`massimo` itself is 9.1 MB on linux/amd64 and 8.8 MB on linux/armv6 today.

### SQLite through `modernc.org/sqlite`

A translation of SQLite's C source to Go,
so it builds with `CGO_ENABLED=0` for every target `make cross` produces.
Checked: a minimal program using it cross-compiles on one machine for linux/armv6,
linux/arm64, darwin/arm64 and windows/amd64 without any C toolchain.

- Binary size: the minimal program is 6.6 MB against 1.6 MB for a program that
  only prints a line,
  so about 5 MB more per release binary on every target (6.5 MB on armv6 and arm64,
  6.4 MB on darwin/arm64, 6.5 MB on windows/amd64 for the same program).
  `massimo` would go from about 9 MB to about 14 MB.
  The Pi 1 downloads that once per update;
  the helper's test run of the new binary is unchanged.
- Dependencies: eight modules compiled in (`modernc.org/sqlite`,
  `modernc.org/libc`, `modernc.org/mathutil`, `modernc.org/memory`,
  `github.com/remyoudompheng/bigfft`, `github.com/dustin/go-humanize`,
  `github.com/google/uuid`, `golang.org/x/sys`,
  the last one already ours) and two more indirect in `go.mod` (`mattn/go-isatty`,
  `ncruces/go-strftime`) against the two of today. govulncheck in CI covers them;
  the review's "standard library first" rule gets its first large exception,
  which this document is the place to record.
- Go version: `modernc.org/sqlite` v1.61.1 declares `go 1.26.0`,
  so `go.mod` must rise from 1.24 to 1.26 (an older release of the module may
  declare less; to check when the stage starts).
  CI already pins 1.26 and the handover lists the rise as a follow-up anyway
  (for `x/sys` ≥ 0.44).
- Speed: slower than C SQLite, a few times on small statements.
  The server writes a few thousand rows a day and reads a page of the log now and then;
  on the Pi 1 that is nothing.
  Measured once on the Pi before stage (d) if there is any doubt.
- Queries: real SQL, so the log can be paged and filtered by time,
  kind and device, the audit by user, the backups listed and pruned by age,
  all in the engine with indexes; retention is one `DELETE`.
- The server mode of design.md (PostgreSQL,
  many tenants) gets the same schema with another driver;
  the SQL stays in the common subset (section 5).
  Tenancy is a file per tenant, not a column (section 5, docs/server.md),
  so design.md section 6's `tenant_id` lives in the server's account database,
  not here.
- One file to copy for a backup of the server itself, and a format
  every tool can open (`sqlite3 massimo.db`), which matters for the
  support bundle and the export of design.md section 7.

### `go.etcd.io/bbolt` with files

A key-value store in one file, pure Go, transactional.

- Binary size: 2.0 MB for the minimal program, so about 0.4 MB more;
  one module (`x/sys` is already there); declares `go 1.25`.
- No queries: buckets and byte keys.
  Settings, Cubes, users, tokens and preferences fit (one bucket each,
  JSON values), the log and the audit do not:
  paging by time and filtering by device means designing key prefixes and scanning,
  retention means iterating,
  and every question the UI asks later ("the setpoint changes of this room last
  week") is code instead of a query.
  The proposal's answer, the log and the backups as files next to the database,
  means three formats (bbolt, JSON lines with their own rotation,
  backup files with their own listing) where SQLite has one.
- No path to PostgreSQL: the server mode would be a second
  implementation of the whole store, not another driver.

### A JSON state file

Read at the start, written atomically on change,
like the update helper's `status.json`.

- No dependency at all and no binary growth.
- Enough for settings, Cube entries and a handful of users and preferences,
  read and written as a whole.
  Not for a log of tens of thousands of rows (every append rewrites the file),
  not for a backup history,
  not for anything two processes touch (`massimo user add` while `serve` runs).
  It would be replaced within two stages.

### Recommendation

SQLite through `modernc.org/sqlite`.
The costs are a bigger binary (5 MB), a dependency tree of ten modules,
and Go 1.26; the gain is that every feature in the queue is a table and a query,
that `massimo user add` can run beside the server,
that one file carries everything the box knows,
and that the server mode is a driver away. bbolt is the right answer for a
program that never asks questions of its data; the log view already does.

Either way the engine sits behind `internal/store`:
a `Store` type with methods per concern (`Cubes`, `Settings`, `Users`, `Tokens`,
`Preferences`, `Events`, `Audit`,
`Backups`) and plain Go types in the signatures,
no `database/sql` outside the package,
so the server and the commands never see the engine.
Tests of the store run against a temporary file;
tests of the server get a store in a temporary directory next to the fake Cube,
as today they get the fake Cube.

## 3. Location

One file, `massimo.db`, in the data directory.
SQLite in WAL mode keeps `massimo.db-wal` and `massimo.db-shm` next to it while
the server runs;
they belong to the database and are copied with it or not at all (a copy of the
`.db` alone while the server runs can miss the last transactions;
`massimo store backup <file>`, stage (a),
uses SQLite's online backup to write a consistent copy).

| Where | Data directory | Who | Mode |
|---|---|---|---|
| Linux service (`massimo install`) | `/var/lib/massimo` | user `massimo` | directory 0750 (`StateDirectory=massimo`), file 0600 |
| macOS LaunchDaemon | `/var/lib/massimo` | user `_massimo` | directory 0750 (`install.StateDir`), file 0600 |
| Windows service | `C:\ProgramData\Massimo` | LocalSystem | the directory's ACL: SYSTEM and Administrators only (`winSettings`) |
| Terminal run | the current directory, or `-data <dir>` | the user | file 0600 |

These are the directories the installers create today (`internal/install`:
`StateDirectory=massimo` in the systemd unit, `StateDir` on macOS,
`WinPaths.DataDir` on Windows) and the systemd unit's `WorkingDirectory`;
the update request already lives there on Linux and macOS.
`ProtectSystem=strict` in the unit leaves exactly that directory writable,
so no unit change is needed for the file to be created.

The data directory is a global option like `-env`: `massimo -data
<dir> serve …`, default `MASSIMO_DATA`, else the current directory. The
installers pass it explicitly in the unit,
the plist and the Windows service arguments (`ServiceArgs`),
so nothing depends on the working directory;
a Windows service runs in `System32`, so there the default is not usable,
and a binary updated by hand before `install` has written the new arguments
falls back to `%ProgramData%\Massimo` when it runs as a service (`svc.IsWindowsService`,
which `service_windows.go` already asks).
`massimo config` prints the data directory and whether the database exists.

The file is created by the server with mode 0600 (SQLite creates it with 0644
less the umask,
so the store creates the empty file itself first and opens it after) and the WAL
and SHM files inherit the database's mode.
Nothing in the store is a secret in version 1 except the token hashes and the
password hashes, but the log and the audit hold device names,
temperatures and who did what, which are personal data (design.md section 6).

Settings of the engine: `journal_mode=WAL` (readers do not block the writer,
and the `user` commands can run beside the server),
`synchronous=NORMAL` (a power cut loses at most the last transactions,
never the file; the SD card of a Pi 1 is the reason not to use `FULL`),
`busy_timeout=5000`, `foreign_keys=ON`,
one writing connection (`SetMaxOpenConns(1)` is enough at this load and avoids
`SQLITE_BUSY` inside one process).
Event rows are written by one goroutine in batches (every second or every 100 events),
so a burst after a reconnect is one transaction, not hundreds.

## 4. What moves out of `.env`, what stays

Stays in `.env`, because it is needed before the store is open, or is a secret:

| Variable | Why it stays |
|---|---|
| `MASSIMO_LISTEN`, `-listen` | the API's address, needed to start; changing it is a restart anyway |
| `MASSIMO_TLS`, `MASSIMO_TLS_CERT`, `MASSIMO_TLS_KEY`, `MASSIMO_TLS_NAMES`, `MASSIMO_HSTS`, `MASSIMO_ACME_EMAIL`, `MASSIMO_ACME_DIRECTORY`, `MASSIMO_TLS_HTTP`, `-tls`, `-tls-cert`, `-tls-key`, `-tls-names`, `-hsts`, `-acme-email`, `-acme-directory`, `-tls-http` | the listener's TLS, needed to start; `massimo install -tls …` writes them; the owner's files live beside this one, the self-signed pair and the ACME account and certificates in the data directory's `tls/` |
| `MASSIMO_LOG`, `-log` | opened before anything else, and the place errors of the store itself go |
| `MASSIMO_API_TOKEN`, `-token` | a secret; becomes the owner's API key (section 6) and keeps working as today |
| `MASSIMO_CUBE_<SN>_KEY`, `_IV` | secrets; stay in the file in version 1 (open decision 2) |
| `MASSIMO_DATA`, `-data` | names the store |
| `MASSIMO_ENV_FILE`, `-env` | names the file |
| `MASSIMO_UPDATE_REQUEST`, `MASSIMO_UPDATE_CHANNEL`, `MASSIMO_GITHUB_TOKEN` | the update helper's contract with root and a secret |
| `MASSIMO_CUBE_SN` | selects a Cube for the commands, which do not open the store |

Moves into the store, editable in the UI and through the API afterwards:

| Variable | Store |
|---|---|
| `MASSIMO_CUBE_<SN>_ADDRESS` | `cubes.address`; an empty address with `follow_discovery` is today's "configured by serial only" |
| `MASSIMO_CUBE_<SN>_PROXY` | `cubes.proxy_address` (the listener of the server for that Cube) |
| `MASSIMO_CUBE_<SN>_PROXY_SERIAL` | `cubes.proxy_serial` |
| `MASSIMO_CUBE_<SN>_IGNORE` | `cubes.hidden`, a JSON list of radio addresses |
| `MASSIMO_CUBE_<SN>_NAME` | `cubes.name` |
| the found Cubes | rows in `cubes` with `configured = 0` |
| the event ring | `events` |
| the lockout | `auth_failures` |
| the plain forms (`MASSIMO_CUBE_ADDRESS` with one Cube, `-addr`) | a row whose serial is filled in when the Cube reports it, as `serveTargets` handles it today |

New in the store, with no `.env` counterpart: the server's settings (`settings`:
the stale threshold `view.StaleAfter`, the poll and refresh intervals,
the discovery interval, the log retention, the backup retention;
the flags keep working and win for one run), users, tokens, preferences,
the audit and the backup history.

### Seeding and precedence

At every start, after the migrations, the server reads `.env` as it
does today and compares it with the store:

1. A Cube that has `.env` lines (`_ADDRESS`,
   or `_NAME` or `_PROXY` without an address,
   the same rule as `serveTargets`) and no row in `cubes` is added from those lines,
   with `source = 'env'`, and logged: `cube added from the settings file`,
   with the serial and the file.
   This is the seeding on the first start,
   and it is also the fallback for a Cube added to the file by hand later:
   the file keeps working as a way to add a Cube.
2. A Cube that has a row is served from the row.
   Its `.env` lines, if any, are compared;
   a line whose value differs from the row is logged once at the start
   (`settings file differs from the store, the store wins`,
   with the variable name and both values,
   except that keys are never logged) and shown in Settings → Cubes as a notice
   on that Cube with a button "Take the value from the file".
   The store wins because the UI edited it on purpose; a stale line in
   the file is the expected state on a box that was migrated and
   never had its file cleaned.
3. Keys and IVs come from `.env` by serial in every case;
   a Cube row without a key in the file is served without the portal commands,
   as today.
4. `-addr` and `-sn` on the command line work as today for one run and write nothing.

What the user sees: the first start after the update logs the store's creation,
the migration to version 1, the Cubes seeded and the owner created (section 6),
one line each.
Settings → Cubes shows every Cube with its source (file or UI), its values,
and the notice of a difference.
`massimo config` prints, next to each variable,
whether the store has a row for that Cube and which value is in effect.

Secrets in the store, encrypted at rest with a key from `.env`,
are a later stage (design.md section 7 asks for it).
Keeping them in the file in version 1 means the file stays the one place with secrets,
the backup of the database holds none, and the store's first version is smaller.

## 5. Schema, version 1

As SQLite would take it.
Times are `TEXT` in RFC 3339 with the `Z` suffix (sortable,
no time zone surprises on the Pi); JSON documents are `TEXT`;
booleans are `INTEGER` 0 or 1; identifiers are the rowid.
The dialect stays in the subset PostgreSQL takes with mechanical changes
(`INTEGER PRIMARY KEY` to identity columns, `TEXT` times to `timestamptz`,
JSON `TEXT` to `jsonb`), so the server mode is a second migration directory,
not a second schema.
There is no `tenant_id` and no `tenants` table (decided 2026-11-06,
against design.md section 6's "from day one"): a store is one tenant's,
on a box by nature and on the portal tier as one file per tenant (docs/server.md),
so a tenant column inside the file would be the same value on every row.
The server's account database, where tenants matter, has its own schema.
The file knows whose it is by `tenant.id`, a UUID in `settings`,
seeded at the first start next to `node.id`.

```sql
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,   -- the migration applied
    min_reader INTEGER NOT NULL,   -- the oldest schema version a binary may be built for and still open this file
    applied_at TEXT    NOT NULL,
    applied_by TEXT    NOT NULL    -- the massimo version string
);

CREATE TABLE roles (
    id         INTEGER PRIMARY KEY,
    key        TEXT    NOT NULL,              -- 'owner', 'manager', 'operator', 'guest', 'viewer', 'integration', or the key of a created role
    name       TEXT    NOT NULL DEFAULT '',   -- display name; '' shows the key's translated default
    builtin    INTEGER NOT NULL DEFAULT 0,    -- 1: seeded; owner is builtin and immutable
    disabled   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    created_by INTEGER REFERENCES users (id),
    UNIQUE (key)
);

CREATE TABLE role_capabilities (
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    capability TEXT    NOT NULL,              -- a key from the table in section 6
    PRIMARY KEY (role_id, capability)
);

CREATE TABLE capabilities_disabled (
    capability  TEXT    NOT NULL,
    disabled_at TEXT    NOT NULL,
    disabled_by INTEGER REFERENCES users (id),
    PRIMARY KEY (capability)
);

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL,              -- the login name, lower case
    display_name  TEXT    NOT NULL DEFAULT '',
    password_hash TEXT    NOT NULL DEFAULT '',   -- argon2id in PHC string form; '' = no password login (keys only)
    role_id       INTEGER NOT NULL REFERENCES roles (id),
    disabled      INTEGER NOT NULL DEFAULT 0,
    expires_at    TEXT,                          -- guests: disabled from this time on
    invited_by    INTEGER REFERENCES users (id),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    last_login_at TEXT,
    UNIQUE (name)
);

CREATE TABLE tokens (
    id             INTEGER PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('session', 'api')),
    hash           TEXT    NOT NULL UNIQUE,      -- SHA-256 of the token, hex; the token itself is never stored
    label          TEXT    NOT NULL DEFAULT '',  -- 'MASSIMO_API_TOKEN', 'Loxone', 'Firefox on the laptop'
    created_at     TEXT    NOT NULL,
    expires_at     TEXT,                         -- NULL: never (API keys); sessions: moved on every use
    last_used_at   TEXT,
    last_used_from TEXT    NOT NULL DEFAULT '',  -- the client's address
    revoked_at     TEXT
);
CREATE INDEX tokens_user ON tokens (user_id);

CREATE TABLE settings (
    key        TEXT    NOT NULL,   -- 'stale_after', 'poll_interval', 'log.keep_days', 'backups.keep_days', …
    value      TEXT    NOT NULL,   -- JSON: a string, a number or a document
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    PRIMARY KEY (key)
);

CREATE TABLE cubes (
    id                 INTEGER PRIMARY KEY,
    serial             TEXT    NOT NULL,              -- upper case, as printed on the Cube
    configured         INTEGER NOT NULL DEFAULT 1,    -- 0: found by discovery, not served
    name               TEXT    NOT NULL DEFAULT '',
    address            TEXT    NOT NULL DEFAULT '',   -- host[:port]; '' with follow_discovery: from discovery
    follow_discovery   INTEGER NOT NULL DEFAULT 0,    -- the address follows the Cube's discovery answers
    proxy_address      TEXT    NOT NULL DEFAULT '',   -- the server's listener for this Cube, host[:port]; '' = none
    proxy_serial       TEXT    NOT NULL DEFAULT '',   -- the serial used in discovery replies; '' = the Cube's
    hidden             TEXT    NOT NULL DEFAULT '[]', -- JSON list of radio addresses, as _IGNORE
    source             TEXT    NOT NULL DEFAULT 'env' CHECK (source IN ('env', 'ui', 'discovery', 'flag')),
    -- what discovery reported last, for GET /api/v1/cubes
    mac                TEXT    NOT NULL DEFAULT '',
    rf_address         TEXT    NOT NULL DEFAULT '',
    firmware           TEXT    NOT NULL DEFAULT '',
    discovered_address TEXT    NOT NULL DEFAULT '',
    first_seen         TEXT,
    last_seen          TEXT,
    created_at         TEXT    NOT NULL,
    updated_at         TEXT    NOT NULL,
    updated_by         INTEGER REFERENCES users (id),
    UNIQUE (serial)
);

CREATE TABLE preferences (
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,   -- 'scheme' (the massimo-scheme/1 document), 'ui' (poll, expert, clock, language)
    value      TEXT    NOT NULL,   -- JSON
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (user_id, key)
);

CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    cube_id   INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    device    TEXT    NOT NULL DEFAULT '',       -- radio address, '' for Cube-level events
    data      TEXT    NOT NULL                   -- JSON: the event object as GET …/log returns it today
);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_device_time ON events (cube_id, device, time);

CREATE TABLE audit (
    id        INTEGER PRIMARY KEY,
    time      TEXT    NOT NULL,
    user_id   INTEGER REFERENCES users (id),     -- NULL: no authentication configured, or the server itself
    token_id  INTEGER REFERENCES tokens (id),
    remote    TEXT    NOT NULL DEFAULT '',       -- the client's address
    action    TEXT    NOT NULL,                  -- 'setpoint', 'room.config', 'device.name', 'pairing.start', 'cube.update', 'user.add', 'login', 'login.failed', 'update.start', …
    cube_id   INTEGER REFERENCES cubes (id),
    target    TEXT    NOT NULL DEFAULT '',       -- 'device:0abc12', 'room:3', 'user:anna'
    detail    TEXT    NOT NULL DEFAULT '{}',     -- JSON: the request as received, secrets removed, and the result
    result    TEXT    NOT NULL DEFAULT 'ok' CHECK (result IN ('ok', 'refused', 'failed'))
);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);

CREATE TABLE backups (
    id         INTEGER PRIMARY KEY,
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('connect', 'daily', 'before_restore', 'manual', 'upload')),
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,
    body       BLOB    NOT NULL,                 -- the backup file, gzip
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual and before_restore
    note       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);

CREATE TABLE auth_failures (
    key          TEXT    PRIMARY KEY,            -- 'addr:<ip>' or 'user:<name>'
    window_start TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    locked_until TEXT
);
```

Notes on the tables:

- `cubes` holds the found Cubes too (`configured = 0`),
  so "Add" in the UI is an update of the row and `GET /api/v1/cubes` is one query;
  the discovery columns are what `foundCube` carries today.
  A Cube row is never deleted by the server;
  "Remove" sets `configured = 0` and keeps the events,
  the audit and the backups of the Cube.
- `events.data` is the event JSON as the stream and `GET …/log` send it today,
  so the UI's log view keeps its format; `kind`,
  `device` and `time` are copied out for the indexes.
  Retention is a daily `DELETE … WHERE time < ?` by `log.keep_days` (default 30,
  open decision 5), and a size estimate has to be measured on the Pi:
  a state event is a few hundred bytes,
  a house with thirty devices sends a few thousand a day,
  so roughly a megabyte a day and tens of megabytes at the default retention,
  in a file on an SD card.
- `audit` makes durable what today only the log has:
  which client sent which write.
  The `api()` wrapper in `server.go` is the one place every handler passes through,
  so one hook there records the action,
  the user and the result for every non-GET route; the handlers add the target.
  Login attempts and user changes are recorded as well.
  Nothing in `detail` is a secret:
  passwords and tokens are removed before the row is written.
- `backups.body` holds the file itself, gzip-compressed,
  because a backup of a Cube with thirty devices is tens of kilobytes and a year
  of daily backups is well under a hundred megabytes;
  one file to copy wins over a directory of files to keep in step.
  `before_restore` is the backup the restore API takes before writing (proposal
  section 3), `upload` is a file the user restored from.
  Retention by `backups.keep_days` with `manual` and `before_restore` kept
  longer (open decision 5).
- `auth_failures` is the `authLimiter` of #7 keyed by address,
  plus a key per user name so that password guessing from many addresses behind
  a reverse proxy locks the name rather than the proxy.
  The limits (ten in a minute, five minutes) stay.
  Rows outside the window and the lockout are pruned on every write to the table,
  which keeps it bounded as `maxTracked` does today.
- Identifiers: the rowid stays the internal key,
  and every row that will be replicated or exported (`users`, `tokens`, `cubes`,
  `preferences`, `events`, `audit`, `backups`,
  later `commands`) also carries `uuid TEXT NOT NULL UNIQUE`,
  a UUIDv7 (decision 7, taken 2026-11-06; docs/server.md says why).
  The owned tables (`users`, `tokens`, `cubes`, `settings`, `preferences`,
  `backups`) also carry `version INTEGER`,
  `deleted_at TEXT` and `origin TEXT` (decision 14, taken),
  and the first migration seeds `settings` with `node.id` and `tenant.id`.
  The listing above leaves those columns out so that it stays readable;
  the migration file has them.
- `roles`, `role_capabilities` and `capabilities_disabled` are the capability
  model of section 6;
  the first migration seeds the six default roles with their sets.
  `users.role_id` replaces the role text;
  `expires_at` and `invited_by` are for guests.
- Provenance and the audit (decided 2026-11-06):
  every row a user can edit carries `updated_at` and `updated_by` (`users`,
  `roles`, `cubes`, `settings`; `preferences` belong to their user,
  a token to its user, a backup has `user_id`),
  and `created_by` where it says something (`roles`; `users.invited_by`).
  That is the current state's provenance, cheap to show ("changed by anna,
  3 days ago") without a join over the audit,
  and it survives the audit's retention.
  The audit is the history: every write, who, when, what,
  including the refused ones; it is append-only and pruned,
  so it cannot serve as the current state.
  `role_capabilities` has no timestamps: a row is inserted or deleted,
  never updated,
  and the change of a role's set is recorded on the role row (`updated_at`,
  `updated_by`,
  `version`) and in the audit with the capability added or removed.
  Sync (docs/server.md) replicates owned rows by `version` and `updated_at`,
  so those two are needed on every owned row regardless of the audit;
  a role replicates with its set.
- Times are `TEXT` because SQLite has no date type:
  a column declared `DATETIME` has numeric affinity and stores whatever is inserted.
  The choices are text in ISO 8601,
  integer seconds or milliseconds since the epoch, or a Julian day as a real.
  Text in RFC 3339 with the `Z` suffix sorts correctly,
  reads in `sqlite3` and in exports and diffs,
  is what SQLite's own date functions take,
  and maps to PostgreSQL's `timestamptz` without conversion;
  the cost is 20 bytes against 8 and a string comparison,
  which is nothing at this scale.
  The columns are declared `TEXT`, not `DATETIME`, on purpose.

### Migrations

`internal/store/migrations/NNNN_<name>.sql`, embedded with `go:embed`,
applied in order inside one transaction each,
with the version and the `min_reader` of each written to `schema_version`.
The first file is `0001_initial.sql`, the whole schema above.
`0002_commands_auth.sql` (stage (b)) adds `users.auth_version`,
the `commands` table of docs/server.md item 7 (a parent row per write request
and a step row per engine command beneath it, with `parent_id`,
`idempotency_key`, `state`, `status`, `request` and `result`;
NOTE StoreUsers v1.0 of the plan: one table, `commands.keep_days` the retention)
and `audit.command_id`, additively.
Rules:

- A migration is additive when it only adds tables,
  nullable or defaulted columns, or indexes; it keeps `min_reader`.
  A binary built for schema version N opens a file at version M > N when
  `min_reader` of M is at most N, ignoring what it does not know.
  This is what makes a rollback of the binary safe (section 8).
- A migration that drops or renames something, or changes a meaning,
  raises `min_reader` to its own version,
  and the store copies the file to `massimo.db.before-<version>` first,
  with the WAL checkpointed,
  so that a rollback to an older binary has a file it can open.
  The log says so.
- A binary that finds a file with `min_reader` above its own version
  refuses to start with a message naming the copy and the versions;
  it never downgrades a file.
- The store refuses a file it did not create (no `schema_version`
  table) rather than migrating a stranger's database.
- Tests: migrating an empty file to the latest version,
  migrating a file at every earlier version (the test keeps a fixture database
  per released version),
  and opening a newer additive version with an older reader.

## 6. Users, roles and capabilities

A capability model, decided by the owner on 2026-11-06.
Every route is guarded by a **capability**; a **role** is a set of capabilities;
a user has one role.
The default roles and their sets are seeded at the first start and can be
changed afterwards by whoever holds the `roles` capability, with one exception:
`owner` always has every capability and cannot be edited, disabled or deleted,
and the store refuses to disable or delete the last enabled owner user.
The default roles keep their keys (`owner`, `manager`, `operator`, `guest`,
`viewer`, `integration`), so that the code,
the documentation and the translations can refer to them;
each has a display name that can be overwritten,
which is how a role is "renamed";
further roles can be created with any set of capabilities and deleted when no
user has them.
A capability can be disabled,
which denies it to every role but `owner` (for example `pair` while nobody should pair,
or `update` on a box the owner updates by hand);
that needs the `capabilities` capability, which only `owner` has by default.
A role can be disabled, which is the `roles.disable` capability;
a disabled role's users keep their login and are refused everything,
and the page says why.

The capabilities, what each allows, and the routes it guards:

| Capability | Allows | Routes |
|---|---|---|
| `read` | see everything of the Cubes and one's own user: rooms, devices, configuration, programmes, the log and the stream, the pairing and restore state, the update state | `GET /api/v1/cubes`, `…/cubes/{sn}`, `…/rooms`, `…/devices`, `…/devices/{rf}`, `…/rooms/{id}/config`, `…/rooms/{id}/program`, `…/log`, `…/events`, `POST …/events/ticket`, `GET …/pairing`, `GET …/restore`, `GET /api/v1/update`, `GET /api/v1/me`, one's own preferences |
| `setpoint` | set temperatures and modes | `PUT …/devices/{rf}/setpoint`, `PUT …/rooms/{id}/setpoint` |
| `lock` | lock and unlock the thermostats' buttons | `PUT …/devices/{rf}/lock` |
| `configure` | temperatures, device settings, week programmes | `PUT …/rooms/{id}/config`, `PUT …/devices/{rf}/config`, `PUT …/rooms/{id}/program`, `PUT …/devices/{rf}/program` |
| `rename` | room and device names | `PUT …/rooms/{id}`, `PUT …/devices/{rf}` with `name` |
| `structure` | move devices, delete rooms, remove devices | `PUT …/devices/{rf}` with `room_id`, `DELETE …/rooms/{id}`, `DELETE …/devices/{rf}` |
| `pair` | put the Cube into pairing mode and stop it | `POST …/pairing`, `DELETE …/pairing` |
| `discover` | search for Cubes on the LAN | `POST /api/v1/cubes/discover` |
| `backup` | download a backup, list the stored ones | `GET …/backup`, the list of stage (e) |
| `restore` | plan and run a restore | `POST …/restore/plan`, `POST …/restore` |
| `guests` | create, invite, disable and remove users whose role is `guest`, with an end date | the users routes, limited to guests |
| `users` | every user: create, role, disable, password reset, tokens | `GET`/`POST /api/v1/users`, `…/users/{name}`, `…/users/{name}/tokens` |
| `roles` | change a role's capabilities, rename a role, create and delete roles | `GET`/`POST /api/v1/roles`, `PUT`/`DELETE /api/v1/roles/{key}` |
| `roles.disable` | disable and enable a role | `PUT /api/v1/roles/{key}/disabled` |
| `capabilities` | disable and enable a capability | `GET /api/v1/capabilities`, `PUT /api/v1/capabilities/{key}` |
| `cubes` | add, configure and remove Cubes | `POST /api/v1/cubes`, `PUT`/`DELETE /api/v1/cubes/{sn}` |
| `settings` | the server's settings | `GET`/`PUT /api/v1/settings` |
| `update` | start a software update | `POST /api/v1/update` |
| `audit` | read the audit | `GET /api/v1/audit` |

The default roles, as seeded.
`owner` has every capability.

| Capability | `manager` | `operator` | `guest` | `viewer` | `integration` |
|---|---|---|---|---|---|
| `read` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `setpoint` | ✓ | ✓ | ✓ | | ✓ |
| `lock` | ✓ | ✓ | | | ✓ |
| `configure` | ✓ | ✓ | ✓ | | |
| `rename` | ✓ | | | | |
| `structure` | ✓ | | | | |
| `pair` | ✓ | | | | |
| `discover` | ✓ | ✓ | | | |
| `backup` | ✓ | ✓ | | | |
| `restore` | ✓ | | | | |
| `guests` | ✓ | | | | |
| `users`, `roles`, `roles.disable`, `capabilities`, `cubes`, `settings`, `update`, `audit` | | | | | |

So `manager` is `operator` plus guests, pairing, names, structure and restore;
`operator` runs the heating, locks, configures, discovers and takes backups;
`guest` is `operator` without the lock, backups and discovery; `viewer` reads;
`integration` is what a home automation needs and nothing it does not.
Moves and deletions (`structure`) go with the names to `manager`,
because a move changes the house as a name does; if `operator` should keep them,
that is one row of the seed.

Guests are users with the role `guest`,
created by a holder of `guests` with an invitation (a one-time code or link;
the guest sets a password) and an optional end date,
after which the user is disabled:
the time-limited share of design.md section 6 in its simplest form,
with `users.expires_at` and `users.invited_by` in the schema.
A holder of `guests` may only create,
change and remove users whose role is `guest`; everything else needs `users`.

The mapping is a table in `internal/server/auth.go` from route pattern to capability,
checked in the `api()` wrapper after the token is resolved: the user's role,
its capabilities minus the disabled ones, nothing for a disabled role,
everything for `owner`.
A new route without an entry fails closed,
and the test that compares the routes with the OpenAPI file (`openapi_test.go`)
also checks that every route has a capability;
the OpenAPI file documents the capability per operation.
The two routes whose capability depends on the body (`PUT …/devices/{rf}`:
`name` is `rename`,
`room_id` is `structure`) are checked in the handler after decoding,
and the table lists both.
The list of capabilities is Go code, not a table,
because a capability is a line in the route table;
the roles and their sets are rows (section 5), seeded by the first migration.

### Bootstrap and compatibility

- With `MASSIMO_API_TOKEN` set and no user in the store (the first start after
  the update),
  the server creates the user `owner` with the role `owner` and no password,
  and a token of kind `api` with the hash of `MASSIMO_API_TOKEN`,
  label `MASSIMO_API_TOKEN`.
  Every client that sends the token today,
  the Pi's scripts and the web UI with its stored token among them,
  keeps working with full rights, which is what the token means today.
- At every later start, a `MASSIMO_API_TOKEN` whose hash is not in `tokens` is
  added to the owner's keys with the same label,
  and one that was removed from the file is revoked.
  The file stays the way to recover an owner who lost the password:
  put a token into the file, restart, log in with it, set a password.
  Whoever can edit the file is root or the service user,
  who can edit the database anyway.
- Without `MASSIMO_API_TOKEN` and without users,
  the API stays open as today (acceptable on the default localhost listener,
  docs/api-v1.md says so already),
  and Settings shows "No users yet" with a form that creates the first owner
  with a password; the form works only while there are no users.
  `massimo user add` does the same from the command line (open decision 4 on
  whether the form should exist).
- With users present, every `/api` request needs a valid token:
  the `Authorization: Bearer` header as today,
  with a session token or an API key.
  Tokens are random, 32 bytes, shown once; the store holds the SHA-256.
  The tickets for the event stream stay as they are, in memory,
  issued to any token that may read the Cube; they carry the user for the audit.

### Login in the web UI

`POST /api/v1/login` with `{"user": "anna", "password": "…"}` answers
`{"token": "…", "expires_at": "…", "user": {"name": …, "role": …}}`;
`POST /api/v1/logout` revokes the session; `GET /api/v1/me` returns the user,
the role and its effective capabilities,
so the page can hide what the role may not do.
The page keeps the token in `localStorage` as `massimo.token` and sends the header,
exactly as it does with the API token today,
so the change in app.js is the login form in place of the token prompt (name and password,
or a token pasted into a second field for the integration case) and the
role-dependent hiding.
No cookies, so no CSRF handling; the token never goes into a URL.
Sessions last 30 days and are extended on use (`expires_at` moved,
at most once an hour);
the owner sees every session and key under Settings → Users and revokes them there.
Passwords are hashed with argon2id from `golang.org/x/crypto`, pure Go,
one more module;
the parameters are the library's recommended ones and are stored in the PHC
string so they can rise later.

### The commands

`massimo user add <name> -role owner|operator|viewer|integration`
(asks for the password twice, or `-no-password` for a key-only user),
`user list`, `user passwd <name>`, `user disable <name>`, `user enable
<name>`, `user token <name> -label <text>` (prints a new API key once),
`user revoke <name> [<token id>]`.
These are the only commands that open the store;
they take `-data` like `serve` and run as the service user
(`sudo -u massimo massimo -data /var/lib/massimo user add anna`),
beside a running server thanks to WAL.
The audit records them with `user_id` NULL and `remote` `cli`.

### The lockout

`authLimiter` keeps its behaviour, backed by `auth_failures` (section 5):
a failed token, ticket or password check counts against the address and,
for a login, against the user name;
ten in a minute lock for five minutes with `429` and `Retry-After`.
The counters survive a restart,
which closes the loophole where a restart cleared them.

### Later: ownership per Cube

Roadmap 6 names owner arbitration per Cube.
That is a stage after (b): a column `cubes.control_owner` (`''`, `app`,
`thermostat`, `loxone`, as design.md section 4 defines them,
per Cube first and per room when the room model exists) and a check in the write
paths that refuses or forwards writes according to it,
added by its own migration, and the lease for Loxone with it.
Nothing in version 1 depends on it.

## 7. The commands and the store

`status`, `backup`, `restore`, `watch`, `cube`, `discover`, `portal-crypto`,
`portal`, `config`, `legacy`,
`install` and `update` keep reading `.env` through `internal/env` and talk to
the Cube or to the proxy, as they do today; they do not open the store.
That is why `_ADDRESS`,
`_PROXY` and `_KEY` keep their meaning in the file on a box that runs the commands:
for the commands they are the configuration, for `serve` the seed.
`massimo config` says which is which.

The exceptions are `user` (section 6) and `store` (stage (a):
`store backup <file>` for a consistent copy,
`store migrate` to apply migrations without starting the server,
`store info` for the version and the counts), which open the store directly.

A later mode `massimo -server <url> status` lets the commands use the server's
API with a key instead of the Cube protocol,
which gives them the server's names and hidden lists, the audit,
and the no-direct-fallback behaviour the handover asks for.
It is not part of the store stages.

## 8. Migration of an installed box

The Pi runs `massimo serve` as a systemd service with `/etc/massimo/env`,
`WorkingDirectory=/var/lib/massimo` and `StateDirectory=massimo`,
and takes updates through the helper.

First start after the update to a release with stage (a):

1. `serve` reads `/etc/massimo/env` as before.
2. There is no `massimo.db` in the data directory (the working directory,
   `/var/lib/massimo`, until the unit passes `-data`; the same directory).
   The store creates the file with mode 0600,
   applies `0001_initial.sql` and writes `schema_version` 1.
3. Seeding (section 4): every Cube with lines in the file gets a row;
   the log says `store created` with the path,
   `schema migrated` with the version,
   and `cube added from the settings file` per Cube.
4. With stage (b): the owner and its API key from `MASSIMO_API_TOKEN`,
   logged as `owner created from MASSIMO_API_TOKEN`.
5. The Cubes connect as before,
   the proxies listen on the addresses from the rows, which are the file's.
   `/healthz` reports the new version, the helper's health check passes,
   the update is done.

The user does nothing.
The file keeps its lines; they are now seeds and fallbacks (section 4),
and the next `massimo install` writes a template whose comments say so.
The UI shows the Cubes under Settings → Cubes with source "settings file",
and from there on edits go to the store.

Rollback (`massimo update -rollback`,
or the helper after a failed health check):
the previous binary from `/usr/local/bin/massimo.prev` starts,
reads the file as it always did and ignores `massimo.db`, which stays in place.
Edits made in the UI meanwhile are not in the file,
so the rolled-back server serves the file's values;
the log says nothing about it because that binary knows no store.
The next update finds the database, opens it (section 5, migrations:
a later version opens an earlier file;
an earlier binary with store support opens a later additive file) and logs the
differences to the file as in section 4.

Windows: the same, with `C:\ProgramData\Massimo\massimo.db`;
the service's working directory is `System32`,
so the fallback for a service without `-data` (section 3) covers the binary
updated by hand before `install` has written the new arguments. macOS:
the same as Linux,
`WorkingDirectory` in the plist is `/var/lib/massimo` already.
A terminal run (`massimo serve` in a project directory with `.env`)
gets `./massimo.db` next to the file; `.gitignore` gets `massimo.db*`.

`install -remove` keeps the data directory as it keeps the settings;
`-purge` removes it, as it does today with `/var/lib/massimo`.

## 9. Stages

One pull request each, in this order,
every one leaving the box working without the next.
Every stage updates docs/api-v1.md and the OpenAPI file for its routes
(`TestOpenAPICoversRoutes` enforces it),
README.md for its variables and commands, packaging/README.md for the paths,
and docs/ui.md for its panels;
`go test -race ./...` and `make ui-test` at 1100 and 390 px before the merge.

### (a) The store, Cubes and settings

- Packages and files: `internal/store` (new: `store.go` with `Open`, `Close`,
  `Migrate`, the engine settings; `migrations/0001_initial.sql`; `cubes.go`,
  `settings.go`, `seed.go` taking the values `internal/env` reads;
  `backup.go` for the online copy; tests with a temporary file).
  `cmd/massimo/options.go` and `main.go` for `-data` and `MASSIMO_DATA`;
  `cmd/massimo/serve.go` opens the store and seeds from `serveTargets`' result
  instead of handing `CubeConfig` straight to the server;
  `cmd/massimo/store.go` (new: `backup`, `migrate`, `info`);
  `cmd/massimo/config.go` prints the store's view.
  `internal/server`: the runtimes come from `cubes` rows, a Cube can be added,
  changed and removed while the server runs (a runtime starts or stops;
  a changed proxy address restarts that listener), the found Cubes are rows,
  `summary` and `listCubes` read them; the settings (`StaleAfter`,
  intervals) come from `settings` with the flags winning.
  `internal/install`: `-data` in the unit's `ExecStart`,
  the plist and `ServiceArgs`; the env template's comments; `-purge` unchanged.
  `internal/env`: `Known` learns the two missing names and `MASSIMO_DATA`.
- API: `POST /api/v1/cubes` `{"serial": "KEQ0000000"}` (add a found Cube,
  or one by serial), `PUT /api/v1/cubes/{sn}` with any of `name`, `address`,
  `follow_discovery`, `proxy_address`, `proxy_serial`, `hidden`,
  `DELETE /api/v1/cubes/{sn}`; `GET`/`PUT /api/v1/settings`.
  `GET /api/v1/cubes` gains `source` and the difference notice.
- UI: Settings → Cubes: the list with the found ones and "Add",
  a panel per Cube with the fields above,
  the hidden devices as a list of the Cube's devices with switches (replacing
  the hand-written `_IGNORE` line), the notice when the file differs;
  Settings → Server: the stale threshold and the intervals.
- Tests: store (migrate from empty; seed twice is idempotent;
  the precedence of section 4 for each variable;
  a Cube added by hand to the file after the first start);
  server with the fake Cube and a temporary store (add, change,
  remove a Cube at run time; the found Cube becomes configured;
  the hidden list from the row hides as the variable did;
  `hidden_test.go` unchanged in outcome); install (`-data` in the unit,
  the plist and the Windows arguments); the smoke test (edit a name,
  hide a device, the tile disappears).

### (b) Users, login, tokens, audit

Implemented on 2026-11-06, the web UI on the components of the UI structure
work (AP StoreUsers v1.0).

- Packages and files: `internal/store/users.go`, `roles.go`, `tokens.go`,
  `audit.go`, `lockout.go`, `commands.go`, `password.go`, `bootstrap.go`,
  `migrations/0002_commands_auth.sql`; `internal/engine`:
  an `Observer` of every command with an `ID`;
  `internal/server/auth.go` rewritten around the store (token lookup by hash,
  the capability per route and the effective set per user,
  the limiter on `auth_failures`), `login_api.go` (new), `users_api.go` (new),
  `roles_api.go` (new),
  `audit.go` and `commands.go` (new) with the hooks in `api()`;
  `cmd/massimo/user.go` (new);
  `golang.org/x/crypto` for argon2id and `golang.org/x/term` for the prompt;
  `internal/server/ui`: `login.js`, `users.js`, `roles.js` (new),
  app.js (the token prompt becomes the login form, hiding by capability),
  i18n.js (one `auth.*` block).
- API: `POST /api/v1/login`, `POST /api/v1/logout`, `GET /api/v1/me`;
  `GET`/`POST /api/v1/users`, `GET`/`PUT`/`DELETE /api/v1/users/{name}`,
  `GET`/`POST /api/v1/users/{name}/tokens`,
  `DELETE /api/v1/users/{name}/tokens/{id}`; `GET`/`POST /api/v1/roles`,
  `PUT`/`DELETE /api/v1/roles/{key}`, `PUT /api/v1/roles/{key}/disabled`;
  `GET /api/v1/capabilities`, `PUT /api/v1/capabilities/{key}`;
  `GET /api/v1/audit?before=&limit=&user=`; `GET /api/v1/commands/{id}` and
  `Idempotency-Key` on every write route.
  Every existing route gets its capability (`x-capability` in the OpenAPI file).
- UI: the login page; Settings → Users: the list, add,
  invite a guest with an end date, role, disable, reset password,
  the sessions and keys with revoke; Settings → Roles:
  the capability grid per role, rename, create, disable,
  and the switch per capability; the first-owner form when there are no users,
  answered only to clients on the local network;
  "Forget token" becomes "Log out".
- Tests: the bootstrap from `MASSIMO_API_TOKEN` (the old token keeps full rights;
  a changed token is added; a removed one is revoked);
  each default role against each route (a table test over the route list),
  a created role, a disabled capability, a disabled role,
  the immutable owner and the last owner; a guest's end date;
  the lockout across a server restart;
  the audit row of every write handler with the fake Cube;
  the smoke test logs in as an operator and sees no Settings → Users.

### (c) Preferences

- Files: `internal/store/preferences.go`, `internal/server/me_api.go`;
  app.js and theme.js from the colour-scheme work (proposal section 2)
  read the server copy when logged in and fall back to the browser's.
- API: `GET`/`PUT`/`DELETE /api/v1/me/preferences/{key}`, the value a
  JSON document (`scheme` is `massimo-scheme/1`; `ui` is poll, expert,
  clock, language).
- UI: nothing visible; the settings follow the user across browsers;
  the browser copy is the cache and the fallback without a login.
- Tests: store round trip;
  the server refuses another user's key for a non-owner;
  the smoke test sets a scheme, reloads in a fresh context, logs in, sees it.

### (d) The log in the store

- Files: `internal/store/events.go` (batched writer, paging queries,
  the daily prune),
  `internal/server/server.go` (`remember` hands the event to the writer;
  `getLog` reads the store), app.js (the log view pages: "Older",
  filters by kind and device).
- API: `GET …/log?before=<time>&limit=&kind=&device=` replaces the one `limit`;
  the answer keeps the event objects.
  The ring of 500 goes; the stream is unchanged.
- Settings: `log.keep_days` under Settings → Server.
- Tests: a burst of events lands in one transaction;
  paging returns every event once; the prune keeps the window;
  the size per event measured in the test and noted in the document;
  the smoke test pages the log.

### (e) Backup history

After the restore API (proposal section 3) and its UI panel are merged.

- Files: `internal/store/backups.go`, `internal/server/backups_api.go` (new),
  `internal/backup` gains a diff between two files (the restore planner's comparison,
  read-only), backup.js (the panel from the restore work gains the list).
- API: `GET …/backups` (list: time, reason, size, who),
  `POST …/backups` (take one now), `GET …/backups/{id}` (download,
  format by `Accept` or `?format=`), `DELETE …/backups/{id}`,
  `GET …/backups/{id}/diff?against={id}`,
  and `POST …/restore` taking `backup_id` in place of a body.
- Automatic backups: one at the first connect of a day per Cube (`connect`),
  one before every restore (`before_restore`); retention by `backups.keep_days`,
  with `manual`, `upload` and `before_restore` kept for a longer window.
- UI: Settings → Backup and restore: the list, download,
  restore from a stored one, the diff view between two.
- Tests: the connect backup is taken once a day;
  the restore takes its backup first;
  the diff of a backup against itself is empty and against an edited copy lists the edits;
  the smoke test downloads one from the list.

After (e): ownership per Cube (section 6),
`-server` for the commands (section 7), secrets in the store,
the export and import of design.md section 7 on top of the store,
the PostgreSQL driver with the server mode.

## 10. Open decisions for the owner

Status on 2026-11-06: the owner took 4 (the form,
for local-network clients only), 7 (UUIDv7),
replaced 6 by the capability model of section 6,
and took every other item as recommended, 2, 3, 5 and 8 to 18 included.
Item 1, the engine, was taken the same day: SQLite;
the comparison the owner asked for stays below for the record.
Also decided that day:
no `tenant_id` in the site store and the provenance columns,
both in the notes of section 5.
Stage (a) shipped in v0.7.0-alpha.1.
Stage (b) was built the same day (AP StoreUsers v1.0):
item 6's seed row confirmed, the engine writing the states of each command's
steps (item 16), the first owner through `POST /api/v1/users`,
guests without an invitation flow until the UI needs one,
`golang.org/x/term` for the password prompt,
and one `commands` table with `commands.keep_days` rather than a history
table (NOTE StoreUsers v1.0: commands history).

1. Taken: SQLite through `modernc.org/sqlite`.
   For it: SQL with indexes and queries for the log,
   the audit and the backup history;
   one file per tenant on the portal tier with the same code (docs/server.md)
   and the same SQL for PostgreSQL later;
   transactions and migrations as plain SQL files;
   the `sqlite3` tool on any box to look at a file.
   Against: about 5 MB more per binary and eight modules more (section 2,
   measured), `go 1.26` in `go.mod`,
   a translated C code base as the dependency (large,
   but maintained and widely used), more surface for govulncheck.
   For `go.etcd.io/bbolt`: one small pure-Go module, 0.4 MB,
   transactions over key-value buckets, nothing translated from C.
   Against: no queries, so every list is a scan or a hand-built index kept in
   step by the code; the log and the backups as files beside it,
   each with its own retention;
   no path to the per-tenant files of the hosted engine or to PostgreSQL;
   no tool to inspect a file;
   a store interface that would have to hide two very different shapes if the
   choice ever changed.
   The recommendation stays SQLite;
   bbolt is the right answer only if the dependency weight outweighs the log,
   the audit and the server mode.
2. Secrets: keys, IVs and the API token stay in `.env` in version 1 (recommended),
   or move into the store encrypted with a key from `.env` from the start.
3. Precedence when the file and the store differ for a Cube:
   the store wins with a notice and a "take the file's value" button (recommended),
   or the file wins,
   which keeps file-based administration but silently undoes UI edits at every restart.
4. Taken: the first owner without `MASSIMO_API_TOKEN` is created with the
   one-time form in the UI,
   which works while there are no users and is answered only to clients on the
   local network: the request's own address is loopback,
   a private range (RFC 1918) or link-local; `X-Forwarded-For` is not trusted,
   so behind a reverse proxy the form is not offered and `massimo user add` on
   the box is the way.
5. Retention defaults: events 30 days, automatic backups 90 days,
   manual and before-restore backups 365 days; and whether `events`
   should also be capped by size for the SD card.
6. Replaced by the capability model of section 6: six default roles (`owner`,
   `manager`, `operator`, `guest`, `viewer`,
   `integration`) as sets of capabilities, adjustable except `owner`,
   with renames, further roles, and capabilities and roles that can be disabled.
   `MASSIMO_API_TOKEN` bootstraps an `owner`.
   Confirmed 2026-11-06:
   moves and deletions (`structure`) with `manager`, not with `operator`.
7. Taken: UUIDv7 from version 1 on every row that will be replicated
   or exported (docs/server.md says why and what it costs).
8. Browser sessions in the `Authorization` header stored in `localStorage`,
   as the token is today (recommended, no cookies and no CSRF),
   or a cookie session with CSRF tokens.
9. The data directory of a terminal run: the current directory (recommended;
   the file lands next to `.env`),
   or a per-user directory such as `$XDG_STATE_HOME/massimo`.
10. Whether the proxy listener addresses move into the store with the
    other Cube values (recommended; the listener restarts on a change)
    or stay in the file as listen addresses like `MASSIMO_LISTEN`.
11. Whether stage (b) follows (a) directly (recommended:
    the audit and the roles are what the UI edits of (a) ask for) or the
    preferences and the log come first because the colour-scheme work is waiting for them.

From docs/server.md, the later questions:

12. **End to end as the goal of the server mode** (recommended,
    since the data path, edge owns and server relays, is the same either way),
    or server-side encryption with operator-held keys, which keeps rendering,
    search and notifications on the server.
    The choice shapes what the server mode may ever do with the data;
    it does not change version 1 of the store.
13. **Local encryption at rest on the box** as a stage (f) with the key in a
    file in the state directory, once `internal/seal` exists,
    or left to the filesystem.
    Either way, the plaintext log file is the hole to close first.
14. **Tombstones, `version`,
    `origin` and the node identity from version 1** (recommended,
    docs/server.md, "What version 1 prepares", items 2 and 3).
15. **The password flow prepared for client-side derivation** with
    `auth_version` (recommended, docs/server.md, "What version 1 prepares",
    item 6).
16. Taken: `commands` with `Idempotency-Key` in stage (b) (docs/server.md,
    "What version 1 prepares", item 7),
    the engine writing the states of each step through an observer,
    one table with `commands.keep_days` rather than a history table.
17. **The browser's key derivation when end to end comes**:
    a WebAssembly argon2id module in the no-build UI,
    or PBKDF2 from WebCrypto with a high count.
    Not needed until then;
    noted so that the UI's "no dependencies" rule is bent knowingly, if it is.
18. **The trust anchor for the page** (docs/server.md, "Trusting the code"):
    the web app installed from the edge on the LAN with the service worker as
    the verifier, the app shells, or both.
