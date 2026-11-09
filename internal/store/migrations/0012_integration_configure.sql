-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The role integration gains configure (AP HomeAssistant v1.0, Block 2;
-- docs/store.md section 6): a home automation such as Home Assistant
-- writes week programmes, temperatures and device settings as the web UI
-- does. Only where the role still holds exactly its seeded capabilities,
-- read, setpoint and lock: an owner who changed what an automation may
-- do decided it, and an upgrade does not widen that. Additive: a binary
-- built for an earlier version reads the row like any other.
INSERT OR IGNORE INTO role_capabilities (role_id, capability)
SELECT r.id, 'configure' FROM roles r
WHERE r.key = 'integration' AND r.deleted_at IS NULL
  AND (SELECT count(*) FROM role_capabilities c WHERE c.role_id = r.id) = 3
  AND (SELECT count(*) FROM role_capabilities c
       WHERE c.role_id = r.id AND c.capability IN ('lock', 'read', 'setpoint')) = 3;
