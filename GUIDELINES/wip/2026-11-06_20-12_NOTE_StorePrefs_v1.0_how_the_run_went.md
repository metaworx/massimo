# NOTE StorePrefs v1.0: how the run went

The run of Blocks 1 to 6 under `EXEC++` on 2026-11-06,
where the work went differently from the plan, a line each.

- **Block 5 ran in this run**:
  pull request 56, the pytest suite, was already in `main`,
  the owner said mid-run, so the smoke block did not wait.
- **The browser keeps its keys** (`massimo.poll`, `massimo.theme`,
  `massimo.scheme` and the others) as the cache of the user's values,
  in place of the plan's `massimo.ui`; the suite and the screenshots read them.
  Only the overrides are new, in `massimo.device`.
- **The schemes are merged** at sign-in,
  the server's with the browser's own it lacks,
  rather than one replacing the other;
  there is no marker of whose cache a browser holds,
  so the first sign-in without a server copy seeds from what the browser shows.
- **A pending write is sent when the page is left or hidden** (`pagehide`,
  `visibilitychange`, `fetch` with `keepalive`):
  the suite showed the last change of a page lost in the second's wait,
  and the next sign-in would have taken the server's older value over it.
- **The suite removes the token's user's preferences before every page**,
  since its four cases share the server;
  the screenshots answer the preferences routes themselves,
  so that one shot's mode is not the next's.
- **The routes take relative URLs** in the page,
  a rule `main` gained meanwhile (`TestUIRelativeURLs`).
- **The wrapper's exclusion from the command rows** is a set of actions,
  `login`, `logout`, `preference.set` and `preference.delete`.
- **Block 6 fixed a stale line** of docs/store.md that AP StoreUsers' Block 10 left,
  which still said stage (b)'s web UI was missing.
- **The model changed** to Claude Opus 5.5 before the run;
  its commits carry `Assisted-by: claude-code:claude-opus-5-5`.
