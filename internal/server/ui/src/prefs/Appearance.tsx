// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/Appearance.tsx
 *
 * My settings' Appearance as a part, drawn once and updated in place:
 * the mode, the colour scheme, the blinking signs and the window's and
 * the radio's drawings, each saved at once as appearance.js saved it
 * (theme.js chooseMode and its like), each with its switch that keeps it
 * on this device alone (DevRow); Customise…, Export and Import…, the
 * line under them, and the scheme editor while it is open. Beside the
 * window's and the radio's choice their drawing as the page shows it,
 * drawn again by the icons' signal when the choice changes.
 */

import { ResultLine } from "../shared/badge";
import { Select } from "../shared/field";
import { FileButton } from "../shared/file";
import { Icon, Sign } from "../shared/icon";
import { useIcons } from "../shared/look";
import { Editor } from "./Editor";
import { DevRow } from "./rows";
import { closeEditor, editor, exportScheme, importScheme, openEditor, said, schemeMessage } from "./scheme";
import { load } from "./store";

/** schemeOptions are the scheme's choices: the built-in schemes by their translated names, then the user's own under a group. */
function schemeOptions() {
  const options: ([string, string] | { group: string; options: [string, string][] })[] = Object.keys(BUILTIN_SCHEMES).map((id) => [id, t("appearance.scheme." + id)]);
  if (themeState.custom.length) options.push({ group: t("appearance.customGroup"), options: themeState.custom.map((c) => [CUSTOM_PREFIX + c.name, c.name]) });
  return options;
}

/**
 * Preview is a drawing beside its choice as the page shows it: the
 * window closed, open, and with a warning's and a failure's marks; the
 * radio with them.
 */
function Preview({ id, kind }: { id: string; kind: "window" | "radio" }) {
  useIcons();
  const type = kind === "window" ? iconChoice("window").type : undefined;
  return (
    <span id={id} class="icon-preview" aria-hidden="true">
      {type && <Icon symbol={type} cls="ni" />}
      {kind === "window" && <Sign kind="window" state="" />}
      <Sign kind={kind} state="warn" />
      <Sign kind={kind} state="bad" />
    </span>
  );
}

/** choose makes a choice theme.js keeps (chooseBlink, chooseIcon), which applies it without drawing the card, and draws the card again. */
function choose(fn: () => void): void {
  fn();
  load.bump();
}

/** Appearance is the card's Appearance: its rows, its buttons and their line, and the editor while it is open. */
export function Appearance() {
  load.useLoad();
  const s = state.settings;
  const open = editor.open;
  return (
    <>
      <div class="dev-group">
        <span>{t("appearance.title")}</span>
        <span class="dev-cell needs-sync" />
      </div>
      <DevRow setting="theme">
        <Select label={t("appearance.mode")} id="theme-select" help="theme" value={s.theme} onChange={chooseMode}
          options={[["system", t("appearance.modeSystem")], ["light", t("appearance.modeLight")], ["dark", t("appearance.modeDark")]]} />
      </DevRow>
      <DevRow setting="scheme">
        <Select label={t("appearance.scheme")} id="scheme-select" wide help="scheme" value={s.scheme} options={schemeOptions()} onChange={(v) => {
          closeEditor();
          schemeMessage("");
          chooseScheme(v);
        }} />
      </DevRow>
      <DevRow setting="blink">
        <Select label={t("appearance.blink")} id="blink-select" help="blink" value={s.blink} onChange={(v) => choose(() => chooseBlink(v))}
          options={[["system", t("appearance.blinkSystem")], ["on", t("appearance.blinkOn")], ["first", t("appearance.blinkFirst")], ["off", t("appearance.blinkOff")]]} />
      </DevRow>
      <DevRow setting="iconWindow">
        <Select label={t("appearance.iconWindow")} id="icon-window-select" help="iconWindow" value={s.iconWindow} onChange={(v) => choose(() => chooseIcon("iconWindow", v))}
          options={[["sash", t("appearance.iconWindowSash")], ["paned", t("appearance.iconWindowPaned")]]} before={<Preview id="icon-window-preview" kind="window" />} />
      </DevRow>
      <DevRow setting="iconRadio">
        <Select label={t("appearance.iconRadio")} id="icon-radio-select" help="iconRadio" value={s.iconRadio} onChange={(v) => choose(() => chooseIcon("iconRadio", v))}
          options={[["wifi", t("appearance.iconRadioWifi")], ["tower", t("appearance.iconRadioTower")]]} before={<Preview id="icon-radio-preview" kind="radio" />} />
      </DevRow>
      <DevRow last>
        <div class="scheme-actions">
          <button type="button" id="scheme-customise" aria-expanded={open ? "true" : "false"} aria-controls="scheme-editor"
            onClick={() => (open ? closeEditor() : openEditor())}>{t("appearance.customise")}</button>
          <button type="button" id="scheme-export" onClick={exportScheme}>{t("appearance.export")}</button>
          <FileButton id="scheme-import" inputId="scheme-file" inputLabel={t("appearance.importFile")} label={t("appearance.import")} accept="application/json,.json"
            onFile={importScheme} />
        </div>
      </DevRow>
      <ResultLine id="scheme-msg" cls="small scheme-msg" role="status" res={said.scheme} />
      <div id="scheme-editor" class={"panel scheme-editor" + (open ? "" : " hidden")} role="region" aria-labelledby="scheme-editor-title">
        {open && <Editor />}
      </div>
    </>
  );
}
