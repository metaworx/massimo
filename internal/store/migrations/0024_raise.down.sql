-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0024_raise.sql (AP StoreRaise v1.2, Block 3, D1-D;
-- docs/store.md section 5, "Migrations"): massimo store downgrade runs
-- it, with foreign keys off and checked before the commit, to take a file
-- back to schema 23, which a binary built for any earlier version opens
-- again. Every table has the text of schema 23 again, as
-- migrations/store.sql held it, and every row a binary for 23 wrote comes
-- back as it was; what the raised schema added is lost, and so is what
-- schema 23 cannot hold. A table is made again by copying it to a TEMP
-- table, dropping it and creating it under its own name and text, so that
-- sqlite_master is that of schema 23 to the byte.
--
-- The losses, which downgrade_test.go (TestRaiseDownLosses) and
-- docs/store.md ("Migrations") name by their numbers:
--
-- L1. log.real_user_id and audit.real_user_id go: who made a row where
--     an integration acted for a linked user. An audit row keeps the
--     integration and its user in detail (via, for).
-- L2. user_identities: a removed identity (deleted_at) is deleted;
--     updated_at, version and origin go.
-- L3. An integration's users are named <key>, <key>.system and
--     <key>.<the subject's first 8 characters> again, the whole subject
--     where another of its users has those 8.
-- L4. integrations.key is made from the name again, as the approval made
--     it: where the name changed after the approval, or a -2 told it from
--     another integration's, it may differ; where a user that is no
--     integration's has that name, -<id> is added.
-- L5. A removed user is named <name>-del-YYYYMMDD-HHNN of its removal
--     again, -<id> added where another user removed in that minute, or
--     one not removed, had that name first.
-- L6. roles.uuid goes. Of several rows of one key (a role added again
--     after its removal), the one not removed stays, else the newest
--     removed; the others go with their capabilities.
-- L7. cubes.proxy_virtual goes back into cube_options: a row for each
--     Cube row with the option on, its updated_at and updated_by the
--     Cube's; a row with the option off is not made again.
--
-- Not lost (D15-C): of several rows of one serial (a Cube added again
-- after its removal), the one not removed keeps the serial, else the
-- newest removed; each other, removed, is named <serial>~<its id> in
-- cubes.serial alone, so that schema 23's UNIQUE (serial) holds, and
-- keeps its log, backups, rooms, commands and audit rows. log.serial and
-- the serials in JSON (log.data, audit.detail) are history and stay as
-- written. A binary for 23 reads the Cubes not removed alone, and 0024
-- takes the ~<id> off again.
-- L8. tokens.revoked_at is deleted_at again, and the old deleted_at,
--     which no release wrote, is NULL; commands.deleted_at, never
--     written either, is NULL.
--
-- Without loss: the view events and its triggers, the backups' files
-- (x'' for a tombstone), auth_providers.auto_domains comma-separated,
-- the indexes of schema 23, and the sequences of backups and commands.
-- What the up part did to the rows of schema 23 stays: the sources its
-- fill gave rows of a binary before 0009, and the '' it made of a
-- last_level outside '', view, control and admin.

-- What the tables made again need, kept while they are dropped: the
-- integrations' keys, the names of the users, the role rows that go, the
-- Cube rows renamed, and the sequences.
CREATE TEMP TABLE down_keys (integration_id INTEGER PRIMARY KEY, key TEXT NOT NULL);
WITH RECURSIVE
    walk(id, rest, out) AS (
        SELECT g.id, lower(g.name), '' FROM integrations g
        WHERE EXISTS (SELECT 1 FROM users u WHERE u.integration_id = g.id AND u.name = g.uuid || ':')
        UNION ALL
        SELECT id, substr(rest, 2),
               CASE WHEN substr(rest, 1, 1) BETWEEN 'a' AND 'z' OR substr(rest, 1, 1) BETWEEN '0' AND '9' THEN out || substr(rest, 1, 1)
                    WHEN out <> '' AND substr(out, -1) <> '-' THEN out || '-'
                    ELSE out END
        FROM walk WHERE rest <> ''),
    base(id, key) AS (
        SELECT id, CASE WHEN length(trim(out, '-')) > 40 THEN trim(substr(trim(out, '-'), 1, 40), '-') ELSE trim(out, '-') END
        FROM walk WHERE rest = ''),
    ranked(id, key, n) AS (
        SELECT id, key, row_number() OVER (PARTITION BY key ORDER BY id)
        FROM (SELECT id, CASE key WHEN '' THEN 'integration' ELSE key END AS key FROM base))
INSERT INTO temp.down_keys (integration_id, key)
SELECT id, CASE WHEN n = 1 THEN key ELSE key || '-' || n END FROM ranked;
UPDATE temp.down_keys SET key = key || '-' || integration_id
WHERE integration_id > (SELECT min(k.integration_id) FROM temp.down_keys k WHERE k.key = down_keys.key)
   OR EXISTS (SELECT 1 FROM users u WHERE u.integration_id IS NULL AND u.name IN (down_keys.key, down_keys.key || '.system'));
INSERT INTO temp.down_keys (integration_id, key)
SELECT id, 'request-' || uuid FROM integrations WHERE id NOT IN (SELECT integration_id FROM temp.down_keys);

CREATE TEMP TABLE down_users (user_id INTEGER PRIMARY KEY, integration_id INTEGER NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL);
INSERT OR IGNORE INTO temp.down_users (user_id, integration_id, kind, name)
SELECT u.id, g.id, 'own', k.key FROM users u JOIN integrations g ON g.id = u.integration_id JOIN temp.down_keys k ON k.integration_id = g.id
WHERE u.name = g.uuid || ':';
INSERT OR IGNORE INTO temp.down_users (user_id, integration_id, kind, name)
SELECT u.id, g.id, 'system', k.key || '.system' FROM users u JOIN integrations g ON g.id = u.integration_id JOIN temp.down_keys k ON k.integration_id = g.id
WHERE u.name = g.uuid || ':system';
WITH ranked(user_id, integration_id, key, subject, n) AS (
    SELECT i.real_user_id, i.integration_id, k.key, lower(i.subject),
           row_number() OVER (PARTITION BY i.integration_id, substr(lower(i.subject), 1, 8) ORDER BY i.id)
    FROM user_identities i JOIN temp.down_keys k ON k.integration_id = i.integration_id
    WHERE i.real_user_id IS NOT NULL AND i.subject IS NOT NULL AND i.subject <> 'system')
INSERT OR IGNORE INTO temp.down_users (user_id, integration_id, kind, name)
SELECT user_id, integration_id, 'user', key || '.' || CASE WHEN n = 1 THEN substr(subject, 1, 8) ELSE subject END FROM ranked;

CREATE TEMP TABLE down_names (user_id INTEGER PRIMARY KEY, name TEXT NOT NULL);
INSERT INTO temp.down_names (user_id, name)
SELECT u.id, coalesce(n.name, u.name) || CASE WHEN u.deleted_at IS NULL THEN '' ELSE '-del-' || strftime('%Y%m%d-%H%M', u.deleted_at) END
FROM users u LEFT JOIN temp.down_users n ON n.user_id = u.id;
UPDATE temp.down_names SET name = name || '-' || user_id
WHERE user_id IN (SELECT user_id FROM (
    SELECT d.user_id, row_number() OVER (PARTITION BY d.name ORDER BY u.deleted_at IS NOT NULL, u.deleted_at, u.id) AS n
    FROM temp.down_names d JOIN users u ON u.id = d.user_id) WHERE n > 1);

CREATE TEMP TABLE down_roles (id INTEGER PRIMARY KEY, keep INTEGER NOT NULL);
INSERT INTO temp.down_roles (id, keep)
SELECT r.id, (SELECT k.id FROM roles k WHERE k.key = r.key ORDER BY k.deleted_at IS NOT NULL, k.deleted_at DESC, k.id DESC LIMIT 1) FROM roles r;
DELETE FROM temp.down_roles WHERE id = keep;

CREATE TEMP TABLE down_cubes (id INTEGER PRIMARY KEY);
INSERT INTO temp.down_cubes (id)
SELECT c.id FROM cubes c
WHERE c.id <> (SELECT k.id FROM cubes k WHERE k.serial = c.serial ORDER BY k.deleted_at IS NOT NULL, k.deleted_at DESC, k.id DESC LIMIT 1);

CREATE TEMP TABLE down_sequence AS SELECT name, seq FROM sqlite_sequence WHERE name IN ('backups', 'commands');

-- D15-C: the other rows of a serial named <serial>~<id>, cubes.serial alone.
UPDATE cubes SET serial = serial || '~' || id WHERE id IN (SELECT id FROM temp.down_cubes);

-- L6: what the role rows that go leave.
UPDATE users SET role_id = (SELECT keep FROM temp.down_roles WHERE id = users.role_id) WHERE role_id IN (SELECT id FROM temp.down_roles);
UPDATE auth_providers SET auto_role_id = (SELECT keep FROM temp.down_roles WHERE id = auth_providers.auto_role_id)
WHERE auto_role_id IN (SELECT id FROM temp.down_roles);
DELETE FROM role_capabilities WHERE role_id IN (SELECT id FROM temp.down_roles);

-- 12. auth_providers and commands (L8).
CREATE TEMP TABLE down_rows AS SELECT * FROM auth_providers;
DROP TABLE auth_providers;
CREATE TABLE auth_providers (
    id            INTEGER PRIMARY KEY,
    uuid          TEXT    NOT NULL UNIQUE,
    key           TEXT    NOT NULL,                -- in the routes, as a role's key
    name          TEXT    NOT NULL DEFAULT '',     -- on the sign-in button; '' shows the key
    issuer        TEXT    NOT NULL,                -- https://…, its discovery under /.well-known/openid-configuration
    client_id     TEXT    NOT NULL,
    client_secret TEXT    NOT NULL DEFAULT '',
    scopes        TEXT    NOT NULL DEFAULT 'openid email profile',
    auto_create   INTEGER NOT NULL DEFAULT 0,      -- 1: a sign-in without an identity creates the user
    auto_role_id  INTEGER REFERENCES roles (id),   -- the role of a user created so; NULL: viewer
    auto_domains  TEXT    NOT NULL DEFAULT '',     -- the e-mail domains it creates users for, comma-separated; '': any
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT ''
, groups_claim TEXT NOT NULL DEFAULT 'groups', allowed_groups TEXT NOT NULL DEFAULT '[]');
INSERT INTO auth_providers (id, uuid, key, name, issuer, client_id, client_secret, scopes, auto_create, auto_role_id, auto_domains, enabled,
                            created_at, updated_at, updated_by, version, deleted_at, origin, groups_claim, allowed_groups)
SELECT p.id, p.uuid, p.key, p.name, p.issuer, p.client_id, p.client_secret, p.scopes, p.auto_create, p.auto_role_id,
       (SELECT coalesce(group_concat(value, ','), '') FROM json_each(p.auto_domains)), p.enabled,
       p.created_at, p.updated_at, p.updated_by, p.version, p.deleted_at, p.origin, p.groups_claim, p.allowed_groups
FROM temp.down_rows p;
DROP TABLE temp.down_rows;
CREATE UNIQUE INDEX auth_providers_key ON auth_providers (key) WHERE deleted_at IS NULL;

CREATE TEMP TABLE down_rows AS SELECT * FROM commands;
DROP TABLE commands;
CREATE TABLE "commands" (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- never given out twice
    uuid            TEXT    NOT NULL UNIQUE,
    parent_id       INTEGER REFERENCES commands (id) ON DELETE CASCADE,  -- a step's request
    cube_id         INTEGER REFERENCES cubes (id),
    user_id         INTEGER REFERENCES users (id),                       -- NULL: no authentication, or the server itself
    token_id        INTEGER REFERENCES tokens (id),
    idempotency_key TEXT    NOT NULL DEFAULT '',   -- the client's Idempotency-Key; '' after 24 hours
    action          TEXT    NOT NULL,              -- as audit.action; 'step' for a step
    target          TEXT    NOT NULL DEFAULT '',   -- 'device:0abc12', 'room:3', 'user:anna'
    request         TEXT    NOT NULL DEFAULT '{}', -- JSON: the method, the path, the body (secrets removed) and its hash; a step: its key and lines
    state           TEXT    NOT NULL CHECK (state IN ('queued', 'sent', 'acknowledged', 'failed', 'dropped')),
    status          INTEGER NOT NULL DEFAULT 0,    -- the HTTP status answered; 0 while running, and for a step
    result          TEXT    NOT NULL DEFAULT '{}', -- JSON: the response body; a step: discarded, the budget, the error
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL,
    finished_at     TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    deleted_at      TEXT,
    origin          TEXT    NOT NULL DEFAULT ''
);
INSERT INTO commands (id, uuid, parent_id, cube_id, user_id, token_id, idempotency_key, action, target, request, state, status, result,
                      created_at, updated_at, finished_at, version, deleted_at, origin)
SELECT id, uuid, parent_id, cube_id, user_id, token_id, idempotency_key, action, target, request, state, status, result,
       created_at, updated_at, finished_at, version, NULL, origin
FROM temp.down_rows;
DROP TABLE temp.down_rows;
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');

-- 11. tokens (L8).
CREATE TEMP TABLE down_rows AS SELECT * FROM tokens;
DROP TABLE tokens;
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
INSERT INTO tokens (id, uuid, user_id, kind, hash, label, created_at, expires_at, last_used_at, last_used_from, revoked_at, version, deleted_at, origin)
SELECT id, uuid, user_id, kind, hash, label, created_at, expires_at, last_used_at, last_used_from, deleted_at, version, NULL, origin
FROM temp.down_rows;
DROP TABLE temp.down_rows;
CREATE INDEX tokens_user ON tokens (user_id);

-- 10. cubes and cube_options (L7).
CREATE TABLE cube_options (
    cube_id       INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    proxy_virtual INTEGER NOT NULL DEFAULT 0,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id)
);
INSERT INTO cube_options (cube_id, proxy_virtual, updated_at, updated_by)
SELECT id, 1, updated_at, updated_by FROM cubes WHERE proxy_virtual = 1;
CREATE TEMP TABLE down_rows AS SELECT * FROM cubes;
DROP TABLE cubes;
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
INSERT INTO cubes (id, uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, hidden, source,
                   mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, updated_by, version, deleted_at, origin)
SELECT id, uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, hidden, source,
       mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, updated_by, version, deleted_at, origin
FROM temp.down_rows;
DROP TABLE temp.down_rows;

-- 9. roles (L6).
CREATE TEMP TABLE down_rows AS SELECT * FROM roles WHERE id NOT IN (SELECT id FROM temp.down_roles);
DROP TABLE roles;
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
INSERT INTO roles (id, key, name, builtin, disabled, created_at, updated_at, updated_by, created_by, version, deleted_at, origin)
SELECT id, key, name, builtin, disabled, created_at, updated_at, updated_by, created_by, version, deleted_at, origin FROM temp.down_rows;
DROP TABLE temp.down_rows;

-- 8. users (L3, L5).
CREATE TEMP TABLE down_rows AS SELECT * FROM users;
DROP TABLE users;
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
    origin        TEXT    NOT NULL DEFAULT '', auth_version INTEGER NOT NULL DEFAULT 1, opaque_record BLOB, opaque_ksf TEXT NOT NULL DEFAULT '',
    UNIQUE (name)
);
INSERT INTO users (id, uuid, name, display_name, password_hash, role_id, disabled, expires_at, invited_by, created_at, updated_at, updated_by,
                   last_login_at, version, deleted_at, origin, auth_version, opaque_record, opaque_ksf)
SELECT u.id, u.uuid, n.name, u.display_name, u.password_hash, u.role_id, u.disabled, u.expires_at, u.invited_by, u.created_at, u.updated_at, u.updated_by,
       u.last_login_at, u.version, u.deleted_at, u.origin, u.auth_version, u.opaque_record, u.opaque_ksf
FROM temp.down_rows u JOIN temp.down_names n ON n.user_id = u.id;
DROP TABLE temp.down_rows;

-- 7. user_identities (L2).
CREATE TEMP TABLE down_rows AS SELECT * FROM user_identities WHERE deleted_at IS NULL;
DROP TABLE user_identities;
CREATE TABLE "user_identities" (
    id               INTEGER PRIMARY KEY,
    uuid             TEXT    NOT NULL UNIQUE,
    user_id          INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id      INTEGER REFERENCES auth_providers (id) ON DELETE CASCADE,
    integration_id   INTEGER REFERENCES integrations (id) ON DELETE CASCADE,
    subject          TEXT,                             -- the provider's sub, the integration's user id, or system; NULL until a sign-in binds an e-mail
    email            TEXT    NOT NULL DEFAULT '',      -- lower case: the e-mail assigned, or the one of the sign-in that bound it
    created_at       TEXT    NOT NULL,
    created_by       INTEGER REFERENCES users (id),
    bound_at         TEXT,
    last_used_at     TEXT,
    name             TEXT    NOT NULL DEFAULT '',
    username         TEXT    NOT NULL DEFAULT '',
    groups           TEXT    NOT NULL DEFAULT '[]',
    own_user_id      INTEGER REFERENCES users (id),   -- an integration's: the user made at its first call, kept when linked
    system_generated INTEGER NOT NULL DEFAULT 0,      -- an integration's own system user (Home Assistant's Supervisor, Cloud, …)
    last_level       TEXT    NOT NULL DEFAULT '',     -- the level of its last call, for display
    CHECK ((provider_id IS NULL) <> (integration_id IS NULL))
);
INSERT INTO user_identities (id, uuid, user_id, provider_id, integration_id, subject, email, created_at, created_by, bound_at, last_used_at,
                             name, username, groups, own_user_id, system_generated, last_level)
SELECT id, uuid, effective_user_id, provider_id, integration_id, subject, email, created_at, created_by, bound_at, last_used_at,
       name, username, groups, real_user_id, system_generated, last_level
FROM temp.down_rows;
DROP TABLE temp.down_rows;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_integration ON user_identities (integration_id, subject) WHERE integration_id IS NOT NULL;
CREATE INDEX user_identities_user ON user_identities (user_id);

-- 6. integrations (L4).
CREATE TEMP TABLE down_rows AS SELECT * FROM integrations;
DROP TABLE integrations;
CREATE TABLE integrations (
    id               INTEGER PRIMARY KEY,
    uuid             TEXT    NOT NULL UNIQUE,
    kind             TEXT    NOT NULL CHECK (kind IN ('homeassistant')),
    key              TEXT    NOT NULL,                -- in user names and routes: lower case, from the name
    name             TEXT    NOT NULL DEFAULT '',     -- the installation's name (Home Assistant's location_name)
    instance_id      TEXT    NOT NULL DEFAULT '',     -- the installation id as accepted by the owner (32 hex)
    pending_instance TEXT    NOT NULL DEFAULT '',     -- an installation id seen and not accepted yet
    url              TEXT    NOT NULL DEFAULT '',     -- the installation's address, for display
    app_version      TEXT    NOT NULL DEFAULT '',     -- the installation's version, for display
    max_caps         TEXT    NOT NULL DEFAULT '[]',   -- JSON list: the key's maximum, within the role integration
    user_id          INTEGER REFERENCES users (id),   -- the integration's own user, holding its key
    system_user_id   INTEGER REFERENCES users (id),   -- the user "system": calls no user started
    state            TEXT    NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'active', 'disabled', 'refused')),
    request_code     TEXT    NOT NULL DEFAULT '',     -- the short code both sides show while pending
    request_expires  TEXT,                            -- a pending request's end
    request_remote   TEXT    NOT NULL DEFAULT '',     -- the address the request came from
    key_issued_at    TEXT,                            -- when its key was handed out
    merged_into      INTEGER REFERENCES integrations (id),  -- a request the owner attached to an integration it was before
    created_at       TEXT    NOT NULL,
    updated_at       TEXT    NOT NULL,
    updated_by       INTEGER REFERENCES users (id),
    version          INTEGER NOT NULL DEFAULT 1,
    deleted_at       TEXT,
    origin           TEXT    NOT NULL DEFAULT ''
);
INSERT INTO integrations (id, uuid, kind, key, name, instance_id, pending_instance, url, app_version, max_caps, user_id, system_user_id, state,
                          request_code, request_expires, request_remote, key_issued_at, merged_into, created_at, updated_at, updated_by, version, deleted_at, origin)
SELECT g.id, g.uuid, g.kind, k.key, g.name, g.instance_id, g.pending_instance, g.url, g.app_version, g.max_caps,
       (SELECT u.user_id FROM temp.down_users u WHERE u.integration_id = g.id AND u.kind = 'own'),
       (SELECT u.user_id FROM temp.down_users u WHERE u.integration_id = g.id AND u.kind = 'system'), g.state,
       g.request_code, g.request_expires_at, g.request_remote, g.key_issued_at, g.merged_into, g.created_at, g.updated_at, g.updated_by, g.version, g.deleted_at, g.origin
FROM temp.down_rows g JOIN temp.down_keys k ON k.integration_id = g.id;
DROP TABLE temp.down_rows;
CREATE UNIQUE INDEX integrations_key ON integrations (key) WHERE deleted_at IS NULL;

-- 4. The log and the audit (L1).
DROP INDEX log_real_user;
ALTER TABLE log DROP COLUMN real_user_id;
ALTER TABLE log RENAME COLUMN effective_user_id TO user_id;
DROP INDEX audit_real_user;
DROP INDEX audit_user;
ALTER TABLE audit DROP COLUMN real_user_id;
ALTER TABLE audit RENAME COLUMN effective_user_id TO user_id;
CREATE INDEX audit_user ON audit (user_id, time);

-- 3. backups.
CREATE TEMP TABLE down_rows AS SELECT b.*, d.data AS data FROM backups b LEFT JOIN backup_data d ON d.id = b.id;
DROP TABLE backup_data;
DROP TABLE backups;
CREATE TABLE "backups" (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,  -- never given out twice
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    taken_at   TEXT    NOT NULL,
    reason     TEXT    NOT NULL CHECK (reason IN ('auto', 'manual', 'before_restore', 'upload')),
    format     TEXT    NOT NULL CHECK (format IN ('json', 'yaml')),
    tool       TEXT    NOT NULL DEFAULT '',      -- backup.File.Tool, the version that took it
    size       INTEGER NOT NULL,                 -- bytes before compression
    sha256     TEXT    NOT NULL,                 -- of the file before compression
    body       BLOB    NOT NULL,                 -- the backup file, gzip; empty in a tombstone
    user_id    INTEGER REFERENCES users (id),    -- who asked, for manual, before_restore and upload
    note       TEXT    NOT NULL DEFAULT '',
    version    INTEGER NOT NULL DEFAULT 1,
    deleted_at TEXT,
    origin     TEXT    NOT NULL DEFAULT ''
);
INSERT INTO backups (id, uuid, cube_id, taken_at, reason, format, tool, size, sha256, body, user_id, note, version, deleted_at, origin)
SELECT id, uuid, cube_id, taken_at, reason, format, tool, size, sha256, coalesce(data, x''), user_id, note, version, deleted_at, origin
FROM temp.down_rows;
DROP TABLE temp.down_rows;
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);

-- The sequences, which the tables made again start at their highest id.
DELETE FROM sqlite_sequence WHERE name IN ('backups', 'commands');
INSERT INTO sqlite_sequence (name, seq) SELECT name, seq FROM temp.down_sequence;

-- 1. The view and its triggers, as 0014 made them.
CREATE VIEW events AS
SELECT id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data FROM log;
CREATE TRIGGER events_insert INSTEAD OF INSERT ON events
BEGIN
    INSERT INTO log (id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
    VALUES (NEW.id, NEW.uuid, NEW.cube_id, NEW.time, NEW.kind, coalesce(NEW.source, ''), coalesce(NEW.level, 0),
            coalesce(NEW.device, ''), coalesce(NEW.serial, ''), NEW.user_id, NEW.data);
END;
CREATE TRIGGER events_update INSTEAD OF UPDATE ON events
BEGIN
    UPDATE log SET uuid = NEW.uuid, cube_id = NEW.cube_id, time = NEW.time, kind = NEW.kind, source = NEW.source,
                   level = NEW.level, device = NEW.device, serial = NEW.serial, user_id = NEW.user_id, data = NEW.data
    WHERE id = OLD.id;
END;
CREATE TRIGGER events_delete INSTEAD OF DELETE ON events
BEGIN
    DELETE FROM log WHERE id = OLD.id;
END;

DROP TABLE temp.down_keys;
DROP TABLE temp.down_users;
DROP TABLE temp.down_names;
DROP TABLE temp.down_roles;
DROP TABLE temp.down_cubes;
DROP TABLE temp.down_sequence;
