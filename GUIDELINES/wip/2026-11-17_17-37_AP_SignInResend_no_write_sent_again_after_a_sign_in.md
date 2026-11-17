# AP SignInResend v1.0: no write after a sign-in

## TL;DR

A request that meets an expired session opens the sign-in,
and the page sends it again once anyone has signed in.
A read may do that.
A write no longer does: it fails, and says so.

- `api()` and its twin for a backup's upload: a 401 → the sign-in →
  the same request again, whatever it is
- a write → not sent again; "not sent: you were signed out"
- a read → sent again, as today
- whoever signs in, however much later → no write from before goes out

### Blocks

1. Resend: the rule in the page's two request functions, the text,
   the tests, the documents

## Discussion

### What it is about

Found while AP UnsavedMarks' Block 3 was built, outside that block:
where a session has expired, the next request is answered 401,
the page opens its sign-in, and every request that waited is sent again
once someone has signed in.
That someone need not be who sent it, and it need not be soon:
a setpoint clicked last night goes out when the page is signed into this
morning, under whoever's key that is.

### What is decided

The owner, 2026-11-17, `PLAN++ all A, stack it on top of #347`:

- **D1-A.**
  A write that met an expired session is never sent again.
  It fails with "not sent: you were signed out", the sign-in opens,
  and the user does it again afterwards.
  A form's draft is still there, so that is one click on Save.
- **D2-A.**
  This session builds it, as one pull request,
  on a branch cut on that of AP UnsavedMarks' Block 3 (#347),
  which changes the same sign-in function.
- **`PLAN++`,** as its gate stated: the plan written and registered on main,
  its one block built by a sub-agent, read a second time, committed,
  main merged, pushed with its pull request.

### Decided in the writing

Each as recommended, and each small enough to change afterwards:

- **D3-A.
  What a write is.**
  Every request that is not a GET.
  A request the page makes by itself and that changes nothing,
  as the ticket of the event stream, is named as an exception where it
  needs to be sent again; a user's action never is one.
- **D4-A.
  When the write fails.**
  At once, not after the sign-in: the button that sent it stops turning,
  a form's Save shows the failed save,
  and the message is there when the page is shown again.
- **D5-A.
  Where the message shows.**
  Where the action's result shows today: a tile's line, a form's,
  the page's message line.
  No caller changes for it.
  Where the reload of the page's data after the sign-in would clear it,
  the page's message line says it instead.

### What stays as it is

- A read is sent again after the sign-in.
- The server: nothing changes there.
- The sign-in after a 401 keeps a form's drafts for the same user and
  forgets them for another, as AP UnsavedMarks' Block 3 built it.

## Analysis

- **One function does it for every request:**
  `api()` of `internal/server/ui/app.js` catches a 401,
  awaits `askLogin()` of `login.js` and, where that resolves true,
  calls itself again with the same method, path and body.
  `apiRaw()`, for a backup's file, does the same.
- **`askLogin()` is one promise for all that wait:**
  every request that met the 401 is sent again at the same moment.
- **About sixty writes go through `api()`** in the plain scripts,
  and five in the parts: setpoints, modes, deletions, users, roles,
  providers, integrations, backups, the maintenance switches.
- **The event stream's ticket is a POST** through `api()` (`app.js`,
  `events/ticket`): a request of the page's own.
- **After a sign-in the page reads who is in and starts its data and the stream
  again** (`afterLogin` of `login.js`), which may draw over a result line.
- **The parts' forms**
  (AP UnsavedMarks) show a failed save in red and keep their drafts;
  a save that fails so is a failed save like any other.

## Implementation Plan

As the blocks of the plans before it:
a sub-agent on Opus builds the block without a commit,
a second reader checks it, the commits are made from recorded trees.
The branch is cut on that of AP UnsavedMarks' Block 3.

### Block 1: Resend

- **`app.js`:**
  in `api()` and `apiRaw()`,
  a 401 on a read waits for the sign-in and sends the read again;
  on a write it opens the sign-in and fails at once with the text.
  The page's own requests that may be sent again are named there,
  each with its reason.
- **The text,** in all six languages.
- **After the sign-in** the user can see that the action was not sent:
  checked for a tile's line, a form's line and the page's message line.
- **Tests,** against the fake Cube, the 401 answered by a route:
  a write is not sent again after the sign-in, by the same user and by another;
  its message is shown; a read is sent again; the event stream opens again;
  a backup's upload likewise; a form's Save shows the failed save,
  keeps its draft and sends at the next click.
- **Documents:**
  `docs/ui.md`, the sign-in; a CHANGELOG fragment.

**Verification:**
`make ui`, gofmt, `go vet` for the three platforms, `go test ./...`,
`make ui-check`, the guidelines' checks;
the touched UI test files in all four cases;
the new tests fail on the page before the change.

## Proposed commit messages

- The plan: `chore(aps): +SignInResend v1.0`
- Block 1:
  `fix(security:ui): A write that met an expired session is not sent again after a sign-in`

## Change History

- v1.0 (2026-11-17): first revision, by `PLAN++`,
  from a defect found outside AP UnsavedMarks' Block 3,
  with D1-A and D2-A as the owner chose, stacked on that block's branch,
  and D3-A to D5-A as recommended.
