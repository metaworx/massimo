- `POST /api/v1/tokens/revoke` ends a token by its revoke code, without
  the token, for an app that forgot it while its server did not answer
  (`docs/api-v1.md`); the store keeps each code's hash (migration 0100).
