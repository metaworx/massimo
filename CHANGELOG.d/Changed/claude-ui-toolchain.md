- A build from source runs `make`, or `make ui` before `go build`: the
  web UI's parts are bundled from `internal/server/ui/src/` into
  `parts.js`, which is embedded and not committed (AP UiParts).
