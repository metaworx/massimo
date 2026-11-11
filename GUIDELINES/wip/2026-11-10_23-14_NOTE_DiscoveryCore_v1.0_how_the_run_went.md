# NOTE DiscoveryCore v1.0: how the run went

Where the run of Blocks 1 to 3, under the owner's `PLAN++`,
went differently from the plan's steps.
The blocks themselves stand.

## Block 1: OneSearch

- **Three exported pieces went with the old loop.**
  `IdentifyRequest`, `ConfigRequest` and `ParseConfigReply` in
  `internal/cubelan/scan.go` existed only for the responder's own loop,
  so they left with it.
  `TestConfigRequestAndReply` keeps the captured `c` reply of a Cube with
  firmware 1.1.3 and checks it through the parsers `Search` uses.
- **Two tests changed, though step 7 said they would pass unchanged.**
  `TestQueryConfigReadsTheDHCPFlag` drove the removed `queryConfig`,
  so its point, the DHCP flag from `c`, is now part of `TestSearchMarksTheAnswers`.
  `TestMarkProxies` built answers from a MAC alone;
  it now sets the kinds the search gives,
  and checks that an answer taken for a Cube's stays one at a server's address.
- **A short wait lasts a second with a proxy among the answers.**
  An answer without `N` and `c` gets the second the late wait gives,
  as the responder's `c` did before;
  with the default three seconds a proxy that answers early adds nothing.
  The tests with waits of a few hundred milliseconds take about a second more.

## Block 2: StickyFollow

- **A Cube without an address takes an answer the search could not tell.**
  D11-A says such an answer is followed only while it is the current address;
  a Cube configured by serial that has no address yet has nothing to keep,
  and before this plan took any answer.
  It now takes one only when no answer is marked a proxy's or a Cube's.
- **The other answers' addresses came with Block 3's field.**
  Step 1 kept them on the entry already; they are written with the
  `proxies` field instead, where something reads them.
- **The LAN test's version.**
  `tools/lan-test/run.sh` went to 1.3.0 with check 4's new form,
  and to 1.4.0 in Block 3, as its last change bumped it.

## Block 3: ShownProxies

- **The UI test checks the proxy on a live event.**
  `test_cube_found_live` sends its `cube_found` with a proxy and finds
  "via proxy at …" on the line;
  `test_search_held_then_answered` finds the fake Cube's gateway from its
  answer to `N`, in the expert part's text.
