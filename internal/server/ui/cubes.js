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

// removeCube asks in the panel, in place of its actions, whether to stop
// serving the Cube; Keep puts the actions back.
function removeCube(serial, actions) {
  const box = confirmBox(t("cubes.removeConfirm", { serial }), { yes: t("cubes.remove"), no: t("action.keep") }, () => {
    cubesUI.open.delete(serial);
    cubeAction(() => api("DELETE", "/api/v1/cubes/" + encodeURIComponent(serial)), t("cubes.removed", { serial }), serial);
  }, () => {
    box.replaceWith(actions);
    actions.querySelector(".remove-cube").focus();
  });
  actions.replaceWith(box);
  box.focusFirst();
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
  const list = cubesUI.edits[c.serial] || (cubesUI.edits[c.serial] = [...(c.hidden || [])]);
  const ul = elem("ul", "cube-hidden-list");
  if (!list.length) ul.appendChild(textEl("li", t("cubes.hiddenNone"), "muted small"));
  for (const rf of list) {
    const show = button(t("cubes.unhide"), { cls: "subtle", aria: t("cubes.unhideLabel", { device: deviceLabel(rf) }) }, () => {
      cubesUI.edits[c.serial] = list.filter((x) => x !== rf);
      renderCubes();
    });
    ul.appendChild(elem("li", "", textEl("span", deviceLabel(rf), "mono"), show));
  }
  const input = document.createElement("input");
  input.type = "text";
  input.name = "hide";
  input.className = "mono";
  input.autocomplete = "off";
  input.spellcheck = false;
  input.setAttribute("aria-label", t("cubes.hideLabel"));
  input.placeholder = "0abc12";
  const dl = elem("datalist");
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
  const add = elem("div", "inline cube-hide-add", input, dl, button(t("cubes.hide"), "", hide));
  return elem("div", "cube-hidden", textEl("span", t("cubes.hidden"), "cube-hidden-title"), ul, add);
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

// cubePanel is one configured Cube.
function cubePanel(c) {
  const badges = [badgeEl(t(c.connected ? "cubes.connected" : "cubes.disconnected"), c.connected ? "ok" : "warn")];
  if (c.source) badges.push(badgeEl(t("cubes.source." + c.source), "mode"));
  if ((c.file_differs || []).length) badges.push(badgeEl(tn("cubes.differsCount", c.file_differs.length), "warn"));
  const d = detailsPanel({ name: c.name ? c.name + " " : "", mono: c.serial, badges, open: cubesUI.open.has(c.serial), cls: "cube-panel", onToggle: (open) => {
    if (open) {
      cubesUI.open.add(c.serial);
      if (!cubesUI.devices[c.serial]) loadCubeDevices(c.serial).then(() => devicesLoaded(c.serial));
    } else {
      cubesUI.open.delete(c.serial);
      delete cubesUI.edits[c.serial];
    }
  } });
  d.dataset.serial = c.serial;

  if (c.source === "flag" || !c.source) {
    d.appendChild(textEl("p", t("cubes.flagNote"), "muted small"));
    return d;
  }
  for (const fd of c.file_differs || []) {
    const b = button(t("cubes.takeFile"), "take-file", () => saveCube(c.serial, { [fd.field]: fd.file }));
    b.dataset.field = fd.field;
    const text = t("cubes.differs", { variable: fd.variable, file: valueText(fd.file), store: valueText(fd.store) });
    d.appendChild(elem("div", "cube-differs", textEl("p", text, "small"), b));
  }

  const form = formEl(() => {
    const body = {};
    const name = form.querySelector("input[name=name]").value.trim();
    if (name !== (c.name || "")) body.name = name;
    const a = addr.value.trim();
    if (a !== (c.configured_address || "")) body.address = a;
    if (follow.control.checked !== !!c.follow_discovery) body.follow_discovery = follow.control.checked;
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
  }, "cube-form");
  form.appendChild(cubeField(c, "name", "cubes.name", c.name));
  form.appendChild(cubeField(c, "address", "cubes.address", c.configured_address, "cubes.addressHint"));
  const addr = form.querySelector("input[name=address]");
  if (!c.configured_address && c.address) addr.placeholder = t("cubes.addressFound", { address: c.address });
  const follow = switchEl(t("cubes.follow"), c.follow_discovery, null, "follow");
  form.appendChild(fieldRow(null, follow));
  form.appendChild(cubeField(c, "proxy", "cubes.proxy", c.proxy_address, "cubes.proxyHint"));
  if ((c.proxy_listening || []).length) {
    form.appendChild(textEl("p", t("cubes.listening", { addr: c.proxy_listening.join(", ") }), "muted small cube-listening"));
  }
  form.appendChild(cubeField(c, "proxy_serial", "cubes.proxySerial", c.proxy_serial, "cubes.proxySerialHint"));
  form.appendChild(hiddenEditor(c));
  const remove = button(t("cubes.remove"), "remove-cube", () => removeCube(c.serial, actions));
  const actions = formFooter({ submit: t("cubes.save"), extra: [remove] });
  form.appendChild(actions);
  d.appendChild(form);
  return d;
}

// foundRow is a Cube found on the LAN: its serial, address and firmware,
// in expert mode its radio and MAC address, how long ago it answered, and
// Add (addCube), which serves it at once; the header's list of found
// Cubes (app.js) and Settings → Cubes show it.
function foundRow(c) {
  const li = elem("li", "", textEl("span", c.serial, "mono"));
  li.dataset.serial = c.serial;
  const facts = [c.address, c.firmware ? t("budget.firmware") + " " + c.firmware : ""].filter((x) => x);
  if (facts.length) li.appendChild(textEl("span", " · " + facts.join(" · ")));
  const extra = [c.rf_address, c.mac].filter((x) => x);
  if (extra.length) li.appendChild(textEl("span", " · " + extra.join(" · "), "mono expert-only"));
  const age = ageEl(c.last_seen, false, true);
  if (age) li.appendChild(age);
  li.appendChild(button(t("cubes.add"), { cls: "add-cube", aria: t("cubes.addLabel", { serial: c.serial }) }, () => addCube(c.serial)));
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
