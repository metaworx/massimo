# AP HomeAssistant v1.0: Home Assistant integration and week editor

## TL;DR

Massimo gets what a Home Assistant integration needs,
then the integration itself in a repository of its own:
rooms as `climate` entities, sensors, services and a week-programme card
that uses Massimo's own editor.

- event stream → no ids, events dropped for a slow client → `id:` per event,
  `Last-Event-ID` or `last_event_id` replays, `resync` when too old,
  a slow client's stream ended (D2-A)
- role `integration` → read, setpoint, lock → also `configure`,
  existing stores where the role is still the seed's (D3-B)
- week editor → functions in `config.js` → `<massimo-week-program>`,
  a plain-JS element, the web UI's and the card's (D5-A)
- the integration → `metaworx/massimo-homeassistant`,
  one repository with its card (D1-A), a `climate` per room (D4-A),
  Apache-2.0 (D6-A)
- this plan's `PLAN++` → Blocks 1 to 4, in this repository

### Blocks

1. Resume: event ids, replay from a ring, `resync`, the slow client ended,
   the UI's `last_event_id`; tests, `docs/api-v1.md`, OpenAPI, CHANGELOG
2. IntegrationConfigure: migration 0012, the seed, tests, `docs/store.md`,
   `docs/api-v1.md`, CHANGELOG
3. WeekElement: `<massimo-week-program>` from `config.js`,
   the room's programme tab on it, UI tests at 1100 and 390 px, `docs/ui.md`,
   CHANGELOG
4. HaDocs: `docs/homeassistant.md` (today's paths:
   HA's `maxcube` through the proxy, and what comes), design.md §4,
   the roadmap's item
5. Repository: `metaworx/massimo-homeassistant`, manifest, config flow
   (mDNS `_massimo._tcp` and a URL), CI against `serve` and `fakecube`
6. Entities: rooms, device sensors, the Cube's diagnostics,
   pending and availability
7. Services: `set_program`, `copy_program`, `lock`
8. Card: the week programme on `<massimo-week-program>`,
   served by the integration
9. Release: tag, the Marketplace / HACS listing, README, licence
10. Breno: the integration installed, the `maxcube` stopgap removed

## Discussion

**The owner's request** of 2026-11-09:
Home Assistant becomes the hub at the breno site and reads three sources,
the Loxone Miniserver (PyLoxone), the Windhager boiler (HeatNexus) and the
MAX! heating through Massimo.
The first two run on the breno Pi since that evening;
Massimo has no integration yet.
Massimo publishes its own data and devices,
and Home Assistant renders them as one more client, beside the web UI,
the apps and Loxone; writes go through Massimo's API,
its queue and its radio budget, as the web UI's do.

**Decided** at the TL;DR gate and its EVAL, by the owner's `PLAN++`:

- **D1-A.**
  One new repository, `metaworx/massimo-homeassistant`,
  holding the integration and its card: one install, one version.
  B, two repositories, means two releases to keep in step;
  C, inside this repository, does not work: HACS and the Marketplace want
  the integration at a repository's root.
- **D2-A.**
  The event stream gets ids and resumes:
  every client gains, the integration, the web UI and the apps.
  B, the integration re-reading everything after a reconnect,
  works around the gap in one client only.
- **D3-B.**
  The seeded role `integration` gains `configure`,
  so that an integration's key can write week programmes, temperatures and
  device settings.
  A, a role the owner creates, needed no code, but every installation would
  have to make it.
  Every week-programme write costs radio duty cycle,
  as it does from the web UI.
- **D4-A.**
  A `climate` entity per room:
  MAX! thermostats of a room act together, as Massimo's room tiles do.
  B, one per thermostat as HA's `maxcube` has them,
  duplicates a room and lets the setpoints of one room drift apart.
- **D5-A.**
  Massimo's week-programme editor becomes a plain-JS custom element,
  `<massimo-week-program>`, used by the web UI and by the card:
  one editor, one look, no build step.
  The owner asked whether TypeScript and Lit would let the card share
  components or import them, such as HeatNexus's time programmes.
  They do not:
  HeatNexus is plain JavaScript itself, its editor a mixin of its panel,
  GPL-3.0, and it edits Windhager's switching windows,
  not MAX!'s "temperature until" points;
  HA's own elements (`ha-card`, `ha-switch`) are global custom elements,
  used the same from plain JS as from Lit;
  and a Lit component could not be shared with Massimo's UI,
  which has no build step.
  B, an editor of the card's own, means two editors;
  C, TypeScript and Lit with a bundler, cannot be shared and adds a build.
- **D6-A.**
  The integration is Apache-2.0, as Home Assistant's core,
  which leaves a later move into core open.
  It talks to Massimo over HTTP and is a work of its own.

**Not decided by this plan:**
the new repository and its listing are steps on GitHub the owner takes or
authorises at Block 5's gate;
Blocks 5 to 10 are a later signal's.

**Names chosen here**, routine:
the blocks Resume, IntegrationConfigure, WeekElement and HaDocs;
the query parameter `last_event_id`, the event `resync`;
the element `<massimo-week-program>` in `ui/week-program.js`.

## Analysis

### The event stream today

`GET /api/v1/cubes/{sn}/events` (`internal/server/server.go`,
`events`) subscribes a channel of 64 events to the Cube's runtime and writes,
on connect, one `state` event per device, then every event the runtime hands it.
Three places hand events over: `send`, `sendUser` (a user's notification,
on that user's streams alone) and the pump's loop over the Cube's events.
A full channel drops the event silently (`default: // slow subscriber`).
No event carries an id; the `id` of the JSON is the store's row,
which the stream's events have not.

The web UI (`app.js`, `connectEvents`) opens the stream with a single-use ticket
and, after an error, closes it and opens a new one with a fresh ticket:
the browser's own retry would reuse the spent ticket.
So the browser's `Last-Event-ID` header never comes;
a resume needs the id in the URL as well.

### Resume

- **One publish path.**
  The three fan-outs become one, `rt.publish(ev, userID)`:
  it numbers the event, keeps it in the runtime's replay ring with its user
  (0 for everyone's), and hands it to the subscribers it is for.
- **The id** is `<epoch>-<seq>`: the epoch names the runtime's start
  (its start time in base 36), the sequence counts up from 1.
  An id of another epoch is from before a restart and cannot be resumed.
- **The ring** keeps the last 1024 events,
  about a quarter of an hour on a busy house; its size is a constant,
  not a setting.
- **On connect**, `Last-Event-ID` or `?last_event_id=` naming an event still in
  the ring: the stream replays the events after it that the caller may see,
  and skips the initial `state` burst.
  An id the ring no longer holds, or of another epoch: one `resync` event,
  then the burst as today.
  No id: as today.
- **A slow client** whose channel is full is ended rather than left with a
  gap it cannot see: its next connection resumes from the last id it got.
  The keep-alive comment stays.
- **The web UI** remembers the last id it read and sends it as
  `last_event_id` with the next ticket's stream;
  on `resync` it reloads the rooms as it does after an error today.

### The role `integration`

Migration 0012 adds `configure` to `integration` as 0006 and 0008 added theirs,
with one difference:
only where the role still holds exactly its seeded capabilities, `read`,
`setpoint` and `lock`.
An owner who has narrowed or widened it decided what an automation may do,
and an upgrade does not widen that silently.
The seed of a new store has `configure` from the start.
`docs/store.md` section 6's table and `docs/api-v1.md`'s role line change with
it.

### The week editor as an element

The programme tab is about fifteen functions of `config.js`
(`programModel` to `validateDay`, `renderProgramTab`, `markProgram`,
`renderDayCopy`, `renderDayBar`, `renderPoints`),
built on `components.js` and the global `t()`.
`ui/week-program.js` defines `<massimo-week-program>`:

- **in:**
  the property `program` (the API's days, points `{temp, until}`), `source`,
  and `translate`,
  a function from a key and values to a text (Massimo passes `t`,
  the card its own);
- **out:**
  a `change` event with the edited days and which differ,
  and the methods `value()`, `dirtyDays()`, `resetDay()`, `resetAll()`;
- **looks:**
  an open shadow root with its own styles,
  from CSS custom properties with fallbacks (Massimo's variables inherit into
  it; the card maps HA's), so the element looks the same wherever it is used;
  its parts keep their classes (`prog-days`, `prog-note`, …),
  which Playwright's selectors reach through an open shadow root;
- **keeps:**
  no `innerHTML`, texts through `textContent`, every text by key.

The tab keeps its form, Send and Cancel in `config.js`, around the element.
The element is a classic script like the others, loaded before `config.js`;
the card later carries a copy of the file, pinned to a Massimo version.

**The risk** is the editor's behaviour: "only this day",
editing several days together, copying to days, the two resets,
the 13-point limit, the bar.
`test_rooms.py` and `test_send.py` cover the tab today;
they run unchanged against the element, at 1100 and 390 px,
and a test for each behaviour they do not reach is added.

### Tests

- Block 1: `internal/server` tests against the fake Cube: ids increase,
  a resume replays without the burst,
  another user's notification is not replayed,
  an old id and another epoch get `resync`, a full channel ends the stream;
  `make race` for `internal/server`.
- Block 2: the migration on a store with the seeded role and on one with the
  role changed; the seed's set; `TestStoreSQL`.
- Block 3: `make ui-test` at 1100 and 390 px, the editor's behaviours.

## Implementation Plan

### Block 1: Resume

1. `rt.publish(ev, userID)` with the ring and the counter;
   `send`, `sendUser` and the pump use it.
2. `events`:
   read `Last-Event-ID` and `last_event_id`, replay or `resync`,
   write `id:` lines; end the stream of a full channel.
3. `app.js`:
   remember the last id, send it, handle `resync`.
4. Tests as above; `docs/api-v1.md` ("GET …/events"), the OpenAPI file's
   parameter and header; the CHANGELOG under Added.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`,
`go test -race -count=1 ./internal/server/`, `make cross`, `gofmt -l .`,
`go vet` for linux, windows, darwin; `make ui-test`.

### Block 2: IntegrationConfigure

1. `internal/store/migrations/0012_integration_configure.sql`;
   `store.sql` regenerated as `TestStoreSQL` asks.
2. Tests: seeded role gains it, a changed role does not, a new store has it.
3. `docs/store.md` section 6, `docs/api-v1.md` "Authentication";
   the CHANGELOG under Changed.

**Verification:**
`go test ./internal/store/ ./internal/server/`,
the whole gate as in Block 1 without the UI tests.

### Block 3: WeekElement

1. `ui/week-program.js` with the element; its texts' keys stay `prog.*`.
2. `config.js`'s programme tab on the element; the moved functions removed.
3. `index.html` loads it; `ui.go`'s file list if it names scripts.
4. Tests for the behaviours not yet covered; `docs/ui.md`;
   the CHANGELOG under Changed (nothing a user sees changes).

**Verification:**
`make ui-test` at 1100 and 390 px, the Go gate.

### Block 4: HaDocs

1. `docs/homeassistant.md`:
   HA's `maxcube` through Massimo's proxy as it works today (YAML,
   the proxy's address, polling, no programmes),
   the integration to come and what each block brings.
2. `docs/design.md` section 4:
   Home Assistant as a platform client beside Loxone;
   the roadmap's item 14 names Home Assistant.
3. No CHANGELOG bullet: documents only.

**Verification:**
`reflow.sh --check` on the documents changed.

### Block 5: Repository

The repository, the manifest, the config flow (mDNS and a URL, an API key),
a coordinator on the event stream with resume, CI with hassfest, the HACS
check and pytest against `massimo serve` with `fakecube`.
Its own gate: the repository is created on the owner's word.

### Block 6: Entities

A `climate` per room (setpoint 4.5 to 30.5 °C in 0.5 steps; `auto`,
`heat` for manual, presets eco and comfort from the room's temperatures,
away for vacation), sensors per device (valve, battery, link, window),
the Cube's diagnostics (connected, duty cycle, free slots, queued),
pending while radiograms wait, unavailable when stale.

### Block 7: Services

`massimo.set_program`, `massimo.copy_program`, `massimo.lock`;
the programme as a room's attribute.

### Block 8: Card

The week-programme card on a pinned copy of `week-program.js`,
the integration serving and registering it.

### Block 9: Release

Tag, the listing, the README, the licence.

### Block 10: Breno

The integration installed on the breno Pi, `maxcube` removed,
the box's documentation updated.

## Proposed commit message

For this registration:

```
chore(aps): +HomeAssistant v1.0

The plan for Massimo's Home Assistant integration: event ids and resume,
configure for the integration role, the week editor as an element, the
documents, then the integration in a repository of its own.

File-revision-v1: GUIDELINES/wip/2026-11-09_22-19_AP_HomeAssistant_integration_and_week_editor.md::v1.0::2026-11-09::All::the plan, D1-A, D2-A, D3-B, D4-A, D5-A, D6-A at its gates
Assisted-by: claude-code:claude-opus-5-5
```

Each block's own commit cites `AP HomeAssistant v1.0, Block <n>`.

## Change History

- v1.0 (2026-11-09): the plan, from the TL;DR gate and its EVAL;
  D1-A, D2-A, D3-B, D4-A, D5-A, D6-A.
