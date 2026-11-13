# Background: the evaluation before the project, and the routes to a public server

Two texts from before the first line of code,
kept as the reasoning behind docs/design.md.
Dates and facts are as of the evaluation (autumn 2026);
what the project found out since is in docs/cube-protocol.md.

## Part 1: options for the eQ-3 MAX! Cube after end of life

Short answer at the time: do not build a replacement for the eQ-3 cloud server.
Build one small bridge on the local network that keeps the Cube's single
connection open and offers MQTT and HTTP to everything else; with Loxone,
that bridge gives programming in Loxone Config, a web UI and a mobile app.

### Background

- eQ-3 discontinued MAX! in favour of Homematic IP and ran the cloud until 1 July 2023,
  with an exchange offer for moving to Homematic IP.
- Local control still works:
  the Cube speaks a reverse-engineered text protocol over TCP port 62910,
  with no cloud involved.
  Every existing open-source tool uses this.
- The Cube accepts only one TCP client at a time.
  One process must own the connection,
  and everything else talks to that process.
  (Confirmed on hardware; it is why `massimo serve` answers as a Cube itself,
  see docs/proxy.md.)
- The Cube can lose its configuration if clients connect and disconnect too often;
  a persistent connection avoids this.
  The 868 MHz radio has a legal limit on transmit time;
  the Cube reports how much is left and too many commands get blocked.

### Option 1: a replacement server for web and mobile

Redirecting the Cube to another server is easy:
the `u:` command sets the server address.
The hard part is the Cube↔server protocol, which is not publicly documented;
no project emulates it, and the early notes mention AES commands on the Cube.
Even then the official app is gone, so a new web UI and app are needed anyway.
The better version of the same goal is a local service with a persistent connection,
a REST API, MQTT and an installable web app, with a VPN for access from outside.

### Option 2: Loxone integration

| Approach | Feasibility | Notes |
|---|---|---|
| Loxone virtual inputs/outputs only | no | Messages are base64-encoded binary; the Miniserver's text matching cannot read them |
| Pico-C program block on the Miniserver | possible | Pico-C can open TCP connections; hard to debug, limited, fragile |
| LoxBerry plugin or a small service plus MQTT | recommended | The bridge owns the Cube connection, sends status to the Miniserver over UDP/HTTP or the LoxBerry MQTT Gateway, receives setpoints and modes from Loxone. No existing LoxBerry plugin for MAX! was found |

Design points: decide who controls the heating (Loxone's room controller,
or the thermostats' own schedules with Loxone reading and overriding);
rate-limit commands because of the duty cycle;
remote access comes free with the Loxone app.

### Option 3: alternatives

1. Replace the Cube with a radio stick (CUL/nanoCUL with FHEM or
   Homegear) and keep the thermostats; they have to be paired again.
   An archived project reflashes the Cube itself to act as a stick.
2. Use a home automation platform as the bridge:
   openHAB's MAX! binding (mature),
   Home Assistant's core integration (no weekly schedules), FHEM's modules;
   connect to Loxone over MQTT or UDP/HTTP.
3. Replace the hardware gradually: Homematic IP, Loxone Air, or Zigbee
   thermostats via Zigbee2MQTT, once the ten-year-old hardware fails.

### Option 4: existing solutions

Protocol documentation: Bouni/max-cube-protocol (most complete, includes `u:`).
Platforms: openHAB, Home Assistant `maxcube`, FHEM, Gladys, ioBroker.
Bridges and tools: `maxcube-mqtt`, Juerd/eq3-max.
Mobile: the "MAX!
Remote" Android app reportedly talks to the Cube directly.
Loxone: no ready-made plugin, only forum threads.

### Recommendation at the time

Quick win: `maxcube-mqtt` or openHAB on a LoxBerry or Pi with the MQTT Gateway.
Reusable product: a proper LoxBerry plugin with discovery, template export,
Cube backup, duty-cycle-aware queuing and a web UI.
Skip the server replacement unless the reverse engineering is interesting for
its own sake.
(The project then chose to do both the bridge and the research;
the research found that the Cube ignores commands in portal replies,
see docs/cube-protocol.md.)

Sources:
- [eQ-3: Wechselangebot von MAX! zu Homematic IP (cloud until
  01.07.2023)](https://stadt-bremerhaven.de/eq-3-wechselangebot-von-max-zu-homematic-ip/)
- [Bouni/max-cube-protocol](https://github.com/Bouni/max-cube-protocol) and its
  [U-Message (portal
  URL)](https://github.com/Bouni/max-cube-protocol/blob/master/U-Message.md)
- [schrankmonster: reverse-engineering the ELV MAX!
  Cube
  protocol](https://www.schrankmonster.de/2012/08/17/reverse-engineering-the-elv-max-cube-protocol/)
- [Home Assistant eQ-3 MAX!
  integration](https://www.home-assistant.io/integrations/maxcube/)
- [openHAB MAX! binding](https://v51.openhab.org/addons/bindings/max/)
- [maxcube-mqtt (PyPI)](https://pypi.org/project/maxcube-mqtt)
- [Gladys MAX!
  Cube integration](https://gladysassistant.com/docs/integrations/external/max-cube/)
- [Juerd/eq3-max](https://github.com/Juerd/eq3-max)
- [dktzde/maxcube (Cube → a-culfw)](https://github.com/dktzde/maxcube)
- [Loxforum: MAX! eQ-3 Thermostatregelung in
  Loxone](https://www.loxforum.com/forum/faqs-tutorials-howto-s/144183-max-eq3-thermostatregelung-in-loxone)
- [MAX!
  Remote FAQ](https://jutzig.github.io/max-remote/pages/faq.html)
- [python-maxcube-api issue (connection
  handling)](https://github.com/uebelack/python-maxcube-api/issues/12)

## Part 2: prerequisites of the public server, licence, and the two routes

Short answers: the radio stick (phase 3) is not a prerequisite for the public server;
a local device is, for now, the only reliable route.

### AGPL

AGPL fits: whoever runs a modified version as a hosted service must publish the source.
Two things to keep in mind:
a contributor agreement or a clear "everything stays AGPL" rule,
with billing as a separate service; and AGPL-compatible dependencies (MIT, BSD,
Apache-2.0 are fine).
The project is AGPL-3.0-or-later.

### Phase 3 is not a prerequisite

The public server needs the `remote` driver plus the edge agent from phase 4,
which work with whichever local driver is installed.
Order 0 → 1 → 2 → 4 works;
phase 3 matters for the business (independence from ageing Cubes,
signal strength, pairing in the app), not for the technology.
A stick still needs a host,
and a flashed Cube on TCP 2323 has no authentication either.

### Route A: edge agent on a local device (start with this)

Works with every Cube and firmware, TLS-encrypted, on any always-on device:
LoxBerry or Raspberry Pi, a NAS as a container, a Windows service,
an OpenWrt router (Go cross-compiles),
or a small preconfigured box sold with the service.
The Miniserver itself cannot host it.

### Route B: the Cube connects to the server directly

`u:` or a DNS override points the Cube at the server, as the old portal worked.
The 2016 ERNW analysis found HTTP POST with only the body encrypted (AES),
no TLS, the key never sent, firmware files encrypted and decrypted on the Cube.
What makes it hard: nothing is left to listen in on since July 2023;
the key and the message format have to come from the firmware or old recordings;
the security is only as good as the Cube's fixed key;
local access may be blocked while the Cube is in server mode;
the one-time setup still happens locally.
Cheap first step: point a test Cube at a listener and record what it sends.
(Done: see docs/cube-protocol.md.
The Cube's `e:`/`d:` commands do decrypt and encrypt with the portal key,
the fake portal decodes every POST,
but the reply format stays unknown and the Cube ignored every reply line tried;
the firmware dump is the remaining lead,
its procedure on the branch `archive/portal`.)

### Recommendation

Plan the public server around the edge agent;
prepare a `portal` driver next to `remote` behind the same gateway interface so
that Route B becomes an agentless option with a stated security level if the
research succeeds;
stop the research track if it stalls after the decryption-helper step.
(It did not stall there; it stalled at the reply format,
hence the firmware route.)

Sources:
- [ERNW/Insinuator: Discover the Unknown – Analyzing an IoT Device (2016, MAX!
  Cube)](https://insinuator.net/2016/04/discover-the-unknown-analyzing-an-iot-device/)
- [Bouni/max-cube-protocol –
  protocol.md](https://github.com/Bouni/max-cube-protocol/blob/master/protocol.md)
- [schrankmonster: reverse-engineering the ELV MAX!
  Cube protocol (AES
  commands)](https://www.schrankmonster.de/2012/08/17/reverse-engineering-the-elv-max-cube-protocol/)
- [FHEM users: MAX!
  Cube LAN Gateway oder besser
  CUL](https://groups.google.com/d/topic/fhem-users/7P1TTQc65WA)
- [Dmitry Kazakov: MAX! cube home
  automation](https://www.dmitry-kazakov.de/ada/max_home_automation.htm)
- [dktzde/maxcube](https://github.com/dktzde/maxcube)
