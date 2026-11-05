# Testing the Windows service on a real PC

A test plan for a person or an agent at a Windows machine. `massimo
install` as a Windows service and `massimo update -restart massimo`
exist since v0.3.0-alpha.1 (packaging/README.md, "Windows") and have
so far only run on a GitHub CI runner. This plan exercises them on a
real PC against a fake Cube. Do not change the code during the test;
bugs go into the report.

What install does on Windows:

- binary: `C:\Program Files\Massimo\massimo.exe`
- settings and logs: `C:\ProgramData\Massimo\` (`env`, `serve.log`,
  `service.log`), readable only by SYSTEM and Administrators
- service `massimo`: automatic start, runs as LocalSystem, restarted
  after 10 s on failure; command line
  `massimo -env C:\ProgramData\Massimo\env serve -listen :8099 -log …`
- firewall rule "Massimo" for the program, on private and domain networks

## Ground rules

- Run in a terminal opened with **Run as administrator**. Install and
  update need it.
- **Do not connect to a real Cube unless the owner says so.** A Cube
  accepts exactly one client, and the Raspberry Pi serves the house
  Cube. Test against the fake Cube (step 2).
- No secrets in the report: no keys, IVs, tokens, or the contents of
  `env`.
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
  only `NT AUTHORITY\SYSTEM` and `BUILTIN\Administrators`, nothing
  inherited.
- In a normal, **non**-elevated PowerShell:
  `Get-Content C:\ProgramData\Massimo\env` must be refused.
- `netsh advfirewall firewall show rule name=Massimo` shows the rule.
  Open `http://<this machine's IP>:8099/` from another device (the UI
  asks for the token `test-token`). Note the machine's network profile
  (`Get-NetConnectionProfile`): the rule does not apply on "Public"
  networks.
- In the Services console (`services.msc`), the service is "Massimo"
  with Automatic start, and the "Recovery" tab says "Restart the Service".

### 5. Stop, start, install again

```powershell
sc.exe stop massimo;  sc.exe query massimo
sc.exe start massimo; sc.exe query massimo
.\massimo-windows-amd64.exe install
curl.exe -s http://127.0.0.1:8099/healthz
```

Installing again keeps the settings ("exists; kept it") and restarts
the service.

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

Reinstalling the same release with `-version` exercises the whole
path: download, check, rename of the running `.exe`, restart, health
check.

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -check
& "C:\Program Files\Massimo\massimo.exe" update -yes -version v0.5.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz
dir "C:\Program Files\Massimo"
curl.exe -s http://127.0.0.1:8099/healthz
```

Expected: "installed v0.5.0-alpha.1", `massimo.exe.prev` exists, the
service runs, and `/healthz` answers.

Rollback:

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -rollback -restart massimo
curl.exe -s http://127.0.0.1:8099/healthz
```

Rollback to a version that cannot run as a service (v0.2.0-alpha.1
has no Windows service): this should time out after about two minutes
and then put the previous binary back.

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -yes -version v0.2.0-alpha.1 -restart massimo -health http://127.0.0.1:8099/healthz
curl.exe -s http://127.0.0.1:8099/healthz
dir "C:\Program Files\Massimo"
```

Expected: an error ending in "rolling back", `/healthz` still on the
release under test, and `massimo.exe.failed` present.

### 8. Reboot (only with the owner's OK)

After a reboot, `sc.exe query massimo` must say RUNNING without anyone
logging in. The fake Cube from step 2 is gone after the reboot, so
start it again; the service reconnects by itself (see `serve.log`).

### 9. Remove

```powershell
.\massimo-windows-amd64.exe install -remove
sc.exe query massimo
dir "C:\Program Files\Massimo", C:\ProgramData\Massimo
.\massimo-windows-amd64.exe install -remove -purge
netsh advfirewall firewall show rule name=Massimo
```

After `-remove`, the service and the program folder are gone and
`C:\ProgramData\Massimo` is still there. After `-purge`, that folder is
gone too, and so is the firewall rule. Stop the fake Cube
(`Stop-Process -Name massimo-windows-amd64`) and delete
`$env:USERPROFILE\massimo-test`.

## Report

One block per step:

- **OK / FAIL**
- for a FAIL: the command, the full output, and the last lines of
  `service.log` and `serve.log`
- for any failure around the service manager:
  `Get-WinEvent -FilterHashtable @{LogName='System'; ProviderName='Service Control Manager'} -MaxEvents 10 | Format-List TimeCreated,Id,Message`

Also state the Windows version (`winver`, or
`[Environment]::OSVersion`), whether Defender or SmartScreen complained,
and the network profile.
