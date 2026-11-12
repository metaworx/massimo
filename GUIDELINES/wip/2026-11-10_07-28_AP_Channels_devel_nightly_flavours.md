# AP Channels v1.1: Rolling channels and flavours

## TL;DR

Every channel, end to end. Blocks 1 to 9, done (pull request #159),
gave `devel` and `nightly` as rolling releases and made the channel a
box's own setting. Blocks 10 to 16 add the rest, roadmap item 19:
apt and Homebrew for the rolling channels, `beta`, `rc` and `stable`,
and the channel chosen in the web UI.

- `devel` < `nightly` < `alpha` < `beta` < **`rc`** < `stable`;
  a box, suite or formula takes its channel and every more stable flavour
- UI: System settings → Software → the channel (`update.channel`);
  an apt box's helper switches the suite, a brew box shows the commands
- apt `devel`, `nightly` suites → a second apt key in `signing-rolling`;
  the keyring package carries both keys, one file each
- each suite's signed index → an asset of its own release;
  `build.sh` assembles them
- Homebrew `massimo-devel`, `massimo-nightly` → the tap
  `metaworx/homebrew-massimo-rolling`, its key in `signing-rolling`
- `make release-alpha` → `make release-cut FLAVOUR=…`;
  `release/beta`, `release/rc`, `release/stable`
- each channel's landing page names its own suite

### Blocks

1. Flavours: the updater's rank, rolling releases, the rolling key (done)
2. Box channel: the stored setting, serve, `massimo update`, the API (done)
3. Pipeline: `ci.yml` calls the reusable `release.yml`; label → flavour (done)
4. Release branches: `release/alpha` moved by the pipeline;
   `make release-alpha` pushes `main` and the tag only (done)
5. `devel`: published on every green push to `main` (done)
6. `nightly`: the schedule and the owner's selection rule (done)
7. apt index as a release asset, for the Site plan (done)
8. Documents (done)
9. Roadmap (done)
10. The channel selector in the web UI
11. Rolling apt suites: the rolling apt key, both keys in the keyring
    package, each suite's index on its own release
12. Suites and formulae that take every more stable flavour
13. `beta`, `rc` and `stable`: the flavour `rc`, `make release-cut`
14. The apt helper switches the suite
15. Rolling Homebrew formulae in their own tap
16. Documents, each landing page's suite, the roadmap

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

### Item 19, 2026-11-12 (v1.1)

The owner asked for roadmap item 19 to be planned as part of this plan.
The "Website build strategy" session investigated it and asked
D33 to D41 as D1 to D9 of its gates; the owner answered
`PLAN++ as recommended`, with D36 as revised in the second gate.
The decision numbers go on from the NOTEs', which end at D32.

- **D33-A:** v1.1 of this plan, Blocks 10 to 16: the same subject,
  its NOTEs kept. Not B, a v2.0 retiring them; not C, a plan of its own.
- **D34-A:** the rolling apt suites are signed with a second apt key,
  in `signing-rolling`. The keyring package installs both keys,
  one file each, and only a rolling box's source names the rolling one,
  so that key signs nothing an `alpha` box accepts.
  Not B, no rolling suites.
- **D35-A:** each suite's signed index is an asset of its own release:
  the `v*` releases carry the numbered suites', `release/devel` and
  `release/nightly` their own, and `build.sh` assembles them.
  No key leaves the run that signs. Not B, one index that needs both keys
  in one run.
- **D36-A, revised:** one rolling tap, `metaworx/homebrew-massimo-rolling`,
  with two formulae, `massimo-devel` and `massimo-nightly`, and one deploy
  key in `signing-rolling`. The owner pointed out that the rolling
  channels are two flavours; but `main` builds and signs both, and
  `signing-rolling` admits `main` alone, so a second key would be usable by
  exactly the runs that use the first. The alpha tap's key stays out of
  `main`'s reach, which is what a tap of its own buys.
  Not B, two taps and two keys; not C, no rolling formulae.
- **D37-A:** `rc` is a flavour of its own, `-rc.N` tags, ranked between
  `beta` and `stable`, with a branch `release/rc`.
  Not B, `rc` tags under the flavour `beta`: the flavour is the tag's label.
- **D38-A:** one target taking the flavour, `make release-alpha` kept as a
  name for it. The gate called it `make release FLAVOUR=…`;
  `make release` is the build `build.yml` runs, so the target is
  `make release-cut FLAVOUR=…`, with `release-alpha`, `release-beta`,
  `release-rc` and `release-stable` as its names per flavour.
  Not B, a target of its own per flavour.
- **D39-A:** `site.py` writes the channel's suite into the `Suites:` line
  of that channel's landing page; the fragment stays plain text that GitHub
  shows as it is. Not B, a placeholder the README would show raw.
- **D40-A:** on an apt box the update helper switches the suite when the
  owner chooses another channel; on a brew box the selector shows the
  channel fixed, with the commands that change the formula.
  Not B, fixed on both, with the line to edit.
- **D41-A:** the "Nightly channel release strategy" session, which wrote this
  plan, runs Blocks 10 to 16 and retires it; one session writes its
  revisions. Not B, Block 10 and 14 to the "Web UI updates" session;
  not C, the "Website build strategy" session.

**Done before the revision**, so item 19 has less left than it lists:

- "All tests passed" is the one required check beside Guidelines,
  since 2026-11-11;
- the rolling builds' notes use `changelog.sh preview`:
  the guidelines are at v4.57.0;
- the old `alpha` branch is deleted;
- AptUpdate's DebVersion (pull request #171) gives the rolling debs a
  version that sorts after their tag;
- **the owner's steps, 2026-11-12:**
  - the rolling apt key, `MASSIMO_ROLLING_APT_KEY` in `signing-rolling`,
    uid "Massimo apt repository (rolling)", RSA 4096, until 2028-11-11;
    no passphrase secret was set, which the job treats as none,
    as the apt key's;
  - the tap `metaworx/homebrew-massimo-rolling`, public, with a README;
    its deploy key with write access, "rolling formulae (release.yml)",
    the private half `HOMEBREW_ROLLING_TAP_DEPLOY_KEY` in `signing-rolling`
    and kept at `~/.ssh/massimo-homebrew-rolling-tap-deploy-key`;
- **no new rules are needed:** the ruleset
  "release/* branches: fast-forward only" covers `refs/heads/release/*`,
  so `release/beta`, `release/rc` and `release/stable` too,
  and `signing`'s tag rule `v[0-9]*.[0-9]*.[0-9]*` admits `-beta.N` and
  `-rc.N` tags.

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

### Item 19 (v1.1): where things stand

What the code does on 2026-11-12, after pull request #182:

- **The web UI** shows the channel in effect (`update.js`,
  "Running {version} from channel {channel}") and offers no choice;
  `update.channel` is a setting of `PUT /api/v1/settings` since Block 2.
- **On an apt or brew box** the server takes the channel from the suite
  (`install.AptSuite`) or the formula, as `UpdateOptions.Channel`, and the
  stored setting does not count. The apt helper's commands take nothing
  from the request, the channel included; the brew helper ignores it too.
- **`packaging/apt/publish.sh`** builds one suite per numbered flavour from
  the `v[0-9]*` releases, signs every suite with the one key it is given,
  and makes the keyring package from that key.
  `massimo-archive-keyring` installs `/usr/share/keyrings/massimo-archive-keyring.gpg`.
- **`release.yml`**: `apt-build` (environment `signing`, tags only) builds
  and attaches `massimo-apt-index.tar.gz` to the release; `rolling`
  (environment `signing-rolling`, `main`) signs and replaces
  `release/devel` or `release/nightly`, with no apt or Homebrew step.
- **`publish.yml`** renders one formula per numbered flavour
  (`massimo-alpha`, `massimo` for `stable`) into `metaworx/homebrew-massimo`
  with `HOMEBREW_TAP_DEPLOY_KEY`, from `signing`.
- **`packaging/site/build.sh`** takes one apt directory or one index.
- **`make release` is the build of a release** (`build.yml`);
  `make release-alpha` is the only cut.

### Two apt keys, two signers

Each suite is signed by the one run that may:

| Suites | Signed by | Key | `Signed-By:` of a box's source |
|---|---|---|---|
| `alpha`, `beta`, `rc`, `stable` | the tag's run, `apt-build` | `MASSIMO_APT_KEY`, `signing` | `/usr/share/keyrings/massimo-archive-keyring.gpg` |
| `devel`, `nightly` | the rolling build's run, `rolling` | `MASSIMO_ROLLING_APT_KEY`, `signing-rolling` | `/usr/share/keyrings/massimo-rolling-archive-keyring.gpg` |

- **The public halves are committed**, `packaging/apt/archive-key.asc`
  (the key the site publishes as `massimo.asc`) and
  `packaging/apt/rolling-archive-key.asc`, so that either run makes the
  same keyring package with both files. `publish.sh` refuses to sign a
  suite with a key whose fingerprint is not the committed one for it.
- **A box on `alpha` names the first file alone:** a leaked rolling key
  reaches no box on a numbered suite, as the rolling update key reaches no
  box on `alpha` (D2-A).
- **The keyring package's version stays the time of the publish:**
  both runs make one, the contents are the same, and the newer wins.
- **The site publishes both public keys**, `massimo.asc` and
  `massimo-rolling.asc`, for a check by hand.

### Each suite's index on its own release

- The tag's run attaches `massimo-apt-index.tar.gz` to its `v*` release,
  holding the numbered suites, as now.
- The rolling run attaches one to `release/<flavour>`, replaced in place
  with the release's other files, holding its one suite:
  `dists/<flavour>`, the keyring package, and `pool.tsv` with the URLs
  of the debs it indexes.
- `build.sh` assembles them: `--apt-index newest` takes the newest `v*`
  release's index and each rolling release's; `--apt <dir>` (a tag's run)
  takes the directory and adds the rolling releases' indexes.
  Every `pool.tsv` is fetched and checked by its sums, as now.
  A rolling release without an index leaves its suite out with a warning:
  the site must not go offline because a rolling build failed.

### Every more stable flavour

A suite or a formula takes the newest packages of its flavour and of every
more stable one, the order of `update.Accepts`:

- **a numbered suite**, built by a tag's run: the newest releases of its
  flavour and of each more stable flavour, ten of each as now;
- **a rolling suite**, built by its rolling run: its build, and the newest
  numbered releases as they are at that run;
- **a numbered formula:** a tag's run renders its flavour's formula and
  moves every less stable numbered formula whose version is older;
- **a rolling formula:** the newer of its build and the newest numbered
  release, at the rolling run.

The tag's run cannot sign a rolling suite or push to the rolling tap,
so **a numbered release reaches the rolling channels at their next
build**: `devel` at the next push to `main` (the release commit's own push
builds nothing, NOTE "one run per release"), `nightly` the next night.
A box that installs its packages through `massimo update` is not held up:
`Latest` already takes the newest release it accepts.

### `rc` and the cut

- **`update.Flavours`** becomes `devel`, `nightly`, `alpha`, `beta`, `rc`,
  `stable`. A binary already out does not know `rc` and matches it only on
  that name; no box follows `beta` yet, so none misses an `rc`.
- **`make release-cut FLAVOUR=alpha|beta|rc|stable`**, with
  `make release-alpha` and its siblings as its names per flavour:
  - `alpha`: as now, the next `-alpha.N`, `BUMP=minor` for the next minor;
  - `beta`, `rc`: the next `-beta.N`, `-rc.N` of the minor of the newest
    numbered release;
  - `stable`: that minor's `vX.Y.Z` without a label;
    `BUMP=patch` for a fix of a stable release.

  The checks of `release-alpha` (clean `main` equal to `origin/main`,
  not yet released, `changelog.sh check`) stay, as do `DRY=1` and the
  atomic push.
- **`sign` already pushes `release/<flavour>`** for the tag's flavour,
  so `release/beta`, `release/rc` and `release/stable` follow without a
  change there; the ruleset covers them.
- **`publish.sh`'s `channel_of`** already names a suite by the label,
  `stable` without one.

### The selector and the switch

- **Block 10, the selector:** System settings → Software gets a `.field`
  with a `<select>` of the flavours and "the build's flavour" for none,
  written to `update.channel` with `PUT /api/v1/settings` (`null` for
  none), then the update check again. Where a package manager owns the
  binary, the channel is shown fixed with the reason ("follows the apt
  suite alpha"), and the way to change it.
- **Block 14, the switch (D40-A):** the request file already carries the
  channel. Where apt owns massimo and the request's channel is one of
  `update.Flavours` and differs from `AptSuite`, the helper rewrites
  `/etc/apt/sources.list.d/massimo.sources` through `internal/safefile`:
  the `Suites:` line, and `Signed-By:` to the rolling file for `devel`
  and `nightly` or back, every other line kept. It then runs
  `apt-get update` and the upgrade as now, and puts the old file back
  where `apt-get update` fails.
  - **Nothing is downgraded:** `--only-upgrade` keeps a newer version
    until the more stable channel catches up.
  - **A source it did not write is left alone:** several suites, another
    URI, or a file `publish.sh`'s snippet does not match; the status says
    why and names the line to edit.
  - **The trust is the same as now:** the helper of a box `massimo update`
    installed already takes the channel from the request.
  - **The server reads the suite at each check**, so that the UI shows the
    new channel once the switch is done.
  - **On a brew box** the selector stays fixed, with the commands:
    `brew uninstall massimo-alpha && brew install metaworx/massimo-rolling/massimo-nightly`
    and `brew services restart` for the new formula. A formula is changed
    by an uninstall and an install, which is too much for a helper.

### The rolling formulae

- `packaging/homebrew/render.sh` renders `massimo-devel` and
  `massimo-nightly` from the rolling build's commit: GitHub's archive of
  the commit and its SHA-256, the flavour, and a `version` that brew sorts
  after the tag it follows (checked in the block; the deb version is the
  first candidate). Each conflicts with every formula of both taps.
- The `rolling` job pushes it to `metaworx/homebrew-massimo-rolling` with
  `HOMEBREW_ROLLING_TAP_DEPLOY_KEY`; it is skipped with a warning while the
  secret is missing.
- `homebrewJob` already takes the label of any `massimo-<name>` formula.

### The landing pages' suites

`site.py` (AP Site, Block 6) writes each channel's page from its build.
D39-A: it puts the channel's suite into the page's `Suites:` line, and for
`devel` and `nightly` the rolling file into `Signed-By:`; the root keeps
the most stable channel's. `README.md` and the fragment stay on `alpha`.

### Tests for Blocks 10 to 16

- `make ui-test`: the selector, at 1100 and 390 px, light and dark;
  fixed on a box with a manager; the six languages' keys.
- `packaging/apt/test.sh`: a numbered and a rolling suite with two
  throwaway keys; the keyring package with both files; `publish.sh`
  refusing a key that is not the committed one; `build.sh` assembling
  three indexes and leaving out a rolling one that is missing;
  each suite holding its more stable flavours' packages;
  apt in a container verifying both suites with their `Signed-By:`.
- `internal/update`: `rc` ranked between `beta` and `stable`.
- `cmd/massimo`: the apt helper's switch (a source it wrote, one it did not,
  a failed `apt-get update` putting the old file back, an unknown channel);
  `AptSuite` read again at each check.
- `make release-cut FLAVOUR=… DRY=1` for each flavour against a copy
  of the tags.
- `packaging/homebrew/render.sh` for a rolling build, and brew's ordering of
  its version, where brew is at hand (the Mac, on the owner's word).

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

### Blocks 10 to 16 (v1.1)

They run as Blocks 1 to 9 did, with the contract's test gate at the end
of each, `make ui-test` where `internal/server/ui/` changes and
`make apt-test` where `packaging/apt/` or `packaging/site/` does.
Each commit cites `AP Channels v1.1, Block <n>` and carries its
`CHANGELOG.d/` fragment. A branch per pull request, the owner merging:
Block 10 first, since it needs no key; then 11 and 12; 13; 14; 15 and 16.
Block 14 needs 11 and 13, so that every suite it may switch to exists.
Nothing goes to a real box without the owner's word;
the checks on GitHub follow each merge.

### Block 10: The channel selector

1. `internal/server/ui/partials/` (the Software panel) and `update.js`:
   the `<select>` of `update.Flavours` and "the build's flavour", from
   `GET /api/v1/update` (`flavour`, `channel`, `manager`);
   `PUT /api/v1/settings` with `update.channel`, then the check again;
   fixed with the reason where `manager` is set.
2. `ui/i18n/<lang>.json`: the keys in all six, FR/IT/NL/PL marked.
3. `docs/ui.md`; the `CHANGELOG.d/Added/` fragment.

**Verification:** the test gate; `make ui-test` with the new test at
1100 and 390 px.

### Block 11: Rolling apt suites

1. `packaging/apt/archive-key.asc` and `rolling-archive-key.asc`:
   the public halves; the rolling one exported from the owner's keyring,
   `gpg --armor --export "=Massimo apt repository (rolling)"`, read only.
2. `packaging/apt/keyring.nfpm.yaml`: both files.
3. `packaging/apt/publish.sh`: `--rolling <flavour>` builds the one suite
   from `release/<flavour>`'s debs; the key checked against the committed
   one for the suites it signs; `massimo-rolling.asc`; a snippet per kind
   of suite.
4. `.github/workflows/release.yml`, `rolling`: with
   `MASSIMO_ROLLING_APT_KEY` (and `MASSIMO_ROLLING_APT_KEY_PASSPHRASE` where
   set), build the suite and attach its index to `release/<flavour>`;
   skipped with a warning without the key.
5. `packaging/site/build.sh`: the indexes assembled as in the Analysis;
   `site.yml` passes them.
6. `packaging/apt/test.sh`: as in the Analysis; `packaging/README.md`,
   "The apt repository"; `GUIDELINES/project/CI.md`, the secret;
   the `CHANGELOG.d/Added/` fragment.

**Verification:** the test gate; `make apt-test`; `make site`.
On GitHub: the next push to `main` attaches the devel index, and the
site's `dists/devel/InRelease` verifies with the rolling key.

### Block 12: Every more stable flavour

1. `packaging/apt/publish.sh`: a numbered suite with the newest releases of
   its flavour and of each more stable one; a rolling suite with its build
   and the newest numbered releases.
2. `.github/workflows/publish.yml` and `packaging/homebrew/render.sh`:
   a numbered release moves every less stable numbered formula whose
   version is older.
3. `packaging/apt/test.sh`; `packaging/README.md`;
   the `CHANGELOG.d/Changed/` fragment.

**Verification:** the test gate; `make apt-test`.

### Block 13: `beta`, `rc` and `stable`

1. `internal/update`: `rc` in `Flavours`; the tests of the rank;
   `mkmanifest`.
2. `Makefile`: `release-cut` with `FLAVOUR` and `BUMP` as in the Analysis;
   `release-alpha`, `release-beta`, `release-rc`, `release-stable` as its
   names.
3. `README.md`, "Versions"; `GUIDELINES/project/CI.md`, the releases;
   the `CHANGELOG.d/Added/` fragment.

**Verification:** the test gate; `make release-cut FLAVOUR=<each> DRY=1`.

### Block 14: The apt helper switches the suite

1. `cmd/massimo/update_apt.go`: the switch as in the Analysis;
   `internal/install`: writing the source, `AptSuite` read at each check.
2. `internal/server/update_api.go`: on an apt box, a request for another
   channel is accepted; `docs/api-v1.md` and the OpenAPI file.
3. Block 10's selector: not fixed on an apt box any more; `docs/ui.md`.
4. `packaging/README.md`, "Debian package"; the `CHANGELOG.d/Added/`
   fragment.

**Verification:** the test gate; `make deb-systemd-test`, with a switch
from `alpha` to `devel` and back in the container against a local repository.

### Block 15: Rolling Homebrew formulae

1. `packaging/homebrew/render.sh` and `massimo.rb.in`: the rolling build's
   formula as in the Analysis.
2. `.github/workflows/release.yml`, `rolling`: render and push to
   `metaworx/homebrew-massimo-rolling` with
   `HOMEBREW_ROLLING_TAP_DEPLOY_KEY`; skipped with a warning without it.
3. `packaging/README.md`, "Homebrew"; `GUIDELINES/project/CI.md`, the
   secret; the `CHANGELOG.d/Added/` fragment.

**Verification:** the test gate; the render for a rolling build locally.
On GitHub: the next push to `main` puts `Formula/massimo-devel.rb` in the
tap. On the Mac, on the owner's word: `brew install` from it and
`massimo update -check`.

### Block 16: Documents, the landing pages, the roadmap

1. `packaging/site/site.py`: each channel's suite into its page, as in the
   Analysis.
2. `README.md`, "Versions" and "Install"; `packaging/README.md`;
   `docs/ui.md`.
3. `docs/roadmap.md`: item 19 pruned at the retirement, as the contract
   asks; anything left open moved to an item of its own.

**Verification:** `make site`, the links checked; the guidelines'
`tools/check`.

## Proposed commit message

The revision, a commit of its own on `main` before the first project
write of Blocks 10 to 16:

```
chore(aps): +Channels v1.1

The owner asked for roadmap item 19 to be planned as part of AP Channels: apt suites and Homebrew formulae for devel and nightly, beta, rc and stable releases, suites and formulae that take every more stable flavour, and the channel chosen in the web UI.

Key changes:
- GUIDELINES/wip/2026-11-10_07-28_AP_Channels_devel_nightly_flavours.md: Blocks 10 to 16; D33 to D41 as recommended, D36 one rolling tap with one key and D38 named release-cut; what item 19 had already and the owner's steps of 2026-11-12; the TL;DR for the plan as it stands

File-revision-v1: GUIDELINES/wip/2026-11-10_07-28_AP_Channels_devel_nightly_flavours.md::v1.1::2026-11-12::TL;DR, Discussion, Analysis, Blocks 10 to 16::roadmap item 19, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits, as they were made for Blocks 1 to 9
(NOTE "how the run went": `ci(pipeline):`, the component `release`
being the release role's) and as proposed for Blocks 10 to 16:

1. `feat(update): Releases carry a flavour, and boxes take every more stable one`
2. `feat(update): A box keeps the channel it follows`
3. `ci(pipeline): A release waits for the full suite`
4. `ci(pipeline): The release branch follows the published release`
5. `ci(pipeline): Every green push to main is published as devel`
6. `ci(pipeline): A nightly build of the newest green commit`
7. `feat(apt): The release carries its signed apt index`
8. `docs(update): Flavours, channels and the release pipeline`
9. `docs(roadmap): The channels' next steps`
10. `feat(ui): The update channel is chosen under System settings, Software`
11. `feat(apt): The rolling builds have apt suites of their own, with their own key`
12. `feat(apt): A suite and a formula take every more stable flavour`
13. `feat(pipeline): beta, rc and stable are cut as alpha is`
14. `feat(update): The apt helper switches the suite to the chosen channel`
15. `feat(homebrew): The rolling builds have formulae of their own`
16. `feat(site): Each channel's landing page names its suite`

## Change History

- v1.0 (2026-11-10): the first revision, by the owner's `PLAN++`;
  D1 to D15 as recommended (D12-A from the "Website build strategy"
  session), D8-A with `changelog.sh preview`, the owner's nightly rule.
- v1.1 (2026-11-12): roadmap item 19 as Blocks 10 to 16, by the owner's
  `PLAN++ as recommended`; D33 to D41 (asked as D1 to D9 by the
  "Website build strategy" session), D36-A as revised for the two rolling
  flavours, D38-A named `release-cut` since `make release` is the build.
