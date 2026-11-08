# NOTE TokenDownload v1.0: how the run went

Blocks 1 and 2 ran under the owner's `PLAN++` of 2026-11-08,
the plan committed into `main` as `f682264` first,
then two signed commits on the branch `claude/token-download`,
and pull request 113.
Where it went differently from the plan:

- **`send`, not `try`.**
  `do` makes each attempt's context itself and builds the request with it,
  then hands both to `send(hc, req, end)`,
  which starts the answer wait and wraps the body.
  So an error in building a request returns at once, as before,
  instead of reaching `retryable`, which would have sent a malformed URL again:
  such an error is no `net.Error`.
- **`retryable`'s explicit rule** for a missing answer is redundant today:
  the error wraps `errNoAnswer`, which is no `net.Error` either,
  so the rule for a connection that failed before an answer covers it already.
  It stays, stating the intent,
  and a change of the error's type cannot make it a timeout that is not retried.
  A mutation of that line therefore passes the tests;
  the tests' check is a mutation of the wait itself,
  under which the three new tests fail.
- **`fileAccept` on every release file**,
  the download URLs included, not only the API's:
  a download URL sends the bytes whatever is asked,
  and one header for all spares a rule on the URL.
- **One more test, `TestAnswerWaitLeavesTheBody`:**
  a binary that keeps sending for longer than the wait is not cut off,
  so the wait ends with the answer, as the plan says.
- **The way the hang was placed** was checked before the plan was written,
  with a server that redirects to one that never answers:
  net/http's error names the second.
- **The token's CHANGELOG bullet** names "a release's manifest, signature
  and binary" for the plan's "an update's files", and "as its list does",
  within its three lines.

The pull request's checks were pending when the run ended.
