// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, translations: the lookup t() of every text of the
// page by key, number formatting for the chosen language, and the
// language choice. The texts are in ui/i18n/<lang>.json, one flat table
// per language, English the base; English and the chosen language are
// fetched once, when chosen. The languages are those of the original
// MAX! software: English, German, French, Italian, Dutch and Polish. A
// key missing in a language falls back to English. Loaded before app.js.
"use strict";

// I18N holds the tables fetched so far, by language.
const I18N = {};

// A language whose table has "i18n.machineNote" was machine-translated
// and not yet reviewed by a native speaker; the settings say so, in that
// language and in English. The key is deliberately absent from en and de.
const MACHINE_NOTE_EN = "Machine-translated: Massimo's texts in this language have not yet been reviewed by a native speaker.";

function machineNote() {
  const note = (I18N[i18n.lang] || {})["i18n.machineNote"];
  return note ? [note, MACHINE_NOTE_EN] : null;
}

// LANGUAGE_NAMES lists the languages the settings offer, by their own
// name; each has its file ui/i18n/<code>.json.
const LANGUAGE_NAMES = { en: "English", de: "Deutsch", fr: "Français", it: "Italiano", nl: "Nederlands", pl: "Polski" };

const i18n = { lang: "en", setting: "auto" };

function languages() {
  return Object.keys(LANGUAGE_NAMES);
}

const i18nLoads = {}; // by language: the fetch of its table, while it runs or once it is in
let i18nSeq = 0; // counts the language changes; only the latest is applied

// loadLanguage fetches the table of a language into I18N, once, and
// returns a promise of it; a failed fetch is tried again next time.
function loadLanguage(lang) {
  if (!i18nLoads[lang]) {
    const path = "ui/i18n/" + lang + ".json";
    i18nLoads[lang] = fetch(path)
      .then((resp) => {
        if (!resp.ok) throw new Error(path + ": " + resp.status + " " + resp.statusText);
        return resp.json();
      })
      .then((table) => {
        I18N[lang] = table;
        return table;
      })
      .catch((e) => {
        delete i18nLoads[lang];
        throw e;
      });
  }
  return i18nLoads[lang];
}

// browserLanguage picks the first of the browser's preferred languages
// the UI offers, English otherwise.
function browserLanguage() {
  const have = languages();
  const prefs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || "en"];
  for (const p of prefs) {
    const base = String(p).toLowerCase().split("-")[0];
    if (have.includes(base)) return base;
  }
  return "en";
}

// setLanguage applies a setting, "auto" or a language code, once English
// and the language are loaded, and returns the promise of that. When the
// language cannot be loaded the page stays in English and the promise
// is rejected with the error; a change made meanwhile wins over it, and
// the promise of a load it superseded resolves without one, so that its
// error does not show on a page in the newer language.
function setLanguage(setting) {
  const seq = ++i18nSeq;
  i18n.setting = setting === "auto" || languages().includes(setting) ? setting : "auto";
  const lang = i18n.setting === "auto" ? browserLanguage() : i18n.setting;
  const apply = (l) => {
    if (seq !== i18nSeq) return;
    i18n.lang = l;
    document.documentElement.lang = l;
  };
  const loads = [loadLanguage("en")];
  if (lang !== "en") loads.push(loadLanguage(lang));
  return Promise.all(loads).then(
    () => apply(lang),
    (e) => {
      if (seq !== i18nSeq) return;
      apply("en");
      throw e;
    },
  );
}

// t returns the text of a key in the current language, English when the
// language has none, the key itself when English has none either, with
// {name} placeholders filled from params.
function t(key, params) {
  const table = I18N[i18n.lang] || {};
  let s = table[key];
  if (s === undefined) s = (I18N.en || {})[key];
  if (s === undefined) s = key;
  if (params) s = s.replace(/\{(\w+)\}/g, (m, k) => (params[k] === undefined ? m : String(params[k])));
  return s;
}

// tn picks the plural form of a key (key.one, key.other, …) for n by the
// language's plural rules, and fills {n}.
function tn(key, n, params) {
  let form = "other";
  try {
    form = new Intl.PluralRules(i18n.lang).select(n);
  } catch (e) {
    // no plural rules: "other"
  }
  const full = key + "." + form;
  const exists = (I18N[i18n.lang] || {})[full] !== undefined || (I18N.en || {})[full] !== undefined;
  return t(exists ? full : key + ".other", Object.assign({ n }, params));
}

// fmtNum formats a number with the given decimals in the language's
// style (20.5 in English, 20,5 in German).
function fmtNum(v, digits) {
  return new Intl.NumberFormat(i18n.lang, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(v);
}

// applyI18n fills the static texts of a document or template: the
// text of elements with data-i18n, and the attributes named in
// data-i18n-attr ("title:key aria-label:key2").
function applyI18n(root) {
  for (const el of root.querySelectorAll("[data-i18n]")) el.textContent = t(el.dataset.i18n);
  for (const el of root.querySelectorAll("[data-i18n-attr]")) {
    for (const pair of el.dataset.i18nAttr.split(/\s+/)) {
      const i = pair.indexOf(":");
      if (i > 0) el.setAttribute(pair.slice(0, i), t(pair.slice(i + 1)));
    }
  }
  if (root.querySelectorAll) {
    for (const tpl of root.querySelectorAll("template")) applyI18n(tpl.content);
  }
}
