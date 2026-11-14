# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → System Info (update.js loadSystemInfo), from GET
/api/v1/system: the server's version and what it was built with, its
start and clock, the certificate (or a proxy's), how it was installed,
the computer, its full name and the interfaces it listens on, and the
address the server sees this browser at; first as the test server
answers, in a terminal without TLS on the loopback, then stubbed boxes."""
import json
import re
import time

from helpers import scroll_width, show_view, wait_until

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
    address; the tile first among the cards, nothing overflowing."""
    show_view(page, "system")
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
    show_view(page, "system")
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
    show_view(page, "system")
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
    show_view(page, "system")
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
    show_view(page, "system")
    assert wait_until(page, "document.querySelector('#info-addrs .sys-note') && !document.getElementById('info-addrs').textContent.includes('*')", 5), \
        "the web interface everywhere: no asterisk"
    assert page.locator("#info-addrs .sys-note").inner_text() == ALL[case.lang], "the note says so instead"
    cert, served = tls_lines(page)[:2]
    assert cert.endswith(" " + PROXIED[case.lang]) and served == "https://massimo.example.net", \
        "a certificate not read yet, proxied all the same"
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
    show_view(page, "system")
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
    counted in the card's badge; none listed, a dash and no badge."""
    box = dict(BOX, cube_proxies=[{"serial": "KEQ0000100", "addr": "192.0.2.5:62910", "listening": True}, FAILING])
    page.route("**/api/v1/system", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(box)))
    show_view(page, "system")
    assert wait_until(page, "document.querySelectorAll('#info-proxies .sys-proxy').length === 2", 5), "two listeners listed"
    lines = page.locator("#info-proxies .sys-proxy")
    assert lines.nth(0).inner_text().split() == ["KEQ0000100", "192.0.2.5:62910", LISTENING[case.lang]], "the listening one"
    notice = lines.nth(1).locator(".notice")
    assert notice.count() == 1, "the one that does not listen, as a notice"
    text = notice.inner_text()
    assert "192.0.2.10:62910" in text and "cannot assign requested address" in text, "its address and its error"
    assert notice.locator(".notice-since").count() == 1, "since when"
    assert notice.get_attribute("title"), "the next try as its tooltip"
    assert page.locator("#sys-info .section-badges").inner_text().strip() == PROXIES_BADGE[case.lang], "the card's badge"
    assert scroll_width(page) <= case.width, "no horizontal scroll with a long error"
    show_view(page, "tiles")
    box["cube_proxies"] = []
    show_view(page, "system")
    assert wait_until(page, "document.getElementById('info-proxies').textContent === '–'", 5), "none listed: a dash"
    assert page.locator("#sys-info .section-badges .flag").count() == 0, "no badge"
    show_view(page, "tiles")
