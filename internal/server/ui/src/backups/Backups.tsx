// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/backups/Backups.tsx
 *
 * Backup and restore as parts, a box per Cube and place: on Cube settings
 * the chosen Cube's, in System settings a fold per Cube, each built once
 * and updated in place, so that a fold open, a question asked and the
 * format chosen stay when data comes in. A box holds the format, Download
 * backup, Take a backup now where the server keeps backups, Restore from
 * file…, the automatic backup pending, the backups kept, each with
 * Download, Restore…, Compare with the one before and Delete, which asks
 * in its row, and the restore shown with its steps and Send, then how
 * each step went. A restore is a command, no settings form.
 */

import { useEffect, useRef } from "preact/hooks";
import type { Cube, StoredBackup } from "../api";
import { CubeIcon, CubeSign } from "../model/cube";
import { Badge, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Select } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { FileButton } from "../shared/file";
import { BusyButton, Question, useAsk } from "../shared/question";
import { fmtWhen } from "../shared/time";
import { useLook } from "../shared/look";
import {
  boxOf, closePlan, compareBackup, deleteBackup, downloadNow, downloadStored, foldUI, of, planFile, planStored, running, sendRestore, takeBackup,
} from "./store";

/** Diff is what changed from the backup before: the planner's steps. */
function Diff({ d }: { d: { steps?: { what?: string }[] } }) {
  const steps = d.steps || [];
  if (!steps.length) return <div class="backup-diff"><p class="muted small">{t("backup.same")}</p></div>;
  return (
    <div class="backup-diff">
      <p class="small">{t("backup.changes")}</p>
      <ul class="small">{steps.map((s, i) => <li key={i}>{s.what}</li>)}</ul>
    </div>
  );
}

/** Row is a stored backup: when, why, the size and who, the note, and its actions; older is the one before it, to compare with. */
function Row({ place, serial, b, older }: { place: string; serial: string; b: StoredBackup; older?: StoredBackup }) {
  useLook();
  const ask = useAsk<HTMLButtonElement>();
  const diff = of(serial).diff[b.id];
  const who = [t("backup.size", { size: fmtNum(b.size / 1024, 1) })];
  if (b.user) who.push(t("backup.by", { user: b.user }));
  return (
    <div class="backup-row">
      <div class="backup-head">
        <span class="backup-when">{fmtWhen(b.taken_at)}</span>
        <span class="backup-reason flag mode">{t("backup.reason." + b.reason)}</span>
        <span class="backup-who muted small">{who.join(" · ")}</span>
      </div>
      <p class={"backup-note muted small" + hid(!b.note)}>{b.note}</p>
      <div class="actions backup-actions">
        <BusyButton cls="small" label={t("backup.downloadOne")} run={() => downloadStored(place, serial, b)} />
        <BusyButton cls="small needs-restore" label={t("backup.restore")} off={running(serial)} run={() => planStored(place, serial, b)} />
        {older && <BusyButton cls="small" label={t("backup.compare")} run={() => compareBackup(place, serial, b, older)} />}
        <button ref={ask.ref} type="button" class="small needs-restore" onClick={ask.ask}>{t("action.delete")}</button>
      </div>
      <div class="backup-detail">
        {ask.asking ? (
          <Question text={t("backup.deleteAsk", { when: fmtWhen(b.taken_at) })} yes={t("action.delete")} no={t("action.cancel")} onNo={() => ask.done()}
            onYes={async () => {
              // refused, the question stays, the message beside it, as the plain one did
              if (await deleteBackup(place, serial, b)) ask.done(false);
            }} />
        ) : diff && <Diff d={diff} />}
      </div>
    </div>
  );
}

/** PlanBox is the restore shown: its steps with how each went once sent, the notes, the cost, and Send or how it ended. */
function PlanBox({ place, serial }: { place: string; serial: string }) {
  useLook();
  const c = of(serial);
  const p = c.plan;
  const run = c.run;
  const ref = useRef<HTMLDivElement>(null);
  const asked = c.asked === place;
  // a plan shown is brought into view once, in the box that asked for it
  useEffect(() => {
    if (!asked) return;
    of(serial).asked = "";
    ref.current?.scrollIntoView({ block: "nearest" });
  }, [asked, serial]);
  if (!p) return null;
  const steps = p.plan.steps || [];
  const n = steps.length;
  let result: { text: string; ok: boolean } | null = null;
  if (!run && !n) result = { text: t("restore.nothing"), ok: true };
  else if (run?.state === "running") result = { text: t("restore.running", { done: run.results.filter((r) => r).length, n }), ok: true };
  else if (run?.state === "unseen") result = { text: t("restore.unseen"), ok: false };
  else if (run) {
    result = run.state === "done" ? { text: t("restore.done", { sent: run.sent || 0, n }), ok: true }
      : { text: t("restore.failedAt", { sent: run.sent || 0, n, error: run.error || "" }), ok: false };
  }
  const plan = p.plan;
  return (
    <div ref={ref} class="panel restore-plan">
      <p class="restore-title">{p.title}</p>
      <ol class="restore-steps small">
        {steps.map((s, i) => {
          const r = run?.results[i];
          return (
            <li key={i}>
              <span>{s.what}</span>
              {s.radio && <> <Badge kind="mode" text={t("restore.radio")} /></>}
              {r && <> <Badge kind={r.ok ? "ok" : "bad"} text={r.ok ? t("restore.sent") : t("restore.stepFailed")} /></>}
            </li>
          );
        })}
      </ol>
      <ul class="restore-notes muted small">{(plan.notes || []).map((x, i) => <li key={i}>{x}</li>)}</ul>
      <p class={"restore-cost muted small" + hid(!plan.radio_commands)}>
        {plan.radio_commands ? t("restore.cost", { n: plan.radio_commands, pct: fmtNum(plan.duty_cost_pct, 1), now: plan.budget ? String(plan.budget.duty_cycle_pct ?? "?") : "?" }) : ""}
      </p>
      <ResultLine cls="restore-result small" role="status" res={result} />
      <div class="actions restore-actions">
        {!run && <button type="button" onClick={() => closePlan(serial)}>{n ? t("action.cancel") : t("action.close")}</button>}
        {!run && n > 0 && <BusyButton cls="primary" label={tn("restore.send", n)} run={() => sendRestore(place, serial)} />}
        {run && run.state !== "running" && <button type="button" onClick={() => closePlan(serial)}>{t("action.close")}</button>}
      </div>
    </div>
  );
}

/**
 * Box is a Cube's box for a place ("cube" or "sys"), which its ids carry,
 * as both pages may hold the same Cube's.
 */
export function Box({ serial, place }: { serial: string; place: string }) {
  useLook();
  const c = of(serial);
  const ui = boxOf(place, serial);
  const due = !!(c.changed && c.due);
  const list = c.list;
  const format = "backup-format-" + place + "-" + serial;
  return (
    <div class="backup-box" data-serial={serial}>
      <div class="backup-bar">
        <Select label={t("backup.format")} id={format} options={[["yaml", "YAML"], ["json", "JSON"]]} value={ui.format} onChange={(v) => { ui.format = v; }} />
        <BusyButton cls="backup-download" label={t("backup.download")} run={() => downloadNow(place, serial)} />
        <BusyButton cls={"backup-take" + hid(!c.history)} label={t("backup.take")} run={() => takeBackup(place, serial)} />
        <FileButton cls="restore-file needs-restore" label={t("restore.fromFile")} accept=".json,.yaml,.yml,application/json,application/yaml" off={running(serial)}
          onFile={(file) => planFile(place, serial, file)} />
      </div>
      <p class={"backup-pending muted small" + hid(!due)}>{due ? t("backup.pending", { changed: fmtWhen(c.changed || ""), due: fmtWhen(c.due || "") }) : ""}</p>
      <div class="backups-list">
        {c.loaded && !c.history && <p class="muted small">{t("backup.noHistory")}</p>}
        {c.loaded && c.history && !list.length && <p class="muted small">{t("backup.none")}</p>}
        {c.loaded && c.history && list.map((b, i) => <Row key={b.id} place={place} serial={serial} b={b} older={list[i + 1]} />)}
      </div>
      <div class="restore-box"><PlanBox place={place} serial={serial} /></div>
      <ResultLine cls="backup-msg small cubes-msg" role="status" res={ui.said} />
    </div>
  );
}

/** keptText counts a Cube's kept backups, "" before its list came or where the server keeps none. */
export function keptText(serial: string): string {
  const b = of(serial);
  return b.loaded && b.history ? tn("backup.badge.kept", b.list.length) : "";
}

/** CubeFold is a Cube's fold in System settings: the cube, its name and serial, its network sign after them, the count kept, and its box. */
function CubeFold({ c }: { c: Cube }) {
  useLook();
  const ui = foldUI.of(c.serial);
  // a single Cube's fold open at first, kept so once another Cube comes
  // biome-ignore lint/correctness/useExhaustiveDependencies: the fold's first state is decided once, at its first drawing.
  useEffect(() => {
    if (ui.open === null) ui.open = state.cubes.length === 1;
  }, []);
  const kept = keptText(c.serial);
  return (
    <Fold cls="panel backup-fold" open={ui.open ?? state.cubes.length === 1} onToggle={(o) => { ui.open = o; }} data={{ serial: c.serial }}
      summary={<FoldHead before={<CubeIcon />} name={c.name || undefined} mono={c.serial} after={<CubeSign c={c} />}
        badges={[kept && <Badge key="kept" kind="mode" text={kept} />]} />}>
      <Box serial={c.serial} place="sys" />
    </Fold>
  );
}

/** SystemBackups is System settings' card: a fold per Cube, or that none is configured. */
export function SystemBackups() {
  useLook();
  if (!state.cubes.length) return <p class="muted small">{t("cubes.none")}</p>;
  return <>{state.cubes.map((c) => <CubeFold key={c.serial} c={c} />)}</>;
}

/** CubeBackup is Cube settings' card: the chosen Cube's box, nothing where none is chosen. */
export function CubeBackup() {
  useLook();
  const serial = state.serial;
  if (!serial) return null;
  return <Box key={serial} serial={serial} place="cube" />;
}
