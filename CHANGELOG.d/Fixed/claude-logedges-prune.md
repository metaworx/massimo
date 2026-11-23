- Web UI: the log's warning sign no longer goes quiet after the server
  was off longer than the log keeps its lines: the prune keeps the
  newest line, so a new warning is not taken for one already seen.
