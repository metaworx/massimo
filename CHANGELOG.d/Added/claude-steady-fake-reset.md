- `massimo fakecube -control`: `POST /reset` puts the fake back as it
  started and drops its clients, which read it anew, so that each UI
  test file starts from the recorded Cube (AP SteadyTests).
