# Roadmap

The frame for this list is docs/design.md: target environments, architecture,
drivers, Loxone ownership, multi-user and the public server, with their phases.
What is done is in CHANGELOG.md, one section per release;
this file lists what is open, in order.
docs/handover-session.md says where things run and what is owed on hardware.

1. **The store, stages (d) and (e)** (docs/store.md section 9):
   the log in the store with paging and retention,
   then the backup history.
   Stages (a) to (c) shipped in v0.7.0-alpha.1 to v0.9.0-alpha.1 and have not
   run on a real box yet;
   stage (d) is planned (AP StoreLog v1.0) for the store session.
2. **Pairing**:
   the room's settings on a new device, an API to remove a device and the
   panel in the UI are being built (AP Pairing v1.0) in a session of its own;
   then the hardware test of the whole.
3. **Packaging, the rest**:
   the Debian package, the apt repository and the Homebrew tap shipped in
   v0.9.0-alpha.1 and wait on the owner's keys, Pages and tap repository;
   then Windows code signing, rpm, MSI and winget,
   the UI's update button through apt or brew, a container image,
   the LoxBerry plugin and a notarized `.pkg`, each on its own decision.
4. **TLS, the rest**:
   the five sources shipped in v0.9.0-alpha.1; the owner's hardware day is owed,
   then the plan retires.
   Later, each a plan of its own: UPnP, a local CA, a Pebble job in CI.
5. **Backup and restore in the web UI**:
   the API is in (plan, send, progress); the Settings panel is open,
   a session brief is ready; written from the UI's components (docs/ui.md,
   "How the page is built").
6. **The UI when the server is gone**:
   a prominent notice, edits blocked, polling until it is back; brief ready.
7. **Settings → Server** in the UI:
   the intervals and the stale threshold the API already takes,
   the certificate's details, in the header's expert list for now,
   and the log's retention once stage (d) adds it.
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
