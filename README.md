# Massimo

Open-source successor software for eQ-3 MAX! heating systems (MAX! Cube LAN
gateway, radiator and wall thermostats, window contacts, eco switches) after
the vendor's end of life.

Status: early development (Phase 0). Licensed under the
[GNU Affero General Public License v3.0 or later](LICENSE).

"MAX!" and "eQ-3" are trademarks of eQ-3 AG. This project is not affiliated
with or endorsed by eQ-3.

## Command line

Build with `make` (or `make windows` for a Windows binary in `dist/`).
`massimo help` lists the commands; the ones useful today:

```bash
massimo backup -addr <cube-ip> [-out backup.yaml]
```

saves rooms, device names, thermostat settings, week programs, NTP servers
and time zone rules, together with the raw protocol lines, as JSON or YAML.

```bash
massimo restore -addr <cube-ip> -from backup.yaml -dry-run
```

compares a backup (which may be edited by hand) with the Cube and shows
what would be written; without `-dry-run` it asks and then sends only the
differences, respecting the radio duty cycle. Each device's live state
(mode, setpoint, measured temperature, valve) is in the file for
information; `-state` re-applies mode and setpoint too. Pairing cannot be restored
over the LAN: devices missing from the Cube are listed, and the restore can
be repeated after they have been paired again.

```bash
massimo status
massimo watch -addr <cube-ip>
massimo fakecube -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt
```

`status` lists every device the Cube knows, grouped by room, with its
last state and whether the Cube holds state for it at all: after a Cube
restart every device is "not reported" until it transmits again, window
contacts only on a window change, and the Cube discards room commands
while a room has such a member, so the room is marked not ready (`-json`
for machines). `watch` prints state changes as they happen; `fakecube`
serves recorded data on port 62910 so everything can be tried without
hardware.

```bash
massimo discover
massimo portal-crypto iv -addr <cube-ip>
```

```bash
massimo portal -listen :80 -probe default -probe-target 1514b6:6 -probe-cube 192.168.1.46
```

Probe mode answers each poll with the next reply variant (an `s:` radio
command in different forms) and judges it by the Cube's free memory
slots in the following poll: an accepted radio command occupies a slot.
With `-probe-cube` the portal sends `i:` over the LAN after each reply so
the next poll comes within seconds. `-probe <file>` takes custom
variants, one `name=text` per line with `\r\n` escapes and the
placeholders `{i}`, `{s}`, `{shex}`, `{r}`, `{serial}`, `{rf}`, `{fw}`.

`discover` lists the Cubes on the LAN (serial, IP, MAC, RF address,
firmware) via the UDP broadcast the original software uses; it needs UDP
port 23272 and a real LAN interface (not WSL 2). `portal-crypto iv`
recovers a Cube's portal IV from one `e:` exchange.

Several Cubes: put `MASSIMO_CUBE_<SERIAL>_KEY`, `MASSIMO_CUBE_<SERIAL>_IV`
and `MASSIMO_CUBE_<SERIAL>_ADDRESS` into `.env` and select one with
`-sn <serial>` (default `MASSIMO_CUBE_SN`); without an address the Cube
is discovered on the LAN. With `-addr`, the serial the Cube reports
selects its key. `-env <file>` picks another env file. `massimo portal`
serves every Cube that has a key and IV configured.

`MASSIMO_CUBE_<SERIAL>_PROXY` (or `MASSIMO_CUBE_PROXY`) is the address
of a running `massimo serve` that answers as that Cube; every command
except `serve` tries it first, with a two-second timeout, and falls
back to the Cube's own address, because the Cube serves one client at
a time and `serve` usually holds that connection. `-addr` always wins.
`MASSIMO_CUBE_<SERIAL>_PROXY_SERIAL` (or `MASSIMO_CUBE_PROXY_SERIAL`)
is the serial `serve` uses in discovery replies, see
[docs/proxy.md](docs/proxy.md). `serve -sn <serial>` serves that one
Cube instead of every configured one.

`MASSIMO_CUBE_<SERIAL>_IGNORE` (or `MASSIMO_CUBE_IGNORE`) lists radio
addresses, comma separated, that `serve`, `status` and `backup` leave
out together with any room that holds nothing else, that `serve` also
hides from every client of its proxy (so one list on the server
suffices), and that `restore` never touches (`-no-ignore` before the command name switches the list
off for one run, for example to edit the parking room with a backup and
a restore): a device that is paired but never reports blocks every
command to the room the Cube counts it in, and the Cube never drops it,
so the way out is to park it in a room of its own on the Cube and hide
it here (see docs/portal-protocol.md). A restore keeps such devices and
their room exactly as the Cube has them.

## Server

```bash
massimo serve -listen 127.0.0.1:8099
```

keeps every Cube from the environment connected (one connection and one
command queue per Cube) and serves a JSON API: `/api/v1/cubes`,
`/api/v1/cubes/{serial}` (with rooms), `/devices`, `/devices/{rf}`,
`PUT …/devices/{rf}/setpoint` and `PUT …/rooms/{id}/setpoint` with
`{"mode":"manual","temp":21.5}`, `{"mode":"auto"}` or
`{"mode":"vacation","temp":17,"until":"2026-11-10 12:00"}`,
`PUT …/devices/{rf}/lock` with `{"locked":false}` for the key lock,
`/backup?format=yaml`, and `/events` as server-sent events; see
[docs/api-v1.md](docs/api-v1.md) and the OpenAPI description in
[docs/api-v1-openapi.yaml](docs/api-v1-openapi.yaml). Set
`MASSIMO_API_TOKEN` (or `-token`) to require `Authorization: Bearer`.
The listener defaults to localhost; put a reverse proxy with TLS in
front before exposing it.

The Cube serves only one LAN client at a time, so `serve` is itself a
Cube for everyone else: it listens on port 62910 on every interface
(`-proxy <addr>[:<port>]`, repeatable, `-proxy off` to disable) and
answers UDP discovery. Every other massimo command and the original
software connect to the host running `serve` (`-addr <serve host>`)
and work beside it: reads are answered from the server's copy, writes
go through its queue to the Cube. The original software keeps only one
discovery answer per serial, and the real Cube answers first, so give
the server a different serial for discovery with `-proxy-serial`, for
example the real one with the first letter changed, or block the Cube's
UDP answers on the PC's firewall. See [docs/proxy.md](docs/proxy.md).

## Original software

```bash
massimo legacy -out max-session.log
```

starts the original MAX! software (Java 8, installation directory
`C:\ProgramData\MAX` by default, `-dir` otherwise) and prints only the
protocol-relevant lines: API and gateway calls, radiograms, send results.
The software never prints the LAN lines it writes, not even at debug
level, so `legacy` reconstructs them from the logged gateway calls
(`s:`, `t:`, `m:`, `z:`, `c:`, `l:`, `n:`, `x:`, `q:`) and prints each
beneath its call, marked `lan (derived)`. `-out` saves the complete
output, `-all` shows it, `-width` shortens long lines. With `serve`
running, point the software at the serve host rather than the Cube: in
its Cube list pick the entry with the serve host's address (started
with `-proxy-serial`, it appears under that serial).
