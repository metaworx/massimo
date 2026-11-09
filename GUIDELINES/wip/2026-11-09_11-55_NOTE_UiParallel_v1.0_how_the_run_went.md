# NOTE UiParallel v1.0: how the run went

Where Blocks 1 to 4 of
[AP UiParallel v1.0](2026-11-09_10-53_AP_UiParallel_parallel_ui_suite.md)
went differently from the plan, under the owner's `PLAN++` of 2026-11-09.

1. **The suite grew** from 436 runs to 484 (121 tests)
   between the measurements and the run, with `test_connect.py` and the log's tests.
2. **Block 3 fixed five tests, not three.**
   Two more failed in the first three runs without the trace,
   and passed on the retry:
   - `test_connect.py`, `test_server_page_token_from_bridge`
     (3 of 4 cases, then 1): after the sign-in it waited for `.room`,
     which stays drawn behind the dialog,
     and read the bridge's `setToken` calls before the session's;
     it now waits for the second call.
   - `test_discover.py`, `test_search_held_then_answered` (all four cases on one set):
     a server logs a Cube's `cube_found` once.
     Run serially, `test_cubes.py`'s search comes just before,
     and its line is among the log's last 20 events;
     a worker ran `test_opaque.py` in between,
     whose user changes pushed it out.
     Run in one process in that file order, the old test failed 4 of 4.
     It now chooses the kind `cube_found`,
     which the log asks the server for.
3. **One failure with the trace, left as is:**
   `test_users.py`, `test_new_user_name_checked_while_typed[1100-de]`
   failed once in Block 2's run, which still recorded the trace.
   It checks that nothing is said 500 ms after typing a name that is checked after 2 s,
   and a traced step at 1100 px takes about 840 ms.
   Without the trace it passed in all six runs; on the retry it would be slow again.
4. **Block 2's one-process check was a subset**
   (`UI_TEST_WORKERS=1`, the 16 runs of `test_discover.py` and the found-Cube test,
   and the file-order check of item 2),
   not the full suite, which takes 26 minutes with the trace and 11½ without;
   it is the same pytest call without `-n`.
5. **Block 1** changed the 503's text in `docs/api-v1-openapi.yaml` too,
   which the plan's file table did not list.
6. **`run.sh`**: shellcheck took the trap's `cleanup` for unreachable (SC2317, info)
   once the script ended in `exit 1`;
   silenced with its reason, as `GUIDELINES/shared/lang/shell/LINTING.md` asks.
7. **Measured**, four workers, 484 runs, on the 16-core box with other sessions' load
   (load 9 to 12): Block 3's three runs 4:42, 5:33 and 6:27, no failure;
   pinned to four CPUs, as CI's runner has, 5:46 at load 13, no failure.
8. **Found, not changed:**
   two runs of `run.sh` in one checkout share `tools/ui-test/results/`
   and pytest's cache, which the retry's `--last-failed` reads;
   ports apart are not enough, a checkout each is.
