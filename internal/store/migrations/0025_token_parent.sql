-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 24
--
-- A session names the key that made it (AP MobileApp v1.1, Block 8,
-- D31-A; docs/store.md section 6, "A session from a key"):
-- tokens.parent_id is the row of the API key POST /api/v1/login/key made
-- the session from, NULL for a session of a sign-in and for a key, as
-- commands.parent_id names a step's request. Revoking a key revokes its
-- sessions, and a key's next session ends its earlier one. Additive: a
-- binary for 24 reads the file, writes its tokens without the column,
-- and makes no session from a key; one that revokes a key leaves its
-- sessions, which this binary then refuses, since a session counts only
-- while its key does (TokenByHash). No foreign key, so that the down
-- part can drop the column.
ALTER TABLE tokens ADD COLUMN parent_id INTEGER;
CREATE INDEX tokens_parent ON tokens (parent_id) WHERE parent_id IS NOT NULL;
