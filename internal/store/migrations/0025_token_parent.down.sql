-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0025_token_parent.sql (AP MobileApp v1.1, Block 8;
-- docs/store.md section 5, "Migrations"): massimo store downgrade runs
-- it to take a file back to schema 24. It loses what 0025 added, and
-- only that: which key made a session. A session made from a key lives
-- on as a session of a sign-in does, and revoking its key no longer ends
-- it; it is listed and revoked under its user's tokens.
DROP INDEX tokens_parent;
ALTER TABLE tokens DROP COLUMN parent_id;
