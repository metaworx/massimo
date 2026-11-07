# Massimo Windows service: test report

Tested on 2026-11-05 on Martin's machine "jaguar",
following the plan in docs/testing-windows.md,
which at the time was a briefing in a session's scratch space.
Release under test: **v0.4.0-alpha.1** (Martin's choice;
the briefing named v0.3.0-alpha.1,
and v0.5.0-alpha.1 was published while the test ran).
No Massimo code was changed.
No real Cube was contacted;
all Cube traffic went to `massimo fakecube` on 127.0.0.1:62999.

## Environment

- Windows 11 Pro for Workstations 25H2, build 26200.9711
  (`[Environment]::OSVersion` reports 10.0.26200.0; the registry ProductName
  still says "Windows 10 Pro for Workstations").
- PowerShell 7 from the Claude desktop app, non-elevated.
  Elevated steps ran as `gsudo pwsh -NoProfile -NonInteractive -File <script>`
  with a gsudo credentials cache that Martin enabled for the session process.
- Defender: real-time protection on, signatures 1.463.134.0.
  **No detections** for any massimo binary.
  The test itself left no Defender/Operational event;
  Martin's browser download afterwards left one (event 2050, Information:
  "has uploaded a file for further analysis",
  the automatic sample submission of an unknown binary).
  See observation 8.
- SmartScreen: **silent during the test**,
  because curl.exe writes no Mark-of-the-Web stream and there was nothing to check.
  **It warns on a browser download launched by double-click**:
  Martin downloaded v0.5.0-alpha.1 with a browser afterwards and got "Windows
  protected your PC", Unknown publisher.
  The same file runs from a console without a dialog.
  See observation 8.
- Network profile: the only active connection is Wi-Fi "Altocumulus",
  categorised **Public** at the start.
  See step 4.
- Pre-existing and untouched:
  a `massimo portal` process from Martin's WSL build listening on :8080
  and 192.168.1.45:80.
  No port overlap with the service (8099, 62910, UDP 23272).

## Step 1: download and verify. OK

- `massimo-windows-amd64.exe` (9 458 176 bytes) and `SHA256SUMS` fetched
  from the v0.4.0-alpha.1 release.
- Local SHA-256 `adea9ad6…98bc41` equals the SHA256SUMS entry.
- `version` prints `massimo v0.4.0-alpha.1 (go1.26.10, windows/amd64)`.
- First attempt earlier in the day: v0.3.0-alpha.1 did not exist yet (404);
  PR #6 was still open.
  Both appeared later.

## Step 2: fake Cube and settings. OK

- `fakecube -greeting cube-fw0113-greeting.txt -listen 127.0.0.1:62999`
  starts, prints "fake cube listening on 127.0.0.1:62999 (10 lines)", and
  keeps running after its launching shell exits.
- `.env` written with the per-serial address form and a test token.

## Step 3: install. OK

- `install -dry-run -settings .env` lists: install the binary,
  the icacls command
  (`/inheritance:r /grant:r *S-1-5-18:(OI)(CI)F *S-1-5-32-544:(OI)(CI)F`),
  copy the settings, create the service with the expected command line,
  delete and add the firewall rule, start the service.
  Exit 0.
- `install -settings .env`:
  exit 0, prints install / copy / create / start.
- `sc.exe query massimo`:
  `STATE : 4 RUNNING`, WIN32_OWN_PROCESS, STOPPABLE, ACCEPTS_SHUTDOWN.
- `/healthz` answered on the first try:
  `{"ok": true, "version": "massimo v0.4.0-alpha.1 (go1.26.10, windows/amd64)"}`.
- `service.log`:
  version banner, "started as a service: [...]" with the full argv,
  then "running".
  `serve.log`: "serving cube", "api listening addr=[::]:8099 auth=true",
  "cube proxy listening addr=:62910", "cube connected addr=127.0.0.1:62999",
  six "device state" lines.
- Folder contents: `C:\Program Files\Massimo\massimo.exe`;
  `C:\ProgramData\Massimo\{env, serve.log, service.log}`.

## Step 4: check the install. OK

- `icacls C:\ProgramData\Massimo`:
  only `BUILTIN\Administrators:(OI)(CI)(F)` and `NT AUTHORITY\SYSTEM:(OI)(CI)(F)`.
  `Get-Acl` confirms AreAccessRulesProtected = True, owner Administrators,
  no inherited ACEs.
- `icacls …\env`, `serve.log`, `service.log`: the same two principals,
  each marked `(I)` because they inherit from the Massimo folder (not from ProgramData).
  That matches the intent.
- Non-elevated shell: `Get-Content C:\ProgramData\Massimo\env` →
  UnauthorizedAccessException "Access to the path … is denied".
  Listing the folder and `icacls` are refused too.
- `netsh advfirewall firewall show rule name=Massimo`:
  Enabled Yes, In, Profiles **Domain,Private**,
  Program `C:\Program Files\Massimo\massimo.exe`, Action Allow.
- Remote device: Martin's machine "mint" could **not** connect at first.
  Diagnosis: the active firewall profile was Public only,
  the Massimo rule covers Domain and Private,
  and no other enabled inbound rule covered massimo.exe or port 8099.
  After switching "Altocumulus 2" to Private (`Set-NetConnectionProfile`),
  **mint reached the UI at http://192.168.1.73:8099/** with no service restart.
  So the rule works and the Public exclusion behaves as documented.
- Service configuration (`sc qc`, `sc qfailure`, `sc qdescription`, registry):
  START_TYPE 2 AUTO_START (not delayed), SERVICE_START_NAME LocalSystem,
  DISPLAY_NAME "Massimo", description "Keeps the MAX!
  Cubes connected, serves the API and web UI and answers as a Cube on the LAN.",
  failure actions RESTART after 10 000 ms, three times, reset period 86 400 s.
  That is what the Services console's General and Recovery tabs show;
  I verified it with sc.exe rather than the GUI.
- `Get-Service massimo` prints DisplayName "massimo" in lowercase while `sc qc`
  and the registry say "Massimo".
  That is a PowerShell quirk, not a Massimo problem.

## Step 5: stop, start, install again. OK

- `sc.exe stop massimo`:
  the next `sc.exe query` already says `STOPPED`.
  `service.log` gains "stop requested" and "stopped".
  `/healthz` refuses (curl exit 7).
- `sc.exe start massimo`:
  START_PENDING with a PID, then RUNNING within a second; `/healthz` answers.
- `install` (no `-settings`): "C:\ProgramData\Massimo\env exists; kept it",
  "update the service massimo: …", "start the service massimo".
  Exit 0.
  The settings file's size, mtime and hash are unchanged.
  Service RUNNING, `/healthz` answers.

## Step 6: port change. OK

- `install -listen :8100`:
  service command line now has `-listen :8100`; `/healthz` answers on 8100,
  refused on 8099.
- `install` (no `-listen`): command line still `-listen :8100`;
  8100 still answers, 8099 refused.
- `install -listen :8099`:
  back to 8099; 8100 refused.
  Service RUNNING.

## Step 7: update through the service manager. OK

- `update -check`:
  "running: v0.4.0-alpha.1 (channel alpha)", "release:
  v0.5.0-alpha.1 (channel alpha, a7b9dba…)", "a newer release is out".
  Exit 0.
- `update -yes -version v0.4.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz`:
  "downloading massimo-windows-amd64.exe …",
  "installed v0.4.0-alpha.1 as C:\Program Files\Massimo\massimo.exe (the old one
  is …\massimo.exe.prev)", "stop the service massimo", "running v0.4.0-alpha.1".
  Exit 0, **4 s**.
  `massimo.exe` and `massimo.exe.prev` both present with identical hashes.
  Service RUNNING, `/healthz` answers.
- `update -rollback -restart massimo`:
  "rolled back …; the replaced binary is now …\massimo.exe.prev".
  Exit 0, 1 s.
  The two files swapped (mtimes prove it), service RUNNING, `/healthz` answers.
- `update -yes -version v0.2.0-alpha.1 -restart massimo -health …`:
  downloads and installs v0.2.0-alpha.1,
  then
  `update: start the service massimo: The service did not respond to the start or control request in a timely fashion. - rolling back` and `error: start the service massimo: …`.
  Exit 1 after **35 s**, not the ~2 min the briefing expected:
  the service manager's own 30 s connect timeout fired first (System log,
  Service Control Manager event 7009,
  "A timeout was reached (30000 milliseconds) while waiting for the massimo
  service to connect") and update rolled back on that error.
  Afterwards: `massimo.exe` is v0.4.0-alpha.1 again (hash unchanged),
  `massimo.exe.failed` (8 513 536 bytes, the v0.2.0 binary) is present,
  service RUNNING, `/healthz` on v0.4.0-alpha.1.
  No orphaned process of the failed binary.
  `service.log` shows the stop at 18:58:52 and the v0.4.0 restart at 18:59:23.
- Extra, because v0.5.0-alpha.1 existed by then:
  `update -yes -restart massimo -health …` (no `-version`) installed
  v0.5.0-alpha.1 in 4 s;
  `/healthz` reported v0.5.0-alpha.1 (it now also has `started_at`).
  `update -rollback -restart massimo`, run by the v0.5.0 binary,
  put v0.4.0-alpha.1 back in 1 s.
  Both OK.
  `massimo.exe.failed` from the previous test stayed in place throughout.

## Step 8: reboot. OK

- Last boot before: 2026-10-31.
  Rebooted at 19:15:04 (`shutdown /r`).
- After the reboot, before any elevation: `sc.exe query massimo` → `RUNNING`,
  exit codes 0; `/healthz` answers with v0.4.0-alpha.1.
- The service process was created at **19:15:36**, the interactive logon
  (Winlogon event 7001) came at **19:16:04** and explorer at 19:16:09, so
  the service ran before anyone logged in.
- The fake Cube was gone, as expected.
  Restarted it at 19:19:09;
  the API (`GET /api/v1/cubes`) showed `"connected": false` until 19:19:51
  and `"connected": true` with firmware, 6 rooms and 6 devices at 19:19:53.
  The service reconnected by itself, about 44 s after the Cube came back.
- Side effect of the reboot, not a Massimo issue:
  the Wi-Fi came back under the profile name "Altocumulus" (Public) instead of
  "Altocumulus 2" (Private), so the step 4 Private setting did not carry over.

- `serve.log` (read before removal) shows the reconnect in detail:
  after the boot-time start at 19:15:36 it logged
  `level=WARN msg="cube connection lost" … connectex: No connection could be made because the target machine actively refused it. retry_in=1s`, then 2s, 4s, 8s, 16s, 32s and 1m4s, repeating at 1m4s, then `cube connected` at 19:19:53.
  The 44 s delay is that backoff cap, not a stall.
  (It also logged four such lines at 19:14:15–19:14:29, before the reboot:
  shutdown ended the fake Cube before the service.)

## Step 9: remove. OK

- `install -remove`:
  "stop the service massimo", "delete the service massimo",
  "remove C:\Program Files\Massimo",
  "kept C:\ProgramData\Massimo (remove it with -purge)".
  Exit 0.
- After it: `sc.exe query massimo` → OpenService FAILED 1060, "does not exist";
  `C:\Program Files\Massimo` gone;
  `C:\ProgramData\Massimo` still there with `env`, `serve.log`, `service.log`;
  the firewall rule is **already gone** at this point ("No rules match").
  That is what the README says `-remove` does;
  the briefing's step 9 text expected the rule to go with `-purge`.
- `install -remove -purge` on the already removed install: "remove
  C:\ProgramData\Massimo", exit 0, no complaint about the missing service.
  Afterwards both folders are gone, the rule is gone,
  the service does not exist and its registry key under
  `HKLM\SYSTEM\CurrentControlSet\Services` is gone.
- The only massimo process left was my fake Cube.
  Stopped it and deleted `%USERPROFILE%\massimo-test`.

## Re-run on v0.5.0-alpha.1 with the lone-address settings. OK

docs/testing-windows.md (commit 1153b6d) differs from the briefing above in two points:
the release under test is v0.5.0-alpha.1,
and step 2 uses a lone `MASSIMO_CUBE_ADDRESS` (fine since v0.3.0-alpha.1,
commit 539b113) instead of the per-serial variable.
Steps 1, 2, 3 and 9 were therefore repeated with the v0.5.0-alpha.1 binary
and an `.env` holding only `MASSIMO_CUBE_ADDRESS=127.0.0.1:62999` and the token.
Steps 4 to 8 were not repeated.

- Step 1: hash `35588e68…0adbf0` matches SHA256SUMS;
  `version` prints v0.5.0-alpha.1.
  SmartScreen and Defender silent, as before.
- Step 2: fake Cube up on 127.0.0.1:62999.
- Step 3: dry-run and install as in the first run, exit 0.
  Service RUNNING, `/healthz` on the first try:
  `{"ok": true, "started_at": "2026-11-05T19:37:58…", "version": "massimo v0.5.0-alpha.1 …"}`.
  `GET /api/v1/cubes` reports
  `serial=KEQ0000100 address=127.0.0.1:62999 configured=True connected=True firmware=1.1.3 rooms=6 devices=6`: the serial came from the greeting
  (shown as the greeting's placeholder since 2026-11-07, here and below).
  Listeners: TCP 8099 and 62910 on `::`, UDP 23272 on 0.0.0.0,
  all the service's PID.
  ACLs, firewall rule, start type,
  LocalSystem and failure actions identical to the first run.
- The proxy serves the learned identity:
  a TCP client on 62910 gets `H:KEQ0000100,0a0107,0113,…` as its first line,
  and `massimo discover -addr 127.0.0.1` (unicast to loopback only,
  nothing sent to the LAN) returns `serial KEQ0000100, rf_address 0a0107, firmware 1.1.3`.
  A raw UDP probe to 23272 gets `eQ3MaxApKEQ0000100>I…` back.
- Step 9: identical to the first run: `-remove` deletes service,
  program folder and firewall rule, keeps the settings folder;
  `-purge` removes the settings folder and the registry key.
  Fake Cube stopped, test folder deleted.
- Observation (not a failure):
  with the lone-address form the log lines keep `serial=""` and `cube=""`
  ("serving cube serial=\"\"",
  "cube proxy listening … serial=\"\" discovery_serial=\"\"",
  "device state cube=\"\"") even after the greeting delivered the serial,
  while the API, the proxy greeting and discovery all show it.
  Only the logging is behind.

## Observations for the Windows code session

None of these is a failure.

1. **Quiet steps in the real run.**
   `install -dry-run` announces the icacls command and the two netsh commands;
   the real `install` and `install -remove` print nothing for them,
   although they run (step 4 shows the ACLs and the rule).
   Either print them in the real run too or drop them from the dry-run,
   for symmetry.
2. **service.log duplicates serve.log.**
   Every INFO line from serve ("serving cube",
   "device state" …) also lands in `service.log`,
   so it grows as fast as `serve.log` (163 lines after ~20 min of testing).
   If `service.log` is meant for the service manager's view only ("started as a service",
   "running", "stop requested", "stopped", errors),
   the duplication looks unintended.
3. **Output ordering.**
   The version banner sometimes prints after the first action line ("stop the
   service massimo" before "massimo v0.4.0-alpha.1 …"),
   and in `-dry-run` after all the "would" lines.
   Looks like stdout/stderr interleaving; cosmetic.
4. **Failed update is quicker than documented.**
   The service manager gives up on a binary that never connects after 30 s (error 1053,
   System log event 7009),
   so `update` rolls back after ~35 s rather than the ~2 min the briefing expected.
   Good behaviour; the expectation can be updated in docs/testing-windows.md,
   step 7.
5. **`massimo.exe.failed` is never cleaned up.**
   It survived a later successful update and a rollback.
   Worth deciding whether a successful update should remove it.
6. **Public networks block the UI silently.**
   By design the rule excludes Public,
   but a home Wi-Fi that Windows classifies as Public (this one was) just gives
   a browser timeout on other devices.
   The README's Windows section could tell people to check
   `Get-NetConnectionProfile` and switch the network to Private.
   Windows also re-matched the SSID to a different profile after the reboot
   ("Altocumulus 2" → "Altocumulus"), which was Public again.
7. **`-remove` deletes the firewall rule**, not `-purge`.
   The README is right; step 9 of docs/testing-windows.md says otherwise.
8. **SmartScreen warns when a browser download is double-clicked.**
   The test's curl downloads carried no Mark-of-the-Web,
   so SmartScreen never ran during the nine steps.
   After the test Martin downloaded v0.5.0-alpha.1 from the releases page with a
   browser (the file got the mark: `ZoneId=3`,
   referrer `github.com/metaworx/massimo/releases`;
   hash matches SHA256SUMS) and launched it from Explorer.
   Result: the blue "Windows protected your PC" dialog,
   "Microsoft Defender SmartScreen prevented an unrecognised app from starting",
   Application massimo-windows-amd64.exe, Publisher: Unknown publisher,
   with "Run anyway" and "Don't run" offered.
   Fifteen seconds after the download Defender also uploaded the file for
   analysis (event 2050), with no detection.
   The same marked file run from a non-elevated PowerShell
   (`.\massimo-windows-amd64.exe version`) printed the version in half a second
   with no dialog, and the mark stays on the file.
   So the documented flow is unaffected:
   `install` is run from an elevated prompt,
   the installed copy is written fresh (`os.ReadFile` plus `writeAtomic`,
   no mark), the Service Control Manager does not consult SmartScreen,
   and `update` fetches with Go's HTTP client, which adds no mark.
   Only a double-click on the download,
   which for a command-line binary does nothing useful anyway,
   meets the warning.
   The README's Windows section could say so in one line;
   the real fix is an Authenticode signature,
   which also needs reputation to build before the warning goes away.

   ![SmartScreen: "Windows protected your PC", Unknown publisher,
   with Run anyway and Don't run](images/windows-smartscreen-unknown-publisher.png)
9. **Reconnect backoff caps at 64 s.**
   After a Cube outage the service can take up to a minute to notice the Cube is back.
   Fine for heating, just worth knowing when reading logs.

## State left behind

- Nothing of Massimo is installed: no service, no `C:\Program Files\Massimo`,
  no `C:\ProgramData\Massimo`, no firewall rule, no test folder,
  and no massimo process at all (Martin's WSL `portal` did not come back after the reboot;
  I did not start it).
- The Wi-Fi profile "Altocumulus 2" was set to Private on Martin's request during step 4;
  after the reboot the active profile was "Altocumulus" (Public).
  Martin set that one to Private afterwards.
- Martin's browser download of v0.5.0-alpha.1 is still in his Downloads folder,
  Mark-of-the-Web included; it was only run with `version`.
- The repository clone is in `C:\Users\mdr\projects\massimo` on jaguar.
