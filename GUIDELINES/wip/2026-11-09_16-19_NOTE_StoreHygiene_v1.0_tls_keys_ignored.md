# NOTE StoreHygiene v1.0: tls keys ignored

A change to Block 1 of
[AP StoreHygiene v1.0](2026-11-09_16-06_AP_StoreHygiene_help_without_store.md),
on the owner's `EXEC tls` of 2026-11-09,
after the completion gate named the gap.

1. **What:** `.gitignore` ignores `/tls/`, after serve's store.
2. **Why:** serve keeps its TLS certificates and their private keys
   in the data directory's `tls/` (`internal/certs`):
   the self-signed pair, `self-signed.pem` and `self-signed.key`,
   and the ACME ones in `tls/acme/`.
   A `serve` with TLS in a checkout's root, the data directory by default,
   left them untracked, where a `git add -A` would commit them,
   as it would have `opaque.key`.
3. **Anchored at the root:** no tracked file lies in a `tls/` anywhere,
   and the anchor leaves a future source directory of that name alone.
4. **No CHANGELOG bullet:** `.gitignore` is not a shipped path.
