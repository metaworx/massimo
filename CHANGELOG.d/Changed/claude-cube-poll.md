- `serve` polls each Cube every 10 s instead of every minute, as the
  original software does; a lost Cube is marked within 30 s, and the
  settings name the flag that decides an interval (`fixed_by`).
