// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/KeepOpen.tsx
 *
 * The confirmation that keeps a server without users open, in the sign-in
 * card in the first owner's form's place (AP FirstOwner, Block 3, D4):
 * the owner's warning first, what keeping it open means, this page's
 * address as a text that copies itself with one tap (D8), a field to
 * enter it into, and Confirm, off until the address entered is this
 * page's own (D13, D28; store.ts sameAddress), the line under the field
 * saying why; Back shows the form again. The browser's own check of the
 * address does not stand in: the form does not validate, and Enter with
 * another address sends nothing. Confirm writes the setting, reads who is
 * in and closes the card as Skip for now closes it; while it runs Back
 * and the card's other ways out are off. A refusal for lack of a session
 * or because the server has users now turns the card to the sign-in; any
 * other is said under the field. What is typed stays through Back and is
 * emptied at the next opening.
 */

import { useLayoutEffect, useRef, useState } from "preact/hooks";
import { ResultLine } from "../shared/badge";
import { hid } from "../shared/cls";
import { Copy } from "../shared/copy";
import { Field } from "../shared/field";
import { useLang } from "../shared/lang";
import { busyProps, useBusy } from "../shared/question";
import { ask, card, keepBack, sameAddress } from "./store";

/** KeepOpen is the confirmation's box, drawn once with the card and shown in the keep step. */
export function KeepOpen() {
  useLang();
  const shown = card.ownerStep === "keep";
  const opened = card.opened;
  const focus = card.focus;
  const working = card.working;
  const [address, setAddress] = useState("");
  const fieldRef = useRef<HTMLInputElement>(null);
  const warnRef = useRef<HTMLParagraphElement>(null);
  const confirm = useBusy();
  const same = sameAddress(address);
  const differs = !!address.trim() && !same;
  // each opening empties the address, as the card's own fields
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed.
  useLayoutEffect(() => {
    if (opened) setAddress("");
  }, [opened]);
  // the field in focus as the box is shown; on a touch screen, where a field's focus brings the soft keyboard up by
  // itself (UiParts D224), the warning, which a screen reader then reads first
  // biome-ignore lint/correctness/useExhaustiveDependencies: focus is read from the card's store, which Biome takes for a stable import; the effect is for each focus asked, so it is listed.
  useLayoutEffect(() => {
    if (!focus.n || focus.on !== "keep") return;
    if (matchMedia("(pointer: coarse)").matches) warnRef.current?.focus();
    else fieldRef.current?.focus();
  }, [focus]);
  const submit = (ev: Event) => {
    ev.preventDefault();
    if (confirm.busy || !sameAddress(address)) return;
    const a = ask();
    card.keepSaid = null;
    card.working = true;
    confirm.run(async () => {
      try {
        await api("PUT", "api/v1/settings", { "auth.keep_open": true });
        await loadMe(); // who is in says kept_open now: no card asks again
        if (a && !a.closed) a.dismiss(); // the card closes as Skip for now closes it, the page goes on
      } catch (e) {
        const err = e as Error & { status?: number };
        // the server has users now: refused for lack of a session (api() has turned the card already) or as a server
        // with users (409): the card becomes the sign-in
        if (err.status === 401 || err.status === 409) {
          askLogin();
          return;
        }
        card.keepSaid = { text: t("keepOpen.failed", { error: err.message }), ok: false };
      } finally {
        card.working = false;
      }
    });
  };
  return (
    <form id="keep-open" class={"keep-open" + hid(!shown)} noValidate onSubmit={submit}>
      <p id="keep-open-warning" ref={warnRef} class="notice" role="note" tabIndex={-1}>{t("keepOpen.warning")}</p>
      <p id="keep-open-text" class="muted small">{t("keepOpen.text")}</p>
      <p class="keep-open-origin"><Copy id="keep-open-origin" text={location.origin} label={t("keepOpen.address")} icon /></p>
      <Field label={t("keepOpen.enter")} id="keep-open-address" wide hint={differs ? t("keepOpen.differs") : undefined} hintId="keep-open-hint">
        <input ref={fieldRef} id="keep-open-address" type="url" class="field-control" autocomplete="off" autocapitalize="none" spellcheck={false}
          inputMode="url" value={address} aria-invalid={differs ? "true" : undefined}
          aria-describedby={differs ? "keep-open-warning keep-open-hint" : "keep-open-warning"} onInput={(ev) => setAddress(ev.currentTarget.value)} />
      </Field>
      <ResultLine id="keep-open-msg" cls="small" role="alert" res={card.keepSaid} />
      <div class="actions">
        <span class="actions-right">
          <button type="button" id="keep-open-back" class="form-cancel" disabled={working} onClick={keepBack}>{t("keepOpen.back")}</button>
          <button type="submit" id="keep-open-confirm" {...busyProps("primary", confirm.busy, !same)}>{t("keepOpen.confirm")}</button>
        </span>
      </div>
    </form>
  );
}
