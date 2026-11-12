-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- A removed user gives its name up (AP UserCli v1.0, Block 2;
-- docs/store.md section 6): its row is renamed <name>-del-YYYYMMDD-HHNN
-- of its removal in UTC, as DeleteUser now does at the removal, -<id>
-- added where that is taken. Before, the row kept the name, and a user
-- added under it took the row over with its id, uuid and history. Rows
-- only, no schema: a binary built for an earlier version reads the
-- renamed rows as any other removed user's.
UPDATE users
SET name = name || '-del-' || strftime('%Y%m%d-%H%M', deleted_at)
    || CASE WHEN EXISTS (SELECT 1 FROM users o
                         WHERE o.name = users.name || '-del-' || strftime('%Y%m%d-%H%M', users.deleted_at))
            THEN '-' || id ELSE '' END
WHERE deleted_at IS NOT NULL;
