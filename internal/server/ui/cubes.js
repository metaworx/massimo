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
  const m = $("cubes-msg");
  m.textContent = text || "";
  m.classList.toggle("hidden", !text);
  m.classList.toggle("ok", !!ok);
  m.classList.toggle("bad", !ok);
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
    const devs = (await api("GET", "/api/v1/cubes/" + encodeURIComponent(serial) + "/devices")) || [];
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
  return cubeAction(() => api("PUT", "/api/v1/cubes/" + encodeURIComponent(serial), body), t("cubes.saved", { serial }), serial);
}

function addCube(serial) {
  return cubeAction(() => api("POST", "/api/v1/cubes", { serial }), t("cubes.added", { serial }), serial);
}

function removeCube(serial) {
  if (!confirm(t("cubes.removeConfirm", { serial }))) return;
  cubesUI.open.delete(serial);
  return cubeAction(() => api("DELETE", "/api/v1/cubes/" + encodeURIComponent(serial)), t("cubes.removed", { serial }), serial);
}

// cubeField is a labelled text input of a Cube panel, the hint its
// placeholder.
function cubeField(c, name, labelKey, value, hintKey) {
  const input = document.createElement("input");
  input.type = "text";
  input.name = name;
  input.value = value || "";
  input.autocomplete = "off";
  input.spellcheck = false;
  if (hintKey) input.placeholder = t(hintKey);
  return fieldRow(t(labelKey), input, { id: "cube-" + c.serial + "-" + name, wide: true });
}

// hiddenEditor is the list of hidden devices with Show again per entry,
// and a field with the Cube's devices to hide another; the changes are
// sent with Save.
function hiddenEditor(c) {
  const box = document.createElement("div");
  box.className = "cube-hidden";
  box.appendChild(textEl("span", t("cubes.hidden"), "cube-hidden-title"));
  const list = cubesUI.edits[c.serial] || (cubesUI.edits[c.serial] = [...(c.hidden || [])]);
  const ul = document.createElement("ul");
  ul.className = "cube-hidden-list";
  if (!list.length) ul.appendChild(textEl("li", t("cubes.hiddenNone"), "muted small"));
  for (const rf of list) {
    const li = document.createElement("li");
    li.appendChild(textEl("span", deviceLabel(rf), "mono"));
    const b = button(t("cubes.unhide"), "subtle", () => {
      cubesUI.edits[c.serial] = list.filter((x) => x !== rf);
      renderCubes();
    });
    b.setAttribute("aria-label", t("cubes.unhideLabel", { device: deviceLabel(rf) }));
    li.appendChild(b);
    ul.appendChild(li);
  }
  box.appendChild(ul);
  const add = document.createElement("div");
  add.className = "inline cube-hide-add";
  const input = document.createElement("input");
  input.type = "text";
  input.name = "hide";
  input.className = "mono";
  input.autocomplete = "off";
  input.spellcheck = false;
  input.setAttribute("aria-label", t("cubes.hideLabel"));
  input.placeholder = "0abc12";
  const dl = document.createElement("datalist");
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
  add.append(input, dl, button(t("cubes.hide"), "", hide));
  box.appendChild(add);
  return box;
}

// fillDatalist offers the Cube's visible devices that are not hidden
// yet in the hide field.
function fillDatalist(dl, serial, hidden) {
  clear(dl);
  for (const d of cubesUI.devices[serial] || []) {
    if (hidden.includes(d.rf_address)) continue;
    const o = document.createElement("option");
    o.value = d.rf_address;
    o.label = d.name || d.rf_address;
    dl.appendChild(o);
  }
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

// cubePanel is one configured Cube.
function cubePanel(c) {
  const d = document.createElement("details");
  d.className = "panel cube-panel";
  d.dataset.serial = c.serial;
  d.open = cubesUI.open.has(c.serial);
  d.addEventListener("toggle", () => {
    if (d.open) {
      cubesUI.open.add(c.serial);
      if (!cubesUI.devices[c.serial]) loadCubeDevices(c.serial).then(() => devicesLoaded(c.serial));
    } else {
      cubesUI.open.delete(c.serial);
      delete cubesUI.edits[c.serial];
    }
  });
  const sum = document.createElement("summary");
  sum.appendChild(textEl("span", c.name ? c.name + " " : "", "cube-panel-name"));
  sum.appendChild(textEl("span", c.serial, "mono"));
  const badges = document.createElement("span");
  badges.className = "cube-badges";
  badges.appendChild(badgeEl(t(c.connected ? "cubes.connected" : "cubes.disconnected"), c.connected ? "ok" : "warn"));
  if (c.source) badges.appendChild(badgeEl(t("cubes.source." + c.source), "mode"));
  if ((c.file_differs || []).length) badges.appendChild(badgeEl(tn("cubes.differsCount", c.file_differs.length), "warn"));
  sum.appendChild(badges);
  d.appendChild(sum);

  if (c.source === "flag" || !c.source) {
    d.appendChild(textEl("p", t("cubes.flagNote"), "muted small"));
    return d;
  }
  for (const fd of c.file_differs || []) {
    const p = document.createElement("div");
    p.className = "cube-differs";
    p.appendChild(textEl("p", t("cubes.differs", { variable: fd.variable, file: valueText(fd.file), store: valueText(fd.store) }), "small"));
    const b = button(t("cubes.takeFile"), "take-file", () => saveCube(c.serial, { [fd.field]: fd.file }));
    b.dataset.field = fd.field;
    p.appendChild(b);
    d.appendChild(p);
  }

  const form = document.createElement("form");
  form.className = "cube-form";
  form.appendChild(cubeField(c, "name", "cubes.name", c.name));
  form.appendChild(cubeField(c, "address", "cubes.address", c.configured_address, "cubes.addressHint"));
  const addr = form.querySelector("input[name=address]");
  if (!c.configured_address && c.address) addr.placeholder = t("cubes.addressFound", { address: c.address });
  const follow = document.createElement("div");
  follow.className = "field";
  const sw = document.createElement("label");
  sw.className = "switch";
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.name = "follow";
  cb.checked = !!c.follow_discovery;
  const slider = document.createElement("span");
  slider.className = "slider";
  slider.setAttribute("aria-hidden", "true");
  sw.append(cb, slider, textEl("span", t("cubes.follow"), "switch-label"));
  follow.appendChild(sw);
  form.appendChild(follow);
  form.appendChild(cubeField(c, "proxy", "cubes.proxy", c.proxy_address, "cubes.proxyHint"));
  if ((c.proxy_listening || []).length) {
    form.appendChild(textEl("p", t("cubes.listening", { addr: c.proxy_listening.join(", ") }), "muted small cube-listening"));
  }
  form.appendChild(cubeField(c, "proxy_serial", "cubes.proxySerial", c.proxy_serial, "cubes.proxySerialHint"));
  form.appendChild(hiddenEditor(c));

  const actions = document.createElement("div");
  actions.className = "actions";
  const left = document.createElement("span");
  left.className = "actions-left";
  left.appendChild(button(t("cubes.remove"), "remove-cube", () => removeCube(c.serial)));
  actions.append(left, submitButton(t("cubes.save")));
  form.appendChild(actions);
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = {};
    const name = form.querySelector("input[name=name]").value.trim();
    if (name !== (c.name || "")) body.name = name;
    const a = addr.value.trim();
    if (a !== (c.configured_address || "")) body.address = a;
    if (cb.checked !== !!c.follow_discovery) body.follow_discovery = cb.checked;
    const proxy = form.querySelector("input[name=proxy]").value.trim();
    if (proxy !== (c.proxy_address || "")) body.proxy_address = proxy;
    const ps = form.querySelector("input[name=proxy_serial]").value.trim();
    if (ps.toUpperCase() !== (c.proxy_serial || "")) body.proxy_serial = ps;
    const hidden = cubesUI.edits[c.serial] || c.hidden || [];
    if (!sameList(hidden, c.hidden)) body.hidden = hidden;
    if (!Object.keys(body).length) {
      cubesMsg(t("cubes.unchanged"), true);
      return;
    }
    saveCube(c.serial, body);
  });
  d.appendChild(form);
  return d;
}

// foundRow is a Cube found on the LAN with its Add button.
function foundRow(c) {
  const li = document.createElement("li");
  li.className = "cube-found-row";
  li.dataset.serial = c.serial;
  li.appendChild(textEl("span", c.serial, "mono"));
  if (c.address) li.appendChild(textEl("span", " · " + c.address));
  const b = button(t("cubes.add"), "add-cube", () => addCube(c.serial));
  b.setAttribute("aria-label", t("cubes.addLabel", { serial: c.serial }));
  li.appendChild(b);
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
    const ul = document.createElement("ul");
    ul.className = "cube-found-list";
    for (const c of state.found) ul.appendChild(foundRow(c));
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
