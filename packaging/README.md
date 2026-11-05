# Running `massimo serve` as a service

`massimo serve` is meant to run all the time on a small always-on Linux box
next to the Cubes, such as a Raspberry Pi. This directory has what a manual
install needs; a future `massimo service install` (roadmap item 8) will do
the same steps.

- `systemd/massimo.service`: the unit. Runs as the user `massimo`, reads
  its settings from `/etc/massimo/env`, listens on port 8099 on every
  interface (API and web UI), logs to `/var/log/massimo/serve.log` and the
  journal, and restarts on failure.
- `logrotate/massimo`: weekly rotation of that log.

Each Cube accepts exactly one TCP client. Only one `massimo serve` can be
connected to a Cube at a time, so stop any other server (and close the
original MAX! software) before starting this one; afterwards, clients
connect to the server's proxy instead of the Cube.

## Build

Pick the binary for the board (all need a 32-bit or 64-bit Linux to match):

| Board                                  | OS             | Target              |
|----------------------------------------|----------------|---------------------|
| Raspberry Pi 1 (A, B, B+), Zero, Zero W | 32-bit         | `make linux-armv6`  |
| Raspberry Pi 2, 3, 4, Zero 2 W         | 32-bit         | `make linux-armv7`  |
| Raspberry Pi 3, 4, 5, Zero 2 W         | 64-bit         | `make linux-arm64`  |
| PC, NAS                                | 64-bit         | `make linux`        |

`uname -m` on the box says which: `armv6l` needs the armv6 build (the armv7
one dies there with "illegal instruction"), `armv7l` the armv7 build,
`aarch64` the arm64 build.

## Install by hand

From the build machine (here for a Pi 1 at `pi.local`, user `pi`):

```bash
make linux-armv6
scp dist/massimo-linux-armv6 packaging/systemd/massimo.service packaging/logrotate/massimo pi@pi.local:
```

On the Pi:

```bash
sudo install -m 0755 massimo-linux-armv6 /usr/local/bin/massimo
massimo version
sudo useradd --system --home-dir /var/lib/massimo --no-create-home --shell /usr/sbin/nologin massimo
sudo install -d -m 0750 -o massimo -g massimo /etc/massimo
sudo install -m 0644 massimo.service /etc/systemd/system/massimo.service
sudo install -m 0644 massimo /etc/logrotate.d/massimo
```

Then write `/etc/massimo/env` (same format as the `.env` described in the
main README) and make it readable only by the service:

```bash
sudo install -m 0600 -o massimo -g massimo /dev/null /etc/massimo/env
sudoedit /etc/massimo/env
```

What to set there:

- `MASSIMO_CUBE_<SN>_ADDRESS`, `_KEY`, `_IV` for every Cube.
- `MASSIMO_CUBE_<SN>_PROXY`: the address this box answers on for that
  Cube, i.e. one of the box's own IP addresses. With several Cubes each
  needs its own address (port 62910 and discovery can be bound once per
  address), so give the box a second address, or start with `-proxy off`
  added to `ExecStart` (`sudo systemctl edit --full massimo`).
- `MASSIMO_CUBE_<SN>_PROXY_SERIAL` if the original software should see a
  serial other than the Cube's.
- `MASSIMO_API_TOKEN` to require a Bearer token on the API.
- `MASSIMO_LOG=/var/log/massimo/serve.log`, so that `massimo` commands run
  with `-env /etc/massimo/env` agree with the unit (which passes `-log`).

Check the settings and that the Pi reaches a Cube before enabling the
service (with no other server connected to that Cube):

```bash
sudo -u massimo massimo -env /etc/massimo/env config
sudo -u massimo massimo -env /etc/massimo/env status -sn <serial>
```

Start it and follow the log:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now massimo
journalctl -u massimo -f
```

From another machine:

```bash
curl -H "Authorization: Bearer <token>" http://<box>:8099/api/v1/cubes
massimo status -addr <proxy address>
```

and the web UI is at `http://<box>:8099/`.

## Updating

```bash
sudo install -m 0755 massimo-linux-armv6 /usr/local/bin/massimo
sudo systemctl restart massimo
```

## Resource use

MEASUREMENTS
