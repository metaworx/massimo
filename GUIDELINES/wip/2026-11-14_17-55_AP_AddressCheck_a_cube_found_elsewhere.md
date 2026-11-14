# AP AddressCheck v1.0: a Cube found elsewhere

## TL;DR

A Cube with a fixed address that discovery finds at another one is
shown in the API and the UI, with a fix one click away; serve's log
warns of it once rather than at every run.

- API → `found_elsewhere` on a Cube: the address it answers at, with the
  configured port, and since when; only for a fixed address
- log → Warn when new or when the found address changes, Debug for
  repeats (D9-A)
- UI → a warn badge on the Cube's top line and in the section's badges,
  a cog item in the header's and the Cube's pill, a notice above the
  facts with "Use the found address" and "Follow discovery" (D10-A)
- no shuffle icon: the cog's sign (D7-A, CubeBadges' D73-A)

### Blocks

1. FoundApi: `found_elsewhere`, the log rule; `docs/api-v1.md`, the
   OpenAPI file, README, tests
2. FoundUi: badges, the cog item, the notice and its two buttons; six
   languages, `docs/ui.md`, UI tests, screenshots

## Discussion

The owner's request of 2026-11-13, relayed by "Web UI updates": a Cube
whose address differs between `.env`, the store and discovery gets the
shuffle icon. `PLAN AddressCheck`, then `PLAN++ as recommended` on
2026-11-14, taking:

- D6-B: a plan of its own, not a NOTE on CubeBadges;
- D7-A: no shuffle icon, which CubeBadges' D73-A keeps for mixed modes;
  the warning carries the cog's sign, as the other settings warnings do;
- D8-A: on main now;
- D9-A: the log's Warn once, when the mismatch is new or the found
  address changes, Debug for the runs that repeat it;
- D10-A: both buttons, "Use the found address" and "Follow discovery".

**What changed since the gate.** The gate offered `found_address` as
this plan's, for CubeBadges to take. CubeBadges was merged on main
the same day, with `found_address` (its Block 1) and the pill's cog
group: so Block 1 adds the mismatch itself, `found_elsewhere`, and
Block 2 adds an item to the cog group (`cubeSettingItems`) beside the
proxy's and the settings file's. Nothing is asked of CubeBadges.

**`.env` against the store** is already shown: `file_differs` lists a
differing `address` line, with "Take the file's value". This plan does
not touch it.

## Analysis

**Discovery** (`discovery.go` `noteSerial`). For a configured Cube with
a fixed address (`Addr` set, `FollowDiscovery` off) whose host is an IP,
a run whose answers for the serial come from neither that host, the
Cube's nor a proxy's (`answeredFrom`), logs a Warn "cube answers
discovery from another address than configured", at every run: every
30 minutes, every minute while the Cube is lost (`searchLost`). Nothing
in the API says so. A Cube that follows discovery moves to where it
answers (`followAddress`), so it cannot disagree.

**The Cube's view** (`server.go` `cubeView`). `found_address` is the
host the Cube itself answered from (`foundCube.Address`), shown once
discovery has seen it, also where it differs for a good reason (the
server connects through a proxy). `configured_address`,
`follow_discovery` and `address` are there. The UI cannot tell a
mismatch from a proxy without the run's other answers
(`foundCube.Proxies`), so the server decides.

**The UI** (`cubes.js`, `app.js`). A poll redraws an open Cube's facts
(`fillCubeFacts`): its link's notice (`fillLinkState`), its proxy's
(`fillProxyState`), its head and pill (`fillCubeHead`, `cubeEntry`).
The settings warnings are items of `cubeSettingItems`, shown in the
header's cog group for one with the settings right and in the Cube's
own pill. The top line is `cubeBadges`; the section's are in
`renderCubes`. A PUT is `saveCube`.

## Implementation Plan

Every commit cites `AP AddressCheck v1.0, Block <n>`; its docs and
CHANGELOG fragment are in the same commit.

### Block 1: FoundApi

- `foundCube`: unexported `elsewhere` (the configured host the Cube did
  not answer at) and `elsewhereSince`, under `foundMu`, in memory only.
- `noteSerial`: for a fixed address, where the run's answers miss its
  host, set them (since: now, where the host or the found address is
  new) and Warn; where they are the same as before, Debug; where the
  Cube answers at its host, or the Cube follows discovery, clear them.
- `cubeView`: `found_elsewhere` `{address, configured, since}`, where
  the Cube is present, its address fixed, and `elsewhere` its current
  host: `address` the found host with the configured address's port,
  ready for a PUT.
- `docs/api-v1.md` (the Cube's fields, the discovery paragraph),
  the OpenAPI file, README's discovery sentence.
- Tests over `fixedCube` (127.0.0.1, answered for at 127.0.0.3): the
  field and one Warn over several runs; a PUT of the found address and
  one of `follow_discovery` each clear the field.
- CHANGELOG fragment, `Added`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt`,
`go vet` for linux, windows, darwin, `make cross`, `make race`.

### Block 2: FoundUi

- `cubes.js`: `fillAddressState(d, c)`, called by `fillCubeFacts`, fills
  a `.cube-address-state` box above the facts: a `noticeEl` "Found at
  … , not at …" with its since, and for a Cube with the form
  ("Use the found address", "Follow discovery") through `saveCube`;
  rebuilt only where the finding changed, so a poll leaves a focused
  button. `cubeBadges` and `renderCubes`: a warn badge, a count.
- `app.js` `cubeSettingItems`: a cog item, `problem.address` and
  `problem.addressOwn`.
- `style.css` beside `.cube-link-state`; the six `i18n/*.json`
  (FR/IT/NL/PL machine-translated and marked as the others are).
- `tools/ui-test`: a stubbed `found_elsewhere` gives the badge, the
  notice and the cog item, a button's PUT body, and its clearing.
- `docs/ui.md`; the screenshots retaken where they change.
- CHANGELOG fragment, `Added`.

**Verification:** as Block 1, and `make ui-test` (1100 and 390 px).

## Proposed commit messages

- `chore(aps): +AddressCheck v1.0`, this plan, on main.
- Block 1, `feat(api): A Cube that answers discovery at another address`.
- Block 2, `feat(ui): A Cube found at another address, and the fix`.

## Change History

- v1.0 (2026-11-14): first revision, by `PLAN++ as recommended`.
