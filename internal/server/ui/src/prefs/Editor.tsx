// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/Editor.tsx
 *
 * The scheme editor as a part, drawn while it is open and updated in
 * place: the light and the dark set, a row per token with its contrast
 * badge (or an empty place of the same width), its colour input and its
 * hex field, the line that says why a hex field was refused, the note on
 * contrast, the name with Save as, then Reset, Delete and Close. It is a
 * settings form (shared/forms.ts) whose changes are the draft's colours
 * and the name typed against the scheme it started from: Save as shows
 * its state, off while the editor holds no change, and Enter in the name
 * saves only where Save as would (UiParts D204); left unsaved My
 * settings' pill shows the disk, which opens it. A hex field takes a typed or pasted value: a colour applies
 * as it is typed and is written as #rrggbb once the field is left;
 * anything else is refused, the field marked invalid, and the colour
 * stays; Escape puts it back.
 */

import { useLayoutEffect, useRef } from "preact/hooks";
import { ResultLine } from "../shared/badge";
import { SaveButton, useDraft } from "../shared/field";
import { focusForm, useForm } from "../shared/forms";
import { changed, closeEditor, coloursChanged, deleteDraft, editor, hexUI, resetColours, SCHEME_FORM, saveDraft, takeColour } from "./scheme";
import { load } from "./store";
import { showCard } from "../shared/card";

/** tokenLabel is a token's name for people. */
function tokenLabel(k: string): string {
  return t("appearance.token." + k);
}

/** Contrast is a token's badge: the ratio of its pair, graded, the pair as title and label; an empty place for a token without a pair. */
function Contrast({ set, k }: { set: SchemeSet; k: string }) {
  const mark = MARK_PAIRS[k];
  if (!mark && !CONTRAST_PAIRS[k]) return <span class="contrast none" aria-hidden="true" />;
  const [fg, bg] = mark ? [k, mark] : [CONTRAST_PAIRS[k], k];
  const ratio = contrastRatio(set[fg], set[bg]);
  const grade = mark ? (ratio >= 3 ? "ok" : "bad") : contrastClass(ratio);
  const label = t("appearance.contrast", { text: tokenLabel(fg), bg: tokenLabel(bg), ratio: fmtNum(ratio, 1) });
  return <span class={"contrast " + grade} role="img" title={label} aria-label={label}>{fmtNum(ratio, 1)}</span>;
}

/** Row is a token's row in a set: its name, its badge, its colour input and its hex field. */
function Row({ mode, k, onTouch }: { mode: "light" | "dark"; k: string; onTouch: () => void }) {
  void editor.ver; // the draft's colours, which are the plain code's
  const d = themeState.draft;
  const id = "scheme-" + mode + "-" + k;
  const ui = hexUI.of(id);
  if (!d) return null;
  const colour = d[mode][k];
  const refuse = (text: string | null) => {
    ui.refused = text;
    if (text !== null) editor.refusedLast = id;
  };
  return (
    <div class="scheme-row">
      <label for={id}>{tokenLabel(k)}</label>
      <Contrast set={d[mode]} k={k} />
      <input type="color" id={id} title={"--" + k} value={colour} onInput={(ev) => {
        const c = hexColour(ev.currentTarget.value);
        if (!c) return;
        ui.typed = null;
        refuse(null);
        takeColour(mode, k, c);
        onTouch();
      }} />
      <input type="text" class="scheme-hex mono" id={id + "-hex"} size={7} spellcheck={false} autocomplete="off" autocapitalize="off"
        aria-label={t("appearance.hex", { token: tokenLabel(k) })} data-token={tokenLabel(k)} value={ui.typed ?? colour}
        aria-invalid={ui.refused !== null ? "true" : undefined} aria-describedby={ui.refused !== null ? "scheme-hex-msg" : undefined}
        data-refused={ui.refused ?? undefined}
        onInput={(ev) => {
          const text = ev.currentTarget.value;
          ui.typed = text;
          const c = typedColour(text);
          if (!c) return; // perhaps not typed to the end yet; judged on change
          if (ui.refused !== null) refuse(null);
          takeColour(mode, k, c);
          onTouch();
        }}
        onChange={(ev) => {
          const c = typedColour(ev.currentTarget.value);
          if (c) {
            ui.typed = null;
            refuse(null);
            if (c !== themeState.draft?.[mode][k]) takeColour(mode, k, c);
          } else {
            refuse(ev.currentTarget.value.trim());
          }
          changed();
        }}
        onKeyDown={(ev) => {
          if (ev.key === "Escape" && ui.refused !== null) {
            ev.preventDefault();
            ui.typed = null;
            refuse(null);
            changed();
          }
        }} />
    </div>
  );
}

/** HexMessage says under the sets why a hex field was refused: the one refused last, or else one still marked; nothing where none is. */
function HexMessage() {
  void editor.ver;
  const sets = ["light", "dark"].flatMap((mode) => THEME_TOKENS.map((k) => "scheme-" + mode + "-" + k));
  const last = editor.refusedLast;
  const id = last && hexUI.of(last).refused !== null ? last : sets.find((x) => hexUI.of(x).refused !== null);
  const text = id ? t("appearance.hexBad", { token: tokenLabel(id.replace(/^scheme-(light|dark)-/, "")), value: hexUI.of(id).refused || "" }) : "";
  return <ResultLine id="scheme-hex-msg" cls="small scheme-msg" role="status" res={text ? { text, ok: false } : null} />;
}

/**
 * Editor is the editor of the draft, the first colour taking the focus
 * as it opens: a form kept by its key, its name a draft of the draft's
 * name, Save as by the form's state, Reset back to the scheme it started
 * from, Delete for the user's own, Close dropping the draft.
 */
export function Editor() {
  load.useLoad();
  void editor.ver; // the draft's changes
  const d = themeState.draft;
  const own = schemeById(themeState.base);
  const form = useForm({
    key: SCHEME_FORM,
    name: { key: "appearance.editorTitle" },
    open: () => {
      if (!isShown("me")) showView("me");
      showCard("prefs"); // a folded card unfolded, the editor in sight
      focusForm(SCHEME_FORM);
    },
  });
  const name = useDraft(d ? d.name : "", (v) => v.trim(), form.place("name"));
  const colours = { take: resetColours };
  form.track(name.changed || coloursChanged(), [name, colours]);
  const first = useRef<HTMLFieldSetElement>(null);
  // as it opens, the focus on its first colour, in the drawing's own turn as the plain editor gave it
  useLayoutEffect(() => {
    first.current?.querySelector("input")?.focus();
  }, []);
  if (!d) return null;
  const touch = () => form.place("colours").changed();
  // as Save as: nothing while it is off, the editor holding no change, or while it saves (D204)
  const off = form.look === "clean" || form.look === "saved" || form.look === "saving";
  const save = () => form.save(async () => {
    saveDraft(name.value);
    return true;
  });
  return (
    <form {...form.props} class="scheme-form" onSubmit={(ev) => ev.preventDefault()}>
      <h3 id="scheme-editor-title">{t("appearance.editorTitle")}</h3>
      <div class="scheme-sets">
        {(["light", "dark"] as const).map((mode) => (
          <fieldset key={mode} ref={mode === "light" ? first : undefined} class="scheme-set" data-mode={mode}>
            <legend>{t(mode === "light" ? "appearance.light" : "appearance.dark")}</legend>
            {THEME_TOKENS.map((k) => <Row key={k} mode={mode} k={k} onTouch={touch} />)}
          </fieldset>
        ))}
      </div>
      <HexMessage />
      <p class="muted small">{t("appearance.contrastNote")}</p>
      <div class="field wide scheme-name">
        <label for="scheme-name">{t("appearance.name")}</label>
        <span class="inline field-control">
          <input type="text" id="scheme-name" maxLength={SCHEME_NAME_MAX} value={name.value} onInput={(ev) => name.set(ev.currentTarget.value)}
            onKeyDown={(ev) => {
              if (ev.key === "Enter") {
                ev.preventDefault();
                if (!off) save();
              }
            }} />
          <SaveButton label={t("appearance.saveAs")} look={form.look} type="button" onClick={save} />
        </span>
      </div>
      <div class="actions">
        <span class="actions-left">
          <button type="button" class="scheme-reset form-reset" onClick={form.reset}>{t("appearance.reset")}</button>
          <button type="button" id="scheme-delete" disabled={!own || own.builtin} onClick={deleteDraft}>{t("action.delete")}</button>
        </span>
        <span class="actions-right"><button type="button" class="scheme-close" onClick={closeEditor}>{t("action.close")}</button></span>
      </div>
    </form>
  );
}
