# NOTE MobileApp v1.4: gate decisions

The owner's answer at v1.4's plan gate, on 2026-11-23:
"D84 yes but build it with a flag so it could be disabled", and `EXEC++`.

## D84-A, with a switch

The password's eye is built as Block 15's step 8 says,
in the shared field (`src/shared/password.tsx`), for every password field,
and it can be switched off:

- a server setting, on by default, among System settings' own,
  written by whoever may write the server's settings;
- the page learns it before anyone signs in,
  where the sign-in card learns its ways,
  so that the card shows the eye or not from its first drawing;
- switched off, no password field shows the eye,
  and a browser's own eye stays hidden as with it on;
- its name, its place in the settings and the route that carries it
  are the block's to settle, in `docs/api-v1.md`, the OpenAPI file and
  `docs/ui.md`, with a Go test of the setting and a UI test of the eye
  switched off.

## The run

`EXEC++`, as the gate stated it:
v1.4 and this NOTE committed onto `main`;
Blocks 17, 16, 15 and 18 in that order,
each on its own branch and pull request,
each merged by this session once its CI is green,
the next started from `main`;
the phone checks of the four blocks gathered for one phone session at the end;
no retirement, no release, no roadmap edit.
The session "UI refactor" is told before Blocks 16 and 15 start.
