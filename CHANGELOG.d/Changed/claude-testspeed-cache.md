- CI's Go test jobs keep a build cache each, run the tests that need root
  as root, and take no test's pass from a cache, and CodeQL's caches of
  older commits are deleted every hour (AP TestSpeed).
