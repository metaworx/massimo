# NOTE AptUpdate v1.1: block 5 run

How Block 5 of [AP AptUpdate
v1.1](2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md) went,
under the owner's `EXEC++` of 2026-11-10, in pull request #171.

1. **The bullet is a fragment**, `CHANGELOG.d/Fixed/claude-deb-version.md`.
   The guidelines ask for one since the plan was first written (COMMIT.md §5.5),
   and the owner said so during the run.
   v1.1 was corrected to say so before its commit.
2. **No commit hashes.**
   v1.1's "Done" lines and the real box run's NOTE named commits by their hash.
   Both were corrected before the commit: the plan names the pull requests,
   and the NOTE names its commit by its summary line, in "chore(aps):
   NOTE AptUpdate v1.0 names main by its merge, not a commit hash".
3. **`TestDebVersion` and `make debver`, beyond the plan.**
   The systemd cases go up from v0.12.0-alpha.1, and v0.13.0-alpha.2 is out,
   so any build from `main` sorts above their old package.
   Dropping `--allow-downgrades` alone therefore proves nothing about the tag a
   build follows.
   The test reads the Makefile's `DEBVER` through `make debver` for each form
   `git describe` gives, checks that `debTag` reads it back, and,
   where dpkg is installed, checks the order apt sees.
   With the old `DEBVER` it fails on four of the six forms.
4. **The rolling builds.**
   Since AP Channels, the devel and nightly builds are releases between tags,
   with their packages attached, so F1 reached more than hand builds.
   `packaging/apt/publish.sh` keeps them out of the apt repository
   "until its packages' versions sort".
   They sort from Block 5 on; giving them suites is not this plan's.
5. **Verification:**
   - the test gate (`go test ./...`, `gofmt`, `go vet` for three platforms,
     `make cross`);
   - `make packages`, whose version sorted after `0.13.0~alpha.2` and before
     `0.13.0~alpha.3`;
   - `make deb-test`, `make deb-systemd-test` (all five cases)
     and `make apt-test` (with the update helper's case),
     on trixie and bookworm.

   The branch then took `main`, with the fragments made required,
   and the tests passed again.
