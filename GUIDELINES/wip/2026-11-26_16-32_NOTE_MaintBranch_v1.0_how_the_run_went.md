# NOTE MaintBranch v1.0: how the run went

The owner's PLAN++ of 2026-11-26 ran Blocks 1 to 3 on one branch,
`claude/maint-branch`, built by two sub-agents in parallel:
Block 1 alone, Blocks 2 and 3 together.
Where the work went differently from the plan:

## For Block 1

- **Two more places assumed contiguous numbers.**
  The plan's Analysis said downgrades and `min_reader` need no change;
  two of them did.
  `copyBefore` refused unless the store stood at the version just below,
  so a migration at 0100 that raises `min_reader` would have failed on a
  store at 0026.
  `downParts` counted down one by one,
  and so would have looked for the down parts of 27 to 99.
  It now reads the versions the file's `schema_version` holds.
- **The exceptions of the column check** are all migration 0024's
  renames and drops, each named in the test with its reason.
  Without them, five columns failed on the current dumps.
  The schema check passed on all eleven dumps with no exception.
- **The data check sees a fix's column only where the fix line's dump
  holds a value in it.**
  `tools/store-fixture/make.sh` sends every release the same requests,
  so a fix that adds a column needs a request of its own there;
  `docs/store.md` says so.
- **A decrease** of the numbers cannot occur as a set of files,
  since they are read in order;
  the test refuses 0000 and a number twice instead.

## For Block 2

- **The version comes out of the branch.**
  On `maint/<x.y>`, `make release` counts only the tags of x.y and of the
  lines before it, and refuses a next tag outside x.y;
  `BUMP=minor` starts a line that has no tag yet.
  On `main` it refuses a version of a line that has a maintenance branch
  on origin.
- **Build and Release stay off a push to `maint/<x.y>`:**
  release.yml's Site job runs even where nothing is released,
  and would have deployed the site from such a push.
- **The branch filter** is `maint/[0-9]+.[0-9]+`,
  GitHub's form, as the tag patterns are.
- **The report of jobs no runner took** stays `main`'s:
  on a push to `maint/<x.y>`, as on a tag,
  a macOS or Windows job without a runner fails the run.
- **The commits' component** is `releases`:
  `release` is the release role's token, which `commit-msg.sh` refuses.

## For Block 3

- `make freeze` also refuses a line older than one already released.
- The contract's version manifest row names the maintenance branch too.

## Verification

- `go test ./internal/store/`, `gofmt`, `go vet` for linux, windows and
  darwin, `make actionlint`, and `GUIDELINES/shared/tools/check`
  (12 checks, all well); the whole suite is CI's.
- `make release` and `make freeze` ran in a scratch clone whose `origin`
  was a local bare repository, never against GitHub:
  each refusal, `DRY=1`, and a real freeze there.
