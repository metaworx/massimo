# NOTE IntegrationUsers v1.1: how block 8 went

Block 8, on the owner's `PLAN++ D18-C D19-A D20-A`, 2026-11-16: #322.

## Where the run went differently

1. **An API change against D19-A:** the integration's view gave
   `request_expires` for a pending request only, so the page could not tell
   an approval waiting for its key from one expired. An approved
   integration whose key is not collected now carries it too
   (`viewIntegration`); docs/api-v1.md and the OpenAPI file say so.
2. **The tests:** the re-reading after Approve and "approval expired" are
   part of `test_register_and_link`, not a test of their own: each case's
   request counts against the address's ten requests in ten minutes, and a
   second test per case met `429`.

## Measured

- `test_integrations.py`: 20 passed at 1100 and 390 px, English and German.
- `go test -race ./internal/server/` failed once at a load average of about
  12, the sub-agent's build beside it, the test not captured; it passed the
  two runs after.
