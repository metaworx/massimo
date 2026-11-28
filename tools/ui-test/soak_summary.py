#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The UI soak's summary (.github/workflows/ui-soak.yml, AP SteadyTests,
Block 8): of a JUnit XML report of the suite's first pass (run.sh,
UI_TEST_JUNIT), the tests that failed in at least one of their repeats
(UI_TEST_REPEAT, the ids ending in <i>of<n>), each with its case, how
many of its runs failed, each failed run with its worker and what the
retry did with it, and the first failure's message, as Markdown for the
job's summary ($GITHUB_STEP_SUMMARY), on standard output.

usage: soak_summary.py <junit.xml> [<title>]

A run is a test's repeat that was not skipped; an error at its set-up or
tear-down fails it as a failure does. A run's worker is the pytest-xdist
worker, gw<n>, whose server set it ran against, which conftest.py
records as the property worker of its testcase: a test that fails on one
worker alone met what an earlier file left on that server (AP
SteadyTests, Block 11). The retry is run.sh's run once more of the
failed tests, whose JUnit report run.sh writes beside the first's, its
name ending in .retry.xml in place of .xml (ui-soak-junit.retry.xml);
each failed run says whether the retry passed it or failed it again, and
on which worker, which is whichever was free: a retry that passed may
only have met a server without the leftover. No retry report, as where
the retry did not run, says so. UI_TEST_SHUFFLE, UI_TEST_REPEAT and
UI_TEST_CASES, as the run had them, give the line that runs the suite
again in the same order where a test failed. It exits 0 where it read
the report, whatever failed in it, since the suite's own step fails the
job; 1 where the report is missing or cannot be read, after saying so.
"""
import html
import os
import re
import sys
import xml.etree.ElementTree as ET

# The repeat at the end of an id's parameters (conftest.py's last one).
REPEAT = re.compile(r"-([0-9]+of[0-9]+)(?=\]$)")
# A case's id among them, such as 1100-en (conftest.py, CASES).
CASE = re.compile(r"(?<=[\[-])[0-9]+-[a-z]{2}(?=[-\]])")


def runs(report):
    """Each run of the report that was not skipped, as (the test's id
    without its repeat, the repeat (such as 2of3, "" without one), the
    first line of its failure's message, or None where it passed, and its
    worker, "" where the report names none)."""
    for case in ET.parse(report).getroot().iter("testcase"):
        if case.find("skipped") is not None:
            continue
        module, name = case.get("classname", ""), case.get("name", "")
        repeat = REPEAT.search(name)
        test = f"{module.replace('.', '/')}.py::{REPEAT.sub('', name)}" if module else REPEAT.sub("", name)
        failed = case.find("failure")
        if failed is None:
            failed = case.find("error")
        message = None
        if failed is not None:
            lines = (failed.get("message") or failed.text or failed.tag).strip().splitlines()
            message = lines[0] if lines else failed.tag
        worker = next((p.get("value", "") for p in case.iter("property") if p.get("name") == "worker"), "")
        yield test, repeat.group(1) if repeat else "", message, worker


def retry_report(report):
    """The path of the retry's report beside the first pass's (run.sh):
    its name ending in .retry.xml in place of .xml."""
    stem = report[:-len(".xml")] if report.endswith(".xml") else report
    return stem + ".retry.xml"


def retried(report):
    """The retry's runs, by (test, repeat): (passed, worker); None where
    there is no retry report, or none that can be read."""
    try:
        return {(test, repeat): (message is None, worker) for test, repeat, message, worker in runs(retry_report(report))}
    except (OSError, ET.ParseError):
        return None


def cell(text, width=120):
    """Text for a table's cell: one line, at most width characters, with
    nothing in it that Markdown or HTML would take for their own."""
    text = text if len(text) <= width else text[:width - 1] + "…"
    return html.escape(text, quote=False).replace("|", "\\|").replace("`", "'")


def failure_line(repeat, worker, retry):
    """A failed run as a summary names it: its repeat and its worker,
    then what the retry did with it."""
    where = " ".join(x for x in (repeat, f"on {worker}" if worker else "") if x) or "the run"
    if retry is None:
        return f"{where}: no retry"
    if not retry:
        return f"{where}: not retried"
    passed, again_on = retry
    return f"{where}: retry {'passed' if passed else 'failed'}{' on ' + again_on if again_on else ''}"


def summary(report, title):
    """The summary's Markdown for the report."""
    tests = {}
    for test, repeat, message, worker in runs(report):
        ran, failures, first = tests.get(test, (0, [], None))
        if message is not None:
            failures = failures + [(repeat, worker)]
        tests[test] = (ran + 1, failures, first or message)
    total = sum(ran for ran, _, _ in tests.values())
    bad = sorted(((t, v) for t, v in tests.items() if v[1]), key=lambda tv: (-len(tv[1][1]), tv[0]))
    again = retried(report)
    seed = os.environ.get("UI_TEST_SHUFFLE", "")
    repeat = os.environ.get("UI_TEST_REPEAT", "") or "1"
    cases = os.environ.get("UI_TEST_CASES", "")
    how = (f"Seed {seed}" if seed else "Not shuffled") + f", each test {repeat} {'time' if repeat == '1' else 'times'}"
    out = [f"## {title}", ""]
    if not bad:
        out.append(f"{how}: no test failed in {total} runs.")
    else:
        fails = sum(len(v[1]) for _, v in bad)
        out += [f"{how}: {len(bad)} {'test' if len(bad) == 1 else 'tests'} failed at least once, "
                f"in {fails} of {total} runs.", "",
                "| Test | Case | Failed | Its failed runs: worker, retry | First failure |",
                "|------|------|--------|--------------------------------|---------------|"]
        for test, (ran, failures, first) in bad:
            case = CASE.search(test)
            each = "; ".join(failure_line(r, w, None if again is None else again.get((test, r), ()))
                             for r, w in sorted(failures, key=lambda rw: int(rw[0].split("of")[0]) if rw[0] else 0))
            out.append(f"| `{test}` | {case.group(0) if case else ''} | {len(failures)} of {ran} | {cell(each, 400)} | {cell(first)} |")
        line = " ".join(f"{k}={v}" for k, v in (("UI_TEST_CASES", cases), ("UI_TEST_REPEAT", repeat if repeat != "1" else ""),
                                                ("UI_TEST_SHUFFLE", seed)) if v)
        out += ["", f"Again in the same order: `{line + ' ' if line else ''}make ui-test`"]
        if again is None:
            out += ["", f"No report of the retry ({os.path.basename(retry_report(report))}): "
                    "the retry did not run, or stopped before it wrote one."]
    return "\n".join(out) + "\n"


def main(argv):
    if len(argv) not in (2, 3):
        print("usage: soak_summary.py <junit.xml> [<title>]", file=sys.stderr)
        return 2
    report, title = argv[1], argv[2] if len(argv) == 3 else "UI soak"
    try:
        text = summary(report, title)
    except (OSError, ET.ParseError) as e:
        print(f"## {title}\n\nNo report of the suite's first pass ({e}): the suite stopped before it wrote one; "
              "the log of its step says why.")
        return 1
    sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
