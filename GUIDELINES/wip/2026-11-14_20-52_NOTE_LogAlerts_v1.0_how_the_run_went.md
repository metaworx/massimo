# NOTE LogAlerts v1.0: how the run went

Plan: `wip/2026-11-14_19-05_AP_LogAlerts_log_icon_by_unseen_problems.md`.
Run under the owner's `PLAN++ split`, after UpdateHistory in the same session.

## The pull request

Blocks 1 to 3 are #263, from main, not merged:

| Block | Commit |
|-------|--------|
| 1 | `feat(api/log-alerts): The log says which warnings and errors are unsettled` |
| 2 | `feat(api/log-alerts): A person marks the log's lines as seen` |
| 3 | `feat(ui/log-alerts): The log's button shows the worst unseen problem` |

## Where the run went differently from the plan

1. **What the log writes at warn and error.**
   Read from the code, the kinds are `disconnected`, `command` (retried, failed,
   dropped), `hold`, `backup`, `restore_done`, `proxy_refused`,
   `proxy_discarded`, `proxy_delete_refused`, `proxy_warning`, `update_result`,
   `log_rotated`, `login_failed`, `locked_out`, and `restart`,
   `reboot` and `update` at error.
   - A held link is a `disconnected` with `held_until`,
     settled by the next `connected`, as any disconnect is.
   - A proxy listener's failure and "listening", and the shared
     discovery port's troubles (#237), go to the text log only, so the
     log has nothing of theirs to settle.
   - `hold` and `locked_out` settle once they end.
   - `proxy_discarded` and `proxy_delete_refused` settle only by a mark.
2. **A device's flags.**
   A device's transmit error, link error and low battery are logged at info,
   inside its state's changes.
   Each counts as an alert at warn while the device has the flag now, at
   the newest line that turned it on.
   The device's current flags come from the Cube's live state, so a flag
   set weeks ago is not lost behind newer state lines.
3. **Limits.**
   An alerts request reads the newest 1000 warnings and errors and the
   newest 1000 events of each settling kind since the oldest of them.
   For a flagged device it reads that device's newest 200 state lines.
4. **Docs in Block 1.**
   `docs/api-v1.md` and the OpenAPI file came with Block 1:
   `TestOpenAPICoversRoutes` fails on a route without its entry.
5. **`log.seen` is checked.**
   `PUT /api/v1/me/preferences/log.seen` refuses a document of another form,
   as it does `notify`'s.
   The page reads the marks from the whole preference list, because the
   key alone answers 404 before the first mark, and the UI suite fails
   on any console error.
6. **The mark on ☰, and the active button.**
   - On a phone the log's button is in ☰, so ☰ carries the mark.
   - The open log's own button is blue; there the mark keeps its shape
     in the button's own colour.
7. **Two tests in `test_log.py`** held every request whose path holds `/log`;
   they now let `/log/alerts` through.
8. **The race run.**
   With the UI suite running beside it,
   `internal/server` under `-race` ran past the default 10-minute timeout.
   With `-timeout 30m` it passed but for `TestVirtualWallThermostatEvents`,
   which missed a stream event once and then passed 5 of 5 alone.
9. **Found, not changed.**
   - `docs/store.md` says the server writes no line at error, and
     `docs/api-v1.md` calls a restart's or a reboot's failure a warning.
     The code writes `restart`, `reboot` and `update` failures at error.
   - The log's level choice offers debug, info and warn, but not error.
