// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, tiles: one tile per room with the setpoint
// stepper, the mode buttons with the More menu (Eco, Comfort, Hold,
// Holiday…) and the inline panels (vacation, delete),
// the House tile with its panel, the device list, the devices without a
// room with Put into a room… and Remove from the Cube, and the actions
// behind them. Uses the state, api and helpers of app.js, and
// pairDeviceForm of cubes.js.
"use strict";

// --- rendering --------------------------------------------------------

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
// order there, each with its icon (index.html's sprite): a sign where
// its state has a mark (components.js signOf), the radio and the window,
// drawn as the person chooses (problemIcon); and a holiday, which is no
// problem but is listed with its end.
const PROBLEM_KINDS = {
  radio: null,
  battery: "icon-battery",
  window: null,
  mixed: "icon-shuffle",
  holiday: "icon-suitcase",
};

// PROBLEM_SIGNS are the kinds of PROBLEM_KINDS drawn as signs.
const PROBLEM_SIGNS = { radio: "radio", window: "window" };

// problemIcon is a kind's icon: its sign's, as the person chooses it,
// for the radio and the window.
function problemIcon(kind) {
  return PROBLEM_SIGNS[kind] ? signOf(PROBLEM_SIGNS[kind]).icon : PROBLEM_KINDS[kind];
}

// windowState is an open window's state by the page's clock and the
// window's times (the server's limits): "open" until its warning time,
// "warn" until its alert time, "bad" after; "" for a window not open.
function windowState(d) {
  const s = d.state || {};
  if (s.window_open !== true) return "";
  const w = d.window;
  if (!w) return "open";
  const now = Date.now();
  if (now >= Date.parse(w.alert_at)) return "bad";
  if (now >= Date.parse(w.warn_at)) return "warn";
  return "open";
}

// WINDOW_TONE is the badge of an open window's state: filled blue while
// it is open briefly, an amber outline (a warning) once it is open long,
// filled red (a failure) once it is open too long; a closed window's is
// filled green.
const WINDOW_TONE = { open: "fill-open", warn: "warn", bad: "fill-bad" };

// planWindowRedraw draws again, when an open window's state changes by
// the clock alone, the tiles of the rooms whose window changes then.
let windowTimer = null;
function planWindowRedraw() {
  if (windowTimer) clearTimeout(windowTimer);
  windowTimer = null;
  const now = Date.now();
  let next = Infinity;
  for (const r of state.rooms) {
    for (const d of r.devices) {
      if (!d.window || windowState(d) === "") continue;
      for (const at of [Date.parse(d.window.warn_at), Date.parse(d.window.alert_at)]) {
        if (at > now && at < next) next = at;
      }
    }
  }
  if (next === Infinity) return;
  windowTimer = setTimeout(() => {
    windowTimer = null;
    for (const r of state.rooms) {
      if (!r.devices.some((d) => d.window)) continue;
      if (configFocused(r)) roomUI(r).deferred = true;
      else rerenderRoom(r);
    }
    planWindowRedraw();
  }, Math.min(next - now + 250, 2 ** 31 - 1));
}

// MODE_TONE colours a mode's badge: auto green, manual (and boost) amber,
// holiday violet.
const MODE_TONE = { auto: "fill-ok", manual: "fill-warn", holiday: "fill-info" };

// modeTone is the badge kind of a thermostat's mode.
function modeTone(mode) {
  const m = mode === "vacation" ? "holiday" : mode === "boost" ? "manual" : mode;
  return MODE_TONE[m] || "mode";
}

// deviceIcon is the icon of a device's type: a radiator, a wall
// thermostat, a window as the person chooses it, closed, or a leaf for
// the eco button.
function deviceIcon(d) {
  if (d.type === "wall_thermostat") return "icon-wall";
  if (d.type === "shutter_contact") return iconChoice("window").type; // components.js
  if (d.type === "eco_button") return "icon-leaf";
  return "icon-radiator";
}

// deviceProblems are what is wrong with a device, as problemsEl's items:
// a radio fault (the Cube could not send to it, or it lost its link) and
// a weak or low battery; withWindow, also an open window, with its age.
function deviceProblems(d, withWindow) {
  const s = d.state || {};
  const out = [];
  if (!d.reported) return out;
  if (s.error) out.push({ kind: "radio", icon: problemIcon("radio"), sign: PROBLEM_SIGNS.radio, tone: "bad", text: t("badge.transmitError") });
  if (s.link_error) out.push({ kind: "radio", icon: problemIcon("radio"), sign: PROBLEM_SIGNS.radio, tone: "warn", text: t("badge.linkError") });
  // The battery's level, the server's (internal/battery): weak amber, low
  // red; a device of a server without levels by its flag, amber.
  if (d.battery === "low") out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "bad", text: t("badge.batteryLow"), at: received(d, "battery_low"), stale: d.stale });
  else if (d.battery === "weak") out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "warn", text: t("badge.batteryWeak"), at: received(d, "battery_low"), stale: d.stale });
  else if (d.battery === undefined && s.battery_low) out.push({ kind: "battery", icon: PROBLEM_KINDS.battery, tone: "warn", text: t("badge.batteryLow"), at: received(d, "battery_low"), stale: d.stale });
  if (withWindow && s.window_open === true) {
    out.push({ kind: "window", icon: problemIcon("window"), sign: PROBLEM_SIGNS.window, tone: WINDOW_TONE[windowState(d)], text: t("badge.open"), at: received(d, "window_open"), stale: d.stale });
  }
  return out;
}

// problemsLabel names the kinds of problems among items, and a holiday
// with its end, for a screen reader and the tooltip.
function problemsLabel(items) {
  const kinds = Object.keys(PROBLEM_KINDS).filter((k) => k !== "holiday" && items.some((it) => it.kind === k));
  const parts = kinds.length ? [t("problem.label", { list: kinds.map((k) => t("problem." + k)).join(", ") })] : [];
  const holiday = items.find((it) => it.kind === "holiday");
  if (holiday) parts.push(holiday.text);
  return parts.join("; ");
}

// renderProblems shows in the tile's head what is wrong in the room, as
// icons that open the list of its devices with their problems: a radio
// fault, a low battery, an open window, and the thermostats' modes where
// they differ; a thermostat on holiday with the holiday's end, in its
// mode's colour, in place of its mode among those that differ. The
// room's virtual wall thermostat is not listed: its flags are the wall
// thermostat's.
function renderProblems(el, room) {
  const modes = roomModes(room);
  const mixed = modes.length > 1;
  const entries = room.devices.map((d) => {
    const items = deviceProblems(d, true);
    const s = d.state || {};
    if (isThermostat(d) && s.mode === "vacation") {
      items.push({ kind: "holiday", icon: PROBLEM_KINDS.holiday, tone: modeTone(s.mode), text: t("room.holidayUntil", { until: fmtUntil(s.until) }) });
    } else if (mixed && isThermostat(d) && s.mode) {
      items.push({ kind: "mixed", icon: PROBLEM_KINDS.mixed, tone: modeTone(s.mode), mark: "warn", text: modeName(s.mode) });
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
  const moreSum = el.querySelector(".more-sum");
  // With mixed modes no button is pressed; those in use are marked as
  // partly pressed, and the head's problems name the devices and their
  // modes (renderProblems). A holiday presses More.
  const pressed = [[autoBtn, "auto"], [manuBtn, "manual"]];
  for (const [btn, m] of pressed) {
    const on = mixed ? false : mode === m || (m === "manual" && mode === "boost");
    const part = mixed && modes.includes(m);
    btn.classList.toggle("active", on);
    btn.classList.toggle("partial", part);
    btn.setAttribute("aria-pressed", part ? "mixed" : on ? "true" : "false");
  }
  moreSum.classList.toggle("active", !mixed && mode === "vacation");
  moreSum.classList.toggle("partial", mixed && modes.includes("vacation"));
  if (mode === "vacation") {
    // On holiday, More says until when (D99), as the room's pill does.
    const label = t("mode.holidayUntil", { until: fmtUntil(s.until) });
    moreSum.querySelector(".more-label").textContent = label;
    moreSum.title = label;
  }
  renderMore(el, room, mode);
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
}

// renderMore wires the tile's More menu (D99): Eco and Comfort, the
// room thermostat's eco or comfort temperature in the room's mode
// (presetBody), each with its temperature once the menu has opened; the
// Hold switch while the room is in Auto, which makes them hold theirs in
// Manual; and Holiday…, the holiday form. The arrow keys move between
// the items.
function renderMore(el, room, mode) {
  const ui = roomUI(room);
  const menu = el.querySelector(".more-menu");
  const hold = menu.querySelector(".more-hold");
  const input = hold.querySelector("input");
  hold.classList.toggle("hidden", mode !== "auto");
  input.checked = ui.hold;
  input.addEventListener("change", () => {
    ui.hold = input.checked;
  });
  const items = { eco: menu.querySelector(".menu-eco"), comfort: menu.querySelector(".menu-comfort") };
  for (const [which, b] of Object.entries(items)) b.addEventListener("click", () => sendPreset(room, which));
  menu.querySelector(".menu-holiday").addEventListener("click", () => openVacation(room, el));
  menu.addEventListener("toggle", () => {
    if (!menu.open) return;
    presetTemps(roomThermostat(room)).then((temps) => {
      for (const [which, b] of Object.entries(items)) b.querySelector(".more-temp").textContent = temps[which] === undefined ? "" : fmtTemp(temps[which]);
    }).catch(() => {});
  });
  dropdown(menu);
  menuArrows(menu, ".menu-items button:not(:disabled), .menu-items .switch:not(.hidden) input");
}

// presetTemps are a thermostat's configured eco and comfort
// temperatures, { eco, comfort }, undefined where it has none.
async function presetTemps(th) {
  const detail = await deviceDetail(th);
  const cfg = detail.thermostat || detail.wall_thermostat || {};
  return { eco: cfg.eco_temp, comfort: cfg.comfort_temp };
}

// presetBody is the setpoint body that sets temp and keeps the room's
// mode (D99): in Auto until the next switch point, or in Manual where
// the tile's Hold switch is on; Manual and a boost hold it, a holiday
// keeps its end. A room whose thermostats differ takes its room
// thermostat's mode, and no Hold, which it does not show.
function presetBody(room, temp) {
  const s = roomState(room) || {};
  if (s.mode === "auto") return roomModes(room).length <= 1 && roomUI(room).hold ? { mode: "manual", temp } : { mode: "auto", temp };
  if (s.mode === "vacation" && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/.test(s.until || "")) return { mode: "vacation", temp, until: s.until };
  return { mode: "manual", temp };
}

// presetLabel names a preset's write by the body it sends.
function presetLabel(which, body) {
  const temp = fmtNum(body.temp, 1);
  const label = which === "eco" ? t("label.eco", { temp }) : t("label.comfort", { temp });
  if (body.mode === "auto") return t("label.presetAuto", { label });
  if (body.mode === "vacation") return t("label.presetUntil", { label, until: fmtUntil(body.until) });
  return t("label.presetManual", { label });
}

// sendPreset sets the room to its thermostat's eco or comfort
// temperature (which) in the room's mode, from the More menu, which
// keeps the focus.
function sendPreset(room, which) {
  leaveEditing(room);
  const sum = document.querySelector('.room[data-id="' + room.id + '"] .more-sum');
  if (sum) sum.focus();
  return tileWrite(room, which, async () => {
    const what = t(which === "eco" ? "room.eco" : "room.comfort");
    let temp;
    try {
      temp = (await presetTemps(roomThermostat(room)))[which];
    } catch (e) {
      showTileResult(room.id, errorText(what, e), false);
      return null;
    }
    if (temp === undefined) {
      showTileResult(room.id, t("result.error", { label: what, error: t(which === "eco" ? "house.noEco" : "house.noComfort") }), false);
      return null;
    }
    const body = presetBody(room, temp);
    return setpointRequest(room, body, presetLabel(which, body));
  });
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
// button that turns while it runs, More's summary for what its menu
// sent; "device:<rf>" is a device's Lock.
const SENDING_BUTTON = { auto: ".mode-auto", manual: ".mode-manual", vacation: ".more-sum", eco: ".more-sum", comfort: ".more-sum", setpoint: ".pending-save", boost: ".menu-boost", lock: ".menu-lock", unlock: ".menu-unlock" };

// WRITE_BUTTONS are the buttons of a tile that send, or change what is
// to be sent: all off while one of the tile's writes runs.
const WRITE_BUTTONS = ".mode-group button, .more-sum, .stepper button, .pending-row button, .vacation-panel button[type=submit], .menu-boost, .menu-lock, .menu-unlock, .device-lock";

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

// closeMenu closes the open menus of a tile: its ⋯, More and its
// problems.
function closeMenu(el) {
  if (!el) return;
  for (const m of el.querySelectorAll("details.menu[open]")) m.open = false;
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

function toggleDevices(room, el) {
  const ui = roomUI(room);
  ui.open = !ui.open;
  if (ui.open) ui.config = null; // one of device list and configuration at a time
  closeMenu(el);
  rerenderRoom(room);
}
