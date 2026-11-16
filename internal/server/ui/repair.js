// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the repairs of a Cube's rooms and devices
// (docs/ui.md, "Repairs"; docs/api-v1.md, "Repairs"), for a role with
// repair: a box under a room tile, a device's row or a Cube's panel
// that lists the repairs offered there. Each says what it would send
// and what that costs, from its preview (GET, which sends nothing), and
// asks before it runs (POST with the preview's plan). A box's state is
// kept by its scope, so that a tile or a panel drawn again draws it as
// it was.
"use strict";

// REPAIRS are the repairs a box of each kind offers, in their order:
// key names their texts (repair.<key>, repair.<key>Help), route is the
// end of their path, needs a capability they need besides repair, and
// warn says that Run… warns first and asks twice ("unlink all and
// re-link", D163-B).
const REPAIRS = {
  cube: [{ key: "reset", route: "/repair/reset" }],
  room: [
    { key: "roomReset", route: "/repair/reset" },
    { key: "master", route: "/repair/master" },
    { key: "relink", route: "/repair/relink" },
    { key: "unlinkRelink", route: "/repair/unlink-relink", needs: "structure", warn: true },
  ],
  device: [
    { key: "config", route: "/repair/config" },
    { key: "place", route: "/repair/place" },
    { key: "list", route: "/repair/list" },
  ],
};

// repairBoxes are the open boxes by scope ("<serial>|room|<id>", …):
// the kind, the API path the routes hang off, the Cube's serial, and per
// repair its state: "loading", "ready" (preview), "none" (error, the
// preview's refusal), "name" (a listing that needs the room's name,
// room), "running", then "ready" again with the outcome of the run
// (result, ok); a listing keeps the names given, roomName and devName.
const repairBoxes = new Map();

// repairScope is the scope of a box: the Cube, a room or a device of it.
function repairScope(serial, kind, id) {
  return serial + "|" + kind + (id === undefined ? "" : "|" + id);
}

// toggleRepair opens a box, or closes it where it is open, and draws it
// again in place; skip names the repairs the box leaves out.
function toggleRepair(scope, kind, path, skip) {
  if (repairBoxes.has(scope)) {
    repairBoxes.delete(scope);
  } else {
    repairBoxes.set(scope, { kind, path, serial: scope.split("|")[0], skip: skip || [], items: {} });
    loadRepairs(scope);
  }
  redrawRepair(scope);
}

// repairsOf are the repairs a box offers: those the box does not skip,
// for which the user has what they need besides repair.
function repairsOf(box) {
  return REPAIRS[box.kind].filter((r) => !box.skip.includes(r.key) && (!r.needs || hasCap(r.needs)));
}

// loadRepairs reads every preview of a box, keeping a run's outcome.
function loadRepairs(scope) {
  const box = repairBoxes.get(scope);
  if (!box) return;
  for (const r of repairsOf(box)) {
    const item = box.items[r.key] || (box.items[r.key] = {});
    item.state = "loading";
    loadRepair(scope, r, item);
  }
}

// repairQuery is the query of a repair's routes: a listing's room name,
// and the name of a device the list lacks.
function repairQuery(item) {
  const q = [];
  if (item.roomName) q.push("room_name=" + encodeURIComponent(item.roomName));
  if (item.devName) q.push("name=" + encodeURIComponent(item.devName));
  return q.length ? "?" + q.join("&") : "";
}

// loadRepair reads one repair's preview into item and draws the box: a
// preview with nothing to send says why, one that needs a room's name
// asks for it. A listing of a device the list lacks takes the name it
// offers (defaultDeviceName, config.js) the first time, and reads the
// preview again with it.
async function loadRepair(scope, r, item) {
  const box = repairBoxes.get(scope);
  try {
    const p = await api("GET", box.path + r.route + repairQuery(item));
    item.preview = p;
    item.state = "ready";
    const d = p.details || {};
    if (d.code === "room_name_needed") {
      item.state = "name";
      item.room = d.room;
    } else if (p.nothing) {
      item.state = "none";
      item.error = p.nothing;
    } else if (r.key === "list" && d.listed === false && item.devName === undefined) {
      item.devName = defaultDeviceName(d.room_name, typeName(d.type), roomDeviceNames(box.serial, d.to)); // config.js, rooms.js
      item.state = "loading"; // its plan is the server's default's
      return loadRepair(scope, r, item);
    }
  } catch (e) {
    item.preview = null;
    item.state = "none";
    item.error = e.message;
  }
  redrawRepair(scope);
}

// redrawRepair draws a box again where the page shows it, or its empty
// slot where it was closed.
function redrawRepair(scope) {
  for (const el of document.querySelectorAll(".repair-box")) {
    if (el.dataset.scope === scope) el.replaceWith(repairSlot(scope));
  }
  for (const btn of document.querySelectorAll(".repair-open")) {
    if (btn.dataset.scope === scope) btn.setAttribute("aria-expanded", repairBoxes.has(scope) ? "true" : "false");
  }
}

// repairButton is the button that opens and closes the box of a scope;
// label names what it repairs, for its accessible name.
function repairButton(scope, kind, path, { label, skip } = {}) {
  const b = button(t("repair.open"), { cls: "repair-open needs-repair", aria: label ? t("repair.openLabel", { what: label }) : undefined },
    () => toggleRepair(scope, kind, path, skip));
  b.dataset.scope = scope;
  b.setAttribute("aria-expanded", repairBoxes.has(scope) ? "true" : "false");
  return b;
}

// repairSlot is where a scope's box goes: the box while it is open, an
// empty placeholder otherwise, which redrawRepair fills.
function repairSlot(scope) {
  const el = repairBox(scope) || elem("div", "repair-box hidden");
  el.dataset.scope = scope;
  return el;
}

// repairCost says what a preview costs: its radiograms, their duty cycle
// and memory slots, or that it costs no radio time.
function repairCost(p) {
  if (p.radiograms > 0) {
    return t("repair.costRadio", { radiograms: p.radiograms, duty: fmtNum(p.duty_cost_pct, 1), slots: p.slots, held: p.held_slots,
      wakeups: p.wakeups });
  }
  if (p.metadata) return t("repair.costMetadata");
  return t("repair.costResets", { n: p.resets });
}

// repairBox draws a scope's box from its state, null while it is closed.
function repairBox(scope) {
  const box = repairBoxes.get(scope);
  if (!box) return null;
  const el = elem("div", "panel repair-box needs-repair");
  el.dataset.scope = scope;
  el.setAttribute("role", "group");
  const head = elem("div", "repair-head", textEl("h3", t("repair.title"), "repair-title"));
  head.appendChild(button(t("action.close"), { cls: "repair-close", aria: t("repair.close") }, () => toggleRepair(scope)));
  el.appendChild(head);
  for (const r of repairsOf(box)) el.appendChild(repairItem(scope, r, box.items[r.key] || { state: "loading" }));
  return el;
}

// repairItem draws one repair of a box: its name and what it does, then
// by its state what it would cost and Run…, or why it has nothing to do.
function repairItem(scope, r, item) {
  const row = elem("div", "repair-item");
  row.dataset.repair = r.key;
  row.appendChild(textEl("h4", t("repair." + r.key), "repair-name"));
  row.appendChild(textEl("p", t("repair." + r.key + "Help"), "muted small repair-help"));
  if (item.result) row.appendChild(textEl("p", item.result, "repair-result " + (item.ok ? "ok" : "bad")));
  switch (item.state) {
    case "loading":
      row.appendChild(textEl("p", t("repair.loading"), "muted small"));
      break;
    case "none":
      row.appendChild(textEl("p", item.error, "muted small repair-none"));
      break;
    case "name":
      row.appendChild(repairNameForm(scope, r, item));
      break;
    case "ready":
    case "running":
      if (r.key === "list" && item.preview.details && item.preview.details.listed === false) row.appendChild(repairDeviceNameForm(scope, r, item));
      row.append(...repairReady(scope, r, item));
      break;
  }
  return row;
}

// repairDeviceNameForm asks for the name a device the Cube's list lacks
// is listed under, offering the default (defaultDeviceName), and reads the
// preview again with the name changed; an empty name or one longer than
// NAME_BYTES is refused in place.
function repairDeviceNameForm(scope, r, item) {
  const input = document.createElement("input");
  const note = elem("p", "small repair-name-note hidden");
  note.setAttribute("role", "status");
  const form = formEl(() => {
    const name = input.value.trim();
    if (!name) return showResult(note, t("cfg.nameRequired"), false);
    if (byteLength(name) > NAME_BYTES) return showResult(note, t("cfg.devNameTooLong", { name, n: NAME_BYTES }), false);
    if (name === item.devName) return showResult(note, "", true);
    item.devName = name;
    item.state = "loading";
    redrawRepair(scope);
    loadRepair(scope, r, item);
  }, "repair-device-name-form");
  input.name = "name";
  input.required = true;
  input.value = item.devName || "";
  form.append(fieldRow(t("repair.deviceName"), input, { wide: true }), note, elem("div", "actions", submitButton(t("repair.preview"))));
  return form;
}

// repairNameForm asks for the name of the room a listing makes, then
// reads the preview with it.
function repairNameForm(scope, r, item) {
  const form = formEl(() => {
    const name = input.value.trim();
    if (!name) return;
    item.roomName = name;
    item.state = "loading";
    redrawRepair(scope);
    loadRepair(scope, r, item);
  }, "repair-name-form");
  const input = document.createElement("input");
  input.name = "room_name";
  input.required = true;
  input.maxLength = 32;
  input.value = item.roomName || "";
  form.append(fieldRow(t("repair.roomName", { room: item.room }), input, { wide: true }),
    elem("div", "actions", submitButton(t("repair.preview"))));
  return form;
}

// repairReady draws a preview: its cost, its notes, the lines in expert
// mode, a refusal for now, and Run…, which asks before it sends.
function repairReady(scope, r, item) {
  const p = item.preview;
  const out = [textEl("p", repairCost(p), "small repair-cost")];
  for (const n of p.notes || []) out.push(textEl("p", n, "muted small repair-note"));
  const lines = [];
  for (const st of p.steps || []) {
    if (st.room) lines.push("z:…,G," + String(st.room).padStart(2, "0"));
    lines.push(...st.lines);
  }
  if (lines.length) {
    const d = document.createElement("details");
    d.className = "repair-lines expert-only";
    d.appendChild(textEl("summary", t("repair.lines", { n: lines.length })));
    d.appendChild(textEl("pre", lines.join("\n"), "mono small"));
    out.push(d);
  }
  if (p.refused) {
    out.push(textEl("p", t("repair.refusedNow", { reason: p.refused }), "repair-refused bad"));
    return out;
  }
  const actions = elem("div", "actions repair-actions");
  const run = button(t("repair.run"), "repair-run", () => (r.warn ? warnRepair : askRepair)(scope, r, item, actions));
  actions.appendChild(run);
  if (item.state === "running") setBusy(run, true);
  out.push(actions);
  return out;
}

// askRepair asks in place of Run… whether to send the repair, naming
// its cost; Yes runs it.
function askRepair(scope, r, item, actions) {
  const text = t("repair.confirm", { what: t("repair." + r.key), cost: repairCost(item.preview) });
  confirmInPlace(actions, text, { yes: t("repair.runNow"), no: t("action.cancel") }, () => runRepair(scope, r, item), ".repair-run");
}

// warnRepair warns in place of Run… before "unlink all and re-link":
// what it costs, that it has not yet run on hardware, and what a run
// that stops halfway leaves; Continue asks once more, by its own words.
function warnRepair(scope, r, item, actions) {
  const p = item.preview;
  const box = elem("div", "panel repair-warning");
  box.setAttribute("role", "group");
  const text = textEl("p", t("repair.unlinkRelinkWarning", { cost: repairCost(p) }), "repair-warning-text");
  text.id = "repair-warning-" + ++fieldSeq;
  box.setAttribute("aria-labelledby", text.id);
  const back = () => {
    box.replaceWith(actions);
    actions.querySelector(".repair-run").focus();
  };
  const cancel = button(t("action.cancel"), "repair-warning-no", back);
  const go = button(t("repair.continue"), "repair-warning-yes", () => {
    const again = confirmBox(t("repair.unlinkRelinkConfirm"), { yes: t("repair.unlinkRelinkNow"), no: t("action.cancel") },
      () => runRepair(scope, r, item), () => {
        again.replaceWith(actions);
        actions.querySelector(".repair-run").focus();
      });
    box.replaceWith(again);
    again.focusFirst();
  });
  box.append(text, elem("div", "actions", cancel, go));
  box.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    ev.preventDefault();
    ev.stopPropagation();
    back();
  });
  actions.replaceWith(box);
  cancel.focus();
}

// runRepair sends a repair with its preview's plan, says how it went,
// and reads every preview of the box again.
async function runRepair(scope, r, item) {
  const box = repairBoxes.get(scope);
  if (!box) return;
  const q = repairQuery(item);
  item.state = "running";
  redrawRepair(scope);
  try {
    const res = await api("POST", box.path + r.route + (q ? q + "&" : "?") + "plan=" + encodeURIComponent(item.preview.plan));
    item.ok = true;
    item.result = repairOutcome(res);
  } catch (e) {
    item.ok = false;
    item.result = e.message;
  }
  loadRepairs(scope);
}

// repairOutcome says how a run went: the lines the Cube accepted, that
// the devices take it at their next wakeup, the flags still set.
function repairOutcome(res) {
  let lines = 0;
  let accepted = 0;
  for (const st of res.steps || []) {
    lines += st.lines;
    accepted += st.accepted;
  }
  const parts = [t("repair.done", { accepted, lines })];
  if (res.confirmation === "pending") parts.push(t("result.structurePending"));
  if (res.still_flagged) {
    parts.push(res.still_flagged.length ? t("repair.stillFlagged", { devices: res.still_flagged.join(", ") }) : t("repair.cleared"));
  }
  return parts.join(" ");
}
