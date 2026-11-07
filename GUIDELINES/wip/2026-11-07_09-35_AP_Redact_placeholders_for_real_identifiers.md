# AP Redact v1.2: placeholders for real identifiers

## TL;DR

The recorded greeting, its UI-test copy, and the tests, CI, documents and
screenshots around them carried the owner's real Cube serials.
They become placeholders, written over in the greetings' decoded bytes,
every expectation, document and screenshot following.

- spare Cube → `KEQ0000100`, in every case and derived form the tests use;
  its thermostats → `MEQ0000101`–`MEQ0000106`, by room;
  house Cube → `KEQ0000200`
- every line keeps its length and stays valid
- guard: a test fails on a greeting serial that is not a placeholder
- screenshots: retaken from the new greeting
- kept: the room and device names, invented for the recording (D2, v1.2),
  RF addresses, LAN details, the public protocol examples,
  the captured ciphertexts
- history: not rewritten; two pull request descriptions edited

v1.2 keeps the recorded names and amends Blocks 1 and 3 in place,
the pull request's branch rebuilt and force-pushed.

### Blocks

1. Fixtures: both greetings re-encoded, the guard test, every test, CI and
   UI-test expectation, the CHANGELOG bullet
2. Documents: placeholders in the examples, a Cube's role and `$SN` where a
   document is worked from
3. Screenshots: the set retaken with `make ui-screenshots`, the slideshow
   with `slideshow.sh`

## Discussion

Seven decisions, settled by the owner at the TL;DR gate as recommended,
an eighth at v1.0's completion gate;
D2 revised by the owner at v1.1's completion gate.

### D1. Placeholder serials: A

- **A (chosen).** Spare Cube `KEQ0000100`, its thermostats
  `MEQ0000101`–`MEQ0000106`, house Cube `KEQ0000200`.
  Nothing used them, so they cannot clash with `KEQ0000000`–`KEQ0000003`,
  `KEQ0000008` and `KEQ0000009`, which the same tests use as other Cubes.
- B. `KEQ0000000` for the fake Cube: the contract's example, but it clashes
  with those tests.

### D2. Room and device names: C, kept (v1.2)

- **C (chosen in v1.2).** Kept as recorded.
  The owner, at v1.1's completion gate: the recorded names were invented for
  the recording (`TestRealCubeGreeting` called them anonymised) and are
  generic, better suited than the replacements; Blocks 1 and 3 are amended
  in place to keep them.
- A (chosen in v1.0 and v1.1, replaced). Invented names of the old byte
  lengths, umlauts and sort order (a coast's: `Düne`, `Fels 1`, …).
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

### D8. The screenshots: A, retaken

- **A (chosen).** Block 3 retakes the set from the new greeting with
  `make ui-screenshots` and the slideshow with `slideshow.sh`, in one commit
  (Analysis §7).
  Every image changes, and this machine's fonts may draw other pixels than
  the set's last machine.
- B. Left: `main` would keep showing the serial in pictures.

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
  They were invented for the recording and stay (D2, v1.2).

### 2. The mapping

Keyed by room id, which each thermostat has to itself, and by RF address,
which ties a C line to its M entry; both greetings get the same values.

| Room | Thermostat serial |
|------|-------------------|
| 1 | `MEQ0000101` |
| 2 | `MEQ0000102` |
| 3 | `MEQ0000103` |
| 4 | `MEQ0000104` |
| 5 | `MEQ0000105` |
| 6 | `MEQ0000106` |

- The Cube: `KEQ0000100`, in the H line and its C line.
- The UI copy's window contact, room 5: its serial, `MEQ0000001`,
  is a placeholder already.
- The names stay as recorded (D2, v1.2);
  v1.0 and v1.1 replaced them with a coast's, of the same byte lengths,
  umlauts and order.

### 3. Rewriting the greetings

A one-off script, `GUIDELINES/temp/redact-greeting.py`, not committed:

- reads each greeting, splits its lines, keeps its line ends;
- decodes the H, M and C lines as `ParseHello`, `DecodeMetadata` and
  `DecodeConfig` lay them out;
- refuses unless each old serial has ten bytes,
  so the new bytes go over the old at the same offsets;
  in v1.2 it leaves the names as they are;
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
- the names at the nine lines of §1 stay (D2, v1.2);
- `TestRealCubeGreeting`'s comment, "(room and device names anonymised,
  everything else verbatim)", adds that the serials are placeholders;
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
- The guard reads `tools/ui-test/greeting.txt` from `internal/protocol/cube`,
  three directories up; a move of either file fails it loudly.
- The serials stay public in history and in the two descriptions' edit
  history (D6, D7).

### 7. The screenshots

Found during Blocks 1 and 2
([NOTE Redact v1.0](2026-11-07_09-51_NOTE_Redact_v1.0_how_the_run_went.md)):
the 28 images of `docs/screenshots/` are taken from `tools/ui-test/greeting.txt`
by `tools/ui-test/screenshots.py`, so the Cube choice and Settings → Cubes
show the spare Cube's serial (and, under v1.1, the names it had replaced).
A text search cannot see them.

- `make ui-screenshots` (`tools/ui-test/run.sh screenshots`) removes the old
  PNGs and takes the set again, byte for byte the same on one machine;
  it runs here with the main checkout's Playwright environment
  (`UI_TEST_PYTHON`), as `make ui-test` did.
- `tools/ui-test/slideshow.sh` builds `slideshow.gif` from the 1100 px light
  English shots; it needs ImageMagick, installed here (6.9).
- The set's file names do not change; a shot that cannot be taken fails the
  run. CI's Screenshots job takes the set again on the pull request and
  uploads it as an artifact, without comparing.
- `docs/images/windows-smartscreen-unknown-publisher.png`, the tree's only
  other image, shows no Cube.

## Implementation Plan

### Block 1: Fixtures

1. Write `GUIDELINES/temp/redact-greeting.py` (§3) and run it over both greetings.
2. Add `TestGreetingsUsePlaceholders` after `TestRealCubeGreeting`,
   and bring that test's comment and expectations to the new values.
3. Replace the serial in every form of §4 in the Go tests,
   `.github/workflows/ci.yml` and `tools/ui-test/`,
   and the house Cube's serial in `auth_test.go`;
   the names stay (v1.2).
4. Add the CHANGELOG bullet under `[Unreleased]` → `Changed`.

**Verification:**

- a masked search of every tracked file outside `docs/`:
  neither real serial in any case or derived form;
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

### Block 3: Screenshots

1. `make ui-screenshots`, with `UI_TEST_PYTHON` set to the main checkout's
   Playwright environment.
2. `tools/ui-test/slideshow.sh`.

**Verification:**

- the same 28 file names in `docs/screenshots/`, none missing, none new;
- the rooms shots and Settings → Cubes, looked at:
  `KEQ0000100` and the recorded names, no real serial;
- the slideshow's frames are the new shots.

Commit (Proposed commit message, Block 3).

### v1.2: Blocks 1 and 3 amended

The owner asked for the recorded names back and the commits amended,
with #78 open.
The branch is rebuilt from its base, `main` as of
`chore(aps): +NOTE Pages v1.0`:

1. the plan's v1.0 commit, as it was;
2. Block 1 again, the greetings' serials only (§3), the name edits of its
   test files undone, its comment, bullet and message without the names;
3. Block 2, the NOTE and v1.1, as they were;
4. this revision, v1.2;
5. Block 3 again, from the greeting of step 2;
6. the merge of `origin/main`, its bullet moved to `[Unreleased]` again;
7. the `style(docs)` commit, as it was.

**Verification:**

- against the branch before the rebuild, the tree differs only in the two
  greetings' M lines, the name lines of five test files, the comment, the
  bullet, the screenshots and this plan;
- the test gate as after the blocks, and `make ui-test`;
- then `git push --force-with-lease` to the pull request's branch,
  whose title and description lose the names.

### After the blocks

Under the signal that opens this plan's pull request:

1. Merge `origin/main`.
   Its release cut moved `[Unreleased]` into v0.10.0-alpha.1,
   so `CHANGELOG.md` conflicts: Block 1's bullet goes into the new
   `[Unreleased]` → `Changed`.
   `origin/main` adds no use of either serial (checked at v1.0's completion gate).
2. The test gate again: `go test ./...`, `make cross`, `make ui-test`,
   `gofmt` and `go vet` for the three platforms; the masked search.
3. Push the branch, open the pull request against `main`.
4. D7's two descriptions, by `gh api -X PATCH …/pulls/<n> -F body=@<file>`,
   the old serial in any form → its placeholder, nothing else changed.

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
test(fixtures): Placeholders for the recorded Cube's serials

The recorded greeting and its UI-test copy carried the spare Cube's serial and its six thermostats' serials; they now carry placeholders, written over the old bytes so that every line keeps its length and stays valid, and every test, CI and UI-test expectation follows (AP Redact v1.0, Block 1). The room and device names stay as recorded: they were invented for the recording.

Key changes:
- internal/protocol/cube/testdata/cube-fw0113-greeting.txt and tools/ui-test/greeting.txt: the Cube is KEQ0000100 and its thermostats MEQ0000101 to MEQ0000106 by room.
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

The revision v1.1:

```
chore(aps): +Redact v1.1

The plan gets Block 3, the screenshots, which were taken from the UI test's old greeting and show the old names and the spare Cube's serial (D8, settled at v1.0's completion gate).

File-revision-v1: GUIDELINES/wip/2026-11-07_09-35_AP_Redact_placeholders_for_real_identifiers.md::v1.1::2026-11-07::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::Block 3, the screenshots; the merge before the pull request
Assisted-by: claude-code:claude-opus-5-5
```

The revision v1.2:

```
chore(aps): +Redact v1.2

The recorded room and device names stay: they were invented for the recording and are generic, so D2 keeps them, at the owner's word after pull request 78 opened; Blocks 1 and 3 are amended in place and the branch rebuilt.

File-revision-v1: GUIDELINES/wip/2026-11-07_09-35_AP_Redact_placeholders_for_real_identifiers.md::v1.2::2026-11-07::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::D2 revised, the recorded names kept; Blocks 1 and 3 amended
Assisted-by: claude-code:claude-opus-5-5
```

Block 3:

```
docs(ui): Retake the screenshots from the placeholder greeting

The screenshot set and the slideshow were taken from the UI test's greeting while it carried the spare Cube's serial; they are taken again from the placeholder greeting (AP Redact v1.2, Block 3).

Key changes:
- docs/screenshots/*.png: retaken with make ui-screenshots, the same 28 files.
- docs/screenshots/slideshow.gif: rebuilt with tools/ui-test/slideshow.sh.
- No CHANGELOG bullet: docs/ is exempt, and nothing shipped changes.

Assisted-by: claude-code:claude-opus-5-5
```

The merge of `origin/main`, its conflict in `CHANGELOG.md` resolved:

```
merge: main, with the v0.10.0-alpha.1 release, into the placeholders branch

Block 1's CHANGELOG bullet moves from the released section into the new [Unreleased]; nothing else conflicted.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-07): the first revision,
  from the TL;DR gate of the same morning;
  D1 to D7 settled there as recommended.
- v1.1 (2026-11-07): Block 3, the screenshots (D8-A, settled at v1.0's
  completion gate after Blocks 1 and 2 had run);
  "After the blocks" spells out the merge of `origin/main` and its
  `CHANGELOG.md` conflict.
- v1.2 (2026-11-07): D2 revised to keep the recorded room and device names,
  invented for the recording, at the owner's word at v1.1's completion gate
  with #78 open; Blocks 1 and 3 amended in place, the branch rebuilt from
  its base and force-pushed ("v1.2: Blocks 1 and 3 amended").
