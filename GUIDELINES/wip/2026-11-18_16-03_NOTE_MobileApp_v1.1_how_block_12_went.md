# NOTE MobileApp v1.1: how block 12 went

Block 12, Polish, run on 2026-11-18 under the owner's `EXEC++ as recommended`,
in pull request #366 on the branch `claude/mobile-polish`:
the two decisions its gate found, what was built,
and where the work went differently from the plan.

## Decisions

The gate numbered them D33 and D34;
the owner had them renumbered after Block 8's D32 to D37
(`2026-11-18_13-47_NOTE_MobileApp_v1.1_block_8_decisions.md`),
merged on `main` while the gate was open.

- **D38-A, what becomes a toast**:
  every outcome of the connect page's list and add form:
  a server added, known, saved or removed, one that does not answer,
  no answer at an address, a certificate not trusted, and the failures,
  in red.
  An address typed that is not one stays under its field,
  and a server's open settings keep their own line.
- **D39-A, discovery's name**:
  a server named by one of its addresses takes the name discovery hears
  for its node, saved; a name the user gave stays.
  "Named by the host" was read as the address's host with its port,
  as the page named such a server before (`192.168.1.20:8099`),
  so that its row shows it once.

## What was built

- `connect.html`: the toast's markup, a copy of `partials/toast.html`,
  since the app's copy of `ui/` is not assembled from partials;
  the list's message line `#servers-msg` is gone.
- `connect.js`: `say` shows an outcome in the toast;
  a probe that finds a server takes back the toast saying it does not answer;
  `takeHeardName` gives a saved server discovery's name,
  and a server added by its address takes it where discovery heard its node before;
  `showWhere` leaves out an address that is the row's name,
  in the saved rows and the found ones.
- `header.js` `renderTLS` hands the header part whether the page runs in the app
  (`appBridge()`), a signal `pinned` in `src/header/store.ts`;
  `Tls.tsx` then leaves out the self-signed sentence,
  and the sign-in's `#login-tls` stays hidden.
- `app/README.md`: the test's rig, `adb reverse` stalling on one port,
  the whole URL probing one connection, the WebView's DevTools socket,
  a phone's VPN that keeps LAN connections out.
- `.github/dependabot.yml`: the `gradle` entry for `/app/android`,
  monthly, one group;
  `GUIDELINES/project/CI.md` section 8 says so.
- Tests: `test_connect.py` checks the toasts, the toast taken back and the names
  (`test_connect_names`); `test_tls.py` the app's case with the fake bridge
  (`test_app_leaves_out_self_signed_notice`).
  Docs: `docs/app.md`, `docs/ui.md`.

## Where it went differently from the plan

- **The decision numbers**: D38 and D39 in place of the gate's D33 and D34.
- **Step 1** named four messages for the toast; D38-A sends every outcome there.
  The found rows show their address only where it differs from the name too.
- **Step 2** named `partials/login.html` and `login.js`; neither changed:
  `header.js` `renderTLS` already toggles the sign-in's line.
- **`GUIDELINES/project/CI.md`**, not in the steps, describes the Gradle entry,
  as its section 8 describes every other.
- **Three commits and two merges** where the plan's message list had two:
  "fix(app): The connect page's toasts and names, the notice left out",
  "docs(app): The phone test's rig and pitfalls in app/README.md",
  "chore(ci): Dependabot reads the app's Gradle files",
  the merge of `main` with StoreRaise's shortened bullet before the pull request,
  and "merge: main, with TestInfrastructure's Blocks 8 and 9 and the Cube
  choice's colour, into the polish" while it waited.
- **CI.md's version**: `main` had taken v1.61.0 for its own change
  (sections 2 and 3) while the branch had v1.61.0 for section 8;
  the second merge makes the two v1.62.0 and records it.
- **The macOS job** found no runner twice
  ("The job was not acquired by Runner of type hosted even after multiple
  attempts", GitHub's macOS arm64 capacity),
  in the first run and in its re-run of the failed jobs;
  every other job of that run passed.
  The second merge's push ran CI again.
- **The fragments**: the plan's two bullets, the first reworded to name the header
  too, under Fixed, since the connect page shipped in 0.16.0-alpha.1;
  the second under Added, naming the plugins and the one pull request.
- **The guidelines check before the Dependabot commit** could not pass:
  that commit is the one that records CI.md's v1.61.0.
  The commit was made with that as the check's one finding,
  and the check passed 12 of 12 on the committed tree before the push.

## Tests

- Locally: `test_connect.py` and `test_tls.py` in all four cases, 56 passed,
  and again on the second merge;
  `make ui-check`; `go test ./internal/server`; `make actionlint`;
  the guidelines check, 12 of 12
  (over the pull request's range after the second merge:
  read from the branch's upstream it also reads two squash merges of `main`'s).
- CI: the run of the second merge (44340592143) passed, 18 jobs:
  the whole UI suite in both widths, the race run, the cross builds,
  the Android app, macOS and Windows, "All tests passed".
  The first run (44331040494) passed 16 jobs, all but macOS (above).
- On the phone: the owner's, at the next phone test.
