# NOTE ServerAnnouncements v1.0: gate decisions

The owner's answer of 2026-11-09 to the plan gate of AP ServerAnnouncements v1.0:
register and push the plan on `main`,
the implementation in a session of its own, and two additions to Block 2.

## Decisions

- **D1-A**:
  the mDNS responder of our own in `internal/announce`,
  over `golang.org/x/net/dns/dnsmessage`, `ipv4` and `ipv6`.
- **D2-A**:
  a user's notification choices in the preference key `notify`.
- **D3-A**:
  a push subscription needs the capability `preferences`.
- **D4-A**:
  `serve -announce` and `MASSIMO_ANNOUNCE`,
  on by default when the listener is reachable from the LAN,
  with the addition below.

## Additions to Block 2

1. **`discover` marks a proxy by its MAC as well.**
   A Cube's MAC begins with eQ-3's prefix, `00:1A:22`, as the captures and
   the API's examples show; a `serve` answering discovery as a Cube answers
   from its host's interface, whose MAC does not.
   So `discover` marks a found Cube as `proxy` when its MAC, from the ARP
   cache (`internal/cubelan/arp.go`), does not begin with that prefix,
   as well as when its address is an announced server's;
   where the ARP cache gives no MAC, the address match alone decides.
   This takes the second half of roadmap item 18 into the plan;
   the "Not in this plan" line that excluded it no longer holds.
2. **The announce switch is the reboot switch's.**
   `-announce` and `MASSIMO_ANNOUNCE` are parsed by `parseSwitch` of
   `cmd/massimo/reboot.go`, which `-allow-reboot` and `MASSIMO_REBOOT` use,
   and the helper gains `enabled` and `disabled`,
   so that both switches take on, true, yes, 1, enabled and their opposites,
   in any case.
   The texts that list the values, `serve -help`, `README.md` ("Server"),
   the settings file's template and the packaging READMEs, say so;
   the helper moves to a file of its own, `switch.go`,
   since it serves two flags now.

## What runs it

A session of its own, on `claude/server-announcements` from `origin/main`
after this registration; one pull request per block, Block 1 first,
which AP MobileApp waits for.
