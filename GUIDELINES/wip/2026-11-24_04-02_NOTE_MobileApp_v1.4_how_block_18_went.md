# NOTE MobileApp v1.4: how block 18 went

Block 18, the emulator (D71-A),
run on 2026-11-24 as one pull request,
metaworx/massimo#411 (`claude/mobile-emulator`), of two commits:
"test(app): The app's flows on an emulator, locally and in CI" (steps 1 to 7),
and "test(app): The emulator suite counts what each run ran,
and CI keeps a pass's results", added after CI's first run;
merged as "merge: MobileApp, Block 18: The app's flows on an emulator,
locally and in CI (#411)".
It was built by a sub-agent from a brief and read by a second reader;
a second builder did the rework, the first one's context being nearly full.

## How it ran

- **The suite could not run here.**
  The development host's `/dev/kvm` is `root:kvm`, mode 660,
  and the user is not in the group;
  D85, the owner's word on opening it to the user,
  asked at the block's start, is still open,
  so `make app-emulator-test` stops at once with its KVM message.
  The emulator 37.3.10 and the system image android-34, google_apis, x86_64,
  revision 14, were installed in the host's SDK for the build (about 1.9 GB).
- **Checked here**, each exit 0:
  `make ui`;
  the Gradle tasks, `:app:assembleFossDebugAndroidTest` among them;
  `go test ./tools/app-emulator/... ./packaging/`;
  `go test ./tools/identifiers/...`;
  `gofmt -l .` and `go vet` for linux, windows and darwin;
  `shellcheck -x`; `make actionlint`;
  `GUIDELINES/shared/tools/check`;
  `commit-msg.sh` and the changelog check on each commit.
  Beside them:
  the servers' start-up on scratch ports;
  the suite's page scripts in Chromium against a real server,
  the UI suite's fake bridge in the app's place;
  `run.sh`'s cleanup, its step and its device log,
  on a harness under the runner's signals;
  the count on fake results of a skip and of a passing enrollment.
- **Left to CI**: the suite itself, and the whole Go and UI suites.
- **CI's first run, on the pull request, was the suite's first**,
  and the job Android emulator passed at its first try:
  the emulator had booted by the builds' end, 223 s after its start;
  the finger was enrolled by `EnrollTest` alone;
  the suite ran in about 4 minutes, the job in 10.
  The run's one red job was UI test (390-en):
  `test_maint_updates.py::test_another_setting_changed_during_the_own_put_is_read_after_it`
  failed once and passed on its retry, which fails the job all the same;
  the block changes nothing of the page.
- **The second run**, on the count's commit, passed every job,
  Android emulator in 8 minutes, and printed the counts:
  the enrollment 1 test;
  the suite 14, 0 skipped, 0 failed,
  and none of the 14 the sources declare left unrun,
  `EnrollTest`'s again among them, passing where a finger is there:
  - `ConnectPageTest.theConnectPageMayNotAskForAToken`
  - `ConnectPageTest.theBridgeAnswersTheConnectPage`
  - `EnrollTest.aFingerIsEnrolled`
  - `ForgetAllTest.forgetAllAsksOnceForTwoServers`
  - `KeepTest.theLoginIsKeptAfterAPasswordSignInWithOnePrompt`
  - `KeepTest.aServerIsAddedAndItsCertificateTrusted`
  - `SessionsTest.turnOffWithASessionRevokesTheKeyWithItAndSignsItOut`
  - `SessionsTest.removeSignsTheSessionOutFirst`
  - `SessionsTest.turnOffWithoutASessionRevokesTheKeyWithItselfAfterThePrompt`
  - `TapTest.aTapAsksForTheFingerprintAndItsCancelOpensTheCard`
  - `TapTest.theCardNamesItsServerAndTheMenuOpensItsSettingsAndTheApps`
  - `TapTest.backFromTheListLeavesTheAppAndTheServersPageShowsItsCard`
  - `TapTest.thePageIsCoveredAfterHomeUntilTheFingerprint`
  - `TapTest.aServerAskedAtEachReturnAsksAtEveryTap`

## What the reader found

Each was fixed in the rework, in the first commit.

- **The results were kept only on a failure**,
  and the job's own time limit marks a run cancelled,
  so a run stopped there would have kept nothing.
  The suite's step now has a limit of 32 minutes inside the job's 45,
  which GitHub counts as a failure;
  the upload runs on a failure or a cancellation;
  `run.sh` takes the step's shell's place (`exec`),
  so that the runner's signal reaches it;
  and its cleanup takes the screens and the reports home,
  before it stops the emulator.
- **A failed enrollment would have been tried again by every test**,
  about 44 minutes:
  the finger is now enrolled once, before the suite, by `EnrollTest` alone,
  and each test's setup fails at once without it.
- **The helper** says a failed touch and answers the next,
  and reads the log again while adb sees the emulator.
- **The boot's checks** are bounded by `timeout 10` each.
- **The device's log** is read from the moment adb sees the device,
  and the emulator's log carries the kernel's messages (`-show-kernel`).
- **The boot's time** is counted from the builds' end,
  the emulator booting while they run.
- **The audit's key** is passed in a file on the emulator,
  not on Gradle's command line.
- **A failure's message** names its step,
  never its script, which may hold the test password.
- **`sdk.sh`** leaves nothing behind of a failed download.

## Departures from the steps

- **The finger is enrolled through Settings, by UiAutomator** (step 2),
  in a test of its own, `EnrollTest`, run first,
  not by the make target in the shell.
- **A cancel is Back** (step 2):
  on Android 14 the prompt with the PIN's fallback has no Cancel.
- **CI downloads the pinned archives itself** (step 4),
  `sdkmanager` installing no given revision,
  each held to its size and SHA-1 from Google's repository (`sdk.sh`).
- **The AVD is written by hand** (step 3).
- **A setup that is not the case under test** goes through the bridge (step 1).
- **The server's page reached again** is reached by script (step 1),
  Back from the list no longer reaching it since Block 15.
- **The audit** is read by the test over the pinned certificate,
  with an owner key made for the run (step 1).
- **What to test's `app` paths** include `tools/app-emulator/` (step 4);
  the server's page (`index.html`, `partials/`, `src/`) is not among them,
  and a change to it is caught on the push to `main`:
  `GUIDELINES/project/CI.md` says so, and what adding it would cost.
  The Tested record's `app=` needs both Android jobs,
  in `packaging/tested-record.sh` and its test.
- **The count**, the second commit, beyond the steps:
  Gradle passes a run in which a test was skipped by an assume,
  or left out by a filter,
  so `tools/app-emulator/tally/` counts each of the two Gradle runs
  from its JUnit results against the `@Test`s the sources declare,
  and a skip or a test not run fails under CI or `APP_EMU_STRICT=1`;
  CI keeps a pass's results too, for a week.
- **`GUIDELINES/project/CI.md`** went to v1.69.0 and then v1.70.0:
  beyond step 4's table and paragraph,
  the Tested record, the time limits, the count,
  and section 8's pins, which no Dependabot entry reads.
- **The pinned image's WebView is Chrome 113** and never updates:
  page code beyond it fails here first.

## Not done here

- **`make app-emulator-test` on the development host**, waiting for D85.
- **The Verification's "each case of Blocks 15 to 17 seen failing once
  with its fix undone"**: it needs an emulator, and so KVM, here.
- **The contract's test-gate row** does not name `make app-emulator-test` yet.
- **No phone check of its own**, as the plan has it;
  what only a phone shows (step 6),
  a real fingerprint sensor, the phone maker's autofill and the look,
  is what `app/README.md` leaves to the phone.
