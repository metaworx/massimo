# AP KeyringUpgrade v1.0: The helper upgrades the keyring

## TL;DR

The apt update helper upgrades `massimo-archive-keyring` with `massimo`,
and fetches it from the current suite before a switch to a rolling suite
needs its rolling key, so that the web UI alone moves a box to `devel` or
`nightly`.

- `apt-get install --only-upgrade massimo` → `… massimo massimo-archive-keyring`:
  a new or rotated key arrives as any update does
- switch to `devel`/`nightly`, rolling keyring missing: refused →
  `apt-get update` on the current suite, keyring upgraded, checked again;
  refused only where that suite has no keyring with the rolling key either
- the apt test's switch starts from a keyring package without the rolling key

### Blocks

1. The helper upgrades the keyring, its tests and documents

## Discussion

On 2026-11-13 the owner switched breno, an apt box on `alpha`, to the
channel `devel` in the web UI. The update failed:

> the suite devel is signed with the rolling key, which the installed
> massimo-archive-keyring does not carry
> (/usr/share/keyrings/massimo-rolling-archive-keyring.gpg)

The alpha suite has carried a keyring package with both keys since the
release of v0.15.0-alpha.1 (`1.0.20261113082328`, both files checked).
`apt upgrade` would have installed it, but the helper upgrades `massimo`
alone, so a box updated only from the web UI never gets a newer keyring.
AP Channels v1.2, Block 14, left that step to the owner by hand.

**The owner's decision** (`EXEC D69-A`, 2026-11-14):

- **D69-A:** every apt update upgrades both `massimo` and
  `massimo-archive-keyring`; a switch to a rolling suite with the rolling
  keyring missing first fetches the current suite and upgrades the
  keyring there, then checks again. Not B, the manual step with a better
  message.

## Analysis

- `cmd/massimo/update_apt.go`: `aptInstallArgs` ends in `massimo`;
  `aptSwitch` refuses a rolling suite while `aptRollingKeyring` is
  missing, before any apt command runs.
- Every box set up as `packaging/README.md` "The apt repository" says has
  the keyring package installed (its first step), and every suite's pool
  holds `pool/keyring` (AP Channels v1.2, Block 11). `--only-upgrade`
  skips a package that is not installed, so a box set up otherwise is not
  broken by the second name.
- The keyring package owns only `/usr/share/keyrings/*.gpg`, no conffile,
  so the `--force-conf*` options change nothing for it.
- A rotated numbered key reaches a box the same way: the keyring package
  of a rotation is newer (packaging/apt/test.sh, "A rotated key").
- The pre-fetch runs `apt-get update` on the current suite, then the
  switched one is fetched as before: two fetches only on the run that
  switches to a rolling suite without the key.
- Tests: `fakeApt` tells the keyring's install from massimo's by its last
  argument; a run without the rolling key that gets it from the keyring
  install switches; one that does not is refused with the source
  unchanged. `packaging/apt/test.sh`'s switch test installs a keyring
  package rebuilt without the rolling key and at a lower version first.

## Implementation Plan

### Block 1: The helper upgrades the keyring

1. `cmd/massimo/update_apt.go`:
   - `aptInstallArgs` names `massimo massimo-archive-keyring`;
     `aptKeyringArgs`, the same options with the keyring alone.
   - `aptUpdate`: where the request's channel is a rolling flavour, the
     source on another suite and `aptRollingKeyring` missing, it runs
     `aptGetUpdate` and the keyring's install before `aptSwitch`
     (status `checking`); a failure there fails the run, the source
     untouched.
   - `aptSwitch`'s refusal names the suite the keyring came from:
     it has no keyring package with the rolling key yet.
2. `cmd/massimo/update_apt_test.go`: the fixed commands; a switch that
   gets the key from the keyring's install; the refusal where it does
   not, `apt-get update` run once and the source unchanged.
3. `packaging/apt/test.sh`: the switch container installs a keyring
   package without the rolling key (`dpkg-deb -R`, the file removed,
   version `0.1`) and the helper's switch to `devel` upgrades it.
4. `packaging/README.md` "Updates through apt": the command, and the
   rolling key arriving with the keyring's upgrade.
5. `CHANGELOG.d/Fixed/apt-helper-keyring.md`.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `make cross`, `gofmt -l .`, `go vet` on the three platforms,
  `make apt-test`, `reflow.sh` no worse, `changelog.sh check --staged`.
  After the merge, the owner's devel build on breno: the switch from the
  web UI, without the hand step.

## Proposed commit message

```
fix(update): The apt helper upgrades the keyring with massimo

A box updated from the web UI never got a newer massimo-archive-keyring, so its switch to devel or nightly was refused for the rolling key the alpha suite's keyring has carried since v0.15.0-alpha.1. The helper now upgrades the keyring with massimo, and before a switch to a rolling suite without the rolling key fetches the current suite and upgrades the keyring there first.

AP KeyringUpgrade v1.0, Block 1.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-14): first revision, D69-A.
