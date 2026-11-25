// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/sysinfo/Credits.tsx
 *
 * System info's fold Credits and licences: everything the server
 * includes (internal/credits, /ui/credits.json), fetched as the fold is
 * first opened. An entry is a fold of its own, its head the name, the
 * version and the licence, its body where it comes from, what else it
 * includes, and the texts of its licence's files, each with its licence
 * in a box that scrolls; a fetch that failed says why, with a button
 * that tries again. The texts are text; every sentence around the
 * list's names and identifiers is the page's.
 */

import type { ComponentChildren } from "preact";
import { useLayoutEffect, useRef } from "preact/hooks";
import { Fold } from "../shared/fold";
import { tParts } from "../shared/lang";
import { busyProps } from "../shared/question";
import { type CreditEntry, type CreditFile, creditFolds, credits, folds, loadCredits } from "./store";

/** origin says where an entry comes from, in the page's language. */
function origin(e: CreditEntry): string {
  if (e.kind === "module") return e.path ? t("credits.kind.moduleCopy", { path: e.path }) : t("credits.kind.module");
  if (e.kind === "vendored") return t("credits.kind.vendored", { path: e.path || "" });
  return t("credits.kind." + e.kind);
}

/**
 * licenceOf says an entry's licence: its SPDX identifier, which does not
 * break inside itself, or that its text says it; with others among its
 * files where they are not all under it, words that wrap.
 */
function licenceOf(e: CreditEntry): ComponentChildren {
  const id = e.licence ? <span class="credit-id">{e.licence}</span> : t("credits.seeText");
  return e.mixed ? tParts("credits.mixed", {}, { licence: id }) : id;
}

/** fileLicence says what a licence file is: its licence's identifier, a notice, or that its text says it. */
function fileLicence(f: CreditFile): string {
  if (f.notice) return t("credits.notice");
  return f.licence || t("credits.seeText");
}

/**
 * Entry is an entry's fold: its head the name, version and licence,
 * wrapping as one text beside the chevron; its body the origin, what it
 * includes, whose licence texts are among its files, and its licence's
 * files, each named with its licence above a box that scrolls, which
 * the name labels. n is its place in the list, for the boxes' labels.
 */
function Entry({ e, n }: { e: CreditEntry; n: number }) {
  const f = creditFolds.of(e.name);
  return (
    <Fold cls="sys-sub credit" open={f.open} data={{ name: e.name }} onToggle={(open) => {
      f.open = open;
    }} summary={<span class="credit-head"><span class="credit-name">{e.name}</span>{e.version && <>{" "}<span class="mono muted">{e.version}</span></>}{" "}<span class="muted credit-licence">{licenceOf(e)}</span></span>}>
      <p class="muted small credit-origin">{origin(e)}</p>
      {e.includes && <p class="muted small credit-includes">{t("credits.includes", { what: e.includes })}</p>}
      {f.open && e.files.map((file, i) => (
        <div key={file.name} class="credit-file">
          <div id={"credit-file-" + n + "-" + i} class="small credit-file-name"><span class="mono">{file.name}</span>{" · "}<span class="muted">{fileLicence(file)}</span></div>
          <pre class="credit-text" tabIndex={0} aria-labelledby={"credit-file-" + n + "-" + i}>{file.text}</pre>
        </div>
      ))}
    </Fold>
  );
}

/** retried is set by Try again, so that the try's end gives the focus to the list's first entry, or back to the button. */
let retried = false;

/** CreditsBody is the fold's body: the list, or the fetch under way, or why it failed and Try again. */
function CreditsBody() {
  const list = credits.list;
  const loading = credits.loading;
  const box = useRef<HTMLDivElement>(null);
  const retry = useRef<HTMLButtonElement>(null);
  // a try ended: the list that replaces Try again gives the focus to its first entry; a try that failed again gives it back to Try again, which lost it while it was off
  // biome-ignore lint/correctness/useExhaustiveDependencies: list and loading are read from the module's live credits, which the rule takes for constant; the effect is to run as the list comes and as a fetch ends.
  useLayoutEffect(() => {
    if (loading || !retried) return;
    retried = false;
    if (list) box.current?.querySelector("summary")?.focus();
    else retry.current?.focus();
  }, [list, loading]);
  if (list) {
    return (
      <div id="credits-info-body" ref={box}>
        <p class="muted small">{t("credits.intro")}</p>
        {list.entries.map((e, n) => <Entry key={e.name} e={e} n={n} />)}
      </div>
    );
  }
  return (
    <div id="credits-info-body">
      {credits.error ? (
        <>
          <p class="small cubes-msg bad">{t("credits.failed", { error: credits.error })}</p>
          <button ref={retry} type="button" id="credits-retry" {...busyProps("", loading)} onClick={() => {
            retried = true;
            loadCredits();
          }}>{t("credits.retry")}</button>
        </>
      ) : <p class="muted small">{t("credits.loading")}</p>}
    </div>
  );
}

/** CreditsFold is System info's fold Credits and licences, the number of entries in its head once they are read. */
export function CreditsFold() {
  const list = credits.list;
  return (
    <Fold id="credits-info" cls="sys-sub" open={folds.credits} onToggle={(open) => {
      folds.credits = open;
      if (open) loadCredits();
    }} summary={<><span>{t("credits.title")}</span><span id="credits-info-sum" class="muted">{list ? tn("credits.sum", list.entries.length) : ""}</span></>}>
      <CreditsBody />
    </Fold>
  );
}
