# AP ReleaseDumps v1.0: store dump per release

## TL;DR

`make release` dumps the store the release writes, and commits it right before the
release commit, so that the tag holds its own dump;
the release CI refuses a tag without one.

- the dump of a release → made by hand in the next pull request with a migration, unchecked
  → by `make release`, before the cut, from `HEAD` under the coming tag's name
- `tools/store-fixture/make.sh <tag>` → also `--ref <commit>` for a tag not yet made,
  and the release's flavour, `alpha` before
- release CI (`sign`) → fails a tag whose tree lacks `internal/store/testdata/releases/<tag>.sql`
- `TestMigratesReleasedStores` → a dump at the binary's own schema is opened, not refused
- `DRY=1` → says it would make the dump

### Blocks

1. The dump in the release: make.sh, the Makefile, release.yml, the test,
   docs/store.md, CI.md and README.md; a fake release in a scratch clone

## Discussion

The owner asked on 2026-11-12 why AP UserCli and AP Oidc added no store dump to
`internal/store/testdata/releases/`.
None was missing: a dump is the store a release wrote,
one per release tag (`docs/store.md`, "Migrations"),
and the newest release, v0.14.0-alpha.1, has its own;
migrations 0015 and 0016 are in no release yet.
But nothing made sure of it:
the rule asked the next pull request with a migration to add the newest release's dump,
by hand, and nothing checked it.

The owner asked whether `make release` could make the dump, and the release CI check it,
and then whether the dump could be committed before the release commit.
It can: the release commit changes only the changelog,
so `HEAD` before the cut is the release's code,
and `make.sh` stamps the version (`-X main.version=<tag>`) the store records.
The release CI cannot make it before the release commit, the tag being there already,
and `main` takes no push from it but through a pull request.

**The owner's decisions**, 2026-11-12, `PLAN++ ReleaseDumps` on the evaluation's gate:

- **D17-C.**
  The dump in a commit of its own before the release commit,
  built from `HEAD` under the coming tag's name.
- **D18-A.**
  The release CI checks that the tag's own tree holds its dump;
  the tag's test run opens it with the others.
- **D19-A.**
  Every tag `make release` cuts, of any flavour.

**Names chosen here**, routine and not decisions of the owner's:
the options `--ref` and `--flavour` of `make.sh`;
the dump commit's summary `test(store): The store <tag> writes`;
the step "Check the release's store dump";
the branch `claude/release-dumps`.

**Not in this plan:**
dumps of the rolling builds (devel, nightly), which have no tag of their own;
`project.shipped` excluding `testdata/`
(`changelog.sh check --staged` passes the dump commit as it is).

**Who runs it:**
this session, under the owner's `PLAN++ ReleaseDumps` of 2026-11-12:
the plan, its registration on main, and the block committed on `claude/release-dumps`.
No push of the branch, no merge, no release.

## Analysis

**`make release`** today:
the checks (a flavour, `main` checked out, clean, equal to `origin/main`,
not released yet, `changelog.sh check`),
the next tag from `packaging/next-version.sh`,
`DRY=1` running the cut as a dry run,
`changelog.sh cut --commit --tag create` making the release commit and the tag,
the check that the release commit sits on `origin/main` and the tag on it,
and `git push --atomic origin main refs/tags/<tag>`.
The dump goes between the checks and the cut;
the check after the cut then finds the release commit on the dump commit,
which sits on `origin/main`.
Where the dump or its commit fails, nothing is pushed,
and `make release` says how to start over (`git reset --keep <origin/main>`).

**`make.sh`** builds a tag in a temporary worktree
with `-X main.version=<tag> -X main.flavour=alpha`.
`--ref` builds that commit instead and keeps the tag's name for the version and the file;
the header names the tag, and the commit it was built from;
`--flavour` gives the flavour, else the tag's own (none: stable).
It needs git, go, curl, jq, python3 and sqlite3, and runs about a minute.

**`release.yml`'s `sign`** checks out the tag;
a step before signing fails where the dump is missing,
naming `make release` and, for a tag cut otherwise,
`tools/store-fixture/make.sh <tag>` and a new tag.

**The test** refuses a dump whose schema is at least the binary's;
the tag's own run meets its own dump at the binary's schema,
which is opened as it is: the same checks, no migration.

What changes, by file:

| File | Change |
|------|--------|
| `tools/store-fixture/make.sh` | `--ref`, `--flavour`; the header |
| `Makefile` | `release`: the dump and its commit before the cut; `DRY=1`; the check after the cut |
| `.github/workflows/release.yml` | `sign`: the dump in the tag's tree |
| `internal/store/releases_test.go` | a dump at the binary's schema opened |
| `docs/store.md` section 5, `GUIDELINES/project/CI.md` section 6, `README.md` "Versions" | the dump of a release |
| `CHANGELOG.d/Changed/` | the fragment |

## Implementation Plan

The block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make actionlint`, `shellcheck` on `make.sh`,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, a box or GitHub.
One functional commit, citing `AP ReleaseDumps v1.0, Block 1`.

### Block 1: The dump in the release

1. `make.sh`: `[--ref <commit>] [--flavour <f>] <tag> [<out>]`.
2. `Makefile`: after `changelog.sh check`, the dump of `HEAD` as the next tag
   with its flavour, `git add` and commit, both refusing to go on where they fail;
   `DRY=1` says it would; the check after the cut one commit up.
3. `release.yml`: the step in `sign`.
4. `releases_test.go`: `>` for `>=`, the comment.
5. The documents; `CHANGELOG.d/Changed/claude-release-dumps.md`:

   ```
   - `make release` dumps the store the release writes into
     `internal/store/testdata/releases/` before the release commit, and
     the release refuses a tag without it.
   ```

**Verification:**
the test gate;
`make release alpha DRY=1` in this checkout says it would make the dump;
in a scratch clone with a scratch bare remote, `make release alpha` makes the dump commit,
the release commit and the tag, and pushes them to that remote;
the dump loads in `TestMigratesReleasedStores` as one at the binary's schema;
`make.sh v0.14.0-alpha.1` makes the same rows as its dump in the tree.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +ReleaseDumps v1.0

The store dumps of the releases were made by hand, in the next pull request with a migration, and nothing checked them; the owner asked that make release make them and the release CI check them, the dump committed before the release commit. PLAN++ ReleaseDumps of 2026-11-12, with D17-C, D18-A and D19-A.

Key changes:
- GUIDELINES/wip/2026-11-12_20-52_AP_ReleaseDumps_store_dump_per_release.md: the plan, one block, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-12_20-52_AP_ReleaseDumps_store_dump_per_release.md::v1.0::2026-11-12::All::the first revision, by PLAN++ ReleaseDumps
Assisted-by: claude-code:claude-opus-5-5
```

The block's commit: `build(make): make release dumps the release's store before the release commit`.

## Change History

- v1.0 (2026-11-12): the first revision,
  from the owner's questions on AP Oidc's pull request and D17-C, D18-A and D19-A.
