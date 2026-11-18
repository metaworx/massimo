// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/problems.tsx
 *
 * The problem list as a part, wherever it is put (problemsEl of
 * components.js): one icon per kind of problem among its entries, each in
 * the colour of the worst tone of its kind, as the summary of a menu that
 * lists the entries in one format at every level, with the entries under
 * an entry (subs) indented under it. Its details is made once and kept
 * while there is something to show, so that an open list stays open, and
 * follows, when data comes in.
 */

import { Fragment, type ComponentChildren, type TargetedMouseEvent } from "preact";
import { Age, Badge, Since } from "./badge";
import { Icon, Sign } from "./icon";
import { Dropdown } from "./menu";

/**
 * Item is a problem or a state of an entry, one badge in the list: kind groups the summary's
 * icons (sign, a kind of SIGNS, draws it with its state's mark); one
 * without a kind is listed and adds no icon; tone is a badge's kind,
 * which colours the badge and, without "fill-", the icon, or mark where
 * the item says how its icon is coloured; title is the badge's tooltip,
 * at when the value came; since, an API time, from when a state holds,
 * said after the text ("since 16.11., 00:41"); action, where given, makes
 * the badge a button that runs it (an unsaved form's item opens the form).
 */
export interface Item {
  kind?: string;
  icon?: string;
  sign?: string;
  tone?: string;
  mark?: string;
  text: string;
  title?: string;
  at?: string;
  stale?: boolean;
  since?: string;
  action?: () => void;
}

/**
 * Entry is who has problems: its icon, or sign where its own state shows,
 * or node, a part of its own with the class problem-icon; its name, tail
 * after it on its line (a sign of its state where its icon says what it
 * is), a grey detail (a serial, or parts that are not monospaced), its
 * items, and subs, the entries under it (a group's members); info lists
 * the entry and those under it without an item, as information, and
 * summary is what the summary shows after the kinds' icons.
 */
export interface Entry {
  icon?: string;
  sign?: { kind: string; state: string };
  node?: ComponentChildren;
  title: string;
  tail?: ComponentChildren;
  sub?: string | ComponentChildren[];
  items?: Item[];
  subs?: Entry[];
  info?: boolean;
  summary?: ComponentChildren;
}

/** entryItems are an entry's items and those of the entries under it. */
export function entryItems(e: Entry): Item[] {
  return (e.items || []).concat(...(e.subs || []).map(entryItems));
}

/** A kind of problem in the summary: its icon or sign, in its worst tone. */
interface Kind {
  kind: string;
  icon?: string;
  sign?: string;
  tone: string;
}

const RANK: Record<string, number> = { bad: 3, warn: 2 };

/**
 * problemKinds are the kinds of problems among entries' items, one per
 * kind in the order they come, each in its worst tone; skip leaves kinds
 * out (kindIcons of components.js).
 */
export function problemKinds(entries: Entry[], skip: string[] = []): Kind[] {
  const kinds = new Map<string, Kind>();
  const rank = (x: string) => RANK[x] || (x ? 1 : 0);
  for (const it of entries.flatMap(entryItems)) {
    if (!it.kind || skip.includes(it.kind)) continue;
    const k = kinds.get(it.kind) || { kind: it.kind, icon: it.icon, sign: it.sign, tone: "" };
    const tone = it.mark || (it.tone || "").replace(/^fill-/, "");
    if (rank(tone) > rank(k.tone)) k.tone = tone;
    kinds.set(it.kind, k);
  }
  return [...kinds.values()];
}

/**
 * kindsLabel names the kinds of problems among items, in the order kinds
 * gives, "Problems: radio, battery" (problem.label, each kind's text
 * problem.<kind>), for a screen reader and a list's tooltip; "" where
 * there is none of them.
 */
export function kindsLabel(items: Item[], kinds: string[]): string {
  const found = kinds.filter((k) => items.some((it) => it.kind === k));
  return found.length ? t("problem.label", { list: found.map((k) => t("problem." + k)).join(", ") }) : "";
}

/** whoOf names the entries with an item, for the blink of the summary's marks. */
function whoOf(entries: Entry[]): string {
  return entries.filter((e) => entryItems(e).length).map((e) => e.title + " " + (typeof e.sub === "string" ? e.sub : "")).join("|");
}

/** KindIcon is a kind's icon in the summary, a sign where the kind has one; who names the entries for its mark's blink. */
function KindIcon({ k, who }: { k: Kind; who: string }) {
  if (k.sign) return <Sign kind={k.sign} state={k.tone} blink={[who, k.sign, k.tone].join(":")} />;
  return <Icon symbol={k.icon || ""} cls={"ni" + (k.tone ? " tone-" + k.tone : "")} />;
}

/**
 * KindIcons are the icons of the kinds of problems among entries, one per
 * kind in its worst tone, without the list (kindIcons of components.js):
 * the summary of Problems, and where a place shows the icons alone (the
 * Cube choice's items); skip leaves kinds out.
 */
export function KindIcons({ entries, skip }: { entries: Entry[]; skip?: string[] }) {
  const who = whoOf(entries);
  return <>{problemKinds(entries, skip).map((k) => <KindIcon key={k.kind} k={k} who={who} />)}</>;
}

/** entryKey names an entry among its siblings. */
function entryKey(e: Entry): string {
  return e.title + "|" + (typeof e.sub === "string" ? e.sub : "");
}

/**
 * ProblemEntry is one entry of the list, the same at every level: its
 * icon in a column of its own, its name and grey detail, under the name
 * its badges, each led by its icon (a warning's or a failure's by its
 * mark), with its age, a badge with an action a button that runs it;
 * then the entries under it. Nothing for an entry without an item, its
 * own or under it, but for information.
 */
function ProblemEntry({ e, info }: { e: Entry; info?: boolean }) {
  const shown = info || e.info;
  if (!entryItems(e).length && !shown) return null;
  const sub = typeof e.sub === "string" ? e.sub : "";
  let icon: ComponentChildren;
  if (e.sign) icon = <Sign kind={e.sign.kind} state={e.sign.state} cls="problem-icon" blink={e.title + ":" + e.sign.state} />;
  else if (e.node) icon = e.node;
  else if (e.icon) icon = <Icon symbol={e.icon} cls="ni problem-icon" />;
  else icon = <span class="problem-icon" />;
  const subs = (e.subs || []).filter((x) => entryItems(x).length || shown);
  return (
    <div class="problem-entry">
      {icon}
      <div class="problem-who">
        <span class="problem-name">{e.title}</span>
        {e.tail && <span class="problem-tail">{e.tail}</span>}
        {Array.isArray(e.sub) ? <span class="problem-sub">{e.sub}</span> : sub && <span class="mono problem-sub">{sub}</span>}
      </div>
      {(e.items || []).length > 0 && (
        <div class="problem-flags">
          {(e.items || []).map((it) => {
            const key = (it.kind || "") + ":" + it.text;
            const badge = (
              <Badge key={key} kind={it.tone || "mode"} title={it.title} blink={[e.title, sub, it.kind, it.text].join(":")}>
                {it.icon && !badgeState(it.tone || "") && <Icon symbol={it.icon} cls="ni" />}
                {it.text}
                <Age at={it.at} stale={it.stale} />
                <Since at={it.since} />
              </Badge>
            );
            if (!it.action) return badge;
            return <button key={key} type="button" class="problem-action" onClick={it.action}>{badge}</button>;
          })}
        </div>
      )}
      {subs.length > 0 && (
        <div class="problem-subs">
          {subs.map((x) => <ProblemEntry key={entryKey(x)} e={x} info={shown} />)}
        </div>
      )}
    </div>
  );
}

/**
 * Problems is what is wrong in a place: the kinds' icons as the summary
 * of a menu (Dropdown) that lists the entries; label names them to a
 * screen reader and in the tooltip; onClick, where given, sees every
 * click in it (a list inside a panel's summary keeps a click in it from
 * folding the panel). Where no item has a kind and no entry a summary,
 * nothing is wrong: the green check (OkPill) where ok gives its label,
 * else nothing; with always the list stays, and where ok is given its
 * summary is the green check and its list says ok's words as a line. An
 * entry's summary alone is information, said after the green check in
 * the summary. lead is drawn first in the
 * summary, before the kinds' icons (a sign of the place's own state);
 * head first in the list, before the entries (items that act on the
 * place, or a row of its own state). onOpen and onClose are told when the list opens and closes.
 */
export function Problems({ entries, label, ok, lead, always, head, onClick, onOpen, onClose }: {
  entries: Entry[];
  label: string;
  ok?: string;
  lead?: ComponentChildren;
  always?: boolean;
  head?: ComponentChildren;
  onClick?: (ev: TargetedMouseEvent<HTMLDetailsElement>) => void;
  onOpen?: () => void;
  onClose?: () => void;
}) {
  const kinds = problemKinds(entries);
  const extra = entries.filter((e) => e.summary);
  const well = !kinds.length && !extra.length;
  if (well && !always) return ok ? <OkPill label={ok} /> : null;
  return (
    <Dropdown cls="problems" summaryCls="problems-sum" listCls="problems-list" label={label} onClick={onClick} onOpen={onOpen} onClose={onClose} summary={
      <>
        {lead}
        <KindIcons entries={entries} />
        {!kinds.length && (!well || ok) && <Icon symbol="icon-check" cls="ni ok-check" />}
        {extra.map((e) => <Fragment key={entryKey(e)}>{e.summary}</Fragment>)}
      </>
    }>
      {head}
      {well && ok && <p class="problems-ok">{ok}</p>}
      {entries.map((e) => <ProblemEntry key={entryKey(e)} e={e} />)}
    </Dropdown>
  );
}

/**
 * OkPill stands in a problem list's place where nothing is wrong: a
 * green check in the list's frame, label its name and tooltip, so that
 * a list that comes takes its place without moving what is beside it.
 */
export function OkPill({ label }: { label: string }) {
  return (
    <span class="ok-pill" role="img" aria-label={label} title={label}>
      <Icon symbol="icon-check" cls="ni ok-check" />
    </span>
  );
}
