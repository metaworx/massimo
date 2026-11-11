- A pruned backup's id is never given to a later backup, so a list
  loaded before the prune can no longer delete or download another
  backup by its id (migration 0013; `docs/store.md` section 9).
