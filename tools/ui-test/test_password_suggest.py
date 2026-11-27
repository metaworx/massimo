# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""A suggested password (AP PasswordRules, Block 4, D14-A to D19-A;
ui/src/shared/suggest.ts, ui/src/shared/password.tsx): a dice beside the
eye in the first new password of each of the page's five forms, the first
owner's, My profile's Set a password and Change password, and in System
settings → Users the adding of a user and a user's panel. A press fills
the form, both fields where it has two, with five words of EFF's long list
joined by "-" or twenty characters of an alphabet of 56, which the rule
takes and the form saves, and shows it as text, eye or none, until it is
edited. The presses give five words and twenty characters by turns, five
words first and again at each new start, a key leaving the turn, and the
dice's label says what the next gives (AP AccessEnds, D44-A, D50-A,
D51-A). While the field holds the suggestion, a line right under the
field shows it whole, not translated, on two lines but in the rarest
draws (AP PasswordRules, D58-A): a text that copies itself on a click or
Enter, saying "Copied", with its copy icon after it, both named Copy the
password, its words hidden from a screen reader (D52-A). The line keeps a
place of two lines from the first press on, empty after a key; there is
no line before, and no row under the field at any time (D49-A). The
rule's line says that a password was suggested, and no status region,
label, console line or node of the accessibility tree but the field's
holds it.
Picking is without bias, numbers at or above the limit drawn again,
which the test shows by handing the page's crypto.getRandomValues chosen
numbers; a suggestion the rule refuses, a word of the list that is the
user's name, is drawn again, at most 20 times, and the line says so
where none is found. The list is fetched once, at the first press that
needs it; a fetch that fails says so in the line, and the next press
gives twenty characters, which need none; a suggestion that comes after
the card has opened anew fills nothing."""
import json
import os
import re

import pytest

from conftest import TOKEN, URL, api_call
from helpers import BUBBLE_STATUS, OPERATOR_PASSWORD, bubble, frames, named_buttons, open_fold, scroll_width, show_view, toggle_fold, wait_bubble_at, wait_held, wait_until
from test_first_owner import OPEN, owner_card, start_page, stub_creation
from test_login import sign_in
from test_password_eye import STATE, eye_of, fresh_page, reopen, say_reveal
from test_password_rule import REAL_REGISTER, SAME_LINE, STUB_REGISTER, WATCH_LINE, line, words
from test_users import remove_user

LIST = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "internal", "server", "ui", "words-en.txt")
I18N = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "internal", "server", "ui", "i18n")
with open(LIST, encoding="utf-8") as _f:
    WORD_LIST = _f.read().split("\n")[:-1]
ALPHABET = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"
WORD_LIMIT = (2 ** 32 // 7776) * 7776
SUGGESTED = "password.suggested"

# crypto.getRandomValues of the page replaced by one that fills each array
# it is given from values, one number an element, and then, where always is
# a number, with it, else with the browser's own; window.__draws counts the
# calls. REAL_RANDOM puts the browser's back.
STUB_RANDOM = """([values, always]) => {
  if (!window.__realRandom) window.__realRandom = crypto.getRandomValues.bind(crypto);
  const queue = values.slice();
  window.__draws = 0;
  crypto.getRandomValues = (a) => {
    window.__draws++;
    if (!queue.length && always === null) return window.__realRandom(a);
    for (let i = 0; i < a.length; i++) a[i] = queue.length ? queue.shift() : always;
    return a;
  };
}"""
REAL_RANDOM = "() => { if (window.__realRandom) { crypto.getRandomValues = window.__realRandom; delete window.__realRandom; } }"

# Every text of the page a screen reader is told or reads as a name: the
# status and live regions' texts, but what is aria-hidden in them, which
# a screen reader is not told (the words of the line that shows a
# suggestion whole, which shown_line holds aria-hidden inside its copy),
# every aria-label and title; and
# the console's lines, which the test gathers (watch_console). An
# aria-hidden text that a description or a label refers to is read out
# all the same: not_spoken reads the browser's accessibility tree too
# (ax_holders).
SPOKEN = """() => {
  const out = [];
  const told = (el) => {
    const copy = el.cloneNode(true);
    for (const hidden of copy.querySelectorAll('[aria-hidden="true"]')) hidden.remove();
    return copy.textContent;
  };
  for (const el of document.querySelectorAll('[role=status], [aria-live], [role=alert]')) out.push(told(el));
  for (const el of document.querySelectorAll('[aria-label], [title]')) out.push(el.getAttribute('aria-label') || '', el.getAttribute('title') || '');
  return out;
}"""


def five_words(value):
    """The five words of the list value is made of, joined by "-", or None:
    each word drawn may hold a "-" of its own (drop-down)."""
    parts = value.split("-")
    known = set(WORD_LIST)

    def split(i, n):
        if i == len(parts):
            return [] if n == 0 else None
        for j in range(i + 1, len(parts) + 1):
            w = "-".join(parts[i:j])
            if w in known:
                rest = split(j, n - 1)
                if rest is not None:
                    return [w] + rest
        return None
    return split(0, 5)


def is_chars(value):
    return len(value) == 20 and all(c in ALPHABET for c in value)


def copy_of(page, field):
    """The shown line's suggestion, a text that copies itself."""
    return page.locator(f"#{field}-shown .pw-copy")


def icon_of(page, field):
    """The copy icon after the shown line's suggestion."""
    return page.locator(f"#{field}-shown .copy-icon")


def dice_of(page, field):
    return page.locator(f".pw-box:has(> #{field}) > .pw-dice")


def value(page, field):
    return page.locator("#" + field).input_value()


NEXT = {"words": "password.suggestWords", "chars": "password.suggestChars"}


def says_next(page, field, kind, what):
    """The dice labelled, aria-label and title, by what its next press
    gives: five words or twenty characters."""
    dice = dice_of(page, field)
    want = words(page, NEXT[kind])
    got = [dice.get_attribute("aria-label"), dice.get_attribute("title")]
    assert got == [want, want], f"{field}: {what}: the dice says {kind} next: {got}"


# What the field's row holds besides its label and the field's box: the
# id or the first class of each element.
ROW = """(field) => [...document.getElementById(field).closest('.field').children]
  .filter((c) => c.tagName !== 'LABEL' && !c.classList.contains('pw-box')).map((c) => c.id || c.classList[0] || c.tagName)"""


def no_row(page, field):
    """Nothing under the field but the shown line, where there is one, the
    rule's line and the hint: no row of a choice of kind, no Copy of its
    own, no radio button (AP AccessEnds, D49-A)."""
    row = page.evaluate(ROW, field)
    assert all(c in (f"{field}-shown", "pw-rule", "muted") for c in row), f"{field}: no row under the field: {row}"
    assert page.locator(f".field:has(> .pw-box > #{field}) input[type=radio]").count() == 0, f"{field}: no choice of kind"


def watch_console(page):
    lines = []
    page.on("console", lambda m: lines.append(m.text))
    return lines


def ax_holders(page, password):
    """The nodes of Chromium's accessibility tree, as a screen reader is
    given it (CDP's Accessibility.getFullAXTree, the nodes it ignores left
    out), whose name, description or value holds the password: none but a
    text field's value and its own text under it."""
    cdp = page.context.new_cdp_session(page)
    try:
        nodes = cdp.send("Accessibility.getFullAXTree")["nodes"]
    finally:
        cdp.detach()
    by_id = {n["nodeId"]: n for n in nodes}

    def role(n):
        return (n.get("role") or {}).get("value")

    def in_field(n):
        n = by_id.get(n.get("parentId"))
        while n:
            if role(n) == "textbox":
                return True
            n = by_id.get(n.get("parentId"))
        return False
    held = []
    for n in nodes:
        if n.get("ignored"):
            continue
        for key in ("name", "description", "value"):
            text = str((n.get(key) or {}).get("value", ""))
            if password in text and not (role(n) == "textbox" and key == "value") and not in_field(n):
                held.append(f"{role(n)} {key}: {text[:120]}")
    return held


def not_spoken(page, console, password, what):
    """No status region, label, title or console line holds the password,
    nor any node of the accessibility tree but the field's (ax_holders)."""
    told = [t for t in page.evaluate(SPOKEN) + console if password in t]
    assert not told, f"{what}: the password is not told: {told}"
    held = ax_holders(page, password)
    assert not held, f"{what}: the password in the accessibility tree outside its field: {held}"


# The line under a new password's field that shows its suggestion whole,
# null where there is none: its text, its attributes, and of the
# suggestion it holds, a text that copies itself (.pw-copy) with its copy
# icon after it: their names, roles and focus order, and whether the
# words are under aria-hidden inside the copy; the words' font's size,
# their lines (a Range's boxes over their text, the icon left out) and
# the character before each break, the line's height, its width and its
# row's, whether its content fits its width, the icon's place (after the
# words, within the row, and within the line's height); and the line's
# place: next after the field's box and before the rule's line, under the
# box, inside the field's row. most is the lines the widest suggestion of
# the value's kind takes there, laid out in a copy of the line out of the
# flow, icon and all: the list's widest word five times, or the
# alphabet's widest character twenty times (given as worst). room is the
# width the words have beside the icon.
SHOWN_LINE = """([field, worst]) => {
  const line = document.getElementById(field + '-shown');
  if (!line) return null;
  const box = document.getElementById(field).closest('.pw-box');
  const row = box.closest('.field');
  const l = line.getBoundingClientRect(), b = box.getBoundingClientRect(), f = row.getBoundingClientRect();
  const next = line.nextElementSibling;
  const base = {text: line.textContent, hidden: line.getAttribute('aria-hidden'), translate: line.getAttribute('translate'),
                height: l.height, lineHeight: parseFloat(getComputedStyle(line).lineHeight),
                after: box.nextElementSibling === line, before: !!next && next.classList.contains('pw-rule'), under: l.top >= b.bottom - 0.5,
                inside: l.left >= f.left - 0.5 && l.right <= f.right + 0.5, width: l.width, rowWidth: f.width};
  const copy = line.querySelector('.pw-copy');
  if (!copy) return Object.assign(base, {copy: null});
  const icon = line.querySelector('.copy-icon');
  const textOf = (el) => document.createTreeWalker(el, NodeFilter.SHOW_TEXT).nextNode();
  const countLines = (el) => {
    const r = document.createRange();
    r.selectNodeContents(el);
    return new Set([...r.getClientRects()].map((x) => Math.round(x.top))).size;
  };
  const text = textOf(copy);
  const breaks = [];
  for (let i = 1, last = null; text && i <= text.length; i++) {
    const one = document.createRange();
    one.setStart(text, i - 1);
    one.setEnd(text, i);
    const top = Math.round(one.getBoundingClientRect().top);
    if (last !== null && top !== last) breaks.push(text.data[i - 2]);
    last = top;
  }
  const hiddenWords = text && text.parentElement.closest('[aria-hidden="true"]');
  const c = copy.getBoundingClientRect(), i = icon ? icon.getBoundingClientRect() : null;
  const clone = line.cloneNode(true);
  clone.removeAttribute('id');
  for (const el of clone.querySelectorAll('[id]')) el.removeAttribute('id');
  clone.style.cssText = `position: absolute; visibility: hidden; left: 0; top: 0; width: ${l.width}px; margin: 0`;
  document.body.append(clone);
  textOf(clone.querySelector('.pw-copy')).data = worst;
  const most = countLines(clone.querySelector('.pw-copy'));
  clone.remove();
  const tabbable = [...line.querySelectorAll('[tabindex], button')].filter((el) => el.tabIndex >= 0);
  return Object.assign(base, {
    copy: {role: copy.getAttribute('role'), tabindex: copy.getAttribute('tabindex'), name: copy.getAttribute('aria-label'), title: copy.getAttribute('title'),
           copyable: copy.classList.contains('copyable'), words: text ? text.data : null,
           wordsHidden: !!hiddenWords && copy.contains(hiddenWords) && hiddenWords !== copy, spanHidden: copy.getAttribute('aria-hidden')},
    icon: icon ? {name: icon.getAttribute('aria-label'), title: icon.getAttribute('title'), tag: icon.tagName,
                  after: i.left >= c.right - 0.5, within: i.right <= f.right + 0.5 && i.top >= l.top - 0.5 && i.bottom <= l.bottom + 0.5} : null,
    order: tabbable.map((el) => (el === copy ? 'copy' : el === icon ? 'icon' : el.tagName)),
    lines: text ? countLines(copy) : 0, breaks, most, room: c.width,
    fontSize: getComputedStyle(copy).fontSize, fits: line.scrollWidth <= line.clientWidth});
}"""


# The list's widest word and the alphabet's widest character in the
# body's font, which the line that shows a suggestion draws in.
WIDEST = """([list, alphabet]) => {
  const c = document.createElement('canvas').getContext('2d');
  c.font = getComputedStyle(document.body).font;
  const widest = (xs) => xs.reduce((a, x) => (c.measureText(x).width > c.measureText(a).width ? x : a));
  return [widest(list), widest([...alphabet])];
}"""


def worst(page, value):
    """The widest suggestion of value's kind: the list's widest word five
    times, or the alphabet's widest character twenty times."""
    word, char = page.evaluate(WIDEST, [WORD_LIST, ALPHABET])
    return char * 20 if is_chars(value) else "-".join([word] * 5)


def shown_state(page, field):
    """SHOWN_LINE's look at the shown line under field, None where there is none."""
    return page.evaluate(SHOWN_LINE, [field, ""])


def shown_line(page, field, value):
    """The line under field shows value, the field's, whole, as a text
    that copies itself with its copy icon after it (AP AccessEnds, D44-A):
    the line not aria-hidden, which a control inside it cannot be, and
    not translated; the copy a button to the keyboard, it and its icon
    both named Copy the password, the words under aria-hidden inside the
    copy, so that no screen reader is told them (D52-A), the copy before
    the icon in the focus order; the line right under the field's box and
    before the rule's line, as wide as the field's row, inside it, the
    words in the body's font at 16 px (D21-A), within the line's width,
    broken after a "-" alone, on no more lines than the widest suggestion
    of its kind takes there beside the icon (most), at most three, the
    icon after the words, within the row and the line's height; its place
    two lines tall or as tall as the words' lines."""
    s = page.evaluate(SHOWN_LINE, [field, worst(page, value)])
    assert s is not None and s["copy"] is not None, f"{field}: the shown line, with the suggestion's copy: {s}"
    c, i = s["copy"], s["icon"]
    name = words(page, "password.copyName")
    assert s["text"] == value and c["words"] == value, f"{field}: the shown line holds the field's value whole: {s['text']!r}, {value!r}"
    assert s["hidden"] is None and s["translate"] == "no", f"{field}: the line not aria-hidden, not translated: {s}"
    assert c["role"] == "button" and c["tabindex"] == "0" and c["copyable"] and c["name"] == name and c["title"] == name, \
        f"{field}: the suggestion copies itself, named {name!r}: {c}"
    assert c["wordsHidden"] and c["spanHidden"] is None, f"{field}: its words hidden from a screen reader inside the copy: {c}"
    assert i is not None and i["tag"] == "BUTTON" and i["name"] == name and i["title"] == name, f"{field}: its copy icon, named alike: {i}"
    assert s["order"] == ["copy", "icon"], f"{field}: the copy, then its icon, to the keyboard: {s['order']}"
    assert s["after"] and s["before"] and s["under"] and s["inside"], f"{field}: right under the field, before the rule's line: {s}"
    assert abs(s["width"] - s["rowWidth"]) <= 0.5 and s["fontSize"] == "16px", f"{field}: the row's width, 16 px: {s}"
    assert 1 <= s["lines"] <= s["most"] <= 3 and s["fits"], f"{field}: within its width, on no more lines than the widest takes: {s}"
    assert all(ch == "-" for ch in s["breaks"]), f"{field}: broken after a \"-\" alone, never inside a word: {s}"
    assert i["after"] and i["within"], f"{field}: the icon after the words, within the row and the line: {s}"
    assert abs(s["height"] - max(2, s["lines"]) * s["lineHeight"]) <= 0.5, f"{field}: its place two lines tall, or as its lines: {s}"
    return s


def press(page, field, before=None):
    """The dice pressed: the field holds a new value, which is returned."""
    old = value(page, field) if before is None else before
    dice_of(page, field).click()
    assert wait_until(page, f"document.getElementById({field!r}).value !== {old!r} && document.getElementById({field!r}).value !== ''", 5), \
        f"{field}: the dice fills it"
    # the rule's line written anew: emptied, then its words (PasswordRule's fresh)
    assert wait_until(page, f"document.getElementById({field + '-rule'!r}).textContent !== ''", 3), f"{field}: the rule's line written again"
    frames(page)
    return value(page, field)


def check_suggest(page, case, field, again, console):
    """The form's first new password, field, and its second, again where it
    has one: no shown line before the first press, and no row under the
    field at any time (no_row); the dice before the eye inside the box,
    labelled Suggest five words; a press fills both with five words of the
    list, shown as text and whole in the shown line under the first field
    alone, a copy with its icon (shown_line), the rule's line saying it was
    suggested and nothing telling the password (not_spoken), the dice
    labelled Suggest twenty characters; the next press gives twenty of the
    alphabet, the rule's line written anew, the dice labelled for five
    words again; the next five words, another, each in the shown line.
    Returns the last value."""
    assert page.locator(f"#{field}-shown").count() == 0, f"{field}: no line before the first press"
    no_row(page, field)
    dice = dice_of(page, field)
    dice.wait_for(state="visible")
    says_next(page, field, "words", "before the first press")
    eye = eye_of(page, field)
    if eye.count():
        assert dice.bounding_box()["x"] + dice.bounding_box()["width"] <= eye.bounding_box()["x"] + 0.5, f"{field}: the dice before the eye"
    if again:
        assert dice_of(page, again).count() == 0, f"{again}: no dice on the second field"
    v = press(page, field)
    assert five_words(v), f"{field}: five words of the list: {v!r}"
    if again:
        assert value(page, again) == v, f"{again}: filled alike"
        assert page.locator(f"#{again}-shown").count() == 0, f"{again}: no line under the second field"
    s = page.evaluate(STATE, field)
    assert s["type"] == "text", f"{field}: the suggestion shown: {s}"
    assert line(page, field)["text"] == words(page, SUGGESTED), f"{field}: the rule's line says it was suggested: {line(page, field)}"
    assert page.evaluate("(p) => window.MassimoParts.passwordProblem(p, [])", v) == "", f"{field}: the rule takes it"
    assert page.evaluate("document.activeElement && document.activeElement.classList.contains('pw-dice')"), f"{field}: the focus on the dice"
    says_next(page, field, "chars", "after five words")
    no_row(page, field)
    shown_line(page, field, v)
    not_spoken(page, console, v, field)
    page.evaluate(WATCH_LINE, field)
    c = press(page, field, v)
    assert is_chars(c), f"{field}: the next press, twenty of the alphabet: {c!r}"
    if again:
        assert value(page, again) == c, f"{again}: filled alike"
    shown_line(page, field, c)
    assert wait_until(page, f"window.__lineWrites > 0 && document.getElementById({field + '-rule'!r}).textContent === t('password.suggested')", 3), \
        f"{field}: the rule's line written anew at the second press, for a screen reader: {page.evaluate('window.__lineWrites')}"
    assert page.evaluate(SAME_LINE, field), f"{field}: the same line"
    page.evaluate("() => { window.__lineWatch.disconnect(); }")
    says_next(page, field, "words", "after twenty characters")
    no_row(page, field)
    not_spoken(page, console, c, field)
    named_buttons(page, f"{field} with its dice")
    w = press(page, field, c)
    assert five_words(w) and w != v, f"{field}: the next, five words again, another: {w!r}"
    says_next(page, field, "chars", "after five words again")
    shown_line(page, field, w)
    return w


def edited(page, field):
    """A key typed into the field: a password's field again, the eye as it
    was (hidden), the rule's line saying what the rule finds, no copy in
    the shown line, which is empty, its place kept, and the dice still
    labelled for what its next press gives (a key leaves the turn, AP
    AccessEnds, D50-A)."""
    label = dice_of(page, field).get_attribute("aria-label")
    page.locator("#" + field).press("End")
    page.locator("#" + field).press_sequentially("x")
    frames(page)
    assert page.evaluate(STATE, field)["type"] == "password", f"{field}: edited, a password's field again"
    assert line(page, field)["text"] == words(page, "password.fine"), f"{field}: the rule's line"
    assert copy_of(page, field).count() == 0 and icon_of(page, field).count() == 0, f"{field}: edited, no copy in the shown line"
    shown = shown_state(page, field)
    assert shown is not None and shown["text"] == "", f"{field}: edited, no password in the shown line, its place kept: {shown}"
    assert dice_of(page, field).get_attribute("aria-label") == label, f"{field}: a key leaves the turn: {label!r}"


def reset_ends(page, field, reset):
    """A suggestion, then the form's Reset: the field emptied is a
    password's field again, its eye not pressed, the shown line and its
    copy gone, and the next press gives five words again, a new start (AP
    AccessEnds, D50-A)."""
    press(page, field)
    if dice_of(page, field).get_attribute("aria-label") != words(page, NEXT["chars"]):
        press(page, field)  # so that, but for the Reset, the next press would give twenty characters
    says_next(page, field, "chars", "before Reset")
    assert page.evaluate(STATE, field)["type"] == "text"
    page.locator(reset).click()
    frames(page)
    s = page.evaluate(STATE, field)
    assert s["value"] == "" and s["type"] == "password" and s.get("pressed", "false") == "false", f"{field}: Reset ends the suggestion: {s}"
    assert copy_of(page, field).count() == 0 and page.locator(f"#{field}-shown").count() == 0, f"{field}: Reset: no line, no copy"
    no_row(page, field)
    says_next(page, field, "words", "after Reset")


def test_the_first_owners_form(new_context, case, watch, guard):
    """The first owner's form, its fields not controlled: the dice beside
    Password fills both, five words and twenty characters by turns, the
    shown line coming with the first press; a submit the form refuses,
    Again changed, keeps the suggestion in
    the field, a password's field again, and empties the shown line, its
    place kept; a suggestion creates the owner."""
    page = start_page(new_context, guard, watch, OPEN)
    console = watch_console(page)
    owner_card(page)
    page.locator("#setup-user").fill("owner")
    check_suggest(page, case, "setup-password", "setup-again", console)
    kept = press(page, "setup-password")
    page.locator("#setup-again").fill(kept + "x")
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "document.getElementById('setup-msg').textContent === t('setup.mismatch')", 3), "the passwords differ"
    frames(page)
    assert value(page, "setup-password") == kept and page.evaluate(STATE, "setup-password")["type"] == "password", \
        "refused: the suggestion kept in the field, a password's field again"
    shown = shown_state(page, "setup-password")
    assert shown is not None and shown["text"] == "", f"refused: the shown line empty, its place kept: {shown}"
    v = press(page, "setup-password")
    made = []
    stub_creation(page, made)
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-dialog').open", 10), "the suggestion creates the owner"
    assert len(made) == 1, f"created once: {made}"
    not_spoken(page, console, v, "after the creation")


def test_the_shown_line_ends_as_the_card_closes(new_context, case, watch, guard):
    """A suggestion in the first owner's card, then Skip for now: the card
    closed keeps what its fields hold (Owner.tsx), but its shown line is
    emptied and the password is in no text of the page; the card opened
    again, from the open server's banner, shows no line until the next
    press. Fails on the page before: the closed card's line held the
    suggestion until the card's next opening."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    v = press(page, "setup-password")
    page.locator("#setup-skip").click()
    assert wait_until(page, "!document.getElementById('login-dialog').open", 5), "Skip for now closes the card"
    frames(page)
    assert value(page, "setup-password") == v, "the card closed keeps what its fields hold"
    shown = shown_state(page, "setup-password")
    assert shown is None or shown["text"] == "", f"the card closed: the shown line empty: {shown}"
    assert not page.evaluate("(v) => document.documentElement.textContent.includes(v)", v), "the password in no text of the page"
    page.wait_for_selector(".room", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the page started"
    page.locator("#open-banner-create").click()
    owner_card(page)
    assert page.locator("#setup-password-shown").count() == 0, "opened again: no shown line before a press"


def test_my_profiles_set_a_password(page, case):
    """My profile of a user with keys only (the test's owner): Set a
    password, the fields controlled; the suggestion is saved, registered as
    suggested, and hidden at the submit, the shown line gone with the save,
    which is a new start: the next press gives five words again (AP
    AccessEnds, D50-A; the PUT stubbed, so that the user stays as it
    was)."""
    console = watch_console(page)
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").wait_for(state="visible")
    check_suggest(page, case, "me-new", "me-again", console)
    edited(page, "me-new")
    v = press(page, "me-new")
    if dice_of(page, "me-new").get_attribute("aria-label") != words(page, NEXT["chars"]):
        v = press(page, "me-new", v)  # so that, but for the save, the next press would give twenty characters
    says_next(page, "me-new", "chars", "before the save")
    put = []

    def user(route):
        if route.request.method != "PUT":
            return route.continue_()
        put.append(json.loads(route.request.post_data or "{}"))
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route("**/api/v1/users/owner", user)
    page.evaluate(STUB_REGISTER)
    try:
        page.locator("#me-password-form button[type=submit]").click()
        assert wait_until(page, "document.getElementById('me-password-msg').classList.contains('ok')", 10), "the suggestion is saved"
        assert page.evaluate("window.__registered") == [v] and len(put) == 1, f"registered as suggested: {put}"
        assert page.evaluate(STATE, "me-new")["type"] == "password", "hidden at the submit"
        assert page.locator("#me-new-shown").count() == 0, "saved: no shown line"
        says_next(page, "me-new", "words", "after the save, a new start")
    finally:
        page.evaluate(REAL_REGISTER)
        page.unroute("**/api/v1/users/owner")
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def test_my_profiles_change_password(new_context, case, watch, guard):
    """My profile of the operator: Change password, the current one apart;
    the dice fills the new one twice and leaves the current one alone."""
    page = fresh_page(new_context, guard, watch)
    console = watch_console(page)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-current").fill(OPERATOR_PASSWORD)
    assert dice_of(page, "me-current").count() == 0, "no dice on the current password"
    check_suggest(page, case, "me-new", "me-again", console)
    assert value(page, "me-current") == OPERATOR_PASSWORD, "the current one left as it was"
    reset_ends(page, "me-new", "#me-password-form .form-reset")


def test_the_users_forms(page, case):
    """System settings → Users: adding a user, whose suggestion creates it
    with that password, and a user's panel, its one field filled; the
    panel closed and opened is a new start, the next press five words."""
    console = watch_console(page)
    name = f"suggest-{case.width}-{case.lang}"
    remove_user(name)
    show_view(page, "system")
    try:
        page.locator("#user-add").click()
        page.locator("#user-new-password").wait_for(state="visible")
        page.locator("#user-new input[name=name]").fill(name)
        check_suggest(page, case, "user-new-password", None, console)
        with page.expect_response(lambda r: r.request.method == "POST" and r.url.endswith("/api/v1/users")):
            page.locator("#user-new button[type=submit]").click()
        assert wait_until(page, "!document.querySelector('#user-new form')", 10), "the suggestion creates the user"
        assert api_call("GET", f"api/v1/users/{name}")["auth_version"] == 2, "with the password, registered by OPAQUE"
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        field = f"user-{name}-password"
        page.locator("#" + field).wait_for(state="visible")
        check_suggest(page, case, field, None, console)
        reset_ends(page, field, ".user-panel[data-name='%s'] form .form-reset" % name)
        press(page, field)
        says_next(page, field, "chars", "a press after Reset")
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{name}']\").open", 3)
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        page.locator("#" + field).wait_for(state="visible")
        assert page.locator(f"#{field}-shown").count() == 0, "the panel closed and opened: the shown line gone with the new start"
        assert page.evaluate(STATE, field)["type"] == "password", "and the suggestion hidden"
        says_next(page, field, "words", "the panel closed and opened, a new start")
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_shown_with_the_eye_off(new_context, case, watch, guard):
    """With auth.password_reveal off there is no eye, and the suggestion
    still shows, until it is edited; the press after the key gives the
    kind the dice named before it, twenty characters after five words (a
    key leaves the turn, AP AccessEnds, D50-A). With the eye on, a
    password shown by it stays shown after an edit of a suggestion, and
    the eye pressed on a suggestion hides it."""
    page = start_page(new_context, guard, watch, OPEN)
    say_reveal(page, False)
    page.reload()
    owner_card(page)
    assert wait_until(page, "state.version !== null", 5), "/healthz answered, its eye taken (checkVersion)"
    frames(page)
    assert page.locator(".pw-eye").count() == 0, "no eye"
    shown = press(page, "setup-password")
    assert page.evaluate(STATE, "setup-password")["type"] == "text", "the suggestion shown, without the eye"
    shown_line(page, "setup-password", shown)
    edited(page, "setup-password")
    after = press(page, "setup-password")
    assert is_chars(after), f"the press after a key gives the kind the dice named, twenty characters: {after!r}"
    on = start_page(new_context, guard, watch, OPEN)
    owner_card(on)
    eye_of(on, "setup-password").click()
    press(on, "setup-password")
    edited_on = on.evaluate(STATE, "setup-password")
    assert edited_on["type"] == "text" and edited_on["pressed"] == "true", f"shown by the eye and suggested: {edited_on}"
    on.locator("#setup-password").press_sequentially("x")
    frames(on)
    s = on.evaluate(STATE, "setup-password")
    assert s["type"] == "text" and s["pressed"] == "true", f"edited: as the eye had it, shown: {s}"
    eye_of(on, "setup-password").click()
    press(on, "setup-password")
    eye_of(on, "setup-password").click()
    s = on.evaluate(STATE, "setup-password")
    assert s["type"] == "password" and s["pressed"] == "false", f"the eye pressed on a suggestion hides it: {s}"
    frames(on)
    assert shown_state(on, "setup-password")["text"] == "", "and the shown line with it"


FOCUSED = "(sel) => document.activeElement === document.querySelector(sel)"


def test_the_dice_alternates_and_the_keyboard_reaches_it(page, case):
    """The keyboard: Tab from the field reaches the dice, Enter and Space
    press it, five words then twenty characters, the focus staying there;
    Tab from the eye then reaches the shown line's copy and its icon,
    after it the next field. After a load the first press gives five
    words, whatever a browser kept under massimo.suggest, where the page
    kept the kind chosen before (AP AccessEnds, D44-A). Fails on the page
    before: Space gave five words again, and a kept "chars" gave
    characters after the load."""
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").focus()
    page.keyboard.press("Tab")
    assert page.evaluate(FOCUSED, ".pw-box:has(> #me-new) > .pw-dice"), "Tab reaches the dice"
    page.keyboard.press("Enter")
    assert wait_until(page, "document.getElementById('me-new').value !== ''", 5), "Enter presses it"
    first = value(page, "me-new")
    assert five_words(first), f"Enter: five words: {first!r}"
    page.keyboard.press("Space")
    assert wait_until(page, f"document.getElementById('me-new').value !== {first!r}", 5), "and Space"
    frames(page)
    assert is_chars(value(page, "me-new")), f"Space: twenty characters: {value(page, 'me-new')!r}"
    assert page.evaluate(FOCUSED, ".pw-box:has(> #me-new) > .pw-dice"), "the focus stays on the dice"
    if eye_of(page, "me-new").count():
        page.keyboard.press("Tab")
        assert page.evaluate(FOCUSED, ".pw-box:has(> #me-new) > .pw-eye"), "Tab reaches the eye"
    page.keyboard.press("Tab")
    assert page.evaluate(FOCUSED, "#me-new-shown .pw-copy"), "Tab from the eye reaches the shown line's copy"
    page.keyboard.press("Tab")
    assert page.evaluate(FOCUSED, "#me-new-shown .copy-icon"), "then its icon"
    page.keyboard.press("Tab")
    assert page.evaluate("document.activeElement === document.getElementById('me-again')"), "then the next field"
    page.evaluate("localStorage.setItem('massimo.suggest', 'chars')")
    try:
        page.reload()
        assert wait_until(page, "!!window.MassimoParts && state.eventsOpen", 10), "loaded again"
        show_view(page, "profile")
        open_fold(page, "#me-password")
        says_next(page, "me-new", "words", "after a load")
        v = press(page, "me-new", "")
        assert five_words(v), f"after a load, five words, whatever the browser kept: {v!r}"
    finally:
        page.evaluate("localStorage.removeItem('massimo.suggest')")
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def lang_init(lang):
    """A script that makes lang the browser's language, before the page's own."""
    return f"Object.defineProperty(navigator, 'languages', {{ get: () => [{lang!r}] }}); " \
           f"Object.defineProperty(navigator, 'language', {{ get: () => {lang!r} }});"


def texts_of(lang):
    """The page's texts in lang, as its file holds them."""
    with open(os.path.join(I18N, lang + ".json"), encoding="utf-8") as f:
        return json.load(f)


# The dice's label in the languages the suite's cases do not run: before
# five words, before twenty characters.
DICE_SAYS = {
    "fr": ("Suggérer cinq mots", "Suggérer vingt caractères"),
    "it": ("Suggerisci cinque parole", "Suggerisci venti caratteri"),
    "nl": ("Vijf woorden voorstellen", "Twintig tekens voorstellen"),
    "pl": ("Zaproponuj pięć słów", "Zaproponuj dwadzieścia znaków"),
}


@pytest.mark.cases("1100-en")
def test_the_dice_names_its_next_kind_in_every_language(new_context, case, watch, guard):
    """In French, Italian, Dutch and Polish, each the browser's language,
    as the suite's cases run English and German: the first owner's dice
    labelled, aria-label and title, with its language's words (DICE_SAYS)
    for five words before the first press and after the second, and for
    twenty characters after the first (AP AccessEnds, D44-A), each press
    giving the kind its label named. Fails on the page before: Suggest a
    password."""
    for lang, said in DICE_SAYS.items():
        page = start_page(new_context, guard, watch, OPEN, init=lang_init(lang))
        owner_card(page)
        assert wait_until(page, f"i18n.lang === {lang!r}", 5), f"the page in {lang}"
        frames(page)
        dice = dice_of(page, "setup-password")

        def says(kind, what):
            want = said[0 if kind == "words" else 1]
            got = [dice.get_attribute("aria-label"), dice.get_attribute("title")]
            assert got == [want, want], f"{lang}: {what}: the dice says {kind} next in its words, {want!r}: {got}"
        says("words", "before the first press")
        v = press(page, "setup-password")
        assert five_words(v), f"{lang}: five words: {v!r}"
        says("chars", "after five words")
        c = press(page, "setup-password", v)
        assert is_chars(c), f"{lang}: twenty characters: {c!r}"
        says("words", "after twenty characters")
        page.context.close()


# The line under a new password's field that says what the rule finds:
# its height and its line's, and its place's, as tall as the tallest of
# its texts, each drawn hidden in the same cell.
RULE_PLACE = """(field) => {
  const line = document.getElementById(field + '-rule');
  return {text: line.textContent, height: line.getBoundingClientRect().height, lineHeight: parseFloat(getComputedStyle(line).lineHeight),
          place: line.closest('.pw-rule').getBoundingClientRect().height};
}"""


@pytest.mark.cases("390-en")
def test_the_rule_line_is_one_line_in_every_language(new_context, case, watch, guard):
    """Add a user at 390 px, the narrowest field of a new password there,
    in each of the six languages, the browser's: the word list not had,
    the line under the field says so on one line, in its language's words,
    and its place is one line tall, each of the line's texts fitting one
    line, so that the rows under it stay where they are (docs/ui.md). Fails
    with a text that takes two lines there, as German's first words for
    the word list not had did."""
    for lang in ("en", "de", "fr", "it", "nl", "pl"):
        texts = texts_of(lang)
        ctx = new_context()
        ctx.add_init_script(f"localStorage.setItem('massimo.token', {TOKEN!r})")
        ctx.add_init_script(lang_init(lang))
        page = ctx.new_page()
        guard.page = page
        page.set_default_timeout(5000)
        page.route("**/ui/words-en.txt", lambda route: route.fulfill(status=503, body="no"))
        page.goto(URL)
        page.wait_for_selector(".room", timeout=10000)
        assert wait_until(page, f"i18n.lang === {lang!r}", 5), f"the page in {lang}"
        show_view(page, "system")
        page.locator("#user-add").click()
        page.locator("#user-new-password").wait_for(state="visible")
        dice_of(page, "user-new-password").click()
        assert wait_until(page, "document.getElementById('user-new-password-rule').textContent === t('password.noList')", 5), \
            f"{lang}: the word list not had, said"
        r = page.evaluate(RULE_PLACE, "user-new-password")
        assert r["text"] == texts["password.noList"], f"{lang}: in its words: {r}"
        assert r["height"] <= r["lineHeight"] + 0.5 and abs(r["place"] - r["lineHeight"]) <= 0.5, \
            f"{lang}: the line and its place one line tall at 390 px: {r}"
        ctx.close()


def copied_here(page, sel, v, what, clipboard=True):
    """The element sel, clicked or given Enter (what), copied v and said
    so: "Copied" above it, in the page's bubble, and in the bubble's
    status region #bubble-status, the focus on it still; with clipboard,
    the browser's clipboard holds v."""
    where = wait_bubble_at(page, sel, words(page, "copy.done"))
    assert where in ("above", "below"), f"{what}: Copied shows at it: {where}"
    assert wait_until(page, BUBBLE_STATUS + " === t('copy.done')", 3), f"{what}: said in the status region"
    assert page.evaluate(FOCUSED, sel), f"{what}: the focus stays on it"
    if clipboard:
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === %r)" % v, 3), f"{what}: the suggestion copied"


def test_copy(page, case):
    """The shown line's suggestion copies itself, as the page's other
    strings do (Copy; AP AccessEnds, D44-A): a click on it, Enter and
    Space on it, copy the field's value, "Copied" showing on it and said
    in the status region, the focus staying, with the browser's clipboard
    and without it, as over plain HTTP on the LAN; its copy icon likewise.
    The line or its icon copied, then the dice pressed while Copied still
    shows: the new suggestion is not marked Copied. Fails on the page
    before: no copy in the shown line."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    show_view(page, "profile")
    open_fold(page, "#me-password")
    v = press(page, "me-new", "")
    line, icon = "#me-new-shown .pw-copy", "#me-new-shown .copy-icon"
    name = words(page, "password.copyName")
    assert page.locator(line).get_attribute("aria-label") == name and page.locator(icon).get_attribute("aria-label") == name, "named"
    # the clipboard and the status region emptied, the bubble taken away, so that each copy is seen anew
    clear = """() => {
      navigator.clipboard.writeText('');
      const s = document.getElementById('bubble-status');
      if (s) s.textContent = '';
      hideBubble();
    }"""
    page.evaluate(clear)
    page.locator(line).click()
    copied_here(page, line, v, "a click on the line")
    page.evaluate(clear)
    page.locator(line).focus()
    page.keyboard.press("Enter")
    copied_here(page, line, v, "Enter on the line")
    page.evaluate(clear)
    page.locator(line).focus()
    page.keyboard.press("Space")
    copied_here(page, line, v, "Space on the line")
    page.evaluate(clear)
    page.locator(icon).click()
    copied_here(page, icon, v, "a click on the icon")
    page.evaluate(clear)
    page.locator(icon).focus()
    page.keyboard.press("Enter")
    copied_here(page, icon, v, "Enter on the icon")
    page.evaluate(clear)
    page.evaluate("""() => {
      window.__clip = navigator.clipboard;
      Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    }""")
    try:
        page.locator(line).click()
        copied_here(page, line, v, "without the clipboard, a click on the line", clipboard=False)
        page.evaluate("() => { const s = document.getElementById('bubble-status'); if (s) s.textContent = ''; }")
        page.locator(icon).focus()
        page.keyboard.press("Enter")
        copied_here(page, icon, v, "without the clipboard, Enter on the icon", clipboard=False)
    finally:
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: window.__clip, configurable: true })""")
    assert page.evaluate("navigator.clipboard.readText()") == v, "the fallback copies"
    assert value(page, "me-new") == v and page.evaluate(STATE, "me-new")["type"] == "text", "the suggestion still shown"
    # copied, then the dice pressed while Copied still shows: the new suggestion, which nobody copied, is not marked so:
    # the bubble goes with the element it was shown at
    for what, sel in (("the line", line), ("its icon", icon)):
        page.evaluate(clear)
        page.locator(sel).click()
        where = wait_bubble_at(page, sel, words(page, "copy.done"))
        assert where in ("above", "below"), f"{what}: Copied shows at it: {where}"
        n = press(page, "me-new", value(page, "me-new"))
        shown = bubble(page)
        assert shown is None and page.locator(line).text_content() == n, \
            f"{what} copied, then the dice: the new suggestion not marked Copied: {shown}"
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def test_a_thousand_draws_within_the_rule(page, case):
    """A thousand suggestions of each kind through the handle: each five
    words of the list or twenty of the alphabet, each taken by the rule
    with the context the test gives, all different. Fails on the page
    before: no handle."""
    out = page.evaluate("""async () => {
      const s = window.MassimoParts.suggestPassword;
      if (typeof s !== 'function') return null;
      const ctx = ['operator', 'Anna Maria'];
      const refused = (p) => !!window.MassimoParts.passwordProblem(p, ctx);
      const w = [], c = [];
      for (let i = 0; i < 1000; i++) { w.push(await s('words', refused)); c.push(await s('chars', refused)); }
      return { w, c, bad: [...w, ...c].filter((p) => p === null || refused(p)) };
    }""")
    assert out is not None, "the handle"
    assert not out["bad"], f"all within the rule: {out['bad'][:5]}"
    assert all(five_words(v) for v in out["w"]), [v for v in out["w"] if not five_words(v)][:5]
    assert all(is_chars(v) for v in out["c"]), [v for v in out["c"] if not is_chars(v)][:5]
    assert len(set(out["w"])) == 1000 and len(set(out["c"])) == 1000, "all different"


def test_numbers_at_the_limit_are_drawn_again(page, case):
    """The page's crypto.getRandomValues handed chosen numbers: a word's
    32-bit number at or above floor(2^32 / 7776) × 7776 is drawn again, the
    one below it taken (7775, the last word); a character's byte at or above
    224 is drawn again, 223 taken (55, z)."""
    page.evaluate(STUB_RANDOM, [[WORD_LIMIT, 2 ** 32 - 1, WORD_LIMIT - 1, 0, 1, 2, 3], None])
    try:
        w = page.evaluate("window.MassimoParts.suggestPassword('words')")
        draws = page.evaluate("window.__draws")
        page.evaluate(STUB_RANDOM, [[224, 255] + [223] * 18 + [0, 1], None])
        c = page.evaluate("window.MassimoParts.suggestPassword('chars')")
    finally:
        page.evaluate(REAL_RANDOM)
    assert (WORD_LIMIT - 1) % 7776 == 7775
    assert w == "-".join([WORD_LIST[7775]] + WORD_LIST[0:4]), f"two numbers at the limit drawn again: {w!r}"
    assert draws == 7, f"seven numbers for five words: {draws}"
    assert c == "z" * 18 + "23", f"two bytes at 224 and above drawn again: {c!r}"


def test_a_name_that_is_a_word_is_drawn_again(page, case):
    """A user to be added named as a word of the list: a suggestion holding
    it is refused by the rule and drawn again; where every one of the 20
    draws holds it, at the press of five words after the press of twenty
    characters, the line says that none was found and the field keeps
    what it had."""
    console = watch_console(page)
    name = f"{WORD_LIST[0]}-{case.width}-{case.lang}"  # abacus-…
    show_view(page, "system")
    page.locator("#user-add").click()
    page.locator("#user-new-password").wait_for(state="visible")
    page.locator("#user-new input[name=name]").fill(name)
    page.evaluate(STUB_RANDOM, [[0, 1, 2, 3, 4, 5, 6, 7, 8, 9], None])
    try:
        v = press(page, "user-new-password", "")
        draws = page.evaluate("window.__draws")
    finally:
        page.evaluate(REAL_RANDOM)
    assert v == "-".join(WORD_LIST[5:10]), f"the draw with the name refused, the next taken: {v!r}"
    assert draws == 10, f"two draws of five words: {draws}"
    assert WORD_LIST[0] not in v
    c = press(page, "user-new-password", v)
    assert is_chars(c), f"the next press, twenty characters: {c!r}"
    page.evaluate(STUB_RANDOM, [[], 0])
    try:
        dice_of(page, "user-new-password").click()
        assert wait_until(page, "document.getElementById('user-new-password-rule').textContent === t('password.noSuggestion')", 5), \
            "none found: the line says so"
        draws = page.evaluate("window.__draws")
    finally:
        page.evaluate(REAL_RANDOM)
    assert draws == 100, f"20 draws of five words: {draws}"
    assert value(page, "user-new-password") == c, "the field keeps what it had"
    not_spoken(page, console, v, "the name's test")
    not_spoken(page, console, c, "the name's test")
    page.locator("#user-new .form-cancel").click()
    show_view(page, "tiles")


def fetched(page):
    """The requests for the word list the page sent."""
    return len([u for _, u in page.requests if u.endswith("/ui/words-en.txt")])


def test_the_list_is_fetched_once(new_context, case, watch, guard):
    """The word list is fetched at the first press of words, once for two
    presses of words while it loads, the press between them giving twenty
    characters at once, and the last press's words filling once it has
    loaded; a fetch that fails says so in the line, the next press still
    gives twenty characters, which need no list, the label saying so (AP
    AccessEnds, D51-A), and the press after fetches the list again; a
    suggestion that comes after the card opened anew fills nothing. Fails
    on the page before: the second press gave words, and the press after a
    failed fetch asked for words again."""
    page = start_page(new_context, guard, watch, OPEN)
    held = []
    page.route("**/ui/words-en.txt", lambda route: held.append(route))
    owner_card(page)
    assert not fetched(page), "not fetched before a press"
    dice_of(page, "setup-password").click()
    wait_held(page, held)
    says_next(page, "setup-password", "chars", "the first press waiting for the list")
    c = press(page, "setup-password", "")
    assert is_chars(c), f"the second press while it loads: twenty characters at once: {c!r}"
    dice_of(page, "setup-password").click()
    says_next(page, "setup-password", "chars", "the third press waiting for the list")
    held[0].continue_()
    assert wait_until(page, f"document.getElementById('setup-password').value !== {c!r}", 5), "filled once loaded"
    v = value(page, "setup-password")
    assert five_words(v), f"the last press's words: {v!r}"
    assert len(held) == 1, f"two presses of words while it loads, one fetch: {len(held)}"
    n = press(page, "setup-password", v)
    assert is_chars(n), f"the next, twenty characters: {n!r}"
    w = press(page, "setup-password", n)
    assert five_words(w) and w != v, f"the list kept: the next press of words fills: {w!r}"
    assert fetched(page) == 1, "fetched once"
    # a fetch that fails, on a page of its own
    failing = start_page(new_context, guard, watch, OPEN)
    failing.route("**/ui/words-en.txt", lambda route: route.fulfill(status=503, body="no"))
    owner_card(failing)
    dice_of(failing, "setup-password").click()
    assert wait_until(failing, "document.getElementById('setup-password-rule').textContent === t('password.noList')", 5), "the line says so"
    assert value(failing, "setup-password") == "", "nothing filled"
    says_next(failing, "setup-password", "chars", "after the list not had")
    dice_of(failing, "setup-password").click()
    assert wait_until(failing, "document.getElementById('setup-password').value.length === 20", 5), "the next press gives twenty characters"
    assert is_chars(value(failing, "setup-password")), value(failing, "setup-password")
    assert wait_until(failing, "document.getElementById('setup-password-rule').textContent === t('password.suggested')", 3), "and the line says so"
    failing.unroute("**/ui/words-en.txt")
    dice_of(failing, "setup-password").click()
    assert wait_until(failing, "document.getElementById('setup-password').value.split('-').length >= 5", 5), "the press after fetches it again"
    assert fetched(failing) == 2, f"fetched again: {fetched(failing)}"
    # a suggestion that comes after the card opened anew: nothing filled, no shown line
    late = start_page(new_context, guard, watch, OPEN)
    held = []
    late.route("**/ui/words-en.txt", lambda route: held.append(route))
    owner_card(late)
    dice_of(late, "setup-password").click()
    wait_held(late, held)
    reopen(late, "owner")
    owner_card(late)
    held.pop().continue_()
    # the list loaded: the press waiting for it has gone on before this one's answer comes
    late.evaluate("() => window.MassimoParts.suggestPassword('words').then(() => {})")
    frames(late)
    assert value(late, "setup-password") == "" and value(late, "setup-again") == "", "nothing filled after the new start"
    assert late.locator("#setup-password-shown").count() == 0, "and no shown line"
    says_next(late, "setup-password", "words", "the card opened anew, a new start")


def held_list(page):
    """The word list's request held until the test lets it go: the list of
    held routes."""
    held = []
    page.route("**/ui/words-en.txt", lambda route: held.append(route))
    return held


def let_list_go(page, held):
    """The held request let go, and the list loaded: a press that waited
    for it has gone on before the next one's answer comes."""
    assert wait_held(page, held), "the list's request held"
    held.pop().continue_()
    page.evaluate("() => window.MassimoParts.suggestPassword('words').then(() => {})")
    frames(page)


def test_a_late_suggestion_does_not_overwrite_a_later_press(new_context, case, watch, guard):
    """Words pressed while the list loads, then a second tap, which gives
    twenty characters at once: the characters fill, the words that come
    later do not replace them, and the dice says five words next."""
    page = start_page(new_context, guard, watch, OPEN)
    held = held_list(page)
    owner_card(page)
    dice_of(page, "setup-password").click()
    assert wait_held(page, held), "the list's request held"
    dice_of(page, "setup-password").click()
    assert wait_until(page, "document.getElementById('setup-password').value.length === 20", 5), "the second tap fills twenty characters"
    c = value(page, "setup-password")
    let_list_go(page, held)
    assert value(page, "setup-password") == c and value(page, "setup-again") == c, "the late words fill nothing"
    says_next(page, "setup-password", "words", "after the late words")


def test_a_late_suggestion_does_not_overwrite_a_typed_password(new_context, case, watch, guard):
    """Words pressed while the list loads, then a password typed by hand:
    the suggestion that comes later does not replace it."""
    page = start_page(new_context, guard, watch, OPEN)
    held = held_list(page)
    owner_card(page)
    dice_of(page, "setup-password").click()
    page.locator("#setup-password").press_sequentially("typed-by-hand")
    let_list_go(page, held)
    assert value(page, "setup-password") == "typed-by-hand" and value(page, "setup-again") == "", "what was typed stays"
    assert page.evaluate(STATE, "setup-password")["type"] == "password", "a password's field"


def test_a_late_suggestion_does_not_fill_after_a_save(page, case):
    """My profile: a password typed twice, Words pressed while the list
    loads, then Save: the typed password is saved, and the suggestion that
    comes later fills nothing (the PUT stubbed, so that the user stays as
    it was)."""
    show_view(page, "profile")
    open_fold(page, "#me-password")
    held = held_list(page)
    page.locator("#me-new").fill("saved-by-hand")
    page.locator("#me-again").fill("saved-by-hand")
    put = []

    def user(route):
        if route.request.method != "PUT":
            return route.continue_()
        put.append(json.loads(route.request.post_data or "{}"))
        route.fulfill(status=200, content_type="application/json", body="{}")
    page.route("**/api/v1/users/owner", user)
    page.evaluate(STUB_REGISTER)
    try:
        dice_of(page, "me-new").click()
        assert wait_held(page, held)
        page.locator("#me-password-form button[type=submit]").click()
        assert wait_until(page, "document.getElementById('me-password-msg').classList.contains('ok')", 10), "saved"
        assert page.evaluate("window.__registered") == ["saved-by-hand"], "the typed password saved"
        let_list_go(page, held)
        left = [value(page, "me-new"), value(page, "me-again")]
        assert left[0] in ("", "saved-by-hand") and left[1] in ("", "saved-by-hand"), f"nothing filled after the save: {left}"
        assert page.evaluate(STATE, "me-new")["type"] == "password", "a password's field"
    finally:
        page.evaluate(REAL_REGISTER)
        page.unroute("**/api/v1/users/owner")
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def test_a_list_of_another_form_is_refused(new_context, case, watch, guard):
    """The list is held to its form: with line ends of CRLF, or a word
    twice, it is not taken, and the line says that it was not loaded; a
    failure of the browser's random numbers at the next press, which gives
    twenty characters, says that no suggestion could be made."""
    with open(LIST, encoding="utf-8") as f:
        good = f.read()
    for what, body in (("CRLF", good.replace("\n", "\r\n")), ("a word twice", good.replace("zoom\n", "abacus\n"))):
        page = start_page(new_context, guard, watch, OPEN)
        page.route("**/ui/words-en.txt", (lambda b: lambda route: route.fulfill(status=200, content_type="text/plain", body=b))(body))
        owner_card(page)
        dice_of(page, "setup-password").click()
        assert wait_until(page, "document.getElementById('setup-password-rule').textContent === t('password.noList')", 5), f"{what}: not loaded"
        assert value(page, "setup-password") == "", f"{what}: nothing filled"
    page.evaluate("() => { crypto.getRandomValues = () => { throw new Error('no random numbers'); }; }")
    says_next(page, "setup-password", "chars", "after the list refused")
    dice_of(page, "setup-password").click()
    assert wait_until(page, "document.getElementById('setup-password-rule').textContent === t('password.noSuggestion')", 5), \
        "a failure of the random numbers: no suggestion could be made"


def reset_after_a_press(page, field, again, reset):
    """A password typed, Words pressed while the list loads, then the
    form's Reset: the suggestion that comes later fills nothing, the shown
    line has gone with the Reset, and the next press gives five words."""
    held = held_list(page)
    page.locator("#" + field).fill("typed-before-reset")
    if again:
        page.locator("#" + again).fill("typed-before-reset")
    dice_of(page, field).click()
    assert wait_held(page, held)
    page.locator(reset).click()
    frames(page)
    assert value(page, field) == "", f"{field}: emptied by Reset"
    let_list_go(page, held)
    assert value(page, field) == "" and (not again or value(page, again) == ""), f"{field}: nothing filled after the Reset"
    assert page.evaluate(STATE, field)["type"] == "password", f"{field}: a password's field"
    assert page.locator(f"#{field}-shown").count() == 0, f"{field}: no shown line after the Reset"
    says_next(page, field, "words", "after the Reset")


def test_a_late_suggestion_does_not_fill_after_my_profiles_reset(page, case):
    """My profile's Set a password: Reset after a press while the list loads."""
    show_view(page, "profile")
    open_fold(page, "#me-password")
    reset_after_a_press(page, "me-new", "me-again", "#me-password-form .form-reset")
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def test_a_late_suggestion_does_not_fill_after_a_panels_reset(page, case):
    """A user's panel: Reset after a press while the list loads."""
    name = f"late-reset-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "fixed-pw-01"})
    try:
        show_view(page, "system")
        panel = page.locator(".user-panel[data-name='%s']" % name)
        panel.wait_for()
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        field = f"user-{name}-password"
        page.locator("#" + field).wait_for(state="visible")
        reset_after_a_press(page, field, None, ".user-panel[data-name='%s'] form .form-reset" % name)
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_a_late_suggestion_does_not_overwrite_again(new_context, case, watch, guard):
    """Words pressed while the list loads, then a password typed into
    Again: the suggestion that comes later fills neither field."""
    page = start_page(new_context, guard, watch, OPEN)
    held = held_list(page)
    owner_card(page)
    dice_of(page, "setup-password").click()
    page.locator("#setup-again").press_sequentially("typed-in-again")
    let_list_go(page, held)
    assert value(page, "setup-again") == "typed-in-again" and value(page, "setup-password") == "", "nothing filled over Again"


# The places of the field's row and of the row after the field's: the
# line that shows the suggestion (its top and height, None without it),
# the rule's line's top, and the field's and the next field's tops.
PLACES = """(field) => {
  const shown = document.getElementById(field + '-shown');
  const row = document.getElementById(field).closest('.field').nextElementSibling;
  return {rule: Math.round(document.getElementById(field + '-rule').getBoundingClientRect().top), next: Math.round(row.getBoundingClientRect().top),
          own: Math.round(document.getElementById(field).getBoundingClientRect().top),
          shown: shown ? [Math.round(shown.getBoundingClientRect().top), Math.round(shown.getBoundingClientRect().height)] : null};
}"""


def test_the_shown_line_stays_while_one_types(new_context, case, watch, guard):
    """After a suggestion, the field emptied by keys and typed in again:
    the line that showed the suggestion keeps its place, empty, its copy
    gone, no row comes, and nothing under the field, nor the field in the
    centred card, moves."""
    page = start_page(new_context, guard, watch, OPEN)
    owner_card(page)
    press(page, "setup-password")
    before = page.evaluate(PLACES, "setup-password")
    f = page.locator("#setup-password")
    f.focus()
    page.keyboard.press("ControlOrMeta+a")
    page.keyboard.press("Backspace")
    frames(page)
    assert value(page, "setup-password") == "", "emptied by keys"
    emptied = page.evaluate(PLACES, "setup-password")
    f.press_sequentially("m")
    frames(page)
    typed = page.evaluate(PLACES, "setup-password")
    assert before == emptied == typed and before["shown"] is not None, \
        f"the shown line's place stays, nothing moves: {before}, {emptied}, {typed}"
    assert copy_of(page, "setup-password").count() == 0 and icon_of(page, "setup-password").count() == 0, "no copy for what is typed"
    no_row(page, "setup-password")


# The lines five words take at a width, by their widths in the body's
# font, broken after a "-" as the browser breaks the shown line: each line
# holds the words that fit it, a word with its "-" but the last.
WORDS_LINES = """([words, width]) => {
  const c = document.createElement('canvas').getContext('2d');
  c.font = getComputedStyle(document.body).font;
  let lines = 1, x = 0;
  words.forEach((w, i) => {
    const n = c.measureText(i === words.length - 1 ? w : w + '-').width;
    if (x > 0 && x + n > width) { lines++; x = 0; }
    x += n;
  });
  return lines;
}"""


@pytest.mark.cases("390-en", "390-de")
def test_the_widest_suggestion_in_a_users_panel(page, case):
    """The widest suggestion of five words there can be, the list's widest
    word five times, the page's random numbers handed its place in the
    list, in a user's panel at 390 px: the line under the field shows it
    whole inside the field's row, its copy icon after it, broken after a
    "-" alone, on as many lines as the words' widths give at the width
    the icon leaves them (WORDS_LINES): three with the test machine's
    fonts, where three of the widest do not fit a line, the place as tall
    as they; and the page has no horizontal scroll (AP AccessEnds, D21-A,
    D44-A). Fails on the page before: no copy in the line."""
    name = f"widest-{case.width}-{case.lang}"
    remove_user(name)
    api_call("POST", "api/v1/users", {"name": name, "role": "viewer", "password": "fixed-pw-01"})
    panel = ".user-panel[data-name='%s']" % name
    try:
        show_view(page, "system")
        page.locator(panel).wait_for()
        toggle_fold(page, panel)
        field = f"user-{name}-password"
        page.locator("#" + field).wait_for(state="visible")
        widest = page.evaluate(WIDEST, [WORD_LIST, ALPHABET])[0]
        page.evaluate(STUB_RANDOM, [[WORD_LIST.index(widest)] * 5, None])
        try:
            v = press(page, field, "")
        finally:
            page.evaluate(REAL_RANDOM)
        assert v == "-".join([widest] * 5), f"the widest suggestion: {v!r}"
        s = shown_line(page, field, v)
        given = page.evaluate(WORDS_LINES, [[widest] * 5, s["room"]])
        assert s["lines"] == given >= 2, f"on the lines the widths give, {given}: {s}"
        assert scroll_width(page) <= case.width, f"no horizontal scroll: {scroll_width(page)}"
        page.locator(panel + " form .form-reset").click()
        toggle_fold(page, panel)
    finally:
        remove_user(name)
    show_view(page, "tiles")
