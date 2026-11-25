// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package packaging

import (
	"encoding/json"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// scriptShell returns bash, or skips where the platform lacks bash or jq,
// which CI's scripts need and GitHub's Linux runners have.
func scriptShell(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("CI runs these scripts on Linux")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("no jq")
	}
	return bash
}

// fakeGH is gh as ci-report.sh calls it: `gh api` GETs answer from
// $FAKE_GH/latest.json, all.json and comments.json through --jq; a write
// (-X PATCH or POST) is logged with its body, and fails where
// $FAKE_GH/fail-writes exists. Every call is logged to $FAKE_GH/calls.
const fakeGH = `#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || exit 2
shift
method=GET jqx=. path="" body=""
while [ $# -gt 0 ]; do
	case "$1" in
	-X) method="$2"; shift 2 ;;
	--jq) jqx="$2"; shift 2 ;;
	--paginate) shift ;;
	-F) body="${2#body=@}"; shift 2 ;;
	*) path="$1"; shift ;;
	esac
done
printf '%s %s\n' "$method" "$path" >>"$FAKE_GH/calls"
if [ "$method" != GET ]; then
	[ ! -e "$FAKE_GH/fail-writes" ] || { echo "HTTP 403: Resource not accessible by integration" >&2; exit 1; }
	{ cat "$body"; echo '<!-- end -->'; } >>"$FAKE_GH/calls"
	exit 0
fi
case "$path" in
*"/jobs?filter=latest&"*) f=latest ;;
*"/jobs?filter=all&"*) f=all ;;
*/comments) f=comments ;;
*) exit 1 ;;
esac
[ -e "$FAKE_GH/$f.json" ] || { echo "HTTP 404" >&2; exit 1; }
jq -c "$jqx" "$FAKE_GH/$f.json"
`

// ciJob is a job of a run as the API lists it, with the fields
// ci-report.sh reads.
type ciJob struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	RunAttempt int      `json:"run_attempt"`
	Status     string   `json:"status"`
	Conclusion *string  `json:"conclusion"`
	RunnerName string   `json:"runner_name"`
	StartedAt  string   `json:"started_at"`
	Steps      []ciStep `json:"steps"`
}

// ciStep is a step of a ciJob; only their number is read.
type ciStep struct {
	Name string `json:"name"`
}

// ran is a job that a runner took and that ended so, started at the
// minute given: a job a re-run leaves alone is carried into the next
// attempt with its start.
func ran(id int64, attempt int, name, conclusion string, minute int) ciJob {
	c := conclusion
	return ciJob{ID: id, Name: name, RunAttempt: attempt, Status: "completed", Conclusion: &c,
		RunnerName: "GitHub Actions " + strconv.FormatInt(id, 10),
		StartedAt:  "2026-11-18T10:" + strconv.Itoa(10+minute) + ":00Z", Steps: []ciStep{{"Set up job"}}}
}

// notRun is a job that no runner took: skipped, or cancelled before.
func notRun(id int64, attempt int, name, conclusion string) ciJob {
	c := conclusion
	return ciJob{ID: id, Name: name, RunAttempt: attempt, Status: "completed", Conclusion: &c,
		StartedAt: "2026-11-18T10:0" + strconv.Itoa(attempt) + ":00Z"}
}

// running is the job that runs the script, All tests passed.
func running(id int64, attempt int) ciJob {
	return ciJob{ID: id, Name: "All tests passed", RunAttempt: attempt, Status: "in_progress",
		RunnerName: "GitHub Actions 99", StartedAt: "2026-11-18T11:00:00Z", Steps: []ciStep{{"Set up job"}}}
}

// ciReportRun is one run of ci-report.sh: what the API holds, and what
// the step's environment gives.
type ciReportRun struct {
	attempt    int
	pr         string
	needs      string
	latest     []ciJob
	all        []ciJob // nil: the API refuses filter=all
	allJSON    any     // where set, what filter=all answers in place of all
	comments   string  // the pull request's comments, JSON
	failWrites bool
	unrunJobs  string // UNRUN_JOBS, where not ci.yml's
	ref        string // GITHUB_REF of a push: refs/heads/main where empty
}

// ciReportResult is what a run of ci-report.sh left: its output, the
// summary it wrote, the step's outputs and the calls of gh it made, with
// their bodies.
type ciReportResult struct {
	out, summary, outputs, calls string
}

// ciUnrunJobs is UNRUN_JOBS as ci.yml gives it: the jobs that a pull
// request and a push to main leave unrun where no runner took them (AP
// TestSpeed, D5-A, D16-A), by the names the API gives a called workflow's
// jobs, with their caller's id among the needs.
const ciUnrunJobs = `{"Platforms / macOS LaunchDaemon": "platforms", "Platforms / Windows service": "platforms"}`

// runCIReport runs packaging/ci-report.sh against fakeGH and fails the
// test where it exits with anything but 0.
func runCIReport(t *testing.T, r ciReportRun) ciReportResult {
	t.Helper()
	bash := scriptShell(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("latest.json", map[string]any{"jobs": r.latest})
	switch {
	case r.allJSON != nil:
		write("all.json", r.allJSON)
	case r.all != nil:
		write("all.json", map[string]any{"jobs": r.all})
	}
	comments := r.comments
	if comments == "" {
		comments = "[]"
	}
	if err := os.WriteFile(filepath.Join(dir, "comments.json"), []byte(comments), 0o644); err != nil {
		t.Fatal(err)
	}
	if r.failWrites {
		if err := os.WriteFile(filepath.Join(dir, "fail-writes"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	summary := filepath.Join(dir, "summary.md")
	unrunJobs := r.unrunJobs
	if unrunJobs == "" {
		unrunJobs = ciUnrunJobs
	}
	// GitHub's own, which a test run on GitHub would otherwise pass on.
	event, ref := "push", r.ref
	if r.pr != "" {
		event, ref = "pull_request", "refs/pull/"+r.pr+"/merge"
	} else if ref == "" {
		ref = "refs/heads/main"
	}
	cmd := exec.Command(bash, "ci-report.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH="+dir,
		"GITHUB_REPOSITORY=owner/repo",
		"GITHUB_RUN_ID=100",
		"GITHUB_RUN_ATTEMPT="+strconv.Itoa(r.attempt),
		"GITHUB_SERVER_URL=https://github.example.invalid",
		"GITHUB_STEP_SUMMARY="+summary,
		"GITHUB_OUTPUT="+filepath.Join(dir, "outputs"),
		"NEEDS="+r.needs,
		"PR="+r.pr,
		"SHA=abcdef0123456789",
		"SELF=All tests passed",
		"CALLED=^(Build|Release|Deploy site)( / |$)",
		"UNRUN_JOBS="+unrunJobs,
		"GITHUB_EVENT_NAME="+event,
		"GITHUB_REF="+ref,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ci-report.sh: %v\n%s", err, out)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	return ciReportResult{out: string(out), summary: read("summary.md"), outputs: read("outputs"), calls: read("calls")}
}

// wants fails the test for each of the strings that s does not hold.
func wants(t *testing.T, what, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			t.Errorf("%s lacks %q:\n%s", what, sub, s)
		}
	}
}

// lacks fails the test for each of the strings that s holds.
func lacks(t *testing.T, what, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			t.Errorf("%s holds %q:\n%s", what, sub, s)
		}
	}
}

const passedNeeds = `{"changes":{"result":"success"},"test":{"result":"success"},"android":{"result":"success"},"platforms":{"result":"success"},"fuzz":{"result":"skipped"}}`

// TestCIReportFirstAttempt: a push's first attempt reads the latest jobs
// alone, lists them, and comments nowhere; this job and a called
// workflow's are left out.
func TestCIReportFirstAttempt(t *testing.T) {
	res := runCIReport(t, ciReportRun{
		attempt: 1,
		needs:   passedNeeds,
		latest: []ciJob{
			ran(1, 1, "What to test", "success", 0),
			ran(2, 1, "Vet and test", "success", 1),
			notRun(3, 1, "Fuzz", "skipped"),
			ran(4, 1, "Build / Binaries and packages", "success", 1),
			running(5, 1),
		},
	})
	wants(t, "summary", res.summary, "### Every test job passed or was skipped\n",
		"| Vet and test | passed |", "| Fuzz | skipped |")
	lacks(t, "summary", res.summary, "Build", "| All tests passed", "re-run")
	if want := "GET repos/owner/repo/actions/runs/100/jobs?filter=latest&per_page=100\n"; res.calls != want {
		t.Errorf("calls = %q, want %q", res.calls, want)
	}
	lacks(t, "output", res.out, "::")
}

// rerunJobs is a run whose first attempt had Android app fail and macOS
// LaunchDaemon cancelled before a runner took it, both re-run and passed
// in the second, the jobs that passed carried into it with their start.
func rerunJobs() (latest, all []ciJob) {
	first := []ciJob{
		ran(1, 1, "What to test", "success", 0),
		ran(2, 1, "Vet and test", "success", 1),
		ran(3, 1, "Android app", "failure", 1),
		notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled"),
		notRun(5, 1, "Fuzz", "skipped"),
		ran(6, 1, "All tests passed", "failure", 9),
	}
	latest = []ciJob{
		ran(11, 2, "What to test", "success", 0),
		ran(12, 2, "Vet and test", "success", 1),
		ran(13, 2, "Android app", "success", 20),
		ran(14, 2, "Platforms / macOS LaunchDaemon", "success", 20),
		notRun(15, 2, "Fuzz", "skipped"),
		running(16, 2),
	}
	return latest, append(first, latest...)
}

// TestCIReportRerunOnAPullRequest: the jobs that passed only on a re-run
// count as passed, and are named in the summary with a warning, in the
// pull request's report, which is edited, and in a comment of their own,
// which notifies; a comment of someone else's with the marker is not the
// report.
func TestCIReportRerunOnAPullRequest(t *testing.T) {
	latest, all := rerunJobs()
	res := runCIReport(t, ciReportRun{
		attempt: 2, pr: "7", needs: passedNeeds, latest: latest, all: all,
		comments: `[{"id":11,"user":{"login":"github-actions[bot]"},"body":"<!-- ci-report -->\n**1 test job(s) did not pass**"},
			{"id":12,"user":{"login":"someone"},"body":"<!-- ci-report --> quoted"}]`,
	})
	wants(t, "summary", res.summary,
		"### Every test job passed or was skipped, 2 only on a re-run\n",
		"| Android app | passed |",
		"Passed only on a re-run, and counted as passed:\n\n"+
			"- Android app: attempt 1 failed, passed in attempt 2\n"+
			"- Platforms / macOS LaunchDaemon: attempt 1 cancelled before a runner took it, passed in attempt 2\n")
	lacks(t, "summary", res.summary, "- Vet and test", "Warning")
	wants(t, "output", res.out,
		"::warning::Android app passed only on a re-run: attempt 1 failed, passed in attempt 2\n",
		"::warning::Platforms / macOS LaunchDaemon passed only on a re-run: attempt 1 cancelled before a runner took it, passed in attempt 2\n")
	lacks(t, "output", res.out, "::error::")
	wants(t, "calls", res.calls,
		"GET repos/owner/repo/actions/runs/100/jobs?filter=all&per_page=100\n",
		"PATCH repos/owner/repo/issues/comments/11\n<!-- ci-report -->\n"+
			"**Every test job passed or was skipped, 2 only on a re-run** on abcdef0, "+
			"[run 100 attempt 2](https://github.example.invalid/owner/repo/actions/runs/100).\n",
		"| Android app | attempt 1 failed | passed in attempt 2 |\n",
		"POST repos/owner/repo/issues/7/comments\n<!-- ci-rerun -->\n**2 job(s) passed only on a re-run** on abcdef0",
		"| Platforms / macOS LaunchDaemon | attempt 1 cancelled before a runner took it | passed in attempt 2 |\n")
	lacks(t, "calls", res.calls, "comments/12")
	if n := strings.Count(res.calls, "POST "); n != 1 {
		t.Errorf("%d comments written, want the re-run's alone:\n%s", n, res.calls)
	}
}

// TestCIReportRerunOnAPush: a push to main names the re-run job in its
// summary and annotations and comments nowhere.
func TestCIReportRerunOnAPush(t *testing.T) {
	latest, all := rerunJobs()
	res := runCIReport(t, ciReportRun{attempt: 2, needs: passedNeeds, latest: latest, all: all})
	wants(t, "summary", res.summary, "- Android app: attempt 1 failed, passed in attempt 2\n")
	wants(t, "output", res.out, "::warning::Android app passed only on a re-run")
	lacks(t, "calls", res.calls, "comments", "PATCH", "POST")
}

// TestCIReportRerunAgain: a third attempt after the second passed names
// the job that needed the re-run again, with the attempt it passed in,
// in the summary and the report, but tells no one again.
func TestCIReportRerunAgain(t *testing.T) {
	_, all := rerunJobs()
	third := []ciJob{
		ran(21, 3, "What to test", "success", 30),
		ran(22, 3, "Vet and test", "success", 31),
		ran(23, 3, "Android app", "success", 20), // carried, as it started in attempt 2
		ran(24, 3, "Platforms / macOS LaunchDaemon", "success", 32),
		notRun(25, 3, "Fuzz", "skipped"),
		running(26, 3),
	}
	res := runCIReport(t, ciReportRun{
		attempt: 3, pr: "7", needs: passedNeeds, latest: third, all: append(all, third...),
		comments: `[{"id":11,"user":{"login":"github-actions[bot]"},"body":"<!-- ci-report -->\nan earlier one"}]`,
	})
	wants(t, "summary", res.summary,
		"- Android app: attempt 1 failed, passed in attempt 2\n",
		"- Platforms / macOS LaunchDaemon: attempt 1 cancelled before a runner took it, passed in attempt 2\n")
	wants(t, "calls", res.calls, "PATCH repos/owner/repo/issues/comments/11\n")
	lacks(t, "calls", res.calls, "POST", "ci-rerun")
}

// TestCIReportCarriedFailure: a job that failed, was carried into the
// next attempt without a re-run and passed in the one after counts its
// failure once, and its re-run tells the pull request.
func TestCIReportCarriedFailure(t *testing.T) {
	all := []ciJob{
		ran(1, 1, "Race", "failure", 1),
		ran(2, 1, "Android app", "failure", 1),
		ran(11, 2, "Race", "failure", 1), // carried: the second attempt re-ran Android app alone
		ran(12, 2, "Android app", "success", 20),
	}
	third := []ciJob{
		ran(21, 3, "Race", "success", 40),
		ran(22, 3, "Android app", "success", 20),
		running(23, 3),
	}
	res := runCIReport(t, ciReportRun{attempt: 3, pr: "7", needs: passedNeeds, latest: third, all: append(all, third...)})
	wants(t, "summary", res.summary,
		"- Android app: attempt 1 failed, passed in attempt 2\n",
		"- Race: attempt 1 failed, passed in attempt 3\n")
	wants(t, "calls", res.calls, "<!-- ci-rerun -->\n**1 job(s) passed only on a re-run**",
		"| Race | attempt 1 failed | passed in attempt 3 |\n")
	// The one comment is the re-run's, and Android app's re-run told before.
	lacks(t, "calls", res.calls, "<!-- ci-report -->", "| Android app |")
}

// TestCIReportCommentsFail: comments that cannot be written leave a
// warning, in the summary too, and the script still exits 0; a job that
// failed in both attempts is no re-run.
func TestCIReportCommentsFail(t *testing.T) {
	latest, all := rerunJobs()
	latest = append(latest, ran(17, 2, "Race", "failure", 21))
	all = append(all, ran(7, 1, "Race", "failure", 2), ran(17, 2, "Race", "failure", 21))
	res := runCIReport(t, ciReportRun{
		attempt: 2, pr: "7", latest: latest, all: all, failWrites: true,
		needs: `{"test":{"result":"success"},"race":{"result":"failure"}}`,
	})
	wants(t, "summary", res.summary, "### 1 test job(s) did not pass\n", "| Race | failed |",
		"- Android app: attempt 1 failed, passed in attempt 2\n",
		"- **Warning:** no report on the pull request (a read-only token?).\n",
		"- **Warning:** the pull request was not told of the re-run (a read-only token?).\n")
	lacks(t, "summary", res.summary, "- Race")
	wants(t, "output", res.out, "::error::Race: failed\n",
		"::warning::no report on the pull request", "::warning::the pull request was not told of the re-run")
	wants(t, "calls", res.calls, "POST repos/owner/repo/issues/7/comments\n")
}

// TestCIReportFirstFailure: a pull request's first failing attempt writes
// the report, with the jobs that did not pass, and reads no earlier
// attempt.
func TestCIReportFirstFailure(t *testing.T) {
	res := runCIReport(t, ciReportRun{
		attempt: 1, pr: "7",
		needs: `{"test":{"result":"success"},"android":{"result":"failure"}}`,
		latest: []ciJob{
			ran(2, 1, "Vet and test", "success", 1),
			ran(3, 1, "Android app", "failure", 1),
			running(6, 1),
		},
	})
	wants(t, "calls", res.calls, "POST repos/owner/repo/issues/7/comments\n<!-- ci-report -->\n**1 test job(s) did not pass**",
		"| Android app | failed |\n")
	lacks(t, "calls", res.calls, "filter=all", "ci-rerun")
}

// TestCIReportEarlierAttemptsUnread: where the earlier attempts cannot be
// read, the summary says that a re-run job goes unnamed.
func TestCIReportEarlierAttemptsUnread(t *testing.T) {
	latest, _ := rerunJobs()
	res := runCIReport(t, ciReportRun{attempt: 2, needs: passedNeeds, latest: latest})
	wants(t, "summary", res.summary, "### Every test job passed or was skipped\n",
		"- **Warning:** the earlier attempts' jobs could not be read: a job that passed only on a re-run is not named.\n")
}

// TestCIReportEarlierAttemptsUnreadable: earlier attempts whose jobs the
// script cannot make sense of, here one without a name, leave the same
// warning, and the summary and log are those of a run without a re-run.
func TestCIReportEarlierAttemptsUnreadable(t *testing.T) {
	latest, all := rerunJobs()
	jobs := make([]any, 0, len(all)+1)
	for _, j := range all {
		jobs = append(jobs, j)
	}
	jobs = append(jobs, map[string]any{"id": 9, "name": nil, "run_attempt": 1,
		"status": "completed", "conclusion": "failure", "started_at": "2026-11-18T10:13:00Z"})
	res := runCIReport(t, ciReportRun{attempt: 2, pr: "7", needs: passedNeeds, latest: latest,
		allJSON: map[string]any{"jobs": jobs}})
	wants(t, "summary", res.summary, "### Every test job passed or was skipped\n",
		"- **Warning:** the earlier attempts' jobs could not be read: a job that passed only on a re-run is not named.\n")
	lacks(t, "summary", res.summary, "re-run, and counted", "only on a re-run\n")
	lacks(t, "output", res.out, "integer expression", "::warning::Android app")
	lacks(t, "calls", res.calls, "ci-rerun")
}

// platformsNeeds is a run's needs with Platforms' result as given, the
// result GitHub gives the caller of platforms.yml's two jobs together.
func platformsNeeds(result string) string {
	return `{"changes":{"result":"success"},"test":{"result":"success"},"platforms":{"result":"` +
		result + `"},"fuzz":{"result":"skipped"}}`
}

// unrunNeeds is a run's needs where no runner took macOS LaunchDaemon:
// abandoned, as GitHub gave a job of ci.yml's own (runs 44151208458 and
// 44335801523).
var unrunNeeds = platformsNeeds("abandoned")

// unrunJobs is a first attempt whose macOS LaunchDaemon no runner took,
// or, with started, one that a runner took before it was cancelled.
func unrunJobs(started bool) []ciJob {
	macos := notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled")
	if started {
		macos = ran(4, 1, "Platforms / macOS LaunchDaemon", "cancelled", 2)
	}
	return []ciJob{
		ran(1, 1, "What to test", "success", 0),
		ran(2, 1, "Vet and test", "success", 1),
		ran(3, 1, "Platforms / Windows service", "success", 1),
		macos,
		notRun(5, 1, "Fuzz", "skipped"),
		running(6, 1),
	}
}

// TestCIReportUnrunOnAPullRequest: on a pull request, a macOS job no
// runner took counts as not run (AP TestSpeed, D5-A), its caller
// abandoned, cancelled or failed among the needs, as a called workflow's
// may be: its caller's id is the output unrun, the summary names it with
// a warning and passes, and no report is written for it alone.
func TestCIReportUnrunOnAPullRequest(t *testing.T) {
	for _, result := range []string{"abandoned", "cancelled", "failure"} {
		t.Run(result, func(t *testing.T) {
			testUnrunOnAPullRequest(t, platformsNeeds(result))
		})
	}
}

// testUnrunOnAPullRequest is TestCIReportUnrunOnAPullRequest for the
// needs given.
func testUnrunOnAPullRequest(t *testing.T, needs string) {
	t.Helper()
	res := runCIReport(t, ciReportRun{attempt: 1, pr: "7", needs: needs, latest: unrunJobs(false)})
	if res.outputs != "unrun=platforms\n" {
		t.Errorf("outputs = %q, want unrun=platforms", res.outputs)
	}
	wants(t, "summary", res.summary,
		"### Every test job passed or was skipped, 1 not run for want of a runner\n",
		"| Platforms / macOS LaunchDaemon | not run, as no runner took it |",
		"Not run, as no runner took them; the Tested record leaves them out, and the push of the merged tree runs them:\n\n"+
			"- Platforms / macOS LaunchDaemon\n")
	wants(t, "output", res.out,
		"::warning::Platforms / macOS LaunchDaemon: no runner took it, so it did not run on this pull request; the push of the merged tree runs it\n")
	lacks(t, "output", res.out, "::error::")
	lacks(t, "summary", res.summary, "Retry")
	lacks(t, "calls", res.calls, "POST", "PATCH")
}

// TestCIReportUnrunInTheReport: a pull request's report, where there is
// one, names the job not run, and the run passes.
func TestCIReportUnrunInTheReport(t *testing.T) {
	res := runCIReport(t, ciReportRun{attempt: 1, pr: "7", needs: unrunNeeds, latest: unrunJobs(false),
		comments: `[{"id":11,"user":{"login":"github-actions[bot]"},"body":"<!-- ci-report -->\nan earlier one"}]`})
	wants(t, "calls", res.calls,
		"PATCH repos/owner/repo/issues/comments/11\n<!-- ci-report -->\n"+
			"**Every test job passed or was skipped, 1 not run for want of a runner** on abcdef0",
		"Not run, as no runner took them; the Tested record leaves them out, and the push of the merged tree runs them:\n\n"+
			"- Platforms / macOS LaunchDaemon\n")
	lacks(t, "calls", res.calls, "| Platforms / macOS LaunchDaemon |", "POST")
}

// TestCIReportUnrunOnMain: on a push to main, a merge's included, a job no
// runner took counts as not run too (AP TestSpeed, D16-A): the run passes,
// and the summary says that the nightly build runs it.
func TestCIReportUnrunOnMain(t *testing.T) {
	for _, result := range []string{"abandoned", "cancelled", "failure"} {
		t.Run(result, func(t *testing.T) {
			res := runCIReport(t, ciReportRun{attempt: 1, needs: platformsNeeds(result), latest: unrunJobs(false)})
			if res.outputs != "unrun=platforms\n" {
				t.Errorf("outputs = %q, want unrun=platforms", res.outputs)
			}
			wants(t, "summary", res.summary,
				"### Every test job passed or was skipped, 1 not run for want of a runner\n",
				"| Platforms / macOS LaunchDaemon | not run, as no runner took it |",
				"Not run, as no runner took them; the Tested record says so, and the nightly build runs them before it ships this code:\n\n"+
					"- Platforms / macOS LaunchDaemon\n")
			wants(t, "output", res.out,
				"::warning::Platforms / macOS LaunchDaemon: no runner took it, so it did not run on this push to main; the nightly build runs it before it ships this code\n")
			lacks(t, "output", res.out, "::error::")
			lacks(t, "calls", res.calls, "comments", "POST", "PATCH")
		})
	}
}

// TestCIReportUnrunOnATag: on a tag a job no runner took fails as before,
// in the first attempt as in any; the summary says that Retry runs it
// again.
func TestCIReportUnrunOnATag(t *testing.T) {
	res := runCIReport(t, ciReportRun{attempt: 1, needs: unrunNeeds, latest: unrunJobs(false), ref: "refs/tags/v1.2.3-alpha.1"})
	if res.outputs != "unrun=\n" {
		t.Errorf("outputs = %q, want unrun empty", res.outputs)
	}
	wants(t, "summary", res.summary, "### 1 test job(s) did not pass\n",
		"| Platforms / macOS LaunchDaemon | cancelled before a runner took it |",
		"No runner took them, which fails a tag's run; where nothing else failed, Retry (retry.yml) runs them again, "+
			"at most twice, about 20 minutes after the first attempt and after the second:\n\n- Platforms / macOS LaunchDaemon\n")
	wants(t, "output", res.out, "::error::Platforms / macOS LaunchDaemon: cancelled before a runner took it\n")
	lacks(t, "summary", res.summary, "not run")
	lacks(t, "output", res.out, "::warning::")
}

// TestCIReportRerunOnATag: Retry's re-run of a tag's run, in which a
// runner took the job, passes, and names the job that passed only on it.
func TestCIReportRerunOnATag(t *testing.T) {
	first := unrunJobs(false)
	first[5] = ran(6, 1, "All tests passed", "failure", 9)
	second := []ciJob{
		ran(1, 2, "What to test", "success", 0),
		ran(2, 2, "Vet and test", "success", 1),
		ran(3, 2, "Platforms / Windows service", "success", 1),
		ran(14, 2, "Platforms / macOS LaunchDaemon", "success", 40),
		notRun(5, 2, "Fuzz", "skipped"),
		running(16, 2),
	}
	res := runCIReport(t, ciReportRun{attempt: 2, needs: platformsNeeds("success"), latest: second,
		all: append(first, second...), ref: "refs/tags/v1.2.3-alpha.1"})
	wants(t, "summary", res.summary, "### Every test job passed or was skipped, 1 only on a re-run\n",
		"- Platforms / macOS LaunchDaemon: attempt 1 cancelled before a runner took it, passed in attempt 2\n")
	lacks(t, "output", res.out, "::error::")
	lacks(t, "summary", res.summary, "Retry")
}

// TestCIReportCancelledAfterItStarted: a macOS job cancelled once a
// runner had taken it fails on a pull request and on main too.
func TestCIReportCancelledAfterItStarted(t *testing.T) {
	for name, pr := range map[string]string{"pull request": "7", "main": ""} {
		t.Run(name, func(t *testing.T) {
			res := runCIReport(t, ciReportRun{attempt: 1, pr: pr, needs: platformsNeeds("cancelled"), latest: unrunJobs(true)})
			if res.outputs != "unrun=\n" {
				t.Errorf("outputs = %q, want unrun empty", res.outputs)
			}
			wants(t, "summary", res.summary, "### 1 test job(s) did not pass\n", "| Platforms / macOS LaunchDaemon | cancelled |")
			wants(t, "output", res.out, "::error::Platforms / macOS LaunchDaemon: cancelled\n")
		})
	}
}

// TestCIReportUnrunOnlyForItsJobs: a job that UNRUN_JOBS does not name,
// or whose caller's other job failed, or whose caller's needs say it
// passed or was skipped, or UNRUN_JOBS that is no object, leaves a job no
// runner took failed; a failed job never passes.
func TestCIReportUnrunOnlyForItsJobs(t *testing.T) {
	windowsFailed := unrunJobs(false)
	windowsFailed[2] = ran(3, 1, "Platforms / Windows service", "failure", 1)
	for name, r := range map[string]ciReportRun{
		"another job":    {unrunJobs: `{"Platforms / Windows service": "platforms"}`},
		"another id":     {unrunJobs: `{"Platforms / macOS LaunchDaemon": "test", "Platforms / Windows service": "platforms"}`},
		"windows failed": {latest: windowsFailed, needs: platformsNeeds("failure")},
		"passed":         {needs: platformsNeeds("success")},
		"no object":      {unrunJobs: `["Platforms / macOS LaunchDaemon"]`},
		"no JSON":        {unrunJobs: `platforms`},
	} {
		t.Run(name, func(t *testing.T) {
			for _, pr := range []string{"7", ""} {
				r.attempt, r.pr = 1, pr
				if r.latest == nil {
					r.latest = unrunJobs(false)
				}
				if r.needs == "" {
					r.needs = unrunNeeds
				}
				res := runCIReport(t, r)
				if res.outputs != "unrun=\n" {
					t.Errorf("pr %q: outputs = %q, want unrun empty", pr, res.outputs)
				}
				wants(t, "output", res.out, "::error::Platforms / macOS LaunchDaemon: cancelled before a runner took it\n")
			}
		})
	}
}

// TestCIReportUnrunBesideAFailure: a job no runner took counts as not run
// beside another job that failed, which fails the run.
func TestCIReportUnrunBesideAFailure(t *testing.T) {
	latest := unrunJobs(false)
	latest[1] = ran(2, 1, "Vet and test", "failure", 1)
	res := runCIReport(t, ciReportRun{attempt: 1, needs: `{"test":{"result":"failure"},"platforms":{"result":"abandoned"}}`, latest: latest})
	if res.outputs != "unrun=platforms\n" {
		t.Errorf("outputs = %q, want unrun=platforms", res.outputs)
	}
	wants(t, "summary", res.summary, "### 1 test job(s) did not pass\n", "| Vet and test | failed |",
		"| Platforms / macOS LaunchDaemon | not run, as no runner took it |")
	wants(t, "output", res.out, "::error::Vet and test: failed\n")
}

// TestCIReportUnrunAfterAFailure: a re-run no runner took does not hide
// a failure: where the job's last run that a runner took, in an earlier
// attempt, failed, it fails, on a pull request and on main, and so it does
// where the earlier attempts cannot be read; a job that passed earlier, or
// that no runner took before either, counts as not run.
func TestCIReportUnrunAfterAFailure(t *testing.T) {
	const mac = "Platforms / macOS LaunchDaemon"
	attempt := func(n int, macos ciJob) []ciJob {
		jobs := unrunJobs(false)
		for i := range jobs {
			jobs[i].RunAttempt = n
		}
		jobs[3] = macos
		jobs[5] = running(int64(6+10*n), n)
		return jobs
	}
	failedThenNotTaken := append(attempt(1, ran(4, 1, mac, "failure", 2)), attempt(2, notRun(14, 2, mac, "cancelled"))...)
	notTakenTwice := append(attempt(1, notRun(4, 1, mac, "cancelled")), attempt(2, notRun(14, 2, mac, "cancelled"))...)
	failedPassedNotTaken := append(append(attempt(1, ran(4, 1, mac, "failure", 2)), attempt(2, ran(14, 2, mac, "success", 20))...),
		attempt(3, notRun(24, 3, mac, "cancelled"))...)
	for _, tc := range []struct {
		name    string
		attempt int
		all     []ciJob
		spared  bool
	}{
		{"failed, then no runner", 2, failedThenNotTaken, false},
		{"earlier attempts unread", 2, nil, false},
		{"no runner twice", 2, notTakenTwice, true},
		{"failed, passed, then no runner", 3, failedPassedNotTaken, true},
	} {
		for _, pr := range []string{"7", ""} {
			latest := attempt(tc.attempt, notRun(int64(4+10*tc.attempt), tc.attempt, mac, "cancelled"))
			res := runCIReport(t, ciReportRun{attempt: tc.attempt, pr: pr, needs: platformsNeeds("failure"), latest: latest, all: tc.all})
			if tc.spared {
				if res.outputs != "unrun=platforms\n" {
					t.Errorf("%s, pr %q: outputs = %q, want unrun=platforms", tc.name, pr, res.outputs)
				}
				lacks(t, tc.name+" output", res.out, "::error::")
				continue
			}
			if res.outputs != "unrun=\n" {
				t.Errorf("%s, pr %q: outputs = %q, want unrun empty", tc.name, pr, res.outputs)
			}
			wants(t, tc.name+" output", res.out, "::error::"+mac+": cancelled before a runner took it\n")
		}
	}
}

// TestCIReportUnrunJobsAreCIJobs: the names and ids ci.yml gives as
// UNRUN_JOBS are those of its jobs: the caller Platforms, which calls
// platforms.yml, and that workflow's jobs, which are all of its jobs, so
// that the report sees every job of the call; a renamed job would
// silently fail a pull request again.
func TestCIReportUnrunJobsAreCIJobs(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	if !strings.Contains(ci, "\n  UNRUN_JOBS: '"+ciUnrunJobs+"'\n") {
		t.Fatalf("ci.yml does not give UNRUN_JOBS as %s", ciUnrunJobs)
	}
	var jobs map[string]string
	if err := json.Unmarshal([]byte(ciUnrunJobs), &jobs); err != nil {
		t.Fatal(err)
	}
	called := workflowJobNames(t, "platforms.yml")
	for name, id := range jobs {
		caller, job, ok := strings.Cut(name, " / ")
		if !ok {
			t.Errorf("%q is no called workflow's job", name)
			continue
		}
		if !strings.Contains(ci, "\n  "+id+":\n    name: "+caller+"\n") {
			t.Errorf("ci.yml has no job %s named %q", id, caller)
		}
		if !strings.Contains(ci, "\n    name: "+caller+"\n    needs: changes\n") ||
			!strings.Contains(ci, "    uses: ./.github/workflows/platforms.yml\n") {
			t.Errorf("ci.yml's %s does not call platforms.yml", id)
		}
		if !called[job] {
			t.Errorf("platforms.yml has no job named %q", job)
		}
	}
	if len(called) != len(jobs) {
		t.Errorf("platforms.yml's jobs %v, UNRUN_JOBS %v: every job of the call must be one of UNRUN_JOBS", called, jobs)
	}
}

// readWorkflow returns the workflow file of .github/workflows given.
func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// workflowJobNames returns the names of a workflow's jobs, the lines
// "    name: " that follow a job's id at two spaces.
func workflowJobNames(t *testing.T, name string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	lines := strings.Split(readWorkflow(t, name), "\n")
	jobs := false
	for i, line := range lines {
		if line == "jobs:" {
			jobs = true
			continue
		}
		if !jobs || i+1 >= len(lines) || !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") ||
			strings.HasPrefix(line, "  #") || !strings.HasSuffix(line, ":") {
			continue
		}
		if n, ok := strings.CutPrefix(lines[i+1], "    name: "); ok {
			names[n] = true
		}
	}
	if len(names) == 0 {
		t.Fatalf("%s has no named jobs", name)
	}
	return names
}

// ciGateLines open the lines of ci.yml's What to test that make the macOS
// and Windows gates (AP TestSpeed, D1-A to D3-A): the announcement's
// paths, the Go files each platform leaves out by name, and the two lists.
var ciGateLines = []string{"announce=", "notmac=", "macfiles=", "winfiles="}

// ciWhatLines returns the lines of ci.yml that open with the prefixes
// given, in the file's order, one line each: the parts of What to test's
// script that a test runs.
func ciWhatLines(t *testing.T, prefixes []string) string {
	t.Helper()
	var script strings.Builder
	found := map[string]bool{}
	for _, line := range strings.Split(readWorkflow(t, "ci.yml"), "\n") {
		line = strings.TrimSpace(line)
		for _, p := range prefixes {
			if strings.HasPrefix(line, p) {
				if found[p] {
					t.Fatalf("ci.yml has two lines opening with %s", p)
				}
				found[p] = true
				script.WriteString(line + "\n")
			}
		}
	}
	for _, p := range prefixes {
		if !found[p] {
			t.Fatalf("ci.yml has no line opening with %s", p)
		}
	}
	return script.String()
}

// ciPlatformGates runs ci.yml's macOS and Windows gates over the paths
// given, as What to test does on a pull request, and returns the paths
// each lets through and those each leaves out by name.
func ciPlatformGates(t *testing.T, paths []string) (mac, win, notMac, notWin map[string]bool) {
	t.Helper()
	bash := scriptShell(t)
	var script strings.Builder
	script.WriteString("set -euo pipefail\nchanged=\"$(cat)\"\n")
	script.WriteString(ciWhatLines(t, ciGateLines))
	script.WriteString(`out() { while IFS= read -r f; do [ -z "$f" ] || echo "$1 $f"; done; }
out mac <<<"$macfiles"
out win <<<"$winfiles"
grep -E "$notmac" <<<"$changed" | out notmac || true
grep -E "$notwin" <<<"$changed" | out notwin || true
`)
	cmd := exec.Command(bash, "-c", script.String())
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the gates: %v\n%s", err, out)
	}
	sets := map[string]map[string]bool{"mac": {}, "win": {}, "notmac": {}, "notwin": {}}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		set, path, ok := strings.Cut(line, " ")
		if !ok || sets[set] == nil {
			t.Fatalf("the gates printed %q", line)
		}
		sets[set][path] = true
	}
	return sets["mac"], sets["win"], sets["notmac"], sets["notwin"]
}

// goFiles lists the Go files under cmd/ and internal/, as paths from the
// repository's root, leaving out the directories go/build leaves out.
func goFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, top := range []string{"cmd", "internal"} {
		root := filepath.Join("..", top)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if path != root && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(name, ".go") {
				rel, err := filepath.Rel("..", path)
				if err != nil {
					return err
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// builtFor reports whether go/build takes the file, a path from the
// repository's root, into its package for the system given.
func builtFor(t *testing.T, goos, goarch, file string) bool {
	t.Helper()
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, goarch, false
	ok, err := ctx.MatchFile(filepath.Join("..", filepath.FromSlash(filepath.Dir(file))), filepath.Base(file))
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return ok
}

// TestCIGatesHoldPlatformFiles: every Go file under cmd/ and internal/
// that is built for darwin and not for linux runs the macOS job, and for
// windows the Windows job, since Linux's tests never build it; and a file
// a gate leaves out by its name (notmac, notwin) is not built for that
// platform, _unix by its build tag, which Go's file names do not give.
func TestCIGatesHoldPlatformFiles(t *testing.T) {
	files := goFiles(t)
	mac, win, notMac, notWin := ciPlatformGates(t, files)
	var darwinOnly, windowsOnly int
	for _, f := range files {
		linux := builtFor(t, "linux", "amd64", f)
		darwin := builtFor(t, "darwin", "arm64", f)
		windows := builtFor(t, "windows", "amd64", f)
		if darwin && !linux {
			darwinOnly++
			if !mac[f] {
				t.Errorf("%s is built for darwin and not for linux, and the macOS gate does not hold it", f)
			}
		}
		if windows && !linux {
			windowsOnly++
			if !win[f] {
				t.Errorf("%s is built for windows and not for linux, and the Windows gate does not hold it", f)
			}
		}
		if notMac[f] && darwin {
			t.Errorf("%s is built for darwin, and the macOS gate leaves it out by its name", f)
		}
		if notWin[f] && windows {
			t.Errorf("%s is built for windows, and the Windows gate leaves it out by its name", f)
		}
	}
	if darwinOnly == 0 || windowsOnly == 0 || len(notWin) == 0 {
		t.Fatalf("%d files for darwin alone, %d for windows alone, %d left out of Windows by name: the walk found too little",
			darwinOnly, windowsOnly, len(notWin))
	}
}

// TestCIGatesHoldTheirWorkflow: a pull request that changes the jobs'
// own workflow, platforms.yml, or ci.yml, which calls it, runs both; the
// nightly workflow, which runs them on main alone, runs neither.
func TestCIGatesHoldTheirWorkflow(t *testing.T) {
	paths := []string{".github/workflows/platforms.yml", ".github/workflows/ci.yml", ".github/workflows/nightly.yml"}
	mac, win, _, _ := ciPlatformGates(t, paths)
	for _, p := range paths[:2] {
		if !mac[p] || !win[p] {
			t.Errorf("%s: macOS %v, Windows %v; want both", p, mac[p], win[p])
		}
	}
	if mac[paths[2]] || win[paths[2]] {
		t.Errorf("%s runs a platform's job", paths[2])
	}
}

// TestCIGatesHoldTheWindowsTests: a pull request that changes the
// start's guard or the other tests that run the built massimo.exe on
// Windows, the notes' and the reload's, or the settings file's parser,
// which Windows reads otherwise, names in any case, runs the Windows job
// (AP StartConfiguration, D22-A, D23-A); the guard's neighbours do not.
func TestCIGatesHoldTheWindowsTests(t *testing.T) {
	asks := []string{"cmd/massimo/startchecks.go", "cmd/massimo/startchecks_test.go", "cmd/massimo/serve_stop_test.go",
		"cmd/massimo/notes_test.go", "cmd/massimo/reload.go", "cmd/massimo/reload_test.go",
		"internal/env/env.go", "internal/env/dotenv.go", "internal/env/dotenv_windows_test.go"}
	not := []string{"cmd/massimo/config.go", "cmd/massimo/serve_test.go", "internal/server/restart_api.go"}
	_, win, _, _ := ciPlatformGates(t, append(slices.Clone(asks), not...))
	for _, p := range asks {
		if !win[p] {
			t.Errorf("%s does not run the Windows job", p)
		}
	}
	for _, p := range not {
		if win[p] {
			t.Errorf("%s runs the Windows job", p)
		}
	}
}

// ciUILines open the lines of ci.yml's What to test that answer ui on a
// pull request: the paths the UI tests read, and the answer.
var ciUILines = []string{"uifiles=", `[ -n "$uifiles" ] || ui=false`}

// ciUIGate runs ci.yml's UI gate over a pull request's paths, as What to
// test does, and returns its answer ui.
func ciUIGate(t *testing.T, paths []string) string {
	t.Helper()
	bash := scriptShell(t)
	script := "set -euo pipefail\nchanged=\"$(cat)\"\nui=true\n" + ciWhatLines(t, ciUILines) + "echo \"$ui\"\n"
	cmd := exec.Command(bash, "-c", script)
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the UI gate: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestCIUIGateLeavesOutServerTests: a pull request that changes the
// server's tests alone runs no UI job, since the binary under test does
// not hold them (AP TestSpeed, D9-A); one that changes a server Go file
// beside them, the page, the suite or the jobs' workflow runs them.
func TestCIUIGateLeavesOutServerTests(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		{[]string{"internal/server/rooms_test.go"}, "false"},
		{[]string{"internal/server/rooms_test.go", "internal/server/api_test.go", "internal/store/store.go"}, "false"},
		{[]string{"internal/server/rooms_test.go", "internal/server/rooms.go"}, "true"},
		{[]string{"internal/server/rooms.go"}, "true"},
		{[]string{"internal/server/ui/app.js"}, "true"},
		{[]string{"internal/server/ui/src/rooms/tile.tsx", "internal/server/rooms_test.go"}, "true"},
		{[]string{"tools/ui-test/test_parts_guard.py"}, "true"},
		{[]string{".github/workflows/ci.yml"}, "true"},
		{[]string{"docs/api-v1-openapi.yaml"}, "true"},
		{[]string{"internal/server/testdata/greeting.txt"}, "false"},
		{[]string{"internal/store/store.go", "docs/ui.md"}, "false"},
	} {
		if got := ciUIGate(t, tc.paths); got != tc.want {
			t.Errorf("%v: ui=%s, want %s", tc.paths, got, tc.want)
		}
	}
}

// testEvent is a line of `go test -json`.
type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package,omitempty"`
	Test    string `json:"Test,omitempty"`
	Output  string `json:"Output,omitempty"`
}

// out is an output event of a package's test, or of the package itself
// where test is empty.
func out(pkg, test, s string) testEvent { return testEvent{"output", pkg, test, s} }

// timedEvent is a line of `go test -json` that ends a test or a package,
// with the seconds it took.
type timedEvent struct {
	testEvent
	Elapsed float64 `json:"Elapsed"`
}

// ended is the end of a package's test, or of the package itself where
// test is empty, as the action (pass, fail, skip), after s seconds.
func ended(action, pkg, test string, s float64) timedEvent {
	return timedEvent{testEvent{action, pkg, test, ""}, s}
}

// stream is the lines of `go test -json` for the events, each line given
// by extra after the event at its index put in after it as it is.
func stream[E any](t *testing.T, events []E, extra map[int]string) string {
	t.Helper()
	var in strings.Builder
	for i, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		in.Write(line)
		in.WriteByte('\n')
		if s, ok := extra[i]; ok {
			in.WriteString(s)
		}
	}
	return in.String()
}

// runGoTestReport runs packaging/go-test-report.sh on the stream, with the
// environment's additions, and returns its log and the summary it wrote.
func runGoTestReport(t *testing.T, in, title string, env ...string) (log, summary string) {
	t.Helper()
	bash := scriptShell(t)
	file := filepath.Join(t.TempDir(), "summary.md")
	args := []string{"go-test-report.sh"}
	if title != "" {
		args = append(args, title)
	}
	cmd := exec.Command(bash, args...)
	cmd.Stdin = strings.NewReader(in)
	cmd.Env = append(append(os.Environ(), "GITHUB_STEP_SUMMARY="+file), env...)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go-test-report.sh: %v\n%s", err, got)
	}
	s, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(got), string(s)
}

// TestGoTestReport: go-test-report.sh writes the log as go test writes it,
// a failed test's output as -v does, while the stream comes; a test that
// never ended, as one that ran into go test's -timeout, comes out at its
// package's end, before the package's line; each skipped test is named
// with its reason in the summary.
func TestGoTestReport(t *testing.T) {
	const a, b, c = "example.invalid/m/a", "example.invalid/m/b", "example.invalid/m/c"
	events := []testEvent{
		{"start", a, "", ""},
		{"run", a, "TestPass", ""},
		out(a, "TestPass", "=== RUN   TestPass\n"),
		out(a, "TestPass", "    a_test.go:5: quiet\n"),
		out(a, "TestPass", "--- PASS: TestPass (0.00s)\n"),
		{"pass", a, "TestPass", ""},
		out(a, "TestSkip", "=== RUN   TestSkip\n"),
		out(a, "TestSkip", "    a_test.go:6: no 127.0.0.2 | here\n"),
		out(a, "TestSkip", "--- SKIP: TestSkip (0.00s)\n"),
		{"skip", a, "TestSkip", ""},
		out(a, "TestFail", "=== RUN   TestFail\n"),
		out(a, "TestFail", "    a_test.go:7: boom\n"),
		out(a, "TestFail", "--- FAIL: TestFail (0.00s)\n"),
		{"fail", a, "TestFail", ""}, // 13
		out(a, "TestSub/one", "=== RUN   TestSub/one\n"),
		out(a, "TestSub/one", "    a_test.go:9: sub 1\n"),
		out(a, "TestSub/one", "    --- SKIP: TestSub/one (0.00s)\n"),
		{"skip", a, "TestSub/one", ""},
		out(a, "TestSub", "--- PASS: TestSub (0.00s)\n"),
		{"pass", a, "TestSub", ""},
		out(a, "", "FAIL\n"),
		out(a, "", "FAIL\texample.invalid/m/a\t0.01s\n"),
		{"fail", a, "", ""},
		out(b, "TestHang", "=== RUN   TestHang\n"),
		out(b, "TestHang", "    b_test.go:8: waiting\n"),
		out(b, "", "2026/11/18 10:00:00 INFO a line of the package's own\n"),
		out(b, "TestHang", "panic: test timed out after 2s\n"),
		out(b, "", "FAIL\texample.invalid/m/b\t2.01s\n"),
		{"fail", b, "", ""},
		out(c, "", "PASS\n"),
		out(c, "", "ok  \texample.invalid/m/c\t0.01s\n"),
		{"pass", c, "", ""}, // 31
		{"build-output", "", "", "# example.invalid/m/d\n"},
	}
	log, summary := runGoTestReport(t, stream(t, events, map[int]string{
		13: "go: a line that is not go test's\n",
		31: `[1, 2]` + "\n" + `{"Action":"output","Package":"x","Test":"T","Output":5}` + "\n",
	}), "make race")
	want := "=== RUN   TestFail\n    a_test.go:7: boom\n--- FAIL: TestFail (0.00s)\n" +
		"go: a line that is not go test's\n" +
		"FAIL\nFAIL\texample.invalid/m/a\t0.01s\n" +
		"2026/11/18 10:00:00 INFO a line of the package's own\n" +
		"=== RUN   TestHang\n    b_test.go:8: waiting\npanic: test timed out after 2s\n" +
		"FAIL\texample.invalid/m/b\t2.01s\n" +
		"ok  \texample.invalid/m/c\t0.01s\n" +
		"[1, 2]\n" + `{"Action":"output","Package":"x","Test":"T","Output":5}` + "\n" +
		"# example.invalid/m/d\n"
	if log != want {
		t.Errorf("log:\n%s\nwant:\n%s", log, want)
	}
	wantSummary := "### Skipped tests, make race: 2\n\n" +
		"| Package | Test | Why |\n|---------|------|-----|\n" +
		"| example.invalid/m/a | TestSkip | a_test.go:6: no 127.0.0.2 \\| here |\n" +
		"| example.invalid/m/a | TestSub/one | a_test.go:9: sub 1 |\n\n"
	if summary != wantSummary {
		t.Errorf("summary:\n%s\nwant:\n%s", summary, wantSummary)
	}

	// No test at all: none skipped, and nothing in the log.
	log, summary = runGoTestReport(t, "", "")
	if log != "" || summary != "### Skipped tests, go test: 0\n\n" {
		t.Errorf("on nothing: log %q, summary %q", log, summary)
	}
}

// TestGoTestReportHoldsALimit: a failed test's output is held up to its
// first and last lines, the number left out said between them.
func TestGoTestReportHoldsALimit(t *testing.T) {
	const p = "example.invalid/m/a"
	events := []testEvent{out(p, "TestLong", "=== RUN   TestLong\n")}
	for i := 1; i <= 9; i++ {
		events = append(events, out(p, "TestLong", "line "+strconv.Itoa(i)+"\n"))
	}
	events = append(events, out(p, "TestLong", "--- FAIL: TestLong (0.00s)\n"), testEvent{"fail", p, "TestLong", ""})
	log, _ := runGoTestReport(t, stream(t, events, nil), "", "GO_TEST_REPORT_FIRST=3", "GO_TEST_REPORT_LAST=2")
	want := "=== RUN   TestLong\nline 1\nline 2\n" +
		"go-test-report: 6 line(s) of this test left out\n" +
		"line 9\n--- FAIL: TestLong (0.00s)\n"
	if log != want {
		t.Errorf("log:\n%s\nwant:\n%s", log, want)
	}
}

// TestGoTestReportReadingDies: where the reading of the stream dies, the
// whole stream goes to the log at its end, as go test -v writes it, and
// the summary says the skips were not read. A jq that stops after three
// lines stands in for the reading.
func TestGoTestReportReadingDies(t *testing.T) {
	real, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("no jq")
	}
	bin := t.TempDir()
	dying := "#!/usr/bin/env bash\n" +
		"case \" $* \" in *' --unbuffered '*) for _ in 1 2 3; do IFS= read -r _; done; exit 5 ;; esac\n" +
		"exec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "jq"), []byte(dying), 0o755); err != nil {
		t.Fatal(err)
	}
	const p = "example.invalid/m/a"
	events := []testEvent{
		out(p, "TestSkip", "--- SKIP: TestSkip (0.00s)\n"),
		{"skip", p, "TestSkip", ""},
		out(p, "", "ok  \texample.invalid/m/a\t0.01s\n"),
		{"pass", p, "", ""},
	}
	log, summary := runGoTestReport(t, stream(t, events, nil), "", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	wants(t, "log", log, "go-test-report: the stream could not be read; all of it as go test -v writes it:\n"+
		"--- SKIP: TestSkip (0.00s)\nok  \texample.invalid/m/a\t0.01s\n")
	if want := "### Skipped tests, go test: not read\n"; !strings.HasPrefix(summary, want) {
		t.Errorf("summary:\n%s\nwant it to open with %q", summary, want)
	}
	wants(t, "summary", summary, "the summary names no test's time")
	lacks(t, "summary", summary, "### Slowest")
}

// TestGoTestReportTimes: the summary names the run's 20 slowest
// top-level tests, a failed one among them, and its 10 slowest packages,
// with how many top-level tests ended in each; a subtest and a skipped
// test are no row of their own, and a package whose result came from go
// test's cache is left out of both, though it would head each, and
// counted.
func TestGoTestReportTimes(t *testing.T) {
	const m = "example.invalid/m/"
	var events []any
	for i := 1; i <= 10; i++ {
		p := fmt.Sprintf("%sp%02d", m, i)
		events = append(events, ended("pass", p, "TestQuick", 0.01), ended("pass", p, "", float64(i)))
	}
	for i := 1; i <= 22; i++ {
		events = append(events, ended("pass", m+"slow", fmt.Sprintf("Test%02d", i), float64(i)+0.5))
	}
	events = append(events,
		ended("pass", m+"slow", "Test22/sub", 22.4),
		ended("fail", m+"slow", "TestFailed", 30),
		ended("skip", m+"slow", "TestSkipped", 40),
		out(m+"slow", "", "FAIL\t"+m+"slow\t300.12s\n"),
		ended("fail", m+"slow", "", 300.123),
		ended("pass", m+"cached", "TestCached", 99),
		out(m+"cached", "", "ok  \t"+m+"cached\t(cached)\n"),
		ended("pass", m+"cached", "", 500),
	)
	_, summary := runGoTestReport(t, stream(t, events, nil), "")
	var want strings.Builder
	want.WriteString("### Skipped tests, go test: 1\n\n" +
		"| Package | Test | Why |\n|---------|------|-----|\n" +
		"| " + m + "slow | TestSkipped |  |\n\n" +
		"### Slowest tests, go test\n\n" +
		"1 package(s) came from go test's cache and are left out: their times are an earlier run's.\n\n" +
		"| Package | Test | Seconds |\n|---------|------|--------:|\n" +
		"| " + m + "slow | TestFailed | 30.00 |\n")
	for i := 22; i >= 4; i-- {
		fmt.Fprintf(&want, "| %sslow | Test%02d | %d.50 |\n", m, i, i)
	}
	want.WriteString("\n### Slowest packages, go test\n\n" +
		"| Package | Tests | Seconds |\n|---------|------:|--------:|\n" +
		"| " + m + "slow | 24 | 300.12 |\n")
	for i := 10; i >= 2; i-- {
		fmt.Fprintf(&want, "| %sp%02d | 1 | %d.00 |\n", m, i, i)
	}
	want.WriteString("\n")
	if summary != want.String() {
		t.Errorf("summary:\n%s\nwant:\n%s", summary, want.String())
	}

	// The times' reading failing alone: the skips are named, the times
	// not read. A jq that fails on the times' program stands in for it.
	real, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("no jq")
	}
	bin := t.TempDir()
	failing := "#!/usr/bin/env bash\n" +
		"case \" $* \" in *'packages: {}, cached: {}'*) exit 5 ;; esac\n" +
		"exec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "jq"), []byte(failing), 0o755); err != nil {
		t.Fatal(err)
	}
	_, summary = runGoTestReport(t, stream(t, events, nil), "", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	wants(t, "summary", summary, "### Skipped tests, go test: 1\n", "### Slowest tests, go test: not read\n")
	lacks(t, "summary", summary, "| Seconds |")
}

// TestGoTestReportTimesOfManyTests: among 3000 tests of one package, in
// no order of their times, the 20 slowest are named, slowest first.
func TestGoTestReportTimesOfManyTests(t *testing.T) {
	const p, n = "example.invalid/m/many", 3000
	type timed struct {
		name  string
		cents int
	}
	var events []timedEvent
	var all []timed
	for i := range n {
		// 7919 is prime to n, so every hundredth of 0 to 29.99 s comes once.
		c := timed{fmt.Sprintf("Test%04d", i), i * 7919 % n}
		all = append(all, c)
		events = append(events, ended("pass", p, c.name, float64(c.cents)/100))
	}
	events = append(events, ended("pass", p, "", 4500.5))
	_, summary := runGoTestReport(t, stream(t, events, nil), "")
	slices.SortFunc(all, func(a, b timed) int { return b.cents - a.cents })
	var want strings.Builder
	want.WriteString("### Skipped tests, go test: 0\n\n" +
		"### Slowest tests, go test\n\n" +
		"| Package | Test | Seconds |\n|---------|------|--------:|\n")
	for _, c := range all[:20] {
		fmt.Fprintf(&want, "| %s | %s | %d.%02d |\n", p, c.name, c.cents/100, c.cents%100)
	}
	want.WriteString("\n### Slowest packages, go test\n\n" +
		"| Package | Tests | Seconds |\n|---------|------:|--------:|\n" +
		"| " + p + " | 3000 | 4500.50 |\n\n")
	if summary != want.String() {
		t.Errorf("summary:\n%s\nwant:\n%s", summary, want.String())
	}
}
