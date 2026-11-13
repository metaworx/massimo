// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, tiles: one tile per room with the setpoint
// stepper, the mode buttons and the inline panels (vacation, delete),
// the House tile with its panel, the device list, the devices without a
// room with Put into a room… and Remove from the Cube, and the actions
// behind them. Uses the state, api and helpers of app.js, and
// pairDeviceForm of cubes.js.
"use strict";

const STEP = 0.5;
const MIN_TEMP = 4.5;
const MAX_TEMP = 30.5;
const DEFAULT_TEMP = 20; // stepper start when the room has no setpoint yet
const SEND_DELAY_MS = 2000;
const RESULT_FADE_MS = 5000;

// --- room facts -------------------------------------------------------

// roomThermostat returns the thermostat whose state stands for the room:
// the wall thermostat when there is one, else the first radiator
// thermostat; null for a room without thermostats.
function roomThermostat(room) {
  const wall = room.devices.find((d) => d.type === "wall_thermostat");
  return wall || room.devices.find(isThermostat) || null;
}

function roomState(room) {
  const th = roomThermostat(room);
  return th && th.state ? th.state : null;
}

// roomModes lists the modes of the room's thermostats, boost counted as
// manual: one entry when they agree, several when a device was set on
// its own (by another client or at the device).
function roomModes(room) {
  const modes = [];
  for (const d of room.devices) {
    if (!isThermostat(d) || !d.state || !d.state.mode) continue;
    const m = d.state.mode === "boost" ? "manual" : d.state.mode;
    if (!modes.includes(m)) modes.push(m);
  }
  return modes;
}

// roomActualDevice is the device whose measured temperature stands for
// the room: its virtual wall thermostat, which the server computes from
// the wall thermostat, else the radiator thermostats' median or mean, so
// that every client shows the same; null while it has none.
function roomActualDevice(room) {
  const v = room.vwt;
  return v && v.state && v.state.actual_temp !== undefined ? v : null;
}

function roomActual(room) {
  const d = roomActualDevice(room);
  return d ? d.state.actual_temp : undefined;
}

// staleText is the warning for a device the API calls stale ("Not seen
// for 30 hours", from two days on in days), "" for any other.
function staleText(d) {
  if (!d.stale) return "";
  const ms = Date.parse(d.last_seen || "");
  const h = isNaN(ms) ? 24 : Math.max(0, Math.floor((Date.now() - ms) / 3600000));
  return h < 48 ? tn("stale.hours", h) : tn("stale.days", Math.floor(h / 24));
}

// received is when the server received the current value of a device's
// state field, as the API's time, or undefined.
function received(d, field) {
  return d.received ? d.received[field] : undefined;
}

function roomUI(room) {
  if (!state.ui[room.id]) state.ui[room.id] = { open: false, pending: null, timer: null, vacation: false, confirm: false, config: null, deferred: false, sending: null };
  return state.ui[room.id];
}

function clampTemp(v) {
  return Math.min(MAX_TEMP, Math.max(MIN_TEMP, Math.round(v * 2) / 2));
}

// apiUntil turns a datetime-local value into the API's "YYYY-MM-DD HH:MM".
function apiUntil(value) {
  return (value || "").replace("T", " ").slice(0, 16);
}

// --- rendering --------------------------------------------------------

function renderRooms() {
  const section = $("rooms");
  clear(section);
  for (const r of state.rooms) section.appendChild(renderRoom(r));
  $("house").classList.toggle("hidden", !state.rooms.some(roomThermostat));
  renderHouseRooms();
  const unl = $("unlisted");
  const list = $("unlisted-devices");
  clear(list);
  for (const rf of Object.keys(unlistedUI)) {
    if (!state.unlisted.some((d) => d.rf_address === rf)) delete unlistedUI[rf];
  }
  if (state.unlisted.length) {
    for (const d of state.unlisted) list.appendChild(renderUnlisted(d));
    unl.classList.remove("hidden");
  } else {
    unl.classList.add("hidden");
  }
}

// unlistedUI is per device without a room what its row has open: form
// (Put into a room…, with edit, what the form holds) or confirm (Remove
// from the Cube asked).
const unlistedUI = {};

// renderUnlisted is a device of Devices without a room: its row
// (renderDevice) with Put into a room… and Remove from the Cube
// (capability structure), and under it the form that puts it into a
// room (pairDeviceForm) or the question whether to remove it.
function renderUnlisted(d) {
  const el = renderDevice(d);
  const ui = unlistedUI[d.rf_address] || (unlistedUI[d.rf_address] = { edit: {} });
  const redraw = () => {
    const old = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
    if (old) old.replaceWith(renderUnlisted(d));
  };
  if (!ui.form && !ui.confirm) {
    const label = deviceName(d);
    el.querySelector(".device-actions").append(
      button(t("pairing.putOpen"), { cls: "needs-structure put-open", aria: t("pairing.putOpenLabel", { device: label }) }, () => {
        ui.form = true;
        redraw();
        const row = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
        if (row) row.querySelector("form select, form input").focus();
      }),
      button(t("device.remove"), { cls: "needs-structure remove-device", aria: t("device.removeLabel", { device: label }) }, () => {
        ui.confirm = true;
        redraw();
        const row = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
        if (row) row.querySelector(".confirm-box").focusFirst();
      }));
  }
  if (ui.form) {
    el.appendChild(pairDeviceForm(state.serial, d, ui.edit, {
      onDone: () => {
        delete unlistedUI[d.rf_address];
        showUnlistedResult(ui.edit.result.text, true);
      },
      onFail: () => {
        // A copy that stopped put the device into its room all the same.
        if (!state.unlisted.some((x) => x.rf_address === d.rf_address)) {
          delete unlistedUI[d.rf_address];
          showUnlistedResult(ui.edit.result.text, false);
        }
      },
      onCancel: () => {
        ui.form = false;
        redraw();
      },
    }));
  }
  if (ui.confirm) {
    const box = confirmBox(t("device.removeConfirm", { device: deviceName(d) }), { yes: t("device.remove"), no: t("action.keep") }, async () => {
      if (ui.removing) return;
      ui.removing = true; // one DELETE: the question stays until it is answered
      for (const b of box.querySelectorAll("button")) b.disabled = true;
      const res = await removeFromCube(d);
      ui.confirm = false;
      ui.removing = false;
      showUnlistedResult(res.text, res.ok);
      redraw();
    }, () => {
      ui.confirm = false;
      redraw();
    });
    el.appendChild(box);
  }
  return el;
}

// showUnlistedResult says under Devices without a room how the last
// action went, the device's row being gone after it, or on the page's
// message line when the section went with its last device; a success
// fades.
function showUnlistedResult(text, ok) {
  const p = $("unlisted").classList.contains("hidden") ? $("message") : $("unlisted-msg");
  showResult(p, text, ok);
  if (ok) {
    setTimeout(() => {
      if (p.textContent === text) p.classList.add("hidden");
    }, RESULT_FADE_MS);
  }
}

// removeFromCube deletes a device from the Cube (DELETE …/devices/{sn})
// and reloads the rooms, which the metadata and device_forgotten events
// do as well; it returns the outcome as a text.
async function removeFromCube(d) {
  const label = deviceName(d);
  try {
    await api("DELETE", devicePath(d));
    delete state.details[d.rf_address];
    await loadRooms();
    return { text: t("device.removed", { device: label }), ok: true };
  } catch (e) {
    return { text: errorText(label, e), ok: false };
  }
}

function renderRoom(room) {
  const el = fromTemplate("room-template");
  const ui = roomUI(room);
  ui.deferred = false; // this render shows the state a state event left waiting
  el.dataset.id = room.id;
  el.querySelector(".room-name").textContent = room.name;
  el.querySelector(".room-name-btn").addEventListener("click", () => toggleDevices(room));
  renderTools(el, room);
  const actual = roomActual(room);
  el.querySelector(".temp-is").textContent = fmtTemp(actual, "°");
  el.querySelector(".temp-is").title = t(actual === undefined ? "room.noMeasured" : "room.measured");
  const src = roomActualDevice(room);
  el.querySelector(".temp-val").classList.toggle("stale", !!(src && src.stale));
  const age = src && ageEl(received(src, "actual_temp"), src.stale);
  if (age) el.querySelector(".temp-age").appendChild(age);

  renderProblems(el, room);
  renderNotReady(el, room);
  renderStale(el, room);
  renderSetpoint(el, room);
  renderMenu(el, room);
  renderVacation(el, room);
  renderDeleteConfirm(el, room);
  renderConfig(el, room);
  // A state event that came while the configuration had the focus is
  // drawn once the focus has left it (updateDevice); a tick later, since
  // focusout comes before the focus is anywhere else.
  el.querySelector(".config-area").addEventListener("focusout", () => {
    setTimeout(() => {
      const r = state.rooms.find((x) => x.id === room.id);
      if (!r || !roomUI(r).deferred || configFocused(r)) return;
      rerenderRoom(r);
    }, 0);
  });

  const devs = el.querySelector(".devices");
  if (ui.open) {
    devs.classList.remove("hidden");
    fillDevices(devs, room);
  }
  markSending(el, room);
  return el;
}

function renderNotReady(el, room) {
  const notReady = el.querySelector(".not-ready");
  if (room.ready) return;
  const silent = room.devices.filter((d) => !d.reported);
  notReady.textContent = t("room.notReady");
  if (silent.length) {
    notReady.appendChild(document.createTextNode(": "));
    silent.forEach((d, i) => {
      if (i > 0) notReady.appendChild(document.createTextNode(", "));
      notReady.appendChild(textEl("span", deviceName(d)));
      if (d.name) notReady.appendChild(textEl("span", " " + d.rf_address, "mono expert-only"));
    });
  }
  notReady.appendChild(document.createTextNode("."));
  notReady.classList.remove("hidden");
}

// PROBLEM_KINDS are the problems a tile's head shows as icons, in their
// order there, each with its icon (index.html's sprite) and its name.
const PROBLEM_KINDS = {
  radio: "icon-radio",
  battery: "icon-battery",
  window: "icon-window-open",
  mixed: "icon-shuffle",
};

// MODE_TONE colours a mode's badge: auto green, manual (and boost) amber,
// holiday violet.
const MODE_TONE = { auto: "fill-ok", manual: "fill-warn", holiday: "fill-info" };

// modeTone is the badge kind of a thermostat's mode.
function modeTone(mode) {
  const m = mode === "vacation" ? "holiday" : mode === "boost" ? "manual" : mode;
  return MODE_TONE[m] || "mode";
}

// deviceIcon is the icon of a device's type: a radiator, a wall
// thermostat, a window or a leaf for the eco button.
function deviceIcon(d) {
  if (d.type === "wall_thermostat") return "icon-wall";
  if (d.type === "shutter_contact") return "icon-window";
  if (d.type === "eco_button") return "icon-leaf";
  return "icon-radiator";
}

// deviceProblems are what is wrong with a device, as problemsEl's items:
// a radio fault (the Cube could not send to it, or it lost its link) and
// a low battery; with window, also an open window, with its age.
function deviceProblems(d, window) {
  const s = d.state || {};
  const out = [];
  if (!d.reported) return out;
  if (s.error) out.push({ kind: "radio", icon: PROBLEM_KINDS.radio, tone: "bad", text: t("badge.transmitError") });
  if (s.link_error) out.push({ kind: "radio", icon: PROBLEM_KINDS.radio, tone: "warn", text: t("badge.linkError") });
  if (s.battery_low) out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "warn", text: t("badge.batteryLow"), at: received(d, "battery_low"), stale: d.stale });
  if (window && s.window_open === true) {
    out.push({ kind: "window", icon: PROBLEM_KINDS.window, tone: "fill-bad", text: t("badge.open"), at: received(d, "window_open"), stale: d.stale });
  }
  return out;
}

// problemsLabel names the kinds of problems among items, for a screen
// reader and the tooltip.
function problemsLabel(items) {
  const kinds = Object.keys(PROBLEM_KINDS).filter((k) => items.some((it) => it.kind === k));
  return t("problem.label", { list: kinds.map((k) => t("problem." + k)).join(", ") });
}

// renderProblems shows in the tile's head what is wrong in the room, as
// icons that open the list of its devices with their problems: a radio
// fault, a low battery, an open window, and the thermostats' modes where
// they differ. The room's virtual wall thermostat is not listed: its
// flags are the wall thermostat's.
function renderProblems(el, room) {
  const modes = roomModes(room);
  const mixed = modes.length > 1;
  const entries = room.devices.map((d) => {
    const items = deviceProblems(d, true);
    if (mixed && isThermostat(d) && d.state && d.state.mode) {
      items.push({ kind: "mixed", icon: PROBLEM_KINDS.mixed, tone: modeTone(d.state.mode), mark: "warn", text: modeName(d.state.mode) });
    }
    return { icon: deviceIcon(d), title: deviceName(d), sub: d.serial || d.rf_address, items };
  });
  const box = el.querySelector(".room-problems");
  const p = problemsEl(entries, problemsLabel(entries.flatMap((e) => e.items))); // components.js
  if (p) box.appendChild(p);
}

// renderStale warns on the tile when a device of the room is stale, by
// how long each has not been seen.
function renderStale(el, room) {
  const stale = room.devices.filter((d) => d.stale);
  if (!room.stale && !stale.length) return;
  const note = el.querySelector(".stale-note");
  note.textContent = t("room.stale");
  const groups = new Map();
  for (const d of stale) {
    const k = staleText(d);
    if (!groups.has(k)) groups.set(k, []);
    groups.get(k).push(deviceName(d));
  }
  for (const [text, names] of groups) note.appendChild(document.createTextNode(" " + text + ": " + names.join(", ") + "."));
  note.classList.remove("hidden");
}

function renderSetpoint(el, room) {
  const ui = roomUI(room);
  const th = roomThermostat(room);
  const s = roomState(room);
  const row = el.querySelector(".setpoint-row");
  const modeGroup = el.querySelector(".mode-group");
  if (!th) {
    row.classList.add("hidden");
    modeGroup.classList.add("hidden");
    return;
  }
  const current = s && s.setpoint !== undefined ? s.setpoint : null;
  drawSetpoint(el, room);
  el.querySelector(".step-minus").addEventListener("click", () => stepSetpoint(room, -STEP));
  el.querySelector(".step-plus").addEventListener("click", () => stepSetpoint(room, STEP));
  el.querySelector(".pending-save").addEventListener("click", () => flushSetpoint(room));
  el.querySelector(".pending-cancel").addEventListener("click", () => cancelSetpoint(room));

  const modes = roomModes(room);
  const mixed = modes.length > 1;
  const mode = mixed ? null : s ? s.mode : null;
  const autoBtn = el.querySelector(".mode-auto");
  const manuBtn = el.querySelector(".mode-manual");
  const holidayBtn = el.querySelector(".mode-holiday");
  // With mixed modes no button is pressed; those in use are marked as
  // partly pressed, and the head's problems name the devices and their
  // modes (renderProblems).
  const pressed = [[autoBtn, "auto"], [manuBtn, "manual"], [holidayBtn, "vacation"]];
  for (const [btn, m] of pressed) {
    const on = mixed ? false : mode === m || (m === "manual" && mode === "boost");
    const part = mixed && modes.includes(m);
    btn.classList.toggle("active", on);
    btn.classList.toggle("partial", part);
    btn.setAttribute("aria-pressed", part ? "mixed" : on ? "true" : "false");
  }
  holidayBtn.addEventListener("click", () => openVacation(room, el));
  autoBtn.addEventListener("click", () => {
    leaveEditing(room);
    sendRoomSetpoint(room, { mode: "auto" }, t("label.auto"), "auto");
  });
  manuBtn.addEventListener("click", () => {
    // Manual keeps the room's current setpoint; a pending stepper value
    // is dropped. Without a known setpoint nothing is guessed: the user
    // picks a temperature with the stepper, which sends manual itself.
    leaveEditing(room);
    if (current === null) {
      showTileResult(room.id, t("room.noSetpoint"), false);
      return;
    }
    sendRoomSetpoint(room, { mode: "manual", temp: current }, t("label.manual", { temp: fmtNum(current, 1) }), "manual");
  });
  const vac = el.querySelector(".mode-vacation");
  if (mode === "vacation") {
    const until = s.until || "?";
    holidayBtn.title = t("room.holidayUntil", { until });
    vac.appendChild(textEl("span", "\u{1F9F3}", "suitcase"));
    vac.appendChild(document.createTextNode(" " + t("room.until", { until })));
    vac.title = t("room.holidayUntil", { until });
    vac.classList.remove("hidden");
  }
}

// drawSetpoint shows the tile's setpoint, or the stepper's value that
// waits to be sent (pending), with Save and Cancel for it.
function drawSetpoint(el, room) {
  const ui = roomUI(room);
  const s = roomState(room);
  const current = s && s.setpoint !== undefined ? s.setpoint : null;
  const big = el.querySelector(".setpoint-big");
  big.textContent = fmtTemp(ui.pending !== null ? ui.pending : current, "°");
  big.classList.toggle("pending", ui.pending !== null);
  el.querySelector(".pending-row").classList.toggle("hidden", ui.pending === null);
}

// SENDING_BUTTON is, per write of a tile (roomUI(room).sending), the
// button that turns while it runs; "device:<rf>" is a device's Lock.
const SENDING_BUTTON = { auto: ".mode-auto", manual: ".mode-manual", vacation: ".mode-holiday", setpoint: ".pending-save", boost: ".menu-boost", lock: ".menu-lock", unlock: ".menu-unlock" };

// WRITE_BUTTONS are the buttons of a tile that send, or change what is
// to be sent: all off while one of the tile's writes runs.
const WRITE_BUTTONS = ".mode-group button, .stepper button, .pending-row button, .vacation-panel button[type=submit], .menu-boost, .menu-lock, .menu-unlock, .device-lock";

function sendingSelector(key) {
  if (key && key.startsWith("device:")) return '.device[data-rf="' + key.slice(7) + '"] .device-lock';
  return SENDING_BUTTON[key] || "";
}

// markSending draws the tile's write buttons off while one of its writes
// runs, the one sent turning (setBusy). renderRoom calls it, and
// tileWrite on the tile in place at the start and the end of a write,
// so that the tile's result line stays.
function markSending(el, room) {
  const key = roomUI(room).sending;
  const sel = sendingSelector(key);
  for (const b of el.querySelectorAll(WRITE_BUTTONS)) {
    if (key && sel && b.matches(sel)) {
      setBusy(b, true);
    } else {
      setBusy(b, false);
      b.disabled = !!key;
    }
  }
}

// redrawSending applies the tile's write state to its tile in place.
function redrawSending(room) {
  const el = document.querySelector('.room[data-id="' + room.id + '"]');
  if (!el) return;
  drawSetpoint(el, room);
  markSending(el, room);
}

// tileWrite runs fn, a write of the room's tile, as key: the tile's
// write buttons are off meanwhile (markSending), so that a double click
// sends one radio command; a write asked for while one runs sends
// nothing and returns null.
async function tileWrite(room, key, fn) {
  const ui = roomUI(room);
  if (ui.sending) return null;
  ui.sending = key;
  redrawSending(room);
  try {
    return await fn();
  } finally {
    ui.sending = null;
    redrawSending(room);
  }
}

// renderMenu wires the tile's ⋯ menu: lock and unlock of the room's
// thermostats, and the deletion of an empty room. A room with devices
// but no thermostat has nothing to offer, so its menu is hidden.
function renderMenu(el, room) {
  const thermostats = room.devices.filter(isThermostat);
  const lock = el.querySelector(".menu-lock");
  const unlock = el.querySelector(".menu-unlock");
  const del = el.querySelector(".menu-delete");
  const boost = el.querySelector(".menu-boost");
  if (thermostats.length) {
    // A boost opens the valves by the thermostats' boost settings for
    // their boost time; they then return to the mode they had.
    boost.addEventListener("click", () => {
      closeMenu(el);
      leaveEditing(room);
      sendRoomSetpoint(room, { mode: "boost" }, t("label.boost"), "boost");
    });
    lock.addEventListener("click", () => lockRoom(room, true, el));
    unlock.addEventListener("click", () => lockRoom(room, false, el));
  } else {
    boost.classList.add("hidden");
    lock.classList.add("hidden");
    unlock.classList.add("hidden");
  }
  if (thermostats.length === 0 && room.devices.length > 0) {
    el.querySelector(".menu").classList.add("hidden");
  }
  if (room.devices.length === 0) {
    del.addEventListener("click", () => {
      closeMenu(el);
      roomUI(room).confirm = true;
      rerenderRoom(room);
      const no = document.querySelector('.room[data-id="' + room.id + '"] .room-confirm .confirm-no');
      if (no) no.focus();
    });
  } else {
    del.classList.add("hidden");
  }
  dropdown(el.querySelector(".menu"));
}

// renderTools wires the info and cog buttons of the mode row: the device
// list and the configuration area, one of them open at a time, the
// open one highlighted, a click on it closes it.
function renderTools(el, room) {
  const ui = roomUI(room);
  const info = el.querySelector(".tile-info");
  const cog = el.querySelector(".tile-cog");
  const cfgOpen = !!(ui.config && ui.config.open);
  info.classList.toggle("active", ui.open);
  info.setAttribute("aria-expanded", ui.open ? "true" : "false");
  cog.classList.toggle("active", cfgOpen);
  cog.setAttribute("aria-expanded", cfgOpen ? "true" : "false");
  info.addEventListener("click", () => toggleDevices(room, el));
  cog.addEventListener("click", () => {
    if (cfgOpen) closeConfig(room);
    else openConfig(room, el);
  });
}

function closeMenu(el) {
  const m = el && el.querySelector("details.menu");
  if (m) m.open = false;
}

// renderVacation wires the inline vacation panel of a tile.
function renderVacation(el, room) {
  const ui = roomUI(room);
  const panel = el.querySelector(".vacation-panel");
  panel.classList.toggle("hidden", !ui.vacation);
  const temp = panel.querySelector(".vacation-temp");
  const until = panel.querySelector(".vacation-until");
  if (ui.vacation) {
    const s = roomState(room);
    if (!temp.dataset.set) {
      temp.value = ui.vacationTemp !== undefined ? ui.vacationTemp : (s && s.setpoint !== undefined ? s.setpoint : 17);
      until.value = ui.vacationUntil || tomorrowMorning();
      temp.dataset.set = "1";
    }
  }
  const keep = () => {
    ui.vacationTemp = parseFloat(temp.value);
    ui.vacationUntil = until.value;
  };
  temp.addEventListener("change", keep);
  until.addEventListener("change", keep);
  panel.querySelector(".vacation-minus").addEventListener("click", () => {
    temp.value = clampTemp(parseFloat(temp.value) - STEP);
    keep();
  });
  panel.querySelector(".vacation-plus").addEventListener("click", () => {
    temp.value = clampTemp(parseFloat(temp.value) + STEP);
    keep();
  });
  panel.querySelector(".vacation-cancel").addEventListener("click", () => closeVacation(room));
  panel.addEventListener("submit", (ev) => {
    ev.preventDefault();
    keep();
    const v = clampTemp(parseFloat(temp.value));
    const u = apiUntil(until.value);
    if (!u) return;
    closeVacation(room);
    sendRoomSetpoint(room, { mode: "vacation", temp: v, until: u }, t("label.vacation", { temp: fmtNum(v, 1), until: u }), "vacation");
  });
}

// leaveEditing closes the tile's vacation panel and drops a pending
// stepper value, for a mode button that takes over.
function leaveEditing(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  ui.pending = null;
  ui.vacation = false;
  ui.vacationTemp = undefined;
  ui.vacationUntil = undefined;
  rerenderRoom(room);
}

function openVacation(room, el) {
  closeMenu(el);
  const ui = roomUI(room);
  ui.vacation = true;
  ui.vacationTemp = undefined;
  ui.vacationUntil = undefined;
  rerenderRoom(room);
  const tile = document.querySelector('.room[data-id="' + room.id + '"]');
  const input = tile && tile.querySelector(".vacation-until");
  if (input) input.focus();
}

function closeVacation(room) {
  const ui = roomUI(room);
  ui.vacation = false;
  ui.vacationTemp = undefined;
  ui.vacationUntil = undefined;
  rerenderRoom(room);
}

// renderDeleteConfirm asks, while the room's deletion is asked for,
// whether to delete it (confirmBox): Keep or Delete.
function renderDeleteConfirm(el, room) {
  const ui = roomUI(room);
  if (!ui.confirm) return;
  el.querySelector(".room-confirm").appendChild(confirmBox(t("room.deleteConfirm"), { yes: t("action.delete"), no: t("action.keep") }, () => {
    ui.confirm = false;
    return deleteRoom(room);
  }, () => {
    ui.confirm = false;
    rerenderRoom(room);
  }));
}

// --- devices ----------------------------------------------------------

// deviceBadges lists the badges in words of a device: not reported,
// stale and locked, outlined, then its mode or window state as a filled
// badge, always the last one; its problems are icons (deviceProblems).
// Each is [text, kind, the time of the value for its age].
function deviceBadges(d) {
  const s = d.state || {};
  const out = [];
  if (!d.reported) {
    out.push([t("badge.notReported"), "warn"]);
    return out;
  }
  if (d.stale) out.push([staleText(d), "warn stale-badge"]);
  if (s.panel_locked) out.push([t("badge.locked"), "warn"]);
  if (s.window_open !== undefined) {
    const at = received(d, "window_open");
    out.push(s.window_open ? [t("badge.open"), "fill-bad", at] : [t("badge.closed"), "fill-ok", at]);
  } else if (s.mode && isThermostat(d)) {
    out.push([modeName(s.mode), modeTone(s.mode)]); // a boost says so, coloured as manual
  }
  return out;
}

// valueCell builds one cell of the values grid: label, number, unit, an
// optional icon and the value's age after it. The number sits
// right-aligned in a fixed-width span so the decimal points of a column
// line up; an empty label makes a spacer. Temperatures (icon given) are
// bold.
function valueCell(label, value, unit, icon, age) {
  const cell = textEl("span", "", "cell");
  cell.appendChild(textEl("span", label, "k"));
  cell.appendChild(textEl("span", value, "num" + (icon ? " temp" : "")));
  cell.appendChild(textEl("span", value === "" ? "" : unit, "u"));
  if (icon && value !== "") cell.appendChild(iconEl(icon, "di"));
  if (age && value !== "") {
    if (!icon) cell.appendChild(textEl("span", "")); // the icon's column
    cell.appendChild(age);
    cell.classList.add("aged");
  }
  return cell;
}

// deviceValues fills the two-column values grid of a thermostat: valve
// and offset on the left, set and actual on the right.
function deviceValues(grid, d, offset) {
  const s = d.state || {};
  const left = [];
  if (s.valve_pct !== undefined) left.push([t("val.valve"), String(s.valve_pct), "%", undefined, ageEl(received(d, "valve_pct"), d.stale)]);
  if (isThermostat(d)) {
    const off = offset === undefined ? "" : (offset > 0 ? "+" : "") + fmtNum(offset, 1);
    left.push([t("val.offset"), off, "°C"]);
  }
  const right = [];
  right.push([t("val.set"), fmtTemp(s.setpoint, ""), "°C", "icon-set"]);
  right.push([t("val.actual"), fmtTemp(s.actual_temp, ""), "°C", "icon-is", ageEl(received(d, "actual_temp"), d.stale)]);
  clear(grid);
  const rows = Math.max(left.length, right.length);
  for (let i = 0; i < rows; i++) {
    const l = left[i] || ["", "", ""];
    const r = right[i] || ["", "", ""];
    grid.appendChild(valueCell(l[0], l[1], l[2], l[3], l[4]));
    grid.appendChild(valueCell(r[0], r[1], r[2], r[3], r[4]));
  }
  if (s.until) {
    // The vacation end spans the grid, under the temperatures.
    const cell = textEl("span", "", "cell wide");
    cell.appendChild(textEl("span", t("val.until"), "k"));
    cell.appendChild(textEl("span", s.until, ""));
    grid.appendChild(cell);
  }
  grid.classList.toggle("hidden", rows === 0);
  grid.classList.toggle("stale", !!d.stale);
}

// renderDevice draws a device of a room's list: its type's icon before
// its name, its problems as icons that list them (problemsEl), then its
// badges in words; in expert mode its type, serial and radio address.
function renderDevice(d) {
  const el = fromTemplate("device-template");
  el.dataset.rf = d.rf_address;
  const type = el.querySelector(".device-type-icon");
  type.title = typeName(d.type);
  type.setAttribute("aria-label", typeName(d.type));
  type.appendChild(iconEl(deviceIcon(d), "ni"));
  // Type and radio address are for expert mode (.device-sub); a device
  // without a name keeps its S/N, the only way to tell it apart.
  el.querySelector(".device-name").textContent = d.name || t("device.unnamed") + " · " + (d.serial || d.rf_address);
  el.querySelector(".device-type").textContent = typeName(d.type);
  const ids = el.querySelector(".device-rf");
  clear(ids);
  ids.appendChild(deviceIds(d.serial, d.rf_address));
  const badges = el.querySelector(".device-badges");
  const items = deviceProblems(d, false);
  const problems = problemsEl([{ icon: deviceIcon(d), title: deviceName(d), sub: d.serial || d.rf_address, items }], problemsLabel(items));
  if (problems) badges.appendChild(problems);
  for (const [label, kind, at] of deviceBadges(d)) {
    const b = badgeEl(label, kind);
    const age = ageEl(at, d.stale);
    if (age) b.appendChild(age);
    badges.appendChild(b);
  }
  const seen = Date.parse(d.last_seen || "");
  if (!isNaN(seen)) {
    const line = t("age.lastSeen", { time: fmtShortTime(seen) });
    el.querySelector(".device-sub").appendChild(textEl("span", line, "device-seen expert-only"));
    const sb = badges.querySelector(".stale-badge");
    if (sb) sb.title = line;
  }
  const grid = el.querySelector(".device-vals");
  if (isThermostat(d) && d.reported) {
    deviceValues(grid, d, undefined);
    deviceDetail(d).then((detail) => {
      const cfg = detail.thermostat || detail.wall_thermostat;
      if (cfg && cfg.temp_offset !== undefined) deviceValues(grid, d, cfg.temp_offset);
    }).catch(() => {});
  }
  const actions = el.querySelector(".device-actions");
  if (isThermostat(d)) {
    const locked = !!(d.state && d.state.panel_locked);
    const lock = button(t(locked ? "device.unlock" : "device.lock"), "needs-lock device-lock", () => lockDevice(d, !locked, lock));
    actions.appendChild(lock);
  }
  return el;
}

function fillDevices(container, room) {
  clear(container);
  if (room.devices.length === 0) container.appendChild(textEl("p", t("room.noDevices"), "muted"));
  for (const d of room.devices) container.appendChild(renderDevice(d));
  if (room.vwt) container.appendChild(renderVirtual(room)); // expert mode's, after the real devices
}

// renderVirtual is the room's virtual wall thermostat in expert mode:
// its values as a device's, what they come from, and its two settings,
// how a room without a wall thermostat takes its temperature and its
// radio address.
function renderVirtual(room) {
  const v = room.vwt;
  const el = renderDevice(v);
  el.classList.add("virtual", "expert-only");
  for (const b of el.querySelectorAll(".device-lock")) b.remove(); // the real wall thermostat's
  el.querySelector(".device-type").textContent = t("vwt.type");
  const type = el.querySelector(".device-type-icon");
  type.title = t("vwt.type");
  type.setAttribute("aria-label", t("vwt.type"));
  type.querySelector("use").setAttribute("href", "#icon-room");
  // the sources by their serial, the radio address of one not in the room
  const known = new Map(room.devices.map((d) => [d.rf_address, d]));
  const from = ((v.sources && v.sources.actual_temp) || []).map((rf) => (known.get(rf) && known.get(rf).serial) || rf);
  if (from.length) el.querySelector(".device-sub").appendChild(textEl("span", t("vwt.from", { devices: from.join(", ") }), "device-seen"));

  const form = document.createElement("form");
  form.className = "vwt-settings needs-configure";
  const method = document.createElement("select");
  method.name = "temp_method";
  for (const m of ["median", "mean"]) {
    const o = document.createElement("option");
    o.value = m;
    o.textContent = t("vwt." + m);
    method.appendChild(o);
  }
  const addr = document.createElement("input");
  addr.name = "rf_address";
  addr.maxLength = 6;
  addr.pattern = "[0-9a-fA-F]{6}";
  addr.placeholder = v.rf_address;
  const result = elem("p", "tile-result hidden");
  deviceDetail(v).then((detail) => {
    const s = (detail && detail.settings) || {};
    method.value = s.temp_method || "median";
    addr.value = s.rf_address || "";
  }).catch(() => {});
  form.append(
    fieldRow(t("vwt.method"), method, { hint: t("vwt.methodHint") }),
    fieldRow(t("vwt.address"), addr, { hint: t("vwt.addressHint") }),
  );
  const actions = elem("div", "actions");
  actions.appendChild(submitButton(t("action.save")));
  form.append(actions, result);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      await api("PUT", devicePath(v) + "/config", { temp_method: method.value, rf_address: addr.value.trim().toLowerCase() });
      state.details = {};
      showResult(result, t("vwt.saved"), true);
    } catch (err) {
      showResult(result, err.message, false);
    }
  });
  el.appendChild(form);
  return el;
}

function toggleDevices(room, el) {
  const ui = roomUI(room);
  ui.open = !ui.open;
  if (ui.open) ui.config = null; // one of device list and configuration at a time
  closeMenu(el);
  rerenderRoom(room);
}

// rerenderRoom replaces a room's tile in place, keeping its UI state.
// Removing a tile blurs a focused input, whose change handler may ask
// for another render; that request is deferred until this one is done.
let rendering = false;
function rerenderRoom(room) {
  if (rendering) {
    queueMicrotask(() => rerenderRoom(room));
    return;
  }
  const old = document.querySelector('.room[data-id="' + room.id + '"]');
  if (!old) return;
  rendering = true;
  try {
    old.replaceWith(renderRoom(room));
  } finally {
    rendering = false;
  }
}

// configFocused reports whether the focus is in a room tile's
// configuration.
function configFocused(room) {
  const area = document.querySelector('.room[data-id="' + room.id + '"] .config-area');
  return !!(area && area.contains(document.activeElement));
}

// updateDevice applies a state event to the room or the unlisted list.
// A tile whose configuration has the focus is not drawn again under the
// user's typing: the state is taken, and drawn once the focus leaves the
// configuration (renderRoom).
function updateDevice(d) {
  if (d.virtual) {
    const r = state.rooms.find((x) => x.id === d.room_id);
    if (!r) return;
    r.vwt = d;
    if (configFocused(r)) roomUI(r).deferred = true;
    else rerenderRoom(r);
    return;
  }
  for (const r of state.rooms) {
    const i = r.devices.findIndex((x) => x.rf_address === d.rf_address);
    if (i >= 0) {
      r.devices[i] = d;
      r.ready = r.devices.every((x) => x.reported);
      r.stale = r.devices.some((x) => x.stale);
      if (configFocused(r)) roomUI(r).deferred = true;
      else rerenderRoom(r);
      return;
    }
  }
  const j = state.unlisted.findIndex((x) => x.rf_address === d.rf_address);
  if (j >= 0) {
    state.unlisted[j] = d;
    const old = document.querySelector('#unlisted-devices .device[data-rf="' + d.rf_address + '"]');
    // A form being typed in keeps its row; the next render brings the state.
    if (old && !(unlistedUI[d.rf_address] || {}).form) old.replaceWith(renderUnlisted(d));
  }
}

// --- results ----------------------------------------------------------

function resultText(label, res) {
  if (res && res.discarded) return t("result.discarded", { label });
  let text = t("result.ok", { label });
  if (res && res.budget) text = t("result.budget", { text, duty: res.budget.duty_cycle_pct, slots: res.budget.free_memory_slots });
  return text;
}

function errorText(label, e) {
  let text = t("result.error", { label, error: e.message });
  if (e.data && e.data.no_state) {
    text += " " + t("result.noState", { devices: e.data.no_state.join(", ") });
  }
  return text;
}

// modeName is a device mode as a word, typeName a device type.
function modeName(mode) {
  const key = "mode.name." + mode;
  const s = t(key);
  return s === key ? mode : s;
}

function typeName(type) {
  const key = "type." + type;
  const s = t(key);
  return s === key ? type.replace(/_/g, " ") : s;
}

// showTileResult says how an action went on the room's tile (or the
// page's message line when the tile is gone); a success fades.
function showTileResult(roomId, text, ok) {
  const tile = document.querySelector('.room[data-id="' + roomId + '"]');
  if (!tile) {
    showMessage(text, ok);
    return;
  }
  const p = tile.querySelector(".tile-result");
  showResult(p, text, ok);
  if (ok) {
    setTimeout(() => {
      if (p.textContent === text) p.classList.add("hidden");
    }, RESULT_FADE_MS);
  }
}

// --- actions ----------------------------------------------------------

function stepSetpoint(room, delta) {
  const ui = roomUI(room);
  const s = roomState(room);
  const from = ui.pending !== null ? ui.pending : (s && s.setpoint !== undefined ? s.setpoint : DEFAULT_TEMP);
  ui.pending = clampTemp(from + delta);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = setTimeout(() => flushSetpoint(room), SEND_DELAY_MS);
  rerenderRoom(room);
}

function cancelSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  ui.pending = null;
  rerenderRoom(room);
}

// flushSetpoint sends the stepper's value, which stays shown, with Save
// turning, until the answer; a value whose time comes while another
// write of the tile runs waits for that one's answer.
async function flushSetpoint(room) {
  const ui = roomUI(room);
  if (ui.timer) clearTimeout(ui.timer);
  ui.timer = null;
  if (ui.pending === null) return;
  if (ui.sending) {
    ui.timer = setTimeout(() => flushSetpoint(room), SEND_DELAY_MS);
    return;
  }
  const temp = ui.pending;
  await tileWrite(room, "setpoint", async () => {
    try {
      return await setpointRequest(room, { mode: "manual", temp }, t("label.manual", { temp: fmtNum(temp, 1) }));
    } finally {
      ui.pending = null;
    }
  });
}

// sendRoomSetpoint sends a mode or a setpoint of the tile as the write
// key (tileWrite).
function sendRoomSetpoint(room, body, label, key) {
  return tileWrite(room, key, () => setpointRequest(room, body, label));
}

async function setpointRequest(room, body, label) {
  showTileResult(room.id, t("result.waiting", { label }), true);
  try {
    const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
    showTileResult(room.id, resultText(label, res), !(res && res.discarded));
    return res;
  } catch (e) {
    showTileResult(room.id, errorText(label, e), false);
    return null;
  } finally {
    loadCubes().catch(() => {});
  }
}

async function setLock(d, locked) {
  const label = t(locked ? "label.lock" : "label.unlock", { name: deviceName(d) });
  const roomId = d.room_id;
  showTileResult(roomId, t("result.waiting", { label }), true);
  try {
    const res = await api("PUT", devicePath(d) + "/lock", { locked });
    showTileResult(roomId, resultText(label, res), !(res && res.discarded));
  } catch (e) {
    showTileResult(roomId, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
}

async function lockRoom(room, locked, el) {
  closeMenu(el);
  await tileWrite(room, locked ? "lock" : "unlock", async () => {
    for (const d of room.devices.filter(isThermostat)) {
      await setLock(d, locked);
    }
  });
}

// lockDevice locks or unlocks one device from its row, as a write of
// its room's tile, or with its button busy for a device in no room.
function lockDevice(d, locked, btn) {
  const room = state.rooms.find((r) => r.id === d.room_id);
  if (room) return tileWrite(room, "device:" + d.rf_address, () => setLock(d, locked));
  return whileBusy(btn, () => setLock(d, locked));
}

async function deleteRoom(room) {
  try {
    await api("DELETE", base() + "/rooms/" + room.id);
    showMessage(t("room.deleted", { name: room.name }), true);
  } catch (e) {
    showMessage(t("room.deleteFailed", { name: room.name, error: e.message }), false);
  }
  await loadRooms();
}

// --- house ------------------------------------------------------------

function housePanel() {
  return $("house-panel");
}

function houseMode() {
  const checked = document.querySelector('input[name="house-mode"]:checked');
  return checked ? checked.value : "auto";
}

function houseDuration() {
  const checked = document.querySelector('input[name="house-duration"]:checked');
  return checked ? checked.value : "permanent";
}

// houseRoomRow is a checkbox of the House panel with its label, from
// its template (tpl-house-room); label.control is the checkbox.
function houseRoomRow(label, checked, onChange) {
  const row = fromTemplate("tpl-house-room");
  const cb = row.control;
  cb.checked = !!checked;
  cb.addEventListener("change", () => onChange(cb.checked));
  row.append(" " + label);
  return row;
}

// renderHouseRooms fills the room checkboxes of the House panel: All
// rooms, then a room with a thermostat each.
function renderHouseRooms() {
  const box = $("house-rooms");
  const wasChecked = {};
  for (const cb of box.querySelectorAll("input[data-room]")) wasChecked[cb.dataset.room] = cb.checked;
  clear(box);
  const rooms = () => [...box.querySelectorAll("input[data-room]")];
  const all = houseRoomRow(t("house.allRooms"), true, (on) => {
    for (const cb of rooms()) cb.checked = on;
  });
  all.control.id = "house-all";
  box.appendChild(all);
  for (const room of state.rooms) {
    if (!roomThermostat(room)) continue;
    const row = houseRoomRow(room.name, wasChecked[room.id] === undefined ? true : wasChecked[room.id], () => {
      all.control.checked = rooms().every((x) => x.checked);
    });
    row.control.dataset.room = String(room.id);
    box.appendChild(row);
  }
  all.control.checked = rooms().every((x) => x.checked);
  if (houseSendCheck) houseSendCheck();
}

// houseSendCheck keeps the House panel's Send off while no room is
// ticked (sendWhenChanged), set by initTiles.
let houseSendCheck = null;

// updateHousePanel shows or hides the parts that depend on the mode.
function updateHousePanel() {
  const mode = houseMode();
  $("house-temp-row").classList.toggle("hidden", mode !== "manual");
  $("house-duration-row").classList.toggle("hidden", mode === "auto");
  $("house-until").disabled = houseDuration() !== "until";
}

function openHouse(mode) {
  const panel = housePanel();
  for (const r of document.querySelectorAll('input[name="house-mode"]')) r.checked = r.value === mode;
  if (!$("house-until").value) $("house-until").value = tomorrowMorning();
  const first = state.rooms.find(roomThermostat);
  const s = first ? roomState(first) : null;
  if (s && s.setpoint !== undefined) $("house-temp").value = s.setpoint;
  $("house-result").classList.add("hidden");
  panel.classList.remove("hidden");
  updateHousePanel();
  const focus = mode === "manual" ? $("house-temp") : panel.querySelector('input[name="house-mode"]:checked');
  if (focus) focus.focus();
}

function closeHouse() {
  housePanel().classList.add("hidden");
}

// sendHouse applies the panel: one room command after another, with the
// per-room results in the panel.
async function sendHouse() {
  const submit = housePanel().querySelector("button[type=submit]");
  await whileBusy(submit, houseRequests);
}

async function houseRequests() {
  const mode = houseMode();
  const until = houseDuration() === "until" && mode !== "auto" ? apiUntil($("house-until").value) : "";
  const manualTemp = clampTemp(parseFloat($("house-temp").value) || DEFAULT_TEMP);
  const ids = [...document.querySelectorAll("#house-rooms input[data-room]:checked")].map((cb) => Number(cb.dataset.room));
  const out = $("house-result");
  if (ids.length === 0) {
    showResult(out, t("house.noRoom"), true);
    return;
  }
  showResult(out, t("house.sending"), true);
  const results = [];
  let failed = false;
  for (const room of state.rooms.filter((r) => ids.includes(r.id))) {
    const th = roomThermostat(room);
    if (!th) continue;
    try {
      let temp = manualTemp;
      if (mode === "eco" || mode === "comfort") {
        const detail = await deviceDetail(th);
        const cfg = detail.thermostat || detail.wall_thermostat || {};
        temp = mode === "eco" ? cfg.eco_temp : cfg.comfort_temp;
        if (temp === undefined) throw new Error(t(mode === "eco" ? "house.noEco" : "house.noComfort"));
      }
      let body = { mode: "auto" };
      if (mode !== "auto") body = until ? { mode: "vacation", temp, until } : { mode: "manual", temp };
      const res = await api("PUT", base() + "/rooms/" + room.id + "/setpoint", body);
      const what = mode === "auto" ? t("house.auto") : t(until ? "house.tempUntil" : "house.temp", { temp: fmtNum(temp, 1), until });
      results.push(res && res.discarded ? t("house.roomDiscarded", { room: room.name }) : t("house.roomOk", { room: room.name, what }));
      if (res && res.discarded) failed = true;
    } catch (e) {
      results.push(t("house.roomError", { room: room.name, error: e.message }));
      failed = true;
    }
    showResult(out, results.join("\n"), true);
  }
  showResult(out, out.textContent, !failed);
  loadCubes().catch(() => {});
}

// closePanels closes every inline panel (vacation, delete confirm,
// House), for the Escape key; dropdown (components.js) closes the menus.
function closePanels() {
  if (!housePanel().classList.contains("hidden")) closeHouse();
  for (const room of state.rooms) {
    const ui = state.ui[room.id];
    if (!ui) continue;
    if (ui.vacation || ui.confirm) {
      ui.vacation = false;
      ui.vacationTemp = undefined;
      ui.vacationUntil = undefined;
      ui.confirm = false;
      rerenderRoom(room);
    }
  }
}

// --- wiring -----------------------------------------------------------

function initTiles() {
  $("house-eco").addEventListener("click", () => openHouse("eco"));
  $("house-auto").addEventListener("click", () => openHouse("auto"));
  $("house-comfort").addEventListener("click", () => openHouse("comfort"));
  $("house-manu").addEventListener("click", () => openHouse("manual"));
  $("house-cancel").addEventListener("click", closeHouse);
  housePanel().addEventListener("submit", (ev) => {
    ev.preventDefault();
    sendHouse();
  });
  housePanel().addEventListener("change", updateHousePanel);
  houseSendCheck = sendWhenChanged(housePanel(), () => !!document.querySelector("#house-rooms input[data-room]:checked"));
  $("house-temp-minus").addEventListener("click", () => {
    $("house-temp").value = clampTemp(parseFloat($("house-temp").value) - STEP);
  });
  $("house-temp-plus").addEventListener("click", () => {
    $("house-temp").value = clampTemp(parseFloat($("house-temp").value) + STEP);
  });
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    closePanels();
  });
  document.addEventListener("click", (ev) => {
    // Clicking outside closes the House panel once it has sent (and an
    // open menu, dropdown in components.js).
    const panel = housePanel();
    if (!panel.classList.contains("hidden") && !ev.target.closest("#house") && !$("house-result").classList.contains("hidden")) {
      closeHouse();
    }
  });
}
