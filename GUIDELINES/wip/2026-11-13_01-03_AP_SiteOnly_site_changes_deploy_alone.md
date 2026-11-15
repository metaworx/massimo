# AP SiteOnly v1.5: Site changes deploy alone

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
- v1.2: `code=false` → a **Code result** job, the verdict of the last run
  that tested this code; red → All tests passed red on `main`, a warning on a
  pull request; nothing re-run (D68-C, D73-A, D74-A, D75-A)
- v1.3: no run's verdict on a commit (a queued run cancelled) → its Tested
  status, or a merge's pull request head's, with `HEAD`'s fingerprint → green
  (D76-A, D77-A)
- v1.4: a pull request's `lan=false` → its Tested status `lan=unneeded` →
  main's push skips LAN for that code (D78-C); LAN's paths + `internal/sharedport/`,
  `go.mod`, `go.sum` (D79-A); the summary's `ui` and `lan` rows say what the job
  does (D80-A)
- v1.5: a docs-only push to `main` on code no run tested (its merge's run
  cancelled waiting, its pull request on another base) → What to test asks
  Code result first → `code=true`, the run tests it (D81-A)

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
5. The Code result job: a docs-only change carries the code's last verdict
6. Code result reads the Tested status where no run gives a verdict
7. LAN: the pull request's "not needed" carried to `main`; its paths' gaps
8. The summary's `ui` and `lan` rows say whether the job runs
9. What to test tests code no run tested, rather than Code result calling it red

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

**v1.2** (`PLAN++ as recommended`, 2026-11-14): the owner asked why run
43641366251, the merge of `ci(pipeline): The UI suite runs each width in a
job of its own` (#264), published a devel build when the merges with code
before it had failed. It was sound: its tree was the one a pull request run
had tested whole and green. But between the failed merge of #254 and #264,
docs-only commits on `main` had All tests passed green with every code job
skipped, and the nightly takes the newest such commit: it would have
released code that failed. CI.md said their code was "tested"; tested, not
passed. The owner: never re-run a suite whose result is known; a job that
runs where the code jobs are skipped, and carries their last result.

- **D68-C:** the owner's Code result job. Not A, a record per section as
  JSON in a check run: a store and fingerprints of its own. Not B, a green
  base record required: copies that keep a flaky red.
- **D70-B** (settled in the evaluation): a known red is forwarded, not
  re-run.
- **D73-A:** on a pull request the red is a warning and passes, since the
  pull request adds no code; on `main` it fails All tests passed. Not B, red
  on pull requests too: a docs fix could not be merged while `main` is red.
- **D74-A:** where the run to read is still going, Code result waits for
  it. Not B, fail as not known yet.
- **D75-A:** Code result is among All tests passed's needs, nothing more:
  What to release already takes only a commit whose All tests passed
  succeeded. Not B, a second source for Build's What to release.

**v1.3** (`PLAN++ D76-A D77-A`, 2026-11-15): Code result was red on the
push of `chore(aps): -LanPaths` (run 43673977102). The run on the merge of
`Cap Cube pairing time at 255 seconds` (#273) before it, run 43673876936, had
been cancelled while it waited, with no job run: "Canceling since a higher
priority waiting request for CI-refs/heads/main exists". The group keeps one
run going and one waiting, and the push a minute later replaced the waiting
one. The walk took the cancelled run as no verdict and reached the merge of
#274, other code. The owner: Code result should look for other results where
a run was cancelled.

- **D76-A:** a Block 6 of its own, on a branch from `main`, its own pull
  request. Not B, on the What to test summary's pull request (#270).
- **D77-A:** the fingerprint compared is `HEAD`'s, taken by `fingerprint.sh`
  in Code result. Not B, What to test's `fp`, which it takes only where
  `code=true`.

**v1.4** (`PLAN++ D78-C D79-A D80-A`, 2026-11-15): the owner asked whether
LAN against avahi should not run only with the other code tests, and whether
its paths could be tightened; then why it ran in run 43698938871, the merge
of #282, which changed `tools/ui-test/test_scroll.py` alone. Its pull request
answered `lan=false` and wrote `lan=false` into its Tested status; the merge's
push, `tested=true`, skipped every job that had passed and ran LAN and Fuzz,
as a push to `main` always wants them.

- **D78-C:** a pull request's `lan=false` is recorded as `lan=unneeded`, and
  a push to `main` with that record for its code skips LAN. Not A, main runs
  LAN once for every code: the backstop for the path list, and runner time
  for a change that cannot reach discovery. Not B, skip on `lan=false` alone,
  which cannot tell "not needed" from "not passed". The cost: a change the
  list misses never runs LAN, so the list has to be right (D79).
- **D79-A:** the list keeps `internal/server/`'s own files (LanPaths' D1-A)
  and adds `internal/sharedport/` (the shared UDP port of the announcement and
  of discovery), `go.mod` and `go.sum` (`golang.org/x/net`'s multicast). Not B,
  the five files of `internal/server/` that touch discovery, which would undo
  D1-A; not C, as it is.
- **D80-A:** the summary's `ui` and `lan` rows say what the job does: skipped
  with no code test, not needed, passed before, or it runs. It was to go onto
  #270, which the owner merged before this revision, so it is Block 8 on
  Block 7's branch.

**v1.5** (`PLAN++ D81-A with the new AP revision in the same commit`,
2026-11-15): the owner asked whether Code result was right on the push of
`chore(aps): -IconChoice` (run 43731468781). It was, by its rules: the run on
the merge of #294 before it, run 43731237633, was cancelled three seconds
after it was made, and #294's pull request had tested it on a `main` without
#293 (page tabs), merged in between, so its record holds other code. No run
tested the two together. But that is not known to fail, and red keeps
`main` and the nightly stuck until the next code push.

- **D81-A:** What to test asks Code result's question before the run, on a
  push to `main` whose answer is `code=false`: where no run gave a verdict
  and none is going, `code=true`, and the run tests the code. Code result's
  red then means a run that failed. Not B, Code result starting a run
  (`workflow_dispatch`, `actions: write`): a trigger of its own and a
  second run. Not C, red until the next code push.
- The plan's revision is committed with Block 9 on its branch, as the
  owner's signal asks, not on `main` first.
- D82 (re-running the cancelled run to clear `main` now) was not signalled
  and is not done.

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

**The Code result job (v1.2).** It runs where What to test answers
`code=false`, `site=true` among them, and nowhere else; All tests passed
needs it. Its script, `packaging/code-result.sh` beside
`packaging/wait-for-job.sh`, walks `main`'s first parents from `HEAD^1`
(a push's commits before it, a pull request's base) while a commit holds the
same code as `HEAD`: `git diff --quiet <commit> HEAD -- .` with every
`[ci] untested` and `[ci] site` path excluded, as What to test's site check
does. For each such commit, newest first, it reads ci.yml's runs on it
(`actions/workflows/ci.yml/runs?head_sha=`), this run left out, and of each
the latest attempt's jobs, the code jobs being all but What to test,
Guidelines, Site, Code result, All tests passed, Deploy site and the called
Build and Release:

- a run not completed yet: it waits, polling as `wait-for-job.sh` does,
  which itself only waits for a job of its own run (D74-A);
- a run in which Code result ran: a verdict carried, not made; the walk
  goes on, so that the run that made it is read, its latest attempt: a
  failed job re-run green clears every docs-only commit after it;
- a cancelled run, or one whose code jobs were all skipped and whose commit
  carries no green Tested status: no verdict; the walk goes on (a release
  commit's push runs nothing; its tag's run, on the same commit, is read);
- a code job failed, timed out or was cancelled: **red**, that run named;
- else **green**: the jobs passed, or were skipped on a green record.

The walk ends at the first commit with other code, after 100 commits, or
where history ends: no run tested this code, which is red. Red fails the job
on a push and passes it with a warning on a pull request (D73-A); either way
the summary names the run. It needs `actions: read` and `statuses: read`.

**Where no run gives a verdict (v1.3).** The pull request's head of #273
carries a green Tested status, `fp1:03d9dda1…`, and the merge commit and
`-LanPaths` have that fingerprint: the run on the merge, had it started,
would have found it as What to test's `prev` (a merge's second parent),
skipped every code job and passed. Code result read a Tested status only for
a run whose code jobs were skipped, and only on its own commit. So, once a
commit's runs give no verdict (none, or all cancelled or skipped), it reads
the Tested status where What to test reads it: on the commit, then, for a
merge, on its second parent. Green, written by this workflow and opening
with `HEAD`'s fingerprint → green, the status's run named. Anything else and
the walk goes on. A run's red still comes first, since a commit's runs are
read before its record. The fingerprint needs `fingerprint.sh`, so Code
result's checkout takes the submodules. The per-run check of a run that
skipped its jobs folds into it.

**LAN's record (v1.4).** The Tested status holds `lan=true` where LAN
passed for the code, in the run or before it, and `lan=false` otherwise;
What to test reads `lan=true` as `lan_done=true`. A push sets `lan=true`,
and only a pull request's filter sets it false. So the merge of a pull
request the filter judged clear of discovery runs LAN on `main`, alone. Now
All tests passed writes `lan=unneeded` where What to test answered
`lan=false` and LAN did not pass; What to test, on a push, reads it as
`lan=false`, so LAN is skipped and the run's own status carries
`lan=unneeded` on. A pull request ignores it: its own filter decides. A
direct push to `main` without a record runs LAN as before. The status stays
within 140 characters: 68 for the fingerprint, 32 for the rest. Fuzz keeps
its rule: a pull request never runs it.

**LAN's paths (v1.4).** `serve` imports nearly every package, so the list
cannot come from the import graph. Of the packages the LAN test goes
through, only `internal/sharedport/` is missing: `internal/announce`'s
responder and `internal/cubelan`'s search share their UDP ports through it,
and the test checks `serve` beside avahi-daemon on 5353. `go.mod` and
`go.sum` pin `golang.org/x/net`, whose `ipv4` and `ipv6` the announcement's
multicast uses.

**The summary's rows (v1.4).** `ui` and `lan` said "wanted" or "not wanted"
whatever the run did with them. The row says, in this order: skipped, no
code test (`suite` or `code` false); not needed (the answer false); passed
for this code before (`ui_done`, `lan_done`); else it runs. Fuzz's row stays.

**Code whose verdict is unknown (v1.5).** Code result's walk ends in one
of three ways: a verdict (a run green or red, or the record green); a run
still going, which it waits for; or no run tested this code (the walk
reaches other code, 100 commits, or the history's end). The third is no
verdict, but it fails as red. `CODE_CHECK=1` makes the script answer only
which way it ends, waiting for nothing and writing no summary: exit 0 for
the first two, 1 for the third, 2 where the history or the API cannot be
read. What to test runs it after the site check on a push with
`code=false`. On 0 it keeps `code=false` and says the verdict; on anything
else it answers `code=true site=false`, so the run tests the code, Build and
Release run, and Release's Site deploys the documents (Deploy site is for
`site=true` alone and would deploy twice). A pull request is left out: its
Code result is a warning anyway, it adds no code, and its checkout's
history, three commits deep, cannot be walked. What to test needs
`actions: read` for the runs.

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

### Block 5: Code result (v1.2)

1. `packaging/code-result.sh`: the walk, the wait, the verdict, the summary.
2. `ci.yml`: the job `code-result`, "Code result", for `code == 'false'`
   with `suite != 'false'`, a full checkout; in All tests passed's needs;
   its comment and What to test's name it.
3. `GUIDELINES/project/CI.md` section 2 (v1.49.0): the job, replacing
   "its code is that of the commit before it, which was tested".
4. `CHANGELOG.d/Fixed/claude-code-result.md`.

**Verification:** the script against `main`'s real runs: on
`chore(aps): -DiscoveryShare, -ANALYSIS DiscoveryShare` (after the failed
merge of #254: red, that run named), on
`chore(aps): +NOTE CopyFacts v1.0, +NOTE CubeBadges v1.1` (red through the
docs-only commits before it), on a docs-only commit after the merge of #264
(green), as a pull request (red as a warning, exit 0); shellcheck;
`make actionlint`.

### Block 6: The Tested record where no run gives a verdict (v1.3)

1. `packaging/code-result.sh`: `HEAD`'s fingerprint at the start; after a
   commit's runs, its Tested status, then a merge's second parent's, matched
   on that fingerprint; the header comment says it.
2. `ci.yml`: Code result's checkout with `submodules: true`; its comment.
3. `GUIDELINES/project/CI.md` section 2 (v1.54.0): the record where no run
   gives a verdict.
4. `CHANGELOG.d/Fixed/claude-code-result-record.md`.

**Verification:** the script against `main`'s real runs: on
`chore(aps): -LanPaths` (red before, green now, the status of #273's head
named); on `chore(aps): -DiscoveryShare, -ANALYSIS DiscoveryShare` (red, as
before); on a docs-only commit after the merge of #264 (green); a mismatched
fingerprint (red); shellcheck; `make actionlint`.

### Block 7: LAN's record and paths (v1.4)

1. `ci.yml`, What to test: `lan=unneeded` in the record → `lan=false` on a
   push; the filter adds `internal/sharedport/`, `go.mod`, `go.sum`; the job
   comments.
2. `ci.yml`, All tests passed: `LAN` is `unneeded` where `lan` was false and
   LAN did not pass.
3. `GUIDELINES/project/CI.md` (v1.56.0): the record's `lan=unneeded`, the
   table's row and section 2's list.
4. `CHANGELOG.d/Changed/claude-lan-unneeded.md`.

**Verification:** What to test's step, run by hand as in Block 1, on the
merge of #282 with a record `lan=unneeded` (`lan=false`), with `lan=false`
(`lan=true`), as a pull request with that record and a LAN path (`lan=true`);
the filter on a change to `internal/sharedport/` and to `go.mod`
(`lan=true`); All tests passed's expression by actionlint; `make actionlint`.

### Block 8: The summary's rows (v1.4)

1. `ci.yml`, What to test: the `ui` and `lan` rows by the order of the
   Analysis.

**Verification:** the step's summary, run by hand, for a docs-only push, a
pull request of the UI suite alone, a merge with `ui=true`, and a push with
nothing recorded; `make actionlint`.

### Block 9: Code no run tested is tested (v1.5)

1. `packaging/code-result.sh`: `CODE_CHECK=1`, exit 0, 1 or 2 as the
   Analysis says; the header comment.
2. `ci.yml`, What to test: the check after the site check on a push with
   `code=false`; `actions: read`; the code row's words.
3. `GUIDELINES/project/CI.md` section 2 (v1.57.0).
4. `CHANGELOG.d/Fixed/claude-code-unknown.md`.
5. This revision, in the same commit.

**Verification:** What to test's step, run by hand with this branch's
script on `main`'s real commits: `chore(aps): -IconChoice` (`code=true`),
`chore(aps): -LanPaths` (green through the record, `code=false`),
`chore(aps): -DiscoveryShare, -ANALYSIS DiscoveryShare` (red, `code=false`),
`chore(aps): +SiteOnly v1.2` (green, `code=false`); Code result without the
check on `-IconChoice` (red, exit 1, as before); the check with an unreadable
API (exit 2); shellcheck; `make actionlint`.

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

Block 5:

```
fix(pipeline): A docs-only change carries the last verdict on its code

For AP SiteOnly v1.2, Block 5, with the owner's D68-C, D70-B, D73-A, D74-A and D75-A: a docs-only commit on a main whose code had failed had All tests passed green with every code job skipped, and the nightly takes the newest such commit; a Code result job now carries the verdict of the last run that tested the code, without running it again.

Key changes:
- packaging/code-result.sh walks main's first parents while the code is the same and reads the latest attempt of the run that tested it, waiting for one still going
- ci.yml: Code result for code=false, among All tests passed's needs; red fails it on a push, and is a warning on a pull request
- CI.md section 2 states it

File-revision-v1: GUIDELINES/project/CI.md::v1.49.0::2026-11-14::2::a docs-only change carries the last verdict on its code
Assisted-by: claude-code:claude-opus-5-5
```

Block 6:

```
fix(pipeline): Code result reads the Tested record where a run was cancelled

For AP SiteOnly v1.3, Block 6, with the owner's D76-A and D77-A: a run on main cancelled while it waited, replaced by the next push, gave no verdict, and Code result walked past code that had passed in its pull request; it now reads the Tested status where What to test reads it.

Key changes:
- packaging/code-result.sh: where a commit's runs give no verdict, its Tested status, or a merge's pull request head's, with HEAD's fingerprint is green
- ci.yml: Code result checks out the guidelines, for fingerprint.sh
- CI.md section 2 states it

File-revision-v1: GUIDELINES/project/CI.md::v1.54.0::2026-11-15::2::Code result reads the Tested record where a run was cancelled
Assisted-by: claude-code:claude-opus-5-5
```

Block 7:

```
ci(pipeline): A pull request's LAN not needed is not run again on main

For AP SiteOnly v1.4, Block 7, with the owner's D78-C and D79-A: the merge of a pull request whose filter found LAN not needed ran LAN alone on main, since a push always wants it; the Tested status now records it as unneeded, and the job's paths take in the shared port and the Go module.

Key changes:
- ci.yml: All tests passed writes lan=unneeded where LAN was not wanted and did not pass; What to test reads it on a push as lan=false
- What to test's LAN filter adds internal/sharedport/, go.mod and go.sum
- CI.md states the record and the list

File-revision-v1: GUIDELINES/project/CI.md::v1.56.0::2026-11-15::2::a pull request's LAN not needed is not run again on main
Assisted-by: claude-code:claude-opus-5-5
```

Block 8:

```
ci(pipeline): The summary's UI and LAN rows say whether the job runs

For AP SiteOnly v1.4, Block 8, with the owner's D80-A: the rows said wanted or not wanted whatever the run did; they now say skipped without a code test, not needed, passed for this code before, or that the job runs.

Key changes:
- ci.yml: What to test's ui and lan rows

Assisted-by: claude-code:claude-opus-5-5
```

Block 9:

```
fix(pipeline): A docs-only push tests the code no run tested

For AP SiteOnly v1.5, Block 9, with the owner's D81-A: a merge whose run main's group cancelled while it waited, its pull request tested on another base, left code no run tested, and Code result called every docs-only push after it red until the next code push; What to test now asks Code result first, and the run tests such code.

Key changes:
- packaging/code-result.sh: CODE_CHECK=1 answers whether the code's verdict is known, without waiting
- ci.yml: What to test, on a push with code=false and no verdict, answers code=true; it reads the runs with actions: read
- CI.md section 2 states it
- the plan's v1.5, in this commit

File-revision-v1: GUIDELINES/project/CI.md::v1.57.0::2026-11-15::2::a docs-only push tests the code no run tested
File-revision-v1: GUIDELINES/wip/2026-11-13_01-03_AP_SiteOnly_site_changes_deploy_alone.md::v1.5::2026-11-15::9::Block 9, code no run tested is tested, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-13): first revision, D62-A, D63-A, D64-C.
- v1.1 (2026-11-13): Block 4, `packaging/site/`'s pages, D66-A, D67-A;
  Blocks 1 to 3 merged as pull request #199.
- v1.2 (2026-11-14): Block 5, the Code result job, D68-C, D70-B, D73-A,
  D74-A, D75-A; Block 4 merged as pull request #214.
- v1.3 (2026-11-15): Block 6, the Tested record where no run gives a verdict,
  D76-A, D77-A; Block 5 merged as pull request #269.
- v1.4 (2026-11-15): Blocks 7 and 8, LAN's record and paths and the
  summary's rows, D78-C, D79-A, D80-A; Block 6 merged as pull request #280,
  the summary as #270.
- v1.5 (2026-11-15): Block 9, code no run tested is tested, D81-A;
  Blocks 7 and 8 merged as pull request #284.
