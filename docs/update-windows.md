# Updates from the web UI on Windows

The design for starting a software update of the Windows service from
the web UI (System settings → Software, the Update button) and from the API
(`POST /api/v1/update`), the way it works on Linux and macOS.

Status: implemented on 2026-11-06 as option A with the nine positions of section
9 (cmd/massimo/update_helper_windows.go, the CI job's stages 1 and 2);
the test on a real PC (section 7,
docs/testing-windows.md step 7b) is still to do.
Where the implementation goes beyond the text below:
the helper's environment keeps the test-only `MASSIMO_UPDATE_API` besides
`MASSIMO_GITHUB_TOKEN` (stage 2 needs it in the helper),
and the client sends no GitHub token to an API base set that way;
a job object that would kill the helper reaches `blocked` through `UpdateOptions.Blocked`;
a 409 from the starter's own guard leaves the request file alone (the running
helper may not have taken it yet);
the stage 2 stand-in's rollback is checked for event 7000, 7009 or 7034,
since a process that exits at once may not end in 7009;
and the CI test's second POST, which must be refused,
goes on the first's connection from the same curl,
since in stage 2 the helper stops the service before a second curl process
is up (GUIDELINES/project/CI.md, "Runner Constraints").
Since 2026-11-07 the restart from the API (`POST /api/v1/restart`)
uses the same starter for `massimo restart -service massimo`, detached,
its output in `service.log`.

The text below is the design as written before it.
Written on 2026-11-06 from the code on `main` after v0.5.0-alpha.1 (internal/update,
internal/install, cmd/massimo, internal/server/update_api.go,
internal/server/ui/update.js,
.github/workflows/ci.yml) and from the test of the Windows service on a real PC
(docs/windows-test-2026-11-05.md).
Read packaging/README.md ("Updating", "From the web UI", "Windows") first;
this document does not repeat what is there.

Sections: 1 today, 2 what every option must keep, 3 the options,
4 the recommendation, 5 the changes per file, 6 the CI test,
7 the test on a real PC, 8 the two loose ends of the test report,
9 open questions.

## 1. Today

### What works from an administrator prompt

With the installed binary, in a PowerShell opened with "Run as administrator":

```powershell
& "C:\Program Files\Massimo\massimo.exe" update -check
& "C:\Program Files\Massimo\massimo.exe" update -restart massimo -health http://127.0.0.1:8099/healthz
& "C:\Program Files\Massimo\massimo.exe" update -rollback -restart massimo
```

`runUpdate` in cmd/massimo/update.go does, in this order:
the release list from GitHub's API,
the manifest and its signature checked against the key compiled in
(internal/update/release-key.pem),
the download of `massimo-windows-amd64.exe` into
`C:\Program Files\Massimo\.massimo.new.exe` (`stagedPath` keeps the `.exe` so
that Windows runs it) with its SHA-256 checked against the manifest,
the smoke test (`<new binary> version` in an empty temporary directory must
print the tag), the swap (`swapRenaming`:
`massimo.exe` is renamed to `massimo.exe.prev`,
the staged file to `massimo.exe`;
Windows lets a running executable be renamed but not replaced),
the restart (`install.RestartService`: `Control(Stop)`,
wait for `Stopped` for up to a minute, `Start`,
wait for `Running` for up to a minute),
and with `-health` the poll of `/healthz` every two seconds for
`-health-timeout` (two minutes) until its `version` carries the tag.
Otherwise `update.Rollback(target, FailedPath)` puts `massimo.exe.prev` back,
keeps the failed binary as `massimo.exe.failed` and restarts the service again.
`-rollback` swaps `massimo.exe` and `massimo.exe.prev` (a second one goes forward again),
`-version <tag>` installs a given release, `-yes` skips the question.
The test report's step 7 saw all of it: four seconds for an update,
one second for a rollback,
35 s for a release that cannot run as a service (the service manager gives up
after 30 s with error 1053 and event 7009; `update` rolls back on that error).

Running `install` again from a newer downloaded binary also replaces the
binary (the service is stopped first) and keeps the settings.

### What the API refuses and why

`updateBlocked` in internal/server/update_api.go has `GOOS == "windows"` as its
first case,
with the reason "updates are not installed automatically on Windows yet".
So `GET /api/v1/update` answers `can_update: false` and that `blocked` text,
the web UI hides the Update button and shows "Updating from here is not possible:
updates are not installed automatically on Windows yet."
(the `update.blocked` text in i18n.js), and `POST /api/v1/update` answers 403.
`TestUpdateEndpoints` skips on Windows for the same reason.

The reason behind the refusal: on Linux the server runs as the user `massimo`,
cannot write `/usr/local/bin`,
and asks a root helper by writing `/var/lib/massimo/update-request`;
`massimo-update.path` (`PathExists=`) starts `massimo-update.service`,
on macOS launchd's `WatchPaths` starts `ch.metaworx.massimo-update`.
On Windows nothing watches a file:
the service's command line (`install.ServiceArgs`) is
`-env C:\ProgramData\Massimo\env serve -listen :8099 -log C:\ProgramData\Massimo\serve.log` with no `-update-request`, and `serve -update-status` defaults to `update.DefaultStatusPath`, the Unix path `/var/lib/massimo-update/status.json`, which on Windows names `C:\var\lib\massimo-update\status.json` and never exists.

### What exists already and is not Linux-specific

The protocol between server and helper is three things,
all of them platform-neutral code in internal/update/helper.go and cmd/massimo/update.go:

- the request file: `update.WriteRequest(path, channel)` writes the channel name
  and nothing else, atomically (`.tmp` plus rename);
  `update.TakeRequest` in the helper reads it (no symbolic link, a regular file,
  at most 256 bytes,
  a valid channel name or nothing) and removes it after closing,
  "which Windows needs", as its comment says;
- the status file: `update.Status` with the states `checking`, `installing`,
  `restarting`, `done`, `up-to-date`, `failed`, `rolled-back`, `from`, `to`,
  `channel`, `message` and `time`;
  `Running()` is true for the first three unless the file is older than fifteen
  minutes ("an older one died"); `WriteStatus` writes it atomically,
  `ReadStatus` reads it,
  the server includes it as `helper` in `GET /api/v1/update`;
- the helper mode of the update command: `massimo update -request
  <file> -status <file> -restart <service> -health <url>` takes the
  request (`-yes` implied), does the update above and writes every
  state to the status file.

The web UI needs nothing Windows-specific either. update.js disables
both buttons (`setUpdateBusy`), posts, then polls `GET /api/v1/update`
every three seconds for up to five minutes (`followUpdate`), with a
thirty-second grace period after the request for the moment before the
helper has written its first state; a poll that fails (the server is
restarting) counts as busy; after a reload while the helper works,
`renderUpdate` sees `helper.state` and resumes the polling. app.js's
`checkVersion` compares `/healthz`'s `version` with the one the page
was loaded with and reloads the page when nothing is open, or shows the
reload button (the Settings page counts as open, `somethingOpen`).

### The Windows service

What `install` makes (internal/install/windows.go, windows_args.go,
confirmed by the test report's steps 3 and 4):
`C:\Program Files\Massimo\massimo.exe`; `C:\ProgramData\Massimo\` with `env`,
`serve.log` and `service.log`,
the folder's ACL reduced to SYSTEM and the Administrators
(`icacls … /inheritance:r /grant:r *S-1-5-18:(OI)(CI)F *S-1-5-32-544:(OI)(CI)F`);
the service `massimo` ("Massimo"), automatic start,
running as LocalSystem (the default account of `mgr.CreateService`),
failure actions restart after 10 s three times with a reset period of a day;
the firewall rule "Massimo" for the program path on private and domain networks.
The process answers the service manager itself (cmd/massimo/service_windows.go):
`svc.IsWindowsService()`, then `svc.Run` in a goroutine;
on `Stop` or `Shutdown` the handler reports stop pending with a wait hint of 20 s,
ends `serve`'s context and waits for `serve` to return,
which shuts the HTTP server down,
writes the log's pending events and closes the store,
then returns, and the goroutine calls `os.Exit(0)`.
A `serve` that has not returned after 20 s is ended all the same.
`service.log` is the process's stderr while it runs as a service
(`os.Stderr = f`) plus the lines `started as a service`, `running`,
`stop requested`, `serve stopped`, `stopped`.

Two facts drive the options.
The service runs as LocalSystem, so unlike on Linux there is no privilege gap:
the service process may write `C:\Program Files\Massimo`,
talk to the service manager and start processes as SYSTEM.
And a process cannot restart its own service: once it has stopped,
nobody is left to call `Start`.
The update therefore needs a second process whose life does not depend on the service's,
and the question is only who starts it and who its parent is.

## 2. What every option must keep

- Who may trigger an update: `POST /api/v1/update` with the API token,
  as today (`startUpdate` answers 403 from `updateBlocked` when there is no token).
  The request carries the channel name and nothing else;
  the helper finds the newest release itself,
  verifies the signed manifest against the compiled-in key and the binary's
  SHA-256 against the manifest,
  so the most the API can do is install the newest signed release on the channel.
  No version choice, no file, from the API.
- The swap and the rollback as they are: `.prev`, the health check, `.failed`;
  `update.Swap`, `update.Rollback` and `install.RestartService` unchanged.
- The contract with the UI: the status file and its states, the request file;
  nothing in update.js or i18n.js.
- A log an administrator can read, and the service manager's own record
  in the System event log.
- No new dependency (golang.org/x/sys is there), no second binary,
  no installer framework.

## 3. The options

### A. The server starts the helper itself

**How it works.**
`POST /api/v1/update` does what it does today (`updateBlocked`,
the release check, 409 while a run is in progress,
`update.WriteRequest`) and then, on Windows,
starts the helper as a detached child process and answers 202.
The helper is the installed binary in its existing helper mode:

```
"C:\Program Files\Massimo\massimo.exe" update -request C:\ProgramData\Massimo\update-request -status C:\ProgramData\Massimo\update-status.json -restart massimo -health http://127.0.0.1:8099/healthz
```

It takes the request, finds and verifies the release, downloads,
runs the smoke test, swaps the binary, stops the service (its parent exits),
starts the service, waits for `/healthz` and rolls back if that fails:
the sequence of section 1, unchanged, with the status file written at each step.
The service's command line gains the two paths:

```
"C:\Program Files\Massimo\massimo.exe" -env C:\ProgramData\Massimo\env serve -listen :8099 -log C:\ProgramData\Massimo\serve.log -update-request C:\ProgramData\Massimo\update-request -update-status C:\ProgramData\Massimo\update-status.json
```

The health URL follows `-listen` with the rule `install -listen` uses for the
Linux drop-in and the macOS plist (`healthURL` in internal/install/listen.go:
`:8099` gives `http://127.0.0.1:8099/healthz`,
`192.168.1.5:8099` gives `http://192.168.1.5:8099/healthz`);
`serve` computes it from its `-listen` flag,
so `install -listen` needs no further change.
With TLS the URL is `https://…`,
and the helper verifies its certificate unless the host is the loopback
or an address one of the box's interfaces has when the check starts,
since the certificate may be self-signed or for a name.
Request, status and the helper's log live next to the settings in
`C:\ProgramData\Massimo`, readable by SYSTEM and the Administrators only.

**Starting the child.**
`os/exec` with
`SysProcAttr{CreationFlags: DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP, HideWindow: true}` (both flags and `HideWindow` are in Go's `syscall.SysProcAttr`; the constants are in golang.org/x/sys/windows).
`DETACHED_PROCESS` gives the child no console: a service has none,
and without the flag Windows makes one for a console program,
invisible in session 0 and pointless;
`CREATE_NEW_PROCESS_GROUP` puts it into its own process group,
so that a control event meant for the parent cannot reach it (moot without a console,
harmless with the flag).
The standard input is empty,
standard output and error go to `C:\ProgramData\Massimo\update.log`,
opened for appending (the helper prints its progress lines there:
`downloading …`, `installed …`, `stop the service massimo`, `running v…`,
and `update: … - rolling back`).
The working directory is `C:\ProgramData\Massimo`.
The environment is the server's own environment without the `MASSIMO_*` variables,
plus `MASSIMO_GITHUB_TOKEN` when the server has it
and `MASSIMO_ENV_FILE=C:\nonexistent\.env`:
`env.LoadFile` puts every line of the settings file into the service's process
environment, Cube keys and API token included,
and the helper needs none of them;
a missing env file is not an error (`LoadFile` returns nil on `IsNotExist`),
the same trick as the Linux unit's `/nonexistent/.env`.
The server keeps the `*exec.Cmd`,
calls `Wait` in a goroutine and logs the exit when it sees one (it usually does not:
the helper stops the server first),
and refuses a second start while that goroutine has not returned,
which closes the gap between the request and the helper's first status write
that the UI's grace period papers over.

Two things to check at run time, both cheap.
First, job objects: a job with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` kills its
processes when the last handle closes,
and a child inherits its parent's job unless it breaks away.
The service manager does not put the services it starts into a job object,
so the service is expected not to be in one;
the starter asks anyway with `IsProcessInJob(GetCurrentProcess(), NULL, &inJob)`
(not wrapped by x/sys/windows:
a `NewLazySystemDLL("kernel32.dll").NewProc` call of three lines,
with `windows.CurrentProcess()` for the handle) and, if so,
`windows.QueryInformationJobObject(0, JobObjectExtendedLimitInformation, …)` (a
null handle names the caller's own job) for the limit flags:
with `JOB_OBJECT_LIMIT_BREAKAWAY_OK` it adds `CREATE_BREAKAWAY_FROM_JOB`,
with `JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK` children are out anyway,
and with `KILL_ON_JOB_CLOSE` and neither it reports the reason in `blocked`
instead of starting a helper that would die with the server.
The creation flags, `QueryInformationJobObject`,
the `JobObjectExtendedLimitInformation` class and the `JOB_OBJECT_LIMIT_*` flags
are in x/sys/windows (checked in the v0.45.0 copy of the module;
go.mod pins v0.41.0).
Second, handle inheritance:
the child must not hold the server's listening sockets,
or the restarted server could not bind `:8099`,
`:62910` and UDP 23272 and the health check would fail.
Go creates sockets with `WSA_FLAG_NO_HANDLE_INHERIT` (net/sock_windows.go)
and `os/exec` passes only the child's standard handles through
`PROC_THREAD_ATTRIBUTE_HANDLE_LIST` (syscall/exec_windows.go),
so nothing else is inherited;
the CI test proves it by the restarted server answering on the same port.

The helper runs from `massimo.exe` and renames that very file under itself.
That is what happens by hand today:
the test report's step 7 ran `"C:\Program Files\Massimo\massimo.exe" update …`,
which renamed its own image aside,
installed the new one and restarted the service.
Windows keeps the mapping of a renamed image.
The firewall rule names the program path, not a hash,
so the new binary at the same path is covered (unlike macOS,
where the application firewall follows the cdhash).

**Privilege and security.**
No privilege changes hands:
server and helper are the same binary running as SYSTEM.
The API token is the one gate, as on Linux.
The request file, the status file and `update.log` are in a folder only SYSTEM
and the Administrators can write,
so nobody else can plant a request or forge a status;
`TakeRequest`'s checks (no symbolic link, a regular file,
a channel name) still apply.
The helper verifies the release against the compiled-in key as it does today.
The helper's environment carries no Cube key and no API token (above).
The starter executes `os.Executable()` of the running server,
`C:\Program Files\Massimo\massimo.exe`, a path only Administrators can write,
with arguments built from the server's own flags, never from the request.
The Linux rule that the helper reads nothing the server's user can write has no
meaning here: the server's user is SYSTEM, which can write anything;
what remains of it is that the helper reads nothing it does not need.

**Atomicity and rollback.**
Exactly today's: the download goes to a staged file next to the target,
the swap is two renames, `.prev` is kept,
the health check and the rollback follow.
The service manager's 30 s connect timeout (error 1053,
event 7009) ends the case of a binary that cannot run as a service after about
35 s (observation 4, section 8);
a binary that answers the manager but never reports the new version on
`/healthz` (serve fails later, for instance it cannot bind a port;
the manager's failure actions restart it every 10 s meanwhile) is given up after
`-health-timeout`, two minutes.
A rollback restarts the service twice in all;
the stop of a running server is a clean stop, not a failure,
and does not count towards the failure actions.

**What the user sees.**
The confirmation dialog ("Install v… and restart the server?
It is offline for a short while; if the new version does not come up,
the old one is put back."), both buttons greyed out, "Update to v… requested …",
then the helper's states as "Last update: checking for a release",
"installing v…", "restarting the server".
The download takes a few seconds on a normal line (4 s for the whole update in
the test report); then the server is gone for one to two seconds (stop, start,
the listeners up).
During that gap each poll fails and the Software section's text says "Cannot ask
the server about updates:
…" (the browser's fetch error) until the next poll succeeds;
the event stream drops and reconnects with its backoff (app.js, `retryEvents`).
The new server reads the same status file, answers "installed v…",
the buttons come back, and `checkVersion` sees the new version:
on the Settings page `somethingOpen()` is true,
so the reload button appears rather than an automatic reload.
After a rollback the text says "failed and rolled back v…:
the server did not come up with v… within 2m0s (…)" or the 1053 message,
and the old version keeps running.
Roadmap item 14 (a notice when the server is gone) would make the gap more explicit;
nothing here depends on it.

**Logging.**
Four places, each for one view: `update.log` for the helper's own lines (new;
the counterpart of `journalctl -u massimo-update` and `/var/log/massimo-update.log`);
`serve.log` for the server's `update requested from=… to=… channel=…` (exists)
and a new `update helper started pid=…` line, plus the new server's start;
`service.log` for `stop requested`, `stopped`, `started as a service`,
`running`;
the System event log for the service manager (events 7036 "entered the
stopped/running state", 7009 and 7000 on a failed start).
`update.log` is never rotated, like `serve.log` on Windows;
a run adds a dozen lines, so that is years of updates.

**Defender and SmartScreen.**
Go's HTTP client writes no Mark-of-the-Web,
so neither the staged download nor the installed binary meets SmartScreen;
a process started by a service with `CreateProcess` is not started by the shell
and is not checked either;
the service manager does not consult SmartScreen (observation 8).
What remains is Defender's real-time scan of a new executable in
`C:\Program Files\Massimo`:
it scanned every binary of the test without a detection.
A false positive would quarantine the staged file,
and the smoke test or the swap fails with "failed" in the status and the old
binary untouched.
The one bad moment is a quarantine between the swap and the start:
`Start` fails with "file not found",
and the rollback's `swapRenaming` fails too because `massimo.exe` is gone (it
renames the target aside first);
the service is then without a binary until an administrator renames
`massimo.exe.prev` back.
`Rollback` could tolerate a missing target (rename `.prev` into place directly)
for a few lines; listed under the changes as optional.

**Failure modes.**

| Failure | What happens | What the user sees |
|---|---|---|
| The starter cannot create the process (an unexpected job, a wrong path) | 500, the request file removed, the reason logged in serve.log | "Update not started: …" |
| The release check or the download fails (no network, GitHub's rate limit, a wrong SHA-256) | the helper writes `failed` with the message and exits; nothing is swapped | "Last update: failed v…: …", the buttons come back |
| The smoke test fails (the binary does not run here, or says another version) | `failed`, the staged file discarded | the same |
| The new binary cannot run as a service | `Start` fails after 30 s with 1053; rollback; `rolled-back` | "failed and rolled back v…: start the service massimo: …" after about 35 s |
| The new binary starts but never reports its version on `/healthz` | rollback after two minutes; `rolled-back` | the same, after two minutes plus two restarts |
| The helper dies before the swap | the status stays at `checking` or `installing`; `Running()` is false after fifteen minutes; the UI stops following after five | "installing v…" until the next run overwrites it; the server keeps running |
| The helper dies after the swap, before the stop | the new binary is in place, the old server keeps running from its renamed image; the next start (a reboot) runs the new binary without a health check | the same stale state; `massimo.exe.prev` is there for `update -rollback` |
| The helper dies between the stop and the start | the service stays stopped: a clean stop is no failure, so the failure actions do not restart it; `sc.exe start massimo` or a reboot brings it back | the page loses the server for good |
| A second `POST` while a run is in progress | 409 "an update is already running" from the status file, or from the starter's own guard in the moment before the first status write | "Update not started: an update is already running (installing)" |
| The server is restarted (by hand, a reboot) while the helper works | the helper carries on; it is not the server's child in any way that matters | as above |

The third-from-last row is the one difference from Linux worth knowing:
`systemctl restart` is one call, `RestartService` is a stop,
a wait and a start in the helper, a window of about a second.
A helper that has just downloaded,
verified and swapped a binary without trouble does not die in that second for a
reason of its own; what can kill it there is an administrator or an antivirus,
and `sc.exe start massimo` is the remedy either way.

**Install and remove.**
`install.ServiceArgs` gains the two flags;
the `WinPaths` struct gains `Request`, `Status` and `UpdateLog`.
Nothing else: no second service, no task, no rule.
`install -remove` already removes the whole program folder (`.prev`
and `.failed` with it), and `-purge` the data folder with the three new files.
`-no-update-helper`, refused on Windows today ("-root and -no-update-helper are
for Linux"), can be accepted and leave the two flags out,
which hides the button, as it does on Linux and macOS.

**The CI test.**
The existing Windows job, extended: section 6.

**Verdict.**
The smallest change: one new file in cmd/massimo with the process start,
a callback in `UpdateOptions`, two flags in the service command line,
and the helper exactly as it runs by hand today.
Its one open question, whether the child outlives the service on every Windows,
is answered by the CI job in minutes and by the test on a real PC,
and if the answer were no on some edition the switch to option B is contained in
that one file.

### B. A second service, `massimo-update`

The mirror of the Linux and macOS helpers: a second Windows service,
`massimo-update` ("Massimo update helper"),
running as LocalSystem with the command line

```
"C:\Program Files\Massimo\massimo.exe" update -request C:\ProgramData\Massimo\update-request -status C:\ProgramData\Massimo\update-status.json -restart massimo -health http://127.0.0.1:8099/healthz
```

Two shapes:

- **B1, always running and watching.**
  Windows has no path units:
  the helper would have to poll the request file every few seconds or use
  `ReadDirectoryChangesW` on the data folder,
  and `update -request` would become a loop.
  A process that runs all year for a job that happens once a month,
  and a second service in the Services console that is "Running" for no visible reason.
- **B2, started on demand.**
  Start type manual; the server, after writing the request,
  opens the service with `mgr.OpenService` and calls `Start()`, one call;
  the helper takes the request, does the update and exits.
  The service manager is its parent, the `massimo` service is unrelated to it,
  so the questions of option A (job objects, inherited handles,
  the parent's exit) do not arise;
  a second `Start` while it runs fails with `ERROR_SERVICE_ALREADY_RUNNING`,
  a natural 409; `sc.exe query massimo-update` shows it;
  the event log records each start.

What B2 costs: `install` creates and updates a second service (and rewrites its
`-health` on `install -listen`), `install -remove` stops and deletes it,
the CI job and the README explain two services,
and the update command has to behave as a service process:
the `init` in cmd/massimo/service_windows.go already answers the service manager
for whatever command runs as a service (`svc.Run`'s name is ignored for an
own-process service), but it only reports `Running` and waits for a stop,
so a helper that simply returns from `main` ends with event 7034 "terminated
unexpectedly" and, if failure actions were set, their restart;
it needs a small hook that reports `StopPending` and `Stopped` before the exit
(about fifteen lines).
A service has no console:
the helper's stdout lines are lost unless `runUpdate` gets a `-log` flag
or prints progress to stderr, which `init` already redirects to `service.log`.
Its environment is the system's:
the GitHub token would go into the service's `Environment` registry value or the
helper would read the settings with `-env`.
Privilege, security, atomicity, the UI,
Defender and the failure modes are those of option A,
minus the two run-time checks,
plus the second service's own ways of being wrong (disabled by an administrator,
deleted by hand,
a stale `-health` after a port change done by editing the first service).

Verdict: the right fallback if a Windows edition kills the detached child of option A,
and otherwise more install surface and more explaining for the same result.

### C. A scheduled task

A task created at install and run on demand by the server:

```
schtasks /Create /TN "Massimo\Update" /TR "\"C:\Program Files\Massimo\massimo.exe\" update -request C:\ProgramData\Massimo\update-request -status C:\ProgramData\Massimo\update-status.json -restart massimo -health http://127.0.0.1:8099/healthz" /SC ONCE /ST 00:00 /SD 01/01/2000 /RU SYSTEM /RL HIGHEST /F
schtasks /Run /TN "Massimo\Update"
schtasks /Delete /TN "Massimo\Update" /F
```

Like B2 it gives the helper a parent that is not the server (the Task Scheduler service),
with a history in the Task Scheduler library, and no job-object question.
Against it: a task needs a trigger even when it is only ever run on demand
(hence the `ONCE` in the past, or an XML definition with `/XML`);
`/TR` is limited to 261 characters and the command above is about 230,
so a flag more and it no longer fits;
the server has no Go API for the scheduler without COM and would shell out to
`schtasks.exe` and parse its text;
the task's output goes nowhere unless the command is wrapped in
`cmd /c "… >> update.log 2>&1"` with one more level of quoting;
the task's environment and working directory are the scheduler's;
group policy can disable the Task Scheduler for non-system tasks,
and some tidy-up tools delete unknown tasks;
and it is one more thing to explain and to remove.
It trades option A's one checkable question for a tool with several.

### D. The service manager's own recovery, or the service re-executing itself

Three shapes, none of which fit:

- **The failure action "run a program"** (`SetRecoveryCommand`).
  The action runs on a failure,
  so the server would have to crash on purpose to trigger it,
  the program would run on every genuine crash as well,
  and the first action today is the restart that keeps the service alive.
  A misuse of a safety feature.
- **The service swaps the binary itself and exits with a failure**,
  so that the manager's restart after 10 s runs the new binary.
  No helper at all, and it is what `update` without `-restart` plus a crash would do.
  But nobody checks the health and nobody rolls back:
  a new binary that cannot run as a service leaves the manager restarting it
  every 10 s (the last failure action applies to all later failures) and the box
  without heating control until an administrator runs `update -rollback`;
  the Cube connection and the UI are gone for the 10 s delay plus the 30 s timeout;
  and each update is logged as a crash (event 7034) and counts towards the
  failure actions.
  The rollback is the point of the whole mechanism.
- **The service stops itself.**
  Once it has reported `Stopped` nobody is left to call `Start`;
  the manager restarts a service only on a failure, and a clean stop is none.

### Side by side

| | A: the server starts the helper | B2: a demand-start service | C: a scheduled task | D |
|---|---|---|---|---|
| New install objects | none (two flags) | one service | one task | none |
| The helper's parent | the server (detached) | the service manager | the Task Scheduler | — |
| To verify on a real Windows | the child outlives the service | the stop report at the end | `schtasks` quoting, the 261 characters | — |
| Logging of the helper | `update.log` | `service.log` (stderr only) and the event log | needs a `cmd /c` wrapper | — |
| Second request while one runs | the status file, plus the server's guard | `ERROR_SERVICE_ALREADY_RUNNING` | `schtasks` says "already running" | — |
| Rollback | as today | as today | as today | none |
| Code | one file in cmd/massimo, a callback, two flags | the above plus a second service in install and remove, a stop hook, a `-log` flag | the above plus shelling out to `schtasks` | — |

## 4. Recommendation

Option A: the server starts the helper itself.
It adds nothing to the install,
nothing to the Services console and nothing to the README's list of things to remove;
the helper is the command that already works by hand on the same machine,
with the same swap, restart, health check and rollback;
the request and status files and the UI stay as they are on Linux and macOS,
so the three platforms differ only in who starts the helper (a path unit,
launchd, the server).
What it relies on, a detached child surviving its parent's exit,
is standard Windows behaviour, is checked at run time,
and is proved by the CI job on every push and by the test on a real PC.
If that proof fails on some edition, option B2 is the fallback,
and the switch is confined to the starter function.

## 5. The changes per file

**internal/update/helper.go.**
The package comment at the top ("a systemd path unit sees the file") gains the
two other triggers: launchd on macOS, the server itself on Windows.
`DefaultRequestPath` and `DefaultStatusPath` stay the Unix paths (used by the
`serve` flags' defaults and the documentation);
the Windows paths come from `install.WindowsPaths`,
which `serve` passes explicitly.
Optional: `WriteStatus` retries its final rename a few times (ten times 50 ms) on Windows,
since Go opens files with `FILE_SHARE_READ | FILE_SHARE_WRITE`
and not `FILE_SHARE_DELETE`,
so a rename over the status file in the millisecond the server has it open for
`ReadFile` fails;
today a failed write is only reported on stderr and the state is lost until the next one.

**internal/update/install.go.**
`Swap` removes `FailedPath(target)` once the new binary is in place (section 8,
observation 5).
Optional: `Rollback` tolerates a missing target (section 3 A,
Defender). internal/update/open_other.go's comment ("the update helper is Linux
only for now") goes.

**internal/install/windows_args.go.**
`WinPaths` gains `Request` (`…\Massimo\update-request`),
`Status` (`…\Massimo\update-status.json`) and `UpdateLog` (`…\Massimo\update.log`);
`ServiceArgs` appends `-update-request <Request> -update-status <Status>` unless
the helper is left out.
`ListenFromCommandLine` is unaffected. internal/install/windows_args_test.go's
`TestWindowsLayout` checks the new command line.

**internal/install/listen.go.**
`healthURL(host, port)` becomes exported, `HealthURL`,
so that `serve` can build the helper's `-health` from `-listen` with the rule
the drop-ins and plists use.

**internal/install/windows.go.**
Nothing structural.
`InstallWindows` passes `in.NoUpdateHelper` to `ServiceArgs` if the flag is
accepted on Windows; the package comment mentions the helper start.
`RemoveWindows` removes the data folder as a whole already.

**cmd/massimo/update_helper_windows.go (new) and update_helper_other.go (new).**
`updateHelperStarter(exe, request, status, health, log string) func() error`:
on Windows it builds the command line of section 3 A,
the environment (the server's without `MASSIMO_*`,
plus `MASSIMO_GITHUB_TOKEN` and `MASSIMO_ENV_FILE`), the log file,
the creation flags with the job-object check, starts the process, logs the PID,
waits in a goroutine and guards against a second start;
elsewhere it returns nil.
`install.ServiceName` is the `-restart` argument.

**cmd/massimo/serve.go.**
After parsing, on Windows and when `-update-request` is set,
`UpdateOptions.Start = updateHelperStarter(os.Executable(), *updateRequest, *updateStatus, install.HealthURL(*listen), <log next to the status file>)`.
The `-update-status` flag's help text says that on Windows `install` passes the path.

**cmd/massimo/update.go.**
No change to the helper mode.
The usage text ("the update helper that lets the web UI start an update on Linux
and macOS") drops "on Linux and macOS".
The `.failed` removal lives in `update.Swap`, not here.

**cmd/massimo/service_windows.go.**
Nothing for option A.
(B2 would need the stop report at the end of a service process.)

**cmd/massimo/install.go.**
`-no-update-helper` accepted on Windows,
its help text "Linux and macOS:" widened;
the usage text's Windows line mentions that the web UI can start updates.

**internal/server/update_api.go.**
`UpdateOptions` gains `Start func() error`,
"starts the helper once the request is written;
nil where the operating system does that (the systemd path unit,
launchd's WatchPaths)".
`updateBlocked` loses its Windows case.
`startUpdate` calls `Start` after `WriteRequest`;
a `update.ErrHelperRunning` from it is a 409,
any other error a 500 with the request file removed;
the log line `update requested` stays.
The message for a missing `Request` can say on Windows "start updates from the
installed service (massimo install)" rather than pointing at
packaging/README.md's helper. internal/server/update_api_test.go:
the Windows skip goes;
a test with a recording `Start` checks that it is called once per accepted request,
that its error gives 500 and removes the request file,
and that `ErrHelperRunning` gives 409.

**internal/server/ui/update.js, i18n.js.**
Nothing.
The `update.blocked` text is still used for the other reasons.

**docs/api-v1.md.**
The two mentions of Windows as a 403 reason go;
the POST paragraph names the three starters.

**packaging/README.md.**
"Windows": the last paragraph ("The web UI cannot start updates on Windows
yet…") becomes a description of the server starting the helper,
the three files in `C:\ProgramData\Massimo` and `update.log`;
the 35 s versus two minutes (section 8).
"From the web UI": the last sentence gains Windows next to macOS.
The introduction's list of files is unchanged (no packaging file for Windows).

**README.md.**
"Windows", "As a service": the Update part says the button works as on Linux,
through `update.log`; the Documentation list keeps the line for this document.

**docs/testing-windows.md.**
Step 7: the expected time for the v0.2.0-alpha.1 rollback is about 35 s,
not two minutes (section 8); a new step 7b, the update from the web UI,
with the checks of section 7 of this document; step 9:
the firewall rule goes with `-remove` (observation 7).
Both reports are referenced from the plan.

**.github/workflows/ci.yml.**
The Windows job as in section 6.

**docs/roadmap.md, docs/handover-session.md.**
Not touched by this pull request;
the implementing session updates item 8 and 15 and the follow-up list.

## 6. The CI test

The "Windows service" job of ci.yml today builds `massimo.exe` as `v0.0.0-ci.1`
on channel `alpha`, starts the fake Cube,
installs the service with `-listen 127.0.0.1:18099` and a settings file holding
the Cube's address and the token `ci`, checks `/healthz`, installs again,
runs `update -yes -restart massimo -health … -health-timeout 90s` by hand
against the real alpha channel (the newest alpha is always newer than `v0.0.0-ci.1`:
it either installs it, or rolls back when that release cannot run as a service),
and removes everything.
Two additions make it test the design, in two stages.

**Stage 1, the API path against GitHub**,
the shape of the macOS job and enough to prove that the detached helper outlives
the service and that the request, status and log files work.
After the first health check,
with `MASSIMO_GITHUB_TOKEN` in `ci.env` so that the server (and through it the
helper) has it:

```bash
sc.exe qc massimo | grep -- '-update-request'
code="$(curl -sS -o post.json -w '%{http_code}' -X POST -H 'Authorization: Bearer ci' http://127.0.0.1:18099/api/v1/update)"
cat post.json; test "$code" = 202
# A second request in the same moment is refused.
curl -sS -o /dev/null -w '%{http_code}\n' -X POST -H 'Authorization: Bearer ci' http://127.0.0.1:18099/api/v1/update | grep -q 409
for i in $(seq 90); do
  st="$(cat C:/ProgramData/Massimo/update-status.json 2>/dev/null || true)"
  echo "$st" | grep -Eq '"state": "(done|rolled-back|failed)"' && break
  sleep 2
done
echo "$st"; cat C:/ProgramData/Massimo/update.log
for i in $(seq 30); do curl -fsS "$health" && break; sleep 2; done
now="$(curl -fsS "$health")"
if echo "$st" | grep -q '"state": "done"'; then
  echo "$now" | grep -vq 'v0.0.0-ci.1'
  test -f "C:/Program Files/Massimo/massimo.exe.prev"
  "$bin" update -rollback -restart massimo       # back, with the binary that is installed
  for i in $(seq 30); do curl -fsS "$health" && break; sleep 2; done
  curl -fsS "$health" | grep -q 'v0.0.0-ci.1'
else
  echo "$st" | grep -q '"state": "rolled-back"'
  echo "$now" | grep -q 'v0.0.0-ci.1'
  test -f "C:/Program Files/Massimo/massimo.exe.failed"
fi
```

`"$bin"` is `C:/Program Files/Massimo/massimo.exe` as today.
The existing by-hand update stays as the check against the real key and the real API;
the existing removal checks stay, with `update.log`,
`update-status.json` and `update-request` added to the files that must be gone
after `-purge`, and the "Logs" step prints `update.log`.

**Stage 2, a stubbed release**,
so that both outcomes are tested every time whatever the alpha channel holds,
and the rollback goes through the API rather than by hand.
It needs one small code change,
`MASSIMO_UPDATE_API` read by `update.NewClient()` as the API base in place of
`https://api.github.com` (the release files are fetched from whatever URLs the API lists,
so a stub needs no second variable), and in the job:

1. A test key: `openssl genpkey -algorithm ed25519 -out ci-key.pem` and its
   public half written over `internal/update/release-key.pem` before the
   binaries are built,
   so that the binaries of this job accept releases signed with it (the real-key
   binary for the by-hand step of stage 1 is built before the swap).
   Nothing built here is published.
2. Three "releases" in a folder served by `python -m http.server 8123` (the
   runner has Python; any static server does),
   with `repos/metaworx/massimo/releases` holding the JSON list of the GitHub
   shape (`tag_name`, `html_url`,
   `assets` with `name` and `browser_download_url` pointing at
   `http://127.0.0.1:8123/dl/<tag>/…`;
   the `?per_page=100` is dropped by the server) and under `dl/<tag>/` the binary,
   `massimo-release.json` from
   `go run ./internal/update/mkmanifest -tag <tag> -commit ci -channel ci -dir … massimo-windows-amd64.exe`, and `massimo-release.json.sig` made as release.yml makes it (`openssl pkeyutl -sign -inkey ci-key.pem -rawin -in massimo-release.json | base64 -w0`): `v0.0.0-ci.2`, the same source built with `-X main.version=v0.0.0-ci.2`, which comes up healthy; and `v0.0.0-ci.3`, a stand-in of a few lines built in the job (prints `massimo v0.0.0-ci.3 (ci)` for `version`, exits 1 otherwise), which passes the smoke test, cannot be a service and must be rolled back.
   The list holds one of them at a time:
   the file is rewritten between the two runs.
3. `install` again from the test-key `v0.0.0-ci.1` binary,
   built with `-X main.channel=ci` as the stub's `v0.0.0-ci.2` is,
   since a check leaves out unfetched a release whose tag names another channel,
   with `MASSIMO_UPDATE_API=http://127.0.0.1:8123` in `ci.env`;
   then the block of stage 1 twice:
   with `v0.0.0-ci.2` listed it must end in `done` and `/healthz` on `v0.0.0-ci.2`,
   `massimo.exe.prev` present and no `massimo.exe.failed` (the by-hand rollback
   of stage 1 left one, which this successful update removes: observation 5);
   with `v0.0.0-ci.3` listed it must end in `rolled-back` within about 40 s,
   `/healthz` on `v0.0.0-ci.2`, `massimo.exe.failed` present,
   and the System log must show event 7009 for `massimo`.

The stand-in, for the job's shell:

```go
package main

import ("fmt"; "os")

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("massimo v0.0.0-ci.3 (ci)")
		return
	}
	os.Exit(1)
}
```

`go test ./...` on Linux runs the server test with the recording `Start`;
the Windows job's "Unit tests" step runs it for real on Windows,
where `TestUpdateEndpoints` no longer skips.

## 7. The test on a real PC

A step for docs/testing-windows.md, after its step 7,
once the design is implemented;
it follows the plan's ground rules (an administrator prompt,
the fake Cube from step 2, no secrets in the report).
`$v` is the release under test, which must carry the implementation;
the install from step 3 is in place with `MASSIMO_API_TOKEN=test-token`.

### 7b. Update from the web UI

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
   In the API:
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
   `installed v… as C:\Program Files\Massimo\massimo.exe (the old one is …\massimo.exe.prev)`, `stop the service massimo`, `running v…`; the status file with `"state": "done"`; `service.log` with `stop requested`, `stopped`, `started as a service`, `running`; `serve.log` with `update requested` and `update helper started pid=…`; two 7036 events (stopped, running); exactly one `massimo` process, the service's, and no `update-request` file.
4. A second request right after a first one is refused:
   run step 2 again after installing v0.5.0-alpha.1 once more (the first command above),
   and in the two seconds after the click send
   `curl.exe -s -o NUL -w "%{http_code}" -X POST -H "Authorization: Bearer test-token" http://127.0.0.1:8099/api/v1/update` from a second window.
   Expected: `409`.
   Afterwards the update finishes as in step 2.
5. From another device on the LAN (step 4 of the plan): the same as step 2,
   to see the page lose and regain the server over the network.
6. The rollback through the helper,
   by hand as in step 7 (the API cannot name a version):
   `update -yes -version v0.2.0-alpha.1 -restart massimo -health …` must end in
   "rolling back" after about 35 s with `massimo.exe.failed` present;
   then step 2 once more (the button,
   to the newest release) must succeed and `massimo.exe.failed` must be gone afterwards.
7. Optional, for the rollback through the web UI: the stub of section 6,
   stage 2, on the PC (a binary built from the branch with a test key,
   `MASSIMO_UPDATE_API` in `C:\ProgramData\Massimo\env`,
   the stand-in listed as the newest release);
   the button must end in "failed and rolled back v0.0.0-ci.3:
   start the service massimo: …", the service on the previous version,
   within about 40 s.
8. Windows Security → Protection history must show no event for `massimo.exe`,
   `.massimo.new.exe` or `massimo.exe.prev` (observation 8); note it either way.

For the report: the times of steps 2 and 6, the contents of
`update.log` and the status file, and the event IDs.

## 8. The two loose ends of the test report

**Observation 5, `massimo.exe.failed` is never cleaned up.**
The file is the binary that failed its health check,
kept for diagnosis ("which build did not come up here?").
Its value ends when a later binary installs and proves itself,
so `update.Swap` removes `FailedPath(target)` when it puts a new binary into place,
on every platform:
a successful update leaves `massimo.exe` and `massimo.exe.prev`;
an update that fails again writes a fresh `massimo.exe.failed` through
`Rollback` (whose `os.Rename(swap, keep)` replaces an older one anyway,
on Windows too: Go's rename there uses `MOVEFILE_REPLACE_EXISTING`).
So `.failed`, when present,
is always the most recent failure and never older than the running binary.
The alternative, removing it only after the health check passes,
keeps the file for the two minutes in which the new binary could still fail,
and needs a second place in cmd/massimo/update.go; not worth it.
A manual `update -rollback` leaves `.failed` alone.
`install -remove` already removes it on all three platforms (the program folder
on Windows, `BinPath + ".failed"` on Linux and macOS).
The documentation sentence "the failed one stays as `massimo.failed`" in
packaging/README.md gains "until the next update succeeds".

**Observation 4, the documented two minutes versus the real 35 s.**
Two timeouts are at work.
The service manager waits `ServicesPipeTimeout` (30 000 ms by default) for a
started process to connect;
a binary that never does (v0.2.0-alpha.1 has no service code) makes `Start` fail
with error 1053 and event 7009, and `update` rolls back at once:
about 35 s with the stop and the two restarts.
The two minutes are `-health-timeout`,
which only runs once `Start` has succeeded:
a binary that answers the manager but never reports the new version on
`/healthz` is given up after two minutes,
during which the manager's failure actions restart it every 10 s if it crashes.
docs/testing-windows.md step 7 says "about two minutes" for the v0.2.0-alpha.1
case and should say about 35 s, with the two minutes named as the other case;
packaging/README.md "Windows" gets the same sentence;
the CI job's `-health-timeout 90s` is unaffected.

## 9. Open questions for the owner

Positions taken above, for the owner to confirm or change.
Confirmed by the owner on 2026-11-06, all nine as positioned;
the implementation follows them.

1. Option A over B2.
   The reasons are in section 4;
   B2 is the fallback if the CI job or the real PC shows the detached child
   dying with the service.
2. The helper's environment: the server's without the `MASSIMO_*` variables,
   plus `MASSIMO_GITHUB_TOKEN` and a `MASSIMO_ENV_FILE` that does not exist.
   The alternative is to start it with `-env C:\ProgramData\Massimo\env`,
   which is simpler and harmless on Windows (same account,
   same ACL) but hands the Cube keys and the API token to a process that does
   not need them.
3. The helper's log is its own file, `C:\ProgramData\Massimo\update.log`,
   not `service.log`, which observation 2 already finds too full;
   it is not rotated, like the other Windows logs.
4. The health URL is derived by `serve` from `-listen` with the drop-ins' rule
   rather than passed as a further flag; `sc.exe qc` then does not show it.
   A `-update-health` flag on `serve` would.
5. `.failed` is removed by `Swap`, on all platforms (section 8).
6. `-no-update-helper` is accepted on Windows and leaves the two flags
   out of the service's command line.
7. `MASSIMO_UPDATE_API` as a test-only environment variable in the update client,
   read on every platform,
   so that the CI job can serve its own releases (section 6, stage 2).
   Without it the CI test stays dependent on what the alpha channel holds,
   as the macOS job is.
8. The two optional robustness changes:
   `WriteStatus` retrying its rename on Windows,
   `Rollback` tolerating a missing target.
9. The helper's stop-and-start window (section 3 A,
   failure modes) is accepted as a residual risk with `sc.exe start massimo` as
   the remedy,
   rather than giving the service a failure action for clean stops
   (`SetRecoveryActionsOnNonCrashFailures`),
   which would also fight every deliberate `sc.exe stop massimo`.

Not verified in writing this, because nothing here ran on Windows:
that the service manager puts no job object around a service it starts (expected
from the documentation and from the run-time check being cheap,
settled by the CI job),
that `QueryInformationJobObject` with a null handle names the caller's job (from
the documentation),
and the 261-character limit of `schtasks /TR` (from the documentation;
it only counts against option C).
