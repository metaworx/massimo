#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Cut the web UI's scripts along their regions, by moves alone (AP UiParts).

  split.py [--only SCRIPT ...]            cut, in the working tree
  split.py --check [--only SCRIPT ...]    the tree is what a cut makes of it
  split.py --compare A B [--only ...]     the cut of A is B (git trees or dirs)
  split.py --list SCRIPT                  the top-level items of a script
  split.py --self-test                    the tool's own tests

The map (map.txt, README.md) names, for each script that is cut, every
top-level item of it and the script it belongs in. The tool moves each item
with the comment block directly above it, byte for byte, in the order the
map gives, and puts the scripts into ui.go's go:embed line and the page's
script tags in the place of the script cut. SCRIPT is a cut script's name as
the map's group has it (app.js). Python 3, the standard library alone.
"""

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest

UI = "internal/server/ui"
UI_GO = "internal/server/ui.go"
INDEX = UI + "/index.html"
MAP = "tools/ui-split/map.txt"

USE_STRICT = '"use strict";\n'
HEADING = re.compile(r"^// --- ")
DECL = re.compile(r"^(?:async\s+)?function\s*\*?\s*([\w$]+)\s*\(|^(?:const|let|var)\s+([\w$]+)|^class\s+([\w$]+)\b")
SCRIPT_TAG = re.compile(r'^<script src="ui/([^"]+\.js)"></script>\n$')
COPYRIGHT = "// SPDX-License-Identifier: AGPL-3.0-or-later\n// Copyright (C) 2026 The Massimo Authors\n"


def readfile(path, mode="r"):
    """readfile returns a file's text as it is, its line ends untouched,
    or its bytes for mode "rb"."""
    if mode == "rb":
        with open(path, "rb") as f:
            return f.read()
    with open(path, encoding="utf-8", newline="") as f:
        return f.read()


class Stop(Exception):
    """A reason the tool stops without writing anything."""


# --- reading a script -------------------------------------------------------


class Unit:
    """A top-level item of a script: a declaration, a statement, a section
    heading or a comment that stands alone, with the comment block directly
    above it. key names it in the map: a declaration's name, or "> " and
    the first line of anything else. gap is the number of blank lines
    before it, pred the key of the unit before it where it was read (None
    for the first)."""

    def __init__(self, kind, key, text, gap, pred, code):
        self.kind, self.key, self.text, self.gap, self.pred, self.code = kind, key, text, gap, pred, code


def parse(text, name):
    """parse returns a script's header (up to and with its "use strict"
    line) and its units. A unit starts on a line at column 0 that is not
    blank, not a comment and not a closing bracket; the lines after it that
    are indented, blank or start with a closing bracket are its own, as the
    scripts' indentation puts everything nested further in."""
    lines = text.splitlines(keepends=True)
    if not text.endswith("\n"):
        raise Stop(f"{name}: does not end with a newline")
    try:
        h = lines.index(USE_STRICT) + 1
    except ValueError:
        raise Stop(f'{name}: no line "use strict";') from None
    header = "".join(lines[:h])
    units, pending, cur = [], [], None
    open_units = []  # (unit, its comment block, its lines) until the end

    def flush_free(block, gap):
        # a heading, or a comment block that a blank line parts from what
        # follows it, is a unit of its own
        pred = units[-1].key if units else None
        units.append(Unit("free", "> " + block[0].rstrip("\n"), "".join(block), gap, pred, ""))

    def split_pending():
        # pending is what came between two units: blank lines, headings and
        # comments. It returns the blank lines before the comment block
        # attached to the next unit, and that block.
        gap, block = 0, []
        for line in pending:
            if not line.strip():
                if block:
                    flush_free(block, gap)
                    block, gap = [], 0
                gap += 1
            elif HEADING.match(line):
                if block:
                    flush_free(block, gap)
                    gap = 0
                flush_free([line], gap)
                block, gap = [], 0
            else:
                block.append(line)
        return gap, block

    for line in lines[h:]:
        blank = not line.strip()
        if blank or line.startswith("//"):
            pending.append(line)
            continue
        if line[0] in " \t}])":
            if cur is None:
                raise Stop(f"{name}: an indented line before any item: {line.rstrip()}")
            cur.extend(pending)
            pending = []
            cur.append(line)
            continue
        # a new unit
        gap, block = split_pending()
        pending = []
        m = DECL.match(line)
        if m:
            kind, key = "decl", next(g for g in m.groups() if g)
        else:
            kind, key = "stmt", "> " + line.rstrip("\n")
        cur = [line]
        pred = units[-1].key if units else None
        units.append(Unit(kind, key, None, gap, pred, None))
        open_units.append((units[-1], "".join(block), cur))
    gap, block = split_pending()
    if block:
        flush_free(block, gap)
    for u, comment, code in open_units:
        u.code = "".join(code)
        u.text = comment + u.code
        balanced(u.code, f"{name}: {u.key}")
    return header, units


def balanced(code, where):
    """balanced stops where an item's brackets do not close, a sign that
    the item was not read as one: strings, comments and regular expressions
    are skipped as well as a few lines of looking can."""
    depth, i, n, prev = [], 0, len(code), ""
    pairs = {")": "(", "]": "[", "}": "{"}
    templ = []  # the bracket depth at each open ${ of a template
    while i < n:
        c = code[i]
        if c in " \t\n":
            i += 1
            continue
        if code.startswith("//", i):
            i = code.find("\n", i)
            i = n if i < 0 else i
            continue
        if code.startswith("/*", i):
            j = code.find("*/", i + 2)
            if j < 0:
                raise Stop(f"{where}: a comment that does not close")
            i = j + 2
            continue
        if c in "'\"":
            j = i + 1
            while j < n and code[j] != c:
                j += 2 if code[j] == "\\" else 1
            i, prev = j + 1, "s"
            continue
        if c == "`" or (c == "}" and templ and templ[-1] == len(depth)):
            if c == "}":
                templ.pop()
            j = i + 1
            while j < n and code[j] != "`":
                if code[j] == "\\":
                    j += 2
                    continue
                if code.startswith("${", j):
                    break
                j += 1
            if j < n and code[j] == "$":
                templ.append(len(depth))
                i = j + 2
            else:
                i, prev = j + 1, "s"
            continue
        if c == "/" and (prev == "" or prev in "(,=:[!&|?{};+-*%<>~^" or prev == "kw"):
            j, cls = i + 1, False
            while j < n and code[j] != "\n":
                if code[j] == "\\":
                    j += 2
                    continue
                if code[j] == "[":
                    cls = True
                elif code[j] == "]":
                    cls = False
                elif code[j] == "/" and not cls:
                    break
                j += 1
            i, prev = j + 1, "s"
            continue
        if c in "([{":
            depth.append(c)
        elif c in ")]}":
            if not depth or depth.pop() != pairs[c]:
                raise Stop(f"{where}: a {c} that closes nothing")
        if c.isalnum() or c in "_$":
            j = i
            while j < n and (code[j].isalnum() or code[j] in "_$"):
                j += 1
            word = code[i:j]
            prev = "kw" if word in ("return", "typeof", "case", "in", "of", "instanceof", "new", "delete", "void", "throw", "else", "do") else "w"
            i = j
            continue
        prev = c
        i += 1
    if depth or templ:
        raise Stop(f"{where}: {''.join(depth)} left open")


# --- the map ------------------------------------------------------------------


class Group:
    """A script that is cut: its name, the scripts it is cut into in their
    load order, a sentence for each new one's header, and every item with
    the script it belongs in, in the order the scripts get them."""

    def __init__(self, source, order):
        self.source, self.order = source, order
        self.about, self.items = {}, []


def read_map(text, name="map.txt"):
    """read_map returns the map's groups (README.md, "The map"); a line it
    cannot read, or an item placed twice, stops it."""
    groups, g, about = [], None, None
    for no, raw in enumerate(text.splitlines(), 1):
        line = raw.rstrip()
        where = f"{name}:{no}"
        if not line or line.lstrip().startswith("#"):
            about = None
            continue
        if line.startswith("["):
            m = re.match(r"^\[([\w.-]+\.js)\]((?:\s+[\w.-]+\.js)+)$", line)
            if not m:
                raise Stop(f"{where}: a group is [script.js] and the scripts it is cut into")
            order = m.group(2).split()
            if len(set(order)) != len(order):
                raise Stop(f"{where}: a script named twice")
            if any(x.source == m.group(1) for x in groups):
                raise Stop(f"{where}: {m.group(1)} has a group already")
            g = Group(m.group(1), order)
            groups.append(g)
            about = None
            continue
        if g is None:
            raise Stop(f"{where}: before the first group")
        if line[0] in " \t":
            if about is None:
                raise Stop(f"{where}: an indented line continues an about line alone")
            g.about[about] += " " + line.strip()
            continue
        if line.startswith("about "):
            m = re.match(r"^about ([\w.-]+\.js): (.+)$", line)
            if not m or m.group(1) not in g.order:
                raise Stop(f"{where}: about names a script of the group: about name.js: text")
            about = m.group(1)
            g.about[about] = m.group(2).strip()
            continue
        about = None
        parts = line.split(None, 1)
        if len(parts) != 2 or parts[0] not in g.order:
            raise Stop(f"{where}: an item line is a script of [{g.source}] and an item: {line}")
        g.items.append((parts[1], parts[0], no))
    for g in groups:
        seen = {}
        for key, _, no in g.items:
            if key in seen:
                raise Stop(f"{name}:{no}: {key} is placed twice in [{g.source}] (first on line {seen[key]})")
            seen[key] = no
    return groups


# --- cutting -------------------------------------------------------------------


def new_header(script, about):
    """new_header is a new script's header: the SPDX and copyright lines,
    the map's sentence for it as a comment, and "use strict"."""
    if not about:
        raise Stop(f"{script}: a new script, and the map has no about line for it")
    body = textwrap.wrap(about, width=69, break_long_words=False, break_on_hyphens=False)
    return COPYRIGHT + "\n" + "".join("// " + x + "\n" for x in body) + USE_STRICT


def emit(header, units):
    """emit writes a script: its header, then its units, each after the
    blank lines it had where it was read if it follows the unit it followed
    there, after one blank line otherwise."""
    out, prev = [header], None
    for u in units:
        out.append("\n" * (u.gap if u.pred == prev else 1))
        out.append(u.text)
        prev = u.key
    return "".join(out)


def cut_group(files, g):
    """cut_group returns the group's scripts as the map has them, from the
    scripts in files (name: text) that exist: the source, and those it has
    been cut into already."""
    if g.source not in files:
        raise Stop(f"{g.source}: not there")
    parsed = {s: parse(files[s], s) for s in [g.source] + g.order if s in files}
    placed = {key: (target, no) for key, target, no in g.items}
    seen = {}  # key: [(script, unit)]
    errors = []
    for s, (_, units) in parsed.items():
        for u in units:
            seen.setdefault(u.key, []).append((s, u))
            if u.key not in placed:
                errors.append(f"{s}: {u.key} is not in the map's [{g.source}]")
    for key, (target, no) in placed.items():
        where = seen.get(key, [])
        if not where:
            errors.append(f"map.txt:{no}: {key} is in none of [{g.source}]'s scripts")
            continue
        scripts = [s for s, _ in where]
        if len(where) == 1:
            continue
        # a merge of main brought a moved item back into the source: the
        # source's version takes the place of its copy in the target
        if len(where) == 2 and g.source in scripts and target in scripts and target != g.source:
            src = next(u for s, u in where if s == g.source)
            copy = next(u for s, u in where if s == target)
            copy.text, copy.kind, copy.code = src.text, src.kind, src.code
            seen[key] = [(target, copy)]
            continue
        errors.append(f"{key} is there {len(where)} times, in {', '.join(scripts)}")
    if errors:
        raise Stop("\n".join(errors))
    out = {}
    for s in g.order:
        units = [seen[key][0][1] for key, target, _ in g.items if target == s]
        if not units:
            raise Stop(f"{s}: the map gives it no item")
        header = parsed[s][0] if s in parsed else new_header(s, g.about.get(s))
        out[s] = emit(header, units)
    if g.source not in g.order:
        out[g.source] = None  # it is cut into others entirely: it goes
    return out


def replace_run(items, key, names):
    """replace_run takes the group's scripts out of a list and puts them
    back, in their order, where the first of them was."""
    at = [i for i, x in enumerate(items) if key(x) in names]
    if not at:
        return None
    rest = [x for i, x in enumerate(items) if i not in at]
    return rest[: at[0]], rest[at[0]:]


def update_lists(ui_go, index, g):
    """update_lists puts the group's scripts into ui.go's go:embed line
    and the page's script tags, in the place of the script cut."""
    names = set([g.source] + g.order)
    lines = ui_go.splitlines(keepends=True)
    em = [i for i, x in enumerate(lines) if x.startswith("//go:embed ")]
    if len(em) != 1:
        raise Stop(f"ui.go: {len(em)} go:embed lines, want 1")
    toks = lines[em[0]].rstrip("\n").split(" ")
    r = replace_run(toks[1:], lambda x: x[3:] if x.startswith("ui/") else None, names)
    if r is None:
        raise Stop(f"ui.go: go:embed names no script of [{g.source}]")
    lines[em[0]] = " ".join([toks[0]] + r[0] + ["ui/" + s for s in g.order] + r[1]) + "\n"
    ui_go = "".join(lines)
    lines = index.splitlines(keepends=True)
    tags = [i for i, x in enumerate(lines) if SCRIPT_TAG.match(x)]
    if not tags or tags != list(range(tags[0], tags[0] + len(tags))):
        raise Stop("index.html: the script tags are not lines of their own, one after the other")
    block = lines[tags[0]: tags[-1] + 1]
    r = replace_run(block, lambda x: SCRIPT_TAG.match(x).group(1), names)
    if r is None:
        raise Stop(f"index.html: no script tag of [{g.source}]")
    block = r[0] + [f'<script src="ui/{s}"></script>\n' for s in g.order] + r[1]
    index = "".join(lines[: tags[0]] + block + lines[tags[-1] + 1:])
    return ui_go, index


def page_scripts(index):
    """page_scripts are the scripts the page's tags load, in their order."""
    return [m.group(1) for m in map(SCRIPT_TAG.match, index.splitlines(keepends=True)) if m]


IDENT = re.compile(r"(?<![\w$.])[A-Za-z_$][\w$]*")


def strip_code(code):
    """strip_code is code without its comments and its strings' text, for
    the identifiers it names."""
    code = re.sub(r"//[^\n]*", "", code)
    code = re.sub(r"/\*.*?\*/", "", code, flags=re.S)
    return re.sub(r"'(?:\\.|[^'\\\n])*'|\"(?:\\.|[^\"\\\n])*\"|`(?:\\.|[^`\\])*`", '""', code)


CALL = re.compile(r"(?<![\w$.])([A-Za-z_$][\w$]*)\s*\(")


def load_order(files, index):
    """load_order lists what runs as the page's scripts load and needs a
    top-level item of a script that loads later: a statement or a const or
    let initialiser names it, or calls a function that calls it, at any
    depth. A call inside a function that runs later counts as well, so a
    hit is a question: run stops on one that the tree it starts from does
    not have already, by the item and the name."""
    order = page_scripts(index)
    where, units, body = {}, {}, {}
    for s in order:
        if s not in files or files[s] is None:
            continue
        try:
            _, us = parse(files[s], s)
        except Stop:
            continue
        units[s] = us
        for u in us:
            if u.kind == "decl" and u.key not in where:
                where[u.key] = s
                if re.match(r"(async\s+)?function\b", u.code):
                    body[u.key] = set(CALL.findall(strip_code(u.code)))
    hits = []
    for i, s in enumerate(order):
        for u in units.get(s, []):
            if not (u.kind == "stmt" or (u.kind == "decl" and re.match(r"(const|let|var)\s", u.code))):
                continue
            code = strip_code(u.code)
            reached, called = set(IDENT.findall(code)), set()
            todo = list(CALL.findall(code))
            while todo:
                f = todo.pop()
                if f in called:
                    continue
                called.add(f)
                todo.extend(body.get(f, ()))
            reached |= called
            for name in sorted(reached):
                d = where.get(name)
                if d and d != s and order.index(d) > i:
                    hits.append(((u.key, name), f"{s}: {u.key} needs {name} of {d}, which loads after it"))
    return hits


def run(root, groups, only, write=True):
    """run cuts the groups (only those named, where only is given) in the
    tree at root; it returns the files that differ, and writes them where
    write is true. Nothing is written where anything stops it."""
    if only:
        unknown = set(only) - {g.source for g in groups}
        if unknown:
            raise Stop(f"no group in the map for {', '.join(sorted(unknown))}")
        groups = [g for g in groups if g.source in only]
    def read(p):
        return readfile(os.path.join(root, p))
    files = {}
    for g in groups:
        for s in [g.source] + g.order:
            p = os.path.join(root, UI, s)
            if os.path.exists(p):
                files[s] = read(os.path.join(UI, s))
    ui_go, index = read(UI_GO), read(INDEX)
    before = page_scripts(index)
    known = {k for k, _ in load_order(_all_scripts(root, before), index)}
    out = dict(files)
    for g in groups:
        out.update(cut_group(files, g))
        ui_go, index = update_lists(ui_go, index, g)
    allfiles = _all_scripts(root, page_scripts(index))
    allfiles.update(out)
    new = [m for k, m in load_order(allfiles, index) if k not in known]
    if new:
        raise Stop("the load order breaks:\n" + "\n".join(new))
    changed = {}
    for s, text in out.items():
        if files.get(s) != text:
            changed[os.path.join(UI, s)] = text
    if read(UI_GO) != ui_go:
        changed[UI_GO] = ui_go
    if read(INDEX) != index:
        changed[INDEX] = index
    if write:
        for p, text in sorted(changed.items()):
            full = os.path.join(root, p)
            if text is None:
                os.remove(full)
                continue
            tmp = full + ".ui-split"
            with open(tmp, "w", encoding="utf-8", newline="") as f:
                f.write(text)
            os.replace(tmp, full)
    return changed


def _all_scripts(root, names):
    out = {}
    for s in names:
        p = os.path.join(root, UI, s)
        if os.path.exists(p):
            out[s] = readfile(p)
    return out


# --- comparing ------------------------------------------------------------------


def materialize(spec, into, repo):
    """materialize puts the UI's files of spec, a directory or a git tree
    (a commit, a tree id), into the directory into; it returns the
    directory that holds them."""
    if os.path.isdir(spec):
        return spec
    os.makedirs(into)
    tar = subprocess.run(["git", "-C", repo, "archive", "--format=tar", spec, UI, UI_GO, MAP], capture_output=True)
    if tar.returncode:
        raise Stop(f"{spec}: {tar.stderr.decode().strip()}")
    subprocess.run(["tar", "-x", "-C", into], input=tar.stdout, check=True)
    return into


def ui_tree(root):
    """ui_tree returns the files the tool reads and writes, by path."""
    out = {}
    for p in [UI_GO, INDEX]:
        out[p] = readfile(os.path.join(root, p), "rb")
    d = os.path.join(root, UI)
    for f in sorted(os.listdir(d)):
        if f.endswith(".js"):
            out[UI + "/" + f] = readfile(os.path.join(d, f), "rb")
    return out


def compare(a, b, repo, mapfile, only):
    """compare cuts a copy of tree a and returns how it differs from tree
    b, in the files the tool writes, and, for two git trees, every other
    path the two trees differ in."""
    tmp = tempfile.mkdtemp(prefix="ui-split-")
    try:
        da = materialize(a, os.path.join(tmp, "a"), repo)
        db = materialize(b, os.path.join(tmp, "b"), repo)
        work = os.path.join(tmp, "work")
        shutil.copytree(da, work)
        mp = mapfile or os.path.join(da, MAP)
        groups = read_map(readfile(mp), mp)
        run(work, groups, only)
        got, want = ui_tree(work), ui_tree(db)
        diffs = [f"{p}: {'missing in B' if p not in want else 'not made by the tool' if p not in got else 'differs'}"
                 for p in sorted(set(got) | set(want)) if got.get(p) != want.get(p)]
        if not (os.path.isdir(a) or os.path.isdir(b)):
            names = subprocess.run(["git", "-C", repo, "diff", "--name-only", a, b], capture_output=True, text=True, check=True).stdout.split()
            for p in names:
                if not (p in (UI_GO, INDEX) or (p.startswith(UI + "/") and p.endswith(".js") and "/" not in p[len(UI) + 1:])):
                    diffs.append(f"{p}: changed beside the cut")
        return diffs
    finally:
        shutil.rmtree(tmp)


# --- the self-test --------------------------------------------------------------

T_UI_GO = "package server\n\n//go:embed ui/index.html ui/a.js ui/big.js ui/z.js ui/style.css\nvar uiFiles embed.FS\n"
T_INDEX = '<body>\n<script src="ui/a.js"></script>\n<script src="ui/big.js"></script>\n<script src="ui/z.js"></script>\n</body>\n'
T_BIG = (COPYRIGHT + "\n// big: two regions.\n" + USE_STRICT + "\n"
         "const ONE = 1;\nconst TWO = 2;\n\n"
         "// --- first ---\n\n"
         "// f does one thing.\nfunction f() {\n  return ONE;\n\n  // inside\n}\n\n"
         "// --- second ---\n\n"
         "// g does another,\n// in two lines.\nfunction g() {\n  return `${TWO}` + \"}\";\n}\n\n"
         "// a word on its own\n\n"
         "const re = /[}]/;\n\n"
         'document.addEventListener("DOMContentLoaded", f);\n')
T_MAP = ("# test\n[big.js] big.js second.js\n"
         "about second.js: The second region, in a script of its own, which is\n"
         "  long enough to wrap onto a second line of the header's comment.\n"
         "big.js ONE\nsecond.js TWO\nbig.js > // --- first ---\nbig.js f\n"
         "second.js > // --- second ---\nsecond.js g\nsecond.js > // a word on its own\nsecond.js re\n"
         'big.js > document.addEventListener("DOMContentLoaded", f);\n')


class SelfTest(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp(prefix="ui-split-test-")
        os.makedirs(os.path.join(self.root, UI))
        self.write(UI_GO, T_UI_GO)
        self.write(INDEX, T_INDEX)
        self.write(UI + "/a.js", COPYRIGHT + USE_STRICT + "\nconst A = 1;\n")
        self.write(UI + "/z.js", COPYRIGHT + USE_STRICT + "\nf();\n")
        self.write(UI + "/big.js", T_BIG)
        self.groups = read_map(T_MAP)

    def tearDown(self):
        shutil.rmtree(self.root)

    def write(self, p, text):
        with open(os.path.join(self.root, p), "w", newline="") as f:
            f.write(text)

    def read(self, p):
        return readfile(os.path.join(self.root, p))

    def test_move(self):
        changed = run(self.root, self.groups, None)
        self.assertEqual(sorted(changed), sorted([UI_GO, UI + "/big.js", UI + "/second.js", INDEX]))
        big = self.read(UI + "/big.js")
        self.assertEqual(big, COPYRIGHT + "\n// big: two regions.\n" + USE_STRICT + "\n"
                         "const ONE = 1;\n\n// --- first ---\n\n// f does one thing.\nfunction f() {\n  return ONE;\n\n  // inside\n}\n\n"
                         'document.addEventListener("DOMContentLoaded", f);\n')
        second = self.read(UI + "/second.js")
        self.assertEqual(second, COPYRIGHT + "\n// The second region, in a script of its own, which is long enough to\n"
                         "// wrap onto a second line of the header's comment.\n" + USE_STRICT + "\n"
                         "const TWO = 2;\n\n// --- second ---\n\n// g does another,\n// in two lines.\nfunction g() {\n"
                         "  return `${TWO}` + \"}\";\n}\n\n// a word on its own\n\nconst re = /[}]/;\n")
        self.assertIn("ui/a.js ui/big.js ui/second.js ui/z.js ui/style.css", self.read(UI_GO))
        self.assertEqual(self.read(INDEX), T_INDEX.replace('big.js"></script>\n', 'big.js"></script>\n<script src="ui/second.js"></script>\n'))

    def test_idempotent(self):
        run(self.root, self.groups, None)
        again = run(self.root, self.groups, None, write=False)
        self.assertEqual(again, {})

    def test_unknown_item(self):
        self.write(UI + "/big.js", T_BIG + "\nfunction h() {}\n")
        with self.assertRaisesRegex(Stop, r"big.js: h is not in the map"):
            run(self.root, self.groups, None)
        self.assertEqual(self.read(UI + "/big.js"), T_BIG + "\nfunction h() {}\n")  # nothing written
        self.assertFalse(os.path.exists(os.path.join(self.root, UI, "second.js")))

    def test_missing_and_twice(self):
        self.write(UI + "/big.js", T_BIG.replace("const TWO = 2;\n", ""))
        with self.assertRaisesRegex(Stop, r"TWO is in none"):
            run(self.root, self.groups, None)
        self.write(UI + "/big.js", T_BIG + "\nconst TWO = 3;\n")
        with self.assertRaisesRegex(Stop, r"TWO is there 2 times"):
            run(self.root, self.groups, None)

    def test_recut(self):
        run(self.root, self.groups, None)
        cut = self.read(UI + "/second.js")
        # a merge of main brings g back into big.js where main changed it,
        # its older copy still in second.js
        big = self.read(UI + "/big.js")
        newg = "// g does another,\n// now changed on main.\nfunction g() {\n  return TWO + 1;\n}\n"
        self.write(UI + "/big.js", big.replace("}\n\ndocument", "}\n\n" + newg + "\ndocument"))
        run(self.root, self.groups, None)
        self.assertEqual(self.read(UI + "/second.js"), cut.replace(
            "// g does another,\n// in two lines.\nfunction g() {\n  return `${TWO}` + \"}\";\n}\n", newg))
        self.assertNotIn("function g", self.read(UI + "/big.js"))
        self.assertEqual(run(self.root, self.groups, None, write=False), {})
        # a declaration new on main stops the tool until the map places it
        self.write(UI + "/big.js", self.read(UI + "/big.js") + "\nfunction fresh() {}\n")
        with self.assertRaisesRegex(Stop, "fresh is not in the map"):
            run(self.root, self.groups, None)
        groups = read_map(T_MAP + "second.js fresh\n")
        run(self.root, groups, None)
        self.assertTrue(self.read(UI + "/second.js").endswith("const re = /[}]/;\n\nfunction fresh() {}\n"))

    def test_load_order(self):
        groups = read_map(T_MAP.replace("[big.js] big.js second.js", "[big.js] second.js big.js").replace("big.js ONE", "second.js ONE").replace("second.js TWO", "big.js TWO"))
        self.write(UI + "/big.js", T_BIG.replace("const TWO = 2;", "const TWO = ONE + 1;"))
        run(self.root, groups, None)  # ONE in second.js, which loads first
        self.write(UI + "/big.js", T_BIG.replace("const TWO = 2;", "const TWO = ONE + 1;"))
        os.remove(os.path.join(self.root, UI, "second.js"))
        self.write(INDEX, T_INDEX)
        self.write(UI_GO, T_UI_GO)
        groups = read_map(T_MAP.replace("big.js ONE", "second.js ONE").replace("second.js TWO", "big.js TWO"))
        with self.assertRaisesRegex(Stop, "TWO needs ONE of second.js, which loads after it"):
            run(self.root, groups, None)

    def test_compare(self):
        cut = self.root + "-cut"
        shutil.copytree(self.root, cut)
        try:
            mp = os.path.join(self.root, "map.txt")
            with open(mp, "w") as f:
                f.write(T_MAP)
            run(cut, self.groups, None)
            self.assertEqual(compare(self.root, cut, self.root, mp, None), [])
            with open(os.path.join(cut, UI, "second.js"), "a") as f:
                f.write("\n")
            self.assertEqual(compare(self.root, cut, self.root, mp, None), [UI + "/second.js: differs"])
        finally:
            shutil.rmtree(cut)

    def test_unbalanced(self):
        with self.assertRaisesRegex(Stop, "left open"):
            parse(COPYRIGHT + USE_STRICT + "function f() {\n  if (x) {\n}\n", "t.js")


# --- the command line -----------------------------------------------------------


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--root", help="the repository (default: the one this file is in)")
    ap.add_argument("--map", help="the map (default: tools/ui-split/map.txt of the tree)")
    ap.add_argument("--only", nargs="+", metavar="SCRIPT", help="cut these groups alone")
    ap.add_argument("--check", action="store_true", help="the tree is its own cut")
    ap.add_argument("--compare", nargs=2, metavar=("A", "B"), help="the cut of A is B")
    ap.add_argument("--list", metavar="SCRIPT", help="list a script's top-level items")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    root = a.root or os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    try:
        if a.self_test:
            r = unittest.main(argv=[sys.argv[0]], exit=False, verbosity=1).result
            return 0 if r.wasSuccessful() else 1
        if a.list:
            _, units = parse(readfile(os.path.join(root, UI, a.list)), a.list)
            for u in units:
                print(u.key)
            return 0
        if a.compare:
            diffs = compare(a.compare[0], a.compare[1], root, a.map, a.only)
            for d in diffs:
                print(d)
            print("the cut of A is B" if not diffs else f"{len(diffs)} differences", file=sys.stderr)
            return 1 if diffs else 0
        mp = a.map or os.path.join(root, MAP)
        groups = read_map(readfile(mp), mp)
        changed = run(root, groups, a.only, write=not a.check)
        for p in sorted(changed):
            print(("would change " if a.check else "") + p)
        if a.check:
            if changed:
                print("split.py --check: the tree is not the map's cut", file=sys.stderr)
                return 1
            print("split.py --check: the tree is the map's cut", file=sys.stderr)
        return 0
    except Stop as e:
        print(f"split.py: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
