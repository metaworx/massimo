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

## Preparing a Raspberry Pi 1

The current Raspberry Pi OS Lite (32-bit) still runs on the Pi 1 and needs
about 2.8 GB, so a 4 GB card is enough. Raspberry Pi Imager can preset the
user, SSH key and hostname. When writing the image by hand, the boot
partition's `user-data` (cloud-init) does the same on first boot, for
example:

```yaml
#cloud-config
hostname: massimo
users:
- name: pi
  groups: users,adm,sudo,netdev
  shell: /bin/bash
  sudo: ALL=(ALL) NOPASSWD:ALL
  ssh_authorized_keys:
  - ssh-ed25519 AAAA... you@host
enable_ssh: true
ssh_pwauth: false
```

The Pi 1 has no network over USB (its micro-USB port only powers it), so
it needs Ethernet. The first boot takes about ten minutes.

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
  Cube, one of the box's own IP addresses. Each Cube needs its own
  address, because port 62910 can be bound once per address. Keeping the
  box's own address free and adding one extra address per Cube is the
  tidiest setup; addresses outside the router's DHCP range, on Raspberry
  Pi OS (NetworkManager):

  ```bash
  sudo nmcli con mod "Wired connection 1" +ipv4.addresses 192.168.1.49/24 +ipv4.addresses 192.168.1.50/24
  sudo nmcli device reapply eth0
  ```

  For a first run without the proxy, add `-proxy off` to `ExecStart`
  (`sudo systemctl edit --full massimo`).
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

## Moving to another network

- Cubes ship with DHCP on (factory fallback 192.168.0.222/16), so a Cube
  gets an address from the new router. Reserve it there, because
  `MASSIMO_CUBE_<SN>_ADDRESS` is fixed; then update that variable and
  `sudo systemctl restart massimo`.
- The fixed proxy addresses added with `nmcli` stay on the interface when
  the box moves. On the new network they must be free and outside the
  router's DHCP range: remove them with `-ipv4.addresses` and add new ones
  as above. The address from DHCP itself follows the new router.
- A Cube that was moved may still answer ping but not TCP 62910 or UDP
  23272 until it is power-cycled.
- A Cube that already has a client (for example this server) accepts no
  second one and drops the connection attempt silently, so a test
  connection from elsewhere times out instead of being refused. Test
  through the proxy instead: `massimo status -addr <proxy address>`.

## Updating

```bash
sudo install -m 0755 massimo-linux-armv6 /usr/local/bin/massimo
sudo systemctl restart massimo
```

## Resource use

Measured on a Raspberry Pi 1 Model B (BCM2835, 700 MHz, 512 MB of which
426 MB are left to Linux) with Raspberry Pi OS Lite 32-bit (trixie),
serving two Cubes with about 30 devices, the API, the web UI and two proxy
listeners:

- `massimo` resident memory: 7 MB after start, 9 MB after five minutes
  (`VmRSS` in `/proc/<pid>/status`; the Pi kernel has memory accounting
  off, so `systemctl status` shows no memory line).
- CPU: under 1 % on average, about 1 s of CPU time for the first five
  minutes; both Cubes connected within a second of starting.
- The whole system uses about 110 MB (`free -m`), leaving over 300 MB
  available; the root filesystem needs 2.1 GB.
- The web UI's page loads in about 0.25 s over the 100 Mbit Ethernet.
