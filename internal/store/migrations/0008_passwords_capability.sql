-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The capability passwords, which sets the password of a user whose role
-- has no capability the setter's lacks, never an owner's (docs/api-v1.md,
-- "Authentication"). Seeded for manager; owner has every capability by
-- code. Additive: a binary built for an earlier version ignores a
-- capability it does not know.
INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT id, 'passwords' FROM roles WHERE key = 'manager' AND deleted_at IS NULL;
