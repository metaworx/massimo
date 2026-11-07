# AP Redact v1.0: placeholders for real identifiers

## TL;DR

The recorded greeting, its UI-test copy, and the tests, CI and documents
around them carry the owner's real Cube serials and the recorded room and
device names.
They become placeholders: both greetings re-encoded in their decoded bytes,
every expectation and document following; history stays as it is.

- spare Cube → `KEQ0000100`, in every case and derived form the tests use;
  its thermostats → `MEQ0000101`–`MEQ0000106`, by room;
  house Cube → `KEQ0000200`
- rooms → `Düne`, `Fels 1`, `Fels 2`, `Küste`, `Meeresstrand`, `Wattenmeer`;
  thermostats → `Radiator <room>`, room 3's `Radiator Strandkorb 1`;
  the UI copy's window contact → `Fenster Fels 1`
- every line keeps its length: same byte lengths, umlauts and sort order
- guard: a test fails on a greeting serial that is not a placeholder
- kept: RF addresses, LAN details, the public protocol examples,
  the captured ciphertexts
- history: not rewritten; two pull request descriptions edited once this
  plan's pull request is open

### Blocks

1. Fixtures: both greetings re-encoded, the guard test, every test, CI and
   UI-test expectation, the CHANGELOG bullet
2. Documents: placeholders in the examples, a Cube's role and `$SN` where a
   document is worked from

## Discussion

Seven decisions, settled by the owner at the TL;DR gate as recommended.

### D1. Placeholder serials: A

- **A (chosen).** Spare Cube `KEQ0000100`, its thermostats
  `MEQ0000101`–`MEQ0000106`, house Cube `KEQ0000200`.
  Nothing used them, so they cannot clash with `KEQ0000000`–`KEQ0000003`,
  `KEQ0000008` and `KEQ0000009`, which the same tests use as other Cubes.
- B. `KEQ0000000` for the fake Cube: the contract's example, but it clashes
  with those tests.

### D2. Room and device names: A

- **A (chosen).** Invented names, each of the byte length, umlaut count and
  sort position of the name it replaces (Analysis §2):
  the M line keeps its size, the UTF-8 cases stay,
  and a test that picks by order still picks the same room.
- B. `Room 1`–`Room 6`, ASCII: loses the UTF-8 cases and reorders the rooms.

### D3. The greeting's RF addresses: A, kept

- **A (chosen).** Kept.
  They are not on the contract's list, every radio packet carries them in
  clear, and without the serials they do not lead to the owner;
  replacing them would rewrite expected lines in about 40 test files.
- B. Replaced: a second, larger mechanical block.

### D4. The house Cube's serial: A, in this plan

- **A (chosen).** Treated as the spare Cube's; the documents a session works
  from name the Cube by its role and point to the env file for the serial.
- B. A plan of its own.

### D5. `docs/handover-session.md`: A, edited here

- **A (chosen).** The project contract (§3, "Documents move with the code")
  gives this file to the coordinating session;
  the owner's answer at the TL;DR gate is the go to redact its serials here.
  It is a redaction, not a session update.
- B. Left to the coordinating session, the serials on `main` until then.

### D6. Git history: A, not rewritten

- **A (chosen).** No rewrite.
  The repository is public; the spare Cube's serial has been in it since
  2026-11-03, in 65 commits and every release tag.
  A rewrite would force-push `main` and `alpha`, re-tag every release,
  break every clone and pull request,
  and GitHub would serve the old commits until its support purged them.
  A serial identifies a Cube and unlocks nothing:
  the portal's key is printed on the board and not derived from it
  (`docs/portal-protocol.md`, "Transport").
  This plan stops new copies; it does not unpublish old ones.
- B. `git filter-repo` and a purge request: the owner's action, at that cost.

### D7. Two pull request descriptions: A, edited

- **A (chosen).** The descriptions of metaworx/massimo#42 (spare Cube) and
  metaworx/massimo#9 (house Cube) get the placeholders, through the REST API,
  once this plan's pull request is open and under the signal that opens it.
  Their edit history keeps the old text until the owner deletes it.
- B. Left as they are.

## Analysis

The inventory, file by file and with every identifier masked, is
[ANALYSIS Redact](2026-11-07_09-25_ANALYSIS_Redact_real_identifiers_inventory.md).

### 1. Where the identifiers are

- **Encoded:** only in the two greetings,
  `internal/protocol/cube/testdata/cube-fw0113-greeting.txt` (CRLF)
  and `tools/ui-test/greeting.txt` (LF, with one window contact more).
  The H line carries the Cube's serial in plain text;
  the Cube's C line carries it again (bytes 8–17);
  each thermostat's C line carries its serial;
  the M line carries every room and device name and the thermostat serials.
- **Plain text:** the spare Cube's serial in 39 files (27 Go test files,
  `.github/workflows/ci.yml`, 6 files of `tools/ui-test/`, 5 documents),
  in lower case in 6 test files and as its digits behind an `XEQ` or `LEQ`
  proxy serial in 2;
  the house Cube's in 6 documents and, lower case, in `auth_test.go`.
- **Names:** 9 test lines name a greeting's room or device
  (`cube_test.go:513`, `backup_test.go:77,118,141,389`,
  `tools/ui-test/README.md:13`, `screenshots.py:14,251`, `test_rooms.py:39`).
  `cube_test.go`'s `metadataExample` and the documents use common room
  words of their own; they stay.

### 2. The mapping

Keyed by room id, which each thermostat has to itself, and by RF address,
which ties a C line to its M entry; both greetings get the same values.

| Room | Room name | Thermostat serial | Thermostat name |
|------|-----------|-------------------|-----------------|
| 1 | `Wattenmeer` | `MEQ0000101` | `Radiator Wattenmeer` |
| 2 | `Düne` | `MEQ0000102` | `Radiator Düne` |
| 3 | `Küste` | `MEQ0000103` | `Radiator Strandkorb 1` |
| 4 | `Fels 2` | `MEQ0000104` | `Radiator Fels 2` |
| 5 | `Fels 1` | `MEQ0000105` | `Radiator Fels 1` |
| 6 | `Meeresstrand` | `MEQ0000106` | `Radiator Meeresstrand` |

- The Cube: `KEQ0000100`, in the H line and its C line.
- The UI copy's window contact, room 5: `Fenster Fels 1`;
  its serial, `MEQ0000001`, is a placeholder already.
- Each new name has its old one's byte length,
  the umlauts sit in the same names,
  and both byte order and locale order are the old ones:
  rooms `Düne < Fels 1 < Fels 2 < Küste < Meeresstrand < Wattenmeer`,
  devices from `Fenster Fels 1` to `Radiator Wattenmeer`.

### 3. Rewriting the greetings

A one-off script, `GUIDELINES/temp/redact-greeting.py`, not committed:

- reads each greeting, splits its lines, keeps its line ends;
- decodes the H, M and C lines as `ParseHello`, `DecodeMetadata` and
  `DecodeConfig` lay them out;
- refuses unless each old name has its new name's byte length and each old
  serial ten bytes, so the new bytes go over the old at the same offsets;
- encodes base64 again (same length), writes to a temporary file and moves it
  over the greeting (`/AGENTS.md` §1.5);
- decodes the result once more and checks the mapping and that no old value
  is left.

The old values pass only through the script's memory:
they are read from the greeting, never typed into a script, a message or a
document.

### 4. The tests

- the serial in upper case → `KEQ0000100`, in lower case → `keq0000100`;
  `XEQ`, `leq` and `LEQ` before its digits → the same prefix before `0000100`;
- the house Cube's serial in `internal/server/auth_test.go` → `keq0000200`;
- the names at the nine lines of §1 → their new names,
  including the renamed `Heizung <room>` of `backup_test.go`;
- `TestRealCubeGreeting`'s comment, "(room and device names anonymised,
  everything else verbatim)", says which values are placeholders;
- the comment in `internal/portal/portalcrypto/portalcrypto_test.go` quoting
  the captured plaintext's first block names the serial by role:
  the ciphertexts are the spare Cube's, and stay;
- the guard, `TestGreetingsUsePlaceholders` in `internal/protocol/cube`,
  parses both greetings and fails on an H, M or C serial not of the form
  `[A-Z]EQ0000` and three digits.

### 5. The documents

- `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/proxy.md`:
  the examples use `KEQ0000100` and `KEQ0000200`, and `XEQ0000200` for the
  proxy serial.
- `docs/portal-protocol.md`: the captured discovery reply carries
  `KEQ0000100`, said so beside it; its RF address stays (D3).
- `docs/windows-test-2026-11-05.md`: the fake Cube's serial is the greeting's,
  so the record shows `KEQ0000100`, and says once that it is a placeholder
  since 2026-11-07.
- `docs/handover-session.md` (D5): the house Cube and the spare Cube by
  their role, their serials in the Pi's `/etc/massimo/env` and the checkout's
  `.env`; `-sn` with the house Cube's serial.
- `docs/handover-write-test.md`: `SN` set to the house Cube's serial once,
  the commands with `"$SN"` and `$SN`.
- `docs/write-test-2026-11-05.md`: the house Cube by its role,
  `-sn <serial>` in its backup command.

### 6. Risks

- An expectation missed: the test gate and `make ui-test` fail on it.
- A test matching a name by substring:
  `Fels 1` is in no other room's name, as the old name was in none.
- The guard reads `tools/ui-test/greeting.txt` from `internal/protocol/cube`,
  three directories up; a move of either file fails it loudly.
- The serials stay public in history and in the two descriptions' edit
  history (D6, D7).

## Implementation Plan

### Block 1: Fixtures

1. Write `GUIDELINES/temp/redact-greeting.py` (§3) and run it over both greetings.
2. Add `TestGreetingsUsePlaceholders` after `TestRealCubeGreeting`,
   and bring that test's comment and expectations to the new values.
3. Replace the serial in every form of §4 in the Go tests,
   `.github/workflows/ci.yml` and `tools/ui-test/`,
   the house Cube's serial in `auth_test.go`, and the names at the nine lines.
4. Add the CHANGELOG bullet under `[Unreleased]` → `Changed`.

**Verification:**

- a masked search of every tracked file outside `docs/`:
  neither real serial in any case or derived form,
  no old name but `metadataExample`'s coinciding words;
- `gofmt -l .` prints nothing;
  `go vet ./...`, and again with `GOOS=windows` and `GOOS=darwin`, exit 0;
- `MASSIMO_ENV_FILE=/dev/null go test ./...` passes,
  and with `-race` for `internal/server`, `internal/cubelan` and `internal/proxy`;
- `make cross` builds every platform;
- `make ui-test` passes.

Commit (Proposed commit message, Block 1).

### Block 2: Documents

1. Edit the documents of §5.

**Verification:**

- a masked search of every tracked file:
  neither real serial in any case or derived form;
- `MASSIMO_ENV_FILE=/dev/null go test ./internal/server -run OpenAPI` passes.

Commit (Proposed commit message, Block 2).

### After the blocks

Under the signal that opens this plan's pull request:
merge `origin/main`, push, open the pull request against `main`,
then D7's two descriptions, by `gh api -X PATCH …/pulls/<n> -F body=@<file>`.
Retirement is the owner's call.

## Proposed commit message

The plan's registration, with its analysis:

```
chore(aps): +Redact v1.0, +ANALYSIS Redact

The recorded greeting and the tests, CI, UI test and documents around it get their plan for placeholders in place of the real Cubes' serials and the recorded room and device names, with the inventory it works from.

File-revision-v1: GUIDELINES/wip/2026-11-07_09-35_AP_Redact_placeholders_for_real_identifiers.md::v1.0::2026-11-07::All::the first revision, D1 to D7 settled at the TL;DR gate
File-revision-v1: GUIDELINES/wip/2026-11-07_09-25_ANALYSIS_Redact_real_identifiers_inventory.md::v1::2026-11-07::All::the inventory of the TL;DR gate
Assisted-by: claude-code:claude-opus-5-5
```

Block 1:

```
test(fixtures): Placeholders for the recorded Cube's serials and names

The recorded greeting and its UI-test copy carried the spare Cube's serial, its six thermostats' serials and the room and device names of a real installation; they now carry placeholders, written over the old bytes so that every line keeps its length and stays valid, and every test, CI and UI-test expectation follows (AP Redact v1.0, Block 1).

Key changes:
- internal/protocol/cube/testdata/cube-fw0113-greeting.txt and tools/ui-test/greeting.txt: the Cube is KEQ0000100 and its thermostats MEQ0000101 to MEQ0000106 by room; rooms and devices are named after a coast, in the old names' byte lengths, umlauts and order.
- internal/protocol/cube: TestGreetingsUsePlaceholders fails when a greeting's H, M or C line carries a serial other than a placeholder.
- The Go tests, CI's fake-Cube variables and the UI test name the placeholders, in every case and derived form they used; the second real serial in a test becomes keq0000200.
- CHANGELOG.md: the bullet under Changed.

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```
docs(cubes): Placeholders for the real Cubes' serials

The documents named the spare Cube's and the house Cube's serials; the examples now use KEQ0000100 and KEQ0000200, and the documents a session works from name a Cube by its role and take its serial from the env file (AP Redact v1.0, Block 2).

Key changes:
- docs/api-v1.md, docs/api-v1-openapi.yaml, docs/proxy.md: the examples use the placeholders.
- docs/portal-protocol.md: the captured discovery reply carries the placeholder, and says so.
- docs/handover-session.md, docs/handover-write-test.md, docs/write-test-2026-11-05.md, docs/windows-test-2026-11-05.md: a Cube by its role, its serial in the env file, the commands with $SN.
- No CHANGELOG bullet: docs/ is exempt, and nothing shipped changes.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-07): the first revision,
  from the TL;DR gate of the same morning;
  D1 to D7 settled there as recommended.
