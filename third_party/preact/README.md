# Preact and its signals, vendored

The web UI's parts (`docs/ui.md`, "How the page is built") are written
with [Preact](https://preactjs.com/) and its signals, and bundled by
`make ui` (`tools/uibundle`) into `internal/server/ui/parts.js`.
The bundler takes these files and nothing from a `node_modules`;
the type check (`make ui-check`) takes their types from here too.
No other library is used (the contract, "The web UI").

| Directory | Package | Version | Licence |
|-----------|---------|---------|---------|
| `preact/` | `preact`, with `preact/hooks` and `preact/jsx-runtime` | 11.2.0 | MIT, `preact/LICENSE` |
| `signals/` | `@preact/signals` | 2.11.4 | MIT, `signals/LICENSE` |
| `signals-core/` | `@preact/signals-core` | 1.14.5 | MIT, `signals-core/LICENSE` |

`@preact/signals-core` is the version `@preact/signals` 2.11.4 asks for (`^1.14.5`),
the newest at the time.

Each directory holds files of its package as published, unchanged,
at the paths the package has them:
the ES module of each entry point the parts import
(the `import` condition of the package's `exports`),
the `.d.ts` files its types need, and its `LICENSE`.
The bundler maps `preact`, `preact/hooks`, `preact/jsx-runtime`,
`@preact/signals` and `@preact/signals-core` to these modules,
and `internal/server/ui/src/tsconfig.json` maps them to these types.
The npm integrity of each tarball:

```
sha512-/HfHI/7N84e6GrWBJ3J280z3mLaRJiDcSvj/A+y/4AbLnND/1UlGHe1sR6oYuTGwId0nP7dqDU9OnLxMPZ7xBg==  preact-11.2.0.tgz
sha512-Rb+5RPo9gWSaHtzJeauhHxiC8KP/IF2DEPkHPEOGOnuy2wnJzRqyeLRankUCYd6ADqtxS6GnWGv6Ie343d8aYw==  preact-signals-2.11.4.tgz
sha512-vcRv7Ityz5JVmeiVqIluLFzT/PDS/AlhHUpw3/aUWqcVUs6bDGIPXE/6RSC4evBNWYvnniozg5Syfj33al3gkg==  preact-signals-core-1.14.5.tgz
```

The SHA-256 of each file:

```
1fe6958409c8c257a70c587a18b6f7f412b179b456630790d30b2ec9a8e4b7d4  preact/LICENSE
bdcbd9abdf2c77c5c45326eab42191e949222d479d0d5f69467387a77040cb55  preact/dist/preact.mjs
e862ab2973aa6455eaa0f77c5d832d2157d6be59a17c8ba546550a575b58c6b4  preact/hooks/dist/hooks.mjs
21898d939444825cee1232af435330610b09171bea22a2f892cf2058d764b63d  preact/hooks/src/index.d.ts
4761ad0e33728d1488ad98a194fe8c6a93cf940fec80e49995c82dc013d081cd  preact/jsx-runtime/dist/jsxRuntime.mjs
a563e6d3e9a0f337066d8407d48304943b961ef6b3f7fbb38651455a6af46afc  preact/jsx-runtime/src/index.d.ts
8199eb2fc00bb9ce68266df5e97ba914e03a56d4dd823629480c7e68d20b5129  preact/src/dom.d.ts
a5437961b23f82d3959e1cc4630c29dfe405068caee25321088b45d3868fef79  preact/src/index.d.ts
5d082f468e25c12528dc4486ab6f3f48ba03c938d1c472ab6b76c8eb0aa42818  preact/src/jsx.d.ts
a11fc89e4c6b118854c7a667734a0b2e6bf2af5e45c6686de31adbccc8f3ae8d  signals/LICENSE
6761e07a03f7704624ff7af23678a1bcdbd32edd593de8b3f4d1e24e983ab1ef  signals/dist/signals.d.ts
ba8177597f45acaca4d05dcf3966b527d0a8264cfa2b0a87ace92dff324217f8  signals/dist/signals.mjs
a11fc89e4c6b118854c7a667734a0b2e6bf2af5e45c6686de31adbccc8f3ae8d  signals-core/LICENSE
7fd86ad5dfe671ca797ac75d99a657f2ae66022140428717f6c17373282e3050  signals-core/dist/signals-core.d.ts
0c80bc127d72ecb1496ef116ffddae75079829a3af01e7f417c8e5a6474c68b6  signals-core/dist/signals-core.mjs
```

`TestVendoredPreact` (`internal/server/ui_test.go`) checks every file here
against this list, and that the list names every file but this one,
so that a change is a change on purpose.
`.gitattributes` keeps their line endings,
so that the bundle is the same on every checkout.

An update, in an empty directory outside the checkout:

1. `npm pack preact@<version> @preact/signals@<version> @preact/signals-core@<version>`,
   which downloads the tarballs and runs nothing,
   and `npm view <package>@<version> dist.integrity` for each,
   the same as `sha512-` and the base64 of the tarball's SHA-512;
2. each tarball unpacked, and from its `package/` the files above copied
   here at the same paths;
   a file an entry point now needs besides them is added,
   and one it no longer needs removed;
3. the versions and both lists above, and the list in `TestVendoredPreact`,
   changed;
4. `make credits`, which takes each directory's name and version for
   System info's credits from the first table above;
5. `make ui`, `make ui-check`, `MASSIMO_ENV_FILE=/dev/null go test ./...`
   and `make ui-test` run.
