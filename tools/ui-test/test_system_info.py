# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System info, the page of ☰ (ui/src/sysinfo/), from GET
/api/v1/system: the server's version and what it was built with, its
start and clock, the certificate (or a proxy's), how it was installed,
the computer, its full name and the interfaces it listens on, and the
address the server sees this browser at; first as the test server
answers, in a terminal without TLS on the loopback, then stubbed boxes;
and the fold Credits and licences, fetched once from /ui/credits.json
(internal/credits), an answer after the page was left or after a
sign-out changing nothing."""
import json
import os
import re
import time

import pytest

from conftest import TOKEN, URL
from helpers import OPERATOR_PASSWORD, frames, hold, owner_page, safe_route, scroll_width, show_view, sign_out, wait_held, wait_until
from test_login import sign_in
from test_sign_out_late import nothing_waits

OTHER = {"en": "Other: by hand, a container or another supervisor",
         "de": "Andere: von Hand, ein Container oder ein anderer Supervisor"}
APT = {"en": "Debian package (apt)", "de": "Debian-Paket (apt)"}
NO_TLS = {"en": "none, plain HTTP", "de": "keins, einfaches HTTP"}

BOX = {"version": "massimo v0.0.1-test (go1.26.0, linux/arm64)", "started_at": "2026-11-13T08:00:00+01:00",
       "tls": {"source": "self-signed", "names": ["massimo.local"], "not_after": "2027-11-13T08:00:00+01:00",
               "fingerprint": "00", "self_signed": True},
       "install": "apt", "os": "linux", "os_name": "Debian GNU/Linux 13 (trixie)", "arch": "arm64",
       "hostname": "massimo-box", "client_ip": "192.0.2.20"}


def cell(page, name):
    return page.locator("#info-" + name).inner_text().strip()


def tls_lines(page):
    """The certificate row's lines, where a proxy serves the page."""
    return page.evaluate("[...document.querySelectorAll('#info-tls div')].map((d) => d.textContent.trim())")


def test_system_info_of_the_test_server(page, case):
    """The test server: its version, a start, no certificate, "other" as
    the installation, the computer, and the loopback as this browser's
    address; the card shown, nothing overflowing."""
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-client').textContent !== '–'", 5), "System Info filled"
    version = page.evaluate("splitVersion(state.version)")
    assert cell(page, "version") == version["short"], "the server's version"
    assert cell(page, "started") not in ("", "–"), "when the server started"
    assert cell(page, "tls") == NO_TLS[case.lang], "no certificate without TLS"
    assert cell(page, "install") == OTHER[case.lang], "run from a terminal: other"
    arch = page.evaluate("api('GET', 'api/v1/system').then((v) => v.arch)")
    assert cell(page, "system").endswith(" · " + arch), "the operating system and the processor"
    assert cell(page, "host") not in ("", "–"), "the host's name"
    assert page.locator("#info-host strong").count() == 1, "the host's own name bold"
    assert cell(page, "client") == "127.0.0.1", "the address the server sees this browser at"
    assert cell(page, "now") not in ("", "–"), "the server's clock"
    # the API on 127.0.0.1 alone: the loopback's line, its address marked
    assert "127.0.0.1*" in cell(page, "addrs") and page.locator("#info-addrs .sys-note").count() == 1, \
        "the loopback the API listens on, marked and explained"
    assert page.locator("#sys-info").is_visible(), "the tile shown without expert mode"
    assert scroll_width(page) <= case.width, "no horizontal scroll with System Info"
    show_view(page, "tiles")


def test_system_info_of_a_debian_box(page, case):
    """A stubbed Debian box behind TLS: the version split, the certificate,
    the Debian package, the distribution with the processor; a package
    manager the page has no text for is named as the server names it."""
    box = dict(BOX)
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-client').textContent === '192.0.2.20'", 5), "System Info filled"
    assert cell(page, "version") == "v0.0.1-test", "the short version"
    assert cell(page, "built") == "go1.26.0, linux/arm64", "what it was built with"
    assert cell(page, "tls").startswith("self-signed · massimo.local · "), "the certificate's source, names and expiry"
    assert cell(page, "install") == APT[case.lang], "the Debian package"
    assert cell(page, "system") == "Debian GNU/Linux 13 (trixie) · arm64", "the distribution and the processor"
    assert cell(page, "host") == "massimo-box", "the host's name"
    assert scroll_width(page) <= case.width, "no horizontal scroll with a long certificate line"
    show_view(page, "tiles")
    box["install"] = "dnf"
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-install').textContent === 'dnf'", 5), \
        "a package manager without a text, as the server names it"
    page.unroute("**/api/v1/system")
    show_view(page, "tiles")


PROXIED = {"en": "(proxied)", "de": "(über Proxy)"}
UNTRUSTED = {"en": "not trusted", "de": "nicht vertrauenswürdig"}
AHEAD = {"en": "3 min ahead of this browser", "de": "3 min vor diesem Browser"}
ALL = {"en": "The web interface listens on all of them, port 8099.", "de": "Die Weboberfläche hört auf alle, Port 8099."}


def test_system_details_behind_a_proxy(page, case):
    """A stubbed box behind a trusted proxy: the server's clock three
    minutes ahead, ticking, in its zone; the full name with the host bold;
    an interface a line, the MAC first, the web interface's address marked
    and explained; the proxy's certificate, "(proxied)", with the address
    the proxy serves; then the web interface on every address."""
    now = time.strftime("%Y-%m-%dT%H:%M:%S+00:00", time.gmtime(time.time() + 180))
    box = dict(BOX, tls=None, now=now, time_zone="Europe/Zurich", fqdn="massimo-box.example.net", api_port=8099,
               interfaces=[{"name": "eth0", "mac": "02:00:00:00:00:01", "addresses": [{"ip": "192.0.2.8", "api": True}, {"ip": "192.0.2.31"}]},
                           {"name": "wg0", "addresses": [{"ip": "10.8.0.2"}]}],
               proxy={"proto": "https", "host": "massimo.example.net", "port": 443,
                      "certificate": {"source": "proxy", "names": ["massimo.example.net"], "not_after": "2027-02-10T12:00:00Z",
                                      "fingerprint": "00", "self_signed": False, "issuer": "Example CA", "trusted": False,
                                      "checked_at": "2026-11-13T09:50:02+01:00"}})
    box.pop("tls")
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-client').textContent === '192.0.2.20'", 5), "System Info filled"
    clock = cell(page, "now")
    assert "Europe/Zurich" in clock and AHEAD[case.lang] in clock, f"the server's clock in its zone, ahead ({clock!r})"
    page.wait_for_timeout(1200)
    assert cell(page, "now") != clock, "the clock ticks"
    assert page.locator("#info-host strong").inner_text() == "massimo-box" and cell(page, "host") == "massimo-box.example.net", \
        "the full name, the host bold"
    lines = page.locator("#info-addrs .sys-iface")
    assert lines.count() == 2, "an interface a line"
    assert lines.nth(0).inner_text().split() == ["eth0", "02:00:00:00:00:01", "192.0.2.8*,", "192.0.2.31"], \
        "the name, the MAC, then the addresses, the API's marked"
    assert lines.nth(1).inner_text().split() == ["wg0", "10.8.0.2"], "the name and the addresses where there is no MAC"
    # the start as the clock reads it: to the second, the zone, the uptime
    # after it; both times in this browser's zone on hover
    started = cell(page, "started")
    assert re.search(r"\d\d:\d\d:\d\d Europe/Zurich", started) and started.split("Europe/Zurich", 1)[1].strip(), \
        f"the start to the second, its zone, then the uptime ({started!r})"
    zone = page.evaluate("Intl.DateTimeFormat().resolvedOptions().timeZone")
    for name in ("started", "now"):
        title = page.locator("#info-" + name).get_attribute("title") or ""
        assert zone in title and re.search(r"\d\d:\d\d:\d\d", title), f"the {name} time in this browser's zone on hover ({title!r})"
    assert "8099" in page.locator("#info-addrs .sys-note").inner_text(), "the asterisk explained, with the port"
    cert, served = tls_lines(page)[:2]
    parts = cert.removesuffix(" " + PROXIED[case.lang]).split(" · ")
    assert parts[0] == "massimo.example.net" and parts[2].endswith("Example CA") and parts[3] == UNTRUSTED[case.lang] \
        and cert.endswith(" " + PROXIED[case.lang]), f"the proxy's certificate: names, expiry, issuer, not trusted, proxied ({cert!r})"
    assert served == "https://massimo.example.net", "the address the proxy serves"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the details"
    show_view(page, "tiles")
    box["interfaces"] = [{"name": "eth0", "mac": "02:00:00:00:00:01", "addresses": [{"ip": "192.0.2.8", "api": True}, {"ip": "192.0.2.31", "api": True}]}]
    box["proxy"] = {"proto": "https", "host": "massimo.example.net", "port": 443}
    show_view(page, "info")
    assert wait_until(page, "document.querySelector('#info-addrs .sys-note') && !document.getElementById('info-addrs').textContent.includes('*')", 5), \
        "the web interface everywhere: no asterisk"
    assert page.locator("#info-addrs .sys-note").inner_text() == ALL[case.lang], "the note says so instead"
    cert, served = tls_lines(page)[:2]
    assert cert.endswith(" " + PROXIED[case.lang]) and served == "https://massimo.example.net", \
        "a certificate not read yet, proxied all the same"
    page.unroute("**/api/v1/system")
    show_view(page, "tiles")


PENDING = {"en": "being read…", "de": "wird gelesen…"}
UNREAD = {"en": "could not be read", "de": "konnte nicht gelesen werden"}


def run_reads(page, asked, n, seconds=5):
    """The page's clock moved on, one reading's pace (2 s) at a time,
    until asked has n entries."""
    end = time.time() + seconds
    while len(asked) < n and time.time() < end:
        page.clock.run_for(2000)
        page.wait_for_timeout(50)
    return len(asked) >= n


@pytest.mark.clock
def test_proxy_certificate_followed(page, case):
    """A proxy's certificate the server has not read yet: "being read…",
    asked for again every 2 s until it is read, then shown; one that stays
    unread, "could not be read" after the last of 15 tries. The address
    the proxy serves is a link to a new window and copies by its icon."""
    page.context.grant_permissions(["clipboard-read", "clipboard-write"], origin=page.evaluate("location.origin"))
    box = dict(BOX, now=time.strftime("%Y-%m-%dT%H:%M:%S+00:00", time.gmtime()), time_zone="UTC", interfaces=[],
               proxy={"proto": "https", "host": "massimo.example.net", "port": 443})
    box.pop("tls")
    read = {"source": "proxy", "names": ["massimo.example.net"], "not_after": "2027-02-10T12:00:00Z", "fingerprint": "00",
            "self_signed": False, "issuer": "Example CA", "trusted": True, "checked_at": "2026-11-13T09:50:02+01:00"}
    asked = []

    def answer(route):
        asked.append(1)
        body = dict(box, proxy=dict(box["proxy"], certificate=read)) if len(asked) >= 3 else box
        route.fulfill(status=200, content_type="application/json", body=json.dumps(body))
    page.route("**/api/v1/system", answer)
    try:
        show_view(page, "info")
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes(%r)" % PENDING[case.lang], 5), \
            "being read… first"
        assert run_reads(page, asked, 3), f"asked for again ({len(asked)})"
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes('Example CA')", 5), \
            "asked for again until read, then shown"
        page.clock.run_for(4000)
        assert len(asked) == 3, f"no more asking once read ({len(asked)})"
        link = page.locator("#info-tls a")
        assert link.get_attribute("href") == "https://massimo.example.net" and link.get_attribute("target") == "_blank", \
            "the address a link to a new window"
        page.evaluate("navigator.clipboard.writeText('')")
        page.locator("#info-tls .copy-icon").click()
        assert wait_until(page, "navigator.clipboard.readText().then((t) => t === 'https://massimo.example.net')", 3), \
            "and copies by its icon"
        show_view(page, "tiles")
        page.unroute("**/api/v1/system")
        asked.clear()
        page.route("**/api/v1/system", lambda route: (asked.append(1), route.fulfill(status=200, content_type="application/json", body=json.dumps(box))))
        show_view(page, "info")
        assert run_reads(page, asked, 16, 20), f"the first read and 15 tries ({len(asked)})"
        assert wait_until(page, "document.getElementById('info-tls').textContent.includes(%r)" % UNREAD[case.lang], 5), \
            "could not be read after the last try"
        page.clock.run_for(4000)
        assert len(asked) == 16, f"no more asking after the last try ({len(asked)})"
    finally:
        page.unroute("**/api/v1/system")
        show_view(page, "tiles")


NO_SCHEME = {"en": "The proxy sends no X-Forwarded-Proto", "de": "Der Proxy sendet kein X-Forwarded-Proto"}


def test_proxy_without_a_scheme(page, case):
    """A trusted proxy that names no scheme, as the Raspberry Pi's: the row
    takes the page's own scheme and address, which the browser knows, and
    names the header the proxy does not send."""
    box = dict(BOX, now=time.strftime("%Y-%m-%dT%H:%M:%S+00:00", time.gmtime()), time_zone="UTC", interfaces=[],
               proxy={"host": "massimo.example.net"})
    box.pop("tls")
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-client').textContent === '192.0.2.20'", 5), "System Info filled"
    lines = tls_lines(page)
    origin = page.evaluate("location.protocol + '//' + location.host")
    assert lines[0].endswith(" " + PROXIED[case.lang]) and lines[1] == origin, f"the page's own scheme and address ({lines})"
    assert lines[2].startswith(NO_SCHEME[case.lang]), f"the header the proxy does not send ({lines})"
    page.unroute("**/api/v1/system")
    show_view(page, "tiles")


FAILING = {"serial": "KEQ0000101", "addr": "192.0.2.10:62910", "listening": False,
           "error": "listen tcp 192.0.2.10:62910: bind: cannot assign requested address",
           "since": "2026-11-13T08:00:00+01:00", "next_try": "2026-11-13T08:02:30+01:00", "tries": 9}
PROXIES_BADGE = {"en": "1 proxy not listening", "de": "1 Proxy horcht nicht"}
LISTENING = {"en": "listening", "de": "horcht"}


def test_system_info_cube_proxies(page, case):
    """The Cube proxies' listeners, a line each: one listening, one that
    does not with its error, since when and the next try as its tooltip,
    counted in the page's badge, in its row; none listed, a dash and no
    badge."""
    box = dict(BOX, cube_proxies=[{"serial": "KEQ0000100", "addr": "192.0.2.5:62910", "listening": True}, FAILING])
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    show_view(page, "info")
    assert wait_until(page, "document.querySelectorAll('#info-proxies .sys-proxy').length === 2", 5), "two listeners listed"
    lines = page.locator("#info-proxies .sys-proxy")
    assert lines.nth(0).inner_text().split() == ["KEQ0000100", "192.0.2.5:62910", LISTENING[case.lang]], "the listening one"
    notice = lines.nth(1).locator(".notice")
    assert notice.count() == 1, "the one that does not listen, as a notice"
    text = notice.inner_text()
    assert "192.0.2.10:62910" in text and "cannot assign requested address" in text, "its address and its error"
    assert notice.locator(".notice-since").count() == 1, "since when"
    assert notice.get_attribute("title"), "the next try as its tooltip"
    assert page.locator("#info-page .section-badges").inner_text().strip() == PROXIES_BADGE[case.lang], "the page's badge"
    assert scroll_width(page) <= case.width, "no horizontal scroll with a long error"
    show_view(page, "tiles")
    box["cube_proxies"] = []
    show_view(page, "info")
    assert wait_until(page, "document.getElementById('info-proxies').textContent === '–'", 5), "none listed: a dash"
    assert page.locator("#info-page .section-badges .flag").count() == 0, "no badge"
    show_view(page, "tiles")


CREDITS = "**/ui/credits.json"
CREDITS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "internal", "credits", "credits.json")
GO_MOD = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "go.mod")
FAILED = {"en": "The credits could not be loaded: broken", "de": "Die Mitwirkenden konnten nicht geladen werden: broken"}
MODULE = {"en": "Go module", "de": "Go-Modul"}
ELSEWHERE = {"en": "Go module, in another platform's build", "de": "Go-Modul, im Build einer anderen Plattform"}
AMONG = {"en": " and the crates they are built with; their licence texts, collected for Massimo from the crates' archives, are among its files.",
         "de": " und die Crates, mit denen diese gebaut sind; deren Lizenztexte, für Massimo aus den Archiven der Crates gesammelt, stehen unter seinen Dateien."}


def credit_requests(page):
    """The page's requests of the credits, from now on."""
    seen = []
    page.on("request", lambda r: seen.append(r.url) if r.url.split("?")[0].endswith("/ui/credits.json") else None)
    return seen


def toggle_credits(page):
    """Opens or closes the fold Credits and licences, as a user does."""
    page.locator("#credits-info > summary").click()


def entry(page, name):
    """An entry's fold in the credits, by its name."""
    return page.locator(f"#credits-info .credit[data-name='{name}']")


def held_first(page, held):
    """Holds the first request of the credits until the test lets it go,
    and lets every later one through to the server."""
    def handle(route):
        if held:
            route.continue_()
        else:
            held.append(route)
    safe_route(page, CREDITS, handle)


def listed(name):
    """An entry of the committed credits.json, by its name."""
    with open(CREDITS_FILE, encoding="utf-8") as f:
        return next(e for e in json.load(f)["entries"] if e["name"] == name)


def required(module):
    """The version go.mod requires of a module, the one the server is
    built with."""
    with open(GO_MOD, encoding="utf-8") as f:
        return next(line.split()[1] for line in f if line.split()[:1] == [module])


def test_credits_fetched_once(page, case):
    """System info's fold Credits and licences: nothing fetched before it
    opens; at its first opening the list, Massimo first, Preact with its
    version and licence as credits.json has them, its text in a fold of
    its own, as text, in a box that scrolls and that its file's name
    labels; a Go module, golang.org/x/net, at the version go.mod requires,
    which credits.json leaves out and the server fills in as it was built
    with; an entry whose files are under other licences too says so;
    the OPAQUE client, MIT and others, says what it includes, the crates
    those are built with, and that their licence texts, collected for
    Massimo, are among its files, in the page's language,
    and its files are its own licence and its crates' texts, which
    scroll in their box; closed and opened again, and System info left
    and shown again, nothing fetched again. Fails on the page before
    the crates' texts: the OPAQUE client is MIT alone, its one file
    opaque.LICENSE, and their texts not reproduced; and on a server that
    fills in no module's version: golang.org/x/net without one."""
    seen = credit_requests(page)
    show_view(page, "info")
    page.locator("#credits-info > summary").wait_for()
    frames(page)
    assert not seen, "nothing fetched before the fold opens"
    toggle_credits(page)
    preact = entry(page, "preact")
    preact.wait_for()
    assert len(seen) == 1, f"fetched once as it opens: {seen}"
    names = page.evaluate("[...document.querySelectorAll('#credits-info .credit')].map((e) => e.dataset.name)")
    assert names[:2] == ["Massimo", "Go"], f"Massimo first, then Go: {names[:3]}"
    assert {"golang.org/x/crypto", "modernc.org/sqlite", "@preact/signals", "@tabler/icons", "@serenity-kit/opaque"} <= set(names), \
        "the modules, the vendored packages, the icons and the OPAQUE client"
    assert str(len(names)) in page.locator("#credits-info-sum").inner_text(), "the number of entries in the fold's head"
    off = page.evaluate("""[...document.querySelectorAll('#credits-info .credit-head')].filter((h) => {
      const beside = h.parentElement.getBoundingClientRect().left + 4;
      return [...h.children].some((c) => [...c.getClientRects()].some((r) => r.left < beside));
    }).map((h) => h.textContent)""")
    assert not off, f"a head that wraps goes on under the name, not under the chevron: {off}"
    broken = page.evaluate("[...document.querySelectorAll('#credits-info .credit-id')].filter((e) => e.getClientRects().length > 1).map((e) => e.textContent)")
    assert not broken, f"a licence's identifier does not break inside itself: {broken}"
    want = listed("preact")
    head = preact.locator("summary").inner_text().split()
    assert head == ["preact", want["version"], want["licence"]], f"Preact's name, version and licence ({head})"
    net = entry(page, "golang.org/x/net").locator("summary").inner_text().split()
    assert net == ["golang.org/x/net", required("golang.org/x/net"), listed("golang.org/x/net")["licence"]], \
        f"a module at the version the server was built with ({net})"
    assert preact.locator("pre").count() == 0, "its text not drawn while its fold is closed"
    preact.locator("summary").click()
    text = preact.locator("pre.credit-text")
    text.first.wait_for()
    assert text.count() == 1 and "Permission is hereby granted, free of charge" in text.inner_text(), "Preact's licence text"
    assert page.evaluate("document.querySelector(\"#credits-info .credit[data-name='preact'] pre\").children.length") == 0, \
        "the text as text"
    assert "third_party/preact/preact" in preact.locator(".credit-origin").inner_text(), "where the repository holds it"
    assert text.evaluate("(e) => document.getElementById(e.getAttribute('aria-labelledby')).textContent") == "LICENSE · MIT", \
        "the box named by its file and that file's licence"
    sqlite = entry(page, "modernc.org/sqlite")
    mixed = page.evaluate("t('credits.mixed', { licence: 'BSD-3-Clause' })")
    assert sqlite.locator(".credit-licence").inner_text() == mixed, "an entry whose files are under other licences too says so"
    opaque = entry(page, "@serenity-kit/opaque")
    includes = opaque.locator(".credit-includes").text_content()
    assert includes == page.evaluate("(what) => t('credits.includes', { what })", listed("@serenity-kit/opaque")["includes"]) \
        and "opaque-ke 4.0.1" in includes and includes.endswith(AMONG[case.lang]), \
        f"what it includes, whose texts are among its files, in the page's words ({includes!r})"
    assert opaque.locator(".credit-licence").inner_text() == page.evaluate("t('credits.mixed', { licence: 'MIT' })"), \
        "the OPAQUE client: MIT, its crates' texts under other licences"
    opaque.locator("summary").click()
    opaque.locator(".credit-file-name").nth(1).wait_for()
    files = opaque.locator(".credit-file-name").evaluate_all("(es) => es.map((e) => e.textContent)")
    assert files == ["opaque.LICENSE · MIT", "opaque.LICENSE-3RD-PARTY · " + page.evaluate("t('credits.seeText')")], \
        f"its own licence, then its crates' texts, known by no one licence ({files})"
    crates = opaque.locator("pre.credit-text").nth(1)
    assert "\n==> opaque-ke 4.0.1 (Apache-2.0 OR MIT) <==\n" in crates.text_content(), "the crates' licence texts, opaque-ke's among them"
    assert crates.evaluate("(e) => e.scrollHeight > e.clientHeight && getComputedStyle(e).overflowY === 'auto'"), \
        "the crates' texts scroll inside their box"
    assert crates.evaluate("(e) => e.scrollWidth <= e.clientWidth"), "their lines wrap inside the box"
    massimo = entry(page, "Massimo")
    massimo.locator("summary").click()
    box = massimo.locator("pre.credit-text")
    assert "GNU AFFERO GENERAL PUBLIC LICENSE" in box.inner_text(), "Massimo's own licence"
    assert box.evaluate("(e) => e.scrollHeight > e.clientHeight && getComputedStyle(e).overflowY === 'auto'"), \
        "a long text scrolls inside its box"
    assert box.evaluate("(e) => e.scrollWidth <= e.clientWidth"), "its lines wrap inside the box"
    assert scroll_width(page) <= case.width, "no horizontal scroll with the licences open"
    toggle_credits(page)
    toggle_credits(page)
    show_view(page, "tiles")
    show_view(page, "info")
    assert entry(page, "preact").locator("pre").first.is_visible(), "the fold and Preact's still open"
    frames(page)
    assert len(seen) == 1, f"fetched once: {seen}"
    show_view(page, "tiles")


def test_credits_module_of_another_build(page, case, reads):
    """A Go module the served list gives no version, as a server built
    for linux gives the modules only the windows and darwin builds link:
    its head without a version, its fold opened says where it comes from,
    a module in another platform's build; a module with its version stays
    a Go module. Fails on the page before: it read "Go module" alone."""
    def unversioned(body):
        for e in body["entries"]:
            if e["name"] == "golang.org/x/net":
                e.pop("version", None)
        return body
    reads.change(CREDITS, unversioned)
    show_view(page, "info")
    toggle_credits(page)
    net = entry(page, "golang.org/x/net")
    net.wait_for()
    head = net.locator("summary").inner_text().split()
    assert head == ["golang.org/x/net", listed("golang.org/x/net")["licence"]], f"no version in its head ({head})"
    for name in ("golang.org/x/net", "golang.org/x/crypto"):
        entry(page, name).locator("summary").click()
        entry(page, name).locator("pre.credit-text").first.wait_for()
    assert net.locator(".credit-origin").inner_text() == ELSEWHERE[case.lang], "a module of another platform's build"
    assert entry(page, "golang.org/x/crypto").locator(".credit-origin").inner_text() == MODULE[case.lang], \
        "a module with its version: a Go module"
    show_view(page, "tiles")


def test_credits_failed_and_tried_again(page, case, watch):
    """A fetch of the credits that fails says why in the fold, with Try
    again; a try that fails again gives the focus back to Try again,
    which lost it while it was off; one that goes through gives it to the
    first entry."""
    watch.expected.append(("/ui/credits.json", 500))
    failing = [True]
    tries = []

    def handle(route):
        tries.append(1)
        if failing[0]:
            route.fulfill(status=500, content_type="text/plain", body="broken")
        else:
            route.continue_()
    safe_route(page, CREDITS, handle)
    show_view(page, "info")
    toggle_credits(page)
    retry = page.locator("#credits-retry")
    retry.wait_for()
    assert page.locator("#credits-info-body .bad").inner_text() == FAILED[case.lang], "why the fetch failed"
    assert page.locator("#credits-info .credit").count() == 0, "no list"
    retry.focus()
    page.keyboard.press("Enter")
    assert wait_until(page, "!document.getElementById('credits-retry').disabled", 5) and len(tries) == 2, "the try failed again"
    frames(page)
    assert page.evaluate("document.activeElement === document.getElementById('credits-retry')"), \
        "Try again has the focus back after a try that failed"
    failing[0] = False
    page.keyboard.press("Enter")
    entry(page, "Massimo").wait_for()
    assert page.evaluate("document.activeElement === document.querySelector('#credits-info .credit > summary')"), \
        "the first entry takes the focus from Try again"
    assert page.locator("#credits-retry").count() == 0, "Try again gone"
    show_view(page, "tiles")


def test_credits_answer_after_leaving(page, case):
    """The credits' answer that comes after the page has left System info
    changes nothing; shown again with the fold open, the page fetches
    them then."""
    held = []
    held_first(page, held)
    seen = credit_requests(page)
    show_view(page, "info")
    toggle_credits(page)
    assert wait_held(page, held), "the fetch is on its way"
    show_view(page, "tiles")
    with page.expect_response(lambda r: r.url.endswith("/ui/credits.json")) as answered:
        held[0].continue_()
    answered.value.finished()
    frames(page)
    assert page.locator("#credits-info .credit").count() == 0, "the late answer drew nothing"
    show_view(page, "info")
    entry(page, "Massimo").wait_for()
    assert len(seen) == 2, f"fetched again as System info is shown with the fold open: {seen}"
    show_view(page, "tiles")


def test_credits_answer_after_sign_out(new_context, case, watch, guard):
    """The credits' answer of the user before comes while the next user's
    System info, the fold still open, waits for a fetch of its own: it
    changes nothing, and the fetch under way stays the next user's (the
    fold opened again starts none beside it); its answer draws the list.
    Only the session's number tells the two answers apart here."""
    page = owner_page(new_context, guard, watch, URL, TOKEN)
    held, sent = [], []
    hold(page, CREDITS, "GET", held, sent)
    show_view(page, "info")
    toggle_credits(page)
    assert wait_held(page, held), "the owner's fetch is on its way"
    sign_out(page)
    sign_in(page, "operator", OPERATOR_PASSWORD)
    assert page.evaluate("state.me.user.name") == "operator", "the next user is in"
    show_view(page, "info")
    assert wait_held(page, held, n=2), "the operator's System info fetches the credits, the fold still open"
    with page.expect_response(lambda r: r.url.endswith("/ui/credits.json")) as answered:
        held[0].continue_()
    answered.value.finished()
    frames(page)
    assert page.locator("#credits-info .credit").count() == 0, "the answer of the user before drew nothing"
    toggle_credits(page)
    toggle_credits(page)
    frames(page)
    assert len(sent) == 2, f"the operator's fetch still under way, none beside it: {sent}"
    with page.expect_response(lambda r: r.url.endswith("/ui/credits.json")) as answered:
        held[1].continue_()
    answered.value.finished()
    entry(page, "Massimo").wait_for()
    nothing_waits(page)
    show_view(page, "tiles")
