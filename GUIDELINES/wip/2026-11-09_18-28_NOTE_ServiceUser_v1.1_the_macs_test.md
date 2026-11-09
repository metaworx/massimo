# NOTE ServiceUser v1.1: the Mac's test

The owner's Mac (macOS 27.2, arm64, clean of massimo before and after)
ran `massimo install` from main after pull request #143
(`v0.12.0-alpha.1-155-g933af00`) on 2026-11-09,
the owner typing sudo's password in a shared terminal:
40 checks of 40 passed.

- A fresh install on the template: not loaded, the message,
  `/var/lib/massimo` 750 `_massimo:_massimo`, nothing on port 8099.
  The message's line is
  `sudo -u _massimo /usr/local/bin/massimo -env /etc/massimo/env -data /var/lib/massimo user add <name> -role owner`.
- That line, taken from the message and run from the owner's home directory
  (`drwxr-x---`), creates the owner;
  the same line without the full path and `-env` had failed there that
  afternoon with `sudo: massimo: command not found`.
  Loaded with the message's `launchctl bootstrap`, the API asks for a sign-in.
- `install` again: a loaded job is restarted (a new PID and `started_at`);
  a booted-out one stays stopped, with the message,
  also on an older template's comments; a setting of its own starts it.
- The web UI's way: the open server answers the Mac itself,
  the first owner from the form's route, then closed;
  `install` again restarts it.
- `install -remove -purge` leaves no job, binary, directory, log,
  `_massimo` user or firewall entry.
