# NOTE CiHardening v1.0: how the run went

Blocks 1 to 5 ran under the owner's `PLAN++` of 2026-11-08,
one signed commit each on the branch `claude/ci-hardening`,
two fixes of what the fuzzer found between them,
and main merged in before the push.
Where it went differently from the plan:

- **More identifiers than the TL;DR named.**
  The check found real-looking radio addresses beyond the greeting's:
  three in `docs/portal-protocol.md`'s hardware findings,
  two in the write-test reports, the Cube address beside `KMD1016788`,
  and four in test inputs (an ignore list, a delete, a temperature command).
  All got placeholders.
  The mapping is one range in the old values' order,
  `0a0101` to `0a010f` and `0a0201` to `0a0204`,
  rather than the plan's separate numbering for the Cube and the devices:
  no sorted expectation changed.
- **Forms the first rewrite missed.**
  The script found the plain text,
  Go byte lists and base64 of eight characters and more.
  The tests then showed the rest:
  four-character base64 of an address alone (`r:01,FNCO`),
  paths and log fields its guard against base64 runs had skipped,
  the original software's log lines with decimal addresses and signed Java bytes,
  a byte string, and spaced hex in a doc.
  The check gained one context for the future,
  a six-digit hex literal in Go code.
- **What the check leaves out and allows.**
  It skips the plans and NOTEs in `GUIDELINES/wip/`,
  which name what a change replaces and go when the plan retires,
  and its own files.
  The allowlist holds the three serials and five radio addresses of
  `Bouni/max-cube-protocol`, and eight values tests plainly made up (`000001`,
  `123456`, `0b0c0d`…), which also appear elsewhere as ordinary numbers.
- **Block 1 was rebuilt before the push.**
  The check scanned its own test file, whose samples it refuses;
  it passed while the file was untracked and failed once committed,
  which Block 2's gate caught.
  The fix went into Block 1's commit,
  the two fix commits after it replayed on the corrected one,
  Block 2's work parked in a temporary commit meanwhile (no stash,
  which the worktrees share).
- **Screenshots.**
  Only the two log images showed an address, the Cube's;
  they were taken again and the slideshow rebuilt.
  Four others differed through UI changes on main, outside this plan,
  and were left as they were.
- **The fuzzer found two faults**,
  each fixed in a commit of its own before Block 2's, its input kept as a seed:
  a metadata block without trailing bytes gained `0x01` when written back (`fix(cube)`),
  and `parseHash` accepted any argon2 version and an empty salt (`fix(store)`).
  A third failure was the target's own:
  it called `DecodeWeekProgram` on fewer than the 182 bytes its contract and its
  callers give it.
  After the fixes every target ran 30 s clean.
  The proxy's client commands are parsed by the protocol package,
  so their targets are there and `internal/proxy` has no fuzz file.
- **A header of two zero bytes** in a metadata block reads as none and encodes
  as the default;
  documented in `Encode`'s comment and left out of the round trip,
  rather than a field that would change how decoded metadata compares in tests.
- **Race.**
  `make race` took 80 s here.
  The session fixing `TestBackupAutomatic` (pull request 101,
  open when this was pushed) measured its race at 3 runs in 50,
  which the Race job will show until 101 is on main.
- **The contract changed too** (v1.3.1), beyond the plan's Block 5 list:
  its test gate still called `-race` merely recommended,
  and its rule on identifiers now names `tools/identifiers`.
- **The release of main.**
  `v0.11.0-alpha.1` was cut while the run went on.
  Merging main moved every bullet of this branch into the released section,
  without a conflict,
  and Block 1's amendment had changed a bullet released since;
  `CHANGELOG.md` was taken from main as released and the bullets put under `[Unreleased]`,
  Block 1's as a new one.
- **One shell command** was refused by the harness's safety check,
  which could not parse a heredoc with the Makefile's `$$`; nothing ran,
  and the same edits were made with the editor.
