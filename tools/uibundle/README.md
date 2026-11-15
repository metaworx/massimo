# uibundle

`make ui` bundles the web UI's parts:
`internal/server/ui/src/index.ts` and what it imports,
TypeScript and TSX, into `internal/server/ui/parts.js`,
one classic script that the page loads after its plain scripts
and the server embeds (`docs/ui.md`, "How the page is built").
git does not keep the bundle;
every Makefile target that compiles Go makes it first.

The program is a few lines over esbuild's Go API (`github.com/evanw/esbuild/pkg/api`),
pinned by this directory's `go.mod`, a module of its own,
so that esbuild stays out of massimo's `go.mod`.
The target runs it from the repository root,
the same on Linux, macOS and Windows:

```sh
go -C tools/uibundle run .
```

It finds the repository's top as the nearest directory above it
that holds the entry point.
What it makes:

- an IIFE, minified, for ES2022, as a classic script;
- TSX with the automatic runtime of `preact/jsx-runtime`;
- `preact`, `preact/hooks`, `preact/jsx-runtime`, `@preact/signals` and
  `@preact/signals-core` from `third_party/preact/` (its `README.md`),
  each with a licence comment naming its package's `LICENSE`,
  which esbuild keeps at the bundle's end;
  any other package is an error, and nothing comes from a `node_modules`;
- Massimo's licence line and where the file comes from, as its banner.

The same sources give the same bytes,
and a bundle that would not change is left as it is.
An error or a warning of esbuild's is printed with its file and line,
the bundle left as it was, and the exit status is 1.

Dependabot proposes a new release of esbuild (`.github/dependabot.yml`);
by hand, in this directory:

```sh
go get github.com/evanw/esbuild@vX.Y.Z
go mod tidy
```

then `make ui`, `make ui-check` and `make ui-test`.
