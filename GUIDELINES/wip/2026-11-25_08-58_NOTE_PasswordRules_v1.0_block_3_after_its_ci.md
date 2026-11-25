# NOTE PasswordRules v1.0: block 3 after its ci

What the pull request's CI found after the NOTE "how block 3 went" was written,
and what was done.

## Two suites' own passwords

Three jobs of the first run failed, for a reason that is the block's:
two suites made their users with passwords the new rule refuses.

- **The Debian package's systemd test** makes its owner on a container named
  `massimo-test` with a password that began with "test".
  The host's first label gives the words `massimo` and `test`,
  so `massimo user add` refused it,
  with "password: contains the user's or the server's name".
- **The Android emulator's suite** adds its user `tester` with a password that
  began with "tester": the user's own name.
- **The launchd template's test** carried the first password too,
  and would have passed by its runner's name alone.

The builder had searched the Go tests and the UI tests for a password that holds
its user's name, not the scripts under `packaging/` and `tools/`.

- **A second commit** on the pull request:
  "test(packaging):
  The suites' passwords hold no name of their user or their host".
  The three scripts take passwords that hold neither.
- **Seen before the push,** with the block's binary under the container's host
  name: both old passwords refused, both new ones taken;
  `GUIDELINES/shared/tools/check`, twelve checks well.
- The second run was green, and the pull request was merged.

## Not run with the new rule yet

The macOS LaunchDaemon and the Windows service jobs run on other triggers.
The launchd script's password is changed with the others;
the Windows scripts set no password.

## For the later blocks

A block that changes what `massimo user add` or the users' API takes searches
every script that makes a user, under `packaging/`, `tools/` and `.github/`,
beside the Go tests and the UI tests.

## For the completion gate

A real example of D50-A's reach: on a box named `massimo-test`,
a new password that begins with "test" is refused,
and the line says "the server's name".
