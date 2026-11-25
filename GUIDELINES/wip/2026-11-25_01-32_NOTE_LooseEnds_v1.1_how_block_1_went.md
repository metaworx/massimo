# NOTE LooseEnds v1.1: how block 1 went

Where Block 1, Open, went differently from the plan,
the choices taken in the run, and what verified it.
It grew from one commit to four:
its first reading found a hole that the block itself would have opened.

## What it made

- **A server is open while it has no regular user**
  (D1-A): a user that is not an integration's, a disabled one still counting.
  One rule in the store says it, and the count, the first owner's check,
  the keep-open setting's write and its sweep at a start all use it.
  An integration's users no longer close a server,
  block the first owner or remove keep open.
- **The trap of the walk is gone:**
  an integration approved on an open server leaves it open,
  and the first owner can still be made.
  It is held as a test at the API,
  and was seen on a real open server with the walk's own script.
- **A server whose only users are an integration's is open after the upgrade**
  (D2-A).
  Its log warns at each start, with their number,
  unless it is kept open on purpose.
- **An integration's user acts through its integration alone.**
  It gets no key, no password, no provider's link and no sign-in of its own:
  the store refuses each, and the users' routes answer 403.
  Whatever key it comes by,
  it is held to the role `integration` and its integration's maximum,
  and refused where its integration is disabled, revoked or gone.
  It is not counted as the last owner.
- **Users and My profile** offer such a user no password, new key or link,
  and say in a line that it acts through its integration.
  A key it still holds stays listed with Revoke;
  a password it still holds can be removed.
- **The settings file's token** goes to the first regular owner at a start,
  and where an older start gave it to an integration's user,
  it moves to the owner.
- **Four commits:**
  "fix(security:server): An integration's user gets no key,
  password or link of its own", "fix(server):
  An integration's users leave an open server open", "fix(ui):
  An integration's user is offered no password, key or link in Users",
  "fix(store):
  The settings file's key moves from an integration's user to the owner".

## The hole the first reading found

Before this block an open server never had an integration's users:
the approval closed it.
With D1 the two exist together,
and on such a server anyone on the local network, without a token,
could give an integration's user the role owner and a key of its own.
That key outlived the first owner's creation, at an owner's rank, from anywhere.
A reader showed it with a probe.
The second commit of the plan became the first of the block, the hardening,
so that no commit on main has the opening without it.

## Different from the plan

Each decision taken in the run, the owner's to change:

1. **The hardening, not in the plan** (above).
   The other way: none that keeps D1.
2. **Every sign-in as an integration's user is refused,** not only the making of
   its credentials.
   The readers checked that nothing legitimate goes that way:
   an integration's own key was refused a session before,
   the event stream and the on-behalf chain make none,
   and the app uses a regular user's key.
3. **A key such a user holds from before this version keeps working,
   under the ceiling.**
   The builder first left it its whole role;
   that was not consistent with a password that no longer signs in.
   A password it holds from before signs in nowhere and can be removed in Users.
   The other way: revoke such keys at the start, with a warning.
4. **The warning at a start is silent while the server is kept open:**
   a server kept open on purpose, with its integration,
   is not told at every start to create the owner.
5. **A warning when a running server turns open:**
   where the removal of a regular user leaves none,
   the log says who removed whom.
   The routes cannot make that state (the last owner without an end date stays);
   an owner with an end date, made on the box or by an older version,
   can remove itself.
6. **The page follows the server** (the third commit, not in the plan):
   the Users card offered what the server now refuses.
7. **The settings file's key is moved, not revoked** (the fourth commit,
   not in the plan).
   The builder found the case when the block met AP MobileApp's #418:
   on a store an older version left, with the key on an integration's user,
   a start would have made the user `owner`, failed on the key's unique hash,
   and left a closed server nobody could get into.
   A revoke does not free the hash, so the row moves to the owner.
8. **D40-A as taken:**
   on an open server an integration's key is not looked at.
   `/api/v1/me` with it says `open` without `integration`,
   nothing is attributed, and from beyond the local network the answer is 403.
   It is said in the API's document where an integration's author reads,
   and held by a test.
9. **D42-A as taken:**
   "No users yet" stays the page's and the API's wording on an open server that
   has an integration.
10. **The changelog:**
    the opening's bullet is under Fixed, as its commit's type asks,
    where the plan had Changed,
    and tells the owner of a server that opens at the upgrade what to do;
    the hardening's is under Security.

## Where it met AP MobileApp on main

Two pull requests of that plan merged while the block was read,
and it was merged with main twice:

- **#418, the settings file's key kept where a user's tokens are revoked:**
  its row in a user's keys is drawn whatever this block's `listOnly` says,
  which hides only the making of a key.
  The users' test of My profile makes a key of its own,
  since the suite's owner key is the file's.
- **#420, the user that holds that key is not removed:**
  in the removal's route its refusal comes first, then the removal,
  then this block's warning.

## What the readings found

One builder built it in the worktree.
Two readers read it three times, and one of them the fourth commit:
one the store, the server's rules and their security,
with five probes of its own; the other the tests, the documents,
the fragments and the messages.

- **First reading:**
  the block's own change confirmed (one rule,
  the four counts in their transactions, the first owner's race held).
  Two musts on security: the hole,
  and the last-owner rule counting an integration's user.
  Two musts on form: the fragment's heading, the plan's version in the message.
- **Second reading:**
  both attacks refused by the hardening alone.
  Two more musts in it: the start's token failed to apply where an integration's
  user held the role owner with the lowest id,
  since the start took "the first enabled owner" without the rule;
  and the key held from before (3 above).
- **Third reading:**
  nothing left that must change.
  Small points, done: an integration's user of a revoked integration refused;
  the warning of 5 guarded; on the page, tests that click what they counted,
  and the line's words.
- **Fourth reading, of the settings file's key alone:**
  the move is right, nobody can bring it about for a key of their choosing,
  and its tests fail before for their reasons.
  One point, done: a key of the file's hash that was already revoked on the
  integration's user blocked the owner's key just the same,
  since a token's hash is unique over revoked rows too;
  the move takes such a row and makes it live again,
  and the owner a start makes and its key are one transaction,
  so that a failure leaves no owner without a key.

## For AP PasswordRules' blocks

- **Users:**
  an integration's user's panel has no password field,
  so the rule's line and the dice are not drawn there;
  `Tokens` takes `listOnly`.
- **The server's plain `password`:**
  the users' route refuses one for an integration's user before anything else of
  the request is written; the rule's check comes after that refusal.

## For AP SecondFactor's blocks

- **Every sign-in passes `openSessionFrom`,** which now refuses an integration's user:
  the second step is never asked of one.
- **`principalFor`** bounds an integration's user; a key made only by a session
  that passed the factor meets the refusal of a key for such a user first.

## Seen and left

- **What is set up on an open server outlives the first owner.**
  An integration approved by anyone on the local network keeps its key;
  it is listed in Integrations.
  Worth proposing: the first owner's creation says what was set up while the
  server was open.
- **Home Assistant's own integration** on an open server:
  what it does when the server calls itself open could not be read here;
  the owner's check with it.
- **The settings file's key on an integration's user,
  a regular user beside it and no enabled regular owner:**
  the key stays where it is, under the ceiling, and the start warns.
- **A revoked key of the file's hash on a removed regular user** still blocks a
  new key: the start then fails cleanly, with no owner left half made,
  but the file's token gives no way in.
  Only an older version could leave that state;
  the two newer rules of AP MobileApp refuse both steps that led to it.
- **On the box,** `massimo user remove` can leave no regular user without the
  warning of 5.
- **Two server tests under load,**
  Block 5's: the log's burst test timed out twice under the race detector.
- **Some plural keys** are in only some of the six language files;
  not this block's.

## What verified it

- **The builder, on the last tree:**
  `make ui`, `make ui-check`, `gofmt`, `go vet` for linux, windows and darwin,
  `go run ./tools/apitypes -check`, the whole Go suite, `make cross`,
  each exit 0; six UI test files of the users,
  the profile and the sign-in in all four cases, 342 passed.
- **On a real open server,** with the walk's script for the trap,
  on the first build: after Approve the page stayed open,
  a reload asked for the owner, the owner was made,
  and a request without a token was then refused.
- **Seen failing first:**
  each new test on the tree before its commit;
  the readers took the tests over exports of the trees before and saw each fail
  for its reason.
- **The readers' probes:**
  the two attacks, the start's token, the key held from before,
  a revoked integration's key, an owner with an end date removing itself:
  each refused or warned of now.
- **The coordinating session:**
  each recorded tree against the working tree;
  the four commits made after the strict changelog check of each;
  both merges' resolutions read; `GUIDELINES/shared/tools/check` on the branch,
  twelve checks all well.
