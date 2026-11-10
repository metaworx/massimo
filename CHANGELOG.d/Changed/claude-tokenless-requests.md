- A request to the API without a token, ticket or `?token=` is still
  refused with 401 but no longer counts toward the address's lockout:
  only wrong tokens, tickets and passwords do (`docs/api-v1.md`).
