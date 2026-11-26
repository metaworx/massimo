# NOTE StartConfiguration v1.3: how the run went

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.

This is the run of v1.3's Blocks 23 to 28, by the owner's `PLAN++` of
2026-11-26.
- Each block was built by a sub-agent in a worktree of its own: Blocks 23
  to 27 from `maint/0.17`, Block 28 from `main`.
- The session verified and committed each block.
- Blocks 23 to 27 were gathered onto one branch by cherry-pick, and a
  second reader read them there; Block 28 had a reader of its own.
- Every test fails on its planted fault, the Windows ones through interop.
- Nothing reached real hardware.

## The pull requests

| Blocks | Pull request | Base | State |
|--------|--------------|------|-------|
| 23 to 27 | #471 | `maint/0.17` | open |
| 28 | #470 | `main` | open |

v1.3 itself went onto `maint/0.17` and was merged upward into `main` at
once (D69-A). AP MaintBranch's NOTE then made that the rule: plan commits
follow the work.

## Where the run went differently from the plan

- **Block 23** also hides the value of names that Linux ignores for their
  case, such as `MASSIMO_Listen` or a Cube's name in another case, since
  `massimo config` already marks them unknown.
  - docs/ui.md is updated too.
  - `massimo config`'s usage still says "secrets redacted", which is true
    but incomplete.
- **Block 24** amends Block 17's unreleased Fixed bullet. After the
  reading, the bullet says again that serve used to read a Cube's line in
  another case.
- **Block 25** is two commits.
  - The `certs` bullet names the start alone: install's refusal of a TLS
    pair is Block 15's, unreleased.
  - The hook's absolute path is judged by `filepath.IsAbs` on Windows and
    by a leading `/` for Linux and macOS, on any host.
- **Block 26** is two commits, with no bullet. docs/store.md is updated,
  though the plan named no docs.
  - The Analysis overstated item 8: a zone written alike did tie. The one
    defect was a false tie to the same zone in another case.
- **Block 27:**
  - `tr -d '\000'` alone still left grep's message under a UTF-8 locale,
    so its grep runs with `LC_ALL=C`, and the strip comes before sed.
  - A UTF-16 template without a byte order mark now stays stopped, as the
    template does. Block 19's unreleased bullet says so, and a second test
    case pins it.
- **Block 28** is two commits.
  - The reader showed the downgrade is released, so `fix(update)` carries
    a Fixed bullet after all; the plan said none (COMMIT.md §5.1).
  - Its comment names "file exists" as one of the hidden errors.
- **After the reading, three fragments were reworded,** by rebuilding the
  unpushed commits of Blocks 23 to 27.

## Found outside the plan, not changed

- `keepAs` (internal/update/install.go) ignores its removal of `<dst>.tmp`
  in the same way, so a `.tmp` that cannot be removed shows as "file
  exists".
- The server's own tie warning, where the tie is by serial
  (internal/server/server.go, `plainServedAlready`), gives the plain address
  alone.
- cmd/massimo/tls.go wraps `certs.Generate`'s "tls:" once more. It cannot
  be reached, since `Names()` always adds the box's names.
- `checkTLS` joins a stray `-root` onto a Windows drive path. `-root` is
  for Linux and macOS, and this is older than the plan.
- `env.Get` falls back to the plain `MASSIMO_CUBE_ADDRESS` for any `-sn`.
  This is older; Block 24 only adds a Cube's name in another case to what
  reaches it.
- Under load, `TestProxyRetriedUntilTheAddressIsThere` races its own 5 ms
  retries. `TestHeldReferenceLifetime` saw a PUT take 5.3 s.
  `TestDownloadOutlastsTheClientTimeout` failed once on Windows through
  interop, against its 250 ms timeout.
- The owner's Windows service was not touched. Its logs and store changed
  during the runs from its own work.
