-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Stage (e) of docs/store.md: the backups' reasons are those of the
-- backup history as built, auto (taken by the server after a change of
-- the Cube's configuration), manual, before_restore and upload, in place
-- of connect and daily, which belonged to the daily backups the owner
-- replaced. SQLite cannot change a CHECK, so the table is made again; no
-- binary wrote it before this one, and a row that has one of the old
-- reasons is copied as auto all the same. Additive for a reader: an older
-- binary does not read the table.
CREATE TABLE backups_new (
    id         INTEGER PRIMARY KEY,
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
INSERT INTO backups_new (id, uuid, cube_id, taken_at, reason, format, tool, size, sha256, body, user_id, note, version, deleted_at, origin)
SELECT id, uuid, cube_id, taken_at, CASE WHEN reason IN ('connect', 'daily') THEN 'auto' ELSE reason END,
       format, tool, size, sha256, body, user_id, note, version, deleted_at, origin
FROM backups;
DROP TABLE backups;
ALTER TABLE backups_new RENAME TO backups;
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
