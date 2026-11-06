# NOTE StoreUsers v1.0: how the run went

The run of Blocks 1 to 9 under `PLAN++ D3-B` and `EXEC++ D6-A` on 2026-11-06,
where the work went differently from the plan, a line each.

- **Block 1**, amended once:
  the state index is partial over the live states (NOTE StoreUsers v1.0:
  commands history), and `audit.command_id` is `ON DELETE SET NULL`,
  so that the retention can remove a command without stranding its audit row.
- **Block 2** is two commits: the users, roles, tokens and the bootstrap;
  then the audit, the lockout and the commands.
  A removed user's name is taken over by the next user of that name,
  as a Cube's serial is.
- **Block 3**:
  `PUT …/devices/{rf}` is registered with `rename` and admits a `structure`
  holder at the gate;
  the handler asks for `rename` with a name and `structure` with a room,
  so a role with either reaches it.
- **Block 4**:
  two values a route may be registered with that no role holds,
  `none` (the login, still behind the address lockout) and `any` (logout, me),
  where the plan said nothing about how those routes pass the gate.
- **Block 5**:
  `GET /api/v1/users` needs `users` or `guests`;
  a plain user reads itself through `GET /api/v1/me` and `GET /api/v1/users/{name}`.
  `GET /api/v1/roles` answers the capabilities too,
  so that a roles editor needs no `capabilities`.
- **Block 6**:
  the target comes from the path's values;
  a handler names it only where the path lacks it (the login's user,
  a created user, role or key).
  A capability refusal returns the principal, so the row names who was refused.
- **Block 7**:
  the parent row is written after authorization,
  so a refused request is no command; a replay leaves no audit row;
  whole-number settings beside the durations for `commands.keep_days`;
  the store's state update is order-independent,
  since a restore's run can end before its `202` is recorded (found by the race run).
- **Block 8**:
  `massimo user` reads the password from a terminal through `golang.org/x/term`
  or from standard input;
  the subcommands are the seven of docs/store.md section 6, no `remove`,
  which the API does.
- **Tests**:
  the server tests over a store give the fake Cube a row,
  as a served Cube has in production, so that the audit can name it;
  `make ui-test` was not run, since the UI did not change.
- **The documents**:
  docs/api-v1.md "Authentication" names the login and users routes two commits
  before they land, since the section describes the stage.
