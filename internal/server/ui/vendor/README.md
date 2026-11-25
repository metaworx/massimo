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
under their own licences (most of them MIT or Apache-2.0).
Includes names those the WebAssembly itself names,
in the paths of their sources that Rust leaves in it (`<crate>-<version>/src`),
`dlmalloc` and `hashbrown` with Rust's standard library (their paths under `/rust/deps/`);
crates that leave no such path are not named there,
and the package publishes no lock file that would name them.
`opaque.LICENSE-3RD-PARTY` holds their licence texts
and those of the crates they are built with,
collected for Massimo from the crates' archives (the credits say so beside Includes):
those Includes names, at their versions;
every other crate at the newest version of its series on crates.io
when npm published the package,
resolved from the package's `Cargo.toml` at the commit npm's provenance names.
It is a reconstruction, not the build's lock, which is not published.
Its head says so and how the versions were had,
and lists the crates, each by the SHA-256 of its archive on crates.io;
`go run ./tools/opaquecrates` writes the texts from it (`tools/opaquecrates/README.md`).
`gloo-utils` 0.2.0 has no licence file in its archive:
the file says so, with the licence it declares.
The WebAssembly holds Rust's own `core`, `alloc` and `std` as well
(their paths `/rustc/<commit>/library/…`), under MIT or Apache-2.0;
their texts are not in the file, which names the compiler's commit.
The credits take the file up as one of the OPAQUE client's licence files;
it is not served under `/ui/vendor/`.
`TestVendoredOpaqueCrates` holds Includes to the crates `opaque.js` names,
the list of `opaque.LICENSE-3RD-PARTY` to them too,
the file's body to its own list,
and the file to its SHA-256.

An update: `npm pack @serenity-kit/opaque@<version>` in an empty directory,
`esm/index.js` copied here as `opaque.js` and `LICENSE` as `opaque.LICENSE`,
the version, the integrity and the SHA-256 above and in the test changed;
the crates resolved again (`tools/opaquecrates/README.md`),
the head of `opaque.LICENSE-3RD-PARTY` written again,
and the commit, the time and the file's SHA-256 in the test;
then `go run ./tools/opaquecrates`, `make credits` and `make ui-test` run.
System info's credits (`internal/credits`, `tools/credits/README.md`) take
each file here from the table above: its name from Package, its version,
its licence and what it includes; a file without a row is named by its file name,
and a file with no licence file beside it (`<name>.LICENSE`) stops
`make credits`.
Its WebAssembly needs `'wasm-unsafe-eval'` in the page's
`Content-Security-Policy` (`securityHeaders` in `ui.go`).
