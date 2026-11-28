# NOTE SteadyTests v1.2: how the run went

The owner's `PLAN++` for v1.2 on 2026-11-28,
with D19-B and D20-A, D17-A and D18-A as recommended,
and the rest of v1.1's `EXEC++`: Block 10 once #473 had merged.

## What was built

- **#473**
  (Blocks 4 to 8 and 11, against `main`) merged.
  Its CI on Block 7 failed twice in a row, and a builder fixed it on the branch.
  The idioms guard had counted `main`'s new fixed pauses in `test_connect.py`
  and `test_copy.py`; they now wait for what they waited for.
  `test_a_joined_device_keeps_what_is_typed` failed because the test's
  `asked()` waited for the answer to reach Playwright, not the page;
  the kit's `taken` now waits until the page has read it,
  in 30 places.
- **#498**
  (Block 13's D14-A, into `maint/0.17`) merged:
  a settings change refused with a 401 goes back to the value that held,
  with the kit's `server_holds_settings`.
- **#501**
  (Block 10, against `main`) merged:
  `docs/testing.md` the front page of every suite, the kit by pattern;
  `docs/ui-testing.md` opens with its kit;
  the contract's "Tests" carries the authors' rule (v1.25.0).
- **#503**
  (Blocks 19 to 21, into `maint/0.17`): a refused colour scheme goes back;
  a stream opens only with its own Cube's ticket;
  a pairing read older than a join keeps the joined device.
- **#504**
  (Blocks 15 to 18, against `main`):
  the room checks' reasons to skip a room as one function the drift rig waits
  on;
  `ConnectPageTest` and four other hand-written polls on the app kit's `Wait`;
  the moved Cube's wait counts the connections since the move;
  the guard lists the host's `net::ERR_NETWORK_CHANGED`.
- v1.2 and #498 reached `main` in one upward merge, with
  `docs/ui-testing.md` v1.42.0 above both lines.

## Where the run went otherwise than the plan

- **Block 15's cause** is shown by a plant, not by the original run:
  no reason to skip a room held at a check in 344 runs without the plant,
  native and under QEMU.
  The rig's new failure message names the reason next time.
- **Block 17:**
  both readings were real by plant (a second connection,
  and a reconnect's backoff past 5 s); which one CI met is unknown.
- **Block 16's plant** is a hold behind the first evaluation:
  a stall posted ahead of it cannot fail the old class,
  since `onActivity` waits for the main thread before its latch starts.
  The sweep took four more hand-written polls onto a new `Wait.kt`.
- **Block 19** extended `server_holds_settings` with `schemes`
  rather than adding a second helper.
- **Block 10** took contract v1.25.0,
  since open ServerBackup pull requests hold v1.24.0,
  and `GUIDELINES/project/CI.md` v1.77.1 for one reference.
- **Version numbers of `docs/ui-testing.md`** were given to the builders in
  advance (v1.43.0 to v1.45.0), so that no two branches took the same one;
  the merges then took a number above both lines (v1.42.0, v1.44.1, v1.46.0,
  v1.46.1), each with its trailer.
- **The documents' line-break pass** on both lines left #503 and #504
  unmergeable, so that their CI did not start;
  each branch merged its base again.
- **A builder's wait loop** never ended:
  its `pgrep -f` matched its own command line.
  It was stopped after 14 hours; the briefs since warn of it.

## Found outside the plan

- **The app killed with Google Play services' process** is confirmed in CI
  twice on 2026-11-28 (#490, #481): `ActivityManager` kills the app because it
  depends on `com.google.android.gms/.fonts.provider.FontsProvider` in the
  dying `com.google.android.gms.persistent`.
  Likely appcompat's emoji font; a choice in the app.
- **An ANR at the connect page's close** on a loaded emulator, four times in
  Block 16's runs: the main thread blocked 5 to 6 s after the multicast lock
  and discovery, likely `Discovery.start()`/`stop()` on the main thread.
- **The emulator's first boot** missed `APP_EMU_BOOT_TIMEOUT` once in CI and
  three times locally (420 s and 900 s) on a host at a load of 28 to 38.
- **Other emulator failures** of 2026-11-28, from AP KeyLife's session:
  a SIGSEGV in `ForgetAllTest.forgetAllAsksOnceForTwoServers` (maint/0.17),
  and a fingerprint prompt that stayed in
  `TapTest.thePageIsCoveredAfterHomeUntilTheFingerprint`.
- **The copy tests under extreme load** (a quarter CPU under QEMU) get an extra
  `c:0a0201` from the follow-up's 15 s re-read: a window the test should set.
- **A test idiom that saves a field twice:**
  `fill` with a synthetic `change`, then the real one on blur.
- **`make test`** does not set `MASSIMO_ENV_FILE=/dev/null`;
  `make race` and `make test-armv6` do.
- **`AddressesTest`** (the app's JVM tests) asserts wall-clock bounds.
- **`test_login.py`'s poll test** depends on the operator's poll being 10 or
  absent on the server.
