-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 24
-- foreign_keys: off
--
-- The store's first min_reader raise (AP StoreRaise v1.2, Block 3;
-- docs/store.md section 5, "Migrations"): what was kept only for older
-- binaries goes, the schema is tidied, and an integration's users get
-- names and columns that need no fixed key. A binary built for an earlier
-- version cannot read the result: it refuses the file, and massimo store
-- downgrade -to <its version>, run with this binary or a later one,
-- takes the file back by 0024_raise.down.sql, whose header names what
-- that loses. The runner applies this file with foreign keys off and
-- checks them before the commit; a table is made again as <name>_new,
-- copied, dropped and renamed, never renamed aside first, since SQLite
-- rewrites the references of the tables that point at a table renamed.
--
-- In this order:
--
-- 1. The view events and its INSTEAD OF triggers (0014), which only a
--    binary before 0014 used.
-- 2. fillSources' UPDATE, once: the rows a binary before 0009 wrote get
--    their source, level and serial, as 0009 gave them.
-- 3. backup_data: the backups' files in a table of their own (D5-B), so
--    that a list of backups no longer walks their overflow pages; a
--    tombstone has no row there.
-- 4. The log and the audit (D2-A): user_id becomes effective_user_id,
--    whose rights a row was made with, and real_user_id is who made it,
--    filled for every row with a user: the same user, but for an audit
--    row of a call through an integration (detail.via and detail.for.id),
--    whose real user is the integration user's own. Altered, never made
--    again: the UPDATE reads the rows with a user through log_user.
-- 5. An integration's users (D3-A, D8-C): users.integration_id, and the
--    names <integrations.uuid>:, <uuid>:system and <uuid>:<subject>; the
--    display names stay.
-- 6. integrations made again without key, user_id and system_user_id, and
--    with request_expires_at (D9-A).
-- 7. user_identities made again (D2-A, D6-B, D9-A): effective_user_id
--    and real_user_id, the columns of a replicated row, a CHECK on
--    last_level, and partial uniques that ignore removed rows.
-- 8. users made again (D10-A): the name unique among the users that are
--    not removed, a removed user's -del-YYYYMMDD-HHNN[-<id>] stripped.
-- 9. roles made again (D6-B, D10-A): a uuid, written by the store right
--    after this file (afterMigration, since SQL makes no UUIDv7), and the
--    key unique among the roles that are not removed.
-- 10. cubes made again (D11-A): the serial unique among the Cubes that
--    are not removed, and proxy_virtual from cube_options, which goes; a
--    removed row's serial loses the ~<id> the down part gave it (D15-C).
-- 11. tokens (D7): deleted_at, never written, dropped, and revoked_at
--    renamed deleted_at, every table's tombstone.
-- 12. The rest of D9-A: commands.deleted_at and commands_cube_time,
--    never used, dropped; auth_providers.auto_domains a JSON array.
--
-- Names inside old rows' JSON (log.data, the audit's via) are history,
-- kept as written.

-- 1. The view.
DROP TRIGGER events_insert;
DROP TRIGGER events_update;
DROP TRIGGER events_delete;
DROP VIEW events;

-- 2. fillSources' UPDATE.
UPDATE log SET
    source = CASE
        WHEN coalesce(json_extract(data, '$.source'), '') != '' THEN json_extract(data, '$.source')
        WHEN kind IN ('restore_step', 'restore_done') THEN 'system'
        WHEN device = '' THEN 'cube'
        ELSE 'device' END,
    level = CASE kind WHEN 'disconnected' THEN 4 WHEN 'hello' THEN -4 ELSE 0 END,
    serial = coalesce(nullif(json_extract(data, '$.serial'), ''), nullif(json_extract(data, '$.state.serial'), ''), '')
WHERE source = '';

-- 3. backup_data.
CREATE TABLE backup_data (
    id   INTEGER PRIMARY KEY REFERENCES backups (id) ON DELETE CASCADE,  -- the backup's
    data BLOB    NOT NULL                                                -- the backup file, gzip
);
INSERT INTO backup_data (id, data) SELECT id, body FROM backups WHERE deleted_at IS NULL;
ALTER TABLE backups DROP COLUMN body;

-- 4. The log and the audit.
ALTER TABLE log RENAME COLUMN user_id TO effective_user_id;
ALTER TABLE log ADD COLUMN real_user_id INTEGER REFERENCES users (id);
UPDATE log SET real_user_id = effective_user_id WHERE effective_user_id IS NOT NULL;
CREATE INDEX log_real_user ON log (real_user_id) WHERE real_user_id IS NOT NULL;
ALTER TABLE audit RENAME COLUMN user_id TO effective_user_id;
ALTER TABLE audit ADD COLUMN real_user_id INTEGER REFERENCES users (id);
-- The integration and its user are read from detail in the SET, never
-- in a WHERE: detail is a content column (rule_test.go).
UPDATE audit SET real_user_id = CASE
    WHEN json_extract(detail, '$.via') IS NULL THEN effective_user_id
    ELSE coalesce(
        (SELECT i.own_user_id FROM integrations g
         JOIN user_identities i ON i.integration_id = g.id AND g.deleted_at IS NULL
          AND g.key = json_extract(audit.detail, '$.via') AND i.subject = json_extract(audit.detail, '$.for.id')),
        effective_user_id) END
WHERE effective_user_id IS NOT NULL;
DROP INDEX audit_user;
CREATE INDEX audit_user ON audit (effective_user_id, time) WHERE effective_user_id IS NOT NULL;
CREATE INDEX audit_real_user ON audit (real_user_id, time) WHERE real_user_id IS NOT NULL;

-- 5. An integration's users: their integration and their names, kept
-- here until users is made again.
CREATE TEMP TABLE raise_users (
    user_id        INTEGER PRIMARY KEY,
    integration_id INTEGER NOT NULL,
    name           TEXT    NOT NULL
);
INSERT OR IGNORE INTO temp.raise_users (user_id, integration_id, name)
SELECT user_id, id, uuid || ':' FROM integrations WHERE user_id IS NOT NULL;
INSERT OR IGNORE INTO temp.raise_users (user_id, integration_id, name)
SELECT system_user_id, id, uuid || ':system' FROM integrations WHERE system_user_id IS NOT NULL;
INSERT OR IGNORE INTO temp.raise_users (user_id, integration_id, name)
SELECT i.own_user_id, i.integration_id, g.uuid || ':' || lower(i.subject)
FROM user_identities i JOIN integrations g ON g.id = i.integration_id
WHERE i.own_user_id IS NOT NULL AND i.subject IS NOT NULL;

-- 6. integrations.
CREATE TABLE integrations_new (
    id                 INTEGER PRIMARY KEY,
    uuid               TEXT    NOT NULL UNIQUE,         -- its users' names begin with it
    kind               TEXT    NOT NULL CHECK (kind IN ('homeassistant')),
    name               TEXT    NOT NULL DEFAULT '',     -- the installation's name as reported (Home Assistant's location_name)
    instance_id        TEXT    NOT NULL DEFAULT '',     -- the installation id as accepted by the owner (32 hex)
    pending_instance   TEXT    NOT NULL DEFAULT '',     -- an installation id seen and not accepted yet
    url                TEXT    NOT NULL DEFAULT '',     -- the installation's address, for display
    app_version        TEXT    NOT NULL DEFAULT '',     -- the installation's version, for display
    max_caps           TEXT    NOT NULL DEFAULT '[]',   -- JSON list: the key's maximum, within the role integration
    state              TEXT    NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'active', 'disabled', 'refused')),
    request_code       TEXT    NOT NULL DEFAULT '',     -- the short code both sides show while pending
    request_expires_at TEXT,                            -- a pending request's end
    request_remote     TEXT    NOT NULL DEFAULT '',     -- the address the request came from
    key_issued_at      TEXT,                            -- when its key was handed out
    merged_into        INTEGER REFERENCES integrations (id),  -- a request the owner attached to an integration it was before
    created_at         TEXT    NOT NULL,
    updated_at         TEXT    NOT NULL,
    updated_by         INTEGER REFERENCES users (id),
    version            INTEGER NOT NULL DEFAULT 1,
    deleted_at         TEXT,
    origin             TEXT    NOT NULL DEFAULT ''
);
INSERT INTO integrations_new (id, uuid, kind, name, instance_id, pending_instance, url, app_version, max_caps, state, request_code,
                              request_expires_at, request_remote, key_issued_at, merged_into, created_at, updated_at, updated_by, version, deleted_at, origin)
SELECT id, uuid, kind, name, instance_id, pending_instance, url, app_version, max_caps, state, request_code,
       request_expires, request_remote, key_issued_at, merged_into, created_at, updated_at, updated_by, version, deleted_at, origin
FROM integrations;
DROP TABLE integrations;
ALTER TABLE integrations_new RENAME TO integrations;

-- 7. user_identities.
CREATE TABLE user_identities_new (
    id                INTEGER PRIMARY KEY,
    uuid              TEXT    NOT NULL UNIQUE,
    effective_user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,  -- the user it acts as: its own, or the one it is linked to
    provider_id       INTEGER REFERENCES auth_providers (id) ON DELETE CASCADE,
    integration_id    INTEGER REFERENCES integrations (id) ON DELETE CASCADE,
    subject           TEXT,                             -- the provider's sub, the integration's user id, or system; NULL until a sign-in binds an e-mail
    email             TEXT    NOT NULL DEFAULT '',      -- lower case: the e-mail assigned, or the one of the sign-in that bound it
    created_at        TEXT    NOT NULL,
    created_by        INTEGER REFERENCES users (id),
    bound_at          TEXT,
    last_used_at      TEXT,
    name              TEXT    NOT NULL DEFAULT '',
    username          TEXT    NOT NULL DEFAULT '',
    groups            TEXT    NOT NULL DEFAULT '[]',
    real_user_id      INTEGER REFERENCES users (id),   -- an integration's: the user made at its first call, kept when linked
    system_generated  INTEGER NOT NULL DEFAULT 0,      -- an integration's own system user (Home Assistant's Supervisor, Cloud, …)
    last_level        TEXT    NOT NULL DEFAULT '' CHECK (last_level IN ('', 'view', 'control', 'admin')),  -- the level of its last call, for display
    updated_at        TEXT    NOT NULL,
    version           INTEGER NOT NULL DEFAULT 1,
    deleted_at        TEXT,
    origin            TEXT    NOT NULL DEFAULT '',
    CHECK ((provider_id IS NULL) <> (integration_id IS NULL))
);
INSERT INTO user_identities_new (id, uuid, effective_user_id, provider_id, integration_id, subject, email, created_at, created_by, bound_at, last_used_at,
                                 name, username, groups, real_user_id, system_generated, last_level, updated_at)
SELECT id, uuid, user_id, provider_id, integration_id, subject, email, created_at, created_by, bound_at, last_used_at,
       name, username, groups, own_user_id, system_generated,
       CASE WHEN last_level IN ('', 'view', 'control', 'admin') THEN last_level ELSE '' END,
       coalesce(last_used_at, bound_at, created_at)
FROM user_identities;
DROP TABLE user_identities;
ALTER TABLE user_identities_new RENAME TO user_identities;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL AND provider_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL AND provider_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX user_identities_integration ON user_identities (integration_id, subject) WHERE integration_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX user_identities_user ON user_identities (effective_user_id);
CREATE INDEX user_identities_real_user ON user_identities (real_user_id) WHERE real_user_id IS NOT NULL;

-- 8. users.
CREATE TABLE users_new (
    id             INTEGER PRIMARY KEY,
    uuid           TEXT    NOT NULL UNIQUE,
    name           TEXT    NOT NULL,              -- the login name, lower case; an integration's users: <integrations.uuid>:[<subject>]
    display_name   TEXT    NOT NULL DEFAULT '',
    password_hash  TEXT    NOT NULL DEFAULT '',   -- argon2id in PHC string form; '' = no password login (keys only)
    opaque_record  BLOB,                          -- the OPAQUE registration record (auth_version 2)
    opaque_ksf     TEXT    NOT NULL DEFAULT '',   -- its key stretching
    auth_version   INTEGER NOT NULL DEFAULT 1,    -- 1: the server hashes the password; 2: OPAQUE
    role_id        INTEGER NOT NULL REFERENCES roles (id),
    integration_id INTEGER REFERENCES integrations (id),  -- the integration it is a user of; NULL for every other user
    disabled       INTEGER NOT NULL DEFAULT 0,
    expires_at     TEXT,                          -- guests: disabled from this time on
    invited_by     INTEGER REFERENCES users (id),
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL,
    updated_by     INTEGER REFERENCES users (id),
    last_login_at  TEXT,
    version        INTEGER NOT NULL DEFAULT 1,
    deleted_at     TEXT,
    origin         TEXT    NOT NULL DEFAULT ''
);
INSERT INTO users_new (id, uuid, name, display_name, password_hash, opaque_record, opaque_ksf, auth_version, role_id, integration_id, disabled,
                       expires_at, invited_by, created_at, updated_at, updated_by, last_login_at, version, deleted_at, origin)
SELECT u.id, u.uuid,
       CASE
           WHEN n.name IS NOT NULL THEN n.name
           WHEN u.deleted_at IS NOT NULL AND length(u.name) > length(d.suffix) AND substr(u.name, -length(d.suffix)) = d.suffix
               THEN substr(u.name, 1, length(u.name) - length(d.suffix))
           WHEN u.deleted_at IS NOT NULL AND length(u.name) > length(d.suffix || '-' || u.id) AND substr(u.name, -length(d.suffix || '-' || u.id)) = d.suffix || '-' || u.id
               THEN substr(u.name, 1, length(u.name) - length(d.suffix || '-' || u.id))
           ELSE u.name END,
       u.display_name, u.password_hash, u.opaque_record, u.opaque_ksf, u.auth_version, u.role_id, n.integration_id, u.disabled,
       u.expires_at, u.invited_by, u.created_at, u.updated_at, u.updated_by, u.last_login_at, u.version, u.deleted_at, u.origin
FROM users u LEFT JOIN temp.raise_users n ON n.user_id = u.id
    JOIN (SELECT id, '-del-' || coalesce(strftime('%Y%m%d-%H%M', deleted_at), '') AS suffix FROM users) d ON d.id = u.id;
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;
CREATE UNIQUE INDEX users_name ON users (name) WHERE deleted_at IS NULL;
CREATE INDEX users_integration ON users (integration_id) WHERE integration_id IS NOT NULL;
DROP TABLE temp.raise_users;

-- 9. roles; the uuids are written right after this file.
CREATE TABLE roles_new (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
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
    origin     TEXT    NOT NULL DEFAULT ''    -- node.id of the node that made the last change
);
INSERT INTO roles_new (id, uuid, key, name, builtin, disabled, created_at, updated_at, updated_by, created_by, version, deleted_at, origin)
SELECT id, 'raise-' || id, key, name, builtin, disabled, created_at, updated_at, updated_by, created_by, version, deleted_at, origin FROM roles;
DROP TABLE roles;
ALTER TABLE roles_new RENAME TO roles;
CREATE UNIQUE INDEX roles_key ON roles (key) WHERE deleted_at IS NULL;

-- 10. cubes.
CREATE TABLE cubes_new (
    id                 INTEGER PRIMARY KEY,
    uuid               TEXT    NOT NULL UNIQUE,
    serial             TEXT    NOT NULL,              -- upper case, as printed on the Cube
    configured         INTEGER NOT NULL DEFAULT 1,    -- 0: found by discovery, or removed; not served
    name               TEXT    NOT NULL DEFAULT '',
    address            TEXT    NOT NULL DEFAULT '',   -- host[:port]; '' with follow_discovery: from discovery
    follow_discovery   INTEGER NOT NULL DEFAULT 0,    -- the address follows the Cube's discovery answers
    proxy_address      TEXT    NOT NULL DEFAULT '',   -- the server's listener for this Cube, host[:port]; '' = :62910 with a single Cube, none with several
    proxy_serial       TEXT    NOT NULL DEFAULT '',   -- the serial used in discovery replies; '' = the Cube's
    proxy_virtual      INTEGER NOT NULL DEFAULT 0,    -- the proxy shows the rooms' virtual wall thermostats (docs/proxy.md)
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
    origin             TEXT    NOT NULL DEFAULT ''
);
INSERT INTO cubes_new (id, uuid, serial, configured, name, address, follow_discovery, proxy_address, proxy_serial, proxy_virtual, hidden, source,
                       mac, rf_address, firmware, discovered_address, first_seen, last_seen, created_at, updated_at, updated_by, version, deleted_at, origin)
SELECT c.id, c.uuid,
       CASE WHEN c.deleted_at IS NOT NULL AND length(c.serial) > length('~' || c.id) AND substr(c.serial, -length('~' || c.id)) = '~' || c.id
            THEN substr(c.serial, 1, length(c.serial) - length('~' || c.id)) ELSE c.serial END,
       c.configured, c.name, c.address, c.follow_discovery, c.proxy_address, c.proxy_serial, coalesce(o.proxy_virtual, 0), c.hidden, c.source,
       c.mac, c.rf_address, c.firmware, c.discovered_address, c.first_seen, c.last_seen, c.created_at, c.updated_at, c.updated_by, c.version, c.deleted_at, c.origin
FROM cubes c LEFT JOIN cube_options o ON o.cube_id = c.id;
DROP TABLE cube_options;
DROP TABLE cubes;
ALTER TABLE cubes_new RENAME TO cubes;
CREATE UNIQUE INDEX cubes_serial ON cubes (serial) WHERE deleted_at IS NULL;

-- 11. tokens.
ALTER TABLE tokens DROP COLUMN deleted_at;
ALTER TABLE tokens RENAME COLUMN revoked_at TO deleted_at;

-- 12. The rest of D9-A.
DROP INDEX commands_cube_time;
ALTER TABLE commands DROP COLUMN deleted_at;
CREATE TABLE auth_providers_new (
    id             INTEGER PRIMARY KEY,
    uuid           TEXT    NOT NULL UNIQUE,
    key            TEXT    NOT NULL,                -- in the routes, as a role's key
    name           TEXT    NOT NULL DEFAULT '',     -- on the sign-in button; '' shows the key
    issuer         TEXT    NOT NULL,                -- https://…, its discovery under /.well-known/openid-configuration
    client_id      TEXT    NOT NULL,
    client_secret  TEXT    NOT NULL DEFAULT '',
    scopes         TEXT    NOT NULL DEFAULT 'openid email profile',
    auto_create    INTEGER NOT NULL DEFAULT 0,      -- 1: a sign-in without an identity creates the user
    auto_role_id   INTEGER REFERENCES roles (id),   -- the role of a user created so; NULL: viewer
    auto_domains   TEXT    NOT NULL DEFAULT '[]',   -- JSON list: the e-mail domains it creates users for; []: any
    groups_claim   TEXT    NOT NULL DEFAULT 'groups',
    allowed_groups TEXT    NOT NULL DEFAULT '[]',
    enabled        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL,
    updated_by     INTEGER REFERENCES users (id),
    version        INTEGER NOT NULL DEFAULT 1,
    deleted_at     TEXT,
    origin         TEXT    NOT NULL DEFAULT ''
);
INSERT INTO auth_providers_new (id, uuid, key, name, issuer, client_id, client_secret, scopes, auto_create, auto_role_id, auto_domains,
                                groups_claim, allowed_groups, enabled, created_at, updated_at, updated_by, version, deleted_at, origin)
SELECT p.id, p.uuid, p.key, p.name, p.issuer, p.client_id, p.client_secret, p.scopes, p.auto_create, p.auto_role_id,
       (WITH RECURSIVE part(rest, one) AS (
            SELECT p.auto_domains || ',', NULL
            UNION ALL
            SELECT substr(rest, instr(rest, ',') + 1), trim(substr(rest, 1, instr(rest, ',') - 1)) FROM part WHERE rest <> '')
        SELECT json_group_array(one) FROM part WHERE one IS NOT NULL AND one <> ''),
       p.groups_claim, p.allowed_groups, p.enabled, p.created_at, p.updated_at, p.updated_by, p.version, p.deleted_at, p.origin
FROM auth_providers p;
DROP TABLE auth_providers;
ALTER TABLE auth_providers_new RENAME TO auth_providers;
CREATE UNIQUE INDEX auth_providers_key ON auth_providers (key) WHERE deleted_at IS NULL;
