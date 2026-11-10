# NOTE AptUpdate v1.0: real box run

The update helper of
[AP AptUpdate v1.0](2026-11-09_04-42_AP_AptUpdate_update_button_apt_brew.md)
on a real box, run on 2026-11-10 at the owner's word ("Yes, run them"),
from `main` at `d8c2016`.
The real Mac and the button against the signed repository are still to come.

1. **The route back to the release was dropped.**
   The first recipe had the helper take a box on a build from `main` "up" to
   `0.12.0~alpha.1`, which dpkg sorts above it.
   Since the account is `_massimo`, that breaks the box.
   In a systemd container (`debian:trixie-slim`), v0.12.0-alpha.1's postinst
   added a second account, `massimo`, beside `_massimo`,
   `/etc/massimo/env` stayed `_massimo`'s with mode 0600,
   and serve failed with `open /etc/massimo/env: permission denied`.
   This is the version-order finding F1,
   with a worse outcome than it had before the rename.
2. **Two test builds instead**, from `d8c2016` with the channel `alpha`:
   `make packages VERSION=v0.12.0-alpha.1.1 CHANNEL=alpha`, then `.2`.
   dpkg sorts `0.12.0~alpha.1.1` and `0.12.0~alpha.1.2` after `0.12.0~alpha.1`,
   and before `0.12.0~alpha.2`, `0.12.0` and `0.13.0~alpha.1`.
   Build 2 is in a flat local repository:
   `Packages` and `Release` made with `dpkg-scanpackages` and `apt-ftparchive`,
   the source `URIs: file:…`, `Suites: ./`, `Trusted: yes`.
   It lives under `/var/local`:
   the helper's `PrivateTmp=yes` hides `/var/tmp` from it.
3. **The same run in a systemd container first**, with no network.
   Build 1 was installed and started.
   The request was written as `_massimo`.
   The path unit started the helper,
   apt installed build 2 from the massimo source alone,
   the postinst restarted serve (a new PID),
   and the helper logged `running v0.12.0-alpha.1.2` and wrote
   `"state": "done"`.
4. **The real box**:
   a Raspberry Pi 5 without a Cube, Debian 13 on arm64,
   on `0.12.0~alpha.1~117~g3d7588f`, with the account still `massimo`.
   - Build 1 came in as an upgrade.
     The postinst renamed the account `_massimo`, keeping its UID and GID,
     and left no `massimo`.
     Serve restarted as `_massimo`.
     `/etc/massimo`, the settings,
     the store directory and the store are `_massimo`'s, at 0750 and 0600.
     The path unit was enabled and active.
   - With the source on the local repository,
     the request was written as `_massimo`.
     The helper ran 7 s from start to `done`:
     `apt installed 0.12.0~alpha.1.2 (was 0.12.0~alpha.1.1)`,
     then `running v0.12.0-alpha.1.2`.
     Serve restarted on the new version; the request was taken;
     the path unit stayed enabled.
   - With the real source back, apt's candidate stays `0.12.0~alpha.1.2`:
     the release sorts below it, so nothing is downgraded.
     The box can take the next release through the button.
5. **Seen along the way**, not this plan's to change:
   - During the rename, systemd warns that massimo.service "changed on disk".
     The postinst stops the service before its `daemon-reload`.
     It is harmless.
   - `massimo install -dry-run` offers the helper's health drop-in whenever a
     listen drop-in is kept,
     even when the address it names is the unit's default.
     It is harmless too.
6. **Still open:**
   - the button against the signed repository, once a release is out;
   - the real Mac: does the brew helper survive `launchctl kickstart -k`?
   - F1's fix.
