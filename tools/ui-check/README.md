# ui-check

`make ui-check` type-checks the web UI's parts,
the TypeScript and TSX under `internal/server/ui/src/`,
with `tsc --noEmit` and that directory's `tsconfig.json`:
strict, with Preact's and the signals' types taken from
`third_party/preact/` by `paths`.
CI's job "UI types" runs the same (`GUIDELINES/project/CI.md`, section 2).

This directory holds TypeScript and nothing else:
`package.json` names `typescript` 7.0.2 alone,
and `package-lock.json` locks it with its binaries for each platform,
which npm installs one of.
The target installs them here with `npm ci --ignore-scripts`,
which runs no package's scripts, into `node_modules/`, which git does not keep;
it needs Node.js and npm.
The bundle is not made here: `make ui` makes it with esbuild from Go
(`tools/uibundle/README.md`).

Dependabot proposes a new release (`.github/dependabot.yml`);
by hand, in this directory:

```sh
npm install --save-exact --ignore-scripts typescript@X.Y.Z
```

then `make ui-check`.
