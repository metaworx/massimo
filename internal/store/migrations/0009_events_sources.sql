-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The log's sources (AP Logging v1.1, Block 3; docs/store.md section 5):
-- an event is a Cube's or, with cube_id NULL, the server's, and says
-- whose it is (source: device, cube, system or config), how much it
-- matters (level: slog's -4 debug, 0 info, 4 warn, 8 error), which
-- device it concerns by its serial, and who started it. SQLite cannot
-- drop the NOT NULL of cube_id, so the table is made again, the way its
-- documentation gives: a new table, the rows copied with their new
-- columns, the old one dropped, the new one renamed, the indexes made;
-- no table references events. The copy keeps every id and uuid and
-- fills the new columns from each row's own JSON: the serial from the
-- event or its device view, else from another row of the same Cube and
-- radio address, else for the Cube's own C line the Cube's; the source
-- from the event, else system for a restore's, cube for the Cube's own C
-- line and for a row without a device, device for the rest; the level
-- warn for disconnected, debug for hello, info for the rest.
-- (cube_id, device) gives way to (cube_id, serial); (cube_id, source)
-- and the partial events_user are new; the level has no index, since
-- "this level and above" is a range that would give up the id order.
-- Additive in effect, so min_reader stays 1: a binary of version 8 reads
-- its Cube's rows by cube_id = ?, never meeting a NULL one, and writes
-- rows without the new columns, which take their defaults; this binary
-- fills those (source '') at its start.
CREATE TABLE events_new (
    id        INTEGER PRIMARY KEY,
    uuid      TEXT    NOT NULL UNIQUE,
    cube_id   INTEGER REFERENCES cubes (id) ON DELETE CASCADE, -- NULL: the server's
    time      TEXT    NOT NULL,
    kind      TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source    TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config; '' a row of a binary before
    level     INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device    TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial    TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    user_id   INTEGER REFERENCES users (id),     -- who started it; NULL: the Cube, the server's own jobs, a server without users
    data      TEXT    NOT NULL                   -- JSON: the event object as GET …/log returns it
);

INSERT INTO events_new (id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
SELECT e.id, e.uuid, e.cube_id, e.time, e.kind,
       CASE
           WHEN coalesce(json_extract(e.data, '$.source'), '') != '' THEN json_extract(e.data, '$.source')
           WHEN e.kind IN ('restore_step', 'restore_done') THEN 'system'
           WHEN e.device = '' OR e.device = c.rf_address THEN 'cube'
           ELSE 'device'
       END,
       CASE e.kind WHEN 'disconnected' THEN 4 WHEN 'hello' THEN -4 ELSE 0 END,
       e.device,
       coalesce(nullif(json_extract(e.data, '$.serial'), ''),
                nullif(json_extract(e.data, '$.state.serial'), ''),
                m.serial,
                CASE WHEN e.device != '' AND e.device = c.rf_address THEN c.serial END,
                ''),
       NULL,
       e.data
FROM events e
LEFT JOIN cubes c ON c.id = e.cube_id
LEFT JOIN (SELECT cube_id, device,
                  max(coalesce(nullif(json_extract(data, '$.serial'), ''), nullif(json_extract(data, '$.state.serial'), ''))) AS serial
           FROM events WHERE device != '' GROUP BY cube_id, device) m
       ON m.cube_id = e.cube_id AND m.device = e.device;

DROP TABLE events;
ALTER TABLE events_new RENAME TO events;

CREATE INDEX events_cube ON events (cube_id);
CREATE INDEX events_cube_time ON events (cube_id, time);
CREATE INDEX events_cube_kind ON events (cube_id, kind);
CREATE INDEX events_cube_source ON events (cube_id, source);
CREATE INDEX events_cube_serial ON events (cube_id, serial);
CREATE INDEX events_user ON events (user_id) WHERE user_id IS NOT NULL;
