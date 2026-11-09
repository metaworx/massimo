# NOTE MdnsInterop v1.0: gate decisions

The owner's answer of 2026-11-09 to the plan gate of AP MdnsInterop v1.0:
`PLAN++ D3-B`, the plan registered on `main` and both blocks run in this
session, on one pull request.

## Decisions

- **D1-A**, the recommended one:
  a Compose file and a `run.sh` that brings it up, checks, and takes it down
  whatever happened.
- **D2-A**, the recommended one:
  the contract's test gate names `make mdns-test` for a change to the
  announcement, `discover` or the proxy's discovery answers.
- **D3-B**:
  CI's Linux job runs only when those paths change,
  through the job Changed paths, as Screenshots does;
  the macOS and Windows steps run in their jobs as they are.
