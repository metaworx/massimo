# NOTE FirstOwner v1.0: gate decisions

What the owner decided at the plan's gate on 2026-11-21,
and what has come up since the plan was presented.

## The two open decisions

Both as the plan recommends, taken with the owner's `EXEC++`,
whose gate said so:

- **D17-A.** Integration users count as users, as on main today.
  It is named in the plan and left to AP IntegrationUsers, whose subject it is;
  this plan's card asks while `/me` says the server is open.
- **D18-A.** Nothing special for the phone as its own server:
  AP MobileApp's block sets the setting, or creates an owner, when it is built.
  That plan's session has read the point and sees it the same:
  its block would set `auth.keep_open` for the phone's own server
  and clear it when the server is opened to the local network.

## How the blocks are run

The owner's `EXEC++` runs all four blocks.
Each is built by a sub-agent on Opus 5.5 and read by independent readers,
as AP UiParts' blocks are (the owner's word of 2026-11-20 in this session:
"use opus subagents for most of the work
and only do the coordination (and important verification) here");
this session verifies, commits, opens the pull request
and merges it once every check is green.
The plan's retirement is not part of the run.

## For Block 2: the form's fields in the card

AP UiParts' sign-in half is on main since the plan was written.
In it the first owner's form is a part whose fields are controlled:
each draw writes back what its state holds.
The sign-in card's own fields are not (UiParts D210):
no draw sets what they hold, so that a password manager's fill stays,
and they are read at the submit.

In the card the first owner's form follows the card's rule:
its name and its two passwords are not controlled and are read at the submit.
AP MobileApp's session found the difference when it read the sign-in half
for the app; in practice nothing is lost today,
since the fillers it knows fire the input's event.

## Where the plan meets AP UiParts' Block 15

Block 1 touches the server alone and waits for nothing.
Blocks 2 to 4 change files that AP UiParts' last block also changes
while it is built: `login.js`, ☰, System settings → Users
and the handle through which the UI tests reach the page.
Whichever of the two reaches main second merges main first.
