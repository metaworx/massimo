// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the OpenID Connect providers (docs/api-v1.md,
// "Providers" and "Identities"): System settings → Sign-in providers,
// with the capability auth, and a user's identities at them in System
// settings → Users (users.js builds the panel). The sign-in through a
// provider is login.js'.
"use strict";

const providersUI = {
  list: [], // GET /api/v1/providers, with auth
  login: [], // GET /api/v1/login/providers: the enabled ones, by key and name
  open: new Set(), // the keys whose panel is open
};

// ADMIN_CAPS are the capabilities a role may not hold to be given to the
// users a provider creates (adminCapabilities in internal/server/oidc.go).
const ADMIN_CAPS = ["guests", "users", "passwords", "roles", "roles.disable", "capabilities", "cubes", "settings", "update", "restart", "reboot", "audit", "auth"];

function providersMsg(text, ok) {
  showResult($("providers-msg"), text, ok);
}

// loadLoginProviders reads the enabled providers, for the sign-in's
// buttons and a user's identities; none where the server answers none.
async function loadLoginProviders() {
  try {
    providersUI.login = (await apiRequest("GET", "api/v1/login/providers", undefined, { auth: false })) || [];
  } catch (e) {
    providersUI.login = [];
  }
  return providersUI.login;
}

// loadProviders reads every provider, for a user with auth.
async function loadProviders() {
  if (!hasCap("auth")) return;
  providersUI.list = (await api("GET", "api/v1/providers")) || [];
}

// autoRoleOptions are the roles a provider may give the users it creates:
// those read (users.js) that hold no administrative capability, else
// the default ones that hold none; current is kept in the list.
function autoRoleOptions(current) {
  let roles = (usersUI.roles || [])
    .filter((r) => r.key !== "owner" && !(r.capabilities || []).some((c) => ADMIN_CAPS.includes(c)))
    .map((r) => [r.key, roleLabel(r.key, r.name)]);
  if (!roles.length) roles = ["guest", "viewer", "operator", "integration"].map((k) => [k, roleLabel(k)]);
  if (current && !roles.some(([k]) => k === current)) roles.push([current, roleLabel(current)]);
  return roles;
}

// providerAction runs a change, reads the providers again, redraws them
// and says how it went; btn is busy until then.
function providerAction(fn, okText, name, btn) {
  return whileBusy(btn, async () => {
    try {
      await fn();
      await loadProviders();
      await loadLoginProviders();
      renderProviders();
      providersMsg(okText, true);
      return true;
    } catch (e) {
      providersMsg(t("providers.failed", { name, error: e.message }), false);
      return false;
    }
  });
}

// providerForm is a provider's form, or with p null the form that adds
// one: the key only then, the secret written only, the role and the
// domains while it creates users.
function providerForm(p, onCancel) {
  const form = fromTemplate("tpl-provider-form");
  if (p) form.classList.remove("panel"); // in its panel, framed by it
  const id = p ? p.key : "new";
  for (const input of form.querySelectorAll(".field.wide > input, .field.wide > select")) {
    input.id = "provider-" + id + "-" + input.name;
    input.previousElementSibling.htmlFor = input.id;
  }
  const field = (name) => form.querySelector("[name=" + name + "]");
  form.querySelector(".provider-key").classList.toggle("hidden", !!p);
  field("name").value = p ? p.name : "";
  field("issuer").value = p ? p.issuer : "";
  field("client_id").value = p ? p.client_id : "";
  field("scopes").value = p ? p.scopes : "";
  field("enabled").checked = p ? p.enabled : true;
  field("auto_create").checked = !!(p && p.auto_create);
  field("auto_domains").value = p ? (p.auto_domains || []).join(", ") : "";
  field("allowed_groups").value = p ? (p.allowed_groups || []).join(", ") : "";
  field("groups_claim").value = p ? p.groups_claim || "" : "";
  fillOptions(field("auto_role"), autoRoleOptions(p && p.auto_role));
  field("auto_role").value = p ? p.auto_role : "viewer";
  form.querySelector(".secret-hint").textContent = t(p && p.secret_lost ? "providers.secretLost" : p && p.has_secret ? "providers.secretKept" : "providers.secretNone");
  const autoOnly = () => {
    for (const el of form.querySelectorAll(".auto-only")) el.classList.toggle("hidden", !field("auto_create").checked);
  };
  field("auto_create").addEventListener("change", autoOnly);
  autoOnly();
  const actions = form.querySelector(".actions");
  const remove = form.querySelector(".remove-provider");
  remove.classList.toggle("hidden", !p);
  remove.addEventListener("click", () => {
    confirmInPlace(actions, t("providers.removeConfirm", { name: p.name || p.key }), { yes: t("providers.remove"), no: t("action.keep") }, () => {
      providersUI.open.delete(p.key);
      return providerAction(() => api("DELETE", "api/v1/providers/" + encodeURIComponent(p.key)), t("providers.removed", { name: p.name || p.key }), p.key);
    }, ".remove-provider");
  });
  const cancel = form.querySelector(".cancel-provider");
  cancel.classList.toggle("hidden", !!p);
  cancel.addEventListener("click", () => onCancel && onCancel());
  const save = form.querySelector("button[type=submit]");
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = {
      name: field("name").value.trim(),
      issuer: field("issuer").value.trim(),
      client_id: field("client_id").value.trim(),
      scopes: field("scopes").value.trim(),
      enabled: field("enabled").checked,
      auto_create: field("auto_create").checked,
      auto_role: field("auto_role").value,
      auto_domains: field("auto_domains").value.split(",").map((d) => d.trim()).filter(Boolean),
      allowed_groups: field("allowed_groups").value.split(",").map((g) => g.trim()).filter(Boolean),
      groups_claim: field("groups_claim").value.trim(),
    };
    const secret = field("client_secret").value;
    if (secret || !p) body.client_secret = secret;
    if (!p) body.key = field("key").value.trim().toLowerCase();
    const name = body.name || (p ? p.key : body.key);
    const send = () => (p ? api("PUT", "api/v1/providers/" + encodeURIComponent(p.key), body) : api("POST", "api/v1/providers", body));
    providerAction(send, t(p ? "providers.saved" : "providers.added", { name }), name, save).then((ok) => {
      if (ok && !p) {
        providersUI.open.add(body.key);
        renderProviders();
        clear($("provider-new"));
        $("provider-add").classList.remove("hidden");
      }
    });
  });
  return form;
}

// renderProviders draws System settings → Sign-in providers: the
// redirect URI to register, a panel per provider with its form.
function renderProviders() {
  const box = $("providers-list");
  if (!box || !hasCap("auth")) return;
  $("providers-callback").textContent = pageBase() + "api/v1/login/oidc/callback"; // login.js
  clear(box);
  if (!providersUI.list.length) box.appendChild(textEl("p", t("providers.none"), "muted small"));
  for (const p of providersUI.list) {
    const badges = [];
    if (!p.enabled) badges.push(badgeEl(t("providers.off"), "bad"));
    if (p.auto_create) badges.push(badgeEl(t("providers.createsUsers"), "warn"));
    const d = detailsPanel({ name: p.name || p.key, mono: p.key, badges, open: providersUI.open.has(p.key), cls: "provider-panel" }, providerForm(p));
    d.dataset.key = p.key;
    d.addEventListener("toggle", () => (d.open ? providersUI.open.add(p.key) : providersUI.open.delete(p.key)));
    box.appendChild(d);
  }
  sectionBadges($("sys-providers"), providersUI.list.length ? [badgeEl(String(providersUI.list.length), "mode")] : []);
}

// --- a user's identities (System settings → Users, users.js; My profile) --

// identityDetails is what the provider said of the account at its last
// sign-in or link: its name and user name, its groups, and when.
function identityDetails(i) {
  const el = document.createElement("span");
  const parts = [];
  if (i.name) parts.push(i.name);
  if (i.username && i.username !== i.name) parts.push(i.username);
  if (i.email && i.subject && i.subject !== i.username) parts.push(t("users.identityUserID", { id: i.subject }));
  if (i.groups && i.groups.length) parts.push(t("users.identityGroups", { groups: i.groups.join(", ") }));
  if (parts.length) el.appendChild(document.createTextNode(parts.join(" · ") + " · "));
  if (i.last_used_at) {
    el.appendChild(document.createTextNode(t("users.identityLastUsed") + " "));
    const age = ageEl(i.last_used_at, false, true);
    if (age) el.appendChild(age);
  } else {
    el.appendChild(document.createTextNode(t("users.neverUsed")));
  }
  return el;
}

// identityRow is one of a user's identities, with what the provider said
// of it, Remove, and for the manager (where is not "profile") Change of
// one no sign-in used yet.
function identityRow(u, i, where) {
  const li = fromTemplate("tpl-identity-row");
  li.dataset.id = String(i.id);
  const known = providersUI.login.find((p) => p.key === i.provider);
  const providerName = known ? known.name : i.provider;
  li.querySelector(".identity-provider").textContent = providerName;
  const account = i.email || t("users.identityUserID", { id: i.subject });
  li.querySelector(".identity-email").textContent = account;
  li.querySelector(".identity-state").textContent = t(i.bound ? "users.identityBound" : "users.identityWaiting");
  const details = li.querySelector(".identity-details");
  details.replaceChildren(identityDetails(i));
  details.classList.remove("hidden");
  const path = "api/v1/users/" + encodeURIComponent(u.name) + "/identities/" + i.id;
  const remove = li.querySelector(".remove-identity");
  remove.setAttribute("aria-label", t("users.identityRemoveLabel", { email: account, provider: providerName }));
  remove.addEventListener("click", () => {
    userAction(() => api("DELETE", path), t("users.identityRemoved"), u.name, remove, where);
  });
  const change = li.querySelector(".change-identity");
  const form = li.querySelector(".identity-change");
  if (where !== "profile" && !i.last_used_at) {
    change.classList.remove("hidden");
    change.setAttribute("aria-label", t("users.identityChangeLabel", { email: account, provider: providerName }));
    const input = form.querySelector("input[name=account]");
    const bySubject = !i.email;
    input.setAttribute("aria-label", t(bySubject ? "users.identitySubject" : "users.identityEmail"));
    change.addEventListener("click", () => {
      input.value = bySubject ? i.subject : i.email;
      form.classList.remove("hidden");
      change.classList.add("hidden");
      input.focus();
    });
    form.querySelector(".cancel-change").addEventListener("click", () => {
      form.classList.add("hidden");
      change.classList.remove("hidden");
      change.focus();
    });
    form.addEventListener("submit", (ev) => {
      ev.preventDefault();
      const value = input.value.trim();
      if (!value) {
        usersMsg(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
        return;
      }
      userAction(() => api("PUT", path, bySubject ? { subject: value } : { email: value }), t("users.identityChanged"), u.name,
        form.querySelector("button[type=submit]"));
    });
  }
  return li;
}

// --- My profile: the own linked accounts --------------------------------

// renderProfileIdentities draws the user's own linked accounts into My
// profile: each with what its provider said, Remove, and Link per
// enabled provider (startLink). Nothing where no provider is enabled and
// none is linked.
async function renderProfileIdentities(u) {
  const host = $("profile-identities");
  if (!providersUI.login.length) await loadLoginProviders();
  let list = [];
  try {
    list = (await api("GET", "api/v1/users/" + encodeURIComponent(u.name) + "/identities")) || [];
  } catch (e) {
    host.replaceChildren(textEl("p", e.message, "muted small bad"));
    return;
  }
  const providers = appBridge() ? [] : providersUI.login;
  if (!list.length && !providers.length) {
    clear(host);
    return;
  }
  const box = fromTemplate("tpl-profile-identities");
  const ul = box.querySelector(".user-identities");
  if (!list.length) ul.appendChild(textEl("li", t("profile.linkedNone"), "muted small"));
  for (const i of list) ul.appendChild(identityRow(u, i, "profile"));
  const buttons = box.querySelector(".profile-link-buttons");
  for (const p of providers) {
    const linked = list.some((i) => i.provider === p.key && i.bound);
    const btn = textEl("button", t(linked ? "profile.linkAgain" : "profile.link", { name: p.name }));
    btn.type = "button";
    btn.dataset.provider = p.key;
    btn.addEventListener("click", () => startLink(u, p.key, btn));
    buttons.appendChild(btn);
  }
  host.replaceChildren(box);
}

// renderLinkConfirm shows a link come back at the top of Linked
// accounts, before it is stored (AP Oidc v1.2, D38-A): the account at
// the provider and the link it replaces; Link stores it, Cancel drops
// it.
async function renderLinkConfirm(u, code) {
  const path = "api/v1/users/" + encodeURIComponent(u.name) + "/identities/link/pending/" + encodeURIComponent(code);
  let p;
  try {
    p = await api("GET", path);
  } catch (e) {
    profileMsg(t("profile.linkError.expired", { error: e.message }), false);
    return;
  }
  const box = fromTemplate("tpl-link-confirm");
  box.querySelector(".link-confirm-title").textContent = t("profile.confirm.title", { name: p.provider_name });
  const dl = box.querySelector(".link-confirm-details");
  const row = (key, value) => {
    if (!value) return;
    dl.appendChild(textEl("dt", t(key)));
    dl.appendChild(textEl("dd", value));
  };
  row("profile.confirm.email", p.email);
  row("profile.confirm.name", p.name);
  row("profile.confirm.username", p.username);
  row("profile.confirm.subject", p.subject);
  row("profile.confirm.groups", (p.groups || []).join(", "));
  const replaces = box.querySelector(".link-confirm-replaces");
  if (p.replaces && p.replaces.length) {
    const accounts = p.replaces.map((i) => i.email || i.username || i.subject).join(", ");
    replaces.textContent = t("profile.confirm.replaces", { accounts });
    replaces.classList.remove("hidden");
  }
  const link = box.querySelector(".confirm-link");
  const cancel = box.querySelector(".cancel-link");
  link.addEventListener("click", () => {
    userAction(() => api("POST", path), t("profile.linkDone", { name: p.provider_name }), u.name, link, "profile");
  });
  cancel.addEventListener("click", async () => {
    try {
      await whileBusy(cancel, () => api("DELETE", path));
    } catch (e) {
      // gone already: nothing was stored either way
    }
    box.remove();
    profileMsg(t("profile.linkCancelled"), true);
  });
  const host = $("profile-identities");
  host.insertBefore(box, host.firstChild);
  box.querySelector(".link-confirm-title").focus();
}

// startLink starts the link of the user's own account at a provider: a
// sign-in there, which comes back to My profile (oidcReturn, login.js).
async function startLink(u, key, btn) {
  try {
    await whileBusy(btn, async () => {
      const started = await api("POST", "api/v1/users/" + encodeURIComponent(u.name) + "/identities/link/" + encodeURIComponent(key), { redirect: pageBase() });
      location.assign(started.url);
    });
  } catch (e) {
    profileMsg(t("profile.linkFailed", { error: e.message }), false);
  }
}

// fillIdentities loads a user's identities into its open panel.
async function fillIdentities(u, box) {
  const ul = box.querySelector(".user-identities");
  clear(ul);
  let list = [];
  try {
    list = (await api("GET", "api/v1/users/" + encodeURIComponent(u.name) + "/identities")) || [];
  } catch (e) {
    ul.appendChild(textEl("li", e.message, "muted small bad"));
    return;
  }
  if (!list.length) ul.appendChild(textEl("li", t("users.identitiesNone"), "muted small"));
  for (const i of list) ul.appendChild(identityRow(u, i));
}

// wireIdentities wires a user panel's identities once, when it is built:
// the providers to choose, by e-mail or by the user id there, Assign.
// The box is hidden where there is no provider.
function wireIdentities(u, box) {
  box.classList.toggle("hidden", !providersUI.login.length);
  const select = box.querySelector("select[name=provider]");
  const kind = box.querySelector("select[name=kind]");
  const account = box.querySelector("input[name=account]");
  for (const [el, what] of [[select, "provider"], [kind, "kind"], [account, "account"]]) {
    el.id = "user-identity-" + what + "-" + u.name;
    el.previousElementSibling.htmlFor = el.id;
  }
  fillOptions(select, providersUI.login.map((p) => [p.key, p.name]));
  const label = box.querySelector(".identity-value-label");
  kind.addEventListener("change", () => {
    const bySubject = kind.value === "subject";
    label.dataset.i18n = bySubject ? "users.identitySubject" : "users.identityEmail";
    label.textContent = t(label.dataset.i18n);
    account.type = bySubject ? "text" : "email";
  });
  const add = box.querySelector(".add-identity");
  add.addEventListener("click", () => {
    const value = account.value.trim();
    const bySubject = kind.value === "subject";
    if (!value) {
      usersMsg(t(bySubject ? "users.identitySubjectNeeded" : "users.identityEmailNeeded"), false);
      return;
    }
    const body = bySubject ? { provider: select.value, subject: value } : { provider: select.value, email: value };
    userAction(() => api("POST", "api/v1/users/" + encodeURIComponent(u.name) + "/identities", body),
      t("users.identityAdded", { email: bySubject ? t("users.identityUserID", { id: value }) : value }), u.name, add);
  });
}

function initProviders() {
  $("provider-add").addEventListener("click", () => {
    $("provider-add").classList.add("hidden");
    const box = $("provider-new");
    clear(box);
    box.appendChild(providerForm(null, () => {
      clear(box);
      $("provider-add").classList.remove("hidden");
    }));
    box.querySelector("input[name=key]").focus();
  });
  onView("system", () => {
    providersMsg("");
    if (!hasCap("auth")) return;
    loadProviders()
      .then(renderProviders)
      .catch((e) => providersMsg(t("providers.failed", { name: "", error: e.message }), false));
  });
}

document.addEventListener("DOMContentLoaded", initProviders);
