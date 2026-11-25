# opaquecrates

`go run ./tools/opaquecrates` writes `internal/server/ui/vendor/opaque.LICENSE-3RD-PARTY`:
the licence texts of the Rust crates in the WebAssembly of the vendored OPAQUE client,
`opaque.js` (`internal/server/ui/vendor/README.md`).
System info's credits take the file up beside `opaque.LICENSE`, by its name
(`make credits`, `tools/credits/README.md`).
It is no part of the build:
it is run by hand, online, from the repository's top,
and git keeps the file it writes.

```sh
go run ./tools/opaquecrates                  # fetches the archives, writes the file
go run ./tools/opaquecrates -check           # exits 1 where the file is not what the archives give
go run ./tools/opaquecrates -crates <dir>    # the archives from <dir>/<name>-<version>.crate instead
make credits                                 # then the credits take the file up
```

## The file

Its head is written by hand:
what the file holds, how its crates and their versions were had,
and the list, a line a crate:
its name, its version and the SHA-256 of its archive on crates.io, in hex
(the `cksum` of its version's line in crates.io's index).
Every other line of the head is prose;
a line that holds a SHA-256 and is no line of the list,
and a CR, stop the program.
The program keeps the head as it is and writes everything after it again:

1. each crate's archive,
   from `https://static.crates.io/crates/<name>/<name>-<version>.crate`,
   with Go's own HTTP client,
   which sends no credential and nothing of the repository;
   an archive whose SHA-256 is not the list's is refused;
2. its `Cargo.toml`, which must name the listed crate and version,
   and the licence it declares (`license`), given once in `[package]`;
   the lines inside a string of several lines (a description) are passed over;
3. the files at the archive's root named `LICENSE*`, `LICENCE*`, `COPYING*`,
   `NOTICE*` or `UNLICENSE*`, in any case and by name,
   and last the one the `Cargo.toml` names as `license-file`;
   CRLF as LF, each in UTF-8;
4. in the list's order,
   a line `==> <name> <version> (<the licence it declares>) <==`,
   then each file, a line `--- <file>` and its text,
   or `--- <file>: the same text as <crate> <version> <file> above`
   where a text above is the same byte for byte
   (texts that differ in their spaces are each written);
   a crate without a licence file says so, with the licence it declares.

A licence file that is a link,
or whose name holds anything but letters, digits, `.`, `_`, `-` and `/`,
an entry outside the crate's directory,
and a text that is not UTF-8,
that holds a control character other than a tab and a newline,
or a line that reads as one of the file's own,
stop the program, which names the crate and leaves the file as it was.
`tools/credits` takes a licence file of at most 256 KiB:
a larger file stops `make credits`, which names it.

`go test ./tools/opaquecrates` checks the program
on archives it makes up in a temporary directory;
no test reaches the network.
`TestVendoredOpaqueCrates` (`internal/server`) holds the committed file offline
to the crates the WebAssembly names in the paths of their sources
(`<crate>-<version>/src/`), each in the list at that version,
and to its list:
a crate's line for every crate of the list, in its order;
a file, or the line that its archive has none, under every crate;
a text named as one above only where that one stands above;
the head naming the commit of the package's sources,
the time npm published it and the Rust compiler's commit the WebAssembly names;
and the whole file to its SHA-256,
so that a change of it is one on purpose:
the program writes it again, `-check` confirms it online,
and the SHA-256 in the test changes with it.

## How the list was had

The package publishes no lock file, and its build's lock is not published,
so the list is a reconstruction, which the file's head says.
For `@serenity-kit/opaque` 1.1.0, on 2026-11-25:

- npm's provenance of the package names the commit of its sources,
  `ca1cb22f03b9e456159ce367e2975124f87f5ad8` of `serenity-kit/opaque`,
  and npm published it at 2026-02-01 14:40:35 UTC;
  the `Cargo.toml` there has edition 2018 and no `resolver` key (Cargo's resolver 1),
  no default features and no lock file beside it.
- The WebAssembly names 21 crates in the paths of their sources,
  at their versions,
  as the Includes of `internal/server/ui/vendor/README.md` lists them:
  19 of the package's crate graph,
  and `dlmalloc` and `hashbrown`, which come with Rust's standard library.
- Every other crate is at the newest version of its caret series on crates.io
  published before that time, neither yanked nor a pre-release,
  as crates.io's index has them (`pubtime`, `yanked`);
  18 of the 19 are that themselves,
  and the manifest pins the 19th, `generic-array`, exactly.
- The closure for `wasm32-unknown-unknown`:
  the normal dependencies whose `target` holds there,
  with resolver 1's features,
  without dev dependencies, build dependencies,
  the procedural macros (`proc-macro = true`) and what only they reach.
  Cargo 1.75 cannot read an edition-2024 manifest of it (`base64ct` 1.8.3),
  so it was walked outside Cargo, from the index's entries:
  51 crates, the 19 and 32 more, then `dlmalloc` and `hashbrown`.
- Each SHA-256 is the `cksum` of its version's line in the index,
  which each archive's own SHA-256 matched.

At an update of `opaque.js`, the list is had again by the same rule.
A Cargo that reads every manifest of the closure could walk it, untried:
in a copy of the package's `Cargo.toml` at the commit npm's provenance names,
each crate held at its version by an exact requirement (`=<version>`),
the WebAssembly's versions,
and every other crate's newest of its series at the publication,
as the lock file of this list was made (Cargo 1.75 could make it, not walk it);
then `cargo tree --target wasm32-unknown-unknown -e normal,no-proc-macro`,
which leaves out the procedural macros and what only they reach (`-e normal` keeps them).
Then the head is written again, the program run, and `make credits`.
