// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/FromBackup.tsx
 *
 * The start from a backup in the sign-in card, in the first owner's
 * form's place (AP ServerBackup, Block 11, D6-A, D7-A): a new box takes
 * every scope of another box's backup and restarts with its users
 * (docs/api-v1.md, "A new box from a backup"). The backup is a file of
 * this device, sent as it is (POST /api/v1/setup/backup), or one of a
 * target's: its kind chosen of those the server reads from (GET
 * …/server-backup-targets/kinds, none that makes its own key), its fields
 * drawn from what the server says of the kind, as Server backups' form
 * is, with their texts (serverbackup.field.<kind>.<key>), used to list
 * the target's backups (…/setup/backup/list) and to fetch the one picked
 * (…/fetch), and stored nowhere: its fields and its secret are emptied
 * once the backup is fetched, when the card closes and at a sign-out
 * (the security review of Block 11). Its manifest says when it was made, by
 * which Massimo, what it holds, which keys open it and whether it comes
 * from a newer Massimo. The key, pasted into its field or read into it
 * from a file in the browser, goes in the body of the plan and of the
 * restore alone, which name the backup handed by its uuid, so that one
 * handed meanwhile is refused; the field is not controlled, and is emptied at each
 * opening, at Back, when the card closes and once the restore is sent.
 * Open with the key plans the restore (…/plan): everything replaced, the
 * Cubes, and the switches of the backup's identity, with the old box's
 * answer on the network warned of (D7-A), of its env file and of its TLS
 * files where it holds them, each switch planned again; the plan's
 * refusal is said, Start from this backup off with it. Start asks first,
 * then restores (…/restore) and follows the restart as Maintenance
 * follows one, by /healthz's start; then who is in is read: the server
 * has the backup's users, since it restores no backup without an enabled
 * owner, and the card turns to the sign-in saying so (store.ts restoring
 * and toSignIn). While a request runs the card is
 * working: Back and the card's Select server are off, and Escape leaves
 * nothing (store.ts leaves). A refusal
 * for lack of a session turns the card to the sign-in, and a 409 reads
 * who is in, which turns it where the server has users now; anything
 * else is said under the step.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import type { NewBoxBackup, ServerBackupKind, ServerRestorePlan, ServerRestoreResult } from "../api";
import type { Outcome } from "../shared/action";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Field, NumberField, Select, Switch, TextField } from "../shared/field";
import { FileButton } from "../shared/file";
import { useLang } from "../shared/lang";
import { Question, busyProps, useAsk } from "../shared/question";
import { fmtWhen } from "../shared/time";
import { backupBack, card, restoring } from "./store";

/** SETUP is the routes' path, the first owner's window's (docs/api-v1.md). */
const SETUP = "api/v1/setup/backup";

/** RESTART_POLL_MS is how often /healthz is asked while the server restarts, RESTART_WAIT_MS how long it is waited for, as Maintenance's restart does. */
const RESTART_POLL_MS = 2000;
const RESTART_WAIT_MS = 2 * 60 * 1000;

/** Where the backup comes from: a file of this device, or a target. */
type Source = "file" | "target";

/** The switches of the restore: the backup's identity, its env file, its TLS files. */
interface Options {
  identity: boolean;
  env: boolean;
  tls: boolean;
}

const NO_OPTIONS: Options = { identity: false, env: false, tls: false };

/** textOr is the text of key, else fallback where the texts have none (a kind or a field a later block adds without its text). */
function textOr(key: string, fallback: string): string {
  const s = t(key);
  return s === key ? fallback : s;
}

/** heldText says what the backup handed is: when, by which Massimo, what it holds. */
function heldText(b: NewBoxBackup): string {
  const c = b.counts || {};
  return t("newBox.held", { time: b.time ? fmtWhen(b.time) : "", version: b.version || "" }) + " " +
    t("newBox.holds", { users: String(c.users ?? 0), cubes: String(c.cubes ?? 0), rooms: String(c.rooms ?? 0) });
}

/** keysText names the keys a backup opens with, by their labels, else their fingerprints. */
function keysText(b: NewBoxBackup): string {
  return (b.recipients || []).map((r) => r.label || r.fingerprint || "").filter(Boolean).join(", ");
}

/** optionText names a backup of a target's list in its select: when, by which Massimo, a newer one marked. */
function optionText(b: NewBoxBackup): string {
  return [b.time ? fmtWhen(b.time) : b.name || "", b.version || "", b.label || ""].filter(Boolean).join(" · ") + (b.newer ? " – " + t("newBox.newerShort") : "");
}

/** serverStart is the server's start from /healthz, "" when it does not answer; /healthz is always open. */
async function serverStart(): Promise<string> {
  try {
    const h = await apiRequest<{ started_at?: string }>("GET", "healthz", undefined, { auth: false });
    return (h && h.started_at) || "";
  } catch {
    return "";
  }
}

/** cameBack waits until /healthz reports another start than before, at most RESTART_WAIT_MS; false where it did not. */
async function cameBack(before: string): Promise<boolean> {
  const until = Date.now() + RESTART_WAIT_MS;
  for (;;) {
    await new Promise((done) => setTimeout(done, RESTART_POLL_MS));
    const now = await serverStart();
    if (now && now !== before) return true;
    if (Date.now() > until) return false;
  }
}

/** FromBackup is the step's box, drawn once with the card and shown in the backup step. */
export function FromBackup() {
  useLang();
  const shown = card.ownerStep === "backup";
  const opened = card.opened;
  const open = card.shown;
  const forgot = card.forgot;
  const focus = card.focus;
  const working = card.working;
  const [source, setSource] = useState<Source>("file");
  const [kinds, setKinds] = useState<ServerBackupKind[] | null>(null);
  const [kind, setKind] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});
  const [secret, setSecret] = useState("");
  const [found, setFound] = useState<NewBoxBackup[] | null>(null);
  const [skipped, setSkipped] = useState(0);
  const [picked, setPicked] = useState("");
  const [held, setHeld] = useState<NewBoxBackup | null>(null);
  const [plan, setPlan] = useState<ServerRestorePlan | null>(null);
  const [opts, setOpts] = useState<Options>(NO_OPTIONS);
  const [said, setSaid] = useState<Outcome | null>(null);
  const keyRef = useRef<HTMLTextAreaElement>(null);
  const sourceRef = useRef<HTMLSelectElement>(null);
  // the request under way, for its button's ring: one at a time, the card working meanwhile
  const [doing, setDoing] = useState("");
  const ask = useAsk<HTMLButtonElement>();
  /** forgetTarget empties the target's fields and its secret, which serve for one list and one fetch. */
  const forgetTarget = () => {
    setConfig({});
    setSecret("");
  };
  const reset = () => {
    setSource("file");
    setKind("");
    setConfig({});
    setSecret("");
    setFound(null);
    setSkipped(0);
    setPicked("");
    setHeld(null);
    setPlan(null);
    setOpts(NO_OPTIONS);
    setSaid(null);
    if (keyRef.current) keyRef.current.value = "";
  };
  // each opening empties the step, the key's field with it, as the card's own fields
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed.
  useLayoutEffect(() => {
    if (opened) reset();
  }, [opened]);
  // the card closed, by a sign-in, Skip for now or Escape: the key goes from its field, and the target's fields and
  // secret with it
  // biome-ignore lint/correctness/useExhaustiveDependencies: open is read from the card's store, which Biome takes for a stable import; the effect is for each closing, so it is listed.
  useLayoutEffect(() => {
    if (open) return;
    if (keyRef.current) keyRef.current.value = "";
    forgetTarget();
  }, [open]);
  // a sign-out empties the whole step (forgetCard)
  // biome-ignore lint/correctness/useExhaustiveDependencies: forgot is read from the card's store, which Biome takes for a stable import; the effect is for each sign-out, so it is listed.
  useLayoutEffect(() => {
    if (forgot) reset();
  }, [forgot]);
  // the source's choice in focus as the step is shown
  // biome-ignore lint/correctness/useExhaustiveDependencies: focus is read from the card's store, which Biome takes for a stable import; the effect is for each focus asked, so it is listed.
  useLayoutEffect(() => {
    if (!focus.n || focus.on !== "backup") return;
    sourceRef.current?.focus();
  }, [focus]);
  const kind0 = kinds?.find((k) => k.name === kind);
  const key = () => keyRef.current?.value.trim() || "";
  /** fails says a refusal: a 401 has turned the card already (api), a 409 reads who is in, which turns it where the server has users now. */
  const fails = (e: unknown, textKey: string) => {
    const err = e as Error & { status?: number };
    if (err.status === 401) return;
    setSaid({ text: t(textKey, { error: err.message }), ok: false });
    if (err.status === 409) void loadMe();
  };
  /** run runs a request of the step, what names its button, with the card working, so that nothing leaves the card meanwhile; none while another runs. */
  const run = async (what: string, fn: () => Promise<void>) => {
    if (card.working) return;
    card.working = true;
    setDoing(what);
    setSaid(null);
    try {
      await fn();
    } finally {
      card.working = false;
      setDoing("");
    }
  };
  const took = (b: NewBoxBackup) => {
    setHeld(b);
    setPlan(null);
    setOpts(NO_OPTIONS);
  };
  const upload = (file: File) =>
    run("upload", async () => {
      try {
        took((await (await apiRaw("POST", SETUP, file, "application/zip")).json()).backup as NewBoxBackup);
      } catch (e) {
        fails(e, "newBox.failed");
      }
    });
  const chooseSource = (v: string) => {
    setSource(v as Source);
    setSaid(null);
    if (v !== "target" || kinds) return;
    run("kinds", async () => {
      try {
        const all = await api<ServerBackupKind[]>("GET", "api/v1/server-backup-targets/kinds");
        const usable = (all || []).filter((k) => !k.makes_secret);
        setKinds(usable);
        if (usable.length) chooseKind(usable[0]);
      } catch (e) {
        fails(e, "newBox.failed");
      }
    });
  };
  const chooseKind = (k: ServerBackupKind | undefined) => {
    setKind(k?.name || "");
    const cfg: Record<string, string> = {};
    for (const f of k?.fields || []) if (f.key && f.key !== k?.secret && f.default) cfg[f.key] = f.default;
    setConfig(cfg);
    setSecret("");
    setFound(null);
    setPicked("");
  };
  const target = () => ({ kind, config, ...(kind0?.secret ? { secret } : {}) });
  const list = () =>
    run("list", async () => {
      setFound(null);
      try {
        const r = await api<{ backups: NewBoxBackup[]; skipped: number }>("POST", SETUP + "/list", target());
        setFound(r.backups || []);
        setSkipped(r.skipped || 0);
        setPicked(r.backups?.[0]?.name || "");
      } catch (e) {
        fails(e, "newBox.listFailed");
      }
    });
  const fetchPicked = () =>
    run("fetch", async () => {
      try {
        took((await api<{ backup: NewBoxBackup }>("POST", SETUP + "/fetch", { ...target(), name: picked })).backup);
        // fetched: the target's fields and secret served their purpose, and the server stores none of them
        forgetTarget();
        setFound(null);
        setPicked("");
      } catch (e) {
        fails(e, "newBox.failed");
      }
    });
  const planWith = (o: Options) =>
    run("plan", async () => {
      const k = key();
      if (!k) {
        setSaid({ text: t("newBox.noKey"), ok: false });
        keyRef.current?.focus();
        return;
      }
      setOpts(o);
      try {
        // the backup handed, by its uuid: another handed since is refused, so that the plan is of the one shown
        const body = { backup: held?.uuid || "", key_file: k, take_identity: o.identity, env: o.env, tls: o.tls };
        setPlan(await api<ServerRestorePlan>("POST", SETUP + "/plan", body));
      } catch (e) {
        setPlan(null);
        fails(e, "newBox.failed");
      }
    });
  const readKey = (file: File) => {
    file.text().then((text) => {
      if (keyRef.current) keyRef.current.value = text;
      setPlan(null);
    });
  };
  const back = () => {
    if (keyRef.current) keyRef.current.value = "";
    setPlan(null);
    setOpts(NO_OPTIONS);
    setSaid(null);
    backupBack();
  };
  // the restore: the key from its field, emptied once it is sent; the server's start before it, to follow the restart by
  const restore = async () => {
    const k = key();
    const before = await serverStart();
    card.working = true;
    restoring(true);
    let res: ServerRestoreResult | null = null;
    try {
      // the backup planned, by its uuid: one handed since is refused, not restored unplanned
      const body = { backup: plan?.backup?.uuid || held?.uuid || "", key_file: k, take_identity: opts.identity, env: opts.env, tls: opts.tls };
      res = await api<ServerRestoreResult>("POST", SETUP + "/restore", body);
    } catch (e) {
      const err = e as Error & { status?: number; data?: ServerRestoreResult };
      if (err.status === 500 && err.data && err.data.restart) {
        res = err.data; // restored, a file not put in place: the server restarts all the same
      } else {
        restoring(false);
        card.working = false;
        ask.done(false);
        fails(e, "newBox.failed");
        return;
      }
    }
    if (keyRef.current) keyRef.current.value = "";
    ask.done(false);
    const extra = res.error ? " " + t("newBox.fileFailed", { error: res.error }) : "";
    if (!res.restart?.started) {
      card.working = false;
      setSaid({ text: t("newBox.blocked", { why: res.restart?.blocked || "" }) + extra, ok: false });
      return;
    }
    setPlan(null);
    setSaid({ text: t("newBox.restarting") + extra, ok: true });
    if (!(await cameBack(before))) {
      card.working = false;
      setSaid({ text: t("newBox.notBack"), ok: false });
      return;
    }
    // who is in: the backup's users, since the server restores none without an enabled owner, so the read is refused
    // without a session and the card turns to the sign-in (api, askLogin, toSignIn)
    void loadMe();
  };
  const field = (f: NonNullable<ServerBackupKind["fields"]>[number]) => {
    const fk = f.key || "";
    const id = "new-box-field-" + fk;
    const label = textOr("serverbackup.field." + kind + "." + fk, fk);
    const set = (v: string) => {
      setConfig((c) => ({ ...c, [fk]: v }));
      setFound(null);
    };
    switch (f.type) {
      case "secret":
        return <TextField key={id} id={id} label={label} type="password" autocomplete="off" value={secret} set={(v) => {
          setSecret(v);
          setFound(null);
        }} />;
      case "number":
        return <NumberField key={id} id={id} label={label} min={0} max={999999} step={1} value={config[fk] || ""} set={set} />;
      case "bool":
        return <Switch key={id} id={id} label={label} checked={config[fk] === "true"} onChange={(on) => set(on ? "true" : "false")} />;
      case "choice":
        return <Select key={id} id={id} label={label} wide options={(f.choices || []).map((c) => [c, c] as [string, string])} value={config[fk] || ""} onChange={set} />;
      default:
        return <TextField key={id} id={id} label={label} value={config[fk] || ""} set={set} />;
    }
  };
  const refused = plan?.refused || "";
  const answers = !!plan && !plan.identity?.this_box && plan.identity?.answers === true;
  const cubes = (plan?.cubes || []).map((c) => (c.name ? c.name + " (" + c.serial + ")" : c.serial || "")).join(", ");
  return (
    <div id="new-box" class={"new-box" + hid(!shown)}>
      <p id="new-box-text" class="muted small">{t("newBox.text")}</p>
      <Select label={t("newBox.source")} id="new-box-source" wide inputRef={sourceRef} disabled={working} value={source} onChange={chooseSource}
        options={[["file", t("newBox.fromFile")], ["target", t("newBox.fromTarget")]]} />
      <div class={"new-box-file" + hid(source !== "file")}>
        <FileButton id="new-box-file" cls={doing === "upload" ? "busy" : undefined} label={t("newBox.chooseFile")} accept=".zip,application/zip" off={working} inputId="new-box-file-input"
          inputLabel={t("newBox.chooseFile")} onFile={upload} />
      </div>
      <div id="new-box-target" class={"new-box-target" + hid(source !== "target")}>
        {kinds && (
          <Select label={t("newBox.kind")} id="new-box-kind" wide disabled={working} value={kind} onChange={(v) => chooseKind(kinds.find((k) => k.name === v))}
            options={kinds.map((k) => [k.name || "", textOr("serverbackup.kind." + k.name, k.name || "")] as [string, string])} />
        )}
        {(kind0?.fields || []).map(field)}
        <div class="actions">
          <button type="button" id="new-box-list" {...busyProps("", doing === "list", working || !kind)} onClick={list}>{t("newBox.list")}</button>
        </div>
        {found && !found.length && <p id="new-box-none" class="muted small">{t("newBox.none")}</p>}
        {found && found.length > 0 && (
          <>
            <Select label={t("newBox.pick")} id="new-box-pick" wide disabled={working} value={picked} onChange={setPicked}
              options={found.map((b) => [b.name || "", optionText(b)] as [string, string])} />
            <div class="actions">
              <button type="button" id="new-box-fetch" {...busyProps("", doing === "fetch", working || !picked)} onClick={fetchPicked}>{t("newBox.fetch")}</button>
            </div>
          </>
        )}
        {found && skipped > 0 && <p id="new-box-skipped" class="muted small">{t("newBox.skipped", { n: String(skipped) })}</p>}
      </div>
      {held && (
        <div id="new-box-held" class="new-box-held">
          <p id="new-box-held-text">{heldText(held)}</p>
          {held.newer && <p id="new-box-newer" class="notice small" role="note">{t("newBox.newer")}</p>}
          <Field label={t("newBox.key")} id="new-box-key" wide hint={keysText(held) ? t("newBox.keyFor", { keys: keysText(held) }) : t("newBox.keyHint")}
            hintId="new-box-key-hint">
            <textarea ref={keyRef} id="new-box-key" class="field-control mono new-box-key" rows={3} autocomplete="off" autocapitalize="none"
              spellcheck={false} aria-describedby="new-box-key-hint" disabled={working} onInput={() => setPlan(null)} />
          </Field>
          <div class="actions">
            <FileButton id="new-box-key-file" label={t("newBox.keyFile")} accept=".txt,.key,text/plain" off={working} inputId="new-box-key-file-input"
              inputLabel={t("newBox.keyFile")} onFile={readKey} />
            <button type="button" id="new-box-open" {...busyProps("", doing === "plan", working || !!held.newer)} onClick={() => planWith(NO_OPTIONS)}>{t("newBox.open")}</button>
          </div>
        </div>
      )}
      {plan && (
        <div id="new-box-plan" class="new-box-plan">
          <p id="new-box-replaces">{t("newBox.replaces")}</p>
          {cubes && <p id="new-box-cubes" class="small">{t("newBox.cubes", { cubes })}</p>}
          {plan.migrated && <p class="muted small">{t("newBox.migrated")}</p>}
          <Switch id="new-box-identity" label={t("newBox.identity")} checked={opts.identity} disabled={working} onChange={(on) => planWith({ ...opts, identity: on })} />
          <p class="muted small new-box-note">{t("newBox.identityHint")}</p>
          {answers && <p id="new-box-answers" class="notice small" role="note">{t("newBox.identityAnswers")}</p>}
          {plan.env && <Switch id="new-box-env" label={t("newBox.env")} checked={opts.env} disabled={working} onChange={(on) => planWith({ ...opts, env: on })} />}
          {(plan.tls || 0) > 0 && <Switch id="new-box-tls" label={t("newBox.tls")} checked={opts.tls} disabled={working} onChange={(on) => planWith({ ...opts, tls: on })} />}
          {refused && <p id="new-box-refused" class="notice small" role="alert">{refused}</p>}
        </div>
      )}
      <ResultLine id="new-box-msg" cls="small" role="status" res={said} />
      {ask.asking ? (
        <Question text={t("newBox.confirm")} yes={t("newBox.start")} no={t("action.cancel")} onYes={restore} onNo={() => ask.done()} boxRef={ask.box} />
      ) : (
        <div class="actions">
          <span class="actions-right">
            <button type="button" id="new-box-back" class="form-cancel" disabled={working} onClick={back}>{t("newBox.back")}</button>
            <button ref={ask.ref} type="button" id="new-box-start" {...busyProps("primary", false, working || !plan || !!refused)} onClick={ask.ask}>
              {t("newBox.start")}
            </button>
          </span>
        </div>
      )}
    </div>
  );
}
