# NOTE StoreRaise v1.2: how block 4 went

Block 4, run by a sub-agent on the owner's `EXEC block 4`, 2026-11-16:
#339, not merged. Tested locally in the touched packages and UI test
files only; the rest was CI's.

## Where the run went differently

1. **A user's `integration`** stays an object (`uuid`, and `id`, `name`,
   `linked_to` for one of its users), not the bare uuid of "The display":
   AP IntegrationUsers' mark already uses the key and the page reads it.
   For the owner to confirm.
2. **`effective_user` and `real_user`** are objects (`name`,
   `display_name`, `integration`, `display`), not login names;
   `real_user.display` carries ", as <effective user>". For the owner to
   confirm.
3. **No log printout:** the command line has no log command; the
   server's event lines carry the chain instead.
4. **More log lines** than the plan's: the users routes' own lines (a user
   changed or removed, a key made, a token revoked) name an integration's
   user by the chain too.
5. **No login name** in the users list for an integration's user:
   `<uuid>:<id>` overflowed at 390 px and is never typed.
6. **Two Block 3 tests** adjusted: the downgrade's L4 test sets the
   reported name directly, since `PUT` no longer sets it; the audit's
   index test expects both indexes.
7. **The copy rule** was already Block 3's; a store test now covers a
   `PUT` followed by a new reported name.
8. **`display_name`** keeps its key and meaning, `/me` included, at the
   Home Assistant integration's word: it reads it, and needs no new key
   (its session, 2026-11-16; its title no longer names a user since its
   SetupPolish).

## Measured

- `go test` of `internal/store`, `internal/server`, `cmd/massimo`;
  `make ui`, `make ui-check`, `make api-types`; `gofmt`; `go vet` on
  three platforms; `check`, all 12.
- `test_integrations.py`, `test_users.py`, `test_log.py` in all four
  cases: 184 passed (the first run's 9 failures: two wrong assertions,
  and the new name row scrolling sideways at 390 px).
- CI on #339: 24 passed, 5 skipped, the UI jobs, Race, armv6, Windows and
  macOS included.
