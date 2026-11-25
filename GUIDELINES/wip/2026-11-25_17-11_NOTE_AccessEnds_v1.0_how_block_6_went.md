# NOTE AccessEnds v1.0: how block 6 went

Where Block 6, Crates, went differently from the plan,
the choices taken in the run, what the reading changed, and what verified it.

## What it made

System info's credits and `massimo credits` now carry the licence texts of the
Rust crates in the WebAssembly of the vendored OPAQUE client
(PasswordRules.D56-A, D30-A to D34-A),
where they said the texts were not reproduced.

- **`internal/server/ui/vendor/opaque.LICENSE-3RD-PARTY`** (new):
  a head written by hand, what the file holds and how its 53 crates were had,
  a reconstruction consistent with the artifact and not the build's lock,
  which is not published;
  the list, each crate by its name, its version and the SHA-256 of its archive;
  then each crate's line with the licence its `Cargo.toml` declares,
  103 file lines in all,
  51 texts written and 52 named as the same text as one above (D34-A).
  `gloo-utils` 0.2.0 says "No licence file in its archive;
  declared MIT OR Apache-2.0" (D32-A).
  Rust's `core`, `alloc` and `std` are named by the compiler's commit,
  their texts not here (D31-A).
  It is not in `go:embed` and not served under `/ui/vendor/` (D33-A).
- **`tools/opaquecrates/`** (new, Go, with a README):
  run by hand and online, it fetches each archive from `static.crates.io`,
  checks its SHA-256 before it reads anything of it,
  and writes everything after the head again;
  `-check` says whether the committed file is what the archives give,
  `-crates <dir>` reads the archives from a directory.
  No Rust is needed (D30-A).
- **The credits** take the file up by its name,
  so the OPAQUE client's entry reads as mixed.
- **What a user sees:**
  in System info the entry's head reads "1.1.0 MIT,
  other licences among its files"; its body "Includes argon2 0.5.3, …,
  wasm-bindgen 0.2.108 and the crates they are built with; their licence texts,
  collected for Massimo from the crates' archives, are among its files.";
  its two files "opaque.LICENSE · MIT" and "opaque.LICENSE-3RD-PARTY · licence
  in its text",
  the second's box holding the 53 crates' texts and scrolling inside itself.
  `massimo credits` lists it as "MIT and others",
  and `massimo credits @serenity-kit/opaque` prints the Includes line,
  then `opaque.LICENSE (MIT)`, then `opaque.LICENSE-3RD-PARTY (see the text)`.
- **Sizes:**
  the file 186,247 bytes, its texts about the Analysis's 179,538 and its head
  6,751 bytes more;
  `credits.json` from 208,601 to 399,188 bytes;
  the linux/amd64 binary 188,416 bytes larger.
- **One commit:**
  "fix(server): The credits hold the licence texts of the OPAQUE client's
  crates".

## Different from the plan

Against the plan's block, item by item:

1. **The tool** is the plan's,
   and refuses more than an archive whose checksum differs: a link,
   an entry outside the crate's directory, a text that is not UTF-8,
   that holds a control character other than a tab and a newline or a line that
   reads as one of the file's own, a `Cargo.toml` that is missing,
   names another crate or version, or gives `license` twice,
   and a licence file's name with anything but letters, digits, `.`, `_`,
   `-` and `/`; in the head, a CR,
   and a line that holds a SHA-256 but is no line of the list.
   The lines inside a string of several lines in a `Cargo.toml` are passed over.
   It caps an archive at 64 MiB,
   its unpacked files at 512 MiB and a file at 1 MiB.
   Its tests make up archives in a temporary directory,
   where the plan named a made-up index.
2. **The list** is in the file's head, one of the plan's two places (Choices,
   below): the 51 crates of the closure and `dlmalloc` and `hashbrown`.
3. **The head** says what the plan's item lists,
   in two places in words the reading changed:
   the 19 crates are "of the package's dependency graph",
   and which crates `core`, `alloc` and `std` use "was not looked up"
   (the lock file of Rust's library at that commit names them),
   the WebAssembly naming none of them but `dlmalloc` and `hashbrown`,
   where D31-A says their dependencies are unknown.
   It also names the Rust compiler's commit the WebAssembly names,
   which the test holds.
4. **`ui/vendor/README.md`** as the plan has it,
   and it says what the test holds.
5. **The Includes text is not the plan's.**
   The plan's "Includes {what}; their licence texts are among its files."
   reads as the whole list, where Includes names the 21 crates the WebAssembly
   names and the file credits 53.
   The coordinating session chose the words, after the reading:
   "Includes {what} and the crates they are built with;
   their licence texts, collected for Massimo from the crates' archives,
   are among its files.",
   in German "Enthält {what} und die Crates, mit denen diese gebaut sind;
   deren Lizenztexte, für Massimo aus den Archiven der Crates gesammelt,
   stehen unter seinen Dateien.";
   FR, IT, NL and PL machine-translated and marked,
   the Dutch "onder de bestanden" as its own `credits.mixed` has it.
   `includesLine`, `ui/vendor/README.md`,
   `docs/ui.md` and `README.md` say it too;
   the comment of `Entry.Includes` as the plan has it.
   The credits' intro stays (For the completion gate, 2).
6. **`make credits`** as the plan has it,
   but `tools/credits/generate.go` changed,
   where the Analysis read that no change was needed:
   the cap's reason says what is so now,
   and an entry with Includes and no licence file beside its first stops
   `make credits`, naming it.
7. **Tests beyond the plan's:**
   `TestVendoredOpaqueCrates` also holds the head to the compiler's commit and
   the whole file to its SHA-256, so that a change of it is one on purpose;
   `TestUIServed` answers 404 for the file under `/ui/vendor/` (D33-A);
   `TestClassifyToday` also fails on a file it names that an entry lacks,
   without which it could not fail on main;
   `TestIncludesNeedTheirTexts` (new, `tools/credits`);
   `cmd/massimo/credits_test.go`, the list's "MIT and others" and the OPAQUE
   client's own licence printed before its crates';
   `test_system_info.py` also holds the crates' box to scroll inside itself
   (`overflow-y: auto`) and to wrap its lines.
8. **Documents beyond the plan's:**
   `README.md` (`massimo credits`), the paragraph of `docs/ui.md` on the
   vendored file besides "System info",
   `tools/opaquecrates/README.md` (the file's form, how the list was had,
   an untried way to have it again),
   and the docstring alone of `src/sysinfo/Credits.tsx`:
   each said the texts were not reproduced.
9. **The CHANGELOG fragment** in the plan's words.
10. **The verification:**
    `go test -count=1` of the six packages the block touched and those that
    import them, not `./...`, which is this pull request's CI;
    `make cross` on the first tree only;
    `GUIDELINES/shared/tools/check` could not run one of its twelve checks in
    the exported copy, which has no `GUIDELINES/shared/`,
    and the coordinating session ran it on the commit.

## Choices the plan left open

- **The list in the file's head:**
  the tool keeps the head as it is and writes everything after it again.
  The other way: `opaque.crates.txt` beside it, which `tools/credits` would take
  for a credits entry without a licence, stopping `make credits`,
  unless it were taught to pass the file over.
- **The declared licence from each archive's `Cargo.toml`:**
  the same values as crates.io's API for all 53.
  The other way: the API, a second source to fetch and trust.
- **The order:**
  the 51 alphabetically, then `dlmalloc` and `hashbrown`,
  as the investigation had them.
  The other way: all 53 in one alphabetical run.
- **`-check` and `-crates <dir>`:**
  `-check` confirms the committed file online and writes nothing.
  The other way: writing alone, a diff the only check.
- **A string of several lines in a `Cargo.toml`:**
  its lines passed over, and a second `license` refused.
  The other way: every such string refused.
- **A licence file's name:**
  letters, digits, `.`, `_`, `-` and `/` alone.
  The other way: a name refused only for a newline or a space.
- **Includes held in `tools/credits`:**
  `make credits` stops on an entry with Includes and no licence file beside its
  first.
  The other way: a check in `TestEmbeddedList`,
  which lets `make credits` write the claim until the test runs.

## What the reading found

One builder built the block in an exported copy of main with the plan.
Two readers read the recorded tree:
one the tool, the file's head and the credits' claims,
the other the tests, the texts and the look.

Confirmed: the WebAssembly, decoded again, names exactly 21 crates,
the 19 and `dlmalloc` 0.2.11 and `hashbrown` 0.16.1 under Rust's own
dependencies, and one compiler commit;
the 53 SHA-256 values equal both the archives' and crates.io's index's;
none is yanked, and all were published before npm published the package,
the latest `zmij` 1.0.18 on 2026-01-30;
each of the 32 resolved crates is the newest of its caret series then,
and of the 19 only `generic-array` is not, which the manifest pins exactly;
the manifest at the attested commit has edition 2018, no resolver key,
no default features and no lock file;
the first reader's own walk with resolver 1 over the index's entries gives the
same 51; all 53 declared licences equal crates.io's API;
the tool reads an archive only after its checksum,
and writes one file at a fixed path once everything is made;
its `-check` offline over the investigation's archives says the file is current;
the box scrolls and is reached by the keyboard with a visible focus outline,
in light and dark; and no "not reproduced" is left outside `GUIDELINES/wip/`.

One must, done:
**the cap's reason was false.**
`tools/credits` gave its 256 KiB as three times the largest licence file of
today, modernc.org/sqlite's of 74 KB, the whole list staying small.
The crates' file is now the largest, 186 KB, 71 % of the cap,
and the list 399 KB.
The code's comment and the README say so now,
and that a later client with more crates can pass the cap,
which then stops `make credits`, naming the file.

Eleven shoulds, done in one rework:

- **The file's SHA-256** pinned in `TestVendoredOpaqueCrates`:
  a word changed in a licence text,
  or `zmij` dropped from the list and the body, passed before.
- **The head's words**
  (Different from the plan, 3).
- **Includes and the intro**
  (Different from the plan, 5), the second reader's Dutch among them.
- **Archive data the tool took on trust:**
  a `license` key inside a description of several lines set the declared
  licence; a licence file's name with a newline wrote a crate's line of its own;
  control characters reached the terminal through `massimo credits`;
  a list line with a CR became prose and its crate dropped out.
  Each is refused now, with a case;
  none of the 53 archives does any of it.
- **The root-only rule had no test that could fail:**
  `src/LICENSE` fails the name's match by its upper-casing alone.
  A made-up `LICENSES/Apache.txt` and a directory `LICENSE-texts/` now must not
  appear in the file.
- **D33-A** held by `TestUIServed`.
- **Includes must not claim what is not there:**
  the rule of `tools/credits` above.
- **The crates' box's assertion** checks `overflow-y` as the Massimo box's does.
- **The `cargo tree` recipe** in the tool's README:
  `-e normal,no-proc-macro`, since `-e normal` keeps the procedural macros;
  each crate held by an exact requirement in a copy of the manifest;
  marked untried.
- **`ui/vendor/README.md`'s sentence** on what the test holds.
- **The commit message:**
  `Credits.tsx`'s docstring alone; the `license-file` named.

Planted faults:
the second reader's 14 changes of the file were caught for a crate the
WebAssembly names dropped, a crate's body missing,
a fold naming a fold or the wrong file, a crate without a file line,
and the time or the source commit gone from the head;
its changes of a text, a licence and a SHA-256 passed,
which the pin now catches.
Its 15 changes of the tool were each caught but the root-only rule's,
which is tested now.
The first reader's probes of the tool are the refusals above.

There was no second reading.
The coordinating session read the rework's diff of `tools/credits`
(`generate.go`, the README) and the six Includes texts,
and found nothing to change.

## Seen and left

- **No test reads the texts against the archives:**
  no test reaches the network (D30-A);
  `go run ./tools/opaquecrates -check`, by hand and online, does,
  and the SHA-256's pin makes any change of the file one on purpose.
  The next update of `opaque.js`.
- **No test of `fromCratesIO`** (the address, a status other than 200,
  the 64 MiB cap), of the line for a crate that declares no licence,
  or of the size caps.
  The tool's next change.
- **The 32 resolved crates are a reconstruction:**
  the builder rechecked the method's facts against the index of today,
  not the closure, which the first reader's own walk confirmed;
  the procedural macros' flags were taken from the investigation,
  not from their archives;
  the `cargo tree` recipe has not been run.
  The next update of `opaque.js`.
- **The `COPYRIGHT` files** of `group`, `rand`, `rand_chacha` and `rand_core`
  are left out by the plan's rule of names;
  they are notices, and the head states the rule exactly.
  The owner's, should the rule take them.
- **The line-break warnings** of the documents the block touched,
  older than it, stay as they were.

## For the completion gate

1. **What a user notices:**
   in System info the OPAQUE client reads "MIT, other licences among its files",
   with its crates' texts in a second box that scrolls;
   `massimo credits` lists it as "MIT and others" and prints its crates' texts
   after its own licence.
2. **The Includes line's words** are the coordinating session's,
   the owner's to change (Different from the plan, 5).
   The other way: the plan's shorter "Includes {what};
   their licence texts are among its files.".
   The credits' intro, unchanged, still says that the texts are as each came
   with its files, where this file's head is the project's own and 52 of its
   103 file lines name a text above.
3. **`credits.includes` is worded for the one entry with Includes today,**
   the OPAQUE client ("the crates they are built with"):
   a later entry with Includes,
   for which `make credits` now asks for a licence file beside its own,
   would need words of its own.
4. **D40-A, the owner's answer while this block was read:**
   the credits take each Go module's version from the binary's build
   information, Block 8 of the plan's revision v1.1, built on this block.
   Until then every bump in `go.mod` fails `TestGeneratedFileCurrent` of
   `tools/credits` until `make credits` runs,
   as a Dependabot pull request showed.
5. **Growth, measured:**
   `credits.json` from 208,601 to 399,188 bytes (the Analysis said about 391 KB),
   fetched as System info's fold first opens;
   the linux/amd64 binary 188,416 bytes larger (about 182 KB);
   the crates' file 186,247 bytes, 71 % of the 256 KiB cap.
   A later `opaque.js` with more crates can pass the cap:
   `make credits` then stops, naming the file.
6. **What the file can claim:**
   its crates and versions are a reconstruction, not the build's lock,
   and its head says so (D30-A);
   `gloo-utils` 0.2.0 has no licence file, its declared licence said (D32-A);
   Rust's `core`, `alloc` and `std` are named by the compiler's commit,
   their texts in Rust's repository,
   and which crates they use was not looked up (D31-A).
7. **FR, IT, NL and PL** of the Includes line are machine-translated and marked.

## What verified it

- **The builder, on the first tree,** each by its exit code, 0:
  `make ui`, `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin, `make cross`,
  `make ui-check` (npm offline), `go run ./tools/apitypes -check`,
  `make credits` twice with the same bytes and `go run ./tools/credits -check`,
  `reflow.sh --check` on each Markdown file changed,
  and the strict changelog check on a scratch index;
  `go test -count=1` of `tools/opaquecrates`, `tools/credits`,
  `internal/credits`, `internal/server`, `internal/serve` and `cmd/massimo`;
  `GUIDELINES/shared/tools/check`, eleven of twelve checks well,
  the twelfth unable to run in the copy.
  `test_system_info.py` and `test_parts_guard.py` in all four cases,
  106 passed; the whole UI suite in 1100-en, 814 passed.
- **The downloads:**
  the tool fetched the 53 archives from `static.crates.io` with Go's own client,
  without credentials, once to write and once with `-check`;
  what it wrote after the head is byte for byte the investigation's,
  but the `gloo-utils` line in D32-A's words;
  the 53 entries of crates.io's index, fetched again,
  carry the investigation's checksums.
- **Seen failing first,** on a copy of the base:
  `TestVendoredOpaqueCrates`, `TestClassifyToday`,
  `TestCreditsList` and `TestCreditsOne`,
  and `test_credits_fetched_once` in all four cases,
  each for the reason it names;
  four broken copies of the file (the `gloo-utils` line gone,
  a fold naming a text below, a wrong version, a wrong compiler commit),
  each caught; the tool's tests with the folding, the line for a crate without
  a licence file and the checksum's check each taken out.
- **The first reader:**
  the WebAssembly decoded, the checksums against the archives and the index,
  a walk of its own with resolver 1,
  the declared licences against crates.io's API,
  the tool's `-check` offline over the investigation's archives,
  and probes of the tool in its export;
  `go test -count=1` of `tools/opaquecrates`, `tools/credits`,
  `internal/credits`, `internal/server`, `cmd/massimo`,
  `internal/serve` and `tools/identifiers`, ok; `gofmt` clean,
  `go vet` of the tool for linux, windows and darwin,
  `go run ./tools/credits -check`,
  `commit-msg.sh` and `reflow.sh --check` on the six Markdown files,
  each exit 0; linux/amd64 builds of its own, 188,416 bytes larger.
  No UI test, no network.
- **The second reader:**
  the new tests pass on the tree and, on the base with them,
  each fails for the reason it names,
  `test_credits_fetched_once` at 1100-de on the Includes text;
  14 changes of the file and 15 of the tool;
  `test_system_info.py`, `test_parts_guard.py` and a scratch test of the look
  in all four cases, 110 passed;
  the box measured, 16em tall with `overflow-y: auto`,
  its lines wrapped, its scroll height 127,800 px at 390 and 67,115 px at 1100,
  no horizontal scroll at either width,
  opened with Enter, reached by Tab after `opaque.LICENSE`,
  scrolled by Page Down and End, readable in light and dark.
- **The builder, after the rework,** each by its exit code, 0:
  `make ui`, `make ui-check` (npm offline), `gofmt -l .` printing nothing,
  `go vet` for linux, windows and darwin;
  `go test -count=1` of the six packages;
  `go run ./tools/credits -check`, `go run ./tools/apitypes -check`
  and the strict changelog check on a scratch index;
  the tool's `-check` online once, current with 53 crates;
  `reflow.sh --check` on the five changed Markdown files, 0 errors.
  Each new or changed test seen failing without its change:
  the pin on a copy with a word changed and on one without `zmij`,
  `TestFileWritten` without the root's check,
  `TestUIServed` with the file in `go:embed` (200, want 404),
  `TestIncludesNeedTheirTexts` without its rule,
  each refusal without it, and the box's assertion with its overflow visible.
  The Includes line at 390 px in all six languages,
  by the page's own setting on Automatic and the browser's language,
  since the second reader's call of `setLanguage` did not redraw the part:
  11 or 12 lines, inside its box, no horizontal scroll.
  `test_system_info.py` and `test_parts_guard.py` in all four cases,
  106 passed; the whole UI suite in 1100-en, 814 passed.
- **The coordinating session:**
  the rework's diff of `tools/credits` and the six Includes texts, read;
  the step carried from the plan's commit, "chore(aps): +AccessEnds v1.0",
  onto main's head, "chore(aps): +ToldData v1.0, +ANALYSIS ToldData",
  by a three-way merge of trees without conflict;
  in a clone with the commit:
  `commit-msg.sh`, 0 errors and 0 warnings;
  `changelog.sh check --staged --strict`, 0;
  `GUIDELINES/shared/tools/check` over the range, twelve checks all well
  (line breaks 0 errors).
