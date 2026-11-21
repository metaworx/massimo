// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/prefs/Settings.tsx
 *
 * My settings' rows as a part, drawn once and updated in place: Poll
 * every, Expert mode, the Cube clock and the language, each saved at
 * once as prefs.js saved it (app.js setSetting), each with its switch
 * that keeps the setting on this device alone, in a column of its own
 * (DevRow). The settings applied again, as when the user's arrive from
 * the server at a sign-in, draw the rows again in place: a poll being
 * typed keeps what is typed (a draft, without the "changed meanwhile"
 * notice, which the plain field had not: the field is saved as it is
 * left, and leaving it for Take theirs would save what is typed).
 */

import { NumberField, numberNorm, Select, Switch, useDraft } from "../shared/field";
import { DevRow } from "./rows";
import { load } from "./store";

/** Language is the language's choice: from the browser, then every language with a translation table, by its own name, and the note of a machine translation. */
function Language() {
  load.useLoad(); // the language chosen is the plain code's, drawn again by the counter
  const options: [string, string][] = [["auto", t("lang.auto") + " (" + LANGUAGE_NAMES[browserLanguage()] + ")"]];
  for (const l of languages()) options.push([l, LANGUAGE_NAMES[l] || l]);
  const texts = machineNote();
  return (
    <Select label={t("settings.language")} id="lang-select" wide help="language" options={options} value={i18n.setting}
      after={<p id="lang-note" class={"muted small lang-note" + (texts ? "" : " hidden")}>{(texts || []).map((s) => <span key={s}>{s}</span>)}</p>}
      onChange={(v) => {
        setSetting("lang", validSetting("lang", v) || "auto");
        changeLanguage(state.settings.lang).then(applyLanguage);
      }} />
  );
}

/** Settings are My settings' head and rows. */
export function Settings() {
  load.useLoad();
  const s = state.settings;
  const poll = useDraft(String(s.poll), numberNorm);
  const commitPoll = (typed: string) => {
    const v = validSetting("poll", typed) || 10;
    poll.set(String(v));
    setSetting("poll", v);
    startPolling();
    load.bump();
  };
  return (
    <>
      <div class="dev-head needs-sync">
        <span />
        <span class="dev-cell dev-colhead" title={t("settings.thisDeviceTitle")}>{t("settings.thisDevice")}</span>
      </div>
      <DevRow setting="poll">
        <NumberField label={t("settings.poll")} id="poll-input" help="poll" unit={t("settings.seconds")} min={2} max={3600} step={1}
          value={poll.value} set={poll.set} onChange={commitPoll} />
      </DevRow>
      <DevRow setting="expert">
        <Switch id="expert-toggle" label={t("settings.expert")} help="expert" checked={s.expert} onChange={(on) => {
          setSetting("expert", on);
          applySettings();
        }} />
      </DevRow>
      <DevRow setting="clock">
        <Select label={t("settings.clock")} id="clock-select" wide help="clock" options={[["browser", t("settings.clockBrowser")], ["cube", t("settings.clockCube")]]}
          value={s.clock} onChange={(v) => {
            setSetting("clock", validSetting("clock", v) || "browser");
            applySettings();
          }} />
      </DevRow>
      <DevRow setting="lang">
        <Language />
      </DevRow>
    </>
  );
}

