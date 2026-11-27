-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 24
--
-- The catalogue of the server's own backups (AP ServerBackup v1.0, Block
-- 2, D11-A, D12-A, D13-A; docs/store.md section 5 and "Server
-- backups"), the first migration of main's block of 100 after the
-- freeze of 0.17 ("Two lines of releases"):
--
--   server_backup_keys     the recipients a backup is encrypted for, each
--                          with its label; a key "Make a key" made waits
--                          for its confirmation with the hash of its
--                          identity's last six characters, never the
--                          identity itself, and goes after an hour;
--   server_backup_targets  where the copies go: a kind, its fields as
--                          JSON, its secret sealed with its uuid as the
--                          additional data (secret.go), what it takes and
--                          its retention as JSON;
--   server_backups         one row a backup, its uuid the manifest's, the
--                          same file on every target; a backup that
--                          failed is a row too, with its retries;
--   server_backup_copies   a backup's copy on a target, with its state
--                          and its retries.
--
-- The catalogue is this server's: no row is replicated, so the rows have
-- no version, deleted_at or origin, and are deleted rather than marked.
-- A restore never replaces it (AP ServerBackup, the analysis's scopes).
--
-- The capability backup.retry, which shows the backups' state and
-- retries a failed backup or copy (D13-A), is seeded for manager, as
-- 0022 seeds repair; backup.server, which makes, pushes and pulls, is
-- seeded for no role; neither is the role integration's (D15-A). owner
-- has every capability by code. Additive: a binary for 24 to 26 reads
-- the file, leaves the tables alone and ignores the two capabilities.
CREATE TABLE server_backup_keys (
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    label        TEXT    NOT NULL DEFAULT '',       -- the owner's name for it, in plain in every manifest
    recipient    TEXT    NOT NULL,                  -- the public key: age1pq1…, age1… or ssh-ed25519 …
    fingerprint  TEXT    NOT NULL UNIQUE,           -- SHA256:… (serverbackup.ParseRecipient)
    kind         TEXT    NOT NULL CHECK (kind IN ('pq', 'x25519', 'ssh-ed25519')),
    confirm_hash TEXT    NOT NULL DEFAULT '',       -- a key made and not confirmed: the hash of its identity's last six characters; '' a key
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE server_backup_targets (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    name       TEXT    NOT NULL,                     -- the owner's, unique without regard to case; store push names it
    kind       TEXT    NOT NULL,                     -- serverbackup.Kind's name: local, webdav, sftp, smb, s3
    config     TEXT    NOT NULL DEFAULT '{}',        -- JSON object: the kind's fields but its secret
    secret     TEXT    NOT NULL DEFAULT '',          -- the kind's secret field, sealed with the uuid as additional data
    takes      TEXT    NOT NULL DEFAULT 'every' CHECK (takes IN ('every', 'daily', 'weekly', 'monthly')),
    retention  TEXT    NOT NULL DEFAULT '{}',        -- JSON: serverbackup.Policy, the last N and the newest of D days, W weeks, M months, Y years
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    updated_by INTEGER REFERENCES users (id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX server_backup_targets_name ON server_backup_targets (name COLLATE NOCASE);

CREATE TABLE server_backups (
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,            -- the manifest's uuid
    time         TEXT    NOT NULL,                   -- the manifest's created; a failed one's last try
    trigger      TEXT    NOT NULL CHECK (trigger IN ('scheduled', 'manual', 'before-update', 'before-restore', 'uploaded')),
    user_id      INTEGER REFERENCES users (id) ON DELETE SET NULL,  -- who asked; NULL: the schedule, a scan, the server
    state        TEXT    NOT NULL DEFAULT 'made' CHECK (state IN ('made', 'failed')),
    size         INTEGER NOT NULL DEFAULT 0,         -- the file's bytes
    sha256       TEXT    NOT NULL DEFAULT '',        -- the file's, hex
    manifest     TEXT    NOT NULL DEFAULT '',        -- manifest.json as signed; '' for a failed one
    signature    TEXT    NOT NULL DEFAULT '',        -- manifest.sig, its signature in base64, as the file holds it
    pinned_until TEXT,                               -- kept by every retention until then; NULL: not pinned
    error        TEXT    NOT NULL DEFAULT '',        -- a failed one's last error
    tries        INTEGER NOT NULL DEFAULT 0,         -- a failed one's tries
    next_try     TEXT                                -- a failed one's next retry; NULL: none (made, or the retries spent)
);

CREATE INDEX server_backups_time ON server_backups (time);

CREATE TABLE server_backup_copies (
    backup_id  INTEGER NOT NULL REFERENCES server_backups (id) ON DELETE CASCADE,
    target_id  INTEGER NOT NULL REFERENCES server_backup_targets (id) ON DELETE CASCADE,
    path       TEXT    NOT NULL DEFAULT '',          -- the file's name on the target
    state      TEXT    NOT NULL CHECK (state IN ('there', 'failed', 'missing')),
    pulled     INTEGER NOT NULL DEFAULT 0,           -- 1: pulled to this target by hand, kept until deleted (D12-A)
    pushed_at  TEXT,                                 -- when it was put there; NULL: never
    checked_at TEXT,                                 -- when its size and hash were last read back
    error      TEXT    NOT NULL DEFAULT '',
    tries      INTEGER NOT NULL DEFAULT 0,           -- a failed copy's tries
    next_try   TEXT,                                 -- a failed copy's next retry; NULL: none
    PRIMARY KEY (backup_id, target_id)
);

CREATE INDEX server_backup_copies_target ON server_backup_copies (target_id);

INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT id, 'backup.retry' FROM roles WHERE key = 'manager' AND deleted_at IS NULL;
