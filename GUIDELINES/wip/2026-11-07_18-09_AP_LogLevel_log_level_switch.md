# AP LogLevel v1.0: log level switch

## TL;DR

Every command logs at Info only, so nothing logged at Debug ever shows.
One switch sets the level, read once at the start through one helper.

- `slog.NewTextHandler(w, nil)` ×6 → `newLogger(w)`,
  sharing one `slog.LevelVar`
- level: `-log-level` (global, or serve's) → `MASSIMO_LOG_LEVEL` → `info`;
  `debug`, `info`, `warn`, `error`, any case
- unknown value → an error at the start of a command that logs;
  `massimo config` shows the level, or `invalid: …`
- `env.Known` += `MASSIMO_LOG_LEVEL`
- services: command lines unchanged, the settings file reaches them;
  `packaging/env.template` gets the variable, commented out
- documents: `README.md`, `docs/store.md` §4, `packaging/README.md`;
  a CHANGELOG bullet under Added

### Blocks

1. Level switch: helper, flags, commands, `massimo config`, tests, documents, CHANGELOG

## Discussion

The owner answered the TL;DR gate on 2026-11-07 with `EXEC++`,
every decision as recommended:

- **D1-A.** Both a variable and a flag, the flag winning.
  Every service starts with `-env <file>`,
  so one line in that file reaches systemd, launchd, Windows and Homebrew;
  the flag is for a run in a terminal.
  The pattern of `-log` and `MASSIMO_LOG`.
- **D2-A.** Every command that builds a logger:
  `serve`, `watch`, `backup`, `restore`, `portal`, `store migrate`, `user`.
  `-log-level` is a global option before the command name, like `-data`,
  and `serve` takes it after its name too, as `-sn` works in both places.
- **D3-A.** No unit, plist or Windows service command line changes;
  the variable, commented out, in `packaging/env.template`,
  which `install` writes for Windows and macOS as well;
  a sentence in `packaging/README.md`.
  `docs/update-windows.md` stays as it is.
- **D4-A.** A restart changes the level; nothing changes it while running.
  The loggers share one `slog.LevelVar`,
  so a setting can follow without redoing this.
- **D5-A.** An unknown value is an error at the start,
  as an invalid `MASSIMO_HSTS` or `MASSIMO_TLS` is;
  `massimo config` shows `invalid: …` before a restart.

## Analysis

What the investigation found (the TL;DR gate of 2026-11-07):

- **The loggers.** `cmd/massimo` builds its loggers with
  `slog.New(slog.NewTextHandler(w, nil))`, always at Info:
  `serve.go` (the server's, also the store's, the proxy's and the Cube
  clients'), `watch.go`, `backup.go` (`connectCube` for backup and restore,
  and the restore's queue), `portal.go`, `store.go` (`store migrate`),
  `user.go`; `config.go` opens the store with a discarding one.
  Commands that pass no logger (`store info`, `store backup`, `tls`)
  get `slog.Default()`, also at Info, and stay so.
- **What logs at Debug today.** The proxy (a command passed to the Cube,
  a malformed line, a discovery request answered or not, a radio command
  not understood), the store (an event dropped), the server
  (a backup without the NTP servers).
  AP CubeLost (metaworx/massimo#91, not merged) adds
  `cube connection attempt failed` to `internal/cubelan`.
- **The environment.** `env.LoadFile` sets a variable of the file
  only where the environment has none, so a variable the global option sets
  wins over the file, as `-data` does over `MASSIMO_DATA` in the file.
  `env.Known` names every variable `massimo config` does not mark as a typo.
- **The services.** systemd (`massimo.service`), launchd
  (`ch.metaworx.massimo.plist`), the Windows service
  (`internal/install/windows_args.go`) and Homebrew (`massimo.rb.in`)
  all start `massimo -env <file> … serve …`.
  The Windows and macOS settings templates are `packaging/env.template`,
  rewritten for their log line.
- **Volume.** At `debug` the proxy logs every command a client passes and
  every discovery answer: a diagnosis setting, not a default.

## Implementation Plan

### Block 1: The level switch

1. `internal/env/env.go`: `LogLevel = "MASSIMO_LOG_LEVEL"`, in `Known`;
   `TestKnown` names it.
2. `cmd/massimo/logging.go`:
   - `parseLogLevel(s)`: `debug`, `info`, `warn`, `error` in any case,
     empty as `info`; anything else an error naming the four;
   - `logLevel`, one `slog.LevelVar` for every logger of the process;
   - `newLogger(w) (*slog.Logger, error)`: the level of `MASSIMO_LOG_LEVEL`,
     set on `logLevel`, a text handler on `w` at it.
3. `cmd/massimo/options.go`: the global `-log-level <level>` checks the value
   and sets `MASSIMO_LOG_LEVEL`, as `-data` sets `MASSIMO_DATA`;
   `main.go`'s usage names it.
4. `cmd/massimo/serve.go`: `-log-level`, checked and set the same way,
   so it wins over the variable; the logger from `newLogger`.
5. `watch.go`, `backup.go` (`connectCube`, the restore's queue), `portal.go`,
   `store.go` (`migrate`), `user.go`: their loggers from `newLogger`.
6. `cmd/massimo/config.go`: a `log level:` line, the level and where it
   came from, or `invalid: …`.
7. Tests, `cmd/massimo/logging_test.go`:
   - `parseLogLevel`: each name, upper case, empty, an unknown name;
   - `newLogger` at `debug` writes a Debug line, at `warn` no Info line,
     and an unknown value is an error;
   - the global option sets the variable and refuses an unknown value;
   - `serve -log-level debug` wins over `MASSIMO_LOG_LEVEL=error`:
     with the fake Cube and `-addr`, the proxy's Debug line
     `proxy: passing command to the cube` appears for a command sent
     through the proxy; and `serve -log-level verbose` fails at the start.
8. Documents: `README.md` (beside `-log` under "Server",
   and the variables that stay in the file under "Cube variables"),
   `docs/store.md` §4 (the table of what stays in `.env`),
   `packaging/env.template` (the variable, commented out),
   `packaging/README.md` (where the log is described);
   `CHANGELOG.md`, `### Added`.

**Verification:** the new tests pass, and the serve test fails without
step 4; `MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for
`internal/server`, `internal/cubelan`, `internal/proxy`), `make cross`,
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin,
`changelog.sh check`, the reflow check of the changed documents.
Then the branch merges origin/main, is pushed, and its pull request opened.

## Proposed commit message

- `feat(cli): A log level, -log-level or MASSIMO_LOG_LEVEL`,
  with this plan's first revision

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate;
  D1-A to D5-A as recommended, `EXEC++`.
