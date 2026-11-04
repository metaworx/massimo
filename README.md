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

`discover` lists the Cubes on the LAN (serial, IP, MAC, RF address,
firmware) via the UDP broadcast the original software uses; it needs UDP
port 23272 and a real LAN interface (not WSL 2). `portal-crypto iv`
recovers a Cube's portal IV from one `e:` exchange.

Several Cubes: put `MASSIMO_CUBE_KEY_<SERIAL>`, `MASSIMO_CUBE_IV_<SERIAL>`
and `MASSIMO_CUBE_ADDRESS_<SERIAL>` into `.env` and select one with
`-sn <serial>` (or `MASSIMO_CUBE_SN`); `-env <file>` picks another env
file. `massimo portal` serves every Cube that has a key and IV configured.
