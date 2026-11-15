// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the House tile: the mode for the whole house or
// the rooms ticked, with its panel, and its wiring.
"use strict";

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
// ticked (sendWhenChanged), set by initHouse.
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
        temp = (await presetTemps(th))[mode];
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

// initHouse wires the House tile: its mode buttons, its panel, and the
// click outside it that closes it once it has sent.
function initHouse() {
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
  document.addEventListener("click", (ev) => {
    // Clicking outside closes the House panel once it has sent (and an
    // open menu, dropdown in components.js).
    const panel = housePanel();
    if (!panel.classList.contains("hidden") && !ev.target.closest("#house") && !$("house-result").classList.contains("hidden")) {
      closeHouse();
    }
  });
}
