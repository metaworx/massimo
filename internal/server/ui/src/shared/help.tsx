// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/help.tsx
 *
 * A setting's help as a part (helpParts of app.js): a labelled button
 * with the question mark that shows and hides the setting's explanation,
 * help.<key> of the texts, hidden until asked for. An explanation shown
 * stays shown wherever the part is drawn again or in another place, by
 * its key, until its button is pressed again. Which are shown is the
 * page's one set (helpOpen of app.js), which the plain help buttons keep
 * too; a part draws again by a counter of its own, so that a key is drawn
 * by one side only, as every key is today. useHelp gives the pair, which
 * a field's row, a switch and a group of My profile draw alike.
 */

import { signal } from "@preact/signals";
import { useId } from "preact/hooks";
import { IconButton } from "./icon";

/** Counts the parts' changes of helpOpen, so that the parts that read it are drawn again. */
const changes = signal(0);

/** shown says whether the explanation of key is shown, read as a signal. */
function shown(help: string): boolean {
  void changes.value;
  return helpOpen.has(help);
}

/** HelpButton is the button that shows and hides the explanation of key, of the id given, named after label, the setting's name. */
export function HelpButton({ help, id, label }: { help: string; id: string; label: string }) {
  const on = shown(help);
  return (
    <IconButton symbol="icon-help" iconCls="hi" cls={on ? "help-btn active" : "help-btn"} label={t("help.button", { label })} title={t("help.title")}
      aria-controls={id} aria-expanded={on ? "true" : "false"} onClick={() => {
        if (on) helpOpen.delete(help);
        else helpOpen.add(help);
        changes.value++;
      }} />
  );
}

/** HelpText is the explanation of key, with the id its button controls; hidden until its button shows it. */
export function HelpText({ help, id }: { help: string; id: string }) {
  return <p id={id} class={shown(help) ? "help-text" : "help-text hidden"}>{t("help." + help)}</p>;
}

/**
 * useHelp is a setting's or a group's help, by the key of its help: its
 * button, named after label, and its explanation, of an id of their own
 * (help-<key>-…); the caller draws the button where it belongs (after a
 * label, behind a group's last button) and the text under it.
 */
export function useHelp(help: string, label: string) {
  const id = "help-" + help + "-" + useId();
  return { button: <HelpButton help={help} id={id} label={label} />, text: <HelpText help={help} id={id} /> };
}
