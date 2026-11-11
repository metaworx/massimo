-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
-- foreign_keys: off
--
-- One log (AP OneLog v1.1, Block 4; docs/store.md section 5): the
-- events become the log, whose rows may name the command they belong to
-- and say who may read them; and a command's id is never given out
-- again. The runner applies this migration with foreign keys off
-- (migrate.go, "-- foreign_keys: off"), since DROP TABLE with them on
-- deletes the rows first, which would set every audit.command_id to
-- NULL and delete every step with its request; it checks the foreign
-- keys before the commit.
--
-- commands: without AUTOINCREMENT SQLite gives a new row the highest id
-- plus one, so once the prune removed the newest command the next one
-- took its id, and GET /api/v1/commands/{id} read before the prune named
-- another command (as 0013 for the backups). The table is made again as
-- it stands: every row copied with its id, parent_id and the rest,
-- the indexes made; the copy starts the sequence at the highest id left,
-- and the rename carries it.
--
-- log: the events table made again under its new name, as 0009 made it,
-- every row copied with its id, with two columns: command_id, the
-- command a row belongs to (its attempt, a discard and a retry, its
-- outcome), NULL once the command is pruned; and visibility, read for
-- what the log shows to a user with read, audit for the rows only a user
-- with audit sees (sign-ins, lockouts). The indexes are made again under
-- the new name, with one of the rows that name a command.
--
-- Additive, so min_reader stays 1: a binary built for an earlier
-- version still reads and writes the log through the view events, the
-- log's columns as that version knew them, whose INSTEAD OF triggers
-- turn its insert, update and delete into the log's (an insert of a
-- binary before 0009, without source, level and serial, takes their
-- defaults). SQLite counts no rows changed through a view, so such a
-- binary's prune removes one chunk per Cube a pass, and the next pass
-- goes on. The view goes in a migration that raises min_reader.
CREATE TABLE commands_new (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- never given out twice
    uuid            TEXT    NOT NULL UNIQUE,
    parent_id       INTEGER REFERENCES commands (id) ON DELETE CASCADE,  -- a step's request
    cube_id         INTEGER REFERENCES cubes (id),
    user_id         INTEGER REFERENCES users (id),                       -- NULL: no authentication, or the server itself
    token_id        INTEGER REFERENCES tokens (id),
    idempotency_key TEXT    NOT NULL DEFAULT '',   -- the client's Idempotency-Key; '' after 24 hours
    action          TEXT    NOT NULL,              -- as audit.action; 'step' for a step
    target          TEXT    NOT NULL DEFAULT '',   -- 'device:0abc12', 'room:3', 'user:anna'
    request         TEXT    NOT NULL DEFAULT '{}', -- JSON: the method, the path, the body (secrets removed) and its hash; a step: its key and lines
    state           TEXT    NOT NULL CHECK (state IN ('queued', 'sent', 'acknowledged', 'failed', 'dropped')),
    status          INTEGER NOT NULL DEFAULT 0,    -- the HTTP status answered; 0 while running, and for a step
    result          TEXT    NOT NULL DEFAULT '{}', -- JSON: the response body; a step: discarded, the budget, the error
    created_at      TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL,
    finished_at     TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    deleted_at      TEXT,
    origin          TEXT    NOT NULL DEFAULT ''
);
INSERT INTO commands_new (id, uuid, parent_id, cube_id, user_id, token_id, idempotency_key, action, target, request, state, status, result,
                          created_at, updated_at, finished_at, version, deleted_at, origin)
SELECT id, uuid, parent_id, cube_id, user_id, token_id, idempotency_key, action, target, request, state, status, result,
       created_at, updated_at, finished_at, version, deleted_at, origin
FROM commands;
DROP TABLE commands;
ALTER TABLE commands_new RENAME TO commands;
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_key ON commands (user_id, idempotency_key);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');

CREATE TABLE log (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    cube_id    INTEGER REFERENCES cubes (id) ON DELETE CASCADE, -- NULL: the server's
    time       TEXT    NOT NULL,
    kind       TEXT    NOT NULL,                  -- connected, state, paired, cube_found, … as the stream names them
    source     TEXT    NOT NULL DEFAULT '',       -- device, cube, system, config, …; '' a row of a binary before 0009
    level      INTEGER NOT NULL DEFAULT 0,        -- slog's: -4 debug, 0 info, 4 warn, 8 error
    device     TEXT    NOT NULL DEFAULT '',       -- radio address, '' when none
    serial     TEXT    NOT NULL DEFAULT '',       -- the device's serial number, or the Cube's for its own lines
    user_id    INTEGER REFERENCES users (id),     -- who started it; NULL: the Cube, the server's own jobs, a server without users
    data       TEXT    NOT NULL,                  -- JSON: the event object as GET …/log returns it
    command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL, -- the command the row belongs to
    visibility TEXT    NOT NULL DEFAULT 'read' CHECK (visibility IN ('read', 'audit'))
);
INSERT INTO log (id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
SELECT id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data
FROM events;
DROP TABLE events;
CREATE INDEX log_cube ON log (cube_id);
CREATE INDEX log_cube_time ON log (cube_id, time);
CREATE INDEX log_cube_kind ON log (cube_id, kind);
CREATE INDEX log_cube_source ON log (cube_id, source);
CREATE INDEX log_cube_serial ON log (cube_id, serial);
CREATE INDEX log_cube_warn ON log (cube_id) WHERE level >= 4;
CREATE INDEX log_user ON log (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX log_command ON log (command_id) WHERE command_id IS NOT NULL;

CREATE VIEW events AS
SELECT id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data FROM log;
CREATE TRIGGER events_insert INSTEAD OF INSERT ON events
BEGIN
    INSERT INTO log (id, uuid, cube_id, time, kind, source, level, device, serial, user_id, data)
    VALUES (NEW.id, NEW.uuid, NEW.cube_id, NEW.time, NEW.kind, coalesce(NEW.source, ''), coalesce(NEW.level, 0),
            coalesce(NEW.device, ''), coalesce(NEW.serial, ''), NEW.user_id, NEW.data);
END;
CREATE TRIGGER events_update INSTEAD OF UPDATE ON events
BEGIN
    UPDATE log SET uuid = NEW.uuid, cube_id = NEW.cube_id, time = NEW.time, kind = NEW.kind, source = NEW.source,
                   level = NEW.level, device = NEW.device, serial = NEW.serial, user_id = NEW.user_id, data = NEW.data
    WHERE id = OLD.id;
END;
CREATE TRIGGER events_delete INSTEAD OF DELETE ON events
BEGIN
    DELETE FROM log WHERE id = OLD.id;
END;
