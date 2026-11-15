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
// key names their texts (repair.<key>, repair.<key>Help) and route is
// the end of their path.
const REPAIRS = {
  cube: [{ key: "reset", route: "/repair/reset" }],
  room: [
    { key: "roomReset", route: "/repair/reset" },
    { key: "master", route: "/repair/master" },
    { key: "relink", route: "/repair/relink" },
  ],
  device: [
    { key: "config", route: "/repair/config" },
    { key: "place", route: "/repair/place" },
    { key: "list", route: "/repair/list" },
  ],
};

// repairBoxes are the open boxes by scope ("<serial>|room|<id>", …):
// the kind, the API path the routes hang off, and per repair its state:
// "loading", "ready" (preview), "none" (error, the preview's refusal),
// "name" (a listing that needs the room's name, room), "running", then
// "ready" again with the outcome of the run (result, ok).
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
    repairBoxes.set(scope, { kind, path, skip: skip || [], items: {} });
    loadRepairs(scope);
  }
  redrawRepair(scope);
}

// repairsOf are the repairs a box offers.
function repairsOf(box) {
  return REPAIRS[box.kind].filter((r) => !box.skip.includes(r.key));
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

// repairQuery is the query of a repair's routes: a listing's room name.
function repairQuery(item) {
  return item.roomName ? "?room_name=" + encodeURIComponent(item.roomName) : "";
}

// loadRepair reads one repair's preview into item and draws the box: a
// preview with nothing to send says why, one that needs a room's name
// asks for it.
async function loadRepair(scope, r, item) {
  const box = repairBoxes.get(scope);
  try {
    const p = await api("GET", box.path + r.route + repairQuery(item));
    item.preview = p;
    item.state = "ready";
    if (p.details && p.details.code === "room_name_needed") {
      item.state = "name";
      item.room = p.details.room;
    } else if (p.nothing) {
      item.state = "none";
      item.error = p.nothing;
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
      row.append(...repairReady(scope, r, item));
      break;
  }
  return row;
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
  const run = button(t("repair.run"), "repair-run", () => askRepair(scope, r, item, actions));
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
