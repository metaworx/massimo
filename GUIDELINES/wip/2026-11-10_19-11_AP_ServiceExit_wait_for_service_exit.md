# AP ServiceExit v1.0: Wait for the service's exit

## TL;DR

A Windows re-install stops the service, then replaces `massimo.exe`; the
stop now waits until the service's process has exited, not only until it
reports STOPPED, and the rename over the binary rides out a short hold.

- `stopService`: STOPPED → STOPPED and the process gone (PID from
  `QueryServiceStatusEx`, opened before the stop, at most 30 s)
- `writeAtomic`: one rename → retried on Windows, at most 5 s, on access
  denied or a sharing violation; its temp file removed when it fails
- covers `install`, `install -remove` and `RestartService` (the update
  helper's restart)
- tests: a rename over a running child's image, the wait for its exit;
  Windows only, no administrator

### Blocks

1. The stop waits for the process, the rename retries; tests; CHANGELOG

## Discussion

Asked by the owner on 2026-11-10: the Windows service job of the tag
v0.13.0-alpha.1's CI run (job 129991499662) failed at the second
`massimo install` with `rename C:\Program Files\Massimo\.massimo.exe.tmp
C:\Program Files\Massimo\massimo.exe: Access is denied.`; the same job
passed on main's earlier commits.

**The owner's decisions** (`PLAN++ pr`, 2026-11-10, as recommended):

- **D1-A:** both, the wait for the process and the bounded retry of the
  rename. Not B, the wait alone, which a scanner's hold after the exit
  would still fail; not C, the retry alone, which leaves the gap in
  `-remove` and the restart.
- **D2** (the release of v0.13.0-alpha.1) is the owner's alone; nothing
  in this plan acts on it.

## Analysis

**The stop returns before the process has gone.** `stopService` sends
the stop and polls `Query()` every 300 ms until STOPPED. The service
reports STOPPED itself: x/sys's `serviceMain` reports it once `Execute`
returned, then `StartServiceCtrlDispatcher` returns, and only then does
the init goroutine of `cmd/massimo/service_windows.go` write `stopped` to
`service.log` and call `os.Exit(0)`. The install's copy starts in that
moment.

**Windows refuses a rename over a running image.** A probe on the
owner's Windows (cross-compiled, run through interop): `os.Rename` over
the .exe of a running child fails with `Access is denied.` (errno 5),
exactly the CI error; it is refused 6 to 15 ms after the child's last
output, until it has exited; after `Wait` 60 of 60 renames succeed. A
rename of a running .exe aside succeeds. In the CI run about 0.4 s lay
between the store closing and the refused rename, more than a plain exit
takes; the log cannot tell whether the process still ran or a scanner
held the file after it.

**The update helper is not hit by the rename.** On Windows `Swap` and
`Rollback` move the running binary aside (`swapRenaming`), which Windows
allows; `renameRetrying` in `internal/update/helper.go` serves only the
request and status files. Its restart, `RestartService`, uses the same
`stopService` and so starts the new process while the old one may still
be exiting; not known to fail, the wait covers it.

**`install -remove`** stops, deletes the service, runs netsh, then
removes `C:\Program Files\Massimo`: the same gap, so far hidden by netsh's
time.

**The wait.** `mgr.Service.Query()` returns the process ID from
`QueryServiceStatusEx`. The process is opened with `SYNCHRONIZE` before
the stop is sent, so that the ID cannot name another process by the time
it is waited for; after STOPPED, `WaitForSingleObject` waits up to 30 s
(serve is given 20, `stopWait`). A process that does not exit by then is
an error naming its ID. When the process cannot be opened, the stop
returns at STOPPED as before, and the rename's retry is left.

**The retry.** `writeAtomic` retries the rename on Windows, every 50 ms
for up to 5 s, while it fails with `ERROR_ACCESS_DENIED` or
`ERROR_SHARING_VIOLATION`, the errors of a file another process holds;
Go's own toolchain retries such renames on Windows in the same way. Any
other error, or one past the bound, ends it, and the temp file is
removed instead of being left beside the target.

**Tests.** `stopService` needs the service manager and an administrator;
CI's Windows service job, installing twice, is its test. The parts are
tested on any Windows: a copy of the test binary runs as a child that
holds its image; a plain rename over it must be refused first, so that
the test cannot pass without the condition; `writeAtomic` over it
succeeds once the child exits. The wait returns when a child exits, and
times out on one that keeps running.

## Implementation Plan

### Block 1: The stop waits, the rename retries

1. `internal/install/windows.go`: `stopService` opens the process of a
   running service before the stop and waits for it after STOPPED;
   `waitExit` and `openProcess` beside it.
2. `internal/install/install.go`: `writeAtomic` renames through
   `renameOver`, which retries while `renameHeld` says the target is held;
   the temp file goes when the rename fails. `renameHeld` is Windows'
   two errors in `windows.go`, false elsewhere (`windows_other.go`).
3. `internal/install/windows_exit_test.go`: the child process, the
   refused plain rename, `writeAtomic` over the child's image, `waitExit`
   returning and timing out.
4. `CHANGELOG.md`: a bullet under Fixed.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
`make cross`, `gofmt -l .` empty, `go vet` for linux, windows and darwin;
the install package's Windows tests through interop with `ProgramData`
set to a temporary directory; after the push, CI's Windows service job.

## Proposed commit message

```
fix(install): Wait for the service's process before replacing massimo.exe

Key changes:
- stopService opens the service's process before the stop and waits for
  it to exit after STOPPED, at most 30 s
- writeAtomic retries the rename on Windows for up to 5 s on access
  denied or a sharing violation, and removes its temp file on failure
- Windows tests: the rename over a running child's image, the wait

AP ServiceExit v1.0, Block 1

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision, D1-A.
