// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/serverbackups/Targets.tsx
 *
 * Server backups' targets (AP ServerBackup, Block 4, D11-A, D12-A; NOTE
 * of the gate decisions, step 1). For a holder of backup.server a panel
 * per target, built once per target and updated in place: its name, its
 * kind, its marks (off, two of its periods behind, its secret to be
 * entered again), what it takes, what it keeps and its last copy; its
 * form, drawn from what the server says of its kind (its fields, their
 * types, which is secret, what a value is warned of: GET
 * …/server-backup-targets/kinds), so that a
 * kind a later block adds needs no form of its own: a secret is a
 * password field that never shows what is stored, written only; while
 * the page does not know a target's kind, its fields are shown off, with
 * why, and Save leaves them as they are; the
 * form is a settings form kept by the target's uuid (shared/forms.ts),
 * with Test, Remove, which asks, Reset and Save. The form that adds one
 * chooses the kind first. For a holder of backup.retry alone each
 * target's state, nothing more. A kind's name and its fields' names are
 * the texts serverbackup.kind.<kind> and serverbackup.field.<kind>.<key>,
 * the key itself where a text is missing.
 */

import { useRef, useState } from "preact/hooks";
import type { ServerBackupKind, ServerBackupPolicy, ServerBackupTarget, ServerBackupTargetState } from "../api";
import { Adder } from "../shared/adder";
import { Badge, ResultLine } from "../shared/badge";
import { Mark } from "../shared/icon";
import { type Draft, FormFooter, Meanwhile, NumberField, Select, Switch, TextField, useDraft } from "../shared/field";
import { Fold, FoldHead } from "../shared/fold";
import { focusForm, type Place, useForm } from "../shared/forms";
import { useLook } from "../shared/look";
import { can } from "../shared/me";
import { Question, BusyButton, busyProps, useAsk, useBusy } from "../shared/question";
import { showCard } from "../shared/card";
import { isShown, showView } from "../shared/pages";
import { fmtWhen } from "../shared/time";
import { addTarget, removeTarget, saveTarget, testTarget } from "./send";
import { card, DEFAULT_KEEP, KEEPS, type KeepKey, sb, TAKES, targetFormKey, targetUI } from "./store";

/** textOr is the text of key, else fallback where the texts have none (a kind or a field a later block adds without its text). */
function textOr(key: string, fallback: string): string {
  const s = t(key);
  return s === key ? fallback : s;
}

/** kindLabel names a kind of target. */
export function kindLabel(kind?: string): string {
  return textOr("serverbackup.kind." + kind, kind || "");
}

/** keepsText says what a retention keeps: each count that is not 0, or that it keeps every copy. */
export function keepsText(p?: ServerBackupPolicy): string {
  const parts = KEEPS.filter((k) => (p?.[k] || 0) > 0).map((k) => t("serverbackup.keepN." + k, { n: p?.[k] || 0 }));
  return parts.length ? t("serverbackup.keeps", { what: parts.join(", ") }) : t("serverbackup.keepsAll");
}

/** lastText says when a target last took a copy, or that it took none yet. */
export function lastText(s: ServerBackupTargetState): string {
  return s.last_success ? t("serverbackup.target.last", { when: fmtWhen(s.last_success) }) : t("serverbackup.target.never");
}

/** stateBadges are a target's marks: off, behind (two of its periods without a copy), its secret no longer opening. */
function stateBadges(s: ServerBackupTargetState & { secret_lost?: boolean }) {
  return [
    !s.enabled && <Badge key="off" kind="mode" text={t("serverbackup.target.off")} />,
    s.enabled && s.behind && <Badge key="behind" kind="warn" text={t("serverbackup.target.behind")} title={t("serverbackup.target.behindTitle")} />,
    s.secret_lost && <Badge key="lost" kind="bad" text={t("serverbackup.target.lostBadge")} />,
  ];
}

/** factsText is a target's line: what it takes, what it keeps, its last copy. */
function factsText(s: ServerBackupTargetState, p?: ServerBackupPolicy): string {
  return [t("serverbackup.takes." + s.takes), p ? keepsText(p) : "", lastText(s)].filter(Boolean).join(" · ");
}

/** A field's value: a draft of a stored value, or a free one of a new target's form. */
interface Val<T> {
  value: T;
  set: (v: T) => void;
  draft?: Draft<T>;
}

/**
 * useMerge sets one entry of a map's value after another, each on top of
 * the last, also where several come before the part is drawn again (a
 * form filled at once): the value a drawing saw is the latest until the
 * next drawing.
 */
function useMerge<K extends string>(v: Val<Record<K, string>>): (key: K, x: string) => void {
  const latest = useRef(v.value);
  latest.current = v.value;
  return (key, x) => {
    latest.current = { ...latest.current, [key]: x };
    v.set(latest.current);
  };
}

/** ofDraft is a draft as a field's value. */
function ofDraft<T>(d: Draft<T>): Val<T> {
  return { value: d.value, set: d.set, draft: d };
}

/** The values of a target's form; a kind's fields are one map, its secret apart. */
interface Values {
  name: Val<string>;
  config: Val<Record<string, string>>;
  takes: Val<string>;
  keep: Val<Record<KeepKey, string>>;
  enabled: Val<boolean>;
}

/** keepOf is a retention as the form's fields hold it, a text per count. */
function keepOf(p?: ServerBackupPolicy): Record<KeepKey, string> {
  const out = {} as Record<KeepKey, string>;
  for (const k of KEEPS) out[k] = String(p?.[k] || 0);
  return out;
}

/** policyOf is the form's retention as the API takes it. */
function policyOf(v: Record<KeepKey, string>): ServerBackupPolicy {
  const out: ServerBackupPolicy = {};
  for (const k of KEEPS) out[k] = Math.max(0, Math.trunc(Number(v[k]) || 0));
  return out;
}

/** sorted is a map as JSON with its keys sorted, how two maps compare. */
function sorted(v: Record<string, unknown>): string {
  return JSON.stringify(Object.keys(v).sort().map((k) => [k, v[k]]));
}

/** configNorm compares a kind's fields, a value trimmed, an empty one as none. */
function configNorm(v: Record<string, string>): string {
  const out: Record<string, string> = {};
  for (const [k, x] of Object.entries(v)) if (String(x).trim()) out[k] = String(x).trim();
  return sorted(out);
}

/** keepNorm compares a retention as numbers. */
const keepNorm = (v: Record<KeepKey, string>) => sorted(policyOf(v) as Record<string, unknown>);

/** fieldsOf are a kind's fields but its secret. */
function fieldsOf(kind?: ServerBackupKind) {
  return (kind?.fields || []).filter((f) => f.key !== kind?.secret && f.type !== "secret");
}

/** secretOf is a kind's secret field, none for a kind without one. */
function secretOf(kind?: ServerBackupKind) {
  return (kind?.fields || []).find((f) => f.key === kind?.secret || f.type === "secret");
}

/**
 * bodyOf is what Save sends: the name, what it takes, its retention,
 * whether it is on, and the kind's fields, which the server takes whole:
 * left out while the page does not know the kind (its kinds not read, or
 * a kind of a newer server), so that they stay as they are; the secret
 * only where one is typed and the kind has one.
 */
function bodyOf(v: Values, kind: ServerBackupKind | undefined, secret: string): Record<string, unknown> {
  const body: Record<string, unknown> = { name: v.name.value.trim(), takes: v.takes.value, retention: policyOf(v.keep.value), enabled: v.enabled.value };
  if (kind) {
    const config: Record<string, string> = {};
    for (const f of fieldsOf(kind)) config[f.key || ""] = String(v.config.value[f.key || ""] ?? "").trim();
    body.config = config;
  }
  if (secret && secretOf(kind)) body.secret = secret;
  return body;
}

/** fieldLabel names a kind's field: serverbackup.field.<kind>.<key>, else the key. */
function fieldLabel(kind: string, key: string): string {
  return textOr("serverbackup.field." + kind + "." + key, key);
}

/**
 * warns says whether a field's value, its default where it is empty,
 * matches the field's warn, a regular expression the server gives
 * (GET …/server-backup-targets/kinds); one this browser cannot read warns
 * of nothing.
 */
function warns(warn: string | undefined, value: string): boolean {
  if (!warn || !value) return false;
  try {
    return new RegExp(warn).test(value);
  } catch {
    return false;
  }
}

/**
 * KindField is one field of a kind's form, drawn from its type: a text
 * or a path (a directory of this box, relative to the data directory),
 * a number, a switch, or a choice; its default the placeholder, a
 * choice's and a switch's value where none is set. While its value
 * matches the field's warn, the note serverbackup.warn.<kind>.<key>, or
 * a general one where a later kind brings none, stands under it, amber
 * with its mark, and describes the control: a note, which keeps nothing
 * from being saved (plain http to a WebDAV target).
 */
function KindField({ id, kind, f, value, merge }: { id: string; kind: string; f: NonNullable<ServerBackupKind["fields"]>[number]; value: string; merge: (key: string, x: string) => void }) {
  const key = f.key || "";
  const label = fieldLabel(kind, key);
  const set = (x: string) => merge(key, x);
  const fid = id + "-" + key;
  const warned = warns(f.warn, value.trim() || f.default || "");
  const describedBy = warned ? fid + "-warn" : undefined;
  const note = warned && (
    <p id={fid + "-warn"} class="small warn-text sb-field-warn" role="note">
      <Mark state="warn" /> {textOr("serverbackup.warn." + kind + "." + key, t("serverbackup.target.warned"))}
    </p>
  );
  let field;
  switch (f.type) {
    case "bool":
      field = <Switch name={key} label={label} checked={(value || f.default) === "true"} onChange={(on) => set(on ? "true" : "false")} />;
      break;
    case "choice":
      field = (
        <Select label={label} id={fid} name={key} wide value={value || f.default || ""} onChange={set} describedBy={describedBy}
          options={(f.choices || []).map((c): [string, string] => [c, textOr("serverbackup.field." + kind + "." + key + "." + c, c)])} />
      );
      break;
    case "number":
      field = <NumberField label={label} id={fid} name={key} wide min={0} max={1e9} step={1} inputMode="numeric" value={value} set={set} describedBy={describedBy} />;
      break;
    default:
      field = (
        <TextField label={label} id={fid} name={key} placeholder={f.default} required={f.required} value={value} set={set} describedBy={describedBy}
          hint={f.type === "path" ? t("serverbackup.target.pathHint") : undefined} />
      );
  }
  return <>{field}{note}</>;
}

/** KeptFields are a target's fields as stored while the page does not know its kind: off, with why, and sent as they are, not at all. */
function KeptFields({ id, kind, config }: { id: string; kind: string; config: Record<string, string> }) {
  useLook();
  return (
    <>
      <p class="small warn-text sb-kind-unknown" role="note"><Mark state="warn" /> {t("serverbackup.target.kindUnknown", { kind: kindLabel(kind) })}</p>
      {Object.entries(config).map(([key, x]) => (
        <TextField key={key} label={fieldLabel(kind, key)} id={id + "-" + key} name={key} disabled value={x} set={() => undefined} />
      ))}
    </>
  );
}

/** Keeps is a target's retention: a count each, the last so many and the newest of so many days, weeks, months and years. */
function Keeps({ id, v }: { id: string; v: Val<Record<KeepKey, string>> }) {
  useLook();
  const merge = useMerge(v);
  return (
    <fieldset class="sb-keeps">
      <legend>{t("serverbackup.target.keeps")}</legend>
      <div class="sb-keeps-row">
        {KEEPS.map((k) => (
          <NumberField key={k} label={t("serverbackup.keep." + k)} id={id + "-" + k} name={"keep_" + k} min={0} max={10000} step={1} inputMode="numeric"
            value={v.value[k]} set={(x) => merge(k, x)} />
        ))}
      </div>
      <p class="muted small field-hint">{t("serverbackup.target.keepsHint")}</p>
      {v.draft && <Meanwhile draft={v.draft} show={(x) => keepsText(policyOf(x))} />}
    </fieldset>
  );
}

/** Fields are a target's fields, the ids under id: its name, its kind's fields and secret, what it takes, its retention, whether it is on. */
function Fields({ id, kind, v, secret, setSecret, stored }: {
  id: string;
  kind: ServerBackupKind | undefined;
  v: Values;
  secret: string;
  setSecret: (s: string) => void;
  stored?: ServerBackupTarget;
}) {
  useLook();
  const merge = useMerge(v.config);
  const f = (x: string) => "sb-target-" + id + "-" + x;
  const sec = secretOf(kind);
  const kname = kind?.name || stored?.kind || "";
  return (
    <>
      <TextField label={t("serverbackup.target.name")} id={f("name")} name="name" maxLength={64} required {...(v.name.draft ? { draft: v.name.draft } : { value: v.name.value, set: v.name.set })} />
      {stored && !kind ? <KeptFields id={f("field")} kind={kname} config={stored.config || {}} />
        : fieldsOf(kind).map((fl) => <KindField key={fl.key} id={f("field")} kind={kname} f={fl} value={v.config.value[fl.key || ""] ?? ""} merge={merge} />)}
      {kind && v.config.draft && (
        <Meanwhile draft={v.config.draft} show={(c) => Object.entries(c).map(([k, x]) => fieldLabel(kname, k) + ": " + x).join(", ")} />
      )}
      {sec && (
        <TextField label={textOr("serverbackup.field." + kname + "." + sec.key, sec.key || "")} id={f("secret")} name="secret" type="password"
          autocomplete="new-password" value={secret} set={setSecret}
          hint={t(stored?.secret_lost ? "serverbackup.target.secretLost" : stored?.has_secret ? "serverbackup.target.secretKept" : "serverbackup.target.secretNone")} />
      )}
      {kname === "local" && <p class="muted small sb-local-note">{t("serverbackup.target.localNote")}</p>}
      <Select label={t("serverbackup.target.takes")} id={f("takes")} name="takes" wide options={TAKES.map((x): [string, string] => [x, t("serverbackup.takes." + x)])}
        {...(v.takes.draft ? { draft: v.takes.draft } : { value: v.takes.value, onChange: v.takes.set })} />
      <Keeps id={f("keep")} v={v.keep} />
      <Switch name="enabled" label={t("serverbackup.target.enabled")} {...(v.enabled.draft ? { draft: v.enabled.draft } : { checked: v.enabled.value, onChange: v.enabled.set })} />
    </>
  );
}

/** openTarget shows a target's form and puts the focus into it, from any page: Maintenance shown, its card unfolded, its panel open. */
function openTarget(uuid: string): void {
  if (!isShown("maint")) showView("maint");
  showCard("sys-server-backups");
  targetUI.of(uuid).open = true;
  focusForm(targetFormKey(uuid));
}

/** useDrafts are a stored target's values, each a draft kept at its form's place. */
function useDrafts(tg: ServerBackupTarget, place: <T>(field: string) => Place<T>): Values {
  return {
    name: ofDraft(useDraft(tg.name || "", (x) => x.trim(), place("name"))),
    config: ofDraft(useDraft(tg.config || {}, configNorm, place("config"))),
    takes: ofDraft(useDraft<string>(tg.takes || "every", undefined, place("takes"))),
    keep: ofDraft(useDraft(keepOf(tg.retention), keepNorm, place("keep"))),
    enabled: ofDraft(useDraft(tg.enabled !== false, undefined, place("enabled"))),
  };
}

/** TargetForm is a target's settings in its panel: its fields, Test, Remove, which asks in place of the actions, Reset and Save; the secret written only. */
function TargetForm({ tg }: { tg: ServerBackupTarget }) {
  useLook();
  const uuid = tg.uuid || "";
  const kind = (sb.kinds || []).find((k) => k.name === tg.kind);
  const form = useForm({ key: targetFormKey(uuid), name: { key: "serverbackup.target.formName", args: { name: tg.name || "" } }, open: () => openTarget(uuid) });
  const v = useDrafts(tg, form.place);
  // written only: the part's own state, not the forms' store
  const [secret, setSecret] = useState("");
  const ask = useAsk<HTMLButtonElement>();
  const drafts = [v.name, v.config, v.takes, v.keep, v.enabled].map((x) => x.draft as Draft<unknown>);
  const changed = drafts.some((x) => x.changed) || secret !== "";
  form.track(changed, [...drafts, { take: () => setSecret("") }]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    form.save(() => saveTarget(tg, bodyOf(v, kind, secret)));
  };
  return (
    <form {...form.props} class="sb-target-form" onSubmit={submit}>
      <Fields id={uuid} kind={kind} v={v} secret={secret} stored={tg} setSecret={(x) => {
        setSecret(x);
        form.place("secret").changed();
      }} />
      {ask.asking ? (
        <Question text={t("serverbackup.target.removeAsk", { name: tg.name || "" })} yes={t("serverbackup.target.remove")} no={t("action.keep")} boxRef={ask.box}
          onNo={() => ask.done()} onYes={async () => {
            if (!(await removeTarget(tg))) ask.done();
          }} />
      ) : (
        <FormFooter submit={t("action.save")} look={form.look} reset={form.reset} off={!changed}
          extra={(
            <>
              <BusyButton cls="sb-target-test" label={t("serverbackup.target.test")} run={() => testTarget(tg)} />
              <button ref={ask.ref} type="button" class="sb-target-remove" onClick={ask.ask}>{t("serverbackup.target.remove")}</button>
            </>
          )} />
      )}
      <ResultLine cls="small cubes-msg sb-target-msg" role="status" res={targetUI.of(uuid).said} />
    </form>
  );
}

/** TargetPanel is a target's panel: its head with its name, its kind and its marks; its line; its form. */
function TargetPanel({ tg }: { tg: ServerBackupTarget }) {
  useLook();
  const uuid = tg.uuid || "";
  const ui = targetUI.of(uuid);
  return (
    <Fold cls="panel sb-target" open={ui.open} onToggle={(o) => { ui.open = o; }} data={{ uuid }}
      summary={<FoldHead name={tg.name} after={<span class="muted small">{kindLabel(tg.kind)}</span>} badges={stateBadges(tg)} />}>
      <p class="muted small sb-target-facts">{factsText(tg, tg.retention)}</p>
      <TargetForm tg={tg} />
    </Fold>
  );
}

/** NewTarget is the form that adds a target: the kind first, then its fields, as a free form of its own, the defaults of the kind and of the server; another kind chosen forgets the secret typed. */
function NewTarget() {
  useLook();
  const kinds = sb.kinds || [];
  const [chosen, setKind] = useState("");
  // the first kind until one is chosen, also where the kinds come after the form
  const kname = chosen || (kinds.length ? kinds[0].name || "" : "");
  const kind = kinds.find((k) => k.name === kname);
  const [name, setName] = useState("");
  const [configs, setConfigs] = useState<Record<string, Record<string, string>>>({});
  const [takes, setTakes] = useState<string>("every");
  const [keep, setKeep] = useState(keepOf(DEFAULT_KEEP));
  const [enabled, setEnabled] = useState(true);
  const [secret, setSecret] = useState("");
  const add = useBusy();
  const v: Values = {
    name: { value: name, set: setName },
    config: { value: configs[kname] || {}, set: (c) => setConfigs({ ...configs, [kname]: c }) },
    takes: { value: takes, set: setTakes },
    keep: { value: keep, set: setKeep },
    enabled: { value: enabled, set: setEnabled },
  };
  const submit = (ev: Event) => {
    ev.preventDefault();
    void add.run(() => addTarget({ ...bodyOf(v, kind, secret), kind: kname })).then((ok) => {
      if (ok) card.addingTarget = false;
    });
  };
  return (
    <form class="panel sb-target-form sb-target-new" onSubmit={submit}>
      <Select label={t("serverbackup.target.kind")} id="sb-target-new-kind" name="kind" wide value={kname} onChange={(k) => {
        // a secret typed for one kind is not another's
        setKind(k);
        setSecret("");
      }}
        options={kinds.map((k): [string, string] => [k.name || "", kindLabel(k.name)])} />
      <Fields id="new" kind={kind} v={v} secret={secret} setSecret={setSecret} />
      <div class="actions">
        <span class="actions-right">
          <button type="button" class="form-cancel" onClick={() => { card.addingTarget = false; }}>{t("action.cancel")}</button>
          <button type="submit" {...busyProps("primary", add.busy, !kind)}>{t("serverbackup.target.addOne")}</button>
        </span>
      </div>
    </form>
  );
}

/** TargetState is a target as a holder of backup.retry alone sees it: its name, its kind, its marks and its line. */
function TargetState({ s }: { s: ServerBackupTargetState }) {
  useLook();
  return (
    <li class="sb-target-state" data-uuid={s.uuid}>
      <div class="sb-row-head">
        <span class="panel-name">{s.name}</span>
        <span class="muted small">{kindLabel(s.kind)}</span>
        {stateBadges(s)}
      </div>
      <p class="muted small">{factsText(s)}</p>
    </li>
  );
}

/** Targets is the targets' part of the card: what a target is, the panels and the form that adds one, or each target's state alone, and the line. */
export function Targets() {
  useLook();
  const server = can("backup.server");
  const states = sb.list?.targets || [];
  const none = server ? sb.targetsLoaded && !sb.targets.length : !!sb.list && !states.length;
  return (
    <section class="sb-group sb-targets" aria-labelledby="sb-targets-h">
      <h4 id="sb-targets-h" class="sb-group-title">{t("serverbackup.targets")}</h4>
      {none && <p class="muted small sb-targets-none">{t("serverbackup.targetsNone")}</p>}
      {server ? (
        <>
          <div class="sb-target-list">{sb.targets.map((tg) => <TargetPanel key={tg.uuid} tg={tg} />)}</div>
          <Adder id="sb-target-add" formId="sb-target-new" label={t("serverbackup.target.add")} shown={card.addingTarget} set={(x) => { card.addingTarget = x; }}>
            <NewTarget />
          </Adder>
        </>
      ) : <ul class="sb-target-states">{states.map((s) => <TargetState key={s.uuid} s={s} />)}</ul>}
      <ResultLine id="sb-targets-msg" cls="small cubes-msg" role="status" res={card.targets} />
    </section>
  );
}
