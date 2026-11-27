# ANALYSIS ServerBackup: roadmap item 4, the server's own backups

The owner's `EVAL roadmap item 4` of 2026-11-25, for the next version,
after D1 of the same day (the server's own data gets a backup of its own: A).
The owner's answers are in AP ServerBackup's Discussion;
where the two differ, the plan holds.

## TL;DR

Your four points hold. I recommend making the backup a plan of its own and
leaving the rest of item 4 for later.

- The backup is a consistent copy of the whole store plus the files beside it,
  in a zip whose manifest anyone can read and whose contents only your key opens.
- Encryption in the age format: the server keeps only the public key,
  and a file opens with the `age` tool anywhere, without Massimo.
- Restore by scope: system settings, users and sign-in, integrations,
  each Cube, history, the box's identity.
  A scope is replaced whole, and an automatic backup is taken first.
- Targets: local, WebDAV and SSH (SFTP), all built into the server;
  SMB first as a mounted share, S3 later with a small signing client of our own.
- One schedule makes each backup once.
  Each target takes the ones it wants, keeps its own retention,
  and shows up as a copy on the backup's row, with pull and push.

Open: D2–D12 below, each with a recommendation.

## 1. What is there today

- Everything of the server is in `massimo.db`:
  settings, roles and capabilities, users, sign-in providers and identities,
  tokens, preferences, push subscriptions, integrations,
  the Cubes, rooms, Cube backups, commands, the audit and the event log.
- Beside it, `opaque.key` (no password of `auth_version` 2 signs in
  and no sealed secret opens without it) and `node.key` (the box's identity).
- Outside the data directory: `/etc/massimo/env` (the API token, the listen
  address, TLS and update settings, Cube lines) and the TLS files
  (`/etc/massimo/cert.pem` and `key.pem`, or `<data>/tls/acme/`).
- `massimo store backup <file>` writes a consistent copy (`VACUUM INTO`),
  also while `serve` runs. It is for the command line only, by hand,
  unencrypted, and without the two key files.
- `massimo.db.before-<N>` is made only before a migration that raises
  min_reader. It exists for a rollback, not as a backup.
- The UI's "Backup and restore" is the Cubes' configurations only
  (design.md section 7, "device backup").
  Design.md section 7 also plans scheduled snapshots and a whole-database
  export and import; roadmap item 4 carries the latter. Neither is built.
- The rows carry a `uuid` (every table but `settings`, `backup_data` and the
  link and bookkeeping tables), so a row can be matched across boxes by uuid,
  not by id.
- Integrations are entangled with users: an integration has system users of
  its own (`users.integration_id`), identities and tokens.
- Go 1.26's standard library has `crypto/hpke` (RFC 9180).
  `golang.org/x/crypto` (which has `ssh`) and `golang.org/x/net`
  (which has a WebDAV server, for tests) are already modules of `go.mod`.
- `massimo backup` and `massimo restore` are the Cube's configuration, so the
  server's commands belong under `massimo store`.
- The plan name StoreBackups was taken and has retired; ServerBackup is free.

## 2. Item 4 and this work

**D2. How item 4 is split**

- **A (recommended).** The backup becomes a plan of its own, AP ServerBackup,
  for the next version.
  Item 4 keeps ownership per Cube, `-server`, secrets in the store,
  the logical export and import, and the PostgreSQL driver.
- **B.** One plan for the backup and design.md's export and import together.
- **C.** A backup built on the logical export: each scope as JSON,
  and the restore as an import.

Why A: they serve different purposes.
A backup is for disaster recovery and for moving to a new box.
The export is for reading, diffing, merging and moving to another database.
A snapshot of the whole store covers every table by construction:
a table added later is in the backup without new code.
Under C, each new table needs an export handler, and a missing one silently
leaves data out.
An old snapshot is brought up to date by the migrations that already exist.
The export and import can come later, and the archive leaves room for it
(`payload` names its kind).

## 3. Your points weighed

### 3.1 Encryption, metadata, zip

An asymmetric key is right.
The server can then make a backup at 03:30 without anything secret,
and a stolen box or a target that leaks does not open the backups.

**D3. The format**

- **A (recommended).** age (the C2SP spec, `filippo.io/age`).
  - Recipients can be X25519 keys (`age1…`) or the owner's existing
    `ssh-ed25519` key, several per file.
  - A file opens with `age -d -i key.txt` on any computer, without Massimo.
  - age's newer releases add a post-quantum hybrid recipient
    (its version is to be checked when the plan is written).
  - The cost is a new module. The contract says "the standard library first",
    and recovery without our own binary is the argument for the exception.
- **B.** HPKE from `crypto/hpke` in a container of our own.
  No new module, but a file opens only with `massimo store decrypt`.
- **C.** age's format written ourselves with the standard library and
  `x/crypto`, tested against the spec's test vectors.
  No module, and the files stay interoperable, but it is crypto code of our own.

**D4. The private key**

- **A (recommended).** Either the owner brings their own key or the server makes one.
  - Under System settings → Server backups, the owner pastes a recipient of
    their own (`age1…`, or an `ssh-ed25519` line from their own computer).
  - Or "Make a key" shows the secret key once, as text and as a file.
    The owner types its last six characters to confirm they kept it,
    and the server keeps only the public part.
  - Several recipients, each with a label; a backup's manifest names the
    labels and fingerprints it was made for.
  - At a restore, the key is handed in for that restore only, pasted or
    uploaded, held in memory and never written.
- **B.** A key derived from a passphrase (argon2id → X25519):
  nothing to keep but a passphrase. But a weak passphrase can be guessed
  offline against the public key that every file carries.
- **C.** Keys made only off the box (`massimo store keygen` or `age-keygen`):
  the server never sees a secret key. This is part of A anyway
  (paste a recipient); C would only drop "Make a key".

**D5. What a file shows without the key**

- **A (recommended).** A zip, the outer layer stored without compression:
  - `manifest.json`, plain: format version, time, trigger (scheduled, by hand,
    before an update, before a restore), Massimo's version,
    schema version and min_reader, the box's node fingerprint and an optional
    label the owner gives, the scopes it holds with counts
    ("3 users, 2 Cubes, 1 integration"), the recipients' labels and
    fingerprints, and the payload's size and SHA-256.
  - `payload.age`: the encrypted inner zip (deflated: the store, the key files,
    the env file, the TLS files).
  - `manifest.sig`: the manifest signed with `node.key`,
    so that a restore can say "made by this box" or "by another box (fingerprint)".
- **B.** The Cubes' serials and names and the users' names in plain, too:
  a Cube could then be picked without the key,
  but those are identifying data on a remote target.
- **C.** Nothing in plain, one age file: each file of a target would then need
  the key just to be listed.

Why A: a target's backups can be listed, and the catalogue rebuilt, without
the key. Integrity (hash, signature) can be checked without it too.
Nothing personal sits on a remote target.

### 3.2 Restore by scope

What each scope holds, from the store's tables and the files beside it:

| Scope | Rows and files |
|-------|----------------|
| System settings | `settings` (`backups.*`, `log.*`, the sealed VAPID key, …), the backup targets and their sealed secrets, the env file, the TLS files |
| Users and sign-in | `users` (not an integration's), `roles`, `role_capabilities`, `capabilities_disabled`, `auth_providers`, `user_identities` of the providers, `tokens` of kind `api`, `preferences`, `push_subscriptions`, with `opaque.key` |
| Integrations | `integrations`, their system users, their identities and those users' tokens |
| Cubes, each by serial | `cubes`, `rooms`, `backups` and `backup_data` (the Cube's configuration history), its `commands` |
| History | `audit`, `log` (the events; up to about 60 MB for 30 days), optionally per Cube |
| The box's identity | `node.key` |
| Never restored | `schema_version`, `auth_failures`, `backup_pending`, sessions (`tokens` of kind `session`), the live backup catalogue |

Anything else? Beyond your list:

- **History**, as its own scope, since it is most of the file's size.
- **The box's identity**, because moving to new hardware differs from going back.
- **The env file and TLS files**, as long as the env file still holds secrets
  (item 4's "secrets in the store" would move them).
- **The targets themselves.** They go with System settings,
  so that a new box can reach them again.

**D6. What a restore of a scope does**

- **A (recommended).** A scope is replaced whole.
  - Its live rows give way to the backup's, matched by uuid,
    with the ids renumbered.
  - A reference to another scope is kept where the row it names exists by
    uuid; otherwise the restore refuses and lists what is missing.
  - A Cube is replaced by its serial, with its rooms, its Cube backups and
    its commands. Writing its configuration onto the Cube stays the existing
    Cube restore, with its radio cost, as a separate step offered afterwards.
  - Every restore first takes an automatic backup to local, pinned.
  - A backup of an older schema is migrated on a copy first.
    A newer one is refused with "update first".
- **B.** A merge per scope (design.md's create-only, merge, replace and sync
  modes). That belongs with the logical import, later.
- **C.** The whole store only, no scopes.

**D7. A backup from another box (moving to new hardware)**

A password record only opens with the `opaque.key` it was made with,
and so does a sealed secret.
So restoring "Users and sign-in" from another box takes that box's `opaque.key`.
The live sealed secrets (providers' client secrets, the VAPID key, the targets'
secrets) are opened with the old key and sealed again with the new one,
in the same restore.

- **A (recommended).** This box keeps its identity, `node.key`,
  unless the owner ticks "take the backup's identity".
  Ticking it warns when the other box still answers on the network,
  since two boxes with one identity clash.
- **B.** A full restore always takes the backup's identity.

### 3.3 Targets

- **Local.** A directory, by default `<data>/backups/`.
  The UI says a backup on the same disk does not survive the disk.
- **WebDAV.** The standard library is enough: PUT to a temporary name,
  then MOVE; GET, DELETE, PROPFIND at depth 1, MKCOL.
  It covers Nextcloud (with an app password), Synology and storage boxes.
  Tests run against `x/net/webdav` in-process. No decision needed.

**D8. SSH (scp, SFTP)**

OpenSSH 9.0 and later run `scp` over SFTP anyway,
so the target is SFTP, which NAS boxes and storage boxes speak.

- **A (recommended).** In-process, `x/crypto/ssh` (a module already in
  `go.mod`) with `github.com/pkg/sftp`.
  - Massimo makes the target's key pair and shows the public line to put
    into `authorized_keys`.
  - The host key is pinned at the first connection, after the owner confirms
    its fingerprint.
  - Tests use pkg/sftp's server.
- **B.** The same with an SFTP client of our own (about six operations).
  No new module, but protocol code of our own on the data path.
- **C.** The system's `scp` or `sftp`. This needs no protocol code,
  but the service user `_massimo` has no home for a key or `known_hosts`.
  A listing would have to be parsed from text output,
  errors come back as localised text, and the Windows service needs
  OpenSSH's path.

**D9. SMB**

- **A (recommended) for the next version.** No SMB code: the local target
  points at a mounted share.
  - Linux: an fstab or systemd automount line (`cifs`).
  - macOS: `mount_smbfs` or autofs.
  - Windows: a UNC path.
  - packaging/README.md says how on each platform.
  - On Linux the target checks that its path is a mount point, so that a
    missing mount does not fill the SD card.
  - Weak spot: on Windows the service account needs rights on the share,
    which a NAS in a workgroup makes awkward.
- **B, the evaluation you asked for.** SMB2/3 in-process
  (`github.com/cloudsoda/go-smb2`, a maintained fork).
  It is the same on every platform and fixes Windows' service account,
  for one more module.
- **C.** `smbclient` from the system: the same drawbacks as D8-C,
  and it is not on Windows.

**D10. S3**

- **A (recommended), after the evaluation.** A subset of our own,
  standard library only: SigV4 signing; PUT, GET, HEAD, DELETE,
  ListObjectsV2.
  - Works with AWS, MinIO, Backblaze B2, Wasabi, Cloudflare R2 and Hetzner.
  - Tested against an `httptest` fake.
  - A bucket with object lock gives protection against ransomware.
- **B.** `aws-sdk-go-v2`: a large tree of modules for five calls.

### 3.4 Several targets, retention, on demand, pull and push

**D11. Schedule and targets**

- **A (recommended).** One schedule makes the backups, and each target
  decides which ones it takes.
  - Backups are made daily at a time (default 03:30), by hand from the CLI
    or the UI, and before an update or a restore.
  - Each target takes every backup, or the first of each day, week or month,
    and keeps its own retention.
  - Local is a target like the others.
  - A backup is made once and copied, so the same file (same id, same hash)
    sits wherever it is stored.
- **B.** A schedule per target, each making its own backup:
  the same point in time would then be different files on different targets.

**D12. Retention per target**

- **A (recommended).** Like restic's forget policy.
  - Keep the last N, plus the newest of each of the last D days,
    W weeks, M months and Y years.
  - A pinned backup is never pruned: one made before an update or a restore
    (pinned for 30 days), or one pinned by hand.
  - A copy pulled to local stays until it is deleted.
  - The newest copy on a target is never pruned.
- **B.** A maximum count and a maximum age only.

**The catalogue, pull and push** (a shape, not a decision):

- The store keeps the server backups (id, time, hash, size, manifest) and
  their copies per target (where, when pushed, when last checked).
- Listing a target adds the files it finds there, by their plain manifests,
  so a new box sees a target's backups as soon as the target is added.
- In the UI, under System settings → Maintenance → Server backups,
  each backup is a row with its copies (local, WebDAV, SSH, each present or not).
  Its actions: push to, pull from, download, upload (a file from elsewhere,
  into local), delete a copy, check, restore….
- A restore never replaces the live catalogue.
- The CLI goes through the running `serve` when there is one, as commands go
  through its proxy, and acts on the store directly otherwise:
  - `massimo store backups` lists them;
  - `store backup [-target <name>]` makes one;
  - `store push` and `store pull <id> <target>` copy one;
  - `store restore <file|id> -scope users,cube:KEQ0000000 -key <file>` restores;
  - `store keygen` and `store decrypt` handle keys and files.
  - The plain `store backup <file>` stays as it is.

## 4. Additions worth having

1. **Before an update**, `massimo update` takes a backup to local, pinned.
   If it cannot, it says so and stops, unless `-no-backup` is given.
2. **A new box from a backup.** The first-owner card offers
   "Start from a backup": upload a file and its key, or add a target and pick.
   This touches AP FirstOwner's card and AP StartConfiguration's start.
3. **Alerts.** A failed backup or push is a notification and a Warn in the log.
   Maintenance shows each target's last success,
   and a target two periods behind is marked.
4. **Checks.** A pushed copy is read back for its size and hash, which needs
   no key. "Check with the key" decrypts a backup, opens its store and runs
   `PRAGMA integrity_check`. A backup nobody has ever opened is a hope, not a backup.
5. **Who may.** A new capability, `server-backup`, covers making, pushing and
   pulling. Setting up the key and restoring stay with the owner alone.
   Each of these is an audit line.

## 5. Settled and open

- **Settled:**
  - D1-A: the server's own data gets a backup of its own, for the next version.
  - Your points: an asymmetric key with the public part on the server,
    readable metadata, a zip, and restore by scope with the scopes listed.
  - Local, WebDAV and SSH come first; SMB and S3 are evaluated.
  - Several targets with their own retention, backups on demand from the CLI
    and the UI, and each backup's copies shown, with pull and push.
- **Open:** D2 to D12 above, recommended A each,
  and whether the additions of section 4 belong in the plan.
- **A sketch of the blocks** for the plan, to be refined when it is written:
  1. the archive, the keys and `store keygen`/`decrypt`;
  2. the catalogue, the local target, the schedule, retention and the UI's list;
  3. the restore by scope, with the backup taken before it;
  4. WebDAV;
  5. SSH;
  6. the backup before an update;
  7. a new box from a backup.

  SMB in-process and S3 follow once evaluated.
