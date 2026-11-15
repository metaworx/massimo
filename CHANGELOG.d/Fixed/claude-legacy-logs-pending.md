- `pending_radiograms` reads 50, not 0, when the Cube has no memory slot
  left, as a Cube whose radiograms wait for silent devices can have.
