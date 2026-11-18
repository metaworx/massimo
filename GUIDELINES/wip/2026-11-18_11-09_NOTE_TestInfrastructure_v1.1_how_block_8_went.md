# NOTE TestInfrastructure v1.1: how block 8 went

Where Block 8, Holes, went differently from the plan,
what it left, and what verified it.

## What it made

- **Every hole on the list,** the analysis's section 7 and what Blocks 2 to 7
  set aside, made to check what it says, or found not to be one:
  each corrected test failed on a fault planted for it,
  and where the old test could run with the same fault, it passed.
- **In the Go tests:**
  waits for the client's `Ready` and for a reply with a deadline,
  where a client that never greets hung until `go test`'s timeout;
  "nothing written" with an end (a later line through the proxy, the next `Send`,
  an `f:` after a refused `t:`);
  counts that wait until the watch holds the last event sent;
  exact bursts and exact line counts per row;
  `reportAndSettle` gone, its callers on `settleState`;
  the backups' looks waited for, where a 300 ms sleep stood.
- **`testkit.BindOrSkip`** (D9-A): a test that binds a second loopback address
  skips only where it does not exist, and fails on any other bind error;
  the server's and the Cube client's 21 sites use it.
- **In the UI tests:** `held_poll`, a hold of the page's poll that fails where
  something starts it again; requests counted where a fixed sleep stood for
  "nothing more"; the log's range exact; a command's detail by its own number.
- **`docs/testing.md` and `docs/ui-testing.md` (v1.3.0), `tools/ui-test/README.md`.**

## Different from the plan

1. **Who did it.**
   Two builders on Opus, one for the Go tests (with a sub-agent of its own for
   the server's), one for the UI tests; one second reader for each half;
   the builders made their readers' points;
   the session made the commits.
2. **Three commits, one pull request:**
   the server's tests with the kit's `BindOrSkip`,
   the Cube client's and the smaller packages',
   the UI tests'.
3. **The two tests for the owner are unchanged,** read only:
   `TestDisagreeResendsMassimosSetpoint` and
   `test_a_jump_down_stays_where_it_went`; the completion gate says what each
   locks in.
4. **Not every hole could be shown in the old test:**
   the backup button's lone POST shows only on a slow runner,
   and the planted fault failed the old test and the new alike;
   the reader's plant of a second POST sent at once is caught by the new count
   alone.
5. **F7, the log's range:** its runs, old and new, also switched the log's
   isolation off; with isolation intact, no server line can reach the log.
6. **Found on the way, fixed:**
   `backup_test.go` waited on `Ready` without a deadline too;
   the guest's preferences test read the owner's preferences;
   `TestClientWaitsForItsAddress` could not fail,
   since a dial without an address never reaches the fake's port;
   `TestABusyCube`'s "two dials" was right, the NOTE of Block 2 misread it,
   but it never waited for the second.
7. **Not a hole:** `ask` returning nil in the announcer's tests
   (every positive ask checks it), `TestProxyRefusesToDeleteHiddenDevices`,
   `test_log_server_lines`, and logfile's rotation wait in what it detects.

## Left

- `TestTimeZoneName` skips on any `os.Symlink` error;
  Linux always has symbolic links (D9-A).
- The second binds of 127.0.0.3 at 127.0.0.2's port now fail on "address in
  use", which comes from the moment, not the platform;
  on Linux the risk is near nil, on Windows unknown; a retry would cover it.
- Bare channel receives in `cubelan/inclusion_test.go` and the event counts'
  drop test; a 100 ms sleep before `StartPairing` in the Cube I/O's pairing test;
  a 1 s upper bound in `TestWaitingSendEndsWithItsContext`.
- The Windows job of the pull request is the first run of `BindOrSkip` there:
  before, a bind failure on Windows was a silent skip.

## Verification

- **The builders:** each corrected test with its planted fault, old and new,
  the UI's in the four cases and both passes;
  the changed Go tests under `-race` at scale 2, 20 times;
  the changed UI files in the four cases, and with the page's poll at 300 ms.
- **The readers:** the Go reader ran nine plants again, new failing and old
  passing as recorded, and the changed tests 10 times under `-race`;
  the UI reader ran the plants and the old tests' copies
  and found the drawings' "then nothing" unchecked
  (its plant passed 4 of 4; now it fails).
- **The session, before the commits:**
  `gofmt`, and `go vet` for linux, windows and darwin;
  `go test ./...` (113 s at load 0.6 to 1.1);
  under `-race` at scale 2: the kit, the Cube client, the fake Cube, the proxy,
  the Cube I/O, backup, certs and logfile 3 runs, `internal/server` once;
  the changed UI files in 1100-de, 96 passed; all passed;
  the first two commits' trees vetted on their own.
- **After the merge of `main`,** which brought the tile notices of #355:
  the bundle, `go vet` of the server, the changed UI files and `test_rooms.py`
  in 390-en and 1100-de (218 passed), the guidelines check (12 of 12).
- **Left to CI:** `make test-armv6`, `make race`, the Windows and macOS jobs,
  the UI's whole matrix.
- **Pull request #361.**
