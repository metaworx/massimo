-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Copyright (C) 2026 The Massimo Authors
--
-- min_reader: 1
--
-- The sign-in by OPAQUE (RFC 9807, docs/store.md section 6): what
-- users.auth_version 2 now means. The client stretches the password and
-- keeps it; the server keeps a registration record, the client's public
-- key, a masking key and an envelope, 192 bytes, that holds no hash of
-- anything the client sends and opens only with the server's key file
-- (opaque.key) and the password. opaque_ksf is the client's argon2id
-- parameters for the record, m=<KiB>,t=<passes>,p=<lanes>, sent with the
-- sign-in so that they can rise for new records. A user of version 2 has
-- password_hash empty. Additive: a binary of before reads such a user as
-- one without a password, keys only, which refuses rather than lets in.
ALTER TABLE users ADD COLUMN opaque_record BLOB;
ALTER TABLE users ADD COLUMN opaque_ksf TEXT NOT NULL DEFAULT '';
