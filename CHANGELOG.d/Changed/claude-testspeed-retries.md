- CI no longer fails a push to `main` whose macOS or Windows job never
  got a runner: the nightly build runs that job first, and a release's
  run gets it run again, at most twice (AP TestSpeed).
