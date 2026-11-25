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
edited; another press gives another. From the first press on, a row under
the field offers Words or Characters, kept in this browser, and Copy; the
row is not there before. The line under the field says that a password
was suggested, and no status region, label or console line holds it.
Picking is without bias, numbers at or above the limit drawn again,
which the test shows by handing the page's crypto.getRandomValues chosen
numbers; a suggestion the rule refuses, a word of the list that is the
user's name, is drawn again, at most 20 times, and the line says so
where none is found. The list is fetched once, at the first press that
needs it; a fetch that fails says so in the line, and Characters still
works; a suggestion that comes after the card has opened anew fills
nothing."""
import json
import os
import re

from conftest import TOKEN, api_call
from helpers import OPERATOR_PASSWORD, frames, named_buttons, show_view, wait_held, wait_until
from test_first_owner import OPEN, owner_card, start_page, stub_creation
from test_login import sign_in
from test_password_eye import STATE, eye_of, fresh_page, reopen, say_reveal
from test_password_rule import REAL_REGISTER, SAME_LINE, STUB_REGISTER, WATCH_LINE, line, words
from test_users import remove_user

LIST = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "internal", "server", "ui", "words-en.txt")
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
# status and live regions' texts, every aria-label and title; and the
# console's lines, which the test gathers (watch_console).
SPOKEN = """() => {
  const out = [];
  for (const el of document.querySelectorAll('[role=status], [aria-live], [role=alert]')) out.push(el.textContent);
  for (const el of document.querySelectorAll('[aria-label], [title]')) out.push(el.getAttribute('aria-label') || '', el.getAttribute('title') || '');
  return out;
}"""


# A fold's summary clicked, and the fold's toggle event taken: the event,
# which tells the page's part the fold's new state (Fold's onToggle) and
# so is a new start of its password fields, comes in a task of its own
# after the click; a press on the dice before it would be dropped by that
# new start.
TOGGLE = """(sel) => new Promise((done) => {
  const d = document.querySelector(sel);
  d.addEventListener('toggle', () => setTimeout(done, 0), { once: true });
  d.querySelector(':scope > summary').click();
})"""


def toggle_fold(page, sel):
    """The fold sel opened or closed by its summary, its toggle event taken
    and the part drawn again."""
    page.evaluate(TOGGLE, sel)
    frames(page)


def open_fold(page, sel):
    """The fold sel open, its toggle event taken (toggle_fold)."""
    if not page.evaluate("(sel) => document.querySelector(sel).open", sel):
        toggle_fold(page, sel)


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
    """The Copy of the field's row."""
    return page.locator(f".field:has(> .pw-box > #{field}) .pw-copy")


def dice_of(page, field):
    return page.locator(f".pw-box:has(> #{field}) > .pw-dice")


def value(page, field):
    return page.locator("#" + field).input_value()


def kind_of(page, field, kind):
    return page.locator(f"#{field}-kind input[value={kind}]")


def watch_console(page):
    lines = []
    page.on("console", lambda m: lines.append(m.text))
    return lines


def not_spoken(page, console, password, what):
    """No status region, label, title or console line holds the password."""
    told = [t for t in page.evaluate(SPOKEN) + console if password in t]
    assert not told, f"{what}: the password is not told: {told}"


def press(page, field, before=None):
    """The dice pressed: the field holds a new value, which is returned."""
    old = value(page, field) if before is None else before
    dice_of(page, field).click()
    assert wait_until(page, f"document.getElementById({field!r}).value !== {old!r} && document.getElementById({field!r}).value !== ''", 5), \
        f"{field}: the dice fills it"
    # the line written anew: emptied, then its words (PasswordRule's fresh)
    assert wait_until(page, f"document.getElementById({field + '-rule'!r}).textContent !== ''", 3), f"{field}: the line written again"
    frames(page)
    return value(page, field)


def check_suggest(page, case, field, again, console):
    """The form's first new password, field, and its second, again where it
    has one: no row before the first press; the dice before the eye inside
    the box, labelled; a press fills both with five words of the list, shown
    as text, the line saying it was suggested and nothing telling the
    password; the row comes with it, Words chosen; another press gives
    another; Characters gives twenty of the alphabet; an edit makes it a
    password's field again, the line the rule's. Returns the last value."""
    assert page.locator(f"#{field}-kind").count() == 0, f"{field}: no row before the first press"
    dice = dice_of(page, field)
    dice.wait_for(state="visible")
    assert dice.get_attribute("aria-label") == words(page, "password.suggest") and dice.get_attribute("title") == words(page, "password.suggest"), \
        f"{field}: the dice labelled"
    eye = eye_of(page, field)
    if eye.count():
        assert dice.bounding_box()["x"] + dice.bounding_box()["width"] <= eye.bounding_box()["x"] + 0.5, f"{field}: the dice before the eye"
    if again:
        assert dice_of(page, again).count() == 0, f"{again}: no dice on the second field"
    v = press(page, field)
    assert five_words(v), f"{field}: five words of the list: {v!r}"
    if again:
        assert value(page, again) == v, f"{again}: filled alike"
    s = page.evaluate(STATE, field)
    assert s["type"] == "text", f"{field}: the suggestion shown: {s}"
    assert line(page, field)["text"] == words(page, SUGGESTED), f"{field}: the line says it was suggested: {line(page, field)}"
    assert page.evaluate("(p) => window.MassimoParts.passwordProblem(p, [])", v) == "", f"{field}: the rule takes it"
    assert kind_of(page, field, "words").is_checked(), f"{field}: the row with Words chosen"
    assert page.evaluate("document.activeElement && document.activeElement.classList.contains('pw-dice')"), f"{field}: the focus on the dice"
    assert copy_of(page, field).count() == 1, f"{field}: Copy offered for the suggestion"
    not_spoken(page, console, v, field)
    page.evaluate(WATCH_LINE, field)
    w = press(page, field, v)
    assert five_words(w) and w != v, f"{field}: another press, another: {w!r}"
    assert wait_until(page, f"window.__lineWrites > 0 && document.getElementById({field + '-rule'!r}).textContent === t('password.suggested')", 3), \
        f"{field}: the line written anew at the second press, for a screen reader: {page.evaluate('window.__lineWrites')}"
    assert page.evaluate(SAME_LINE, field), f"{field}: the same line"
    page.evaluate("() => { window.__lineWatch.disconnect(); }")
    kind_of(page, field, "chars").check()
    assert wait_until(page, f"document.getElementById({field!r}).value.length === 20", 5), f"{field}: Characters suggests at once"
    c = value(page, field)
    assert is_chars(c), f"{field}: twenty of the alphabet: {c!r}"
    if again:
        assert value(page, again) == c, f"{again}: filled alike"
    not_spoken(page, console, c, field)
    named_buttons(page, f"{field} with its dice")
    kind_of(page, field, "words").check()
    assert wait_until(page, f"document.getElementById({field!r}).value.length !== 20", 5), f"{field}: Words again"
    return value(page, field)


def edited(page, field):
    """A key typed into the field: a password's field again, the eye as it
    was (hidden), the line the rule's."""
    page.locator("#" + field).press("End")
    page.locator("#" + field).press_sequentially("x")
    frames(page)
    assert page.evaluate(STATE, field)["type"] == "password", f"{field}: edited, a password's field again"
    assert line(page, field)["text"] == words(page, "password.fine"), f"{field}: the rule's line"
    assert copy_of(page, field).count() == 0, f"{field}: edited, no Copy"


def reset_ends(page, field, reset):
    """A suggestion, then the form's Reset: the field emptied is a
    password's field again, its eye not pressed, and the row and Copy
    gone."""
    press(page, field)
    assert page.evaluate(STATE, field)["type"] == "text"
    page.locator(reset).click()
    frames(page)
    s = page.evaluate(STATE, field)
    assert s["value"] == "" and s["type"] == "password" and s.get("pressed", "false") == "false", f"{field}: Reset ends the suggestion: {s}"
    assert page.locator(f"#{field}-kind").count() == 0 and copy_of(page, field).count() == 0, f"{field}: Reset: no row, no Copy"


def test_the_first_owners_form(new_context, case, watch, guard):
    """The first owner's form, its fields not controlled: the dice beside
    Password fills both; the row comes with the first press; an edit hides
    it; a suggestion creates the owner."""
    page = start_page(new_context, guard, watch, OPEN)
    console = watch_console(page)
    owner_card(page)
    page.locator("#setup-user").fill("owner")
    check_suggest(page, case, "setup-password", "setup-again", console)
    v = press(page, "setup-password")
    made = []
    stub_creation(page, made)
    page.locator("#setup-form button[type=submit]").click()
    assert wait_until(page, "!document.getElementById('login-dialog').open", 10), "the suggestion creates the owner"
    assert len(made) == 1, f"created once: {made}"
    not_spoken(page, console, v, "after the creation")


def test_my_profiles_set_a_password(page, case):
    """My profile of a user with keys only (the test's owner): Set a
    password, the fields controlled; the suggestion is saved, registered as
    suggested, and hidden at the submit (the PUT stubbed, so that the
    user stays as it was)."""
    console = watch_console(page)
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").wait_for(state="visible")
    check_suggest(page, case, "me-new", "me-again", console)
    edited(page, "me-new")
    v = press(page, "me-new")
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
    with that password, and a user's panel, its one field filled."""
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
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        assert wait_until(page, f"!document.querySelector(\".user-panel[data-name='{name}']\").open", 3)
        toggle_fold(page, ".user-panel[data-name='%s']" % name)
        page.locator("#" + field).wait_for(state="visible")
        assert page.locator(f"#{field}-kind").count() == 0, "the panel closed and opened: the row gone with the new start"
        assert page.evaluate(STATE, field)["type"] == "password", "and the suggestion hidden"
    finally:
        remove_user(name)
    show_view(page, "tiles")


def test_shown_with_the_eye_off(new_context, case, watch, guard):
    """With auth.password_reveal off there is no eye, and the suggestion
    still shows, until it is edited. With the eye on, a password shown by
    it stays shown after an edit of a suggestion, and the eye pressed on a
    suggestion hides it."""
    page = start_page(new_context, guard, watch, OPEN)
    say_reveal(page, False)
    page.reload()
    owner_card(page)
    assert wait_until(page, "state.version !== null", 5), "/healthz answered, its eye taken (checkVersion)"
    frames(page)
    assert page.locator(".pw-eye").count() == 0, "no eye"
    press(page, "setup-password")
    assert page.evaluate(STATE, "setup-password")["type"] == "text", "the suggestion shown, without the eye"
    edited(page, "setup-password")
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


def test_the_choice_is_kept_and_the_keyboard_reaches_the_dice(page, case):
    """The keyboard: Tab from the field reaches the dice, Enter and Space
    press it, the focus staying there. Characters chosen is kept after a
    load: the next press suggests characters."""
    show_view(page, "profile")
    open_fold(page, "#me-password")
    page.locator("#me-new").focus()
    page.keyboard.press("Tab")
    assert page.evaluate("document.activeElement === document.querySelector('.pw-box:has(> #me-new) > .pw-dice')"), "Tab reaches the dice"
    page.keyboard.press("Enter")
    assert wait_until(page, "document.getElementById('me-new').value !== ''", 5), "Enter presses it"
    first = value(page, "me-new")
    page.keyboard.press("Space")
    assert wait_until(page, f"document.getElementById('me-new').value !== {first!r}", 5), "and Space"
    assert page.evaluate("document.activeElement.classList.contains('pw-dice')"), "the focus stays on the dice"
    kind_of(page, "me-new", "chars").check()
    assert wait_until(page, "document.getElementById('me-new').value.length === 20", 5)
    page.reload()
    assert wait_until(page, "!!window.MassimoParts && state.eventsOpen", 10), "loaded again"
    show_view(page, "profile")
    open_fold(page, "#me-password")
    v = press(page, "me-new", "")
    assert is_chars(v), f"after a load, characters: {v!r}"
    assert kind_of(page, "me-new", "chars").is_checked(), "Characters chosen"
    page.evaluate("localStorage.removeItem('massimo.suggest')")
    toggle_fold(page, "#me-password")
    show_view(page, "tiles")


def test_copy(page, case):
    """Copy copies what the field holds and says so, with the browser's
    clipboard and without it, as over plain HTTP on the LAN."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    show_view(page, "profile")
    open_fold(page, "#me-password")
    v = press(page, "me-new", "")
    page.evaluate("navigator.clipboard.writeText('')")
    copy = page.locator(".pw-suggest .pw-copy")
    assert copy.get_attribute("aria-label") == words(page, "password.copyName"), "Copy named"
    copy.click()
    assert wait_until(page, "navigator.clipboard.readText().then((t) => t === %r)" % v, 3), "Copy copies the suggestion"
    assert wait_until(page, "document.getElementById('copy-status').textContent === t('copy.done')", 3), "and says so"
    page.evaluate("navigator.clipboard.writeText('')")
    page.evaluate("""() => {
      window.__clip = navigator.clipboard;
      Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    }""")
    try:
        page.evaluate("document.getElementById('copy-status').textContent = ''")
        copy.click()
        assert wait_until(page, "document.getElementById('copy-status').textContent === t('copy.done')", 3), "without the clipboard: said"
        assert page.evaluate("document.activeElement === document.querySelector('.pw-suggest .pw-copy')"), "the focus stays on Copy"
    finally:
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: window.__clip, configurable: true })""")
    assert page.evaluate("navigator.clipboard.readText()") == v, "the fallback copies"
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
    draws holds it, the line says that none was found and the field keeps
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
    page.evaluate(STUB_RANDOM, [[], 0])
    try:
        dice_of(page, "user-new-password").click()
        assert wait_until(page, "document.getElementById('user-new-password-rule').textContent === t('password.noSuggestion')", 5), \
            "none found: the line says so"
        draws = page.evaluate("window.__draws")
    finally:
        page.evaluate(REAL_RANDOM)
    assert draws == 100, f"20 draws of five words: {draws}"
    assert value(page, "user-new-password") == v, "the field keeps what it had"
    not_spoken(page, console, v, "the name's test")
    page.locator("#user-new .form-cancel").click()
    show_view(page, "tiles")


def test_the_list_is_fetched_once(new_context, case, watch, guard):
    """The word list is fetched at the first press, once for two presses
    while it loads; a fetch that fails says so in the line, Characters
    still suggests, and the next press of Words fetches it again; a
    suggestion that comes after the card opened anew fills nothing."""
    page = start_page(new_context, guard, watch, OPEN)
    held = []
    page.route("**/ui/words-en.txt", lambda route: held.append(route))
    owner_card(page)
    assert not [u for _, u in page.requests if u.endswith("/ui/words-en.txt")], "not fetched before a press"
    dice_of(page, "setup-password").click()
    dice_of(page, "setup-password").click()
    wait_held(page, held)
    held[0].continue_()
    assert wait_until(page, "document.getElementById('setup-password').value !== ''", 5), "filled once loaded"
    assert len(held) == 1, f"two presses while it loads, one fetch: {len(held)}"
    v = value(page, "setup-password")
    assert five_words(v), v
    dice_of(page, "setup-password").click()
    assert wait_until(page, f"document.getElementById('setup-password').value !== {v!r}", 5), "the list kept: the next press fills"
    assert len([u for _, u in page.requests if u.endswith("/ui/words-en.txt")]) == 1, "fetched once"
    # a fetch that fails, on a page of its own
    failing = start_page(new_context, guard, watch, OPEN)
    failing.route("**/ui/words-en.txt", lambda route: route.fulfill(status=503, body="no"))
    owner_card(failing)
    dice_of(failing, "setup-password").click()
    assert wait_until(failing, "document.getElementById('setup-password-rule').textContent === t('password.noList')", 5), "the line says so"
    assert value(failing, "setup-password") == "", "nothing filled"
    kind_of(failing, "setup-password", "chars").check()
    assert wait_until(failing, "document.getElementById('setup-password').value.length === 20", 5), "Characters still suggests"
    assert wait_until(failing, "document.getElementById('setup-password-rule').textContent === t('password.suggested')", 3), "and the line says so"
    failing.unroute("**/ui/words-en.txt")
    kind_of(failing, "setup-password", "words").check()
    assert wait_until(failing, "document.getElementById('setup-password').value.split('-').length >= 5", 5), "Words fetches it again"
    failing.evaluate("localStorage.removeItem('massimo.suggest')")
    # a suggestion that comes after the card opened anew: nothing filled, no row
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
    assert late.locator("#setup-password-kind").count() == 0, "and no row"


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
    """Words pressed while the list loads, then Characters: the characters
    fill, and the words that come later do not replace them."""
    page = start_page(new_context, guard, watch, OPEN)
    held = held_list(page)
    owner_card(page)
    dice_of(page, "setup-password").click()
    kind_of(page, "setup-password", "chars").check()
    assert wait_until(page, "document.getElementById('setup-password').value.length === 20", 5), "Characters fills"
    c = value(page, "setup-password")
    let_list_go(page, held)
    assert value(page, "setup-password") == c and value(page, "setup-again") == c, "the late words fill nothing"
    assert kind_of(page, "setup-password", "chars").is_checked(), "Characters still chosen"
    page.evaluate("localStorage.removeItem('massimo.suggest')")


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
    failure of the browser's random numbers at a press of Characters says
    that no suggestion could be made."""
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
    kind_of(page, "setup-password", "chars").check()
    assert wait_until(page, "document.getElementById('setup-password-rule').textContent === t('password.noSuggestion')", 5), \
        "a failure of the random numbers: no suggestion could be made"
    page.evaluate("localStorage.removeItem('massimo.suggest')")


def reset_after_a_press(page, field, again, reset):
    """A password typed, Words pressed while the list loads, then the
    form's Reset: the suggestion that comes later fills nothing, and the
    row has gone with the Reset."""
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
    assert page.locator(f"#{field}-kind").count() == 0, f"{field}: no row after the Reset"


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
# kind's choice (None without it) and the next field's top.
PLACES = """(field) => {
  const kind = document.getElementById(field + '-kind');
  const row = document.getElementById(field).closest('.field').nextElementSibling;
  return {kind: kind ? Math.round(kind.getBoundingClientRect().top) : null, next: Math.round(row.getBoundingClientRect().top),
          own: Math.round(document.getElementById(field).getBoundingClientRect().top)};
}"""


def test_the_row_stays_while_one_types(new_context, case, watch, guard):
    """After a suggestion, the field emptied by keys and typed in again:
    the row stays, Copy goes, and nothing under the field, nor the field
    in the centred card, moves."""
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
    assert before == emptied == typed and before["kind"] is not None, f"the row stays, nothing moves: {before}, {emptied}, {typed}"
    assert copy_of(page, "setup-password").count() == 0, "no Copy for what is typed"
