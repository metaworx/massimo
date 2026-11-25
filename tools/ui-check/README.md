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
The target needs Node.js and npm.

## Where they are installed

`install.sh` installs them once per lock file for every checkout of the project,
and prints where; the target then runs `tsc` and `biome` from there:

- **The directory** is `GUIDELINES/temp/ui-check/` of the main checkout:
  the one whose `.git` git names as the common directory,
  so that every worktree and every copy exported into one find the same.
  `UI_CHECK_CACHE` names another.
- **An install** is keyed by the first 16 digits of `package-lock.json`'s SHA-256
  and Node's platform and architecture, `<hash>-linux-x64`,
  with `-musl` on a Linux without glibc,
  so that a branch with another lock and another platform each have their own.
- **A key not installed yet** is installed with `npm ci --ignore-scripts`,
  which runs no package's scripts,
  into a directory of its own beside it, `.new.XXXXXX`,
  then renamed into place.
  Where two runs install at once, the second rename fails, and that run removes
  its copy and uses the first: no run uses a half install.
- **Each use** touches the key's directory;
  a run removes a key that has not been used for 30 days,
  and a `.new.*` that a stopped run left more than a day ago,
  each known by this directory's `package.json` in it,
  so that a `UI_CHECK_CACHE` shared with other programs loses nothing of theirs.
  None of this fails the check.
  Removing the whole directory by hand is safe while no check runs:
  the next run installs again, in a few seconds with npm's cache.
- **Without git**, or where the directory cannot be written,
  npm installs them into `node_modules/` here, which git does not keep,
  as before.

In CI the main checkout is the job's, so they are installed into its
`GUIDELINES/temp/ui-check/` at every run.
An editor that looks for TypeScript or for `biome.jsonc`'s `$schema`
in `node_modules/` here finds neither, unless npm installed them here.

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
