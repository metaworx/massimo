- `make race` leaves SQLite's pointer steps unchecked (`checkptr` off for
  `modernc.org`), halving its store-heavy tests' time; the race detector
  stays on everywhere (AP TestSpeed, docs/testing.md).
