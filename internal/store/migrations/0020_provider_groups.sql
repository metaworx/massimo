-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- A provider's groups (AP Oidc v1.1, Block 8; docs/store.md section 6,
-- "Sign-in through OpenID Connect"): the claim its groups come in, and
-- the groups it admits, a JSON array of strings; a sign-in or a link of
-- an account in none of them is refused, none named admits any.
-- Additive: a binary built for an earlier version opens this file and
-- ignores the columns, admitting any group.
ALTER TABLE auth_providers ADD COLUMN groups_claim TEXT NOT NULL DEFAULT 'groups';
ALTER TABLE auth_providers ADD COLUMN allowed_groups TEXT NOT NULL DEFAULT '[]';
