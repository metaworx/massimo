-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- What a linked account is at its provider (AP Oidc v1.1, Block 7;
-- docs/store.md section 6, "Sign-in through OpenID Connect"): the name
-- and the user name (preferred_username) the provider gave at the last
-- sign-in or link, and its groups, a JSON array of strings, for the user
-- and the managers to see. email becomes the e-mail of the last sign-in
-- once bound. Additive: a binary built for an earlier version opens this
-- file and ignores the columns.
ALTER TABLE user_identities ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE user_identities ADD COLUMN username TEXT NOT NULL DEFAULT '';
ALTER TABLE user_identities ADD COLUMN groups TEXT NOT NULL DEFAULT '[]';
