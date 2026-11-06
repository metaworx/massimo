> **Fragment** — inlined by `tools/sync-docs.sh`; not a standalone document.

# {{project_name}} (v1.0.0)

<!-- TEMPLATE: installed as {{guidelines_root}}/project/_README.md. This is the
     opening of the generated {{guidelines_root}}/README.md, and the only
     project document written for people first. Replace every {{TODO: …}}.
     Keep it short: what this is, how to work on it, where the rules live. -->

Massimo is open-source successor software for eQ-3 MAX! heating systems (the MAX!
Cube LAN gateway, radiator and wall thermostats, window contacts,
eco switches) after the vendor's end of life:
one Go binary per platform that keeps the Cubes connected,
serves a JSON API and a web UI,
answers as a Cube for the original software and other programs,
and backs up and restores a Cube.
It is for the owners of such systems and for the people and agents who develop it;
`README.md` at the root is the user's document.

## Working on it

```bash
git clone --recurse-submodules git@github.com:metaworx/massimo.git
cd massimo
make            # go test ./... and the binary
make cross      # every target, without cgo
```

Go 1.26 is the toolchain (`go.mod` says the minimum, CI pins 1.26).
Everything runs against the fake Cube: `./massimo fakecube -listen
127.0.0.1:62910 -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt`
in one terminal, `MASSIMO_ENV_FILE=/dev/null ./massimo serve -addr
127.0.0.1:62910 -proxy off` in another, the web UI at
<http://127.0.0.1:8099/>. The UI smoke test needs Python with Playwright
once (`tools/ui-test/README.md`), then `make ui-test`.
Real hardware is reached through the `.env` file (gitignored; `README.md`,
"Cube variables") and only through the proxy of a running `serve`.

## The rules that apply here

The conventions below are not AI configuration:
they are how this project is written,
and an agent is simply another party required to follow them.

- The contract that follows is generated into `/AGENTS.md` as well,
  so people and agents are held to the same text.
- The shared conventions live in `{{shared_root}}` — commit policy,
  quality pass, per-language testing, linting and code style.
- `{{guidelines_root}}/README.md` explains the gate protocol:
  what an agent asks before it changes anything,
  and what `EXEC` and its relatives authorise.

Both generated documents come from the fragments in `{{guidelines_root}}`.
Editing a generated file is wasted work — change the fragment and re-run
`{{shared_root}}/tools/sync-docs.sh`.
