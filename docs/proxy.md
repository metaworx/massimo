# The LAN proxy: `massimo serve` as a Cube

The Cube serves a single TCP client on port 62910.
While `massimo serve` holds that connection, every other program waits:
the other massimo commands, the original software, FHEM, openHAB.
The server therefore answers as a Cube itself, on every interface, port 62910,
and the other programs connect to the server instead.
Only the server talks to the Cube,
and its command queue sees every radio command,
so the duty cycle and the memory slots are accounted for once.

```bash
massimo serve                       # proxy on :62910, discovery on UDP 23272
massimo serve -proxy 192.168.1.5    # one interface
massimo serve -proxy off            # API only
massimo serve -proxy KEQ0000200=:62910 -proxy KEQ0000100=:62911
```

Without `-proxy`, each Cube listens on its `MASSIMO_CUBE_<SERIAL>_PROXY` address,
which is also the address clients use,
and a single Cube without that variable listens on `:62910`.
With several Cubes each listener names its Cube,
or `-sn <serial>` restricts the server to one Cube.
Discovery is answered only for a listener on port 62910,
since a discovery reply carries no port.

## Several Cubes on one host

A discovery reply carries no address either:
the client connects to the IP the reply came from, port 62910.
So every Cube needs an IP of its own on the host.
Give the host a second address,
on Linux for example `ip addr add 192.168.1.47/24 dev eth0` (or the equivalent
in NetworkManager so it survives a reboot), and set

```bash
MASSIMO_CUBE_KEQ0000200_PROXY=192.168.1.44
MASSIMO_CUBE_KEQ0000100_PROXY=192.168.1.47
```

The server then listens on each address and answers discovery for each
Cube from its address, on one shared UDP socket.

## Clients find the proxy by themselves

Set `MASSIMO_CUBE_<SERIAL>_PROXY=<server host>` in `.env`.
Every command except `serve` then connects to the proxy when it answers within
two seconds and to the Cube itself otherwise, so `status`, `backup`, `restore`,
`cube` and `watch` work the same whether the server is running or not.
`-addr` bypasses this.

`massimo discover` lists the proxy among the Cubes, marked `proxy`,
and the servers that announce themselves by mDNS in a table of their own (README.md,
"Server").
The proxy answers a search with the serial of the Cube it serves,
unless `-proxy-serial` gives another,
so a search hears that serial twice, from the Cube and from the proxy:
`discover` lists both answers, each with its address and MAC.
It tells the proxy's by the MAC, which the client's ARP cache has for the
address that answered:
a Cube's begins with eQ-3's `00:1a:22`,
the proxy's is its host's network card's.
Where the cache has none (the server on the same box as `discover`),
the address decides: it is an announced server's.
The Cube's discovery reply has no field for a MAC address,
so the proxy cannot announce one inside the Cube protocol, and need not.

## What a client gets

On connect, the greeting a real Cube sends: `H` with a fresh connection id,
the current duty cycle, free memory slots and clock, then `M`,
the `C` line of every device and `L`.
When the Cube reports a state change,
the new `L` line is pushed to every connected client, as the Cube does.
Several clients may be connected at the same time.

| Client line | Handling |
|---|---|
| `l:` | answered from the server's copy, the Cube is not asked |
| `c:<rf>` | answered from the server's copy; passed through when unknown, and the answer added to the copy |
| `q:` | closes the connection |
| `s:`, `r:` | passed through the queue; the Cube's `S:` is relayed |
| `m:` | passed through, `A:` relayed, the server's copy of the metadata updated |
| `z:`, `t:`, `v:`, `f:`, `u:`, `i:`, `a:`, `w:`, `e:`, `d:` | passed through, the reply relayed |
| `n:`, `x:` | pairing through the server, see below |

Passed-through lines go to the Cube exactly as sent, one queue entry per line,
in order with the server's own commands.
The queue adds no wakeup or error reset for them:
a client that speaks the protocol sends its own `z:` and `r:` lines,
as the original software does.

A client's radio write (`s:`) that the Cube took is followed up as the server's
own writes are (README, "Server"): after a configuration write (week programme,
preset temperatures, valve settings, display mode,
room group) the server reads the device's `C` line again for about six minutes
until it changes,
for every thermostat of the room when the write was addressed to the room;
after any radio write it reconnects once for the budget if radiograms were pending.
A setpoint, a key lock or a link re-reads nothing.
A write the Cube discarded is not followed up.

## Hidden devices

The devices of room 255 and those on the server's hidden list
(`MASSIMO_CUBE_<SERIAL>_IGNORE`,
see the README) are hidden from proxy clients too:
they are left out of the greeting's metadata and C lines and of every L line,
so `status` on another computer and the original software do not show them.
Room 255 itself is hidden as well;
the original software allocates the lowest free id and never reaches 255,
so it cannot reuse it.
Any other room of hidden devices stays visible, empty.
Park devices that never report in room 255 by editing a backup (the room id
and the devices' `room_id`) and restoring it with `massimo -show-hidden restore`,
which moves the devices and sets the Cube's room byte for them.
A metadata write from a client is forwarded with the hidden devices and their
room put back, so a client that never saw them cannot remove them from the Cube;
the server logs when it does that.
A delete command (`t:`) naming a hidden device is refused for that device and logged:
the others in it are deleted,
and when none is left the proxy answers the client itself without sending anything,
so a client cannot delete a hidden device (and make the Cube send it a factory reset).
The Cube answers a `t:` with an `A:` line for each device it names,
each when it is done with it;
the server waits for all of them before the next command,
and the client gets one for each device it named, the hidden ones included.
`serve -show-hidden` switches all of this off.
Once the Cube answered a client's `t:`,
the server's copy forgets the devices it deleted,
as after `DELETE /api/v1/cubes/{sn}/devices/{rf}`,
so that the original software's deletions show at once.

## Pairing

`n:<seconds>` from a client starts pairing on the server,
the same mode as `POST /api/v1/cubes/{sn}/pairing` (docs/api-v1.md):
the server sends `n:` to the Cube through its queue,
and every `N` line the Cube sends while the mode lasts is relayed to that client
however late it comes, including the empty `N` line at the end.
A client that sends `n:` while pairing already runs,
from the API or another client, joins it and gets the lines from then on.
`x:` ends pairing for everyone; the Cube's answer to it, if any, is relayed.
The server fetches the C line of every device that joins,
so its device list and the API have the new device (in no room) at once,
and the `c:` the original software sends next is answered as well.
The rest of the original software's sequence, the wakeup, the links, the group,
the presets and the metadata write that puts the device into a room,
is passed through as usual.
Without a time, or with more than ten minutes,
the proxy asks for 60 seconds or ten minutes.

## Firewall

On a host with ufw, the ports to open on the LAN interface: 62910/tcp (proxy),
23272/udp (discovery), 5353/udp (the announcement by mDNS),
and the API's port when it listens on a LAN address (8099/tcp by default):

```bash
sudo ufw allow 62910/tcp && sudo ufw allow 23272/udp && sudo ufw allow 5353/udp && sudo ufw allow 8099/tcp
```

## Budget

When the duty cycle the Cube last reported is at or above the queue's limit (95 %),
a radio command from a client is not sent;
the client receives `S:<duty>,1,<slots>`, a discarded answer,
which is what the Cube itself does at 100 %.
Below the limit the command is sent and the real answer relayed.

## Discovery

The server answers the UDP discovery request (port 23272,
opcode `I`) with the Cube's radio address and firmware and its own IP address,
so `massimo discover` and the original software list it.
The real Cube answers too, under the same serial.
The original software keeps only the first answer per serial,
so it may never show the proxy;
give the server a different serial for discovery replies,
for example the real one with the first letter changed:

```bash
massimo serve -proxy-serial XEQ0000200
```

or, in `.env`, `MASSIMO_CUBE_<SERIAL>_PROXY_SERIAL=XEQ0000200`.
The TCP greeting always carries the real serial.
Alternatively block the Cube's UDP answers on the PC's firewall.
If port 23272 is in use on the server's host, for example by `massimo discover`,
the server runs without discovery and logs a warning;
clients are then given the address by hand.

The answer follows the Cube:
from the loss the server logs (`cube connection lost`) to the next greeting
it answers nothing,
so that other servers and the original software see the Cube go,
as they would without the proxy.
A reconnect of the server's own (the refresh, a follow-up after a write,
a new address) is no loss, and the answers go on.
A client already connected keeps its connection,
and one given the address still connects and reads the last state.
The server itself is found whatever its Cube does:
it announces itself by mDNS as `_massimo._tcp`,
which the apps and `massimo discover` browse (README.md, "Server");
the Cube's discovery has no answer for a server,
since the original software would take one for another Cube.

The same socket serves the server's own discovery of Cubes (the API's
`POST /api/v1/cubes/discover`, at the start and every 30 minutes):
the Cubes answer to port 23272 only,
so the request goes out from it and their answers come back to it.
The proxy answers that request too;
an answer from one of the host's own addresses with one of the server's serials
is its own and left out.

## Not supported yet

The proxy has no authentication, like the Cube; bind it to a trusted LAN.
