// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, a tile's device rows: each device's badges and
// values, its row, the list of a room's devices, and the virtual
// thermostat's.
"use strict";

// --- devices ----------------------------------------------------------

// deviceBadges lists the badges in words of a device: not reported,
// stale and locked, outlined, then its mode or window state as a filled
// badge, always the last one; its problems are icons (deviceProblems).
// Each is [text, kind, the time of the value for its age].
function deviceBadges(d) {
  const s = d.state || {};
  const out = [];
  if (!d.reported) {
    // A device the Cube has not heard for some minutes keeps its last
    // values, out of date; one it never heard has none.
    out.push([t(d.out_of_date ? "badge.outOfDate" : "badge.notReported"), "warn", d.out_of_date ? d.last_seen : undefined]);
    return out;
  }
  if (d.stale) out.push([staleText(d), "warn stale-badge"]);
  if (s.panel_locked) out.push([t("badge.locked"), "warn"]);
  if (s.window_open !== undefined) {
    const at = received(d, "window_open");
    out.push(s.window_open ? [t("badge.open"), WINDOW_TONE[windowState(d)] || "fill-bad", at] : [t("badge.closed"), "fill-ok", at]);
  } else if (s.mode && isThermostat(d)) {
    out.push([modeName(s.mode), modeTone(s.mode)]); // a boost says so, coloured as manual
  }
  return out;
}

// mismatchBadge is a finding of the device (d.mismatches, docs/api-v1.md)
// as an outlined badge naming the other side, the room or the type the
// Cube has; its whole sentence (tiles.js mismatchText) is its title, and
// the room's tile says it in full.
function mismatchBadge(m) {
  const b = badgeEl(t("mismatchBadge." + m.finding, mismatchArgs(m)), "warn mismatch-badge"); // tiles.js
  b.title = mismatchText(m);
  return b;
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
    cell.appendChild(textEl("span", fmtUntil(s.until), ""));
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
  for (const m of d.mismatches || []) badges.appendChild(mismatchBadge(m));
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
  if (isThermostat(d) && (d.reported || (d.out_of_date && d.last_known))) {
    // An out-of-date device shows the values of its last report.
    const shown = d.reported ? d : Object.assign({}, d, { state: d.last_known });
    if (!d.reported) grid.classList.add("out-of-date");
    deviceValues(grid, shown, undefined);
    deviceDetail(d).then((detail) => {
      const cfg = detail.thermostat || detail.wall_thermostat;
      if (cfg && cfg.temp_offset !== undefined) deviceValues(grid, shown, cfg.temp_offset);
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
