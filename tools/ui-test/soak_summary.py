#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The UI soak's summary (.github/workflows/ui-soak.yml, AP SteadyTests,
Block 8): of a JUnit XML report of the suite's first pass (run.sh,
UI_TEST_JUNIT), the tests that failed in at least one of their repeats
(UI_TEST_REPEAT, the ids ending in <i>of<n>), each with its case, how
many of its runs failed and the first failure's message, as Markdown for
the job's summary ($GITHUB_STEP_SUMMARY), on standard output.

usage: soak_summary.py <junit.xml> [<title>]

A run is a test's repeat that was not skipped; an error at its set-up or
tear-down fails it as a failure does. UI_TEST_SHUFFLE, UI_TEST_REPEAT and
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
REPEAT = re.compile(r"-[0-9]+of[0-9]+(?=\]$)")
# A case's id among them, such as 1100-en (conftest.py, CASES).
CASE = re.compile(r"(?<=[\[-])[0-9]+-[a-z]{2}(?=[-\]])")


def runs(report):
    """Each run of the report that was not skipped, as (the test's id
    without its repeat, the first line of its failure's message, or None
    where it passed)."""
    for case in ET.parse(report).getroot().iter("testcase"):
        if case.find("skipped") is not None:
            continue
        module, name = case.get("classname", ""), case.get("name", "")
        test = f"{module.replace('.', '/')}.py::{REPEAT.sub('', name)}" if module else REPEAT.sub("", name)
        failed = case.find("failure")
        if failed is None:
            failed = case.find("error")
        message = None
        if failed is not None:
            lines = (failed.get("message") or failed.text or failed.tag).strip().splitlines()
            message = lines[0] if lines else failed.tag
        yield test, message


def cell(text, width=120):
    """Text for a table's cell: one line, at most width characters, with
    nothing in it that Markdown or HTML would take for their own."""
    text = text if len(text) <= width else text[:width - 1] + "…"
    return html.escape(text, quote=False).replace("|", "\\|").replace("`", "'")


def summary(report, title):
    """The summary's Markdown for the report."""
    tests = {}
    for test, message in runs(report):
        ran, failed, first = tests.get(test, (0, 0, None))
        tests[test] = (ran + 1, failed + (message is not None), first or message)
    total = sum(ran for ran, _, _ in tests.values())
    bad = sorted(((t, v) for t, v in tests.items() if v[1]), key=lambda tv: (-tv[1][1], tv[0]))
    seed = os.environ.get("UI_TEST_SHUFFLE", "")
    repeat = os.environ.get("UI_TEST_REPEAT", "") or "1"
    cases = os.environ.get("UI_TEST_CASES", "")
    how = (f"Seed {seed}" if seed else "Not shuffled") + f", each test {repeat} {'time' if repeat == '1' else 'times'}"
    out = [f"## {title}", ""]
    if not bad:
        out.append(f"{how}: no test failed in {total} runs.")
    else:
        fails = sum(v[1] for _, v in bad)
        out += [f"{how}: {len(bad)} {'test' if len(bad) == 1 else 'tests'} failed at least once, "
                f"in {fails} of {total} runs.", "",
                "| Test | Case | Failed | First failure |", "|------|------|--------|---------------|"]
        for test, (ran, failed, first) in bad:
            case = CASE.search(test)
            out.append(f"| `{test}` | {case.group(0) if case else ''} | {failed} of {ran} | {cell(first)} |")
        again = " ".join(f"{k}={v}" for k, v in (("UI_TEST_CASES", cases), ("UI_TEST_REPEAT", repeat if repeat != "1" else ""),
                                                 ("UI_TEST_SHUFFLE", seed)) if v)
        out += ["", f"Again in the same order: `{again + ' ' if again else ''}make ui-test`"]
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
