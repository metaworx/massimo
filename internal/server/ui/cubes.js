// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Settings → Cubes: the Cubes the server serves,
// from its store, each in a panel with its name, address, proxy and
// hidden devices (PUT /api/v1/cubes/{sn}), a notice with "Take the
// file's value" for a line of the settings file that differs, and
// Remove (DELETE); then the Cubes found on the LAN with Add (POST
// /api/v1/cubes). The list is drawn when the settings open, after every
// change, and on a poll while no panel is open, so that a poll never
// throws away what is being typed.
"use strict";

const cubesUI = {
  open: new Set(), // serials of the open panels
  edits: {}, // per serial: the hidden list being edited
  names: {}, // radio address → device name, from the device lists seen
  devices: {}, // per serial: the Cube's visible devices, for the hide choice
  busy: false,
};

// cubesMsg shows the outcome of the last action under the list.
function cubesMsg(text, ok) {
  showResult($("cubes-msg"), text, ok);
}

// cubesVisible reports whether the settings page is shown.
function cubesVisible() {
  return !$("settings").classList.contains("hidden");
}

// cubesPolled is called by loadCubes after every load of the list.
function cubesPolled() {
  if (cubesVisible() && cubesUI.open.size === 0 && !cubesUI.busy) renderCubes();
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
    const devs = (await api("GET", "api/v1/cubes/" + encodeURIComponent(serial) + "/devices")) || [];
    cubesUI.devices[serial] = devs;
    for (const d of devs) if (d.name) cubesUI.names[d.rf_address] = d.name;
  } catch (e) {
    cubesUI.devices[serial] = [];
  }
}

// cubeAction runs a change and redraws the list, the header and the
// tiles afterwards; the message says how it went.
async function cubeAction(fn, okText, serial) {
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
}

function saveCube(serial, body) {
  return cubeAction(() => api("PUT", "api/v1/cubes/" + encodeURIComponent(serial), body), t("cubes.saved", { serial }), serial);
}

function addCube(serial) {
  return cubeAction(() => api("POST", "api/v1/cubes", { serial }), t("cubes.added", { serial }), serial);
}

// removeCube asks in the panel, in place of its actions, whether to stop
// serving the Cube; Keep puts the actions back.
function removeCube(serial, actions) {
  const box = confirmBox(t("cubes.removeConfirm", { serial }), { yes: t("cubes.remove"), no: t("action.keep") }, () => {
    cubesUI.open.delete(serial);
    cubeAction(() => api("DELETE", "api/v1/cubes/" + encodeURIComponent(serial)), t("cubes.removed", { serial }), serial);
  }, () => {
    box.replaceWith(actions);
    actions.querySelector(".remove-cube").focus();
  });
  actions.replaceWith(box);
  box.focusFirst();
}

// fillHiddenEditor fills the template's list of hidden devices (box,
// .cube-hidden) with Show again per entry, and wires the field that
// offers the Cube's devices to hide another; the changes are sent with
// Save.
function fillHiddenEditor(c, box) {
  const list = cubesUI.edits[c.serial] || (cubesUI.edits[c.serial] = [...(c.hidden || [])]);
  const ul = box.querySelector(".cube-hidden-list");
  if (!list.length) ul.appendChild(textEl("li", t("cubes.hiddenNone"), "muted small"));
  for (const rf of list) {
    const show = button(t("cubes.unhide"), { cls: "subtle", aria: t("cubes.unhideLabel", { device: deviceLabel(rf) }) }, () => {
      cubesUI.edits[c.serial] = list.filter((x) => x !== rf);
      renderCubes();
    });
    ul.appendChild(elem("li", "", textEl("span", deviceLabel(rf), "mono"), show));
  }
  const input = box.querySelector("input[name=hide]");
  const dl = box.querySelector("datalist");
  dl.id = "cube-devices-" + c.serial;
  fillDatalist(dl, c.serial, list);
  input.setAttribute("list", dl.id);
  const hide = () => {
    const rf = input.value.trim().toLowerCase();
    if (!/^[0-9a-f]{6}$/.test(rf)) {
      cubesMsg(t("cubes.hideBad", { value: input.value.trim() }), false);
      return;
    }
    if (!list.includes(rf)) cubesUI.edits[c.serial] = [...list, rf];
    cubesMsg("");
    renderCubes();
  };
  input.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      ev.preventDefault();
      hide();
    }
  });
  box.querySelector(".cube-hide-add button").addEventListener("click", hide);
}

// fillDatalist offers the Cube's visible devices that are not hidden
// yet in the hide field.
function fillDatalist(dl, serial, hidden) {
  const devs = (cubesUI.devices[serial] || []).filter((d) => !hidden.includes(d.rf_address));
  fillOptions(dl, devs.map((d) => [d.rf_address, d.name || d.rf_address]));
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

// cubePanel is one configured Cube, from its template (tpl-cube-panel):
// the summary with the name, the serial and the badges; a notice per
// line of the settings file that differs (tpl-cube-differs); the form
// with the fields, the hidden devices and Remove and Save. A Cube served
// from serve's command line has a note instead of the form.
function cubePanel(c) {
  const d = fromTemplate("tpl-cube-panel");
  d.dataset.serial = c.serial;
  d.open = cubesUI.open.has(c.serial);
  const sum = d.querySelector("summary");
  sum.querySelector(".panel-name").textContent = c.name ? c.name + " " : "";
  sum.querySelector(".mono").textContent = c.serial;
  const badges = [badgeEl(t(c.connected ? "cubes.connected" : "cubes.disconnected"), c.connected ? "ok" : "warn")];
  if (c.source) badges.push(badgeEl(t("cubes.source." + c.source), "mode"));
  if ((c.file_differs || []).length) badges.push(badgeEl(tn("cubes.differsCount", c.file_differs.length), "warn"));
  sum.querySelector(".panel-badges").append(...badges);
  d.addEventListener("toggle", () => {
    if (d.open) {
      cubesUI.open.add(c.serial);
      if (!cubesUI.devices[c.serial]) loadCubeDevices(c.serial).then(() => devicesLoaded(c.serial));
    } else {
      cubesUI.open.delete(c.serial);
      delete cubesUI.edits[c.serial];
    }
  });

  const form = d.querySelector("form");
  if (c.source === "flag" || !c.source) {
    form.replaceWith(textEl("p", t("cubes.flagNote"), "muted small"));
    return d;
  }
  for (const fd of c.file_differs || []) {
    const row = fromTemplate("tpl-cube-differs");
    row.querySelector("p").textContent = t("cubes.differs", { variable: fd.variable, file: valueText(fd.file), store: valueText(fd.store) });
    const b = row.querySelector(".take-file");
    b.dataset.field = fd.field;
    b.addEventListener("click", () => saveCube(c.serial, { [fd.field]: fd.file }));
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
  if ((c.proxy_listening || []).length) {
    const p = textEl("p", t("cubes.listening", { addr: c.proxy_listening.join(", ") }), "muted small cube-listening");
    field("proxy_serial").parentElement.before(p);
  }
  fillHiddenEditor(c, form.querySelector(".cube-hidden"));
  const actions = form.querySelector(".actions");
  actions.querySelector(".remove-cube").addEventListener("click", () => removeCube(c.serial, actions));

  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = {};
    const name = field("name").value.trim();
    if (name !== (c.name || "")) body.name = name;
    const a = addr.value.trim();
    if (a !== (c.configured_address || "")) body.address = a;
    if (follow.checked !== !!c.follow_discovery) body.follow_discovery = follow.checked;
    const proxy = field("proxy").value.trim();
    if (proxy !== (c.proxy_address || "")) body.proxy_address = proxy;
    const ps = field("proxy_serial").value.trim();
    if (ps.toUpperCase() !== (c.proxy_serial || "")) body.proxy_serial = ps;
    const hidden = cubesUI.edits[c.serial] || c.hidden || [];
    if (!sameList(hidden, c.hidden)) body.hidden = hidden;
    if (!Object.keys(body).length) {
      cubesMsg(t("cubes.unchanged"), true);
      return;
    }
    saveCube(c.serial, body);
  });
  return d;
}

// foundRow is a Cube found on the LAN, from its template
// (tpl-found-row): its serial, address and firmware, in expert mode its
// radio and MAC address, how long ago it answered, and Add (addCube),
// which serves it at once; the header's list of found Cubes (app.js) and
// Settings → Cubes show it.
function foundRow(c) {
  const li = fromTemplate("tpl-found-row");
  li.dataset.serial = c.serial;
  li.querySelector(".found-serial").textContent = c.serial;
  const part = (cls, values) => {
    const el = li.querySelector(cls);
    const v = values.filter((x) => x);
    if (v.length) el.textContent = " · " + v.join(" · ");
    else el.remove();
  };
  // dhcp is absent until the Cube has answered the c request.
  const dhcp = c.dhcp === true ? t("cubes.dhcp") : c.dhcp === false ? t("cubes.static") : "";
  part(".found-facts", [c.address, dhcp, c.firmware ? t("budget.firmware") + " " + c.firmware : ""]);
  part(".found-extra", [c.rf_address, c.mac]);
  const add = li.querySelector(".add-cube");
  const age = ageEl(c.last_seen, false, true);
  if (age) add.before(age);
  add.setAttribute("aria-label", t("cubes.addLabel", { serial: c.serial }));
  add.addEventListener("click", () => addCube(c.serial));
  return li;
}

// renderCubes draws Settings → Cubes from state.cubes and state.found.
function renderCubes() {
  const box = $("cubes-list");
  if (!box) return;
  clear(box);
  if (!state.cubes.length) box.appendChild(textEl("p", t("cubes.none"), "muted small"));
  for (const c of state.cubes) box.appendChild(cubePanel(c));
  if (state.found.length) {
    box.appendChild(textEl("p", t("cubes.found"), "muted cube-found-title"));
    const ul = elem("ul", "cube-found-list");
    for (const c of state.found) {
      const li = foundRow(c);
      li.classList.add("cube-found-row");
      ul.appendChild(li);
    }
    box.appendChild(ul);
  }
}

function initCubes() {
  $("view-settings").addEventListener("click", () => {
    cubesMsg("");
    renderCubes();
    for (const serial of cubesUI.open) loadCubeDevices(serial).then(() => devicesLoaded(serial));
  });
}

document.addEventListener("DOMContentLoaded", initCubes);
