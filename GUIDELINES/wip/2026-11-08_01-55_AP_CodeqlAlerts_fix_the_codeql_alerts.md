# AP CodeqlAlerts v1.0: Fix the CodeQL alerts

## TL;DR

CodeQL's first analysis of `main` left three alerts:
two integer conversions are fixed in the code,
and the certificate check skipped on the box's own address is dismissed with its reason.

- #1 `parseHexInt`: `ParseUint(…, 16, 32)` → `int`, negative on a 32-bit build
  past `7fffffff` → 16 bits, `ffff` the most; the Cube sends two digits
- #2 `ParseKSF`: `p ≤ 16` checked on an array element, which CodeQL does not follow
  → the three values named, the same checks and errors
- #3 `healthClient`: `InsecureSkipVerify` for the box's own address, as meant
  → dismissed "won't fix", the code's comment as the reason
- `GUIDELINES/project/CI.md` §8: the switches on, how an alert is closed
- one CHANGELOG bullet under Fixed

### Blocks

1. `parseHexInt` reads 16 bits: `incoming.go`, a test, CHANGELOG
2. `ParseKSF` checks named values: `opaque.go`, the bullet amended
3. `CI.md` §8: the switches on, how an alert is closed
4. The branch pushed, its pull request opened, CodeQL's check read, #3 dismissed

## Discussion

The owner asked for a plan of the CodeQL alerts (`PLAN`, 2026-11-08)
and answered its TL;DR gate with `PLAN++`, naming no decision,
so D1 is as recommended:

- **D1-A.** Alert #3 is dismissed in GitHub as "won't fix", with a comment,
  through the API under this signal.
  Pinning the local `serve`'s certificate would clear it too,
  but the updater would have to find that file for every form of install;
  new code where a mistake stops an update.

The owner asked for the plan to be committed into `main` before the work
branches off, as LogAddress's was.

## Analysis

- **The alerts**, all three on `main` after pull request 103
  (CodeQL default setup: Go, JavaScript and TypeScript, Python, the workflows;
  every language but Go reports none):
  - #1 `go/incorrect-integer-conversion`, `internal/protocol/cube/incoming.go:80`:
    "an unsigned 32-bit integer from strconv.ParseUint to … int without an
    upper bound check".
  - #2 the same rule, `internal/store/opaque.go:163`:
    "… to a lower bit size type uint8".
  - #3 `go/disabled-certificate-check`, `cmd/massimo/update.go:268`.
- **`parseHexInt`** parses the H message's duty cycle and free memory slots
  (fields 6 and 7) and the S message's (fields 1 and 3).
  The Cube sends each as two hex digits (`00`, `32`, `64`);
  a value past `7fffffff` comes only from a broken or hostile peer,
  and turns negative only where `int` is 32 bits (armv6, armv7).
  `ParseUint` with bit size 16 refuses one over `ffff`, and an `int` holds
  every 16-bit value on every platform, so the conversion needs no check.
  Its error says "is not hex", which an out-of-range value is not;
  it becomes "is not a hex number up to ffff".
- **`ParseKSF`** parses `m=`, `t=` and `p=` in a loop into `v [3]uint64`,
  each at bit size 32,
  then refuses any over `maxArgonMemory`, `maxArgonTime`,
  `maxArgonThreads` (16) in one `switch` case, and converts.
  The bound is there; CodeQL's guard analysis follows a variable,
  most likely not an array element, so `uint8(v[2])` reads unchecked.
  Naming `m, t, p := v[0], v[1], v[2]` before the checks gives it one;
  the order of the checks and their errors stay as they are,
  and `FuzzParseKSF` holds the round trip.
  Should the alert remain, `p` is parsed at bit size 8 on its own,
  recorded in a NOTE.
- **`healthClient`** skips verification only for an `https://` URL whose host
  is the box's loopback or one of its interface addresses (`ownHost`);
  a connection to the box's own address never leaves it,
  the certificate may be self-signed or for a name,
  and the version it reads is no secret.
  Its comment says so.
  CodeQL flags the field wherever it is set; a dismissal records the reason
  where the alert is read.
- **`CI.md` §8** says the switches "were off on 2026-11-08";
  the owner turned them on that day, and the API shows them on.
  It gains how a code scanning alert is closed: by a fix,
  which the pull request's CodeQL check shows,
  or, where the code means what it does, a dismissal with its reason.
- **CHANGELOG**: `parseHexInt` and `ParseKSF` are released
  (v0.11.0-alpha.1), so one bullet under Fixed, written in Block 1 and
  amended in Block 2, as COMMIT.md §5.2 asks of one area,
  here the alerts' integer conversions.

## Implementation Plan

The branch `claude/codeql-alerts` from `main` after this plan's commit.

### Block 1: `parseHexInt` reads 16 bits

- `internal/protocol/cube/incoming.go`: `ParseUint(…, 16, 16)`,
  the error "%s: %q is not a hex number up to ffff".
- `internal/protocol/cube/cube_test.go`: H and S messages with `ffff` read
  as 65535, with `10000` refused.
- `CHANGELOG.md`, `### Fixed`:

  ```
  - A Cube's duty cycle or free memory slots over `ffff` are refused
    rather than read as negative on a 32-bit system, as code scanning
    warned (`internal/protocol/cube`).
  ```

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
`GOARCH=arm GOARM=7 go test -c ./internal/protocol/cube` compiles,
`make cross`, `gofmt -l .` empty, `go vet` on linux, windows and darwin.

### Block 2: `ParseKSF` checks named values

- `internal/store/opaque.go`: `m, t, p := v[0], v[1], v[2]`,
  the `switch` on them, `uint32(m), uint32(t), uint8(p)` returned.
- `CHANGELOG.md`: Block 1's bullet amended to name the sign-in's key
  stretching as passing the same check, within three lines.

**Verification:** as Block 1; `go test -run 'KSF' ./internal/store`,
and `go test -fuzz FuzzParseKSF -fuzztime 30s ./internal/store` clean.

### Block 3: `CI.md` §8

- `GUIDELINES/project/CI.md` v1.13.0: the switches on since 2026-11-08;
  a code scanning alert closed by a fix, its pull request's CodeQL check
  showing it fixed, or dismissed as "won't fix" with its reason where the
  code means what it does, as for `massimo update`'s health check on the box's
  own address.
  No CHANGELOG bullet: `GUIDELINES/**` is outside `project.shipped`.

**Verification:** `check` from the main checkout's `GUIDELINES/shared/tools`.

### Block 4: The pull request

- Push `claude/codeql-alerts`, open its pull request.
- CodeQL's analyses of `refs/pull/<n>/merge`: Go's reports one result, #3,
  and #1 and #2 read fixed there.
- Alert #3 dismissed: `gh api -X PATCH …/code-scanning/alerts/3`,
  `state=dismissed`, `dismissed_reason=won't fix`, a comment naming
  `ownHost` and why.

**Verification:** the pull request's checks are green;
the alerts API shows #3 dismissed.

## Proposed commit message

- `fix(cube): A duty cycle or memory slot count over ffff is refused`
- `fix(store): ParseKSF checks its bounds on named values`
- `docs(ci): The security switches are on, and how an alert is closed`

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate of the owner's
  `PLAN`; D1-A by `PLAN++`;
  committed into `main` before the branch, as the owner asked.
