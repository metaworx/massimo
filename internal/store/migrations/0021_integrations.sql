-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
-- foreign_keys: off
--
-- Integrations that act for their own users (AP IntegrationUsers v1.0,
-- Block 1; docs/store.md section 6, "Integrations"). A registered
-- integration, Home Assistant first, has its own user, which holds its
-- API key, a user "system" for the calls no user started, and a user per
-- user of its own it acted for, each an identity of user_identities whose
-- subject is that user's id at the integration.
--
-- A breaking change, by the owner's word (D10-C): user_identities is made
-- again with provider_id nullable beside integration_id, exactly one of
-- them set. min_reader stays 1. The table exists since 0.15.0-alpha.1
-- (OIDC); a store of those releases cannot be opened by them again once
-- migrated. Older releases never had the table.
--
-- The role integration becomes fixed, like owner: its set is the ceiling
-- of every call through an integration (internal/server/auth.go,
-- integrationCeiling). Its users so far, API keys made for an integration
-- by hand, move to the new role integration-v0, "integration (before
-- 1.0)", with exactly the set they had (D11-A), so that nothing widens.

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
    key_issued_at    TEXT,                            -- when its key was handed out, once
    created_at       TEXT    NOT NULL,
    updated_at       TEXT    NOT NULL,
    updated_by       INTEGER REFERENCES users (id),
    version          INTEGER NOT NULL DEFAULT 1,
    deleted_at       TEXT,
    origin           TEXT    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX integrations_key ON integrations (key) WHERE deleted_at IS NULL;

CREATE TABLE user_identities_new (
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
INSERT INTO user_identities_new (id, uuid, user_id, provider_id, subject, email, created_at, created_by, bound_at, last_used_at, name, username, groups)
SELECT id, uuid, user_id, provider_id, subject, email, created_at, created_by, bound_at, last_used_at, name, username, groups FROM user_identities;
DROP TABLE user_identities;
ALTER TABLE user_identities_new RENAME TO user_identities;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL AND provider_id IS NOT NULL;
CREATE UNIQUE INDEX user_identities_integration ON user_identities (integration_id, subject) WHERE integration_id IS NOT NULL;
CREATE INDEX user_identities_user ON user_identities (user_id);

-- D11-A: the role integration's users so far keep exactly their set.
INSERT INTO roles (key, name, builtin, created_at, updated_at)
SELECT 'integration-v0', 'integration (before 1.0)', 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE EXISTS (SELECT 1 FROM users u JOIN roles r ON r.id = u.role_id WHERE r.key = 'integration');
INSERT INTO role_capabilities (role_id, capability)
SELECT n.id, c.capability FROM roles n, roles r JOIN role_capabilities c ON c.role_id = r.id
WHERE n.key = 'integration-v0' AND r.key = 'integration';
UPDATE users SET role_id = (SELECT id FROM roles WHERE key = 'integration-v0'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), version = version + 1
WHERE role_id = (SELECT id FROM roles WHERE key = 'integration') AND EXISTS (SELECT 1 FROM roles WHERE key = 'integration-v0');

-- The role integration: fixed at the ceiling, enabled, its default name.
UPDATE roles SET disabled = 0, name = '', builtin = 1, deleted_at = NULL, version = version + 1,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE key = 'integration';
DELETE FROM role_capabilities WHERE role_id = (SELECT id FROM roles WHERE key = 'integration');
INSERT INTO role_capabilities (role_id, capability)
SELECT r.id, c.capability FROM roles r JOIN (
              SELECT 'read' AS capability
    UNION ALL SELECT 'preferences'
    UNION ALL SELECT 'setpoint'
    UNION ALL SELECT 'lock'
    UNION ALL SELECT 'configure'
    UNION ALL SELECT 'rename'
    UNION ALL SELECT 'structure'
    UNION ALL SELECT 'pair'
    UNION ALL SELECT 'discover'
    UNION ALL SELECT 'backup'
    UNION ALL SELECT 'restore'
    UNION ALL SELECT 'guests'
    UNION ALL SELECT 'cubes'
    UNION ALL SELECT 'cubes.reboot'
    UNION ALL SELECT 'settings'
    UNION ALL SELECT 'update'
    UNION ALL SELECT 'restart'
    UNION ALL SELECT 'reboot'
    UNION ALL SELECT 'audit'
) c WHERE r.key = 'integration';

-- The capability that manages integrations: the manager's, never an
-- integration's.
INSERT INTO role_capabilities (role_id, capability)
SELECT id, 'integrations' FROM roles WHERE key = 'manager'
AND NOT EXISTS (SELECT 1 FROM role_capabilities c WHERE c.role_id = roles.id AND c.capability = 'integrations');
