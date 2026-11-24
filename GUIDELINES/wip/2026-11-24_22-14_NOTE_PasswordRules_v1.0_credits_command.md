# NOTE PasswordRules v1.0: credits command

What the owner decided on 2026-11-24, after the plan was committed.

## D53 becomes A

The plan took D53-B with `PLAN++`: System info alone shows the credits,
and the command was to be offered again at the completion gate.
Told of it, the owner wrote:
"would the `credits` command be easy to implement? then please do".
It is: the list is embedded, and the command reads it.

- **D53-A.** `massimo credits` prints the list,
  and `massimo credits <name>` one entry's licence text,
  both from the embedded file that System info's part reads,
  so that the two cannot differ.
  A box without a browser can then still say what it ships.

## What the plan's fifth block gains

- **The command** in `cmd/massimo`, with its line in the usage;
  a name the list does not hold is an error that names the command for the list.
- **Its test:** the list as printed holds every entry of the embedded file,
  one entry's text is printed, an unknown name is refused.
- **`README.md`:** the command among the commands,
  beside the sentence on the credits in System info.
- **The CHANGELOG bullet** of the block names both, System info and the command.

The other blocks stand as they are.
