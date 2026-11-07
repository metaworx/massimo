# Testing the Windows service on a real PC

A test plan for a person or an agent at a Windows machine.
`massimo install` as a Windows service and `massimo update -restart massimo`
exist since v0.3.0-alpha.1 (packaging/README.md,
"Windows") and have so far only run on a GitHub CI runner.
This plan exercises them on a real PC against a fake Cube.
Do not change the code during the test; bugs go into the report.

What install does on Windows:

- binary: `C:\Program Files\Massimo\massimo.exe`
- settings and logs: `C:\ProgramData\Massimo\` (`env`, `serve.log`,
  `service.log`), readable only by SYSTEM and Administrators
- service `massimo`: automatic start, runs as LocalSystem, restarted
  after 10 s on failure; command line
  `massimo -env C:\ProgramData\Massimo\env serve -listen :8099 -log …`
- firewall rule "Massimo" for the program, on private and domain networks

## Ground rules

- Run in a terminal opened with **Run as administrator**.
  Install and update need it.
- **Do not connect to a real Cube unless the owner says so.**
  A Cube accepts exactly one client, and the Raspberry Pi serves the house Cube.
  Test against the fake Cube (step 2).
- No secrets in the report: no keys, IVs, tokens, or the contents of `env`.
- Ask before rebooting, and before anything that changes the system
  beyond what is described here.
- PowerShell or `cmd`; `curl.exe` comes with Windows 10 and 11.
- Replace `v0.5.0-alpha.1` below with the release under test; the
  update step needs a second, older release to roll back to.

## Steps

### 1. Download and verify

```powershell
mkdir $env:USERPROFILE\massimo-test; cd $env:USERPROFILE\massimo-test
$v = "v0.5.0-alpha.1"
curl.exe -fLO "https://github.com/metaworx/massimo/releases/download/$v/massimo-windows-amd64.exe"
curl.exe -fLO "https://github.com/metaworx/massimo/releases/download/$v/SHA256SUMS"
(Get-FileHash massimo-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
Select-String massimo-windows-amd64.exe SHA256SUMS
.\massimo-windows-amd64.exe version
```

The two hashes must match, and `version` must name the release.
SmartScreen may warn about an unsigned program; note that in the report.

### 2. A fake Cube and the settings

```powershell
curl.exe -fLO "https://raw.githubusercontent.com/metaworx/massimo/main/internal/protocol/cube/testdata/cube-fw0113-greeting.txt"
Start-Process -NoNewWindow .\massimo-windows-amd64.exe -ArgumentList 'fakecube','-greeting','cube-fw0113-greeting.txt','-listen','127.0.0.1:62999'
Set-Content -Encoding ascii .env "MASSIMO_CUBE_ADDRESS=127.0.0.1:62999`nMASSIMO_API_TOKEN=test-token"
```

The fake Cube reports the fixture's serial in its greeting; a lone
`MASSIMO_CUBE_ADDRESS` is fine since v0.3.0-alpha.1.

### 3. Install

```powershell
.\massimo-windows-amd64.exe install -dry-run -settings .env
.\massimo-windows-amd64.exe install -settings .env
sc.exe query massimo
curl.exe -s http://127.0.0.1:8099/healthz
Get-Content C:\ProgramData\Massimo\service.log
Get-Content C:\ProgramData\Massimo\serve.log -Tail 20
```

Expected: the service is `RUNNING`, `/healthz` reports the release,
`service.log` shows "started as a service" and "running", and
`serve.log` shows the fake Cube connected.

### 4. Check the install

- `icacls C:\ProgramData\Massimo` and `icacls C:\ProgramData\Massimo\env`:
  only `NT AUTHORITY\SYSTEM` and `BUILTIN\Administrators`, nothing inherited.
- In a normal, **non**-elevated PowerShell:
  `Get-Content C:\ProgramData\Massimo\env` must be refused.
- `netsh advfirewall firewall show rule name=Massimo` shows the rule.
  Open `http://<this machine's IP>:8099/` from another device (the UI asks for
  the token `test-token`).
  Note the machine's network profile (`Get-NetConnectionProfile`):
  the rule does not apply on "Public" networks.
- In the Services console (`services.msc`),
  the service is "Massimo" with Automatic start,
  and the "Recovery" tab says "Restart the Service".

### 5. Stop, start, install again

```powershell
sc.exe stop massimo;  sc.exe query massimo
sc.exe start massimo; sc.exe query massimo
.\massimo-windows-amd64.exe install
curl.exe -s http://127.0.0.1:8099/healthz
```

Installing again keeps the settings ("exists;
kept it") and restarts the service.

### 6. Port change

```powershell
.\massimo-windows-amd64.exe install -listen :8100
curl.exe -s http://127.0.0.1:8100/healthz
.\massimo-windows-amd64.exe install
curl.exe -s http://127.0.0.1:8100/healthz
.\massimo-windows-amd64.exe install -listen :8099
```

After the second install, port 8100 must still answer.

### 7. Update through the service manager

Reinstalling the same release with `-version` exercises the whole path:
download, check, rename of the running `.exe`, restart, health check.

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -check
& "C:\Program Files\Massimo\massimo.exe" update -yes -version v0.5.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz
dir "C:\Program Files\Massimo"
curl.exe -s http://127.0.0.1:8099/healthz
```

Expected: "installed v0.5.0-alpha.1", `massimo.exe.prev` exists,
the service runs, and `/healthz` answers.

Rollback:

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -rollback -restart massimo
curl.exe -s http://127.0.0.1:8099/healthz
```

Rollback to a version that cannot run as a service (v0.2.0-alpha.1 has no
Windows service): the service manager gives up on it after 30 s (error 1053,
event 7009),
so this should end after about 35 s and put the previous binary back.
(The two minutes of `-health-timeout` are the other case:
a binary that answers the manager but never reports the new version on `/healthz`.)

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -yes -version v0.2.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz
curl.exe -s http://127.0.0.1:8099/healthz
dir "C:\Program Files\Massimo"
```

Expected: an error ending in "rolling back",
`/healthz` still on the release under test, and `massimo.exe.failed` present.

### 7b. Update from the web UI

The release under test must carry the update from the web UI (docs/update-windows.md);
the install from step 3 is in place with `MASSIMO_API_TOKEN=test-token`.
Install an older release first, so that the channel has something newer:

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -yes -version v0.5.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz
sc.exe qc massimo
curl.exe -s http://127.0.0.1:8099/healthz
```

Expected: the service runs v0.5.0-alpha.1 (the health line says so);
the command line shown by `sc.exe qc` ends in
`-update-request C:\ProgramData\Massimo\update-request -update-status C:\ProgramData\Massimo\update-status.json` (the service definition was written by the release under test and survives the downgrade of the binary).
Note the time.

Then, in a browser on this machine, http://127.0.0.1:8099/ with the token,
System settings → Software:

1. "Check for updates": the text names the newest release as out and the button
   "Update to v…" is there.
   In the API,
   `curl.exe -s -H "Authorization: Bearer test-token" http://127.0.0.1:8099/api/v1/update`
   says `"can_update": true` and no `blocked`.
2. Click the button, confirm.
   Expected: both buttons greyed out, "Update to v… requested …",
   then "Last update: checking for a release", "installing v…",
   "restarting the server", a moment with "Cannot ask the server about updates:
   …", then "Last update: installed v…" and the reload button in the header.
   Reload: the header shows the new version.
   Note how long it took.
3. On disk and in the logs:

   ```powershell
   dir "C:\Program Files\Massimo"
   Get-Content C:\ProgramData\Massimo\update.log
   Get-Content C:\ProgramData\Massimo\update-status.json
   Get-Content C:\ProgramData\Massimo\service.log -Tail 6
   Select-String "update" C:\ProgramData\Massimo\serve.log | Select-Object -Last 3
   Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Service Control Manager'} -MaxEvents 6 | Format-List TimeCreated,Id,Message
   Get-Process massimo
   ```

   Expected: `massimo.exe` and `massimo.exe.prev`,
   no `massimo.exe.failed` and no `.massimo.new.exe`;
   `update.log` with `downloading massimo-windows-amd64.exe …`,
   `installed v… as C:\Program Files\Massimo\massimo.exe (the old one is …\massimo.exe.prev)`, `stop the service massimo`, `running v…`; the status file with `"state": "done"`; `service.log` with `stop requested`, `stopped`, `started as a service`, `running`; `serve.log` with `update requested` and `update helper started pid=…`; two 7036 events (stopped, running); exactly one `massimo` process, the service's (the fake Cube runs as `massimo-windows-amd64`), and no `update-request` file.
4. A second request right after a first one is refused:
   install v0.5.0-alpha.1 once more (the first command above), run step 2 again,
   and in the two seconds after the click send
   `curl.exe -s -o NUL -w "%{http_code}" -X POST -H "Authorization: Bearer test-token" http://127.0.0.1:8099/api/v1/update` from a second window.
   Expected: `409`.
   Afterwards the update finishes as in step 2.
5. From another device on the LAN (step 4): the same as step 2, to see
   the page lose and regain the server over the network.
6. The rollback through the helper,
   by hand as in step 7 (the API cannot name a version):
   `update -yes -version v0.2.0-alpha.1 -restart massimo -health …` must end in
   "rolling back" after about 35 s with `massimo.exe.failed` present;
   then step 2 once more (the button,
   to the newest release) must succeed and `massimo.exe.failed` must be gone afterwards.
7. Optional, for the rollback through the web UI:
   the stub of the CI job's stage 2 (docs/update-windows.md,
   section 6) on the PC, a binary built from the branch with a test key,
   `MASSIMO_UPDATE_API` in `C:\ProgramData\Massimo\env`,
   the stand-in listed as the newest release;
   the button must end in "failed and rolled back v0.0.0-ci.3: …",
   the service on the previous version, within about a minute.
8. Windows Security → Protection history must show no event for `massimo.exe`,
   `.massimo.new.exe` or `massimo.exe.prev` (observation 8 of the first report);
   note it either way.

For the report: the times of steps 2 and 6, the contents of
`update.log` and the status file, and the event IDs.

### 8. Reboot (only with the owner's OK)

After a reboot, `sc.exe query massimo` must say RUNNING without anyone logging in.
The fake Cube from step 2 is gone after the reboot, so start it again;
the service reconnects by itself (see `serve.log`).

### 9. Remove

```powershell
.\massimo-windows-amd64.exe install -remove
sc.exe query massimo
dir "C:\Program Files\Massimo", C:\ProgramData\Massimo
.\massimo-windows-amd64.exe install -remove -purge
netsh advfirewall firewall show rule name=Massimo
```

After `-remove`, the service,
the program folder and the firewall rule are gone and `C:\ProgramData\Massimo`
is still there.
After `-purge`, that folder is gone too, with `update.log`,
`update-status.json` and any `update-request`.
Stop the fake Cube (`Stop-Process -Name massimo-windows-amd64`) and delete
`$env:USERPROFILE\massimo-test`.

## Report

One block per step:

- **OK / FAIL**
- for a FAIL: the command, the full output, and the last lines of
  `service.log` and `serve.log`
- for any failure around the service manager:
  `Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Service Control Manager'} -MaxEvents 10 | Format-List TimeCreated,Id,Message`

Also state the Windows version (`winver`, or `[Environment]::OSVersion`),
whether Defender or SmartScreen complained, and the network profile.
