# AP TestedTree v1.0: Carry a green run

## TL;DR

A green run leaves its code fingerprint on the commit as a status, and a
later commit with the same fingerprint runs only what has not run for that
code yet: a retirement on a tested branch, and the merge into `main`.

- retirement on a tested PR branch: full suite → Guidelines only
- merge into `main` of a tested PR: full suite → Fuzz, the Screenshots/LAN
  jobs the PR skipped, Build, Release
- doc-only test: DocOnly's pathspec diff in ci.yml →
  `fingerprint.sh same`, the list in config.ini `[ci] untested`
- release commit: message prefix → `commit_title.py`'s role
- status "Tested": `fp1:<sha256> ui= lan= fuzz=`, written by a green run,
  forwarded by a run that skipped as tested

### Blocks

1. Guidelines v4.57.0 and `[ci] untested`
2. What to test on `fingerprint.sh`
3. The Tested status and its forwarding
4. Documents and the changelog fragment

## Discussion

The owner asked on 2026-11-11 whether `main` must test again a merge of a
pull request whose suite passed, and a branch's doc-only retirement after
its green run. A first idea recorded the tested base and head; the owner
proposed a status that the gate forwards from commit to commit, which a
code fingerprint makes simple. The "CLI scripts libraries" session, asked
to evaluate a guidelines tool, released it as guidelines v4.57.0:
`tools/fingerprint.sh` and the `[ci] untested` key.

**The owner's decisions** (`PLAN++`, 2026-11-11, as recommended):

- **D48-A:** a "Tested" commit status holding the code fingerprint of the
  tree that ran and what ran for it, written at the end of a green run,
  forwarded by a run the gate found tested.
  Not B, Blocks 1 and 2 alone.
- **D51-A:** the bump to v4.57.0 in this plan's pull request, as Block 1.
  Not B, a `chore(guidelines)` commit of its own first.

## Analysis

**The fingerprint.** `fingerprint.sh fingerprint <commit>` hashes every
file the commit tracks but those its own config.ini names as
`[ci] untested`, without a checkout; two equal fingerprints are the same
code. It answers exit 0, or 2 where it cannot, which the gate reads as
"test everything". It refuses a list that covers config.ini or a shipped
path, so DocOnly's separate shipped diff goes.
`same <a> <b>` replaces DocOnly's pathspec diff for `code`.

**What the status holds.** Context `Tested`, state `success`, written by
`github-actions[bot]`, description
`fp1:<hex> ui=<bool> lan=<bool> fuzz=<bool>`: the fingerprint of HEAD, the
tree that ran (in a pull request's run its merge commit, as the guidelines
session advised), and whether Screenshots, LAN and Fuzz passed for that
code, in this run or in the one it forwards. A pull request's run writes
it on the pull request's head commit, a push's on the pushed commit.

**Which earlier status counts.** For a pull request: the status on the
head's first parent, the commit the previous run tested
(`fetch-depth: 3` reaches it). For a push to `main`: its first parent's
status is DocOnly's case, `code=false`, and needs none; the status on the
second parent of a merge, the pull request's head, makes it `tested`.
A status counts only where its state is `success`, its creator
`github-actions[bot]` and its fingerprint the commit's own; anything else,
or an API error, is "test". A fork's pull request cannot write statuses,
its token being read-only, and runs everything.

**The jobs on `tested`.** Skipped: Vet and test, Race, armv6,
govulncheck, Debian, Windows, macOS, Site, which ran green for this code.
Run: Fuzz unless `fuzz=true` (a pull request never fuzzes), Screenshots and
LAN where the pull request asked for them and they had not passed
(`ui`/`lan`), Guidelines, All tests passed, Build and Release.

**The status job.** "Tested", after All tests passed, needs the jobs whose
results it records; `statuses: write`; `continue-on-error`, so a fork's
read-only token fails nothing. What to test reads statuses with
`statuses: read`.

**Release commit.** `commit_title.py --root GUIDELINES parse` gives the
role of any summary config.ini allows, `chore(release:<component>)`
included, where the prefix test missed one.

**Trust.** A status counts only from `github-actions[bot]`; a fork's
token cannot write one; `.github/**` is shipped, so never untested, and a
change to the workflows always changes the fingerprint.

## Implementation Plan

### Block 1: Guidelines v4.57.0

1. `install.sh --ref v4.57.0`: the submodule and the regenerated documents.
2. `[ci] untested` in `GUIDELINES/config.ini`, DocOnly's list;
   the UNTESTED-PATHS markers in `GUIDELINES/project/_CONTRACT.md`;
   `fingerprint.sh update-contract`; the documents regenerated.

**Verification:** `fingerprint.sh check`; `tools/check`; the fingerprints
of known doc-only and code commits.

### Block 2: What to test on fingerprint.sh

1. The checkout with the submodule; `code` from `fingerprint.sh same`;
   the release commit from `commit_title.py`.

**Verification:** the gate's script against real commits, as for DocOnly.

### Block 3: The Tested status

1. What to test: `fp`, `tested`, `ui_done`, `lan_done`, `fuzz_done`
   from the earlier status; the jobs' conditions.
2. The job "Tested" after All tests passed.

**Verification:** shellcheck; ci.yml parses; after the merge, a pull
request's run writes the status, and its merge's run on `main` skips.

### Block 4: Documents

1. `GUIDELINES/project/CI.md`, section 2.
2. `CHANGELOG.d/Changed/tested-tree.md`.

**Verification:** reflow; `changelog.sh check --staged`; `go test ./...`.

## Proposed commit message

```
chore(guidelines): The guidelines at v4.57.0, with [ci] untested
perf(pipeline): A green run is carried to the commits with its code
```

## Change History

- v1.0 (2026-11-11): first revision, D48-A, D51-A.
