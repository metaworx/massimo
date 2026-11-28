# AP ServerBackup v1.0: Encrypted backups of the server

## TL;DR

The server backs up its whole store and the files beside it,
encrypted for the owner's key, onto several targets,
and restores one scope at a time; for the next version
(`wip/2026-11-25_10-31_ANALYSIS_ServerBackup_roadmap_item_4.md`).

- `store backup <file>`, by hand, open → a scheduled zip:
  plain manifest, `payload.age`, a signature by `node.key`
- the key → its public part alone on the server; pasted, or made and shown once
- targets → local, WebDAV, SFTP, SMB, S3; each its own backups and retention
- a backup → one file, its copies per target; push, pull, download, upload, check
- restore → by scope, each replaced whole, a pinned backup first; the owner's alone
- a failure → retried three times, then an alert
- rights → `backup.server`, `backup.retry`; the update's backup under `update`
- UI → Maintenance: Server backups, and Cube backups moved from System settings
- also → a backup before each update; a new box from a backup

### Blocks

1. Archive: the zip, the age payload, the signed manifest; `store keygen`, `store decrypt`
2. Catalogue: tables, local target, schedule, retention, retries, alerts, rights, API, CLI
3. CubeBackups: System settings' Backup and restore moves to Maintenance
4. ServerCard: Maintenance's Server backups card
5. Restore: by scope, from the CLI, the API and the card; the check with the key
6. WebDav: the WebDAV target
7. Sftp: the SFTP target
8. Smb: the SMB target
9. S3: the S3 target
10. BeforeUpdate: a pinned backup before each update
11. NewBox: a new box started from a backup, in the first-owner card

## Discussion

### What is decided

The owner, on the analysis (2026-11-25)
and on the TL;DR gates of 2026-11-25 and 2026-11-27 (`PLAN+ as recommended`):

- **D1-A.** The server's own data gets a backup of its own,
  for the next version, not for 0.17.
- **D2-A.** A plan of its own, and a snapshot of the whole store,
  not a logical export.
  Roadmap item 4 keeps ownership per Cube, `-server`, secrets in the store,
  the logical export and import, and the PostgreSQL driver.
- **D3-A.** The age format, `filippo.io/age`:
  a file opens with `age -d` on any computer, without Massimo.
- **D4-A.** The owner pastes a recipient of their own,
  or "Make a key" shows the secret key once, as text and as a file,
  confirmed by typing its last six characters.
  Several recipients, each with a label.
  At a restore the key is handed in for that restore alone,
  held in memory and never written.
- **D5-A.** A zip, stored without compression:
  `manifest.json` in plain (no names of Cubes, rooms or users),
  `payload.age` (the inner zip, deflated),
  and `manifest.sig`, the manifest signed with `node.key`.
- **D6-A.** A scope is replaced whole:
  - its rows matched by uuid, their ids renumbered;
  - a reference to a row of another scope that is missing refuses the restore,
    with the list of what is missing;
  - a Cube by its serial, with its rooms, Cube backups and commands;
    writing its configuration onto the Cube stays the Cube restore,
    offered afterwards;
  - every restore first takes a local backup, pinned;
  - a backup of an older schema is migrated on a copy,
    a newer one refused with "update first".
- **D7-A.** `opaque.key` travels with Users and sign-in,
  and the live sealed secrets are sealed again with it.
  This box keeps its `node.key` unless "take the backup's identity" is ticked,
  which warns when the other box still answers.
- **D8-A.** SFTP in-process: `golang.org/x/crypto/ssh` with `github.com/pkg/sftp`.
  Massimo makes the target's key pair,
  and the host key is pinned once the owner confirms its fingerprint.
- **D9-B.** SMB in-process, `github.com/cloudsoda/go-smb2` (D16 below).
- **D10-A.** S3 as a subset of our own:
  SigV4; PUT, GET, HEAD, DELETE and ListObjectsV2.
- **D11-A.** One schedule (daily, 03:30 by default) makes each backup once;
  each target takes every backup, or the first of each day, week or month.
- **D12-A.** Retention per target, as restic's:
  the last N, and the newest of each of D days, W weeks, M months, Y years.
  A pinned backup is kept (30 days for one before an update or a restore);
  a copy pulled to local stays until it is deleted;
  a target's newest copy is never pruned.
- **The additions** of the analysis's section 4, all:
  a backup before an update, a new box from a backup, alerts, checks,
  the capability, and audit lines.
- **The update's backup** needs no backup capability:
  it runs as the server, under whoever holds `update`.
- **D13-A, under the owner's names.**
  - `backup.server` makes, pushes and pulls.
  - `backup.retry` shows the backups' state and retries a failed backup or copy,
    nothing more, and is seeded for `manager`.
    A retry repeats what the schedule decided,
    reveals nothing, since the file is encrypted,
    and prunes no more than the schedule would.
  - "Back up now" stays with `backup.server`:
    repeated, it could push older backups out of a target that keeps the last N.
- **D14-C.** On the Maintenance page:
  a card "Server backups",
  and System settings' Backup and restore moved there as "Cube backups",
  two cards of their own beside Software update and Maintenance.
- **D15-A.** Neither capability is in the integration's ceiling,
  and both are on the list a sign-in provider's role may not hold
  (`adminCapabilities`):
  a server backup holds what the integration is kept from,
  people, sign-in and the integrations.
- **The key and the restore are the owner's alone,** with no capability:
  restoring Users and sign-in from a backup in which one was owner
  makes one owner, and a key of one's own reads every later backup.

### Open

Two, found while the plan was written:

- **D16. The SMB module.**
  `cloudsoda/go-smb2` has no release (a pseudo-version of 2026-11-05)
  and links Kerberos: `jcmturner/gokrb5/v8` with five more `jcmturner` modules,
  `hashicorp/go-uuid`, `cloudsoda/sddl` and `geoffgarside/ber`.
  - **A (recommended).** The fork, pinned at a commit,
    and checked again when Block 8 starts:
    it is maintained and speaks SMB 3.1.1; Dependabot moves the pin.
  - **B.** The original, `hirochachacha/go-smb2` v1.1.0:
    a release and one module beside it,
    but unmaintained since 2022, while it parses what a NAS answers.
  - **C.** Back to D9-A: a mounted share as a local target, no module;
    Windows' service account then needs rights on the share.
- **D17. The key "Make a key" and `store keygen` make.**
  age v1.3 adds a post-quantum hybrid recipient
  (ML-KEM-768 with X25519, `age1pq1…`),
  which age refuses to mix with other recipients in one file.
  - **A (recommended).** A post-quantum key:
    a backup lives for years, and a copy taken today stays closed
    to a quantum computer later.
    Its recipient is about 2000 characters,
    so the card shows it as a file and a copy button.
    A box's recipients are then all post-quantum or all not,
    and the card says why it refuses a mix.
  - **B.** X25519 (`age1…`, 62 characters):
    short, and it mixes with an `ssh-ed25519` line, but it is not post-quantum.

### Decided in the writing

- **Who holds what.**
  - `backup.server` is seeded for no role, as `auth` and `reboot` are:
    the owner gives it.
    It covers making, pushing, pulling, downloading, uploading,
    deleting a copy, the size-and-hash check,
    and the targets with their retention.
  - `backup.retry` is seeded for `manager`.
  - The recipients, the restore and the check with the key are the owner's.
- **No key, no backups.** Until a recipient is set,
  the schedule makes nothing and the card says so;
  the backup before an update is skipped with a line in the update's log,
  and the update goes on.
- **The update's backup failing** stops the update.
  "Update without a backup" (`massimo update -no-backup`)
  is offered to whoever holds `update`, and is an audit line.
- **The env file and the TLS files** are restored only where ticked,
  each on its own, since a new box's paths can differ;
  the file replaced stays beside as `<name>.before-restore`.
- **The CLI** goes through a running `serve` (the API, with the owner's key)
  where one answers, and acts on the store directly otherwise;
  a restore while a `serve` runs is always that server's.
- **Builders:** Opus sub-agents, a block each, on branches of their own;
  the coordinating session reviews, commits signed and opens the pull requests.
- **Pull requests:** four, each mergeable alone, from `main`:
  - `claude/server-backup-core`: Blocks 1 and 2, the CLI and the API;
  - `claude/server-backup-ui`: Blocks 3 to 5;
    Block 3 needs nothing of Blocks 1 and 2, and may go first;
  - `claude/server-backup-targets`: Blocks 6 to 9;
  - `claude/server-backup-start`: Blocks 10 and 11.

## Analysis

The design, its options and what each scope holds are in
`wip/2026-11-25_10-31_ANALYSIS_ServerBackup_roadmap_item_4.md`;
where it and this plan differ, the plan holds
(the card's place, D14-C; the SMB module, D9-B and D16).
What it does not hold, and what each block rests on:

### The modules, checked 2026-11-27

| Module | Version | Licence | Modules it adds | Bare binary |
|--------|---------|---------|-----------------|-------------|
| `filippo.io/age` | v1.3.2, 2026-08-29 | BSD-3-Clause | `filippo.io/hpke` | +0.8 MB |
| `github.com/pkg/sftp` | v1.13.13, 2026-11-24 | BSD-2-Clause | `github.com/kr/fs` | +1.4 MB, with `x/crypto/ssh` |
| `github.com/cloudsoda/go-smb2` | pseudo-version, 2026-11-05 | BSD-2-Clause | `cloudsoda/sddl`, `geoffgarside/ber`, `jcmturner/gokrb5/v8` and five more `jcmturner`, `hashicorp/go-uuid` | +2.5 MB |
| `github.com/hirochachacha/go-smb2` | v1.1.0, 2022-03-12 | BSD-2-Clause | `geoffgarside/ber` | +0.8 MB |

- "Bare binary": a stripped program that imports the module alone,
  against 1.6 MB without it; Massimo's binary shares part of each.
- age's `filippo.io/edwards25519` and `filippo.io/nistec` are in `go.sum` already.
- Each asks for Go 1.25 at most; `go.mod` has 1.26.
- `testify`, which each requires, is a test module of theirs and is not linked.
- Every licence is compatible with the AGPL.

### For Block 1: the archive's order

- The payload is written first, to a temporary file, and hashed;
  the zip then takes `manifest.json` first, `payload.age`, `manifest.sig`.
  The manifest is found through the zip's central directory,
  so every target lists its backups by ranged reads,
  without fetching a whole file.
- Nothing is held whole in memory: a Pi has 1 GB.
  The snapshot and the payload take about twice the store on disk;
  that is checked first, and refused with the numbers.
- The snapshot is the store's own `copyTo` (`internal/store/migrate.go:465`,
  `VACUUM INTO`), which `store backup <file>` uses already.
- `node.key` signs (`identity.go:139`); the store has no verify helper yet,
  the only `ed25519.Verify` is the update's (`internal/update/update.go:498`).

### For Block 2: who passes a route

- No route is the owner's alone today: a route takes a capability,
  `capNone` or `capAny` (`internal/server/auth.go`).
  A third value, `capOwner`, admits the role `owner` alone,
  which no integration and no other role holds.
- `includes` (`auth.go:114`) gets `backup.server` ⊇ `backup.retry`,
  so that a holder of the first sees the state and retries too.
- Every list of capabilities moves together:
  `auth.go` `capabilities`, `oidc.go` `adminCapabilities`,
  `ui/login.js` `CAPABILITIES`, `ui/src/admin/sendProviders.ts` `ADMIN_CAPS`,
  `ui/src/model/cap.ts` `CAP_INCLUDES`, the `cap.*` keys of the six
  `ui/i18n/*.json`, the tables of `docs/store.md` and `docs/api-v1.md`
  (`TestCapabilityTableCoversRoutes`), and the tests' own lists
  (`internal/store/roles_test.go` `allCaps`, `TestSeededRoles`,
  `internal/server/capabilities_test.go` `roleSets`).
  `integrationCeiling` and `ui/src/integrations/store.ts`
  `INTEGRATION_CEILING` stay as they are (D15-A).
- The migration takes the next number of `main`'s block of 100
  (0100 or after; `docs/store.md`, "Two lines of releases"),
  with a down part, and seeds `backup.retry` as 0022 seeds `repair`.

### For Block 5: references between scopes

- Each table of the store belongs to one scope, in one list,
  and a test fails on a table that has none:
  a table added later must be placed, or a restore would leave it out.
- The references are read from the schema (`PRAGMA foreign_key_list`),
  so a later table's are checked without new code.
- A row's id is renumbered on insert, and its children follow
  through a map of old ids to new.
- A sealed secret is opened with the backup's seal key and sealed with the
  live one (`internal/store/secret.go`, `openSecret` and `sealSecret`),
  in the restore's transaction.

### For Blocks 8 and 9: tests against real servers

- Go has no SMB server to test against, and an S3 fake proves only our reading
  of the protocol.
  `make target-test` runs Samba and MinIO in Docker, as `make lan-test` does
  (`tools/lan-test/`), and a CI job of its own runs it;
  the contract's test gate gets a line for a change to a target.
- WebDAV and SFTP need no container:
  `golang.org/x/net/webdav` and `pkg/sftp`'s server run in-process.
- SigV4 is checked against AWS's documented example signatures as well.

### For Block 11: the first owner's window

- Until the first regular user exists, the server is open
  (`internal/server/opensetup.go`), and whoever reaches it may make that user.
  Starting from a backup in that window gives nobody more than that,
  and it still needs the file and its key.
- The card is AP FirstOwner's (built, its retirement open),
  and AP SecondFactor, for the next version too, changes the sign-in card;
  whichever comes second merges the other.

## Implementation Plan

Each block ends at its Verification.
A block's docs come with it (`docs/store.md`, `docs/api-v1.md` and the
OpenAPI file, `docs/ui.md`, `README.md`), and each block carries
a `CHANGELOG.d/` fragment, its paths being shipped.

### Block 1: Archive

- **Goal:** a server backup is made, read and opened on the command line.
- **Changes:**
  - `go.mod`, `go.sum`: `filippo.io/age` (D3-A).
  - `internal/serverbackup`, a new package:
    - `Write`: the snapshot, the files beside it (`opaque.key`, `node.key`,
      the env file `serve` reads, the TLS files), the inner zip deflated,
      age for the recipients, the outer zip stored (D5-A);
    - `ReadManifest`: the manifest and the signature's verdict,
      from an `io.ReaderAt`, without the key;
    - `Open`: the payload's hash checked, then decrypted into a directory;
    - `ParseRecipient`: `age1pq1…`, `age1…`, `ssh-ed25519`;
      a box's recipients all post-quantum or all not (D17).
  - `internal/store`: `Store.Snapshot(path)` beside `Backup`, both `copyTo`;
    `VerifyNode`, a manifest's signature by the key it names.
  - `cmd/massimo/store.go`: `store keygen [-out <file>]`, of the kind D17 settles,
    `store inspect <file>`, `store decrypt <file> -key <file> -out <dir>`.
  - Docs: `docs/store.md`, a section "Server backups" (the archive,
    the manifest's fields, the signature, the kinds of key);
    `README.md`, the three commands.
- **Tests:** a round trip for each kind of recipient; a mix refused;
  a changed payload and a changed manifest each refused;
  the manifest read without the key; a snapshot taken while a writer writes;
  a file opened by age's own `Decrypt`, as `age -d` would;
  too little space refused with the numbers.
- **Commit:** `feat(store): A server backup is an encrypted archive`
- **Verification:** `go test` of `internal/serverbackup`, `internal/store`
  and `cmd/massimo`; `gofmt -l .`; `go vet` for linux, windows and darwin;
  `make cross-check`.

### Block 2: Catalogue

- **Goal:** the server makes its backups on a schedule,
  keeps them with their retention on the local target,
  retries and alerts, and the API and the CLI reach them.
- **Changes:**
  - A migration (main's next number): `server_backup_keys`
    (label, recipient, fingerprint, kind), `server_backup_targets`
    (name, kind, configuration, sealed secret, what it takes, retention),
    `server_backups` (time, trigger, by whom, size, SHA-256, manifest,
    pinned until), `server_backup_copies` (backup, target, path, pushed,
    checked, state, error, tries, next try); `backup.retry` seeded for
    `manager`; the setting `serverbackup.time`, 03:30; a down part;
    `store.sql` written again.
  - `internal/store/serverbackups.go`: the catalogue;
    a target's secret sealed with its uuid as the AAD.
  - `internal/serverbackup`: the `Target` interface (put by a temporary
    name, ranged get, list, stat, delete); `Local`, `<data>/backups/`;
    `Keep`, the retention of D12-A; the scan of a target by its manifests.
  - `internal/server/serverbackups.go`:
    - the loop, started in `Server.Run` beside `pruneBackupsDaily`:
      the daily time, a run missed made 5 minutes after the start,
      the copies by what each target takes, the retention after each;
    - retries after 5 min, 30 min and 2 h, then a notification
      (a kind of `internal/notify`), a Warn line and a log alert
      (`logalerts.go`, beside `backup`);
    - an audit line for each action by hand (`auditAction`);
    - the routes:
      - `GET /api/v1/server-backups`, the list with its copies
        and each target's last success (`backup.server` or `backup.retry`);
      - `POST /api/v1/server-backups`, one now;
        `PUT …/{id}/pin`; `POST …/{id}/check`, size and hash;
        `GET …/{id}/file`; `POST /api/v1/server-backups/upload`
        (`backup.server`);
      - `POST …/{id}/retry` (`backup.retry`);
      - `PUT`/`DELETE …/{id}/copies/{target}`, a push and a copy deleted,
        `POST …/{id}/copies/{target}/pull` (`backup.server`);
      - `GET`/`POST /api/v1/server-backup-targets`,
        `PUT`/`DELETE …/{uuid}`, `POST …/{uuid}/test` (`backup.server`);
      - `GET /api/v1/server-backup-keys` (`backup.server`),
        `POST` and `DELETE …/{uuid}` (`capOwner`).
  - `internal/server/auth.go`: `backup.server`, `backup.retry`,
    `capOwner`, the inclusion; the other lists of the Analysis.
  - `cmd/massimo/store.go`: `store backups`; `store backup` without a file,
    one now (`-target <name>`; with a file it stays the plain copy);
    `store push <id> <target>`, `store pull <id> <target>`:
    through a running `serve` with `MASSIMO_API_TOKEN`, on the store otherwise.
  - Docs: `docs/store.md` (the tables; section 6's capabilities and seeds;
    "Server backups"), `docs/api-v1.md` ("Server backups", the capability
    table and list), the OpenAPI file, `README.md`,
    `docs/design.md` section 7 (the snapshots built).
- **Tests:** `Keep` against restic's documented examples;
  the schedule with a fake clock (a missed run, the time across a DST change);
  the retries and the alert after the third;
  a route's capability for each (`TestOpenAPICoversRoutes`,
  `TestCapabilityTableCoversRoutes`, `roleSets`); the seeds;
  the local target's temporary name; a scan adding a file it finds;
  a sealed target secret; the CLI both ways.
- **Commits:** the store's, the package's, the server's and the CLI's,
  `feat(store): …`, `feat(server): …`, `feat(cli): …`.
- **Verification:** `go test ./...`; `make race` for `internal/server`
  and `internal/store`; `gofmt -l .`; `go vet` ×3; `make cross-check`.

### Block 3: CubeBackups

- **Goal:** System settings' Backup and restore becomes Maintenance's
  "Cube backups" (D14-C), for whoever holds `backup`.
- **Changes:**
  - `internal/server/ui.go` `uiPartials`: `backup.html` from `system-end`
    to `maint`; its title "Cube backups", in the six `ui/i18n/*.json`.
  - `ui/src/shared/me.ts` `settingsPageOf` and `ui/login.js`
    `hasSettingsPage` without `backup`; `systemOf` keeps it.
  - The Cube page's text that names System settings → Backup and restore.
  - Docs: `docs/ui.md` (System settings, Maintenance, the Cube's settings,
    the parts table).
- **Tests:** `tools/ui-test`: `test_backup.py`, `test_system_layout.py`,
  `test_shell_updates.py` (the reads move from `system` to `maint`),
  `test_parts_guard.py`, `test_admin_updates.py`, `test_sign_in_resend.py`,
  `test_cube_page.py`, `screenshots.py`;
  a user with `backup` alone sees Maintenance and no System settings.
- **Commit:** `feat(ui): Cube backups move to Maintenance`
- **Verification:** `make ui-check`; the touched UI files in the four cases;
  `go test ./internal/server`.

### Block 4: ServerCard

- **Goal:** Maintenance's "Server backups" card.
- **Changes:**
  - `ui/partials/server-backups.html` in `maint`, after Software update;
    `ui/src/serverbackups/`, a region of its own:
    - the keys (the owner): paste a recipient; "Make a key", shown once,
      as text and as a file, confirmed by its last six characters;
      a label each; a mix of kinds refused, with the reason;
    - the targets: add, change, test, what each takes, its retention,
      its last success, and a mark two periods behind;
    - the backups: a row each (time, trigger, size, pinned),
      its copies per target (there, missing, failed),
      the actions its holder may take; a `backup.retry` holder sees
      the state and "Retry", nothing more.
  - `systemOf` with `backup.server` and `backup.retry`; the six i18n files.
  - Docs: `docs/ui.md` (the card, the parts table, "The data").
- **Tests:** `tools/ui-test/test_server_backups.py`: no key; a key made and
  confirmed; a target added; a backup made, a copy failed and retried by a
  user with `backup.retry` alone; the reads of `maint` in
  `test_shell_updates.py`; `screenshots.py`.
- **Commit:** `feat(ui): Maintenance shows the server's backups`
- **Verification:** `make ui-check`; the touched UI files in the four cases;
  `go test ./internal/server`.

### Block 5: Restore

- **Goal:** the owner restores chosen scopes of a backup (D6-A, D7-A),
  and checks one with the key.
- **Changes:**
  - `internal/serverbackup/scopes.go`: each table in one scope;
    the restore: decrypted into a temporary directory, its store migrated
    where older and refused where newer, a pinned local backup first,
    each scope replaced in one transaction, references checked,
    `opaque.key` with Users and sign-in and the secrets sealed again,
    `node.key`, the env file and the TLS files where ticked;
    then the server's restart; the Cube restore offered for each Cube.
  - Routes (`capOwner`): `POST /api/v1/server-backups/{id}/restore/plan`
    (the scopes, their counts, what is missing),
    `POST …/{id}/restore` (the key in the body, in memory alone),
    `POST …/{id}/check-key` (decrypt, open, `PRAGMA integrity_check`).
  - `cmd/massimo/store.go`: `store restore <file|id> -scope <list>
    -key <file> [-identity] [-env] [-tls]`.
  - The card: "Restore…" and "Check with the key", for the owner.
  - Docs: `docs/store.md`, `docs/api-v1.md`, the OpenAPI file,
    `docs/ui.md`, `README.md`.
- **Tests:** a store with two Cubes, users and an integration:
  each scope alone and all together; a missing reference refused with its list;
  an older backup migrated, a newer refused; a backup of another box:
  a user signs in with the old password and a provider's secret opens;
  `node.key` kept unless ticked; the pinned backup first;
  no key file written; every table in a scope; the dialog in a UI test.
- **Commits:** `feat(store): …` for the restore, `feat(ui): …` for the card.
- **Verification:** `go test ./...`; `make race` for `internal/server`
  and `internal/store`; `make ui-check`; the touched UI files in four cases.

### Block 6: WebDav

- **Goal:** a WebDAV target.
- **Changes:** `internal/serverbackup/webdav.go`: PUT to a temporary name,
  then MOVE; GET with a range; DELETE; PROPFIND at depth 1; MKCOL;
  basic authentication, its password sealed; a client with a timeout;
  the card's form; docs.
- **Tests:** against `golang.org/x/net/webdav` in `httptest`:
  a PUT broken off leaves nothing under the final name; a 401; a 507.
- **Commit:** `feat(server): A WebDAV target for the server's backups`
- **Verification:** `go test ./internal/serverbackup`; the card's UI test;
  `gofmt`, `go vet` ×3.

### Block 7: Sftp

- **Goal:** an SFTP target (D8-A).
- **Changes:** `go.mod`: `github.com/pkg/sftp`;
  `internal/serverbackup/sftp.go`: an Ed25519 pair per target, its private
  part sealed, its public line shown; the host key pinned once the owner
  confirms its fingerprint, a changed one refused with both;
  a temporary name, then a rename; the card's form; docs.
- **Tests:** `pkg/sftp`'s server behind an in-process `x/crypto/ssh` server;
  a changed host key; a refused key.
- **Commit:** `feat(server): An SFTP target for the server's backups`
- **Verification:** as Block 6.

### Block 8: Smb

- **Goal:** an SMB target (D9-B, the module of D16).
- **Changes:** `go.mod`; `internal/serverbackup/smb.go`: user and password
  by NTLM, the password sealed; share and path; a temporary name, then a
  rename; the card's form; `tools/target-test/` with Samba,
  `make target-test`, a CI job for it; the contract's test gate
  (`GUIDELINES/project/_CONTRACT.md`, inlined by `sync-docs.sh`); docs.
- **Tests:** the target's logic against a fake of the client;
  `make target-test`.
- **Commit:** `feat(server): An SMB target for the server's backups`
- **Verification:** as Block 6; `make target-test`; `make actionlint`.

### Block 9: S3

- **Goal:** an S3 target (D10-A).
- **Changes:** `internal/serverbackup/s3.go`: SigV4 with the standard library;
  PUT, GET with a range, HEAD, DELETE, ListObjectsV2;
  path and virtual-host style, region, access key with its secret sealed;
  a DELETE that the bucket's object lock refuses is kept, not failed;
  MinIO in `tools/target-test/`; the card's form; docs.
- **Tests:** AWS's example signatures; an `httptest` fake that checks each
  signature; `make target-test`.
- **Commit:** `feat(server): An S3 target for the server's backups`
- **Verification:** as Block 8.

### Block 10: BeforeUpdate

- **Goal:** every update takes a pinned local backup first.
- **Changes:** `internal/server/update_api.go` `beginUpdate`, which the night's
  update (`update_auto.go`) and the answer (`update_answer.go`) call too:
  the backup before the helper starts, its id in the update's history;
  a failure stops the update and offers `no_backup`, an audit line;
  no key: a line in the update's log, and on;
  `cmd/massimo/update.go`: `-no-backup`;
  `ui/src/software/`: the line, and "Update without a backup";
  docs: `docs/api-v1.md`, the OpenAPI file, `docs/ui.md`, `README.md`.
- **Tests:** the helper starts after the backup; a failed backup stops it;
  `no_backup` goes on with its audit line; no key goes on;
  the night's update takes one too; the card's line in a UI test.
- **Commit:** `feat(update): An update takes a backup of the server first`
- **Verification:** `go test ./...`; `make race` for `internal/server`;
  the touched UI files in four cases.

### Block 11: NewBox

- **Goal:** a new box starts from a backup, in the first-owner card.
- **Changes:** `ui/src/signin/` (`Owner.tsx`, `store.ts`, `Card.tsx`):
  "Start from a backup": a file and its key, or a target added and one of
  its backups picked; every scope restored (D6-A), the identity as D7-A;
  the route open while the server is (`internal/server/opensetup.go`),
  refused once a regular user exists; the page then signs in as before;
  docs: `docs/ui.md`, `docs/api-v1.md`, the OpenAPI file,
  `README.md` ("Moving to a new box").
- **Tests:** a fresh server started from another's backup, signed in with
  the old password; the route refused once a user exists; the UI test.
- **Commit:** `feat(ui): A new box starts from a backup`
- **Verification:** `go test ./internal/server`; `make ui-check`;
  the touched UI files in four cases.

### Before each pull request

The branch merges `origin/main`; `GUIDELINES/shared/tools/check` passes;
the whole UI suite once in one case on the committed tree;
the pull request's text in commit form, each block's proofs in it.

## Proposed commit message

The plan: `chore(aps): +ServerBackup v1.0, +ANALYSIS ServerBackup`

Block 1:

```
feat(store): A server backup is an encrypted archive

For AP ServerBackup v1.0, Block 1 (D3-A, D5-A): a zip with a plain manifest, the store and the files beside it encrypted with age for the owner's recipients, and the manifest signed by node.key; massimo store keygen, inspect and decrypt make a key, read a file without one and open it with one.

Key changes:
- internal/serverbackup: Write, ReadManifest, Open, ParseRecipient.
- internal/store: Snapshot and VerifyNode.
- cmd/massimo/store.go: store keygen, inspect, decrypt.
- go.mod: filippo.io/age.
- docs/store.md "Server backups"; README.md; a CHANGELOG.d fragment.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-27): first revision, from the owner's `PLAN+ as recommended`
  on the TL;DR gate of 2026-11-27:
  D1–D12 as recommended but D9-B, D13-A as `backup.server` and `backup.retry`,
  D14-C, D15-A; D16 and D17 open.
