// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/backups/Backups.tsx
 *
 * The Cubes' backups as parts, a box per Cube and place: on Cube settings
 * the chosen Cube's, in Maintenance's Cube backups a fold per Cube, each built once
 * and updated in place, so that a fold open, a question asked and the
 * format chosen stay when data comes in. A box holds the format, Download
 * backup, Take a backup now where the server keeps backups, Restore from
 * file…, the automatic backup pending, the backups kept, each with
 * Download, Restore…, Compare with the one before and Delete, which asks
 * in its row, under them that nothing changed since the newest where none
 * is pending, and the restore shown with its steps and Send, then how
 * each step went. A restore is a command, no settings form. After the
 * served Cubes' folds, a fold for each Cube the server keeps backups of
 * and does not serve, marked so, whose box keeps the format, the backups
 * kept with Download, Compare with the one before and Delete, and a note
 * that a backup of it goes onto another Cube by that Cube's Restore from
 * file…: no Download backup, Take a backup now, Restore… or Restore from
 * file…, which need the Cube served (AP BackupWatch v1.0, D4-B).
 */

import { useEffect, useRef } from "preact/hooks";
import type { Cube, StoredBackup, UnservedCube } from "../api";
import { CubeIcon, CubeSign } from "../model/cube";
import { Badge, ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Select } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { FileButton } from "../shared/file";
import { BusyButton, Question, useAsk } from "../shared/question";
import { fmtWhen } from "../shared/time";
import { useLook } from "../shared/look";
import { isShown } from "../shared/pages";
import {
  boxOf, closePlan, compareBackup, deleteBackup, downloadNow, downloadStored, foldUI, of, planFile, planStored, readUnloaded, running, sendRestore, takeBackup,
  unserved,
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

/**
 * Row is a stored backup: when, why, the size and who, the note, and its
 * actions; older is the one before it, to compare with; a Cube not served
 * has no Restore….
 */
function Row({ place, serial, b, older, served }: { place: string; serial: string; b: StoredBackup; older?: StoredBackup; served: boolean }) {
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
        {served && <BusyButton cls="small needs-restore" label={t("backup.restore")} off={running(serial)} run={() => planStored(place, serial, b)} />}
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

/** The page of each place's boxes. */
const placeView: Record<string, string> = { cube: "cube", sys: "maint" };

/**
 * Box is a Cube's box for a place ("cube" or "sys"), which its ids carry,
 * as both pages may hold the same Cube's. Drawn on the page shown before
 * its list was read, as on a load onto the page, which shows it before it
 * knows its Cubes, it reads the list itself, once. A Cube not served
 * (served false) has the format, a note and its backups kept, and none of
 * what needs it served.
 */
export function Box({ serial, place, served = true }: { serial: string; place: string; served?: boolean }) {
  useLook();
  const c = of(serial);
  const ui = boxOf(place, serial);
  const loaded = c.loaded;
  // a list that no showing of its page read, read once the box is drawn there
  useEffect(() => {
    if (!loaded && isShown(placeView[place])) readUnloaded(serial);
  }, [loaded, place, serial]);
  const due = !!(c.changed && c.due);
  const list = c.list;
  // a clean Cube's list, nothing pending: why no newer backup follows
  const clean = served && loaded && c.history && list.length > 0 && !c.changed && !c.due;
  const format = "backup-format-" + place + "-" + serial;
  return (
    <div class="backup-box" data-serial={serial}>
      <div class="backup-bar">
        <Select label={t("backup.format")} id={format} options={[["yaml", "YAML"], ["json", "JSON"]]} value={ui.format} onChange={(v) => { ui.format = v; }} />
        {served && <BusyButton cls="backup-download" label={t("backup.download")} run={() => downloadNow(place, serial)} />}
        {served && <BusyButton cls={"backup-take" + hid(!c.history)} label={t("backup.take")} run={() => takeBackup(place, serial)} />}
        {served && (
          <FileButton cls="restore-file needs-restore" label={t("restore.fromFile")} accept=".json,.yaml,.yml,application/json,application/yaml" off={running(serial)}
            onFile={(file) => planFile(place, serial, file)} />
        )}
      </div>
      {!served && <p class="backup-unserved-note muted small">{t("backup.notServedNote")}</p>}
      <p class={"backup-pending muted small" + hid(!due)}>{due ? t("backup.pending", { changed: fmtWhen(c.changed || ""), due: fmtWhen(c.due || "") }) : ""}</p>
      <div class="backups-list">
        {c.loaded && !c.history && <p class="muted small">{t("backup.noHistory")}</p>}
        {c.loaded && c.history && !list.length && <p class="muted small">{t("backup.none")}</p>}
        {c.loaded && c.history && list.map((b, i) => <Row key={b.id} place={place} serial={serial} b={b} older={list[i + 1]} served={served} />)}
        {clean && <p class="backup-clean muted small">{t("backup.unchanged")}</p>}
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

/** CubeFold is a Cube's fold in Cube backups: the cube, its name and serial, its network sign after them, the count kept, and its box. */
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

/**
 * UnservedFold is the fold in Cube backups of a Cube the server keeps
 * backups of and does not serve: the cube, its name where its row has
 * one, its serial, that it is not served and the count kept in its head,
 * closed at first, and its box without what needs the Cube served.
 */
function UnservedFold({ c }: { c: UnservedCube }) {
  useLook();
  const ui = foldUI.of(c.serial);
  const kept = keptText(c.serial);
  return (
    <Fold cls="panel backup-fold backup-unserved" open={ui.open ?? false} onToggle={(o) => { ui.open = o; }} data={{ serial: c.serial }}
      summary={<FoldHead before={<CubeIcon />} name={c.name || undefined} mono={c.serial}
        badges={[<Badge key="unserved" kind="mode" text={t("backup.notServed")} />, kept && <Badge key="kept" kind="mode" text={kept} />]} />}>
      <Box serial={c.serial} place="sys" served={false} />
    </Fold>
  );
}

/**
 * CubeFolds is what Maintenance's Cube backups holds: a fold per Cube
 * served, or that none is configured, then a fold per Cube not served
 * that has backups.
 */
export function CubeFolds() {
  useLook();
  const others = unserved();
  return (
    <>
      {!state.cubes.length && <p class="muted small">{t("cubes.none")}</p>}
      {state.cubes.map((c) => <CubeFold key={c.serial} c={c} />)}
      {others.map((c) => <UnservedFold key={c.serial} c={c} />)}
    </>
  );
}

/** CubeBackup is Cube settings' card: the chosen Cube's box, nothing where none is chosen. */
export function CubeBackup() {
  useLook();
  const serial = state.serial;
  if (!serial) return null;
  return <Box key={serial} serial={serial} place="cube" />;
}
