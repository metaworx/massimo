-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- Sign-in through OpenID Connect (AP Oidc v1.0, Block 1; docs/store.md
-- sections 5 and 6): the providers a user may sign in through, and each
-- user's identities at them. A provider is found by its issuer's
-- discovery document; its client secret is kept here, as the Cubes' keys
-- are, and never answered. An identity is a provider's subject (sub)
-- for a user, or before its first sign-in an e-mail the owner assigned,
-- bound to the subject at the first sign-in with that e-mail verified.
-- New tables only: a binary built for an earlier version never reads
-- them.
CREATE TABLE auth_providers (
    id            INTEGER PRIMARY KEY,
    uuid          TEXT    NOT NULL UNIQUE,
    key           TEXT    NOT NULL,                -- in the routes, as a role's key
    name          TEXT    NOT NULL DEFAULT '',     -- on the sign-in button; '' shows the key
    issuer        TEXT    NOT NULL,                -- https://…, its discovery under /.well-known/openid-configuration
    client_id     TEXT    NOT NULL,
    client_secret TEXT    NOT NULL DEFAULT '',
    scopes        TEXT    NOT NULL DEFAULT 'openid email profile',
    auto_create   INTEGER NOT NULL DEFAULT 0,      -- 1: a sign-in without an identity creates the user
    auto_role_id  INTEGER REFERENCES roles (id),   -- the role of a user created so; NULL: viewer
    auto_domains  TEXT    NOT NULL DEFAULT '',     -- the e-mail domains it creates users for, comma-separated; '': any
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    updated_by    INTEGER REFERENCES users (id),
    version       INTEGER NOT NULL DEFAULT 1,
    deleted_at    TEXT,
    origin        TEXT    NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX auth_providers_key ON auth_providers (key) WHERE deleted_at IS NULL;

CREATE TABLE user_identities (
    id           INTEGER PRIMARY KEY,
    uuid         TEXT    NOT NULL UNIQUE,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id  INTEGER NOT NULL REFERENCES auth_providers (id) ON DELETE CASCADE,
    subject      TEXT,                             -- the provider's sub; NULL until the first sign-in binds it
    email        TEXT    NOT NULL DEFAULT '',      -- lower case: the e-mail assigned, or the one of the sign-in that bound it
    created_at   TEXT    NOT NULL,
    created_by   INTEGER REFERENCES users (id),
    bound_at     TEXT,
    last_used_at TEXT
);
CREATE UNIQUE INDEX user_identities_subject ON user_identities (provider_id, subject) WHERE subject IS NOT NULL;
CREATE UNIQUE INDEX user_identities_email ON user_identities (provider_id, email) WHERE subject IS NULL;
CREATE INDEX user_identities_user ON user_identities (user_id);
