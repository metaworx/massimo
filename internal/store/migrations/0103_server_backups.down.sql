-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- The down part of 0103_server_backups.sql (AP ServerBackup v1.0, Block
-- 2; docs/store.md section 5, "Migrations"): massimo store downgrade
-- runs it to take a file back to schema 102. It loses what 0103 added,
-- and only that: the catalogue of the server's backups, their keys and
-- targets, and the capabilities backup.server and backup.retry of every
-- role. The backups' files stay where they are, and a later 0103 finds
-- them again by a scan of the target added anew.
DROP TABLE server_backup_copies;
DROP TABLE server_backups;
DROP TABLE server_backup_targets;
DROP TABLE server_backup_keys;
DELETE FROM role_capabilities WHERE capability IN ('backup.server', 'backup.retry');
