# NOTE ServerAnnouncements v1.0: block 4 deliver

How Block 4 went, run on 2026-11-09 at the owner's `EXEC++ 4-5`.

## Where the work went differently from the plan

1. **A deliverer takes a user, not a subscription.**
   The plan's `Deliver(ctx, Subscription, Notification)` became
   `Deliver(ctx, userID, Notification)`:
   `WebPush` reads the user's subscriptions that may be sent to and sends
   to each, and `Local` gets every notification with its user,
   which a subscription would not name for an app embedding the server.
2. **The fan-out is not in `internal/notify/sink.go`.**
   The Sink stays as Block 3 left it, knowing no store:
   its `Deliver` is the server's `deliverNotification`,
   which logs the notification, sends it on the user's streams and hands it
   to `WebPush` and to `Options.Deliverers`, each on a goroutine the
   server's stop ends, eight at most at once.
   `internal/serve` passes `Config.Deliverers` on, for AP MobileApp.
3. **HKDF is the standard library's** `crypto/hkdf` (Go 1.24 and later),
   not `golang.org/x/crypto/hkdf`.
4. **The push service's answers:**
   every `2xx` is delivered, not `201` alone, since push services answer
   `200` and `202` as well;
   an answer that never came (a refused connection, a timeout) counts a
   failure and is not tried again, as the plan's "any other error";
   a redirect is not followed, so that an endpoint cannot send the server's
   requests elsewhere;
   the log names the push service's host, never the endpoint's path,
   which is the subscriber's capability.
5. **A stream subscriber knows its user.**
   `runtime.subs` keeps each subscriber's user id,
   and `sendUser` sends the `notification` event to that user's streams
   alone, with source `system` and level `info`;
   it is not kept in the log, whose line `notification` is the text log's.
   A notification of a Cube the server serves goes on that Cube's stream,
   one of none (`cube_found`, `update`) on every Cube's.
6. **The tests reach no network.**
   Block 3's tests subscribe endpoints at `push.example.net`, which
   resolves; `storeOptions` gives the server a Web Push client that dials
   the loopback alone, and waits of 10 ms between tries.
   `internal/push/pushtest` is a push service for the tests of both
   packages: it checks the VAPID token as a push service does and decrypts
   each message as its subscriber.
7. **The plan's manual check was not run as a program.**
   `TestNotificationDelivered` stands in for it: against the fake Cube,
   a battery that goes low reaches a `pushtest` service as a message that
   decrypts, with the TTL, the urgency and the topic, on the owner's
   stream and not on an operator's, and through a `Local` deliverer.
8. **`internal/push` is not under `make race`.**
   It runs no goroutine of its own; its tests ran under `-race` with the
   race list, three times.
   Adding it to the list is the owner's decision.
9. **The quality pass had no IDE.**
   No IDE is attached to this session: `gofmt`, `go vet` for linux,
   windows and darwin, and a YAML parse of `docs/api-v1-openapi.yaml`
   stood in for it.

## Risks for the owner

- **The VAPID subject is `urn:uuid:<node_id>`,** as the plan says, a user
  having no address to give.
  RFC 8292 asks for a `mailto:` or `https:` URI; Apple's push service may
  refuse another. Not tried: nothing here reaches a real push service.
- **An endpoint can be any `http` or `https` address,** the LAN's
  included (Block 3 allows `http` for a UnifiedPush distributor there).
  A user with `preferences` can so make the server POST an encrypted body
  to a host of the user's choosing and see, in `GET /api/v1/me/push`,
  whether it answered `2xx`.
  The server follows no redirect and reads no answer beyond its status.
- **The rules still run only for a user with a subscription.**
  An app embedding the server gets, through `Local`, only the
  notifications of such users; whether its own user needs no subscription
  is AP MobileApp's Block 7 to decide.
