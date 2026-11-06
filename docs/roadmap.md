# Roadmap

The frame for this list is docs/design.md: target environments, architecture,
drivers, Loxone ownership, multi-user and the public server, with their phases.
What is done is in CHANGELOG.md, one section per release;
this file lists what is open, in order.
docs/handover-session.md says where things run and what is owed on hardware.

1. **The store, stages (c) to (e)** (docs/store.md section 9):
   the preferences with the colour-scheme sync,
   the log in the store with retention, and the backup history.
   Stage (a) shipped in v0.7.0-alpha.1 and stage (b), users, login,
   the capability model, the audit and `commands`, in v0.8.0-alpha.1;
   neither has run on a real box yet.
2. **Packaging**:
   a Debian package, an apt repository on GitHub Pages and a Homebrew tap are in
   pull request #63 from a session of its own (AP Packaging v1.0),
   with the owner's steps for the keys, Pages and the tap repository;
   then a release carrying them.
   Windows code signing, rpm, MSI, a `.pkg`, a container image and LoxBerry
   wait on their own decisions.
3. **TLS for `serve`**:
   the five sources are on `main` (files, self-signed, a reverse proxy,
   Let's Encrypt by TLS-ALPN-01 and HTTP-01, and by DNS-01 through a hook);
   the owner's hardware day is owed, then the plan retires.
   A release with it ends CI's macOS rollback branch.
   Later, each a plan of its own: UPnP, a local CA, a Pebble job in CI.
4. **Backup and restore in the web UI**:
   the API is in (plan, send, progress); the Settings panel is open,
   a session brief is ready; written from the UI's components (docs/ui.md,
   "How the page is built").
5. **Pairing**:
   the hardware test of the server side, then the panel in the UI,
   copying the room's settings to a new device as the original software does,
   and an API to remove a device (the protocol line is built;
   a device that never answers is never dropped).
6. **The UI when the server is gone**:
   a prominent notice, edits blocked, polling until it is back; brief ready.
7. **Settings → Server** in the UI:
   the intervals and the stale threshold the API already takes,
   and the certificate's details, in the header's expert list for now.
8. **Translations**:
   a native speaker's review of FR, IT, NL and PL;
   the server's error texts in the UI's language.
9. **Release engineering**:
   Windows step 7b on a real PC; macOS notarization;
   log rotation through SIGHUP; the `edge` mode.
10. **Firmware dump** of the spare Cube once the FT232H is in
    (docs/firmware-extraction.md),
    then the DataFlash and the portal reply format.
11. **Small ones**:
    the stale threshold once real data shows what fits;
    discovery on the Pi with the spare Cube; `serve -log` under a supervisor;
    a UI test for each new panel in tools/ui-test/.
12. **Later**:
    the server mode with encryption at rest and sync (docs/server.md),
    Loxone (design.md section 4), a radio-stick driver without a Cube.
