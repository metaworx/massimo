- CI runs the web UI's tests as a job per case and the screenshots as a
  job of their own, records a test's video only on its retry, and skips
  the UI jobs for a change to the server's tests alone (AP TestSpeed).
