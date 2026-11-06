# NOTE StoreUsers v1.0: block 10

How Block 10, the web UI, went where it differs from the plan,
on 2026-11-06 under `EXEC 10+`, after UiStructure was merged.

- **Hiding by capability** is a class per missing capability on the body (`no-setpoint`,
  `no-roles-disable`) and a class `needs-<capability>` on what it hides,
  in the templates and the scripts, with one CSS rule;
  `GET /api/v1/me` gives the set, and outside users mode nothing is hidden.
- **The first-owner form** is a section of the skeleton under the message line,
  shown on every view while the server is open and the caller local (`setup` of
  `GET /api/v1/me`),
  since the partials are for the settings and the templates only.
- **The own password** has a form under Settings beside Log out,
  which the plan did not list:
  a user without `users` has no other way to change it.
- **Log out** forgets an API key and ends a session;
  the dialog takes a key in a second field for the integration case,
  as section 6 says, and a wrong pair stays in the dialog with the reason.
- **A configuration opens on the temperatures** when the role cannot rename,
  since the Room tab is a rename.
- **Settings → Roles** draws the switch per capability for every role under the roles,
  with `capabilities`, rather than a panel of its own;
  `GET /api/v1/roles` carries the capabilities,
  so a `roles` holder needs nothing more.
- **Guests** are created with a password and an end date (D4-A);
  the invitation link is still to come.
- **The screenshot set** was retaken for the Settings page;
  the Cubes shot's locator wanted the class `cubes-settings` on one section,
  so the new sections have classes of their own.
