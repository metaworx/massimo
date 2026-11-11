# AP Channels v1.0: Rolling channels and flavours

## TL;DR

Two rolling channels, `devel` and `nightly`, each one build replaced in
place, and a box that chooses its channel instead of inheriting it from
the binary; every release goes through one pipeline:
tests → build → sign → publish.

- channel compiled in → **flavour** compiled in (the tag's label,
  `stable` without one); the **channel** is the box's setting, the
  flavour its default
- a box takes its channel and every more stable flavour:
  `devel` < `nightly` < `alpha` < `beta` < `stable`
- `release/devel`, `release/nightly`: tags of one GitHub release each,
  replaced in place; signed with a rolling key that boxes accept for
  those two flavours only
- `release/alpha` (later `release/beta`, `release/stable`): branches,
  fast-forwarded once the release is out
- `v*` tag → the full CI suite first, then the release;
  `release.yml` reusable, called by `ci.yml`
- nightly: the newest commit of `main` that passed the suite and is newer
  than the current nightly
- the release attaches its signed apt index, for the Site plan

### Blocks

1. Flavours: the updater's rank, rolling releases, the rolling key
2. Box channel: the stored setting, serve, `massimo update`, the API
3. Pipeline: `ci.yml` calls the reusable `release.yml`; label → flavour
4. Release branches: `release/alpha` moved by the pipeline;
   `make release-alpha` pushes `main` and the tag only
5. `devel`: published on every green push to `main`
6. `nightly`: the schedule and the owner's selection rule
7. apt index as a release asset, for the Site plan
8. Documents
9. Roadmap

## Discussion

Asked by the owner on 2026-11-10: a nightly channel of every green suite
on `main`, or a `devel` one beside a `nightly` updated at most once a day,
one build kept for each, in a line `devel` → `alpha` → `beta` →
(`rc`) → `stable`. Worked out over four gates the same morning, with the
"Website build strategy" session handing over its apt-key decision.

**The owner's decisions** (`PLAN++`, 2026-11-10, "everything else as
recommended"):

- **D1-A:** `devel` and `nightly`. Not B, `devel` only; not C, `nightly` only.
- **D2-A:** a second Ed25519 key, `MASSIMO_ROLLING_KEY`, in an environment
  `signing-rolling` that admits `main`; boxes accept it for the flavours
  `devel` and `nightly` only. The release key stays in `signing`, tags only.
  Not B, `signing` extended to `main`: every direct push to `main`
  (admins bypass its ruleset, and agents push plan commits there)
  would reach the release key.
- **D3-A:** apt and Homebrew for the rolling channels later,
  after AptUpdate's DebVersion, with an apt key of their own.
- **D4-A:** `beta`, `rc` and `stable` not now; on the roadmap.
- **D5-A:** one pipeline, `ci.yml`'s last job calling `release.yml`.
- **D6-A:** the release branch is fast-forwarded by the pipeline after the
  release is out. Not B, no branches; not C, `make release-alpha` moving
  it before the release.
- **D7-A:** the release waits for every test job but Fuzz until FuzzDeadline
  merges. It merged as PR #156 before this plan, so Fuzz is included.
- **D8-A**, as the owner amended it: the rolling releases' notes are
  `changelog.sh preview` (the `[Unreleased]` section as the next cut would
  write it) and the commit range since the previous build.
- **D9-A:** a tag ruleset on `v*`: tags are never moved or deleted.
- **D10-A:** the owner is required reviewer of `signing`.
- **D11-A:** release branches `release/alpha` and so on, a ruleset on
  `release/*` refusing force-pushes and deletion; `alpha` deleted after the
  first release through `release/alpha`.
- **D12-A:** the apt key stays tag-only; the release attaches its signed
  index, which a site deploy from `main` reuses (Block 7).
- **D13-A:** tags `release/devel` and `release/nightly`; a name is never
  both a branch and a tag.
- **D14-A:** the flavour is the tag's label, `stable` without one;
  the `channel:` line of a tag message and the fallback to the branch go.
- **D15-A:** the stored setting and `massimo update -channel` now,
  the UI's selector later.
- **Nightly**, the owner's rule: the newest commit of `main` that passed all
  tests and is newer than the current nightly, so that a red last commit
  does not cost a night whose second-last commit was green.

**Found while planning**, after the decisions:

- **`GITHUB_TOKEN` cannot move `release/alpha`.**
  GitHub refuses a ref update by an Actions token when it changes
  `.github/workflows/`, and a fast-forward over a release's commits often
  does; the token cannot be given the `workflows` permission.
  Block 4 pushes the branch with a deploy key,
  `RELEASE_BRANCH_DEPLOY_KEY`, in `signing` beside the release key:
  one more owner step (the commands file), the same exposure as the key
  it sits beside. Tags are not subject to that rule, so the rolling tags
  move with the token.
- **`changelog.sh preview` is in the guidelines from v4.54.0;**
  massimo's submodule is at v4.53.0, and its bump awaits the owner
  (the changelog fragments). Until then the notes say so and carry the
  commit range alone.
- **`nightly` reruns no tests.** The analysis had the schedule run the whole
  suite again; the owner's rule picks a commit whose push run passed,
  so the schedule only selects, builds, signs and publishes.
- **The release commit is tested twice:** by its push to `main` and by its
  tag. file_checksum_search's one-run-per-release skip would need a
  condition on every one of `ci.yml`'s eleven jobs; the repository is public,
  so the second run costs no paid minutes, and the `main` run publishes
  `devel` while the tag's publishes the release, without coordination.

## Analysis

### Flavour and channel

Today the Makefile compiles a channel into the binary (`main.channel`),
the branch it was built from or a tag's label; `MASSIMO_UPDATE_CHANNEL`
overrides it; `update.Client.Latest` returns the newest release whose
signed manifest names exactly that channel.

The flavour replaces the compiled channel, and the rule becomes a rank:

| Flavour | Rank | Made by |
|---|---|---|
| `devel` | 0 | every green push to `main` |
| `nightly` | 1 | the schedule |
| `alpha` | 2 | a `v…-alpha.N` tag |
| `beta` | 3 | a `v…-beta.N` tag |
| `stable` | 4 | a `vX.Y.Z` tag |

A box on channel C takes a release of flavour F where F = C, or where both
are in the table and rank(F) ≥ rank(C). Any other name (the phase-0 of the
first releases, a branch's own channel) matches only itself.
Semantic versioning already orders `-alpha` < `-beta` < the release, and
`git describe` orders a rolling build after its tag and before the next.

**The manifest keeps its JSON field `channel`,** holding the flavour,
so that the boxes already out accept the next alpha release as before;
a new field `release` names the GitHub release's tag where it is not the
version (`release/devel`). `verify` checks the release's tag against
`release`, else against `tag`, as now.

**The rolling key** is a second compiled-in public key,
`internal/update/rolling-key.pem`. A manifest that verifies against the
release key is taken for any flavour; one that verifies only against the
rolling key is taken for `devel` and `nightly` alone. A leaked rolling key
then reaches no box on `alpha` or above.

**`Latest`** reads the release list once, as now: the rolling releases
(tag `release/<flavour>`, at most two) are verified first, since their
version is in their manifest only; the numbered ones are tried newest
first by their tag, skipping those whose label is not acceptable
(`hintedChannel` becomes the release's hinted flavour), until one verifies;
the newer of the two answers. Rolling builds need `Tag()` relaxed:
a version with commits ahead is a release where its flavour is rolling.

### The box's channel

serve resolves the channel at each check:
`MASSIMO_UPDATE_CHANNEL`, else the store's `update.channel`,
else the binary's flavour.
When the owner asks for an update to a release of another flavour and
neither is set, serve stores its channel first, so that a beta box which
installs a stable release stays on beta.
The request file carries the channel to the helper as it does now.
`update.channel` is a setting of `PUT /api/v1/settings`, one of the five
names, `null` to clear; the UI's selector is a later item.

`massimo update` takes `-channel`, else the variable, else the store read
only (`openStoreToRead`: a root process must not write serve's store),
else the flavour, and says so where it installs another flavour with
nothing stored.
Where apt or brew own the binary, the channel stays the suite or the
formula, as now.

### The pipeline

```
ci.yml (push to main, v* tag, pull request)
  test, race, fuzz, screenshots, lan, govulncheck, deb, windows, macos, guidelines
  └─ tests: "All tests passed"  (needs all; fails unless none failed or was cancelled)
     └─ release: uses release.yml   (push only)
release.yml (workflow_call; schedule 02:30 UTC; workflow_dispatch)
  meta    → kind (release | devel | nightly | skip), flavour, sha, version
  build   → make release VERSION=… FLAVOUR=… RELEASE=…   (no secret)
  sign    → environment signing:          kind release
            sign, verify, publish the release, fast-forward release/<flavour>
  rolling → environment signing-rolling:  kind devel | nightly
            sign, verify, ancestry guard, replace release/<flavour>
  homebrew, apt-build, apt-deploy → kind release, as now
```

- **A called workflow runs with the caller's ref,** so the environments'
  rules see `refs/tags/v…` or `refs/heads/main`; the schedule runs on
  `main`. That is why the release is called from `ci.yml` rather than
  started by `workflow_run`, which always carries the default branch.
- **`meta`** for a tag: the label as flavour, `stable` without one;
  the tag's commit must be on `origin/main`.
  For a push to `main`: `devel`, the version `git describe --tags --match 'v*'`,
  and `skip` where `release/devel` already names this commit or one after it.
  For the schedule: the owner's rule, below.
- **`rolling`** checks the ancestry once more right before it moves the tag,
  since runs of `main` are never cancelled and an older one can finish last;
  the job's concurrency group is the flavour, so two never publish at once.
  It refuses a flavour the run may not publish: `devel` only from a push to
  `main`, `nightly` only from the schedule or by hand on `main`.
  `sign` refuses a flavour other than the tag's label.
- **Replacing in place:** the first run creates the pre-release; later runs
  move the tag with the API (`PATCH git/refs/tags/release/devel`, `force`),
  upload with `--clobber`, delete the assets the new build no longer has
  (the debs carry the version in their name) and rewrite the notes.
  Editing sends watchers no notification; creating would, at every merge.
  A box downloading during the swap fails the signature check and tries
  again at its next check.
- **The nightly selection:** walk `main`'s first-parent history from its head,
  at most 100 commits, stopping at the commit `release/nightly` names;
  the first commit with a `ci.yml` run whose job "All tests passed"
  succeeded is built. None: nothing published, the run says why.

### The release branch

`make release-alpha` pushes `main` and the tag, atomically, and no longer
`alpha`; its check that `alpha` holds nothing `main` lacks goes.
After the release is published, `sign` pushes the tag's commit to
`release/<flavour>` with the deploy key, a plain push that only
fast-forwards (the ruleset refuses anything else) and creates the branch
the first time. The Makefile's flavour of a checkout: a release tag's label,
`stable` without one; anything else `devel`.

### The apt index for the Site plan

`apt-build` already builds the whole site. It also attaches to the release
`massimo-apt-index.tar.gz`: `apt/dists/` (signed), `apt/massimo.asc`,
`apt/massimo.sources`, the keyring package, and `apt/pool.tsv`, one line per
package: its path under `apt/`, its SHA-256 and the download URL of the
release asset it came from. A site deploy from `main` then rebuilds the
pool from that map and checks each hash, so the pool always matches the
signed index (the Site session's request).

### Tests

- `internal/update`: rank and acceptance; `Latest` across a rolling and a
  numbered release, each newer in turn; a rolling manifest signed with the
  release key, with the rolling key for `devel`, and with the rolling key
  for `alpha` (refused); `verify` with `release`; the manifests already out
  (no `release` field) still verify.
- `mkmanifest`: a rolling version is refused for a numbered flavour.
- `internal/server`: the setting (valid, unknown name, `null`), the channel
  in effect from each source, the channel stored before a flavour change,
  the request's channel; `TestOpenAPICoversRoutes` stays green.
- `cmd/massimo`: `update -check` with each source.
- The workflows: `actionlint` where installed; otherwise only GitHub runs
  them, and Blocks 3 to 7 are verified there (below).

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over the Makefile's `RACEPKGS` where the block
touches Go, and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`
from the main checkout.
Work happens on the branch `claude/channels`, one commit per block,
citing `AP Channels v1.0, Block <n>`, each with its CHANGELOG bullet.
Nothing is pushed but the plan's registration on `main`.

### Block 1: Flavours

1. `internal/update/rolling-key.pem`: the public half the owner made
   (the commands file, step 1). Without it the run stops here.
2. `internal/update/update.go`: `Manifest.Flavour` (JSON `channel`),
   `Manifest.Release`; `RollingKey()`; `Client.RollingKey`;
   `Flavours` and `Accepts(channel, flavour)`;
   `verify` with both keys and the flavour restriction;
   `Latest` as in the Analysis; `hintedFlavour`; `ByTag` for
   `release/<flavour>`.
3. `internal/update/mkmanifest`: `-flavour`, `-release`;
   a version with commits ahead only for a rolling flavour.
4. Every caller of `Release.Channel` → `Flavour`.
5. Tests as in the Analysis.
6. `CHANGELOG.md`, Unreleased, Added.

**Verification:** the test gate.

### Block 2: Box channel

1. `internal/store`: `UpdateChannelKey = "update.channel"`.
2. `internal/server/settings_api.go`: a choice setting, the five names,
   applies now; `settings_api` tests.
3. `internal/server/update_api.go`: `UpdateOptions.Flavour` and
   `UpdateOptions.Override` (the variable) in place of `Channel`;
   the channel in effect resolved at each check; stored before a request
   for another flavour; `GET /api/v1/update` gains `flavour`.
4. `cmd/massimo`: `main.flavour` in place of `main.channel`;
   `serve` and `update` resolve as in the Analysis;
   apt and brew unchanged.
5. `docs/api-v1.md`, `docs/api-v1-openapi.yaml`: the setting, `flavour`.
6. `CHANGELOG.md`, Unreleased, Added.

**Verification:** the test gate.

### Block 3: Pipeline

1. `.github/workflows/ci.yml`: `v*` tags trigger it; the job `tests`
   ("All tests passed"); the job `release` calling `release.yml`
   on a push, `secrets: inherit`, `contents: write`.
2. `.github/workflows/release.yml`: `workflow_call` in place of its tag
   trigger; `meta` (tag → label or `stable`, the commit on `origin/main`;
   the `channel:` line and the branch fallback removed); `build` without
   vet and test; `sign` refusing another flavour; the rest as now.
3. `Makefile`: `FLAVOUR` in place of `CHANNEL` (a tag's label, `stable`
   without one, else `devel`), `-X main.flavour`, `RELEASE` passed to
   `mkmanifest`.
4. `GUIDELINES/project/CI.md`: the pipeline, the tag's flavour.
5. `CHANGELOG.md`, Unreleased, Changed.

**Verification:** the test gate; `make release VERSION=v0.12.0-alpha.1 DRY`
prints the manifest with `flavour` alpha; `actionlint` if present.
On GitHub: the next release cut.

### Block 4: Release branches

1. `release.yml`, `sign`: after publishing, push the tag's commit to
   `release/<flavour>` with `RELEASE_BRANCH_DEPLOY_KEY`; skipped with a
   warning while the secret is missing.
2. `Makefile`, `release-alpha`: pushes `main` and the tag; the `alpha`
   check goes; its comment.
3. `README.md`, "Versions"; `CHANGELOG.md`, Changed.

**Verification:** the test gate; `make release-alpha DRY=1` prints the
push of `main` and the tag only. On GitHub: the next release cut.

### Block 5: `devel`

1. `release.yml`: `meta` for a push to `main`; the job `rolling` in
   `signing-rolling` (sign, verify against `rolling-key.pem`, the flavour
   check, the ancestry guard, replace or create `release/devel`);
   notes from `changelog.sh preview` where the guidelines have it,
   and the commit range since the previous build.
2. The check that no branch `release/devel` exists.
3. `CHANGELOG.md`, Added.

**Verification:** the test gate; the notes step run locally against
`main`. On GitHub: the first push to `main` after the merge, and
`massimo update -check -channel devel`.

### Block 6: `nightly`

1. `release.yml`: `schedule` (02:30 UTC) and `workflow_dispatch`;
   `meta`'s selection as in the Analysis; `rolling` for `nightly`.
2. `CHANGELOG.md`, Added.

**Verification:** the selection script run locally against the
repository's runs; on GitHub, the first night, or
`gh workflow run release.yml --ref main` after the merge.

### Block 7: apt index asset

1. `packaging/apt/publish.sh`: `--index-asset <file>` writes the tarball
   with `pool.tsv`; `packaging/apt/test.sh` checks it.
2. `release.yml`, `apt-build`: attaches it to the release.
3. `packaging/README.md`, "The apt repository"; `CHANGELOG.md`, Added.

**Verification:** the test gate; `make apt-test`.

### Block 8: Documents

1. `README.md`: "Versions" (flavours, channels, the line), `update`.
2. `packaging/README.md`: the environments, the keys, the rulesets, the
   reviewer.
3. `docs/ui.md` if it names the channel.

**Verification:** `GUIDELINES/shared/tools/check`; the links.

### Block 9: Roadmap

1. `docs/roadmap.md`: apt and brew for `devel` and `nightly`;
   `beta`, `rc`, `stable` with `make release-beta`; the UI's channel
   selector; the `All tests passed` job as the ruleset's single required
   check.

**Verification:** `GUIDELINES/shared/tools/check`.

## Proposed commit message

The plan's registration, a commit of its own on `main` before the first
project write:

```
chore(aps): +Channels v1.0

The owner asked for a nightly channel of every green suite on main, or a devel channel beside a nightly one; the plan adds both as rolling releases, separates a build's flavour from the box's channel and makes the release one pipeline after the full suite.

Key changes:
- GUIDELINES/wip/2026-11-10_07-28_AP_Channels_devel_nightly_flavours.md: the plan, nine blocks, D1 to D15 as recommended, D8 with changelog.sh preview, the owner's nightly rule.

File-revision-v1: GUIDELINES/wip/2026-11-10_07-28_AP_Channels_devel_nightly_flavours.md::v1.0::2026-11-10::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `feat(update): Releases carry a flavour, and boxes take every more stable one`
2. `feat(update): A box keeps the channel it follows`
3. `ci(release): A release waits for the full suite`
4. `ci(release): The release branch follows the published release`
5. `ci(release): Every green push to main is published as devel`
6. `ci(release): A nightly build of the newest green commit`
7. `feat(apt): The release carries its signed apt index`
8. `docs(update): Flavours, channels and the release pipeline`
9. `docs(roadmap): The channels' next steps`

## Change History

- v1.0 (2026-11-10): the first revision, by the owner's `PLAN++`;
  D1 to D15 as recommended (D12-A from the "Website build strategy"
  session), D8-A with `changelog.sh preview`, the owner's nightly rule.
