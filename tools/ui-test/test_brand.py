# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""Massimo's logo (components.js brandMarks, ui/brand/ from make ui-brand):
drawn over the word in the header, the full logo at full width and the
small mark alone under 600 px, in the colours of the mode chosen, on every
element marked for it; and the word, which a screen reader reads, in view
where the sprite never comes."""
import base64

from helpers import scroll_width, wait_until

ORANGE = (0xED, 0x6E, 0x21)  # the hands, in light and dark alike
LIGHT_M, DARK_M = "rgb(31, 99, 198)", "rgb(91, 149, 236)"


def colour_count(page, locator, rgb, tolerance=40):
    """How many pixels of the element's screenshot are near rgb, counted in
    the page: the screenshot drawn into a canvas."""
    shot = base64.b64encode(locator.screenshot()).decode()
    return page.evaluate("""async ([data, rgb, tol]) => {
      const img = new Image();
      img.src = 'data:image/png;base64,' + data;
      await img.decode();
      const c = document.createElement('canvas');
      c.width = img.width;
      c.height = img.height;
      const g = c.getContext('2d');
      g.drawImage(img, 0, 0);
      const d = g.getImageData(0, 0, c.width, c.height).data;
      let n = 0;
      for (let i = 0; i < d.length; i += 4) {
        if (Math.abs(d[i] - rgb[0]) + Math.abs(d[i + 1] - rgb[1]) + Math.abs(d[i + 2] - rgb[2]) <= tol) n++;
      }
      return n;
    }""", [shot, list(rgb), tolerance])


def visible_drawing(case):
    return ".top .brand-name .brand-mark" if case.width < 600 else ".top .brand-name .brand-logo"


def test_logo_over_the_word(page, case):
    """The header's logo: the sprite comes and the drawing shows its hands
    in orange; the full logo at full width, the small mark alone under
    600 px with the rest of the word for screen readers only; the word the
    heading's text; the M's colour that of the mode chosen; every element
    marked for it drawn, the letters' overshoot below the box not cut."""
    h1 = page.locator(".top h1.brand-name")
    assert h1.inner_text().replace("\n", "") == "Massimo", "the heading's text is the word"
    drawing = page.locator(visible_drawing(case))
    hidden = ".top .brand-name .brand-logo" if case.width < 600 else ".top .brand-name .brand-mark"
    assert drawing.is_visible() and page.locator(hidden).is_hidden(), "the logo at full width, the mark alone under 600 px"
    if case.width < 600:
        width = page.evaluate("document.querySelector('.top .brand-rest').getBoundingClientRect().width")
        assert width <= 1, "the rest of the word for screen readers only"
    assert wait_until(page, "document.querySelectorAll('.top .brand-name svg').length === 2", 5), "the drawings added"
    page.wait_for_timeout(300)
    assert colour_count(page, h1, ORANGE) > 4, "the drawing's hands, the sprite come"
    m = page.locator(visible_drawing(case) + " use.brand-m")
    assert m.evaluate("(u) => getComputedStyle(u).fill") == LIGHT_M, "the M's blue on a light page"
    page.evaluate("document.documentElement.dataset.theme = 'dark'")
    assert m.evaluate("(u) => getComputedStyle(u).fill") == DARK_M, "the lighter blue in dark mode"
    page.evaluate("document.documentElement.dataset.theme = 'light'")
    marked = page.evaluate("[...document.querySelectorAll('[data-brand]')].map((e) => e.querySelectorAll('svg').length)")
    assert len(marked) >= 3 and all(n == 2 for n in marked), f"every marked element drawn, logo and mark ({marked})"
    clipped = page.evaluate("[...document.querySelectorAll('[data-brand] svg')].filter((e) => getComputedStyle(e).overflow !== 'visible').length")
    assert clipped == 0, "the round letters' overshoot below the baseline painted, not cut at the tight box"
    # each cover in the background of its place, so that it hides the word
    covers = page.evaluate("""[['.top', '.top'], ['#login-dialog', '#login-dialog'], ['#setup', '#setup']].map(([place, bg]) => [
      getComputedStyle(document.querySelector(place + ' [data-brand] .brand-logo use.brand-cover')).fill,
      getComputedStyle(document.querySelector(bg)).backgroundColor])""")
    assert all(fill == bg for fill, bg in covers), f"the covers in their places' backgrounds ({covers})"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the logo"
    page.evaluate("delete document.documentElement.dataset.theme")


def test_word_without_the_sprite(page, case):
    """With a sprite that draws nothing, as where it never comes, the word
    stays in view: its letters where the drawing would be, and no orange of
    its hands. (A refused file would be a console error for the guard, one
    the browser does not name the file in.)"""
    page.route("**/ui/brand/brand.svg", lambda route: route.fulfill(
        status=200, content_type="image/svg+xml", body='<svg xmlns="http://www.w3.org/2000/svg"></svg>'))
    page.reload()
    page.wait_for_selector(".room", timeout=10000)
    h1 = page.locator(".top h1.brand-name")
    page.wait_for_timeout(300)
    assert colour_count(page, h1, ORANGE) == 0, "no drawing without the sprite"
    letters = colour_count(page, h1, (0x1F, 0x63, 0xC6), 60) + colour_count(page, h1, (0x1C, 0x1C, 0x1A), 60)
    assert letters > 20, "the word in view, its M in blue"
    page.unroute("**/ui/brand/brand.svg")
