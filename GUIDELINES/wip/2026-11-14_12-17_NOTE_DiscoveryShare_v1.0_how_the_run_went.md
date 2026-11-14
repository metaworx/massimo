# NOTE DiscoveryShare v1.0: how the run went

Where the run of AP DiscoveryShare v1.0 went differently from the plan,
and what it measured.

## Block 1

- **The changelog fragment** came with Block 1, not Block 3:
  Block 1 already touches shipped paths, which owe a bullet in their commit;
  Block 2 widened the same fragment to name the proxies found.
- **On Windows through interop**, the `sharedport` tests and the server's
  discovery tests passed; the service's `service.log` was not touched.

## Block 2

- **The test's seam** is a `sent` hook on the Responder, told of each reply sent,
  the plan's "recording sender";
  the Linux listener on the loopback's broadcast was not needed.
  With the broadcast taken out, `TestResponderAnswersItsOwnHostByBroadcast` fails.
- **The Windows end-to-end check** used a scratch program instead of a `serve`:
  the real Responder over a fake Cube, its port shared on 23999,
  and Java 8's `MulticastSocket` probe sending a search to 192.168.178.255.
  A `serve` answers discovery only for a proxy on port 62910,
  which the owner's running service holds, so it could not have been checked
  without touching the service.
  Block 1 alone: the probe binds, and hears only its own request.
  Block 2: it gets the proxy's answer, from the host's LAN address.
  The same on Linux with Java 17.
- **`make lan-test`** passed, all 14 checks.
