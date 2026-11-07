# ANALYSIS Redact: real identifiers in the tree

2026-11-07, on `main` as of `chore(aps): +NOTE Pages v1.0`.
The inventory behind AP Redact's TL;DR gate, which its blocks work from.
No real serial, name or RF address appears below:
the serials are named by their Cube's role, the names by labels
(`ROOM1`–`ROOM6`, `NAME7`–`NAME13`) from a decoder kept outside the repository.

## 1. The two greetings

`internal/protocol/cube/testdata/cube-fw0113-greeting.txt` (CRLF, 10 lines)
is loaded by 26 test files through `fakecube.LoadGreeting`,
and by CI's `massimo fakecube -greeting …` (`.github/workflows/ci.yml`, two jobs).
`tools/ui-test/greeting.txt` (LF) is a copy with one device more:
its M and L lines differ, its other lines are the same apart from the line ends.
Neither is embedded in a binary or a package.

| Line | Carries in plain text | Carries inside its base64 |
|------|-----------------------|---------------------------|
| `H:` | the spare Cube's serial (field 1), its RF address | - |
| `M:00,01,…` (281 bytes; UI copy 311) | - | 6 rooms: ids, names, first RF address; 6 thermostats (7 in the UI copy): RF address, serial, name, room |
| `C:` × 7 | each device's RF address | the Cube's own serial (bytes 8–17) in the Cube's C line; each thermostat's serial (bytes 8–17) in its own |
| `L:` | - | RF addresses and states only |

- **Serials**: the Cube's one, six thermostat serials (`MEQ` prefix);
  the UI copy's window contact is already a placeholder (`MEQ0000001`).
  The thermostat serials appear nowhere else in the tree, not even decoded.
- **Names**: six rooms (common German room words, two of them with an umlaut),
  six thermostats (`Radiator <room>`), one window contact in the UI copy
  (`Fenster <ROOM4>`).
  Byte order and locale order of the names agree:
  rooms `ROOM3 < ROOM4 < ROOM5 < ROOM2 < ROOM6 < ROOM1`,
  devices `NAME13 < NAME7 < NAME10 < NAME9 < NAME11 < NAME8 < NAME12`.
- **No other copy**: every base64 token of 16 characters or more in the tree
  was decoded and searched; the serials and names are encoded only in the two
  greetings.
  `cube_test.go:108–135` holds an M line of its own (`metadataExample`, the
  public protocol example), whose common room words coincide with `ROOM1`
  and `ROOM6`: it stays, and so does `cmd/massimo/legacy_test.go`'s short
  `sendMetaData` line, which carries no name.
- **Re-encoding**: `DecodeMetadata` / `Encode` and `DecodeConfig` give the
  layout; substituting the serials and names in the decoded bytes and
  encoding again keeps every line valid.
  The M line's length changes only if a name's byte length does.

## 2. Plain-text occurrences

### The spare Cube's serial, as is

| Area | Files | Occurrences | Files (occurrences) |
|---|---|---|---|
| `cmd/` | 2 | 6 | `cmd/massimo/serve_test.go` (5), `cmd/massimo/status_test.go` (1) |
| `internal/` | 25 | 193 | `internal/backup/backup_test.go` (1), `internal/cubelan/client_test.go` (1), `internal/cubelan/discover_test.go` (4), `internal/portal/portal_test.go` (6), `internal/portal/portalcrypto/portalcrypto_test.go` (1), `internal/portal/probe_test.go` (4), `internal/protocol/cube/cube_test.go` (1), `internal/server/audit_test.go` (9), `internal/server/auth_test.go` (5), `internal/server/capabilities_test.go` (11), `internal/server/commands_test.go` (6), `internal/server/config_api_test.go` (37), `internal/server/cubes_api_test.go` (23), `internal/server/cubes_race_test.go` (8), `internal/server/discovery_test.go` (9), `internal/server/followup_test.go` (9), `internal/server/hidden_test.go` (11), `internal/server/log_api_test.go` (8), `internal/server/pairing_api_test.go` (4), `internal/server/poll_test.go` (5), `internal/server/restore_api_test.go` (6), `internal/server/server_test.go` (20), `internal/server/store_test.go` (1), `internal/server/ui_test.go` (2), `internal/server/users_api_test.go` (1) |
| `.github/` | 1 | 2 | `.github/workflows/ci.yml` (2) |
| `tools/` | 6 | 19 | `tools/ui-test/README.md` (1), `tools/ui-test/run.sh` (2), `tools/ui-test/screenshots.py` (1), `tools/ui-test/test_cubes.py` (11), `tools/ui-test/test_pairing.py` (2), `tools/ui-test/test_settings.py` (2) |
| `docs/` | 5 | 9 | `docs/api-v1.md` (1), `docs/handover-session.md` (1), `docs/portal-protocol.md` (1), `docs/proxy.md` (2), `docs/windows-test-2026-11-05.md` (4) |

Total: 39 files, 229 occurrences.

### The spare Cube's serial, lower case

| Area | Files | Occurrences | Files (occurrences) |
|---|---|---|---|
| `internal/` | 6 | 24 | `internal/server/auth_test.go` (8), `internal/server/config_api_test.go` (9), `internal/server/cubes_api_test.go` (1), `internal/server/cubes_race_test.go` (2), `internal/server/restore_api_test.go` (2), `internal/server/server_test.go` (2) |

Total: 6 files, 24 occurrences.

### The spare Cube's serial's digits behind another prefix

| Area | Files | Occurrences | Files (occurrences) |
|---|---|---|---|
| `internal/` | 2 | 5 | `internal/proxy/proxy_test.go` (3, an `XEQ` proxy serial), `internal/server/cubes_api_test.go` (2, an `LEQ` proxy serial, lower and upper case) |

Total: 2 files, 5 occurrences.

### The house Cube's serial, any prefix or case

| Area | Files | Occurrences | Files (occurrences) |
|---|---|---|---|
| `internal/` | 1 | 1 | `internal/server/auth_test.go` (1) |
| `docs/` | 6 | 18 | `docs/api-v1-openapi.yaml` (1), `docs/api-v1.md` (3), `docs/handover-session.md` (2), `docs/handover-write-test.md` (6), `docs/proxy.md` (4), `docs/write-test-2026-11-05.md` (2) |

Total: 7 files, 19 occurrences.

### Room and device names from the greeting

| File:line | Names (labels) |
|---|---|
| `docs/api-v1.md:1034` | ROOM2 |
| `docs/api-v1.md:1040` | ROOM2 |
| `docs/handover-write-test.md:20` | ROOM2 |
| `internal/backup/backup_test.go:77` | NAME11, ROOM6 |
| `internal/backup/backup_test.go:118` | ROOM6 |
| `internal/backup/backup_test.go:141` | NAME11, ROOM6 |
| `internal/backup/backup_test.go:389` | ROOM2, ROOM6 |
| `internal/protocol/cube/cube_test.go:123` | ROOM1, ROOM6 |
| `internal/protocol/cube/cube_test.go:135` | ROOM6 |
| `internal/protocol/cube/cube_test.go:513` | ROOM2 |
| `tools/ui-test/README.md:13` | NAME13, ROOM4 |
| `tools/ui-test/screenshots.py:14` | ROOM4 |
| `tools/ui-test/screenshots.py:251` | ROOM4 |
| `tools/ui-test/test_rooms.py:39` | ROOM4 |

The names in `docs/api-v1.md:1034,1040` are a common room word used as an
API example; `docs/handover-write-test.md:20` names a room of the house Cube.

## 3. Outside this inventory

Seen while searching, not counted above:

- **RF addresses**: the greeting's seven are the spare Cube's devices,
  used in about 40 test files' expected lines, `README.md`, the API
  documents and the `-probe-target` help text of `cmd/massimo/portal.go`.
  `docs/handover-write-test.md` and `docs/portal-protocol.md` name some of
  the house's.
- **LAN details** in the handover and proxy documents: private addresses,
  a network's name, the machines' names.
- **Captured ciphertexts** of the spare Cube in
  `internal/portal/portalcrypto/portalcrypto_test.go`: no key or IV;
  its comment quotes the plaintext's first block, serial included.
- **Other serials** in `cube_test.go`, `backup_test.go` and `docs/api-v1.md`
  come from the public protocol examples, or are placeholders.

## 4. History and GitHub

- The repository is public (`metaworx/massimo`), 597 commits.
- The spare Cube's serial: added 2026-11-03, in 65 commits' diffs and in all
  20 release tags; the house Cube's: from 2026-11-04, 8 commits, 19 tags.
- No commit message, tag message or `CHANGELOG.md` entry names either.
- Pull request descriptions: one names the spare Cube's serial (#42),
  one the house Cube's (#9). No issues exist.
- What a serial unlocks: nothing by itself.
  It selects a Cube in LAN discovery and `netconfig`, and names it to the
  portal; the portal's AES key is printed on the board and not derivable
  from it (`docs/portal-protocol.md`, "Transport").
