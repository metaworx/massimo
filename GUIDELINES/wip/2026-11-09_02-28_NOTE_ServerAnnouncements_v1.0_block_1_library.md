# NOTE ServerAnnouncements v1.0: block 1 library

How Block 1 went, run on 2026-11-09 at the owner's `EXEC++ 1`,
and the decision taken at its gate.

## Decision

- **D5-A**, the recommended option, which the owner's `EXEC++ 1` took
  without naming another:
  the API's listener is opened in `Open`, so that `Instance.Addr()` holds
  the bound port as soon as `Open` returns
  (an embedder listening on `127.0.0.1:0` points its web view there before `Run`);
  `Instance.Close()` releases an instance that is not run,
  and does nothing once `Run` has returned.
  The plan's table had `net.Listen` in `Run`.

## Where the work went differently from the plan

1. **The intervals are pointers, not zero for the store's.**
   The Analysis said `Config` values that are zero take the store's;
   `-refresh 0` (never) and `-discover-interval 0` (only on request) are
   zeros that win over the store, so `Config.PollInterval`,
   `RefreshInterval` and `DiscoverInterval` are `*time.Duration`:
   nil takes the store's setting, else the default.
   The defaults, which `internal/server`'s settings API and serve's flags
   held as copies, are `server.DefaultPollInterval`,
   `DefaultRefreshInterval` and `DefaultDiscoverInterval`,
   used by the server, the package and the flags.
2. **Three helpers are exported, not kept in the command.**
   `massimo config` and `massimo tls` use them too:
   `serve.TLSMode` (the plan had `tlsMode` stay in the command),
   `serve.ListenHost`, and `serve.FileCubes(serial)`, which takes the
   serial that the package-level `selectedSN` gave it.
   `hstsDays` and the TLS variables' names stay in the command, with its flags.
3. **The tests moved with the code.**
   `cmd/massimo/serve_test.go`'s setup tests, all of `serve_seed_test.go`,
   `TestTLSSource` (but its `hstsDays` half, now `TestHSTSDays` in the command)
   and `TestServeTLS` are in `internal/serve/setup_test.go` and `tls_test.go`,
   their assertions unchanged:
   `serveSetup(ctx, st, addr, proxies, proxySerial, log)` is
   `setupCubes(ctx, st, Config{…}, log)`, `selectedSN` and `snExplicit`
   are `Config.Serial` and `Config.SerialGiven`.
   `TestDataDir`, the stop, reload and restart tests stay in the command.
   Beside the plan's `TestOpenRunsAgainstFakeCube` and `TestOpenWithoutStore`,
   `TestCloseWithoutRun` (D5-A) and `TestOpenIntervals` (item 1).
4. **The first log line is the same; some lines around it change places.**
   The command prepares the restart, reboot and update helpers before
   `Open`, being the server's options, so before the certificate source
   and the store; it wires the reloader, the settings file's watcher and
   SIGHUP between `Open` and `Run`, so before "api listening"; and at a
   stop "store closed" (by `Run`) comes before "stopped for a restart"
   (by the command).
   `Config.LogAttrs` carries the command's `sighup` into the first line;
   the package writes `restart` and `reboot` from the options.
5. **`Run` waits for the server whenever the HTTP server returns.**
   Before, the wait for the server's run, with its ten seconds of grace,
   came only when the context had ended;
   a listener that failed otherwise closed the store under a running server.
   `Run` now ends the run's context and waits in both cases.
6. **Two comments in `internal/server`**, which named `cmd/massimo` as the
   one wrapping the listener with the certificate, name `internal/serve`.
