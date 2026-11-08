// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Cubes: the Cubes the server serves,
// from its store, each in a panel with its name, address, proxy and
// hidden devices (PUT /api/v1/cubes/{sn}), a notice with "Take the
// file's value" for a line of the settings file that differs, and
// Remove (DELETE); then the Cubes found on the LAN with Add (POST
// /api/v1/cubes). The list is drawn when the settings open, after every
// change, and on a poll while no panel is open, so that a poll never
// throws away what is being typed. Each panel starts with Pair a device
// (capability pair), the pairing flow: the time, Start, the countdown
// with Stop, and each device that joins with the form that puts it into
// a room (pairDeviceForm, which Devices without a room uses too). A role
// with pair but without cubes sees the panels with the pairing alone.
"use strict";

const cubesUI = {
  open: new Set(), // serials of the open panels
  edits: {}, // per serial: the hidden list being edited
  names: {}, // radio address → device name, from the device lists seen
  devices: {}, // per serial: the Cube's visible devices, for the hide choice
  rooms: {}, // per serial: the rooms of a Cube that is not the one shown
  pairing: {}, // per serial: the pairing flow, see pairingOf
  busy: false,
};

// cubesMsg shows the outcome of the last action under the list.
function cubesMsg(text, ok) {
  showResult($("cubes-msg"), text, ok);
}

// cubesVisible reports whether System settings, with the Cubes, is shown.
function cubesVisible() {
  return isShown("system");
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
    ul.appendChild(elem("li", "", textEl("span", deviceLabel(rf), "mono"), show));
  }
  fillDatalist(box.querySelector("datalist"), serial, list);
  // The list is part of what Save sends: its form checks again (sendWhenChanged).
  box.dispatchEvent(new Event("change", { bubbles: true }));
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

// --- pairing ----------------------------------------------------------

// cubePath is the API path of a Cube.
function cubePath(serial) {
  return "api/v1/cubes/" + encodeURIComponent(serial);
}

// pairingOf is the pairing flow of a Cube's panel, which outlives a
// redraw: form (the time and Start shown), seconds, active and until
// (the server's state), paired (the devices that joined while this page
// watched), msg (the last outcome), edits (per device, what its form
// holds), tick (the countdown's timer).
function pairingOf(serial) {
  return cubesUI.pairing[serial] || (cubesUI.pairing[serial] = { form: false, seconds: 60, active: false, until: 0, paired: [], msg: null, edits: {}, tick: null });
}

// cubeRooms are the rooms a device of the Cube can be put into: the
// page's for the Cube shown, else those loadCubeRooms read.
function cubeRooms(serial) {
  return serial === state.serial ? state.rooms : cubesUI.rooms[serial] || [];
}

async function loadCubeRooms(serial) {
  if (serial === state.serial) return;
  try {
    cubesUI.rooms[serial] = (await api("GET", cubePath(serial) + "/rooms")) || [];
    redrawPairing(serial);
  } catch (e) {
    cubesUI.rooms[serial] = [];
  }
}

// applyPairing takes the server's pairing state, the answer of GET, POST
// and DELETE …/pairing. The devices of a pairing this page did not see
// running are not shown; the end of one it saw is said in the panel.
function applyPairing(serial, st) {
  const p = pairingOf(serial);
  const was = p.active;
  p.active = !!(st && st.active);
  p.until = Date.parse((st && st.until) || "") || 0;
  if (p.active || was) p.paired = (st && st.paired) || [];
  if (p.active) p.form = false;
  if (was && !p.active) p.msg = { text: tn("pairing.ended", p.paired.length), ok: true };
  if (p.active) startPairingTick(serial);
  else stopPairingTick(serial);
  redrawPairing(serial);
}

// loadPairing asks for a Cube's pairing state. Three failures in a row
// while it runs end the countdown, which would ask on forever, and say
// why.
async function loadPairing(serial) {
  const p = pairingOf(serial);
  try {
    const st = await api("GET", cubePath(serial) + "/pairing");
    p.failures = 0;
    applyPairing(serial, st);
  } catch (e) {
    p.failures = (p.failures || 0) + 1;
    if (p.active && p.failures >= 3) {
      p.active = false;
      stopPairingTick(serial);
      p.msg = { text: t("pairing.failed", { error: e.message }), ok: false };
      redrawPairing(serial);
    }
  }
}

// PAIRING_BUTTONS are, per pairing request (pairingOf(serial).sending),
// the button that turns while it runs.
const PAIRING_BUTTONS = { start: ".pairing-form button[type=submit]", stop: ".pair-stop" };

// markPairing draws the pairing's Start and Stop turning while theirs
// runs; pairingSection calls it, and pairingWrite on the panel in place.
function markPairing(box, p) {
  for (const [key, sel] of Object.entries(PAIRING_BUTTONS)) setBusy(box.querySelector(sel), p.sending === key);
}

// pairingWrite runs fn, Start's or Stop's request, as key: its button
// turns, also across a redraw of the section, and a second click sends
// nothing.
async function pairingWrite(serial, key, fn) {
  const p = pairingOf(serial);
  if (p.sending) return;
  p.sending = key;
  const mark = () => {
    const panel = document.querySelector('.cube-panel[data-serial="' + serial + '"]');
    const box = panel && panel.querySelector(".cube-pairing");
    if (box) markPairing(box, p);
  };
  mark();
  try {
    await fn();
  } finally {
    p.sending = null;
    mark();
  }
}

// startPairing sends Start: POST …/pairing with the seconds.
function startPairing(serial) {
  const p = pairingOf(serial);
  const secs = Number(p.seconds);
  if (!Number.isInteger(secs) || secs < 1 || secs > 600) {
    p.msg = { text: t("pairing.secondsBad"), ok: false };
    redrawPairing(serial);
    return;
  }
  return pairingWrite(serial, "start", async () => {
    let st;
    try {
      st = await api("POST", cubePath(serial) + "/pairing", { seconds: secs });
    } catch (e) {
      p.msg = { text: t("pairing.failed", { error: e.message }), ok: false };
      redrawPairing(serial);
      return;
    }
    p.msg = null;
    p.edits = {};
    p.active = false;
    applyPairing(serial, st);
    loadCubeRooms(serial);
  });
}

// stopPairing sends Stop: DELETE …/pairing.
function stopPairing(serial) {
  return pairingWrite(serial, "stop", async () => {
    try {
      applyPairing(serial, await api("DELETE", cubePath(serial) + "/pairing"));
    } catch (e) {
      pairingOf(serial).msg = { text: t("pairing.failed", { error: e.message }), ok: false };
      redrawPairing(serial);
    }
  });
}

// startPairingTick counts down every second while pairing runs. The
// stream reports the shown Cube's devices and the end; another Cube's,
// or the shown one's without a stream or past its time, are asked for.
function startPairingTick(serial) {
  const p = pairingOf(serial);
  if (p.tick) return;
  let n = 0;
  p.tick = setInterval(() => {
    if (!document.querySelector('.cube-panel[data-serial="' + serial + '"]')) {
      stopPairingTick(serial); // the Cube is no longer served
      return;
    }
    n++;
    renderCountdown(serial);
    const watched = serial === state.serial && state.eventsOpen && Date.now() < p.until;
    if (!watched && n % 3 === 0) loadPairing(serial);
  }, 1000);
}

function stopPairingTick(serial) {
  const p = pairingOf(serial);
  if (p.tick) clearInterval(p.tick);
  p.tick = null;
}

// renderCountdown writes the time left into the panel, in place.
function renderCountdown(serial) {
  const panel = document.querySelector('.cube-panel[data-serial="' + serial + '"]');
  const el = panel && panel.querySelector(".pairing-countdown");
  if (!el) return;
  const left = Math.ceil((pairingOf(serial).until - Date.now()) / 1000);
  el.textContent = left > 0 ? t("pairing.left", { seconds: left }) : t("pairing.ending");
}

// pairingEvent takes the shown Cube's pairing_started and pairing_ended
// events (app.js): the state is asked for.
function pairingEvent(serial) {
  loadPairing(serial);
}

// pairingJoined takes a paired event of the shown Cube: the device is
// listed at once.
function pairingJoined(serial, dev) {
  const p = pairingOf(serial);
  if (!p.active) {
    loadPairing(serial);
    return;
  }
  if (dev && !p.paired.some((x) => x.rf_address === dev.rf_address)) p.paired.push(dev);
  redrawPairing(serial);
}

// redrawPairing draws a panel's pairing again, in place, so that the
// Cube's form keeps what is typed in it.
function redrawPairing(serial) {
  const panel = document.querySelector('.cube-panel[data-serial="' + serial + '"]');
  const old = panel && panel.querySelector(".cube-pairing");
  if (old) old.replaceWith(pairingSection(serial));
}

// pairingSection is a panel's pairing (tpl-pairing): Pair a device; the
// time and Start; while pairing runs, the countdown and Stop; the last
// outcome; the devices that joined.
function pairingSection(serial) {
  const p = pairingOf(serial);
  const box = fromTemplate("tpl-pairing");
  const open = box.querySelector(".pair-open");
  const form = box.querySelector(".pairing-form");
  const running = box.querySelector(".pairing-running");
  open.classList.toggle("hidden", p.form || p.active);
  form.classList.toggle("hidden", !p.form || p.active);
  running.classList.toggle("hidden", !p.active);
  open.addEventListener("click", () => {
    p.form = true;
    p.msg = null;
    redrawPairing(serial);
    const panel = document.querySelector('.cube-panel[data-serial="' + serial + '"]');
    if (panel) panel.querySelector(".pairing-form input[name=seconds]").focus();
  });
  const secs = form.querySelector("input[name=seconds]");
  secs.id = "pair-seconds-" + serial;
  secs.closest(".field").querySelector("label").htmlFor = secs.id;
  secs.value = String(p.seconds);
  secs.addEventListener("input", () => {
    p.seconds = secs.value;
  });
  form.querySelector(".pair-cancel").addEventListener("click", () => {
    p.form = false;
    redrawPairing(serial);
  });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    startPairing(serial);
  });
  running.querySelector(".pair-stop").addEventListener("click", () => stopPairing(serial));
  if (p.msg) showResult(box.querySelector(".pairing-msg"), p.msg.text, p.msg.ok);
  const list = box.querySelector(".pairing-devices");
  for (const d of p.paired) list.appendChild(pairedRow(serial, d));
  if (p.active) {
    const left = Math.ceil((p.until - Date.now()) / 1000);
    box.querySelector(".pairing-countdown").textContent = left > 0 ? t("pairing.left", { seconds: left }) : t("pairing.ending");
  }
  markPairing(box, p);
  return box;
}

// pairedRow is a device that joined: its type, serial and radio address,
// and the form that puts it into a room, or how that went.
function pairedRow(serial, d) {
  const p = pairingOf(serial);
  const e = p.edits[d.rf_address] || (p.edits[d.rf_address] = {});
  const row = elem("div", "pair-device",
    elem("div", "pair-device-head", textEl("span", typeName(d.type), "pair-device-type"), textEl("span", d.serial || "", "mono muted"),
      textEl("span", d.rf_address, "mono muted expert-only")));
  row.dataset.rf = d.rf_address;
  if (e.done) {
    const done = textEl("p", "", "small pair-result");
    showResult(done, e.result.text, true);
    row.appendChild(done);
  } else {
    row.appendChild(pairDeviceForm(serial, d, e, { onDone: () => redrawPairing(serial), onFail: () => redrawPairing(serial) }));
  }
  return row;
}

// pairDeviceForm is the form that puts a device that is in no room into
// one (tpl-pair-device, capability structure): its name (with rename),
// the room, and for a thermostat the switch that copies the room's
// settings (with configure, which the copy needs as well), on by
// default; PUT …/devices/{rf}. e keeps what is typed
// across redraws, the result, and done once the device is in the room;
// onDone runs then, onFail after a refusal, and onCancel, when given,
// is the form's Cancel.
function pairDeviceForm(serial, dev, e, { onDone, onFail, onCancel } = {}) {
  const form = fromTemplate("tpl-pair-device");
  const id = "pair-" + serial + "-" + dev.rf_address;
  const name = form.querySelector("input[name=name]");
  name.id = id + "-name";
  name.previousElementSibling.htmlFor = name.id;
  name.value = e.name || "";
  name.addEventListener("input", () => {
    e.name = name.value;
  });
  const rooms = cubeRooms(serial);
  const select = form.querySelector("select[name=room]");
  select.id = id + "-room";
  select.previousElementSibling.htmlFor = select.id;
  fillOptions(select, rooms.map((r) => [String(r.id), r.name]));
  if (!rooms.some((r) => r.id === e.room) && rooms.length) e.room = rooms[0].id;
  if (e.room !== undefined) select.value = String(e.room);
  select.addEventListener("change", () => {
    e.room = Number(select.value);
  });
  const copyRow = form.querySelector(".pair-copy");
  const copy = copyRow.querySelector("input[name=copy]");
  if (isThermostat(dev)) {
    copy.checked = e.copy !== false;
    copy.addEventListener("change", () => {
      e.copy = copy.checked;
    });
    const h = helpParts("pairCopy", t("pairing.copy"));
    copyRow.append(h.btn, h.text);
  } else {
    copyRow.remove();
  }
  const result = form.querySelector(".pair-result");
  const submit = form.querySelector("button[type=submit]");
  if (e.result) showResult(result, e.result.text, e.result.ok);
  if (!rooms.length) {
    submit.disabled = true;
    showResult(result, t("pairing.noRooms"), false);
  }
  setBusy(submit, !!e.busy); // a redraw while it is being sent
  const cancel = form.querySelector(".pair-device-cancel");
  if (onCancel) cancel.addEventListener("click", onCancel);
  else cancel.remove();
  form.addEventListener("submit", async (ev) => {
    ev.preventDefault();
    if (e.busy || !rooms.length) return;
    const nm = (e.name || "").trim();
    const label = nm || dev.name || dev.rf_address;
    const body = { room_id: e.room };
    if (isThermostat(dev)) body.copy = hasCap("configure") && e.copy !== false;
    if (nm && hasCap("rename")) {
      if (byteLength(nm) > NAME_BYTES) {
        e.result = { text: t("cfg.devNameTooLong", { name: nm, n: NAME_BYTES }), ok: false };
        showResult(result, e.result.text, false);
        return;
      }
      body.name = nm;
    }
    const target = rooms.find((r) => r.id === e.room);
    const roomName = target ? target.name : String(e.room);
    e.busy = true;
    e.result = { text: t("pairing.putting", { device: label, room: roomName }), ok: true };
    setBusy(submit, true);
    showResult(result, e.result.text, true);
    let res;
    try {
      res = await api("PUT", cubePath(serial) + "/devices/" + dev.rf_address, body);
    } catch (err) {
      // A copy that stopped leaves the device in its room: the reload
      // shows where it is, and onFail says the error where it is seen.
      e.busy = false;
      e.result = { text: errorText(label, err), ok: false };
      showResult(result, e.result.text, false);
      setBusy(submit, false);
      if (serial === state.serial) await loadRooms().catch(() => {});
      if (onFail) onFail();
      return;
    }
    let text = t("pairing.done", { device: label, room: roomName });
    if (res && res.copied) {
      text += " " + t("pairing.copied", { days: res.copied.days });
      if (res.copied.mode_not_set) text += " " + t("pairing.modeNotSet");
    }
    if (res && res.budget) text = t("result.budget", { text, duty: res.budget.duty_cycle_pct, slots: res.budget.free_memory_slots });
    e.result = { text, ok: true };
    e.done = true;
    e.busy = false;
    if (serial === state.serial) await loadRooms().catch(() => {});
    else loadCubeRooms(serial);
    if (onDone) onDone();
  });
  return form;
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
function cubePanel(c) {
  const badges = [badgeEl(t(c.connected ? "cubes.connected" : "cubes.disconnected"), c.connected ? "ok" : "warn")];
  if (c.source) badges.push(badgeEl(t("cubes.source." + c.source), "mode"));
  if ((c.file_differs || []).length) badges.push(badgeEl(tn("cubes.differsCount", c.file_differs.length), "warn"));
  const d = detailsPanel({ name: c.name || undefined, mono: c.serial, badges, open: cubesUI.open.has(c.serial), cls: "cube-panel" },
    fromTemplate("tpl-cube-panel"));
  d.dataset.serial = c.serial;
  const sum = d.querySelector("summary");
  d.addEventListener("toggle", () => {
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
  // two columns where there is room (style.css .cube-cols): the Cube's
  // fields at the left, the pairing and the hidden devices at the right
  const form = d.querySelector("form.cube-form");
  sum.after(elem("div", "cube-cols", pairingSection(c.serial), form));
  if (c.source === "flag" || !c.source) {
    form.replaceWith(textEl("p", t("cubes.flagNote"), "muted small needs-cubes"));
    return d;
  }
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
  if ((c.proxy_listening || []).length) {
    const p = textEl("p", t("cubes.listening", { addr: c.proxy_listening.join(", ") }), "muted small cube-listening");
    field("proxy_serial").parentElement.before(p);
  }
  fillHiddenEditor(c, form.querySelector(".cube-hidden"));
  const actions = form.querySelector(".actions");
  actions.querySelector(".remove-cube").addEventListener("click", () => removeCube(c.serial, actions));

  // changes is what Save sends: the fields that differ from the Cube's.
  const changes = () => {
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
    return body;
  };
  const save = actions.querySelector("button[type=submit]");
  sendWhenChanged(form, () => Object.keys(changes()).length > 0, { btn: save });
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

// foundRow is a Cube found on the LAN, from its template
// (tpl-found-row): its serial, address and firmware, in expert mode its
// radio and MAC address, how long ago it answered, and Add (addCube),
// which serves it at once; the header's list of found Cubes (app.js) and
// System settings → Cubes show it.
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
  add.addEventListener("click", () => addCube(c.serial, add));
  return li;
}

// renderCubes draws System settings → Cubes from state.cubes and state.found.
function renderCubes() {
  const box = $("cubes-list");
  if (!box) return;
  clear(box);
  if (!state.cubes.length) box.appendChild(textEl("p", t("cubes.none"), "muted small"));
  for (const c of state.cubes) box.appendChild(cubePanel(c));
  const off = state.cubes.filter((c) => !c.connected).length;
  const badges = [badgeEl(tn("cubes.badge.served", state.cubes.length), "mode")];
  if (off) badges.push(badgeEl(tn("cubes.badge.disconnected", off), "bad"));
  if (state.found.length) badges.push(badgeEl(tn("cubes.badge.found", state.found.length), "warn"));
  sectionBadges($("sys-cubes"), badges);
  if (state.found.length) {
    box.appendChild(textEl("p", t("cubes.found"), "muted cube-found-title needs-cubes"));
    const ul = elem("ul", "cube-found-list needs-cubes");
    for (const c of state.found) {
      const li = foundRow(c);
      li.classList.add("cube-found-row");
      ul.appendChild(li);
    }
    box.appendChild(ul);
  }
}

// initCubes draws System settings → Cubes when the page opens; the panels
// still open from before load what they would load when opened.
function initCubes() {
  onView("system", () => {
    cubesMsg("");
    renderCubes();
    for (const serial of cubesUI.open) {
      loadCubeDevices(serial).then(() => devicesLoaded(serial));
      loadPairing(serial);
      loadCubeRooms(serial);
    }
  });
}

document.addEventListener("DOMContentLoaded", initCubes);
