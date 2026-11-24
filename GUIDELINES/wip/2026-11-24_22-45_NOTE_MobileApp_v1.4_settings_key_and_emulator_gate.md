# NOTE MobileApp v1.4: settings key and emulator gate

D85 to D87, the owner's words at the completion gate of v1.4's run,
built on 2026-11-24 as one pull request,
metaworx/massimo#418 (`claude/mobile-settings-key`), of three commits:
"fix(api): The settings file's key is not revoked under a user's tokens either",
"docs(contract): The test gate names the Android emulator suite"
and "test(app): The emulator suite keeps the lock screen away from the enrollment",
with `main` merged in without a conflict;
merged as "merge: MobileApp, D86-A, D87-A: The settings file's key kept,
the emulator in the test gate (#418)".
It was built by a sub-agent from a brief;
the session read the diff itself,
without a second reader for so small a fix.

## The decisions

- **D85-A, KVM for the user**, the owner's word at the completion gate
  on 2026-11-24: "added".
  The owner put the user into the group `kvm` themselves;
  a process started before that reaches `/dev/kvm` through `sg kvm -c '…'`,
  without a new login.
  The owner also said:
  "Reboot or session restart needs to wait on other sessions still running".
  Nothing was rebooted.
- **D86-A, the settings file's key**: "small fix here".
  `DELETE /api/v1/users/{name}/tokens/{id}` answers `409` for the key
  that is the settings file's `MASSIMO_API_TOKEN`,
  with the words `DELETE /api/v1/me/token` has answered since Block 16;
  `GET …/tokens` marks it `"from_settings": true`;
  and its row, under My profile and in System settings → Users,
  says "from the settings file: change or remove it there" in place of Revoke,
  in all six languages.
  The server finds the key by the hash of the token it holds from the file,
  which the store keeps unique, and not by its label, which any key may carry;
  no migration.
  A revoked key keeps its row and its hash, and `store.Bootstrap`,
  which looks at live keys alone, then failed on the hash at every start
  ("store: bootstrap failed") until the file's token changed:
  that way to it is closed, and Bootstrap stays as it is.
- **D87-A, the emulator suite in the test gate**:
  "and install the required emulator".
  The contract, v1.19.0, names `make app-emulator-test`
  for a change under `app/` or to what of the server the app drives,
  where KVM is open to the user;
  CI's job Android emulator runs it in any case.
  `tools/app-emulator/README.md` says how the user joins the group `kvm`,
  and that `sg kvm -c` runs the target until a new login.
  The emulator 37.3.10 and the image android-34, google_apis, x86_64,
  revision 14, were installed already, for Block 18.

## How it ran

- **Seen failing first**:
  `TestSettingsFileKey`, with no key marked before the fix,
  and with the mark alone, the key revoked by itself with `200`;
  `test_users.py::test_settings_file_key_has_no_revoke`,
  in the four cases, a Revoke on the row.
- **Checked on the branch**, each exit 0:
  `make ui`;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`,
  `TestOpenAPICoversRoutes` among them;
  `gofmt -l .` and `go vet` for linux, windows and darwin;
  `make ui-check`; `make api-types`;
  `test_users.py` and `test_settings.py` in the four cases, 88 passed;
  `go test ./tools/identifiers/... ./tools/apitypes/... ./tools/app-emulator/...`;
  `shellcheck -x` on the suite's scripts; `sync-docs.sh`;
  `commit-msg.sh` and `changelog.sh check --staged --message`
  on each commit's state.
  `GUIDELINES/shared/tools/check` found only the contract's version
  not yet recorded, which its commit's `File-revision-v1` trailer records.
- **Checked on the merged tree**, each exit 0:
  `make ui`;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/... ./internal/store/...`;
  `gofmt`;
  `commit-msg.sh --history` over the four commits;
  `GUIDELINES/shared/tools/check`, its 12 checks.
- **The suite's first local runs**, five,
  each `sg kvm -c '… make app-emulator-test'` with JDK 17:
  1. `run.sh` as on `main`, 543 s:
     the enrollment failed.
     The lock screen still showed as `run.sh` set the PIN,
     and `EnrollTest` drove Settings under it,
     UiAutomator finding nothing of Settings for four minutes.
  2. With a first unlock of about ten seconds, 153 s:
     the lock screen stayed, and the run stopped before the PIN.
  3. With 120 s for it, 717 s:
     the lock screen went before the PIN, showed again after it,
     and the enrollment failed.
     The check read the whole `dumpsys window`,
     whose dump opens with the last ANR's copy of the state,
     which said it was away.
  4. With a guard while Gradle runs, 296 s,
     stopped by the session once the enrollment had passed:
     the guard read the same copy, which now said the lock screen showed,
     and kept typing the PIN.
  5. The guard reading the window manager's policy and the activities'
     keyguard state alone, 1319 s:
     the lock screen taken away before the PIN,
     shown again 40 s after it and taken away by the guard within 27 s;
     the enrollment passed;
     "the suite: 14 tests, 0 skipped, 2 failed;
     0 of the 14 the sources declare not run".
     The two failures were time limits,
     the host's load average about 21 on 16 cores
     while another worktree's UI suite ran:
     a server's probe without an answer within four seconds
     (`TapTest.backFromTheListLeavesTheAppAndTheServersPageShowsItsCard`),
     and the connect page's call without an answer within 59 s
     (`ConnectPageTest.theConnectPageMayNotAskForAToken`).
     The lock screen did not show during the suite.
- **Unlike CI**: the builds ended 31 to 66 s after the emulator's start
  and the boot 102 to 202 s after it,
  where CI's had booted by the builds' end, 223 s after its start;
  SystemUI's keyguard service timed out and SystemUI had ANRs;
  and the suite's Gradle run took 16 minutes, where CI's takes about 4.
- **CI** on the pull request:
  the job Android emulator passed with the guard,
  and every job but one at the first try;
  UI test (390-de) failed
  `test_battery_words.py::test_battery_words_in_the_problem_lists`
  once, which passed on its retry, the change touching nothing of it;
  the failed jobs run again passed.
- **Left running**: nothing, no emulator and no server;
  every adb command named `emulator-5584`, and adb knew no other device.

## Departures

- **A third commit**,
  "test(app): The emulator suite keeps the lock screen away from the enrollment",
  beyond D86-A and D87-A, after the suite's first local run:
  `run.sh` reads whether the lock screen shows
  from the window manager's policy and the activities' keyguard state,
  takes it away, with the PIN where it is set,
  before the PIN, after it and before each Gradle run,
  and keeps it away while Gradle runs with a guard,
  which writes `results/keyguard.log`;
  the README says so.
  None of its paths is shipped, so it carries no CHANGELOG bullet.
- **The UI test does not revoke the suite's token**:
  the server's refusal is `TestSettingsFileKey`'s,
  so that a regression there does not end the suite's token
  for every test after it.
- **`docs/store.md`**, beyond the documents the brief named:
  its bootstrap section says that the API does not revoke the file's key.

## Not done here

- **D89, open, asked at the next gate**:
  removing the user that holds the settings file's key
  (`DELETE /api/v1/users/{name}`, an owner while another enabled owner stays)
  revokes the key with that user's tokens,
  and Bootstrap then fails at every start, its hash being taken,
  until the file's token changes.
- **A store whose key was revoked before this change**
  keeps that error at each start until the file's token changes,
  Bootstrap staying as it is.
- **A local run of the whole suite without a failure**:
  the fifth run passed 12 of the 14 on a loaded host;
  CI's job Android emulator runs it on the pull request.
