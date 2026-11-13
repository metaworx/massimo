# Design: phases, target environments, architecture

The design proposal the project started from,
kept here as the frame for docs/roadmap.md.
Written before phase 0; the "Status" section says what has happened since.
Names: the product is Massimo; "the bridge" below means the same thing.
The evaluation that preceded it,
the licence reasoning and the two routes to a public server are in docs/background.md,
with the sources.

References used throughout:
[Bouni/max-cube-protocol](https://github.com/Bouni/max-cube-protocol) (LAN protocol),
[FHEM MAX modules](https://fhem.de/commandref.html#MAX) (radio protocol reference), [ERNW:
Analyzing an IoT
Device](https://insinuator.net/2016/04/discover-the-unknown-analyzing-an-iot-device/)
(portal transport, firmware),
[LoxBerry](https://www.loxwiki.eu/display/LOXBERRY) and [Loxone virtual
inputs/outputs](https://www.loxone.com/enen/kb/virtual-inputs/) (integration targets),
[dktzde/maxcube](https://github.com/dktzde/maxcube) (Cube reflashed as a radio stick).

### Status (2026-11-05)

- Language: Go, as recommended below.
  Module `go.metaworx.ch/massimo`, licence AGPL-3.0-or-later.
- Phase 0 is done and went further than planned: the Cube protocol package,
  the fake Cube (`sim` in the proposal), the LAN driver, the duty-cycle queue,
  backup and restore, discovery, the fake portal and the portal research,
  the REST API v1 with a web UI,
  the LAN proxy (serve as a Cube for other clients), and the configuration API.
  See docs/cube-protocol.md for the hardware findings.
- Phase 1 is partly done: engine, queue, API, live events,
  web UI and Cube backup exist; missing are Loxone, MQTT, SQLite, rules,
  and the packages for LoxBerry, `.deb` and Windows services.
- Phases 2 to 4 are untouched.
  The portal research track found that the Cube ignores commands in the portal reply;
  the firmware dump is the remaining route,
  its procedure on the branch `archive/portal` with the portal code.
- Deviations from the proposal:
  the web UI is plain JavaScript embedded in the binary,
  not a TypeScript single-page app;
  the API is written by hand with an OpenAPI description maintained next to it,
  not generated.
  Both can change when the UI grows.

### Target environments

| Mode | Where it runs | What it does |
|---|---|---|
| `standalone` | LoxBerry, Raspberry Pi, a Linux box, Windows | Everything local; the default, what `massimo serve` is today |
| `edge` | Same as standalone, plus a link to a server | Works locally and also opens an outbound connection to a public server |
| `server` | Public hosted service | Many tenants; reaches the Cubes only through `edge` agents |

Gateways: the Cube over LAN (today), a radio stick (`cul`, phase 3),
a simulator for tests and demos (the fake Cube today),
and `remote` for the server mode.
Clients: the web UI (installable on a phone), REST API, Loxone, MQTT, webhooks,
later native app shells.

## 1. Go vs Python

| Criterion | Go | Python |
|---|---|---|
| Packaging for Windows, Pi, LoxBerry and server | One self-contained binary per platform, cross-compiled from any machine. Web UI embedded. | Needs the interpreter, a virtual environment and dependencies on every target. The Windows installer is clumsy. |
| Running as a service | One library covers Windows services and systemd | Needs NSSM/WinSW on Windows, a systemd unit on Linux |
| Memory on a Pi | ~15–30 MB | ~50–100 MB |
| Many Cubes, many connections (public server) | Lightweight concurrency built in | asyncio works, but it is easy to block it by mistake |
| Timing on the radio-stick backend | Predictable response times | Garbage collection and blocking calls cause occasional delays |
| Existing MAX! libraries | None maintained; the Cube protocol is small | `maxcube-api` covers the Cube LAN protocol only |
| Safety in a multi-tenant server | Static types and a strong standard library | Possible, needs more discipline |
| Contributors from the Loxone/HA community | Fewer | More |
| Speed of a first prototype | Good | Better |

Decision: Go for the core.
Packaging for three platforms,
the public server and timing on the radio-stick backend decided it.

## 2. Architecture

Request path: inputs (web UI, REST API, Loxone, MQTT, webhooks) → API
layer (authentication, authorization, audit) → engine (desired and
reported state per device, arbitration of who is in control, scheduler,
rules, event bus, per-gateway command queue respecting the radio
budget) → gateway driver (`cubelan`, `cul`, `sim`, `remote`) → storage
(SQLite in standalone and edge, PostgreSQL in server mode).

Package layout as built (the proposal's layout, adapted):

```
cmd/massimo/                 command line: serve, status, backup, restore, cube, watch, discover, config, portal, legacy …
internal/protocol/cube/      H/M/C/L/S and the command builders, metadata, week programs
internal/cubelan/            LAN driver, snapshot, discovery, UDP, fake Cube
internal/engine/             command queue with duty-cycle and memory-slot policy
internal/backup/             backup files, restore planner
internal/portal/             fake portal, crypto, probes
internal/proxy/              serve as a Cube for other LAN clients
internal/server/             HTTP API, event stream, embedded web UI
docs/                        protocol findings, API, proxy, firmware, roadmap, handovers
```

Planned: `internal/domain` (driver-agnostic model), `internal/driver/cul`,
`internal/integrations/{loxone,mqtt,webhook,email}`, `internal/authz`,
`internal/store`, `internal/diag`, `packaging/{windows,deb,loxberry,docker}`.

## 3. Cube-LAN and radio-stick drivers

Both drivers implement one interface and declare their capabilities (can pair,
has RSSI, stores week programs, reports the duty cycle).

| Responsibility | Cube LAN | Radio stick |
|---|---|---|
| Pairing | Done by the Cube | Done by the app (FHEM's radio module as reference) |
| Retries and acknowledgements | Done by the Cube | Done by the app |
| Sending the time to thermostats | Done by the Cube | Done by the app |
| Transmit-time budget | Cube reports it in `H:`/`S:` | Stick reports it via a status command |
| Signal strength | Not available | Per packet, for diagnostics |
| Configuration storage | Cube's memory, mirrored by the app | The app is the only copy |

The 868 MHz band allows about 1 % transmit time, roughly 36 seconds per hour;
the queue drops a command when a newer one for the same device supersedes it
and uses priorities.
Migration from Cube to stick:
the stick driver can take over the Cube's radio address,
so existing pairings may keep working; the data model is the same for both.

## 4. Control ownership and Loxone

Each room or room group has an owner:
`thermostat` (the device's week program rules,
the app only sets temporary overrides),
`app` (the app's scheduler and rules) or `loxone` (the Miniserver decides;
the app passes requests through).
Arbitration, highest first: safety limits, manual override with expiry,
the owner's setpoint, fallback schedule.

Passthrough with Loxone as owner:
a UI change becomes a request event to a Loxone virtual input;
Loxone logic accepts or adapts it and writes the setpoint back through a virtual
output to the REST API.
A lease: Loxone sends a heartbeat;
silence for N minutes falls control back to `thermostat` or `app` and raises an alert.
Interfaces: UDP `key=value` lines or HTTP to the Miniserver,
virtual output HTTP calls to the API with a scoped key,
Loxone Config template export (XML), a LoxBerry plugin.

Home Assistant is the other platform, and where a site makes it the hub,
it takes the role Loxone has here
([homeassistant.md](homeassistant.md); AP HomeAssistant, 2026-11-09):
Massimo publishes its own rooms and devices,
and the platform renders them as one more client, beside the web UI and the
apps; its writes go through the API, the queue and the radio budget.
Until Massimo's own integration exists, Home Assistant's `maxcube` reads
the Cube through the LAN proxy.
The integration signs in with a key of the role `integration`,
follows the event stream with its ids and resume,
and edits week programmes with the web UI's editor element.
Its owner, with a lease as Loxone's, is still to be decided.

## 5. REST API, web UI, events and webhooks

API versioned under `/api/v1`, OpenAPI description, resources for sites,
gateways, rooms, devices, groups, schedules, rules, webhooks, users, shares,
API keys, backups, diagnostics, and a live event stream.
Writes are asynchronous with a command id and status (queued, sent,
acknowledged, failed, dropped), an `Idempotency-Key` makes retries safe.
The web UI uses only the public API.
Rules: trigger (event or schedule), conditions,
actions (HTTP with templated body, e-mail, MQTT, set temperature or mode);
signed webhooks with retries and a delivery log; on the public server,
targets in private networks are blocked.

## 6. Multiple Cubes, users, groups, sharing, public server

Entities: Tenant (every row carries `tenant_id` from day one), User, Membership,
Site, Gateway, Room, Device, Group, Share (time-limited grant), ApiKey,
AuditLog.
Roles: owner/admin, operator, guest (within limits during the share), viewer,
integration.
Holiday-guest share: grant from check-in to check-out with limits,
invitation by e-mail, automatic actions at start and end.

Public server: the Cube's LAN protocol has no authentication,
so port 62910 is never exposed;
the server reaches Cubes through edge agents (outbound TLS,
device certificate or token), and the local site keeps working offline.
Redirecting the Cube to the server with `u:` needs the portal protocol (research track).
PostgreSQL with row-level security, hosting in Switzerland or the EU, OIDC,
billing.
Temperatures and window states are personal data (nDSG, GDPR);
liability for frozen pipes needs terms of service;
no "MAX!" or eQ-3 in the product name.

## 7. Debugging, logging, export/import, backup

Structured logs with per-component levels;
a protocol trace per gateway in a ring buffer, decoded in the UI,
downloadable and replayable in the simulator;
a support bundle with secrets redacted;
`/metrics` for Prometheus and `/healthz`.
Versioned JSON/YAML export and import with a schema version, dry run,
selective scope; device backup of the Cube's full configuration (exists today);
scheduled database snapshots to local, SMB, WebDAV or S3,
encrypted when they contain secrets.
Security: argon2id, secrets encrypted at rest, TLS, CSRF protection,
rate limiting, signed releases.

### Whole-database export and import

Header with format, schema version, time, source instance and scope;
JSON and YAML as two spellings of the same data validated against a JSON Schema;
deterministic output for diffs and git.
Stable IDs (UUIDv7) plus natural keys (Cube serial, radio address, e-mail).
Secrets encrypted with a passphrase by default (Argon2id plus AES-GCM).
Import modes create-only, merge, replace, sync with prune;
conflict rules per object type; a dry run that shows the plan; one transaction;
an automatic backup before every import; migrations step by step.
Every object type registers an export handler, a matcher and a merge rule,
with a test that export → import → export is byte-identical.

## 8. Native apps

The REST API is the same in every mode, so an app keeps a list of connections:
on the LAN via mDNS (`_massimo._tcp`), through the public server as a relay,
or through the user's own VPN.
Pairing by QR code with URL, one-time token and certificate fingerprint;
login with OAuth2 PKCE.
The same web app in a Capacitor shell for the stores (native push, widgets,
biometrics, LAN discovery);
an installable web app stays the free route with Web Push.
Native push on iOS needs a relay server holding the Apple key.
Store review needs a demo account (simulator driver).
AGPL and the App Store conflict: publish the shell under Apache-2.0 or MPL-2.0,
or with a contributor agreement.

## 9. Phases and rough effort (one developer)

| Phase | Content | Effort |
|---|---|---|
| 0 Spike | Cube protocol, command-line tool, simulator, traces against a real Cube | done, 1–2 weeks planned |
| 1 MVP (standalone) | Engine, duty-cycle queue, REST API, live events, Loxone (UDP/HTTP, templates, ownership + lease), MQTT, web UI, SQLite, backup, packages for LoxBerry, `.deb` and Windows | 6–10 weeks, about half done |
| 2 Multi-user | Users, roles, groups, shares, audit log, rules, webhooks, e-mail | 4–6 weeks |
| 3 Radio stick | `cul` driver: pairing, acknowledgements, time sync, week programs, Cube→stick migration | 6–10 weeks, the riskiest part |
| 4 Public server | Edge agent, PostgreSQL, tenancy, OIDC, billing, operations | 6–10 weeks plus operations |
| Research | The Cube↔portal protocol, now via the firmware dump | open-ended |

Decisions taken: AGPL for the core; `tenant_id` and the permission layer
from phase 1 (not yet in the code); development hardware is two Cubes,
the second one spare for the firmware work.
