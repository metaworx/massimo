# Changelog

All notable changes to Massimo are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions below
1.0.0 are pre-releases, tagged `vX.Y.Z-alpha.N` by `make release-alpha`
(`README.md`, "Versions"). Releases up to v0.6.0-alpha.1 are described
by their tags and the pull requests they merged.

## [Unreleased]

### Added

- `massimo netconfig` reads and writes a Cube's network configuration
  over UDP (static address or DHCP), with a broadcast recovery for a
  Cube on a wrong address.

- `massimo discover` shows whether a Cube takes its address by DHCP.

- `-direct` and `-no-direct` on every command: skip the configured
  proxy, or refuse the fallback to the Cube when the proxy does not
  answer; the fallback warns that it takes the Cube's only connection.

- Updates from the web UI on Windows: the service starts the update
  helper itself as a detached process; the CI Windows job exercises
  the API path and a rollback.

- `massimo legacy` shows the original software's Cube search lines.

### Changed

- The first device list of a connection carries the greeting's time as
  `observed_since`.

- `.gitattributes` keeps the embedded unit files, plists and scripts LF
  on every checkout; the protocol fixture keeps its CRLF.

- `massimo config` knows the update variables as names.

- `update` removes `massimo.exe.failed` once a newer binary is in
  place, and a rollback tolerates a missing target.

[Unreleased]: https://github.com/metaworx/massimo/compare/v0.6.0-alpha.1...HEAD
