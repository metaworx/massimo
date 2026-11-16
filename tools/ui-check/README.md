# ui-check

`make ui-check` checks the web UI's parts,
the TypeScript and TSX under `internal/server/ui/src/`, twice:

- **their types,** with `tsc --noEmit` and that directory's `tsconfig.json`:
  strict, with Preact's and the signals' types taken from
  `third_party/preact/` by `paths`;
- **their hooks,** with `biome lint` and this directory's `biome.jsonc`:
  two rules and no other, `useHookAtTopLevel`
  (a hook at the top of a part's drawing, never after an early return or in a branch)
  and `useExhaustiveDependencies` (an effect lists what it reads).
  A hook is known by its name, `use` and a capital letter,
  so a region's counter is read with `useLoad()`.
  An effect that must not list what it reads says why in a
  `// biome-ignore lint/correctness/useExhaustiveDependencies: <reason>` comment,
  which Biome does not accept without its reason.
  No formatter and no layout rule.

CI's job "UI types" runs the same (`GUIDELINES/project/CI.md`, section 2).

This directory holds TypeScript and Biome and nothing else:
`package.json` names `typescript` 7.0.2 and `@biomejs/biome` alone,
and `package-lock.json` locks them with their binaries for each platform,
which npm installs one of.
The target installs them here with `npm ci --ignore-scripts`,
which runs no package's scripts, into `node_modules/`, which git does not keep;
it needs Node.js and npm.
The bundle is not made here: `make ui` makes it with esbuild from Go
(`tools/uibundle/README.md`).

Dependabot proposes a new release of either (`.github/dependabot.yml`);
by hand, in this directory:

```sh
npm install --save-exact --save-dev --ignore-scripts typescript@X.Y.Z
npm install --save-exact --save-dev --ignore-scripts @biomejs/biome@X.Y.Z
```

then `make ui-check`.
A new Biome may change what its two rules find, or the form of `biome.jsonc`:
`biome migrate --write` in this directory brings the configuration to the new form.
