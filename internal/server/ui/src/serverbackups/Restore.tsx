// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/Restore.tsx
 *
 * The owner's dialog of a server backup (AP ServerBackup, Block 5, D6-A,
 * D7-A): Restore… and Check with the key…, a modal dialog for the backup
 * its row opened it for. First the key, pasted or read from its file in
 * the browser, held while the dialog is open and stored nowhere. To
 * restore, then what the backup holds as the plan answers it: each scope
 * with its counts and each Cube by its serial, ticked by the owner;
 * where the backup is another box's, "Take the backup's identity" with
 * its warning, and that the other box still answers; the env file and
 * the TLS files, off with why where this server cannot put them in place;
 * users and sign-in and the identity off where it cannot restart itself;
 * what every scope together, or the choice, would refuse, with the rows
 * missing. Then a confirmation that names what it replaces and that a
 * backup is taken first, and the restore: its outcome, the server's
 * restart as the page follows it, and the Cubes the Cube restore could
 * write, each a link to it. To check, the verdict. Each step gives the
 * focus to its first control or its heading as it is shown.
 */

import type { JSX, Ref } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import type { ServerRestorePlan, ServerRestoreReport } from "../api";
import { Notice, ResultLine } from "../shared/badge";
import { Modal } from "../shared/dialog";
import { Field, Switch } from "../shared/field";
import { FileButton } from "../shared/file";
import { useLook } from "../shared/look";
import { BusyButton, busyProps, useBusy } from "../shared/question";
import { fmtWhen } from "../shared/time";
import {
  backToChoice, backToKey, chosenAny, closeRestore, forgetKeyFile, hasKey, openBackup, planChoice, readKeyFile, restoreRestart, rs, runCheck, runRestore,
  SCOPES, tick, tickFile, toCubeRestore,
} from "./restoring";

/** countsOf sums a report's rows per scope: those taken from the backup and this server's that go. */
function countsOf(report?: ServerRestoreReport): Record<string, { restored: number; removed: number }> {
  const out: Record<string, { restored: number; removed: number }> = {};
  for (const tb of report?.tables || []) {
    const c = (out[tb.scope || ""] ||= { restored: 0, removed: 0 });
    c.restored += tb.restored || 0;
    c.removed += (tb.removed || 0) - (tb.kept_removed || 0);
  }
  return out;
}

/** cubeName is a Cube of the plan as a text names it: its name and serial. */
function cubeName(p: ServerRestorePlan | null, serial: string): string {
  const c = (p?.cubes || []).find((x) => x.serial === serial);
  return c?.name ? t("serverbackup.restore.cube", { name: c.name, serial }) : serial;
}

/** choiceList is what is ticked, in the page's words: the scopes, each Cube, the identity, the env file and the TLS files. */
function choiceList(): string[] {
  const out = rs.scopes.map((s) => (s.startsWith("cube:") ? cubeName(rs.plan, s.slice(5)) : t("serverbackup.restore.scope." + s)));
  if (rs.identity) out.push(t("serverbackup.restore.identityItem"));
  if (rs.env) out.push(t("serverbackup.restore.envItem"));
  if (rs.tls) out.push(t("serverbackup.restore.tlsItem"));
  return out;
}

/** Refusal is why the choice, or every scope together, would be refused, and the rows missing it lists. */
function Refusal() {
  useLook();
  if (!rs.refused) return null;
  return (
    <div class="sb-restore-refused" role="alert">
      <p class="small bad">{t(rs.refusedAll ? "serverbackup.restore.refusedAll" : "serverbackup.restore.refused", { error: rs.refused })}</p>
      {rs.missing.length > 0 && (
        <>
          <p class="small">{t("serverbackup.restore.missing")}</p>
          <ul class="small sb-restore-missing">
            {rs.missing.map((m, i) => <li key={i}>{t("serverbackup.restore.missingRow", { name: m.name || m.uuid || "", table: m.table || "", by: m.by || "" })}</li>)}
          </ul>
        </>
      )}
    </div>
  );
}

/** KeyStep is the key, pasted or read from its file, and the step's button: Open the backup, or Check. */
function KeyStep({ focus }: { focus: Ref<HTMLElement> }) {
  useLook();
  const send = useBusy();
  const text = useRef<HTMLTextAreaElement>(null);
  const forgot = useRef(false);
  const check = rs.mode === "check";
  // the file forgotten, the field for a key pasted takes the focus in the drawing that shows it
  // biome-ignore lint/correctness/useExhaustiveDependencies: the file is the trigger; forgot says it went by Forget the file
  useLayoutEffect(() => {
    if (rs.file || !forgot.current) return;
    forgot.current = false;
    text.current?.focus();
  }, [rs.file]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (hasKey()) void send.run(check ? runCheck : openBackup);
  };
  return (
    <form class="sb-restore-step sb-restore-key" onSubmit={submit}>
      <p class="small">{t("serverbackup.restore.keyText")}</p>
      {rs.file ? (
        <p class="sb-restore-keyfile">
          <span>{t("serverbackup.restore.keyFrom", { name: rs.file.name })}</span>
          <button ref={focus as Ref<HTMLButtonElement>} type="button" class="small sb-restore-keyfile-forget" onClick={() => {
            forgot.current = true;
            forgetKeyFile();
          }}>{t("serverbackup.restore.keyForget")}</button>
        </p>
      ) : (
        <Field label={t("serverbackup.restore.key")} id="sb-restore-key" wide>
          <textarea ref={(el) => {
            text.current = el;
            if (typeof focus === "function") focus(el);
            else if (focus) focus.current = el;
          }} id="sb-restore-key" class="field-control mono" rows={4} autocomplete="off" spellcheck={false} placeholder="AGE-SECRET-KEY-…"
            value={rs.key} onInput={(ev) => { rs.key = ev.currentTarget.value; }} />
        </Field>
      )}
      <div class="actions sb-restore-keyfile-actions">
        <FileButton id="sb-restore-keyfile" cls="small" label={t("serverbackup.restore.keyFile")} accept="" inputId="sb-restore-keyfile-input"
          inputLabel={t("serverbackup.restore.keyFileInput")} onFile={(f) => void readKeyFile(f)} />
      </div>
      <ResultLine id="sb-restore-msg" cls="small cubes-msg" role="alert" res={rs.said} />
      <div class="actions">
        <button type="button" class="form-cancel" onClick={closeRestore}>{t("action.cancel")}</button>
        <button type="submit" id="sb-restore-open" {...busyProps("primary", send.busy, !hasKey())}>
          {t(check ? "serverbackup.checkKey.run" : "serverbackup.restore.open")}
        </button>
      </div>
    </form>
  );
}

/** ScopeSwitch is a scope's switch, or a Cube's, with its line: its counts, or whether this server has the Cube. */
function ScopeSwitch({ scope, label, line, off }: { scope: string; label: string; line: string; off?: boolean }) {
  useLook();
  const id = "sb-restore-scope-" + scope.replace(":", "-");
  return (
    <Switch id={id} name={scope} label={label} checked={rs.scopes.includes(scope)} disabled={off} data={{ scope }} rowCls="sb-restore-scope"
      onChange={(on) => tick(scope, on)}>
      <p class="muted small field-hint sb-restore-line">{line}</p>
    </Switch>
  );
}

/** ChooseStep is what the backup holds, as its plan answers it, to tick: the scopes, the Cubes, the identity and the files; the refusal; Restore…. */
function ChooseStep({ focus }: { focus: Ref<HTMLElement> }) {
  useLook();
  const send = useBusy();
  const p = rs.plan as ServerRestorePlan;
  const restarts = !!p.restart?.available;
  const blocked = p.restart?.blocked || "";
  const counts = countsOf(p.report);
  const other = !p.identity?.this_box;
  const tls = p.tls || 0;
  const submit = (ev: Event) => {
    ev.preventDefault();
    void send.run(planChoice);
  };
  return (
    <form class="sb-restore-step sb-restore-choose" onSubmit={submit}>
      <h4 ref={focus as Ref<HTMLHeadingElement>} tabIndex={-1} class="sb-restore-h">{t("serverbackup.restore.chooseTitle")}</h4>
      <p class="muted small sb-restore-made">
        {[t("serverbackup.restore.made", { when: p.backup?.time ? fmtWhen(p.backup.time) : "", version: p.backup?.version || "" }),
          other ? t("serverbackup.restore.otherBox") : "", p.migrated ? t("serverbackup.restore.migrated") : ""].filter(Boolean).join(" ")}
      </p>
      {!restarts && <Notice id="sb-restore-no-restart" role="note" text={t("serverbackup.restore.noRestart", { blocked })} />}
      <fieldset class="sb-restore-group sb-restore-scopes">
        <legend>{t("serverbackup.restore.what")}</legend>
        <p class="muted small">{t("serverbackup.restore.whatText")}</p>
        {SCOPES.map((s) => (
          <ScopeSwitch key={s} scope={s} label={t("serverbackup.restore.scope." + s)} off={s === "users" && !restarts}
            line={t("serverbackup.restore.count", { restored: fmtNum(counts[s]?.restored || 0, 0), removed: fmtNum(counts[s]?.removed || 0, 0) })} />
        ))}
        {(p.cubes || []).map((c) => (
          <ScopeSwitch key={c.serial} scope={"cube:" + c.serial} label={cubeName(p, c.serial || "")}
            line={t(c.live ? "serverbackup.restore.cubeLive" : "serverbackup.restore.cubeNew")} />
        ))}
      </fieldset>
      {(other || p.env || tls > 0) && (
        <fieldset class="sb-restore-group sb-restore-files">
          <legend>{t("serverbackup.restore.also")}</legend>
          {other && (
            <Switch id="sb-restore-identity" name="identity" label={t("serverbackup.restore.identity")} checked={rs.identity} disabled={!restarts}
              rowCls="sb-restore-scope" onChange={(on) => tickFile("identity", on)}>
              <p class="small field-hint warn-text sb-restore-line">{t("serverbackup.restore.identityWarn")}</p>
              {p.identity?.answers && <p id="sb-restore-answers" class="small field-hint bad sb-restore-line" role="note">{t("serverbackup.restore.answers")}</p>}
            </Switch>
          )}
          {p.env && (
            <Switch id="sb-restore-env" name="env" label={t("serverbackup.restore.env")} checked={rs.env} disabled={!!p.env_refused} rowCls="sb-restore-scope"
              onChange={(on) => tickFile("env", on)}>
              {p.env_refused && <p id="sb-restore-env-off" class="muted small field-hint sb-restore-off sb-restore-line">{p.env_refused}</p>}
            </Switch>
          )}
          {tls > 0 && (
            <Switch id="sb-restore-tls" name="tls" label={t("serverbackup.restore.tls", { n: tls })} checked={rs.tls} disabled={!!p.tls_refused}
              rowCls="sb-restore-scope" onChange={(on) => tickFile("tls", on)}>
              {p.tls_refused && <p id="sb-restore-tls-off" class="muted small field-hint sb-restore-off sb-restore-line">{p.tls_refused}</p>}
            </Switch>
          )}
          {(p.env || tls > 0) && <p class="muted small">{t("serverbackup.restore.filesText")}</p>}
        </fieldset>
      )}
      <Refusal />
      <ResultLine id="sb-restore-msg" cls="small cubes-msg" role="alert" res={rs.said} />
      <div class="actions">
        <button type="button" class="form-cancel" onClick={backToKey}>{t("serverbackup.restore.back")}</button>
        <button type="submit" id="sb-restore-next" {...busyProps("primary", send.busy, !chosenAny())}>{t("serverbackup.restore.next")}</button>
      </div>
    </form>
  );
}

/** ConfirmStep names what the restore replaces, that a backup is taken first and whether the server then restarts; Back and Restore. */
function ConfirmStep({ focus }: { focus: Ref<HTMLElement> }) {
  useLook();
  const p = rs.plan as ServerRestorePlan;
  const restarts = !!p.restart?.available;
  return (
    <div class="sb-restore-step sb-restore-confirm">
      <h4 ref={focus as Ref<HTMLHeadingElement>} tabIndex={-1} class="sb-restore-h">{t("serverbackup.restore.confirmTitle")}</h4>
      <p class="small">{t("serverbackup.restore.replaces")}</p>
      <ul class="small sb-restore-what">{choiceList().map((w, i) => <li key={i}>{w}</li>)}</ul>
      {rs.identity && <Notice role="note" text={t("serverbackup.restore.identityWarn")} />}
      <p class="small">{t("serverbackup.restore.backupFirst")}</p>
      <p class="small">{restarts ? t("serverbackup.restore.restarts") : t("serverbackup.restore.restartByHand", { blocked: p.restart?.blocked || "" })}</p>
      <ResultLine id="sb-restore-msg" cls="small cubes-msg" role="alert" res={rs.said} />
      <div class="actions">
        <button type="button" class="form-cancel" disabled={rs.running} onClick={backToChoice}>{t("serverbackup.restore.back")}</button>
        <BusyButton id="sb-restore-run" cls="primary" label={t("serverbackup.restore.run")} run={runRestore} />
      </div>
    </div>
  );
}

/** RestartLine is how the server's restart after the restore goes: followed, back, not back, or why it did not restart. */
function RestartLine() {
  useLook();
  const r = rs.result;
  if (!r) return null;
  if (!r.restart?.started) {
    return <p id="sb-restore-restart" class="small cubes-msg bad" role="status">{t("serverbackup.restore.restartBlocked", { blocked: r.restart?.blocked || "" })}</p>;
  }
  const said = restoreRestart.text;
  return <p id="sb-restore-restart" class={"small cubes-msg" + (restoreRestart.on ? "" : restoreRestart.ok ? " ok" : " bad")} role="status">{said ? said() : ""}</p>;
}

/** DoneStep is the restore's outcome: what it restored, an error after the store changed, secrets to enter again, the gap, the restart and the Cube restore's links. */
function DoneStep({ focus }: { focus: Ref<HTMLElement> }) {
  useLook();
  const r = rs.result;
  if (!r) return null;
  const lost = r.report?.secrets_lost || 0;
  const cubes = (r.cubes || []).filter((c) => c.backup && c.serial);
  return (
    <div class="sb-restore-step sb-restore-done">
      <h4 ref={focus as Ref<HTMLHeadingElement>} tabIndex={-1} class="sb-restore-h">{t("serverbackup.restore.doneTitle")}</h4>
      <ul class="small sb-restore-what">{choiceList().map((w, i) => <li key={i}>{w}</li>)}</ul>
      {r.error && <p id="sb-restore-error" class="small bad">{t("serverbackup.restore.error", { error: r.error })}</p>}
      {lost > 0 && <Notice role="note" text={tn("serverbackup.restore.secretsLost", lost)} />}
      {r.before_backup_time && r.restored_at && (
        <p class="muted small sb-restore-gap">{t("serverbackup.restore.gap", { before: fmtWhen(r.before_backup_time), at: fmtWhen(r.restored_at) })}</p>
      )}
      <RestartLine />
      {cubes.length > 0 && !restoreRestart.on && (
        <div class="sb-restore-cubes">
          <p class="small">{t("serverbackup.restore.cubes")}</p>
          <ul class="small">
            {cubes.map((c) => (
              <li key={c.serial}>
                <a href="#cube" class="sb-cube-restore" data-serial={c.serial} onClick={(ev) => {
                  ev.preventDefault();
                  toCubeRestore(c.serial || "");
                }}>{t("serverbackup.restore.cubeLink", { cube: c.name ? t("serverbackup.restore.cube", { name: c.name, serial: c.serial || "" }) : c.serial || "" })}</a>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div class="actions">
        <button type="button" class="primary" onClick={closeRestore}>{t("action.close")}</button>
      </div>
    </div>
  );
}

/** VerdictStep is the check's verdict: whether the store is whole, which box made the backup and the files it holds. */
function VerdictStep({ focus }: { focus: Ref<HTMLElement> }) {
  useLook();
  const v = rs.verdict;
  if (!v) return null;
  const files = (v.files || []).map((f) => f.name || "").filter(Boolean);
  return (
    <div class="sb-restore-step sb-check-verdict" data-ok={v.ok ? "true" : "false"}>
      <h4 ref={focus as Ref<HTMLHeadingElement>} tabIndex={-1} class={"sb-restore-h " + (v.ok ? "ok" : "bad")}>
        {v.ok ? t("serverbackup.checkKey.ok") : t("serverbackup.checkKey.bad", { integrity: (v.integrity || []).join("; ") })}
      </h4>
      <p class="muted small">{t(v.this_box ? "serverbackup.checkKey.thisBox" : "serverbackup.checkKey.otherBox")}</p>
      {files.length > 0 && <p class="muted small sb-check-files">{t("serverbackup.checkKey.files", { files: files.join(", ") })}</p>}
      <div class="actions">
        <button type="button" class="primary" onClick={closeRestore}>{t("action.close")}</button>
      </div>
    </div>
  );
}

/** RestoreDialog is the dialog, for the backup a row opened it for: the step shown, the focus given to it as it comes. */
export function RestoreDialog() {
  useLook();
  const b = rs.backup;
  const step = rs.step;
  const focus = useRef<HTMLElement>(null);
  const when = b?.time ? fmtWhen(b.time) : "";
  // each step shown, its first control or its heading takes the focus, the dialog opened by then (Modal's effect runs first)
  // biome-ignore lint/correctness/useExhaustiveDependencies: the step and the backup are the triggers; focus is a ref
  useLayoutEffect(() => {
    if (b) focus.current?.focus();
  }, [step, b?.uuid]);
  const steps: Record<string, (props: { focus: Ref<HTMLElement> }) => JSX.Element | null> = {
    key: KeyStep, choose: ChooseStep, confirm: ConfirmStep, done: rs.mode === "check" ? VerdictStep : DoneStep,
  };
  const Step = steps[step];
  return (
    <Modal shown={!!b} cls="sb-restore-dialog" held={rs.running} onClose={closeRestore}
      title={b ? t(rs.mode === "check" ? "serverbackup.checkKey.title" : "serverbackup.restore.title", { when }) : ""}>
      {b && <Step focus={focus} />}
    </Modal>
  );
}
