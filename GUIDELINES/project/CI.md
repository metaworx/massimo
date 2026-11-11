# Massimo — CI Conventions (v1.38.0)

What CI does in this project, for whoever has to change it.
`/AGENTS.md` names this file as the canonical owner of CI conventions,
so a rule that lives only in a workflow YAML — a version pin,
an ordering constraint, a reason a step exists — belongs here as well,
in words.
The pipeline is the source of truth for *what* runs;
this document is the source of truth for *why*.

## Contents

<!-- BEGIN GENERATED CONTENTS - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
1. Where CI Is Defined
2. What Runs, and When
3. The Commands Are the Project's Commands
4. Guideline Gates
5. Secrets and Variables
6. Releases
7. Runner Constraints
8. Dependabot and the Repository's Settings
9. Document Governance
<!-- END GENERATED CONTENTS -->

## 1. Where CI Is Defined

| Platform | Files |
|----------|-------|
| GitHub Actions | `.github/workflows/ci.yml` (a push to `main` or of a tag `v*`, a pull request), `.github/workflows/build.yml` (the release's build, called by ci.yml beside its tests and by nightly.yml), `.github/workflows/release.yml` (signing and publishing, called by ci.yml once its tests have passed and the build is done, and by nightly.yml), `.github/workflows/nightly.yml` (the nightly build, daily at 02:30 UTC and by hand on `main`), `.github/workflows/publish.yml` (called by release.yml, or run by hand for a tag) |

There is no other platform.

## 2. What Runs, and When

| Trigger | Jobs |
|---------|------|
| push to `main`, pull request (each push to its branch) | Vet and test (gofmt, go vet per platform, go test, the store's benchmarks once each with `-short`, `make cross`); Race (`make race`, the race detector on `internal/server`, `internal/store`, `internal/cubelan`, `internal/proxy`, `internal/serve`, `internal/announce` and `internal/notify`); Test (armv6, QEMU) (`make test-armv6`, every package's tests with `-short` on 32-bit ARM, run by Ubuntu's `qemu-user` as the Raspberry Pi 1's core); Fuzz on a push to `main` only (`make fuzz`, every fuzz target for 10 s, a crash's input kept as the artifact `fuzz-crash`, Go 1.26's own deadline passed with a warning); govulncheck for linux and windows; Debian package (`make packages`, then the amd64 one installed, checked and purged in a trixie and a bookworm container, then under systemd, each case in a container of its own (`make deb-systemd-test`: the first start, the restart, the upgrade from v0.12.0-alpha.1 with the service running), and installed again from an apt repository built with a throwaway key; the packages uploaded as the artifact `packages`); Windows service (the unit tests of the update, the installer, the command line, discovery and `internal/safefile`; install, run, update from the API and by hand, roll back, remove, on a Windows runner; then a second `serve` announcing itself beside Windows' own mDNS, found by `massimo discover`); macOS LaunchDaemon (the unit tests of the update, the installer, the command line and `internal/safefile`; first `install` on the template's settings, `packaging/launchd/template-test.sh`: the message and its owner line run as printed, a loaded job restarted, a booted-out one left stopped, the web UI's way, the purge; then the same as Windows on a macOS runner, with a self-signed certificate: the service installed with `-tls self-signed`, the health check and the API over `https://`; then the Homebrew formula rendered for a tarball of the checkout, built from source, tested and checked for the marker; then a second `serve` beside Apple's mDNSResponder, found and resolved by `dns-sd`); Screenshots (the web UI's pytest suite, then the screenshot set, uploaded as the artifact `ui-screenshots`, the PNGs only, and on failure the failed tests' screenshots and videos and the traces of their run once more as `ui-test-results`; on a pull request only when it changes the UI, its suite or `ci.yml`, as the job What to test answers; all four of the suite's cases, in four workers); LAN against avahi (`make lan-test`: a second `serve`'s searches finding a fake Cube, losing it with its cable pulled and finding it again, then from a box's proxy too; `serve` beside avahi-daemon, `avahi-browse` and `massimo discover` on a client, in Docker; on a pull request only when it changes the announcement, discovery, the proxy, the LAN test or `ci.yml`, as the job What to test answers); Guidelines (section 4); Site (`packaging/site/build.sh --no-apt`: the site's pages and the docs of the commit, every link checked, a document under `docs/` missing from `docs/fragments/_documents.md` failing it; never deployed) |
| beside the jobs above, on a push | Build (build.yml): What to release decides what the run releases, then every target built without cgo with the Debian packages, the build record, `SHA256SUMS` and the manifest, the provenance attested; nothing signed (section 6) |
| after every job above | All tests passed: fails unless none of them failed or was cancelled; then, on a push, once Build is done, Release (release.yml): a push to `main` becomes the `devel` build, the GitHub release `release/devel` replaced in place, signed with the rolling key (section 6); then, at every run of release.yml, Site (site.yml): the site built with the newest release's signed apt index and the docs of every channel, and deployed to GitHub Pages; nothing where no release carries the index yet |
| push to a release branch (`release/alpha`, later `release/beta` and `release/stable`) | nothing: ci.yml listens to `main` and the tags; the release moved the branch to a commit ci.yml had tested |
| tag `v*` | every job above, as for a push to `main`, Build included; then release.yml: sign the manifest with the release key, verify the signature against the public key compiled into `massimo`, publish the GitHub release; then, with the deploy key set, the Homebrew formula of the release's channel rendered and pushed to the tap (publish.yml, section 6), and, with the apt key set, the apt repository rebuilt from the releases and deployed to GitHub Pages |
| daily at 02:30 UTC, or by hand (`gh workflow run nightly.yml --ref main`) | nightly.yml: build.yml, then release.yml, no tests: the nightly build, `release/nightly`, of the newest commit of `main` whose suite passed and that is newer than the one it carries; nothing where there is none (section 6) |
| by hand (`gh workflow run publish.yml -f tag=<tag>`) | publish.yml: the formula of a release already out, such as the one published before the tap existed |

Each commit runs the workflow once:
a push to a pull request's branch runs it as the pull request's,
not a second time as a push,
and the commits `main` receives run it as pushes to `main`.
A release commit runs the suite once, as its tag's:
its push to `main` runs What to test alone (below).
The release waits for the whole suite of the tag's run,
and runs with the tag as its ref,
which is why ci.yml calls release.yml (`workflow_call`)
rather than release.yml listening for ci.yml's end (`workflow_run`):
such a run carries the default branch,
which the `signing` environment does not admit.
The release branches receive only commits already on `main`,
so a push to one runs nothing.
The release is built beside the test jobs (build.yml, ci.yml's Build),
not after them:
the build holds no secret, and nothing is signed until All tests passed,
which Release waits for with the build;
a build whose tests fail is thrown away with the run.
That saves the build's four and a half minutes,
which used to follow the six of Screenshots,
the longest test job (owner, 2026-11-10, AP ParallelBuild).
build.yml's What to release therefore decides at the start of the run;
`rolling` checks the release's commit again just before publishing.
A job after one that can be skipped names a status function in its `if:`:
GitHub otherwise adds `success()`,
which is false while any job up the chain was skipped, not only its own `needs`.
Fuzz is skipped on every pull request, and every job on a release commit's push,
so the test jobs, All tests passed and Release
(`!cancelled() && … && needs.tests.result == 'success'`) name one;
without it the first `devel` build, on 2026-11-10, was skipped.
Every test job waits for What to test, the gate that opens ci.yml:
on a push to `main` of a release commit,
whose summary opens with `chore(release): `,
it answers `suite=false`,
and every job is skipped, All tests passed and Release too.
`make release-alpha` pushes `main` and the tag atomically,
and `main` must equal `origin/main` before,
so the tag's run tests that very commit;
running the suite on `main` as well tested it twice
and made a `devel` build under the release's own version (owner, 2026-11-10,
as file_checksum_search keeps one run per release).
The nightly build still finds the commit, through its tag's run.
A change to documents no test reads answers `code=false`:
every job but Guidelines is skipped, All tests passed passes,
and `main` makes no devel build,
its binaries being those of the commit before (owner, 2026-11-11, AP DocOnly).
Those documents are `GUIDELINES/wip/**`, `GUIDELINES/project/**`,
the generated `AGENTS.md` and `GUIDELINES/README.md`,
`CHANGELOG.md`, `CHANGELOG.d/**` and `LICENSE`,
declared as `[ci] untested` in `GUIDELINES/config.ini` and rendered into the contract;
`GUIDELINES/shared/tools/fingerprint.sh` refuses a list that names a shipped path
or config.ini itself.
What to test asks it whether the code is the same:
`fingerprint.sh same` of a pull request's base and its merge commit,
or of the commit `main` was at (`github.event.before`) and the one pushed;
a tag is always tested whole,
and the tool's exit 2, an unknown commit included, counts as code.
A release commit is recognised by its role,
as `commit_title.py` reads the summary,
so that `chore(release:<component>)` is one too.

A green run is carried to later commits with the same code (owner, 2026-11-11,
AP TestedTree).
All tests passed, once it has, writes a commit status, context `Tested`,
on the commit the run tested:
a pull request's head, or the commit pushed to `main`.
It holds `fingerprint.sh fingerprint` of the tree that ran,
a pull request's merge commit in its run,
and whether Screenshots, LAN and Fuzz passed for that code
(`fp1:<hex> ui=… lan=… fuzz=…`).
What to test reads the status on the commit the run before tested:
for a pull request its head's first parent,
for a merge into `main` its second parent, the pull request's head.
Where it is green, written by `github-actions[bot]` and its fingerprint the commit's own,
it answers `tested=true`:
the jobs that passed for that code are skipped,
Fuzz, which a pull request never runs,
and Screenshots and LAN run where they had not passed,
and Guidelines, All tests passed, Build and Release run as always;
the run's own Tested status then carries the record on.
So a branch's retirement after its green run tests nothing again,
nor does its merge into `main`, where `main` moved by documents alone meanwhile;
where `main` moved by code, the merge's fingerprint is new and everything runs.
A fork's pull request has a read-only token and writes no status,
and the step's failure fails nothing.
The same code is not the same environment: a Go patch or a runner image
can change in between, as between two runs of one commit,
and a flaky test that passed is not retried.
Other paths stay out of the list because something tests them:
`docs/**` (`TestOpenAPICoversRoutes` reads the OpenAPI file,
`docs/fragments/` feeds README's generated parts, Site builds the docs),
`README.md` and `CONTRIBUTING.md` (generated parts, Site, identifiers),
`GUIDELINES/shared` (Site renders with its `md_html.py`,
the rolling notes use its `changelog.sh`),
`.gitattributes` and `.gitignore` (line endings, what `make` counts as dirty),
and `tools/**`.
The Guidelines job runs `tools/identifiers` itself,
since that check reads the project's documents and changelog too.
The nightly build may take a commit of documents alone;
its code is that of the commit before it, which was tested.
Only an answer of `false` skips a job:
a gate that fails leaves its answers empty, so every job runs,
and All tests passed fails on the gate's result,
since the required checks, which a skipped job passes, do not include it.
A pipe into `grep` in a step reads to the end (`grep … >/dev/null`,
not `grep -q`):
under `pipefail` a `grep -q` that exits at its match kills a writer still writing,
which failed the macOS job with exit code 141 on 2026-11-10
(`massimo tls show | grep -q`).
Runs share a concurrency group per ref:
a new push to a pull request cancels the run it supersedes,
while a run of `main` is never cancelled,
so every commit of `main` keeps its result.

The armhf package ships the armv6 binary, which `make cross` only builds:
Test (armv6, QEMU) runs every package's tests on it,
as `qemu-arm -cpu arm1176` (the Raspberry Pi 1's core) runs each test binary,
without binfmt and without a new action, Ubuntu's `qemu-user` from apt.
`-short` keeps the emulation to a few minutes:
`internal/server` takes the longest, about two minutes on a PC.
The emulation's slowness brings out tests that rely on timing,
as it did the store's prune racing a test's fill;
such a test is fixed, not skipped under QEMU.

The web UI smoke test (`make ui-test`) and the screenshot set
(`make ui-screenshots`) run in the Screenshots job,
on the Chromium of Microsoft's Playwright image (section 3).
The slideshow (`tools/ui-test/slideshow.sh`) is built where the set is taken
and committed with it, not in CI,
which would need ImageMagick from apt.
The UI changes are still verified with the smoke test on the developer's
machine before a pull request (the contract's test gate),
and the set committed under `docs/screenshots/` is the one taken there:
the runner's fonts give other pixels,
so the artifact is for looking at,
and nothing compares it with the committed set.

The Screenshots job is the run's longest.
The suite's 484 runs (121 tests,
each at 1100 and 390 px, in English and German)
run in four pytest-xdist workers, one per CPU of the runner,
each against a server set of its own (`tools/ui-test/README.md`):
under 6 minutes on four CPUs here, against 9 for half the cases in one process,
which a pull request ran before;
the screenshot set takes half a minute.
The first pass records no Playwright trace,
whose DOM snapshots made every step slower;
the failed tests run once more with it.
A pull request runs the job only when it changes `internal/server/ui/`,
`tools/ui-test/` or `.github/workflows/ci.yml`,
the paths the contract asks `make ui-test` for and the job's own definition,
and then on all four cases,
so that a case it would have left out does not fail first on `main`.
The job What to test answers that from the pull request's merge commit,
whose first parent is the base branch.
The condition sits on the job, not on the workflow's trigger:
"Screenshots" is a required check on `main`,
and a job skipped by its `if:` counts as passed,
while a workflow that a path filter never starts leaves the check waiting.
Only an answer of `false` skips the job,
so a What to test job that fails runs it rather than letting the pull request
through untested.
A push to `main` runs the job whatever it changes,
so a change elsewhere that breaks the page shows on the merge commit's run.

The LAN job runs `make lan-test` (`tools/lan-test/README.md`):
four hosts in Docker on a bridge network of their own,
a fake Cube with a MAC of eQ-3's prefix,
a searcher, a second `serve` with no Cube that searches through its API,
a box running `serve` beside avahi-daemon, and a client with avahi of its own.
With no proxy on the LAN, the searcher must find the Cube,
lose it at the first search with the Cube's cable pulled (its link down),
and find it again with the cable back;
then, with the box up, list the Cube's serial from the Cube and from the proxy.
It is discovery's only test with each host at an address of its own,
on the real ports;
the Go tests ask a fake Cube on the loopback.
Then `serve` must share UDP 5353 with avahi,
`avahi-browse` resolve it on both hosts,
`massimo discover` list it and mark the proxy but not the Cube,
and its goodbye take it out of the client's avahi.
It is the announcement's only test against another implementation on Linux;
the macOS and Windows jobs add Apple's mDNSResponder and Windows' mDNS,
with a second `serve` on every address in a step of their own,
the macOS one as a launchd daemon (section 7).
A pull request runs the job only when it changes `internal/announce/`,
`internal/serve/`, `internal/proxy/`, `internal/cubelan/`,
`cmd/massimo/discover.go`, `cmd/massimo/serve.go`, `tools/lan-test/`
or `.github/workflows/ci.yml`,
as the job What to test answers, and a push to `main` always,
as the Screenshots job does.

Hardware is never touched by CI;
the Windows and macOS jobs run against the fake Cube.

## 3. The Commands Are the Project's Commands

CI runs the commands of the contract's facts table (`GUIDELINES/project/_CONTRACT.md`):
`gofmt -l .`, `go vet ./...` for linux, windows and darwin,
`go test ./...` and `make cross`.
`go test` includes `tools/identifiers`,
which fails on a serial, a MAC or a radio address in the tree that is not a
placeholder (the contract, "Secrets and identities").
Where CI differs, and why:

- `make race` is the contract's recommended `-race`, in a Race job of its own
  beside Vet and test, so that neither waits for the other:
  about 80 s on four cores,
  the server's and the store's packages taking the longest.
- `make fuzz` runs on a push to `main` only.
  A pull request runs the fuzz targets' seeds in `go test`, the crashes found
  before among them;
  fuzzing finds what it finds by chance, so a pull request's result would
  depend on its luck, and `main` is where a new fault is worth the minutes.
  A crash fails the job and keeps its input as the artifact `fuzz-crash`,
  which goes into the package's `testdata/fuzz/` with its fix.
  A target whose only failure is `context deadline exceeded`,
  with no input written, passes with a warning:
  Go 1.26's fuzzing coordinator reports its own `-fuzztime` deadline that way
  now and then (golang/go#75804),
  which failed the job twice on 2026-11-09, on FuzzLine and on FuzzConfig.
  `tools/fuzz/run.sh` lets it pass only on a toolchain whose `internal/fuzz`
  lacks Go 1.27's fix,
  which it reads from the toolchain's own source rather than its version,
  so the exception ends by itself once CI's Go carries the fix.

- `govulncheck ./...` runs in CI only, per platform with code of its own,
  because a known vulnerability is a reason to fail a build but not a tool
  every developer installs.
- The Screenshots job runs what `make ui-test` and `make ui-screenshots` run,
  `tools/ui-test/run.sh` and `run.sh screenshots`,
  in the container `mcr.microsoft.com/playwright/python:v<version>-noble`,
  whose Python takes the place of the venv (`UI_TEST_PYTHON`)
  and which has git and curl but no `make`;
  `actions/setup-go` installs Go inside it.
  The job sets `GOFLAGS=-buildvcs=false`,
  because git there refuses the checkout,
  which belongs to another user than the container's root,
  and the binary under test needs no version stamp.
  The image holds the browsers and their system libraries,
  so the job installs nothing through apt:
  `playwright install --with-deps chromium` on the runner took 60 s on a good run
  and 672 s on a slow one,
  the ImageMagick install for the slideshow up to 246 s,
  the job 2 to 24 minutes.
  The job pins `playwright==<version>` in its pip install,
  on top of `tools/ui-test/requirements.txt`, which keeps its minimum versions,
  because the browsers in the image fit that Playwright version only:
  the image's tag and the pin move together.
  The suite's step sets `UI_TEST_REQUIRED=1`,
  so that a missing Playwright, pytest, pytest-playwright or pytest-xdist
  fails the job instead of skipping the tests or running them in one process,
  as `run.sh` does on a developer's machine.
  When a step fails, the job uploads `tools/ui-test/results/`,
  where pytest-playwright keeps a failed test's screenshot and video,
  and under `retry/` the trace of its run once more,
  so that a failure on the runner can be replayed rather than retold.
  The smoke test runs in the same job, before the captures,
  because Playwright and Chromium are there anyway;
  it is not part of the test gate's job,
  which would then need the image as well.
- The Windows and macOS jobs install the real service on the runner,
  which a developer cannot do on another operating system.
  They use the alpha channel's newest release for the update by hand and a
  stubbed release, signed with a test key, for the rollback.
- The Windows job runs discovery's tests as well,
  `internal/cubelan` and the server's searches:
  Windows binds UDP ports otherwise than Linux,
  and lets a wildcard bind a port that a socket holds on one address,
  which then gets the answers sent to that address;
  a test of `internal/cubelan` that passed on Linux failed there.
- The Windows and macOS jobs run `internal/safefile`'s tests as well:
  what a replaced file keeps is per platform,
  its DACL and the retried rename on Windows,
  its extended attributes on macOS.
- The Debian package job runs `make packages` and `make deb-test` as a developer does,
  on two Debian releases; `packaging/deb/test.sh` is the one script for both.

## 4. Guideline Gates

The Guidelines job runs `GUIDELINES/shared/tools/check`,
which runs every check the guidelines have and names them with `check --list`:
the generated documents are current (`sync-docs.sh --check`),
the declared languages match the tree (`detect-languages.sh --check`),
every document version has its revision record (`check-versions.sh`),
the heading numbers, the line breaks (`reflow.sh --check`),
`CHANGELOG.md`'s form and its released sections against their tags (`changelog.sh check`),
the vendored files, and the commit messages.

The job checks out with the submodule and the full history with the tags,
because the changelog check compares the released sections with the tag messages.

The commit-message check reads a range,
and the job gives it one on pull requests only:
`base..head` of the pull request, not `..HEAD`,
because on a pull request HEAD is the merge commit GitHub makes for the run,
whose message is not the project's to write.
On a push to `main` the check is given an empty range, `HEAD..HEAD`,
and reads nothing:
the merge commits `main` receives from GitHub do not follow COMMIT.md,
and the pull request's own commits were checked when it was open.
The history before the guidelines were adopted (2026-11-06) does not
follow COMMIT.md either and is not rewritten.
A pull request of Dependabot's is given the empty range too,
since its messages cannot take the project's form (section 8);
every other check runs on it.
The range is named rather than left out:
without one the check reads every commit not on the remote's default branch
wherever the checkout has an `origin/HEAD`,
and on 2026-11-08 it read a Dependabot pull request's that way.

## 5. Secrets and Variables

| Name | Set where | Used by |
|------|-----------|---------|
| `MASSIMO_RELEASE_KEY` | the `signing` environment on GitHub, usable by `v*` tags only, each run approved by the owner | release.yml, to sign the release manifest; its public half is `internal/update/release-key.pem` |
| `MASSIMO_ROLLING_KEY` | the `signing-rolling` environment, usable by `main` only | release.yml's `rolling`, to sign the manifest of a `devel` or `nightly` build; its public half is `internal/update/rolling-key.pem`, which boxes accept for those two flavours alone |
| `ROLLING_TAG_DEPLOY_KEY` | the `signing-rolling` environment; its public half a deploy key with write access, "rolling tags (release.yml)" | release.yml's `rolling`, to move `release/devel` and `release/nightly` to the new build (section 6); without it the token tries |
| `RELEASE_BRANCH_DEPLOY_KEY` | the `signing` environment; its public half a deploy key with write access, "release branches (release.yml)" | release.yml's `sign`, to fast-forward `release/<flavour>` to the published release (section 6); without it the branch stays |
| `HOMEBREW_TAP_DEPLOY_KEY` | a repository secret, once the owner has made the tap and the key (packaging/README.md, "Homebrew") | publish.yml, handed over by release.yml's `homebrew` job, to push the formula to `metaworx/homebrew-massimo`; without it the job is skipped |
| `MASSIMO_APT_KEY`, `MASSIMO_APT_KEY_PASSPHRASE` | the `signing` environment, once the owner has made the key (packaging/README.md, "The apt repository") | release.yml's `apt-build` job, to sign the apt suites; the site's `massimo.asc` is its public half; without them the apt jobs are skipped |
| `GITHUB_TOKEN` | provided by GitHub | the release jobs to publish, the Windows and macOS jobs as `MASSIMO_GITHUB_TOKEN`, for the release checks against the API's rate limit and the release files through the API |

Never echo any slice of a secret to the log.

## 6. Releases

`make release-alpha` on `main` (checked out, clean,
equal to `origin/main`) cuts `CHANGELOG.md`'s `[Unreleased]` into the section of
`vX.Y.Z-alpha.N` (`BUMP=minor` for a feature,
nothing for a fix) with `changelog.sh cut --commit --tag create`:
the `chore(release):` commit on `main`,
and the signed tag at it with the section as its message.
`main` and the tag are pushed atomically;
the tag's push runs ci.yml, whose last job calls release.yml.
Once the release is published, `sign` pushes the tag's commit to
`release/<flavour>` (`release/alpha`) with the deploy key
`RELEASE_BRANCH_DEPLOY_KEY`:
GitHub refuses `GITHUB_TOKEN` a branch update that changes `.github/workflows/`,
which a release's commits often do.
The ruleset on `release/*` lets only a fast-forward through and no deletion;
the first release makes the branch,
and the old `alpha` goes once `release/alpha` has had its first release.
A tag of the same name is refused, since `release/devel` and
`release/nightly` are tags (section 2).
So a released section and its tag agree from the start, which is what
`changelog.sh check` holds them to in the guidelines job.
`DRY=1` runs every step as a dry run.
The flavour of a release (`internal/update`) is the pre-release identifier
of the tag's name (`v0.7.0-alpha.1` is `alpha`), `stable` for a tag without one;
a box on a channel takes its own flavour's releases and every more stable one's.
The `channel:` line of a tag message and the branch holding the commit,
which older tags fell back to, no longer count;
a tag whose commit is not on `main` is refused.
The GitHub release is created with the tag's commit as its target;
a box reads the flavour from the tag's name to leave a release its channel
does not take unfetched (`internal/update`, `hintedFlavour`);
the name is not signed, the manifest decides,
and `sign` refuses a manifest whose flavour is not the tag's.
The GitHub release's notes are the body of the tag's message,
that section, then a sentence naming the flavour and how to install and verify;
a tag with no body (an old `channel:` line does not count) gets the sentence alone.
A tag whose commit `git describe` does not name is refused.
`build`, and `apt-build`, which holds the apt key, restore no Actions cache
(`setup-go`'s `cache: false`):
a cache another run wrote would otherwise reach the release's binaries.
CodeQL's alert 6, cache poisoning by the nightly build's checkout of the commit
`meta` picked, was dismissed as a false positive:
that commit is always one of `main`'s first parents;
`meta` moved to build.yml since, and an alert raised there gets the same answer.
`make release` runs the build through `tee`,
four targets at a time (`-j4 -Otarget`, `RELEASE_JOBS`),
each one's output kept together,
and writes the build record,
`massimo-build.txt` (`packaging/build-record.sh`):
the runner and its image,
the toolchain and the `go env` values that shape a build,
the command and its `-ldflags`,
which Go leaves out of a binary built with `-trimpath`,
`go version -m` of every binary and of nfpm, built again for it,
then the whole log.
`SHA256SUMS` and the manifest list it, so the release key's or the rolling
key's signature covers it;
`build` holds no secret, so the log holds none.
`build` then attests the provenance of every file it made,
the manifest and the record included (`actions/attest-build-provenance`,
with `id-token` and `attestations`, which ci.yml's `release` job grants):
a statement signed through Sigstore with the job's OIDC token,
not with the project's keys,
naming the repository, the workflow, the commit and the runner;
`gh attestation verify <file> -R metaworx/massimo` checks one.
Only the manifest's signature is not attested,
since `sign` and `rolling` make it.
It runs in `build`, not beside the keys, which run nothing but openssl and gh.

A push to `main` whose suite passed becomes the `devel` build:
the same `make release`, as the version `git describe` calls the commit
(`v0.12.0-alpha.1-37-gabc1234`), with the flavour `devel`,
signed by `rolling` in the `signing-rolling` environment with
`MASSIMO_ROLLING_KEY`, whose public half boxes accept for `devel` and
`nightly` alone, so that a key `main` can reach signs nothing an `alpha`
box takes.
It is one GitHub release, `release/devel`, a pre-release replaced in place:
the binaries, packages and sums uploaded over the old ones,
then the manifest and its signature,
so that a box reading the new manifest finds its files already there,
the tag pushed with the deploy key `ROLLING_TAG_DEPLOY_KEY`,
the notes rewritten and the files the new build lacks deleted.
`GITHUB_TOKEN` moved the tag once and was refused the next time
(`Resource not accessible by integration`, 2026-11-10),
after other merges had changed `main`'s workflow files since the commit;
GitHub keeps the token from some reference updates that touch workflow files,
and a deploy key is not held to that.
Editing a release notifies no watcher, as creating one at every merge would.
Runs of `main` are never cancelled, so an older one can end last:
`meta` and `rolling` each publish only a commit after the one the release names,
and `rolling` runs one at a time per flavour.
GitHub keeps one waiting job per concurrency group,
so a newer push cancels an older push's waiting `rolling` job,
and that run reads "cancelled" though its tests passed;
the nightly build goes by the job "All tests passed", not by the run,
and dropping the rule could interleave two builds' uploads.
`rolling` also checks the flavour against the run's event,
`devel` only for a push to `main`.
The notes name the build, the commits since the previous one,
and `CHANGELOG.md`'s `[Unreleased]` as `changelog.sh preview` writes it,
once the guidelines carry that command (v4.54.0).
Once a day nightly.yml runs build.yml and release.yml for the `nightly` build,
`release/nightly`, replaced in place as `devel` is.
It runs no tests: it walks `main`'s first parents from the newest,
down to the commit the nightly build carries,
and takes the first whose run of ci.yml (its push, or its tag's)
had "All tests passed" succeed,
so that a red newest commit does not cost a night whose second-last was green
(the owner's rule); `meta` reads the runs with `actions: read`,
which the `build` jobs of ci.yml and nightly.yml grant.
`rolling` takes `nightly` only from the schedule or by hand on `main`.
The rolling builds' tags are no versions,
so every `git describe` asks for the version tags alone (`--match 'v[0-9]*'`):
the Makefile's `VERSION` and `FLAVOUR`, `make release-alpha`'s check,
and `meta`'s.
Without it, on 2026-11-10 every commit after the first `release/devel`
described as `release/devel-1-g…`, which `make packages` could not name a package by.
A rolling tag moves, so nothing fetches it unforced:
`git fetch --tags` refuses to move a checkout's older copy of one
("would clobber existing tag", 2026-11-10).
`make release-alpha` fetches `main` and the version tags alone (`refs/tags/v*`),
and `git fetch --force origin 'refs/tags/release/*:refs/tags/release/*'`
brings a checkout's own copies up to date.
`packaging/apt/publish.sh` leaves the rolling releases out:
their packages' versions do not sort yet (AptUpdate's DebVersion).
Every release below 1.0.0 is a pre-release on GitHub.
The boxes on the alpha channel (the Raspberry Pi) take a release through
`massimo update` or the web UI;
nothing in CI touches them.
The Homebrew formula is pushed from the same run:
release.yml's `homebrew` job calls publish.yml once `sign` has published the release.
publish.yml does not listen for `release: published` itself,
because the release is created with the run's own `GITHUB_TOKEN`,
and GitHub starts no workflow for an event that token causes
(only `workflow_dispatch` and `repository_dispatch` are exempt):
until this was moved, the tap was never updated and nothing said so.
A release published before the tap existed gets its formula by hand,
`gh workflow run publish.yml -f tag=<tag>`,
which renders the formula from the tag's own template
and changes nothing when the tap holds it already.
A release also carries a Debian package per architecture,
and once the apt key is set the `apt-build` job rebuilds the apt repository from
the newest ten releases of every channel (`packaging/apt/publish.sh`,
one suite per channel) and hands it to the job `site` as the artifact `apt`.
`site` (site.yml, `packaging/site/build.sh`; packaging/README.md, "The site")
runs at every run of release.yml, after `apt-build` and the rolling build:
it puts the pages of `packaging/site/` at the root and the docs of every
channel under `docs/`, and deploys it to the project's GitHub Pages,
<https://massimo.metaworx.ch> by the custom domain of the repository's Pages settings
(a workflow's deployment needs no `CNAME` file),
one deploy at a time (`concurrency: pages`), none cancelled.
The Pages source must be set to GitHub Actions,
and the `github-pages` environment must allow the tags `v*`:
GitHub makes it allowing the default branch only,
and a release's deploy runs for its tag;
a box following a suite takes the release through `apt upgrade`.
`apt-build` also attaches the signed index to the release,
`massimo-apt-index.tar.gz` with `pool.tsv`
(packaging/README.md, "The apt repository"),
so that a deploy of the site from `main` can serve the repository
without the apt key, which stays with the tags:
a run that releases nothing takes the newest release's index,
downloads the pool by `pool.tsv` and checks every package by its sum,
and deploys nothing where no release carries one yet,
since a site without `apt/` would take the repository offline.

## 7. Runner Constraints

- The Windows job's update by hand needs the alpha channel to hold a
  release the runner can download;
  the stubbed release covers the rollback without the network.
- The service manager on Windows gives a binary that never connects 30 s
  (error 1053) before the job's own two-minute health timeout matters;
  a stand-in that exits at once fails in two seconds.
- The Windows job's stage 2 serves its releases locally,
  and the helper then stops the service within about 100 ms of the API's 202;
  so `post_update` sends the second POST, which must be refused,
  on the first's connection from the same curl (`--next`):
  a second curl process started too late once.
  The server's answer to it is less work than the helper needs before the stop,
  so the answer comes first at any runner speed.
  It can meet the helper reading the request,
  whose open handle refuses the request's rename on Windows for a few milliseconds:
  the rename is retried (`internal/update`, `renameRetrying`),
  and the answer is 409.
- The macOS job runs the LaunchDaemons as root on the runner;
  `launchctl` needs the system domain there, not a user's.
- The macOS job's update from the API installs the newest alpha release,
  which knows no `-tls` until a release with it is cut:
  that server serves plain HTTP, the helper's `https://` health check fails,
  and the job takes its rollback branch, which it accepts.
  Once such a release is out the job takes the other branch by itself.
- `ubuntu-latest` moves to a newer Ubuntu on GitHub's schedule;
  the annotation on every run says when.
- The LAN job needs the runner's Docker with Compose v2,
  and builds the hosts' image, with avahi from Debian's packages, at each run.
- The macOS and Windows jobs' mDNS steps need the runner's own multicast
  to come back to it (the second `serve`'s announcements reach the system's
  responder on the same machine); a runner that filters it fails the step.
- On the macOS runner, macOS's local network privacy refuses the runner's own
  processes every multicast send ("no route to host"),
  and spares launchd daemons:
  the macOS step runs its second `serve` as a launchd daemon in the system
  domain, as `massimo install` runs the service,
  asks `dns-sd`, which talks to mDNSResponder alone,
  and does not ask `massimo discover`, a process of the runner's.
  `dns-sd` is read through a pseudo-terminal of Python's:
  under `script`, what it printed was lost.
- The Debian package job's `deb-test` container has no systemd:
  the unit is enabled through deb-systemd-helper's symlinks and nothing starts.
  `deb-systemd-test` runs the cases that need one in privileged containers,
  systemd their first process, one per case,
  so that no case sees an account another left behind.
  Debian's images ship a `policy-rc.d` that refuses every start and stop,
  which the test's image takes out, as a real box has none;
  `--network none` keeps a running `serve` off the LAN and GitHub.
  v0.12.0-alpha.1's package, the last whose account was `massimo`,
  comes from its release through `gh` with the job's token.
  A build from `main` sorts before it until the next release,
  so its upgrade is a downgrade to dpkg, which runs the same scripts for it.

## 8. Dependabot and the Repository's Settings

`.github/dependabot.yml` has Dependabot propose,
once a month and in one pull request each,
the updates of the Go modules the code imports directly and of the actions
the workflows use.
Its pull requests differ from the project's in two ways, both on purpose:

- Their commit messages, `build(deps): bump …`, carry no `Key changes:` list
  and no `Assisted-by` trailer,
  so the Guidelines job does not read them (section 4).
- They carry no `CHANGELOG.md` bullet,
  though `go.mod` and `.github/**` are shipped paths.
  Whoever merges one adds its updates to a bullet
  `- Dependencies updated: …` under `[Unreleased]`, Changed,
  in a commit of the project's form before the next release is cut,
  so that every release names them.

Every action is pinned to the commit of its release,
with the version as a comment (`actions/checkout@<sha> # v5.1.0`),
so that a moved tag cannot change what runs;
Dependabot updates the pin and the comment together.
A new action is pinned the same way,
its tag's commit from `gh api repos/<owner>/<repo>/git/ref/tags/<tag>`
(an annotated tag points at a tag object; its `object.sha` is the commit).
`github.com/bytemare/ksf` is indirect and replaced by `third_party/bytemare-ksf`,
so Dependabot leaves it alone until a release has the fix.

The repository's own security settings are the owner's,
in its Settings, and need no file here:
code scanning by CodeQL's default setup (Go, JavaScript and TypeScript,
Python, the workflows) on every push to `main`, every pull request and weekly,
secret scanning with push protection,
and Dependabot alerts and security updates.
They are on since 2026-11-08.

A code scanning alert is closed in one of two ways:

- **Fixed**, in a change of the project's form;
  the CodeQL check of its pull request shows the alert fixed there,
  and the alert closes when the change reaches `main`.
- **Dismissed** as "won't fix", with its reason as the comment,
  where the code means what it does and its own comment says why:
  `massimo update`'s health check skips verifying the certificate
  of the box's own address, which a connection to it never leaves.
  A false positive the code cannot be made plainer for is dismissed the same way,
  as "false positive".

## 9. Document Governance

- This document follows the shared governance rules in `GUIDELINES/shared/GOVERNANCE.md`.
