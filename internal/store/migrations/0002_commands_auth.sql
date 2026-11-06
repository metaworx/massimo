-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Stage (b) of docs/store.md: what version 1 left for it (docs/server.md,
-- "What version 1 prepares", items 6 and 7). Additive: a binary built for
-- version 1 opens this file and ignores what it does not know.
--
-- users.auth_version says who derives the authentication key: 1, the
-- server hashes the password; 2, the client derives the key and the
-- server hashes that. Read at login, so that the key scheme arrives as a
-- change at the next login rather than as a reset of every password.
ALTER TABLE users ADD COLUMN auth_version INTEGER NOT NULL DEFAULT 1;

-- commands: every write request is a row (the parent), and every
-- engine.Command the request enqueues is a row beneath it (parent_id
-- set, action 'step'), written by the queue's observer. The states are
-- those of docs/design.md section 5. The idempotency key is the client's
-- Idempotency-Key, kept per user for 24 hours so that a retried write
-- answers what the first one did and sends nothing twice.
--
-- The content and structure rule (rule_test.go): request and result are
-- content columns.
CREATE TABLE commands (
    id              INTEGER PRIMARY KEY,
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
CREATE INDEX commands_cube_time ON commands (cube_id, created_at);
CREATE INDEX commands_parent ON commands (parent_id);
CREATE INDEX commands_pending ON commands (state, created_at) WHERE state IN ('queued', 'sent');
CREATE INDEX commands_key ON commands (user_id, idempotency_key);

-- The audit row of a request points at its command.
ALTER TABLE audit ADD COLUMN command_id INTEGER REFERENCES commands (id) ON DELETE SET NULL;
