// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, configuration: the per-room configuration area
// (name, temperatures, device settings, week programme, copy to other
// rooms), inline in the tile, one screen per section with Send and
// Cancel. Uses the state, api and helpers of app.js and tiles.js.
"use strict";

const DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
const MAX_POINTS = 13;
const NAME_BYTES = 32;
// The common fields of a room's configuration, as GET /rooms/{id}/config
// returns them and PUT /rooms/{id}/config takes them.
const TEMP_FIELDS = ["comfort_temp", "eco_temp", "window_open_temp", "window_open_minutes", "max_setpoint", "min_setpoint"];
const VALVE_FIELDS = ["boost_minutes", "boost_valve_pct", "decalc_day", "decalc_hour", "max_valve_pct", "valve_offset_pct"];

// dayShort and dayLong name a weekday ("monday" …) in the language.
function dayShort(day) {
  return t("dayShort." + day);
}

function dayLong(day) {
  return t("day." + day);
}

function dayList(days) {
  return days.map(dayShort).join(", ");
}

// --- model ------------------------------------------------------------

function configUI(room) {
  return roomUI(room).config || null;
}

// openConfig opens the configuration area of a tile and loads the
// room's configuration and week programme.
async function openConfig(room, el, tab) {
  closeMenu(el);
  const ui = roomUI(room);
  ui.open = false; // one of device list and configuration at a time
  // The Room tab is a rename, which the role may not do: the temperatures then come first.
  ui.config = { open: true, tab: tab || (hasCap("rename") ? "room" : "temps"), loading: true, cfg: null, prog: null, edit: {}, result: null, sending: null };
  rerenderRoom(room);
  try {
    const [cfg, prog] = await Promise.all([
      api("GET", base() + "/rooms/" + room.id + "/config"),
      api("GET", base() + "/rooms/" + room.id + "/program").catch(() => null),
    ]);
    if (!ui.config) return;
    ui.config.cfg = cfg;
    ui.config.prog = programModel(prog);
    ui.config.edit = freshEdit(room, cfg);
    ui.config.loading = false;
  } catch (e) {
    if (!ui.config) return;
    ui.config.loading = false;
    ui.config.error = e.message;
  }
  rerenderRoom(room);
}

function closeConfig(room) {
  roomUI(room).config = null;
  rerenderRoom(room);
}

// freshEdit is the editable copy of what the configuration area shows.
function freshEdit(room, cfg) {
  const edit = { name: room.name, common: {}, devices: {} };
  for (const f of TEMP_FIELDS.concat(VALVE_FIELDS)) edit.common[f] = cfg ? cfg[f] : undefined;
  const byRf = {};
  for (const d of (cfg && cfg.devices) || []) byRf[d.rf_address] = d;
  for (const d of room.devices) {
    const c = byRf[d.rf_address] || {};
    edit.devices[d.rf_address] = {
      name: d.name || "",
      temp_offset: c.temp_offset,
      display_actual: c.display_actual,
      room_id: room.id,
      confirmMove: false,
    };
  }
  return edit;
}

// programModel turns the API's week programme into the editor's model:
// an editable copy per day and the loaded one (orig) for reset and for
// telling the changed days, the day being edited, the selected days
// (those with the edited day's programme, edited together unless
// "only" is on), and the days ticked in the copy section.
function programModel(prog) {
  const days = {};
  for (const day of DAYS) {
    const pts = prog && Array.isArray(prog[day]) ? prog[day] : [];
    days[day] = pts.map((p) => ({ temp: p.temp, until: p.until }));
    if (days[day].length === 0) days[day] = [{ temp: 17, until: "24:00" }];
  }
  const p = { days, orig: copyDays(days), source: prog ? prog.source : "", day: "monday", selected: new Set(), only: false, copyTo: new Set() };
  selectDay(p, "monday");
  return p;
}

function copyPoints(pts) {
  return pts.map((pt) => ({ temp: pt.temp, until: pt.until }));
}

function copyDays(days) {
  const out = {};
  for (const day of DAYS) out[day] = copyPoints(days[day]);
  return out;
}

function samePoints(a, b) {
  return a.length === b.length && a.every((pt, i) => pt.temp === b[i].temp && pt.until === b[i].until);
}

// dirtyDays lists the days whose programme differs from the loaded one.
function dirtyDays(p) {
  return DAYS.filter((d) => !samePoints(p.days[d], p.orig[d]));
}

// selectDay makes day the edited day and selects every day that has
// the same programme.
function selectDay(p, day) {
  p.day = day;
  p.selected = new Set(DAYS.filter((d) => samePoints(p.days[d], p.days[day])));
  for (const d of p.selected) p.copyTo.delete(d);
}

// programEdited is called after every change to the edited day: with
// "only" on, the change sets the day apart and the other days leave the
// selection; with it off, the change goes to every selected day.
function programEdited(p) {
  if (p.only) {
    p.selected = new Set([p.day]);
    return;
  }
  for (const d of p.selected) if (d !== p.day) p.days[d] = copyPoints(p.days[p.day]);
}

// --- rendering --------------------------------------------------------

// renderConfig fills the tile's configuration area, or hides it.
function renderConfig(el, room) {
  const area = el.querySelector(".config-area");
  const c = configUI(room);
  if (!c || !c.open) {
    area.classList.add("hidden");
    return;
  }
  area.classList.remove("hidden");
  clear(area);
  area.appendChild(renderTabs(room, c));
  const body = elem("div", "config-body");
  area.appendChild(body);
  if (c.loading) {
    body.appendChild(textEl("p", t("cfg.loading"), "muted"));
    return;
  }
  if (c.error) {
    body.appendChild(textEl("p", t("cfg.cannotLoad", { error: c.error }), "tile-result bad"));
    body.appendChild(button(t("action.close"), "", () => closeConfig(room)));
    return;
  }
  switch (c.tab) {
    case "room":
      body.appendChild(renderRoomTab(room, c));
      break;
    case "temps":
      body.appendChild(renderTempsTab(room, c));
      break;
    case "devices":
      body.appendChild(renderDevicesTab(room, c));
      break;
    case "program":
      body.appendChild(renderProgramTab(room, c));
      break;
    case "copy":
      body.appendChild(renderCopyTab(room, c));
      break;
  }
  if (c.result) {
    const p = elem("p", "tile-result");
    showResult(p, c.result.text, c.result.ok);
    body.appendChild(p);
  }
  markConfigSending(body, c);
}

// markConfigSending draws every Send of the area off while one of its
// writes runs (c.sending), the Send of the form that sent turning.
function markConfigSending(body, c) {
  if (!c.sending) return;
  for (const b of body.querySelectorAll("form button[type=submit]")) {
    if (b.closest("form").dataset.send === c.sending) setBusy(b, true);
    else b.disabled = true;
  }
}

// configWrite runs fn, a write of the configuration area, as the form
// key (the form's data-send): every Send of the area is off meanwhile,
// key's turning (markConfigSending), so that a double click sends one
// radio command, and a write asked for while one runs sends nothing.
// The tile is drawn again at the end, with the room as it is then.
async function configWrite(room, c, key, fn) {
  if (c.sending) return;
  c.sending = key;
  try {
    await fn();
  } finally {
    c.sending = null;
    const r = state.rooms.find((x) => x.id === room.id);
    if (r && configUI(r) === c) rerenderRoom(r);
  }
}

// configForm is a form of the area that sends as key through
// configWrite; whenChanged, given after its Send is in it, keeps the
// Send off while there is nothing to send (sendWhenChanged).
function configForm(room, c, key, send) {
  const form = formEl(() => configWrite(room, c, key, send), "cfg-form");
  form.dataset.send = key;
  form.whenChanged = (changed, lazy) => sendWhenChanged(form, () => !c.sending && changed(), { lazy });
  return form;
}

// renderTabs builds the configuration area's sub-navigation from its
// template (tpl-config-tabs): the section tabs in a tab list, each with
// its section in data-tab, and the close button outside it.
function renderTabs(room, c) {
  const bar = fromTemplate("tpl-config-tabs");
  for (const b of bar.querySelectorAll("[role=tab]")) {
    const key = b.dataset.tab;
    b.classList.toggle("active", c.tab === key);
    b.setAttribute("aria-selected", c.tab === key ? "true" : "false");
    b.addEventListener("click", () => {
      c.tab = key;
      c.result = null;
      rerenderRoom(room);
    });
  }
  bar.querySelector(".close").addEventListener("click", () => closeConfig(room));
  return bar;
}

// --- room tab ---------------------------------------------------------

function renderRoomTab(room, c) {
  const form = configForm(room, c, "room", () => sendRoomName(room, c));
  const name = document.createElement("input");
  name.type = "text";
  name.value = c.edit.name;
  name.maxLength = NAME_BYTES;
  name.required = true;
  name.addEventListener("input", () => {
    c.edit.name = name.value;
  });
  form.appendChild(fieldRow(t("cfg.roomName"), name, { help: "roomName", wide: true }));
  const hint = textEl("p", t("cfg.nameHint", { n: NAME_BYTES }), "muted small");
  form.appendChild(hint);
  form.appendChild(formFooter({ cancel: () => closeConfig(room), submit: t("action.send") }));
  form.whenChanged(() => c.edit.name.trim() !== room.name);
  return form;
}

function byteLength(s) {
  return new TextEncoder().encode(s).length;
}

async function sendRoomName(room, c) {
  const name = c.edit.name.trim();
  if (!name) return setConfigResult(room, c, t("cfg.nameRequired"), false);
  if (byteLength(name) > NAME_BYTES) return setConfigResult(room, c, t("cfg.nameTooLong", { n: NAME_BYTES }), false);
  if (name === room.name) return setConfigResult(room, c, t("cfg.nameUnchanged"), true);
  setConfigResult(room, c, t("cfg.renaming"), true);
  try {
    await api("PUT", base() + "/rooms/" + room.id, { name });
    setConfigResult(room, c, t("cfg.renamed", { name }), true);
    await loadRooms();
  } catch (e) {
    setConfigResult(room, c, t("cfg.renameFailed", { error: e.message }), false);
  }
}

// --- temperatures tab -------------------------------------------------

function renderTempsTab(room, c) {
  const form = configForm(room, c, "temps", () => sendRoomConfig(room, c, TEMP_FIELDS, t("cfg.tab.temps")));
  const e = c.edit.common;
  const set = (k) => (v) => {
    e[k] = v;
  };
  form.appendChild(fieldRow(t("cfg.comfort"), numberInput(e.comfort_temp, MIN_TEMP, MAX_TEMP, STEP, set("comfort_temp")), { unit: "°C", help: "comfort" }));
  form.appendChild(fieldRow(t("cfg.eco"), numberInput(e.eco_temp, MIN_TEMP, MAX_TEMP, STEP, set("eco_temp")), { unit: "°C", help: "eco" }));
  form.appendChild(fieldRow(t("cfg.windowTemp"), numberInput(e.window_open_temp, MIN_TEMP, MAX_TEMP, STEP, set("window_open_temp")), { unit: "°C", help: "windowTemp" }));
  form.appendChild(fieldRow(t("cfg.windowMinutes"), numberInput(e.window_open_minutes, 0, 60, 5, set("window_open_minutes")), { unit: t("unit.minutes"), help: "windowMinutes" }));
  form.appendChild(fieldRow(t("cfg.maxTemp"), numberInput(e.max_setpoint, MIN_TEMP, MAX_TEMP, STEP, set("max_setpoint")), { unit: "°C", help: "maxTemp" }));
  form.appendChild(fieldRow(t("cfg.minTemp"), numberInput(e.min_setpoint, MIN_TEMP, MAX_TEMP, STEP, set("min_setpoint")), { unit: "°C", help: "minTemp" }));
  form.appendChild(sourceNote(c));
  form.appendChild(formFooter({ cancel: () => closeConfig(room), submit: t("action.send") }));
  form.whenChanged(() => Object.keys(changedFields(c, TEMP_FIELDS)).length > 0, true);
  return form;
}

function sourceNote(c) {
  const src = c.cfg && c.cfg.source;
  const dev = src && c.cfg.devices.find((d) => d.rf_address === src);
  const text = src ? t("cfg.valuesFrom", { device: dev && dev.name ? dev.name : src }) : t("cfg.noThermostat");
  return textEl("p", text, "muted small");
}

// changedFields returns the body of a room config write: the fields of
// the list whose value differs from the loaded configuration.
function changedFields(c, fields) {
  const body = {};
  for (const f of fields) {
    const v = c.edit.common[f];
    if (v === undefined || v === null || v === "") continue;
    const was = c.cfg ? c.cfg[f] : undefined;
    if (String(v) !== String(was)) body[f] = f === "decalc_day" ? v : Number(v);
  }
  return body;
}

async function sendRoomConfig(room, c, fields, label) {
  const body = changedFields(c, fields);
  if (Object.keys(body).length === 0) return setConfigResult(room, c, t("cfg.nothingChanged", { label }), true);
  setConfigResult(room, c, t("result.waiting", { label }), true);
  try {
    const res = await api("PUT", base() + "/rooms/" + room.id + "/config", body);
    setConfigResult(room, c, deviceResults(label, res), !!(res && res.ok));
    for (const k of Object.keys(body)) if (c.cfg) c.cfg[k] = body[k];
    state.details = {};
  } catch (e) {
    setConfigResult(room, c, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
}

function deviceResults(label, res) {
  const lines = [t(res && res.ok ? "result.ok" : "cfg.notAll", { label })];
  for (const d of (res && res.devices) || []) {
    const name = d.name || d.rf_address;
    if (d.ok) lines.push(t("cfg.devOk", { name }));
    else if (d.discarded) lines.push(t("cfg.devDiscarded", { name }));
    else lines.push(t("cfg.devFailed", { name, error: d.error || t("cfg.failed") }));
  }
  if (res && res.budget) lines.push(t("cfg.budgetLine", { duty: res.budget.duty_cycle_pct, slots: res.budget.free_memory_slots }));
  return lines.join("\n");
}

// --- devices tab ------------------------------------------------------

function renderDevicesTab(room, c) {
  const wrap = elem("div");
  const thermostats = room.devices.filter(isThermostat);
  if (thermostats.length) {
    const form = configForm(room, c, "valves", () => sendRoomConfig(room, c, VALVE_FIELDS, t("cfg.deviceSettings")));
    const e = c.edit.common;
    const set = (k) => (v) => {
      e[k] = v;
    };
    form.appendChild(textEl("h4", t("cfg.radiators")));
    form.appendChild(fieldRow(t("cfg.boostMinutes"), numberInput(e.boost_minutes, 0, 60, 5, set("boost_minutes")), { unit: t("unit.minutes"), help: "boostMinutes" }));
    form.appendChild(fieldRow(t("cfg.boostValve"), numberInput(e.boost_valve_pct, 0, 100, 5, set("boost_valve_pct")), { unit: "%", help: "boostValve" }));
    const dayOpts = DAYS.map((d) => [d, dayLong(d)]);
    const hour = numberInput(e.decalc_hour, 0, 23, 1, set("decalc_hour"));
    hour.setAttribute("aria-label", t("cfg.decalcHour"));
    const decalc = elem("span", "inline", selectInput(dayOpts, e.decalc_day, set("decalc_day")), hour, textEl("span", ":00", "muted"));
    form.appendChild(fieldRow(t("cfg.decalc"), decalc, { help: "decalc" }));
    form.appendChild(fieldRow(t("cfg.maxValve"), numberInput(e.max_valve_pct, 0, 100, 1, set("max_valve_pct")), { unit: "%", help: "maxValve" }));
    form.appendChild(fieldRow(t("cfg.valveOffset"), numberInput(e.valve_offset_pct, 0, 100, 1, set("valve_offset_pct")), { unit: "%", help: "valveOffset" }));
    form.appendChild(formFooter({ cancel: () => closeConfig(room), submit: t("action.send") }));
    form.whenChanged(() => Object.keys(changedFields(c, VALVE_FIELDS)).length > 0, true);
    wrap.appendChild(form);
  }
  const h = textEl("h4", t("cfg.devices"));
  wrap.appendChild(h);
  if (room.devices.length === 0) wrap.appendChild(textEl("p", t("room.noDevices"), "muted"));
  for (const d of room.devices) wrap.appendChild(renderDeviceForm(room, c, d));
  return wrap;
}

// renderDeviceForm is one device's row: name, offset and display mode
// for thermostats, and a move to another room, with one Send; and
// Remove from the Cube (capability structure), asked in the row.
function renderDeviceForm(room, c, d) {
  const e = c.edit.devices[d.rf_address];
  const key = "device:" + d.rf_address;
  const form = configForm(room, c, key, () => sendDevice(room, c, d));
  form.className += " cfg-device";
  form.appendChild(elem("div", "device-main",
    textEl("span", d.name || d.rf_address, "device-name"),
    textEl("span", typeName(d.type), "muted expert-only"),
    textEl("span", d.rf_address, "muted mono expert-only")));
  const name = document.createElement("input");
  name.type = "text";
  name.value = e.name;
  name.maxLength = NAME_BYTES;
  name.addEventListener("input", () => {
    e.name = name.value;
  });
  const nameRow = fieldRow(t("cfg.name"), name, { help: "deviceName", wide: true });
  nameRow.classList.add("needs-rename");
  form.appendChild(nameRow);
  if (isThermostat(d)) {
    form.appendChild(fieldRow(t("cfg.tempOffset"), numberInput(e.temp_offset, -3.5, 3.5, 0.5, (v) => {
      e.temp_offset = v;
    }), { unit: "°C", help: "tempOffset" }));
    if (d.type === "wall_thermostat") {
      const sw = switchEl(t("cfg.displayActual"), e.display_actual, (on) => {
        e.display_actual = on;
      });
      form.appendChild(fieldRow(null, sw, { help: "displayActual" }));
    }
  }
  const others = state.rooms.map((r) => [String(r.id), r.id === room.id ? t("cfg.thisRoom", { name: r.name }) : r.name]);
  const roomRow = fieldRow(t("cfg.room"), selectInput(others, e.room_id, (v) => {
    e.room_id = Number(v);
    e.confirmMove = false;
  }), { help: "deviceRoom" });
  roomRow.classList.add("needs-structure");
  form.appendChild(roomRow);
  if (e.confirmMove) {
    const target = state.rooms.find((r) => r.id === e.room_id);
    const text = t("cfg.moveConfirm", { device: d.name || d.rf_address, room: target ? target.name : e.room_id });
    form.appendChild(confirmBox(text, { yes: t("cfg.move"), no: t("action.keep") }, () => {
      e.confirmMove = false;
      return configWrite(room, c, key, () => sendDevice(room, c, d, true));
    }, () => {
      e.confirmMove = false;
      e.room_id = room.id;
      rerenderRoom(room);
    }));
  }
  const label = d.name || d.rf_address;
  const remove = button(t("device.remove"), { cls: "needs-structure remove-device", aria: t("device.removeLabel", { device: label }) }, () => {
    e.confirmRemove = true;
    rerenderRoom(room);
  });
  if (e.confirmRemove) {
    form.appendChild(confirmBox(t("device.removeConfirm", { device: label }), { yes: t("device.remove"), no: t("action.keep") }, () => {
      e.confirmRemove = false;
      return configWrite(room, c, "remove", () => removeDeviceOfRoom(room, c, d));
    }, () => {
      e.confirmRemove = false;
      rerenderRoom(room);
    }));
  }
  form.appendChild(formFooter({ submit: t("action.send"), extra: [remove] }));
  form.whenChanged(() => {
    const { meta, cfg } = deviceChanges(room, c, d);
    return Object.keys(meta).length + Object.keys(cfg).length > 0;
  }, true);
  return form;
}

// removeDeviceOfRoom removes a device of the room from the Cube
// (removeFromCube, tiles.js) and shows the Devices tab again without it.
async function removeDeviceOfRoom(room, c, d) {
  setConfigResult(room, c, t("device.removing", { device: d.name || d.rf_address }), true);
  const res = await removeFromCube(d);
  const again = state.rooms.find((r) => r.id === room.id);
  if (!again || !configUI(again)) {
    showTileResult(room.id, res.text, res.ok);
    return;
  }
  const cc = configUI(again);
  cc.edit = freshEdit(again, cc.cfg);
  cc.tab = "devices";
  cc.result = { text: res.text, ok: res.ok };
  rerenderRoom(again);
}

// deviceChanges is what a device's Send writes: meta, the name and the
// room, through PUT /devices/{rf}; cfg, the offset and the display
// mode, through its config; loaded is the device's configuration as
// loaded, which a write updates.
function deviceChanges(room, c, d) {
  const e = c.edit.devices[d.rf_address];
  const meta = {};
  const name = e.name.trim();
  if (name && name !== (d.name || "")) meta.name = name;
  if (e.room_id !== room.id) meta.room_id = e.room_id;
  const cfg = {};
  const loaded = ((c.cfg && c.cfg.devices) || []).find((x) => x.rf_address === d.rf_address) || {};
  if (isThermostat(d) && e.temp_offset !== undefined && Number(e.temp_offset) !== Number(loaded.temp_offset)) cfg.temp_offset = Number(e.temp_offset);
  if (d.type === "wall_thermostat" && e.display_actual !== undefined && e.display_actual !== loaded.display_actual) cfg.display_actual = e.display_actual;
  return { meta, cfg, loaded };
}

// sendDevice writes what changed on a device (deviceChanges); a move to
// another room is asked first.
async function sendDevice(room, c, d, moveConfirmed) {
  const e = c.edit.devices[d.rf_address];
  const label = d.name || d.rf_address;
  const { meta, cfg, loaded } = deviceChanges(room, c, d);
  if (meta.name !== undefined && byteLength(meta.name) > NAME_BYTES) return setConfigResult(room, c, t("cfg.devNameTooLong", { name: label, n: NAME_BYTES }), false);
  if (meta.room_id !== undefined) {
    if (!moveConfirmed) {
      e.confirmMove = true;
      rerenderRoom(room);
      return;
    }
    if (!hasCap("configure")) meta.copy = false; // the copy of the room's settings needs configure
  }
  if (Object.keys(meta).length === 0 && Object.keys(cfg).length === 0) return setConfigResult(room, c, t("cfg.nothingChanged", { label }), true);
  setConfigResult(room, c, t("result.waiting", { label }), true);
  const lines = [];
  let ok = true;
  try {
    if (Object.keys(cfg).length) {
      const res = await api("PUT", base() + "/devices/" + d.rf_address + "/config", cfg);
      lines.push(resultText(t("cfg.devSettings", { name: label }), res));
      if (res && res.discarded) ok = false;
      loaded.temp_offset = cfg.temp_offset !== undefined ? cfg.temp_offset : loaded.temp_offset;
      if (cfg.display_actual !== undefined) loaded.display_actual = cfg.display_actual;
      state.details = {};
    }
    if (Object.keys(meta).length) {
      const res = await api("PUT", base() + "/devices/" + d.rf_address, meta);
      const done = t(meta.room_id !== undefined ? "cfg.devMoved" : "cfg.devRenamed", { name: label });
      lines.push(res && res.budget ? t("cfg.dutySuffix", { text: done, duty: res.budget.duty_cycle_pct }) : done);
    }
  } catch (e2) {
    lines.push(errorText(label, e2));
    ok = false;
  }
  setConfigResult(room, c, lines.join("\n"), ok);
  if (Object.keys(meta).length) {
    await loadRooms();
    const again = state.rooms.find((r) => r.id === room.id);
    if (again && configUI(again)) {
      const cc = configUI(again);
      cc.edit = freshEdit(again, cc.cfg);
      cc.tab = "devices";
      cc.result = { text: lines.join("\n"), ok };
      rerenderRoom(again);
    }
  }
  loadCubes().catch(() => {});
}

// --- week programme tab -----------------------------------------------

function minutesOf(until) {
  const m = /^(\d{1,2}):(\d{2})$/.exec(until || "");
  return m ? Number(m[1]) * 60 + Number(m[2]) : 0;
}

function clockOf(minutes) {
  const pad = (n) => String(n).padStart(2, "0");
  return pad(Math.floor(minutes / 60)) + ":" + pad(minutes % 60);
}

// renderProgramTab is the week programme editor: the day buttons (a
// click edits that day and selects every day with the same programme),
// the "Only {day}" switch, the day as a bar and as rows of switch
// points, the copy section, and the resets left of Cancel and Send.
// The rows are drawn in the order of their times; a time typed into one
// is marked in place (markProgram), not drawn again.
function renderProgramTab(room, c) {
  const p = c.prog;
  sortPoints(p.days[p.day]);
  const form = configForm(room, c, "program", () => sendProgram(room, c));
  const days = elem("div", "day-bar week prog-days");
  days.setAttribute("role", "group");
  days.setAttribute("aria-label", t("prog.dayToEdit"));
  for (const day of DAYS) {
    const b = button(dayShort(day), { title: t("prog.dayTitle", { day: dayLong(day) }) }, () => {
      selectDay(p, day);
      rerenderRoom(room);
    });
    b.dataset.day = day;
    days.appendChild(b);
  }
  form.appendChild(days);
  const onlyLabel = t("prog.only", { day: dayLong(p.day) });
  const only = switchEl(onlyLabel, p.only, (on) => {
    p.only = on;
    rerenderRoom(room);
  });
  form.appendChild(fieldRow(null, only, { help: "onlyDay" }));
  const together = DAYS.filter((d) => p.selected.has(d));
  const scope = p.only || together.length < 2
    ? t("prog.appliesOnly", { day: dayLong(p.day) })
    : t("prog.appliesTogether", { days: dayList(together) });
  form.appendChild(textEl("p", scope, "muted small"));
  form.appendChild(renderDayBar(p.days[p.day]));
  form.appendChild(renderPoints(room, c, p.day, () => markProgram(form, p)));
  form.appendChild(renderDayCopy(room, p));
  const src = p.source;
  const dev = src && c.cfg && c.cfg.devices.find((d) => d.rf_address === src);
  const note = textEl("p", "", "muted small prog-note");
  note.dataset.stored = src ? t("prog.storedOn", { device: dev && dev.name ? dev.name : src }) + " " : "";
  form.appendChild(note);
  const resetDay = button(t("prog.resetDay"), { cls: "prog-reset-day", title: t("prog.resetDayTitle", { day: dayLong(p.day) }) }, () => {
    p.days[p.day] = copyPoints(p.orig[p.day]);
    selectDay(p, p.day);
    rerenderRoom(room);
  });
  const resetAll = button(t("prog.resetAll"), { cls: "prog-reset-all", title: t("prog.resetAllTitle") }, () => {
    p.days = copyDays(p.orig);
    p.copyTo = new Set();
    selectDay(p, p.day);
    rerenderRoom(room);
  });
  form.appendChild(formFooter({ cancel: () => closeConfig(room), submit: t("action.send"), extra: [resetDay, resetAll] }));
  form.whenChanged(() => dirtyDays(p).length > 0, true);
  markProgram(form, p);
  return form;
}

// markProgram shows in the programme editor which days differ from the
// Cube's programme (on the day buttons, in the note, on the resets) and
// which are edited together, and draws the edited day's bar.
function markProgram(form, p) {
  const dirty = new Set(dirtyDays(p));
  for (const b of form.querySelectorAll(".prog-days button")) {
    const day = b.dataset.day;
    const editing = p.day === day;
    const sel = p.selected.has(day);
    let name = dayLong(day);
    if (sel && !editing) name = t("prog.editedTogether", { day: name });
    if (dirty.has(day)) name = t("prog.changed", { day: name });
    b.className = "day" + (sel ? " selected" : "") + (editing ? " editing" : "") + (dirty.has(day) ? " dirty" : "");
    b.setAttribute("aria-label", name);
    b.setAttribute("aria-pressed", editing ? "true" : "false");
  }
  const sorted = copyPoints(p.days[p.day]);
  sortPoints(sorted);
  form.querySelector(".prog-bar").replaceWith(renderDayBar(sorted));
  const note = form.querySelector(".prog-note");
  note.textContent = note.dataset.stored + (dirty.size ? t("prog.sendNoteDays", { days: dayList([...dirty]) }) : t("prog.sendNote"));
  form.querySelector(".prog-reset-day").disabled = !dirty.has(p.day);
  form.querySelector(".prog-reset-all").disabled = dirty.size === 0;
}

// renderDayCopy is the copy section: every day as a toggle, the days
// already sharing the edited day's programme greyed out, "All days",
// and Copy, which gives the ticked days the edited day's programme.
// With "only" off they then join the selection and are edited together.
function renderDayCopy(room, p) {
  const title = t("prog.copyTo", { day: dayLong(p.day) });
  const row = elem("div", "day-bar week");
  const free = DAYS.filter((d) => !p.selected.has(d));
  for (const day of DAYS) {
    const on = p.copyTo.has(day);
    const has = p.selected.has(day);
    const b = button(dayShort(day), { cls: "day" + (on ? " chosen" : ""), aria: has ? t("prog.hasProgramme", { day: dayLong(day) }) : dayLong(day) }, () => {
      if (p.copyTo.has(day)) p.copyTo.delete(day);
      else p.copyTo.add(day);
      rerenderRoom(room);
    });
    b.disabled = has;
    b.setAttribute("aria-pressed", on ? "true" : "false");
    row.appendChild(b);
  }
  const all = button(t("prog.allDays"), "day all", () => {
    for (const d of free) p.copyTo.add(d);
    rerenderRoom(room);
  });
  all.disabled = free.length === 0 || free.every((d) => p.copyTo.has(d));
  const copy = button(t("prog.copy"), "day copy", () => {
    for (const d of p.copyTo) {
      p.days[d] = copyPoints(p.days[p.day]);
      if (!p.only) p.selected.add(d);
    }
    p.copyTo = new Set();
    rerenderRoom(room);
  });
  copy.disabled = p.copyTo.size === 0;
  const box = elem("div", "day-copy", textEl("h4", title), row, elem("div", "day-bar", all, copy));
  box.setAttribute("role", "group");
  box.setAttribute("aria-label", title);
  return box;
}

// renderDayBar draws the day as segments proportional to their length.
function renderDayBar(points) {
  const bar = elem("div", "prog-bar");
  let start = 0;
  const temps = points.map((p) => p.temp);
  const lo = Math.min(...temps), hi = Math.max(...temps);
  for (const pt of points) {
    const end = minutesOf(pt.until);
    // A label only when the segment is wide enough to hold it.
    const seg = textEl("span", end - start >= 120 ? fmtNum(pt.temp, 1) + "°" : "", "prog-seg");
    seg.style.flexGrow = String(Math.max(1, end - start));
    const warm = hi > lo ? (pt.temp - lo) / (hi - lo) : 0.5;
    seg.style.setProperty("--warm", warm.toFixed(2));
    seg.title = clockOf(start) + " – " + pt.until + ": " + fmtTemp(pt.temp);
    bar.appendChild(seg);
    start = end;
  }
  return bar;
}

// renderPoints lists the switch points of a day as editable rows; every
// change goes through programEdited, and a time typed calls onTyped.
function renderPoints(room, c, day, onTyped) {
  const p = c.prog;
  const pts = p.days[day];
  const edited = () => {
    programEdited(p);
    rerenderRoom(room);
  };
  const list = elem("div", "prog-points");
  pts.forEach((pt, i) => {
    const last = i === pts.length - 1;
    const row = elem("div", "prog-point", textEl("span", t("prog.until"), "muted"));
    if (last) {
      row.appendChild(textEl("span", "24:00", "prog-end"));
    } else {
      const time = document.createElement("input");
      time.type = "time";
      time.step = "300";
      time.value = pt.until;
      time.required = true;
      time.setAttribute("aria-label", t("prog.pointUntil", { n: i + 1 }));
      // Chrome fires change at each part of a time typed: the value is
      // kept and marked in place (onTyped), without a redraw, which would
      // replace the input in the middle of the time, or, once it is left,
      // the button being clicked; the rows take the order of their times
      // at the next draw, and Send sorts them (sendProgram).
      time.addEventListener("change", () => {
        if (!time.value) return;
        pt.until = time.value;
        programEdited(p);
        onTyped();
      });
      row.appendChild(time);
    }
    row.appendChild(textEl("span", t("prog.at"), "muted"));
    const temp = numberInput(pt.temp, MIN_TEMP, MAX_TEMP, STEP, (v) => {
      if (v === undefined) return;
      pt.temp = clampTemp(v);
      edited();
    });
    temp.setAttribute("aria-label", t("prog.pointTemp", { n: i + 1 }));
    row.appendChild(temp);
    row.appendChild(textEl("span", "°C", "muted"));
    if (!last) {
      const rm = button("−", { cls: "step small", aria: t("prog.remove"), title: t("prog.removeTitle") }, () => {
        pts.splice(i, 1);
        edited();
      });
      row.appendChild(rm);
    } else if (pts.length < MAX_POINTS) {
      const add = button("+", { cls: "step small", aria: t("prog.add"), title: t("prog.addTitle") }, () => {
        const prev = pts.length > 1 ? minutesOf(pts[pts.length - 2].until) : 0;
        const mid = Math.round(((prev + 1440) / 2) / 5) * 5;
        pts.splice(pts.length - 1, 0, { temp: pt.temp, until: clockOf(Math.min(1435, Math.max(prev + 5, mid))) });
        edited();
      });
      row.appendChild(add);
    }
    list.appendChild(row);
  });
  return list;
}

// sortPoints puts a day's switch points in the order of their times.
function sortPoints(pts) {
  pts.sort((a, b) => minutesOf(a.until) - minutesOf(b.until));
}

function validateDay(pts) {
  if (pts.length < 1 || pts.length > MAX_POINTS) return t("prog.errPoints", { n: MAX_POINTS });
  let prev = 0;
  for (let i = 0; i < pts.length; i++) {
    const m = minutesOf(pts[i].until);
    if (i < pts.length - 1 && m <= prev) return t("prog.errTimes");
    prev = m;
  }
  if (pts[pts.length - 1].until !== "24:00") return t("prog.errEnd");
  return "";
}

// sendProgram writes the days that differ from the loaded programme;
// once the Cube took them they count as loaded.
async function sendProgram(room, c) {
  const p = c.prog;
  const body = {};
  for (const day of dirtyDays(p)) {
    sortPoints(p.days[day]); // a time typed may not have been left yet
    const err = validateDay(p.days[day]);
    if (err) return setConfigResult(room, c, t("prog.dayError", { day: dayShort(day), error: err }), false);
    body[day] = copyPoints(p.days[day]);
  }
  if (Object.keys(body).length === 0) return setConfigResult(room, c, t("prog.nothing"), true);
  const label = t("prog.label", { days: dayList(Object.keys(body)) });
  setConfigResult(room, c, t("result.waiting", { label }), true);
  try {
    const res = await api("PUT", base() + "/rooms/" + room.id + "/program", body);
    if (!(res && res.discarded)) for (const d of Object.keys(body)) p.orig[d] = copyPoints(body[d]);
    setConfigResult(room, c, resultText(label, res), !(res && res.discarded));
    state.details = {};
  } catch (e) {
    setConfigResult(room, c, errorText(label, e), false);
  }
  loadCubes().catch(() => {});
}

// --- copy tab ---------------------------------------------------------

// renderCopyTab copies this room's stored settings to other rooms: the
// Rooms section, a switch per room with "All rooms" and "None", and the
// Settings section, a switch per group of settings.
function renderCopyTab(room, c) {
  if (!c.copy) c.copy = { rooms: new Set(), temps: true, program: false, radiator: true };
  const k = c.copy;
  const form = configForm(room, c, "copy", () => sendCopy(room, c));
  form.appendChild(textEl("p", t("copy.intro"), "muted small"));
  const others = state.rooms.filter((r) => r.id !== room.id && roomThermostat(r));

  const allBtn = button(t("copy.allRooms"), "", () => {
    k.rooms = new Set(others.map((r) => r.id));
    rerenderRoom(room);
  });
  allBtn.disabled = others.length === 0 || others.every((r) => k.rooms.has(r.id));
  const noneBtn = button(t("copy.none"), "", () => {
    k.rooms = new Set();
    rerenderRoom(room);
  });
  noneBtn.disabled = k.rooms.size === 0;
  const list = elem("div", "copy-list");
  if (others.length === 0) list.appendChild(textEl("p", t("copy.noOther"), "muted"));
  for (const r of others) {
    list.appendChild(switchEl(r.name, k.rooms.has(r.id), (on) => {
      if (on) k.rooms.add(r.id);
      else k.rooms.delete(r.id);
      rerenderRoom(room);
    }));
  }
  const rooms = elem("div", "copy-section", elem("div", "copy-head", textEl("h4", t("copy.rooms")), allBtn, noneBtn), list);
  rooms.setAttribute("role", "group");
  rooms.setAttribute("aria-label", t("copy.roomsAria"));
  form.appendChild(rooms);

  const temps = switchEl(t("copy.temps"), k.temps, (on) => {
    k.temps = on;
  });
  const prog = switchEl(t("copy.program"), k.program && !!c.prog, (on) => {
    k.program = on;
  });
  if (!c.prog) prog.control.disabled = true;
  const radiator = switchEl(t("copy.radiator"), k.radiator, (on) => {
    k.radiator = on;
  });
  const settings = elem("div", "copy-section", textEl("h4", t("copy.settings")), elem("div", "copy-list", temps, prog, radiator));
  settings.setAttribute("role", "group");
  settings.setAttribute("aria-label", t("copy.settingsAria"));
  form.appendChild(settings);
  form.appendChild(formFooter({ cancel: () => closeConfig(room), submit: t("copy.send") }));
  form.whenChanged(() => k.rooms.size > 0 && (k.temps || k.radiator || (k.program && !!c.prog)));
  return form;
}

// sendCopy writes this room's stored settings to every selected room,
// one room after another: temperatures and radiator settings in one
// config write, the week programme as a second write.
async function sendCopy(room, c) {
  const k = c.copy;
  const ids = [...k.rooms];
  if (ids.length === 0) return setConfigResult(room, c, t("copy.noRoom"), false);
  const program = k.program && c.prog;
  if (!k.temps && !k.radiator && !program) return setConfigResult(room, c, t("copy.noSetting"), false);
  if (!c.cfg || !c.cfg.source) return setConfigResult(room, c, t("copy.noSource"), false);
  const fields = (k.temps ? TEMP_FIELDS : []).concat(k.radiator ? VALVE_FIELDS : []);
  const body = {};
  for (const f of fields) if (c.cfg[f] !== undefined && c.cfg[f] !== null) body[f] = c.cfg[f];
  const days = program ? copyDays(c.prog.orig) : null;
  const lines = [];
  let allOk = true;
  for (const id of ids) {
    const target = state.rooms.find((r) => r.id === id);
    const name = target ? target.name : String(id);
    setConfigResult(room, c, lines.concat([t("copy.sending", { room: name })]).join("\n"), allOk);
    const parts = [];
    try {
      if (Object.keys(body).length) {
        const res = await api("PUT", base() + "/rooms/" + id + "/config", body);
        parts.push(t(res && res.ok ? "copy.settingsOk" : "copy.settingsPartial"));
        if (!(res && res.ok)) allOk = false;
      }
      if (days) {
        const pr = await api("PUT", base() + "/rooms/" + id + "/program", days);
        parts.push(t(pr && pr.discarded ? "copy.programDiscarded" : "copy.programOk"));
        if (pr && pr.discarded) allOk = false;
      }
      lines.push(t("copy.roomResult", { room: name, parts: parts.join(", ") }));
    } catch (e) {
      lines.push(t("copy.roomResult", { room: name, parts: parts.concat([e.message]).join(", ") }));
      allOk = false;
    }
  }
  state.details = {};
  setConfigResult(room, c, lines.join("\n"), allOk);
  loadCubes().catch(() => {});
}

// --- results ----------------------------------------------------------

// setConfigResult keeps the result of the last send for the area, which
// renderConfig shows (showResult) under the section.
function setConfigResult(room, c, text, ok) {
  c.result = { text, ok };
  rerenderRoom(room);
}
