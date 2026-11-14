# NOTE LinkOutage v1.0: how the run went

For [AP LinkOutage v1.0](2026-11-14_00-05_AP_LinkOutage_hold_stale_busy_reboot.md):
how the run of Blocks 1 to 6 under the owner's `PLAN++` of 2026-11-14 went,
where it went differently from the plan, and what it checked.

## Where it went differently from the plan

1. **`MASSIMO_CUBE_HOLD` is left out**; `-cube-hold` alone exists (Block 3).
   A variable would need the settings file's rules: `env.Known`, the reload's
   flag map, and store.md's table of the variables that stay.
   The flag serves the LAN test and the owner's checks.
2. **The kernel's limit is longer than the hold**
   (Block 3, fixed after Block 6).
   The kernel counts from the first byte left unacknowledged, a missed poll's,
   so a `TCP_USER_TIMEOUT` of the hold alone ended the connection the moment
   the hold began; the LAN test's outage showed it.
   The limit is now the hold, plus the time the loss takes to be marked
   (two polls and their reply timeouts, or the keepalive's 30 s,
   whichever is longer),
   plus a probe's interval: 45 min + 145 s by default (`Options.kernelHold`).
3. **`TCP_INFO` marks the loss sooner than planned on a link that is down.**
   There the kernel's count of unanswered probes is its probes for a poll
   it could not send, not the keepalive's, so the loss was marked about 12 s
   after the poll.
   The loss's error says "the kernel counts 3 probes unanswered".
4. **The hold reaches serve's own lines through the events**
   (Block 3, fixed after Block 6).
   The client's lines are at debug in serve (`EventLines`).
   So `cubelan.Event` carries `HeldUntil` on the loss, which logs as `held_until`,
   and `Held` on the return, which logs as `held=true`.
   `cube rebooting` is the client's own line at Info, since no event says it.
5. **Check 12 of the LAN test stages the Cube's side with the fake's silence**,
   not a real outage (Block 6).
   A real outage left the Cube free: the abandoned connection's FIN reached it
   when the link came back within the kernel's limit.
   Which a real outage shows depends on the kernel's timers;
   check 11 takes the real outage for the hold.
6. **Two defects of Block 2's fake**, fixed in commits of their own:
   - a client leaving while the fake is silent was seen only after the silence
     (found by Block 4's stale test);
   - the parked connection went to `::1` behind a dual-stack listener,
     which a Docker container may lack (found by Block 6).
7. **The refusal's end is logged by the client**, "cube no longer refuses",
   beside the warning when it begins (Block 4, as planned).
   The UI learns the link state from the list it polls; no event kind was added.
8. **Routine choices** (Blocks 4 and 5):
   - **The Reboot button** is at the bottom of the Cube's panel, outside its form,
     since the form needs `cubes` and is replaced by a note for a Cube of `-addr` or `-sn`.
   - **The panel's notice** of a held, stale, busy or rebooting Cube is above its facts,
     and Rooms' notice says the same first.
   - **A test hook** `cubeOptions` in `server.Options` sets the clients'
     reply timeout and dialer.
   - **The reboot's 409** carries the count and its time in its message,
     no fields of their own.
   - **The R goes to the discovery target at the Cube's IP**, where there is one
     (a fake's port in the tests), else to the Cube's IP and port 23272.

## What was checked

- **Each block:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `gofmt` and `go vet` for linux, windows and darwin, and `make cross`.
- **Blocks 3 to 6:** `go test -race -count=1` over the race packages.
- **Windows:** the cubelan and fakecube tests through interop,
  `TCP_MAXRT` read back from the socket.
- **`make ui-test`:** 676 passed after Block 4, 680 after Block 5.
- **`make lan-test`:** all 13 checks passed twice after Block 6;
  checks 1 to 10 passed after Blocks 3 and 4.
- Nothing was sent to a real Cube.

## Left for the owner

- **A check on breno**, on the owner's word:
  - an nft drop of serve's traffic to a Cube, shorter than the hold:
    the Cube resumes on the same connection;
  - with a shortened `-cube-hold`, longer than it and the kernel's limit:
    `link` is `stale`;
  - then **Reboot the Cube** from the panel.
