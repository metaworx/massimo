// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, a Cube's panel: its head with its badges, its
// facts, its actions (save, add, reboot, remove), the editor of its
// hidden devices, and the panel itself, in System settings → Cubes and
// in Cube settings.
"use strict";

// hostPort splits a Cube's address, host:port or [v6]:port, into the
// host and the port; an address without a port is its host.
function hostPort(addr) {
  const a = addr || "";
  const m = /^\[([^\]]+)\]:(\d+)$/.exec(a) || /^([^:]+):(\d+)$/.exec(a);
  return m ? { host: m[1], port: m[2] } : { host: a, port: "" };
}

// fillCubeFacts fills a Cube's panel from its entry of the list: the
// head's host beside the serial, without the port, and its radio budget
// (duty cycle, free slots and pending); the facts block (tpl-cube-info)
// with the address and the port, the firmware, the radio address, the
// Cube's clock and the whole budget.
function fillCubeFacts(d, c) {
  const { host, port } = hostPort(c.address);
  const f = budgetFacts(c); // app.js
  d.querySelector("summary .cube-head-who > .mono").textContent = host ? c.serial + " · " + host : c.serial;
  d.querySelector(".cube-head-stats").textContent =
    [t("budget.duty") + " " + f.duty, t("budget.slots") + " " + f.slots, t("budget.pending") + " " + f.pending].join(" · ");
  const clock = clockText(c); // app.js
  const facts = { serial: c.serial, host, port, firmware: c.firmware, radio: c.rf_address, clock: clock.text, ...f };
  for (const td of d.querySelectorAll(".cube-info [data-fact]")) {
    const v = facts[td.dataset.fact] || "";
    if (!td.hasAttribute("data-copy")) {
      td.textContent = v || "–";
      continue;
    }
    // the serial, the address and the radio address copy (components.js
    // copyEl), drawn again only where they changed, so that a poll leaves
    // a focused icon where it is
    if (td.dataset.value === v) continue;
    td.dataset.value = v;
    const what = td.closest("tr").querySelector("th").textContent;
    td.replaceChildren(v ? copyEl(v, { label: what, icon: true }) : "–");
  }
  d.querySelector('.cube-info [data-fact="clock"]').title = clock.title;
  fillLinkState(d, c);
  fillAddressState(d, c);
  fillProxyState(d, c);
  fillCubeHead(d, c);
  if (d.syncForm) d.syncForm(c);
}

// fillLinkState says above the Cube's facts why it is not connected,
// where the server knows (linkText, app.js), and since when.
function fillLinkState(d, c) {
  const box = d.querySelector(".cube-link-state");
  if (!box) return;
  clear(box);
  const text = linkText(c);
  if (text) box.appendChild(noticeEl(text, { since: c.link && c.link.since }));
}

// fillAddressState says above the Cube's facts that discovery finds a
// Cube with a fixed address at another one (found_elsewhere), since
// when, and, where the Cube has the form (not one of -addr or -sn),
// offers to take the found address or to follow discovery. It is
// redrawn only where the finding changed, so that a poll leaves a
// focused button where it is.
function fillAddressState(d, c) {
  const box = d.querySelector(".cube-address-state");
  if (!box) return;
  const f = c.found_elsewhere;
  const editable = !!f && !!c.source && c.source !== "flag";
  const key = f ? [f.address, f.configured, f.since, editable].join(" ") : "";
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  clear(box);
  if (!f) return;
  box.appendChild(noticeEl(t("cubes.elsewhere", { found: f.address, configured: f.configured }), { since: f.since })); // components.js
  if (!editable) return;
  const use = textEl("button", t("cubes.useFound"), "use-found");
  use.type = "button";
  use.addEventListener("click", () => saveCube(c.serial, { address: f.address }, use));
  const follow = textEl("button", t("cubes.followFound"), "follow-found");
  follow.type = "button";
  follow.addEventListener("click", () => saveCube(c.serial, { follow_discovery: true }, follow));
  box.appendChild(elem("div", "cube-elsewhere-fix needs-cubes", use, follow));
}

// fillProxyState says under the proxy address where the Cube's proxy
// listens, and, for a listener that does not, why and since when
// (noticeEl), the server trying it again; nothing where the form is not
// there (a Cube of -addr or -sn).
function fillProxyState(d, c) {
  const box = d.querySelector(".cube-proxy-state");
  if (!box) return;
  clear(box);
  for (const f of c.proxy_failing || []) {
    box.appendChild(noticeEl(t("proxy.notListening", { addr: f.addr, error: f.error }), { since: f.since, next: f.next_try }));
  }
  if ((c.proxy_listening || []).length) {
    box.appendChild(textEl("p", t("cubes.listening", { addr: c.proxy_listening.join(", ") }), "muted small cube-listening"));
  }
}

// proxyFailing counts the Cube proxy listeners of cubes that do not listen.
function proxyFailing(cubes) {
  return cubes.reduce((n, c) => n + (c.proxy_failing || []).length, 0);
}

// valueText shows a value of a file difference: a list joined, an empty
// one as "(empty)".
function valueText(v) {
  const s = Array.isArray(v) ? v.join(", ") : String(v === null || v === undefined ? "" : v);
  return s === "" ? t("cubes.empty") : s;
}

function deviceLabel(rf) {
  const name = cubesUI.names[rf];
  return name ? name + " (" + rf + ")" : rf;
}

// loadCubeDevices reads a Cube's visible devices for the hide choice and
// remembers their names.
async function loadCubeDevices(serial) {
  try {
    const devs = (await api("GET", "api/v1/cubes/" + encodeURIComponent(serial) + "/devices?devices=real")) || [];
    cubesUI.devices[serial] = devs;
    for (const d of devs) {
      if (d.name) cubesUI.names[d.rf_address] = d.name;
      if (d.serial) cubesUI.serials[d.rf_address] = d.serial;
    }
  } catch (e) {
    cubesUI.devices[serial] = [];
  }
}

// cubeAction runs a change and redraws the list, the header and the
// tiles afterwards; the message says how it went. btn, the button it
// runs for, is busy until then (whileBusy).
function cubeAction(fn, okText, serial, btn) {
  return whileBusy(btn, async () => {
    cubesUI.busy = true;
    try {
      await fn();
      delete cubesUI.edits[serial];
      await refresh();
      renderCubes();
      cubesMsg(okText, true);
      return true;
    } catch (e) {
      cubesMsg(t("cubes.failed", { serial, error: e.message }), false);
      return false;
    } finally {
      cubesUI.busy = false;
    }
  });
}

function saveCube(serial, body, btn) {
  return cubeAction(() => api("PUT", "api/v1/cubes/" + encodeURIComponent(serial), body), t("cubes.saved", { serial }), serial, btn);
}

function addCube(serial, btn) {
  return cubeAction(() => api("POST", "api/v1/cubes", { serial }), t("cubes.added", { serial }), serial, btn);
}

// rebootRow is the Cube's Reboot at the bottom of its panel, for a role
// with cubes.reboot: a restart frees a Cube that still holds an old
// connection at once (docs/api-v1.md, POST /api/v1/cubes/{sn}/reboot).
function rebootRow(c) {
  const row = elem("div", "actions cube-reboot needs-cubes-reboot");
  row.appendChild(button(t("cubes.reboot"), "cube-reboot-btn", () => rebootCube(c, row)));
  return row;
}

// rebootCube asks in place of the Reboot whether to restart the Cube,
// naming what a restart costs: the Cube gone for a minute and its rooms
// refusing commands for a while, the radiograms pending as the Cube last
// said and when, and for a Cube busy with another client that the client
// may take it back. Yes sends the reboot with the count it was shown.
function rebootCube(c, row) {
  const n = c.pending_radiograms || 0;
  const parts = [t("cubes.rebootConfirm", { serial: c.serial })];
  if (n > 0) {
    const at = Date.parse((c.budget && c.budget.updated_at) || "");
    parts.push(tn("cubes.rebootPending", n, { time: isNaN(at) ? "–" : fmtShortTime(at) }));
  }
  if (linkState(c) === "busy") parts.push(t("cubes.rebootBusy"));
  confirmInPlace(row, parts.join(" "), { yes: t("cubes.reboot"), no: t("action.cancel") },
    () => cubeAction(() => api("POST", "api/v1/cubes/" + encodeURIComponent(c.serial) + "/reboot", { pending: n }),
      t("cubes.rebooted", { serial: c.serial }), c.serial),
    ".cube-reboot-btn");
}

// removeCube asks in the panel, in place of its actions, whether to stop
// serving the Cube; Keep puts the actions back.
function removeCube(serial, actions) {
  confirmInPlace(actions, t("cubes.removeConfirm", { serial }), { yes: t("cubes.remove"), no: t("action.keep") }, () => {
    cubesUI.open.delete(serial);
    return cubeAction(() => api("DELETE", "api/v1/cubes/" + encodeURIComponent(serial)), t("cubes.removed", { serial }), serial);
  }, ".remove-cube");
}

// fillHiddenEditor fills the template's list of hidden devices (box,
// .cube-hidden) and wires the field that offers the Cube's devices to
// hide another; the changes are sent with Save. Hide and Show again
// change the list in place (fillHiddenList), so that what is typed in
// the rest of the form stays.
function fillHiddenEditor(c, box) {
  if (!cubesUI.edits[c.serial]) cubesUI.edits[c.serial] = [...(c.hidden || [])];
  const input = box.querySelector("input[name=hide]");
  const dl = box.querySelector("datalist");
  dl.id = "cube-devices-" + c.serial;
  input.setAttribute("list", dl.id);
  fillHiddenList(c.serial, box, input);
  const hide = () => {
    const rf = input.value.trim().toLowerCase();
    if (!/^[0-9a-f]{6}$/.test(rf)) {
      cubesMsg(t("cubes.hideBad", { value: input.value.trim() }), false);
      return;
    }
    const list = cubesUI.edits[c.serial];
    if (!list.includes(rf)) cubesUI.edits[c.serial] = [...list, rf];
    cubesMsg("");
    input.value = "";
    fillHiddenList(c.serial, box, input);
  };
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      hide();
    }
  });
  box.querySelector(".cube-hide-add button").addEventListener("click", hide);
}

// fillHiddenList draws the list being edited, with Show again per entry,
// and the device choice of the hide field; Show again hands the focus
// to that field, since its own button goes.
function fillHiddenList(serial, box, input) {
  const list = cubesUI.edits[serial];
  const ul = box.querySelector(".cube-hidden-list");
  clear(ul);
  if (!list.length) ul.appendChild(textEl("li", t("cubes.hiddenNone"), "muted small"));
  for (const rf of list) {
    const show = button(t("cubes.unhide"), { cls: "subtle", aria: t("cubes.unhideLabel", { device: deviceLabel(rf) }) }, () => {
      cubesUI.edits[serial] = cubesUI.edits[serial].filter((x) => x !== rf);
      fillHiddenList(serial, box, input);
      input.focus();
    });
    // the name where known, the serial and the radio address, which the
    // Hide field takes, shown always
    const name = cubesUI.names[rf];
    const who = elem("span", "hidden-device", deviceIds(cubesUI.serials[rf], rf, { rf: "always" }));
    if (name) who.prepend(textEl("span", name));
    ul.appendChild(elem("li", "", who, show));
  }
  fillDatalist(box.querySelector("datalist"), serial, list);
  // The list is part of what Save sends: its form checks again (sendWhenChanged).
  box.dispatchEvent(new Event("change", { bubbles: true }));
}

// fillDatalist offers the Cube's visible devices that are not hidden
// yet in the hide field.
function fillDatalist(dl, serial, hidden) {
  const devs = (cubesUI.devices[serial] || []).filter((d) => !hidden.includes(d.rf_address));
  fillOptions(dl, devs.map((d) => [d.rf_address, deviceName(d)]));
}

// devicesLoaded fills an open panel's device choice in place, so that
// what is being typed in the panel stays.
function devicesLoaded(serial) {
  const dl = document.getElementById("cube-devices-" + serial);
  if (dl) fillDatalist(dl, serial, cubesUI.edits[serial] || []);
}

// sameList compares two lists of radio addresses as sets.
function sameList(a, b) {
  const x = [...(a || [])].sort().join(",");
  const y = [...(b || [])].sort().join(",");
  return x === y;
}

// cubePanel is one configured Cube, a detailsPanel: the head with the
// name, the serial and the badges; a notice per line of the settings
// file that differs (tpl-cube-differs); the pairing (pairingSection);
// the form (tpl-cube-panel) with the fields, the hidden devices and
// Remove and Save. A Cube served from serve's command line has a
// note instead of the form. The panel's loads run when the user opens it
// (or opens the settings, initCubes): the toggle event that a panel
// built open queues finds the serial in cubesUI.open and asks for
// nothing, so that a redraw does not redraw the pairing again when the
// answers come, under what the user is typing there.
// cubeBadges are a Cube's own badges, the top line of its open panel:
// its link where it is not connected, a proxy that does not listen, a
// settings file that differs, a fixed address it is found elsewhere
// than, where its configuration comes from.
function cubeBadges(c) {
  const st = linkSign(c); // app.js
  const out = st === "ok" ? [] : [badgeEl(linkWord(c), st === "bad" ? "bad" : "warn")]; // app.js
  const failing = proxyFailing([c]);
  if (failing) out.push(badgeEl(tn("proxy.badge.failing", failing), "warn"));
  if ((c.file_differs || []).length) out.push(badgeEl(tn("cubes.differsCount", c.file_differs.length), "warn"));
  if (c.found_elsewhere) out.push(badgeEl(t("cubes.elsewhereBadge"), "warn"));
  if (c.source) out.push(badgeEl(t("cubes.source." + c.source), "mode"));
  return out;
}

// fillCubeHead draws a Cube's panel's head and top line from its entry
// of the list, again at every poll (fillCubeFacts): its network sign
// before the name; one pill with every problem, its link, its proxy, its
// settings file and its devices (problemsEl), whose click opens its list
// and not the panel; the source a neutral badge; and in the open panel
// its own badges as its top line.
function fillCubeHead(d, c) {
  const sum = d.querySelector("summary");
  sum.querySelector(".conn-sign").replaceWith(cubeSignEl(c)); // app.js
  const entries = [cubeEntry(c, true)]; // app.js
  const was = sum.querySelector(".cube-pill details");
  if (!was || !was.open) {
    const pill = problemsEl(entries, problemsLabelOf(entries)); // components.js, app.js
    if (pill) {
      // a click in its list is not one on the panel's head
      pill.addEventListener("click", (ev) => {
        if (!ev.target.closest("summary.problems-sum")) ev.preventDefault();
      });
    }
    sum.querySelector(".cube-pill").replaceChildren(pill || okPill(t("problem.none"))); // components.js
  }
  d.querySelector(".cube-top-line").replaceChildren(...cubeBadges(c));
}

// On Cube settings (page) the panel is open and stays open, its head
// under the page's row and out of the tab order; its loads run once per
// Cube shown there (cubesUI.page).
function cubePanel(c, { page } = {}) {
  const badges = c.source ? [badgeEl(t("cubes.source." + c.source), "mode")] : [];
  const d = detailsPanel({ name: c.name || undefined, mono: c.serial, badges, open: page || cubesUI.open.has(c.serial), cls: "cube-panel" },
    fromTemplate("tpl-cube-panel"));
  d.dataset.serial = c.serial;
  const sum = d.querySelector("summary");
  // the head (style.css): the Cube's sign, name, serial and radio budget
  // at the left, wrapping; its pill top-right whatever wraps; its badges
  // a line of their own under them
  const who = elem("span", "cube-head-who", elem("span", "conn-sign"), ...sum.querySelectorAll(":scope > .panel-name, :scope > .mono"));
  const pills = elem("span", "cube-pill");
  sum.prepend(who);
  if (sum.querySelector(".panel-badges")) sum.insertBefore(pills, sum.querySelector(".panel-badges"));
  else sum.appendChild(pills);
  if (page) {
    sum.tabIndex = -1;
    // a click on the head folds nothing; its pill's opens the pill
    sum.addEventListener("click", (ev) => {
      if (!ev.target.closest(".cube-pill")) ev.preventDefault();
    });
  }
  d.addEventListener("toggle", () => {
    if (page) {
      if (!d.open || cubesUI.page === c.serial) return; // drawn again: loaded
      cubesUI.page = c.serial;
      if (!cubesUI.devices[c.serial]) loadCubeDevices(c.serial).then(() => devicesLoaded(c.serial));
      loadPairing(c.serial);
      loadCubeRooms(c.serial);
      return;
    }
    if (d.open) {
      if (cubesUI.open.has(c.serial)) return; // built open by a redraw
      cubesUI.open.add(c.serial);
      if (!cubesUI.devices[c.serial]) loadCubeDevices(c.serial).then(() => devicesLoaded(c.serial));
      loadPairing(c.serial);
      loadCubeRooms(c.serial);
    } else {
      cubesUI.open.delete(c.serial);
      delete cubesUI.edits[c.serial];
    }
  });
  // Three groups: the Cube's facts (Info), its settings (the form), and
  // its actions in one row last: Pair a device (its progress under the
  // row), Reboot the Cube, Remove; Info and the settings side by side
  // where there is room (style.css .cube-cols).
  const form = d.querySelector("form.cube-form");
  who.appendChild(textEl("span", "", "cube-head-stats muted small"));
  const acts = elem("div", "cube-actions", pairingSection(c.serial), rebootRow(c));
  const cols = elem("div", "cube-cols", fromTemplate("tpl-cube-info"), form, acts);
  sum.after(elem("div", "cube-top-line"), elem("div", "cube-link-state"), elem("div", "cube-address-state"), cols);
  fillCubeFacts(d, c);
  if (c.source === "flag" || !c.source) {
    form.replaceWith(textEl("p", t("cubes.flagNote"), "muted small needs-cubes"));
    return d;
  }
  const removeBox = elem("div", "cube-remove needs-cubes");
  removeBox.appendChild(button(t("cubes.remove"), "remove-cube", () => removeCube(c.serial, removeBox)));
  acts.appendChild(removeBox);
  for (const fd of c.file_differs || []) {
    const row = fromTemplate("tpl-cube-differs");
    row.querySelector("p").textContent = t("cubes.differs", { variable: fd.variable, file: valueText(fd.file), store: valueText(fd.store) });
    const b = row.querySelector(".take-file");
    b.dataset.field = fd.field;
    b.addEventListener("click", () => saveCube(c.serial, { [fd.field]: fd.file }, b));
    form.before(row);
  }

  const field = (name) => form.querySelector("input[name=" + name + "]");
  for (const input of form.querySelectorAll(".field.wide > input")) {
    input.id = "cube-" + c.serial + "-" + input.name;
    input.previousElementSibling.htmlFor = input.id;
  }
  field("name").value = c.name || "";
  const addr = field("address");
  addr.value = c.configured_address || "";
  if (!c.configured_address && c.address) {
    addr.placeholder = t("cubes.addressFound", { address: c.address });
    delete addr.dataset.i18nAttr; // not the hint's
  }
  const follow = field("follow");
  follow.checked = !!c.follow_discovery;
  field("proxy").value = c.proxy_address || "";
  field("proxy_serial").value = c.proxy_serial || "";
  const virtual = field("proxy_virtual");
  virtual.checked = !!c.proxy_virtual;
  field("proxy_serial").parentElement.before(elem("div", "cube-proxy-state"));
  fillProxyState(d, c);
  fillHiddenEditor(c, form.querySelector(".cube-hidden"));
  const actions = form.querySelector(".actions");

  // cur is the Cube as the list last gave it, which a poll moves on
  // (d.syncForm); changes is what Save sends: the fields that differ
  // from it.
  let cur = c;
  const changes = () => {
    const body = {};
    const name = field("name").value.trim();
    if (name !== (cur.name || "")) body.name = name;
    const a = addr.value.trim();
    if (a !== (cur.configured_address || "")) body.address = a;
    if (follow.checked !== !!cur.follow_discovery) body.follow_discovery = follow.checked;
    const proxy = field("proxy").value.trim();
    if (proxy !== (cur.proxy_address || "")) body.proxy_address = proxy;
    const ps = field("proxy_serial").value.trim();
    if (ps.toUpperCase() !== (cur.proxy_serial || "")) body.proxy_serial = ps;
    if (virtual.checked !== !!cur.proxy_virtual) body.proxy_virtual = virtual.checked;
    const hidden = cubesUI.edits[c.serial] || cur.hidden || [];
    if (!sameList(hidden, cur.hidden)) body.hidden = hidden;
    return body;
  };
  const save = actions.querySelector("button[type=submit]");
  const recheck = sendWhenChanged(form, () => Object.keys(changes()).length > 0, { btn: save });

  // The form's drafts across the polls (D59-A): a field the user has not
  // touched follows a value stored meanwhile, by another client or the
  // settings file; one the user has changed keeps the draft and says
  // under it that the stored value changed, with Take theirs. Each field
  // is how its value is read from the Cube and from the form.
  const text = (input, key, upper) => ({
    input, stored: (x) => (upper ? (x[key] || "").toUpperCase() : x[key] || ""),
    read: () => (upper ? input.value.trim().toUpperCase() : input.value.trim()), write: (v) => { input.value = v; },
  });
  const check = (input, key) => ({ input, stored: (x) => !!x[key], read: () => input.checked, write: (v) => { input.checked = v; } });
  const fields = [text(field("name"), "name"), text(addr, "configured_address"), check(follow, "follow_discovery"),
    text(field("proxy"), "proxy_address"), text(field("proxy_serial"), "proxy_serial", true), check(virtual, "proxy_virtual")];
  const base = fields.map((f) => f.stored(c)); // what the form was drawn from, or last took
  d.syncForm = (nc) => {
    cur = nc;
    fields.forEach((f, i) => {
      const stored = f.stored(nc);
      const row = f.input.closest(".field");
      const conflict = row.nextElementSibling && row.nextElementSibling.classList.contains("cube-conflict") ? row.nextElementSibling : null;
      if (stored === base[i]) return;
      const take = () => {
        f.write(stored);
        base[i] = stored;
        if (row.nextElementSibling && row.nextElementSibling.classList.contains("cube-conflict")) row.nextElementSibling.remove();
        recheck();
      };
      if (f.read() === base[i]) {
        take(); // untouched: it follows
        return;
      }
      const box = elem("div", "cube-conflict");
      box.appendChild(noticeEl(t("cubes.changedMeanwhile", { value: valueText(stored) })));
      box.appendChild(button(t("cubes.takeTheirs"), "take-theirs", take));
      if (conflict) conflict.replaceWith(box);
      else row.after(box);
    });
    recheck();
  };
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = changes();
    if (!Object.keys(body).length) {
      cubesMsg(t("cubes.unchanged"), true);
      return;
    }
    saveCube(c.serial, body, save);
  });
  return d;
}
