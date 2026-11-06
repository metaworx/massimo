-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Stage (c) of docs/store.md: the capability preferences, which keeps
-- the page's settings and the user's colour schemes on the server so
-- that they follow the user across browsers. Seeded for manager,
-- operator and viewer; guest and integration keep their settings in the
-- browser, and owner has every capability by code. Additive: a binary
-- built for version 1 or 2 ignores a capability it does not know.
INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT id, 'preferences' FROM roles WHERE key IN ('manager', 'operator', 'viewer') AND deleted_at IS NULL;
