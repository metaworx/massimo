- `GET /api/v1/restart` answers a holder of `update` too, since every
  update restarts the server; without `restart`, `available` is false and
  `blocked` says `capability "restart" required` (docs/api-v1.md).
