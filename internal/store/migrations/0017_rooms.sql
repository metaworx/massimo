-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The rooms of each Cube as the server knows them (AP RoomApi v1.0,
-- Block 3; docs/store.md section 5): a room's uid, which a client keeps
-- for the room's life and which no later room of the same id takes, and
-- the settings of the room's virtual wall thermostat (docs/api-v1.md,
-- "The virtual wall thermostat"). The server adds a row for a room the
-- Cube's metadata names and marks it deleted when the metadata leaves the
-- room out; a room the Cube gives the same id later gets a row of its
-- own. New table only: a binary built for an earlier version never reads
-- it.
CREATE TABLE rooms (
    id          INTEGER PRIMARY KEY,
    uuid        TEXT    NOT NULL UNIQUE,            -- the room's uid in the API (room_uid)
    cube_id     INTEGER NOT NULL REFERENCES cubes (id) ON DELETE CASCADE,
    room_id     INTEGER NOT NULL,                   -- the Cube's room number, 1..254
    temp_method TEXT    NOT NULL DEFAULT 'median' CHECK (temp_method IN ('median', 'mean')),
    rf_address  TEXT    NOT NULL DEFAULT '',        -- the virtual wall thermostat's radio address, 6 hex digits; '' its room number
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL,
    updated_by  INTEGER REFERENCES users (id),
    version     INTEGER NOT NULL DEFAULT 1,
    deleted_at  TEXT,
    origin      TEXT    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX rooms_room ON rooms (cube_id, room_id) WHERE deleted_at IS NULL;
