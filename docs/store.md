# Persistent storage for `massimo serve`

The design for the store the server keeps its own data in: what it is for,
the engine, where the file lives, what moves out of `.env`, the schema,
users and roles, the commands, the migration of an installed box,
the stages of the work and the decisions the owner takes before any code is written.
It follows section 4 of the proposal of 2026-11-05 and roadmap item 6;
docs/design.md sections 2, 6 and 7 are the frame.
Stages (a) to (c) shipped in v0.7.0-alpha.1 to v0.9.0-alpha.1,
and stages (d) and (e) are implemented (section 9);
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
  Measured on 2026-11-09 (section 5, `events`):
  the writer's batch of 100 events takes 0.15 s on the Pi 1;
  a page of errors, which read all of 90,000 events, took 5 to 16 s there,
  and since migration 0011's index of the warnings 1.7 s.
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
writes a consistent copy with SQLite's `VACUUM INTO`,
to a temporary file renamed into place once complete,
so that a crash leaves the file empty, never a partial database).
A clean stop of `serve` brings the WAL into `massimo.db` before it closes the
store (`PRAGMA wal_checkpoint(TRUNCATE)`, which also runs while another program
has the file open, where SQLite's own checkpoint at the last close does not)
and logs `store closed`, or why the file alone is behind;
`Store.Checkpoint` does the same for the running server.
A process that is killed leaves the WAL beside the file, its data safe,
and the next open reads it:
binaries before v0.10.0-alpha.1 ended so on SIGTERM and at a Windows service stop,
which is why a box updated from them kept `massimo.db` at the size it was created
with until its first clean stop.

| Where | Data directory | Who | Mode |
|---|---|---|---|
| Linux service (`massimo install`) | `/var/lib/massimo` | user `_massimo` | directory 0750 (`StateDirectory=massimo`, `StateDirectoryMode=0750`), file 0600 |
| macOS LaunchDaemon | `/var/lib/massimo` | user `_massimo` | directory 0750 (`install.StateDir`), file 0600 |
| Windows service | `C:\ProgramData\Massimo` | LocalSystem | the directory's ACL: SYSTEM and Administrators only (`winSettings`) |
| A command as `_massimo` (Linux, macOS) | `/var/lib/massimo`, unless `-env`, `-data` or `MASSIMO_DATA` | user `_massimo` | as the services' |
| Terminal run | the current directory, or `-data <dir>` | the user | file 0600 |

These are the directories the installers create today (`internal/install`:
`StateDirectory=massimo` in the systemd unit, `StateDir` on macOS,
`WinPaths.DataDir` on Windows) and the systemd unit's `WorkingDirectory`;
the update request already lives there on Linux and macOS.
`ProtectSystem=strict` in the unit leaves exactly that directory writable,
so no unit change is needed for the file to be created.

The data directory is a global option like `-env`: `massimo -data
<dir> serve …`, default `MASSIMO_DATA`, else the current directory;
for the service user, the owner of `/etc/massimo/env`, without `-env`,
`/var/lib/massimo` and that settings file alone (`useServiceDefaults`). The
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
| `MASSIMO_TLS`, `MASSIMO_TLS_CERT`, `MASSIMO_TLS_KEY`, `MASSIMO_TLS_NAMES`, `MASSIMO_HSTS`, `MASSIMO_ACME_EMAIL`, `MASSIMO_ACME_DIRECTORY`, `MASSIMO_TLS_HTTP`, `MASSIMO_ACME_DNS_HOOK`, `-tls`, `-tls-cert`, `-tls-key`, `-tls-names`, `-hsts`, `-acme-email`, `-acme-directory`, `-tls-http`, `-acme-dns-hook` | the listener's TLS, needed to start; `massimo install -tls …` writes them; the owner's files live beside this one, the self-signed pair and the ACME account and certificates in the data directory's `tls/` |
| `MASSIMO_LOG`, `-log` | opened before anything else, and the place errors of the store itself go |
| `MASSIMO_LOG_LEVEL`, `-log-level` | the level of that log, set before the store opens, whose own lines are logged at it |
| `MASSIMO_API_TOKEN`, `-token` | a secret; becomes the owner's API key (section 6) and keeps working as today |
| `MASSIMO_CUBE_<SN>_KEY`, `_IV` | read by nothing since AP PortalDrop: the portal commands are gone, kept on the branch `archive/portal`; a line left is named as ignored |
| `MASSIMO_DATA`, `-data` | names the store |
| `MASSIMO_ENV_FILE`, `-env` | names the file |
| `MASSIMO_UPDATE_REQUEST`, `MASSIMO_UPDATE_CHANNEL`, `MASSIMO_GITHUB_TOKEN` | the update helper's contract with root and a secret |
| `MASSIMO_REBOOT`, `-allow-reboot` | the owner's switch for the reboot of the computer, which the API must not be able to turn on (`docs/api-v1.md`, "GET /api/v1/reboot, …") |
| `MASSIMO_ANNOUNCE`, `-announce` | the announcement on the LAN by mDNS, which goes with the listener and starts with it (README.md, "Server") |
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
3. Keys and IVs came from `.env` by serial in every case,
   for the portal commands, which are gone since AP PortalDrop.
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
The listing below is the readable one, and leaves some columns out;
`internal/store/migrations/store.sql` is the complete schema at the latest version,
as the migrations leave an empty file ("Migrations", below).
Times are `TEXT` in RFC 3339 with the `Z` suffix (sortable,
no time zone surprises on the Pi); JSON documents are `TEXT`;
booleans are `INTEGER` 0 or 1; identifiers are the rowid.
The dialect stays in the subset PostgreSQL takes with mechanical changes
(`INTEGER PRIMARY KEY`, with `AUTOINCREMENT` or without, to identity columns,
`TEXT` times to `timestamptz`,
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
    key        TEXT    NOT NULL,   -- 'stale_after', 'poll_interval', 'log.keep_days', 'backups.keep_days_auto', …
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
    key        TEXT    NOT NULL,   -- 'scheme' (the massimo-scheme/1 document), 'ui' (poll, expert, clock, language), 'notify'
    value      TEXT    NOT NULL,   -- JSON
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (user_id, key)
);

CREATE TABLE push_subscriptions (            -- migration 0010
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_id   INTEGER NOT NULL REFERENCES tokens (id) ON DELETE CASCADE,  -- the session or key that subscribed
    endpoint   TEXT    NOT NULL UNIQUE,      -- the push service's URL
    p256dh     TEXT    NOT NULL,             -- the subscriber's keys, base64url (RFC 8291)
    auth       TEXT    NOT NULL,
    label      TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL,
    last_ok_at TEXT,                         -- the last message the push service took
    failures   INTEGER NOT NULL DEFAULT 0    -- failed deliveries since then
);

CREATE TABLE rooms (                         -- migration 0017
    id          INTEGER PRIMARY KEY,
    uuid        TEXT    NOT NULL UNIQUE,     -- the room's uid in the API (room_uid)
    cube_id     INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    room_id     INTEGER NOT NULL,            -- the Cube's room number
    temp_method TEXT    NOT NULL DEFAULT 'median',  -- median or mean: a room without a wall thermostat
    rf_address  TEXT    NOT NULL DEFAULT ''  -- the virtual wall thermostat's address; '' its room number
);
CREATE UNIQUE INDEX rooms_room ON rooms (cube_id, room_id) WHERE deleted_at IS NULL;

CREATE TABLE cube_options (                  -- migration 0018
    cube_id       INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    proxy_virtual INTEGER NOT NULL DEFAULT 0,  -- the proxy shows the virtual wall thermostats
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id)
);

CREATE TABLE log (                           -- events until migration 0014, made again by 0009 and 0014
    id         INTEGER PRIMARY KEY,
    cube_id    INTEGER REFERENCES cubes (id) ON DELETE CASCADE,  -- NULL: the server's
    time       TEXT    NOT NULL,
    kind       TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source     TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config, …; '' a row of a binary before 0009
    level      INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device     TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial     TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    user_id    INTEGER REFERENCES users (id),     -- who started it; NULL: the Cube, the server's own jobs, a server without users
    data       TEXT    NOT NULL,                  -- JSON: the event object as GET …/log returns it today
    command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL,  -- migration 0014: the command the row belongs to
    visibility TEXT    NOT NULL DEFAULT 'read' CHECK (visibility IN ('read', 'audit'))  -- 0014: audit for an auditor's eyes alone
);
CREATE INDEX log_cube_time ON log (cube_id, time);
-- migrations 0004 and 0009: each in id order within its terms,
-- since SQLite keeps the rowid last in an index
CREATE INDEX log_cube ON log (cube_id);
CREATE INDEX log_cube_kind ON log (cube_id, kind);
CREATE INDEX log_cube_source ON log (cube_id, source);
CREATE INDEX log_cube_serial ON log (cube_id, serial);
CREATE INDEX log_user ON log (user_id) WHERE user_id IS NOT NULL;
-- migration 0011: the warnings and errors alone, each Cube's in id order
CREATE INDEX log_cube_warn ON log (cube_id) WHERE level >= 4;
-- migration 0014: a command's rows
CREATE INDEX log_command ON log (command_id) WHERE command_id IS NOT NULL;
-- migration 0014: the log as the binaries before it knew it, with
-- INSTEAD OF triggers for their insert, update and delete
CREATE VIEW events AS SELECT id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data FROM log;

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
-- migration 0014: a command's audit row, and the SET NULL of a pruned command
CREATE INDEX audit_command ON audit (command_id) WHERE command_id IS NOT NULL;

CREATE TABLE backups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,  -- migration 0013
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('auto', 'manual', 'before_restore', 'upload')),  -- migration 0005
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,                 -- of the file before compression
    body       BLOB    NOT NULL,                 -- the backup file, gzip; empty in a tombstone
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual, before_restore and upload
    note       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);

CREATE TABLE auth_failures (
    key          TEXT    PRIMARY KEY,            -- 'addr:<ip>' or 'user:<name>'
    window_start TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    locked_until TEXT
);

CREATE TABLE auth_providers (                   -- migration 0016
    id            INTEGER PRIMARY KEY,
    uuid          TEXT    NOT NULL UNIQUE,
    key           TEXT    NOT NULL,             -- unique among the live ones
    name          TEXT    NOT NULL DEFAULT '',
    issuer        TEXT    NOT NULL,             -- https://…, its discovery document under it
    client_id     TEXT    NOT NULL,
    client_secret TEXT    NOT NULL DEFAULT '',  -- never answered
    scopes        TEXT    NOT NULL DEFAULT 'openid email profile',
    auto_create   INTEGER NOT NULL DEFAULT 0,
    auto_role_id  INTEGER REFERENCES roles (id), -- NULL: viewer
    auto_domains  TEXT    NOT NULL DEFAULT '',  -- comma-separated; '': any
    groups_claim  TEXT    NOT NULL DEFAULT 'groups', -- migration 0020: the claim of the groups
    allowed_groups TEXT   NOT NULL DEFAULT '[]', -- migration 0020: a JSON array; []: any
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE user_identities (                  -- migration 0016
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id  INTEGER NOT NULL REFERENCES auth_providers (id) ON DELETE CASCADE,
    subject      TEXT,                          -- the provider's sub; NULL until bound
    email        TEXT    NOT NULL DEFAULT '',   -- lower case
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id),
    bound_at     TEXT,
    last_used_at TEXT,
    name         TEXT    NOT NULL DEFAULT '',   -- migration 0019: at the provider, at the last sign-in or link
    username     TEXT    NOT NULL DEFAULT '',   -- preferred_username, likewise
    groups       TEXT    NOT NULL DEFAULT '[]'  -- a JSON array of strings, likewise
);
-- unique (provider_id, subject) where bound, (provider_id, email) where not
```

Notes on the tables:

- `cubes` holds the found Cubes too (`configured = 0`),
  so "Add" in the UI is an update of the row and `GET /api/v1/cubes` is one query;
  the discovery columns are what `foundCube` carries today.
  A Cube row is never deleted by the server;
  "Remove" sets `configured = 0` and keeps the events,
  the audit and the backups of the Cube.
- `log.data` (`events.data` before migration 0014) is the event JSON as the
  stream and `GET …/log` send it,
  so the UI's log view keeps its format,
  less a `state` event's `state.received`,
  the per-value times the tiles' ages read and the log does not;
  `kind`, `device` and `time` are copied out for the indexes,
  and the row's `id` is the log's cursor (stage (d)).
  Migration 0009 (AP Logging v1.1, Block 3) adds the server's own events,
  with `cube_id` NULL, and copies out `source`, `level`, `serial` and `user_id`:
  a Cube's log shows the server's events beside its own,
  and a page chooses a source, a level and above, a device by its serial
  and a user.
  A page reads one index from its end, in id order,
  under 2 ms at 90,000 events of one Cube (`TestEventsPageQuery`);
  with the server's events it is two such walks, the Cube's and the server's,
  each limited to the page and merged by id, since one query for both would sort.
  A page before a time looks up the newest id before it first.
  The level is no column of an index, its "and above" being a range that would
  give up the id order: a page reads past the rows below it.
  Before migration 0011, a page of warnings among 90,000 rows,
  with fewer than a page of them, read them all:
  140 ms on a PC, 154 ms on a Raspberry Pi 5,
  and 11 s on a Raspberry Pi 1, the store's one connection held all the while.
  0011's partial index `events_cube_warn` (`log_cube_warn` since 0014)
  holds the warnings and errors alone,
  each Cube's in id order, the server's under `cube_id IS NULL`.
  A page at `warn` or above carries the literal term `level >= 4` beside the bound level,
  since SQLite takes a partial index only for terms that imply its `WHERE`,
  and cannot tell that of a bound value.
  Such a page reads those rows only, under 2 ms on a PC at 90,000 events
  where it took 90 to 120 ms (`TestEventsPageQuery`);
  a page of errors reads at most the warnings:
  1.7 s on the Pi 1 at 90,000 events, a third of which are warnings there.
  The server writes no line at `error`, and the UI offers no such choice,
  so only `GET …/log?level=error` meets that walk.
  With a kind, a source, a device or a user as well,
  the page reads that term's index and checks the level on its rows.
  A batch of 100 events with a third warnings took 2 to 3 ms on the PC
  with the index as without it,
  the spread between runs larger than any difference.
  Migration 0014 (AP OneLog v1.1, Block 4) names a row's command,
  `command_id`, which the command's prune sets to NULL,
  and its `visibility`: `read` for what a user with `read` sees,
  `audit` for the rows only a user with `audit` sees (sign-ins, lockouts);
  a page by command reads `log_command`.
  Retention is a daily `DELETE … WHERE time < ?` by `log.keep_days`,
  per Cube and then for the server's rows,
  30 days by default as decision 5 took it, 0 keeping everything.
  Measured in `TestEventSize` (stage (d), and again with 0009's columns):
  a state event of about 420 bytes of JSON takes about 730 bytes in the file,
  its indexes included (640 before 0009),
  so a house with thirty devices that sends 3000 events a day adds about 2.2 MB a day
  and 66 MB at the default retention,
  in a file on an SD card.
  Measured on 2026-11-09 on a copy of each of the owner's two stores,
  with a test binary of the store at schema 8 and one at 9,
  on the copy and on the copy grown to 90,000 events with its own events,
  the median of 30 batches and of 10 pages:
  - The writer's batch of 100 events, in one transaction:
    124 ms at schema 8 and 147 ms at 9 on a Raspberry Pi 1 (one core, 426 MB),
    2.7 ms and 3.4 ms on a Raspberry Pi 5,
    so 0009's two indexes cost about a fifth more.
    Now and then a batch waits seconds on the Pi 1, at 8 as at 9,
    for a checkpoint of the WAL on the SD card.
  - A page of 200 events: 59 ms at 8 and 80 ms at 9 on the Pi 1,
    1.0 ms and 1.5 ms on the Pi 5.
  - A page of 200 warnings: 110 ms on the Pi 1,
    a third of whose events are warnings;
    154 ms on the Pi 5, whose busiest Cube has none,
    so the page reads every row.
    The same walk takes 11 s on the Pi 1
    (a page of errors, of which it has none),
    5 to 16 s from one page to the next.
  - Measured again on 2026-11-09 on new copies of both stores,
    with a test binary at schema 10 and one at 11, at 90,000 events:
    migration 0011 takes a page of 200 warnings from 58 ms to 1.9 ms on the Pi 5,
    and from 99 ms to 75 ms on the Pi 1, whose walk was short already;
    a page of errors, of which neither has any,
    from 153 ms to 2.0 ms on the Pi 5,
    and from 5.6 s to 1.7 s on the Pi 1, where it reads every warning.
    The writer's batch of 100 events took 146 ms at 10 and 145 ms at 11 on the Pi 1,
    3.3 ms and 3.4 ms on the Pi 5:
    the index costs the insert nothing to measure.
- `audit` makes durable what today only the log has:
  which client sent which write.
  The `api()` wrapper in `server.go` is the one place every handler passes through,
  so one hook there records the action,
  the user and the result for every non-GET route; the handlers add the target.
  Login attempts and user changes are recorded as well.
  Nothing in `detail` is a secret:
  passwords and tokens are removed before the row is written.
  Retention: the events' writer prunes the rows older than `audit.keep_days`
  beside the log, daily and when the setting changes, in chunks by
  `audit_time` (365 by default, 0 keeping everything; AP OneLog v1.1, Block 8),
  and says so in an `audit_pruned` line.
  Until then nothing pruned the audit, though this section said so.
- `backups.body` holds the file itself, gzip-compressed,
  because a backup of a Cube with thirty devices is tens of kilobytes,
  about 11 KB compressed (the six-device fixture: 24 KB of JSON, 2.2 KB gzip),
  and the server takes one only after a change;
  one file to copy wins over a directory of files to keep in step.
  `auto` is the server's own after a change of the Cube's configuration,
  `before_restore` the one a restore takes before writing,
  `upload` the file a restore was sent, `manual` one taken by hand (stage (e);
  migration 0005 replaced `connect` and `daily`).
  Retention per Cube: the newest `backups.keep_newest` whatever their age,
  of the others `auto` for `backups.keep_days_auto` days and the rest for
  `backups.keep_days_manual` (open decision 5's 90 and 365).
  The prune deletes the rows, so the id is `AUTOINCREMENT` (migration 0013):
  SQLite never gives a deleted backup's id to a later one,
  and an id the API named before a prune names no other backup.
- `auth_failures` is the `authLimiter` of #7 keyed by address,
  plus a key per user name so that password guessing from many addresses behind
  a reverse proxy locks the name rather than the proxy.
  The limits (ten in a minute, five minutes) stay.
  Rows outside the window and the lockout are pruned on every write to the table,
  which keeps it bounded as `maxTracked` does today.
- Identifiers: the rowid stays the internal key,
  and every row that will be replicated or exported (`users`, `tokens`, `cubes`,
  `preferences`, `log`, `audit`, `backups`,
  later `commands`) also carries `uuid TEXT NOT NULL UNIQUE`,
  a UUIDv7 (decision 7, taken 2026-11-06; docs/server.md says why).
  The owned tables (`users`, `tokens`, `cubes`, `settings`, `preferences`,
  `backups`) also carry `version INTEGER`,
  `deleted_at TEXT` and `origin TEXT` (decision 14, taken),
  and the first migration seeds `settings` with `node.id` and `tenant.id`.
  The listing above leaves those columns out so that it stays readable;
  the migration file has them, and so does `internal/store/migrations/store.sql`.
- `rooms` (migration 0017) are each Cube's rooms as the server knows them,
  kept in step with the Cube's metadata:
  a room it names gets a row and a uid, one it leaves out is marked deleted,
  and a room that takes a deleted room's number later gets a row of its own.
  The uid is the room's `room_uid` in the API, which a client keeps for the
  room's life; the other columns are the settings of the room's virtual wall
  thermostat (docs/api-v1.md, "The virtual wall thermostat").
  A row has `version`, `deleted_at` and `origin`, as the owned tables do.
- `cube_options` (migration 0018) are a Cube's options beyond its row,
  defaults where it has none: `proxy_virtual` (docs/proxy.md).
  A table of its own, so that a binary built for an earlier version reads
  `cubes` as before.
- `push_subscriptions` (migration 0010) are the browsers and apps a user's
  notifications go to by Web Push (docs/api-v1.md, "Notifications"):
  each tied to the session or API key that subscribed,
  so that revoking it, or removing the user, deletes the row;
  one whose session ended or whose user is disabled is kept and not sent to.
  `last_ok_at` is the last message its push service took, `failures` the
  failed ones since; one its push service knows no more (`404`, `410`),
  or failing for seven days, is deleted.
  A row has a `uuid` but no `version`, `deleted_at` or `origin`:
  a subscription is bound to this server's VAPID key, is not replicated,
  and is deleted rather than marked.
  The VAPID key pair (RFC 8292) is the setting `push.vapid`,
  made at its first use: the private key is in the store itself,
  not in a file beside it as the OPAQUE key is,
  so a `store backup` carries it; `GET /api/v1/settings` does not show it.
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

### Measuring on a box

The store's benchmarks (`internal/store/events_bench_test.go`)
time the writer's batch of 100 events, `BenchmarkWriteEvents`,
and pages of 200 of the busiest Cube, `BenchmarkEventsPage`:
all of its events, its warnings, its errors, and with the server's rows.
They share one store a run, made by their first call:
a copy of a box's store when `MASSIMO_STORE_BENCH_COPY` names one,
grown to `MASSIMO_STORE_BENCH_ROWS` events (90,000)
with the newest 1000 events of its busiest Cube, cycled;
else rows of their own, as `TestEventsPageQuery` fills them,
3000 under `-short`.
No writer runs, so nothing is pruned,
and each batch adds its rows to the store the pages read.
CI runs each once, to see that it still runs, and compares no time.

On one of the owner's boxes, only with the owner's go:

1. `make store-bench` builds `dist/store-bench-linux-arm64` for a Pi 5 and
   `dist/store-bench-linux-armv6` for a Pi 1;
   built from two commits, they compare a change.
2. The owner copies the store as the service's user,
   `sudo -u massimo massimo -env /dev/null -data /var/lib/massimo store backup /var/tmp/massimo-copy.db`,
   and gives the copy to the user who runs the benchmarks.
3. Each binary runs on the copy, one after the other,
   its own store on the SD card (`TMPDIR=/var/tmp`, since `/tmp` is a tmpfs):
   `TMPDIR=/var/tmp MASSIMO_STORE_BENCH_COPY=/var/tmp/massimo-copy.db ./store-bench-linux-armv6 -test.run '^$' -test.bench . -test.benchtime 30x -test.count 5`.
   Its first line names the store, its events and their levels;
   each benchmark's line gives the time a batch or a page took (`ns/op`)
   and the events a page held.
4. The copy and the binaries are deleted;
   only the times leave the box, and the service and its file stay untouched.

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
`0007_opaque.sql` adds `users.opaque_record` and `users.opaque_ksf`,
the record of a password of `auth_version` 2 and its key stretching (section 6,
"The sign-in by OPAQUE"), additively:
a binary of before reads such a user as one with keys only.
`0009_events_sources.sql` (AP Logging v1.1, Block 3) makes `events` again,
since SQLite cannot drop the NOT NULL of `cube_id`:
a new table, every row copied with its id and uuid,
the old one dropped, the new one renamed, the indexes made.
It fills the new columns from each row's JSON:
the serial from the event or its device view,
else from another row of the same Cube and radio address,
else for the Cube's own C line the Cube's;
the source from the event, else `system` for a restore's,
`cube` for the Cube's own C line and for a row of no device, `device` for the rest;
the level `warn` for `disconnected`, `debug` for `hello`, `info` for the rest.
It keeps `min_reader` 1: a binary of version 8 reads its Cube's rows by
`cube_id = ?`, never meeting a NULL one, and writes rows without the new
columns, which take their defaults;
the writer of a binary of version 9 fills those (`source` empty) at its start
(`TestOlderReaderOnNine`).
`0011_events_warnings.sql` (AP Logging v1.2, Block 8) adds `events_cube_warn`,
an index of the log's warnings and errors alone (section 5, `events`),
additively: a binary of before writes and reads as it did,
and SQLite keeps the index for its rows too.
`0014_log.sql` (AP OneLog v1.1, Block 4) makes `commands` again with
`AUTOINCREMENT`, for the reason 0013 gave the backups,
and `events` again as `log`, with `command_id` and `visibility`;
it declares `-- foreign_keys: off`, so the runner applies it with the
foreign keys off, set outside its transaction,
since `DROP TABLE` with them on deletes the rows first,
which would set every `audit.command_id` to NULL and delete the steps;
it runs `PRAGMA foreign_key_check` before the commit.
It keeps `min_reader` 1 by a view: `events` shows the log's columns as the
binaries before it knew them,
and its `INSTEAD OF` triggers turn their insert, update and delete into the
log's.
SQLite counts no rows changed through a view,
so the prune of such a binary removes one chunk per Cube a pass;
the next pass goes on (`TestEventsViewForOlderBinaries`).
The view goes in a migration that raises `min_reader` anyway.
`0013_backups_autoincrement.sql` (AP BackupIds v1.0) makes `backups` again
with `AUTOINCREMENT`, as 0005 made it:
without it SQLite gives a new row the highest id plus one,
so a prune that removed the newest backup handed its id to the next.
The copy starts `sqlite_sequence` at the highest id left,
and the rename carries it to the new table;
an id pruned before the upgrade can still be given out once more.
It keeps `min_reader` 1: the columns and their meaning stay,
and a binary of before chooses a new id by the table's schema as well.
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
  migrating a file at every earlier version,
  and opening a newer additive version with an older reader.
- The stores the releases wrote: `internal/store/testdata/releases/<tag>.sql`
  is the text dump of the store a release's `serve` wrote against the fake Cube,
  one for each release since the store came (v0.7.0-alpha.1),
  made by `tools/store-fixture/make.sh <tag>`,
  and since 2026-11-12 by `make release` itself:
  it dumps `HEAD` under the coming tag's name and flavour
  (`make.sh --ref HEAD --flavour <flavour> <tag>`),
  the release commit changing only the changelog,
  and commits the dump as `test(store): The store <tag> writes` before the release commit,
  so that the tag holds its own dump;
  the release CI's `sign` refuses a tag without it.
  The script builds the tag and sends every release the same requests:
  the settings, setpoints, a room renamed, a user and a key,
  a backup by hand, each where the release takes it.
  Then it dumps the release's own `store backup` with `sqlite3 .dump`.
  A dump is text, so a review reads it and `tools/identifiers` checks it;
  only placeholders reach it.
  `TestMigratesReleasedStores` loads each as its release left it and opens it,
  which migrates it:
  the file stays sound, no table loses a row, `events` keeps exactly its own,
  every event has its source, and the log, the users and the backups read.
  Each release's file is so migrated by every later binary;
  a tag's own dump, at its binary's schema, is opened as it is.
- `store.sql`, beside the migrations, is the schema they make of an empty file,
  generated, never edited by hand:
  every table as SQLite keeps its statement (an added column at its end),
  each followed by its indexes, by name,
  under the schema version and its `min_reader`.
  `TestStoreSQL` (`migrate_test.go`) fails, naming the first line that differs,
  until a new migration writes it again with
  `go test ./internal/store -run TestStoreSQL -update`;
  the file goes into the migration's commit,
  so that its pull request shows the migration's effect as a difference of it.
  It is for reading: it is no migration,
  the binary embeds only the numbered files,
  and a store is made by the migrations alone,
  which insert rows as well (the roles and capabilities, the node and tenant
  ids) and are the path every upgrade takes.
  One file is enough, since git keeps each version's
  (`git show <tag>:internal/store/migrations/store.sql`).
  `TestMigrationFilesNamed` holds the directory to the numbered migrations and
  `store.sql`: a file of another name would be skipped without a word.

## 6. Users, roles and capabilities

A capability model, decided by the owner on 2026-11-06.
Every route is guarded by a **capability**; a **role** is a set of capabilities;
a user has one role.
The default roles and their sets are seeded at the first start and can be
changed afterwards by whoever holds the `roles` capability, with one exception:
`owner` always has every capability and cannot be edited, disabled or deleted,
and the store refuses to disable, delete, demote or give an end date to the
last enabled owner user without one.
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
| `read` | see everything of the Cubes and one's own user: rooms, devices, configuration, programmes, the log and the stream, the pairing and restore state, the update state | `GET /api/v1/cubes`, `…/cubes/{sn}`, `…/rooms`, `…/devices`, `…/devices/{rf}`, `…/rooms/{id}/config`, `…/rooms/{id}/program`, `…/log`, `…/events`, `POST …/events/ticket`, `GET …/pairing`, `GET …/restore`, `GET /api/v1/update`, `GET /api/v1/me`, one's own preferences (`GET /api/v1/me/preferences`, any signed-in user) |
| `preferences` | keep one's own settings and colour schemes on the server, so that they follow the user across browsers; without it they stay in the browser | `PUT`/`DELETE /api/v1/me/preferences/{key}` |
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
| `passwords` | set the password of a user | the users routes, limited to a password |
| `roles` | change a role's capabilities, rename a role, create and delete roles | `GET`/`POST /api/v1/roles`, `PUT`/`DELETE /api/v1/roles/{key}` |
| `roles.disable` | disable and enable a role | `PUT /api/v1/roles/{key}/disabled` |
| `capabilities` | disable and enable a capability | `GET /api/v1/capabilities`, `PUT /api/v1/capabilities/{key}` |
| `cubes` | add, configure and remove Cubes | `POST /api/v1/cubes`, `PUT`/`DELETE /api/v1/cubes/{sn}` |
| `settings` | the server's settings | `GET`/`PUT /api/v1/settings` |
| `update` | start a software update | `POST /api/v1/update` |
| `reboot` | reboot the computer the server runs on, where its owner switched that on | `GET`/`POST`/`DELETE /api/v1/reboot` |
| `audit` | read the audit | `GET /api/v1/audit` |
| `auth` | the OpenID Connect providers users sign in through; seeded for no role | `GET`/`POST /api/v1/providers`, `PUT`/`DELETE /api/v1/providers/{key}` |

The default roles, as seeded.
`owner` has every capability.

| Capability | `manager` | `operator` | `guest` | `viewer` | `integration` |
|---|---|---|---|---|---|
| `read` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `preferences` | ✓ | ✓ | | ✓ | |
| `setpoint` | ✓ | ✓ | ✓ | | ✓ |
| `lock` | ✓ | ✓ | | | ✓ |
| `configure` | ✓ | ✓ | ✓ | | ✓ |
| `rename` | ✓ | | | | |
| `structure` | ✓ | | | | |
| `pair` | ✓ | | | | |
| `discover` | ✓ | ✓ | | | |
| `backup` | ✓ | ✓ | | | |
| `restore` | ✓ | | | | |
| `guests` | ✓ | | | | |
| `passwords` | ✓ | | | | |
| `restart` | ✓ | | | | |
| `users`, `roles`, `roles.disable`, `capabilities`, `cubes`, `settings`, `update`, `reboot`, `audit`, `auth` | | | | | |

So `manager` is `operator` plus guests, pairing, names, structure, restore,
the server's restart (migration 0006, 2026-11-07)
and the passwords of other users (migration 0008, 2026-11-08);
`reboot` is seeded for no role (2026-11-08): restarting the server is not
rebooting a computer that may run other services too;
`operator` runs the heating, locks, configures, discovers and takes backups;
`guest` is `operator` without the lock, backups, discovery and preferences;
`viewer` reads;
`integration` is what a home automation needs and nothing it does not:
it reads, sets, locks and configures, so that Home Assistant writes
week programmes as the web UI does (migration 0012, 2026-11-09,
where the role still held exactly read, setpoint and lock:
an owner who had changed it keeps the set chosen).
Moves and deletions (`structure`) go with the names to `manager`,
because a move changes the house as a name does; if `operator` should keep them,
that is one row of the seed.
`preferences` keeps one's settings and colour schemes on the server (stage (c));
a guest has it not, so a guest's settings stay on its devices,
where the defaults apply anyway (decided 2026-11-06),
and `integration` has no page to keep settings for.
A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
The rule holds for `users`, `guests` and `passwords` alike
(docs/api-v1.md, "Authentication").

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
- At every later start, and whenever `serve` reads the file again while it runs
  (every five seconds it looks, and on SIGHUP),
  a `MASSIMO_API_TOKEN` whose hash is not in `tokens` is
  added to the owner's keys with the same label,
  and one that was removed from the file is revoked.
  The file stays the way to recover an owner who lost the password:
  put a token into the file, log in with it, set a password.
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
one more module,
with OWASP's parameters on every box:
19 MiB, two passes, one lane (`m=19456,t=2,p=1`),
so that a hash costs the same wherever the store goes.
The first releases used the library's 64 MiB and one pass on four lanes,
which took 4.5 s a hash and 64 MiB an attempt on a Raspberry Pi 1 (one core,
426 MB, measured on 2026-11-07);
the new ones take about 55 % of that time and 30 % of that memory
(`BenchmarkPassword` in `internal/store`, on one core).
The PHC string carries the parameters, so an old hash still verifies,
and a sign-in that matches one hashes the password again with these:
an old hash goes at its user's next sign-in, at the cost of one hash more then.
Only `password_hash` is written, as `last_login_at` is,
and only where it is still the old one;
the row keeps its version and update time, since the password is the same.
A stored string that asks for more than 256 MiB, 16 passes or 16 lanes
is refused rather than allocated,
and so is one of another argon2 version than 19 or with a salt under 8 bytes.
A hash is the cost of the call, so the answers that need none come first:
a user add finds a taken name or an unknown role,
and a password change a user that is not there, before it hashes.

### The sign-in by OPAQUE

A user of `auth_version` 2 signs in by OPAQUE (RFC 9807),
and the server never sees the password: the client blinds it,
the server evaluates it under a key of its own for the user,
and the client runs argon2id over the result, in the browser,
at the parameters above, and keeps the password.
The server keeps a registration record in `users.opaque_record` (192 bytes:
the client's public key, a masking key and an envelope) with its argon2id
parameters in `users.opaque_ksf`, and `password_hash` empty;
nothing it keeps is a hash of anything the client sends.
A guess against a copy of the store needs the server's keys too,
`opaque.key` in the data directory (mode 0600,
made at the first open as `node.key` is: the AKE private key and the OPRF seed),
and then costs an argon2id a guess, as a hash does.
Lost, `opaque.key` makes every record useless and its user needs a password set again,
so the box's backup keeps the data directory, not the file alone.
The OPRF key of a user follows from the seed and the login name,
which does not change, so a name meets the same key with a record,
with a hash of version 1 or with no user at all;
a name without a record is answered from a fake one of the same shape,
and the start does not tell which names exist.
The suite is ristretto255-SHA512 with 3DH,
`github.com/bytemare/opaque` on the server and `@serenity-kit/opaque`
(`opaque-ke` in WebAssembly) in the browser.
`bytemare/ksf`, which the Go module pulls in, does not build for 32-bit
targets in its release 0.5.0; `third_party/bytemare-ksf` is that release with
the two lines fixed, through a `replace` in `go.mod`, until a release has it.

What it costs the server: a start (KE1 to KE2) takes 0.76 ms and 20 KB on one
amd64 core and 5.1 ms on a 32-bit build, the finish a MAC check
(`BenchmarkOpaqueLogin` in `internal/store`),
so about 0.1 s on the Raspberry Pi 1 by the ratio of section 6's hashes,
against about 2 s and 19 MiB for a hash.
The sign-in takes two requests (docs/api-v1.md, "The sign-in by OPAQUE"),
and so does a registration: the server answers the client's request,
keeping nothing, and the record arrives with the user add or the password
change in place of `password`.
One's own password change proves the current one with a sign-in whose finish
answers a proof, good once for five minutes, in place of a session.

A user of version 1 moves to version 2 at the next sign-in in the browser.
The browser cannot know a user's version, and the start does not say it,
so the start of a user of version 1 is answered from a fake record and the
browser cannot finish it;
while the box takes the password sign-in, the browser then sends the password,
over TLS, with the start's login, which the sign-in takes over,
so the attempt counts once.
The answer adds a proof, good once,
and the browser registers the same password and sets the record with it:
the hash is gone, and the password crosses no more.
That is the one time an existing password crosses;
the other is a mistyped password of a user of version 2,
which the browser cannot tell from one of version 1 while the box takes the
password sign-in.
The setting `auth.password_login` (true by default) ends both:
off, `POST /api/v1/login` answers `403`, the start says so,
and every user signs in by OPAQUE alone.
System settings → Users shows the switch to a holder of `settings`,
with how many users still have a hash,
who cannot sign in once it is off until a password is set for them.

### The commands

`massimo user add <name> -role owner|operator|viewer|integration`
(asks for the password twice, or `-no-password` for a key-only user),
`user list`, `user passwd <name>`,
`user rename <name> <new name>` (asks for the password again, or `-no-password`),
`user disable <name>`, `user enable <name>`,
`user token <name> -label <text>` (prints a new API key once),
`user tokens <name>` (the sessions and keys with their ids),
`user revoke <name> [<token id>]`.
These are the only commands that open the store;
they take `-data` like `serve` and run as the service user,
which reads the service's settings and store from any directory
(`sudo -u _massimo massimo user add anna`),
beside a running server thanks to WAL.
Run by anyone else, the data directory is `-data`, else `MASSIMO_DATA`,
else the current directory, where they create no store:
without `massimo.db` there they refuse,
naming the service's command where the service is installed,
so that a password or a key never goes to a store the server does not read
(the owner's commands on the breno box did, on 2026-11-12, without `sudo`).
`store migrate` creates a store only in a directory named so.
Each change names the store it wrote,
and a store other than the service's, where the service is installed,
draws a warning.
`user add` and `user passwd` set a password of version 2:
both halves of the registration run in the command,
with the server's keys from the data directory,
and the key stretching runs as the browser's library runs it,
with `opaque-ke`'s salt of 16 zero bytes and output of 64,
which RFC 9807 leaves open,
so that the record opens in the browser;
`tools/ui-test` checks that with a user the command added.
The record holds the name it was registered under,
the credential identifier from which the server derives the user's OPRF key,
so `user rename` registers the password again under the new name
and ends the user's sessions; its keys, preferences and audit stay with the row.
A removed user's row is renamed `<name>-del-YYYYMMDD-HHNN`,
the removal's time in UTC, `-<id>` added where that is taken,
so that the name is free and a user added under it is a row of its own
(migration 0015 renamed the rows removed before it;
until then a new user took the removed one's row over, with its history).
The audit records them with `user_id` NULL and `remote` `cli`.

### The lockout

`authLimiter` keeps its behaviour, backed by `auth_failures` (section 5):
a failed token, ticket or password check counts against the address and,
for a login, against the user name;
a request that offers no token and no ticket is refused uncounted
(`docs/api-v1.md`, "Authentication");
ten in a minute lock for five minutes with `429` and `Retry-After`.
The counters survive a restart,
which closes the loophole where a restart cleared them.

The counters see a failure only once its hash is done,
so attempts sent together would all pass them.
An address therefore signs in once at a time:
a login from an address whose sign-in is still being checked is `429`
with `Retry-After: 1`, which counts as no failure,
so that the lockout counts each attempt before the next one starts;
the web UI's dialog sends one at a time.
A sign-in by OPAQUE holds its address from the start to the finish,
at most a minute, and counts as a failure at the start:
the client can tell from the start's answer alone whether its password was right,
so a start never finished is a guess all the same.
The finish that proves the password clears the counters as a password sign-in does,
and lifts a lockout that its own start set.
And the hashes are bounded besides.
At most one a core, four at the most, run at once, and eight more wait;
a check, a user add or a password change beyond them
is answered `503` with `Retry-After: 5`.
So a box holds at most that many 19 MiB blocks for passwords in use at once,
and about as many again freed but not yet collected:
eight checks sent together peak at 38 MiB of heap with one slot,
152 MiB without the bound, and eight times 64 MiB with the parameters
of the first releases (`BenchmarkPasswordChecksAtOnce` in `internal/store`).

### Sign-in through OpenID Connect

A user may sign in through an OpenID Connect provider beside the password (AP Oidc):
Nextcloud, Google, Microsoft, Authentik, Keycloak,
each a row of `auth_providers` with its issuer,
whose discovery document names the endpoints, and the client's id and secret.
A user's identity at a provider is a row of `user_identities`:
the provider's subject (`sub`), which does not change,
finds the user at each sign-in.
An e-mail an owner assigns to a user is a pre-link without a subject,
bound to the subject at the first sign-in with that e-mail verified by the provider,
and by the subject alone from then on, since an address can change hands.
A waiting e-mail is one per provider.
Where a provider creates users (`auto_create`),
a sign-in without an identity creates one with the provider's role
(`viewer` where it names none),
for an e-mail of the allowed domains (any where none are named),
its name from the provider's `preferred_username` or the e-mail,
`-2`, `-3` added where taken.
A removed provider is a tombstone whose identities are deleted,
and a removed user's identities go with it.
An owner may assign the user id at the provider (`sub`) instead of an e-mail,
bound from the start (AP Oidc v1.1).
A user links its own account by a sign-in at the provider started from My profile,
which replaces the user's other identities at that provider;
an account another user has is refused.
Each sign-in and link writes what the provider said of the account:
the e-mail, the name, the user name and the groups (migration 0019).
A provider may admit only some groups, read from its groups claim (migration 0020):
an account in none of them neither signs in nor links.
An identity no sign-in has used may be changed;
a used one is removed and assigned again.
The client secret is sealed (AP Oidc v1.1, Block 9; AP SealedSecrets v1.0):
AES-256-GCM with a key derived from `opaque.key` (HKDF-SHA256, its own label),
so no key file of its own;
the column keeps `v2:` and the base64 of the nonce and the box,
the provider's uuid its additional data,
so that a box copied to another row does not open.
A copy or a backup of `massimo.db` alone so does not hold the secrets.
A plain secret, as v0.15.0-alpha.1 kept them,
is sealed at the next open of a writable store;
so is one a build of main sealed as `v1:` with a key file of its own, `secret.key`,
which is then renamed `secret.key.retired` for the owner to delete.
Lost, or another store's, `opaque.key` leaves each secret unopened, as each password:
the provider answers `secret_lost`, and the owner enters the secret again;
a change that does not write it keeps the box, which the key found again opens.
The secret is never answered (`has_secret`).

### Later: ownership per Cube

Roadmap 6 names owner arbitration per Cube.
That is a stage after (b): a column `cubes.control_owner` (`''`, `app`,
`thermostat`, `loxone`, as design.md section 4 defines them,
per Cube first and per room when the room model exists) and a check in the write
paths that refuses or forwards writes according to it,
added by its own migration, and the lease for Loxone with it.
Nothing in version 1 depends on it.

## 7. The commands and the store

`status`, `backup`, `restore`, `watch`, `cube`, `discover`, `config`, `legacy`,
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

Implemented on 2026-11-06 (AP StorePrefs v1.0).

- Files: `internal/store/preferences.go`,
  `migrations/0003_preferences_capability.sql`, `internal/server/me_api.go`;
  app.js,
  login.js and theme.js read the server copy when signed in and fall back to the
  browser's.
- The capability `preferences` (section 6): a role with it keeps its settings
  on the server; manager, operator and viewer have it,
  guest and integration not, so a guest's settings stay on its devices.
- API: `GET /api/v1/me/preferences`,
  `GET`/`PUT`/`DELETE /api/v1/me/preferences/{key}`, the value a JSON document:
  `ui` is poll, expert, clock, language, the mode and the chosen scheme;
  `schemes` the user's own, a list of `massimo-scheme/1` documents.
  A write is an audit row and no command.
- UI: the settings follow the user across browsers;
  under each a switch "only on this device" keeps its value in the browser,
  which then wins; the browser's keys stay the cache,
  and the only copy without a sign-in or the capability.
- Tests: the store's round trip; the routes per user, a guest refused the write,
  no user and no store; the pytest suite sets the poll and a scheme,
  opens a fresh context with the same token and sees them,
  keeps the mode for one device, and signs a guest in, which has no switches.

### (d) The log in the store

Implemented on 2026-11-06 (AP StoreLog v1.0), with its review's fixes (v1.1).

- Files: `internal/store/events.go` (the writer: a batch in one transaction at
  100 events or a second after the first, before a read and at the stop;
  the paging query; the daily prune per Cube in chunks of 5000,
  answering a read and a stop between two chunks);
  `internal/server/log.go` (`remember` hands a Cube's event to the writer,
  `GET …/log` reads the store), `log.js` (the log view pages with "Older",
  chooses a kind and a device).
  Migration 0004 (v1.1): the indexes a page walks in id order;
  the review found a page sorting every one of a Cube's rows,
  1.3 s at 90,000 events, while it held the store's one connection.
  The writer stops after the server's other goroutines,
  so that what they emit as they stop is written.
- API: `GET …/log?before=&limit=&kind=&device=`,
  `before` an event's `id` or a time, `limit` 1 to 1000 (200),
  each event with its `id`, oldest first; the stream is unchanged.
  The ring of 500 stays only for a Cube without a row in the store (`serve -addr`,
  a `-sn` Cube before discovery noted it);
  once it gains its row, the log is the store's, without the ring's events.
  A device hidden later keeps its events until the prune.
- Settings: `log.keep_days` in `/api/v1/settings` (30, 0 keeps everything,
  at most 3650), applied at once; Settings → Server waits for its own plan.
- UI: "Older events" above the log's lines, a choice of kind and of device;
  the server's log reaches back across restarts.
  The pages fetched stay past 2000 lines; an answer after Clear, another choice
  or another Cube is dropped; the error line shows whatever is chosen.
- Tests: a burst of events in one transaction;
  every event of a Cube once when paged by id, its times shared;
  the time bound and the filters; the prune per Cube in chunks;
  the size per event, about 640 bytes for a state event (section 5's note);
  the log across a restart on one store;
  the pytest suite's filters and Older. v1.1:
  the plan of every page shape without a sort;
  a prune that answers a flush and stops on a stop;
  the last events of a stopping server; the ring by device and by time;
  the pytest suite's late and failed answers.
- Decisions, as recommended: the cursor is the event's `id`,
  since a burst's events share a millisecond (D1);
  a page oldest first, as before (D2);
  the stored copy without `state.received` (D3); 30 days, 0 for ever (D4);
  the ring only as the fallback without a row (D5);
  the setting in the API only (D6); no migration 0004 for a kind index (D7).
  After the review: findings 1 to 8 fixed, 9 documented, 10's gaps closed (D8);
  each fix on the branch of its code (D9);
  migration 0004 after all, D7 reversed (D10).

### (e) Backup history

Implemented on 2026-11-07 (AP StoreBackups v1.0).

- Files: `internal/store/backups.go` (add, list, read,
  delete as a tombstone without the file, the prune),
  migrations 0005 (the four reasons) and 0013 (an id never given out twice);
  `internal/server/backups.go` (the automatic backup) and `backups_api.go` (the
  routes); `backup.ConfigHash` and `cubelan.SnapshotOf` (the comparison);
  `backup.js` with its partials (the Settings panel).
- When the server takes one: only after a change.
  The Cube's configuration, the decoded part of a backup without the live state,
  the time, the NTP servers and the raw lines, is hashed and compared with the
  newest backup's, or after a restore with the file restored;
  the backup follows `backups.auto_min` minutes (60) after the last change,
  at the latest `backups.auto_max` (120) after the first,
  and at once for a Cube without one, also once its last is deleted or pruned.
  The comparison runs once a minute after a connection or a configuration event,
  so changes made outside Massimo count once the Cube reports them;
  the reference lives in memory, read from the newest backup,
  and again from the one then newest once the newest is deleted,
  or pruned, which `backups.keep_newest` 0 allows.
  A restore takes `before_restore` first and is refused when it cannot,
  and keeps the file of its body as `upload`.
- API: `GET`/`POST …/backups` (the list newest first,
  with `changed` and `due` while one is pending; one by hand with a note),
  `GET …/backups/{id}` (as stored, or `?format=`),
  `DELETE …/backups/{id}` (capability `restore`),
  `GET …/backups/{id}/diff?against={id}`;
  the plan and the restore take `backup=<id>`; the settings `backups.auto_min`,
  `backups.auto_max` (never below `auto_min`), `backups.keep_days_auto`,
  `backups.keep_days_manual`, `backups.keep_newest`.
  A Cube without a row in the store has none of it (503).
- UI: Settings → Backup and restore: the download as YAML or JSON,
  a backup by hand, the list with download, the comparison with the one before,
  restore and delete, a restore from a file; a restore shows its plan first,
  then each step as the Cube took it.
- Tests: the store's round trip,
  the prune by reason and age and the migration's copy;
  no id given out again after a prune or a purged tombstone,
  and 0013's copy with its sequence; the hash;
  the first backup, none without a change,
  one after a quiet window and one at `auto_max`;
  the change against the one before after deleting the newest,
  and one at once, with an id of its own,
  after deleting all or pruning the only one;
  the routes and the settings' check;
  a restore keeping `before_restore` and `upload`, by id, and clean after;
  the diff of a backup against itself and an edited copy;
  the pytest suite's panel.
- Decisions, the owner's of 2026-11-07: the whole panel in this stage (D1);
  backups after a change with a quiet window and a latest (D2,
  D3 folded into it); the five settings (D4); deleting needs `restore` (D5);
  a restore refused without its own backup (D6); the planner's diff (D7).
  A newer version that decodes a field the older did not changes the hash,
  so one automatic backup may follow an update.

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
Stage (c) followed (AP StorePrefs v1.0): the keys `ui` and `schemes`,
a switch per setting for this device only,
and the capability `preferences`, which a guest has not.
Stage (d) followed (AP StoreLog v1.0 and v1.1): the log in the store,
paged by the event's id, 30 days kept, the ring only without a row,
migration 0004 for the pages' indexes.
Stage (e) followed (AP StoreBackups v1.0): the backup history,
taken after a change rather than daily, with the five `backups.*` settings,
the Settings panel, and migration 0005 for the reasons.

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
   a private range (RFC 1918) or link-local;
   `X-Forwarded-For` is believed only from the proxies `-trusted-proxies` names
   (docs/reverse-proxy.md),
   so behind one of them the form is offered to local-network clients,
   and behind any other `massimo user add` on the box is the way.
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
15. Taken: the password flow moved to the client with `auth_version` 2,
    the sign-in by OPAQUE (section 6; docs/server.md, "What version 1 prepares",
    item 6).
16. Taken: `commands` with `Idempotency-Key` in stage (b) (docs/server.md,
    "What version 1 prepares", item 7),
    the engine writing the states of each step through an observer,
    one table with `commands.keep_days` rather than a history table.
17. Taken: the browser's key derivation is a WebAssembly module in the no-build UI,
    `@serenity-kit/opaque` vendored as `ui/vendor/opaque.js`,
    with OPAQUE's argon2id in it,
    the UI's "no dependencies" rule bent knowingly for it (docs/ui.md,
    "How the page is built"); PBKDF2 from WebCrypto fits neither OPAQUE library.
    Its export key is the wrapping key's source when end to end comes (docs/server.md,
    "The key scheme").
18. **The trust anchor for the page** (docs/server.md, "Trusting the code"):
    the web app installed from the edge on the LAN with the service worker as
    the verifier, the app shells, or both.
