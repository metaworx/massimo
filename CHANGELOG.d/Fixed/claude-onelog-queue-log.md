- The queue's lines (`room woken up`, `proxy client's command sent`, the
  duty cycle's waits) reach `serve`'s log, its file and `-log-level`,
  rather than Go's default logger on stderr.
