-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0101_token_idle.sql (AP KeyLife v1.0, Block 3;
-- docs/store.md section 5, "Migrations"): massimo store downgrade runs
-- it to take a file back to schema 100. It loses what 0101 added, and
-- only that: which keys were kept from the idle limit. A binary of 100
-- knows no idle limit, so no key ends by it there; a binary of 101 or
-- later that opens the file again holds every key to the limit.
ALTER TABLE tokens DROP COLUMN idle_exempt;
