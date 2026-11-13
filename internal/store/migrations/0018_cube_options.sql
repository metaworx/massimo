-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- A Cube's options beyond its row (AP RoomApi v1.0, Block 7; docs/store.md
-- section 5): proxy_virtual makes its LAN proxy show clients the rooms'
-- virtual wall thermostats in place of the real wall thermostats
-- (docs/proxy.md). A Cube without a row has the defaults. New table only:
-- a binary built for an earlier version never reads it.
CREATE TABLE cube_options (
    cube_id       INTEGER PRIMARY KEY REFERENCES cubes (id) ON DELETE CASCADE,
    proxy_virtual INTEGER NOT NULL DEFAULT 0,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id)
);
