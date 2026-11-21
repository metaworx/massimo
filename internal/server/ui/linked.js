// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My profile's linked accounts: the own identities
// at the sign-in providers, what each provider said of them, Remove,
// Link per enabled provider, and a link come back shown before it is
// stored. System settings → Users draws a user's identities as a part
// (ui/src/admin/Identities.tsx).
"use strict";

// --- the own identities' rows ------------------------------------------

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

// identityRow is one of the own identities in My profile (where
// "profile"), with what the provider said of it and Remove.
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
  addProfileHelp(buttons.lastElementChild || ul, "profileLinked", t("profile.linked"), box); // users.js
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
