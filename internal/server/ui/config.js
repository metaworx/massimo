// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, configuration: the per-room configuration area
// (name, temperatures, device settings, week programme, copy to other
// rooms), inline in the tile, one screen per section with Send and
// Cancel. Uses the state, api and helpers of app.js and tiles.js.
"use strict";

const DAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
const DAY_SHORT = { monday: "Mo", tuesday: "Tu", wednesday: "We", thursday: "Th", friday: "Fr", saturday: "Sa", sunday: "Su" };
const DAY_LONG = { monday: "Monday", tuesday: "Tuesday", wednesday: "Wednesday", thursday: "Thursday", friday: "Friday", saturday: "Saturday", sunday: "Sunday" };
const MAX_POINTS = 13;
const NAME_BYTES = 32;
const TABS = [
  ["room", "Room"],
  ["temps", "Temperatures"],
  ["devices", "Devices"],
  ["program", "Week programme"],
  ["copy", "Copy"],
];
// The common fields of a room's configuration, as GET /rooms/{id}/config
// returns them and PUT /rooms/{id}/config takes them.
const TEMP_FIELDS = ["comfort_temp", "eco_temp", "window_open_temp", "window_open_minutes", "max_setpoint", "min_setpoint"];
const VALVE_FIELDS = ["boost_minutes", "boost_valve_pct", "decalc_day", "decalc_hour", "max_valve_pct", "valve_offset_pct"];

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
  ui.config = { open: true, tab: tab || "room", loading: true, cfg: null, prog: null, edit: {}, result: null };
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
  const body = document.createElement("div");
  body.className = "config-body";
  area.appendChild(body);
  if (c.loading) {
    body.appendChild(textEl("p", "Loading the configuration …", "muted"));
    return;
  }
  if (c.error) {
    body.appendChild(textEl("p", "Cannot load the configuration: " + c.error, "tile-result bad"));
    body.appendChild(button("Close", "", () => closeConfig(room)));
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
    const p = textEl("p", c.result.text, "tile-result " + (c.result.ok ? "ok" : "bad"));
    body.appendChild(p);
  }
}

// renderTabs builds the configuration area's sub-navigation: the
// section tabs in a tab list, and the close button outside it.
function renderTabs(room, c) {
  const bar = document.createElement("div");
  bar.className = "config-tabs";
  const list = document.createElement("span");
  list.className = "config-tablist";
  list.setAttribute("role", "tablist");
  list.setAttribute("aria-label", "configuration sections");
  for (const [key, label] of TABS) {
    const b = button(label, "tab" + (c.tab === key ? " active" : ""), () => {
      c.tab = key;
      c.result = null;
      rerenderRoom(room);
    });
    b.setAttribute("role", "tab");
    b.setAttribute("aria-selected", c.tab === key ? "true" : "false");
    list.appendChild(b);
  }
  bar.appendChild(list);
  const close = button("×", "tab close", () => closeConfig(room));
  close.title = "Close the configuration";
  close.setAttribute("aria-label", "close configuration");
  bar.appendChild(close);
  return bar;
}

// field builds a labelled input and binds it to the edit model.
function field(label, input, unit) {
  const wrap = document.createElement("label");
  wrap.className = "cfg-field";
  wrap.appendChild(textEl("span", label, "cfg-label"));
  const box = document.createElement("span");
  box.className = "inline";
  box.appendChild(input);
  if (unit) box.appendChild(textEl("span", unit, "muted"));
  wrap.appendChild(box);
  return wrap;
}

function numberInput(value, min, max, step, onChange) {
  const i = document.createElement("input");
  i.type = "number";
  i.min = String(min);
  i.max = String(max);
  i.step = String(step);
  if (value !== undefined && value !== null) i.value = String(value);
  i.addEventListener("change", () => onChange(i.value === "" ? undefined : Number(i.value)));
  return i;
}

function selectInput(options, value, onChange) {
  const s = document.createElement("select");
  for (const [v, label] of options) {
    const o = document.createElement("option");
    o.value = v;
    o.textContent = label;
    s.appendChild(o);
  }
  if (value !== undefined && value !== null) s.value = String(value);
  s.addEventListener("change", () => onChange(s.value));
  return s;
}

// formActions builds the Cancel and Send row; extra buttons (resets)
// go to its left end.
function formActions(onCancel, sendLabel, extra) {
  const actions = document.createElement("div");
  actions.className = "actions";
  if (extra && extra.length) {
    const left = document.createElement("span");
    left.className = "actions-left";
    for (const b of extra) left.appendChild(b);
    actions.appendChild(left);
  }
  const right = document.createElement("span");
  right.className = "actions-right";
  right.appendChild(button("Cancel", "", onCancel));
  const send = document.createElement("button");
  send.type = "submit";
  send.className = "primary";
  send.textContent = sendLabel || "Send";
  right.appendChild(send);
  actions.appendChild(right);
  return actions;
}

function configForm(onSubmit) {
  const form = document.createElement("form");
  form.className = "cfg-form";
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    onSubmit();
  });
  return form;
}

// --- room tab ---------------------------------------------------------

function renderRoomTab(room, c) {
  const form = configForm(() => sendRoomName(room, c));
  const name = document.createElement("input");
  name.type = "text";
  name.value = c.edit.name;
  name.maxLength = NAME_BYTES;
  name.required = true;
  name.addEventListener("input", () => {
    c.edit.name = name.value;
  });
  form.appendChild(field("Room name", name));
  const hint = textEl("p", "Up to " + NAME_BYTES + " bytes; umlauts take two.", "muted small");
  form.appendChild(hint);
  form.appendChild(formActions(() => closeConfig(room)));
  return form;
}

function byteLength(s) {
  return new TextEncoder().encode(s).length;
}

async function sendRoomName(room, c) {
  const name = c.edit.name.trim();
  if (!name) return setConfigResult(room, c, "A name is required.", false);
  if (byteLength(name) > NAME_BYTES) return setConfigResult(room, c, "The name is longer than " + NAME_BYTES + " bytes.", false);
  if (name === room.name) return setConfigResult(room, c, "The name is unchanged.", true);
  setConfigResult(room, c, "Renaming …", true);
  try {
    await api("PUT", base() + "/rooms/" + room.id, { name });
    setConfigResult(room, c, "Room renamed to " + name + ".", true);
    await loadRooms();
  } catch (e) {
    setConfigResult(room, c, "Rename: " + e.message, false);
  }
}

// --- temperatures tab -------------------------------------------------

function renderTempsTab(room, c) {
  const form = configForm(() => sendRoomConfig(room, c, TEMP_FIELDS, "Temperatures"));
  const e = c.edit.common;
  const set = (k) => (v) => {
    e[k] = v;
  };
  form.appendChild(field("Comfort temperature", numberInput(e.comfort_temp, MIN_TEMP, MAX_TEMP, STEP, set("comfort_temp")), "°C"));
  form.appendChild(field("Eco temperature", numberInput(e.eco_temp, MIN_TEMP, MAX_TEMP, STEP, set("eco_temp")), "°C"));
  form.appendChild(field("Open window", numberInput(e.window_open_temp, MIN_TEMP, MAX_TEMP, STEP, set("window_open_temp")), "°C"));
  form.appendChild(field("Open window for", numberInput(e.window_open_minutes, 0, 60, 5, set("window_open_minutes")), "min"));
  form.appendChild(field("Maximum temperature", numberInput(e.max_setpoint, MIN_TEMP, MAX_TEMP, STEP, set("max_setpoint")), "°C"));
  form.appendChild(field("Minimum temperature", numberInput(e.min_setpoint, MIN_TEMP, MAX_TEMP, STEP, set("min_setpoint")), "°C"));
  form.appendChild(sourceNote(c));
  form.appendChild(formActions(() => closeConfig(room)));
  return form;
}

function sourceNote(c) {
  const src = c.cfg && c.cfg.source;
  const dev = src && c.cfg.devices.find((d) => d.rf_address === src);
  const text = src ? "Values as stored on " + (dev && dev.name ? dev.name : src) + "; a change goes to every thermostat of the room." : "No thermostat in this room.";
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
  if (Object.keys(body).length === 0) return setConfigResult(room, c, label + ": nothing changed.", true);
  setConfigResult(room, c, label + ": waiting for the cube …", true);
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
  const lines = [label + (res && res.ok ? ": ok" : ": not every device took it")];
  for (const d of (res && res.devices) || []) {
    let t = (d.name || d.rf_address) + ": " + (d.ok ? "ok" : d.discarded ? "discarded" : d.error || "failed");
    lines.push(t);
  }
  if (res && res.budget) lines.push("duty cycle " + res.budget.duty_cycle_pct + " %, " + res.budget.free_memory_slots + " free slots");
  return lines.join("\n");
}

// --- devices tab ------------------------------------------------------

function renderDevicesTab(room, c) {
  const wrap = document.createElement("div");
  const thermostats = room.devices.filter(isThermostat);
  if (thermostats.length) {
    const form = configForm(() => sendRoomConfig(room, c, VALVE_FIELDS, "Device settings"));
    const e = c.edit.common;
    const set = (k) => (v) => {
      e[k] = v;
    };
    form.appendChild(textEl("h4", "Radiator thermostats of the room"));
    form.appendChild(field("Boost duration", numberInput(e.boost_minutes, 0, 60, 5, set("boost_minutes")), "min"));
    form.appendChild(field("Boost valve opening", numberInput(e.boost_valve_pct, 0, 100, 5, set("boost_valve_pct")), "%"));
    const dayOpts = DAYS.map((d) => [d, d.charAt(0).toUpperCase() + d.slice(1)]);
    const decalc = document.createElement("span");
    decalc.className = "inline";
    decalc.appendChild(selectInput(dayOpts, e.decalc_day, set("decalc_day")));
    decalc.appendChild(numberInput(e.decalc_hour, 0, 23, 1, set("decalc_hour")));
    decalc.appendChild(textEl("span", ":00", "muted"));
    form.appendChild(field("Descaling run", decalc));
    form.appendChild(field("Maximum valve opening", numberInput(e.max_valve_pct, 0, 100, 1, set("max_valve_pct")), "%"));
    form.appendChild(field("Valve offset", numberInput(e.valve_offset_pct, 0, 100, 1, set("valve_offset_pct")), "%"));
    form.appendChild(formActions(() => closeConfig(room)));
    wrap.appendChild(form);
  }
  const h = textEl("h4", "Devices");
  wrap.appendChild(h);
  if (room.devices.length === 0) wrap.appendChild(textEl("p", "No devices in this room.", "muted"));
  for (const d of room.devices) wrap.appendChild(renderDeviceForm(room, c, d));
  return wrap;
}

// renderDeviceForm is one device's row: name, offset and display mode
// for thermostats, and a move to another room, with one Send.
function renderDeviceForm(room, c, d) {
  const e = c.edit.devices[d.rf_address];
  const form = configForm(() => sendDevice(room, c, d));
  form.className += " cfg-device";
  const head = document.createElement("div");
  head.className = "device-main";
  head.appendChild(textEl("span", d.name || d.rf_address, "device-name"));
  head.appendChild(textEl("span", d.type.replace(/_/g, " "), "muted expert-only"));
  head.appendChild(textEl("span", d.rf_address, "muted mono expert-only"));
  form.appendChild(head);
  const name = document.createElement("input");
  name.type = "text";
  name.value = e.name;
  name.maxLength = NAME_BYTES;
  name.addEventListener("input", () => {
    e.name = name.value;
  });
  form.appendChild(field("Name", name));
  if (isThermostat(d)) {
    form.appendChild(field("Temperature offset", numberInput(e.temp_offset, -3.5, 3.5, 0.5, (v) => {
      e.temp_offset = v;
    }), "°C"));
    if (d.type === "wall_thermostat") {
      const sw = document.createElement("label");
      sw.className = "switch";
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.checked = !!e.display_actual;
      cb.addEventListener("change", () => {
        e.display_actual = cb.checked;
      });
      sw.appendChild(cb);
      sw.appendChild(textEl("span", "", "slider"));
      sw.appendChild(textEl("span", "Display the measured temperature", "switch-label"));
      form.appendChild(sw);
    }
  }
  const others = state.rooms.map((r) => [String(r.id), r.name + (r.id === room.id ? " (this room)" : "")]);
  form.appendChild(field("Room", selectInput(others, e.room_id, (v) => {
    e.room_id = Number(v);
    e.confirmMove = false;
  })));
  if (e.confirmMove) {
    const box = document.createElement("div");
    box.className = "panel delete-confirm";
    const target = state.rooms.find((r) => r.id === e.room_id);
    box.appendChild(textEl("span", "Move " + (d.name || d.rf_address) + " to " + (target ? target.name : e.room_id) + "? This re-links the device with both rooms, several radiograms."));
    const actions = document.createElement("div");
    actions.className = "actions";
    actions.appendChild(button("Keep", "", () => {
      e.confirmMove = false;
      e.room_id = room.id;
      rerenderRoom(room);
    }));
    actions.appendChild(button("Move", "primary", () => {
      e.confirmMove = false;
      sendDevice(room, c, d, true);
    }));
    box.appendChild(actions);
    form.appendChild(box);
  }
  const actions = document.createElement("div");
  actions.className = "actions";
  const send = document.createElement("button");
  send.type = "submit";
  send.className = "primary";
  send.textContent = "Send";
  actions.appendChild(send);
  form.appendChild(actions);
  return form;
}

// sendDevice writes what changed on a device: name and room through
// PUT /devices/{rf}, offset and display mode through its config.
async function sendDevice(room, c, d, moveConfirmed) {
  const e = c.edit.devices[d.rf_address];
  const label = d.name || d.rf_address;
  const meta = {};
  const name = e.name.trim();
  if (name && name !== (d.name || "")) {
    if (byteLength(name) > NAME_BYTES) return setConfigResult(room, c, label + ": the name is longer than " + NAME_BYTES + " bytes.", false);
    meta.name = name;
  }
  if (e.room_id !== room.id) {
    if (!moveConfirmed) {
      e.confirmMove = true;
      rerenderRoom(room);
      return;
    }
    meta.room_id = e.room_id;
  }
  const cfg = {};
  const loaded = ((c.cfg && c.cfg.devices) || []).find((x) => x.rf_address === d.rf_address) || {};
  if (isThermostat(d) && e.temp_offset !== undefined && Number(e.temp_offset) !== Number(loaded.temp_offset)) cfg.temp_offset = Number(e.temp_offset);
  if (d.type === "wall_thermostat" && e.display_actual !== undefined && e.display_actual !== loaded.display_actual) cfg.display_actual = e.display_actual;
  if (Object.keys(meta).length === 0 && Object.keys(cfg).length === 0) return setConfigResult(room, c, label + ": nothing changed.", true);
  setConfigResult(room, c, label + ": waiting for the cube …", true);
  const lines = [];
  let ok = true;
  try {
    if (Object.keys(cfg).length) {
      const res = await api("PUT", base() + "/devices/" + d.rf_address + "/config", cfg);
      lines.push(resultText(label + " settings", res));
      if (res && res.discarded) ok = false;
      loaded.temp_offset = cfg.temp_offset !== undefined ? cfg.temp_offset : loaded.temp_offset;
      if (cfg.display_actual !== undefined) loaded.display_actual = cfg.display_actual;
      state.details = {};
    }
    if (Object.keys(meta).length) {
      const res = await api("PUT", base() + "/devices/" + d.rf_address, meta);
      lines.push(label + (meta.room_id !== undefined ? " moved" : " renamed") + ": ok" + (res && res.budget ? " (duty cycle " + res.budget.duty_cycle_pct + " %)" : ""));
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
function renderProgramTab(room, c) {
  const p = c.prog;
  const form = configForm(() => sendProgram(room, c));
  const dirty = new Set(dirtyDays(p));
  const days = document.createElement("div");
  days.className = "day-bar week";
  days.setAttribute("role", "group");
  days.setAttribute("aria-label", "day to edit");
  for (const day of DAYS) {
    const editing = p.day === day;
    const sel = p.selected.has(day);
    const b = button(DAY_SHORT[day], "day" + (sel ? " selected" : "") + (editing ? " editing" : "") + (dirty.has(day) ? " dirty" : ""), () => {
      selectDay(p, day);
      rerenderRoom(room);
    });
    b.setAttribute("aria-pressed", editing ? "true" : "false");
    b.setAttribute("aria-label", DAY_LONG[day] + (sel && !editing ? ", edited together" : "") + (dirty.has(day) ? ", changed" : ""));
    b.title = "Edit " + DAY_LONG[day] + " and the days with the same programme";
    days.appendChild(b);
  }
  form.appendChild(days);
  form.appendChild(switchEl("Only " + DAY_LONG[p.day], p.only, (on) => {
    p.only = on;
    rerenderRoom(room);
  }));
  const together = DAYS.filter((d) => p.selected.has(d));
  const scope = p.only || together.length < 2
    ? "Changes apply to " + DAY_LONG[p.day] + " only."
    : "Changes apply to " + together.map((d) => DAY_SHORT[d]).join(", ") + " together.";
  form.appendChild(textEl("p", scope, "muted small"));
  form.appendChild(renderDayBar(p.days[p.day]));
  form.appendChild(renderPoints(room, c, p.day));
  form.appendChild(renderDayCopy(room, p));
  const src = p.source;
  const dev = src && c.cfg && c.cfg.devices.find((d) => d.rf_address === src);
  form.appendChild(textEl("p", (src ? "Programme as stored on " + (dev && dev.name ? dev.name : src) + ". " : "") + "Send writes the changed days to every thermostat of the room" + (dirty.size ? " (" + [...dirty].map((d) => DAY_SHORT[d]).join(", ") + ")" : "") + ".", "muted small"));
  const resetDay = button("Reset day", "", () => {
    p.days[p.day] = copyPoints(p.orig[p.day]);
    selectDay(p, p.day);
    rerenderRoom(room);
  });
  resetDay.disabled = !dirty.has(p.day);
  resetDay.title = "Back to the programme of " + DAY_LONG[p.day] + " as loaded";
  const resetAll = button("Reset all", "", () => {
    p.days = copyDays(p.orig);
    p.copyTo = new Set();
    selectDay(p, p.day);
    rerenderRoom(room);
  });
  resetAll.disabled = dirty.size === 0;
  resetAll.title = "Back to the week programme as loaded";
  form.appendChild(formActions(() => closeConfig(room), "", [resetDay, resetAll]));
  return form;
}

// renderDayCopy is the copy section: every day as a toggle, the days
// already sharing the edited day's programme greyed out, "All days",
// and Copy, which gives the ticked days the edited day's programme.
// With "only" off they then join the selection and are edited together.
function renderDayCopy(room, p) {
  const box = document.createElement("div");
  box.className = "day-copy";
  box.setAttribute("role", "group");
  const title = "Copy " + DAY_LONG[p.day] + " to …";
  box.setAttribute("aria-label", title);
  box.appendChild(textEl("h4", title));
  const row = document.createElement("div");
  row.className = "day-bar week";
  const free = DAYS.filter((d) => !p.selected.has(d));
  for (const day of DAYS) {
    const on = p.copyTo.has(day);
    const b = button(DAY_SHORT[day], "day" + (on ? " chosen" : ""), () => {
      if (p.copyTo.has(day)) p.copyTo.delete(day);
      else p.copyTo.add(day);
      rerenderRoom(room);
    });
    b.disabled = p.selected.has(day);
    b.setAttribute("aria-pressed", on ? "true" : "false");
    b.setAttribute("aria-label", DAY_LONG[day] + (b.disabled ? ", has this programme" : ""));
    row.appendChild(b);
  }
  const all = button("All days", "day all", () => {
    for (const d of free) p.copyTo.add(d);
    rerenderRoom(room);
  });
  all.disabled = free.length === 0 || free.every((d) => p.copyTo.has(d));
  box.appendChild(row);
  const tools = document.createElement("div");
  tools.className = "day-bar";
  tools.appendChild(all);
  const copy = button("Copy", "day copy", () => {
    for (const d of p.copyTo) {
      p.days[d] = copyPoints(p.days[p.day]);
      if (!p.only) p.selected.add(d);
    }
    p.copyTo = new Set();
    rerenderRoom(room);
  });
  copy.disabled = p.copyTo.size === 0;
  tools.appendChild(copy);
  box.appendChild(tools);
  return box;
}

// switchEl builds a slide switch: a checkbox styled as a switch.
function switchEl(label, checked, onChange) {
  const sw = document.createElement("label");
  sw.className = "switch";
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.checked = !!checked;
  cb.addEventListener("change", () => onChange(cb.checked));
  sw.appendChild(cb);
  const slider = textEl("span", "", "slider");
  slider.setAttribute("aria-hidden", "true");
  sw.appendChild(slider);
  sw.appendChild(textEl("span", label, "switch-label"));
  return sw;
}

// renderDayBar draws the day as segments proportional to their length.
function renderDayBar(points) {
  const bar = document.createElement("div");
  bar.className = "prog-bar";
  let start = 0;
  const temps = points.map((p) => p.temp);
  const lo = Math.min(...temps), hi = Math.max(...temps);
  for (const pt of points) {
    const end = minutesOf(pt.until);
    // A label only when the segment is wide enough to hold it.
    const seg = textEl("span", end - start >= 120 ? pt.temp.toFixed(1) + "°" : "", "prog-seg");
    seg.style.flexGrow = String(Math.max(1, end - start));
    const warm = hi > lo ? (pt.temp - lo) / (hi - lo) : 0.5;
    seg.style.setProperty("--warm", warm.toFixed(2));
    seg.title = clockOf(start) + " – " + pt.until + ": " + pt.temp.toFixed(1) + " °C";
    bar.appendChild(seg);
    start = end;
  }
  return bar;
}

// renderPoints lists the switch points of a day as editable rows; every
// change goes through programEdited.
function renderPoints(room, c, day) {
  const p = c.prog;
  const pts = p.days[day];
  const edited = () => {
    programEdited(p);
    rerenderRoom(room);
  };
  const list = document.createElement("div");
  list.className = "prog-points";
  pts.forEach((pt, i) => {
    const last = i === pts.length - 1;
    const row = document.createElement("div");
    row.className = "prog-point";
    row.appendChild(textEl("span", "until", "muted"));
    if (last) {
      row.appendChild(textEl("span", "24:00", "prog-end"));
    } else {
      const time = document.createElement("input");
      time.type = "time";
      time.step = "300";
      time.value = pt.until;
      time.required = true;
      time.setAttribute("aria-label", "switch point " + (i + 1) + ", until");
      time.addEventListener("change", () => {
        if (!time.value) return;
        pt.until = time.value;
        pts.sort((a, b) => minutesOf(a.until) - minutesOf(b.until));
        edited();
      });
      row.appendChild(time);
    }
    row.appendChild(textEl("span", "at", "muted"));
    const temp = numberInput(pt.temp, MIN_TEMP, MAX_TEMP, STEP, (v) => {
      if (v === undefined) return;
      pt.temp = clampTemp(v);
      edited();
    });
    temp.setAttribute("aria-label", "switch point " + (i + 1) + ", temperature");
    row.appendChild(temp);
    row.appendChild(textEl("span", "°C", "muted"));
    if (!last) {
      const rm = button("−", "step small", () => {
        pts.splice(i, 1);
        edited();
      });
      rm.title = "Remove this switch point";
      rm.setAttribute("aria-label", "remove switch point");
      row.appendChild(rm);
    } else if (pts.length < MAX_POINTS) {
      const add = button("+", "step small", () => {
        const prev = pts.length > 1 ? minutesOf(pts[pts.length - 2].until) : 0;
        const mid = Math.round(((prev + 1440) / 2) / 5) * 5;
        pts.splice(pts.length - 1, 0, { temp: pt.temp, until: clockOf(Math.min(1435, Math.max(prev + 5, mid))) });
        edited();
      });
      add.title = "Add a switch point";
      add.setAttribute("aria-label", "add switch point");
      row.appendChild(add);
    }
    list.appendChild(row);
  });
  return list;
}

function validateDay(pts) {
  if (pts.length < 1 || pts.length > MAX_POINTS) return "a day needs 1 to " + MAX_POINTS + " switch points";
  let prev = 0;
  for (let i = 0; i < pts.length; i++) {
    const m = minutesOf(pts[i].until);
    if (i < pts.length - 1 && m <= prev) return "times must increase";
    prev = m;
  }
  if (pts[pts.length - 1].until !== "24:00") return "the last point must end at 24:00";
  return "";
}

// sendProgram writes the days that differ from the loaded programme;
// once the Cube took them they count as loaded.
async function sendProgram(room, c) {
  const p = c.prog;
  const body = {};
  for (const day of dirtyDays(p)) {
    const err = validateDay(p.days[day]);
    if (err) return setConfigResult(room, c, DAY_SHORT[day] + ": " + err + ".", false);
    body[day] = copyPoints(p.days[day]);
  }
  if (Object.keys(body).length === 0) return setConfigResult(room, c, "Week programme: nothing changed.", true);
  const label = "Week programme (" + Object.keys(body).map((d) => DAY_SHORT[d]).join(", ") + ")";
  setConfigResult(room, c, label + ": waiting for the cube …", true);
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
  const form = configForm(() => sendCopy(room, c));
  form.appendChild(textEl("p", "Copy what is stored for this room (not unsent changes) to other rooms.", "muted small"));
  const others = state.rooms.filter((r) => r.id !== room.id && roomThermostat(r));

  const rooms = document.createElement("div");
  rooms.className = "copy-section";
  rooms.setAttribute("role", "group");
  rooms.setAttribute("aria-label", "rooms to copy to");
  const head = document.createElement("div");
  head.className = "copy-head";
  head.appendChild(textEl("h4", "Rooms"));
  const allBtn = button("All rooms", "", () => {
    k.rooms = new Set(others.map((r) => r.id));
    rerenderRoom(room);
  });
  allBtn.disabled = others.length === 0 || others.every((r) => k.rooms.has(r.id));
  head.appendChild(allBtn);
  const noneBtn = button("None", "", () => {
    k.rooms = new Set();
    rerenderRoom(room);
  });
  noneBtn.disabled = k.rooms.size === 0;
  head.appendChild(noneBtn);
  rooms.appendChild(head);
  const list = document.createElement("div");
  list.className = "copy-list";
  if (others.length === 0) list.appendChild(textEl("p", "No other room with a thermostat.", "muted"));
  for (const r of others) {
    list.appendChild(switchEl(r.name, k.rooms.has(r.id), (on) => {
      if (on) k.rooms.add(r.id);
      else k.rooms.delete(r.id);
      rerenderRoom(room);
    }));
  }
  rooms.appendChild(list);
  form.appendChild(rooms);

  const settings = document.createElement("div");
  settings.className = "copy-section";
  settings.setAttribute("role", "group");
  settings.setAttribute("aria-label", "settings to copy");
  settings.appendChild(textEl("h4", "Settings"));
  const slist = document.createElement("div");
  slist.className = "copy-list";
  slist.appendChild(switchEl("Temperatures", k.temps, (on) => {
    k.temps = on;
  }));
  const prog = switchEl("Week programme (7 radiograms per room)", k.program && !!c.prog, (on) => {
    k.program = on;
  });
  if (!c.prog) prog.querySelector("input").disabled = true;
  slist.appendChild(prog);
  slist.appendChild(switchEl("Radiator settings (boost, descaling, valve)", k.radiator, (on) => {
    k.radiator = on;
  }));
  settings.appendChild(slist);
  form.appendChild(settings);
  form.appendChild(formActions(() => closeConfig(room), "Copy"));
  return form;
}

// sendCopy writes this room's stored settings to every selected room,
// one room after another: temperatures and radiator settings in one
// config write, the week programme as a second write.
async function sendCopy(room, c) {
  const k = c.copy;
  const ids = [...k.rooms];
  if (ids.length === 0) return setConfigResult(room, c, "No room selected.", false);
  const program = k.program && c.prog;
  if (!k.temps && !k.radiator && !program) return setConfigResult(room, c, "No setting selected.", false);
  if (!c.cfg || !c.cfg.source) return setConfigResult(room, c, "This room has no thermostat to copy from.", false);
  const fields = (k.temps ? TEMP_FIELDS : []).concat(k.radiator ? VALVE_FIELDS : []);
  const body = {};
  for (const f of fields) if (c.cfg[f] !== undefined && c.cfg[f] !== null) body[f] = c.cfg[f];
  const days = program ? copyDays(c.prog.orig) : null;
  const lines = [];
  let allOk = true;
  for (const id of ids) {
    const target = state.rooms.find((r) => r.id === id);
    const name = target ? target.name : String(id);
    setConfigResult(room, c, lines.concat([name + ": sending …"]).join("\n"), allOk);
    const parts = [];
    try {
      if (Object.keys(body).length) {
        const res = await api("PUT", base() + "/rooms/" + id + "/config", body);
        parts.push("settings " + (res && res.ok ? "ok" : "not on every device"));
        if (!(res && res.ok)) allOk = false;
      }
      if (days) {
        const pr = await api("PUT", base() + "/rooms/" + id + "/program", days);
        parts.push("programme " + (pr && pr.discarded ? "discarded" : "ok"));
        if (pr && pr.discarded) allOk = false;
      }
      lines.push(name + ": " + parts.join(", "));
    } catch (e) {
      lines.push(name + ": " + parts.concat([e.message]).join(", "));
      allOk = false;
    }
  }
  state.details = {};
  setConfigResult(room, c, lines.join("\n"), allOk);
  loadCubes().catch(() => {});
}

// --- results ----------------------------------------------------------

function setConfigResult(room, c, text, ok) {
  c.result = { text, ok };
  rerenderRoom(room);
}
