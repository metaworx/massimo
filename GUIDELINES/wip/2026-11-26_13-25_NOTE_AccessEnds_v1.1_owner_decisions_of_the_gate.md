# NOTE AccessEnds v1.1: owner decisions of the gate

The owner's answer of 2026-11-26 to the gate that ended the run of "PLAN++ as
recommended": seven of the plan's eight blocks are merged (1, 2, 3, 4, 6, 7, 8,
each with its NOTE beside this one), Block 5 is not built.
The owner's words are quoted; what follows each is what it means for the plan.

## Decided

- **D29-E, with a switch to turn it off; in the next version.**
  "D29-E with a cli/env switch to turn it off - e.g. on test servers -
  showing a warning in `System info` if set. goes in the next version.
  so hold off for now."
  - **The source** is Have I Been Pwned's Pwned Passwords:
    the commonest passwords as SHA-1 hashes, cut short,
    which the page and the server compare with the hash of what is typed.
    Its Terms of Use (last updated 27 October 2026) say it "may be used for any
    lawful purpose,
    including within commercial products and for the benefit of third parties"
    and "Attribution isn't required for Pwned Passwords".
    One clause stays unsettled and goes into the block's NOTE and the credits:
    the same Terms forbid "modifying, reproducing, revising, transmitting,
    distributing, … HIBP Data",
    under "interfere with or disrupt the integrity of the performance of our
    Services", and the Pwned Passwords exception does not name it.
  - **What the investigation measured:**
    the top 10,000 can only be had by one scan of all 1,048,576 ranges of the
    range API (about 45 GB; no rate limit, the API says);
    cut to 5 bytes they are a file of 50,000 bytes;
    about half are under 8 characters and can never match;
    the page needs a SHA-1 of its own,
    since `crypto.subtle` is missing over plain HTTP.
    Openwall's list is not taken:
    its commonest entries stand in Openwall's wordlists collection under terms
    that forbid charging for a copy.
  - **D43-A, the owner's addition:**
    a command-line option and an environment variable turn the list off,
    for a test server, say; where it is off,
    System info shows a warning that says so.
    Their names are the block's.
  - **When:**
    the next version.
    Block 5 is held: nothing of it is built in this plan's run,
    and it is written anew from this NOTE when it is taken up.
- **PasswordRules.D59-A, again:**
  "PasswordRules.D59 - A".
  The dice's move at the first press stays as Block 4 measured it.
- **D42-A:**
  "D42-A".
  The open-setup alert stays visible to every reader of the log,
  as Block 3 built it.

## Still open

- **D41,** who manages an integration's user stored as guest:
  the owner asked for A5 explained,
  and noted "`users` holder may manage *all* users".
  A4 is recommended; nothing is built before the answer.
- **AccessEnds.D19,** the place of the suggestion's line: the owner
  asked what it is about; explained in the session, A recommended.
- **D44, new, the owner's question:**
  "what about removing the radio button for selecting the char-vs-words
  and alternate between the two on each tap?"
  The dice would give five words, then twenty characters, then five words again;
  the Words and Characters choice under the field would go, and Copy would stay.
  Weighed in the session; open.

## Elsewhere

- **AP SecondFactor** goes in the next version,
  the owner's word of the same answer; its own NOTE says so.
- **The unsteady tests** of the run went to the session "Fix flaky
  tests" at the owner's "EXEC flaky".
- **The plan's retirement** waits for Block 5's place: held in this
  plan until the next version, or carried into a plan of its own so
  that this one can retire; the owner's word.
