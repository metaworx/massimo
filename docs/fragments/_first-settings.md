> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# The first settings

On the Cubes' LAN the server needs no settings to begin with:
while there are no users,
the web UI, `http://<the box>:8099/`,
asks whoever opens it from the local network to create the first owner,
in its sign-in card at each load:
**Skip for now** shows the page without an owner until the next load,
and **Keep it open…**, once the page's address is entered there,
stops the asking and leaves the server open to the local network;
and System settings → Cubes lists the Cubes the server finds,
each added with **Add**.
The settings take what discovery and the web UI cannot give:
the address of a Cube that discovery does not reach,
on another subnet or behind a VPN (`massimo discover` lists the Cubes it hears,
with serial and address),
and a token that becomes the owner's key at the first start,
which the API and the web UI then ask for,
so that the web UI asks nobody on the LAN for the first owner:

```
MASSIMO_CUBE_KEQ0000000_ADDRESS=192.168.1.10
MASSIMO_API_TOKEN=choose-a-long-random-string
```
