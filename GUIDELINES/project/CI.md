# Massimo — CI Conventions (v1.4.0)

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
| GitHub Actions | `.github/workflows/ci.yml` (every push and pull request), `.github/workflows/release.yml` (tags `v*`), `.github/workflows/publish.yml` (a published release) |

There is no other platform.

## 2. What Runs, and When

| Trigger | Jobs |
|---------|------|
| push to any branch, pull request | Vet and test (gofmt, go vet per platform, go test, `make cross`); govulncheck for linux and windows; Debian package (`make packages`, then the amd64 one installed, checked and purged in a trixie and a bookworm container, and installed again from an apt repository built with a throwaway key; the packages uploaded as the artifact `packages`); Windows service (install, run, update from the API and by hand, roll back, remove, on a Windows runner); macOS LaunchDaemon (the same on a macOS runner, then the Homebrew formula rendered for a tarball of the checkout, built from source, tested and checked for the marker); Screenshots (the web UI smoke test, then the screenshot set and its slideshow, uploaded as the artifact `ui-screenshots`); Guidelines (section 4) |
| release published | publish.yml: the Homebrew formula of the release's channel rendered and pushed to the tap, with the deploy key set |
| tag `v*` | release.yml: build every target without cgo and the Debian packages, sign the manifest with the release key, verify the signature against the public key compiled into `massimo`, publish the GitHub release; then, with the apt key set, rebuild the apt repository from the releases and deploy it to GitHub Pages |

The web UI smoke test (`make ui-test`) and the screenshot set
(`make ui-screenshots`) run in the Screenshots job,
on Chromium that Playwright installs on the runner.
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
- The Screenshots job runs `make ui-test` and `make ui-screenshots` as a developer does,
  with Playwright from `tools/ui-test/requirements.txt` in the runner's Python
  instead of the venv (`UI_TEST_PYTHON`).
  The smoke test runs in the same job, before the captures,
  because Playwright and Chromium are installed there anyway;
  it is not a job of its own, and not part of the test gate's job,
  because installing Chromium with its system libraries takes longer than the
  rest of that job.
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
On a push to a branch the check runs without a range and reads nothing:
the merge commits `main` receives from GitHub do not follow COMMIT.md,
and the pull request's own commits were checked when it was open.
The history before the guidelines were adopted (2026-11-06) does not
follow COMMIT.md either and is not rewritten.

## 5. Secrets and Variables

| Name | Set where | Used by |
|------|-----------|---------|
| `MASSIMO_RELEASE_KEY` | the `signing` environment on GitHub, usable by `v*` tags only | release.yml, to sign the release manifest; its public half is `internal/update/release-key.pem` |
| `HOMEBREW_TAP_DEPLOY_KEY` | a repository secret, once the owner has made the tap and the key (packaging/README.md, "Homebrew") | publish.yml, to push the formula to `metaworx/homebrew-massimo`; without it the job is skipped |
| `MASSIMO_APT_KEY`, `MASSIMO_APT_KEY_PASSPHRASE` | the `signing` environment, once the owner has made the key (packaging/README.md, "The apt repository") | release.yml's `apt-build` job, to sign the apt suites; the site's `massimo.asc` is its public half; without them the apt jobs are skipped |
| `GITHUB_TOKEN` | provided by GitHub | the release jobs to publish, the Windows and macOS jobs as `MASSIMO_GITHUB_TOKEN` for the release checks against the API's rate limit |

Never echo any slice of a secret to the log.

## 6. Releases

`make release-alpha` on `main` tags its head `vX.Y.Z-alpha.N` (`BUMP=minor` for a feature,
nothing for a fix) and fast-forwards the branch `alpha` to it,
both pushed atomically; the tag's push starts release.yml.
The update channel of a release is the pre-release identifier of the tag's name
(`v0.7.0-alpha.1` is on `alpha`),
then the `channel:` line `make release-alpha` writes into the tag message,
then the one branch holding the commit;
the name comes first so that a tag message rewritten with its changelog section
(`changelog.sh tag`) keeps its channel.
A tag whose commit `git describe` does not name is refused.
Every release below 1.0.0 is a pre-release on GitHub.
The boxes on the alpha channel (the Raspberry Pi) take a release through
`massimo update` or the web UI;
nothing in CI touches them.
A release also carries a Debian package per architecture,
and once the apt key is set the `apt-build` job rebuilds the apt repository from
the newest ten releases of every channel (`packaging/apt/publish.sh`,
one suite per channel) and `apt-deploy` puts it on the project's GitHub Pages,
whose source must be set to GitHub Actions;
a box following a suite takes the release through `apt upgrade`.

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
- The Debian package job's container has no systemd:
  the unit is enabled through deb-systemd-helper's symlinks and nothing starts,
  so the start on a real box is the owner's check.

## 8. Document Governance

- This document follows the shared governance rules in `GUIDELINES/shared/GOVERNANCE.md`.
