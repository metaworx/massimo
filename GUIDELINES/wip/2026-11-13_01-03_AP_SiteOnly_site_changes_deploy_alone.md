# AP SiteOnly v1.0: Site changes deploy alone

## TL;DR

A change to documents only the site shows runs Guidelines and Site and,
pushed to `main`, deploys Pages directly: no test jobs and no devel build.

- What to test: two answers (untested / code) → three (untested / **site** / code)
- `site` = every changed path in `[ci] untested` or `[ci] site`, none shipped
- `[ci] site` in `GUIDELINES/config.ini`: `docs/**`, `README.md`, `CONTRIBUTING.md`;
  the OpenAPI file included (D64-C)
- Site job: runs on `site=true` as well
- push to `main`: Release → devel build → Site → a new **Deploy site** job,
  after All tests passed, calling `site.yml`
- any code among the changes → everything, as now

| Change | Guidelines | Site | Tests | Devel build | Pages deploy |
|---|---|---|---|---|---|
| untested only | ✓ | – | – | – | – |
| site (with or without untested) | ✓ | ✓ | – | – | ✓ (`main`) |
| any code | ✓ | ✓ | ✓ | ✓ (`main`) | ✓ through Release |

### Blocks

1. What to test answers `site`; the Site job runs on it; `[ci] site` in `config.ini`
2. The Deploy site job for a site-only push to `main`
3. CI.md section 2, the CHANGELOG fragment, checked against real commits

## Discussion

The run of `chore(aps): -AptUpdate` on `main` (2026-11-12) ran the whole
suite and made a devel build. The push also carried
`docs(roadmap): The update button through apt and brew out of the open work`,
four lines of `docs/roadmap.md`, and `docs/**` is code to the gate (AP DocOnly,
D43-A). The owner (2026-11-13): it makes no sense to test everything
code-related for a change that only changes the published website; how can
the gate tell code tests from docs tests and from the release and deploy?

**The owner's decisions** (`PLAN++ D3-C`, 2026-11-13; numbered D1 to D3 in
the gate, D62 to D64 here):

- **D62-A:** the site list is a project key, `[ci] site`, in
  `GUIDELINES/config.ini`, read with `git config -f` as DocOnly read
  `project.shipped`. Not B, a pathspec in `ci.yml`: a second list away from
  `untested`. Not C, a `--class` option of the shared `fingerprint.sh`:
  a change to mwx-guidelines and a pin bump first.
- **D63-A:** a Deploy site job in `ci.yml`, calling `site.yml`.
  Not B, the nightly's deploy: up to a day late.
  Not C, keep the devel build for site changes: it builds unchanged code.
- **D64-C:** `docs/api-v1-openapi.yaml` is a site document like the rest
  ("if any code actually changes, then the code trigger will be fired
  anyway"). Not A, code; not B, only `go test ./internal/server`.

## Analysis

**What reads the site's documents.** Under `docs/`, Go reads only
`docs/api-v1-openapi.yaml` (`TestOpenAPICoversRoutes`,
`internal/server/openapi_test.go`); the UI suite and the store fixture name
documents in comments only, and Screenshots writes `docs/screenshots/`
without comparing them. Everything there, `README.md` and `CONTRIBUTING.md`
included, is read by the Guidelines job (`tools/identifiers`; the guidelines'
`check`, whose `documents` step keeps README's generated parts in line with
`docs/fragments/`) and by the Site job (`packaging/site/build.sh`: the docs
of the commit, every link checked, a document missing from
`docs/fragments/_documents.md` failing it). None of them is shipped:
`project.shipped` leaves out `docs/**` and `*.md`.

**D64-C's cost.** A change to the OpenAPI file alone no longer runs
`TestOpenAPICoversRoutes`. One that drops or misspells a route passes and
fails the next code change's run instead, which then has to fix it. A change
to a route changes Go and runs it, as before.

**The answer.** After the fingerprint's `same`, where `code` is still true
and the base is known: `git diff --quiet <base> HEAD -- . ':!<p>' …` over
every pathspec of `[ci] untested` and `[ci] site` finds nothing, and
`fingerprint.sh shipped <base>..HEAD` answers `false` → `code=false site=true`.
The range is the one `same` used: a pull request's merge commit against its
first parent, a push's `github.event.before`. An unknown base (all zeros,
not in the history) fails `git diff`, so the answer stays code; a tag and a
release commit have no base. A list value that starts with `:` (pathspec
magic) cannot be negated by prefixing `:!`, so the gate takes such a value
as "no site list" and answers code; `config.ini`'s comment says so.

**The jobs.** Site runs where `site == 'true'`, or as now. Every other test
job keeps `code != 'false'` and is skipped. All tests passed runs and passes
with them skipped; it writes no Tested status, since `fp` is empty where
`code=false`. Build keeps `code != 'false'`, so Release is skipped too.

**The deploy.** `site.yml` is what release.yml calls "for every run, so that
a push to main brings its docs onto the site": it builds the channels' docs
from `release/<channel>` and main, takes the newest release's signed apt
index and the rolling suites' indexes, and deploys in the `pages`
concurrency group. Deploy site calls it from `ci.yml` for a push to `main`
with `site=true` once All tests passed has succeeded, with `contents: read`,
`pages: write`, `id-token: write`; the `github-pages` environment admits
`main`. A pull request never deploys.

**Not changed.** The nightly build may take a site-only commit; its code is
that of the commit before, which was tested (CI.md section 2). Pages keeps
the first deploy of a commit, so a nightly whose head `main` already
deployed changes nothing on the site: true of every push to `main` today,
and not this plan's.

## Implementation Plan

### Block 1: The site answer

1. `GUIDELINES/config.ini`: `[ci] site = docs/**`, `README.md`,
   `CONTRIBUTING.md`, with a comment: what the list is for, positive
   pathspecs only, none shipped.
2. What to test: the `site` output; after `same`, the site check of the
   Analysis; the summary line prints `site=`; the job comment says it.
3. The Site job: `site == 'true'` or its present condition.

**Verification:** the step's script, taken from `ci.yml` and run in a
throwaway worktree with this `config.ini`, for:
the push of `chore(aps): -AptUpdate` (`site=true`);
`chore(aps): -AptUpdate` against its parent (`code=false site=false`);
a Go change (`code=true`); an all-zeros `before` (`code=true`);
a pull request's merge commit of `docs/` alone (`site=true`).
`make actionlint`; `go test ./...`.

### Block 2: Deploy site

1. `ci.yml`: the job `deploy-site`, "Deploy site", `needs: [ changes, tests ]`,
   for a push to `main` with `site == 'true'` and All tests passed a success,
   calling `./.github/workflows/site.yml`; the Release comment and
   release.yml's Site comment name it.

**Verification:** `make actionlint`; after the merge, the next site-only push
to `main` shows Guidelines, Site and Deploy site alone.

### Block 3: Documents

1. `GUIDELINES/project/CI.md` section 2 (v1.47.0): the site answer, its
   list, D64-C's cost, the deploy; the trigger table's Site and the new job.
2. `CHANGELOG.d/Changed/claude-site-only.md`.

**Verification:** `changelog.sh check`; the guidelines' `check` over the
branch; reflow of CI.md.

## Proposed commit message

```
perf(pipeline): A change to the site's documents alone deploys the site alone

For AP SiteOnly v1.0, Blocks 1-3, with the owner's D62-A, D63-A and D64-C:
a push of documents only the site shows, such as a docs/roadmap.md prune with
a plan's retirement, ran the whole suite and made a devel build of unchanged
code; it now runs Guidelines and Site and deploys the site.

Key changes:
- config.ini: [ci] site, docs/**, README.md and CONTRIBUTING.md
- What to test answers site=true where every changed path is untested or
  the site's and none is shipped; Site runs on it, the test jobs and Build
  do not
- ci.yml: Deploy site, for such a push to main, calls site.yml once All
  tests passed has succeeded
- CI.md section 2 states the list, the OpenAPI file's place in it, and the
  deploy

File-revision-v1: GUIDELINES/project/CI.md::v1.47.0::2026-11-13::2::a change to the site's documents alone deploys the site alone
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-13): first revision, D62-A, D63-A, D64-C.
