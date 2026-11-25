# credits

`make credits` writes `internal/credits/credits.json`:
the credits and licences of everything the `massimo` binary includes,
which the binary embeds (`internal/credits`)
and System info shows under **Credits and licences** (`docs/ui.md`, "System info").
git keeps the file;
a change to a module or to a vendored file, or to a vendored file's note, writes it again.

```sh
make credits                      # go list needs the web UI's bundle, which make ui builds first
go run ./tools/credits -check     # exits 1 where the file is not current
```

`go test ./...` fails too while the file is not current:
`TestGeneratedFileCurrent` here compares it byte for byte, licence texts included,
and `TestEverythingIncludedHasAnEntry` (`internal/credits`) names what has no entry.

## What it lists

In this order, each with every licence file it has
(`LICENSE*`, `LICENCE*`, `COPYING*`, and the notices `NOTICE*` and `PATENTS`):

| Entry | Name and version | Licence files from |
|-------|------------------|--------------------|
| Massimo itself | `Massimo`, AGPL-3.0-or-later | the repository's `LICENSE` |
| Go's runtime and standard library | `Go`; no version in the file: the binary shows its own (`runtime.Version()`) | `$(go env GOROOT)`; where a distribution keeps them elsewhere (Debian), `golang.org/x/sys`, whose `LICENSE` and `PATENTS` are Go's |
| each Go module the binary links | its path and version | its directory in the module cache, or the repository's copy that `go.mod` replaces it with |
| each directory of `third_party/` | a row of its note's table, else the directory's name | the directory, or the row's |
| each file of `internal/server/ui/vendor/` | a row of its `README.md`'s table, else the file's name | the files beside it named after it (`opaque.LICENSE`) |

The modules are those `go list -deps ./cmd/massimo` finds,
without cgo, for each platform the Makefile's `cross` builds,
their union by path (`TestTargetsAreTheMakefiles` holds the two to each other,
and fails on a line with `$(CROSS)` it does not read).
go list reads the modules from the module cache,
and downloads one missing there where `GOPROXY` allows;
with `GOPROXY=off` a module that only darwin or windows links,
missing from a cache that a linux build filled, fails the program and the tests:
`go mod download` fills the cache with every module of `go.mod`.
A directory of `third_party/` that replaces a module is the module's entry.

## A vendored directory's note

A directory of `third_party/` has a note, its `MASSIMO.md`, else its `README.md`;
a directory without one stops the program.
The note's first table whose head has a **Version** column names what the directory holds,
a row an entry; without such a table the directory is one entry,
named by the directory, without a version and with the licence its text is known by.
A new directory needs no change to the program, only `make credits`.
The columns read, by their heads, in any order:

| Column | What it holds |
|--------|---------------|
| the first | the entry's path in backticks, relative to the directory: a directory (`` `preact/` ``, or `` `.` `` for the directory itself) that holds its own licence file; a file, or a path outside the directory, stops the program |
| **Package** | the entry's name, the first word in backticks; without it the path's last element |
| **Version** | its version |
| **Licence** (or **License**) | its licence's SPDX identifier, before a comma (`` MIT, `LICENSE` ``) |
| **Includes** | what else it holds, as names and versions alone |

Other columns are for the reader and not read.
The table in the UI's `vendor/README.md` is read the same way,
its first column a file of that directory,
whose licence files lie beside it named after it (`opaque.LICENSE`).

## The licences

Each licence file is named by the SPDX identifier of its text
where exactly one of a few known licences matches it, else by none:
the GNU licences version 3 (AGPL-3.0, GPL-3.0, LGPL-3.0), MPL-2.0 and
the CC's (CC-BY-4.0, CC-BY-3.0-US) by their title alone,
which opens a line, its version in that line or the next;
Apache-2.0 by its title, by sentences of its text or by its notice;
MIT, ISC, BSD-2-Clause and BSD-3-Clause by sentences only each has
(X11's clause against advertising keeps a text from being MIT).
A file in which a second known title opens a line further down,
or a second licence's sentences stand, is named by none,
and so is one in which none matches: the reader reads the text.
A notice is named by none.
An entry's licence is its note's Licence column, where it has one,
which must be the licence its first licence file is known by,
or the program stops naming both; else that file's.
An entry whose licence files are not all under its licence is marked mixed,
and System info says so.

An entry without a licence file stops the program with an error naming it and where it looked,
and the file is left as it was; it is never written as unknown.
A licence file above 256 KiB stops it too, naming the file:
that is three times the largest of today (modernc.org/sqlite's `LICENSE-3RD-PARTY.md`),
and the whole list goes to a phone that opens the fold.

The file is the same on every machine:
no absolute path, no time, no Go version, the order above, line ends as LF
(`.gitattributes` keeps them on a Windows checkout).
