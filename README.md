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
differences, respecting the radio duty cycle. Pairing cannot be restored
over the LAN: devices missing from the Cube are listed, and the restore can
be repeated after they have been paired again.

```bash
massimo watch -addr <cube-ip>
massimo fakecube -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt
```

`watch` prints state changes as they happen; `fakecube` serves recorded
data on port 62910 so everything can be tried without hardware.

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
