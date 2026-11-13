# NOTE PortalDrop v1.0: how the run went

Where the run of `EXEC+push+PR` of 2026-11-13 went otherwise than the plan says.

1. **D34-A**, taken by the signal, which the gate said runs with it:
   the roadmap's item 10 stays, its procedure and the portal code named on `archive/portal`.
2. **The findings kept whole.**
   The plan shrank "Recovering the IV from a Cube" and the key and IV parts of "Transport"
   to a pointer; they are what the Cube does, findings the contract keeps,
   so they stay as they were, and only the removed commands in them,
   `portal-crypto iv` and the fake portal, now name the branch `archive/portal`.
3. **`archive/portal`** is at the plan's registration on main,
   which holds the code and `docs/firmware-extraction.md` as they were.
4. **A test timed out once under the full suite**,
   `TestCubeConfiguredBySerialFollowsItsAddress` ("timed out waiting for the cube at 127.0.0.3"),
   in the run that preceded Block 2's commit, which changes no code;
   five runs of it alone and the full suite after were green.
   It may be the failure whose name AP Oidc v1.1's run lost.
