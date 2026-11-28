-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 24
--
-- A token's revoke code (AP MobileApp v1.6, Block 21, D112-A;
-- docs/store.md section 5, tokens): tokens.revoke_hash is
-- HashToken of the token's code, RevokeCode(secret), which a client that
-- once held the secret posts to POST /api/v1/tokens/revoke to end a
-- token it forgot. Not the hash the store keeps already, with which the
-- store and every backup of it could revoke every token. Written where
-- the store makes the secret, and for an older token at its first use
-- after the update; NULL for a token the store never held the secret
-- of, the settings file's key among them. Additive: a binary for 24 to
-- 26 reads the file and writes its tokens without the column, which the
-- next use with this binary fills.
ALTER TABLE tokens ADD COLUMN revoke_hash TEXT;
CREATE UNIQUE INDEX tokens_revoke ON tokens (revoke_hash) WHERE revoke_hash IS NOT NULL;
