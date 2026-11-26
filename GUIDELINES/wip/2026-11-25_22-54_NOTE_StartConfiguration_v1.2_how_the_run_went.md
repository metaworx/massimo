# NOTE StartConfiguration v1.2: how the run went

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.

This is the run of v1.2's Blocks 14 to 22, by the owner's `PLAN++` of
2026-11-25. Each block was built by a sub-agent, read by an independent
second reader (two for Block 15), and its points fixed. The session then
verified each commit and committed it. Every test fails on its planted
fault, the Windows ones through interop. Nothing reached real hardware.

## The pull requests

| Blocks | Pull request | State |
|--------|--------------|-------|
| 17 WindowsNames | #458 | open |
| 14 to 16 SettingsThere, SettingsValues, KeptUnread | #462 | open; it merges #458's branch, so #458 goes first |
| 18 Tables | none | carried to AP CapabilityTable (D55), merged as #457 and retired |
| 19 PostinstBom, with D43's comment | #459 | open |
| 20 and 21 PlainLogs, PlainResolve | #461 | open |
| 22 TestKit | #460 | open |

Each branch merged `main` before its pull request opened.

## Where the run went differently from the plan

- **Block 17, the security commit,** also fixes `massimo config`'s own
  listing.
  - It had its own check: case-sensitive, and only `_KEY`, `_IV` and
    `_TOKEN`. So it printed in clear any name with PASSWORD or SECRET in it,
    and, on Windows, `MASSIMO_API_Token`.
  - It now redacts by `SecretVariable`, in `printVariables`.
  - The summary line adds "or massimo config", and the fragment names the
    holder of `settings`.
- **Block 17, the second commit:** on Linux and macOS, a known name in
  another case is now set aside with a note, as a duplicate is, rather than
  set.
  - So a Cube's line in another case is no longer read by serve and the
    commands. They used to read it through `PerCubeValue`'s case-blind
    comparison.
  - `values()` keys by `NameKey`, `Note.Name` is the upper case, and
    `Removed` is checked on the key.
- **Block 14** amends Block 13's unreleased bullet, under COMMIT.md §5.2,
  instead of adding its own. The check on "exists; kept it and did not copy"
  moved into `TestInstallKeepsAFileServeRefuses`, for all three kinds.
- **Block 15:**
  - The plan left relative paths to the start, "since the service runs
    from /". The unit and the plist set `WorkingDirectory=/var/lib/massimo`,
    so install now checks a relative path there, under `-root`.
  - On Windows, the refusals of a TLS check that reads a relative path are
    dropped, with a warning.
  - Refusals are one per line.
  - The `-tls` run of the start's checks moved into `checkSettingsFile`, so
    that one refusal names both.
  - `checkTLSSettings` is gone.
  - Windows parses its kept file (D50-A), and a `connectManager` seam tests
    that this comes before the service manager.
  - `-acme-dns-hook` is checked under `-root`.
  - The fragment also names `-tls` options.
- **#462 merges Block 17's branch.**
  - That gives `values()` by `NameKey`, without which a Windows file's
    `massimo_hsts=-1` would pass install.
  - Its commit "fix(install): Windows: install reads the settings' names in
    any case, as the start does" keys `Lookup` and `setEnvLines` by
    `NameKey`, and amends Block 17's fragment.
- **Block 16 goes past D51-A.**
  - `-tls` over a kept file install cannot read is refused before any
    write. Before, install appended UTF-8 lines to a UTF-16 file and
    installed units for TLS the file does not say.
  - The warnings say what was kept, and tell the owner to run install again
    once the file is fixed.
- **Block 19:**
  - The fragment opens "The Debian package", as older bullets do.
  - D43's comment is a commit of its own.
  - The hole is on `main` only, since #425: the released serve refused a
    file with a byte order mark at its start. So #459 should be merged
    before the next release is cut.
- **Block 20:** the plan said the lines "kept an empty `cube=`". In fact the
  key was absent. The fragment also covers `-addr` without `-sn`, whose
  lines never had `cube=`.
- **Block 21** ties only where exactly one row matches. Where several do,
  it ties none, with a Warn. It amends Block 11's unreleased fragment.
- **Block 22:**
  - Fixes also a `Serve` after a `Close`, which served a listener nothing
    closed.
  - `sweepBuilt` takes the time.
  - docs/testing.md says how the directory is removed and swept.
  - The test commit is `test(cli)`, not `test(cmd)`, and its summary no
    longer says Windows.

## Found outside the plan, not changed

- `TestProxyFailingInTheAPI` (internal/server/proxy_retry_test.go) flakes
  under `-race`: it wants `tries == 1`, while `quickRetry` retries every
  5 ms. A fix is `tries >= 1`, as the test's system check already does. It
  is older than this plan, and CI's Race job can hit it.
- `SecretVariable` matches suffixes only, so an unknown name such as
  `MASSIMO_API_TOKEN_OLD` shows its value.
- `PerCubeValue` compares case-blind on Linux for the process environment.
  A `MASSIMO_CUBE_<SN>_Name` set there is read by serve, while
  `massimo config` calls it unknown.
- `certs.Load` doubles its prefix, `tls: tls: private key does not match
  public key`. Install repeats the start's words.
- A relative `-acme-dns-hook` is looked for at the service's working
  directory, so the refusal names a path the user never typed.
- `internal/server/backups.go` and `restore_api.go` read `rt.serial`
  directly. It is no race, since `storeID` orders it, but `cubeSerial()`
  would be the same everywhere.
- `internal/update/store.go` ignores its removal errors.
- After a tie by name, `PlainServedNote` gives the plain address alone.
- `cubeEndpoint` lowercases an IPv6 zone, so an upper-case zone never ties.
- On a UTF-16 settings file, postinst's grep writes "binary file matches"
  into apt's output.
- `TestNextVersion` fails on a Windows host whose `bash` is WSL's. CI's
  runner has Git Bash.
- The owner's Windows service was not touched. Its log, read by a reader,
  shows a Cube's polls unanswered from 21:47:45, held until 22:32.

## Not run

`make ui-test`, since the one UI change is a comment, and `make lan-test`,
since discovery is unchanged; each pull request's CI ran its own jobs.
