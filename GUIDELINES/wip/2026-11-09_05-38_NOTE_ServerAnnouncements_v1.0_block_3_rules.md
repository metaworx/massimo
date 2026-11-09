# NOTE ServerAnnouncements v1.0: block 3 rules

How Block 3 went, run on 2026-11-09 at the owner's `EXEC++ 3`,
and the migration's number, decided by the owner for AP Logging.

## Decision

- **The migration is 0010, not 0009.**
  The Logging session passed on the owner's word of 2026-11-09:
  AP Logging takes `0009_events_sources.sql`,
  the push subscriptions `0010_push_subscriptions.sql`,
  after Logging's generated `internal/store/migrations/store.sql` (#121),
  which this block writes again with
  `go test ./internal/store -run TestStoreSQL -update`.
  `0009` came to `main` with #122.
  `TestMigration9` compared the migrated version with 9, the newest then;
  it now compares with `SchemaVersion()`.

## Where the work went differently from the plan

1. **The server's events reach the rules in three places.**
   The plan named `emit` and `broadcast`; `broadcast` is gone since #122.
   The Cube's events come from the pump (`connected`, `disconnected`,
   `state`, `paired`, `device_forgotten`), `cube_found` from the one place
   a Cube becomes found, and `update` from `GET /api/v1/update`'s check,
   which happens when the page asks.
2. **What a rule decides is logged.**
   Delivery is Block 4's;
   `Deliver` writes the line `notification` with the user, the kind, the Cube,
   the device, its S/N and its room.
3. **`device_stale` comes from a scan.**
   A stale thermostat sends nothing, so the Sink looks at every Cube's
   device views every five minutes;
   the same tick reads the users again,
   so that an ended session or a disabled user stops being told.
4. **The rules, beyond the Analysis:**
   - a device seen for the first time counts as having had none of the
     conditions, so a restart costs at most one repeated notification,
     as the plan accepted;
   - frost is told again only once the temperature rose half a degree above the
     threshold, against a value that hovers at it;
   - `update` is told once per release, not once per half hour;
   - `cube_online` carries `down_minutes`,
     `device_error` its `error` and `link_error`;
   - the users are told in the order of their ids, the same every time.
5. **The preference `notify` is checked when it is written.**
   `PUT /api/v1/me/preferences/notify` answers `400` for a document the rules
   cannot read: an unknown kind, `after_min` outside 1 to 1440,
   `below_c` outside −10 to 20, a delay on a kind without one.
6. **The subscriptions' routes, beyond the Analysis:**
   - `POST` answers `201` for a new row and `200` for a known endpoint;
   - the endpoint may be `http`, for a UnifiedPush distributor on the LAN;
   - `p256dh` must be a point of P-256,
     so that Block 4's encryption cannot fail on it, and `auth` 16 bytes;
   - a label left out is the request's `User-Agent`, 100 characters at most;
   - the list carries the endpoint, which the page compares with its browser's,
     and `current` for the caller's session or key;
   - `DELETE` of another user's subscription is `404`.
7. **The store, beyond the Analysis:**
   - a row is deleted, not marked: it has a `uuid` but no `version`,
     `deleted_at` or `origin`, being bound to this server's VAPID key;
   - `RevokeToken`, `RevokeSessions`, a user's removal and a new user under
     a removed one's name delete the rows they concern;
   - the list of every subscription leaves out those of an ended or
     revoked token and of a disabled, removed or expired user;
   - `RemovePushSubscription` and `NotePushResult` are there for Block 4.
8. **The VAPID key** is a P-256 ECDSA pair, kept as the setting
   `push.vapid` with the private scalar and the uncompressed public point,
   base64url.
   The plan said it stays in the store "like the OPAQUE server key";
   the OPAQUE key is a file beside the store, and `docs/store.md` says so.
9. **The plan's manual check was not run.**
   `massimo fakecube` cannot open a window by command,
   so `TestNotificationFromTheCube` stands in for it:
   it sets the fake Cube's L line, a device's battery goes low,
   and the notification comes through the server's events and the rules
   to the subscriber alone.
10. **`internal/notify` is not under `make race`.**
    Its Sink runs a goroutine of its own; its tests ran under `-race` by hand,
    20 times; adding it to the race list is the owner's decision,
    asked at the completion gate.
