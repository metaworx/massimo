# NOTE ServerBackup v1.0: how the run went

The owner's `EXEC++` of 2026-11-27 ran Blocks 1 to 11,
with no merge, no retirement and no release,
as the gate decisions' NOTE says.
Each block's proofs are in its pull request's text.
This NOTE records where the run went otherwise than the plan and that NOTE,
what it found outside the plan, and what is left for the owner.

## How it was run

Four waves of Opus builders, as many at once as the blocks allowed,
each in a worktree and on a branch of its own:

| Wave | Blocks | Built on |
|------|--------|----------|
| 1 | 1 and 3 | `main` |
| 2 | 2 | Block 1 |
| 3 | 4, 5 without its card part, 6, 7, 8, 9, 10 | Block 2, and Block 4 on Block 3 too |
| 4 | 5's card part, the targets' pieces of the card, 11 | Blocks 1 to 9 stacked |

- **Wave 4** took the targets' pieces of the card as well,
  which no wave of the gate's NOTE named:
  the SFTP target's key and host key, the secret asked for again,
  and the Delete of a backup no target holds.
- **A second reader**, an Opus sub-agent that had not built the block,
  read each block or group:
  Blocks 1, 2, 3, 4 and 10 each, Block 5's store and routes,
  the four targets together, wave 4's two card parts together,
  and Block 11 for its security.
  Its findings went back to the builder,
  for the targets to the assembler,
  as fix commits on the block.
- **An assembler**, one sub-agent at a time in the coordinating session's worktree,
  stacked the builders' commits by signed cherry-picks,
  and joined what two blocks both touched:
  each block's group of texts at the end of the six i18n files,
  the targets' sections of `docs/store.md` in one order,
  every addition to `target.go`, the OpenAPI file with `api.d.ts` written again,
  `go.mod` tidied and the credits made again.
  Block 5's builder had taken Block 2's fixes onto its own branch;
  the assembler took Block 5's own commits alone.
  Every commit of the stack builds.

## The pull requests

Four, stacked in one line, each branch `claude/server-backup-<name>`:
`core` (Blocks 1 and 2) on `main`,
`targets` (Blocks 6 to 9) on `core`,
`ui` (Blocks 3, 4 and 5, with the targets' pieces of the card) on `targets`,
and `start` (Blocks 10 and 11) on `ui`.
They merge in that order.

The gate's NOTE, step 3, put `ui` and `targets` side by side on `core`.
The card's pieces for the targets,
Block 6's note beside a field and Block 7's key and host key,
need the card and the targets on one branch,
so the stack became one line.
The targets need nothing of the card,
so they come first,
and all the card's work sits in the `ui` pull request,
Block 5's card part included.
Block 10, built on Block 2,
met the targets, the restore and the card first when `start` was put together.

## Block by block

- **Block 1, Archive.**
  `store keygen -kind pq|classic` takes the kind, with no default (D17-C);
  `Snapshot` counts the copy's rows per scope,
  and `files.json` names each inner file's role, for Block 5.
  The reader found nine faults:
  another box's signature taken without a word,
  the payload hashed and decrypted from two reads,
  the TLS files' roles lost, a crash leaving a plain copy of the store,
  sizes that overflow, the manifest's text unchecked, no `ErrNewerFormat`,
  Windows device names, and empty cleanup tests.
  "fix(store): A server backup opens what it hashed and names its files' roles"
  fixes them, and `store decrypt` says which box signed.
- **Block 2, Catalogue.**
  Migration 0100, the first of `main`'s block of 100.
  Beyond the plan:
  `GET /api/v1/server-backup-targets/kinds` (the gate's NOTE, step 1),
  a key's confirmation route,
  `DELETE /api/v1/server-backups/{id}` for a backup no target holds,
  the jobs' lock that `serve` and the commands share,
  and `serve.api`, the address and node `serve` publishes in its data directory,
  so that the commands find it and send the key to it alone.
  A backup is a failed row until its first copy is there;
  only the schedule's backups are retried,
  and one made by hand that fails is not kept.
  The reader found seven faults:
  the loop spinning after an early failure, retries without end,
  a made row without a copy after a cancel,
  backups by hand retried and a retry dropping its target and pin,
  a make alert never settled,
  the commands beside `serve` without a lock and sending the key unproven,
  and a nested secret in the audit.
  Three commits fix them:
  "fix(store): A server backup's retries end,
  keep their request and outlive a stop",
  "fix(server): The server backups' loop waits,
  settles its alerts and keeps secrets out of the audit",
  and "fix(cli): The server backups' commands find this box's serve and share its lock".
  Block 4's build found that a store opened at "." refused its local target;
  "fix(store): The catalogue takes a store opened in its own directory"
  fixes it on `core`.
- **Block 3, CubeBackups.**
  The card Cube backups comes after the Maintenance card,
  its title the new key `backup.cubesTitle`.
  A role with `backup` and none of System settings' capabilities,
  the operator's, now has Maintenance and no System settings,
  as the EVAL of D14-C said;
  `test_first_owner.py` takes the Integrations card as high as the shorter page lets it.
  The reader found no fault;
  it saw that `systemOf` and `hasSystem` lacked `settings`,
  older than the plan, which Block 4 fixed.
- **Block 4, ServerCard.**
  The card sits between Maintenance and Cube backups,
  not after Software update as the plan says,
  so that the two backup cards stand together.
  Its target form is drawn from the kinds the server describes (the gate's NOTE, step 1),
  with a field's `warn`, the note Block 6 gives a plain http URL;
  `saveResponse` moved to `shared/save.ts`, and `ownerOf` joined `me.ts`.
  The reader found seven faults:
  an unknown kind's fields reset, a key made lingering,
  a secret kept across a change of kind, a switch's default,
  Delete's warning, the warn note's description and text, and smaller ones.
  "fix(ui): Server backups keep a target's fields and show a key made once" fixes them.
  In wave 4, "feat(ui): The card deletes a backup that no target holds"
  uses Block 2's new route;
  where every copy of a made backup is missing,
  the backup's Delete shows beside the copies' own,
  and stays, since it works with the targets offline.
  A UI test failed once in 1100-de:
  the run the server makes up 5 minutes after its start added a backup,
  and the default retention pruned the test's own.
  "test(ui-test): The server backups' tests keep their own backups"
  gives the tests' targets the last 50.
- **Block 5, Restore.**
  Its store, routes and command came in wave 3,
  its card part in wave 4, in the `ui` pull request (the gate's NOTE, step 2).
  Beyond the plan: `-plan` for the command;
  `Restored.Committed`, after which the server restarts whatever else failed;
  users and sign-in or the identity refused with 409 where the server cannot restart;
  a reference the schema lets go cleared, not refused;
  a restore that leaves no owner refused;
  and the plan says why this server cannot put the env file or the TLS files in place.
  A scope restored alone whose rows name users missing here is refused with the list,
  as D6-A says.
  The reader of the store and the routes found six faults:
  the files written after the commit,
  which fails under the unit's `ProtectSystem=strict`
  with the secrets already sealed for the backup's key;
  no restart after an error past the commit;
  restored log ids below the live ones, hiding later alerts;
  the command beside a `serve` it had not found;
  a settings restore dropping the target of the pinned backup;
  and a 409's list and a plan's upload lost in the command.
  "fix(store): A restore writes its files before the commit,
  keeps the history ahead and the undo's target",
  "fix(server): A restore restarts the server whatever follows its commit,
  and needs a restart for the keys",
  and "fix(cli): store restore needs the store alone,
  prints what is missing and removes a plan's upload" fix them.
  Writes between the backup taken first and the commit are not kept:
  the answer names them by `before_backup_time` and `restored_at`.
  The card's reader found the plan applied after the choice had changed,
  the dialog left modal in a hidden page after the browser's Back,
  and the key kept until Close;
  "fix(ui): The restore's dialog applies only the plan of the choice shown
  and leaves with another page" fixes them.
- **Block 6, WebDav.**
  A field's `Warn`, a pattern the card warns of, came with it;
  a put moves with `Overwrite: F`,
  and a file of the name and size there already is taken as the copy.
  The targets' reader found that a name which did not parse sent a DELETE to the folder,
  which "fix(server): A WebDAV name never reaches the folder itself" closes;
  and that a target's config changed while its secret was kept,
  so that a holder of `backup.server` could send the password to a server of their own.
  "fix(security:server): A target's settings change only with its secret"
  refuses that for every kind with a secret,
  and in wave 4 the card follows,
  "feat(ui): The card asks for a target's secret again when its address changes".
  The assembler moved its section of `docs/store.md` beside the other kinds'.
- **Block 7, Sftp.**
  `Kind.MakeSecret` and `Kind.Public` let the server make a target's key
  and show its public line;
  a host without posix-rename gets a remove and a rename.
  Its card part came in wave 4,
  "feat(ui): The card shows an SFTP target's key and confirms its host".
  The card's reader found a host key that a test met
  trusted for another host after a save,
  and the focus lost after Trust and Delete,
  which "fix(ui): A host key stays with the host tested,
  and the focus follows Trust and Delete" fixes.
- **Block 8, Smb.**
  `cloudsoda/go-smb2` at its commit of 2026-11-05,
  still its newest on 2026-11-27 (D16-A).
  The first `make target-test` found that go-smb2 ends a read without `io.EOF`;
  the target adds it.
  Its test helpers took an `smb` prefix at the stacking, inside its own commit,
  so that every commit builds.
  `make target-test`, the CI job Backup targets,
  `GUIDELINES/project/CI.md` v1.76.0 and the contract's v1.24.0 came with it.
  The targets' reader found the test images pinned by tag,
  fixed by "test(target-test): The target test's images are pinned by digest",
  and encryption only where the server asks for it.
  The module has no way to ask for encryption a server merely offers,
  so the target stays as it is,
  and "docs(server): The SMB target says it cannot ask for encryption" says so;
  age encrypts every copy in any case.
- **Block 9, S3.**
  A copy is one PUT of at most 5 GiB, with no multipart upload,
  as the plan's subset of S3 has it.
  `ErrRetained` in `target.go` lets a prune keep a copy the bucket's lock keeps,
  and "fix(server): A copy the bucket keeps answers 409" came at the stacking.
  MinIO answers a plain DELETE under object lock with a delete marker,
  never a refusal,
  so the refusal is tested against the fake alone.
  The targets' reader found that AWS refuses a PUT without a checksum
  into a bucket with object lock;
  "fix(server): An S3 copy carries its Content-MD5" adds it,
  and the harness makes a locked bucket.
- **Block 10, BeforeUpdate.**
  Beyond the plan: `POST /api/v1/update/backup`, the backup alone,
  which `massimo update` by hand asks of a running server;
  `GET /api/v1/update`'s `backup`, `starting` and `start_error`;
  the history's `backup` and `backup_skipped`.
  Built on Block 2, it was rebased onto Block 2's fixes before its own fix commits.
  The reader found five faults:
  `sudo massimo update` stopping on every box, root having no key;
  the backup bound to the browser's request and its timeout;
  the command's backup packing the wrong env file;
  the key sent to whatever answered; and pinned backups without limit.
  "fix(update): An update's backup runs past the request,
  and an unused one gives up its pin"
  and "fix(cli): massimo update by hand asks the service with its settings,
  and goes on without a key" fix them;
  "fix(update): A second update refused while one is being started says so in every case"
  closes a gap the first left.
- **Block 11, NewBox.**
  Five routes under `/api/v1/setup/backup`:
  upload, list and fetch from a target the request describes, plan and restore;
  `makes_secret` in the kinds' answer,
  and an SFTP target refused "from a target",
  since a new box has no key the host trusts;
  the restart allowed from the local network while the server is open.
  A third commit,
  "fix(cli): massimo store restore starts a new box without a backup first",
  lets the command start a new box too.
  The security reader found no gain of rights, and five faults:
  a restart loop with a backup without an owner,
  no backup first on an open server that holds data,
  the restore not bound to the backup planned,
  a target's password lingering in the card,
  and a held backup outliving the window.
  "fix(server): A new box starts only empty, from the backup it planned,
  with an owner"
  and "fix(ui): The first-owner card offers a backup only where it may,
  and forgets a target's secret" fix them:
  the routes answer only on a store with no configured Cube, room or integration,
  which `new_box` in `GET /api/v1/me` tells the card.
  Its note for the owner:
  whoever reaches an open server can hand it a file and a key of their own making;
  the first owner's window is the protection,
  as it is for making the first owner.

## Tests

Locally:

- **Each builder:** `make ui`;
  `go test` with `MASSIMO_ENV_FILE=/dev/null` over the packages it touched;
  `gofmt -l .`; `go vet` for linux, windows and darwin;
  `make ui-check` where the parts changed;
  the UI test files it touched in the four cases, on ports of its own;
  `GUIDELINES/shared/tools/check` over its range;
  and a fix's new test seen failing without it, where that was cheap.
- **Block 8:** `make target-test` against MinIO and Samba 4.22 with encryption required;
  `make actionlint`, shellcheck on its scripts, and `go test ./packaging`.
- **The assembler, on each branch:** every commit built;
  `go test` of `internal/serverbackup`, `internal/server`, `internal/store`,
  `cmd/massimo`, `tools/apitypes` and `tools/identifiers`;
  `gofmt`, `go vet` for the three platforms, `make ui-check`,
  and the check over the branch's range;
  `make target-test` on `targets`;
  the UI test files of Blocks 3 and 4 in 1100-en on `ui`.
- **The whole UI suite** in 1100-en on `ui` before its last two fixes,
  893 passed;
  on `start`'s head after the merges of `main`,
  905 passed, and `make target-test` passed both live tests.
  The plan asks for it once before each pull request;
  `start`'s head holds every pull request's tree.

Left to CI: the whole `go test ./...`, `make race`, `make cross`,
armv6 under QEMU, and, where a pull request touches their paths,
the UI suite's four cases, Windows, macOS, the LAN test,
the Android emulator and Backup targets.

## Found outside the plan

- `TestProxyRetriedUntilTheAddressIsThere` failed twice
  under the load of builders running side by side, and passed alone.
  `main`'s "test(server): The proxy's retry test waits for tries made after its
  error changed" (AP SteadyTests v1.1, Block 12) fixes it,
  and comes into the stack with the merge of `main`.
- A session scratchpad that the builders shared
  let Block 1's commit message reach Block 3's first commit.
  Block 3's builder amended it before it was taken,
  and the builders' brief now keeps every scratch file in the builder's own worktree.

## For the owner

Each with its options, the recommended first.

- **D18. The MinIO image.**
  `make target-test` and the CI job pull MinIO as `pgsty/minio`,
  a community fork, pinned by digest with its release tag in a comment:
  MinIO publishes no image of its own any more,
  and `minio/minio` refuses an anonymous pull.
  - **A (recommended).** Keep it:
    it runs in tests alone and never ships,
    and the digest holds it to what the last run tested.
  - **B.** Build MinIO from its source in the harness:
    no third party, and a slower run.
- **D19. Backup targets and the Tested record.**
  The record is full at 140 characters and has no word for the job,
  so it runs where Vet and test runs:
  on a push of code that had not passed, and on a tag,
  never where `tested=true`;
  on a pull request only where its paths change.
  - **A (recommended).** Keep it so for v1.0:
    the targets' code is in its paths, and a tag runs it before a release.
  - **B.** Make room for a word of its own, in a plan for CI.
- **D20. What other clients see.**
  - `POST /api/v1/update` and `now` of `PUT /api/v1/update/answer`
    answer 202 with `starting`, GET's answer while the backup runs,
    where a backup is due, in place of `requested`;
    a failure then shows in GET's `start_error`, not in the answer.
  - `sudo massimo update` reads the service's settings file;
    with a key set and no running `serve` to take the backup,
    it stops, since root does not write the service user's store,
    and says to start the service or to pass `-no-backup`.
  - **A (recommended).** Keep both,
    as `docs/api-v1.md`, `README.md` and `packaging/README.md` say:
    in this repository the web UI follows the new answer,
    and the platform jobs update without a key, where nothing changes.
  - **B.** Hold the answer until the backup is done,
    and let root make the backup on the service user's store:
    the browser's timeout would end the backup again, which the reader found,
    and root would write into another user's store.
- **D21. The operator loses System settings.**
  The operator's role holds `backup` and none of System settings' capabilities;
  with Cube backups on Maintenance it has Maintenance and no System settings,
  as the EVAL of D14-C said.
  - **A (recommended).** Keep it, as decided:
    its Cube backups are on Maintenance.
  - **B.** Seed the operator a capability of System settings' cards,
    which gives it more than it holds now.
- **D22. AP BackupWatch's fragments.**
  Three of its `CHANGELOG.d` fragments name Backup and restore,
  one of them on System settings;
  they ship with 0.17 from `maint/0.17`, where the card still is.
  - **A (recommended).** Leave them:
    Block 3's fragment says the move in the release after.
  - **B.** Rewrite them on `main`:
    0.17's notes would then name a card 0.17 does not have.
