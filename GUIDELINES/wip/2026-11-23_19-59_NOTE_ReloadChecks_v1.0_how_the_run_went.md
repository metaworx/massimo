# NOTE ReloadChecks v1.0: how the run went

Where Blocks 1 to 5 went differently from the plan, what verified them,
and what the run found beside them.
The five blocks are one pull request, #406, a commit each,
with `main` merged in before it.

## Block 1, Known

1. **Literals:** `MASSIMO_RESTART` and `MASSIMO_SIGHUP` are literals in
   `env.Known`, as `MASSIMO_REBOOT` is; the new test reads them through
   `flagVars`, so the two spellings cannot drift.
2. **No docs changed:** the README and `docs/api-v1.md` already spoke of
   "every other setting"; the code did not match them.

## Block 2, Table

1. **The start parses again** after the table, rather than using what the
   table parsed; on the second reader's point, the errors stay checked,
   so that a check the table lacks still stops the start.
2. **`CheckDNS01(names, hook)`** for the plan's `CheckDNS01Hook`:
   its name checks differ from ACME's (wildcards allowed, another text).
3. **`server.SecretVariable`**, there being no `env.SecretVariable`;
   of the table's variables only `MASSIMO_TLS_KEY` is one.
   `hideSecrets` replaces its value by text, in one pass.
4. **The start's order changed:** a table refusal now comes before a log
   file that does not open, TLS and a Cube's ignore list before the Windows
   helpers and `tlsSource`'s own failures; a refused start no longer creates
   the log file, the self-signed pair or the store.
   A bad `_IGNORE` now reads "serve: MASSIMO_CUBE_…".
   The start loads the certificate pair three times, a few milliseconds.
5. **Beyond the plan, on the second readers' points:**
   - the texts of HSTS, restart, SIGHUP, the TLS mode and trusted proxies
     name their variable beside the flag, at their source, so the start says
     it too;
   - a test that each entry reads the variables it lists;
   - a removed setting among the cases;
   - `docs/api-v1.md` says that `MASSIMO_REBOOT` applies at once.
6. **`-addr`:** `setupCubes` also refuses a plain `MASSIMO_CUBE_IGNORE`
   through `env.HiddenList`, which the table does not check, `-addr` being
   for one run.

## Block 3, Guard

1. **The list of variables** is read from every file of the module that
   `massimo` links, by `go list -deps` for the three systems, on the second
   reader's point; a name formed with `env.Prefix` inside an argument is
   found.
2. **Serial,** since the Cube check reads the process environment,
   with parallel subtests; about 2 s.
3. **Skipped on Windows,** where `builtBinary` makes no `.exe`.
4. **No fragment:** the exemption is said in the commit.

## Block 4, Unreadable

1. **A missing file** keeps the reloader's own warning, with no
   `settings_read`: the next start takes a missing file as none.
2. **`unread`:** the next good read writes its line even where nothing
   changed, so that the warning settles once the file is right.
3. **`read_at` moves** on a failed read, as the card takes a
   `settings_read`'s time as the file's read.
4. **No secret:** a read error names the file and the line, never its
   content; a line too long too, on the second reader's point.

## Block 5, Card

1. **A refusal's `settings`** are the changed variables its check reads,
   not the one its text names; a file that does not parse has its error
   alone.
2. **The line where the refusals change,** a must-fix of the second reader:
   on the watcher's path, a check that reads the box (a hook made, a key put
   in place) could change the card while the sign kept warning.
3. **`apiReload`:** serve's reload of the API moved into the reloader, so
   that the tests run it.
4. **The overflow test** could not fail, its long path breaking at hyphens;
   a must-fix of the UI reader.
5. **`src/model/event.ts`** gained the event's level, outside
   `src/maintenance/`.

## Found, not changed

1. A name given twice in the settings file is never reloaded:
   `env.LoadFile` takes the second line's name as the environment's,
   and `massimo config` shows that wrong source.
2. A failed reload through the API neither rotates the log nor checkpoints
   the store, as SIGHUP does, though the README says the two do the same.
3. A line `=x` is accepted silently, and reported as an empty name changed
   at every read; a byte order mark makes its line's key unknown.
4. A `MASSIMO_LISTEN` that does not parse stops the start without
   "serve: " and without naming the variable.
5. The guard could run on Windows once `builtBinary` makes an `.exe`.
6. The card's head badge counts only pending settings, and the restart's
   question does not mention a refusal.
7. Two `settings_read` warnings in a row both stay alerts until a clean read.
8. `TestPerCubeValue` fails on Windows, as on `main`.
9. A plain `MASSIMO_CUBE_ADDRESS` with `MASSIMO_CUBE_PROXY` beside a Cube the
   store holds stops the start, from the store's state; and a
   `MASSIMO_ENV_FILE` line inside `.env` redirects the server.

## Verification

Each block: a builder; an independent second reader, two for Blocks 2 and 5,
whose points were fixed before the commit; faults planted for each new or
changed test, the readers' own included; `-race` at scale 2 for the touched
packages; `gofmt`, `go vet` for the three systems; the TLS and reload tests
also on Windows through interop.
After the merge of `main`: `go test ./...`, `make ui-check`, the API types,
the guidelines check.
Block 5's UI test files in the four cases, 224 passed.
The pull request's run green at its first attempt, in 13 minutes.
