// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Backup and restore: the Cube's backup
// as it is now (GET …/backup), the backups the server keeps
// (GET …/backups) with download, the comparison with the one before
// (…/diff), restore and delete, a backup taken by hand (POST …/backups),
// and the restore of a stored backup or of a file: its plan first
// (POST …/restore/plan), then its steps sent (POST …/restore) and
// followed on the event stream (restore_step, restore_done, app.js).
// The list is drawn when the settings open, after every change and after
// a restore; a Cube without a row in the server's store has no list.
"use strict";

const backupUI = {
  list: [],
  history: true, // the server keeps the Cube's backups (it has a row in the store)
  changed: null, // when the configuration changed against the newest backup
  due: null, // when the automatic backup follows
  diff: {}, // per backup id: the comparison with the one before, once asked for
  plan: null, // the restore shown: {title, backup, body, type, plan}
  run: null, // the restore sent: {id, steps, results, state, sent, error}
};

function backupMsg(text, ok) {
  showResult($("backup-msg"), text, ok);
}

// backupFormat is the format chosen for a download.
function backupFormat() {
  return $("backup-format").value === "json" ? "json" : "yaml";
}

// fmtWhen is an API time as date and time.
function fmtWhen(iso) {
  const opts = { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false };
  return new Intl.DateTimeFormat(i18n.lang, opts).format(new Date(iso));
}

async function loadBackups() {
  if (!state.serial || !hasCap("backup")) return;
  try {
    const r = (await api("GET", base() + "/backups")) || {};
    backupUI.history = true;
    backupUI.list = r.backups || [];
    backupUI.changed = r.changed || null;
    backupUI.due = r.due || null;
  } catch (e) {
    if (e.status !== 503) throw e;
    backupUI.history = false;
    backupUI.list = [];
    backupUI.changed = backupUI.due = null;
  }
}

// refreshBackups loads the list again and draws it.
function refreshBackups() {
  return loadBackups()
    .then(renderBackups)
    .catch((e) => backupMsg(t("backup.failed", { error: e.message }), false));
}

// renderBackups draws the list from backupUI.
function renderBackups() {
  const box = $("backups-list");
  if (!box) return;
  clear(box);
  $("backup-take").classList.toggle("hidden", !backupUI.history);
  const pending = $("backup-pending");
  pending.textContent = backupUI.changed && backupUI.due ? t("backup.pending", { changed: fmtWhen(backupUI.changed), due: fmtWhen(backupUI.due) }) : "";
  pending.classList.toggle("hidden", !(backupUI.changed && backupUI.due));
  if (!backupUI.history) {
    box.appendChild(textEl("p", t("backup.noHistory"), "muted small"));
    return;
  }
  if (!backupUI.list.length) {
    box.appendChild(textEl("p", t("backup.none"), "muted small"));
    return;
  }
  backupUI.list.forEach((b, i) => box.appendChild(backupRow(b, backupUI.list[i + 1])));
}

// backupRow is a stored backup: when, why, the size and who, the note,
// and its actions; older is the one before it, to compare with.
function backupRow(b, older) {
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
  actions.append(busy(t("backup.downloadOne"), "small", () => downloadStored(b)), busy(t("backup.restore"), "small needs-restore", () => planStored(b)));
  if (older) actions.append(busy(t("backup.compare"), "small", () => compareBackup(b, older)));
  actions.append(button(t("action.delete"), "small needs-restore", () => askDelete(b, row)));
  if (backupUI.diff[b.id]) row.querySelector(".backup-detail").appendChild(diffEl(backupUI.diff[b.id]));
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

async function downloadNow() {
  const f = backupFormat();
  try {
    await saveResponse(await apiRaw("GET", base() + "/backup?format=" + f), "massimo-backup-" + state.serial + "." + f);
  } catch (e) {
    backupMsg(t("backup.failed", { error: e.message }), false);
  }
}

async function downloadStored(b) {
  const f = backupFormat();
  try {
    await saveResponse(await apiRaw("GET", base() + "/backups/" + b.id + "?format=" + f), "massimo-backup-" + state.serial + "-" + b.id + "." + f);
  } catch (e) {
    backupMsg(t("backup.failed", { error: e.message }), false);
  }
}

async function takeBackup() {
  await whileBusy($("backup-take"), async () => {
    backupMsg("");
    try {
      await api("POST", base() + "/backups", {});
      backupMsg(t("backup.taken"), true);
      await loadBackups();
      renderBackups();
    } catch (e) {
      backupMsg(t("backup.failed", { error: e.message }), false);
    }
  });
}

async function compareBackup(b, older) {
  try {
    backupUI.diff[b.id] = await api("GET", base() + "/backups/" + b.id + "/diff?against=" + older.id);
    renderBackups();
  } catch (e) {
    backupMsg(t("backup.failed", { error: e.message }), false);
  }
}

// askDelete asks in the row before the backup is deleted.
function askDelete(b, row) {
  const detail = row.querySelector(".backup-detail");
  clear(detail);
  const box = confirmBox(
    t("backup.deleteAsk", { when: fmtWhen(b.taken_at) }),
    { yes: t("action.delete") },
    async () => {
      try {
        await api("DELETE", base() + "/backups/" + b.id);
        backupMsg(t("backup.deleted"), true);
        delete backupUI.diff[b.id];
        await refreshBackups();
      } catch (e) {
        backupMsg(t("backup.failed", { error: e.message }), false);
      }
    },
    () => clear(detail),
  );
  detail.appendChild(box);
  box.focusFirst();
}

// --- restore ----------------------------------------------------------

function planStored(b) {
  return showPlan({ title: t("restore.planStored", { when: fmtWhen(b.taken_at) }), backup: b.id });
}

async function planFile(file) {
  const text = await file.text();
  const yaml = /\.ya?ml$/i.test(file.name) || text.trimStart()[0] !== "{";
  return showPlan({ title: t("restore.planFile", { file: file.name }), body: text, type: yaml ? "application/yaml" : "application/json" });
}

// showPlan asks the server what restoring the source would send and
// shows it; nothing is sent yet.
async function showPlan(src) {
  backupMsg("");
  const q = src.backup ? "?backup=" + src.backup : "";
  try {
    const resp = await apiRaw("POST", base() + "/restore/plan" + q, src.body, src.type);
    backupUI.plan = Object.assign({}, src, { plan: await resp.json() });
    backupUI.run = null;
    renderPlan();
    $("restore-box").scrollIntoView({ block: "nearest" });
  } catch (e) {
    backupMsg(t("restore.planFailed", { error: e.message }), false);
  }
}

function closePlan() {
  backupUI.plan = null;
  backupUI.run = null;
  renderPlan();
}

// renderPlan draws the restore shown: its steps with how each went once
// sent, the notes, the cost, and Send or how it ended.
function renderPlan() {
  const box = $("restore-box");
  if (!box) return;
  clear(box);
  const p = backupUI.plan;
  if (!p) return;
  const run = backupUI.run;
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
    actions.appendChild(button(n ? t("action.cancel") : t("action.close"), "", closePlan));
    if (n) {
      const send = button(tn("restore.send", n), "primary", () => whileBusy(send, sendRestore));
      actions.appendChild(send);
    }
  } else if (run.state === "running") {
    showResult(result, t("restore.running", { done: run.results.filter((r) => r).length, n }), true);
  } else {
    showResult(result, run.state === "done" ? t("restore.done", { sent: run.sent || 0, n }) : t("restore.failedAt", { sent: run.sent || 0, n, error: run.error || "" }), run.state === "done");
    actions.appendChild(button(t("action.close"), "", closePlan));
  }
  box.appendChild(el);
}

async function sendRestore() {
  const p = backupUI.plan;
  if (!p || backupUI.run) return;
  let q = "?plan=" + encodeURIComponent(p.plan.plan);
  if (p.backup) q += "&backup=" + p.backup;
  backupUI.run = { id: "", results: [], state: "running" };
  renderPlan();
  try {
    const r = await (await apiRaw("POST", base() + "/restore" + q, p.body, p.type)).json();
    if (backupUI.run && !backupUI.run.id) backupUI.run.id = r.id;
  } catch (e) {
    backupUI.run = null;
    renderPlan();
    backupMsg(t("restore.refused", { error: e.message }), false);
  }
}

// restoreEvent takes the stream's restore_step and restore_done of the
// restore the panel sent.
function restoreEvent(ev) {
  const run = backupUI.run;
  const r = ev && ev.restore;
  if (!run || !r || (run.id && r.id !== run.id)) return;
  run.id = r.id;
  if (ev.kind === "restore_step") run.results[r.step - 1] = { ok: r.ok, error: r.error };
  else {
    run.state = r.state || (r.ok ? "done" : "failed");
    run.sent = r.sent;
    run.error = r.error;
    refreshBackups();
  }
  renderPlan();
}

// backupCubeChanged forgets the shown Cube's backups and restore.
function backupCubeChanged() {
  backupUI.list = [];
  backupUI.diff = {};
  backupUI.changed = backupUI.due = null;
  backupUI.plan = backupUI.run = null;
  renderPlan();
  if (isShown("system")) refreshBackups();
}

function initBackups() {
  $("backup-download").addEventListener("click", (ev) => whileBusy(ev.currentTarget, downloadNow));
  $("backup-take").addEventListener("click", takeBackup);
  const picker = filePicker(".json,.yaml,.yml,application/json,application/yaml", (f) => planFile(f));
  $("restore-file").after(picker);
  $("restore-file").addEventListener("click", () => picker.click());
  onView("system", () => {
    backupMsg("");
    if (hasCap("backup")) refreshBackups();
  });
}

document.addEventListener("DOMContentLoaded", initBackups);
