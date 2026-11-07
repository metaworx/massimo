# AP SendButtons v1.0: Busy and unchanged Send buttons

## TL;DR

Every button of the web UI that sends a request is off, with a spinner,
until the answer comes back, so that a second click sends nothing;
a form that knows what it last sent keeps its Send or Save off
until something differs.
A new user's name is checked while it is typed,
and the server's refusals of users and roles lose their `store: ` prefix.

- `components.js` → `setBusy(button, on)` and `whileBusy(button, fn)`:
  off, class `busy` (a CSS spinner), `aria-busy`; `confirmBox`'s Yes too
- users, sign-in, own password, first owner → busy
  (Create 4.6 s and sign-in 6 s on the Pi 1)
- room tiles, House, configuration tabs, week programme, copy → busy,
  held in the tile's state so that a redraw keeps it;
  Send off while nothing differs; the mode buttons stay on while idle
- Cubes, pairing, backups, roles, capabilities, update → busy;
  Save off while nothing differs
- name check: 2 s after the last keystroke, on leaving the field, on Create;
  the rule of `userName`; a name in use too
- tests at 1100 and 390 px, in English and German, against the fake Cube

### Blocks

1. The busy helper; users, sign-in, own password, first owner
2. Cube writes: tiles, House, configuration tabs, week programme, copy
3. Server writes: Cubes, pairing, backups, roles, capabilities, update
4. Name check for a new user and the first owner; no `store: ` prefix
5. Changed screenshots, branch pushed, pull request

## Discussion

The owner tested v0.10.0-alpha.1 on a Raspberry Pi 1 on 2026-11-07.
Two of the findings are this plan's;
the header, the pages and the Home notice are AP Navigation's,
the Log view's AP LogView's.

- **Item 1.**
  In Settings → Users the name "test manager" was refused only on save,
  with `test manager: store: bad user name: "test manager"`.
  Wanted: a check once the field has not changed for 2 s,
  a plain message through `t()`, no internal `store:` prefix.
- **Item 2.**
  Create user took 4.6 s (argon2id) with nothing visible,
  so the owner clicked again and the second POST answered 409 "user exists";
  the sign-in took 6.1 to 6.4 s.
  Wanted: the button off, with a visible busy state, while the request runs.

The decisions, settled by the owner's `PLAN++ SendButtons` on 2026-11-07
as recommended at the TL;DR gate:

- **D1-A.**
  Both sides: the UI checks the name first and words the 400 and 409 itself,
  and `userFail` stops sending `store: ` for every users or roles error,
  so that "the last enabled owner …" and "the role still has users" lose it too.
- **D2-A, widened by the owner.**
  Not only the password forms: every button that sends a request,
  to the Cube or to the server, is off with a spinner until the answer.
  Where a form knows what it last sent, Send or Save stays off until
  something differs, as the week programme's resets already do.
  The mode buttons (AUTO, MANU, HOLIDAY; the House panel's choice) stay on
  while nothing is being sent:
  sending the mode again resets a temperature turned at the thermostat.
- **D8-A.**
  Three plans, each with its own pull request, merged in this order:
  this one, then AP Navigation, then AP LogView,
  each branch started from the one before.

## Analysis

What the page does now, read on `origin/main` at `d972adf`:

- **The users forms.**
  `users.js` `userAction` sets `usersUI.busy`, which nothing reads;
  Create, Save, Revoke and Create key stay active through the request.
  `login.js` disables nothing in the sign-in dialog, Change password
  or Create the owner, which is a POST and a sign-in, about 11 s on the Pi 1.
- **The Cube writes.**
  A double click on a tile's mode, Save, the vacation Send, a lock,
  the House Send or a configuration Send sends the radio command twice,
  each about 1.25 % of the hourly duty cycle.
  The configuration tabs already compute what changed
  (`changedFields`, `dirtyDays`, the device form's `meta` and `cfg`)
  and answer "nothing changed", but their Send stays active.
- **Redraws.**
  A room's tile is drawn again on every state event (`rerenderRoom`),
  and its configuration area on every result (`setConfigResult`),
  so a button disabled on the element would come back active
  in the middle of its request.
  The busy state lives in the tile's state instead
  (`roomUI(room).sending`, `configUI(room).sending`),
  and the drawing reads it.
- **Number inputs** report a change when they are left or stepped,
  not per keystroke (`numberInput`).
  A Send that is off while nothing differs is turned on by the form's
  `input` event, at the first keystroke,
  and set exactly at each `change`;
  the submit handlers keep their "nothing changed" answer for what slips through.
- **What already holds:**
  `confirmBox` keeps Yes off until its promise settles;
  `pairDeviceForm` keeps its submit off through `e.busy`;
  `takeBackup`, `setUpdateBusy` and the header's refresh disable their buttons.
  They take the spinner, and keep their logic.
- **The prefix.**
  `userFail` in `internal/server/users_api.go` passes the store's text
  through `fail(code, "%v", err)`,
  and every error of `internal/store` starts with `store: `.
  No test or document depends on it.
- **The name rule** is `userName` in `internal/store/users.go`,
  `^[a-z0-9][a-z0-9._@-]{0,63}$`, after `NormalName` (trim, lower case).

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`make cross`, `make ui-test` (from this worktree: `UI_TEST_PYTHON` to the main
venv, free `UI_TEST_*_PORT`s), `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin,
and `GUIDELINES/shared/tools/changelog.sh check`.
Nothing reaches a Cube: the tests use the fake Cube with the recorded greeting.
Each block is one functional commit citing `AP SendButtons v1.0, Block <n>`,
with its CHANGELOG bullet (Block 1 adds it to `### Fixed`,
Blocks 2 to 4 amend it, Block 4 adding its own for the names),
and `docs/ui.md` moves with it.
A held route (`page.route` collecting the route, `route.continue_()` later,
as `test_cubes.py` holds the pairing) stands for a slow server in the tests.

### Block 1: BusyHelper

1. `components.js`:
   `setBusy(btn, on)` sets `disabled`, the class `busy` and `aria-busy`,
   and on its end gives back the `disabled` the button had before;
   `whileBusy(btn, fn)` runs `fn` with the button busy and returns its result,
   a click on a busy button doing nothing;
   `confirmBox` marks its Yes busy through `setBusy`.
2. `style.css`: `button.busy` with a spinner after its text
   (`::after`, a ring in `currentColor`, `@keyframes`),
   `cursor: progress`, less faded than `:disabled`,
   slower under `prefers-reduced-motion`; light and dark through `currentColor`.
3. `users.js`: `userAction(fn, okText, name, btn)` takes the button it runs for;
   Create, a user's Save, Revoke, Create key and Remove's Yes are busy
   through it; a user's Save is off while nothing in the form differs
   from the user (the `input` and `change` events recompute it).
4. `login.js`: the dialog's Sign in, Change password's Save and
   Create the owner are busy through their request(s);
   the dialog's submit ignores a second Enter.
5. Tests, `tools/ui-test/test_users.py` and `test_login.py`:
   with the POST held, Create is off, `busy` and `aria-busy`,
   a second click and Enter send nothing, one POST reaches the server,
   and the button comes back once the answer is in;
   the same for Sign in (as the operator, the login POST held);
   a user's Save is off until the display name changes.
6. `docs/ui.md`: the busy state, under "How the page is built".

**Verification:** the gate; the new tests fail on `origin/main`
(the second POST counted).

### Block 2: CubeWrites

1. `tiles.js`: `roomUI(room).sending` names the action in flight
   (`auto`, `manual`, `setpoint`, `vacation`, `lock`, `unlock`);
   `renderSetpoint`, `renderMenu` and `renderVacation` draw the mode buttons,
   Save, the stepper, the vacation Send and the lock items off while it is set,
   the clicked one with the spinner;
   `sendRoomSetpoint` and `lockRoom` set and clear it,
   a call while it is set sending nothing;
   a stepper timer that fires during a send waits for its answer.
2. The House panel: Send busy through `sendHouse`,
   and off while no room is ticked.
3. `config.js`: `configUI(room).sending` holds the form in flight;
   `renderConfig` draws every Send of the area off while it is set,
   and the sending one with the spinner;
   per form a `…Changed` function from what its send would write
   (`changedFields`, `dirtyDays`, the device's `deviceChanges`
   split out of `sendDevice`, the room name, the copy's rooms and settings)
   keeps Send off while it is empty;
   the form's `input` event turns Send on, `change` and every redraw set it.
4. Tests, `test_rooms.py` (or a new `test_send.py`):
   a mode click with the setpoint PUT held sends one PUT
   and the mode buttons are off with the spinner on the clicked one;
   the Temperatures tab's Send is off when opened, on after a change,
   off again after Cancel and reopen;
   the week programme's Send is off until a point changes;
   the Copy tab's Send is off until a room is ticked;
   with the config PUT held, a second Send sends nothing.
5. `docs/ui.md`: the tile and the configuration area.

**Verification:** the gate; the existing room, configuration and House tests
still pass with their clicks on Send after a change.

### Block 3: ServerWrites

1. `cubes.js`: `cubeAction` takes its button;
   a Cube's Save (off while nothing differs, the hidden devices included),
   "Take the file's value", Add of a found Cube, Remove's Yes,
   pairing Start and Stop, and the pairing form's Put into the room are busy.
2. `roles.js`: `roleAction` takes its button;
   a role's Save (off while nothing differs), Disable and Enable,
   Create and Remove's Yes are busy;
   a capability switch is off while its PUT runs.
3. `backup.js`: Back up now, Restore, Delete's Yes and the download
   of a stored backup are busy.
4. `update.js`: Update and Check are busy through `setUpdateBusy`;
   the header's refresh takes the spinner where it is already disabled.
5. Tests, `test_cubes.py`, `test_backup.py`, `test_update.py`, `test_users.py`:
   a Cube's Save is off until a field changes;
   with the PUT held, a second click sends nothing;
   a role's Save likewise; Back up now with the POST held sends one.
6. `docs/ui.md`: the Settings sections.

**Verification:** the gate.

### Block 4: NameCheck

1. `users.js` and `login.js`: `nameProblem(name, taken)` applies the rule of
   `userName` after trim and lower case, and reports a name in use
   from the users listed;
   the new user's name and the first owner's name are checked
   2 s after the last keystroke (`NAME_CHECK_MS`), when the field is left,
   and at Create, which then sends nothing;
   the message, under the field (`aria-describedby`, `role=status`),
   clears at the first keystroke that makes the name valid;
   a 400 or 409 from the server is worded by the page in the same terms.
2. `ui/i18n/<lang>.json`: `users.nameBad`, `users.nameTaken`
   in all six, FR, IT, NL and PL machine-translated and marked as the files mark them.
3. `internal/server/users_api.go`: `userFail` sends the store's text
   without its `store: ` prefix;
   `users_api_test.go`: the 400 of a bad name reads
   `bad user name: "test manager"`.
4. Tests, `test_users.py` and `test_login.py`
   (the first-owner form shown through a stubbed `/api/v1/me`):
   "test manager" shows nothing at once, the message after 2 s,
   and clears when the space goes; leaving the field checks at once;
   Create with a bad name sends no POST; a listed name is reported as in use.
5. `docs/ui.md` and `docs/api-v1.md` where they quote a store error.

**Verification:** the gate; `go test ./internal/server` (the i18n keys).

### Block 5: Screenshots

1. `tools/ui-test/screenshots.py` retakes the set;
   only the shots whose buttons changed are committed
   (the configuration tabs, Settings), the others come out the same.
2. `origin/main` merged into the branch, the gate once more,
   the branch pushed, the pull request opened for the owner to merge.

**Verification:** `git status` shows only the changed shots;
the pull request's checks run.

## Proposed commit message

Block 1:

```text
fix(ui): Keep a button off, with a spinner, while its request runs

Create user, a user's Save, Revoke, Create key, the sign-in, Change password and Create the owner are off, with a spinner and aria-busy, until the server answers, so that a second click sends nothing (AP SendButtons v1.0, Block 1).

Key changes:
- internal/server/ui/components.js: setBusy and whileBusy; confirmBox's Yes uses them.
- internal/server/ui/style.css: the spinner of button.busy, slower under reduced motion.
- internal/server/ui/users.js, login.js: the users forms and the sign-in busy through their request; a user's Save off while nothing differs.
- tools/ui-test: with the request held, one POST per form, and the button back once the answer is in.
- docs/ui.md: the busy state.

On the Pi 1 the owner's Create took 4.6 s with nothing to see, and the second click was refused with "user exists".

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 to 4 follow the same form, `fix(ui)`, `fix(ui)` and `fix(users)`;
Block 5 is `docs(ui): Retake the screenshots of the Send buttons`.

## Change History

- v1.0 (2026-11-07): The first revision, from the TL;DR gates of 2026-11-07:
  D1-A, D2-A as the owner widened it to every button that sends, D8-A;
  the header, pages and Home notice to AP Navigation, the Log view to AP LogView.
