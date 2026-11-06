-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Stage (d) of docs/store.md: the indexes the log's pages walk. A page
-- is a Cube's events newest first by id, of a kind or a device where
-- chosen; SQLite keeps the rowid last in every index, so each of these
-- is in id order within its terms, and a page reads its rows from the
-- end instead of sorting all of the Cube's. events_cube_time stays for
-- the prune and for a page before a time. Additive: a binary built for
-- an earlier version reads the file as before.
CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
DROP INDEX events_device_time;
CREATE INDEX events_cube_device ON events (cube_id, device);
