# The server mode: encryption at rest, the public server, sync

The owner's questions after the first version of docs/store.md, with
the answers from the design conversation of 2026-11-05 and 2026-11-06,
kept here so that the server mode starts from them: can all data at
rest be encrypted, end to end, with one main key that a user's
password unlocks and that can be shared with or revoked from users;
and how would the store of an edge service and a public server stay
in sync, with a client free to connect to either. Neither is built
now. The last two parts say what version 1 prepares for them and what
that adds to the decisions of docs/store.md section 10.

## The purpose is data protection

The reason for end-to-end encryption is liability: data the operator
cannot read is data there is little to be liable for. That holds,
with a limit: encrypted personal data is still personal data under
the GDPR and the nDSG (the terms, the hosting location and the access
controls remain duties), but the exposure collapses, and the duty to
notify the people themselves of a breach falls away when the data was
unintelligible to whoever took it (GDPR art. 34(3)(a); the Swiss duty
is risk-based, and encryption is what lowers the risk).

Portal mode breaks the argument. The Cube's portal traffic is
encrypted with the Cube's own key and IV, and whatever terminates it
has the plaintext; it cannot be the app, because the Cube expects its
reply within its poll cycle and a phone is not always there. So
portal mode is "the edge runs in the cloud for you": the same engine
and the same store, hosted instead of on a Pi, with something of the
operator's holding the Cube key around the clock. That gives two
tiers with two promises, and the terms say which one a customer is
on:

- **Edge mode**: zero knowledge is possible; the server relays and
  stores what it cannot open.
- **Portal mode**: the server processes plaintext; the case rests on
  the ordinary measures (encryption at rest with the operator's keys,
  access control, hosting, terms).

## What encryption at rest protects

Massimo's data is names of rooms and devices, addresses, week
programmes, and a history of temperatures, valve positions and open
windows: who is at home and when, personal data in the sense of
design.md section 6. "At rest" means the store file, the backups in
it, the exports, and the log file (`serve -log`, `MASSIMO_LOG`), which
carries the same device names and state changes as `events`; an
encrypted store beside a plaintext log protects nothing, so the file
log either stays off, or is redacted, or is encrypted with the rest.
The Cube's own memory and the radio are eQ-3's and out of reach.

- **On a standalone box or an edge** what is protected is a copy: the
  SD card out of a stolen Pi, a `massimo.db` or a backup carried to
  another computer, an old disk. Not protected: the running box. The
  server must decrypt to work (it connects to Cubes, writes events,
  serves the page), so the key is in its memory, and whoever is root
  on the box while it runs has the data. An unattended service after
  a reboot needs its key without a person typing a password, so on
  the box the key sits in a file in the state directory, readable by
  the service user only, or in a TPM where there is one. That is
  honest: it is the protection full-disk encryption gives, done by
  the application for the one file and its copies.
- **On the public server** the operator, and anyone who breaks into
  the server, must not be able to read the tenants' data. That is end
  to end: the server holds ciphertext, the keys are with the users and
  their edge agents, the decryption happens in the browser, the app
  and the edge.

## The key scheme

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
  password's hash as docs/store.md section 6 has it today; guessing against a copy
  of the store still costs an argon2id per guess.
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
  wrapped under a device key in its state directory; an integration
  (Loxone, a script) gets a scoped key or none.
- **A forgotten password loses the data** unless another owner holds
  a wrap or a recovery key was printed at setup. That is the price of
  end to end and has to be said in the UI the moment it is turned on.

## Encryption in the local store

`modernc.org/sqlite` has no page-level encryption (no SQLCipher), so
on a box the choices are the filesystem (LUKS or dm-crypt on the Pi,
BitLocker, FileVault: operational, nothing in the code) or the
application encrypting content column by column. Column encryption
keeps the structural columns in plaintext, because the indexes and
the queries need them: identifiers, `tenant_id`, `cube_id`, `serial`
(the server connects by it and discovery matches it), `users.name`
(the login lookup), `tokens.hash`, times, `kind`, `device`; and it
encrypts the content: names, addresses, `hidden`, `settings.value`,
`preferences.value`, `events.data`, `audit.detail`, `backups.body`,
`note`. Nothing in the schema of docs/store.md section 5 indexes a content column
or filters on one, and the UI's log filter runs in the browser on the
rows it fetched, so the split costs no feature. A content column
holds either plaintext or a versioned envelope with a prefix, and the
store layer decrypts on read by a per-table list of columns; an
unencrypted file and an encrypted one are the same schema.

The envelope is one format for everything: the columns, the backups
in the store, and the passphrase-encrypted export that design.md
section 7 already asks for (argon2id and AES-GCM). One package,
`internal/seal`, with the envelope (version, key id, nonce,
ciphertext, the associated data by convention), the key wrapping and
the passphrase derivation, tested once; stage (e) of docs/store.md uses it first for
the encrypted backup download, the columns come when the owner turns
encryption on.

## Where the data lives on the server

Per-tenant files, not encrypted rows in a shared database. Column
encryption with plaintext structure is for the box; on the server it
would exist so that the server could index events by Cube and time,
and in the model where the edge owns the site the server never needs
that:

- On the **edge tier** the server holds one encrypted blob per tenant
  and site that it cannot open (the latest snapshot the edge pushed,
  the backups) and a small account database it must read (users,
  memberships, billing) that carries no heating data.
- On the **portal tier** the hosted engine is the edge code, so its
  store is one SQLite file per tenant with the schema of docs/store.md section 5,
  the same code and the same migrations. One file per tenant is a
  known pattern at scale (replication of the files to object storage
  for durability) and is the compliance-friendly form: erasure is
  deleting the file and its backups, portability is handing the file
  over, and a query cannot cross tenants. The PostgreSQL route of
  design.md (row-level security in one database) is for cross-tenant
  work only, which the account database does.

The server runs ZFS with encryption already, so at rest is settled
there without touching the application: one dataset per tenant with
its own key gives the files their own key, snapshots and erasure. The
caveat is the same for every server-held key, ZFS, MariaDB or MySQL
with their built-in tablespace encryption, PostgreSQL with `pgcrypto`
or Percona's `pg_tde`, SQLCipher through cgo on a server build,
`fscrypt` with a key per directory: they protect a stolen disk and a
leaked backup, which is what the portal tier and the account database
need, and none of them is the operator-cannot-read promise. Only keys
the server never has give that, which is why the edge tier needs none
of them for site data. No second DBMS in the stack unless scale asks.

## Who queries what on the edge tier

With ciphertext only, the public server cannot render a page from the
data, search it, run rules or schedules, or put content into a push
notification. Clients connect through it, because the edge is behind
NAT with a changing address, but it is a relay, not a database:

- **Edge online**, the normal case: the app's request goes through
  the server to the edge, which answers from its own store, live, the
  same API as on the LAN. The encryption is between app and edge,
  with a session key the two agree through the relay; the server
  copies frames it cannot read. Every query the UI makes today is
  answered as today, one hop later.
- **Edge offline**: the server hands out the last encrypted snapshot
  the edge pushed (state, metadata, the recent history: tens of
  kilobytes for a house); the app decrypts it and shows it as "state
  as of 14:32, edge offline", read-only. The log filter runs in the
  browser already today, so a snapshot is enough. Commands queue at
  the server, encrypted, and go down when the edge is back, with the
  states the `commands` table tracks.

Ruled out on this tier: searching a long history without downloading
it, notifications composed on the server (the edge composes them,
the server pushes an opaque payload), and anything running on the
server while the edge is down, which is moot without a Cube to act
on. Whoever needs those is on the portal tier. The web UI is already a
client-side program over the API, so decrypting in the browser is
natural; what is missing there is argon2id, which WebCrypto does not
have (AES-GCM, HKDF and PBKDF2 it has), so the page would carry a
small WebAssembly argon2 module, one dependency in a UI that has none,
or use PBKDF2 with a high iteration count for the browser's key,
which is weaker.

## The edge's connection

One persistent outbound WebSocket over TLS to port 443, opened by
the edge, a ping every 30 to 60 seconds (home routers drop idle TCP
mappings after a few minutes), reconnection with jittered backoff
capped at about a minute; the server marks the site offline after
about 90 seconds without a ping and the app shows it. Not polling: a
press on the phone's chevron should reach the Cube in a second or
two, polling fast enough for that costs more than the pings and is
still slower, and the Cube's own portal, which polls every few
seconds, is the sluggishness the edge replaces. Cost on a Pi 1: one
connection, one goroutine, a few bytes a minute; on the server one
goroutine per edge.

Over the one connection, multiplexed as streams with a small frame
header: commands down; states, events, snapshots and replication up;
and the relayed app sessions, one stream each, the same API framed
over the stream. At connect: the server's certificate from the public
CA, and the edge's Ed25519 device key signing a server challenge,
which is simpler on a Pi than client certificates and is the "device
certificate or token" of design.md section 6; the release signing in
internal/update uses the same primitives. A long-polling fallback for
networks that block WebSockets fits behind the same framing later.

## Trusting the code: the page comes from the edge

The weak point of end to end in a browser is that the page is only
as trustworthy as whoever serves it: a compromised or coerced server
could serve a page that leaks the key, so "we could not read it"
would become "we would have had to change the code to read it". The
owner's answer: the edge signs what it serves, the client pins the
edge's key after the user has checked its fingerprint once, at setup,
and the server only caches; a tampered server is recognised.

That works, with one condition: the verifier must be anchored outside
the server, because a verifier that arrives from the server is
replaced with the page. Two anchors do it, and both are standard
browser mechanics rather than a custom scheme:

1. **A web app installed from the edge, on the LAN.** The edge serves
   the page over its own TLS; the user confirms the edge's certificate
   fingerprint once (the same fingerprint the edge shows at setup, as
   text and as the QR of design.md section 8). The browser installs
   the service worker from the edge's origin, and from then on the
   page's code lives in the browser and updates only from that origin,
   which is unreachable away from home, so the server can never
   replace it; data goes to the server's relay as a plain fetch. The
   service worker, trusted because it came from the edge, is then the
   verifier for everything after: a new bundle relayed through the
   server is accepted only with a valid signature from the pinned
   edge key, which is exactly the owner's signing scheme, with the
   service worker as the place it runs. The same key authenticates
   the edge in the end-to-end session, so the one fingerprint check
   covers code and data.
2. **An installed app shell** (the Capacitor shells of design.md
   section 8): the shell is the anchor, it holds the pinned key and
   verifies the bundles it loads, wherever they came from.

Residual risks, to be named in the terms: a user who never installed
from the LAN uses the server-served page and is on the weaker
promise; a browser may evict the cached code under storage pressure
(the app asks for persistent storage); a self-signed certificate on
the edge needs the one-time confirmation, which users skip unless the
setup makes it a step (a public certificate for a name that resolves
to the LAN address avoids the dialog); the edge's key lives in its
state directory, so replacing the Pi by copying the directory keeps
the pin, and a new key means re-pairing every client.

## Sync: the edge owns the site, the server owns the accounts

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
A client connects to either side with the same API (design.md section
8); the server answers site data from its replica and marks how fresh
it is. The API already carries `received`, `last_seen` and `stale` per
value and `updated` per Cube; the server mode passes those through
unchanged rather than stamp its own time on a replica.

How replication works:

- Rows of the synced tables carry `version` (bumped on every write),
  `updated_at`, `deleted_at` (a tombstone; nothing synced is ever
  deleted in place, a purge removes old tombstones) and `origin` (the
  node that owns the row). A follower asks the owner for every row
  with a version above the last it saw, per table; append-only tables
  (`events`, `audit`, `backups`) are followed by their identifier.
  With end to end, the replicated content columns are ciphertext and
  the structural columns are not.
- A `commands` table on both sides: identifier, tenant, Cube, origin,
  user, the request, state (`queued`, `sent`, `acknowledged`, `failed`,
  `dropped`), result, the idempotency key. On the edge the engine's
  `Ticket` and `Result` become these rows; the relay forwards `queued`
  rows and writes the states back.

Identity and authentication across nodes:

- A node (edge or server) has an identifier and an Ed25519 key pair,
  generated at first start and kept in the state directory beside the
  device key, not in the store. The server keeps a `nodes` table; an
  edge enrols with a one-time code, the QR of design.md section 8.
- A user logged in at the server must be able to use the edge on the
  LAN with the same session, also when the line to the server is
  down: either the server signs its session tokens (tenant, user,
  role, expiry) with its key and the edge verifies them with the
  server's public key, offline; or the tokens stay opaque and the edge
  asks the server, with a cache. Signed tokens are the one that keeps
  the house usable; they need a token format with a version prefix so
  that today's random tokens and tomorrow's signed ones can coexist in
  the same column.

## Tenants, sites, edges and Cubes

- **Cube to tenant is n:1.** A tenant, a household or an
  organisation, has one or more Cubes; a Cube belongs to one tenant at
  a time and is transferred by an update with an audit row, never
  owned twice. People reach another tenant's Cubes through a
  membership or a share, not through a Cube with two tenants.
- **Tenant to edge is 1:n, through sites.** A tenant has one or more
  sites; a site has one edge (the box holding the Cubes' single LAN
  connections) or none on the portal tier; a site has n Cubes, and
  one edge serves several Cubes already today. A box serving two
  tenants (a property manager's) is the manager being a member of
  each tenant, not a shared edge.
- **Version 1** has `tenant_id`, and `origin` (the node identity
  below) is the site's identity, since a standalone box is one site.
  The identity lives in the store and the state directory and
  survives a replaced Pi by copying them; a `sites` table comes with
  the server.

## What version 1 prepares

Each item says what it costs now and what it saves. They are small;
together they are a day in stages (a) and (b) of docs/store.md.

1. **UUIDv7 on every row that will be replicated or exported**:
   `users`, `tokens`, `cubes`, `preferences`, `events`, `audit`,
   `backups`, and `commands` when it comes. The rowid stays the
   internal key; a `uuid TEXT NOT NULL UNIQUE` column is enough, with
   `github.com/google/uuid`, which is in the dependency tree already.
   Cost: one column and an index per table, 36 bytes a row. Saves:
   the one migration that can never be finished, renumbering rows
   that other nodes and old exports already refer to. This is what
   open decision 7 of docs/store.md now recommends.
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
   content column, kept by a test in `internal/store` that lists the
   content columns and checks the schema's indexes against them.
   Cost: nothing but the discipline. Saves: queries that would have
   to be rewritten when a column becomes ciphertext.
5. **`internal/seal` in stage (e) of docs/store.md** for the encrypted backup download
   that design.md section 7 asks for anyway: the envelope, the
   passphrase derivation, the key wrapping, tested once. Cost: a small
   package. Saves: a second format when the columns are encrypted.
6. **A password flow that can move to the client**: store the KDF
   parameters explicitly (the PHC string has them) and an
   `auth_version` on `users` (1: the server hashes the password, 2:
   the client derives the authentication key and the server hashes
   that), so that the key scheme arrives as a change at the next
   login, not as a reset of every password. Cost: one column and the
   `auth_version` check at login. Saves: every user re-enrolling.
7. **`commands` in stage (b) of docs/store.md** with `Idempotency-Key` on the write
   routes and `GET /api/v1/commands/{id}`: the restore API's id and
   state made general, the engine's tickets durable, `audit` pointing
   at the command. Cost: a table, the wrapper, the engine writing its
   states. Pays now: the UI's "pending" count survives a restart and
   says what is pending; a retried `PUT` does not send twice. Saves:
   the relay's queue later.
8. **Secrets stay out of the store** (open decision 2 of docs/store.md as recommended):
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

## What this adds to the decisions

Items 12 to 18 of docs/store.md section 10. Items 14 to 16 among them touch version 1
and are the ones to take with the first ten; the rest shape the
server mode and can wait for it.
