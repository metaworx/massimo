# Persistent storage for `massimo serve`

The design for the store the server keeps its own data in: what it is for,
the engine, where the file lives, what moves out of `.env`, the schema,
users and roles, the commands, the migration of an installed box,
the stages of the work and the decisions the owner takes before any code is written.
It follows section 4 of the proposal of 2026-11-05 and roadmap item 6;
docs/design.md sections 2, 6 and 7 are the frame.
Stages (a) to (c) shipped in v0.7.0-alpha.1 to v0.9.0-alpha.1,
and stages (d) and (e) are implemented (section 9);
the owner's decisions are listed in section 10, and the improvements left
for later in section 11. docs/server.md looks further ahead,
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
`/var/lib/massimo` and that settings file alone (`useServiceDefaults`).
The installers pass it explicitly in the unit,
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
| `MASSIMO_ANNOUNCE`, `-announce` | the announcement on the LAN by mDNS, which goes with the listener and starts with it, over the setting `announce` where given (README.md, "Server") |
| `MASSIMO_PUBLIC_URL` | the announcement's `url`, where the clients reach the API, before the address the trusted reverse proxy serves it at (README.md, "Server") |
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
   The plain lines (`MASSIMO_CUBE_ADDRESS` and the other plain forms,
   with no line of a serial) are the Cube's that `seed.plain_serial` names,
   compared with its row in the same way.
   The server sets it when the plain Cube first reports its serial,
   and the start when a configured row's address is the plain address
   (the same host whatever its case, an IPv6 zone in its own case,
   the same port, `62910` where one gives none),
   or, where no row's is as written, when a row at the same port has a host
   with an IP address of the plain host's: a host name and an IP of one Cube.
   Where several rows have one, none is tied, and the log names them.
   The start looks each name up once, all of them within 2 s,
   and a lookup that fails or does not answer in time ties nothing;
   `massimo config` shows the tie the same way.
   Two runtimes would dial one Cube, which serves one client.
   The log then says that the plain lines are that Cube's;
   the start's line names its row's address beside the plain one.
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
    uuid       TEXT    NOT NULL UNIQUE,       -- migration 0024: it replicates
    key        TEXT    NOT NULL,              -- 'owner', 'manager', 'operator', 'guest', 'viewer', 'integration', or the key of a created role
    name       TEXT    NOT NULL DEFAULT '',   -- display name; '' shows the key's translated default
    builtin    INTEGER NOT NULL DEFAULT 0,    -- 1: seeded; owner is builtin and immutable
    disabled   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    created_by INTEGER REFERENCES users (id)
);
-- migration 0024: the key unique among the roles not removed (UNIQUE (key) before)
CREATE UNIQUE INDEX roles_key ON roles (key) WHERE deleted_at IS NULL;

CREATE TABLE role_capabilities (
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    capability TEXT    NOT NULL,              -- a capability's key (section 6)
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
    integration_id INTEGER REFERENCES integrations (id),  -- migration 0024: the integration whose user it is
    disabled      INTEGER NOT NULL DEFAULT 0,
    expires_at    TEXT,                          -- guests: disabled from this time on
    invited_by    INTEGER REFERENCES users (id),
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    last_login_at TEXT
);
-- migration 0024: the name unique among the users not removed (UNIQUE (name) before)
CREATE UNIQUE INDEX users_name ON users (name) WHERE deleted_at IS NULL;

CREATE TABLE tokens (
    id             INTEGER PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('session', 'api')),
    hash           TEXT    NOT NULL UNIQUE,      -- SHA-256 of the token, hex; the token itself is never stored; a new secret keeps the old one in a revoked row (AP KeyLife, Block 5)
    label          TEXT    NOT NULL DEFAULT '',  -- 'MASSIMO_API_TOKEN', 'Loxone', 'Firefox on the laptop'
    created_at     TEXT    NOT NULL,
    expires_at     TEXT,                         -- sessions: moved on every use; API keys: the end date, NULL for none (AP KeyLife, Block 4)
    last_used_at   TEXT,
    last_used_from TEXT    NOT NULL DEFAULT '',  -- the client's address
    deleted_at     TEXT,                         -- revoked; revoked_at until migration 0024 (D7)
    parent_id      INTEGER,                      -- migration 0025: the key a session was made from; NULL for a sign-in's and for a key
    revoke_hash    TEXT,                         -- migration 0100: SHA-256 of the token's revoke code, hex; NULL where the store never held the secret
    made_by        INTEGER,                      -- migration 0101: the API key that made a key; NULL for every other token
    idle_exempt    INTEGER NOT NULL DEFAULT 0    -- migration 0102: 1 for an API key kept from the idle limit (auth.key_idle_days)
);
CREATE INDEX tokens_user ON tokens (user_id);
CREATE INDEX tokens_parent ON tokens (parent_id) WHERE parent_id IS NOT NULL;  -- migration 0025
CREATE UNIQUE INDEX tokens_revoke ON tokens (revoke_hash) WHERE revoke_hash IS NOT NULL;  -- migration 0100
CREATE INDEX tokens_made_by ON tokens (made_by) WHERE made_by IS NOT NULL;  -- migration 0101

CREATE TABLE settings (
    key        TEXT    NOT NULL,   -- 'stale_after', 'poll_interval', 'log.keep_days', 'backups.keep_days_auto', 'auth.key_idle_days', …
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
    proxy_virtual      INTEGER NOT NULL DEFAULT 0,    -- migration 0024, cube_options before: the proxy shows the virtual wall thermostats
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
    updated_by         INTEGER REFERENCES users (id)
);
-- migration 0024: the serial unique among the Cubes not removed (D11-A; UNIQUE (serial) before)
CREATE UNIQUE INDEX cubes_serial ON cubes (serial) WHERE deleted_at IS NULL;

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

-- cube_options (migration 0018) went into cubes.proxy_virtual in 0024

CREATE TABLE log (                           -- events until migration 0014, made again by 0009 and 0014
    id         INTEGER PRIMARY KEY,
    cube_id    INTEGER REFERENCES cubes (id) ON DELETE CASCADE,  -- NULL: the server's
    time       TEXT    NOT NULL,
    kind       TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source     TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config, …; '' a row of a binary before 0009
    level      INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device     TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial     TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    effective_user_id INTEGER REFERENCES users (id),  -- 0024, user_id before: whose rights it was started with; NULL: the Cube, the server's own jobs, a server without users
    data       TEXT    NOT NULL,                  -- JSON: the event object as GET …/log returns it today
    command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL,  -- migration 0014: the command the row belongs to
    visibility TEXT    NOT NULL DEFAULT 'read' CHECK (visibility IN ('read', 'audit')),  -- 0014: audit for an auditor's eyes alone
    real_user_id INTEGER REFERENCES users (id)    -- 0024: who started it, the effective user but for an integration's user acting for a linked one
);
CREATE INDEX log_cube_time ON log (cube_id, time);
-- migrations 0004 and 0009: each in id order within its terms,
-- since SQLite keeps the rowid last in an index
CREATE INDEX log_cube ON log (cube_id);
CREATE INDEX log_cube_kind ON log (cube_id, kind);
CREATE INDEX log_cube_source ON log (cube_id, source);
CREATE INDEX log_cube_serial ON log (cube_id, serial);
CREATE INDEX log_user ON log (effective_user_id) WHERE effective_user_id IS NOT NULL;
CREATE INDEX log_real_user ON log (real_user_id) WHERE real_user_id IS NOT NULL;  -- 0024
-- migration 0011: the warnings and errors alone, each Cube's in id order
CREATE INDEX log_cube_warn ON log (cube_id) WHERE level >= 4;
-- migration 0014: a command's rows
CREATE INDEX log_command ON log (command_id) WHERE command_id IS NOT NULL;
-- migration 0014 kept the log as the binaries before it knew it, a view
-- events with INSTEAD OF triggers; 0024 dropped it

CREATE TABLE audit (
    id        INTEGER PRIMARY KEY,
    time      TEXT    NOT NULL,
    effective_user_id INTEGER REFERENCES users (id),  -- 0024, user_id before: whose rights; NULL: no authentication configured, or the server itself
    real_user_id      INTEGER REFERENCES users (id),  -- 0024: who made the request, as in the log
    token_id  INTEGER REFERENCES tokens (id),
    remote    TEXT    NOT NULL DEFAULT '',       -- the client's address
    action    TEXT    NOT NULL,                  -- 'setpoint', 'room.config', 'device.name', 'pairing.start', 'cube.update', 'user.add', 'login', 'login.failed', 'update.start', …
    cube_id   INTEGER REFERENCES cubes (id),
    target    TEXT    NOT NULL DEFAULT '',       -- 'device:0abc12', 'room:3', 'user:anna'
    detail    TEXT    NOT NULL DEFAULT '{}',     -- JSON: the request as received, secrets removed, and the result
    result    TEXT    NOT NULL DEFAULT 'ok' CHECK (result IN ('ok', 'refused', 'failed'))
);
CREATE INDEX audit_time ON audit (time);
-- migration 0024: the filter by user (user_id, time before), and who made it
CREATE INDEX audit_user ON audit (effective_user_id, time) WHERE effective_user_id IS NOT NULL;
CREATE INDEX audit_real_user ON audit (real_user_id, time) WHERE real_user_id IS NOT NULL;
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
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual, before_restore and upload
    note       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);

CREATE TABLE backup_data (                      -- migration 0024 (D5-B), backups.body before
    id   INTEGER PRIMARY KEY REFERENCES backups (id) ON DELETE CASCADE,  -- the backup's
    data BLOB    NOT NULL                       -- the backup file, gzip; a tombstone has no row
);

CREATE TABLE backup_pending (                   -- migration 0026: the windows of a change's backup
    cube_id      INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    ref          TEXT    NOT NULL,  -- backup.ConfigHash of the reference they count against
    hash         TEXT    NOT NULL,  -- backup.ConfigHash at the last look
    first_change TEXT    NOT NULL,  -- backups.auto_max counts from it
    last_change  TEXT    NOT NULL   -- backups.auto_min counts from it
);

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
    auto_domains  TEXT    NOT NULL DEFAULT '[]', -- 0024: a JSON array, comma-separated before; []: any
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

CREATE TABLE integrations (                     -- migration 0021
    id               INTEGER PRIMARY KEY,
    uuid             TEXT    NOT NULL UNIQUE,
    kind             TEXT    NOT NULL,                -- homeassistant
    name             TEXT    NOT NULL DEFAULT '',     -- the installation's name as reported
    instance_id      TEXT    NOT NULL DEFAULT '',     -- the installation id the owner accepted
    pending_instance TEXT    NOT NULL DEFAULT '',     -- one seen and not accepted
    url              TEXT    NOT NULL DEFAULT '',
    app_version      TEXT    NOT NULL DEFAULT '',
    max_caps         TEXT    NOT NULL DEFAULT '[]',   -- JSON: the key's maximum, within the role integration
    state            TEXT    NOT NULL DEFAULT 'pending',  -- pending, active, disabled, refused
    request_code     TEXT    NOT NULL DEFAULT '',
    request_expires_at TEXT,                          -- 0024, request_expires before
    request_remote   TEXT    NOT NULL DEFAULT '',
    key_issued_at    TEXT,
    merged_into      INTEGER REFERENCES integrations (id)  -- a request approved as an earlier integration
    -- and created_at, updated_at, updated_by, version, deleted_at, origin;
    -- 0024 dropped key, user_id and system_user_id (D8-C): its users are
    -- those whose users.integration_id it is, named <uuid>:, <uuid>:system
    -- and <uuid>:<subject>
);

CREATE TABLE user_identities (                  -- migration 0016, made again by 0021 and 0024
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    effective_user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- 0024, user_id before: the user it acts as
    provider_id  INTEGER REFERENCES auth_providers (id) ON DELETE CASCADE,  -- an OIDC provider's, or
    integration_id INTEGER REFERENCES integrations (id) ON DELETE CASCADE,  -- an integration's: exactly one set
    subject      TEXT,                          -- the provider's sub, the integration's user id or system; NULL until bound
    email        TEXT    NOT NULL DEFAULT '',   -- lower case
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id),
    bound_at     TEXT,
    last_used_at TEXT,
    name         TEXT    NOT NULL DEFAULT '',   -- migration 0019: at the provider, at the last sign-in or link
    username     TEXT    NOT NULL DEFAULT '',   -- preferred_username, likewise
    groups       TEXT    NOT NULL DEFAULT '[]', -- a JSON array of strings, likewise
    real_user_id INTEGER REFERENCES users (id), -- 0021 (own_user_id until 0024), an integration's: the user made at its first call, kept when linked
    system_generated INTEGER NOT NULL DEFAULT 0,-- 0021: the integration's own system user (Home Assistant's Supervisor, …)
    last_level   TEXT    NOT NULL DEFAULT ''    -- 0021: the level of its last call, for display; 0024: '', view, control or admin
    -- 0024 (D6-B): and updated_at, version, deleted_at, origin; a removed identity is a tombstone
);
-- unique (provider_id, subject) where bound, (provider_id, email) where not,
-- (integration_id, subject) for an integration's, each among the identities not removed

CREATE TABLE server_backup_keys (               -- migration 0103: the server's backups
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    label        TEXT    NOT NULL DEFAULT '',   -- in plain in every manifest
    recipient    TEXT    NOT NULL,              -- age1pq1…, age1… or ssh-ed25519 …
    fingerprint  TEXT    NOT NULL UNIQUE,
    kind         TEXT    NOT NULL,              -- pq, x25519 or ssh-ed25519
    confirm_hash TEXT    NOT NULL DEFAULT '',   -- a key made, unconfirmed: hash of its last six characters
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id)
);

CREATE TABLE server_backup_targets (            -- migration 0103
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    name       TEXT    NOT NULL,                -- unique without regard to case
    kind       TEXT    NOT NULL,                -- local, webdav, sftp, smb, s3
    config     TEXT    NOT NULL DEFAULT '{}',   -- the kind's fields as JSON, its secret aside
    secret     TEXT    NOT NULL DEFAULT '',     -- sealed, the uuid the additional data
    takes      TEXT    NOT NULL DEFAULT 'every',-- every, daily, weekly or monthly
    retention  TEXT    NOT NULL DEFAULT '{}',   -- JSON: last, daily, weekly, monthly, yearly
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id)
);

CREATE TABLE server_backups (                   -- migration 0103
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,       -- the manifest's
    time         TEXT    NOT NULL,              -- the manifest's created; a failed one's last try
    trigger      TEXT    NOT NULL,              -- scheduled, manual, before-update, before-restore, uploaded
    user_id      INTEGER REFERENCES users (id), -- who asked; NULL: the schedule, a scan
    state        TEXT    NOT NULL DEFAULT 'made', -- made or failed
    size         INTEGER NOT NULL DEFAULT 0,
    sha256       TEXT    NOT NULL DEFAULT '',   -- the file's
    manifest     TEXT    NOT NULL DEFAULT '',   -- manifest.json as signed
    signature    TEXT    NOT NULL DEFAULT '',   -- manifest.sig
    pinned_until TEXT,
    error        TEXT    NOT NULL DEFAULT '',   -- a failed one's
    tries        INTEGER NOT NULL DEFAULT 0,
    next_try     TEXT
);

CREATE TABLE server_backup_copies (             -- migration 0103
    backup_id  INTEGER NOT NULL REFERENCES server_backups (id) ON DELETE CASCADE,
    target_id  INTEGER NOT NULL REFERENCES server_backup_targets (id) ON DELETE CASCADE,
    path       TEXT    NOT NULL DEFAULT '',     -- the file's name there
    state      TEXT    NOT NULL,                -- there, failed or missing
    pulled     INTEGER NOT NULL DEFAULT 0,      -- pulled by hand: kept until deleted
    pushed_at  TEXT,
    checked_at TEXT,
    error      TEXT    NOT NULL DEFAULT '',
    tries      INTEGER NOT NULL DEFAULT 0,
    next_try   TEXT,
    PRIMARY KEY (backup_id, target_id)
);
```

Notes on the tables:

- `cubes` holds the found Cubes too (`configured = 0`),
  so "Add" in the UI is an update of the row and `GET /api/v1/cubes` is one query;
  the discovery columns are what `foundCube` carries today.
  A Cube row is never deleted by the server;
  "Remove" sets `configured = 0` and `deleted_at` (`RemoveCube`) and keeps the events,
  the audit and the backups of the Cube with the row.
  Discovery makes no row for a serial with removed rows (D20-B):
  it is found in memory only until it is added, as new or restored.
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
  and a page chooses a source, a level and above,
  a device by its serial and a user.
  A page reads one index from its end, in id order,
  under 2 ms at 90,000 events of one Cube (`TestEventsPageQuery`);
  with the server's events it is two such walks, the Cube's and the server's,
  each limited to the page and merged by id,
  since one query for both would sort.
  A page after a cursor (`after=<id>`, AP LogCursor) walks the same index
  forward from the cursor, `id > ? ORDER BY id ASC`,
  since every index carries the rowid after its columns:
  the first rows after it, oldest first, without a sort or a new index.
  A page's walks are statements of their own,
  between which the writer can commit:
  a server's row committed after the Cube's walk would come in the server's,
  and a Cube's row below it would stay behind the page's last id for good.
  So every walk of a page forward, or of more than one walk,
  stops at the newest id read before them,
  once the page has written what is pending (`Events`):
  `+id <= ?`, a filter that leaves each walk its index.
  The ids are given out in the order of the commits,
  so a row at or below that id is committed and in every walk that matches it:
  a client that pages on from the last id misses none.
  `log.id` has no `AUTOINCREMENT`:
  SQLite gives a new row the highest id plus one,
  so a log the prune emptied would give its ids out again.
  The prune therefore keeps the log's newest row, whatever its age:
  the highest id never falls, and no id is given out twice;
  the row kept goes at a later prune, once a newer one is written.
  That is AP LogEdges' D5-A,
  reversing AP LogCursor's D10-A, which left the reuse documented.
  A store restored from an older copy still gives ids out again:
  a cursor beyond `max(id)` is answered `409`,
  one that such a reuse has already passed goes unnoticed.
  Section 11 keeps `AUTOINCREMENT` for a later migration.
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
  Only a `restart`, `reboot` or `update` its helper did not start is at `error`,
  and the UI offers no such choice,
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
  Each chunk walks `log_cube_time` and leaves out the log's newest row (above),
  `id < (SELECT max(id) FROM log)`, which SQLite reads at the table's end:
  its last chunk, which finds nothing, reads no row but the one it keeps
  (`TestEventsPrunePlan`).
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
    The same walk takes 11 s on the Pi 1 (a page of errors,
    of which it has none), 5 to 16 s from one page to the next.
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
- `backup_data.data` holds the file itself, gzip-compressed,
  a row of its own beside its backup's since migration 0024 (`backups.body` before),
  so that a list of backups does not walk the files' overflow pages;
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
- `backup_pending` (migration 0026) is the server's watch of a Cube whose
  configuration differs from its reference, kept so that the automatic backup's
  windows hold across a start (section 9 (e)):
  a row while the Cube is dirty, none while it is clean.
  It is a table of its own, not columns of `cubes`,
  so that a Cube row's `version` and `updated_at` keep meaning a change of its
  configuration in the store.
  A row has no `uuid`, `version`, `deleted_at` or `origin`:
  the watch is this server's, is not replicated,
  and is deleted rather than marked.
- `server_backup_keys`, `server_backup_targets`, `server_backups` and
  `server_backup_copies` (migration 0103) are the catalogue of the server's own
  backups ("Server backups", section 7).
  They are this server's: not replicated, without `version`,
  `deleted_at` or `origin`, deleted rather than marked,
  and never replaced by a restore.
  A target's `secret` is sealed as a provider's client secret is (section 6,
  "Sign-in through OpenID Connect"), its `uuid` the additional data.
  A key made by the server is kept pending with a hash of its identity's last six
  characters, never the identity, and goes after an hour unconfirmed.
- `auth_failures` is the `authLimiter` of #7 keyed by address,
  plus a key per user name so that password guessing from many addresses behind
  a reverse proxy locks the name rather than the proxy.
  The limits (ten in a minute, five minutes) stay.
  Rows outside the window and the lockout are pruned on every write to the table,
  which keeps it bounded as `maxTracked` does today.
- Identifiers: the rowid stays the internal key,
  and every row that will be replicated or exported (`users`, `tokens`, `cubes`,
  `preferences`, `log`, `audit`, `backups`, `commands`, and since migration 0024
  `roles` and `user_identities`) also carries `uuid TEXT NOT NULL UNIQUE`,
  a UUIDv7 (decision 7, taken 2026-11-06; docs/server.md says why).
  The owned tables (`users`, `tokens`, `cubes`, `settings`, `preferences`,
  `backups`, and since 0024 `user_identities`) also carry `version INTEGER`,
  `deleted_at TEXT` and `origin TEXT` (decision 14, taken),
  `deleted_at` every table's tombstone (D7: `tokens.revoked_at` until 0024),
  and the first migration seeds `settings` with `node.id` and `tenant.id`.
  The listing above leaves those columns out so that it stays readable;
  the migration file has them,
  and so does `internal/store/migrations/store.sql`.
- `rooms` (migration 0017) are each Cube's rooms as the server knows them,
  kept in step with the Cube's metadata:
  a room it names gets a row and a uid, one it leaves out is marked deleted,
  and a room that takes a deleted room's number later gets a row of its own.
  The uid is the room's `room_uid` in the API,
  which a client keeps for the room's life;
  the other columns are the settings of the room's virtual wall thermostat
  (docs/api-v1.md, "The virtual wall thermostat").
  A row has `version`, `deleted_at` and `origin`, as the owned tables do.
- `cubes.proxy_virtual` (docs/proxy.md) was a table of its own,
  `cube_options` (migration 0018), so that a binary built for an earlier version
  read `cubes` as before; migration 0024 made it a column of the Cube's row,
  which a change of it now versions.
- A removed row keeps its unique name, which is unique among the rows not
  removed only (migration 0024, D10-A and D11-A): `users.name`, `roles.key`,
  `auth_providers.key`, `rooms (cube_id, room_id)` and `cubes.serial`.
  A Cube whose serial has removed rows is added as a row of its own, or one of
  them is restored by its uuid, with its log, backups, rooms, commands and audit
  (`AddCube`, `RemovedCubes`).
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
  made at its first use and kept sealed,
  as the providers' client secrets are (section 6,
  "Sign-in through OpenID Connect"; AP SealedSecrets v1.0):
  its value is the JSON string of the box,
  the setting's key its additional data,
  so a `store backup` carries it but opens it only beside its `opaque.key`;
  a plain one, from before, is sealed at the next open.
  One that no longer opens is made anew, and the browsers subscribe again
  for the new key at their next visit.
  `GET /api/v1/settings` does not show it.
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
The numbers start at 1 and rise, each the one before plus 1
or the next multiple of 100 above it ("Two lines of releases", below).
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
else for the Cube's own C line the Cube's; the source from the event,
else `system` for a restore's,
`cube` for the Cube's own C line and for a row of no device,
`device` for the rest; the level `warn` for `disconnected`, `debug` for `hello`,
`info` for the rest.
It keeps `min_reader` 1:
a binary of version 8 reads its Cube's rows by `cube_id = ?`,
never meeting a NULL one, and writes rows without the new columns,
which take their defaults;
the writer of a binary of version 9 to 23 filled those (`source` empty) at its start,
and 0024 fills them once, the binaries since knowing no such rows.
`0011_events_warnings.sql` (AP Logging v1.2, Block 8) adds `events_cube_warn`,
an index of the log's warnings and errors alone (section 5, `events`),
additively: a binary of before writes and reads as it did,
and SQLite keeps the index for its rows too.
`0014_log.sql` (AP OneLog v1.1,
Block 4) makes `commands` again with `AUTOINCREMENT`,
for the reason 0013 gave the backups, and `events` again as `log`,
with `command_id` and `visibility`; it declares `-- foreign_keys: off`,
so the runner applies it with the foreign keys off, set outside its transaction,
since `DROP TABLE` with them on deletes the rows first,
which would set every `audit.command_id` to NULL and delete the steps;
it runs `PRAGMA foreign_key_check` before the commit.
It keeps `min_reader` 1 by a view:
`events` shows the log's columns as the binaries before it knew them,
and its `INSTEAD OF` triggers turn their insert,
update and delete into the log's.
SQLite counts no rows changed through a view,
so the prune of such a binary removes one chunk per Cube a pass;
the next pass goes on.
0024, which raises `min_reader`, dropped the view.
`0013_backups_autoincrement.sql` (AP BackupIds v1.0) makes `backups` again
with `AUTOINCREMENT`, as 0005 made it:
without it SQLite gives a new row the highest id plus one,
so a prune that removed the newest backup handed its id to the next.
The copy starts `sqlite_sequence` at the highest id left,
and the rename carries it to the new table;
an id pruned before the upgrade can still be given out once more.
It keeps `min_reader` 1: the columns and their meaning stay,
and a binary of before chooses a new id by the table's schema as well.
`0023_down_parts.sql` (AP StoreRaise v1.2, Block 1,
D1-D) adds `schema_version.down_sql`, additively: the down part of each version,
which the binary writes into its row at every start where the row has none (the
"down parts" rule below).
`0024_raise.sql` (AP StoreRaise v1.2,
Block 3) is the first migration that raises `min_reader`, to 24:
no binary built for 23 or earlier opens a file it migrated,
and each refuses it naming `massimo store downgrade -to <its version>`,
which the newer binary runs.
It drops what was kept for older binaries alone (the view `events`,
the fill of rows without a source, `cube_options`,
now `cubes.proxy_virtual`) and tidies the schema:
the backups' files in `backup_data`;
`effective_user_id` and `real_user_id` in `log`,
`audit` and `user_identities` (D2-A;
`real_user_id` filled for every row with a user,
the audit rows of a call through an integration with its user's own);
an integration's users named `<uuid>:`,
`<uuid>:system` and `<uuid>:<subject>` under `users.integration_id`,
and `integrations` without `key`, `user_id` and `system_user_id` (D3-A, D8-C);
the columns of a replicated row on `user_identities`,
whose deletes are tombstones now, and a uuid on `roles` (D6-B);
`tokens.revoked_at` as `deleted_at` (D7);
a removed row's name unique among the live ones only,
the users' `-del-…` suffixes taken off (D10-A, D11-A);
and the rest of D9-A (`commands.deleted_at` and `commands_cube_time` gone,
`auto_domains` a JSON array, `request_expires_at`, a CHECK on `last_level`).
It runs with the foreign keys off and makes a table again as `<name>_new`,
copied, dropped and renamed; `log` is altered, never made again.
Its down part, `0024_raise.down.sql`,
makes the file of version 23 again to the byte of `sqlite_master`,
every row a binary for 23 wrote as it was; its header names what it loses,
L1 to L8: who made a row where an integration acted for a linked user (L1),
the removed identities (L2),
the integration users' and the removed users' names,
made again by the rules of 23 (L3, L5), an integration's key,
made again from its name (L4), the role's uuid and a second row of one key (L6),
the time and the user of a Cube's option (L7),
and the never written `deleted_at` of tokens and commands (L8).
Several rows of one serial (a Cube added again after its removal,
D15-C) are all kept: the one not removed keeps the serial,
else the newest removed, and each other, removed,
is `<serial>~<its id>` in `cubes.serial` alone, with its log, backups, rooms,
commands and audit rows;
`log.serial` and the serials in the rows' JSON stay as written.
A binary for 23 reads only the Cubes not removed, and every read of the log
is by `cube_id`, so their lines do not meet;
0024 takes the `~<id>` off again.
`0025_token_parent.sql` (AP MobileApp v1.1, Block 8, D31-A) adds
`tokens.parent_id` and its index, additively, keeping `min_reader` 24:
a session made from a key names it (section 6, "A session from a key").
A binary for 24 writes its tokens without the column and makes no session from a key;
one that revokes a key leaves the key's session live in its row,
which a binary since 0025 refuses, a session counting only while its key does.
The column has no foreign key, so that its down part,
`0025_token_parent.down.sql`, can drop it and the index;
it loses which key made a session, and only that.
`0026_backup_pending.sql` adds `backup_pending` (AP BackupWatch v1.0, Block 1),
additively, keeping `min_reader` 24:
a dirty Cube's windows of its automatic backup,
kept across a start (section 9 (e)).
A binary for 24 or 25 leaves the table alone and keeps its watch in memory,
as before;
a row it leaves behind, its backup taken without deleting it,
names another reference in `ref`, and this binary's next start deletes it.
Its down part, `0026_backup_pending.down.sql`, drops the table;
it loses the windows of a pending backup, and only that,
which the binary of 25 begins again at its first look.
`0100_token_revoke.sql`, the first of `main`'s block after the freeze of 0.17
("Two lines of releases"), adds `tokens.revoke_hash` and its unique index
(AP MobileApp v1.6, Block 21, D112-A), additively, keeping `min_reader` 24:
the hash of a token's revoke code (section 6, "A token's revoke code").
A binary for 24 to 26 writes its tokens without the column,
which this binary fills at a token's next use.
Its down part, `0100_token_revoke.down.sql`, drops the column and the index;
it loses the tokens' revoke codes, and only that.
`0101_token_maker.sql`, the second of `main`'s block,
after 0100 (AP KeyLife v1.0, Block 2, D5-A),
adds `tokens.made_by` and its index, additively, keeping `min_reader` 24:
the API key that made a key,
where the bearer of its making was that key (section 6, "Keys made by keys").
A binary for 24 to 100 writes its tokens without the column,
so a key it makes has no maker, and its revoke of a key,
by its id or by its code, leaves the keys that key made,
which a revoke by this binary of the key's own maker still reaches,
the makers being followed through a revoked key.
It keeps `min_reader` by the rules below:
it adds a nullable column and an index, and changes the meaning of no column.
What such a binary does not do, revoke the heirs and refuse a session whose key
is past its end, is its code's, not the file's:
`expires_at` and `parent_id` mean what they meant,
and a binary that runs reads them its own way,
as one for 24 left the sessions of a key it revoked (0025).
A raise would make every rollback to the 0.17 line, or to a binary of 100,
a downgrade of the file, and would protect nothing a file can carry:
the heirs a rollback's revoke leaves are reached again by this binary's
revoke of their maker's maker,
and no key has an end before AP KeyLife's Block 4.
The column has no foreign key, so that its down part,
`0101_token_maker.down.sql`, can drop it and the index,
taking the file back to 100;
it loses which key made a key, and only that.
`0102_token_idle.sql`, the third of `main`'s block, after 0101 (AP KeyLife v1.0,
Block 3, D7-A), adds `tokens.idle_exempt`, a column defaulted to 0, additively,
keeping `min_reader` 24:
an API key kept from the idle limit of the setting `auth.key_idle_days` (section 6,
"The life of a key").
A binary for 24 to 101 writes its tokens without the column,
so a key it makes is held to the limit,
and knows no idle limit at all, so it lets in a key this binary ends unused.
A raise of `min_reader` would not keep such a key out:
an end is a state read at each check and nothing is written (D6-A),
so the older binary that a downgrade serves would let it in all the same.
Its down part, `0102_token_idle.down.sql`, drops the column,
taking the file back to 101;
it loses which keys were kept from the limit, and only that,
and a binary of 102 or later that opens the file again holds every key to it.
AP KeyLife's Block 4, an API key's end date, adds no migration:
the end date is `tokens.expires_at`, a column of 0001 that every binary reads
for the token itself,
so a binary for 24 to 102 refuses a key past its end date too;
one before Block 2 lets in that key's session, as the paragraph of 0101 says,
which is its code's and not the file's, and `min_reader` stays 24.
Nor does Block 5, a key's new secret:
it writes the key's `tokens.hash` and a revoked row of the old hash,
columns of 0001 that every binary reads as this one does,
and moves `revoke_hash` (0100) with them,
so a binary for 24 to 102 lets the new secret in
and refuses the old one as a revoked token, and `min_reader` stays 24.
`0103_server_backups.sql`, the fourth of `main`'s block,
after 0102 (AP ServerBackup v1.0, Block 2),
adds the four tables of the server's backups and seeds `backup.retry` for `manager`,
additively, keeping `min_reader` 24:
a binary for 24 to 102 leaves the tables alone and ignores the capability.
Its down part, `0103_server_backups.down.sql`, drops the tables
and removes `backup.server` and `backup.retry` from every role,
taking the file back to 102;
it loses the catalogue, and only that: the files stay on their targets,
and a target added again is scanned for them.
On a store of 500 000 log rows (149 MB,
a tenth with a user) a desktop took 2.0 s for the raise,
its copy `massimo.db.before-24` included, and 2.2 s for the down part,
its copy included (`TestRaiseTimes` with `MASSIMO_STORE_RAISE_ROWS`;
not measured on a Raspberry Pi yet).

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
  it never downgrades a file by itself.
- The down parts: a migration whose `min_reader` is above 1,
  the one that raised it and every one after it (an additive one too,
  since it was made on the raised schema), comes with `NNNN_<name>.down.sql`,
  plain SQL that undoes it;
  `TestRaisingMigrationsHaveDownParts` fails without one.
  The binary embeds them and writes each into `schema_version.down_sql` of its version,
  so that the file carries them for a binary that knows the version by its row alone.
- `massimo store downgrade -to <N>` takes the file back to what a binary for
  schema version N opens: it undoes, newest first,
  the migrations from the first whose `min_reader` is above N up to the file's version,
  in one transaction with the foreign keys off and checked before the commit,
  and deletes their `schema_version` rows;
  the additive migrations before that first one stay,
  since N's binary reads them, so the file can end above N.
  The SQL of each comes from the binary that runs it, else from the file.
  It needs the file alone: it opens it with `locking_mode=EXCLUSIVE`,
  so a running `serve`, which holds a shared lock, makes it fail
  ("stop the service first"), and nobody opens the file while it runs.
  It copies the file first to `massimo.db.before-downgrade-<version>` (`-2`,
  `-3` where an earlier downgrade left that name),
  owned by the file's owner where root runs it.
  A version without a down part in either stops it before anything changes,
  naming the `massimo.db.before-<version>` copy as the way back.
  It loses what the newer schema added, and only that.
- The rollback of `massimo update` (after a failed health check,
  and `-rollback`) asks the binary it put back whether it opens the file (its
  `store info`, which names its schema version N also in its refusal);
  where it does not, the rollback stops the service,
  runs the binary it set aside (`.failed`, after `-rollback` `.prev`,
  the newer one) as `store downgrade -to <N>`, else,
  after a failed health check, the binary put back,
  which is the one that ran the update and downgrades by the file's own down parts,
  and says what it did in the update's status (packaging/README.md,
  "The store").
  The update runs as root and opens no store itself:
  where root runs them, both binaries run as the owner of `massimo.db`.
  apt, brew and the Windows installer never downgrade a binary.
- The store refuses a file it did not create (no `schema_version`
  table) rather than migrating a stranger's database.
- Tests: migrating an empty file to the latest version,
  migrating a file at every earlier version,
  opening a newer additive version with an older reader,
  and a test-only raise undone by its down parts,
  from the binary and from the file,
  to the schema of a fresh file (`downgrade_test.go`).
  For 0024 (`raise_test.go`, D14-A): each release's dump,
  migrated to the newest version and downgraded for its own release,
  equals the dump migrated only as far as that downgrade stops,
  in `sqlite_master` and in every row but for the columns of the losses (`losses0024`,
  which names them by L1 to L8);
  a store of version 23 with a row of every kind 0024 changes goes up and down again so,
  and the losses themselves are checked where the raised schema holds what 23 cannot;
  two rows of one serial go down and up again with all their rows,
  the log byte for byte.
  The comparison takes `sqlite_sequence` as a table like any other, so the
  AUTOINCREMENT counters of `backups` and `commands` are compared too.
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
  every event has its source, and the log, the users and the backups read;
  its schema is a fresh store's, compared by SQLite's pragmas
  (each table's columns with type, `NOT NULL`, default and key,
  its foreign keys and its indexes), not by the text of the statements;
  and no column holds fewer values that are not NULL than before,
  but for the columns a migration moves, drops or empties on purpose,
  which the test lists with the migration and the reason (`columnFates`);
  `TestReleasedStoresDownAgain` takes each down again (above).
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

#### Two lines of releases

A release may come from a fix line, `maint/<x.y>`, while `main` goes on,
so the numbers come in blocks of 100.

- The fix line numbers within the current block (0027, 0028, …).
  `main` starts the next hundred at the freeze that cuts the fix line (0100).
- `loadMigrations` takes each number as the one before plus 1,
  or the next multiple of 100 above it: 0026 to 0100 is a new block,
  0026 to 0050 or to 0200 is refused, and so is a number twice.
  The error names the file.
- The store never applies a number below its own version.
  A store of `main`'s block skips the fix line's files,
  which `main` holds since the fix line merges upward.
- A change of the fix line that `main` needs as well gets a copy at the top
  of `main`'s block, written for `main`'s schema and guarded,
  so that it does nothing where the change is in place:
  on a store that came from the fix line,
  and on a fresh store, which runs every file in order
  and so takes the fix line's migration first.
- A fix line's migration is additive and guarded:
  a new table, column or index,
  `IF NOT EXISTS`, or a check of `pragma_table_info` before a column is added.
  It makes no table again.
- The risk that stays: a migration of the newer line that makes a table again
  copies the columns it knows.
  One written before the fix existed drops the fix's column, with its values,
  on a store that came from the fix line,
  and the guarded copy then adds the column back empty.
  `TestMigratesReleasedStores` catches that before the newer line ships,
  since every later binary migrates every released store's dump:
  the column then holds fewer values than before,
  where the fix line's dump holds a value in it
  (`tools/store-fixture/make.sh` sends every release the same requests,
  so a fix's column needs one of its own there).
  The fix is a copy of that migration that carries the column over.
- A downgrade undoes the versions the file's `schema_version` holds,
  not every number between them.

## 6. Users, roles and capabilities

A capability model, decided by the owner on 2026-11-06.
Every route is guarded by a **capability**; a **role** is a set of capabilities;
a user has one role.
The default roles and their sets are seeded at the first start and can be
changed afterwards by whoever holds the `roles` capability, with one exception:
`owner` always has every capability and cannot be edited, disabled or deleted,
and the store refuses to disable, delete, demote or give an end date to the
last enabled owner user without one,
an integration's user with the role `owner` not counting.
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

The capabilities, what each allows and the routes it guards
are the table of docs/api-v1.md ("Authentication"),
held to the routes the server registers by a test
(`TestCapabilityTableCoversRoutes` in `internal/server/openapi_test.go`).
`integrations` came with migration 0021, `repair` with 0022,
`backup.server` and `backup.retry` with 0103.

<!-- BEGIN GENERATED PARTIAL docs/fragments/_capability-rules.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
`update` and `reboot` each include `restart`, since each restarts the server:
their holder passes every check of `restart` as a holder of `restart` does,
and `GET /api/v1/me` lists `restart` beside them.
`backup.server` includes `backup.retry` alike: who makes,
pushes and pulls the server's backups sees their state and retries one.
The include is taken at the check, on the set the bounds leave
(the role's set less the capabilities disabled for every role,
an integration's ceiling and maximum, a call's level):
an integration whose maximum holds `update` and not `restart` restarts the server,
a call of it at `control`, which leaves no `update`, does not,
and `restart` disabled for every role is disabled for their holders too.
Nothing stored changes, and no migration runs:
`GET /api/v1/roles` shows each role's set as saved,
and the rule below compares the stored sets.
`reboot` does not include `update`, nor `update` `reboot`.

A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
A holder of `integrations` changes an integration's user whatever its role,
but one stored as `owner`, which stays an owner's alone,
and gives it only a role it outranks
that lies within the integration's maximum (AP AccessEnds, D47-A, D48-A):
so the manager, which does not outrank `integration`,
moves such a user off it, and an owner gives it back.
<!-- END GENERATED PARTIAL docs/fragments/_capability-rules.md -->

The rule holds for `users`, `guests` and `passwords` alike,
and for `integrations` in the role it gives (docs/api-v1.md, "Authentication").
Both includes were decided on 2026-11-25 (AP StartConfiguration, D37),
and on the same day that `reboot` does not include `update` (D40-A).

The default roles, as seeded.
`owner` has every capability.

| Capability | `manager` | `operator` | `guest` | `viewer` | `integration` |
|---|---|---|---|---|---|
| `read` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `preferences` | ✓ | ✓ | | ✓ | ✓ |
| `setpoint` | ✓ | ✓ | ✓ | | ✓ |
| `lock` | ✓ | ✓ | | | ✓ |
| `configure` | ✓ | ✓ | ✓ | | ✓ |
| `rename` | ✓ | | | | ✓ |
| `structure` | ✓ | | | | ✓ |
| `pair` | ✓ | | | | ✓ |
| `discover` | ✓ | ✓ | | | ✓ |
| `backup` | ✓ | ✓ | | | ✓ |
| `restore` | ✓ | | | | ✓ |
| `repair` | ✓ | | | | ✓ |
| `guests` | ✓ | | | | ✓ |
| `passwords` | ✓ | | | | |
| `restart` | ✓ | | | | ✓ |
| `integrations` | ✓ | | | | |
| `backup.retry` | ✓ | | | | |
| `cubes`, `cubes.reboot`, `settings`, `update`, `reboot`, `audit` | | | | | ✓ |
| `users`, `roles`, `roles.disable`, `capabilities`, `auth`, `backup.server` | | | | | |

So `manager` is `operator` plus guests, pairing, names, structure, restore,
the server's restart (migration 0006, 2026-11-07),
the passwords of other users (migration 0008,
2026-11-08) and the repairs (migration 0022, 2026-11-15, AP RadioHealth,
D166-A), which the role integration gains with its ceiling,
and the state of the server's backups with the retry of a failed one (`backup.retry`,
migration 0103, 2026-11-27, AP ServerBackup, D13-A), which it does not (D15-A);
`backup.server`, which makes, pushes and pulls the server's backups,
is seeded for no role, as `auth` is: a server backup holds the people,
the sign-in and the integrations, and the owner gives it;
`reboot` is seeded for no role (2026-11-08):
restarting the server is not rebooting a computer that may run other services too;
nor is `cubes.reboot` (2026-11-14):
a Cube's restart drops the radiograms it has not delivered,
and the owner gives it to a role where that is wanted;
`operator` runs the heating, locks, configures, discovers and takes backups;
`guest` is `operator` without the lock, backups, discovery and preferences;
`viewer` reads; `integration` is fixed, as `owner` is (migration 0021,
2026-11-14, AP IntegrationUsers):
it is the role of the registered integrations and of their users,
and the ceiling of every call through an integration ("Integrations", below).
It holds every capability but those that manage people,
sign-in and the integrations themselves (`users`, `passwords`, `roles`,
`roles.disable`, `capabilities`, `auth`, `integrations`),
and the server's backups, which hold all that (`backup.server`, `backup.retry`;
AP ServerBackup, D15-A), so that an integration can never widen its own users;
each integration's maximum narrows it further.
It cannot be changed, disabled or given to a regular user.
Its users before 0021, API keys made by hand for an integration,
moved to the ordinary role `integration-v0`, "integration (before 1.0)",
with exactly the set they had (read, setpoint, lock,
and configure where migration 0012 gave it).
Moves and deletions (`structure`) go with the names to `manager`,
because a move changes the house as a name does; if `operator` should keep them,
that is one row of the seed.
`preferences` keeps one's settings and colour schemes on the server (stage (c));
a guest has it not, so a guest's settings stay on its devices,
where the defaults apply anyway (decided 2026-11-06),
and an integration's users keep theirs if it ever shows them a page.

Guests are users with the role `guest`,
created by a holder of `guests` with an invitation (a one-time code or link;
the guest sets a password) and an optional end date,
after which the user is disabled:
the time-limited share of design.md section 6 in its simplest form,
with `users.expires_at` and `users.invited_by` in the schema.
A holder of `guests` may only create,
change and remove users whose role is `guest`; everything else needs `users`.
It reaches regular guests alone, and a call through an integration,
which holds `guests` where its maximum and its chain do,
that integration's own users stored as `guest` alone, and creates none;
an integration's users are changed by holders of `users`, by the rank rule,
and of `integrations`, every one but one stored as `owner` (AP AccessEnds,
D41-A4 and A5; docs/api-v1.md, "Users").

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
and the table of docs/api-v1.md names the route in both rows.
The list of capabilities is Go code, not a table,
because a capability is a line in the route table;
the roles and their sets are rows (section 5), seeded by the first migration.

### Bootstrap and compatibility

- A regular user is a user that is not removed and is not an integration's
  (`users.integration_id IS NULL`), a disabled one included;
  "users" below means regular users (AP LooseEnds, D1).
  A server whose only users are an integration's is open,
  as one without users, and warns so at its start
  (`the server is open: its only users are an integration's; create the owner`)
  unless it is kept open on purpose;
  a version before counted them, so such a server was closed and becomes open
  at the upgrade (D2).
- With `MASSIMO_API_TOKEN` set and no user in the store (the first start after the update,
  or a store with an integration's users alone),
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
  and one that was removed from the file is revoked,
  with the keys it made ("Keys made by keys", below; AP KeyLife, D8-A).
  The file stays the way to recover an owner who lost the password:
  put a token into the file, log in with it, set a password.
  The API does not revoke the file's key,
  nor remove the user that holds it (`409`; docs/api-v1.md, "Users:
  /api/v1/users and /api/v1/users/{name}" and "Tokens:
  /api/v1/users/{name}/tokens"): a revoked row keeps its hash, unique,
  so the next start could not add it back.
  The file's key that an older start gave to an integration's user with the role `owner`,
  live or revoked since (a revoke of the integration revokes it), moves,
  at the start, to the first enabled regular owner,
  or to the user `owner` made where there is none, in one transaction with it,
  live again and its sessions ended,
  and the log says from whom (`the settings file's key moved from an integration's user`;
  AP LooseEnds, D1): such a user holds no key of its own.
  Whoever can edit the file is root or the service user,
  who can edit the database anyway.
- Without `MASSIMO_API_TOKEN` and without users,
  the API stays open as today (acceptable on the default localhost listener,
  docs/api-v1.md says so already),
  and Settings shows "No users yet" with a form that creates the first owner
  with a password; the form works only while there are no users,
  and makes one owner:
  the store counts the users again in the transaction that writes it,
  which holds the database's one write lock from its start (`_txlock=immediate`),
  so of two at the same moment the second is refused (AP FirstOwner, D15).
  `massimo user add` does the same from the command line (open decision 4 on
  whether the form should exist).
  The first regular user, made by the form,
  by `massimo user add` whatever its role, or from `MASSIMO_API_TOKEN`,
  is told once what was set up while there was none,
  or left by an owner since removed, and still works (`OpenSetup`,
  AP AccessEnds, Block 3): the integrations active with a key,
  or one still to be collected with the approval's code,
  the sign-in providers enabled,
  the roles added or changed (one that is not a default role,
  or a default one past its first version;
  never owner or integration) and the capabilities turned off;
  a request still waiting is not listed (AP AccessEnds, D15).
  It is the log's `open_setup` line at warn,
  a log alert until a person marks it seen,
  the form's toast and the command's second line (docs/api-v1.md,
  "Authentication").
  The setting `auth.keep_open` (false by default) keeps such a server open on purpose:
  the web UI does not ask for the first owner (AP FirstOwner, D11),
  and nothing changes about who is let in.
  It counts only while there are no users:
  every transaction of the store that makes a user removes it,
  the API's first owner, `massimo user add`, the owner of `MASSIMO_API_TOKEN`
  and a provider's user among them;
  an integration approved and its users leave it as it is,
  and the server removes it at its start and at each reload of the settings
  file where it finds users beside it,
  which a process of an older version can leave;
  each removal is an Info line, `keep-open setting removed`.
  A server that loses all its users asks for the first owner again.
  One window is left: where a process of an older version adds a user while the
  setting is on,
  and every user is removed again before the next start or reload,
  the setting is in effect again without anybody having been asked;
  only a start or a reload closes it.
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

### A session from a key

The apps' login by fingerprint (docs/app.md) keeps an API key of the user's
own on the phone, made by `POST /api/v1/users/{name}/tokens`,
and `POST /api/v1/login/key` turns it into a session (docs/api-v1.md).
A session is extended at each use,
so without a tie a revoked key's session would live as long as the phone used it:
the session names its key in `tokens.parent_id` (migration 0025),
empty for a session of a sign-in and for a key.
`RevokeToken` of a key revokes its live sessions with it,
and what they subscribed to push;
the key's next session revokes its earlier one, in the same transaction,
so a key has one live session at a time;
and a session counts only while its key does,
neither revoked, past its end nor ended unused (AP KeyLife, Blocks 2 and 3),
which also ends one whose key a binary before 0025 revoked.
That is one rule, `alive` in `internal/store/tokens.go`,
of a token not revoked, not past its own end, not an API key ended unused
("The life of a key", below), its user not removed,
and of a session made from a key only while the key is neither revoked,
past its end nor ended unused,
wherever a token is let in, counted or sent to:
`TokenByHash` and `TokenAlive` give a token under it,
and `TokenAliveAs` too while it holds the secret it was let in with,
`TokenEnded` says a token the store issued is not,
`Tokens` lists the sessions under it and the keys while not revoked,
a key past its end staying listed, to be revoked,
`PushSubscriptions` sends to a session's devices under it,
and `AddKeySession` and `AddKeySessionAs` make a session from a key under it alone,
the latter also only while the key holds the secret it was let in with.
A password set by someone else ends the sessions,
those made from keys among them, and leaves the keys.

### Keys made by keys

Any key of a user makes keys, for its user and for those its user manages,
so a key that leaked could make more keys,
which its revoke would leave (AP KeyLife v1.0, Block 2, D5-A).
A key made by `POST /api/v1/users/{name}/tokens` whose bearer is an API key
names that key in `tokens.made_by` (migration 0101), its heir;
a key made by a session,
of a sign-in or made from a key by `POST /api/v1/login/key`,
by `massimo user token`,
by the bootstrap or at an integration's collection of its key names none.
A key the bootstrap takes in as the settings file's has no maker either:
the bootstrap clears `made_by` of the file's key it finds,
before it revokes the file's old key, which may have made it.
A key is made with a maker only while the maker is alive,
the check and the insert one statement (`AddKey`, else `ErrNoToken`, a `401`):
a request whose bearer was let in before its revoke,
and makes its key after it, makes none,
so that no live key names a revoked maker that no later revoke reaches;
nor does one let in with a secret the maker no longer holds (`KeyOptions.Maker`,
its hash checked in the same statement; Block 5).
`RevokeToken` revokes, in its transaction, the heirs of the token it revokes,
and so does `RevokeByCode`,
a revoke by the token's code ("A token's revoke code", below), in its own,
followed through `made_by` from key to key in any state,
so that a key past its end, or one an older binary revoked alone,
passes the revoke on to the keys it made,
with their sessions and what those subscribed to push,
and returns the heirs it revoked, by id;
`DeleteUser` revokes the heirs of the user's tokens with them,
and the bootstrap's revoke of a key that is no longer the settings file's takes
them too (D8-A), its log line saying how many;
a key of the file's label that a revoke before it in the same bootstrap took is
passed over, and the file's new token is added all the same.
The settings file's key is never an heir:
the store keeps in memory the hash `Bootstrap` last applied,
at the start and at each reading of the file,
and a revoke's heirs pass over that key and the keys it made,
whoever made it, as the API revokes it neither by a revoke nor with its user
(MobileApp.D86-A, D89-A);
`massimo user`, which runs no bootstrap, knows no such hash.
A key's end, by its end date, takes none:
an heir lives past its maker's end, and only a revoke reaches it.

### A token's revoke code

A client that forgot a token without its server answering ends it by its
revoke code at the next contact (`POST /api/v1/tokens/revoke`,
docs/api-v1.md; the apps, docs/app.md).
The code is `RevokeCode(secret)`,
the hex of SHA-256 of `massimo revoke v1` followed by the token as the server gave it,
and the store keeps `HashToken` of the code in `tokens.revoke_hash` (migration 0100),
not the hash it keeps of the token: with that,
the store and every backup of it could revoke every token,
while no code can be had back from its hash.
The store writes the column where it makes the secret (`addToken`,
`AddKey`, `AddKeySession`); a token whose hash it is handed (`AddTokenHash`)
and the settings file's key (`Bootstrap`) get none.
A token made before 0100 gets it at its first use after it:
the server works it out from the secret presented and `TouchToken` writes it,
within the hour of the last use too, keeping one written;
the settings file's key is passed over there.
`RevokeByCode` revokes the token of a code as `RevokeToken` does,
in one transaction with its sessions, its heirs,
the keys a key made and theirs with their sessions ("Keys made by keys", above),
and what they all subscribed to push, a token past its end included,
and returns the heirs it revoked; a token revoked already,
an unknown code and the settings file's key match nothing,
the store knowing that key by the hash `Bootstrap` last applied,
as the heirs of a revoke do,
so that a key made by `POST …/tokens` and then named in the file keeps its code
but is not ended by it while the file names it;
the code is looked up by a plain read first,
so that one that matches nothing takes no write transaction,
and a match is read again inside it.

### The life of a key

An API key unused for the days of the setting `auth.key_idle_days` ends (AP KeyLife v1.0,
Block 3; AP MobileApp, D75): whole days, 0 or unset for off, the default (D2-A),
at most 3650, read at each check, so that a change applies at once.
The idle clock is the key's last use, or its making where it was never used:
`coalesce(last_used_at, created_at)` at or before the time of the check less
the limit has ended.
A session's use is its key's: where the server writes a session's last use,
at most once an hour, it writes the key's too, with that use's address,
so a phone that uses its session keeps its key.
The idle part joins the rule `alive` ("A session from a key", above),
for the token itself and for the key a session was made from,
so an ended key's session ends with it, and its heirs live on:
an end is no revoke,
and a revoke still reaches the key, by its id or by its code
(`RevokeByCode` matches a code on `deleted_at` alone, D6-A),
its heirs with it.
Two keys never end by it:
one kept from it, `tokens.idle_exempt` (migration 0102),
which only who manages the key's user sets (D7-A)
and an integration's key never has (MobileApp.D75);
and the settings file's key (D3-A), known by its hash,
which the store keeps in memory as `Bootstrap` last applied it,
at the start and at each reading of the file,
not by a mark on its row: the file may change.
An end is a state read at each check, and nothing is written (D6-A):
an ended key stays listed, to be revoked, with why it ended,
and lives again, its session with it while the session's own 30 days last,
where the limit is raised or turned off, or the key kept from it.
Its use never brings it back,
since the rule refuses it before its use is written,
and writes no hash of its revoke code either ("A token's revoke code", above):
a key made before 0100 that ended unused before its first use since keeps none,
and its code matches nothing.
`Tokens` gives each key its life by the same rule in Go (`IdleEndsAt`, `EndsAt`,
the earlier of its end date and its idle end, and `Ended`, `idle` or `expired`),
which `TestKeyEndRule` holds to the SQL on one table of cases;
`SetTokenLife` changes a key's exclusion.

A key may also take an end date (AP KeyLife v1.0, Block 4; AP MobileApp, D75):
`tokens.expires_at`,
which the rule `alive` always read for the token itself and,
since Block 2, for the key a session was made from,
`NULL` for none, as every key had before.
`AddKey` makes a key with one (`KeyOptions.Expires`) and `SetTokenLife` sets,
moves or removes it (`TokenLife.Expires`, the zero time for none);
a date not after the store's clock, as the rule compares the two,
is `ErrEndPassed`, and nothing is made or changed:
a key is ended now by its revoke.
At its date the key ends, its session with it,
and is listed with `Ended` `expired`;
its heirs live on, an end being no revoke (D5-A),
and a revoke still reaches the key, by its id or by its code, its heirs with it
(`RevokeByCode` on `deleted_at` alone, as for an idle end).
The end is a state read at each check, as the idle end is (D6-A):
the date moved later, or removed, brings the key back, its session with it.
Who may move it is the server's question:
the key's user or who manages it, never the key itself as the bearer (D4-A).
The settings file's key never ends by a date (D3-A):
the bootstrap clears the end date of the key it takes in as the file's,
a key made with one and then put into the file among them,
and the end-date part of `alive`, as its idle part, passes over the file's hash,
for the key and for a session made from it,
so that an end date left in its row, by a binary before that, ends nothing;
`SetTokenLife` refuses that key (`ErrSettingsKey`),
and the server finds it by `KeyByHash`, its hash among the keys not revoked,
whatever else of its life, so that it refuses every change of it.

A live key may take a new secret (AP KeyLife v1.0, Block 5; MobileApp.D76-A),
the way back to a key whose secret is lost,
since the store keeps its hash alone (D81-A),
and the way to refuse whatever holds the key's secret now.
Its heirs, the keys made with it, stay:
for a secret that may have leaked, a revoke ends a key and the keys it made.
`RekeyToken` replaces the key's hash in one transaction,
and the hash of its revoke code with it, the new secret's,
so the key keeps its row:
its id, label, making, last use, end date, exclusion, maker and heirs,
and the new secret's code ends it and its heirs ("A token's revoke code").
The old hash is kept in a revoked row of its own,
an API key of the key's user and label, made and revoked at that moment,
with no maker:
the hash is unique, and a revoked row keeps its hash,
so a client still sending the old secret is a token that ended (`TokenEnded`),
not a guess that counts toward the lockout (MobileApp.D64-A);
a hash replaced alone would make it one.
The old secret's revoke code, none for a key made before 0100,
moves onto that row, after the key's row took the new one,
the column being unique:
the old secret's code then matches a revoked row alone and ends nothing,
as a code taken already does, and the key lives on.
The key's session (a key has one at a time, `AddKeySession`) is revoked
with what it subscribed to push, unless it is the caller's own (`keep`),
and what the key itself subscribed goes too:
whoever held the old secret holds no session and no subscription.
What a request let in with the old secret does next is held to that secret,
since the key keeps its id:
`AddKeySessionAs` and a key made with `KeyOptions.Maker` take the key only
while its hash is still the one the request was let in with (`ErrNoToken`),
and `TokenAliveAs`, which the event stream asks at each keep-alive
and at a ticket's redemption,
no longer finds it, so a stream opened with the old secret ends,
and a ticket taken with it opens none.
A key that is not live, by its revoke, its end date or the idle limit,
takes none (`ErrNoToken`),
and an integration's key none (`ErrIntegrationUser`):
the integration learns its key by its approval's code alone.
The settings file's key, known by the hash `Bootstrap` last applied,
takes none (`ErrSettingsKey`, as for `SetTokenLife`; the API's `409`):
the next start would not add the file's key back,
its hash being taken by the old secret's row.
Who may is the server's question, as for the end date.

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

`massimo user add <name> -role owner|operator|viewer|integration` (asks for the
password twice, or `-no-password` for a key-only user), `user list`,
`user passwd <name>`,
`user rename <name> <new name>` (asks for the password again,
or `-no-password`), `user disable <name>`, `user enable <name>`,
`user token <name> -label <text>` (prints a new API key once),
`user tokens <name>` (the sessions and keys with their ids),
`user revoke <name> [<token id>]`.
A new password has 8 to 64 characters and contains no name of the user's or the box's,
nor `massimo` or `maxcube` (`internal/password`);
one set before stays valid, but a rename asks for one again.
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
A `user add` that makes the store's first regular user prints a second line,
what was set up before it and still works, where anything was,
and writes the log's `open_setup` line by the command line (above);
whether it is the first is counted in the transaction that writes it,
so that of it and a first owner made at the same moment one alone is told.
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
A removed user's row keeps its name, which is unique among the users not
removed only (migration 0024, D10-A),
so that the name is free and a user added under it is a row of its own.
From migration 0015 to 0024 a removed row was renamed `<name>-del-YYYYMMDD-HHNN`,
the removal's time in UTC, `-<id>` added where that was taken;
0024 took the suffixes off.
(Until 0015 a new user took the removed one's row over, with its history.)
The audit records them with `user_id` NULL and `remote` `cli`.

### The lockout

`authLimiter` keeps its behaviour, backed by `auth_failures` (section 5):
a failed token, ticket or password check counts against the address and,
for a login, against the user name;
a request that offers no token and no ticket is refused uncounted,
and so is a token the store issued that has ended:
revoked, past its end, its user removed,
or a session whose key was revoked (`TokenEnded`;
`docs/api-v1.md`, "Authentication");
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
A provider may admit only some groups,
read from its groups claim (migration 0020):
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
so is one a build of main sealed as `v1:` with a key file of its own,
`secret.key`,
which is then renamed `secret.key.retired` for the owner to delete.
Lost, or another store's, `opaque.key` leaves each secret unopened,
as each password: the provider answers `secret_lost`,
and the owner enters the secret again;
a change that does not write it keeps the box, which the key found again opens.
The secret is never answered (`has_secret`).

### Integrations

An integration, Home Assistant first,
acts for users of its own (AP IntegrationUsers, migration 0021):
a row of `integrations` per installation the owner approved,
and per request waiting or refused.
A request comes without a key and waits ten minutes with a short code the owner compares;
at most five wait at once.
Approved, the integration gets its own user `<uuid>:`,
the integration's uuid and a colon, holding its API key,
a user `<uuid>:system` for the calls no user started,
both with the role `integration` and `users.integration_id` naming it,
and its maximum, `max_caps` (migration 0024, D3-A and D8-C;
until then a `key` made from its name, `casa-leone`,
and users `<key>` and `<key>.system`).
The own user's `display_name` is the reported name at first,
and follows a name reported anew only while it still is the name reported before:
a name the owner gave stays.
The names of an integration's users follow `^<uuid>:([a-z0-9]{1,64}|system)?$`;
only the store makes them, and the rule of every other name has no colon.
An integration's user acts through its integration alone:
the store gives it no key but the integration's own (`CollectIntegrationKey`),
no session, no password of either version and no identity at a provider
(`ErrIntegrationUser`), whatever its role,
so that no route can give it a way in of its own;
what it held before stays, a key acting while its integration is active and
within the role `integration` and the integration's maximum,
a password or an identity signing in nowhere.
They are not regular users: they neither close an open server nor block its
first owner ("Bootstrap and compatibility").
A request approved as an earlier integration (`merged_into`) reuses that one's row,
users and links, as after a key lost or revoked;
a revoke ends the key and keeps the rest.
`instance_id` is the installation id the owner accepted;
a call from another is refused and the id kept in `pending_instance`
until the owner accepts it.

Each of the integration's users is a row of `user_identities` with `integration_id` set,
its id at the integration as `subject`: made at its first call,
with a user of its own (`real_user_id`, named `<uuid>:<the id, lower case>`;
`own_user_id` and `<key>.<the id's first 8 characters>` before 0024) of the role
`integration`.
`effective_user_id` is the user its calls act as: its own,
or the Massimo user the owner linked it to,
several of an integration's users to one where wanted.
Removing a user removes the identities that act as it,
an integration's user's own too, linked or not (AP AccessEnds, D54-A).
The integration's next call for that user makes its identity anew, unlinked:
with its user `<uuid>:<the id>` made anew where that was removed,
or, where the Massimo user it was linked to was removed and it is left,
with that user again, as the owner left it (AP LinkedRemoval, D2-A);
never with a user of that name of no integration or of another,
nor with one another identity has.
An identity whose own user was removed while the identity stayed,
as an older store holds one, acts for nobody:
no list shows it, no link takes it and no stream goes on for it,
and the next call removes it and makes it anew so, without a migration (D1-A).
One whose linked user was removed only a link racing that removal could leave,
which the link no longer lets through, since it reads its target
in the transaction that writes it;
it is treated so all the same.
The log and the audit write both:
whose rights a row was made with (`effective_user_id`),
and who made it (`real_user_id`).
A call's capabilities are what the user's role, the role `integration`,
the linked user's role,
the maximum and the call's level all allow (docs/api-v1.md,
"Acting for an integration's users").

0021 made `user_identities` again for both kinds,
`provider_id` NULL for an integration's: a store of a 0.15.0 alpha,
once migrated, is not for that alpha again,
although `min_reader` stays 1 (D10-C), since only the alphas had the table.
The settings: `integrations.requests` (true by default) lets integrations ask;
`server.proxied_url`, which the server writes,
is the address a trusted reverse proxy serves it at,
the announcement's `url` where `MASSIMO_PUBLIC_URL` is not set.

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
`store info` for the version and the counts,
`store downgrade -to <N>` to take it back for an older binary, section 5,
"Migrations"), which open the store directly.
`store keygen`, `store inspect` and `store decrypt`
handle the keys and the files of the server's backups,
and open no store ("Server backups", below).
`store backup` without a file makes a server backup now (`-target <name>`,
`-label <text>`), `store backups` lists them,
`store push <id> <target>` and `store pull <id> <target>` copy one,
`store restore <file|id> -scope <list> -key <file>` restores one by scope:
these go through this box's running `serve`, its API with `MASSIMO_API_TOKEN`,
so that the job is the server's, and open the store otherwise.
`serve` publishes its API's address in the data directory, `serve.api`,
however `-listen` was given (a flag, the settings file, a systemd drop-in,
the Windows service's arguments), and removes it at its stop;
without one the commands ask the address of `MASSIMO_LISTEN` on this box
(`https` where `MASSIMO_TLS` is on).
The key goes only to a server whose `/healthz` names the node of the store
in the data directory; another that answers there gets none.
On the store, a command takes the jobs' lock `.serverbackup.lock`,
which serve's jobs take as well, and waits up to 30 seconds for another job.
They are the command line's first use of the API;
the `-server <url>` mode below would widen it.

A later mode `massimo -server <url> status` lets the commands use the server's
API with a key instead of the Cube protocol,
which gives them the server's names and hidden lists, the audit,
and the no-direct-fallback behaviour the handover asks for.
It is not part of the store stages.

### Server backups

A server backup holds the whole store and the files beside it,
encrypted for the owner's keys (`internal/serverbackup`).
It is one zip, stored without compression, of three files in this order:

| File | What it is |
|------|------------|
| `manifest.json` | plain: what the backup is, without a name of a Cube, a room or a user |
| `payload.age` | an [age](https://age-encryption.org/v1) file for the owner's recipients (`filippo.io/age`), of a deflated zip: `files.json`, `massimo.db`, `opaque.key`, `node.key`, `env` (the env file `serve` reads, where it has one) and `tls/<name>` (the TLS files) |
| `manifest.sig` | the Ed25519 signature of `manifest.json`'s bytes by `node.key`, in base64 |

The manifest is found through the zip's central directory,
so a target lists its backups by ranged reads, without fetching a whole file,
and the payload's size and hash are checked without the key.

A backup is made in steps, and nothing of it is held whole in memory:
the snapshot (`VACUUM INTO`, as `store backup <file>` writes it)
goes to a temporary directory in the data directory;
the payload is encrypted into a temporary `payload.age` and hashed as it is written;
the snapshot is removed, the manifest signed,
and the archive written whole (`internal/safefile`), mode 0600.
When the backup ends, failing or not, its temporary files are gone,
and so is the file's name where it failed.
A crash leaves them:
the temporary directory, `.serverbackup-*` in the data directory,
holds the snapshot in plain until the next backup removes it,
once it is older than six hours;
the file's name, empty, and safefile's temporary file beside it
hold nothing of the store in plain.
It needs about twice the store free in the data directory
and the store's size where the file goes;
with less, it is refused with the numbers before anything is written.

The manifest's fields, format 1:

| Field | What it holds |
|-------|---------------|
| `format` | 1; a newer one is refused with "update first" |
| `kind` | `store-snapshot`, a snapshot of the whole store; a later export would be a kind of its own |
| `uuid` | the backup's, a UUIDv7, the same wherever a copy of it is |
| `created` | when it was made, UTC |
| `trigger` | `scheduled`, `manual`, `before-update` or `before-restore`; `uploaded` is a file that came in by upload, which keeps the trigger it was made with |
| `label` | the owner's text, at most 100 characters, without control characters; left out where empty |
| `version` | Massimo's version of the server that made it |
| `schema` | the snapshot's `version` and `min_reader` |
| `node` | the box that made it: `public_key`, `node.key`'s public part in base64, and its `fingerprint` |
| `counts` | rows not removed: `users` (not an integration's), `integrations`, `cubes`, `rooms`, `audit`, `log` |
| `files` | what the payload holds beside the store and its keys: `env`, whether the env file, and `tls`, how many TLS files |
| `recipients` | for each recipient its `label`, `kind` (`pq`, `x25519` or `ssh-ed25519`) and `fingerprint` |
| `payload` | `size` and `sha256` (hex) of `payload.age`, the encrypted file |

A manifest whose text is not one Massimo writes is refused as no backup,
signed or not:
a label or a recipient's label with a control character,
another trigger, a uuid that is not one, a version that is not printable,
a fingerprint or a hash that is not one.

`files.json`, first in the payload, names each other file of it once,
with its `role` and its `source`, where it was on the box that made it,
so that a restore puts each back:

| Role | File |
|------|------|
| `store` | `massimo.db`, the snapshot |
| `opaque-key`, `node-key` | `opaque.key`, `node.key` |
| `env` | `env`, the env file `serve` reads |
| `tls-cert`, `tls-key` | `tls/<name>`, the files `MASSIMO_TLS_CERT` and `MASSIMO_TLS_KEY` name |
| `acme-account`, `acme-cert`, `acme-key`, `acme-…` | `tls/<name>`, the files of `tls/acme/` |

Two TLS files of one name are `tls/<name>` and `tls/2-<name>`.
A name the payload holds is one of these,
checked for every system it may be opened on:
nothing outside the directory,
no Windows device name such as `CON` or `nul.txt`,
no trailing dot or space.

The signature says who made a backup:
this box, another box with its fingerprint, or bad.
It is checked with the public key the manifest names,
so it proves that nothing was changed since that box made it,
not that a box is to be trusted.
A bad signature, or a payload whose size or hash differs from the manifest's,
is refused before the payload is opened.
Opening reads the payload from the file once:
it is copied into the directory it is opened into while it is hashed,
and decrypted from that copy, so that what is opened is what was hashed,
whatever a target answers a second time.
Each file is written no longer than its size,
and sizes the free space cannot take are refused first.
A fingerprint is `SHA256:` and the unpadded base64 of a SHA-256:
of the node's 32-byte public key;
of an `ssh-ed25519` key's wire form, as `ssh-keygen -l` shows it;
of an age recipient's line.

The keys are of two kinds, which do not go into one file,
since a classic recipient beside a post-quantum one would open what that one keeps closed;
age itself refuses the mix, and a box's recipients are all of one kind:

- **Post-quantum**, `age1pq1…`, ML-KEM-768 with X25519, about 2000 characters:
  a copy taken today stays closed to a quantum computer later.
- **Classic**, X25519, `age1…`, 62 characters,
  and the owner's own `ssh-ed25519` line, which goes beside it.

The server keeps the recipients alone; the secret key stays with the owner,
and only it opens a backup.
On the command line:

- `massimo store keygen -kind pq|classic [-out <file>]` makes a key;
  the kind has no default.
  The secret key goes to the file, mode 0600, never over one that exists,
  or to standard output with a warning;
  the public key, the line the server takes, is printed.
  The file is as `age-keygen` writes it.
- `massimo store inspect <file>` prints the manifest,
  who signed it (this box being the one whose `node.key` is in the data directory),
  and whether the payload is the one the manifest names; it needs no key.
  Of a manifest whose signature does not verify it prints nothing.
- `massimo store decrypt <file> -key <file> -out <dir>` checks the signature and the hash,
  then writes the payload's files, `files.json` among them,
  into an empty or new directory, mode 0600.
  It prints who signed the backup, and warns where that was another box.
  The key is an age key file or an OpenSSH `ssh-ed25519` private key without a passphrase.

A backup opens without Massimo too, with `unzip` and age:

```sh
unzip -p backup.zip payload.age | age -d -i key.txt > payload.zip
unzip payload.zip -d restored
```

`age -d -i ~/.ssh/id_ed25519` opens one made for an `ssh-ed25519` line,
and asks for the key's passphrase where it has one.

#### The catalogue

The store keeps what is known of the backups (migration 0103, section 5;
`internal/store/serverbackups.go`), and `internal/serverbackup`'s `Catalogue`
runs the jobs on it, for `serve` and for `massimo store` alike:

- **The keys** a backup is encrypted for, each with its label,
  in plain in every manifest.
  Only the owner adds and removes one (docs/api-v1.md, "Server backups").
  A key the server made is pending until the owner types back the last six
  characters of its identity, which it showed once;
  the store keeps the recipient and a hash of those six, never the identity,
  and drops a pending key after an hour.
  Pending keys encrypt nothing.
- **The targets** the copies go to, each of a kind (`serverbackup.Kind`:
  its fields, the card's form, and how to make one; `local` first, WebDAV, SFTP,
  SMB and S3 after), with its fields as JSON and its one secret field sealed
  with its `uuid` as the additional data, what it takes and its retention.
  A change to the fields of a kind with a secret field brings the secret again,
  since they say where it goes: else a user who may change targets,
  but never sees a secret, could send it to a server of their own.
  A target added is scanned: the backups it holds are added by their manifests,
  read with ranged reads and without the key,
  so that a new box sees them at once; a copy no longer there is `missing`.
  Where a backup finds no target at all, the local one is made:
  `backups/` in the data directory, every backup, the default retention.
  A local target holds a copy on the same disk as the store,
  which does not survive that disk.
- **The backups**, one row each, its `uuid` the manifest's,
  with the file's size and hash, the manifest as signed and its signature,
  and **their copies**, one per target that holds the file, under the same name
  (`massimo-<UTC time>-<uuid>.zip`).
  A copy is put by a temporary name and renamed, so that one broken off leaves
  nothing under the name.

#### The schedule

- One schedule makes each backup once (D11-A), daily at `serverbackup.time`,
  `03:30` by default, in the box's local time:
  after a change to summer time the night is 23 hours, after the change back 25,
  and a time the change skips is the hour after it.
- A run missed while the server was down is made five minutes after its start.
- Nothing is made while no key is set.
- Each target takes every backup, or the first of each day, week (Monday to
  Sunday) or month of which it holds none, in the box's time.
  A backup that no target takes, one made by hand on a daily target's day,
  goes to the local targets.

#### Retention

Each target keeps its copies by restic's forget policy (D12-A),
`{"last": N, "daily": D, "weekly": W, "monthly": M, "yearly": Y}`:
the N newest, and the newest of each of the last D days, W weeks, M months and
Y years that hold a copy; a copy any of them keeps is kept,
and the others are deleted, the files with their rows.
Beyond restic, a pinned backup is kept while it is pinned (one before an update
or a restore for 30 days, one pinned by hand until it is unpinned),
a copy pulled to local stays until it is deleted,
a target's newest copy is never pruned, and a policy of zeros keeps everything,
as restic refuses an empty one.
A new target keeps 7 days, 4 weeks and 6 months.
The prune runs after each copy to the target; a backup left without a copy goes
from the catalogue.
A copy the target refuses to delete for its lock (`ErrRetained`: an S3 bucket's
object lock, a key that may not delete) is kept, with a warning in the log,
and tried again at the next prune.

#### Retries and alerts

A backup is a `failed` row until its first copy is there, its error saying so,
so that a stop or a crash in the middle of its copies leaves a backup to retry,
never one made without a file;
the catalogue's writes after the backup is made outlive a request gone or a stop.
A scheduled backup that fails, before its file or with no copy there,
is kept as `failed`, and so is a copy of one that fails;
each is tried again after 5 minutes, 30 minutes and 2 hours,
the backup in its own row, with its `uuid` and its tries counted on,
a copy as the same file put again from a copy that is there.
After the third retry the failure is a warning in the log, `server_backup`,
an alert of the log until a later success on its target,
and, once, the notification `server_backup_failed`, on by default,
for the users who hold `backup.retry`; then it waits for a retry by hand.
A backup made by hand, by a command, before an update or a restore,
is not retried by itself: its failure is its request's answer and a warning,
it is not kept where no copy of it is there,
and its failed copies wait for a retry by hand.
The next scheduled run replaces a scheduled backup that failed.
A run the schedule missed counts as missed while no scheduled backup since it
has a copy there.
One job runs at a time on a data directory, serve's and a command's alike:
each takes the lock `.serverbackup.lock` in it (flock, LockFileEx).
A holder of `backup.retry` retries at once what failed: as the schedule made it,
pruning no more than the schedule would, and revealing nothing,
the file being encrypted (D13-A).

#### WebDAV

A target of the kind `webdav` is a folder on a WebDAV server,
such as Nextcloud with an app password, a Synology box or a storage box,
reached with the standard library alone (`internal/serverbackup/webdav.go`):

| Field | What it holds |
|-------|---------------|
| `url` | the folder: `https://` where the URL names no scheme, `http://` only where it says so, which the card warns of, since the password then travels in clear; without a user, a password, a query or a fragment in it |
| `user` | the user it signs in as, by basic authentication, without a colon |
| `password` | the secret, sealed; an app password where the server has them |

- A copy is put (`PUT`) to a hidden name beside its own,
  `.<name>.<random>.tmp`, and moved (`MOVE`, `Overwrite: F`) to its name,
  so that a put broken off leaves nothing under the name;
  its temporary file is deleted, or by a later put once it is a day old;
  a hidden file of a name not of that form stays, whoever made it.
  A name goes into the URL escaped, as one part of its path,
  and one that would name the folder itself or another is refused.
  A put asks `Expect: 100-continue`,
  so that a refusal, a quota spent among them, comes before the file is sent.
- A file of the name that is there already is not replaced:
  of the same size it is taken for the copy (a move whose answer was lost),
  of another the put fails and says so.
- A folder missing is made (`MKCOL`), its parents too.
- The backups are listed with `PROPFIND` at depth 1
  and read with ranged `GET`s of 4 MiB at least,
  so that reads that follow one another come from one request.
- The card's Test makes the folder where it is missing,
  puts a small file as a copy is put, reads it back and deletes it.
- A redirect is not followed, the password would go along:
  the error names where it points, to be put in the target.
- The client waits 30 s for a connection and for TLS,
  5 minutes for an answer once the request is sent,
  and gives a transfer up once no byte moved for 2 minutes.
- An error names the server, the request, the file and the HTTP status
  with what it means (`401`, `403`, `413`, `423`, `507`), never the password.

#### The SFTP target

The kind `sftp` (D8-A) reaches a directory on a host that speaks SSH,
in-process, with `golang.org/x/crypto/ssh` and `github.com/pkg/sftp`
(`internal/serverbackup/sftp.go`);
NAS boxes and storage boxes speak it, and OpenSSH's `scp` runs over it.

| Field | What it holds |
|-------|---------------|
| `host` | the host's name or address |
| `port` | 22 by default |
| `user` | the user it logs in as |
| `path` | the directory, relative to the user's login directory where not absolute, that directory where empty; made where missing |
| `host_key` | the host's key, confirmed: its fingerprint, `SHA256:…`, or its public key line |

- **Its key**, an Ed25519 pair of its own,
  is made by the server when the target is added (`Kind.MakeSecret`):
  the private part, in OpenSSH's form, is the target's secret,
  sealed as the others are and never answered;
  the public line, `ssh-ed25519 AAAA… massimo-server-backup`,
  is the target's `public_key`,
  for the user's `~/.ssh/authorized_keys` on the host.
  Nobody enters the secret: `"secret": ""` makes a new pair,
  also for a target whose key no longer opens (`secret_lost`).
- **A host whose key is not confirmed** is offered nothing,
  and nothing is written:
  the connection stops at the host's key (`serverbackup.HostKeyError`),
  and the add's scan and the target's test answer its fingerprint.
  The owner compares it with the host's own,
  `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the host,
  and confirms it as `host_key`.
  A key that changed is refused with both fingerprints, every time,
  until the owner confirms the new one.
  Ed25519 is asked for first, then ECDSA, then RSA signed with SHA-2;
  a key line in `host_key` asks for its own kind alone.
- **A copy** goes to a hidden temporary name beside its own,
  `.<name>.<random>.part`, mode 0600 where the host takes it, and is renamed:
  in one step by `posix-rename@openssh.com` where the host offers it,
  else by removing the name first, which SFTP's own rename does not replace.
  A copy broken off leaves nothing under its name;
  a temporary file a broken line left is removed by the next copy after a day,
  and the hidden files of other programs stay.
  A copy is read by ranges, so a scan reads the manifests alone.
- **Each job** opens a connection of its own, with timeouts:
  30 seconds to connect, 45 up to the SFTP session,
  and 2 minutes without a byte from the host,
  which a keepalive every 30 seconds keeps from striking a connection merely idle.
  An error names `user@host:port` and what refused:
  the host's key, the target's key with where its line goes,
  or the host's SFTP status.

#### The SMB target

An SMB target, of kind `smb`, is a folder of a share on a NAS,
a Windows computer or Samba (`internal/serverbackup/smb.go`).
It is reached in-process over SMB 2 and 3,
with `github.com/cloudsoda/go-smb2`, the same on every platform:
no share is mounted on the box,
so Windows' service account needs no rights on one.

| Field | What it holds |
|-------|---------------|
| `host` | the server's name or address, without `smb://` or `\\` |
| `port` | 445 by default |
| `share` | the share's name, without `/` or `\` |
| `path` | a folder in the share, its parts separated by `/` or `\`, the share's root where empty; made where it is missing |
| `user` | the user it signs in as; `DOMAIN\user`, with no `domain` given, is taken as both |
| `password` | the user's password, the target's secret |
| `domain` | the domain or workgroup, where the server asks for one |

- It signs in by NTLM with the user and the password,
  and signs every message,
  which also refuses a server that would take a wrong password for its guest.
- It offers SMB 3.1.1 with AES-GCM and AES-CCM,
  and encrypts where the server asks for it:
  Samba's `server smb encrypt = required`,
  Windows' `Set-SmbShare -EncryptData $true`.
  It cannot ask for encryption where a server only offers it:
  go-smb2 encrypts where the server asks, for the session or the share,
  and has no setting by which a client asks for it,
  so a share that does not ask is reached signed, not encrypted.
  A copy is encrypted by age in any case;
  the share's encryption hides the names and the listing on the network too.
- Each call connects, signs in, mounts the share and logs off again;
  a copy opened for ranged reads keeps its connection until it is closed.
  Each request to the server has a minute,
  the connection with its sign-in among them:
  one that takes longer ends the connection, and its error says so.
- A copy is written to a hidden temporary name beside its own,
  `.<name>.<random>.part`, in writes of 1 MiB,
  then flushed, closed and renamed over the name,
  so that a copy broken off leaves nothing under the name;
  the temporary file is removed where the put fails,
  and one a crash left by the next put,
  once the server's clock says it is a day old.
- The card's Test makes the folder where it is missing,
  and writes, reads back and deletes a small file in it:
  a target that cannot delete cannot prune.
- An error names the target as `//host/share/path` and what the server answered,
  "The attempted logon is invalid" for a refused password,
  "{Network Name Not Found}" for a share that is not there;
  it never holds the password.
- `make target-test` tests it against Samba with signing and encryption required
  (`tools/target-test/README.md`).

#### The S3 target

The kind `s3` keeps the copies in a bucket of an S3-compatible store (D10-A):
AWS, MinIO, Backblaze B2, Wasabi, Cloudflare R2 and Hetzner among them.
It speaks five calls of the S3 API, signed with AWS Signature Version 4,
all with Go's standard library:
`PUT`, `GET` with a range, `HEAD`, `DELETE` and `ListObjectsV2`.

| Field | What it holds |
|-------|---------------|
| `endpoint` | the store's URL without a path, such as `https://s3.eu-central-1.amazonaws.com`; `https` where it names no scheme, `http` only where it says so |
| `region` | the bucket's region, `us-east-1` by default; R2 takes `auto` |
| `bucket` | the bucket, made with the store's own tools |
| `prefix` | a folder in the bucket, empty for its top; the backups go directly under it, and keys below it are left alone |
| `path_style` | the bucket reached by path, `https://host/bucket/…`, not by virtual host, `https://bucket.host/…`; MinIO needs it, and so do an endpoint given by its address and a bucket with dots over `https` |
| `access_key` | the key's id |
| `secret_key` | the key's secret, the kind's secret field, sealed by the store |

- A copy is put in one `PUT`, under its final name:
  S3 has no rename and needs none, since a `PUT` is atomic,
  and one broken off or refused leaves nothing.
  One `PUT` takes at most 5 GiB, so a larger backup is refused with its size.
  The body is sent unsigned (`UNSIGNED-PAYLOAD`),
  with its `Content-MD5`, read once ahead from the file on the box
  (or from the copy on another target a push reads, which is read twice),
  since a bucket with object lock takes no `PUT` without it,
  AWS's rule, and the store checks the body by it;
  TLS carries it, and the check reads the copy back with its hash.
- A copy is read by ranges, each a `GET` with `Range`,
  and with `If-Match` on the version `HEAD` found,
  so that an object replaced while it is read is refused.
  A read that follows the one before fetches twice as much ahead, up to 8 MiB,
  so that a copy read whole costs a few requests.
- The list goes page by page, with its continuation token,
  and holds the names a backup may have, directly under the prefix.
- The card's test lists the bucket,
  then writes, reads back and deletes a probe, `.massimo-check-<random>`.
- The key needs `s3:ListBucket` on the bucket,
  and `s3:PutObject`, `s3:GetObject` and `s3:DeleteObject` under the prefix.
- A request that moves no byte for two minutes is broken off,
  and so is an answer whose header takes as long; a redirect is not followed.
  An error says the store's status, its code and its message,
  and the bucket's region where the store names another.
- **Object lock.**
  A bucket with object lock is versioned,
  and there a `DELETE` succeeds by hiding the object behind a delete marker:
  the copy goes from the catalogue,
  while the locked version stays in the bucket until its lock ends,
  out of the key's reach, and the store's own tools bring it back.
  A `DELETE` the store refuses keeps the copy (`ErrRetained`):
  `AccessDenied`, a key that may not delete,
  or an answer that speaks of a lock, a retention or WORM,
  such as MinIO's for a locked version.
  The prune warns, `server backups: a copy the target keeps`, and goes on;
  a copy deleted by hand stays, the request answering `409` with why.
- `TestS3Live` runs the target against a real store,
  where `MASSIMO_TARGET_TEST_S3_ENDPOINT`, `_BUCKET`, `_REGION`, `_KEY`
  and `_SECRET` are set, as `make target-test` sets them for its MinIO,
  and against a bucket with object lock and a default retention too,
  `_LOCK_BUCKET`.

#### Restore by scope

The owner restores chosen scopes of a backup (D6-A, D7-A;
`internal/serverbackup`, `Catalogue.Restore`; docs/api-v1.md, "Server backups").
Each table of the store belongs to scopes by its rows,
in one list beside the store's queries (`internal/store/restore.go`,
`scopeTables`), and a test fails on a table that is in none:
a table added later is placed there, or a restore would leave it out.

| Scope | Rows | File |
|-------|------|------|
| `settings` | the settings but `node.id` and `tenant.id`, the targets of the server's backups with their sealed secrets, their keys | – |
| `users` | the users that are not an integration's, `roles`, `role_capabilities`, `capabilities_disabled`, `auth_providers`, the providers' identities, the users' API keys, preferences and push subscriptions on a key | `opaque.key` |
| `integrations` | the integrations, their users, identities, keys and preferences | – |
| `cubes`, `cube:<serial>` | each Cube by its serial, with its rooms, Cube backups and their files, and its commands | – |
| `history` | `audit`, `log`, the commands of no Cube | – |
| the identity, ticked | `node.id`, `tenant.id` | `node.key` |
| never | the sessions and the push subscriptions on one, `auth_failures`, `backup_pending`, `schema_version`, `server_backups` and `server_backup_copies`, the live catalogue | – |

The env file and the TLS files are restored only where ticked, each on its own,
since a new box's paths can differ:
the env file goes where this server reads its own,
the TLS files where it keeps its own (files mode's,
else the self-signed pair's place, and the ACME directory).
Each file replaced stays beside as `<name>.before-restore`.
A place this server cannot write refuses the restore, and the plan says so,
of the env file and of the TLS files whether they are asked for or not:
under the systemd unit's protection (`ProtectSystem=strict`) only the data
directory and the log's are writable to `serve`, so `/etc/massimo/env` and
`/etc/massimo/cert.pem` are put in place by hand, from `massimo store decrypt`.

A restore, in this order:

1. The backup is opened with the owner's key,
   which the request carries and nothing writes,
   into a temporary directory in the data directory (`.serverbackup-restore-*`,
   removed at its end, or by the next backup after a crash).
   A backup whose store is of a newer schema than this one's is refused first:
   update first.
   One of an older schema is migrated there, on its copy alone.
2. It is planned: run in a transaction and rolled back.
   A refusal takes no backup.
   Users and sign-in, or the identity,
   are refused where the server cannot restart itself,
   since it keeps their keys in memory.
3. Every file is written beside its place, with a copy of the file it replaces,
   so that nothing but renames is left once the store is restored;
   one that cannot be written refuses the restore here.
4. A local backup is taken, pinned for 30 days (`before-restore`).
   What the server writes from then until the store's commit, a few seconds,
   is in neither: the answer names both times (`before_backup_time`,
   `restored_at`).
5. The scopes are replaced whole, in one transaction:
   - A row of the backup takes the id of the live row of its uuid,
     or of its natural key (a role's key, a Cube's serial, a target's name,
     a key's fingerprint), or a new one;
     its references follow through a map of the backup's ids to this store's,
     read from the schema (`PRAGMA foreign_key_list`),
     so that a later table's are followed without new code.
     A reference to a row of another scope maps to the live row of its uuid.
   - A reference to a row of another scope that this store lacks refuses the restore
     with the list of what is missing (a user by name, a Cube by serial, …).
     One the schema lets go (`ON DELETE SET NULL`) is cleared,
     and so is one to a session, which never comes back.
   - A live row the backup lacks is removed.
     Where a row outside the scopes still names it and its table keeps removed
     rows (`deleted_at`), it stays, marked removed:
     a user who left since the backup stays in the audit's rows.
     What the schema removes with it (`ON DELETE CASCADE`) goes,
     the sessions of a user removed among them.
     A target that holds a copy of a pinned backup stays whatever the backup says,
     so that the backup taken first keeps its copy and undoes the restore by its id.
   - The audit's and the log's rows take ids above those this store gave,
     as new rows would, so that the newest line a user saw (`log.seen`)
     hides nothing that comes after.
   - Users and sign-in that would leave no owner, where there was one,
     is refused.
   - The sealed secrets of the rows taken are opened with the backup's
     `opaque.key` and sealed with this store's.
     With users and sign-in, `opaque.key` travels too, since a password of
     version 2 opens with it alone: every sealed secret, the live ones too,
     is then sealed with the backup's (D7-A).
6. The files are renamed into place: `opaque.key` with users and sign-in,
   `node.key` with the identity, the env file and the TLS files where ticked.
7. The server restarts, as `POST /api/v1/restart` does:
   it keeps `opaque.key` and its node in memory.
   It restarts also where a file failed after the store's commit,
   which the answer then names.

This box keeps its `node.key` unless "take the backup's identity" is ticked;
the plan says whether the box that made the backup still answers on the network,
since two boxes of one identity clash.
A Cube restored is the store's rows alone:
writing its configuration onto the Cube stays the Cube restore
(`POST /api/v1/cubes/{sn}/restore`), with its radio cost,
which the restore's answer offers for each Cube with the newest of its Cube backups.
"Check with the key" opens a backup into a temporary directory the same way
and runs `PRAGMA integrity_check` on its store, changing nothing:
a backup nobody has opened is a hope, not a backup.

On the command line, `massimo store restore <file|id> -scope <list> -key <file>`,
with `-identity`, `-env`, `-tls` and `-plan`,
goes through a running `serve` (its API with `MASSIMO_API_TOKEN`,
a file uploaded first), so that the restore is that server's,
and acts on the store otherwise: under the jobs' lock,
with the store held alone,
which a `serve` that has it open refuses (`store.ErrInUse`);
`-plan` prints what it would do and changes nothing,
a file uploaded for it removed again where the server did not hold it.

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
  `schemes` the user's own, a list of `massimo-scheme/1` documents;
  `log.seen` the user's marks of the log's alerts (AP LogAlerts,
  `docs/api-v1.md`, "GET /api/v1/log/alerts").
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

- Files: `internal/store/events.go` (the writer:
  a batch in one transaction at 100 events or a second after the first,
  before a read and at the stop; the paging query;
  the daily prune per Cube in chunks of 5000,
  answering a read and a stop between two chunks);
  `internal/server/log.go` (`remember` hands a Cube's event to the writer,
  `GET …/log` reads the store), the log's part,
  `internal/server/ui/src/log/` (the log view pages with "Older",
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
  Since AP LogCursor, `after=<id>` pages forward on both log routes,
  `409` for a cursor beyond the newest id (`ErrBeyondNewest`),
  and the ring's events carry ids too,
  from one counter of the server's, started at its start in microseconds,
  taken with the append under one lock (`logRing`, `internal/server/log.go`).
  A clock set behind the previous run's gives a cursor of that run a `409`,
  and the client reads the log again;
  the unit needs no ordering on the clock,
  since systemd and systemd-timesyncd set it forward before `serve` starts,
  to their build time or the time last saved,
  also on a Raspberry Pi without a clock of its own (AP LogEdges, D6-A).
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
  delete as a tombstone without the file, the prune, the windows kept),
  migrations 0005 (the four reasons),
  0013 (an id never given out twice) and 0026 (the windows kept across a start);
  `internal/server/backups.go` (the automatic backup) and `backups_api.go` (the routes);
  `backup.ConfigHash` and `cubelan.SnapshotOf` (the comparison);
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
- The windows across a start (AP BackupWatch v1.0, Block 1, D1-C):
  a dirty Cube's first and last change are kept in `backup_pending`,
  with the hash of the configuration and of the reference they count against.
  The look that finds the Cube dirty, or changed again while dirty,
  writes them, so at most once a minute;
  a backup stored, the Cube clean again, and a delete, a prune or a restore that
  resets the reference delete them, and a clean Cube costs the store no write.
  The Cube's runtime reads them when it is made,
  so that the list's `changed` and `due` are right from the first request after a start.
  The first look then keeps them while the configuration is the one kept,
  and counts another as a change at that look, the first change kept;
  a change made while no server ran is so a change at the first look.
  Windows kept against another reference than the newest backup are left over,
  by a binary before 0026 that took the backup, and go.
  Before that, every start began both windows again:
  a change got its backup at `auto_min` after the last start,
  not after the change,
  and starts closer together than `auto_min` never let it come.
- The backup at a stop: a clean stop (SIGINT, SIGTERM, a SIGHUP that `-sighup`
  makes a stop, the Windows service manager's stop, the restart from the API,
  the update's restart of the service) takes the `auto` backup of each connected
  Cube with a change pending, after a last look, before the Cubes' runtimes end
  and so before serve closes the store;
  the Cubes at once, within 5 s in all,
  inside serve's 10 s for the server's stop.
  A Cube not connected, under a restore, or whose backup fails, is logged
  (a failure as a `backup` line with `at: stop`,
  a warning for what the Cube did not give, an error for what failed here)
  and keeps its windows for the next start.
  A Cube without any backup gets its first at the first look after the next start.
- API: `GET`/`POST …/backups` (the list newest first,
  with `changed` and `due` while one is pending; one by hand with a note),
  `GET …/backups/{id}` (as stored, or `?format=`),
  `DELETE …/backups/{id}` (capability `restore`),
  `GET …/backups/{id}/diff?against={id}`;
  the plan and the restore take `backup=<id>`; the settings `backups.auto_min`,
  `backups.auto_max` (never below `auto_min`), `backups.keep_days_auto`,
  `backups.keep_days_manual`, `backups.keep_newest`.
  A Cube without a row in the store has none of it (503).
- A Cube no longer served (AP BackupWatch v1.0, Block 3, D4-B, D5-A, D6-A):
  a row of the store that no runtime serves,
  taken out of the configuration or only found,
  keeps its backups, the newest `backups.keep_newest` whatever their age.
  Its list (without `changed` and `due`), the file, the diff and the delete
  find it by its row;
  `POST …/backups` and the restore answer 409,
  as it takes no backup and no restore.
  `GET /api/v1/cubes` gives each Cube's count kept, `backups`,
  and `GET /api/v1/backups/unserved` lists the rows not served that keep backups,
  with the count and the newest's time,
  both from one grouped query by serial (`KeptBackups`).
- UI: Maintenance → Cube backups, and Cube settings → Backup and restore
  for the chosen Cube: the download as YAML or JSON,
  a backup by hand, the list with download, the comparison with the one before,
  restore and delete, a restore from a file; a restore shows its plan first,
  then each step as the Cube took it.
  After the served Cubes,
  a fold for each Cube that route lists, also one no longer on the network,
  marked not served: download, comparison and delete, no backup and no restore;
  one of them goes onto another Cube by that Cube's restore from a file.
- Tests: the store's round trip,
  the prune by reason and age and the migration's copy;
  no id given out again after a prune or a purged tombstone,
  and 0013's copy with its sequence; the hash;
  the first backup, none without a change,
  one after a quiet window and one at `auto_max`;
  the change against the one before after deleting the newest,
  and one at once, with an id of its own,
  after deleting all or pruning the only one;
  the windows across a start,
  `auto_min` after the change and `auto_max` after the first,
  the first request after the start naming both;
  a change while stopped counted at the first look;
  the stop's backup, before the store closes (`internal/serve`),
  of a change pending and of one only the stop's look saw;
  no write of the windows while a Cube is clean; windows left over dropped;
  the table's round trip, 0026 read by a binary for 25, and its down part;
  the routes and the settings' check;
  a restore keeping `before_restore` and `upload`, by id, and clean after;
  the diff of a backup against itself and an edited copy;
  a found Cube's backups listed, given out in both formats,
  compared and deleted,
  409 for its backup and its restore, and the counts by serial;
  the list of the rows not served,
  without the served Cube, a row without backups, a tombstone or a removed row,
  and its capability;
  the pytest suite's panel,
  with the fold of a Cube not served that discovery does not hear.
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

## 11. Future improvements

Changes worth making that wait for other work,
each with what it waits for and why.

**`AUTOINCREMENT` for `log.id`**,
with the next migration that makes `log` again anyway.
Today the prune keeps the log's newest row (section 5, `log.id`),
so no id is given out twice;
that holds only as long as the prune is the only thing that deletes log rows.
`AUTOINCREMENT` would make it the schema's guarantee, covering any later delete:
a route that clears the log, or a `DELETE` run by hand.
On its own it costs a rebuild of `log` at the upgrade,
1.7 s at 90,000 rows on a PC and not measured on a Raspberry Pi,
and a down part under the store's downgrade rule (section 5, "Migrations"),
which is why it waits for a rebuild that is needed anyway.
Migration 0013 did the same for `backups` (section 5, "Migrations").

**`AUTOINCREMENT` for `audit.id`**,
with the next migration that makes `audit` again anyway,
should anything come to key on the audit's ids.
`audit.id` has none either, and nothing keeps the audit's newest row:
once a prune by `audit.keep_days` has emptied the table,
the next row is given id 1 again.
Nothing depends on the ids today:
`before` on `GET /api/v1/audit` is a time, not an id,
no foreign key points at them,
the web UI reads only a command's row,
by its command (`GET /api/v1/commands/{id}`),
a backup of the store copies the rows as they are,
and `massimo store info` only counts them.
The one reader is the app's instrumented tests
(`app/android/app/src/androidTest/kotlin/ch/metaworx/massimo/Host.kt`),
within one test on a fresh server.
