# NOTE Packaging v1.0: blocks 1 to 3

Where the run of Blocks 1 to 3 of `AP Packaging v1.0` (2026-11-06,
under `PLAN++`) went differently from the plan,
and what it leaves for the owner.

## Block 1, the deb

1. The marker `/usr/share/massimo/packaged` has three lines, not two:
   the manager, the upgrade command, and the remove command,
   which `massimo install -remove` prints;
   the plan read the remove hint from Go.
2. `lintian` reports two errors, not "nothing worse than a warning":
   `statically-linked-binary` (every Go binary) and `no-changelog`
   (the Debian changelog format; the changelog is `CHANGELOG.md` on GitHub).
   Both are by design and documented in packaging/README.md.
3. The updater's test (`TestUpdateRefusesPackaged`) swaps a package-level hook
   for `install.Packaged` rather than going through `-binary`:
   the marker is read beside the running binary,
   which a test cannot place next to `go test`'s binary without touching the build tree.
4. `packaging/deb/test.sh` checks the `ExecStart=` line alone for `-update-request`,
   since the derived unit keeps the embedded one's comment naming the flag;
   the header the derivation adds says why there is no helper.
5. The container cannot start the service (no systemd);
   the enable is proved through deb-systemd-helper's symlinks,
   the start on a real box is the owner's check.
6. Two checks fed through a pipe to `grep -q` misread a success under `pipefail`;
   both tests feed such checks from here-strings, as the shell rules ask.

## Block 2, the apt repository

1. The repository's public key is not committed to the repository:
   the site serves it as `massimo.asc`,
   exported from the key in CI at each release,
   so that nothing can drift between the two; the plan had a committed copy.
2. The keyring package's version is the key's expiry date,
   not its creation date,
   so that an extended key is a newer package and reaches the boxes through apt.
3. The container test keeps Debian's own sources:
   the slim images lack `adduser`, which the package depends on;
   `apt-cache policy` proves the package itself came from the suite.
4. The apt steps are two jobs, `apt-build` in the `signing` environment and
   `apt-deploy` in `github-pages`, rather than steps of the sign job:
   an environment secret and a Pages deployment cannot share one job,
   and the sign job stays what it was.
   `apt-build` installs `apt-utils` when the runner lacks it.
5. The pool keeps the newest ten releases per channel and the site has no state,
   so a release deleted on GitHub leaves the pool at the next release.

## Block 3, Homebrew

1. The formula is `massimo-alpha` for the channel and `massimo` for stable from the start,
   each conflicting with the other, as the plan named them.
2. The formula states its `version` explicitly,
   since a tarball served from a local port in CI carries no version in its URL.
3. CI's macOS job serves the tarball of the checkout over a local port
   (`python3 -m http.server`) rather than a `file://` URL,
   which brew's downloader may not take;
   `brew test` and the marker checks run against that install.
4. `brew audit` did not run here (no Homebrew on the build machine);
   the formula's Ruby syntax is checked,
   and the macOS CI job is the first build of it.
5. The formula's test starts `serve -listen 127.0.0.1:<port> -proxy off`
   and reads `/healthz`,
   which answers with no Cube configured (checked on Linux).

## Across the run

1. Windows code signing (Block 4) is not started,
   per the owner's D5 ("no special windows signing ATM").
2. The IDE's quality pass was unavailable; the fallback was `gofmt`,
   `go vet` for the three platforms, `shellcheck` and `bash -n` on every script,
   a YAML parse of the workflows,
   `reflow.sh` on the documents and `GUIDELINES/shared/tools/check`.
3. The guidelines' checks report one finding that predates this work,
   a reflow warning in `tools/ui-test/README.md`.

## The owner's steps left

- Add the new required checks to the ruleset on `main`:
  "Debian package (debian:trixie-slim)" and "(debian:bookworm-slim)".
- The apt key: make it, put `MASSIMO_APT_KEY` and `MASSIMO_APT_KEY_PASSPHRASE`
  into the `signing` environment,
  set Pages to deploy from GitHub Actions (packaging/README.md,
  "The apt repository").
- The tap: create `metaworx/homebrew-massimo`, make the deploy key,
  put `HOMEBREW_TAP_DEPLOY_KEY` into the repository's secrets (packaging/README.md,
  "Homebrew").
- On the Pi: `sudo massimo install -remove`, the apt snippet,
  `sudo apt install massimo`, `sudo massimo install -listen :80`,
  once a release with the package is out.
- The package's maintainer field carries the owner's public git identity.
