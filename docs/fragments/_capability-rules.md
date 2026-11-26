> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# What a capability includes, and whom a user changes

`update` and `reboot` each include `restart`, since each restarts the server:
their holder passes every check of `restart` as a holder of `restart` does,
and `GET /api/v1/me` lists `restart` beside them.
The include is taken at the check, on the set the bounds leave
(the role's set less the capabilities disabled for every role,
an integration's ceiling and maximum, a call's level):
an integration whose maximum holds `update` and not `restart` restarts the server,
a call of it at `control`, which leaves no `update`, does not,
and `restart` disabled for every role is disabled for their holders too.
Nothing stored changes, and no migration runs:
`GET /api/v1/roles` shows each role's set as saved,
and the rule below compares the stored sets.
`reboot` does not include `update`, nor `update` `reboot`.

A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
A holder of `integrations` changes an integration's user whatever its role,
but one stored as `owner`, which stays an owner's alone,
and gives it only a role it outranks
that lies within the integration's maximum (AP AccessEnds, D47-A, D48-A):
so the manager, which does not outrank `integration`,
moves such a user off it, and an owner gives it back.
