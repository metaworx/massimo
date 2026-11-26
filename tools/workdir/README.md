# Work directories

`workdir.sh` makes, sweeps and keeps the work directories of the test suites
that make one at each run:
the UI suite's (`tools/ui-test/run.sh`, `massimo-ui-test.XXXXXX`)
and the emulator suite's (`tools/app-emulator/run.sh`, `massimo-app-emulator.XXXXXX`),
which both source it (AP EmulatorRuns, D3-B).
A run's work directory holds what it makes and needs no longer at its end:
the binaries, the servers' stores and logs,
and in the emulator suite the AVD.

It is a POSIX `sh` library,
since the UI suite's `run.sh` is `sh` and the emulator suite's `bash`.
It sets no shell option, and every name it uses starts with `workdir_`,
so that it takes none of the caller's.
It is safe under `set -eu` and `set -euo pipefail`,
and inside `$(…)` whether `set -e` holds there or not,
since `workdir_make` and the sweep test each step that can fail themselves;
`workdir_pid` and `workdir_done` leave a failure to the caller's `set -e`.

## The functions

- `work=$(workdir_make <name>)`:
  sweeps (below), then makes `<name>.XXXXXX` under `TMPDIR`
  (`/tmp` where it is not set),
  writes the caller's PID into its file `pid`
  (inside `$(…)`, `$$` is still the caller's),
  and under `KEEP=1` an empty file `kept`,
  from the start, so that a run killed before its cleanup keeps it;
  it prints the directory's path, and fails where it cannot make it.
- `workdir_pid <dir> <pid>`:
  adds a PID to `pid`, one per line,
  for a process that may outlive the run:
  the emulator suite adds the emulator's,
  so that an emulator left running by a killed run keeps its AVD.
- `workdir_sweep <name>`: removes the directories of the runs that are gone (below).
- `workdir_done <dir> <who>`, in the run's cleanup:
  removes the directory,
  or under `KEEP=1` keeps it and says so on standard error,
  `<who>: KEEP=1, the work directory is kept: <dir>`.

A name is letters, digits, `-` and `_`;
any other is refused,
so that a sweep's pattern matches its suite's directories alone.

## The sweep

A run that was killed, whose cleanup could not run,
leaves its work directory behind.
A later run of the same suite removes each `<name>.*` under `TMPDIR`
that is all of these:

- a directory, and the user's own;
- without `kept`;
- one none of whose PIDs in `pid` runs (`kill -0`),
  a line that is not a number passed over;
  a directory without `pid` is one whose run was killed as it began;
- older than a day (`find -mmin +1440`,
  since BSD's `find` rounds `-mtime`'s days up).

A directory that cannot be removed stays, and the run goes on.
So a run of another checkout keeps its own while it runs,
and so does an emulator it left running,
and another user's directories are never touched (AP CleanRuns, D3-A).

## KEEP

`KEEP=1` keeps the run's work directory, passed or failed,
with `kept` in it, which the later runs' sweeps leave alone,
and the run prints its path;
it is then removed by hand (AP CleanRuns, D4-A).
The emulator suite's holds its AVD, about 1.2 GB after a run.

## The tests

`workdir_test.go` runs each function in `sh`, and in `bash` where there is one,
under `set -eu` as the suites run,
against directories it plants in a `TMPDIR` of its own;
they are a part of `go test ./...`,
skipped on Windows and where there is no `sh`:

```sh
go test ./tools/workdir/
```
