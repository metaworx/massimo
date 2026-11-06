# AP UpdateRace v1.0: Second update POST on the first's connection

## TL;DR

The Windows job's `post_update` sends its two `POST /api/v1/update` from two
curl processes, and in stage 2 the update helper installs the local stand-in
and stops the service before the second process is up.
One curl invocation sends both, the second after `--next` on the first's
connection the instant the 202 arrives,
so the 409 comes before the stop at any runner speed.
Shell only; no product code.

- `post_update` in `ci-lib.sh`: two curls → one with `--next`;
  `rm -f post.json second.json` first
- roles kept: first 202, second 409; `000` still fails the step
- stage 1 (GitHub) takes the same path, unchanged in substance
- `GUIDELINES/project/CI.md` §7 v1.4.0: the constraint in words
- `docs/update-windows.md`: one line in the status list
- `CHANGELOG.md`: Fixed (the job shipped in 0.7.0-alpha.1)
- the macOS job sends one POST: unaffected

### Blocks

1. The second POST on the first's connection: `ci.yml`, CI.md §7,
   the design document's status line, the CHANGELOG bullet;
   verified by the test gate, the guideline checks and the pull request's
   own Windows job.

## Discussion

The owner answered the TL;DR gate on 2026-11-06 with `PLAN++`,
D1-A (one curl, `--next`) and "base it on PR#61":
pull request 61 (CI once per commit,
the Screenshots job in the Playwright image) was merged into `main` meanwhile,
so the branch `claude/ci-update-race` starts from `main` after it,
where CI.md is v1.3.1.

The designs weighed at the gate:

- **A, taken.** One curl, two URLs with `--next`:
  the second POST leaves on the first's connection as the 202 arrives;
  the roles stay, first 202 and second 409; shell only.
- **B.** `curl --parallel`, both at once: one 202 and one 409 in either order;
  which is which is lost,
  and both write the request file before either starts the helper,
  so a leftover `update-request` can fail stage 1's check.
- **C.** A refused connection accepted when the status file shows the helper
  past `checking`: weaker, the 409 goes unverified on that path.
- **D.** The helper waiting a moment before it stops the service:
  product code, a CHANGELOG bullet under `internal/**`,
  every update slower, and a margin rather than an order.

Branch and pull request: `claude/ci-update-race` from `origin/main` after
pull request 61; `origin/main` merged before the pull request, never rebased;
the pull request opened against `main` and left to the owner,
with no auto-merge.

## Analysis

**What happened** (run 42221219917, attempt 1, job 127903458143,
pull request 56, which changed only `tools/ui-test` and the Screenshots job):

- stage 1, against GitHub: 202 at 12:56:35.253,
  409 "(checking)" at 12:56:35.836, 583 ms later;
  the release check against GitHub takes the time on both sides.
- stage 2, v0.0.0-ci.2: 202 at 12:56:54.855,
  409 "(installing)" at 12:56:54.940, 85 ms later.
- stage 2, v0.0.0-ci.3: 202 at 12:56:57.553,
  `curl: (7) Failed to connect to 127.0.0.1:18099` at 12:56:57.687,
  134 ms later;
  then `cat second.json` printed the previous call's file,
  since curl writes none on a refused connection,
  which reads as if the server had answered.
- the same stage passed in attempt 2 and on `main`'s runs of the day,
  the 409 74 ms after the 202.

**Why the helper is that fast in stage 2:** everything is local.
The server's `startUpdate` (`internal/server/update_api.go`) writes the
request, starts the helper detached (`cmd/massimo/update_helper_windows.go`)
and answers 202;
from then on it answers 409 to another POST, from the status file's
`checking`, `installing` or `restarting`,
or from the starter's own guard (`ErrHelperRunning`, "(starting)"),
until it is stopped.
The helper (`runUpdate` in `cmd/massimo/update.go`) takes the request,
writes `checking`, lists the stub's one release and verifies its manifest
(three requests to the python server on 127.0.0.1:8123), writes `installing`,
downloads the 2 MB stand-in, runs its `version` in a process of its own,
swaps the binaries, writes `restarting` and stops the service
(`stopService` in `internal/install/windows.go`).
On the stop the service process exits at once
(`cmd/massimo/service_windows.go`: `svc.Run` returns, then `os.Exit(0)`),
so the listener and any request in flight die with it;
the graceful `Shutdown` in `serve.go` is for signals, not the manager's stop.

**Why `--next` is deterministic:** curl sends the second request on the
first's connection as soon as the first's answer is complete,
with no process start in between.
The server's answer to it is `updateState` with a refresh
(the same three requests to the stub), a status read and the start guard;
the helper's path to the stop holds those three requests and, besides them,
its own process start, the status writes, the download,
the smoke test's process, the swap and the service control.
The order is in the work, not in milliseconds,
so it holds on a slow runner as on a fast one.
In stage 1 both sides wait on GitHub and the margin is seconds, as before.

**What `--next` means for the options:** it resets every option that is not
global, so `-sS -o -w -X -H` are repeated after it;
`-w '%{http_code}\n'` prints one code per transfer,
`000` for one that could not connect,
and `|| true` keeps curl's exit status from ending the step before the codes
are checked.
Go's server keeps HTTP/1.1 connections alive and curl reuses one across its
URLs; a new connection from the same process would be as quick.

**A leftover request file** is unchanged by this plan:
the "(starting)" 409 writes the request file before the guard refuses,
possibly after the helper took it;
the design document's status list records that the file is left alone,
stage 2 does not check it and the next POST overwrites it;
stage 1's `test ! -e "$data/update-request"` is safe because its second POST
waits on GitHub and finds the status file first.

## Implementation Plan

### Block 1: the second POST on the first's connection

1. `.github/workflows/ci.yml`, `ci-lib.sh` in the step
   "Install and run the service":
   `post_update` removes `post.json` and `second.json` with the status file;
   one `curl` sends both POSTs with `--next` between them,
   each with `-sS -o <file> -w '%{http_code}\n' -X POST -H 'Authorization:
   Bearer ci' "$api"`;
   the two codes are read from its output;
   the checks stay: 202, else the error and the GET; 409, else the error.
   The comment above the function says why.
2. `GUIDELINES/project/CI.md` §7, a bullet:
   the helper stops the service within about 100 ms of the API's 202 in
   stage 2, so the second POST goes on the first's connection;
   a second curl process started too late once.
   Title v1.4.0.
3. `docs/update-windows.md`, the status paragraph:
   a line that the CI test's second POST rides the first's connection,
   and why.
4. `CHANGELOG.md`: `### Fixed` under Unreleased, the bullet.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...` and `make cross`
(the gate; no Go file changes);
`bash -n` over `ci-lib.sh` as the heredoc writes it;
`GUIDELINES/shared/tools/check`;
`GUIDELINES/shared/tools/changelog.sh check --staged --message <file>` and
`commit-msg.sh` before the commit;
the pull request's Windows job is the test of the change itself,
which cannot run locally.

## Proposed commit message

```
fix(ci): send the second update POST on the first POST's connection

AP UpdateRace v1.0, Block 1. The Windows job checks that a second POST /api/v1/update is refused while the first's update runs, and that check raced the update helper: with the stubbed release of stage 2 the helper had installed the stand-in and stopped the service before a second curl process was up, so the second POST met a closed port instead of the 409, and the previous call's second.json made the log read as if the server had answered. Both POSTs now go out of one curl, the second on the first's connection the moment the first is answered: the helper the first starts has not taken a step by then, and the server's answer to the second is less work than the helper needs before it can stop the service, so the 409 comes first whatever the runner's speed.

Key changes:
- post_update in the Windows job's ci-lib.sh (.github/workflows/ci.yml) sends both POSTs in one curl invocation, the second after --next, and removes the previous call's post.json and second.json first.
- GUIDELINES/project/CI.md v1.4.0, "Runner Constraints": why the second POST rides the first's connection.
- docs/update-windows.md: the status list names the change beside the other places where the implementation goes beyond the design.
- A CHANGELOG bullet under Unreleased, Fixed.

File-revision-v1: GUIDELINES/project/CI.md::v1.4.0::2026-11-06::7::the helper's speed against a second curl process in stage 2
Assisted-by: claude-code:claude-fable-5-1
```

## Change History

- v1.0 (2026-11-06): the first revision, after the TL;DR gate;
  D1-A, and the branch from `main` after pull request 61.
