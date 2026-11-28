-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0101_token_maker.sql (AP KeyLife v1.0, Block 2;
-- docs/store.md section 5, "Migrations"): massimo store downgrade runs
-- it to take a file back to schema 100. It loses what 0101 added, and
-- only that: which key made a key. A key made by a key lives on as one
-- made by a session does, and revoking its maker no longer revokes it;
-- it is listed and revoked under its user's tokens.
DROP INDEX tokens_made_by;
ALTER TABLE tokens DROP COLUMN made_by;
