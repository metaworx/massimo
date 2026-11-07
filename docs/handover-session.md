# Context for a session, 2026-11-07

What a session needs to know to work here, on any machine.
What is to be done is docs/roadmap.md; what was done is CHANGELOG.md,
one section per release, and the commits.
Read README.md, AGENTS.md (the contract and the commit form), docs/roadmap.md,
then this file.

## How the project moves

- Develop on `main` through pull requests the owner merges;
  a branch merges `origin/main` before its pull request (merge, never rebase a stack).
  `alpha` is the release branch and the update channel,
  promoted from `main` with `make release-alpha` (`BUMP=minor` for a feature,
  `DRY=1` to see what it would cut, tag and push),
  which cuts CHANGELOG.md's `[Unreleased]` into the release's section,
  tags with it and pushes `main`, `alpha` and the tag atomically (README.md,
  "Versions").
  The release workflow builds every target, signs the manifest and publishes
  the GitHub pre-release with the Debian packages;
  the apt and Homebrew jobs run once their secrets exist.
- CI runs once per commit (a push to `main` or a pull request, never both;
  a release branch runs nothing, being a fast-forward of a tested commit):
  the Go checks, govulncheck, the service installs on Windows and macOS runners,
  the Debian package in two containers,
  the web UI's pytest suite and the screenshot set in Microsoft's Playwright image,
  and the guidelines' checks (`GUIDELINES/shared/tools/check`);
  GUIDELINES/project/CI.md says what runs and why.
- A ruleset on `main` requires seven checks;
  auto-merge and delete-on-merge are on,
  so `gh pr merge --auto` waits for the checks and the branch goes with the merge;
  the owner's own pushes bypass the ruleset.
  The `gh` token on jaguar has the `workflow` scope;
  without it `gh pr merge` refuses a pull request that changes `.github/workflows/`,
  and the way out is a local signed `merge:` commit pushed to `main`.
- Plans live in GUIDELINES/wip/ and retire only on the owner's `EXEC retire`
  (GUIDELINES/shared/COMMIT.md section 6).
  Open there: AP Tls v1.1 with its analysis and NOTE,
  AP Packaging v1.0 with its NOTE and two analyses.
- A session of its own runs on a worktree under `.claude/worktrees/`;
  its branch and worktree go when the session does.
  The coordinating session keeps this file and docs/roadmap.md.

## Where things run

- **The house Cube**
  (its serial in the Pi's `/etc/massimo/env`) is on the M7 network (`Altocumulus`) at .118 by DHCP, not reserved.
- **The Raspberry Pi 1**
  (Raspberry Pi OS Lite 32-bit) runs `massimo serve` as a systemd service on the
  alpha channel: **.200** is the API and web UI on port 80,
  **.201** the proxy for the house Cube.
  Settings in `/etc/massimo/env`, the store in `/var/lib/massimo/massimo.db`;
  its DHCP address is not reserved either (the M7 range is 20–199).
  It serves plain HTTP until `-tls` is set (README.md, "Server").
- **The spare Cube**
  (its serial in the checkout's `.env`) is on jaguar's network at .46 by DHCP,
  served by a Massimo Windows service on jaguar (.45: proxy on 62910,
  web UI on 8099).
  It is the Cube for hardware tests from jaguar and for the firmware dump.
  Its stored fallback address is .46 static since the netconfig test.
- **jaguar** is the development machine, in WSL 2 in NAT mode:
  no LAN broadcast and no inbound connections,
  so `massimo discover` and a proxy from WSL do not work;
  outbound to the Pi and to the Windows service does.
  Windows binaries built with `GOOS=windows` run from WSL through interop
  and use Windows networking;
  elevation goes through `gsudo` with the owner's UAC confirmation.
  The checkout's `.env` selects the spare Cube through the service's proxy;
  the house Cube is reached with `-sn` and its serial through .201.
- **The Mac** (192.168.1.25) had the LaunchDaemon test install, purged.
- **The Mint box** (192.168.1.56) is available upon request.

## Conventions

- Every command that talks to a Cube goes through the proxy of the `serve` that holds it;
  a direct connection takes the Cube's only slot (`-no-direct` refuses the fallback,
  `-direct` forces it).
  Nothing is sent to a real Cube without the owner's go:
  a radio write costs duty cycle.
  Tests and manual runs use `MASSIMO_ENV_FILE=/dev/null` or the fake Cube
  (`massimo fakecube`), never the checkout's `.env`.
- A client of the API needs a token: `MASSIMO_API_TOKEN` is the owner's key,
  every other user has one of their own (docs/api-v1.md, "Authentication");
  without one the API answers 401.
  A signed-in manager, operator or viewer keeps the web UI's settings and
  colour schemes on the server (docs/store.md section 9 (c)).
- Hardware findings go into docs/portal-protocol.md the day they are made,
  with the date and the Cube firmware;
  the lessons of the hardware days are there ("Lessons from the hardware days")
  and in packaging/README.md ("Learnings").
- The owner's Windows service, the Pi and the Mac are touched only on the
  owner's word in the session.

## The checkout on jaguar

Go 1.26, `gh`, commit and tag signing through the repository's git config
and KeePassXC's SSH agent (run `timeout 60 git commit …`:
a locked agent hangs instead of failing),
the Playwright venv for `make ui-test` in tools/ui-test/.venv.
The conventions are AGENTS.md and documents referenced therein;
docs/review-2026-11-05.md has the coding standard they came from.
The web UI's tests are the pytest suite in tools/ui-test/ (`make ui-test`;
one test with `PYTEST_ADDOPTS='-k "<name> and 390-de"'`,
tools/ui-test/README.md); a UI change brings its test there,
and CI keeps a failed test's trace,
screenshot and video as the artifact `ui-test-results`.
The web UI is built from components.js,
partials under internal/server/ui/partials/ and one translation file per
language (docs/ui.md, "How the page is built").
