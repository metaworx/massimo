# NOTE StartConfiguration v1.0: owner decisions d36 to d38

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.
The owner's answer to the run's completion gate, 2026-11-25,
with `EXEC merge` for #425, #427 and #430.
The open points are those of
`wip/2026-11-25_07-52_NOTE_StartConfiguration_v1.0_how_the_run_went.md`,
"Open for the owner", numbered D36 to D38 at the gate.

## Decided

- **D36-A. The plain proxy of a Cube the store has:** the store's rule,
  as built in Block 11: the plain lines are that Cube's, compared with its
  row; the plain proxy is a difference with its button.
- **D37. Who may restart:** `reboot` and `update` each include `restart`,
  in the owner's words.
  This goes past D34-A, which let `update` read `GET /api/v1/restart` only.
  Whether `reboot` also includes `update` is open: the owner is not sure,
  since an update may bring new or incompatible features,
  while a reboot is mainly for maintenance.
- **D38-B. `install -settings` with a file serve refuses**
  (a line without `=`): refused before any write, as UTF-16 is,
  in a pull request of its own.

## What follows

D37 and D38-B change the blocks, so they wait for the plan's next
revision, v1.1, which only `PLAN+` or `PLAN++` writes (`/AGENTS.md` §5.1).
How `include` is built, and whether `reboot` includes `update`,
are weighed at the gate that offers it.
