# AP SiteOnly v1.1: Site changes deploy alone

## TL;DR

A change to documents only the site shows runs Guidelines and Site and,
pushed to `main`, deploys Pages directly: no test jobs and no devel build.

- What to test: two answers (untested / code) → three (untested / **site** / code)
- `site` = every changed path in `[ci] untested` or `[ci] site`, none shipped
- `[ci] site` in `GUIDELINES/config.ini`: `docs/**`, `README.md`, `CONTRIBUTING.md`;
  the OpenAPI file included (D64-C)
- v1.1: the site's own pages in `packaging/site/` too, by name, and no longer
  shipped (D66-A, D67-A); `build.sh` and `index.html` stay code (the apt test)
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
4. `packaging/site/`'s pages in `[ci] site` and out of `project.shipped`

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

**v1.1** (`PLAN++ D66-A D67-A`, 2026-11-13): the owner asked why the pull
request `feat(site): The version line sits under the header` (#212), which changes
`packaging/site/site.py`, `doc.html.in` and `site.css` and a fragment,
answered `code=true`: is `packaging/site` not site-only?

- **D66-A:** the site's pages leave `project.shipped`, as
  `:!packaging/site/<file>` exclusions, so the gate's shipped guard stays as
  it is. A change to them alone owes no CHANGELOG bullet any more.
  Not B, keep them shipped and drop the guard from the site answer.
- **D67-A:** `site.py`, `doc.html.in`, `docs.js`, `site.css` and
  `privacy.html`, listed by name: a new file is code until it is listed.
  `build.sh` and `index.html` stay code: the Debian package job's apt test
  builds the site from them. Not B, all of `packaging/site/**`.

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

**`packaging/site/` (v1.1).** `build.sh` makes the site; the Site job runs
it with `--no-apt`, `site.yml` with the apt repository, and
`packaging/apt/test.sh` (`make apt-test`, the Debian package job) with
`--apt`, `--apt-index`, the rolling suites and `newest`, always `--no-docs`.
With `--no-docs` it renders `index.html` (the `{{SUITES}}` line the test
checks) and copies `site.css` and `privacy.html`, which the test compares
byte for byte with their source, whatever they hold; `site.py`,
`doc.html.in` and `docs.js` make the docs, which only the Site job builds.
So the apt test reads `build.sh` and `index.html` alone; the other five are
the Site job's. `project.shipped` names `packaging/**`, and the gate
answers code for any shipped path; the five leave it by name, as
`:!packaging/site/site.py` and so on, plain exclusions, `*` not needed.
`changelog.sh update-contract` renders the list into the contract, and
`AGENTS.md` and `GUIDELINES/README.md` are generated from it.

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

### Block 4: The site's pages (v1.1)

1. `GUIDELINES/config.ini`: `[ci] site` adds the five files;
   `project.shipped` excludes them; its comment says the site's pages are
   not shipped.
2. `changelog.sh update-contract`, then the generated `AGENTS.md` and
   `GUIDELINES/README.md`.
3. `GUIDELINES/project/CI.md` section 2 (v1.48.0): the five files, why
   `build.sh` and `index.html` stay code, and that they are not shipped.
4. `CHANGELOG.d/Changed/claude-site-pages.md`.

**Verification:** the gate's step, run as in Block 1 with this `config.ini`
on pull request #212's head merged onto its base (`site=true`), and on a
change to `packaging/site/build.sh` and to `index.html` (`code=true`);
`fingerprint.sh check`; `changelog.sh check`; the guidelines' `check`.

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

Block 4:

```
perf(pipeline): The site's own pages are the site's documents too

For AP SiteOnly v1.1, Block 4, with the owner's D66-A and D67-A: a change to the site's pages under packaging/site/ ran the whole suite, since packaging/** is shipped and not in [ci] site; site.py, doc.html.in, docs.js, site.css and privacy.html are now the site's documents and not shipped, while build.sh and index.html, which the apt test builds the site from, stay code.

Key changes:
- config.ini: the five files in [ci] site, and excluded from project.shipped
- the contract's shipped list, regenerated, and AGENTS.md and GUIDELINES/README.md with it
- CI.md section 2 names them and why build.sh and index.html stay code

File-revision-v1: GUIDELINES/project/CI.md::v1.48.0::2026-11-13::2::the site's own pages are the site's documents too
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-13): first revision, D62-A, D63-A, D64-C.
- v1.1 (2026-11-13): Block 4, `packaging/site/`'s pages, D66-A, D67-A;
  Blocks 1 to 3 merged as pull request #199.
