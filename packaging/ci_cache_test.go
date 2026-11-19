// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package packaging

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeCacheGH is gh as cache-prune.sh calls it: a GET of the caches
// answers $FAKE_GH/caches.json through --jq, whatever its key and ref
// filters ask, so that the script's own choice is what the test sees; a
// DELETE is logged to $FAKE_GH/calls, and fails where $FAKE_GH/<kind>-<id>
// exists: gone with HTTP 404, fail with 500, rate with 429, limit with a
// 403 for the secondary rate limit, forbid with a 403 for the permission.
const fakeCacheGH = `#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || exit 2
shift
method=GET jqx=. path=""
while [ $# -gt 0 ]; do
	case "$1" in
	-X) method="$2"; shift 2 ;;
	--jq) jqx="$2"; shift 2 ;;
	--paginate) shift ;;
	*) path="$1"; shift ;;
	esac
done
if [ "$method" = DELETE ]; then
	id="${path##*/}"
	[ ! -e "$FAKE_GH/gone-$id" ] || { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
	[ ! -e "$FAKE_GH/fail-$id" ] || { echo "gh: Server Error (HTTP 500)" >&2; exit 1; }
	[ ! -e "$FAKE_GH/rate-$id" ] || { echo "gh: Too Many Requests (HTTP 429)" >&2; exit 1; }
	[ ! -e "$FAKE_GH/limit-$id" ] || { echo "gh: You have exceeded a secondary rate limit. (HTTP 403)" >&2; exit 1; }
	[ ! -e "$FAKE_GH/forbid-$id" ] || { echo "gh: Resource not accessible by integration (HTTP 403)" >&2; exit 1; }
	echo "DELETE $path" >>"$FAKE_GH/calls"
	exit 0
fi
case "$path" in
*"/actions/caches?"*) jq -r "$jqx" "$FAKE_GH/caches.json" ;;
*) exit 1 ;;
esac
`

// actionsCache is a cache entry as the API lists it.
type actionsCache struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Ref     string `json:"ref"`
	Created string `json:"created_at"`
	Size    int64  `json:"size_in_bytes"`
}

// overlay is the entry of CodeQL's overlay database for the language at
// the commit numbered n, made at minute n, of run 100+n.
func overlay(id int64, lang string, n int) actionsCache {
	sha := strings.Repeat(fmt.Sprintf("%x", n%16), 40)
	return actionsCache{ID: id, Key: fmt.Sprintf("codeql-overlay-base-database-1-433570c23e836bda-%s-2.27.4-%s-%d-1", lang, sha, 100+n),
		Ref: "refs/heads/main", Created: fmt.Sprintf("2026-11-18T10:%02d:00Z", n), Size: 40_000_000}
}

// runCachePrune runs packaging/cache-prune.sh against fakeCacheGH with the
// entries, the deletes that fail by kind (fakeCacheGH), no pause between
// deletes and the environment's additions; it returns the script's
// output, the summary, the ids it deleted, and its exit error.
func runCachePrune(t *testing.T, caches []actionsCache, failing map[string][]int64, env ...string) (out, summary string, deleted []int64, err error) {
	t.Helper()
	bash := scriptShell(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeCacheGH), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"total_count": len(caches), "actions_caches": caches})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "caches.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	for kind, ids := range failing {
		for _, id := range ids {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d", kind, id)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command(bash, "cache-prune.sh")
	cmd.Env = append(append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH="+dir,
		"GITHUB_REPOSITORY=owner/repo",
		"GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary.md"),
		"KEEP=", "DRY_RUN=", "PRUNE_PAUSE=0"), env...)
	got, runErr := cmd.CombinedOutput()
	s, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	for _, line := range strings.Fields(string(calls)) {
		var id int64
		if _, err := fmt.Sscanf(line, "repos/owner/repo/actions/caches/%d", &id); err == nil {
			deleted = append(deleted, id)
		}
	}
	slices.Sort(deleted)
	return string(got), string(s), deleted, runErr
}

// TestCachePrune: CodeQL's overlay databases of all but the newest commits
// go, per language and CLI version; the Go jobs' caches, setup-go's,
// CodeQL's other caches, an entry of another ref and a key of another
// form stay, though the API lists them.
func TestCachePrune(t *testing.T) {
	var caches []actionsCache
	id := int64(1)
	for n := 1; n <= 5; n++ {
		for _, lang := range []string{"go", "javascript", "python"} {
			caches = append(caches, overlay(id, lang, n))
			id++
		}
	}
	// A re-run of commit 4's Go analysis, and an older CLI's two commits.
	rerun := overlay(40, "go", 4)
	rerun.Key = strings.Replace(rerun.Key, "-104-1", "-104-2", 1)
	oldCLI := []actionsCache{overlay(50, "go", 1), overlay(51, "go", 2)}
	for i := range oldCLI {
		oldCLI[i].Key = strings.Replace(oldCLI[i].Key, "2.27.4", "2.27.3", 1)
	}
	pr := overlay(60, "go", 1)
	pr.Ref = "refs/pull/7/merge"
	odd := overlay(61, "go", 1)
	odd.Key = "codeql-overlay-base-database-1-433570c23e836bda-go-2.27.4-not-a-commit"
	others := []actionsCache{
		{ID: 70, Key: "go-vet-Linux-X64-go1.26.11-abc-" + strings.Repeat("1", 40), Ref: "refs/heads/main", Created: "2026-11-18T09:00:00Z", Size: 900_000_000},
		{ID: 71, Key: "setup-go-Linux-x64-ubuntu26-go-1.26.11-abc", Ref: "refs/heads/main", Created: "2026-11-18T09:00:00Z", Size: 13_000_000},
		{ID: 72, Key: "codeql-dependencies-1-Linux-go-abc", Ref: "refs/heads/main", Created: "2026-11-18T09:00:00Z", Size: 225_000_000},
		{ID: 73, Key: "codeql-overlay-status-java-2.27.4-runner-140GB", Ref: "refs/heads/main", Created: "2026-11-18T09:00:00Z", Size: 327},
	}
	caches = append(append(append(caches, rerun, pr, odd), oldCLI...), others...)
	// Commits 1 and 2 of each language go; 3 to 5 stay, the re-run of 4
	// with them; the older CLI's two are its stem's newest.
	want := []int64{1, 2, 3, 4, 5, 6}

	out, summary, deleted, err := runCachePrune(t, caches, nil)
	if err != nil {
		t.Fatalf("cache-prune.sh: %v\n%s", err, out)
	}
	if !slices.Equal(deleted, want) {
		t.Errorf("deleted %v, want %v\n%s", deleted, want, out)
	}
	wants(t, "the summary", summary, "6 overlay database(s) of older commits deleted, 240 MB", "the 3 newest commits' kept",
		"- 1 with a key of another form, left alone")
	// The odd key is counted, as a sign that CodeQL changed its keys.
	wants(t, "the output", out, "::warning::1 of main's caches under codeql-overlay-base-database- have a key of another form")

	t.Run("keep", func(t *testing.T) {
		out, _, deleted, err := runCachePrune(t, caches, nil, "KEEP=4")
		if err != nil || !slices.Equal(deleted, []int64{1, 2, 3}) {
			t.Errorf("KEEP=4 deleted %v (%v), want commit 1's\n%s", deleted, err, out)
		}
		if out, _, _, err := runCachePrune(t, caches, nil, "KEEP=0"); err == nil {
			t.Errorf("KEEP=0 passed:\n%s", out)
		}
	})

	t.Run("dry run", func(t *testing.T) {
		out, summary, deleted, err := runCachePrune(t, caches, nil, "DRY_RUN=1")
		if err != nil || len(deleted) > 0 {
			t.Errorf("a dry run deleted %v (%v)\n%s", deleted, err, out)
		}
		wants(t, "the dry run's output", out, "would delete "+caches[0].Key)
		wants(t, "the dry run's summary", summary, "6 overlay database(s) of older commits to delete (dry run)")
	})

	// An entry already gone counts as done; one that cannot be deleted
	// fails the script once the rest are deleted.
	t.Run("failures", func(t *testing.T) {
		out, summary, deleted, err := runCachePrune(t, caches, map[string][]int64{"gone": {2}})
		if err != nil || !slices.Equal(deleted, []int64{1, 3, 4, 5, 6}) {
			t.Errorf("with 2 gone: deleted %v (%v)\n%s", deleted, err, out)
		}
		wants(t, "the summary", summary, "1 already gone")
		out, summary, deleted, err = runCachePrune(t, caches, map[string][]int64{"fail": {2}})
		if err == nil || !slices.Equal(deleted, []int64{1, 3, 4, 5, 6}) {
			t.Errorf("with 2 failing: deleted %v (%v)\n%s", deleted, err, out)
		}
		wants(t, "the output", out, "::warning::could not delete "+caches[1].Key)
		wants(t, "the summary", summary, "1 could not be deleted")
	})

	// A refusal for the rate stops the run, which passes, and the next
	// run deletes the rest; another 403 stops it too, and fails it. The
	// oldest go first: commit 1's three, then commit 2's.
	t.Run("refusals", func(t *testing.T) {
		for kind, failed := range map[string]bool{"rate": false, "limit": false, "forbid": true} {
			out, summary, deleted, err := runCachePrune(t, caches, map[string][]int64{kind: {5}})
			if (err != nil) != failed || !slices.Equal(deleted, []int64{1, 2, 3, 4}) {
				t.Errorf("%s at 5: deleted %v (%v), want 1 to 4, failed %v\n%s", kind, deleted, err, failed, out)
			}
			if failed {
				wants(t, "the summary of "+kind, summary, "stopped at a refusal (403)")
			} else {
				wants(t, "the output of "+kind, out, "::notice::GitHub's rate limit refused a delete")
				wants(t, "the summary of "+kind, summary, "stopped by GitHub's rate limit")
			}
		}
	})
}

// TestCachePruneWorkflow: the workflow that runs cache-prune.sh holds
// actions: write, its only write, and reads nothing but the script.
func TestCachePruneWorkflow(t *testing.T) {
	wf := readWorkflow(t, "caches.yml")
	for _, s := range []string{"permissions: {}\n", "      actions: write\n", "      contents: read\n",
		"          sparse-checkout: packaging/cache-prune.sh\n", "          persist-credentials: false\n",
		"        run: packaging/cache-prune.sh\n"} {
		if !strings.Contains(wf, s) {
			t.Errorf("caches.yml lacks %q", s)
		}
	}
	writes := 0
	for _, line := range strings.Split(wf, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.HasSuffix(line, ": write") {
			writes++
		}
	}
	if writes != 1 {
		t.Errorf("caches.yml grants %d writes, want actions: write alone", writes)
	}
}

// runGoCache runs packaging/go-cache.sh with the arguments in the
// repository's top directory, as CI's step does, with GitHub's
// environment for the event and ref given, and returns its outputs.
func runGoCache(t *testing.T, event, ref string, env []string, args ...string) map[string]string {
	t.Helper()
	bash := scriptShell(t)
	out := filepath.Join(t.TempDir(), "outputs")
	cmd := exec.Command(bash, append([]string{"packaging/go-cache.sh"}, args...)...)
	cmd.Dir = ".."
	cmd.Env = append(append(os.Environ(), "GITHUB_OUTPUT="+out, "RUNNER_OS=Linux", "RUNNER_ARCH=X64",
		"GITHUB_EVENT_NAME="+event, "GITHUB_REF="+ref), env...)
	if got, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go-cache.sh %v: %v\n%s", args, err, got)
	}
	b, err := os.ReadFile(out)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// GitHub's output file: name=value, or name<<DELIM, lines, DELIM.
	outputs := map[string]string{}
	lines := strings.Split(string(b), "\n")
	for i := 0; i < len(lines); i++ {
		if name, delim, ok := strings.Cut(lines[i], "<<"); ok {
			var v []string
			for i++; i < len(lines) && lines[i] != delim; i++ {
				v = append(v, lines[i])
			}
			outputs[name] = strings.Join(v, "\n")
		} else if name, v, ok := strings.Cut(lines[i], "="); ok {
			outputs[name] = v
		}
	}
	return outputs
}

// TestGoCache: a heavy Go job's cache key is its own, the Go version's,
// go.sum's and the week's; a run restores the same Go's newest, and only a
// push to main saves. A run that saves nothing holds Go's trim off.
func TestGoCache(t *testing.T) {
	goenv := func(name string) string {
		t.Helper()
		b, err := exec.Command("go", "env", name).Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	version := goenv("GOVERSION")
	pr := runGoCache(t, "pull_request", "refs/pull/7/merge", nil, "key", "vet")
	same := "go-vet-Linux-X64-" + version + "-"
	sums, week, ok := strings.Cut(strings.TrimPrefix(pr["key"], same), "-")
	if !strings.HasPrefix(pr["key"], same) || !ok || len(sums) != 16 || !strings.HasPrefix(week, "20") || !strings.Contains(week, "-W") {
		t.Errorf("key %q, want %s<sums>-<year>-W<week>", pr["key"], same)
	}
	if want := same + sums + "-\n" + same; pr["restore-keys"] != want {
		t.Errorf("restore-keys %q, want %q", pr["restore-keys"], want)
	}
	if want := goenv("GOCACHE") + "\n" + goenv("GOMODCACHE"); pr["path"] != want {
		t.Errorf("path %q, want %q", pr["path"], want)
	}
	for _, c := range []struct{ event, ref, save string }{
		{"pull_request", "refs/pull/7/merge", "false"},
		{"push", "refs/heads/main", "true"},
		{"push", "refs/tags/v1.2.3", "false"},
	} {
		if got := runGoCache(t, c.event, c.ref, nil, "key", "race"); got["save"] != c.save ||
			!strings.HasPrefix(got["key"], "go-race-Linux-X64-") {
			t.Errorf("%s of %s: key %q, save %q, want go-race-…, %s", c.event, c.ref, got["key"], got["save"], c.save)
		}
	}

	gocache := filepath.Join(t.TempDir(), "go-build")
	runGoCache(t, "pull_request", "refs/pull/7/merge", []string{"GOCACHE=" + gocache}, "hold")
	b, err := os.ReadFile(filepath.Join(gocache, "trim.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err != nil || time.Since(time.Unix(n, 0)) > time.Minute {
		t.Errorf("trim.txt %q, want the time now", b)
	}
}

// workflowJob returns the lines of ci.yml's job with the id given, up to
// the next job's id.
func workflowJob(t *testing.T, wf, id string) string {
	t.Helper()
	_, rest, ok := strings.Cut(wf, "\n  "+id+":\n")
	if !ok {
		t.Fatalf("no job %s", id)
	}
	for _, line := range strings.SplitAfter(rest, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && !strings.HasPrefix(line, "  #") && strings.HasSuffix(line, ":\n") {
			job, _, _ := strings.Cut(rest, line)
			return job
		}
	}
	return rest
}

// TestGoCacheJobs: each heavy Go job keeps a cache of its own through
// go-cache.sh, with setup-go's off, holds the trim where it saves
// nothing (a push to main that found its key among them), and saves on a
// push to main alone, under the key it restored with, where that key was
// not found.
func TestGoCacheJobs(t *testing.T) {
	wf := readWorkflow(t, "ci.yml")
	jobs := map[string]string{"test": "vet", "race": "race", "armv6": "armv6"}
	for id, name := range jobs {
		job := workflowJob(t, wf, id)
		for _, other := range jobs {
			if other != name {
				lacks(t, "ci.yml's job "+id, job, "packaging/go-cache.sh key "+other+"\n")
			}
		}
		wants(t, "ci.yml's job "+id, job,
			"          cache: false\n",
			"        run: packaging/go-cache.sh key "+name+"\n",
			"      - uses: actions/cache/restore@",
			"          restore-keys: ${{ steps.gocache.outputs.restore-keys }}\n",
			"        if: steps.gocache.outputs.save != 'true' || steps.gocache-restore.outputs.cache-hit == 'true'\n        run: packaging/go-cache.sh hold\n",
			"      - uses: actions/cache/save@",
			"        if: steps.gocache.outputs.save == 'true' && steps.gocache-restore.outputs.cache-hit != 'true'\n")
		if n := strings.Count(job, "          key: ${{ steps.gocache.outputs.key }}\n"); n != 2 {
			t.Errorf("job %s uses the key %d times, want 2: restored and saved", id, n)
		}
		if restore, ui := strings.Index(job, "packaging/go-cache.sh hold"), strings.Index(job, "./.github/actions/ui-bundle"); restore < 0 || ui < restore {
			t.Errorf("job %s builds the UI's bundle before it holds the trim", id)
		}
	}
}
