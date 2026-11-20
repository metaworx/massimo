- The log pages forward from a line: `after=<id>` on both log routes
  reads each line once, a burst longer than a page too; the log kept
  without a store numbers its lines, and `/healthz` says `log_after`.
