# AP StoreHygiene v1.0: Help without a store

## TL;DR

`massimo user` opens the store before it looks at its arguments,
so its help creates one, with `node.key` and `opaque.key`,
and `opaque.key` is not in `.gitignore`;
the commands check their arguments first and open the store last.

- `.gitignore`: `node.key` → `node.key`, `opaque.key`
- `runUser`: store opened first → after the subcommand's flags and arguments
- `user tokens -h` and the other commands without flags → the usage,
  not a user named `-h`
- `store migrate -h` creates a store, `store backup -h` writes a file `-h` → the usage
- test: every help and a wrong argument in an empty directory → still empty

### Blocks

1. Ignore: `opaque.key` in `.gitignore`
2. Help: `massimo user` and `massimo store` check their arguments before the store;
   the test; the bullet

## Discussion

Two hygiene issues found on 2026-11-09 and handed to this session by the owner:
`opaque.key`, the server's OPAQUE private key, is not ignored,
and `MASSIMO_ENV_FILE=/dev/null massimo user add -h` creates a store in the
current directory before it prints the usage.

**The owner's decisions:**
`PLAN++` (2026-11-09) on the TL;DR gate, naming no option:
the recommended ones.

- **D1-A:** each `user` command checks its flags and arguments,
  then opens the store; help, a flag error and a missing name leave nothing.
- **D2-A:** `massimo store`, which has the same fault, is fixed in Block 2.

**Names chosen here**, routine:
the plan `StoreHygiene`, the branch `claude/store-hygiene`,
the test `TestHelpCreatesNoStore`.

**Not in this plan:**
the exit status of a command's `-h`, which stays 1 with "flag: help requested"
as `serve -h` has it, `user -h` and `store -h` exiting 0;
the other commands, none of which opens a store (`serve -h` was checked).

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-09.
Nothing reaches a Cube or a box.

## Analysis

### What leaves files behind

Measured on 2026-11-09 with a fresh build, each command in an empty temporary directory,
`MASSIMO_ENV_FILE=/dev/null` and `MASSIMO_DATA` empty:

| Command | Exit | Left behind |
|---|---|---|
| `user -h` | 0 | `massimo.db`, `node.key`, `opaque.key` |
| `user add -h` | 1 | the same |
| `user tokens -h` | 1 | the same, after looking up a user named `-h` |
| `store migrate -h` | 0 | the same |
| `serve -h` | 1 | nothing |

`store.Open` creates the database, runs every migration
and writes both keys when they are missing (`internal/store`);
`store.OpenReadOnly` creates nothing.

### `massimo user` (`cmd/massimo/user.go`)

`runUser` opens the store before its `switch`,
so even `help`, `-h` and an unknown command create one.
`add`, `passwd` and `token` parse a flag set (`userCmd.flags`),
which answers `-h` with the usage and `flag.ErrHelp`, after the store is open;
`list`, `disable`, `enable`, `tokens` and `revoke` have no flag set,
so `-h` is taken for a name.

The fix: `userCmd` opens the store on its `open()`,
which each command calls once its flags and arguments are good;
`runUser` closes it when one was opened.
The commands without flags get an empty flag set through `userCmd.flags`,
so `-h` prints the usage and an unknown flag is refused, as for the others.
`revoke` parses the token id before it opens the store.
`add` opens the store before it asks for the password,
so a store that cannot be opened (the wrong user, say) fails before the typing.

### `massimo store` (`cmd/massimo/store.go`)

`migrate` ignores its arguments and opens the store, creating it;
`backup -h` takes `-h` for the file and, with a store present, writes it.
The fix: each command parses an empty flag set,
so `-h` prints the usage and a flag is refused;
`migrate` and `info` refuse an argument, `backup` takes exactly one;
`help`, `-h` and `--help` print the usage, as `user` has them.

## Implementation Plan

On a branch `claude/store-hygiene` from `main` after the register commit,
one signed commit per block, one pull request.

### Block 1: Ignore

1. `.gitignore`: `opaque.key` under "serve's store", after `node.key`;
   the comment names both keys.

No CHANGELOG bullet: `.gitignore` is not a shipped path.

**Verification:** `git check-ignore -v opaque.key` names the new line;
`git status --porcelain` shows `.gitignore` alone.

### Block 2: Help

1. `cmd/massimo/user.go`: `userCmd.open()` and `close()`;
   `runUser` dispatches without a store;
   every command parses its flags (an empty set where it has none),
   checks its arguments, then calls `open()`.
2. `cmd/massimo/store.go`: `help`, `-h`, `--help`;
   an empty flag set per command; `migrate` and `info` take no argument.
3. `cmd/massimo/user_test.go`: `TestHelpCreatesNoStore`,
   in an empty directory as the current one with `MASSIMO_DATA` empty:
   every `user` and `store` help, and a wrong argument to each command,
   leaves the directory empty; a help returns nil or `flag.ErrHelp`,
   a wrong argument an error.
4. `CHANGELOG.md`, Fixed:

   ```
   - `massimo user` and `massimo store` print their help, or refuse wrong
     arguments, without creating a store in the data directory first.
   ```

**Verification:** the test fails before the fix and passes after;
`MASSIMO_ENV_FILE=/dev/null go test ./...`; `gofmt -l .` prints nothing;
`go vet ./...` for linux, windows and darwin; `make cross`;
the commands of the Analysis' table, run again, leave nothing;
`check --commit-range origin/main..HEAD` of the guidelines.

## Proposed commit messages

1. `chore(gitignore): Ignore serve's OPAQUE key, opaque.key`:
   the store writes it beside `node.key` in its data directory,
   the current one by default, so a `git add -A` in a checkout would commit
   the server's OPAQUE private key; no CHANGELOG bullet.
2. `fix(cli): Help of massimo user and store creates no store`:
   the commands check their arguments, then open the store;
   `store migrate -h` and `store backup -h`; the test; the Fixed bullet.

Each cites `AP StoreHygiene v1.0, Block <n>` and ends with
`Assisted-by: claude-code:claude-opus-5-5`.

## Change History

- **v1.0** (2026-11-09): the first revision, by the owner's `PLAN++` with D1-A and D2-A.
