# App emulator test

`make app-emulator-test` runs `run.sh`:
the Android app's instrumented tests (`app/android/app/src/androidTest/`)
on an emulator, against two servers on the host built from this tree,
the fingerprint simulated
(AP MobileApp, D71-A).
An emulator runs the WebView, the bridge, the Keystore and the system's
biometric prompt as a phone does;
the JVM tests of `app/android/app/src/test/` reach none of them,
and the UI suite (`tools/ui-test/`) puts a fake bridge in the app's place.

It needs Linux with KVM,
the Android SDK with the pinned emulator and system image,
JDK 17, Go, `curl` and `openssl`
(and `unzip` and `sha1sum` for `sdk.sh fetch`),
and says which is missing before it starts anything.
How long a run takes, its first runs in CI will say;
the first one on a machine also fetches the test libraries.

## What it needs

- **KVM**, readable and writable by the user:
  the image is x86_64, which the emulator runs with hardware acceleration
  alone (`-accel on`), never in software.
  WSL 2 has `/dev/kvm` through Windows 11's nested virtualisation.
  Where it is `root:kvm` and mode `660`, the user joins the group `kvm`
  (`sudo usermod -aG kvm "$USER"`),
  then logs in again:
  the machine owner's step, which the target does not take;
  it says that KVM is not accessible, and stops.
  Until that new login, `sg kvm -c 'make app-emulator-test'`
  runs the target with the group, from a shell that predates it.
  CI's runners give the runner's user the device with a udev rule
  (below, "In CI").
- **The SDK** at `ANDROID_HOME`, as `app/README.md` sets it up
  ("The toolchain in WSL"), with `platform-tools`,
  and the emulator and the image that `pins.sh` names:

  ```bash
  sdkmanager emulator "system-images;android-34;google_apis;x86_64"
  ```

  `sdkmanager` installs the newest revision of each, which may be newer than
  the pins: `sdk.sh check` says so, and the run goes on with it.
  `tools/app-emulator/sdk.sh fetch` installs the pinned revisions themselves,
  from Google's SDK repository, each archive held to the size and the
  SHA-1 the repository gives;
  it needs the SDK's licence accepted (`sdkmanager --licenses`)
  and accepts none itself.
  The two downloads are about 350 MB and 1.6 GB,
  some 5 GB once unpacked.
- **JDK 17** for Gradle, `JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64`
  where another is the default.
- **The host's tools**: Go, `curl` and `openssl` for the servers;
  `unzip` and `sha1sum` for `sdk.sh fetch`;
  GNU coreutils' `timeout` and `env -C` for the run.

## What it does

1. The checks above, then the AVD `massimo-test` in `~/.android/avd`
   (`ANDROID_AVD_HOME`), made where missing,
   its settings written from the pins at each run.
2. The emulator, without a window, audio or snapshots
   (`-no-window -no-audio -no-snapshot`), its data wiped (`-wipe-data`),
   so that every run starts from a phone just set up,
   on its own console port, 5584 (`APP_EMU_PORT`), the serial `emulator-5584`;
   the kernel's messages in its log (`-show-kernel`),
   and the device's log kept from the moment `adb` sees it,
   so that a boot that stalls leaves both.
3. While it boots, the two servers and the app's debug and test APKs built.
4. Once it has booted (within `APP_EMU_BOOT_TIMEOUT`, 300 s,
   counted from the builds' end, so that a slow first build takes none of it):
   its animations and autofill off, the screen kept on,
   the PIN `1111` set (`adb shell locksettings set-pin`),
   which a fingerprint needs,
   the lock screen taken away before it and, with the PIN, after it;
   `adb reverse` for both servers' ports,
   the owner's key written to a file on the emulator,
   the helper started (below).
5. A guard started that keeps the lock screen away while Gradle runs,
   looking every five seconds, the PIN typed where it shows:
   on a busy host the boot can show it again minutes after the PIN was set,
   and the enrollment then runs under it, finding nothing of Settings;
   `keyguard.log` says each change it saw.
   It reads the window manager's policy and the activities' keyguard state alone,
   since `dumpsys window`'s whole dump opens with the last ANR's copy of the state.
6. The finger enrolled by `EnrollTest` alone (below, "The fingerprint");
   where that fails, the run stops there.
7. `:app:connectedFossDebugAndroidTest` on that emulator alone
   (`ANDROID_SERIAL`), given the servers' addresses and fingerprints,
   the user, the PIN and the name of the key's file
   as the instrumentation's arguments;
   `APP_EMU_CLASS` runs one class, or one `class#method`.
   After it, and after the enrollment's run,
   what the run ran (below, "The results").
8. Everything stopped, the emulator by its console,
   and what the run leaves in `results/`,
   also where a signal (`INT`, `TERM`) stops it:
   Gradle runs in the background and is waited for,
   so that the trap runs at once,
   and the cleanup takes the screens and the report home first,
   each step of it bounded.

Every `adb` command names the emulator by its serial:
a phone that `adb` knows, over USB or wireless debugging, is never sent
anything.
The orchestrator runs each test in an instrumentation of its own,
the app's data cleared before it (`clearPackageData`),
its Keystore keys with it,
so that every test starts from the app installed anew.

## The servers

`servers.sh` builds `massimo` from the checkout, the page's parts first,
and starts two servers, `a` on `127.0.0.1:9443` and `b` on `127.0.0.1:9444`
(`APP_EMU_API_PORT_A`, `_B`),
each in a temporary directory of its own with its store,
`MASSIMO_ENV_FILE=/dev/null`, so that nothing of the checkout's `.env`
is read,
over HTTPS with the self-signed certificate it makes at its start,
against a fake Cube of its own with the UI suite's greeting
(`tools/ui-test/greeting.txt`, the serial `KEQ0000100`),
with no proxy, no announcement and no search but at its fake Cube.
The owner's key, the servers' `MASSIMO_API_TOKEN`, is made for the run;
the user `tester` (role operator) signs in with a password.
The fingerprint each server's log names as it makes its certificate is
checked against the one it answers with, and handed to the tests,
which compare it with the app's fingerprint sheet and pin it to read the
audit.
The directories go at the end of the run.

## The fingerprint

A test inside the emulator cannot reach the emulator's console,
which alone touches its fingerprint sensor (`adb emu finger touch`).
So a test asks for each touch with a line in the log,
`touch <n>` under the tag `MassimoEmu`, `n` rising across the suite
(`Rig.touch`),
once the prompt it waits for shows,
and `fingers.sh` reads the log (`adb logcat -v raw -s MassimoEmu:I`)
and answers each such line once,
with `finger touch 1` and `finger remove`, a touch and its lift.
It refuses a serial that is not an emulator's before it calls `adb`.
A prompt still shown after a touch is touched again, three times at most.
A test that cancels a prompt presses its Cancel through UiAutomator,
and where the prompt has none, as from Android 11 on,
where it takes the PIN and shows "Use PIN" instead, presses Back, as a user does.

A touch the console refuses is said in `fingers.log`, and the helper goes on;
a log that ends while `adb` still sees the emulator is read again,
the asks answered before skipped.

The finger is enrolled once per emulator, before the suite,
by `EnrollTest` alone, which `run.sh` runs first:
the system's Settings, from `android.settings.FINGERPRINT_ENROLL`,
driven by UiAutomator, the PIN typed where it asks for it,
its More and I agree pressed,
each touch of the sensor asked of the helper as above,
until the phone has a strong biometric (`Fingers.enroll`).
Where it fails, the run stops with Settings' screen kept,
and the suite does not start;
each case only checks that a finger is enrolled (`Fingers.check`),
so that a failed enrollment is one failure, not a wait in every case.
In the suite's own run `EnrollTest` passes at once.

## The cases

| Test | What it checks | Block |
|---|---|---|
| `KeepTest.aServerIsAddedAndItsCertificateTrusted` | a server added by its address, the fingerprint sheet showing the server's, trusted | 2 |
| `KeepTest.theLoginIsKeptAfterAPasswordSignInWithOnePrompt` | the login kept after a password's sign-in, one prompt, the key made at the server (its audit) | 8, 17 |
| `TapTest.aTapAsksForTheFingerprintAndItsCancelOpensTheCard` | a tap with no session held asks for the fingerprint and opens the page signed in; cancelled, at its card with the fingerprint button | 15 |
| `TapTest.aServerAskedAtEachReturnAsksAtEveryTap` | a server asked at each return, opened twice from the list, asks twice | 15 |
| `TapTest.thePageIsCoveredAfterHomeUntilTheFingerprint` | the cover after Home, taken away by the prompt | 15 |
| `TapTest.backFromTheListLeavesTheAppAndTheServersPageShowsItsCard` | the list forgets the pages before it, Back from it leaves the app, and the server's page reached again without a tap shows its card | 15 |
| `TapTest.theCardNamesItsServerAndTheMenuOpensItsSettingsAndTheApps` | the card's server line and its ⓘ, ☰'s This server's settings… and App settings… | 15 |
| `ForgetAllTest.forgetAllAsksOnceForTwoServers` | Forget all over two servers without a session, one prompt, each key revoking itself (both audits) | 17 |
| `SessionsTest.turnOffWithASessionRevokesTheKeyWithItAndSignsItOut` | Turn off with a session: no prompt, the key revoked with the session, which is signed out after (the audit) | 16 |
| `SessionsTest.turnOffWithoutASessionRevokesTheKeyWithItselfAfterThePrompt` | Turn off without a session: the prompt, the key revoking itself (the audit) | 16 |
| `SessionsTest.removeSignsTheSessionOutFirst` | Remove of a server without a kept login signs its session out, which the server then refuses | 16 |
| `ConnectPageTest` (two tests) | the connect page and the bridge at contract version 5, a server's call refused | 3 |
| `EnrollTest.aFingerIsEnrolled` | the finger enrolled through Settings, run alone before the others | 18 |

A step on the list or a server's page is a script in the WebView
(`Page`), as a tap would reach the page;
the prompt is the system's, found by its title, the app's text.
What the server did is read from its audit
(`GET /api/v1/audit?user=tester`, with the owner's key)
and from `GET /api/v1/me`.

## What it does not cover

What the owner's phone check keeps:
a real fingerprint sensor, the phone maker's autofill and password manager,
and the look.
Nor does it cover discovery (an emulator hears no `serve` on the host's
LAN), push, a server over plain HTTP outside the home network,
or Android 8 to 10, whose prompt takes no PIN.

## The results

`results/`, emptied at each run and not kept by git:
`emulator.log` with the kernel's messages,
the device's `logcat.txt` from the boot on (the suite's steps under
`MassimoEmu`, and for a failed test the window's hierarchy and the page's
text), `fingers.log`, `keyguard.log`, each server's and fake Cube's log,
`shots/` with the screen of each failed test,
Gradle's `report/` and `test-results/` (its JUnit results, `TEST-*.xml`),
and `enroll/`, the enrollment's report and results where it passed.

After each of the two Gradle runs,
`tally/` reads its JUnit results and says what ran,
one line for the run and one for each test with how it went:

```text
app-emulator: the suite: 14 tests, 0 skipped, 0 failed; 0 of the 14 the sources declare not run
app-emulator:   passed   KeepTest.aServerIsAddedAndItsCertificateTrusted
```

The tests it expects are those the sources declare,
each `@Test` of `app/src/androidTest/kotlin/`,
narrowed by the run's class (`APP_EMU_CLASS`,
or `EnrollTest` for the enrollment),
so that a new test is expected as soon as it is written.
Gradle passes a run whose tests were skipped (an `assume`) or that ran fewer than that;
strict, under CI (`CI=true`) or with `APP_EMU_STRICT=1`,
the run fails there too, and says which.

The suite logs no key, token or password:
the owner's key reaches the tests as a file on the emulator,
not as an argument,
and a step that fails names the script it ran, never its text.

## Pins

`pins.sh` pins the image, Android 14 with Google's APIs on x86_64,
by its revision, and the emulator by its version,
each with its archive's address, size and SHA-1
as Google's SDK repository lists them
(`repository2-3.xml`, `sys-img/google_apis/sys-img2-3.xml`).
A pin moves by a decision, its values together;
`TestPinsHoldTogether` (`scripts_test.go`) checks that they fit.

The image's WebView is the one it was built with, and it never updates,
as the image has no Play Store:
Chrome 113 in revision 14 (`com.google.android.webview` 113.0.5672.136).
Page code that needs a newer Chrome fails here first,
where a phone's updated WebView passes it.

## In CI

The job "Android emulator" (`.github/workflows/ci.yml`;
`GUIDELINES/project/CI.md`, section 2) runs beside "Android app",
where What to test answers `app`:
KVM given to the runner's user by a udev rule,
the pinned emulator and image fetched into a directory of the run's
(`APP_EMU_SDK`, `sdk.sh fetch`),
then `make app-emulator-test`'s two steps, `make ui` and `run.sh`,
in a step with a time limit of its own (32 minutes, inside the job's 45),
`run.sh` in the shell's place (`exec`),
so that the runner's signal at that limit reaches it and its cleanup
keeps the results;
`results/` is the artifact `app-emulator-results`,
on a failure or a cancellation,
and on a pass too, kept seven days, so that a passing run can be read;
the run is strict there (`CI=true`).
Its result counts with Android app's in the Tested record's `app=`.

## Files

| File | What |
|---|---|
| `run.sh` | the run, as above |
| `servers.sh` | the two servers, sourced by `run.sh` |
| `fingers.sh` | the helper that answers each touch a test asks for |
| `sdk.sh` | `check` and `fetch` of the pinned emulator and image |
| `pins.sh` | the pins |
| `tally/` | the command that says what a Gradle run ran, against the tests the sources declare, with its Go tests |
| `scripts_test.go`, `doc.go` | the Go tests of the helper, of `sdk.sh` and of the pins, a part of `go test ./...` |
