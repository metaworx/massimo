# AP CiHardening v1.0: tests and CI hardening

## TL;DR

What bytemare/ksf's CI does well, adopted where it pays here,
plus a check of our own that no real identifier is in the tree.

- identifiers: unchecked → a Go test over the tracked files and the decoded
  protocol fixtures; the real-looking serials and RF addresses → placeholders
- fuzzing: none → fuzz targets for every parser of untrusted bytes, their
  seeds run by every `go test`, a Fuzz job on pushes to `main`
- `-race`:
  local only → a Race job for `server`, `store`, `cubelan`, `proxy`
- actions: pinned by tag → by commit SHA, kept current by Dependabot,
  monthly and grouped, with two carve-outs for its PRs
- CodeQL, secret scanning, push protection, Dependabot security updates:
  the repository's own switches, the owner's to turn on; documented

### Blocks

1. Identifiers: the check and its allowlist, the placeholders it asks for
2. Fuzz: the targets, their seeds, `make fuzz`, the Fuzz job
3. Race: `make race`, the Race job
4. Pins: the actions by SHA, `.github/dependabot.yml`, the carve-outs
5. Docs: `GUIDELINES/project/CI.md` and the owner's switches

## Discussion

The owner asked on 2026-11-07 what of bytemare/ksf's tests could be adopted,
and gave `PLAN all four plus the serial, RF or MAC scanner`;
after the TL;DR gate of that investigation, `PLAN++` on 2026-11-08,
the decisions as recommended, D3 changed to B at the owner's question:

- **D1-A.**
  The seven serials of no known public source become placeholders;
  a captured base64 line is re-encoded with its placeholder,
  and its test says the line is constructed, not captured.
- **D2-A.**
  The recorded greeting's RF addresses, and the two others of no known source,
  become placeholders of the `0a…` form, consistently through the greeting,
  its UI copy, the tests and the docs.
- **D3-B.**
  Dependabot, monthly and grouped, `build(deps)`:
  the Guidelines job leaves its PRs out of the commit-message check,
  and its updates are recorded in one `[Unreleased]` bullet by whoever merges
  or cuts the release.
  Simpler in the long run than a script of our own that reports what is behind.
- **D4-A.**
  CodeQL by the repository's default setup, switched on by the owner,
  with secret scanning, push protection and Dependabot security updates.
- **D5-A.**
  Seeds in every `go test`; a Fuzz job on pushes to `main`, 10 s a target.
- **D6-A.**
  A Race job of its own, beside Vet and test.
- **D7-A.**
  The identifier check is a Go test that walks `git ls-files` and decodes the
  protocol fixtures; the allowlist names a reason per entry.

The gate's Blocks 5 (CodeQL) and 6 (docs) are one block here:
with D4-A, CodeQL leaves no code, only its documentation.

Coordinated with the session fixing the `TestBackupAutomatic` race (AP BackupListRace,
`internal/server/backups.go` and `backups_api.go`):
no file in common but `CHANGELOG.md`;
its race fails about one `-race` run in sixteen until it is on `main`,
which the Race job will show.

## Analysis

- **What bytemare/ksf runs** (its four workflows call `bytemare/workflows`):
  `go test -race -vet=all`, coverage to Codecov and SonarQube, fuzz targets in
  `tests/fuzz_test.go` (one checks that validation and hashing agree),
  CodeQL, Semgrep, Gitleaks, govulncheck, Scorecard, Dependency Review, ORT,
  SuperLinter, a do-not-submit check, DCO, every action pinned by SHA under
  harden-runner's egress block, SLSA provenance.
  Not adopted: the egress block (it broke their PRs,
  and our jobs reach many more hosts), the external services (Codecov,
  SonarQube, Semgrep), the heavy linters (SuperLinter, ORT), DCO (one owner),
  SLSA (later, beside the signed updates).
- **Identifiers in the tree.**
  Serial-shaped strings: the placeholders `[A-Z]{3}0000\d{3}`;
  three from the public,
  MIT-licensed `Bouni/max-cube-protocol` (`KEQ0523864` in H-Message.md
  and Cube_Discovery.md, `KEQ0378040` in M-Message.md,
  `LEQ0015340` in N-Message.md, with the RF addresses `097f2c` and `0e15cc`);
  and seven of no known source: `JEQ0544923`, `KEQ0565026`,
  `NEQ0908359` (`internal/protocol/cube/cube_test.go`),
  `JEQ0328398` (`docs/api-v1.md`),
  `IEQ0158740` (`internal/backup/backup_test.go`),
  `KMD1016788` with its RF `099c3e` (the portal tests, `cube_test.go`),
  `LEQ0654321` (the UI tests).
  MAC addresses: all of eQ-3's prefix `00:1a:22` with endings of small numbers,
  placeholders already.
  RF addresses: the greeting's Cube `0b9573` and its six devices `14ce3d`,
  `14ce58`, `14d08e`, `151486`, `151495`, `1514b6`,
  whose serials were made placeholders but not their addresses, and `022839`,
  `012ca9`: about 290 mentions in text and 80 inside base64 (the greeting's C,
  L and M lines, the commands the tests expect, `docs/portal-protocol.md`),
  in about 40 files.
- **Where a scanner sees an RF address.**
  Six hex digits alone are everywhere (colours, hashes),
  so the check reads them in their contexts: `C:<rf>,`,
  the H line's second field, `rf_address`, `RFAddress: 0x…`, `device=`,
  `target=`, `/devices/<rf>`, and the decoded fixtures (H, C, M and L lines).
  Base64 inside Go sources is not decoded;
  the one-time rewrite covers what is there.
- **Untrusted bytes, the fuzz targets.**
  From the Cube and, through the proxy, from any LAN client: `cube.Reader`,
  `ParseLine`, `SplitLines`, `ParseHello`, `ParseDeviceList`,
  `ParseMetadataPart`/`ParseMetadata`/`DecodeMetadata`,
  `ParseConfig`/ `DecodeConfig`, `DecodeWeekProgram`, `ParseSendResult`,
  `ParseNewDevice`, `ParseNTPServers`, `ParseCryptoReply`;
  the proxy's client commands `ParseRFAddress`, `ParseRadioCommand`,
  `ParseDeleteDevices`.
  From an upload: `backup.Unmarshal`, JSON and YAML.
  From the store and the API: `parseHash`, `ParseKSF`.
  Round trips: `Metadata.Encode` and `DecodeMetadata`,
  `RFAddress` and `ParseRFAddress`, `EncodeTemp`,
  a backup file and its `Unmarshal`.
- **Dependabot and the guidelines.**
  Its commit messages fail `commit-msg.sh` in any configuration (no `Key changes:`,
  no `Assisted-by`); CI checks a PR's commit range,
  not the CHANGELOG bullet of each commit.
  The actions in use: `actions/checkout@v5` (12), `actions/setup-go@v6` (8),
  `actions/upload-artifact@v6` (4), `actions/download-artifact@v7`,
  `actions/upload-pages-artifact@v4`, `actions/deploy-pages@v4`.
- **The repository's switches.**
  Public; code scanning "not-configured", default setup would cover Go,
  JavaScript, Python and the workflows; secret scanning,
  push protection and Dependabot security updates off.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/server`,
`internal/store`, `internal/cubelan` and `internal/proxy`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` or `tools/ui-test/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube or the owner's Pi.
Each block is one functional commit citing `AP CiHardening v1.0, Block <n>`,
with its CHANGELOG bullet where it touches a shipped path; one pull request.

### Block 1: Identifiers

1. `tools/identifiers/` (outside the shipped paths): `identifiers_test.go`
   walks `git ls-files` from the repository's root (skipped outside a git
   checkout) and refuses:
   a serial `[A-Z]{3}\d{7}` other than `[A-Z]{3}0000\d{3}`;
   a MAC whose last three octets are not each `0x`, or all zero;
   an RF address in one of the contexts above not of the form `0a????`;
   the same in the decoded H, C, M and L lines of
   `internal/protocol/cube/testdata/*.txt` and `tools/ui-test/greeting.txt`;
   each unless `allowed.txt` lists it with a reason.
   Left out: binary files, `internal/server/ui/vendor/`, `third_party/`,
   the released sections of `CHANGELOG.md` only if they hold one.
2. `allowed.txt`:
   the three serials and two RF addresses of `Bouni/max-cube-protocol`,
   with the file each comes from.
3. The rewrite, by a script in `GUIDELINES/temp/` not committed:
   the seven serials to `[A-Z]{3}00005\d{2}`, `099c3e` with them;
   the greeting's RF addresses to `0a0001` (the Cube) and `0a0101` to `0a0106`
   in their order, `022839` and `012ca9` to `0a0201` and `0a0202`;
   in text and inside every base64 token that decodes to hold one,
   re-encoded at the same length; expected values that follow from them (hashes,
   ciphertexts, sorted orders) recomputed by the tests' own code.
4. `make ui-screenshots` again where a screenshot shows one of them.
5. CHANGELOG, Changed: the fixtures carry placeholder serials and RF addresses.

**Verification:**
the test gate; the check fails with one real-looking serial,
MAC or RF address put back by hand, and passes without.

### Block 2: Fuzz

1. `Fuzz…` targets beside the existing tests: `internal/protocol/cube`
   (the parsers and decoders above, the round trips), `internal/proxy`
   (the client commands), `internal/backup` (`Unmarshal`, JSON and YAML,
   and its round trip), `internal/store` (`parseHash`, `ParseKSF`).
   Each refuses a panic, an allocation beyond the input's bound, and a round
   trip that changes the value; seeds from the greeting and the existing tests.
2. `Makefile`:
   `fuzz`, every target in turn for `FUZZTIME` (10 s).
3. `.github/workflows/ci.yml`:
   a Fuzz job on pushes to `main`, `make fuzz`;
   a crash's input uploaded as an artifact.
4. CHANGELOG, Added.

**Verification:**
the test gate; `make fuzz FUZZTIME=30s` finds nothing,
or what it finds is fixed in its own commit and named in the run's NOTE.

### Block 3: Race

1. `Makefile`:
   `race`, `go test -race` on the four packages.
2. `ci.yml`:
   a Race job beside Vet and test, `make race`,
   with `MASSIMO_ENV_FILE=/dev/null`.
3. CHANGELOG, Changed.

**Verification:**
the test gate; the job's time on the runner noted.

### Block 4: Pins

1. Every `uses:` of the three workflows at the commit of its release,
   `@<sha> # v<x.y.z>`, the release resolved from its tag.
2. `.github/dependabot.yml`:
   `gomod` (direct dependencies,
   `bytemare/ksf` ignored while `third_party/bytemare-ksf` replaces it)
   and `github-actions`, monthly, one group each,
   `commit-message: prefix: build, include: scope`.
3. The Guidelines job checks a PR's commit messages unless the PR is Dependabot's;
   its other checks run on it.
4. CHANGELOG, Changed.

**Verification:**
the test gate; the workflows parse; every pinned SHA is its tag's commit.

### Block 5: Docs

1. `GUIDELINES/project/CI.md`:
   section 2 (the Race and Fuzz jobs), section 3 (`make race`, `make fuzz`),
   Dependabot and its carve-outs (the commit messages,
   the one `[Unreleased]` bullet), and the repository's switches:
   CodeQL's default setup, secret scanning, push protection,
   Dependabot security updates, the owner's in Settings.
2. `README.md` where the Makefile's targets are listed.
3. No CHANGELOG bullet: no shipped path.

**Verification:**
`GUIDELINES/shared/tools/check`.

## Proposed commit message

```
chore(aps): +CiHardening v1.0

Tests and CI hardening (an identifier check, fuzz targets, a race job, actions pinned by SHA with Dependabot) gets its plan registered before the first project write.

Key changes:
- GUIDELINES/wip/2026-11-08_00-04_AP_CiHardening_tests_and_ci_hardening.md: the plan, five blocks, with the owner's decisions D1 to D7 of 2026-11-08.

File-revision-v1: GUIDELINES/wip/2026-11-08_00-04_AP_CiHardening_tests_and_ci_hardening.md::v1.0::2026-11-08::All::the first revision
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `test(fixtures): placeholder serials and RF addresses, and a check that keeps them`
2. `test(fuzz): fuzz the parsers of untrusted bytes`
3. `ci(race): run the race detector on the concurrent packages`
4. `ci(actions): pin the actions by SHA and let Dependabot keep them current`
5. `docs(ci): the new jobs, Dependabot and the repository's switches`

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate's findings and
  the owner's decisions D1 to D7, D3 changed to B.
