#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""The site's documentation pages, for packaging/site/build.sh.

  site.py docs  --tree DIR --out DIR --channel NAME --commit SHA --version V
                --pin SHA --repo OWNER/NAME --md-html PATH [--strict]
  site.py index --out DIR
  site.py check --out DIR

docs: the documents docs/fragments/_documents.md lists, in the tree of one
channel's commit (DIR, as git archive wrote it), each converted by md_html.py
and put in doc.html.in at the same path under OUT/docs/<channel>/, .md
become .html; the list itself becomes that channel's index.html, and
docs/images/ and docs/screenshots/ are copied as they are. A link to any
other file of the repository goes to GitHub at that commit; one to a file
the commit does not have fails. --strict fails where a document under
docs/ is not in the list; without it, that is a warning.

index: OUT/docs/channels.json from the channels docs wrote, and
OUT/docs/index.html, which sends a reader to the most stable of them.

check: every relative or root-relative link and image of every page under
OUT resolves to a file written, and an anchor on a page to an id on it.

Exit 0 done, 1 a finding, 2 a usage error.
"""

import argparse
import html
import json
import os
import re
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
LIST = 'docs/fragments/_documents.md'
# Most stable first: /docs/ sends a reader to the first one built.
ORDER = ['stable', 'beta', 'alpha', 'nightly', 'devel', 'main']
GUIDELINES = 'https://gitlab.com/metaworx/open-source/mwx-guidelines/-/blob/%s/'
LINK = re.compile(r'\]\(([^)\s#]+)(?:#[^)\s]*)?\)')
FENCE = re.compile(r'^\s{0,3}(`{3,}|~{3,})')


def listed(text):
    """The repository paths the list links, in order, each once."""
    seen = []
    for path in LINK.findall(text):
        if '://' not in path and path not in seen:
            seen.append(path)
    return seen


def without_comments(text):
    """The markdown without its HTML comments outside code: the generated
    blocks' markers and banners, which md_html.py would show as text."""
    out, fence, comment = [], None, False
    for line in text.splitlines(keepends=True):
        if comment:
            comment = '-->' not in line
            continue
        m = FENCE.match(line)
        if m:
            mark = m.group(1)
            if fence is None:
                fence = mark[0]
            elif mark[0] == fence:
                fence = None
        elif fence is None and line.lstrip().startswith('<!--'):
            comment = '-->' not in line
            continue
        out.append(line)
    return ''.join(out)


def dead_links(text, source, tree):
    """The relative links of a document, outside code, whose file is not in
    the tree: on the site they would lead to a 404 at GitHub."""
    dead, fence = [], None
    here = os.path.dirname(source)
    for n, line in enumerate(text.splitlines(), 1):
        m = FENCE.match(line)
        if m:
            mark = m.group(1)[0]
            fence = mark if fence is None else (None if mark == fence else fence)
            continue
        if fence is not None:
            continue
        for path in LINK.findall(re.sub(r'`[^`]*`', '', line)):
            if re.match(r'^[a-z][a-z0-9+.-]*:', path):
                continue
            path = path.split('?', 1)[0]
            rel = path.lstrip('/') if path.startswith('/') else os.path.normpath(os.path.join(here, path))
            if rel.startswith('..') or not os.path.exists(os.path.join(tree, rel)):
                dead.append('%s:%d: %s' % (source, n, path))
    return dead


def title_of(text, fallback):
    for line in text.splitlines():
        if line.startswith('# '):
            return re.sub(r'[`*_]', '', line[2:]).strip()
    return fallback


def fill(template, values):
    for key, value in values.items():
        template = template.replace('{{%s}}' % key, value)
    return template


def convert(md_html, source, text, pages, pin, repo, commit):
    args = [sys.executable, md_html, '--anchors', 'github', '--source', source]
    for page in pages:
        args += ['--page', page]
    args += ['--url', 'GUIDELINES/shared/=' + GUIDELINES % pin,
             '--url', '=https://github.com/%s/blob/%s/' % (repo, commit)]
    run = subprocess.run(args, input=text, capture_output=True, text=True)
    if run.returncode != 0:
        sys.stderr.write(run.stderr)
        raise SystemExit(1)
    return run.stdout


def docs(a):
    try:
        with open(os.path.join(a.tree, LIST), encoding='utf-8') as f:
            lst = f.read()
    except FileNotFoundError:
        print('site.py: %s has no %s: no docs for the channel %s' % (a.commit[:12], LIST, a.channel))
        return 0
    paths = listed(lst)
    pages = [p for p in paths if p.endswith('.md')]
    missing = [p for p in paths if not os.path.isfile(os.path.join(a.tree, p))]
    if missing:
        print('site.py: %s names %s, not in %s' % (LIST, ', '.join(missing), a.commit[:12]), file=sys.stderr)
        return 1
    unlisted = []
    for top, dirs, files in os.walk(os.path.join(a.tree, 'docs')):
        dirs[:] = [d for d in dirs if d != 'fragments']
        for name in files:
            rel = os.path.relpath(os.path.join(top, name), a.tree).replace(os.sep, '/')
            if rel.endswith('.md') and rel not in pages:
                unlisted.append(rel)
    if unlisted:
        level = 'error' if a.strict else 'warning'
        print('site.py: %s: %s not in %s' % (level, ', '.join(sorted(unlisted)), LIST), file=sys.stderr)
        if a.strict:
            return 1

    with open(os.path.join(HERE, 'doc.html.in'), encoding='utf-8') as f:
        template = f.read()
    base = os.path.join(a.out, 'docs', a.channel)
    ahead = ''
    if a.channel == 'main':
        ahead = ' What is merged into main and not yet released, so ahead of every channel.'

    def write(rel_html, source_md, title, body):
        depth = rel_html.count('/')
        values = {
            'TITLE': html.escape(title),
            'CHANNEL': html.escape(a.channel),
            'PAGE': html.escape(rel_html),
            'INDEX': '../' * depth + 'index.html',
            'VERSION': html.escape(a.version),
            'SOURCE': html.escape('https://github.com/%s/blob/%s/%s' % (a.repo, a.commit, source_md)),
            'SOURCE_NAME': html.escape(source_md),
            'AHEAD': ahead,
        }
        page = fill(template, values).replace('{{BODY}}', body)
        target = os.path.join(base, rel_html)
        os.makedirs(os.path.dirname(target), exist_ok=True)
        with open(target, 'w', encoding='utf-8') as f:
            f.write(page)

    dead = []
    for p in pages:
        with open(os.path.join(a.tree, p), encoding='utf-8') as f:
            dead += dead_links(f.read(), p, a.tree)
    if dead:
        for d in dead:
            print('site.py: %s: no such file in %s' % (d, a.commit[:12]), file=sys.stderr)
        return 1

    for p in pages:
        with open(os.path.join(a.tree, p), encoding='utf-8') as f:
            text = without_comments(f.read())
        body = convert(a.md_html, p, text, pages, a.pin, a.repo, a.commit)
        write(p[:-3] + '.html', p, title_of(text, p), body)

    # The index: the list without its banner, from the root, where its
    # links start.
    text = without_comments('\n'.join(l for l in lst.splitlines() if not l.startswith('> **Fragment**')) + '\n')
    body = convert(a.md_html, 'index.md', text, pages, a.pin, a.repo, a.commit)
    write('index.html', LIST, 'Documentation (%s)' % a.channel, body)

    for d in ('docs/images', 'docs/screenshots'):
        if os.path.isdir(os.path.join(a.tree, d)):
            shutil.copytree(os.path.join(a.tree, d), os.path.join(base, d), dirs_exist_ok=True)

    with open(os.path.join(base, 'channel.json'), 'w', encoding='utf-8') as f:
        json.dump({'channel': a.channel, 'version': a.version, 'commit': a.commit, 'date': a.date}, f)
    print('site.py: docs/%s: %d pages of %s (%s)' % (a.channel, len(pages) + 1, a.version, a.commit[:12]))
    return 0


def index(a):
    root = os.path.join(a.out, 'docs')
    built = []
    for ch in ORDER:
        try:
            with open(os.path.join(root, ch, 'channel.json'), encoding='utf-8') as f:
                built.append(json.load(f))
        except FileNotFoundError:
            pass
    if not built:
        print('site.py: no channel has docs')
        return 0
    with open(os.path.join(root, 'channels.json'), 'w', encoding='utf-8') as f:
        json.dump(built, f, indent=1)
        f.write('\n')
    first = built[0]['channel']
    target = html.escape('%s/index.html' % first)
    with open(os.path.join(root, 'index.html'), 'w', encoding='utf-8') as f:
        f.write('<!DOCTYPE html>\n<html lang="en">\n<head>\n<meta charset="utf-8">\n'
                '<meta name="viewport" content="width=device-width, initial-scale=1">\n'
                '<meta http-equiv="refresh" content="0; url=%s">\n'
                '<title>Massimo documentation</title>\n'
                '<link rel="stylesheet" href="/site.css">\n</head>\n<body>\n'
                '<p>The documentation of the channel %s: <a href="%s">%s</a>.</p>\n'
                '</body>\n</html>\n' % (target, html.escape(first), target, target))
    print('site.py: docs/ sends a reader to %s; channels %s'
          % (first, ', '.join(c['channel'] for c in built)))
    return 0


ATTR = re.compile(r'\s(?:href|src)="([^"]*)"')
IDS = re.compile(r'\sid="([^"]*)"')


def check(a):
    out = os.path.abspath(a.out)
    ids = {}

    def ids_of(path):
        if path not in ids:
            with open(path, encoding='utf-8') as f:
                ids[path] = set(html.unescape(i) for i in IDS.findall(f.read()))
        return ids[path]

    broken = []
    for top, _, files in os.walk(out):
        for name in files:
            if not name.endswith('.html'):
                continue
            page = os.path.join(top, name)
            with open(page, encoding='utf-8') as f:
                text = f.read()
            for raw in ATTR.findall(text):
                link = html.unescape(raw)
                if re.match(r'^[a-z][a-z0-9+.-]*:', link) or link.startswith('//'):
                    continue
                path, _, frag = link.partition('#')
                path = path.split('?', 1)[0]
                if path.startswith('/'):
                    target = os.path.join(out, path.lstrip('/'))
                elif path:
                    target = os.path.normpath(os.path.join(top, path))
                else:
                    target = page
                if os.path.isdir(target):
                    target = os.path.join(target, 'index.html')
                rel = os.path.relpath(page, out)
                if not os.path.isfile(target):
                    broken.append('%s: %s: no such file' % (rel, link))
                elif frag and target.endswith('.html') and frag not in ids_of(target):
                    broken.append('%s: %s: no such id' % (rel, link))
    for b in broken:
        print('site.py: %s' % b, file=sys.stderr)
    if broken:
        print('site.py: %d broken link(s)' % len(broken), file=sys.stderr)
        return 1
    print('site.py: every link of the pages under %s resolves' % a.out)
    return 0


def main():
    p = argparse.ArgumentParser(prog='site.py', description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest='cmd', required=True)
    d = sub.add_parser('docs')
    for opt in ('tree', 'out', 'channel', 'commit', 'version', 'date', 'pin', 'repo', 'md_html'):
        d.add_argument('--' + opt.replace('_', '-'), required=opt != 'date', default='')
    d.add_argument('--strict', action='store_true')
    for name in ('index', 'check'):
        sub.add_parser(name).add_argument('--out', required=True)
    a = p.parse_args()
    return {'docs': docs, 'index': index, 'check': check}[a.cmd](a)


if __name__ == '__main__':
    sys.exit(main())
