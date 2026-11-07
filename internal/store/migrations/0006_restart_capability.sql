-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The capability restart, which restarts the server from the API through
-- the service manager that runs it (docs/api-v1.md, "POST
-- /api/v1/restart"). Seeded for manager; owner has every capability by
-- code. Additive: a binary built for an earlier version ignores a
-- capability it does not know.
INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT id, 'restart' FROM roles WHERE key = 'manager' AND deleted_at IS NULL;
