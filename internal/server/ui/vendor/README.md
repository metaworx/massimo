# Vendored code of the web UI

The page has no build step and no dependencies (`docs/ui.md`), with one
exception taken knowingly (`docs/store.md` section 10, item 17): the
OPAQUE client for the sign-in of `auth_version` 2, which needs argon2id and
ristretto255 in the browser, compiled to WebAssembly.

| File | What | Version | Licence |
|------|------|---------|---------|
| `opaque.js` | `esm/index.js` of `@serenity-kit/opaque`, `opaque-ke` 4 in WebAssembly, inlined as base64 | 1.1.0 | MIT, `opaque.LICENSE` |

The file is the package's as published, unchanged;
the npm integrity of the tarball and the SHA-256 of the file:

```
sha512-Y6v/+hRMn0MdMEk5+/ArM0vPIiFfFEbdTZc7oAx+cWyvGODGezAQ/sMjXBVogf7NsS9z9EV0Ve5paZCVULuedw==
1e8d56383a77948e453085b6d4f52dcb9645f8bfff32395738b63d3a2e2fcb52  opaque.js
```

`TestVendoredOpaque` checks the SHA-256,
so that a change is a change on purpose.
The WebAssembly inside also holds the Rust crates `opaque-ke` was built
with (`curve25519-dalek`, `argon2` and their own), under MIT or Apache-2.0
and BSD-3-Clause.

An update: `npm pack @serenity-kit/opaque@<version>` in an empty directory,
`esm/index.js` copied here as `opaque.js` and `LICENSE` as `opaque.LICENSE`,
the version, the integrity and the SHA-256 above and in the test changed,
and `make ui-test` run.
Its WebAssembly needs `'wasm-unsafe-eval'` in the page's
`Content-Security-Policy` (`securityHeaders` in `ui.go`).
