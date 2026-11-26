- The fake Cube's `HoldApply` keeps the commands it acknowledges from
  taking effect until the test releases them, so that a test sees a
  write on its way for as long as it needs (AP SteadyTests).
