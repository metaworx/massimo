- A SIGHUP while `serve` starts no longer ends it, which systemd took
  for a clean stop and did not restart: with `-sighup reload` it is
  logged as a warning and ignored until `serve` is ready.
