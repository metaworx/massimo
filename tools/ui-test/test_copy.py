# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Copying (components.js copyable, copyIcon, copyEl): a text that copies
on a click, its copy icon, Enter, and the fallback a page over plain HTTP
needs, where the browser gives it no clipboard, which gives the focus back
where it was. And the bubble that says "Copied" (components.js
showBubble): above its control, below it at the window's top, at the
window's foot without one, said once for a screen reader."""
from helpers import BUBBLE_STATUS, bubble, bubble_at, held_timers, scaled, wait_bubble_at, wait_until

DONE = {"en": "Copied", "de": "Kopiert"}

# A copyable serial with its icon, put at the top of the page.
MAKE = """() => {
  const el = copyEl("KEQ0000100", { label: "the serial", icon: true });
  el.id = "test-copy";
  document.querySelector("main").prepend(el);
}"""


def clipboard(page):
    return page.evaluate("navigator.clipboard.readText()")


def test_copy(page, case):
    """A click on the text copies it, and so does its icon and Enter on
    it; "Copied" shows on it and is said; without the browser's clipboard
    the fallback copies all the same."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    page.evaluate(MAKE)
    try:
        text = page.locator("#test-copy .mono")
        assert text.get_attribute("role") == "button" and text.get_attribute("tabindex") == "0", "a button to the keyboard"
        assert "the serial" in (text.get_attribute("aria-label") or ""), "named by what it copies"
        page.evaluate("navigator.clipboard.writeText('')")
        with held_timers(page, "copied"):  # "Copied" until the checks are done, not its 1400 ms
            text.click()
            assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "a click copies"
            where = wait_bubble_at(page, "#test-copy .mono", DONE[case.lang])
            assert where == "above", f"Copied shows above it: {where}"
            assert wait_until(page, BUBBLE_STATUS + " === %r" % DONE[case.lang], 3), "and is said"
        page.evaluate("navigator.clipboard.writeText('')")
        page.locator("#test-copy .copy-icon").click()
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "the icon copies"
        page.evaluate("navigator.clipboard.writeText('')")
        text.focus()
        page.keyboard.press("Enter")
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'KEQ0000100')", 3), "Enter copies"
        # Plain HTTP: no navigator.clipboard for the page; the fallback copies.
        page.evaluate("navigator.clipboard.writeText('')")
        page.evaluate("""() => {
          window.__clip = navigator.clipboard;
          Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
        }""")
        text.click()
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: window.__clip, configurable: true })""")
        assert clipboard(page) == "KEQ0000100", "the fallback copies"
        assert page.evaluate("document.activeElement === document.querySelector('#test-copy .mono')"), \
            "the focus back where it was, not on the page's body"
        # by the keyboard too: the copy icon, Enter
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true })""")
        page.locator("#test-copy .copy-icon").focus()
        page.keyboard.press("Enter")
        page.evaluate("""() => Object.defineProperty(navigator, "clipboard", { value: window.__clip, configurable: true })""")
        assert wait_until(page, BUBBLE_STATUS + " !== ''", 3)
        assert page.evaluate("document.activeElement === document.querySelector('#test-copy .copy-icon')"), "the focus back on the icon"
    finally:
        page.evaluate("document.getElementById('test-copy').remove()")


def test_proxies_copy(page, case, reads):
    """System Info's Cube proxies: each address copies, by its icon, the
    line's last after the serials'. The test's server runs without
    proxies, so the system's answer gets one."""
    from helpers import show_view
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))

    def with_proxy(v):
        v["cube_proxies"] = [{"serial": "KEQ0000100", "addr": "192.168.1.31:62910", "listening": True}]
        return v
    reads.change("**/api/v1/system", with_proxy)
    try:
        show_view(page, "info")  # which reads the system's answer
        assert wait_until(page, "!!document.querySelector('#info-proxies .copy-icon')", 5), "the proxy with its copy icon"
        page.evaluate("navigator.clipboard.writeText('')")
        page.locator("#info-proxies .sys-proxy .copy-icon").last.click()
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === '192.168.1.31:62910')", 3), "its address copies"
    finally:
        reads.clear("**/api/v1/system")
        show_view(page, "tiles")


# A button of the test fixed in the window, at top px from its top, at
# its left edge, in its middle or at its right edge (where), for a bubble
# to show at.
PLACE = """([top, where]) => {
  let b = document.getElementById('test-bubble-at');
  if (!b) {
    b = button('Here', '');
    b.id = 'test-bubble-at';
    document.body.appendChild(b);
  }
  const w = document.documentElement.clientWidth;
  const left = { left: 0, middle: (w - 80) / 2, right: w - 80 }[where];
  Object.assign(b.style, { position: 'fixed', top: top + 'px', left: left + 'px', width: '80px', height: '32px', margin: '0' });
}"""

# Bubbles of each text, one right after the other, without a control,
# and the texts the live region of bubbles takes from then on, an empty
# one left out: window.__said.
SAY = """(texts) => {
  for (const text of texts) showBubble(text, undefined, { ms: 60000 });
  const live = document.getElementById('bubble-status');
  const said = [];
  window.__said = said;
  if (window.__sayWatch) window.__sayWatch.disconnect();
  window.__sayWatch = new MutationObserver(() => { if (live.textContent) said.push(live.textContent); });
  window.__sayWatch.observe(live, { childList: true, characterData: true, subtree: true });
}"""

# The time from a bubble shown at the test's button with showBubble's
# own time, 1.4 s, to its going, in ms; null where it is still there
# after 10 s.
SHOWN_FOR = """() => new Promise((done) => {
  const t0 = performance.now();
  showBubble('Short', document.getElementById('test-bubble-at'));
  const b = document.getElementById('bubble');
  const o = new MutationObserver(() => {
    if (b.getClientRects().length) return;
    o.disconnect();
    done(performance.now() - t0);
  });
  o.observe(b, { attributes: true });
  setTimeout(() => { o.disconnect(); done(null); }, 10000);
})"""

AT = "#test-bubble-at"
LONG = "A bubble with a longer text"


def test_bubble_at_a_control(page, case):
    """showBubble(text, ctrl): the bubble above the control's centre, as
    near to it as the window lets it at the window's left and right edges,
    inside the window, and below a control at the window's top; a newer
    bubble in the place of the one shown, the one element; gone at its
    time, 1.4 s by default, and as soon as its control has left the page.
    Fails on the page before: no showBubble."""
    try:
        page.evaluate(PLACE, [200, "middle"])
        page.evaluate(f"showBubble('Here it is', document.querySelector({AT!r}), {{ ms: 60000 }})")
        assert bubble_at(page, AT, "Here it is") == "above", f"above the control: {bubble_at(page, AT, 'Here it is')}"
        for where in ("left", "right"):
            page.evaluate(PLACE, [200, where])
            page.evaluate(f"showBubble({LONG!r}, document.querySelector({AT!r}), {{ ms: 60000 }})")
            assert wait_bubble_at(page, AT, LONG, 1) == "above", f"at the window's {where} edge: {bubble_at(page, AT, LONG)}"
            b = bubble(page)
            edge = b["left"] if where == "left" else b["width"] - b["right"]
            assert abs(edge - 8) <= 1, f"8 px inside the window's {where} edge, as near to the control as that lets it: {b}"
        page.evaluate(PLACE, [0, "middle"])
        page.evaluate(f"showBubble('Below', document.querySelector({AT!r}), {{ ms: 60000 }})")
        assert wait_bubble_at(page, AT, "Below", 1) == "below", f"below a control at the window's top: {bubble_at(page, AT, 'Below')}"
        page.evaluate(PLACE, [200, "middle"])
        page.evaluate(f"""() => {{
          showBubble('First', document.querySelector({AT!r}), {{ ms: 60000 }});
          showBubble('Second', document.querySelector({AT!r}), {{ ms: 60000 }});
        }}""")
        assert bubble_at(page, AT, "Second") == "above", f"the newer bubble in the place of the one shown: {bubble(page)}"
        assert page.evaluate("document.querySelectorAll('.bubble').length") == 1, "the one element"
        took = page.evaluate(SHOWN_FOR)
        assert took is not None and 1390 <= took <= 1400 + 1000 * scaled(1), f"gone at its time, 1.4 s: {took} ms"
        page.evaluate(f"showBubble('Gone with it', document.querySelector({AT!r}), {{ ms: 60000 }})")
        assert bubble(page), "shown"
        page.evaluate(f"document.querySelector({AT!r}).remove()")
        assert wait_until(page, "!document.getElementById('bubble').getClientRects().length", 1), "gone as soon as its control has left the page"
    finally:
        page.evaluate(f"() => {{ hideBubble(); document.querySelector({AT!r})?.remove(); }}")


def test_bubble_gone_at_a_scroll(page, case):
    """A bubble above a control of the page is gone when the page
    scrolls, which would leave it where the control was. Fails on the page
    before: no showBubble."""
    page.evaluate("""() => {
      const el = button('Here', '');
      el.id = 'test-bubble-at';
      const room = elem('div', '', el);
      room.id = 'test-bubble-room';
      room.style.height = '3000px';
      document.querySelector('main').prepend(room);
      window.scrollTo({ top: 0, behavior: 'instant' });
    }""")
    try:
        page.evaluate(f"showBubble('Here it is', document.querySelector({AT!r}), {{ ms: 60000 }})")
        assert wait_bubble_at(page, AT, "Here it is", 1) == "above", f"above the control: {bubble_at(page, AT, 'Here it is')}"
        page.evaluate("window.scrollBy({ top: 40, behavior: 'instant' })")
        assert wait_until(page, "!document.getElementById('bubble').getClientRects().length", 1), "gone at the scroll"
    finally:
        page.evaluate("""() => {
          hideBubble();
          document.getElementById('test-bubble-room')?.remove();
          window.scrollTo({ top: 0, behavior: 'instant' });
        }""")


def test_bubble_at_the_foot(page, case):
    """showBubble(text) without a control: the bubble at the window's
    bottom centre, 16 px above its foot (above the safe area, which the
    test's browser has none of), inside the window, over the page's toast.
    Fails on the page before: no showBubble."""
    try:
        page.evaluate("() => { showToast('A toast under the bubble', { ms: 60000 }); showBubble(%r, undefined, { ms: 60000 }); }" % LONG)
        b = bubble(page)
        assert b and b["text"] == LONG, f"the bubble shows: {b}"
        assert abs((b["left"] + b["right"]) / 2 - b["width"] / 2) <= 1, f"at the window's centre: {b}"
        assert abs(b["height"] - 16 - b["bottom"]) <= 1, f"16 px above the window's foot: {b}"
        assert b["left"] >= 0 and b["right"] <= b["width"], f"inside the window: {b}"
        toast = page.evaluate("Number(getComputedStyle(document.getElementById('toast')).zIndex)")
        assert b["z"] > toast, f"over the toast: {b['z']} > {toast}"
    finally:
        page.evaluate("() => { hideBubble(); hideToast(); }")


def test_bubble_said_once(page, case):
    """A bubble's text is said once, in the page's polite live region: the
    region takes it once and holds it alone, no other live region of the
    page has it, and the bubble itself is hidden from a screen reader; two
    bubbles one right after the other: the second alone is said; the same
    text again is said again. Fails on the page before: no showBubble."""
    try:
        page.evaluate(SAY, ["Said once"])
        assert wait_until(page, BUBBLE_STATUS + " === 'Said once'", 1), "said in the live region"
        live = page.evaluate("""() => {
          const l = document.getElementById('bubble-status');
          return { role: l.getAttribute('role'), live: l.getAttribute('aria-live'),
                   regions: [...document.querySelectorAll('[aria-live], [role=status]')].filter((r) => r.textContent.includes('Said once')).length };
        }""")
        assert live == {"role": "status", "live": "polite", "regions": 1}, f"a polite status region, the one that says it: {live}"
        assert bubble(page)["hidden"] == "true", "the bubble hidden from a screen reader, which hears the region"
        page.wait_for_timeout(300)
        assert page.evaluate("window.__said") == ["Said once"], f"once: {page.evaluate('window.__said')}"
        page.evaluate(SAY, ["First", "Second"])
        assert wait_until(page, BUBBLE_STATUS + " === 'Second'", 1), "the second said"
        page.wait_for_timeout(300)
        assert page.evaluate("window.__said") == ["Second"], f"the second alone, once: {page.evaluate('window.__said')}"
        page.evaluate(SAY, ["Second"])
        assert wait_until(page, "window.__said.length > 0", 1), "the same text said again"
        page.wait_for_timeout(300)
        assert page.evaluate("window.__said") == ["Second"], f"once: {page.evaluate('window.__said')}"
        assert page.evaluate(BUBBLE_STATUS) == "Second", "the region holds it alone"
    finally:
        page.evaluate("() => { hideBubble(); if (window.__sayWatch) window.__sayWatch.disconnect(); }")
