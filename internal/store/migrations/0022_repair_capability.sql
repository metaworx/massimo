-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The capability repair (AP RadioHealth v1.2, Block 10, D166-A), which
-- runs the repairs of a Cube's rooms and devices: their communication
-- errors reset, a device's configuration re-sent from its room, a device
-- placed or listed where the Cube and its list disagree, a room's master
-- named, a room re-linked (docs/api-v1.md, "Repairs"). Seeded for
-- manager; owner has every capability by code. The role integration,
-- fixed at the ceiling of every call through an integration
-- (internal/server/auth.go, integrationCeiling), gains it with the
-- ceiling. Additive: a binary built for an earlier version ignores a
-- capability it does not know.
INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT id, 'repair' FROM roles WHERE key IN ('manager', 'integration') AND deleted_at IS NULL;
