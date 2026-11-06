# Massimo — CI Conventions (v1.3.1)

What CI does in this project, for whoever has to change it.
`/AGENTS.md` names this file as the canonical owner of CI conventions,
so a rule that lives only in a workflow YAML — a version pin,
an ordering constraint, a reason a step exists — belongs here as well,
in words.
The pipeline is the source of truth for *what* runs;
this document is the source of truth for *why*.

## Contents

<!-- BEGIN GENERATED CONTENTS - do not edit; run "tools/sync-dev.sh" -->
1. Where CI Is Defined
2. What Runs, and When
3. The Commands Are the Project's Commands
4. Guideline Gates
5. Secrets and Variables
6. Releases
7. Runner Constraints
8. Document Governance
<!-- END GENERATED CONTENTS -->

## 1. Where CI Is Defined

| Platform | Files |
|----------|-------|
| GitHub Actions | `.github/workflows/ci.yml` (a push to `main`, a pull request), `.github/workflows/release.yml` (tags `v*`) |

There is no second pipeline.

## 2. What Runs, and When

| Trigger | Jobs |
|---------|------|
| push to `main`, pull request (each push to its branch) | Vet and test (gofmt, go vet per platform, go test, `make cross`); govulncheck for linux and windows; Windows service (install, run, update from the API and by hand, roll back, remove, on a Windows runner); macOS LaunchDaemon (the same on a macOS runner); Screenshots (the web UI smoke test, then the screenshot set, uploaded as the artifact `ui-screenshots`, the PNGs only); Guidelines (section 4) |
| push to a release branch (`alpha`, later `beta` and a stable one) | nothing: it is a fast-forward of a tested commit; the tag runs release.yml |
| tag `v*` | release.yml: build every target without cgo, sign the manifest with the release key, verify the signature against the public key compiled into `massimo`, publish the GitHub release |

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
Hardware is never touched by CI;
the Windows and macOS jobs run against the fake Cube.

## 3. The Commands Are the Project's Commands

CI runs the commands of the contract's facts table (`GUIDELINES/project/_CONTRACT.md`):
`gofmt -l .`, `go vet ./...` for linux, windows and darwin,
`go test ./...` and `make cross`.
Where CI differs, and why:

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
  The smoke test runs in the same job, before the captures,
  because Playwright and Chromium are there anyway;
  it is not part of the test gate's job,
  which would then need the image as well.
- The Windows and macOS jobs install the real service on the runner,
  which a developer cannot do on another operating system.
  They use the alpha channel's newest release for the update by hand and a
  stubbed release, signed with a test key, for the rollback.

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
On a push to `main` the check runs without a range and reads nothing:
the merge commits `main` receives from GitHub do not follow COMMIT.md,
and the pull request's own commits were checked when it was open.
The history before the guidelines were adopted (2026-11-06) does not
follow COMMIT.md either and is not rewritten.

## 5. Secrets and Variables

| Name | Set where | Used by |
|------|-----------|---------|
| `MASSIMO_RELEASE_KEY` | the `signing` environment on GitHub, usable by `v*` tags only | release.yml, to sign the release manifest; its public half is `internal/update/release-key.pem` |
| `GITHUB_TOKEN` | provided by GitHub | the release jobs to publish, the Windows and macOS jobs as `MASSIMO_GITHUB_TOKEN` for the release checks against the API's rate limit |

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
The GitHub release's notes are the body of the tag's message,
that section, then a sentence naming the channel and how to install and verify;
a tag with no body (a `channel:` line does not count) gets the sentence alone.
A tag whose commit `git describe` does not name is refused.
Every release below 1.0.0 is a pre-release on GitHub.
The boxes on the alpha channel (the Raspberry Pi) take a release through
`massimo update` or the web UI;
nothing in CI touches them.

## 7. Runner Constraints

- The Windows job's update by hand needs the alpha channel to hold a
  release the runner can download;
  the stubbed release covers the rollback without the network.
- The service manager on Windows gives a binary that never connects 30 s
  (error 1053) before the job's own two-minute health timeout matters;
  a stand-in that exits at once fails in two seconds.
- The macOS job runs the LaunchDaemons as root on the runner;
  `launchctl` needs the system domain there, not a user's.
- `ubuntu-latest` moves to a newer Ubuntu on GitHub's schedule;
  the annotation on every run says when.

## 8. Document Governance

- This document follows the shared governance rules in `GUIDELINES/shared/GOVERNANCE.md`.
