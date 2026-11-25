# NOTE PasswordRules v1.0: how block 5 went

Where Block 5, Credits,
went differently from the plan and from its NOTE on the command,
the choices taken in the run, and what verified it.

## What it made

System info has a fold, Credits and licences,
that lists everything the binary includes, 31 entries:
Massimo itself, Go's runtime and standard library,
the 24 Go modules `./cmd/massimo` links on the seven platforms of `make cross`,
Preact and its two signals packages, Tabler's icons
and the vendored OPAQUE client (D30-A).
Each entry is a fold of its own, with its name, version and licence,
then where it comes from and each of its licence files,
named by its licence, in a box that scrolls.
The list is fetched at the fold's first opening and not again.

`make credits` writes it, by `tools/credits`,
into `internal/credits/credits.json`, which git keeps and the binary embeds;
`go test` and `go run ./tools/credits -check` fail where it has fallen behind:
a module linked,
a directory of `third_party/` or a file of `ui/vendor/` without an entry,
a text that differs (D52-A).
The server serves it at `/ui/credits.json` as it serves the page's files,
revalidated by an ETag.
`massimo credits` prints the same list, a line an entry,
and `massimo credits <name>` that entry's licence files with their texts (D53-A).

- **Two commits:**
  "feat(server): System info lists the credits and licences" and "feat(cli):
  massimo credits prints the credits and a licence".

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **Two commits, not the plan's one:**
   the command of the NOTE on the credits command is a change of its own,
   and its commit amends the CHANGELOG bullet so that it names both.
2. **The modules of the seven platforms of `make cross`,**
   not linux, windows and darwin alone:
   `TestTargetsAreTheMakefiles` holds the list of platforms to the Makefile,
   and fails on a line with `$(CROSS)` that it does not read.
3. **Go's version is the running binary's,** not `go.mod`'s line:
   the committed file carries none,
   `internal/credits` sets `runtime.Version()` without its `go` in front,
   for the served file and the command alike,
   and the ETag of `/ui/credits.json` holds it.
   With `go.mod`'s line, a release built by the newest 1.26 printed one Go
   in its version and another in its credits.
4. **Go's licence files** are `LICENSE` and `PATENTS`, not `LICENSE` alone.
5. **A licence is named only where exactly one matches,**
   by its title or by sentences only it has; else by none,
   which the page shows as "licence in its text".
   The plan had "see the text" for a text of none of a few known licences;
   the first build's classifier also gave wrong names (below).
   `gopkg.in/yaml.v3` therefore reads "licence in its text":
   its one `LICENSE` holds the MIT text and Apache's notice.
6. **A vendored entry takes its licence from its note's Licence column,**
   which must agree with the licence its text is known by,
   or `make credits` stops naming both.
   `third_party/tabler-icons/MASSIMO.md` gains a table with the package,
   its version and its licence,
   and `ui/vendor/README.md`'s table gains Package and Includes.
7. **Mixed entries:**
   each file carries its own licence,
   `NOTICE` and `PATENTS` are marked as notices,
   and an entry with a licence file under another licence or none is mixed:
   the page says "other licences among its files" in six languages,
   the command "and others".
8. **What an entry includes**
   (Includes) is a field of its own:
   the OPAQUE client's entry names the Rust crates its WebAssembly names,
   and says that their own licence texts are not reproduced (below).
   `TestVendoredOpaqueCrates` holds the column to the file.
9. **A licence file over 256 KiB stops the generator,** naming the file.
10. **Not in the plan's tests:**
    the focus after a try that failed, an answer after leaving the page and
    after a sign-out, `TestGoVersionIsTheBinarys`, the ETag's Go version,
    the classifier on full texts and on misleading ones,
    a row that names a file, a licence file too large;
    `test_parts_guard.py` opens the credits and Preact's licence.
11. **Not in the plan's documents:**
    `tools/credits/README.md` (new), `docs/ui-testing.md`,
    `third_party/tabler-icons/MASSIMO.md`;
    `.gitattributes` keeps `credits.json` with LF.
12. **The command beyond the NOTE on it:**
    the list ends with a line on what an entry includes;
    `credits <name>` prints that line first, then each file with its licence;
    two names are refused;
    a test runs the command as a binary of its own in an empty directory,
    writing nothing;
    an unknown name exits 1, as a wrong argument to the other commands does,
    and so does `-h`.

## Choices the plan left open

- **Go's licence where a distribution's Go keeps it elsewhere:**
  Debian's `GOROOT` has no `LICENSE` or `PATENTS`;
  the generator then takes them from `golang.org/x/sys`,
  whose copies are the same bytes as a toolchain's.
  The other way: stop, and ask for a toolchain that has them.
- **The note-table convention:**
  a note's first table with a Version column names what its directory holds,
  a row an entry, its first column a directory holding its own licence file,
  its columns Package, Version, Licence and Includes read by their heads;
  a directory without such a table is one entry named by the directory,
  without a version.
  `tools/credits/README.md` says it in full.
  The other way: a fixed list in the code, which would fall behind.
- **How a licence is named:**
  the GNU licences version 3,
  MPL-2.0 and the CC licences by a title that opens a line,
  its version there or in the next; Apache-2.0 by its title,
  its sentences or its notice; MIT,
  ISC and the BSD licences by sentences only each has;
  a second known title or a second licence's sentences name a file by none.
  The other way: the note's Licence column alone, which modules lack.
- **"Mixed" entries:**
  `modernc.org/sqlite`, `libc` and `memory` (`memory`'s `LICENSE-LOGO`,
  a bare address, makes it so).
  The other way: name each file's licence only, with no mark on the entry.
- **The cap on a licence file, 256 KiB:**
  three times today's largest, sqlite's `LICENSE-3RD-PARTY.md`,
  since the whole list goes to a phone that opens the fold.
  The other way: no cap.
- **A sign-out forgets the list,** so the next user's page fetches it again;
  the file is public and the same for everyone, and a 304 makes it cheap.
  The other way: keep it across the sign-out.
- **The ETag** is the version and the file's name, and Go's version,
  through a shared `etagChars` helper.
  The other way: a hash of the body.
- **The bytemare-ksf copy** that `go.mod` replaces the module with is the module's entry,
  with its path, not a second entry.
- **An entry's texts are drawn only while its fold is open.**
- **The focus effect** carries a `biome-ignore` with its reason:
  the hooks rule takes values from signals as constant.
- **`make credits`** depends on `make ui`, since `go list` fails on the missing
  `parts.js`, and runs with `GOOS= GOARCH= GOARM=`;
  two runs give the same bytes.

## What the binary includes, and what not

- **Not named by the plan's Analysis:**
  `go-isatty` and `go-strftime` (darwin and windows only),
  `modernc.org/mathutil` and `memory`, `filippo.io/nistec`,
  bytemare's `ecc`, `hash`, `hash2curve` and `secp256k1`, `bigfft`,
  `golang.org/x/term` and `x/text`.
  The standard library's own copies of `golang.org/x/crypto`, `net`, `sys`
  and `text` fall under Go's licence.
- **Not in the binary:**
  `app/`, `tools/`, `docs/`.
- **The binary grows** by 192,512 bytes (linux/amd64, `-s -w`)
  in the builder's measure of the first build,
  207 KB (`-trimpath`) in the reader's; `credits.json` is about 187 KB.

## What the reading found

One builder built it in an exported copy;
two readers read it, one the page, its tests and the documents,
the other the Go side.
Two things had to change:

- **The classifier gave wrong names:**
  the full GPL-3 and MPL-2.0 read as AGPL-3.0, LGPL-3 as GPL-3.0,
  0BSD as ISC, BSD-4 as BSD-3-Clause.
  No entry of the day was misnamed, but the lists of later blocks go through it.
  It became the rule above, tested on every licence file of the list,
  on 14 Debian texts and on misleading ones.
- **After a Try again that failed again, the focus was on the page's body:**
  the button is disabled while the fetch runs,
  and the browser takes the focus off it.
  The focus goes back to Try again in the layout effect,
  and the test asserts it.

The rest, done in the first rework:

- the sign-out test did not hold the session's guard,
  since the page had left System info; the next user is now on System info
  with a fetch of its own held, and either guard taken out fails it;
- English from the data stood inside a translated sentence;
  the data hold names and identifiers, every sentence is the page's;
- the OPAQUE client's entry and the command say that the crates' own texts are
  not reproduced;
- Go's version became the binary's;
- an entry with several licence files was named by its first; now each file;
- a Makefile line of another form was left out in silence; now it fails;
- a licence file had no size limit; now 256 KiB;
- the documents said "without network"; they now say that `go list` downloads
  a missing module where `GOPROXY` allows, and name `go mod download`;
- the note-table convention was incomplete, and a row naming a file failed with
  a bare error; both settled;
- the command printed nothing of what an entry includes; now it does;
- the UI test pinned Preact's version; it reads it from `credits.json`;
- `ui/vendor/README.md` repeated its table in a paragraph; one is kept;
- a licence box had no accessible name; it is named by its file and licence;
- at 390 px the version and licence wrapped under the chevron;
  now under the name.

And in the second:

- a title matched anywhere in the first two lines,
  and X11 read as MIT; a title now opens its line,
  and X11's clause keeps a text from being MIT;
- the README said a file with two licences is named by none,
  which was not true of two titles; now it is;
- Rust's own `core`, `alloc` and `std` are named in the note;
- the ETag holds Go's version;
- "Go go1.26.8" reads "Go 1.26.8";
- "BSD-3-Clause" broke at its hyphen at 390 px; the identifier no longer does.

## The crates' licence texts

The WebAssembly inside `opaque.js` holds Rust crates,
under MIT or Apache-2.0 and BSD-3-Clause,
whose notices are to be passed on with a binary.
Their texts are not in the repository, and none is added.

- **What was found:**
  the npm tarball's file is the vendored file, byte for byte,
  and the npm provenance names the package's repository and a commit on its main,
  its signature not checked cryptographically.
  The source at the commit its npm provenance names has no `Cargo.lock`
  (its `.gitignore` excludes it),
  and its release builds without `--locked`:
  the crates' versions were resolved on the build's runner,
  and no committed file fixes them.
  The research stopped there, by its own condition.
- **What the entry now says:**
  the 19 crates and versions the WebAssembly itself names,
  in the paths Rust leaves in it, with `dlmalloc` and `hashbrown`,
  most likely of Rust's standard library, and Rust's own `core`,
  `alloc` and `std`; that their licence texts are not reproduced here.
  Crates that leave no path are not named.
  `TestVendoredOpaqueCrates` holds the names to the file, in both directions.
- **D56, for the owner:**
  - **A (recommended).**
    Collect the texts as "consistent with the artifact":
    the artifact's versions as pins,
    the rest of the closure resolved from the source's manifest,
    as a file `opaque.LICENSE-3RD-PARTY` beside the vendored file,
    which the generator takes up by its name.
  - **B.**
    Leave the plain sentence.
  - **C.**
    Ask the package's authors for a lock file or a notices file.

## Seen and left

- **`gopkg.in/yaml.v3`'s head** no longer tells that it is MIT and Apache-2.0;
  the owner's.
- **The crates' notices** are not passed on; D56, the owner's.
- **`/ui/credits.json` makes the modules' versions public,**
  as any page file is; the version was in the ETag before; the owner's.
- **`TestProxyDiscoverySerialInSystem`** failed once in the builder's whole
  `go test ./...` (`internal/server`, a port the test did not get),
  and passed 3 of 3 alone;
  nothing of this block can touch it; not this block's.
- **Firefox and Safari:**
  the focus was seen in Chromium alone; CI's suite.
- **Windows and macOS:**
  no run on either; CI's runners.
- **The armv6 test in Docker** with `GOPROXY=off`:
  its module cache was not checked for the modules only darwin or windows link;
  `TestCreditsOnAFreshBox` skips under `-short`; the session that runs it next.

## For the later blocks

- **Block 4 adds `third_party/eff-wordlist/`:**
  for a named entry with a version and a licence,
  its `MASSIMO.md` needs the table of `tools/credits/README.md`,
  its Licence column one SPDX identifier, before a comma,
  that agrees with the licence its text is known by.
  `make credits` must run, and the credits' tests fail until it has.
- **A licence text over 256 KiB stops the generator.**

## For the completion gate

- **What a user notices:**
  System info lists the credits and licences of everything the server includes,
  each with its texts, and `massimo credits` prints them.
- **D56:**
  the crates' licence texts, A, B or C.
- **`yaml.v3` reads "licence in its text",**
  and sqlite, libc and memory read as mixed.
- **The binary grows** by about 190 to 210 KB.

## What verified it

- **The builder, on the first trees:**
  `make ui`, `gofmt`, `go vet` for linux, windows and darwin, `make cross`,
  `go run ./tools/credits -check`, `make ui-check`,
  `go run ./tools/apitypes -check`, each exit 0;
  the tests of `internal/credits`, `tools/credits`,
  `internal/server` and `cmd/massimo`, exit 0;
  `test_system_info.py` and `test_parts_guard.py` in all four cases, 106 passed.
- **Seen failing first:**
  the credits' tests on a stale list and on a made-up `third_party/` directory;
  `TestUIServed` with a 404 before the route; the UI tests before the fold,
  and the late-answer tests with the guards removed;
  the command's tests before its case.
- **The builder, after the first rework:**
  the same checks, `make credits` twice with the same bytes, each exit 0;
  the whole `go test ./...`, 36 packages ok and the one failure above;
  the two UI files in all four cases, 106 passed;
  each guard and the focus line taken out failing the tests.
- **The builder, after the second rework:**
  the same checks, the tests of the four packages, each exit 0;
  the two UI files in all four cases, 106 passed;
  each new test seen failing first.
- **The reader of the page:**
  `make ui` and `make ui-check`, exit 0;
  the credits' UI tests in all four cases, 16 passed; after the rework,
  the two UI files and its probes, 122 passed,
  and each guard taken out failing the sign-out test.
- **The reader of the Go side:**
  its own `go list -deps` of the seven platforms against the list;
  the whole `go test ./...` with `GOPROXY=off`, `gofmt`,
  `go vet` for three platforms, exit 0; linux/armv6 built,
  the windows test binary compiled; after the rework,
  the tests of the four packages, `-check`,
  `make credits` twice with the same bytes,
  and the classifier on the full Debian texts and on misleading ones,
  all as the rule says.
- **The coordinating session:**
  both messages by `commit-msg.sh`, 0 errors; the strict changelog check,
  passed for each; `GUIDELINES/shared/tools/check`, twelve checks well;
  on the committed tree, `make ui`, `go run ./tools/credits -check` (31 entries,
  current), `go run ./tools/apitypes -check` and `go vet` for linux, exit 0,
  `gofmt -l` listing no file,
  and the tests of `tools/credits`, `internal/credits`, `cmd/massimo`,
  `internal/server` and `tools/identifiers`, exit 0.
  The whole Go suite and the UI suite on the committed tree are the pull
  request's CI.
