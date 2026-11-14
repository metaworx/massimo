# AP LogAlerts v1.0: the log's icon by its unseen problems

## TL;DR

The 📄 log button turns amber or red while a warning or an error of any
Cube's log is neither settled nor marked seen.

- each warn or error event kind → settled by its counterpart where it has
  one (a disconnect by the next connect, a held link by connected, a
  proxy failure by its listening, a transmit error by its clearing)
- each person → "Mark seen" on a line, "Mark all as seen" in the log;
  their marks kept with their settings; opening the log marks nothing
  (D98b-A)
- the log → unseen, unsettled lines highlighted
- the 📄 button → amber or red with a triangle or a cross (AP Signs'
  marks), plain otherwise

### Blocks

1. Settling: the kinds at warn and error with their counterparts; the
   server answers which lines are unsettled; tests
2. SeenMarks: the person's seen marks (a line, all), kept with their
   preferences; the API; tests
3. LogIcon: the 📄 button's sign, the log's highlight and its marks;
   i18n, UI tests, `docs/ui.md`

The three blocks are one pull request.

## Discussion

The owner's points of 2026-11-14 evening:

- "when there are unconfirmed warnings in the log of any cube, make the
  log icon amber, if there are unconfirmed errors, make the icon red. A
  disconnect is confirmed by a connect. Or maybe we need to acknowledge
  errors manually";
- D98-A with "Mark seen" per line and "Mark all as seen", not only by
  opening the log; D98b-A, opening marks nothing;
- the error kinds new since #237 (the shared discovery port) are among
  them.

`PLAN+ as recommended` on 2026-11-14.

## Analysis

**The log** (`internal/server/log.go`, the store's `log` table): each
event has a kind, a source (system, cube, device, server), a level
(info, warn, error) and a time; `GET /api/v1/log` and `GET …/log`
answer it. A Cube's connect and disconnect, a link held, stale or busy,
a proxy's failure and listening, a device's state with its flags, the
server's own (update result, locked out, backups) are kinds there.

**Counterparts.** Block 1 lists every kind logged at warn or error, from
the code, with the kind that settles it for the same Cube or device, or
none: one without a counterpart is settled only by a mark. The list is
one table in the server, so that the API, the UI and a test share it.

**Seen marks.** A person's preferences (`/api/v1/me/preferences/{key}`)
keep the page's settings already; `log.seen` holds the newest event id
seen for "all", plus the ids of lines seen one by one since then. A
browser without a user keeps them in its storage.

**The 📄 button** is `#view-log` (and the log's entry in ☰ on a phone).

## Implementation Plan

### Block 1: Settling

- `internal/server/logalerts.go`: the table of kinds at warn and error
  and their counterparts; `unsettled(after id)`, the warn and error events
  not followed by their counterpart for the same Cube or device.
- `GET /api/v1/log/alerts?after=<id>`: the unsettled events' ids, kinds
  and levels, and the worst level; capability `read`.
- Tests: a disconnect settled by the next connect, a held link by
  connected, a transmit error by its clearing; an error without a
  counterpart stays; `after` leaves out the older ones.

**Verification:** `go test ./internal/server/`, `make race`.

### Block 2: SeenMarks

- The preference `log.seen`: `{all: <id>, lines: [<id>…]}`, written by
  the page; read with the alerts.
- `GET …/log/alerts` takes the person's marks into account.
- Tests: "all" hides the older ones, a line's mark hides it alone.

**Verification:** `go test ./internal/server/`.

### Block 3: LogIcon

- `app.js`: the alerts read at each poll; `#view-log` and ☰'s log entry
  get AP Signs' sign (amber triangle, red cross), its label naming the
  count.
- `log.js`: unsettled, unseen lines highlighted, each with "Mark seen";
  "Mark all as seen" in the log's bar.
- i18n, six languages; UI tests: the icon's colour by level, the marks,
  the highlight; opening marks nothing.
- `docs/ui.md` "Log"; `docs/api-v1.md` and the OpenAPI file; a fragment.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(api): The log says which warnings and errors are unsettled`;
Block 2, `feat(api): A person marks the log's lines as seen`;
Block 3, `feat(ui): The log's button shows the worst unseen problem`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.
