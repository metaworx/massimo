-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0026_backup_pending.sql (AP BackupWatch v1.0, Block
-- 1; docs/store.md section 5, "Migrations"): massimo store downgrade
-- runs it to take a file back to schema 25. It loses what 0026 added,
-- and only that: the windows of a Cube's pending automatic backup. The
-- binary of 25 starts them again at its first look, as every start did
-- before 0026.
DROP TABLE backup_pending;
