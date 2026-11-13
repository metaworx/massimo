# Massimo and Home Assistant

How Home Assistant reads and controls the MAX! heating through Massimo:
today with Home Assistant's own MAX! integration and Massimo's LAN proxy,
and later with an integration of Massimo's own (AP HomeAssistant).
Massimo stays where the Cube is handled:
the radio queue and its duty cycle, pairing, week programmes, backups;
Home Assistant is one more client, beside the web UI, the apps and Loxone.

## Today: Home Assistant's MAX! integration through the proxy

Home Assistant ships an integration for the MAX! Cube, `maxcube`
(`eQ-3 MAX!`), configured in YAML only.
It speaks the Cube's LAN protocol on port 62910,
and Massimo's LAN proxy answers it as a Cube ([proxy.md](proxy.md)):
Home Assistant never holds the Cube,
and every write it makes goes through Massimo's queue.

In Home Assistant's `configuration.yaml`, then restart Home Assistant:

```yaml
maxcube:
  gateways:
    - host: 192.168.1.44   # the proxy's address: MASSIMO_CUBE_<SERIAL>_PROXY
      port: 62910
      scan_interval: 60    # the proxy answers from Massimo's copy
```

- **host** is the address the proxy listens on:
  the Cube's `MASSIMO_CUBE_<SERIAL>_PROXY`,
  or any address of the server's host for a single Cube without one;
  `127.0.0.1` when Home Assistant runs on the same host with the host's
  network.
- **scan_interval** may be short:
  the proxy answers the poll from Massimo's copy of the Cube (`l:`),
  so a poll costs no radio time; the default is 300 s.
- Each thermostat becomes a `climate` entity,
  each window contact and each battery a `binary_sensor`.
  Devices hidden in Massimo stay hidden (proxy.md, "Hidden devices").
- A setpoint set in Home Assistant is a radio write through Massimo's queue,
  about 1.25 % of the hour's duty cycle, as it is from the web UI.
- Not there: week programmes, Massimo's rooms as one entity,
  pushed changes (Home Assistant polls), Massimo's users and roles
  (the Cube protocol has no sign-in, so the proxy's port belongs to the LAN only).

## Coming: Massimo's own integration

A repository of its own, `metaworx/massimo-homeassistant`
(the integration and its card, Apache-2.0), not published yet.
It talks to Massimo's HTTP API, signed in with an API key:

- a `climate` entity per room, as the web UI's tiles are,
  with presets for the room's eco and comfort temperatures, and boost
  (`{"mode": "boost"}` on the room's setpoint);
- per device, sensors for the valve, the battery, the link and the window;
  the Cube's connection, duty cycle and free memory slots as diagnostics;
- services to write a room's week programme, copy it, and lock thermostats;
- a card that edits the week programme with Massimo's own editor.

What Massimo provides for it, and for every other integration:

- **An API key under the role `integration`**,
  which reads, sets, locks and configures
  ([store.md](store.md) section 6; [api-v1.md](api-v1.md), "Authentication").
  Create a user with that role and a key for it in System settings → Users,
  or on the server:
  ```bash
  massimo user add homeassistant -role integration
  ```
- **The event stream with ids** (api-v1.md, "GET …/events"):
  pushed changes, and after a reconnection the events since the last one read,
  or `resync` where the server no longer holds them.
- **The week programme editor as an element**, `<massimo-week-program>`
  (`internal/server/ui/week-program.js`; [ui.md](ui.md)),
  which builds its controls with a kit its host gives it,
  so that the card shows Massimo's editor in Home Assistant's look.

Until it exists, the MAX! integration above covers daily control,
and Massimo's own web UI the week programmes.
