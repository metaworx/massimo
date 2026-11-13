# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""System settings → System Info (update.js loadSystemInfo), from GET
/api/v1/system: the server's version and what it was built with, its
start, the certificate, how it was installed, the computer and the
address the server sees this browser at; first as the test server
answers, in a terminal without TLS, then a stubbed Debian box."""
import json

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
    assert cell(page, "client") == "127.0.0.1", "the address the server sees this browser at"
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
