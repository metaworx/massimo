// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/action.ts
 *
 * Running an action and saying how it went: a request, then a reload of
 * what it changed, then its outcome, green, or red with the error, in the
 * region's message line (showResult of components.js) or in a part's
 * state, which a ResultLine (badge.tsx) draws; a success may fade after a
 * while. A region makes one runner and keeps its own requests beside it;
 * the button that asked turns meanwhile with useBusy (question.tsx).
 */

/** An action's outcome: its text, and whether it went well. */
export interface Outcome {
  text: string;
  ok: boolean;
}

/** A region's runner of actions (actionRunner). */
export interface ActionRunner {
  /** say shows text, ok green, else red; no text hides it. */
  say(text: string, ok?: boolean): void;
  /**
   * run sends fn's request, reloads, and says ok, or what fail makes of
   * the error's message; it resolves to whether the request was taken.
   */
  run(fn: () => Promise<unknown>, ok: string, fail: (error: string) => string): Promise<boolean>;
}

/**
 * fader sets an outcome through set, and where ms is given takes a
 * success back after ms, unless another outcome came meanwhile: the
 * fade of a result line, the same for every part.
 */
export function fader(set: (res: Outcome | null) => void, ms?: number): (res: Outcome | null) => void {
  let last: Outcome | null = null;
  return (res) => {
    last = res;
    set(res);
    if (!res || !res.ok || !ms) return;
    setTimeout(() => {
      if (last === res) {
        last = null;
        set(null);
      }
    }, ms);
  };
}

/**
 * actionRunner makes a region's runner. Its outcome goes to where, the
 * message line at the time of saying (a region shown in two places has
 * two), and to set, a part's state that a ResultLine draws; fade is how
 * long a success stays, in ms, where it goes again; reload is what
 * follows a request taken.
 */
export function actionRunner({ where, set, fade, reload }: {
  where?: () => HTMLElement | null;
  set?: (res: Outcome | null) => void;
  fade?: number;
  reload?: () => Promise<unknown>;
}): ActionRunner {
  const show = fader((res) => {
    const el = where ? where() : null;
    if (el) showResult(el, res ? res.text : "", res ? res.ok : undefined);
    if (set) set(res);
  }, fade);
  const say = (text: string, ok?: boolean) => show(text ? { text, ok: !!ok } : null);
  return {
    say,
    async run(fn, ok, fail) {
      try {
        await fn();
        if (reload) await reload();
        say(ok, true);
        return true;
      } catch (e) {
        say(fail((e as Error).message), false);
        return false;
      }
    },
  };
}
