// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Users: its plain names, which
// call its part (ui/src/admin/, MassimoParts.users), and the check of a
// new user's name, which the part and the first owner's form (login.js)
// share.
"use strict";

// loadUsers reads System settings → Users again, for a change made in My
// profile (profile.js userAction): the part reads the users and draws them.
function loadUsers() {
  return MassimoParts.users.load();
}

// renderUsers draws System settings → Users again (app.js applyLanguage).
function renderUsers() {
  MassimoParts.users.render();
}

// USER_NAME is the store's rule for a user's name (userName in
// internal/store/users.go), after NormalName: trimmed and lower case.
const USER_NAME = /^[a-z0-9][a-z0-9._@-]{0,63}$/;

// NAME_CHECK_MS is how long a name being typed waits before it is
// checked: the owner asked for a check once the field has not changed
// for 2 s.
const NAME_CHECK_MS = 2000;

// nameProblem says what is wrong with a new user's name, "" for
// nothing: the store's rule, or a name in use among taken.
function nameProblem(name, taken) {
  const n = name.trim().toLowerCase();
  if (!n) return "";
  if (!USER_NAME.test(n)) return t("users.nameBad");
  if ((taken || []).includes(n)) return t("users.nameTaken", { name: n });
  return "";
}

// watchName checks a new user's name field: NAME_CHECK_MS after the
// last keystroke and when the field is left. msg, under the field, says
// what is wrong, and clears at the first keystroke that fixes it; taken
// returns the names in use. It returns the check, which the form's
// submit calls: true when the name may be sent. A refusal the server
// words (wordNameError) goes to msg as well.
function watchName(input, msg, taken) {
  let timer = null;
  const say = (text) => {
    showResult(msg, text, false);
    if (text) input.setAttribute("aria-invalid", "true");
    else input.removeAttribute("aria-invalid");
  };
  const check = () => {
    clearTimeout(timer);
    timer = null;
    const p = nameProblem(input.value, taken());
    say(p);
    return !p;
  };
  input.setAttribute("aria-describedby", msg.id);
  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = null;
    if (!nameProblem(input.value, taken())) say("");
    else timer = setTimeout(check, NAME_CHECK_MS);
  });
  input.addEventListener("blur", check);
  check.say = say;
  return check;
}

// wordNameError words the server's refusal of a new user's name as the
// page's check does, or returns "" for another error: 409, the name is
// in use; 400 about the name, the rule.
function wordNameError(e, name) {
  if (e.status === 409) return t("users.nameTaken", { name: name.trim().toLowerCase() });
  if (e.status === 400 && /user name/.test(e.message)) return t("users.nameBad");
  return "";
}
