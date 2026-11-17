# NOTE TestInfrastructure v1.1: how Block 2 went

Where Block 2, Kit, went differently from the plan,
what the kit gained on the way, what was left and for which block,
and what verified it.

## What it made

- **`internal/testkit`:** `Eventually`, `Within`, `Never`, with `For` and `Every`;
  a wait of 5 s and a tick of 10 ms; `Scale`, from `MASSIMO_TEST_SCALE`.
- **The guard:** `TestOnlyTestsImportTheKit`.
- **The scale:** `make race` 2, `make test-armv6` 4.
- **`docs/testing.md`,** new.
- **Every test package on the kit:**
  the generic poll helpers gone (`eventually`, `waitFor`, `within`:
  six in four packages and a local one in a proxy test, 259 calls),
  and every deadline after which a test fails scaled.

## Different from the plan

1. **Who did it.**
   The session wrote the kit;
   three builders on Opus moved the tests at the same time,
   on disjoint paths (`internal/server`; `internal/cubelan` with the fake Cube;
   the other packages), without a commit;
   one second reader on Opus read the whole block;
   the commits were made from trees the session recorded.
2. **Four commits, one pull request:**
   the kit, then the server's tests, the Cube client's, the others'.
   The helpers are each package's own, so each commit removes its own,
   and each passes on its own.
3. **No `CONTRIBUTING.md`,** which the plan names and the contract lists:
   `docs/testing.md` is named in the README's Contributing list
   (`docs/fragments/_documents.md`) instead.
4. **The scale's rule, made exact for the builders:**
   a deadline after which a test fails takes the scale;
   a window after which it passes because nothing came, a sleep,
   a time the test asserts and a time it gives the product do not.
   Lengthening a failure deadline can only let a slow runner pass.
   `docs/testing.md` says so.
5. **No `Never` yet:**
   none of the sleeps before "nothing came" has an end to wait for (D17-A).
   They stay, unscaled, for Blocks 3 and 4:
   in `internal/server` 12 (backups, confirm, follow-up, resend,
   proxy retry, discovery), in `internal/cubelan` and the fake Cube 7,
   in proxy, engine and cubeio 4.
6. **Loops that keep their form,** each with its deadline scaled,
   where they do more than poll:
   the restore loop of `backups_api_test.go` (a tick of its own, a "failed"
   outcome), `cmd/massimo`'s Windows start wait and its reboot helper,
   `logging_test.go`'s attribute wait, the certs hook's child,
   `serve`'s proxy greeting, and the waits on lines D16-A keeps.
7. **The builders went past the brief in one way:**
   socket read deadlines and `context.WithTimeout` deadlines whose end only
   fails a test are scaled too.
   A few loopback `net.DialTimeout`s of 5 s are not; they return at once.
8. **The old ticks:**
   the server's waits take the kit's 10 ms where some polled every 20 or 50 ms;
   the certs and Windows helper waits keep theirs through `Every`.

## What the kit gained in the run

- **A wait never sleeps past its own end:**
  with a tick longer than what was left, it slept the whole tick.
- **The tick from 5 to 10 ms,**
  after the server's builder: many conditions ask the API or flush the store.
- **`Within` takes the options** the other waits take.
- **After the second reader:**
  the guard skips what the go tool skips (names starting with `.` or `_`,
  `testdata`), so that in the main checkout it no longer walks the other
  worktrees under `.claude/` (13 s, and a failure when one goes away
  meanwhile), and it catches a package under the kit too;
  a tick of 0 or less is the default tick;
  `docs/testing.md` names the two kinds that take no scale it had left out,
  and why.

## Tests made stricter

- **`TestClientRefusesZero`** waits for the client's connection:
  before, it went on without one, and checked only the refusal.
- **`TestNothingStartsAfterRun` and `TestHashGate`** had waits without a
  deadline; they have one.

## Seen, left for later blocks

For Block 8 (Holes):

- `TestReleaseAndRestartFreeTheCube` (`fakecube/oneclient_test.go:93`)
  passes where its read times out after 2 s.
- The proxy's "q: closes" (`proxy_test.go:93`, `:175`) passes on a read
  timeout as well as on a close; with the scale, after up to 40 s.
- The certs hook's pid wait (`dns01_test.go:412`) is longer than the child's
  own `sleep 20`, at scale 1 already.
- `link_linux_test.go:100`: its `what` says two dials, its condition counts one.
- Reads of `Ready()` and of a reply channel without a deadline
  (`client_test.go:101`, `:144`, `:468`, `:604`; `proxy_test.go:547`;
  `cubeio_test.go:232`; `cubeio/pairing_test.go:38`):
  a failure hangs until `go test`'s timeout.
- Fixed sleeps before a check of something that is to happen
  (`client_test.go:118`, `fakecube/apply_test.go:295`).

For Block 3 (Watcher):
`eventline_test.go:95`'s 50 ms sleep for the last rows' lines.

## Verification

- **Local, before the commits:**
  `gofmt` and `go vet` for linux, windows and darwin over the module;
  `go test ./...` (112 s at load 1);
  under `-race` with `MASSIMO_TEST_SCALE=2`,
  in two streams side by side at load 1 to 3:
  `internal/cubelan` with the fake Cube 20 runs;
  proxy, engine, announce, cubeio, backup, logfile and testkit 20 runs;
  `internal/serve` 3, `internal/server` 2, `internal/store` 1, all passed.
  The machine was quieter than the plan's "beside a busy machine";
  the builders' own runs, at load about 6, passed too.
- **The guard** failed on a planted import of the kit and of a package under
  it, and passed in 0.02 s from the main checkout, where it had taken 13 s.
- **After the merge of `main`:** the bundle, `go vet` for the three
  platforms, the documents, the tests of the server, the command line,
  the kit and `tools/identifiers`, and the guidelines check (12 of 12).
- **Left to CI** (the owner's rule, 2026-11-16):
  `make test-armv6` at scale 4, `make race` over the race packages,
  the Windows and macOS jobs.
- **Pull request #354.**
