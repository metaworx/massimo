# NOTE HomeAssistant v1.0: block 5 repository

How Block 5 went, under the owner's `EXEC 5` with D7-A:
the repository `metaworx/massimo-homeassistant`, public, Apache-2.0,
its first two commits on its `main`, "feat(integration):
Set up a Massimo server and follow its Cubes' event streams" and "ci: hassfest,
HACS, ruff and the tests against Massimo's main";
its first CI run passed all four jobs.

## What it holds

- `custom_components/massimo`:
  the API client (aiohttp alone, the event stream with its ids and resume),
  the config flow (address and key, mDNS `_massimo._tcp`, re-authentication,
  the server known by its node id), and the coordinator (reads the server once,
  then each Cube's stream, resuming after the last id,
  reading the rooms again after a change of rooms or a `resync`).
  No entities yet: Blocks 6 to 8.
- Tests against a real `massimo serve` and fake Cube (12), no mocks;
  the CI builds `massimo` from this repository's `main`,
  or the ref a manual run names.

## Where it differs from the plan

- **Events in the tests come from a rename.**
  The fake Cube acknowledges a setpoint but changes no state,
  so no state event follows one; the tests rename a room instead,
  a metadata write the server announces.
  The coordinator's handling of a state event is tested directly.
- **The tests skip what an older Massimo cannot do**:
  event ids (Block 1) and `configure` for `integration` (Block 2) are detected
  and skipped where absent, so the CI against `main` passes (10 passed,
  2 skipped) until #154 is merged, and against this plan's branch all 12 pass.
- **Home Assistant 2026.10.4 in the tests.**
  The test harness (pytest-homeassistant-custom-component 0.13.375) has no
  release for 2026.11 yet; the integration targets 2026.10 and newer, and its
  setup returns `True`, as 2026.10 wants and 2026.11 accepts.
- **HACS's check ignores `brands` and `topics`** until the release (Block 9):
  no brand images and no repository topics yet.

## Found on the way

- **#154's armv6 job** failed once in `TestSilentCubeIsLostAfterTwoPolls` under
  QEMU; main passes it on every run, the test passed 30 of 30 natively,
  and the branch's next run passed it.
  A timing flake under emulation.
- **#154's Screenshots job** then failed on the UI suite's ordering fault
  between `test_tls.py` and `test_login.py` (NOTE HomeAssistant v1.0: blocks 1
  to 4) and passed on its rerun.
