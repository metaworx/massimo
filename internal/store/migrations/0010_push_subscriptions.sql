-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Push subscriptions (AP ServerAnnouncements v1.0, Block 3;
-- docs/store.md section 5): a browser's or an app's Web Push
-- subscription, which the server sends a user's notifications to. Each
-- belongs to the user and to the session or API key that subscribed, so
-- that revoking it removes the subscription. The endpoint is the push
-- service's URL, unique: the same browser subscribing again updates its
-- row. p256dh and auth are the browser's keys for the message's
-- encryption (RFC 8291). Not replicated: a subscription is bound to this
-- server's VAPID key, so a row is removed, not marked. Additive: a
-- binary built for an earlier version ignores the table.
CREATE TABLE push_subscriptions (
    id         INTEGER PRIMARY KEY,
    uuid       TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_id   INTEGER NOT NULL REFERENCES tokens (id) ON DELETE CASCADE,
    endpoint   TEXT    NOT NULL UNIQUE,
    p256dh     TEXT    NOT NULL,             -- the browser's P-256 key, base64url
    auth       TEXT    NOT NULL,             -- the browser's auth secret, base64url
    label      TEXT    NOT NULL DEFAULT '',  -- 'Firefox on the phone'
    created_at TEXT    NOT NULL,
    last_ok_at TEXT,                         -- the last message the push service took
    failures   INTEGER NOT NULL DEFAULT 0    -- failed deliveries since then
);
CREATE INDEX push_subscriptions_user ON push_subscriptions (user_id);
CREATE INDEX push_subscriptions_token ON push_subscriptions (token_id);
