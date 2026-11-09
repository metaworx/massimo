# NOTE ServerAnnouncements v1.0: block 2 announce

How Block 2 went, run on 2026-11-09 at the owner's `EXEC++ 2`,
with D6-A, the owner's at Block 1's completion gate.

## Decision

- **D6-A**:
  `./internal/serve` and `./internal/announce` join the Makefile's `RACEPKGS`,
  the contract's test gate (`_CONTRACT.md` v1.3.2,
  inlined into `AGENTS.md` and `GUIDELINES/README.md`)
  and `GUIDELINES/project/CI.md` v1.13.3.
  The two generated files were edited by hand, the worktree's
  `GUIDELINES/shared` being empty;
  `sync-docs.sh --check`, run over a copy with the shared fragments,
  finds both up to date.

## Where the work went differently from the plan

1. **The SRV target takes the node's last eight hex digits, not the first.**
   `node_id` is a UUIDv7, whose first 48 bits are the time in milliseconds:
   its first eight hex digits change every 65 seconds,
   so two boxes set up within a minute would share `massimo-<node8>.local`.
   The last eight are of its random part.
2. **`internal/cubelan` changes too.**
   `Discover` kept one answer per serial and so dropped the proxy's,
   which the item-18 NOTE requires `discover` to list;
   it now keeps one per serial and address.
   `IsEQ3MAC` and `EQ3Prefix` live there, beside the ARP lookup;
   `discover`, an unexported seam, takes the port, for
   `TestDiscoverKeepsEveryAnswerOfASerial`.
3. **The announcement is `internal/serve`'s, not the command's.**
   The plan, written before Block 1, put it in `cmd/massimo/serve.go`.
   `Config.Announce` (`On`, `Name`, `Version`) is off in the zero value,
   so an embedder announces only when asked (AP MobileApp's "no
   announcement unless the options ask");
   `Run` starts the responder after the first log line and waits for its
   goodbye when it stops; a responder that cannot start is a Warn line.
   The command decides the switch: `-announce`, else `MASSIMO_ANNOUNCE`,
   else on unless `-listen` is a loopback address or `localhost`.
4. **`-allow-reboot` and `-announce` are switch flags.**
   NOTE gate_decisions' addition 2 has both switches take on, true, yes, 1,
   enabled and their opposites; `-allow-reboot` was a Go bool flag,
   which takes neither yes nor enabled,
   so both flags are now a `switchFlag` read by `parseSwitch`,
   standing alone for on as before.
   `MASSIMO_ANNOUNCE` joins `env.Known` and the reloader's flag map;
   a change of it applies at the next start.
5. **The records, beyond the Analysis:**
   - the A and AAAA records are 120 s as the SRV;
   - an announcement goes out on every joined interface with that interface's addresses,
     at the start,
     a second later (RFC 6762 section 8.3 asks for two) and every two minutes;
     the goodbye carries every record at TTL 0;
   - a legacy unicast answer gives the addresses of the interface whose
     network holds the asker, else every interface's that has a link;
   - `_services._dns-sd._udp.local` is answered too, for a browser that
     lists every service type (`avahi-browse -a`);
   - the TXT record's `fp` is read at every answer, since a renewal changes it.
6. **`Browse` sends only legacy unicast queries.**
   A box whose firewall drops unsolicited UDP (ufw) does not let in the
   answers to the multicast query, which come from other addresses;
   the answer to `-addr`'s unicast query does come back.
   With `-addr`, as for the Cubes, only the addresses given are asked.
7. **WSL 2 in NAT mode announces rather than warns.**
   Its `eth0` takes multicast, so `serve` there announces into WSL's own
   network and logs "announced on the LAN"; the Warn line comes where no
   interface joins the group, or the port cannot be opened.
   Run here on 2026-11-09 against `massimo fakecube`:
   `discover -addr 127.0.0.1` and a plain `discover` list the server,
   the second at `eth0`'s address;
   with the proxy on `127.0.0.1:62910` the fake Cube's serial is listed
   at 127.0.0.1 and marked `proxy` by the address,
   the ARP cache having no MAC for the loopback.
   A machine with multicast on the LAN,
   `avahi-browse` and `dns-sd` were not tried.
8. **Tests beside the plan's:**
   `TestAnnounceWithoutMulticastWarns` checks the socket that cannot be opened,
   the Warn line itself is `internal/serve`'s `TestRunUnannouncedWarns`;
   also `TestAnnounceTLSFingerprint`, `TestNames`, `TestRunAnnounces`,
   `TestIsEQ3MAC`, `TestMarkProxies`, `TestLoopbackListen` and `TestSwitchFlag`.
   No test opens port 5353 or a group: the responder's port and the
   announcements' destination are options of the tests.
9. **The "Debian hints" were nowhere.**
   `packaging/README.md`'s Debian section gains the 5353/udp line;
   the ufw lines of `README.md` and `docs/proxy.md` gain the port.
10. **`docs/portal-protocol.md` does not record the proxy's double answer.**
    `docs/proxy.md` says it as the proxy's behaviour now;
    the hardware finding is the coordinating session's to write.
