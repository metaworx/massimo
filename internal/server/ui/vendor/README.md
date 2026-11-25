# Vendored code of the web UI

The page has no build step and no dependencies (`docs/ui.md`), with one
exception taken knowingly (`docs/store.md` section 10, item 17): the
OPAQUE client for the sign-in of `auth_version` 2, which needs argon2id and
ristretto255 in the browser, compiled to WebAssembly.

| File | Package | What | Version | Licence | Includes |
|------|---------|------|---------|---------|----------|
| `opaque.js` | `@serenity-kit/opaque` | its `esm/index.js`, `opaque-ke` 4 in WebAssembly, inlined as base64 | 1.1.0 | MIT, `opaque.LICENSE` | `argon2` 0.5.3, `base64` 0.22.1, `blake2` 0.10.6, `block-buffer` 0.10.4, `curve25519-dalek` 4.1.3, `digest` 0.10.7, `dlmalloc` 0.2.11, `elliptic-curve` 0.13.8, `generic-array` 0.14.7, `hashbrown` 0.16.1, `hkdf` 0.12.4, `hmac` 0.12.1, `js-sys` 0.3.85, `once_cell` 1.21.3, `opaque-ke` 4.0.1, `rand_core` 0.6.4, `serde-wasm-bindgen` 0.6.5, `serde_core` 1.0.228, `sha2` 0.10.9, `voprf` 0.5.0, `wasm-bindgen` 0.2.108 |

The file is the package's as published, unchanged;
the npm integrity of the tarball and the SHA-256 of the file:

```
sha512-Y6v/+hRMn0MdMEk5+/ArM0vPIiFfFEbdTZc7oAx+cWyvGODGezAQ/sMjXBVogf7NsS9z9EV0Ve5paZCVULuedw==
1e8d56383a77948e453085b6d4f52dcb9645f8bfff32395738b63d3a2e2fcb52  opaque.js
```

`TestVendoredOpaque` checks the SHA-256,
so that a change is a change on purpose.
The WebAssembly inside also holds the Rust crates `opaque-ke` was built with,
under MIT or Apache-2.0 and BSD-3-Clause.
Includes names those the WebAssembly itself names,
in the paths of their sources that Rust leaves in it (`<crate>-<version>/src`),
`dlmalloc` and `hashbrown` most likely with Rust's standard library;
crates that leave no such path are not named,
and the package publishes no lock file that would name them.
The WebAssembly holds Rust's own `core`, `alloc` and `std` as well
(their paths `/rustc/<commit>/library/…`), under MIT or Apache-2.0.
So the licence texts of these crates and of Rust's libraries are not reproduced here.
`TestVendoredOpaqueCrates` holds Includes to the crates the file names.

An update: `npm pack @serenity-kit/opaque@<version>` in an empty directory,
`esm/index.js` copied here as `opaque.js` and `LICENSE` as `opaque.LICENSE`,
the version, the integrity and the SHA-256 above and in the test changed,
then `make credits` and `make ui-test` run.
System info's credits (`internal/credits`, `tools/credits/README.md`) take
each file here from the table above: its name from Package, its version,
its licence and what it includes; a file without a row is named by its file name,
and a file with no licence file beside it (`<name>.LICENSE`) stops
`make credits`.
Its WebAssembly needs `'wasm-unsafe-eval'` in the page's
`Content-Security-Policy` (`securityHeaders` in `ui.go`).
