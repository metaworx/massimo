# AP PortalDrop v1.0: drop the portal code

## TL;DR

The portal code goes, kept on an archive branch:
the firmware was not extracted, so the Cube keys from the QR code serve nothing.
The hardware findings stay, in a document named for what it holds.

- `massimo portal`, `portal-crypto`, the key derivation, `internal/portal` → gone,
  on `archive/portal`
- `MASSIMO_CUBE_KEY`, `_IV` and their per-Cube forms → read by nothing;
  `massimo config` names a line left as ignored, every printout still hides its value
- `docs/portal-protocol.md` → `docs/cube-protocol.md`, "MAX! Cube protocol";
  the sections only the portal code needed → a pointer to the archive branch
- `docs/firmware-extraction.md` → on the archive branch only
- the Cube's own portal settings (`cube -portal`, `backup -portal`, the `i:` command)
  → stay: they are the Cube's, not Massimo's portal

### Blocks

1. The code: the archive branch, the commands, `internal/portal`, the variables, the tests,
   the Go comments' references; README.md, packaging/env.template
2. The documents: the rename, the trimmed sections, firmware-extraction.md, every reference,
   the contract, the roadmap

## Discussion

The owner decided on 2026-11-13, weighing what `secret.key` might seal (D31):
the Cube keys and IVs in `.env` will not move into the store,
since the firmware was not extracted and the keys from the QR code on the Cube's board
are of no real use;
the portal code is to be dropped, kept on an archive branch.

**The owner's decisions**, 2026-11-13, with `PLAN+ PortalDrop`:

- **D31-A.** The Cube keys do not move into the store; a plan of its own (this one).
- **D33-B.** The code dropped and archived, the hardware findings kept,
  and `docs/portal-protocol.md` renamed to a name that fits what it holds.

**Refined here, routine:**

- **What the portal code is**: `cmd/massimo/portal.go` (the fake portal server),
  `portalcrypto.go` (`portal-crypto`), `derive.go` (the key derivation)
  and `internal/portal` with `portalcrypto`, about 1,600 lines.
  `massimo decode`, `cube -portal`, `backup -portal`, the `i:` command
  and the log's `portal_enabled` are the Cube's own LAN protocol and stay.
- **A `.env` line left over** (`MASSIMO_CUBE_KEY`, `_IV`, `MASSIMO_CUBE_<SN>_KEY`, `_IV`):
  no error, since an owner's file must not stop `serve`;
  `massimo config` lists it as "ignored: the portal commands were removed",
  and `config`, System Info and the reload keep hiding the values of names ending
  `_KEY` and `_IV`, as they hide `_TOKEN`.
- **The archive branch** is `archive/portal`, at `main` before Block 1's commit,
  pushed in Block 1; nothing moves it after.
  It keeps the code and `docs/firmware-extraction.md` as they were.
- **The document's new name** is `docs/cube-protocol.md`,
  its title "MAX! Cube protocol": it holds the LAN and UDP protocols,
  the radio commands, the polling the Cube does and the lessons of the hardware days,
  the portal one part among them.
  The sections that only the portal code needed shrink to a paragraph naming the archive branch:
  "Recovering the IV from a Cube", and the key and IV parts of "Transport".
  What the Cube and the PC software do with the portal
  ("Two clients of the portal", "Portal requests made by the PC software",
  "Lookup service", "Observed polling", "What the PC software knows about the portal")
  stays: it is what the Cube does, which `cube -portal` controls.
- **Other plans and NOTEs naming `docs/portal-protocol.md`** are not edited:
  plans are their sessions', NOTEs are never rewritten,
  and git keeps the old name; `git log --follow` finds it.

**Open, for the owner (the gate):**

- **D34**, the roadmap's item 10, the firmware dump of the spare Cube once the FT232H is in:
  - **A (recommended):** it stays, pointing to `archive/portal`,
    where its document and the code it would revive are.
  - **B:** it goes; the archive branch alone remembers it.

**Not in this plan:**
the Cube keys in the store (D31-A: not at all);
sealing the VAPID key and deriving the sealing key from `opaque.key`
(D30-A and D32-A: AP SealedSecrets, to come).

**Who runs it:**
this session, at the owner's signal on this plan's gate:
the plan registered on main, the archive branch pushed,
both blocks committed on `claude/portal-drop`.

## Analysis

What reads the keys today:
`env.Key`, `env.IV` and their per-Cube forms (`internal/env/env.go`,
in `perCubeSuffixes` and the known names),
`cmd/massimo/portal.go` and `portalcrypto.go`,
`config.go` ("key in effect", "iv in effect", "cubes with a key"),
`reload.go` (`unusedByServe`), the redaction in `config.go` and `serverinfo_api.go`,
and `packaging/env.template`.

Where `docs/portal-protocol.md` is named outside `GUIDELINES/wip/`:
`README.md`, `docs/roadmap.md`, `docs/background.md`, `docs/api-v1.md`, `docs/design.md`,
`docs/fragments/_documents.md`, `GUIDELINES/project/_CONTRACT.md`
(inlined into `AGENTS.md` and `GUIDELINES/README.md` by `sync-docs.sh`),
and the comments of `internal/cubelan/netconfig.go`, `internal/cubeio/pairing.go`,
`internal/cubeio/remove.go`, `internal/server/proxy_virtual.go`, `internal/server/server.go`,
`internal/server/restore_api.go` and `internal/view/view.go`.
`docs/firmware-extraction.md` is named in `README.md`, `docs/roadmap.md`,
`docs/background.md`, `docs/design.md` and `docs/fragments/_documents.md`.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `cmd/massimo/portal.go`, `portalcrypto.go`, `derive.go`, `internal/portal/` | 1 | removed |
| `cmd/massimo/main.go` | 1 | the two commands and their usage lines |
| `internal/env/env.go`, tests | 1 | `Key`, `IV` as removed names; `perCubeSuffixes` |
| `cmd/massimo/config.go`, `reload.go`, tests | 1 | the key lines; the leftover lines named as ignored |
| `packaging/env.template`, `README.md` | 1 | the key lines, the portal commands |
| the Go comments above | 1 | `docs/cube-protocol.md` |
| `docs/portal-protocol.md` → `docs/cube-protocol.md` | 2 | `git mv`, the title, the trimmed sections |
| `docs/firmware-extraction.md` | 2 | removed |
| the documents above, `_CONTRACT.md` | 2 | the new name; the contract's version |
| `CHANGELOG.d/Removed/` | 1 | the fragment |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, a box or GitHub but the pushes of the archive branch
and, at the owner's word, the work branch.
One functional commit per block, citing `AP PortalDrop v1.0, Block <n>`.

### Block 1: The code

1. `archive/portal` created at `main` and pushed.
2. The commands, `internal/portal` and its tests removed; `main.go`'s usage.
3. `env.go`: `Key` and `IV` kept as names only, for `config`'s notice;
   `perCubeSuffixes` without them; `config.go`: the key lines replaced by the notice;
   `reload.go`: `unusedByServe` for the removed names.
4. The Go comments name `docs/cube-protocol.md`, which Block 2 makes.
5. `packaging/env.template`, `README.md` (the commands, the variables, the documents' list).
6. `tools/identifiers/allowed.txt`: entries only the removed code needed.
7. `CHANGELOG.d/Removed/claude-portal-drop.md`:

   ```
   - The portal commands, `massimo portal` and `portal-crypto`, and the Cube
     keys and IVs they read are gone, kept on the branch `archive/portal`;
     a key line left in `.env` is named as ignored.
   ```

**Verification:** the test gate; `massimo config` with a key line in a scratch `.env`
names it as ignored and hides its value.

### Block 2: The documents

1. `git mv docs/portal-protocol.md docs/cube-protocol.md`; its title and contents list;
   the trimmed sections.
2. `docs/firmware-extraction.md` removed.
3. Every reference in the documents named above; `docs/fragments/_documents.md`,
   then `sync-docs.sh`.
4. `GUIDELINES/project/_CONTRACT.md`: the hardware findings go into `docs/cube-protocol.md`;
   its version raised; `sync-docs.sh` into `AGENTS.md` and `GUIDELINES/README.md`.
5. `docs/roadmap.md` item 10 as D34 decides.

**Verification:** the test gate; `GUIDELINES/shared/tools/check-citations.sh`
and a grep for `portal-protocol.md` and `firmware-extraction.md` outside `GUIDELINES/wip/`
find nothing; the site builds (`docs/fragments/_documents.md`).
No CHANGELOG bullet: documents only.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +PortalDrop v1.0

The firmware was not extracted, so the Cube keys from the QR code serve nothing; the owner decided on 2026-11-13 to drop the portal code, kept on an archive branch, and to rename the hardware findings' document. PLAN+ PortalDrop, with D31-A and D33-B.

Key changes:
- GUIDELINES/wip/2026-11-13_12-55_AP_PortalDrop_drop_the_portal_code.md: the plan, two blocks, the decisions in its Discussion

File-revision-v1: GUIDELINES/wip/2026-11-13_12-55_AP_PortalDrop_drop_the_portal_code.md::v1.0::2026-11-13::All::the first revision, by PLAN+ PortalDrop
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `refactor(cli)!: The portal commands and the Cube keys go, kept on archive/portal`;
Block 2, `docs(protocol): The hardware findings are docs/cube-protocol.md`.

## Change History

- v1.0 (2026-11-13): the first revision,
  from the owner's D31-A and D33-B on the evaluation of what `secret.key` may seal.
