# actionlint

`make actionlint` lints the workflows under `.github/workflows/`
with [actionlint](https://github.com/rhysd/actionlint),
and their `run:` scripts with shellcheck where it is on the `PATH`,
as it is on GitHub's Ubuntu runners;
CI's job "actionlint" runs the same (`GUIDELINES/project/CI.md`, section 2).

shellcheck is not pinned: the runner's is newer than Debian's 0.9.0,
and may report what an older one does not,
as SC2329 for a function only a trap calls, which 0.9.0 reports as SC2317.
Code that neither version flags is better than a directive for each.

The tool is pinned by the `tool` line of this directory's `go.mod`,
a module of its own, so that its modules stay out of massimo's `go.mod`;
the target runs it from the repository root:

```sh
go tool -modfile=tools/actionlint/go.mod actionlint
```

Dependabot proposes a new release (`.github/dependabot.yml`);
by hand, in this directory:

```sh
go get -tool github.com/rhysd/actionlint/cmd/actionlint@vX.Y.Z
go mod tidy
```
