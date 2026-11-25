-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 24
--
-- A Cube's automatic backup keeps its windows across a start of the
-- server (AP BackupWatch v1.0, Block 1, D1-C; docs/store.md section 9
-- (e)): backup_pending is the watch of a Cube whose configuration
-- differs from its reference, the newest backup or the file restored,
-- written at the look that finds it changed and deleted once a backup
-- is stored or the Cube is as its reference again. A table of its own,
-- so that a Cube row's version and updated_at keep meaning a change of
-- its configuration in the store. ref says which reference the windows
-- count against: a start whose newest backup has another hash takes the
-- row as left over and deletes it. Not replicated: the watch is this
-- server's, so the row has no uuid, version, deleted_at or origin, and
-- is deleted rather than marked. Additive: a binary for 24 or 25 reads
-- the file and leaves the table alone, its watch in memory as before; a
-- row it leaves behind is told by ref at the next start of this one.
CREATE TABLE backup_pending (
    cube_id      INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    ref          TEXT    NOT NULL,   -- backup.ConfigHash of the reference the windows count against
    hash         TEXT    NOT NULL,   -- backup.ConfigHash of the configuration at the last look
    first_change TEXT    NOT NULL,   -- the first change against ref: backups.auto_max counts from it
    last_change  TEXT    NOT NULL    -- the last change seen: backups.auto_min counts from it
);
