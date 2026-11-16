// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, pairing a device with a Cube: the time, Start,
// the countdown with Stop, each device that joins, and the form that
// puts a device into a room (pairDeviceForm, which Devices without a
// room uses too).
"use strict";

// --- pairing ----------------------------------------------------------

// cubePath is the API path of a Cube.
function cubePath(serial) {
  return "api/v1/cubes/" + encodeURIComponent(serial);
}

// pairingUI is what the pairing keeps per Cube: its flow (pairingOf)
// and, for a Cube that is not the one shown, its rooms (loadCubeRooms).
const pairingUI = { flows: {}, rooms: {} };

// pairingOf is the pairing flow of a Cube's panel, which outlives a
// redraw: form (the time and Start shown), seconds, active and until
// (the server's state), paired (the devices that joined while this page
// watched), msg (the last outcome), edits (per device, what its form
// holds), tick (the countdown's timer).
function pairingOf(serial) {
  return pairingUI.flows[serial] || (pairingUI.flows[serial] = { form: false, seconds: 60, active: false, until: 0, paired: [], msg: null, edits: {}, tick: null });
}

// cubeRooms are the rooms a device of the Cube can be put into: the
// page's for the Cube shown, else those loadCubeRooms read.
function cubeRooms(serial) {
  return serial === state.serial ? state.rooms : pairingUI.rooms[serial] || [];
}

async function loadCubeRooms(serial) {
  if (serial === state.serial) return;
  try {
    pairingUI.rooms[serial] = (await api("GET", cubePath(serial) + "/rooms?devices=real")) || [];
    redrawPairing(serial);
  } catch (e) {
    pairingUI.rooms[serial] = [];
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

// startPairing sends Start: POST …/pairing with the seconds, which the
// form checked (pairingSection).
function startPairing(serial) {
  const p = pairingOf(serial);
  const secs = Number(p.seconds);
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

// redrawPairing draws a panel's pairing again inside its .cube-pairing,
// the element the Cube's panel (ui/src/cubes/Actions.tsx) leaves to it,
// so that the Cube's form keeps what is typed in it.
function redrawPairing(serial) {
  const panel = document.querySelector('.cube-panel[data-serial="' + serial + '"]');
  const box = panel && panel.querySelector(".cube-pairing");
  if (box) box.replaceChildren(...pairingSection(serial).childNodes);
}

// pairingSection is a panel's pairing (tpl-pairing), whose children
// redrawPairing puts into the panel's .cube-pairing: Pair a device; the
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
  // The time's field allows 1 to 255 s, the longest a Cube keeps
  // (cube.MaxInclusion); a time it refuses is marked as it is typed, and
  // Start then goes back to the field instead.
  const secs = form.querySelector("input[name=seconds]");
  const range = form.querySelector(".pairing-range");
  secs.id = "pair-seconds-" + serial;
  range.id = secs.id + "-range";
  secs.closest(".field").querySelector("label").htmlFor = secs.id;
  secs.value = String(p.seconds);
  markRange(secs, range);
  secs.addEventListener("input", () => {
    p.seconds = secs.value;
    markRange(secs, range);
  });
  form.querySelector(".pair-cancel").addEventListener("click", () => {
    p.form = false;
    redrawPairing(serial);
  });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (!markRange(secs, range)) {
      secs.focus();
      return;
    }
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
    elem("div", "pair-device-head", textEl("span", typeName(d.type), "pair-device-type"), deviceIds(d.serial, d.rf_address)));
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
    const label = nm || deviceName(dev);
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
      res = await api("PUT", cubePath(serial) + "/devices/" + encodeURIComponent(dev.serial || dev.rf_address), body);
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
    if (res && res.confirmation === "pending") text += " " + t("result.structurePending");
    e.result = { text, ok: true };
    e.done = true;
    e.busy = false;
    if (serial === state.serial) await loadRooms().catch(() => {});
    else loadCubeRooms(serial);
    if (onDone) onDone();
  });
  return form;
}
