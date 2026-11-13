- The sign-in providers' client secrets are sealed with a key derived
  from `opaque.key`, so that a copy of `massimo.db` alone does not hold
  them; those kept plain are sealed at the next start.
