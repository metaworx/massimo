# NOTE HostReboot v1.0: how the run went

How `PLAN++` (2026-11-08) ran
[AP HostReboot v1.0](2026-11-08_15-06_AP_HostReboot_reboot_from_the_web_ui.md),
where it differs from the plan.

## Deviations

1. **Block 3**: the Debian `postinst` first took debhelper's snippet for a
   unit it enables (`was-enabled` is true on a first install), so the
   package enabled the reboot helper; `make deb-test` caught it. It now
   takes the snippet for a unit debhelper does not enable
   (`debian-installed` before `was-enabled`), and `test.sh` also checks that
   a helper the owner enabled stays enabled through a reinstall and that a
   purge removes its link, on trixie and on bookworm.
2. **Block 4**: `GET /api/v1/reboot` gained `begun`, so that the page tells
   a reboot that waits from one under way without comparing its clock with
   the server's; the OpenAPI file and `docs/api-v1.md` say so.
3. **Block 4**: the page's timer until the reboot's time is held below
   2^31 ms, beyond which `setTimeout` fires at once.
4. **Block 4**: the capability needed its own rule in `style.css`
   (`.no-reboot .needs-reboot`), beside the card's `needs-maintenance`.
5. **Block 4**: a reboot the helper took but the computer did not carry
   out (`systemctl reboot` failing in the helper) left "the reboot has
   begun" until the next start. A server still running two minutes after
   the helper took the request now counts the reboot as failed, says so
   in `error`, and takes the next one; the page shows it as the old
   server still answers.

## How it was verified

Every block: `gofmt -l .`, `go vet` on linux, windows and darwin,
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`; Block 1 also
`-race` for `internal/server`. Block 3: `make packages deb-test` on trixie
and bookworm, `systemd-analyze verify` of the two units, and the macOS
helper's script run with its paths replaced (no request: nothing; a request
or a dangling link: removed, then the reboot). Block 4: the whole UI suite,
`make ui-test` at 1100 and 390 px in English and German, on its state:
431 of 432 passed; `test_back_to_top[390-de]` did not see ↑ reach the top
within 3 s while the suite ran, and passed three times of three run alone
with the rest of `test_system_layout.py`. Blocks 1 to 3 change the page
only by the capability's name and label. The screenshot set was retaken
and looked at: Maintenance and System settings changed, Cubes moved by a
pixel, five others differed by noise of a few pixels and were kept as
they were.

Nothing rebooted a computer: `shutdown.exe` was not run on Windows, no
LaunchDaemon was loaded on a Mac, and no path unit was started under
systemd. The first reboot on a real box is the owner's check.
