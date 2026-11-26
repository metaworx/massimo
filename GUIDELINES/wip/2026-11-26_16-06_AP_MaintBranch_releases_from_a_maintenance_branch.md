# AP MaintBranch v1.0: Releases from a maintenance branch

## TL;DR

At the feature freeze `maint/<x.y>` is cut from `main`. Completions, fixes
and every x.y release go to it. It merges upward into `main`, which opens
for x.y+1. Store migrations are numbered in blocks of 100, and the store
never applies a lower number. A fix migration that `main` needs too gets a
guarded copy at the top of `main`'s block.

- numbers: contiguous → strictly increasing; a gap only up to a multiple of
  100; a duplicate still refused
- a fix line's migration: guarded (a no-op where the change is in place)
  and additive (no table rebuilt)
- `TestMigratesReleasedStores`: rows kept → also the schema of a fresh
  store, and no column loses values
- `make release`: `main` only → `main` or `maint/<x.y>`; `build.yml`
  takes a tag on either; `ci.yml` tests a push to `maint/*`
- devel and nightly builds → `main`'s alone, as before
- `make freeze <x.y>` → cuts `maint/<x.y>` at `origin/main`
- one line per flavour: `release/alpha` only fast-forwards

### Blocks

1. Store: blocks of 100, the schema and column checks, the copy rule
2. Releases from `maint/<x.y>`: Makefile, `build.yml`, `ci.yml`, CI.md
3. The freeze and the upward merge: `make freeze`, the contract, README

## Discussion

### What the owner asked for

On 2026-11-15 v0.16.0-alpha.1 had to leave out the newest merge of `main`
(UiParts Block 1). Main was set back to the commit before it, the release
was cut, and the merge was brought back. On 2026-11-26 the owner asked
what a branch for the next release's features would take. That turned into
the inverse, trunk-based: `main` stays where features land, and a
maintenance branch is cut at the feature freeze. The owner then asked that
it be cut at the freeze, before the release, rather than after it, and that
no lower migration ever be applied: a fix the newer line needs as well is
copied to the top of that line's block instead.

### What is decided

- **D1-A.** The session that merged into `maint/<x.y>`, or cut a release
  there, makes the upward `merge:` commit and pushes it to `main` straight
  away, as plans are pushed: no lag, and a conflict is solved by whoever
  knows the change.
- **D2-A.** `make freeze <x.y>` cuts `maint/<x.y>` at `origin/main` and
  pushes it; it refuses where that branch exists already.
- **D3-A.** An old maintenance branch stays until the next freeze and
  releases nothing after it: one line to care for, as one line per flavour
  is all the channels allow.
- **D4-A.** After a released store is migrated, no column holds fewer
  non-null values than before, but for the columns a migration drops or
  empties on purpose, which the test lists with the migration that does it.

## Analysis

### Why trunk-based

`main` is where every default points: a pull request's base, Dependabot,
CodeQL, and the devel and nightly builds. With many agent sessions, "every
branch targets `next`" would be forgotten; "a fix that must ship without
the newest features targets `maint/0.17`" is rare and said each time.

### What ties releases to `main` today

- `Makefile`, `release`: refuses unless `main` is checked out and equals
  `origin/main`, and pushes `main` with the tag.
- `build.yml`, What to release: a tag "is not on main, where every release
  is cut" fails the run.
- `ci.yml`: a push runs the suite for `main` alone. The What to test case,
  which skips a release commit's suite, and the fingerprint record name
  `refs/heads/main`.
- `release.yml`: devel from a push to `main`, nightly from the schedule on
  `main`. These stay as they are.
- `release.yml` fast-forwards `release/<flavour>` to the tag. A tag on
  `maint/0.17` descends from the last release because `main` held that
  release when the branch was cut. A release of an older line after a newer
  one of the same flavour cannot move it, hence D3-A.
- The `signing` environment admits any version tag, so it needs no change.

### How the store applies migrations

`internal/store/migrate.go`, `loadMigrations`, refuses anything but 1, 2,
3, … without a gap (`expected version %d`). `migrate` applies each
migration above the store's version and up to the binary's, in order, and
never one below. `SchemaVersion` is the number of the last. The proposal
keeps that, and the line stays a line:

- The fix line keeps numbering within the current block (0027, 0028, …),
  and `main` starts the next hundred at the freeze (0100).
- `maint/<x.y>` merges upward, so `main`'s tree holds the fix line's files
  too. A store already on `main`'s block skips them as lower, so a change
  `main` needs as well gets a copy at the top of `main`'s block. That copy
  is written for `main`'s schema, and is guarded so that it does nothing
  where the change is in place: on a store that came from the fix line, and
  on a fresh store, which runs every file in order and so takes the fix
  line's path.
- A fix line's migration is additive and guarded: a new table, column or
  index, `IF NOT EXISTS`, or a `pragma_table_info` check before a column is
  added. It does not rebuild a table.
- **The risk that stays:** a migration of the newer line that rebuilds a
  table copies the columns it knows. One that shipped before the fix
  existed drops the fix's column, with its values, on a store that came
  from the fix line, and the copy then adds the column back empty. The new
  checks of `TestMigratesReleasedStores` catch that before the newer line
  ships, since every later binary migrates every released store's dump.
  The fix is then a copy that carries the column over.
- `TestMigratesReleasedStores` (`internal/store/releases_test.go`) today
  checks the version, `integrity_check`, `foreign_key_check`, and that no
  table has fewer rows. It gains:
  - the schema of a fresh store: the tables, their columns with type,
    `NOT NULL`, default and key, and the indexes;
  - D4-A's check of each column's count of non-null values.
- Downgrades (`downgrade.go`) and `min_reader` compare versions as numbers
  and need no change; Block 1 checks that nothing else assumes contiguous
  numbers (the store's fixture tool, `store.sql`'s generation, the raise
  tests).

### What the freeze leaves to people

- **Main's first migration after the freeze** takes the next hundred. No
  check knows when a freeze happened. A clash with the fix line shows at
  the upward merge, where `loadMigrations` refuses two files of one number,
  or a gap that does not end at a multiple of 100.
- **A ruleset for `maint/*`**, with the required checks, no force push and
  no deletion, is the owner's step after the merge.
- **The first freeze** is the owner's to call; this plan cuts no
  maintenance branch.

## Implementation Plan

Verification as the owner set it on 2026-11-16: the touched packages' tests,
`gofmt -l .`, `go vet` for linux, windows and darwin, `make actionlint` for
a change under `.github/`, and `GUIDELINES/shared/tools/check`. The whole
suite is CI's.

### Block 1: Store migrations in blocks of 100

1. `internal/store/migrate.go`, `loadMigrations`: the numbers start at 1
   and strictly increase, each the one before plus 1 or the next multiple
   of 100 above it; a duplicate is refused as before. The comments of the
   file and of `loadMigrations` say so.
2. Grep for anything else that assumes contiguous numbers (`downgrade.go`,
   `raise_test.go`, `migrate_test.go`, `tools/store-fixture/`, `store.sql`'s
   generation) and adapt it.
3. `migrate_test.go`: a gap up to a multiple of 100 is loaded and applied
   in order; a gap to another number, a duplicate, and a number below the
   one before are refused, with the file named.
4. `releases_test.go`, `TestMigratesReleasedStores`: the migrated store's
   schema equals that of a fresh store; no column holds fewer non-null
   values than before, but for the listed ones (D4-A), each with the
   migration that drops or empties it and why.
5. `docs/store.md`, "Migrations": the blocks, the fix line within the
   current block, the guarded copy at the top of the newer line, the
   additive rule, and the risk of a rebuild with the test that catches it.
6. A fragment under `CHANGELOG.d/Changed/`.

**Verification:** `go test ./internal/store/...`, `gofmt`, `go vet` ×3,
`GUIDELINES/shared/tools/check`.

### Block 2: Releases from `maint/<x.y>`

1. `Makefile`, `release`: runs on `main` or on a branch `maint/<x.y>`,
   checked out, clean and equal to its `origin` copy; it fetches that
   branch, makes the dump's commit and the release commit on it, and
   pushes that branch and the tag atomically. Its comment says so, and that
   devel and nightly stay `main`'s.
2. `build.yml`, What to release: a tag must be on `main` or on a
   `maint/*` branch, fetched for the check; the error names both.
3. `ci.yml`: a push to `maint/[0-9]*.[0-9]*` runs the suite as a push to
   `main` does; What to test skips a release commit's suite there too; the
   fingerprint record takes those pushes. The site, the devel build and the
   nightly stay `main`'s alone.
4. `GUIDELINES/project/CI.md`: the triggers table and §6, "Releases".
5. A fragment under `CHANGELOG.d/Changed/`.

**Verification:** `make actionlint`; in a scratch clone with a bare
repository as its `origin`, `make release alpha DRY=1` on `main` and on a
`maint/0.99` branch, and its refusals on another branch and on a
`maint/0.99` behind its origin; `GUIDELINES/shared/tools/check`.

### Block 3: The freeze and the upward merge

1. `Makefile`, `freeze`: `make freeze <x.y>` checks the name (two numbers),
   fetches `main`, refuses where `origin/maint/<x.y>` exists, pushes
   `origin/main` as `maint/<x.y>`, and prints the next block's first
   migration number for `main`. `DRY=1` prints the push.
2. `GUIDELINES/project/_CONTRACT.md`, "Branches and pull requests": the
   freeze; completions and fixes target `maint/<x.y>`; the upward merge by
   the session that merged or released there (D1-A); an old branch
   releases nothing after the next freeze (D3-A). `AGENTS.md` and
   `GUIDELINES/README.md` regenerated from it.
3. `README.md`, "Versions": the freeze, the maintenance branch, and where
   a fix goes.
4. A fragment under `CHANGELOG.d/Added/`.

**Verification:** in the scratch clone, `make freeze 0.99 DRY=1`, the
freeze itself, and its refusal for a branch that exists and a bad name;
`GUIDELINES/shared/tools/check`.

### The pull request

One branch, `claude/maint-branch`, a commit or more per block, and one pull
request, `merge(MaintBranch/Blocks-1-3): …`, its body in commit form. After
the merge the owner adds the ruleset for `maint/*`.

## Proposed commit messages

- `feat(store): Migrations are numbered in blocks of 100`
- `feat(release): A release is cut on main or a maintenance branch`
- `feat(release): make freeze cuts the maintenance branch`

## Change History

- **v1.0** (2026-11-26): first revision, by the owner's PLAN++; D1-A to
  D4-A as recommended.
