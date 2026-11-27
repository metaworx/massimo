# NOTE ServerBackup v1.0: gate decisions

The owner's answer to the plan gate of 2026-11-27:
"D17-C user who sets up backup choses which type of the two",
"run in parallel agents, where possible", `EXEC++`.

## Decisions

- **D16-A.** The SMB module is the fork, `github.com/cloudsoda/go-smb2`,
  pinned at a commit and checked again when Block 8 starts:
  the gate named A for an answer that names none.
- **D17-C.** Whoever sets up the backup chooses between the two kinds:
  - "Make a key" asks for the kind:
    post-quantum (`age1pq1…`) first, classic X25519 (`age1…`) second,
    each with its line: the first lasts against a quantum computer and is
    about 2000 characters, the second is short and goes beside an
    `ssh-ed25519` key;
  - `massimo store keygen -kind pq|classic` takes the kind, with no default;
  - a pasted recipient may be of either kind;
  - a box's recipients stay of one kind, as age requires,
    and the card says so where a mix is tried.

## The run

`EXEC++` runs Blocks 1 to 11, with no merge, no retirement and no release.
The gate's scope for a push and the pull requests stands:
each pull request is pushed and opened when its blocks are done.

Builders are Opus sub-agents, as many at once as the blocks allow:

| Wave | Blocks | Built on |
|------|--------|----------|
| 1 | 1 and 3 | `main` |
| 2 | 2 | Block 1 |
| 3 | 4, 5 without its card part, 6, 7, 8, 9, 10 | Block 2 |
| 4 | 5's card part, 11 | Blocks 4 and 5 |

- Each builder works in a worktree of its own, on a branch of its own.
- The coordinating session takes their commits onto the pull requests'
  branches, signed, and resolves what two of them both touched.

## Steps changed

The plan's blocks stand; three of their steps change:

1. A target's form in the card is drawn from what the server says of
   each kind: its fields, their types, which of them is secret
   (`GET /api/v1/server-backup-targets/kinds`).
   Block 4 builds that and the local kind's.
   Blocks 6 to 9 each add their kind's description and its texts,
   not a form of their own.
2. Block 5's card part ("Restore…", "Check with the key") waits for
   Block 4 and is built with Block 11.
   Its store, its routes and its command run beside Block 4.
3. The pull requests stack:
   - `claude/server-backup-core` on `main`;
   - `claude/server-backup-ui` and `claude/server-backup-targets` on it;
   - `claude/server-backup-start` on `claude/server-backup-ui`.
