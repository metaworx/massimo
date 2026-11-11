# AP SafeFiles v1.0: files written whole

## TL;DR

Massimo writes its files in six places,
five of them with their own copy of "temporary file, rename",
four without an fsync, and two plainly; one package, `internal/safefile`,
writes them all whole, keeps what the replaced file carried,
and follows a link only a trusted user made.

- five copies of temp-and-rename → `safefile.Write` and `safefile.Replace`
- no fsync in four → file and directory synced
- a link at the path → followed when the writer or root owns it, else replaced
- owner and group → kept: unset tries and ignores a failure, `true` requires it,
  `false` leaves ownership alone (D14)
- mode, extended attributes (Linux, macOS), owner,
  group and DACL (Windows) → kept, best effort; no hardlinks
- no write permission on the directory, but on the file → written in place,
  logged (D15-A)
- `massimo backup` file and the store copy → whole or absent
- `install`'s `chown` after a write → `chown -h`, no link followed
- locks → none yet; the pattern in the package's comment

### Blocks

1. Package: `internal/safefile`, its platform parts and tests,
   CI's Windows and macOS jobs
2. Writers: install, the certificate, the update request and status,
   the reboot marker, and install's `chown -h`
3. Backups: the `massimo backup` file and the store's copy

## Discussion

**The owner's question** of 2026-11-11,
from a flaky test of PR #173 (the settings watch saw a file between its
truncation and its write):
would writing through a temporary file be generally safer,
and would a general filesystem module be good,
one that follows symlinks and copies owner and ACLs, not hardlinks,
with Zoo-Code's `safeWriteJson` as an example (github.com/Zoo-Code-Org/Zoo-Code,
`src/utils/safeWriteJson.ts`).

**Decided** at that EVAL, by the owner's `PLAN++ as recommended` and three
changes of the owner's:

- **D13-A.**
  A module for writes: symlinks followed, owner, group,
  mode and extended attributes kept on Linux and macOS, owner,
  group and DACL on Windows, fsync of the file and the directory,
  the Windows rename retry; every writer moves to it.
  No read wrapper: once every writer is atomic,
  a plain read sees one whole version.
  No locks yet: no file has two writers at a time.
- **D14, the owner's.**
  Owner and group are copied on a best-effort basis: where it fails,
  the process's ownership (or the directory's group) is taken to be fine.
  At most a tri-state flag: `true` requires it and fails,
  `false` does not change ownership, unset tries and ignores a failure.
- **D15-A.**
  With no write permission on the directory but on the file,
  the file is written in place, logged;
  only for a permission error on the directory, never for another one.
- **The owner's:**
  the file lock pattern is written down in the package's source,
  for a later implementation.
- **The owner's:**
  the work branch starts from PR #173's,
  so that the new pull request carries both.

**Names chosen here**, routine and not decisions of the owner's:
the package `internal/safefile` with `Write`, `Replace` and `Options` (`Mode`,
`KeepOwner`, `Logger`), the blocks Package, Writers and Backups,
the tests named in the Implementation Plan, the branch `claude/safe-files`.

**Not in this plan:**

- reads and locks (D13-A);
- the update's executable swap (`keepAs` and its renames in `internal/update`),
  which keeps a hard link of the running binary on purpose;
- `massimo legacy -out`, a live capture that stays a stream;
- the key files the store creates once with `O_EXCL` (`identity.go`,
  `opaque.go`).

**Who runs it:**
this session.
The plan's own commit goes straight onto `main`;
the work branch `claude/safe-files` starts from `claude/discovery-core` (PR
#173), merges `origin/main`,
and becomes one pull request into `main` with both plans' commits,
one or more per block.

## Analysis

### What writes files now

| Writer | Now |
|--------|-----|
| `install` (`writeAtomic`): settings file, units, plists, binary, TLS files | temporary file, fsync, mode, rename with the Windows retry (`renameOver`) |
| `certs` (`writeAtomic`): the self-signed certificate and key | temporary file, rename; no fsync |
| `update` (`WriteRequest`, `WriteStatus`): the update request and status | `path + ".tmp"`, rename with a Windows retry (`renameRetrying`); no fsync |
| `cmd/massimo` (`rebootRequester`): the reboot request | temporary file, rename; no fsync |
| `backup.File.Save`: `massimo backup`'s file | `os.WriteFile`: a kill or a full disk leaves it cut off, over a good one with `-o` |
| `store` (`copyTo`): `store backup`, the copy before a migration | `VACUUM INTO` the final name: a crash leaves a partial copy under it |

### The example

Zoo-Code's `safeWriteJson` takes an advisory lock file,
restores an interrupted replacement,
streams into `.name.new_<time>_<random>.tmp`, syncs it on a best-effort basis,
and renames it over; on Windows, where a reader holds the target,
it renames the target aside first and back on failure.
It does nothing for links, mode, owner or ACLs, and syncs no directory.
Its lock is for several processes writing one JSON file;
massimo has one writer per file.
Its recovery step serves its aside rename; with the retry `install` already has,
there is no aside file to recover, and stale temporary files are removed.

### Links, and who may plant them

`install` runs as root, hands `/etc/massimo` to the service user,
writes `/etc/massimo/env` through a temporary file and a rename,
then runs `chown _massimo:_massimo /etc/massimo/env`.
`chown` follows a link:
a compromised service that swaps the file for a link between the rename and the
`chown` has root give it the link's target.
`chown -h` changes the link instead.

The same reasoning sets the module's rule for following a link:
it is followed when its owner is the writing process's user or root,
as Linux's `protected_symlinks` does in a sticky directory;
another user's link is replaced, as a rename does now.
On Windows a link is followed.
A link to nothing is refused.

### What each platform allows

`golang.org/x/sys` has the extended attribute calls on open files for Linux and
macOS (`Flistxattr`, `Fgetxattr`, `Fsetxattr`),
and `GetNamedSecurityInfo` and `SetNamedSecurityInfo` for Windows;
`os.File.Chown` changes owner and group.
On Linux a POSIX ACL is the extended attribute `system.posix_acl_access`,
so copying the extended attributes copies it; `security.*`,
the SELinux label among them,
needs a privilege to set and is best effort. macOS keeps ACLs out of its
extended attributes, behind `acl(3)`,
which a build with `CGO_ENABLED=0` cannot call: its ACLs are not copied.
On Windows the SACL needs a privilege and is not copied.

### The in-place fallback

When the temporary file cannot be created for want of permission on the
directory, and the file exists and is writable,
`Write` opens it without truncating, writes,
syncs and truncates to the new length.
A reader or a crash can then see mixed content, hence the Warn line;
the file stays the same file, its owner, ACLs and hard links untouched.
`Replace`, which hands its caller a path to fill, has no fallback.

### Locks, for later

A lock, where two processes come to write one file
(a settings editor in the web UI would be the first):
an advisory lock on a lock file beside the target, never on the target,
which a rename replaces;
`flock(LOCK_EX)` on Unix, which the kernel releases when the process dies,
so no stale lock outlives it, unlike the example's lock files with a stale age;
`LockFileEx` on Windows, mandatory, on that lock file alone,
so that readers of the target are not refused;
held across the read and the write of a read-modify-write.
The package's comment says so.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over the Makefile's `RACEPKGS`,
the package's tests built for Windows and run through interop,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`.
Nothing reaches a real Cube or a box.
One or more functional commits per block, citing `AP SafeFiles v1.0, Block <n>`,
each with its CHANGELOG fragment (COMMIT.md §5.5).

### Block 1: Package

1. `internal/safefile`:
   - `Write(path, data, Options)` and `Replace(path, Options, fill)`,
     where `fill` writes a temporary path handed to it, as `VACUUM INTO` needs;
   - `Options`:
     `Mode` (0 keeps an existing file's, 0600 for a new one),
     `KeepOwner` (`*bool`, D14), `Logger`;
   - the link rule, the temporary file beside the real target (`O_EXCL`,
     a random name), its fsync, the metadata copied onto it,
     the rename with the Windows retry, the directory's fsync on Unix,
     temporary files older than an hour removed;
   - the in-place fallback of `Write` (D15-A);
   - the lock pattern in the package comment.
2. Platform parts: Linux and macOS (extended attributes),
   Windows (owner, group and DACL; the held-file retry), others (none).
3. Tests, `internal/safefile/safefile_test.go` and per platform:
   `TestWriteReplacesWhole`, `TestModeKeptOrSet`,
   `TestOwnerTriState` (through a seam for `Chown`),
   `TestLinkFollowedOrReplaced`, `TestDanglingLinkRefused`,
   `TestExtendedAttributesKept` (Linux and macOS), `TestDACLKept` (Windows),
   `TestInPlaceFallback`, `TestNoFallbackForOtherErrors`, `TestReplaceFill`,
   `TestStaleTempRemoved`.
4. `.github/workflows/ci.yml`:
   the package in the Windows and macOS jobs' unit tests;
   `GUIDELINES/project/CI.md` says so.
5. `CHANGELOG.d/Added/claude-safe-files.md`.

**Verification:**
the test gate; the Windows tests through interop pass.

### Block 2: Writers

1. `internal/install`:
   `writeAtomic` and `renameOver` become `safefile.Write`;
   the `chown` of a file it wrote is `chown -h`,
   a security fix of its own commit.
2. `internal/certs`, `internal/update` (`WriteRequest`, `WriteStatus`,
   `renameRetrying` gone), `cmd/massimo` (`rebootRequester`):
   through `safefile.Write`, their modes as now.
3. Tests: the existing ones pass,
   install's with `chown -h` in its command lines.
4. `CHANGELOG.d/Security/claude-safe-files-chown.md`;
   Block 1's fragment amended: the files massimo writes go through it.

**Verification:**
the test gate.

### Block 3: Backups

1. `backup.File.Save` through `safefile.Write` (mode kept, 0600 for a new file).
2. `store.copyTo`:
   the final name reserved with `O_EXCL` as now,
   `VACUUM INTO` a temporary path through `safefile.Replace`,
   renamed over the reservation once complete.
3. Tests: a fill that fails leaves no copy under the final name;
   a backup written over an existing one is whole.
4. `docs/store.md` where it describes the copy;
   `CHANGELOG.d/Fixed/claude-safe-files-backups.md`.

**Verification:**
the test gate.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +SafeFiles v1.0

Massimo writes its files in six places, five with their own copy of a temporary file and a rename, four without an fsync, two plainly, and install runs chown on a file in a directory the service user owns; the plan gives them one package that writes a file whole, keeps its link, owner and attributes, and falls back to an in-place write where the directory is not writable.

Key changes:
- GUIDELINES/wip/2026-11-11_07-32_AP_SafeFiles_files_written_whole.md: the plan, three blocks, D13-A, D14 as the owner gave it, D15-A, the lock pattern for later.

File-revision-v1: GUIDELINES/wip/2026-11-11_07-32_AP_SafeFiles_files_written_whole.md::v1.0::2026-11-11::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `feat(safefile): One way to write a file whole, keeping its link, owner and attributes`
2. `refactor(safefile): The files massimo wrote whole go through safefile` and
   `fix(security:install): install changes a file's owner without following a link`
3. `fix(store): A store copy and a backup file are whole or absent`

## Change History

- v1.0 (2026-11-11): the first revision,
  from the owner's questions after the settings watch's flaky test in PR #173;
  D13-A, D14 as the owner gave it, D15-A,
  the lock pattern and the branch on #173's by the owner's `PLAN++`.
