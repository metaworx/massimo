// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Backup and restore, a box per Cube (backupBox): on
// Cube settings the chosen Cube's, in System settings one fold per Cube.
// A box holds the Cube's backup as it is now (GET …/backup), the backups
// the server keeps (GET …/backups) with download, the comparison with the
// one before (…/diff), restore and delete, a backup taken by hand
// (POST …/backups), and the restore of a stored backup or of a file: its
// plan first (POST …/restore/plan), then its steps sent (POST …/restore)
// and followed on the event stream (restore_step, restore_done, app.js).
// What a Cube has is kept by its serial (backupOf), so that its boxes on
// both pages show the same; they are drawn when their page opens, after
// every change and after a restore, and a box is built once per Cube and
// place, so that a poll closes nothing in it. A Cube without a row in the
// server's store has no list.
"use strict";

const backupUI = {
  of: {}, // per serial: what the Cube has (backupOf)
};

// backupOf is what a Cube has: its list, whether the server keeps its
// backups (history: it has a row in the store) and whether the list was
// loaded, when its configuration changed against the newest backup and
// when the automatic backup follows, the comparisons asked for by backup
// id, the restore shown (plan: {title, backup, body, type, plan}) and the
// one sent (run: {id, results, state, sent, error}).
function backupOf(serial) {
  if (!backupUI.of[serial]) {
    backupUI.of[serial] = { list: [], history: true, loaded: false, changed: null, due: null, diff: {}, plan: null, run: null };
  }
  return backupUI.of[serial];
}

// cubeRoute is the base of a Cube's routes.
function cubeRoute(serial) {
  return "api/v1/cubes/" + encodeURIComponent(serial);
}

// boxesOf are the boxes of a Cube in the page, on either page.
function boxesOf(serial) {
  return [...document.querySelectorAll(".backup-box")].filter((b) => b.dataset.serial === serial);
}

function backupMsg(box, text, ok) {
  showResult(box.querySelector(".backup-msg"), text, ok);
}

// backupFormat is the format chosen in a box for a download.
function backupFormat(box) {
  return box.querySelector(".backup-format").value === "json" ? "json" : "yaml";
}

// fmtWhen is an API time as date and time.
function fmtWhen(iso) {
  const opts = { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false };
  return new Intl.DateTimeFormat(i18n.lang, opts).format(new Date(iso));
}

async function loadBackups(serial) {
  if (!serial || !hasCap("backup")) return;
  const b = backupOf(serial);
  try {
    const r = (await api("GET", cubeRoute(serial) + "/backups")) || {};
    b.history = true;
    b.list = r.backups || [];
    b.changed = r.changed || null;
    b.due = r.due || null;
  } catch (e) {
    if (e.status !== 503) throw e;
    b.history = false;
    b.list = [];
    b.changed = b.due = null;
  }
  b.loaded = true;
}

// refreshBackups loads a Cube's list again and draws its boxes; a
// failure is said in each of them.
function refreshBackups(serial) {
  return loadBackups(serial)
    .then(() => drawBackups(serial))
    .catch((e) => {
      for (const box of boxesOf(serial)) backupMsg(box, t("backup.failed", { error: e.message }), false);
    });
}

// drawBackups draws a Cube's boxes from what it has, and the badges.
function drawBackups(serial) {
  for (const box of boxesOf(serial)) renderBackupBox(box);
  renderBackupBadges();
}

// renderBackups draws every box again, after a change of language.
function renderBackups() {
  for (const box of document.querySelectorAll(".backup-box")) renderBackupBox(box);
  renderBackupBadges();
}

// keptBadges counts a Cube's kept backups, none before its list came or
// where the server keeps none.
function keptBadges(serial) {
  const b = backupOf(serial);
  return b.loaded && b.history ? [badgeEl(tn("backup.badge.kept", b.list.length), "mode")] : [];
}

// renderBackupBadges: Cube settings' card counts the chosen Cube's
// backups, System settings' card every Cube's, each fold its Cube's.
function renderBackupBadges() {
  sectionBadges($("cube-backup"), state.serial ? keptBadges(state.serial) : []);
  const kept = state.cubes.map((c) => backupOf(c.serial)).filter((b) => b.loaded && b.history);
  const n = kept.reduce((sum, b) => sum + b.list.length, 0);
  sectionBadges($("sys-backup"), kept.length ? [badgeEl(tn("backup.badge.kept", n), "mode")] : []);
  for (const d of document.querySelectorAll("#backups-cubes > .backup-fold")) {
    d.querySelector(":scope > summary > .panel-badges").replaceChildren(...keptBadges(d.dataset.serial));
  }
}

// backupBox builds a Cube's box for a place ("cube" or "sys"), which its
// ids carry, as both pages may hold the same Cube's.
function backupBox(serial, place) {
  const box = fromTemplate("tpl-backup-box");
  box.dataset.serial = serial;
  const format = box.querySelector(".backup-format");
  format.id = "backup-format-" + place + "-" + serial;
  box.querySelector(".backup-format-label").htmlFor = format.id;
  box.querySelector(".backup-download").addEventListener("click", (ev) => whileBusy(ev.currentTarget, () => downloadNow(box)));
  const take = box.querySelector(".backup-take");
  take.addEventListener("click", () => takeBackup(box, take));
  const from = box.querySelector(".restore-file");
  const picker = filePicker(".json,.yaml,.yml,application/json,application/yaml", (f) => planFile(box, f));
  from.after(picker);
  from.addEventListener("click", () => picker.click());
  renderBackupBox(box);
  return box;
}

// renderBackupBox draws a box from what its Cube has: Take a backup now
// where the server keeps backups, the pending automatic one, the list,
// and the restore shown.
function renderBackupBox(box) {
  const b = backupOf(box.dataset.serial);
  const list = box.querySelector(".backups-list");
  clear(list);
  box.querySelector(".backup-take").classList.toggle("hidden", !b.history);
  const pending = box.querySelector(".backup-pending");
  const due = !!(b.changed && b.due);
  pending.textContent = due ? t("backup.pending", { changed: fmtWhen(b.changed), due: fmtWhen(b.due) }) : "";
  pending.classList.toggle("hidden", !due);
  if (b.loaded) {
    if (!b.history) list.appendChild(textEl("p", t("backup.noHistory"), "muted small"));
    else if (!b.list.length) list.appendChild(textEl("p", t("backup.none"), "muted small"));
    else b.list.forEach((x, i) => list.appendChild(backupRow(box, x, b.list[i + 1])));
  }
  renderPlan(box);
}

// backupRow is a stored backup: when, why, the size and who, the note,
// and its actions; older is the one before it, to compare with.
function backupRow(box, b, older) {
  const row = fromTemplate("tpl-backup-row");
  row.querySelector(".backup-when").textContent = fmtWhen(b.taken_at);
  row.querySelector(".backup-reason").textContent = t("backup.reason." + b.reason);
  const who = [t("backup.size", { size: fmtNum(b.size / 1024, 1) })];
  if (b.user) who.push(t("backup.by", { user: b.user }));
  row.querySelector(".backup-who").textContent = who.join(" · ");
  if (b.note) {
    const note = row.querySelector(".backup-note");
    note.textContent = b.note;
    note.classList.remove("hidden");
  }
  const actions = row.querySelector(".backup-actions");
  // Each request turns its button until the answer (whileBusy).
  const busy = (label, cls, fn) => {
    const btn = button(label, cls, () => whileBusy(btn, fn));
    return btn;
  };
  actions.append(busy(t("backup.downloadOne"), "small", () => downloadStored(box, b)), busy(t("backup.restore"), "small needs-restore", () => planStored(box, b)));
  if (older) actions.append(busy(t("backup.compare"), "small", () => compareBackup(box, b, older)));
  actions.append(button(t("action.delete"), "small needs-restore", () => askDelete(box, b, row)));
  const diff = backupOf(box.dataset.serial).diff[b.id];
  if (diff) row.querySelector(".backup-detail").appendChild(diffEl(diff));
  return row;
}

// diffEl shows what changed from the backup before: the planner's steps.
function diffEl(d) {
  const box = elem("div", "backup-diff");
  if (!d.steps.length) {
    box.appendChild(textEl("p", t("backup.same"), "muted small"));
    return box;
  }
  box.appendChild(textEl("p", t("backup.changes"), "small"));
  const list = elem("ul", "small");
  for (const s of d.steps) list.appendChild(textEl("li", s.what));
  box.appendChild(list);
  return box;
}

// saveResponse saves an answer as a file, named as the server says.
async function saveResponse(resp, fallback) {
  const m = /filename=([^;]+)/.exec(resp.headers.get("Content-Disposition") || "");
  downloadBlob(m ? m[1].trim() : fallback, await resp.blob());
}

async function downloadNow(box) {
  const serial = box.dataset.serial;
  const f = backupFormat(box);
  try {
    await saveResponse(await apiRaw("GET", cubeRoute(serial) + "/backup?format=" + f), "massimo-backup-" + serial + "." + f);
  } catch (e) {
    backupMsg(box, t("backup.failed", { error: e.message }), false);
  }
}

async function downloadStored(box, b) {
  const serial = box.dataset.serial;
  const f = backupFormat(box);
  try {
    await saveResponse(await apiRaw("GET", cubeRoute(serial) + "/backups/" + b.id + "?format=" + f), "massimo-backup-" + serial + "-" + b.id + "." + f);
  } catch (e) {
    backupMsg(box, t("backup.failed", { error: e.message }), false);
  }
}

async function takeBackup(box, btn) {
  await whileBusy(btn, async () => {
    backupMsg(box, "");
    try {
      await api("POST", cubeRoute(box.dataset.serial) + "/backups", {});
      backupMsg(box, t("backup.taken"), true);
      await refreshBackups(box.dataset.serial);
    } catch (e) {
      backupMsg(box, t("backup.failed", { error: e.message }), false);
    }
  });
}

async function compareBackup(box, b, older) {
  const serial = box.dataset.serial;
  try {
    backupOf(serial).diff[b.id] = await api("GET", cubeRoute(serial) + "/backups/" + b.id + "/diff?against=" + older.id);
    drawBackups(serial);
  } catch (e) {
    backupMsg(box, t("backup.failed", { error: e.message }), false);
  }
}

// askDelete asks in the row before the backup is deleted.
function askDelete(box, b, row) {
  const serial = box.dataset.serial;
  const detail = row.querySelector(".backup-detail");
  clear(detail);
  const ask = confirmBox(
    t("backup.deleteAsk", { when: fmtWhen(b.taken_at) }),
    { yes: t("action.delete") },
    async () => {
      try {
        await api("DELETE", cubeRoute(serial) + "/backups/" + b.id);
        backupMsg(box, t("backup.deleted"), true);
        delete backupOf(serial).diff[b.id];
        await refreshBackups(serial);
      } catch (e) {
        backupMsg(box, t("backup.failed", { error: e.message }), false);
      }
    },
    () => clear(detail),
  );
  detail.appendChild(ask);
  ask.focusFirst();
}

// --- restore ----------------------------------------------------------

function planStored(box, b) {
  return showPlan(box, { title: t("restore.planStored", { when: fmtWhen(b.taken_at) }), backup: b.id });
}

async function planFile(box, file) {
  const text = await file.text();
  const yaml = /\.ya?ml$/i.test(file.name) || text.trimStart()[0] !== "{";
  return showPlan(box, { title: t("restore.planFile", { file: file.name }), body: text, type: yaml ? "application/yaml" : "application/json" });
}

// showPlan asks the server what restoring the source would send and
// shows it in the Cube's boxes; nothing is sent yet.
async function showPlan(box, src) {
  const serial = box.dataset.serial;
  backupMsg(box, "");
  const q = src.backup ? "?backup=" + src.backup : "";
  try {
    const resp = await apiRaw("POST", cubeRoute(serial) + "/restore/plan" + q, src.body, src.type);
    const b = backupOf(serial);
    b.plan = Object.assign({}, src, { plan: await resp.json() });
    b.run = null;
    drawPlans(serial);
    box.querySelector(".restore-box").scrollIntoView({ block: "nearest" });
  } catch (e) {
    backupMsg(box, t("restore.planFailed", { error: e.message }), false);
  }
}

function closePlan(serial) {
  const b = backupOf(serial);
  b.plan = null;
  b.run = null;
  drawPlans(serial);
}

// drawPlans draws a Cube's restore in each of its boxes.
function drawPlans(serial) {
  for (const box of boxesOf(serial)) renderPlan(box);
}

// renderPlan draws a box's restore: its steps with how each went once
// sent, the notes, the cost, and Send or how it ended.
function renderPlan(box) {
  const serial = box.dataset.serial;
  const at = box.querySelector(".restore-box");
  clear(at);
  const p = backupOf(serial).plan;
  if (!p) return;
  const run = backupOf(serial).run;
  const el = fromTemplate("tpl-restore-plan");
  el.querySelector(".restore-title").textContent = p.title;
  const steps = el.querySelector(".restore-steps");
  p.plan.steps.forEach((s, i) => {
    const li = elem("li", "", textEl("span", s.what));
    if (s.radio) li.append(" ", badgeEl(t("restore.radio"), "mode"));
    const r = run && run.results[i];
    if (r) li.append(" ", badgeEl(r.ok ? t("restore.sent") : t("restore.stepFailed"), r.ok ? "ok" : "bad"));
    steps.appendChild(li);
  });
  const notes = el.querySelector(".restore-notes");
  for (const n of p.plan.notes || []) notes.appendChild(textEl("li", n));
  const cost = el.querySelector(".restore-cost");
  if (p.plan.radio_commands) {
    cost.textContent = t("restore.cost", { n: p.plan.radio_commands, pct: fmtNum(p.plan.duty_cost_pct, 1), now: p.plan.budget ? p.plan.budget.duty_cycle_pct : "?" });
  } else cost.classList.add("hidden");
  const result = el.querySelector(".restore-result");
  const actions = el.querySelector(".restore-actions");
  const n = p.plan.steps.length;
  if (!run) {
    if (!n) showResult(result, t("restore.nothing"), true);
    actions.appendChild(button(n ? t("action.cancel") : t("action.close"), "", () => closePlan(serial)));
    if (n) {
      const send = button(tn("restore.send", n), "primary", () => whileBusy(send, () => sendRestore(box)));
      actions.appendChild(send);
    }
  } else if (run.state === "running") {
    showResult(result, t("restore.running", { done: run.results.filter((r) => r).length, n }), true);
  } else {
    showResult(result, run.state === "done" ? t("restore.done", { sent: run.sent || 0, n }) : t("restore.failedAt", { sent: run.sent || 0, n, error: run.error || "" }), run.state === "done");
    actions.appendChild(button(t("action.close"), "", () => closePlan(serial)));
  }
  at.appendChild(el);
}

async function sendRestore(box) {
  const serial = box.dataset.serial;
  const b = backupOf(serial);
  const p = b.plan;
  if (!p || b.run) return;
  let q = "?plan=" + encodeURIComponent(p.plan.plan);
  if (p.backup) q += "&backup=" + p.backup;
  b.run = { id: "", results: [], state: "running" };
  drawPlans(serial);
  try {
    const r = await (await apiRaw("POST", cubeRoute(serial) + "/restore" + q, p.body, p.type)).json();
    if (b.run && !b.run.id) b.run.id = r.id;
  } catch (e) {
    b.run = null;
    drawPlans(serial);
    backupMsg(box, t("restore.refused", { error: e.message }), false);
  }
}

// restoreEvent takes the stream's restore_step and restore_done of a
// restore a box sent: the Cube whose run has its id, or the one whose
// run has none yet, its Cube named in the event where it is.
function restoreEvent(ev) {
  const r = ev && ev.restore;
  if (!r) return;
  const runs = Object.entries(backupUI.of).filter(([, b]) => b.run);
  const found = runs.find(([, b]) => b.run.id === r.id) || runs.find(([serial, b]) => !b.run.id && (!ev.cube || ev.cube === serial));
  if (!found) return;
  const [serial, b] = found;
  const run = b.run;
  run.id = r.id;
  if (ev.kind === "restore_step") run.results[r.step - 1] = { ok: r.ok, error: r.error };
  else {
    run.state = r.state || (r.ok ? "done" : "failed");
    run.sent = r.sent;
    run.error = r.error;
    refreshBackups(serial);
  }
  drawPlans(serial);
}

// renderCubeBackup draws Cube settings' card: the chosen Cube's box,
// kept where it is already that Cube's, and its list loaded again.
function renderCubeBackup() {
  const body = $("cube-backup-body");
  if (!body || !hasCap("backup")) return;
  const serial = state.serial;
  const have = body.querySelector(".backup-box");
  if (!serial) {
    clear(body);
    renderBackupBadges();
    return;
  }
  if (!have || have.dataset.serial !== serial) body.replaceChildren(backupBox(serial, "cube"));
  refreshBackups(serial);
}

// backupFold is a Cube's fold in System settings: its network sign, its
// name and serial, the count kept, and its box; a single Cube's open.
function backupFold(c) {
  const d = detailsPanel({ name: c.name || undefined, mono: c.serial, open: state.cubes.length === 1, cls: "backup-fold" }, backupBox(c.serial, "sys"));
  d.dataset.serial = c.serial;
  const sum = d.querySelector("summary");
  sum.prepend(cubeSignEl(c)); // app.js
  sum.appendChild(elem("span", "panel-badges"));
  return d;
}

// renderSystemBackups draws System settings' card: a fold per Cube,
// those there already kept as they are (open, what is typed), new ones
// added, gone ones removed; every list loaded again.
function renderSystemBackups() {
  const holder = $("backups-cubes");
  if (!holder || !hasCap("backup")) return;
  const have = new Map([...holder.querySelectorAll(":scope > .backup-fold")].map((d) => [d.dataset.serial, d]));
  const folds = state.cubes.map((c) => have.get(c.serial) || backupFold(c));
  if (folds.length) holder.replaceChildren(...folds);
  else holder.replaceChildren(textEl("p", t("cubes.none"), "muted small"));
  for (const c of state.cubes) refreshBackups(c.serial);
}

// backupCubeChanged follows the Cube choice on Cube settings.
function backupCubeChanged() {
  if (isShown("cube")) renderCubeBackup();
}

function initBackups() {
  const quiet = () => {
    for (const box of document.querySelectorAll(".backup-box")) backupMsg(box, "");
  };
  onView("system", () => {
    quiet();
    renderSystemBackups();
  });
  onView("cube", () => {
    quiet();
    renderCubeBackup();
  });
}

document.addEventListener("DOMContentLoaded", initBackups);
