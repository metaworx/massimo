// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package packaging

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The macOS and Windows jobs no runner took (AP TestSpeed, D16-A, D15-B):
// ci.yml's verdict and Tested record on a push to main and a tag, Retry's
// re-runs (packaging/ci-retry.sh) and the nightly build's What to ship
// (packaging/nightly-pick.sh).

// stepScript returns the run: script of the step of ci.yml named, its
// indentation taken off, as GitHub runs it.
func stepScript(t *testing.T, workflow, step string) string {
	t.Helper()
	lines := strings.Split(readWorkflow(t, workflow), "\n")
	at := slices.Index(lines, "      - name: "+step)
	if at < 0 {
		t.Fatalf("%s has no step %q", workflow, step)
	}
	for i := at + 1; i < len(lines) && strings.HasPrefix(lines[i], "        "); i++ {
		if lines[i] != "        run: |" {
			continue
		}
		var script strings.Builder
		for _, line := range lines[i+1:] {
			if line != "" && !strings.HasPrefix(line, "          ") {
				break
			}
			script.WriteString(strings.TrimPrefix(line, "          ") + "\n")
		}
		return script.String()
	}
	t.Fatalf("%s's step %q has no run: | script", workflow, step)
	return ""
}

// ciVerdict runs ci.yml's verdict step on the needs and the report's
// output unrun given, as the step's env gives them, and reports whether
// it passed.
func ciVerdict(t *testing.T, script, needs, unrun string) bool {
	t.Helper()
	bash := scriptShell(t)
	cmd := exec.Command(bash, "-e", "-c", script)
	cmd.Env = append(os.Environ(), "NEEDS="+needs, "UNRUN="+unrun, "RESULTS=")
	out, err := cmd.CombinedOutput()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		t.Fatalf("the verdict: %v\n%s", err, out)
	}
	return err == nil
}

// TestCIVerdict: All tests passed's verdict lets a caller through that
// the report named unrun, abandoned, cancelled or failed among the needs,
// on a pull request and a push to main (D5-A, D16-A); never one the
// report did not name, as on a tag, where it gives no unrun; and never a
// job that failed.
func TestCIVerdict(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	if !strings.Contains(ci, "UNRUN: ${{ (github.event_name == 'pull_request' || github.ref == 'refs/heads/main') && steps.report.outputs.unrun || '' }}\n") {
		t.Error("ci.yml's verdict does not take unrun from the report on a pull request and a push to main alone")
	}
	script := stepScript(t, "ci.yml", "Every test job passed or was skipped")
	for _, tc := range []struct {
		name, needs, unrun string
		pass               bool
	}{
		{"all passed", passedNeeds, "", true},
		{"abandoned, unrun", platformsNeeds("abandoned"), "platforms", true},
		{"cancelled, unrun", platformsNeeds("cancelled"), "platforms", true},
		{"failed caller, unrun", platformsNeeds("failure"), "platforms", true},
		{"abandoned on a tag", platformsNeeds("abandoned"), "", false},
		{"cancelled, no report", platformsNeeds("cancelled"), "", false},
		{"another job failed", `{"test":{"result":"failure"},"platforms":{"result":"abandoned"}}`, "platforms", false},
		{"another job cancelled", `{"test":{"result":"cancelled"},"platforms":{"result":"success"}}`, "platforms", false},
		{"unrun names another job", `{"test":{"result":"failure"},"platforms":{"result":"success"}}`, "platforms test2", false},
	} {
		if got := ciVerdict(t, script, tc.needs, tc.unrun); got != tc.pass {
			t.Errorf("%s: the verdict passed = %v, want %v", tc.name, got, tc.pass)
		}
	}
}

// runTestedRecord runs packaging/tested-record.sh on the needs given and
// returns the description it printed and its stderr.
func runTestedRecord(t *testing.T, needs string) (desc, stderr string) {
	t.Helper()
	bash := scriptShell(t)
	cmd := exec.Command(bash, "tested-record.sh")
	cmd.Env = append(os.Environ(), "NEEDS="+needs, "FP=fp1:"+strings.Repeat("0", 64))
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("tested-record.sh: %v\n%s", err, errb.String())
	}
	return strings.TrimSuffix(string(out), "\n"), errb.String()
}

// recordNeeds is a push's needs with What to test's answers and
// Platforms' result and outputs as given; the UI tests, LAN, Fuzz and
// the Android app passed.
func recordNeeds(changes map[string]string, platforms string, outputs map[string]string) string {
	answers := map[string]string{"ui": "true", "lan": "true", "app": "true", "macos": "true", "windows": "true",
		"ui_done": "false", "lan_done": "false", "app_done": "false", "fuzz_done": "false", "macos_done": "false", "windows_done": "false"}
	for k, v := range changes {
		answers[k] = v
	}
	needs := map[string]any{
		"changes":     map[string]any{"result": "success", "outputs": answers},
		"screenshots": map[string]any{"result": "success"},
		"lan":         map[string]any{"result": "success"},
		"fuzz":        map[string]any{"result": "success"},
		"android":     map[string]any{"result": "success"},
		"platforms":   map[string]any{"result": platforms, "outputs": outputs},
	}
	b, err := json.Marshal(needs)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestTestedRecord: the record says mac=false for a macOS job no runner
// took on a push to main, which passed (D16-A); true for one that passed
// here or before, unneeded where What to test answered false, and the
// rest as before.
func TestTestedRecord(t *testing.T) {
	fp := "fp1:" + strings.Repeat("0", 64)
	for _, tc := range []struct {
		name      string
		changes   map[string]string
		platforms string
		outputs   map[string]string
		want      string
	}{
		{"macOS not taken", nil, "abandoned", map[string]string{"macos": "", "windows": "true"},
			"ui=true lan=true fuzz=true app=true mac=false win=true"},
		{"caller failed", nil, "failure", map[string]string{"macos": "", "windows": ""},
			"ui=true lan=true fuzz=true app=true mac=false win=false"},
		{"both passed", nil, "success", map[string]string{"macos": "true", "windows": "true"},
			"ui=true lan=true fuzz=true app=true mac=true win=true"},
		{"passed before, not needed", map[string]string{"macos_done": "true", "windows": "false"}, "skipped", nil,
			"ui=true lan=true fuzz=true app=true mac=true win=unneeded"},
		{"Windows not needed, macOS failed", map[string]string{"windows": "false"}, "failure", map[string]string{"macos": ""},
			"ui=true lan=true fuzz=true app=true mac=false win=unneeded"},
	} {
		desc, stderr := runTestedRecord(t, recordNeeds(tc.changes, tc.platforms, tc.outputs))
		if want := fp + " " + tc.want; desc != want {
			t.Errorf("%s: %q, want %q", tc.name, desc, want)
		}
		if stderr != "" || len(desc) > 140 {
			t.Errorf("%s: %d characters, stderr %q", tc.name, len(desc), stderr)
		}
	}
	// Failed and not needed: LAN and the Android app; the UI tests and Fuzz
	// know no unneeded.
	needs := `{"changes":{"outputs":{"lan":"false","app":"false","ui":"false","fuzz_done":"false","macos":"true","windows":"true"}},` +
		`"screenshots":{"result":"skipped"},"lan":{"result":"skipped"},"fuzz":{"result":"failure"},"android":{"result":"skipped"},` +
		`"platforms":{"result":"success","outputs":{"macos":"true","windows":"true"}}}`
	if desc, _ := runTestedRecord(t, needs); desc != fp+" ui=false lan=unneeded fuzz=false app=unneeded mac=true win=true" {
		t.Errorf("not needed: %q", desc)
	}
}

// fakeRetryGH is gh as ci-retry.sh calls it, a run in stages: $FAKE_GH/stage
// names the stage, whose run.<stage>.json and jobs.<stage>.json answer
// through --jq, the run failing where its file is missing; a look at the
// jobs moves on to the stage next.<stage> names, where there is one, and
// the re-run (POST) to the one rerun.<stage> names, or fails where there
// is none. Any other GET answers from $FAKE_GH/api/<path>.json, or 404.
// Every call is logged to $FAKE_GH/calls with the stage it met.
const fakeRetryGH = `#!/usr/bin/env bash
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
stage="$(cat "$FAKE_GH/stage")"
printf '%s %s @%s\n' "$method" "$path" "$stage" >>"$FAKE_GH/calls"
if [ "$method" != GET ]; then
	[ -e "$FAKE_GH/rerun.$stage" ] || { echo "HTTP 403: no re-run here" >&2; exit 1; }
	cp "$FAKE_GH/rerun.$stage" "$FAKE_GH/stage"
	exit 0
fi
case "$path" in
*/actions/runs/100/jobs\?*) f="$FAKE_GH/jobs.$stage.json" ;;
*/actions/runs/100) f="$FAKE_GH/run.$stage.json" ;;
*) f="$FAKE_GH/api/${path#repos/owner/repo/}.json" ;;
esac
[ -e "$f" ] || { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
out="$(jq -rc "$jqx" "$f")"
if [ "$f" = "$FAKE_GH/jobs.$stage.json" ] && [ -e "$FAKE_GH/next.$stage" ]; then
	cp "$FAKE_GH/next.$stage" "$FAKE_GH/stage"
fi
printf '%s\n' "$out"
`

// retryStage is a stage of a run as fakeRetryGH gives it.
type retryStage struct {
	attempt int
	status  string
	jobs    []ciJob
	next    string // the stage after a look at the jobs
	rerun   string // the stage a re-run starts
	gone    bool   // the run cannot be read
}

// retryRun is what a run is, as the API and the event give it: its
// workflow's path, its event, its repositories, its branch or tag and its
// commit, and the event's HEAD_BRANCH and HEAD_SHA where they differ.
type retryRun struct {
	path, event, repo, headRepo, branch, sha string
	eventBranch, eventSHA                    string
}

// tagRun is a version tag's run of ci.yml.
var tagRun = retryRun{path: ".github/workflows/ci.yml", event: "push", repo: "owner/repo", headRepo: "owner/repo",
	branch: "v1.2.3", sha: "c0ffee"}

// nightlyRun is a scheduled nightly run.
var nightlyRun = retryRun{path: ".github/workflows/nightly.yml", event: "schedule", repo: "owner/repo", headRepo: "owner/repo",
	branch: "main", sha: "c0ffee"}

// tagFiles are the API's answers for tagRun's tag, annotated, at its
// commit, and no branch of its name.
var tagFiles = map[string]string{
	"git/ref/tags/v1.2.3": `{"object":{"type":"tag","sha":"7a9"}}`,
	"git/tags/7a9":        `{"object":{"type":"commit","sha":"c0ffee"}}`,
}

// retrySetup is one run of ci-retry.sh: the run, its stages from "1", and
// the API's other answers.
type retrySetup struct {
	run    retryRun
	stages map[string]retryStage
	files  map[string]string // nil: tagFiles
}

// retryResult is what a run of ci-retry.sh left: its output, its exit
// code, the summary and the calls of gh.
type retryResult struct {
	out, summary, calls string
	code                int
}

// retryJobsJSON is retry.yml's RETRY_JOBS.
const retryJobsJSON = `["Platforms / macOS LaunchDaemon", "Platforms / Windows service"]`

// runRetry runs packaging/ci-retry.sh from stage 1 of the stages given,
// with no waits.
func runRetry(t *testing.T, s retrySetup) retryResult {
	t.Helper()
	bash := scriptShell(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, b []byte) {
		t.Helper()
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeRetryGH), 0o755); err != nil {
		t.Fatal(err)
	}
	write("stage", []byte("1\n"))
	files := s.files
	if files == nil {
		files = tagFiles
	}
	for path, body := range files {
		write("api/"+path+".json", []byte(body))
	}
	r := s.run
	for name, st := range s.stages {
		if !st.gone {
			run, err := json.Marshal(map[string]any{"id": 100, "run_attempt": st.attempt, "status": st.status,
				"event": r.event, "path": r.path, "head_branch": r.branch, "head_sha": r.sha,
				"repository": map[string]string{"full_name": r.repo}, "head_repository": map[string]string{"full_name": r.headRepo}})
			if err != nil {
				t.Fatal(err)
			}
			write("run."+name+".json", run)
		}
		jobs, err := json.Marshal(map[string]any{"jobs": st.jobs})
		if err != nil {
			t.Fatal(err)
		}
		write("jobs."+name+".json", jobs)
		if st.next != "" {
			write("next."+name, []byte(st.next+"\n"))
		}
		if st.rerun != "" {
			write("rerun."+name, []byte(st.rerun+"\n"))
		}
	}
	eventBranch, eventSHA := r.branch, r.sha
	if r.eventBranch != "" {
		eventBranch = r.eventBranch
	}
	if r.eventSHA != "" {
		eventSHA = r.eventSHA
	}
	summary := filepath.Join(dir, "summary.md")
	cmd := exec.Command(bash, "ci-retry.sh")
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH="+dir,
		"GITHUB_REPOSITORY=owner/repo",
		"GITHUB_STEP_SUMMARY="+summary,
		"RUN_ID=100",
		"HEAD_BRANCH="+eventBranch,
		"HEAD_SHA="+eventSHA,
		"RETRY_JOBS="+retryJobsJSON,
		"RETRY_WAIT=0", "RETRY_POLL=0", "RETRY_DEADLINE=20",
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("ci-retry.sh: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	return retryResult{out: string(out), summary: read("summary.md"), calls: read("calls"), code: code}
}

// pending is a job that has not ended: queued for a runner.
func pending(id int64, attempt int, name string) ciJob {
	return ciJob{ID: id, Name: name, RunAttempt: attempt, Status: "queued", StartedAt: "2026-11-18T11:00:00Z"}
}

// tagAttempt is a tag's attempt as Retry sees it once it has ended: the
// jobs carried from attempt 1, macOS LaunchDaemon not taken or as given,
// and All tests passed with the verdict given.
func tagAttempt(attempt int, macos ciJob, verdict string) []ciJob {
	return []ciJob{
		ran(1, 1, "What to test", "success", 0),
		ran(2, 1, "Vet and test", "success", 1),
		ran(3, 1, "Platforms / Windows service", "success", 1),
		macos,
		ran(5, 1, "Build / Binaries and packages", "success", 1),
		ran(int64(6+10*attempt), attempt, "All tests passed", verdict, 30),
		notRun(int64(7+10*attempt), attempt, "Release", "skipped"),
	}
}

// rerunCalls counts the re-runs ci-retry.sh asked for, each of run 100's
// failed jobs.
func rerunCalls(t *testing.T, calls string) int {
	t.Helper()
	n := strings.Count(calls, "POST repos/owner/repo/actions/runs/100/rerun-failed-jobs ")
	if all := strings.Count(calls, "POST "); all != n {
		t.Errorf("%d writes, %d of them re-runs of run 100's failed jobs:\n%s", all, n, calls)
	}
	return n
}

// TestRetryRerunPasses: a tag's run whose macOS job no runner took is
// re-run once, and a runner takes it while the run goes on to its
// release: Retry is done, the run's verdict decides.
func TestRetryRerunPasses(t *testing.T) {
	res := runRetry(t, retrySetup{run: tagRun, stages: map[string]retryStage{
		"1":  {attempt: 1, status: "completed", jobs: tagAttempt(1, notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled"), "failure"), rerun: "2"},
		"2":  {attempt: 2, status: "in_progress", jobs: tagAttempt(2, pending(14, 2, "Platforms / macOS LaunchDaemon"), "skipped"), next: "2b"},
		"2b": {attempt: 2, status: "waiting", jobs: tagAttempt(2, ran(14, 2, "Platforms / macOS LaunchDaemon", "success", 40), "success")},
	}})
	if res.code != 0 || rerunCalls(t, res.calls) != 1 {
		t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
	}
	wants(t, "summary", res.summary,
		"- No runner took Platforms / macOS LaunchDaemon in attempt 1: re-run 1 of 2 in 0 s.\n",
		"- A runner took them in attempt 2: Platforms / macOS LaunchDaemon passed. The run's own verdict decides.\n")
	wants(t, "calls", res.calls, "GET repos/owner/repo/git/ref/tags/v1.2.3 @1\n", "GET repos/owner/repo/git/ref/heads/v1.2.3 @1\n")
	lacks(t, "output", res.out, "::error::")
}

// TestRetryTwice: no runner in the first re-run either: a second one, in
// which a runner takes both jobs, for the nightly run's verdict job.
func TestRetryTwice(t *testing.T) {
	night := func(attempt int, macos, windows ciJob, verdict string) []ciJob {
		return []ciJob{
			ran(1, 1, "What to ship", "success", 0),
			macos, windows,
			ran(int64(6+10*attempt), attempt, "Platforms passed", verdict, 30),
			notRun(int64(7+10*attempt), attempt, "Build", "skipped"),
			notRun(int64(8+10*attempt), attempt, "Release", "skipped"),
		}
	}
	mac, win := "Platforms / macOS LaunchDaemon", "Platforms / Windows service"
	res := runRetry(t, retrySetup{run: nightlyRun, files: map[string]string{}, stages: map[string]retryStage{
		"1": {attempt: 1, status: "completed", jobs: night(1, notRun(4, 1, mac, "cancelled"), notRun(5, 1, win, "cancelled"), "failure"), rerun: "2"},
		"2": {attempt: 2, status: "completed", jobs: night(2, notRun(14, 2, mac, "cancelled"), ran(15, 2, win, "success", 20), "failure"), rerun: "3"},
		"3": {attempt: 3, status: "in_progress", jobs: night(3, ran(24, 3, mac, "success", 50), ran(15, 2, win, "success", 20), "success")},
	}})
	if res.code != 0 || rerunCalls(t, res.calls) != 2 {
		t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
	}
	wants(t, "summary", res.summary,
		"- No runner took Platforms / macOS LaunchDaemon, Platforms / Windows service in attempt 1: re-run 1 of 2 in 0 s.\n",
		"- No runner took Platforms / macOS LaunchDaemon in attempt 2: re-run 2 of 2 in 0 s.\n",
		"- A runner took them in attempt 3: Platforms / macOS LaunchDaemon passed.")
}

// TestRetryNoRunner: no runner in three attempts: two re-runs and an
// error, the run failed for want of a runner.
func TestRetryNoRunner(t *testing.T) {
	stages := map[string]retryStage{}
	for a := 1; a <= 3; a++ {
		s := retryStage{attempt: a, status: "completed",
			jobs: tagAttempt(a, notRun(int64(4+10*a), a, "Platforms / macOS LaunchDaemon", "cancelled"), "failure")}
		if a < 3 {
			s.rerun = strconv.Itoa(a + 1)
		}
		stages[strconv.Itoa(a)] = s
	}
	res := runRetry(t, retrySetup{run: tagRun, stages: stages})
	if res.code != 1 || rerunCalls(t, res.calls) != 2 {
		t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
	}
	wants(t, "output", res.out, "::error::No runner took Platforms / macOS LaunchDaemon in 3 attempts of run 100: "+
		"the run fails for want of a runner, a tag's release with it, or the night is skipped.\n")
}

// TestRetryNothingToRun: no re-run where another job failed, where a
// runner took the job before it was cancelled or it failed, where the
// verdict did not fail (a run someone cancelled), or where the run is at
// another attempt already; a failed job is never run again.
func TestRetryNothingToRun(t *testing.T) {
	notTaken := notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled")
	otherFailed := tagAttempt(1, notTaken, "failure")
	otherFailed[1] = ran(2, 1, "Vet and test", "failure", 1)
	for name, tc := range map[string]struct {
		stage retryStage
		says  string
	}{
		"another job failed": {retryStage{attempt: 1, status: "completed", jobs: otherFailed},
			"but Vet and test (failure) did not pass either: nothing is run again."},
		"cancelled once taken": {retryStage{attempt: 1, status: "completed",
			jobs: tagAttempt(1, ran(4, 1, "Platforms / macOS LaunchDaemon", "cancelled", 2), "failure")},
			"A runner took every job of"},
		"failed": {retryStage{attempt: 1, status: "completed",
			jobs: tagAttempt(1, ran(4, 1, "Platforms / macOS LaunchDaemon", "failure", 2), "failure")},
			"A runner took every job of"},
		"verdict cancelled": {retryStage{attempt: 1, status: "completed", jobs: tagAttempt(1, notTaken, "cancelled")},
			"and All tests passed is cancelled, not failed (a run someone cancelled?): nothing is run again."},
		"run again by someone": {retryStage{attempt: 2, status: "in_progress", jobs: tagAttempt(2, notTaken, "skipped")},
			"Run 100 is at attempt 2, after 1: someone ran it again, and Retry leaves it."},
	} {
		t.Run(name, func(t *testing.T) {
			tc.stage.rerun = "2"
			res := runRetry(t, retrySetup{run: tagRun, stages: map[string]retryStage{"1": tc.stage}})
			if res.code != 0 || rerunCalls(t, res.calls) != 0 {
				t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
			}
			wants(t, "summary", res.summary, tc.says)
		})
	}
}

// TestRetryRefusesARun: Retry re-runs nothing of a run that is not a
// version tag's run of ci.yml or main's nightly run from the schedule or
// by hand, of this repository: a pull request's run of a workflow named
// Nightly, a fork's, another workflow, a branch named v1, a tag GitHub
// does not have, one at another commit or with a branch of its name, or a
// run that is not the event's.
func TestRetryRefusesARun(t *testing.T) {
	failed := map[string]retryStage{"1": {attempt: 1, status: "completed", rerun: "2",
		jobs: tagAttempt(1, notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled"), "failure")}}
	with := func(r retryRun, change func(*retryRun)) retryRun { change(&r); return r }
	for name, tc := range map[string]struct {
		run   retryRun
		files map[string]string
		says  string
	}{
		"a pull request's Nightly": {with(nightlyRun, func(r *retryRun) { r.event, r.branch = "pull_request", "evil" }), nil,
			"Run 100 of nightly.yml is a pull_request, not the schedule's or one by hand"},
		"a nightly run off main": {with(nightlyRun, func(r *retryRun) { r.event, r.branch = "workflow_dispatch", "evil" }), nil,
			"Run 100 of nightly.yml is on evil, not main"},
		"a fork's head": {with(nightlyRun, func(r *retryRun) { r.headRepo = "fork/repo" }), nil,
			"Run 100 is of owner/repo, its head of fork/repo, not owner/repo"},
		"another workflow": {with(nightlyRun, func(r *retryRun) { r.path = ".github/workflows/evil.yml" }), nil,
			"Run 100 is of .github/workflows/evil.yml, neither ci.yml nor nightly.yml"},
		"a pull request's CI": {with(tagRun, func(r *retryRun) { r.event = "pull_request" }), nil,
			"Run 100 of ci.yml is a pull_request, not a tag's push"},
		"a branch named v1": {with(tagRun, func(r *retryRun) { r.branch = "v1" }), nil,
			"Run 100 of ci.yml is of v1, which is no version tag"},
		"a branch named V1.2.3": {with(tagRun, func(r *retryRun) { r.branch = "V1.2.3" }), nil,
			"Run 100 of ci.yml is of V1.2.3, which is no version tag"},
		"no such tag": {with(tagRun, func(r *retryRun) { r.branch = "v1.2.4" }), nil,
			"The tag v1.2.4 could not be read"},
		"a tag at another commit": {tagRun, map[string]string{
			"git/ref/tags/v1.2.3": `{"object":{"type":"commit","sha":"bad"}}`},
			"The tag v1.2.3 names bad, not run 100's c0ffee"},
		"a branch of the tag's name": {tagRun, map[string]string{
			"git/ref/tags/v1.2.3": `{"object":{"type":"commit","sha":"c0ffee"}}`, "git/ref/heads/v1.2.3": `{"object":{}}`},
			"A branch is named v1.2.3 too"},
		"not the event's run": {with(tagRun, func(r *retryRun) { r.eventSHA = "f00d" }), nil,
			"Run 100 is of v1.2.3 at c0ffee, not the event's v1.2.3 at f00d"},
	} {
		t.Run(name, func(t *testing.T) {
			res := runRetry(t, retrySetup{run: tc.run, files: tc.files, stages: failed})
			if res.code != 1 || rerunCalls(t, res.calls) != 0 {
				t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
			}
			wants(t, "output", res.out, "::error::"+tc.says)
		})
	}
	// The workflow's own condition says the same before a runner starts.
	retry := readWorkflow(t, "retry.yml")
	for _, cond := range []string{
		"github.event.workflow_run.run_attempt == 1",
		"github.event.workflow_run.repository.full_name == github.repository",
		"github.event.workflow_run.head_repository.full_name == github.repository",
		"github.event.workflow_run.path == '.github/workflows/nightly.yml' && github.event.workflow_run.head_branch == 'main'",
		"(github.event.workflow_run.event == 'schedule' || github.event.workflow_run.event == 'workflow_dispatch')",
		"github.event.workflow_run.path == '.github/workflows/ci.yml' && github.event.workflow_run.event == 'push'",
	} {
		if !strings.Contains(retry, cond) {
			t.Errorf("retry.yml's condition lacks %s", cond)
		}
	}
	lacks(t, "retry.yml", retry, "workflow_run.name ==")
}

// TestRetryLeftOut: a re-run that does not take the job no runner took
// fails with an error naming it; a re-run GitHub refuses fails too; and a
// run that cannot be read after the wait fails, as it is not run again
// without being read.
func TestRetryLeftOut(t *testing.T) {
	notTaken := notRun(4, 1, "Platforms / macOS LaunchDaemon", "cancelled")
	res := runRetry(t, retrySetup{run: tagRun, stages: map[string]retryStage{
		"1": {attempt: 1, status: "completed", jobs: tagAttempt(1, notTaken, "failure"), rerun: "2"},
		"2": {attempt: 2, status: "completed", jobs: tagAttempt(2, notTaken, "failure"), rerun: "3"},
	}})
	if res.code != 1 || rerunCalls(t, res.calls) != 1 {
		t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
	}
	wants(t, "output", res.out, "::error::The re-run of attempt 2 left out Platforms / macOS LaunchDaemon, which no runner took: run it again by hand.\n")

	res = runRetry(t, retrySetup{run: tagRun, stages: map[string]retryStage{
		"1": {attempt: 1, status: "completed", jobs: tagAttempt(1, notTaken, "failure")},
	}})
	if res.code != 1 {
		t.Fatalf("exit %d:\n%s", res.code, res.out)
	}
	wants(t, "output", res.out, "::error::GitHub refused the re-run of run 100's failed jobs.\n")

	res = runRetry(t, retrySetup{run: tagRun, stages: map[string]retryStage{
		"1":    {attempt: 1, status: "completed", jobs: tagAttempt(1, notTaken, "failure"), next: "gone", rerun: "2"},
		"gone": {gone: true, rerun: "2"},
	}})
	if res.code != 1 || rerunCalls(t, res.calls) != 0 {
		t.Fatalf("exit %d, calls:\n%s\n%s", res.code, res.calls, res.out)
	}
	wants(t, "output", res.out, "::error::Run 100 could not be read: nothing is run again.\n")
	if n := strings.Count(res.calls, "GET repos/owner/repo/actions/runs/100 @gone\n"); n != 3 {
		t.Errorf("%d reads of the run after the wait, want 3:\n%s", n, res.calls)
	}
}

// fakeCodeResultGH is gh as code-result.sh calls it for a commit
// $FAKE_C1 with one run of ci.yml, 500, completed, whose jobs of every
// attempt are $FAKE_GH/jobs.json; no other commit has runs or statuses.
const fakeCodeResultGH = `#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || exit 2
shift
jqx=. path=""
while [ $# -gt 0 ]; do
	case "$1" in
	--jq) jqx="$2"; shift 2 ;;
	--paginate) shift ;;
	-X) exit 3 ;;
	*) path="$1"; shift ;;
	esac
done
printf 'GET %s\n' "$path" >>"$FAKE_GH/calls"
case "$path" in
*"/actions/workflows/ci.yml/runs?head_sha=${FAKE_C1}&"*) echo '{"workflow_runs":[{"id":500,"html_url":"https://example.invalid/runs/500"}]}' ;;
*"/actions/workflows/ci.yml/runs?"*) echo '{"workflow_runs":[]}' ;;
*/actions/runs/500) echo '{"status":"completed","conclusion":"success"}' ;;
*"/actions/runs/500/jobs?filter=all&"*) cat "$FAKE_GH/jobs.json" ;;
*/statuses*) echo '[]' ;;
*) exit 1 ;;
esac | jq -rc "$jqx"
`

// runCodeResult runs packaging/code-result.sh for a documents' commit on
// one whose run 500 left the jobs given, every attempt's, and returns its
// exit code and output.
func runCodeResult(t *testing.T, jobs []ciJob) (int, string) {
	t.Helper()
	r := newNightlyRepo(t) // a history; its first two commits stand in
	script, err := filepath.Abs("code-result.sh")
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(r.bin, "gh"), []byte(fakeCodeResultGH), 0o755); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(r.dir, "GUIDELINES", "shared", "tools")
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "fingerprint.sh"), []byte("#!/usr/bin/env bash\necho fp1:test\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"jobs": jobs})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "jobs.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(scriptShell(t), script)
	cmd.Dir = r.dir
	cmd.Env = append(slices.Clone(r.env),
		"PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH="+fake,
		"FAKE_C1="+r.commits["c1"],
		"CODE_HEAD="+r.commits["c2"],
		"CODE_DEADLINE=5", "CODE_WALK=2",
		"GITHUB_REPOSITORY=owner/repo",
		"GITHUB_EVENT_NAME=push",
		"GITHUB_RUN_ID=999",
		"GITHUB_STEP_SUMMARY="+filepath.Join(fake, "summary.md"),
		"UNRUN_JOBS="+ciUnrunJobs,
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("code-result.sh: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	return code, string(out)
}

// TestCodeResultUnrun: Code result takes a macOS job no runner took on a
// push to main for no verdict on the code (D16-A), but not where its last
// run that a runner took, in an earlier attempt, failed: a re-run no
// runner took does not hide a failure; one that passed later does.
func TestCodeResultUnrun(t *testing.T) {
	const mac = "Platforms / macOS LaunchDaemon"
	vet := func(attempt int) ciJob { return ran(int64(2+10*attempt), attempt, "Vet and test", "success", 1) }
	for _, tc := range []struct {
		name string
		jobs []ciJob
		code int
		says string
	}{
		{"no runner", []ciJob{vet(1), notRun(4, 1, mac, "cancelled")}, 0,
			"The code passed in run https://example.invalid/runs/500"},
		{"failed, then no runner", []ciJob{vet(1), ran(4, 1, mac, "failure", 2), vet(2), notRun(14, 2, mac, "cancelled")}, 1,
			"::error::The code failed in run https://example.invalid/runs/500 on "},
		{"failed, then passed", []ciJob{vet(1), ran(4, 1, mac, "failure", 2), vet(2), ran(14, 2, mac, "success", 20)}, 0,
			"The code passed in run"},
		{"another job no runner took", []ciJob{vet(1), notRun(4, 1, "Race", "cancelled")}, 1,
			"Race (cancelled before a runner took it)"},
	} {
		code, out := runCodeResult(t, tc.jobs)
		if code != tc.code {
			t.Errorf("%s: exit %d, want %d\n%s", tc.name, code, tc.code, out)
		}
		wants(t, tc.name, out, tc.says)
		if tc.name == "failed, then no runner" {
			wants(t, tc.name, out, mac+" (failure in an earlier attempt, no runner since)")
		}
	}
}

// fakeNightlyGH is gh as nightly-pick.sh calls it: $FAKE_GH/runs.json maps
// a commit to its runs of ci.yml, [{id, conclusion}], the conclusion
// being All tests passed's; records.json maps a commit to its Tested
// record's description. Every call is logged to $FAKE_GH/calls.
const fakeNightlyGH = `#!/usr/bin/env bash
set -euo pipefail
[ "$1" = api ] || exit 2
shift
jqx=. path=""
while [ $# -gt 0 ]; do
	case "$1" in
	--jq) jqx="$2"; shift 2 ;;
	--paginate) shift ;;
	-X) exit 3 ;;
	*) path="$1"; shift ;;
	esac
done
printf 'GET %s\n' "$path" >>"$FAKE_GH/calls"
case "$path" in
*"/actions/workflows/ci.yml/runs?head_sha="*)
	sha="${path#*head_sha=}" sha="${sha%%&*}"
	jq -c --arg s "$sha" '{workflow_runs: [(.[$s] // [])[] | {id}]}' "$FAKE_GH/runs.json" ;;
*"/actions/runs/"*"/jobs?"*)
	id="${path#*/actions/runs/}" id="${id%%/*}"
	jq -c --argjson id "$id" '{jobs: [.[][] | select(.id == $id) | {id: 1, name: "All tests passed", conclusion}]}' "$FAKE_GH/runs.json" ;;
*"/commits/"*"/statuses"*)
	sha="${path#*/commits/}" sha="${sha%%/*}"
	jq -c --arg s "$sha" '[.[$s] // empty | {context: "Tested", state: "success", creator: {login: "github-actions[bot]"}, description: .}]' "$FAKE_GH/records.json" ;;
*) exit 1 ;;
esac | jq -rc "$jqx"
`

// fakeFingerprint is fingerprint.sh's same as nightly-pick.sh asks it: the
// same code where the later commit's summary opens with "docs".
const fakeFingerprint = `#!/usr/bin/env bash
[ "$1" = -q ] && shift
[ "$1" = same ] || exit 2
case "$(git log -1 --format=%s "$3")" in docs*) echo true ;; *) echo false ;; esac
`

// nightlyRepo is a history of main for nightly-pick.sh: its commits by
// name, the git it runs in, and the fakes.
type nightlyRepo struct {
	dir, bin string
	env      []string
	commits  map[string]string
}

// newNightlyRepo makes main's history, first parents only, each commit
// named by its summary's first word: c1 (release/nightly), c2, c3,
// docs4, c5, c6, c7 (v1.2.3, a release commit), docs8.
func newNightlyRepo(t *testing.T) *nightlyRepo {
	t.Helper()
	scriptShell(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	r := &nightlyRepo{dir: filepath.Join(home, "repo"), bin: filepath.Join(home, "bin"), commits: map[string]string{}}
	// No configuration of the machine's: its commits and tags may be
	// signed with a key the test cannot reach.
	r.env = []string{"HOME=" + home, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") && !strings.HasPrefix(kv, "HOME=") {
			r.env = append(r.env, kv)
		}
	}
	for _, d := range []string{r.dir, r.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"gh": fakeNightlyGH, "fingerprint.sh": fakeFingerprint} {
		if err := os.WriteFile(filepath.Join(r.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r.git(t, "init", "-q", "-b", "main")
	for _, name := range []string{"c1", "c2", "c3", "docs4", "c5", "c6", "c7", "docs8"} {
		r.git(t, "commit", "-q", "--allow-empty", "-m", name+" a change")
		r.commits[name] = r.git(t, "rev-parse", "HEAD")
		switch name {
		case "c1":
			r.git(t, "tag", "release/nightly")
		case "c7":
			r.git(t, "tag", "-a", "-m", "v1.2.3", "v1.2.3")
		}
	}
	return r
}

// git runs git in the repository and returns its output.
func (r *nightlyRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = r.dir, r.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// nightlyCase is one run of What to ship: main's head, the commits whose
// suite passed, the records by commit name, and PLATFORMS.
type nightlyCase struct {
	head      string
	passed    []string
	records   map[string]string
	platforms string
}

// pick runs nightly-pick.sh for the case and returns its outputs and the
// summary it wrote.
func (r *nightlyRepo) pick(t *testing.T, tc nightlyCase) (map[string]string, string) {
	t.Helper()
	script, err := filepath.Abs("nightly-pick.sh")
	if err != nil {
		t.Fatal(err)
	}
	fake := t.TempDir()
	runs := map[string][]map[string]any{}
	for i, name := range tc.passed {
		runs[r.commits[name]] = []map[string]any{{"id": 1000 + i, "conclusion": "success"}}
	}
	// A red run on every other commit.
	for _, sha := range r.commits {
		if _, ok := runs[sha]; !ok {
			runs[sha] = []map[string]any{{"id": 2000 + len(runs), "conclusion": "failure"}}
		}
	}
	records := map[string]string{}
	for name, desc := range tc.records {
		records[r.commits[name]] = "fp1:" + strings.Repeat("0", 64) + " " + desc
	}
	for file, v := range map[string]any{"runs.json": runs, "records.json": records} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fake, file), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bash := scriptShell(t)
	head := r.commits[tc.head]
	output, summary := filepath.Join(fake, "outputs"), filepath.Join(fake, "summary.md")
	cmd := exec.Command(bash, script)
	cmd.Dir = r.dir
	cmd.Env = append(slices.Clone(r.env),
		"PATH="+r.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH="+fake,
		"FPSH="+filepath.Join(r.bin, "fingerprint.sh"),
		"GITHUB_REPOSITORY=owner/repo",
		"GITHUB_SHA="+head,
		"GITHUB_OUTPUT="+output,
		"GITHUB_STEP_SUMMARY="+summary,
		"PLATFORMS="+tc.platforms,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nightly-pick.sh: %v\n%s", err, out)
	}
	b, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		got[k] = v
	}
	s, err := os.ReadFile(summary)
	if err != nil {
		t.Fatal(err)
	}
	return got, string(s)
}

// TestNightlyPick: What to ship takes the newest commit of main whose
// suite passed, and runs the macOS or Windows job on main's head where
// the records of it and of the commits before it say the job did not run
// for its code (AP TestSpeed, D15-B); where that commit is older than
// main's head, the night is skipped, and no output names a commit to
// check out.
func TestNightlyPick(t *testing.T) {
	r := newNightlyRepo(t)
	words := func(mac, win string) string {
		return "ui=true lan=true fuzz=true app=true mac=" + mac + " win=" + win
	}
	short := func(name string) string { return r.commits[name][:12] }
	// What to ship's outputs: sha by commit name, site, macos, windows,
	// and whether it skipped the night.
	type outputs struct {
		sha, site, macos, windows string
		skip                      bool
	}
	for _, tc := range []struct {
		name string
		in   nightlyCase
		want outputs
		says string
	}{
		{"a push to main that no macOS runner took",
			nightlyCase{head: "c3", passed: []string{"c2", "c3"},
				records: map[string]string{"c2": words("true", "true"), "c3": words("false", "unneeded")}},
			outputs{"c3", "true", "true", "false", false},
			"macOS LaunchDaemon did not run for " + short("c3") + "'s code (mac=false): it runs."},
		{"an older commit to ship, a job needed",
			nightlyCase{head: "c5", passed: []string{"c2", "c3", "docs4"},
				records: map[string]string{"c2": words("true", "true"), "c3": words("false", "unneeded")}},
			outputs{"", "true", "false", "false", true},
			"No nightly build tonight: macOS LaunchDaemon did not pass for the code of " + short("docs4") +
				", the newest commit of main whose suite passed, and the jobs run on main's head alone, " + short("c5") +
				", whose suite has not passed; a night after main's head passes ships."},
		{"an older commit to ship, nothing needed",
			nightlyCase{head: "c5", passed: []string{"c3"}, records: map[string]string{"c3": words("true", "true")}},
			outputs{"c3", "true", "false", "false", false},
			"Windows service passed for " + short("c3") + "'s code (win=true): not run."},
		{"passed",
			nightlyCase{head: "c3", passed: []string{"c3"}, records: map[string]string{"c3": words("true", "true")}},
			outputs{"c3", "true", "false", "false", false}, ""},
		{"unneeded down to the nightly build's commit",
			nightlyCase{head: "c3", passed: []string{"c3"},
				records: map[string]string{"c2": words("unneeded", "unneeded"), "c3": words("unneeded", "unneeded")}},
			outputs{"c3", "true", "false", "false", false},
			"Windows service: the nightly build carries " + short("c1") + "'s code for it: not run."},
		{"a record from before mac= and win=",
			nightlyCase{head: "c3", passed: []string{"c3"}, records: map[string]string{"c3": "ui=true lan=true fuzz=true app=true"}},
			outputs{"c3", "true", "false", "false", false}, ""},
		{"no record, code of its own",
			nightlyCase{head: "c6", passed: []string{"c6"}, records: map[string]string{"c5": words("true", "true")}},
			outputs{"c6", "true", "true", "true", false},
			"macOS LaunchDaemon: " + short("c6") + " has no record and code of its own: it runs."},
		{"a release commit at main's head",
			nightlyCase{head: "c7", passed: []string{"c6", "c7"}},
			outputs{"", "false", "false", "false", false}, "main's head is the release commit of v1.2.3"},
		{"a release commit before it",
			nightlyCase{head: "docs8", passed: []string{"c7", "docs8"}},
			outputs{"docs8", "true", "false", "false", false}, "is v1.2.3, whose run passed"},
		{"both by hand",
			nightlyCase{head: "c3", passed: []string{"c3"}, records: map[string]string{"c3": words("true", "true")}, platforms: "both"},
			outputs{"c3", "true", "true", "true", false}, "PLATFORMS=both: both jobs run on main's head, " + short("c3")},
		{"both by hand, nothing to ship",
			nightlyCase{head: "c3", platforms: "both"},
			outputs{"", "true", "true", "true", false}, ""},
		{"both by hand, an older commit and a job needed",
			nightlyCase{head: "c5", passed: []string{"docs4"}, records: map[string]string{"c3": words("false", "true")}, platforms: "both"},
			outputs{"", "true", "true", "true", true}, "No nightly build tonight: macOS LaunchDaemon did not pass"},
		{"nothing to ship",
			nightlyCase{head: "c3"},
			outputs{"", "true", "false", "false", false},
			"no commit of main after " + r.commits["c1"] + " has passed its suite: no nightly build."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, summary := r.pick(t, tc.in)
			for k, v := range map[string]string{"sha": r.commits[tc.want.sha],
				"site": tc.want.site, "macos": tc.want.macos, "windows": tc.want.windows} {
				if got[k] != v {
					t.Errorf("%s = %q, want %q\n%s", k, got[k], v, summary)
				}
			}
			if _, ok := got["test"]; ok {
				t.Errorf("an output test names a commit to check out: %q", got["test"])
			}
			if skipped := got["skip"] != ""; skipped != tc.want.skip {
				t.Errorf("skip = %q, want it set %v\n%s", got["skip"], tc.want.skip, summary)
			}
			if tc.says != "" {
				wants(t, "summary", summary, tc.says)
			}
		})
	}
}

// TestRetryNamesCIJobs: Retry follows the workflows by their names, takes
// the jobs ci.yml's UNRUN_JOBS names, which nightly.yml's caller gives
// the same names, and waits for verdict jobs that are there; platforms.yml
// builds with ci.yml's Go.
func TestRetryNamesCIJobs(t *testing.T) {
	retry := readWorkflow(t, "retry.yml")
	if !strings.Contains(retry, "\n          RETRY_JOBS: '"+retryJobsJSON+"'\n") {
		t.Errorf("retry.yml does not give RETRY_JOBS as %s", retryJobsJSON)
	}
	var unrun map[string]string
	if err := json.Unmarshal([]byte(ciUnrunJobs), &unrun); err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal([]byte(retryJobsJSON), &names); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(unrun))
	for k := range unrun {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	slices.Sort(names)
	if !slices.Equal(keys, names) {
		t.Errorf("RETRY_JOBS %v, UNRUN_JOBS %v", names, keys)
	}
	nightly := readWorkflow(t, "nightly.yml")
	if !strings.Contains(nightly, "\n  platforms:\n    name: Platforms\n") ||
		!strings.Contains(nightly, "    uses: ./.github/workflows/platforms.yml\n") {
		t.Error("nightly.yml's platforms is not Platforms calling platforms.yml")
	}
	if !strings.Contains(retry, "workflows: [ CI, Nightly ]") ||
		!strings.HasPrefix(readWorkflow(t, "ci.yml"), "name: CI\n") || !strings.HasPrefix(nightly, "name: Nightly\n") {
		t.Error("retry.yml does not follow CI and Nightly by their names")
	}
	script, err := os.ReadFile("ci-retry.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "\t.github/workflows/ci.yml)\n") || !strings.Contains(string(script), "\t\tfollower=\"All tests passed\"\n") ||
		!strings.Contains(string(script), "\t.github/workflows/nightly.yml)\n") || !strings.Contains(string(script), "\t\tfollower=\"Platforms passed\"\n") ||
		!workflowJobNames(t, "ci.yml")["All tests passed"] || !workflowJobNames(t, "nightly.yml")["Platforms passed"] {
		t.Error("ci-retry.sh's verdict jobs are not ci.yml's All tests passed and nightly.yml's Platforms passed")
	}
	goVersion := regexp.MustCompile(`\n  GO_VERSION: '([0-9.]+)'\n`)
	ci, platforms := goVersion.FindStringSubmatch(readWorkflow(t, "ci.yml")), goVersion.FindStringSubmatch(readWorkflow(t, "platforms.yml"))
	if ci == nil || platforms == nil || ci[1] != platforms[1] {
		t.Errorf("GO_VERSION: ci.yml %v, platforms.yml %v", ci, platforms)
	}
}
