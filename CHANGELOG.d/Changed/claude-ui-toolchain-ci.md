- CI builds the web UI's bundle before every job compiles Go, in one
  shared step, and type-checks the parts and checks their hooks in the
  job UI types (`make ui-check`) (AP UiParts, AP UiLint).
