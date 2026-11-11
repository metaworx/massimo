- The store's `events` are its `log` (migration 0014), whose rows may
  name their command and be for an auditor's eyes alone; older releases
  read and write it through a view `events`, so a rollback still works.
