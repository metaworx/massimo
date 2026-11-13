# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
"""👤 → Notifications (docs/ui.md, "Notifications"): the rules of the
token's user, the owner, with their defaults and the changes kept in its
preference notify; this browser turned on and off against a fake
pushManager, which conftest.py's page takes from an init script and a
reload, since a headless browser has no push service; the toast of a
notification from the stream and from the service worker, shown once;
a guest's page, which says why it has none, and a browser without push.
axe-core finds no violation on the page in each of its states.

The subscriptions go to an endpoint on the loopback's discard port, so
that a notification the server decides meanwhile reaches no network;
each test removes what it made, the four cases sharing the server, and
the page fixture removes the owner's rules before every page."""
import json
import time

from axe_playwright_python.sync_playwright import Axe

from conftest import api_call
from helpers import show_view, wait_until

# The fake push of a browser: a service worker container whose register
# and ready give a registration with a pushManager, whose subscription
# has a real P-256 key made by WebCrypto, as the server checks it; and
# Notification's permission, granted when asked. What the page did is
# kept on window for the test.
FAKE_PUSH = """(() => {
  const endpoint = "http://127.0.0.1:9/push/ui-test/" + Math.random().toString(36).slice(2);
  const b64 = (buf) => btoa(String.fromCharCode(...new Uint8Array(buf))).replace(/\\+/g, "-").replace(/\\//g, "_").replace(/=+$/, "");
  let current = null;
  let registered = false;
  window.__push = { registered: "", messages: [], unsubscribed: false };
  const reg = {
    pushManager: {
      getSubscription: async () => current,
      subscribe: async (opts) => {
        const pair = await crypto.subtle.generateKey({ name: "ECDH", namedCurve: "P-256" }, true, ["deriveBits"]);
        const raw = await crypto.subtle.exportKey("raw", pair.publicKey);
        const auth = crypto.getRandomValues(new Uint8Array(16));
        current = {
          endpoint,
          options: { applicationServerKey: opts.applicationServerKey.buffer },
          toJSON: () => ({ endpoint, expirationTime: null, keys: { p256dh: b64(raw), auth: b64(auth) } }),
          unsubscribe: async () => { current = null; window.__push.unsubscribed = true; return true; },
        };
        return current;
      },
    },
    active: { postMessage: (m) => window.__push.messages.push(m) },
  };
  const sw = new EventTarget();
  sw.register = async (url) => { window.__push.registered = url; registered = true; return reg; };
  sw.getRegistration = async () => (registered ? reg : undefined);
  Object.defineProperty(sw, "ready", { get: () => Promise.resolve(reg) });
  Object.defineProperty(navigator, "serviceWorker", { value: sw, configurable: true });
  const N = function () {};
  N.permission = "default";
  N.requestPermission = async () => (N.permission = "granted");
  window.Notification = N;
})();"""

# A guest signed in, for a test that stubs /me: no capability
# preferences, so no notifications.
GUEST = {"mode": "users", "user": {"name": "guest1", "role": "guest", "has_password": True},
         "role": {"key": "guest"}, "capabilities": ["read"], "token": {"kind": "session"}}


def assert_axe(page, where):
    """axe-core finds no violation on the page, taken to its top first:
    Notifications is a card of the personal page, which opens scrolled to
    it, and a scrolled header on a phone has no room for the brand, the
    page's level-one heading (style.css, body.scrolled)."""
    page.evaluate("window.scrollTo(0, 0)")
    wait_until(page, "window.scrollY === 0 && !document.body.classList.contains('scrolled')", 3)
    results = Axe().run(page)
    assert results.violations_count == 0, f"axe on {where}:\n{results.generate_report()}"


def t(page, key, params=None):
    """The page's text of a key, in the case's language."""
    return page.evaluate("([k, p]) => t(k, p)", [key, params or {}])


def reload_with(page, script=None):
    """The page loaded again, with an init script before its own; its
    room tiles there, shown or not, as the address has it."""
    if script:
        page.context.add_init_script(script)
    page.reload()
    page.wait_for_selector(".room", state="attached", timeout=10000)
    assert wait_until(page, "state.eventsOpen", 10), "the event stream opens again"


def remove_test_subscriptions():
    """The owner's subscriptions a test made: the server keeps them, and
    the cases share it."""
    for s in api_call("GET", "api/v1/me/push") or []:
        if "/push/ui-test/" in s["endpoint"]:
            api_call("DELETE", f"api/v1/me/push/{s['id']}")


def server_rules():
    """The owner's notify preference, None for none."""
    try:
        return api_call("GET", "api/v1/me/preferences/notify")
    except Exception:  # 404: none
        return None


def wait_rules(page, check, seconds=5):
    """Waits until the server's rules satisfy check, the page's writes
    waiting a second for the next."""
    for _ in range(seconds * 5):
        rules = server_rules()
        if rules is not None and check(rules):
            return rules
        page.wait_for_timeout(200)
    return server_rules()


def rule_row(page, kind):
    return page.locator(f"#notify-rules .notify-row[data-kind={kind}]")


def test_rules_kept_on_the_server(page, case):
    """The rules show their defaults, a delay and a switch changed go to
    the preference notify, a delay out of bounds is refused, and the
    page reads them back."""
    show_view(page, "notify")
    rows = page.locator("#notify-rules .notify-row")
    assert rows.count() == 10, "a row per kind"
    win = rule_row(page, "window_open")
    assert win.locator("input[type=checkbox]").is_checked(), "a window open is on by default"
    assert win.locator("input[type=number]").input_value() == "30"
    assert rule_row(page, "frost").locator("input[type=number]").input_value() == "8"
    stale = rule_row(page, "device_stale")
    assert not stale.locator("input[type=checkbox]").is_checked(), "a stale device is off by default"
    assert win.locator(".help-btn").count() == 1, "each kind has its help"
    assert_axe(page, "the rules")

    win.locator("input[type=number]").fill("45")
    win.locator("input[type=number]").dispatch_event("change")
    stale.locator(".switch").click()
    rules = wait_rules(page, lambda r: r.get("window_open", {}).get("after_min") == 45 and r.get("device_stale", {}).get("on"))
    assert rules["window_open"] == {"on": True, "after_min": 45} and rules["device_stale"]["on"] is True, rules

    frost = rule_row(page, "frost")
    frost.locator(".switch").click()
    assert frost.locator("input[type=number]").is_disabled(), "the threshold is off with its kind"
    win.locator("input[type=number]").fill("2000")
    win.locator("input[type=number]").dispatch_event("change")
    msg = page.locator("#notify-msg")
    msg.wait_for(state="visible")
    assert msg.inner_text() == t(page, "notify.badAfter", {"min": 1, "max": 1440})
    rules = wait_rules(page, lambda r: r.get("frost", {}).get("on") is False)
    assert rules["window_open"]["after_min"] == 45, "a delay out of bounds is not sent"

    reload_with(page)
    show_view(page, "notify")
    assert rule_row(page, "window_open").locator("input[type=number]").input_value() == "45", "the page reads the rules back"
    assert not rule_row(page, "frost").locator("input[type=checkbox]").is_checked()
    api_call("DELETE", "api/v1/me/preferences/notify")


def test_this_browser_on_and_off(page, case):
    """Turning this browser on registers the service worker, asks the
    permission, subscribes with the server's key and posts the
    subscription with its name; the list marks it, and the worker knows
    the language. Turning it off removes it at the server and in the
    browser."""
    try:
        reload_with(page, FAKE_PUSH)
        show_view(page, "notify")
        state = page.locator("#notify-device-state")
        assert state.inner_text() == t(page, "notify.isOff")
        assert page.locator("#notify-devices").inner_text() == t(page, "notify.noDevices")
        assert_axe(page, "a browser not notified")
        label = f"ui-test {case.width}-{case.lang}"
        page.locator("#notify-label").fill(label)
        page.locator("#notify-on").click()
        page.locator("#notify-off").wait_for(state="visible")
        assert state.inner_text() == t(page, "notify.isOn")
        assert page.evaluate("window.__push.registered") == "sw.js", "the worker beside the page"
        assert {"type": "lang", "lang": case.lang} in page.evaluate("window.__push.messages"), "the worker is told the language"
        subs = [s for s in api_call("GET", "api/v1/me/push") if "/push/ui-test/" in s["endpoint"]]
        assert len(subs) == 1 and subs[0]["label"] == label, subs
        device = page.locator(".notify-device", has_text=label)
        assert device.locator(".flag").inner_text() == t(page, "notify.thisBrowser")
        assert page.locator("#notify-label-row").is_hidden(), "the name is asked before turning on"
        assert_axe(page, "this browser notified")

        page.locator("#notify-off").click()
        page.locator("#notify-on").wait_for(state="visible")
        assert state.inner_text() == t(page, "notify.isOff")
        assert page.evaluate("window.__push.unsubscribed"), "the browser's subscription ends"
        assert not [s for s in api_call("GET", "api/v1/me/push") if "/push/ui-test/" in s["endpoint"]], "and the server's"
    finally:
        remove_test_subscriptions()


def test_another_device_removed(page, case):
    """A device of the user's other than this browser is listed with its
    name and removed by its Remove."""
    try:
        reload_with(page, FAKE_PUSH)
        made = page.evaluate("""async () => {
          const pair = await crypto.subtle.generateKey({ name: "ECDH", namedCurve: "P-256" }, true, ["deriveBits"]);
          const raw = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
          const b64 = (b) => btoa(String.fromCharCode(...b)).replace(/\\+/g, "-").replace(/\\//g, "_").replace(/=+$/, "");
          return { p256dh: b64(raw), auth: b64(crypto.getRandomValues(new Uint8Array(16))) };
        }""")
        label = f"ui-test phone {case.width}-{case.lang}"
        api_call("POST", "api/v1/me/push", {"endpoint": f"http://127.0.0.1:9/push/ui-test/phone-{time.time_ns()}", "keys": made, "label": label})
        show_view(page, "notify")
        device = page.locator(".notify-device", has_text=label)
        device.wait_for(state="visible")
        assert device.locator(".flag").count() == 0, "not this browser"
        assert t(page, "notify.added", {"when": ""}).strip() in device.inner_text()
        device.locator("button").click()
        device.wait_for(state="detached")
        assert not [s for s in api_call("GET", "api/v1/me/push") if s["label"] == label]
    finally:
        remove_test_subscriptions()


def notification(minutes, when):
    return {"kind": "window_open", "cube": "KEQ0000100", "device": "0a0102", "serial": "MEQ0000001",
            "device_name": "Window", "room": 3, "room_name": "Bath", "values": {"minutes": minutes},
            "time": when, "url": "/"}


def test_toast(page, case):
    """A notification on the stream shows a toast in the page's
    language, once; the service worker's copy of one shows another;
    the toast closes."""
    n = notification(30, "2026-11-09T06:42:00Z")
    send = """(n) => state.events.dispatchEvent(new MessageEvent("notification", { data: JSON.stringify({ kind: "notification", notification: n }) }))"""
    page.evaluate(send, n)
    toast = page.locator("#notify-toast")
    toast.wait_for(state="visible")
    assert page.locator("#notify-toast-title").inner_text() == t(page, "notify.window_open.title")
    assert page.locator("#notify-toast-body").inner_text() == t(page, "notify.window_open.body", {"where": "Window (Bath)", "minutes": "30"})
    assert_axe(page, "the toast")
    page.locator("#notify-toast-close").click()
    toast.wait_for(state="hidden")
    page.evaluate(send, n)
    page.wait_for_timeout(300)
    assert toast.is_hidden(), "the same notification shows once"
    page.evaluate("""(n) => navigator.serviceWorker.dispatchEvent(new MessageEvent("message", { data: { type: "notification", notification: n } }))""",
                  notification(45, "2026-11-09T06:57:00Z"))
    toast.wait_for(state="visible")
    assert "45" in page.locator("#notify-toast-body").inner_text(), "the service worker's notification"


def test_guest_has_none(page, case):
    """A guest has no notifications: the page says why and shows no
    rules."""
    page.route("**/api/v1/me", lambda route: route.fulfill(status=200, content_type="application/json", body=json.dumps(GUEST)))
    reload_with(page)
    show_view(page, "notify")
    none = page.locator("#notify-none")
    none.wait_for(state="visible")
    assert none.inner_text() == t(page, "notify.noCapability")
    assert page.locator("#notify-body").is_hidden()
    assert_axe(page, "a guest's page")


def test_browser_without_push(page, case):
    """A browser without push says so and offers no button."""
    reload_with(page, "delete window.PushManager;")
    show_view(page, "notify")
    assert page.locator("#notify-device-state").inner_text() == t(page, "notify.unsupported")
    assert page.locator("#notify-on").is_hidden() and page.locator("#notify-off").is_hidden()


def test_service_worker_shows_a_push(page, case, context):
    """The page's own service worker, sw.js, takes a push that Chromium's
    DevTools deliver as a push service would: the page's toast while
    the page is in front; else the system's notification, in the
    language the page told it, from the texts it fetched. A headless
    browser has neither a page behind another nor notifications, so the
    worker is made to see no window and to keep what it would show."""
    page.evaluate("navigator.serviceWorker.register('sw.js').then(() => navigator.serviceWorker.ready).then(() => tellWorkerLanguage())")
    cdp = context.new_cdp_session(page)
    regs = {}
    cdp.on("ServiceWorker.workerRegistrationUpdated", lambda e: regs.update({r["scopeURL"]: r["registrationId"] for r in e["registrations"]}))
    cdp.send("ServiceWorker.enable")
    scope = page.evaluate("new URL('.', location.href).href")
    for _ in range(50):
        if scope in regs and context.service_workers:
            break
        page.wait_for_timeout(100)
    assert scope in regs and context.service_workers, f"the worker at {scope}: {regs}"
    worker = context.service_workers[0]
    for _ in range(50):  # the worker keeps the language it was told
        if worker.evaluate("caches.open('massimo-notify').then((c) => c.match(scoped('massimo-lang'))).then((r) => (r ? r.text() : ''))") == case.lang:
            break
        page.wait_for_timeout(100)

    def push(minutes, when):
        n = notification(minutes, when)
        cdp.send("ServiceWorker.deliverPushMessage", {"origin": scope.rstrip("/"), "registrationId": regs[scope], "data": json.dumps(n)})
        return (t(page, "notify.window_open.title"), t(page, "notify.window_open.body", {"where": "Window (Bath)", "minutes": str(minutes)}))

    want = push(30, "2026-11-09T07:00:00Z")
    page.locator("#notify-toast").wait_for(state="visible")
    assert (page.locator("#notify-toast-title").inner_text(), page.locator("#notify-toast-body").inner_text()) == want

    worker.evaluate("""() => {
      self.__shown = [];
      self.clients.matchAll = async () => [];
      self.registration.showNotification = async (title, opts) => { self.__shown.push([title, opts.body, opts.lang, opts.tag]); };
    }""")
    want = push(45, "2026-11-09T07:15:00Z")
    shown = []
    for _ in range(50):
        shown = worker.evaluate("self.__shown")
        if shown:
            break
        page.wait_for_timeout(100)
    assert shown == [[want[0], want[1], case.lang, "window_open-0a0102"]], shown
