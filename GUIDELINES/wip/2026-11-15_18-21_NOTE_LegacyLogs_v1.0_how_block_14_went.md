# NOTE LegacyLogs v1.0: how block 14 went

2026-11-15, the run of `EXEC++ Block 14` (branch `claude/legacy-logs-network`). Where the work went differently from the plan:

## Where the UI says it

- **"Does not answer at its addresses" is not a Cube's text.** `connect.unreachable` is the app's list of Massimo servers (`connect.js`), not a Cube's. The analysis of 04-58 named it as the place where a first-time user gets stuck.
- **The hint goes where a Cube is not connected instead:**
  - the found Cube's row in System settings → Cubes, as the plan has it;
  - a served Cube's panel, above its facts, while it is not connected;
  - Rooms' notice for a Cube not connected, in place of the usual checks.
- **Not in the header's pill.** Its found group keeps its short entries. The row it links to has the hint.

## What the API marks

- **A served Cube has `other_network` too,** by the address it connects to. After **Add**, the Cube leaves the found list, and the panel and Rooms then need the field.
- **The check is by this machine's interface networks only.** A Cube reached through a router, such as one found with `-discover-target`, is marked as well. So the text says that the server reaches it only through a router, not that it cannot reach it. On a served Cube the UI shows it only while the Cube is not connected.
- **A proxy is never marked,** and a host name is not judged.
- **The networks are read again at most every 30 s,** since every listing of the Cubes asks them.

## The netconfig line

- **It carries the Cube's serial and placeholders:** `massimo netconfig -sn <serial> -broadcast -static <ip> -gateway <gateway> -netmask <netmask>`. The text first says to check the router's DHCP and restart the Cube.

## The checks

- **`test_search_held_then_answered` at 1100 px in English failed now and then on its first pass** when the whole discovery file ran, at "and still there once everything has come", and passed on its retry. It fails the same way with main's UI files swapped in by a build overlay, and passed 12 times alone with both. It is older than this block.
- **`make lan-test` passed.** It has no case for another network, since its containers share one network.
