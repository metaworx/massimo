# ANALYSIS CleanRuns: what the test and build scripts leave behind

Written 2026-11-25 for the owner's question: "is there a way to improve test
and build scripts to clean up after successful completion?"
Read-only: no project file changed. Measured on jaguar against `origin/main`
(`chore(aps): +ToldData v1.0, +ANALYSIS ToldData`).

## 1. What each target or script leaves, and where

| Target / script | Left behind | Size measured | Kept for |
|---|---|---|---|
| `make ui-check` | `tools/ui-check/node_modules/` in the checkout (`npm ci`) | 64 MB on disk per checkout (127 MB apparent: Biome 97, TypeScript 7's native binary 27); 11 of 45 worktrees have one | the next run, which runs `npm ci` again all the same (7 s with npm's cache warm) |
| `make cross` (in the test gate), `packages`, `store-bench`, `site`, `deb-unit` | `dist/` | 53-167 MB per checkout, most 77-90 MB (`make cross`); 26 of 45 worktrees have one | `packages` reads `cross`'s binaries; the tests of the packages read `dist/` |
| `make build` | `massimo` | 12-19 MB | the product |
| `make ui` (every Go target) | `internal/server/ui/parts.js` | 30-180 KB | every Go build |
| `make ui-test` (`tools/ui-test/run.sh`) | on a **failure**: `results/` (screenshots, `results/retry/` videos and traces, `results/logs/`); always: `.pytest_cache/` (6-16 KB), `__pycache__/`; `/tmp/tmp.*` work dir only when killed (33-53 MB: the binary and four sets' stores and logs); Playwright's `/tmp/playwright-artifacts-*` only when killed (137 MB now in `/tmp`) | 26 MB per failed copy on average (2.1 GB / 80) | CI uploads `results/` on failure (`ui-test-results-<case>`) |
| `make app-emulator-test` | `tools/app-emulator/results/` after every run, pass included; the AVD `~/.android/avd/massimo-test.avd`, one for all checkouts, rewritten and wiped (`-wipe-data`) at each run; Gradle's `app/android/app/build/` | results 5 MB; AVD 1.2 GB; Gradle 5-29 MB | CI uploads `results/` on a failure, a cancellation **and a pass** (7 days) |
| `make lan-test` | the image `massimo-lan-test:avahi`; nothing else: a compose project per PID, `down --volumes --remove-orphans` and the work dir removed on exit | 101 MB, one for all checkouts | reused, layer cache |
| `make deb-test`, `make apt-test` | nothing (`docker run --rm`, work dir trapped) | - | read `dist/`, which CI uploads after them (`packages`) |
| `make deb-systemd-test` | the image `massimo-systemd-test:<release>`; containers removed, work dir trapped | 110-115 MB per release | reused |
| `make test-armv6`, `make race`, `go test` | Go's build cache only (no `GOCACHE` of their own) | see note 2 | - |
| `make fuzz` | a crash's input under `internal/**/testdata/fuzz/`; generated corpus in `$GOCACHE/fuzz` (1 MB) | - | the crash is the point; CI uploads it on failure |
| `make ui-brand` | `tools/brand/.venv` per checkout | 37 MB | reused |
| `tools/store-fixture/make.sh` | nothing: its temporary worktree is removed on exit | - | - |
| `make build-release` | `dist/` (starts with `make clean`) | - | CI uploads `dist/` |

No container, volume or network of Massimo's is left on jaguar's Docker.

## 2. Removable after a pass, kept after a failure, read later

- **UI suite.** A passing run already leaves nothing but `.pytest_cache`:
  pytest-playwright 0.10 removes the `--output` directory at the first test of
  every run, workers or not (checked in a scratch directory: an old `results/`
  is gone after a run of one passing test; it stays after a run that selects
  none). `run.sh` removes `results/logs` itself and copies the logs only on a
  failure. So the 2.1 GB were failed runs' evidence in copies that never ran
  again, not a pass's leftovers. CI reads `results/` only on failure.
- **Emulator suite.** `results/` is emptied at its start and kept at its end;
  CI uploads a pass's too, so a pass's results may not be removed in CI.
  They are 5 MB.
- **`dist/`.** The gate's `make cross` leaves 77 MB that nothing reads;
  `make packages` needs `cross`'s binaries, and CI uploads `dist/*.deb` after
  the package tests and `dist/` after `build-release`, so `make cross` itself
  must keep writing them.
- **`node_modules`.** Needed by every `make ui-check`, read by nothing else;
  `npm ci` reinstalls it at every run anyway.

## 3. A switch to keep everything

What a run would remove under the proposal is its own temporary state (work
directories); what CI reads (`tools/ui-test/results/`,
`tools/app-emulator/results/`, `dist/`, `testdata/fuzz/`) is never removed.
So cleaning can be the default locally and in CI alike; `KEEP=1` keeps a
suite's work directory (the servers' stores and logs of a passing run, which
today no switch keeps) and prints its path.

## 4. `node_modules` shared between checkouts

`tsc` and `biome` run from an install outside the checkout without change:
`tsconfig.json` has `"types": []` and maps Preact to `third_party/`, and
Biome takes `--config-path=tools/ui-check` (checked: the check passes run from
a scratch install). What it needs:

- a directory keyed by the lock and the platform, so a Mac and WSL never share
  Biome's binary: `${XDG_CACHE_HOME:-~/.cache}/massimo/ui-check/<sha256 of
  package-lock.json, 16 digits>-<node's platform-arch>/`;
- the install into a temporary directory beside it, renamed into place when
  none is there yet, so that two sessions at once never use a half install
  (the second discards its own);
- the directory touched at each use, and one not used for 30 days removed by
  the next run: a branch on an older lock still finds its own, and none in use
  is taken from another session;
- in the Makefile: `ui-check` calls a small script that does this and prints
  the directory, then runs `tsc` and `biome` from it; `sha256sum` or
  `shasum -a 256` for macOS;
- in CI: nothing; the runner's `~/.cache` is fresh each job, as
  `node_modules` is now. An `actions/cache` of the directory would save the
  64 MB download; not proposed.

Lost: an editor that finds TypeScript or Biome's schema in
`tools/ui-check/node_modules` (`biome.jsonc`'s `$schema`) no longer does.

## 5. `make clean` and a `make distclean`

`build-release` runs `clean` first, so a `clean` that took test results would
delete a failed run's evidence at a local release build. Hence a separate
`make clean-tests` (the two `results/`, `.pytest_cache/`, `__pycache__/`), and
`make distclean` for what make itself installed (`tools/ui-check/node_modules`
of before, `tools/brand/.venv`, Gradle's build directories). Never:
`.env`, stores, `tools/ui-test/.venv` (made by hand, its README), and the
caches every checkout shares (Go's, npm's, the shared `ui-check` install,
Docker's images): another checkout's run may be using them.
`git clean -X` is no substitute: it removes `.env` and `massimo.db`.

## 6. An earlier failed run's results when this run passes

Already gone: the UI suite's at its first test (pytest-playwright), its logs
by `run.sh`; the emulator suite's at its start. One gap: a UI run that stops
before its first test (a server that does not start) leaves the earlier run's
`results/`, which then look like this run's.

## 7. Outside the scripts

- **The 12 GB scratch directory.** Now in another worktree's
  `GUIDELINES/temp/` (another run's exported copies): 11 GB, of which
  `node_modules` 0.96 GB, `dist/` 0.89 GB, UI results 0.14 GB. The rest is the
  copies themselves (24 MB of source each, many copies per block) and what
  was built in them. Removing a copy once its reader is done is the copying
  session's job; no project script sees them.
- **Go's build cache.** 34 GB on disk (`~/.cache/go-build`), nearly all
  written in the last two days; Go trims what is unused for five days itself.
  Shared by every session, so no script should clean it. A build without
  `-trimpath` keys its packages by their directory, so each exported copy
  compiles every package anew; `make cross` uses `-trimpath`, `go test` does
  not.
- **The emulator's AVD.** One name, `massimo-test`, for every checkout: two
  runs at once collide even on different `APP_EMU_PORT`s (the second
  rewrites the first's `config.ini`, and the emulator refuses a second
  instance of one AVD). Pre-existing.
- **Docker.** Stale images: `massimo-mdns-test:avahi` (83 MB, from before
  lan-test) and `massimo-armv6-test:latest` (1.35 GB, a rig made by hand,
  not in the repository).
