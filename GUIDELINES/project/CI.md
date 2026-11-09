# Massimo — CI Conventions (v1.13.4)

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
| GitHub Actions | `.github/workflows/ci.yml` (a push to `main`, a pull request), `.github/workflows/release.yml` (tags `v*`), `.github/workflows/publish.yml` (called by release.yml, or run by hand for a tag) |

There is no other platform.

## 2. What Runs, and When

| Trigger | Jobs |
|---------|------|
| push to `main`, pull request (each push to its branch) | Vet and test (gofmt, go vet per platform, go test, `make cross`); Race (`make race`, the race detector on `internal/server`, `internal/store`, `internal/cubelan`, `internal/proxy`, `internal/serve`, `internal/announce` and `internal/notify`); Fuzz on a push to `main` only (`make fuzz`, every fuzz target for 10 s, a crash's input kept as the artifact `fuzz-crash`); govulncheck for linux and windows; Debian package (`make packages`, then the amd64 one installed, checked and purged in a trixie and a bookworm container, and installed again from an apt repository built with a throwaway key; the packages uploaded as the artifact `packages`); Windows service (install, run, update from the API and by hand, roll back, remove, on a Windows runner); macOS LaunchDaemon (the same on a macOS runner, with a self-signed certificate: the service installed with `-tls self-signed`, the health check and the API over `https://`; then the Homebrew formula rendered for a tarball of the checkout, built from source, tested and checked for the marker); Screenshots (the web UI's pytest suite, then the screenshot set, uploaded as the artifact `ui-screenshots`, the PNGs only, and on failure the failed tests' traces, screenshots and videos as `ui-test-results`; on a pull request only when it changes the UI, its suite or `ci.yml`, as the job Changed paths answers, and on two of the suite's four cases); Guidelines (section 4) |
| push to a release branch (`alpha`, later `beta` and a stable one) | nothing: it is a fast-forward of a tested commit; the tag runs release.yml |
| tag `v*` | release.yml: build every target without cgo and the Debian packages, sign the manifest with the release key, verify the signature against the public key compiled into `massimo`, publish the GitHub release; then, with the deploy key set, the Homebrew formula of the release's channel rendered and pushed to the tap (publish.yml, section 6), and, with the apt key set, the apt repository rebuilt from the releases and deployed to GitHub Pages |
| by hand (`gh workflow run publish.yml -f tag=<tag>`) | publish.yml: the formula of a release already out, such as the one published before the tap existed |

Each commit runs the workflow once:
a push to a pull request's branch runs it as the pull request's,
not a second time as a push,
and the commits `main` receives run it as pushes to `main`.
The release branches receive only commits already on `main`,
so a push to one runs nothing.
Runs share a concurrency group per ref:
a new push to a pull request cancels the run it supersedes,
while a run of `main` is never cancelled,
so every commit of `main` keeps its result.

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

The Screenshots job is the run's longest, about 7½ minutes,
6½ of them for the suite's 204 cases (51 tests,
each at 1100 and 390 px, in English and German);
the screenshot set takes half a minute.
A pull request therefore runs it only when it changes `internal/server/ui/`,
`tools/ui-test/` or `.github/workflows/ci.yml`,
the paths the contract asks `make ui-test` for and the job's own definition,
and then on the cases 1100-en and 390-de (`PYTEST_ADDOPTS`),
both widths and both languages, half the cases.
The job Changed paths answers that from the pull request's merge commit,
whose first parent is the base branch.
The condition sits on the job, not on the workflow's trigger:
"Screenshots" is a required check on `main`,
and a job skipped by its `if:` counts as passed,
while a workflow that a path filter never starts leaves the check waiting.
Only an answer of `false` skips the job,
so a Changed paths job that fails runs it rather than letting the pull request
through untested.
A push to `main` runs all four cases,
so a change elsewhere that breaks the page shows on the merge commit's run.

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
  so that a missing Playwright, pytest or pytest-playwright fails the job
  instead of skipping the tests, as `run.sh` does on a developer's machine.
  When a step fails, the job uploads `tools/ui-test/results/`,
  where pytest-playwright keeps a failed test's trace, screenshot and video,
  so that a failure on the runner can be replayed rather than retold.
  The smoke test runs in the same job, before the captures,
  because Playwright and Chromium are there anyway;
  it is not part of the test gate's job,
  which would then need the image as well.
- The Windows and macOS jobs install the real service on the runner,
  which a developer cannot do on another operating system.
  They use the alpha channel's newest release for the update by hand and a
  stubbed release, signed with a test key, for the rollback.
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
| `MASSIMO_RELEASE_KEY` | the `signing` environment on GitHub, usable by `v*` tags only | release.yml, to sign the release manifest; its public half is `internal/update/release-key.pem` |
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
`main`, `alpha` fast-forwarded to that commit and the tag are pushed atomically;
the tag's push starts release.yml.
So a released section and its tag agree from the start, which is what
`changelog.sh check` holds them to in the guidelines job.
`DRY=1` runs every step as a dry run.
The update channel of a release is the pre-release identifier of the tag's name
(`v0.7.0-alpha.1` is on `alpha`),
then a `channel:` line in the tag message (which `make release-alpha` no longer writes:
the name of every tag it makes holds the channel),
then the one branch holding the commit.
The GitHub release is created with the channel as its target,
which a box reads, with the tag's name, to leave another channel's release
unfetched (`internal/update`, `hintedChannel`);
neither is signed, the manifest decides.
The GitHub release's notes are the body of the tag's message,
that section, then a sentence naming the channel and how to install and verify;
a tag with no body (a `channel:` line does not count) gets the sentence alone.
A tag whose commit `git describe` does not name is refused.
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
one suite per channel) and `apt-deploy` puts it on the project's GitHub Pages,
whose source must be set to GitHub Actions,
and whose `github-pages` environment must allow the tags `v*`:
GitHub makes it allowing the default branch only,
and the deploy runs for the release's tag;
a box following a suite takes the release through `apt upgrade`.

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
- The Debian package job's container has no systemd:
  the unit is enabled through deb-systemd-helper's symlinks and nothing starts,
  so the start on a real box is the owner's check.

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
