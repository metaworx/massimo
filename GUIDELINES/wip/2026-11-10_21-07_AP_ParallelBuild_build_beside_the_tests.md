# AP ParallelBuild v1.0: Build beside the tests

## TL;DR

The release build runs while the tests run, and the keys still wait for
both: about five minutes less for every release and devel build, out of
about fifteen.

- What to test → suite → What to release → Build → sign → publish
  → What to test → suite ∥ (What to release → Build) → sign → publish
- `release.yml` split: `build.yml` (What to release, Build; no secret,
  provenance) + `release.yml` (sign, rolling, Homebrew, apt, site;
  What to release's answers as inputs, the files from the same run)
- `nightly.yml`: the schedule and `gh workflow run nightly.yml`,
  calling `build.yml`, then `release.yml`
- `make release`: `-j4 -Otarget`, each target's output kept together
  for the build record
- a build whose tests then fail is thrown away; nothing is signed

### Blocks

1. `build.yml` and `release.yml` split; ci.yml calls both
2. `nightly.yml`
3. `make -j` in `make release`
4. Documents and the changelog

## Discussion

The owner asked on 2026-11-10 whether the build could start beside the
tests, Screenshots being the longest test and Vet and test the second.
On the last green run of `main` before this plan, Screenshots took about
six minutes, the build about four and a half after All tests passed, and
the run about fifteen.

**The owner's decisions** (`PLAN++`, 2026-11-10, as recommended):

- **D33-A:** Screenshots stays in All tests passed:
  262 s of its six minutes are the web UI's pytest suite, `make ui-test`;
  the screenshots are 47 s. Not B, the screenshots in a job of their own.
- **D35-A:** three files, `build.yml`, `release.yml` and `nightly.yml`.
  Not B, `release.yml` kept whole with an input saying whether it builds:
  every job would carry a condition, and the two paths would run
  different jobs.
- **D36-A:** `make release` builds with `-j4 -Otarget`.
  A cold build cache took 224 s sequentially and 129 s so, on 16 cores;
  the runner has 4, so the first run shows its gain there.
  Not B, sequential.
- **D37-A:** a build whose tests then fail is accepted: runner minutes are
  free for a public repository, and nothing is signed or published.
  Not B, a job that cancels the build when a test fails.

Asked beside it: the release of v0.13.0-alpha.2 asked for two approvals.
`sign` and `apt-build` each run in the `signing` environment, and
`apt-build` starts only once the release is published, so they cannot be
approved together; not changed here.

## Analysis

**What moves where.**
`build.yml` (`workflow_call`) holds the jobs `meta` (What to release) and
`build` unchanged, and returns `meta`'s outputs as its own:
`kind`, `tag`, `sha`, `version`, `flavour`, `release`, `prerelease`.
`release.yml` (`workflow_call`) takes them as inputs of the same names and
holds `sign`, `rolling`, `homebrew`, `apt-build` and `site`, each
`needs.meta.outputs.X` becoming `inputs.X`, and `meta` and `build` leaving
their `needs`. The artifact `release` that `build` uploads is the run's,
so `sign` and `rolling` download it as today. Job names and concurrency
groups may use `inputs`.

**ci.yml.** A job `build` (`uses: ./.github/workflows/build.yml`) needs
What to test only, on a push, unless What to test answered
`suite=false`; it holds `contents: read`, `actions: read` (What to
release reads runs for the nightly build), `id-token` and `attestations`
(the provenance). `release` needs `tests` and `build`, runs on a push
when both succeeded, passes `build`'s outputs, and holds what the
publishing jobs need: `contents: write`, `pages: write`, `id-token: write`.

**nightly.yml.** `schedule` (`30 2 * * *`) and `workflow_dispatch`;
`build` then `release`, the same two workflows. The event inside a
called workflow is the caller's, so What to release picks the nightly
commit and `rolling` admits `nightly` as today, and the environments
match `main`, the ref of the run. `release` runs whenever `build`
succeeded, so that the site is deployed also for a run that releases
nothing, as today.

**The earlier decision.** What to release now decides at the start of the
run, not after the suite. A rolling build's check that its release does
not already name this commit or a later one is made again in `rolling`,
just before publishing, so a newer run that published meanwhile still
wins. A tag's checks do not depend on time.

**Nesting.** ci.yml → `release.yml` → `site.yml`/`publish.yml` stays three
levels deep, ci.yml → `build.yml` two, within GitHub's limit.

**CodeQL.** Alert 6, cache poisoning by the nightly build's checkout of
the commit What to release picks, was dismissed as a false positive for
`release.yml`; moved to `build.yml`, it may be raised again, with the
same answer (`cache: false`, a first parent of `main`).

**`make -j`.** `release` runs `release-files` as a sub-make; with
`-j4 -Otarget` its seven binaries and three packages build side by side,
and GNU make holds each target's output until it ends, so the build
record's log reads target by target. `RELEASE_JOBS` carries the flags,
so that a make without `-O` (macOS's 3.81) can set it empty.

## Implementation Plan

### Block 1: build.yml and release.yml

1. `build.yml`: the header comment's build part, `workflow_call` with the
   seven outputs, `meta` and `build` moved from `release.yml` unchanged
   but the build job's name, "Binaries and packages".
2. `release.yml`: `workflow_call` with the seven inputs; `schedule` and
   `workflow_dispatch` leave; `sign`, `rolling`, `homebrew`, `apt-build`
   and `site` read the inputs.
3. ci.yml: the job `build` after What to test; `release` after `tests`
   and `build`, with the inputs and its permissions.

**Verification:** the three workflows parse; no `needs.meta` is left in
`release.yml`; every input is passed; `go test ./...`.

### Block 2: nightly.yml

1. `nightly.yml`: the schedule, `workflow_dispatch`, `build` then
   `release`, each with its permissions and `secrets: inherit`.

**Verification:** it parses; after the merge, `gh workflow run nightly.yml`
on `main` publishes or skips as the nightly build did.

### Block 3: make -j

1. `RELEASE_JOBS ?= -j4 -Otarget`, passed to the sub-make in `release`.

**Verification:** `make release` locally: the record's log grouped by
target, `sha256sum -c` passes; a failing build still fails it.

### Block 4: Documents

1. `GUIDELINES/project/CI.md`, sections 2 and 6: the build beside the
   tests, the three workflows, the nightly entry.
2. The header comments of the workflows; `README.md` or
   `packaging/README.md` where they name `release.yml`'s schedule.
3. `CHANGELOG.md`: a Changed bullet.

**Verification:** reflow of the changed documents; the full test gate and
`make cross`.

## Proposed commit message

```
perf(pipeline): The release is built while the tests run

Key changes:
- release.yml splits into build.yml (What to release, the build) and
  release.yml (signing and publishing), which ci.yml calls after What to
  test and after All tests passed
- nightly.yml carries the nightly build's schedule and its manual start
- make release builds with -j4 -Otarget
- CI.md and the workflows' comments describe the new order

Plan: AP ParallelBuild v1.0 (Blocks 1-4)

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision, D33-A, D35-A, D36-A, D37-A.
