# NOTE StoreRaise v1.2: decisions on block 3

The owner, 2026-11-16, `EXEC D15-C D16-A D17-A`, on the gate after
Block 3 (NOTE StoreRaise v1.2, "how block 3 went", points 4, 5 and 8):

- **D15-C:** one serial with several Cube rows loses nothing on the way
  down. The live row, or else the newest removed one, keeps the serial;
  each other row keeps all its rows and its own `cubes.serial` becomes
  `<serial>~<id>`, which schema 23 never reads (it reads `deleted_at IS
  NULL` only). 0024's up part strips the suffix. Only `cubes.serial`
  changes: `log.serial` and the serials in JSON are history and stay as
  written, the lines tied to their row by `cube_id`. L7 keeps only the
  option's time. Applied on #335.
- **D16-A:** until Block 4 the API's integration `key` fields carry the
  integration's uuid; no release is cut between #335 and Block 4.
- **D17-A:** the downgraded file is compared with schema 23 to the byte,
  `sqlite_master` and every row, `sqlite_sequence` included; that stands in
  for the v23 binary's tests on it.
