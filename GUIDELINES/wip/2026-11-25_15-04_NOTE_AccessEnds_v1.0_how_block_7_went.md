# NOTE AccessEnds v1.0: how block 7 went

Where Block 7, Changelog, went differently from the plan,
the choices taken in the run, and what verified it.

## What it made

The project's shipped paths leave out the test files (LooseEnds.D48-A, D35-A):
a Go test file at any depth, a package's `testdata/`,
the five scripts that test the packages and the service files,
and the Android app's unit and instrumented tests.
A commit that changes only them touches no shipped path,
so `changelog.sh check --staged --strict` no longer asks it for a bullet.
324 tracked files leave the list, 982 shipped before and 658 after;
none is added.

- **`GUIDELINES/config.ini`:**
  nine `:!` lines in `project.shipped`, after the site's pages,
  under a comment that the tests are not shipped and a commit changing only them
  owes no bullet (D37-A).
- **`GUIDELINES/project/_CONTRACT.md`, v1.20.0:**
  section 1's generated list of the shipped paths, written again by
  `changelog.sh update-contract`.
- **`AGENTS.md` and `GUIDELINES/README.md`:**
  the contract inlined again by `sync-docs.sh`.
- **One commit:**
  "docs(contract): A commit changing only tests owes no CHANGELOG bullet",
  with no CHANGELOG bullet, since none of its paths is shipped (D38-A).

## Different from the plan

Nothing in substance; against the plan's block, item by item:

1. **The config's lines** are the plan's nine, in its order,
   written by hand after the site's pages.
2. **The generated documents** were written by `changelog.sh update-contract`
   and `sync-docs.sh` in a scratch clone of main with the block's files on top,
   since the exported copy the builder worked in has no `GUIDELINES/shared/`,
   and copied into the copy byte for byte.
   Only the contract's title was edited by hand, v1.19.0 to v1.20.0,
   as the plan says.
   Nothing under `GUIDELINES/shared/` changed.
3. **More tries than the plan's four:**
   each group of test files alone, then with a shipped file beside it,
   and each of the five files that stay shipped alone (What verified it).
   Every one of the plan's four gave what the plan expected.
4. **The documents:**
   `GUIDELINES/project/CI.md` as it is, as the plan says.
   The builder also read every passage on test-only commits,
   on COMMIT.md §5.2 and on the bullet's rule:
   `docs/testing.md` and the `Makefile`'s release comment stay true as well.
5. **The commit's message** adds a paragraph the plan did not name:
   a test-only commit still says "No CHANGELOG bullet" with its reason (D38-A),
   and CI's choice of what to test does not read `project.shipped`,
   so such a commit still runs the whole suite.
   Its trailer is
   `GUIDELINES/project/_CONTRACT.md::v1.20.0::2026-11-25::1::The shipped paths leave out the test files`.

## Choices the plan left open

- **The comment says why the test kit and CI's tools stay shipped:**
  `internal/testkit/`, `internal/push/pushtest/` and CI's tools under
  `packaging/` stay shipped, as `.github/` does:
  none of them is a test file (D36-A, D39-A);
  and it names what runs the package test scripts (`make deb-test`,
  `deb-systemd-test`, `apt-test`, CI's macOS job).
  The other way: D37-A's sentence alone.
- **The comment sits directly under the site's lines,** with no blank line.
  The other way: a blank line before it, the tests a group of their own.

## What the reading found

There was no separate reader: a change of four documents,
every case of which the builder tried in a scratch clone.
The coordinating session read the whole diff,
the config's lines and comment, the contract's title and generated list,
`AGENTS.md` and `GUIDELINES/README.md` as regenerated, and the message,
and found nothing to change.

## Seen and left

- **A `testdata/` at the root stays shipped:**
  `:!**/testdata/**` needs a `/` before `testdata`.
  There is none today; the change that adds one.
- **A testdata file that a binary came to embed** would be left out of the
  shipped paths all the same, its change owing no bullet;
  no non-test `go:embed` reads one today;
  the change that adds such an embed takes the file back into the list.
- **A new test source set of the app** (`src/testDebug/`, say)
  **or a new package test script** stays shipped until the list names it;
  the change that adds it.

## For the completion gate

1. **What a contributor notices:**
   a commit that changes only test files passes the strict changelog check
   without a fragment, and its `Key changes` still says "No CHANGELOG bullet",
   with its reason, only tests change, which are not shipped (D38-A).
   A user of Massimo notices nothing.
2. **What leaves, measured on main with the plan:**
   273 `_test.go` files, 15 files under `testdata/`,
   the five package test scripts and 31 files of the app's `src/test/` and
   `src/androidTest/`, 324 in all.
3. **What stays shipped and still owes a bullet,** each tried alone:
   the test kit, `internal/testkit/` and `internal/push/pushtest/`,
   11 files besides their tests (D36-A),
   and CI's `packaging/go-test-report.sh` and `packaging/tested-record.sh`
   (D39-A), both the owner's to change; and the fake Cube's package,
   8 files besides its tests, which the binary holds as `massimo fakecube`.
   The plan's Analysis counted 8 commits of the last 400 that changed the kit.
4. **CI still runs the whole suite on a test-only commit:**
   which tests it runs does not depend on `project.shipped`.
   This pull request runs it too, `config.ini` not being `[ci] untested`.
5. **The wording of the config's comment,**
   which the builder named first among what to read hardest.

## What verified it

- **The builder, in the scratch clone** (main with the plan,
  then the block's commit, the same tree as the commit on main),
  each by `changelog.sh check --staged --strict` with a test-role message,
  exit codes before and after the change:

  | Staged | Alone | With a shipped file |
  |---|---|---|
  | `internal/store/audit_test.go` | 1 / 0, "owes no bullet" | 1 / 1, with `internal/store/audit.go` |
  | five Go tests: the command line's, `internal/`'s, `packaging/`'s, the test kit's, the fake Cube's | 1 / 0 | 1 / 1, with `cmd/massimo/reload.go` |
  | three `testdata/` files: the store's, the Cube protocol's, the password's | 1 / 0 | 1 / 1, with `internal/store/audit.go` |
  | the five package test scripts | 1 / 0 | 1 / 1, with `packaging/deb/postinst` |
  | two app tests, `src/test/` and `src/androidTest/` | 1 / 0 | 1 / 1, with a Kotlin file of `src/main/` |
  | every group at once, 15 files | 1 / 0 | 1 / 1, with `packaging/deb/postinst` |

  With a CHANGELOG fragment beside `audit_test.go` and `audit.go`, 0.
  `internal/testkit/clock.go`, `pushtest.go`, the fake Cube's `apply.go`,
  `go-test-report.sh` and `tested-record.sh`, each alone, 1 / 1.
  The block's own change staged, 0 with `--strict` and 0 without,
  "owes no bullet".
  `fingerprint.sh shipped` over the commit, false: it touches no shipped path;
  `fingerprint.sh same`, false: the code's fingerprint changes,
  so the pull request runs the whole suite.
  `GUIDELINES/shared/tools/check` over the range, exit 0,
  twelve checks all well (line breaks 0 errors, 1162 warnings,
  all there before);
  `sync-docs.sh --check`, 0;
  `commit-msg.sh` on the message, 0 errors and 0 warnings;
  `reflow.sh --check` on the contract, `AGENTS.md` and `GUIDELINES/README.md`,
  0 each, the contract's 16 warnings the same as before.
- **The builder, in the exported copy:**
  `make ui`, 0; `gofmt -l .` printing nothing;
  `go vet` for linux, windows and darwin, 0;
  the comment's lines at most 79 columns.
  `commit-msg.sh` in the worktree gave 1 then, "no such plan is committed",
  since the worktree stood at "chore(aps): +NOTE StartConfiguration v1.1,
  how the run went", before the plan; on main with the plan it gave 0.
- **Not run:**
  no `go test`, `make cross` or UI test,
  since no Go, page or test file changed;
  not GitHub's Guidelines job, which the pull request runs.
- **The coordinating session,** the commit made on main's head,
  "chore(aps): +AccessEnds v1.0", the block's own base,
  in a clone with its tree:
  `commit-msg.sh`, 0; `changelog.sh check --staged --strict`, 0,
  "owes no bullet";
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well.
