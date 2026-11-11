-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- A backup's id is never given out again (AP BackupIds v1.0, Block 1;
-- docs/store.md section 9 (e)). Without AUTOINCREMENT SQLite gives a new
-- row the highest id plus one, so once the prune removed the newest
-- backup, past backups.keep_newest or as a purged tombstone, the next
-- backup took its id, and an id from a list loaded before named another
-- backup. With it SQLite keeps the highest id ever used in
-- sqlite_sequence. SQLite cannot add AUTOINCREMENT to a table, so the
-- table is made again as 0005 made it: a new table, every row copied
-- with its id and uuid, the old one dropped, the new one renamed, the
-- index made; no table references backups. The copy starts the sequence
-- at the highest id left, and the rename carries it. Additive, so
-- min_reader stays 1: the columns and their meaning are as before, and
-- how a new id is chosen is in the table's schema, which a binary built
-- for an earlier version follows as well.
CREATE TABLE backups_new (
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
INSERT INTO backups_new (id, uuid, cube_id, taken_at, reason, format, tool, size, sha256, body, user_id, note, version, deleted_at, origin)
SELECT id, uuid, cube_id, taken_at, reason, format, tool, size, sha256, body, user_id, note, version, deleted_at, origin
FROM backups;
DROP TABLE backups;
ALTER TABLE backups_new RENAME TO backups;
CREATE INDEX backups_cube_time ON backups (cube_id, taken_at);
