-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0100_token_revoke.sql (AP MobileApp v1.6, Block 21;
-- docs/store.md section 5, "Migrations"): massimo store downgrade runs
-- it to take a file back to schema 26. It loses what 0100 added, and
-- only that: each token's revoke code. A code a client still posts then
-- ends nothing, and the client falls back to ending its token at its
-- next sign-in.
DROP INDEX tokens_revoke;
ALTER TABLE tokens DROP COLUMN revoke_hash;
