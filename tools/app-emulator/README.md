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
- **Room in `TMPDIR`** (`/tmp` where it is not set) for the run's AVD,
  about 1.2 GB after a run, its data partition able to grow to 4 GB:
  where `/tmp` is a tmpfs, that is memory, and `TMPDIR` names another place.
- **The host's tools**: Go, `curl` and `openssl` for the servers;
  `unzip` and `sha1sum` for `sdk.sh fetch`;
  GNU coreutils' `timeout`, `env -C` and `tail --pid` for the run;
  `flock` (util-linux), which takes the run's locks,
  and `ss` (iproute2), which finds the ports in use (below, "The ports").

## What it does

1. The checks above, then the checkout's lock and the run's slot,
   which gives it its ports (below, "The ports"), then the AVD `massimo-test`,
   made at each run in the run's work directory (below, "The results"),
   its settings written from the pins:
   the run's own, which no other checkout's run starts or rewrites,
   and made anew at no cost, since `-wipe-data` remakes its images anyway.
   `ANDROID_AVD_HOME` is not read; the run sets it for the emulator alone.
2. The emulator, without a window, audio or snapshots
   (`-no-window -no-audio -no-snapshot`), its data wiped (`-wipe-data`),
   so that every run starts from a phone just set up,
   on the slot's console port, 5584 in slot 0,
   or on the one `APP_EMU_PORT` gives, an even port from 5554 to 5584,
   its serial `emulator-<port>` (below, "The ports");
   the kernel's messages in its log (`-show-kernel`),
   and the device's log kept from the moment `adb` sees it,
   so that a boot that stalls leaves both.
3. While it boots, the two servers and the app's debug and test APKs built.
4. Once it has booted (within `APP_EMU_BOOT_TIMEOUT`, 420 s, as CI sets it,
   counted from the builds' end, so that a slow first build takes none of it):
   the error dialogs off first (`hide_error_dialogs`, below, "The ports"),
   its animations and autofill off, the screen kept on;
   then, within the same time, the end of its first setup awaited,
   as the system and SystemUI see it, on two looks in a row at least 3 s apart:
   `user_setup_complete` and `device_provisioned` are 1,
   which the emulator's setup app sets as it ends,
   SystemUI's own dump has the device provisioned and the setup complete,
   and its keyguard has the SIM ready;
   where that does not come, the run stops,
   saying the step it waited for and what `adb` last said of it;
   then the PIN `1111` set (`adb shell locksettings set-pin`),
   which a fingerprint needs,
   the lock screen taken away before it and, with the PIN, after it;
   `adb reverse` for both servers' ports,
   the owner's key written to a file on the emulator,
   the helper started (below).
5. A guard started that keeps the lock screen away while Gradle runs,
   looking every five seconds, the PIN typed where it shows:
   a SystemUI that stopped answering and was started again shows it anew,
   and an enrollment run under it finds nothing of Settings;
   `keyguard.log` says each change it saw,
   SystemUI's pid among them, so that a SystemUI started again shows there.
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
and starts two servers on the slot's ports,
in slot 0 `a` on `127.0.0.1:9443` and `b` on `127.0.0.1:9444`
(`APP_EMU_API_PORT_A`, `_B`; below, "The ports"),
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

## The ports

A run holds a slot, which gives it seven ports of its own,
so that runs from two checkouts at once need no setting
(`slots.sh`; AP EmulatorRuns, D2-A).
Slot `k`, from 0 to 7, gives the console port of slot 0 - 2k,
and each other port of slot 0 + 2k:

| Port | Slot 0 | Slot `k` | Setting |
|---|---|---|---|
| the emulator's console, its serial `emulator-<port>` | 5584 | 5584 - 2k | `APP_EMU_PORT`, even, 5554 to 5584 |
| server `a`, server `b` | 9443, 9444 | 9443 + 2k, 9444 + 2k | `APP_EMU_API_PORT_A`, `_B` |
| their fake Cubes | 62941, 62942 | 62941 + 2k, 62942 + 2k | `APP_EMU_CUBE_PORT_A`, `_B` |
| the fake Cubes' discovery | 23941, 23942 | 23941 + 2k, 23942 + 2k | `APP_EMU_DISCOVERY_PORT_A`, `_B` |

The console ports count down, from 5584 to 5570,
because `adb` finds an emulator by itself, after its server restarts,
on the console ports 5554 to 5584 alone (it looks at 5555 to 5585).
An emulator on another port is known only to the server it told at its start,
and a later `adb` call loses it where that server was started again,
by `adb kill-server`, a crash, or a client of another version.
So `APP_EMU_PORT` takes an even port from 5554 to 5584 alone,
and any other stops the run before it starts anything (AP EmulatorRuns, D6-B).
A setting given replaces that port alone, in every slot.
The run takes the first slot whose lock it can take,
whose emulator `adb` does not list (one left by a killed run, which holds no lock)
and none of whose ports listens, the console's pair (its port + 1, adb's) among them;
`ss -Hltun` reads the listening sockets without connecting to them,
since a connection to another program's server would put an error in its log.
It says each slot it passed over and why,
then the slot and its ports;
where none of the eight is free, it stops before it starts anything.

A slot's lock is a `flock` of the file `massimo-app-emulator-slot.<k>.lock`
in `APP_EMU_LOCK_DIR`, `/tmp` where it is not set,
not `TMPDIR`, so that runs with other `TMPDIR`s see one another's;
the file is made where missing, readable by everyone,
opened for reading, so that one another user made serves too,
and it stays;
a lock path that is a link or no regular file, a FIFO among them, is passed over.
A run also locks this directory, `tools/app-emulator/`:
a second run of the same checkout would take the first one's `results/`
and the build and the reports in `app/android/`,
so it stops at once and says that a run of this checkout is going;
it waits for that, or runs from another checkout.

The locks are taken after `adb devices`, which starts `adb`'s server,
then handed to one holder, `tail --pid` of the run,
which a subshell starts and leaves, so that it is none of the run's children,
and which ignores `INT`, `TERM`, `HUP` and `QUIT`:
it keeps them until `run.sh` has ended and been reaped, however it ends,
a signal to its whole process group (Ctrl-C, `timeout`) and a `kill -9` among it,
so also while the cleanup still writes `results/`,
and lets them go within about a second.
Nothing else the run starts holds them,
neither an `adb` server that a later `adb` call starts again
nor a loop of the run's that a `kill -9` leaves running.

Two runs at once share the host.
On a 16-core host, two runs started together booted their emulators
in about 165 s each at a light load (a load average of about 2)
and in about 335 s each with the host's load near 20,
against 80 to 111 s for one alone.
A busy host also slows SystemUI, which draws the lock screen.
It can see the end of the first setup and the SIM up to two minutes after the system,
about 4 to 116 s after the settings in the runs,
the longest in a SystemUI started again,
and where the PIN was set before then,
the lock screen comes back as it does, under the enrollment.
It can also stop answering for long enough to be declared not responding;
with the error dialogs on and no lock screen showing just then,
the system leaves it running behind its "isn't responding" dialog,
no PIN typed reaches the lock screen,
and Settings' enrollment ends under it.
Before D5-A, four of eight runs failed so,
three of them started two at once,
each while other programs' tests kept the load average at 10 or more.
So the run sets the PIN only once SystemUI too has seen the setup end,
as "What it does" says above,
after which the lock screen does not come back by itself while that SystemUI runs,
and turns the error dialogs off (`settings put global hide_error_dialogs 1`):
the system then kills a SystemUI that stops answering and starts it again.
During the wait for the setup, in each run of two at once,
SystemUI was killed twice: for its ANR,
then, started again, for its content providers' timeout,
which the setting plays no part in;
the wait went on with the new SystemUI.
After the PIN it costs more.
A SystemUI started again, after its ANR or a restart of the system,
shows the lock screen at its start and again at its first SIM events,
and an enrollment or a prompt going on ends with the old one:
Settings is stopped, and `Fingers.enroll` waits out its 240 s;
the guard's PIN then only clears the screen.
In the runs, SystemUI's ANRs came within 75 s of its start alone,
mostly before the setup's end.
An app in front that stops answering is killed so too.
The app goes with its instrumentation ("Test instrumentation process crashed"),
which the run names after a failure (below, "The results"),
and Settings so ends the enrollment;
the run names the app's lines alone,
so Settings', SystemUI's and the other apps' ANRs are in `logcat.txt` only.
`Rig.quiet` presses the dialog's Wait at a test's start alone,
so it never offered Wait to an ANR in the middle of a test,
and it now matters only where the setting was refused.
`APP_EMU_BOOT_TIMEOUT`, 420 s from the builds' end,
counts the wait for the setup too and can be tight on a busy host;
two runs at once want a host with cores to spare.

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
| `TapTest.theCardNamesItsServerAndTheMenuOpensItsSettingsAndTheApps` | the card's server line and its ⓘ, ☰'s This server's settings and App settings | 15 |
| `TapTest.signOutOnAServersPageShowsTheList` | Sign out on a server's page ends the session at the server (`GET /api/v1/me` refused) and shows the list, the app holding no session there | 20 |
| `ForgetAllTest.forgetAllAsksOnceForTwoServers` | Forget all over two servers without a session, one prompt, each key revoking itself (both audits) | 17 |
| `SessionsTest.turnOffWithASessionRevokesTheKeyWithItAndSignsItOut` | Turn off with a session: no prompt, the key revoked with the session, which is signed out after (the audit) | 16 |
| `SessionsTest.turnOffWithoutASessionRevokesTheKeyWithItselfAfterThePrompt` | Turn off without a session: the prompt, the key revoking itself (the audit) | 16 |
| `SessionsTest.removeSignsTheSessionOutFirst` | Remove of a server without a kept login signs its session out, which the server then refuses | 16 |
| `ListTest.exitLeavesTheRecentAppsAndItsSessionIsEndedAtTheNextStart` | Exit from the list's ☰ ends the app's task with nothing sent, and the session it dropped is ended by its code at the next start (the audit) | 19 |
| `ListTest.backClosesTheMenuThenTheSettingsThenLeavesTheApp` | Back closes ☰, then a server's settings, and leaves the app from the bare list | 19 |
| `ListTest.storeLoginKeepsTheLoginWithOnePrompt` | Store login keeps the login of the session held with one prompt, its key made at the server | 19 |
| `ListTest.thePullDownChecksEveryServer` | the list pulled down checks every server again | 19 |
| `ListTest.aPageThatFailsToLoadReturnsToTheList` | a server's page that fails to load gives way to the list, which says the server does not answer | 19 |
| `ListTest.aCertificateThatChangedKeepsItsDialogAndSaysNothingOfAnAnswer` | a certificate that changed keeps its dialog, and the list says nothing of a server that does not answer | 19 |
| `HeldTest.theNightModeKeepsTheSessionHeld` | the phone's night mode, which makes the activity anew, keeps the session held in memory | 21 |
| `HeldTest.aSessionTheServerEndedIsNotLoggedOut` | a session the server ended itself, replaced by its key's next one or revoked with its key, is not logged out: no refused logout in the audit | 21 |
| `CodesTest.aSessionTheProcessForgotIsEndedByItsCode` | a session the process forgot is ended by its revoke code at the next start (the audit's `token.revoke_code`) | 21 |
| `CodesTest.turnOffWhileTheServerIsDownLeavesCodesTheNextContactTakes` | Turn off while the server does not answer leaves the key and the session, which the next contact ends by their codes | 21 |
| `CodesTest.signOutWhileTheServerIsDownIsEndedByItsCode` | Sign out while the server does not answer leaves the session, which the next contact ends by its code | 21 |
| `CodesTest.removeWhileTheServerIsDownEndsItsSessionWhereItIsFoundAgain` | Remove while the server does not answer keeps the session's code in `gone.json`, posted where the server is found again at its node | 21 |
| `CodesTest.aKeyLeftWithoutItsCodeIsRevokedAtTheSignInAndSaidSo` | a key left from a login kept before version 6, without a code, is revoked at the next sign-in, and My profile's toast says so | 21 |
| `CodesTest.aKeyLeftWithItsCodeIsTakenAtTheProbeAndNoToastShows` | a key left with its code is taken at the probe, and no toast shows | 21 |
| `CodesTest.aServerAddedAgainWithItsNodeTakesItsCodesBack` | Remove while the server does not answer, then the server added again with its node: `save`'s answer shows the session left waiting, `gone.json` holds it no more, and the next contact ends it by its code (D117-A) | 20 |
| `ConnectPageTest` (two tests) | the connect page and the bridge at contract version 7, a server's call refused | 3, 19 |
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

The AVD, the servers' stores, their `massimo` and `tally`
are in the run's work directory,
`massimo-app-emulator.XXXXXX` under `TMPDIR` (`/tmp` where it is not set),
the AVD in its `avd/`;
the directory goes at the end of the run,
and `KEEP=1 make app-emulator-test` keeps it, the AVD with it,
about 1.2 GB after a run, and prints its path
(AP CleanRuns, D4-A).
The directory holds the run's PID and the emulator's in `pid`.
A run that was killed, whose cleanup could not run, leaves its own;
a later run removes it once neither runs and it is older than a day,
as the UI suite does with its own
(`tools/workdir/README.md`; AP EmulatorRuns, D1-A, D3-B).

`results/`, emptied at each run and not kept by git:
`emulator.log` with the kernel's messages,
the device's `logcat.txt` from the boot on (the suite's steps under
`MassimoEmu`, and for a failed test the window's hierarchy and the page's
text), its crash buffer `crash.txt` (each crash's stack whole),
`fingers.log`, `keyguard.log`, each server's and fake Cube's log,
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

A failed test has the first lines of its failure under it.
Where the instrumentation ended before every test had run,
by a crash or a kill,
Gradle passes the run all the same,
and its results hold a case of their own, `android-test.testRunFailed`,
the run's record, not a test:
the tally names it so, with the orchestrator's text under it,
and the test it lost is among those not run.
A run that failed or ran short is then followed by what the device's log
says of how the app's processes ended,
beyond the orchestrator's own end of each test (`finished inst`,
`clear data`) and the uninstall after the run:
a kill by the system with its reason, a process that never attached,
a crash (its stack in `crash.txt`), an ANR, the low-memory killer.
Gradle's own logcat of a test killed before it began holds none of it.

```text
app-emulator:   failed   android-test.testRunFailed (the instrumentation ended before the run did: a crash or a kill, the device's lines below)
app-emulator:            Test instrumentation process crashed. Check ch.metaworx.massimo.KeepTest#aServerIsAddedAndItsCertificateTrusted.txt for details
app-emulator:   not run  KeepTest.aServerIsAddedAndItsCertificateTrusted
app-emulator:   device: … ActivityTaskManager: Destroy timeout of remove-task, attempt to kill Task{… #15 type=standard A=10196:ch.metaworx.massimo}
app-emulator:   device: … ActivityManager: Killing 7793:ch.metaworx.massimo/u0a196 (adj -10000): remove task
```

Those two lines are a race of the system's,
which the tests keep away (`Rig.removeTasks`).
Between two tests the orchestrator clears the app's data (`pm clear`),
and Android 14 removes with it every task of the app still in recents
and kills the app's processes;
where the last test's activity is still closing,
it waits a second first (`KILL_TASK_PROCESSES_TIMEOUT_MS`),
and by then the next test's instrumentation may have started:
not yet attached, it is killed with the rest,
and that test does not run, through no fault of its own.
Three of the suite's first runs in CI, on 2026-11-24, lost a test so.
Each test therefore takes the app's tasks out of recents once it has closed its activity,
which leaves the clear no task to wait for.
Where a passing run's log still shows the app's "Destroy timeout of remove-task",
`run.sh` says so in a warning, with the lines, and the run passes.

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
A move of the image also rechecks the three texts that `setup_ended` looks for,
in `run.sh`:
they are Android 14 SystemUI's dump wording (above, "What it does", step 4).

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
| `run.sh` | the run, as above; its work directory from `../workdir/workdir.sh`, which it sources, the UI suite's helper too (`tools/workdir/README.md`) |
| `servers.sh` | the two servers, sourced by `run.sh` |
| `slots.sh` | the checkout's lock, the slot and its ports, and the holder of both locks, sourced by `run.sh` |
| `fingers.sh` | the helper that answers each touch a test asks for |
| `sdk.sh` | `check` and `fetch` of the pinned emulator and image |
| `pins.sh` | the pins |
| `tally/` | the command that says what a Gradle run ran, against the tests the sources declare, with its Go tests |
| `scripts_test.go`, `doc.go` | the Go tests of the helper, of the slots, of `sdk.sh` and of the pins, a part of `go test ./...` |
