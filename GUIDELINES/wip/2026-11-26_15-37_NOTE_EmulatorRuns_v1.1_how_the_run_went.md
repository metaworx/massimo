# NOTE EmulatorRuns v1.1: how the run went

The run of `PLAN++ D5-A D6-B` on AP EmulatorRuns v1.1
(`wip/2026-11-25_20-32_AP_EmulatorRuns_each_run_its_own_avd_and_ports.md`),
2026-11-26.

## What is done

Blocks 3 and 4 are committed on `claude/emulator-runs`, one commit each,
after the revision's own commit, "chore(aps): +EmulatorRuns v1.1".
Nothing is pushed and no pull request is open:
the gate offering `PLAN++` stated no push.

## Where the run went differently from the plan

1. **Block 3 waits for SystemUI, not for the lock screen's return.**
   The image has no Provision app:
   its first setup is GoogleSdkSetup's `ProvisionActivity`,
   which turns the lock screen off and, at its end,
   sets `user_setup_complete` and then `device_provisioned` to 1.
   Without a PIN the lock screen never comes back, so there is no return to wait for.
   With one, it comes back where SystemUI handles its first SIM events after the PIN,
   8 to 69 s after the settings turned 1 on a busy emulator.
   A PIN set once SystemUI had the setup's end and the SIM ready brought no return.
   So `setup_ended` waits for the two settings,
   SystemUI's `DeviceProvisionedControllerImpl` dump and its
   `KeyguardUpdateMonitor`'s SIM ready, on two passing looks in a row.
   Those are Android 14 SystemUI's dump texts:
   a move of the image's pin checks them again (`README.md`, "Pins").
2. **Block 3: `APP_EMU_BOOT_TIMEOUT` defaults to 420 s, CI's value, not 300.**
   It counts the setup's wait as the plan said,
   and one run of two at once took 341 s from the builds' end.
   On CI the emulator has booted by the builds' end (152 to 198 s),
   so the wait has the whole budget there.
3. **Block 3, from the second reading:**
   `hide_error_dialogs` is the first setting after the boot,
   since SystemUI's ANR at its start came about half a minute after it;
   the guard writes SystemUI's pid into `keyguard.log`,
   so that a SystemUI started again shows;
   the stop of the wait names the step and what adb last said;
   README's step 5, on the guard, is rewritten too.
4. **Block 3: what the error dialogs off cost, written down.**
   A SystemUI started again after the PIN shows the lock screen,
   and an enrollment or prompt then going on ends with it.
   An app in front that stops answering is killed with its instrumentation,
   where it would have shown a dialog;
   `Rig.quiet` pressed that dialog's Wait only at a test's start.
   In the runs, SystemUI's ANRs came within 75 s of its start alone.
5. **Block 3's runs differed from the plan's.**
   - The first pair without the fix, its emulators pinned to four cores,
     was starved too far:
     one run failed as `system_server` restarted under `locksettings verify`,
     the other passed; its `APP_EMU_CLASS` lacked the package,
     so its suite ran no test.
     The second pair, on six cores, failed once at the enrollment
     as the NOTE of v1.0 describes.
   - The pinned runs had `APP_EMU_BOOT_TIMEOUT=900`.
   - "Each passes" did not hold:
     with the fix, one pinned run passed the enrollment and failed its suite,
     the app killed with Google Play services' persistent process,
     itself killed for an ANR in the background,
     which the dialogs' setting plays no part in.
   - Every run ran `ForgetAllTest` alone:
     the whole suite with the error dialogs off runs first on CI.
6. **The mechanism check** let SystemUI go on 12 s after the key,
   since a stopped process holds up its ANR's stack dump.
7. **Block 4** also refuses a port with a leading zero,
   which the shell would read as octal.

## How it was verified

- **Block 3,** each run with `ForgetAllTest`, every adb command naming its serial:

  | Run | Code | Cores | Boot | Setup wait | Load | Result |
  |-----|------|-------|------|------------|------|--------|
  | old 1a | Block 2's | 4 | 240 s | - | 3.6-10.0 | failed: `system_server` restarted |
  | old 1b | Block 2's | 4 | 241 s | - | 3.6-6.4 | passed; the lock screen back, the guard's PIN took it |
  | old 2a | Block 2's | 6 | 115 s | - | 3.5-11.2 | failed at the enrollment, as the NOTE of v1.0 says |
  | old 2b | Block 2's | 6 | 116 s | - | 3.5-12.0 | passed; the lock screen back, taken |
  | new 1a | the fix | 4 | 249 s | 172 s | 7.9-11.2 | passed |
  | new 1b | the fix | 4 | 248 s | 167 s | 7.9-13.2 | enrollment passed; suite failed, as item 5 says |
  | new 2a | the fix | 6 | 120 s | 130 s | 4.6-14.1 | passed |
  | new 2b | the fix | 6 | 119 s | 143 s | 4.6-12.2 | passed |
  | alone | the fix | all | 77 s | 27 s | 7.5-5.7 | passed |
  | final | the reworked fix | all | 70 s | 26 s | 0.2-7.5 | passed |

  With the fix the lock screen did not come back in any run.
- **The mechanism**, on an emulator started by hand on a slot's port,
  its lock held: SystemUI stopped through `adb root` and a key sent.
  Without `hide_error_dialogs` its dialog stayed, the same process behind it;
  with it, SystemUI was killed at its ANR and started again,
  and its lock screen took the PIN.
- **Block 4:** `TestSlotPortGiven`, 5554, 5570 and 5584 taken,
  5552, 5553, 5583, 5586, 5682, a word and a leading zero refused;
  `run.sh` with `APP_EMU_PORT=5586` stopped with the message
  and started nothing.
- Both blocks: `go test` of `tools/app-emulator/` and `tools/workdir/`,
  `shellcheck`, `gofmt`, `go vet`, `GUIDELINES/shared/tools/check`;
  a second reader read Block 3 and its rework.
- **Left to CI:** the whole emulator suite, strict, on the pull request.

## Seen and left

- **The guard's TERM trap** prints "unexpected EOF while looking for
  matching `)'" at the cleanup in one run of ten:
  bash 5.2, a trap that fires while the guard is inside a `$(…)`.
  It is older than this plan, from "test(app): The emulator suite keeps the
  lock screen away from the enrollment", and harmless;
  the guard without its trap would end silently.
- **`GUIDELINES/project/CI.md` and `ci.yml`** stay:
  their emulator "boots within `APP_EMU_BOOT_TIMEOUT`" still holds,
  and CI's 420 now repeats the default.
