# NOTE MdnsInterop v1.1: macos local network

What the diagnostic run of #144's macOS step found, on 2026-11-09,
at the owner's `EXEC diagnose` and `EXEC watch`.

## Measured

Run 42682588428, the job "macOS LaunchDaemon" on the image `macos-26-arm64`
(20261102.0473.1); a second `serve` started by the step, as the runner's
child, with `-log-level debug`:

- `en0` is up with `192.168.64.3/24`, and the route to `224.0.0.251` goes through
  `en0` (`MULTICAST`); four sockets hold UDP 5353, mDNSResponder's and ours.
- Every announcement fails: `mdns: not sent` with
  `sendto: no route to host`, for `224.0.0.251` on `en0` and for `ff02::fb` on
  every interface, from the first announcement on.
- `massimo discover`, by multicast and by unicast to `127.0.0.1` and
  `192.168.64.3`, finds nothing.
- `dns-sd` under `script -q` left empty files, not even its header line, for
  `_services._dns-sd._udp` and for `_massimo._tcp`.

## What it means

`no route to host` to a multicast group that has a route is how macOS 15 and
later refuse a process without the Local Network permission (Apple's local
network privacy): the runner's children have none, so neither our responder
nor our browser reaches the network, and the step cannot pass as written.
It is not a fault of our code that a Linux or Windows host shows:
the Linux job and the Windows step pass.

Apple's local network privacy does not apply to launchd daemons, which is how
`massimo install` runs the service on a Mac. It does apply to a LaunchAgent
and to a program started from a terminal, which is how `brew services` runs
it. On a Mac with macOS 15 or later, a `serve` run that way may also be refused
its announcement, discovery's broadcast and a Cube on the LAN;
no check has tried it.

`dns-sd`'s empty files say only that `script` captured nothing on the runner,
whatever mDNSResponder saw.
