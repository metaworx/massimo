# NOTE CodeqlAlerts v1.0: how the run went

Blocks 1 to 4 ran under the owner's `PLAN++` of 2026-11-08,
the fallback below under `EXEC all` the same day.
Where it went differently from the plan:

- **Block 2's fix did not satisfy CodeQL.**
  Naming `m`, `t` and `p` before the `switch` left the alert in place,
  as a new one at the conversion's new line:
  CodeQL does not take the bound in a tagless `switch` case for a guard,
  whether on an array's element or a variable.
  The plan's fallback followed, in a commit of its own on the same branch:
  each value is parsed at the size it is returned in, the lanes at 8 bits,
  so that no conversion needs a bound.
  A `p=` past 255 is now malformed ("want m=…") rather than over the
  ceiling ("is over …p=16"); both are `ErrBadMessage`.
  `TestParseKSFLanes` holds 16, 17 and 256.
- **`CI.md` §8 says a little more than the plan named**:
  when CodeQL runs (pushes to `main`, pull requests, weekly),
  and that a false positive the code cannot be made plainer for is
  dismissed as "false positive".
- **No commit-msg hook** in this checkout, so `commit-msg.sh` ran by hand;
  it read "7fffffff" in Block 1's message as a commit hash,
  and the message was reworded before the push.
