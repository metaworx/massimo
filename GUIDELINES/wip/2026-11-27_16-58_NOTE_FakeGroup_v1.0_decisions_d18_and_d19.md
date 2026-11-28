# NOTE FakeGroup v1.0: decisions d18 and d19

Two decisions of the owner after v1.0 was presented,
which change a step and leave the blocks as they stand.

- **D18-A** (2026-11-26, `EXEC repeat D18-A`):
  the wakeup and `r:` runs were repeated straight to the test Cube,
  the owner's service stopped
  (`wip/2026-11-26_22-54_ANALYSIS_FakeGroup_z_wakeup_direct.md`).
  - With a new LAN connection for each command no wakeup held,
    whatever its value: a wakeup seems to end with its connection.
  - Two `r:` lines while a radiogram was pending added nothing to its
    cost: `r:` does not make the Cube send it again.
  - The duty count held for 30 minutes with nothing sent:
    it is the air time of the last hour.
- **D19-A** (2026-11-27, `PLAN++, D19-A`):
  before Block 1, one series on one standing connection measures whether
  the wakeup's value sets its window: `z:05` against `z:1e`,
  a harmless radiogram every 10 s, the readings from `r:` answers,
  stopping at 60 %. Block 1 records every run, this one included.
