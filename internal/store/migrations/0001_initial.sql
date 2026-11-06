-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Version 1 of the store: the schema of docs/store.md section 5 with the
-- columns its notes add. Every row that will be replicated or exported
-- carries a UUIDv7 (uuid); the owned tables carry version, deleted_at
-- and origin for sync (docs/server.md, "What version 1 prepares"). Times
-- are TEXT in RFC 3339 with the Z suffix, JSON documents TEXT, booleans
-- INTEGER 0 or 1. node.id and tenant.id are seeded in settings by the
-- store right after this file, in the same transaction, since SQL cannot
-- make a UUID.
--
-- The content and structure rule (rule_test.go): no index and no WHERE
-- on a content column (names, addresses, hidden, settings.value,
-- preferences.value, events.data, audit.detail, backups.body, note).

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
    version    INTEGER NOT NULL DEFAULT 1,    -- a role replicates with its set; a change of the set bumps it
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',   -- node.id of the node that made the last change
    UNIQUE (key)
);

CREATE TABLE role_capabilities (
    role_id    INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    capability TEXT    NOT NULL,              -- a capability key, docs/store.md section 6
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
    uuid          TEXT    NOT NULL UNIQUE,
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
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT '',
    UNIQUE (name)
);

CREATE TABLE tokens (
    id             INTEGER PRIMARY KEY,
    uuid           TEXT    NOT NULL UNIQUE,
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('session', 'api')),
    hash           TEXT    NOT NULL UNIQUE,      -- SHA-256 of the token, hex; the token itself is never stored
    label          TEXT    NOT NULL DEFAULT '',  -- 'MASSIMO_API_TOKEN', 'Loxone', 'Firefox on the laptop'
    created_at     TEXT    NOT NULL,
    expires_at     TEXT,                         -- NULL: never (API keys); sessions: moved on every use
    last_used_at   TEXT,
    last_used_from TEXT    NOT NULL DEFAULT '',  -- the client's address
    revoked_at     TEXT,
    version        INTEGER NOT NULL DEFAULT 1,
    deleted_at     TEXT,
    origin         TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX tokens_user ON tokens (user_id);

CREATE TABLE settings (
    key        TEXT    NOT NULL,   -- 'stale_after', 'poll_interval', 'node.id', 'tenant.id', …
    value      TEXT    NOT NULL,   -- JSON: a string, a number or a document
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id),
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (key)
);

CREATE TABLE cubes (
    id                 INTEGER PRIMARY KEY,
    uuid               TEXT    NOT NULL UNIQUE,
    serial             TEXT    NOT NULL,              -- upper case, as printed on the Cube
    configured         INTEGER NOT NULL DEFAULT 1,    -- 0: found by discovery, or removed; not served
    name               TEXT    NOT NULL DEFAULT '',
    address            TEXT    NOT NULL DEFAULT '',   -- host[:port]; '' with follow_discovery: from discovery
    follow_discovery   INTEGER NOT NULL DEFAULT 0,    -- the address follows the Cube's discovery answers
    proxy_address      TEXT    NOT NULL DEFAULT '',   -- the server's listener for this Cube, host[:port]; '' = :62910 with a single Cube, none with several
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
    version            INTEGER NOT NULL DEFAULT 1,
    deleted_at         TEXT,
    origin             TEXT    NOT NULL DEFAULT '',
    UNIQUE (serial)
);

CREATE TABLE preferences (
    uuid       TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,   -- 'scheme' (the massimo-scheme/1 document), 'ui' (poll, expert, clock, language)
    value      TEXT    NOT NULL,   -- JSON
    updated_at TEXT    NOT NULL,
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, key)
);

CREATE TABLE events (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    device    TEXT    NOT NULL DEFAULT '',       -- radio address, '' for Cube-level events
    data      TEXT    NOT NULL                   -- JSON: the event object as GET …/log returns it
);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_device_time ON events (cube_id, device, time);

CREATE TABLE audit (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    time      TEXT    NOT NULL,
    user_id   INTEGER REFERENCES users (id),     -- NULL: no authentication configured, or the server itself
    token_id  INTEGER REFERENCES tokens (id),
    remote    TEXT    NOT NULL DEFAULT '',       -- the client's address
    action    TEXT    NOT NULL,                  -- 'setpoint', 'room.config', 'cube.update', 'user.add', 'login', …
    cube_id   INTEGER REFERENCES cubes (id),
    target    TEXT    NOT NULL DEFAULT '',       -- 'device:0abc12', 'room:3', 'user:anna'
    detail    TEXT    NOT NULL DEFAULT '{}',     -- JSON: the request as received, secrets removed, and the result
    result    TEXT    NOT NULL DEFAULT 'ok' CHECK (result IN ('ok', 'refused', 'failed'))
);
CREATE INDEX audit_time ON audit (time);
CREATE INDEX audit_user ON audit (user_id, time);

CREATE TABLE backups (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('connect', 'daily', 'before_restore', 'manual', 'upload')),
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,
    body       BLOB    NOT NULL,                 -- the backup file, gzip
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual and before_restore
    note       TEXT    NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);

CREATE TABLE auth_failures (
    key          TEXT    PRIMARY KEY,            -- 'addr:<ip>' or 'user:<name>'
    window_start TEXT    NOT NULL,
    count        INTEGER NOT NULL,
    locked_until TEXT
);

-- The default roles of docs/store.md section 6. owner holds every
-- capability by code, so it has no rows in role_capabilities.
INSERT INTO roles (key, builtin, created_at, updated_at) VALUES
    ('owner',       1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('manager',     1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('operator',    1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('guest',       1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('viewer',      1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('integration', 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

INSERT INTO role_capabilities (role_id, capability)
SELECT r.id, c.capability FROM roles r JOIN (
              SELECT 'manager' AS role, 'read' AS capability
    UNION ALL SELECT 'manager', 'setpoint'
    UNION ALL SELECT 'manager', 'lock'
    UNION ALL SELECT 'manager', 'configure'
    UNION ALL SELECT 'manager', 'rename'
    UNION ALL SELECT 'manager', 'structure'
    UNION ALL SELECT 'manager', 'pair'
    UNION ALL SELECT 'manager', 'discover'
    UNION ALL SELECT 'manager', 'backup'
    UNION ALL SELECT 'manager', 'restore'
    UNION ALL SELECT 'manager', 'guests'
    UNION ALL SELECT 'operator', 'read'
    UNION ALL SELECT 'operator', 'setpoint'
    UNION ALL SELECT 'operator', 'lock'
    UNION ALL SELECT 'operator', 'configure'
    UNION ALL SELECT 'operator', 'discover'
    UNION ALL SELECT 'operator', 'backup'
    UNION ALL SELECT 'guest', 'read'
    UNION ALL SELECT 'guest', 'setpoint'
    UNION ALL SELECT 'guest', 'configure'
    UNION ALL SELECT 'viewer', 'read'
    UNION ALL SELECT 'integration', 'read'
    UNION ALL SELECT 'integration', 'setpoint'
    UNION ALL SELECT 'integration', 'lock'
) c ON c.role = r.key;
