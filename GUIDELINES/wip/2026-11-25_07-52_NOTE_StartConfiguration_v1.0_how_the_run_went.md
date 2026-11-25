# NOTE StartConfiguration v1.0: how the run went

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.
Run under the owner's `PLAN++` of 2026-11-25.
Each block: an Opus builder, an independent Opus second reader,
the builder's fixes, the session's verification and commit.

## The pull requests

Three, stacked, none merged:

| Pull request | Blocks | Base |
|--------------|--------|------|
| #425 | 1 to 3, the parser | `main` |
| #427 | 4 to 7, the reload and its signs | #425's branch |
| #430 | 8 to 11, the start and Windows | #427's branch |

#425's first CI run failed on `test_battery_words_in_the_problem_lists[390-de]`,
a click on a tile's problems badge, which passed on its retry
and lies outside the pull request; its second attempt passed.

## Where the run went differently from the plan

1. **Block 1, beyond the brief.**
   - `MASSIMO_ENV_FILE` is matched in any case, as Windows takes the name.
   - A line holding a NUL byte is refused as UTF-16:
     PowerShell 5.1's `>>` appends UTF-16LE without a mark to a UTF-8 file.
2. **Block 2.**
   - A name given twice is quoted only where it is a variable's name in
     upper case: a bare secret given twice, `c2VjcmV0==`, splits into a valid
     lower-case name, which the session's rule now leaves unquoted.
   - `GET /api/v1/config` does not carry the notes:
     D3-B's "config" is `massimo config`,
     and a name's lines reach System info through its source.
   - The web UI's log says the start's line as "settings file read at the
     start", a key in six languages.
3. **Block 3.**
   - Install's checks read the file leniently, by env's rules:
     a line that is not KEY=VALUE is skipped with a note.
     With env's strict read, a re-install with such a line would have
     rewritten the health drop-in to http and dropped the `:80` capability.
     Refusing every parse error was rejected,
     since a package upgrade runs install and would fail.
   - UTF-16 is refused before any write, also where nothing is copied.
   - `-tls` skips and keeps a byte order mark:
     after Block 1 its change was lost behind the file's first line.
4. **Block 4.**
   - The SIGHUP test also awaits the store's checkpoint before the stop.
   - Block 2's notes test scaled its wait twice; one line fixed here.
5. **Block 6.** A caller without `restart` reads `available: false`,
   with `blocked` naming the capability; the rest of the answer is the same.
6. **Block 8.**
   - The certificate code names `-data or MASSIMO_DATA` where it makes the
     data directory, under `-tls self-signed` and DNS-01.
   - A store refused keeps its own text.
7. **Block 10.**
   - On Windows a name in two cases is one name, its first line winning,
     and a reload lists a changed name in upper case,
     so that `massimo_api_token=` applies at once.
   - The guard ran on Windows through interop with a stand-in `go.exe`,
     as Windows here has no Go.
     CI's `windows-latest` job is its first run with real Go.
8. **Block 11.**
   - The plain Cube is tied by its address, `ProxyListener.CubeAddr`.
   - The listeners start before the runtimes,
     and a listener never starts on a Cube being removed.
   - A plain address equal to a stored row's is tied to that row at the
     start, the address alone too: it ends a double dial to a one-client Cube.
     `massimo config` shares the matching.
   - A second round filled a stored row's empty proxy from the plain
     `MASSIMO_CUBE_PROXY` at the session's word.
     The reader showed that it breaks `docs/store.md`'s precedence:
     the store wins, and a file line reaches a known row only by the web
     UI's button.
     The third round dropped it:
     the plain lines are that Cube's, compared with its row,
     and the plain proxy is a difference with its button.

## Open for the owner

1. **The plain proxy of a Cube the store has.**
   - A (built): the store's rule; the plain proxy is a difference with its
     button.
   - B: the row's empty proxy filled from the plain line at the tie.
2. **`reboot` reading `GET /api/v1/restart`,** so that a role with
   `reboot` alone sees the sentence in the reboot's question.
   - A: keep D34-A as it is; the docs say the sentence shows to holders of
     `restart`.
   - B: `restart`, `update` or `reboot`; the route helper would take a list.
3. **Install with a `-settings` file serve refuses** (a line without `=`):
   - A (built): copied, with a warning.
   - B: refused before any write, as UTF-16 is.

## Found outside the plan, not changed

- AP UpdateHistory: the Software card should read `refused`, not
  `available`, of `GET /api/v1/restart` (Block 6).
- `docs/store.md` §6's capability table has drifted from `docs/api-v1.md`'s.
- `packaging/deb/postinst`'s `settings()` does not skip a byte order mark
  (AP Packaging's).
- `install -tls-http :80x` names `-listen` for the wrong flag
  (`internal/install/tls.go`).
- On Windows the reloader's `settings()` sees only names that start with
  `MASSIMO_` exactly, so a setting written all in lower case is neither
  pending nor refused at a reload.
- On Windows a reload that changes both value and case keeps the old
  spelling until the next start.
- On Windows the test binary's directory is sometimes left behind
  (`cmd/massimo/update_store_test.go`).
- `fakecube.Server.Close` panics on a fake that never served.
- The plain Cube's loggers carry no serial learned later.
- A stored Cube whose address is written another way than the plain one
  (a hostname against an IP) is not tied at the start.
