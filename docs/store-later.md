# Storage, later: encryption at rest and sync with a server

Two questions the owner asked after docs/store.md was written, on
2026-11-05 at night: can all data at rest be encrypted, end to end,
with one main key that a user's password unlocks and that can be
shared with or revoked from users; and how would the store of an edge
service and a public server stay in sync, with a client free to
connect to either. Neither is built now. This document says what each
would mean on top of docs/store.md, what the first version of the
store should do differently so that neither needs a rebuild, and what
is better left until then. Section 4 lists the decisions; one of them
changes an open decision of docs/store.md.

## 1. Encryption of the data at rest

### 1.1 What it protects

Massimo's data is names of rooms and devices, addresses, week
programmes, and a history of temperatures, valve positions and open
windows: who is at home and when, personal data in the sense of
design.md section 6. "At rest" means the store file, the backups in
it, the exports, and the log file (`serve -log`, `MASSIMO_LOG`), which
carries the same device names and state changes as `events`; an
encrypted store beside a plaintext log protects nothing, so the file
log either stays off, or is redacted, or is encrypted with the rest.
The Cube's own memory and the radio are eQ-3's and out of reach.

Two settings, two threat models:

- **A standalone box or an edge.** What is protected is a copy: the SD
  card out of a stolen Pi, a `massimo.db` or a backup carried to
  another computer, an old disk. Not protected: the running box. The
  server must decrypt to work (it connects to Cubes, writes events,
  serves the page), so the key is in its memory, and whoever is root
  on the box while it runs has the data. An unattended service after
  a reboot needs its key without a person typing a password, so on
  the box the key sits in a file in the state directory, readable by
  the service user only, or in a TPM where there is one. That is
  honest: it is the protection full-disk encryption gives, done by
  the application for the one file and its copies.
- **The public server.** The operator, and anyone who breaks into the
  server, must not be able to read the tenants' data. That is end to
  end: the server holds ciphertext, the keys are with the users and
  their edge agents, the decryption happens in the browser, the app
  and the edge.

### 1.2 The key scheme, made concrete

- One **data key** per tenant (the "main key"): 256 random bits,
  versioned (`key_id` 1, 2, …). Content is encrypted with it, AES-256-
  GCM, each row's table and identifier as associated data so that a
  ciphertext cannot be moved to another row.
- **Per user, a wrapped copy.** From the password, argon2id with a
  per-user salt gives a master secret; HKDF splits it into an
  authentication key, which goes to the server at login, and a
  wrapping key, which never leaves the client. The data key is stored
  wrapped with the wrapping key (`user_keys`: user, key_id, wrapped
  bytes). The server stores a hash of the authentication key, not the
  password's hash as docs/store.md section 6 has it today; guessing
  against a copy of the store still costs an argon2id per guess.
- **Sharing** is wrapping the data key for another user. For a
  household the simple way is an invitation: the owner, logged in and
  so holding the data key, creates the user with a one-time code; the
  data key travels wrapped under that code; the new user sets a
  password and the client re-wraps. Design.md section 6's time-limited
  shares for guests work the same way, with the wrap deleted at the
  end.
- **Revoking** is deleting the user's wrap and rotating: a new data
  key, the content re-encrypted, the old key kept wrapped for the
  remaining users so that old backups and exports still open
  (`keys` keeps every version). Deleting the wrap alone is not enough,
  the revoked user may have kept the old key.
- **Devices** are key holders too: an edge agent keeps the data key
  wrapped under a device key in its state directory (section 1.1);
  an integration (Loxone, a script) gets a scoped key or none.
- **A forgotten password loses the data** unless another owner holds
  a wrap or a recovery key was printed at setup. That is the price of
  end to end and has to be said in the UI the moment it is turned on.

### 1.3 What end to end does to the server mode

With ciphertext only, the public server cannot render a page from the
data, search it, run rules or schedules, or put content into a push
notification. It stores, relays and authenticates; the edge does the
work on the site, the browser and the app decrypt what they show.
That matches Route A of docs/background.md and section 2 below: the
edge is where the Cube is, and the server never writes site data. The
web UI is already a client-side program over the API, so decrypting
in the browser is natural; what is missing there is argon2id, which
WebCrypto does not have (AES-GCM, HKDF and PBKDF2 it has), so the page
would carry a small WebAssembly argon2 module, one dependency in a UI
that has none, or use PBKDF2 with a high iteration count for the
browser's key, which is weaker. The alternative to end to end is
server-side encryption with keys the operator holds, which keeps
every server feature and protects against a stolen disk but not
against the operator. The owner's wish is end to end; the decision in
section 4 is whether the server mode is designed around it from the
start, which it can be, because the data path (edge owns, server
relays) is the same either way.

### 1.4 Encryption in the local store

`modernc.org/sqlite` has no page-level encryption (no SQLCipher), so
the choices are the filesystem (LUKS or dm-crypt on the Pi, BitLocker,
FileVault: operational, nothing in the code) or the application
encrypting content column by column. Column encryption keeps the
structural columns in plaintext, because the indexes and the queries
need them: identifiers, `tenant_id`, `cube_id`, `serial` (the server
connects by it and discovery matches it), `users.name` (the login
lookup), `tokens.hash`, times, `kind`, `device`; and it encrypts the
content: names, addresses, `hidden`, `settings.value`,
`preferences.value`, `events.data`, `audit.detail`, `backups.body`,
`note`. Nothing in docs/store.md's schema indexes a content column or
filters on one, and the UI's log filter runs in the browser on the
rows it fetched, so the split costs no feature. A content column
holds either plaintext or a versioned envelope with a prefix, and the
store layer decrypts on read by a per-table list of columns; an
unencrypted file and an encrypted one are the same schema.

The envelope is one format for everything: the columns, the backups
in the store, and the passphrase-encrypted export that design.md
section 7 already asks for (argon2id and AES-GCM). One package,
`internal/seal`, with the envelope (version, key id, nonce,
ciphertext, the associated data by convention), the key wrapping and
the passphrase derivation, tested once; stage (e) of docs/store.md
uses it first for the encrypted backup download, the columns come
when the owner turns encryption on.

## 2. Sync between an edge service and a public server

### 2.1 The model: the edge owns the site, the server owns the accounts

Three ways to keep two stores in step:

- The server is the master and the edge a cache: the site stops
  working when the line is down, which design.md section 6 rules out.
- Both write everything and conflicts are resolved (version vectors,
  CRDTs): heavy, and wrong for data that has a physical source of
  truth, the Cube, which only the edge can read and write.
- **One writer per row.** Site data (Cubes, rooms, devices, their
  configuration and state, events, backups, site settings) is written
  by the edge only, which mirrors the Cube; account data (users,
  roles, memberships, shares, preferences) is written by the server
  only, and a standalone box is its own server. A change to site data
  asked for at the server is a **command**, relayed to the edge,
  executed there against the Cube, with the result coming back as
  events, never a write to a replica. With one writer per row there
  is nothing to reconcile: sync is replication from the owner to the
  other side plus a command queue in the other direction.

This is design.md section 5 (asynchronous writes with a command id
and a state, `Idempotency-Key`) and section 6 (edge agents over an
outbound connection, port 62910 never exposed) joined up. The restore
API of pull request #28 already has that shape: a plan, a send that
answers 202 with an id, the progress as events, the state on request.

A client connects to either: the API is the same (design.md section
8); on the LAN the app talks to the edge, away from home to the
server, which answers site data from its replica and marks how fresh
it is. The API already carries `received`, `last_seen` and `stale` per
value and `updated` per Cube; the server mode must pass those through
unchanged rather than stamp its own time on a replica.

### 2.2 How replication would work

- Rows of the synced tables carry `version` (bumped on every write),
  `updated_at`, `deleted_at` (a tombstone; nothing synced is ever
  deleted in place, a purge removes old tombstones) and `origin` (the
  node that owns the row). A follower asks the owner for every row
  with a version above the last it saw, per table; append-only tables
  (`events`, `audit`, `backups`) are followed by their identifier.
  With end to end, the replicated content columns are ciphertext and
  the structural columns are not, so the server can still index by
  Cube and time.
- The edge opens the connection (TLS, a WebSocket or long polls) and
  authenticates with its device key; the server pushes queued commands
  down it and takes the state changes and the replication up it.
- A `commands` table on both sides: identifier, tenant, Cube, origin,
  user, the request, state (`queued`, `sent`, `acknowledged`, `failed`,
  `dropped`), result, the idempotency key. On the edge the engine's
  `Ticket` and `Result` become these rows; the relay forwards `queued`
  rows and writes the states back.

### 2.3 Identity and authentication across nodes

- A node (edge or server) has an identifier and an Ed25519 key pair,
  generated at first start and kept in the state directory beside the
  device key of section 1, not in the store; the release signing in
  internal/update uses the same primitives. The server keeps a `nodes`
  table; an edge enrols with a one-time code, the QR of design.md
  section 8.
- A user logged in at the server must be able to use the edge on the
  LAN with the same session, also when the line to the server is
  down: either the server signs its session tokens (tenant, user,
  role, expiry) with its key and the edge verifies them with the
  server's public key, offline; or the tokens stay opaque and the edge
  asks the server, with a cache. Signed tokens are the one that keeps
  the house usable; they need a token format with a version prefix so
  that today's random tokens and tomorrow's signed ones can coexist in
  the same column.

## 3. What to prepare in version 1

Each item says what it costs now and what it saves. They are small;
together they are a day in stage (a) and (b).

1. **UUIDv7 on every row that will be replicated or exported**:
   `users`, `tokens`, `cubes`, `preferences`, `events`, `audit`,
   `backups`, and `commands` when it comes. The rowid stays the
   internal key; a `uuid TEXT NOT NULL UNIQUE` column is enough, with
   `github.com/google/uuid`, which is in the dependency tree already.
   Cost: one column and an index per table, 36 bytes a row. Saves:
   the one migration that can never be finished, renumbering rows
   that other nodes and old exports already refer to. This changes
   open decision 7 of docs/store.md from "with the export feature" to
   "from version 1".
2. **`version`, `deleted_at` and `origin` on the owned tables**
   (`users`, `tokens`, `cubes`, `settings`, `preferences`, `backups`),
   bumped and set by the store layer, and no `DELETE` of a synced row
   anywhere in the code: "remove" sets `deleted_at`, as the Cube row
   already does with `configured = 0`. Cost: three columns, a few
   lines in the store layer, a purge in the retention job. Saves: a
   change-tracking retrofit over every write path.
3. **A node identity at first start**: a UUID and an Ed25519 key pair
   in the state directory, the identifier in `settings` as `node.id`
   and on `/healthz`. Cost: twenty lines. Needed by item 2's `origin`.
4. **The content and structure rule**: no index and no `WHERE` on a
   content column (section 1.4), kept by a test in `internal/store`
   that lists the content columns and checks the schema's indexes
   against them. Cost: nothing but the discipline. Saves: queries that
   would have to be rewritten when a column becomes ciphertext.
5. **`internal/seal` in stage (e)** for the encrypted backup download
   that design.md section 7 asks for anyway: the envelope, the
   passphrase derivation, the key wrapping, tested once. Cost: a small
   package. Saves: a second format when the columns are encrypted.
6. **A password flow that can move to the client**: store the KDF
   parameters explicitly (the PHC string has them) and an
   `auth_version` on `users` (1: the server hashes the password, 2:
   the client derives the authentication key and the server hashes
   that), so that section 1.2 arrives as a change at the next login,
   not as a reset of every password. Cost: one column and the
   `auth_version` check at login. Saves: every user re-enrolling.
7. **`commands` in stage (b)** with `Idempotency-Key` on the write
   routes and `GET /api/v1/commands/{id}`: the restore API's id and
   state made general, the engine's tickets durable, `audit` pointing
   at the command. Cost: a table, the wrapper, the engine writing its
   states. Pays now: the UI's "pending" count survives a restart and
   says what is pending; a retried `PUT` does not send twice. Saves:
   the relay's queue later.
8. **Secrets stay out of the store** (open decision 2 as recommended):
   Cube keys and IVs and the bootstrap token remain in the env file.
   With end to end they move into the store encrypted; until then the
   file is the only secret-bearing thing on disk, which is simpler to
   reason about.
9. **No API field for freshness** now: `received`, `last_seen`,
   `stale`, `updated` and `started_at` exist; the server mode passes
   them through. Cost: none. The rule is written down here so that
   the server mode does not invent its own.

Not worth preparing now, because each can be added behind the above
without touching what exists: the `keys` and `user_keys` tables
(additive migration when encryption is turned on), the sync protocol
and the `nodes` table, signed tokens, argon2id in the browser,
SQLCipher or a filesystem choice, PostgreSQL migrations, the relay.
The schema stays SQLite-only until the server mode is built.

## 4. Decisions for the owner

1. **End to end as the goal of the server mode** (recommended, since
   the data path, edge owns and server relays, is the same either
   way), or server-side encryption with operator-held keys, which
   keeps rendering, search and notifications on the server. The
   choice shapes what the server mode may ever do with the data; it
   does not change version 1 of the store.
2. **Local encryption at rest on the box** as a stage (f) of
   docs/store.md with the key in a file in the state directory, once
   `internal/seal` exists (section 1.4), or left to the filesystem.
   Either way, the plaintext log file is the hole to close first.
3. **Open decision 7 of docs/store.md becomes UUIDv7 from version 1**
   (recommended, section 3 item 1).
4. **Tombstones, `version`, `origin` and the node identity from
   version 1** (recommended, items 2 and 3).
5. **The password flow prepared for client-side derivation** with
   `auth_version` (recommended, item 6).
6. **`commands` with `Idempotency-Key` in stage (b)** (recommended,
   item 7), or later with the relay.
7. **The browser's key derivation when end to end comes**: a
   WebAssembly argon2id module in the no-build UI, or PBKDF2 from
   WebCrypto with a high count. Not needed until then; noted so that
   the UI's "no dependencies" rule is bent knowingly, if it is.
