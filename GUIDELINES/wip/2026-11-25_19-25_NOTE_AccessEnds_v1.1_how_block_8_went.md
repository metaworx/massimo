# NOTE AccessEnds v1.1: how block 8 went

Where Block 8, Versions, went differently from the plan,
the choices taken in the run, the bump the plan asked for tried both ways,
what the reading changed, and what verified it.

## What it made

`internal/credits/credits.json` names no Go module's version any more (D40-A):
the binary fills each in from its build information
(`runtime/debug.ReadBuildInfo`) as the list is read,
as it already did Go's from `runtime.Version()`,
for System info, `/ui/credits.json` and `massimo credits`.

- **Which version:**
  a module's own; a module replaced by another, the replacement's;
  the copy in `third_party/` that replaces `github.com/bytemare/ksf`,
  the version `go.mod` requires (v0.5.0), as before;
  a module the binary does not link, none.
  A vendored entry keeps its note's.
- **A module the binary does not link:**
  on linux `github.com/mattn/go-isatty` and `github.com/ncruces/go-strftime`,
  which only the windows and darwin builds link.
  `massimo credits` prints `-` for its version and "Go module,
  in another platform's build" for where it comes from;
  System info draws its head without a version and says the same in its body,
  in six languages.
  The command's usage and its line in `massimo`'s list of commands say that the
  list names what a build for any platform includes.
- **So a module's update in `go.mod`**, as Dependabot's of `golang.org/x/net`
  from 0.62.0 to 0.63.0, fails no test and needs no `make credits`;
  a module added or removed, or a licence's text an update changes,
  still fails `TestGeneratedFileCurrent` and `TestEverythingIncludedHasAnEntry`
  until `make credits` runs.
- **The credits' ETag** holds the first 16 hex characters of the SHA-256 of the
  list served, in place of Go's version.
- **`credits.json`** from 399,188 to 398,502 bytes:
  only the 24 modules' version lines are gone.
- **One commit:**
  "fix(server): The credits take each Go module's version from the binary",
  built on Block 6's commit, "fix(server):
  The credits hold the licence texts of the OPAQUE client's crates",
  since both change `tools/credits` and `internal/credits`.

## Different from the plan

Against the plan's block, item by item:

1. **Changes 1, `tools/credits`,** as the plan has it;
   its comments and its README say why the file holds no module's version,
   and what an update still needs.
2. **Changes 2, `internal/credits`,** as the plan has it,
   with a rule the plan left open (Choices, below):
   the build information gives a module replaced by a directory `(devel)`,
   and the list shows the version `go.mod` requires for it.
   `credits.GoVersion` is gone: only the old ETag used it outside the tests;
   its comment moved to `goVersionOf`.
3. **Changes 3, the versions as before,** but for the two modules a linux build
   does not link, which showed the file's version before and show none now,
   with the origin above.
   The origin's words came with the rework (What the reading found);
   they are the coordinating session's.
   FR, IT, NL and PL are machine-translated;
   those files carry no mark per key,
   only their `i18n.machineNote`, which says it for the whole file.
4. **Changes 4, `make credits`,** done; the file as above.
5. **Beyond the plan's changes:**
   - **the ETag** (`internal/server/ui.go`, `creditsDigest`):
     the served list now depends on the modules' versions,
     so the old ETag of the server's version and Go's would answer 304 with a
     stale list from a server of the same version and Go built with other
     modules, a distribution's build with a patched `go.mod` for one.
     The digest is computed once per process over the bytes served;
     `Cache-Control` stays `no-cache`.
   - **the command's usage** and its line in the list of commands
     (`cmd/massimo/credits.go`, `main.go`),
     which said "everything this binary includes".
6. **Tests beyond the plan's:**
   `TestBuilt` holds the rule on made-up build information;
   `TestVersionsAreTheBinarys` (renamed from `TestGoVersionIsTheBinarys`) holds
   the modules' versions too;
   `TestMadeUpTree` writes a module without a version;
   `TestChangedModulesFailCheck` passes another version and fails another
   licence text;
   `TestCreditsList` of `cmd/massimo` holds each module at the binary's version,
   the origin of every module, and `-` with "Go module,
   in another platform's build" for one it does not link;
   `TestUIServed` holds the new ETag;
   `test_system_info.py` holds `golang.org/x/net`'s head at the version `go.mod`
   requires, the one end-to-end check on a real build, and,
   in `test_credits_module_of_another_build`,
   a module the served list has no version for read as another platform's.
7. **Documents beyond the plan's:**
   the Makefile's comment on `make credits`, and the command's usage (5).
8. **The CHANGELOG:**
   `CHANGELOG.d/Added/claude-credits.md` amended
   ("each Go module at the version the server was built with"),
   not the plan's `CHANGELOG.d/Fixed/claude-accessends-credits-versions.md`.
   The credits came in on 2026-11-25, after v0.16.0-alpha.1,
   and are not released yet;
   `GUIDELINES/shared/COMMIT.md` §5.2 has a defect introduced and fixed within
   one release amend the bullet that introduced it,
   and keeps `### Fixed` for a released one.
   Block 6 amended the same fragment.
   The bullet is three lines, of 69, 72 and 71 characters.
9. **The verification:**
   the plan's, each by its exit code;
   the bump (The bump, both ways) on the block's first tree;
   `make cross` and the whole `go test ./...` on the first tree only.

## Choices the plan left open

- **A module replaced by a directory** (`bytemare/ksf` in `third_party/`):
  the version `go.mod` requires, as before.
  The other way: `(devel)`, as the build information gives it, or none.
- **What a linux build says of the two modules it does not link:**
  no version, with "Go module, in another platform's build",
  the words the coordinating session chose at the rework.
  The other ways, the reader's:
  `tools/credits` recording which platforms link each module,
  which changes only where a module is added or removed;
  or the plain "Go module", the reason in the documents alone.
  Keeping the file's version for them would make their bumps fail CI again.
- **The ETag's digest:**
  16 hex characters of the SHA-256 of the list served,
  after the server's version.
  The other way: Go's version in the ETag, as before.
- **`credits.GoVersion` removed,** its only user outside the tests gone.
  The other way: kept, unused.

## The bump, both ways

The plan's check, in scratch copies:
`go get golang.org/x/net@v0.63.0` from the Go module proxy,
`go.mod` and `go.sum` changing for `golang.org/x/net` alone.

- **On the block's first tree:**
  `go get` exit 0;
  `go test -count=1 ./tools/credits/... ./internal/credits/...` exit 0;
  `go run ./tools/credits -check` exit 0;
  a built `massimo credits` lists `golang.org/x/net v0.63.0`.
- **On the base, Block 6's commit:**
  `go get` exit 0; the same tests exit 1,
  `TestGeneratedFileCurrent` ("not current") and
  `TestEverythingIncludedHasAnEntry` (v0.62.0 has an entry but is not linked,
  v0.63.0 is linked with none), as on Dependabot's pull request;
  `-check` exit 1.
  The base's binary built with the bump listed `golang.org/x/net v0.62.0`,
  which was wrong.
- **The reader** did it again offline,
  `golang.org/x/net` v0.63.0 from the module cache with `GOPROXY=off`:
  on the first tree the tests of both packages,
  `-check` and `TestCreditsList` pass, and the built binary lists v0.63.0;
  on the base the same two tests fail, exit 1.

## What the build information holds

What the coordinating session asked beyond the plan,
measured by the builder in a scratch module and on the real tree:

- **`go run`, `-trimpath`** (the cross builds,
  with `-s -w`) **and `-buildvcs=false`** (every build of the copies) all carry
  the modules' versions;
  `-buildvcs` changes only the version control's settings.
- **A test binary** carries the modules its package links:
  `cmd/massimo`'s all of them, `internal/credits`' none,
  so the list that package's tests see has no module's version.
- **A module replaced by another**
  (`golang.org/x/text` replaced by its v0.43.0, offline from the cache):
  the list shows v0.43.0; the tests and `-check` exit 0.
- **What a build lacks:**
  on linux `go-isatty` and `go-strftime`;
  `go version -m` shows them in the windows and darwin binaries,
  and the reader's `go list -deps` finds them on darwin and windows
  and on none of linux amd64, arm64, armv7 and armv6.
- **The page** draws a version only where the list has one (`Credits.tsx`);
  `/ui/credits.json` is the bytes `massimo credits` reads (`TestUIServed`).

## What the reading found

One builder built the block in an exported copy of Block 6's commit.
One reader read the recorded first tree, the code and the tests.

Confirmed:
`go version -m` gives `bytemare/ksf` v0.5.0 replaced by
`./third_party/bytemare-ksf` `(devel)`,
and the list shows v0.5.0 with its copy's origin, as before;
a binary without build information gets no module's version and does not fail;
a linux `massimo credits` on the tree differs from the base's in the two
modules' lines alone;
a version left in the file fails `TestGeneratedFileCurrent` and
`TestVersionsAreTheBinarys`,
a module missing from the list `TestEverythingIncludedHasAnEntry`;
the ETag can keep no stale list and cause no refetch without end,
`TestUIServed`'s loop of 304s covering `/ui/credits.json`;
`/ui/credits.json` was public and carried versions before, and nobody may do
more than before.

Planted faults:
the `(devel)` condition dropped fails `TestBuilt` and `TestCreditsList`
(`bytemare/ksf` printed `(devel)`);
the list served with no build information fails `TestCreditsList` of
`cmd/massimo` alone (`golang.org/x/net` printed `-`).

No must.
Four shoulds:

1. **A linux build did not say why two modules show no version:**
   the reason was only in `README.md` and `docs/ui.md`,
   and the usage still said "everything this binary includes".
   Done in the rework, in the coordinating session's words (Choices, above),
   with a test each,
   `TestCreditsList` on linux and `test_credits_module_of_another_build`,
   each seen failing on the tree before the rework.
2. **The CHANGELOG departs from the plan, and rightly**
   (Different, 8): recorded here.
3. **Three changes the plan does not name,**
   the ETag, `GoVersion`'s removal and the Makefile's comment:
   recorded here (Different, 2, 5 and 7).
4. **Two comments left out a case:**
   `tools/credits/main.go` and `generate.go` said a module's update needs no new
   list unless it changes a licence's text; one that adds or removes a module,
   one it requires in turn too, needs it as well.
   Done in the rework.

There was no second reading:
the rework's two points were the reader's own shoulds,
and the builder's report shows each with its test seen failing.

## Seen and left

- **`massimo credits golang.org/x/net`** prints the entry's licence texts and no
  version, as before the block;
  the coordinating session's brief asked it to show 0.63.0 after the bump,
  and the list does.
  The owner's, should the entry's print name its version.
- **`TestVersionsAreTheBinarys` can see only empty modules' versions:**
  `internal/credits`' test binary links no module.
  `TestCreditsList` of `cmd/massimo` copies `built`'s rule (`builtVersions`)
  where `TestBuilt` pins it with fixed values.
  The tests', as they stand.
- **The UI assertion of `golang.org/x/net`'s version** passes on the base by
  design, the version came from the file there; its docstring says so,
  and the builder saw it fail on a copy whose server fills in no module's
  version.
  The test's, honest as it stands.
- **The builder's first `test_credits_module_of_another_build`** read the origin
  inside a closed fold and got an empty text before and after the rework,
  so its first failure proved nothing;
  it opens the folds now and failed again before the rework for its own reason.
  Settled in the run.

## For the completion gate

1. **What a user on a linux build sees** (a Raspberry Pi, a Debian package):
   `github.com/mattn/go-isatty` and `github.com/ncruces/go-strftime` with no
   version, `-` in `massimo credits`, and "Go module,
   in another platform's build" as where they come from,
   in System info and on the command line;
   before the block they showed v0.0.24 and v1.1.0, the file's,
   which the linux binary does not link.
   On the windows and darwin builds they show their versions,
   and every other version reads as before.
   The words are the coordinating session's, the owner's to change.
2. **The credits' ETag changed:**
   the server's version, then the first 16 hex characters of the SHA-256 of the
   list served, where it held Go's version;
   a server of the same version built with another Go or other modules serves its
   own list.
3. **The CHANGELOG** amends the unreleased `Added` bullet of the credits,
   not the plan's new `Fixed` fragment, as `GUIDELINES/shared/COMMIT.md` §5.2
   asks for a defect introduced and fixed within one release (Different, 8).
   The plan's way: the `Fixed` fragment with the plan's bullet.
4. **A bump of a Go module no longer fails CI:**
   the real bump of `golang.org/x/net` passes on the block and fails on the base
   (The bump, both ways).
   A module added or removed, or a licence's text changed, still fails until
   `make credits` runs, as it should.
5. **Dependabot's pull request that showed the fault** (`golang.org/x/net`
   0.62.0 to 0.63.0) is another session's,
   and the owner's to run again once this block is merged,
   brought up to main first:
   a re-run of its old jobs tests its merge with main as it was then.

## What verified it

- **The builder, on the first tree,** each by its exit code, 0:
  `make ui`, `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin,
  `make credits` twice with the same bytes, `go run ./tools/credits -check`,
  `go test -count=1` of the touched packages and of `internal/serve`,
  which imports them, the whole `go test ./...`, `make cross`,
  `reflow.sh --check` on the four Markdown files changed, 0 errors,
  no new warning in the three of prose,
  the strict changelog check on a scratch index of the tree,
  and `commit-msg.sh` on the message;
  `test_system_info.py` and `test_parts_guard.py` in all four cases,
  106 passed; the whole UI suite in 1100-en, 822 passed.
- **Seen failing first,** on the base with the new tests and a stub of `built`
  that sets Go's version alone: `TestVersionsAreTheBinarys`, `TestBuilt`,
  `TestGeneratedFileCurrent`, `TestMadeUpTree`,
  `TestChangedModulesFailCheck` (another version refused),
  `TestUIServed` (the ETag held `go1.26.8`),
  and `TestCreditsList` of `cmd/massimo`,
  which fails this way only where linux builds the tests (`go-isatty` printed
  v0.0.24, `go-strftime` v1.1.0, want `-`).
- **The bump and what the build information holds,** above,
  by the builder on the first tree and the base.
- **The reader,** on the first tree, in an export of its own:
  `go test -count=1` of `internal/credits`, `tools/credits`, `cmd/massimo`
  and `internal/server`, exit 0;
  `make ui`, exit 0; `gofmt` printing nothing;
  `go vet` for linux, windows and darwin over the touched packages, exit 0;
  `make credits` writing the tree's file byte for byte, its size as above;
  `test_system_info.py` in all four cases, 40 passed;
  `go version -m`, `go list -deps` for each platform,
  builds for linux/amd64, windows/amd64 and darwin/arm64,
  the linux `massimo credits` of the base and the tree compared;
  the two planted faults; the bump offline both ways;
  `reflow.sh --check` on the three documents, 0 errors,
  their warnings from 21 to 20, 404 to 404 and 7 to 6;
  `commit-msg.sh`, exit 0.
  It ran neither the whole `go test ./...`, `make cross`, `make race`,
  the whole UI suite, `GUIDELINES/shared/tools/check` nor the changelog check,
  and did not read `generate.go` past `thirdPartyEntries`.
- **The builder, after the rework,** each by its exit code, 0: `make ui`,
  `make ui-check`, `gofmt -l .` printing nothing, `go vet` for linux,
  windows and darwin, `go test -count=1` of `internal/credits`, `tools/credits`,
  `cmd/massimo` and `internal/server`, `make credits` twice with the same bytes,
  the list the same as before the rework, `go run ./tools/credits -check`,
  `reflow.sh --check` on the four Markdown files changed, 0 errors,
  and `commit-msg.sh` on the message;
  the Go checks before the last fix of the UI test file,
  which touches no Go and no Markdown;
  each new test seen failing on the tree before the rework;
  `test_system_info.py` and `test_parts_guard.py` in all four cases, 110 passed.
  The whole `go test ./...`, `make cross` and the whole UI suite were not run
  again after the rework.
- **The coordinating session:**
  the step carried onto main's head,
  "docs(api): The capability tables name every route the server guards",
  which holds Block 6's commit, merged as #449,
  by a three-way merge of trees without conflict;
  in a clone with the commit:
  `commit-msg.sh`, 0 errors and 0 warnings;
  `changelog.sh check --staged --strict`, 0;
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well.
