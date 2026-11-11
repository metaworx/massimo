# AP ReleaseGates v1.0: One approval per release

## TL;DR

A release asks for the owner's approval once, the Homebrew tap's key is
held behind it like the other release keys, and the nightly build leaves
a release commit to its tag's run.

- approvals per release: 2 (`sign`, then `apt-build`) → 1
  (`sign`, `apt-build`, `homebrew` waiting on `signing` together)
- `apt-build`, `homebrew`: `needs: sign` → start together, wait inside
  for "Sign and publish" to succeed (`packaging/wait-for-job.sh`)
- `HOMEBREW_TAP_DEPLOY_KEY`: repository secret, any ref → `signing`
  environment, `v*` tags, approved
- nightly at a release commit: built and deployed the site at the same
  commit as the release → no build, no site; the tag's run does both

### Blocks

1. The nightly leaves a release commit alone
2. publish.yml in the `signing` environment
3. One approval: the release jobs start together
4. Documents and the changelog

## Discussion

On 2026-11-11 the release of v0.14.0-alpha.1 asked for two approvals,
and the apt repository missed it afterwards:
the scheduled nightly build fired at 09:37 UTC on the release commit,
`main`'s head while the tag's run waited for the second approval,
and deployed the site with the index of the newest published release,
v0.13.0-alpha.2.
The release's own deploy of the same commit, nine minutes later,
reported success and changed nothing:
Pages keeps the first deploy of a commit,
and deploy-pages takes the commit from `GITHUB_SHA`,
which a workflow cannot choose.
A nightly build dispatched by hand on the next commit of `main` repaired it.

The owner asked whether the Homebrew job, which runs after `sign`
without an approval of its own, should not take the apt deploy's path,
and whether the signing approval could be passed on so that one suffices.

**The owner's decisions** (`PLAN++ as recommended`, 2026-11-11):

- **D55-A:** the nightly build stops where `main`'s head is a release
  commit, its version tag at it: no build and no site.
  The tag's run builds, publishes and deploys that commit.
  Not B, a site job that skips a deploy of the commit Pages already
  serves: an API call in every run, for a case only the nightly makes.
- **D56-A:** `HOMEBREW_TAP_DEPLOY_KEY` moves into the `signing`
  environment, publish.yml's job runs in it;
  by hand it needs `--ref <tag>`, since the environment admits `v*` tags alone.
  Not B, an environment of its own with an approval of its own.
- **D57-A:** `sign`, `apt-build` and `homebrew` start together,
  so that one approval of `signing` lets all three run;
  the two followers wait inside their job until "Sign and publish" has
  succeeded, and stop when it failed.
  Not B, the apt and tap steps inside `sign`, which would run gpg,
  apt-ftparchive and `go run` beside the release key;
  not C, an environment without an approval for `apt-build`,
  which would hand the apt key to any run of a `v*` tag unreviewed.
- **D58-A:** this one plan, through a pull request;
  the next release proves Blocks 2 and 3.
  Not B, AP Channels v1.1.

An approval cannot be passed from one job to another:
GitHub asks once per job that waits on a protected environment,
and only a custom deployment protection rule, a GitHub App the project
would host, could approve on its own.
One review does approve every job of the run waiting on the same
environment at that moment, which is what Block 3 uses.

## Analysis

**Why two approvals.**
`apt-build` has `needs: sign`, so it reaches the `signing` environment
only after `sign` has finished, and GitHub asks again.
`homebrew` has `needs: sign` too, but publish.yml's job names no
environment, and its key is a repository secret:
any workflow run on any branch of the repository can read it and push a
formula to the tap, which `brew upgrade` installs.

**What the followers wait for.**
`apt-build` rebuilds the apt repository from the published releases and
attaches the index to the new one; `homebrew` should push no formula for
a release that was not published.
Both wait for the job "Sign and publish" of the same run
(as `Release / Sign and publish` through ci.yml) to conclude:
success lets them on, any other conclusion stops them, and a deadline of
30 minutes ends a wait that sees neither.
The jobs are read with `filter=all`, so that a re-run of a failed
follower finds the attempt in which `sign` succeeded;
a re-run of `sign` itself fails on the existing release as before.
Reading the jobs needs `actions: read`,
which every caller grants: a called workflow's job cannot ask for more
than its caller's job, and GitHub checks that when the run starts,
for nightly.yml too, where no release runs.

**The nightly build's test.**
The release commit and its tag are pushed together (`make release-alpha`),
so a release commit at `main`'s head always has its `v*` tag already:
`git tag --points-at` in `meta`, whose checkout has every tag.
`meta` then answers `kind=skip` and `site=false`;
release.yml takes `site` as an input, `true` unless a caller says otherwise,
and only nightly.yml passes it on.
A nightly build at a later commit, while a release still waits, deploys
another commit, so Pages takes the release's deploy after it.

## Implementation Plan

### Block 1: The nightly leaves a release commit alone

- build.yml's `meta`: for the schedule and a start by hand,
  where a `v*` tag points at `GITHUB_SHA`, say so and answer
  `kind=skip`, `site=false`; a new output `site`, `true` otherwise.
- release.yml: an input `site`, not required, default `true`;
  the job `site` runs only where it is not `false`.
- nightly.yml passes `site` on.
- **Verification:** `actionlint` where available, else a YAML parse;
  the shell of the `meta` step through `bash -n`;
  the test in a checkout at the release commit of v0.14.0-alpha.1.

### Block 2: publish.yml in the `signing` environment

- publish.yml's job: `environment: signing`, `actions: read`,
  and an input `wait` (release.yml passes `true`) that runs the wait of
  Block 3 first; its header says a run by hand needs `--ref <tag>`.
- The owner moves the secret: `HOMEBREW_TAP_DEPLOY_KEY` as a secret of
  `signing`, then the repository secret deleted.
  Until then the job still reads the repository secret.
- **Verification:** YAML parse; the next release.

### Block 3: One approval: the release jobs start together

- `packaging/wait-for-job.sh <job name>`: waits for the job of this run
  and attempt history, by its name or `<caller> / <name>`;
  exit 0 on success, 1 on any other conclusion or after the deadline.
- release.yml: `apt-build` and `homebrew` lose `needs: sign`,
  `apt-build` waits first; both get `actions: read`.
- ci.yml's and nightly.yml's `release` jobs grant `actions: read`.
- **Verification:** `bash -n` and shellcheck where available;
  the script against a past run of ci.yml (`GITHUB_RUN_ID` of the release
  of v0.14.0-alpha.1); the next release asks once.

### Block 4: Documents and the changelog

- `GUIDELINES/project/CI.md` sections 5 and 6, `packaging/README.md`
  ("Homebrew", the owner's steps, and the release key's paragraph),
  `docs/roadmap.md` item 2's secret.
- Fragments under `CHANGELOG.d/Changed/`.
- **Verification:** `reflow.sh` no worse, `changelog.sh check --staged`,
  `check-citations.sh`.

## Proposed commit message

```text
ci(release): One approval per release, the tap key behind it

AP ReleaseGates v1.0, Blocks 1 to 4.

Key changes:
- The nightly build leaves a release commit to its tag's run: no
  build and no site, since Pages keeps the first deploy of a commit.
- sign, apt-build and homebrew start together, so that one approval
  of the signing environment lets all three run; the followers wait
  for "Sign and publish" (packaging/wait-for-job.sh).
- publish.yml runs in the signing environment, where
  HOMEBREW_TAP_DEPLOY_KEY moves.

File-revision-v1: GUIDELINES/wip/2026-11-11_11-23_AP_ReleaseGates_one_approval_per_release.md::v1.0::2026-11-11::All::Blocks 1 to 4
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-11): first revision, with D55-A to D58-A.
