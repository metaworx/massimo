# Signing in through an OpenID Connect provider

Massimo's users may sign in through an OpenID Connect provider beside their password:
a Nextcloud, Google, Microsoft, Authentik or Keycloak account.
The provider says who signed in; Massimo decides what that user may do,
by the user's role, as for a password.
How the sign-in works, route by route, is in [api-v1.md](api-v1.md),
"Sign-in through OpenID Connect";
the tables are in [store.md](store.md), section 6.

## What to register at the provider

Massimo signs in as a confidential client with the authorization code flow and PKCE.
At the provider, create a client (an "application",
an "OAuth client") of the web kind, confidential, with:

- **the redirect URI** that System settings → Sign-in providers shows:
  the address the web UI is opened at, plus `api/v1/login/oidc/callback`,
  for example `https://massimo.example.com/api/v1/login/oidc/callback`.
  Register every address users open the page at, each with its callback;
  the provider sends nobody to an address it does not know.
- **the scopes** `openid email profile`, Massimo's default.

The provider then names a **client ID** and a **client secret**,
and its **issuer**: the URL its discovery document is under,
`<issuer>/.well-known/openid-configuration`, over `https`.
Massimo reads that document when an enabled provider is saved:
a provider not reached,
or one that calls itself by another issuer, is not saved,
and the page says what the document names.
An issuer typed with a trailing slash too many or too few
is kept as the document writes it, which the sign-in compares exactly.

## Adding it in Massimo

As the owner (the capability `auth`),
System settings → Sign-in providers → **Add a provider**: a key (`nextcloud`),
the name on the button (`Nextcloud`), the issuer,
the client ID and the client secret.
The secret is written only; the page never shows it again.
The provider's button then shows on the sign-in card.
The same through the API is `POST /api/v1/providers`.

## Who may sign in

A sign-in through a provider finds its user in one of three ways,
in this order:

1. **The provider's account is known**:
   the user it was bound to before, by the provider's subject (`sub`),
   which never changes.
2. **An e-mail or a user ID assigned**:
   in System settings → Users,
   a user's **Sign-in through providers** takes a provider,
   and either an e-mail or the user ID at the provider.
   - A **user ID** is the provider's subject (`sub`) itself:
     the account is bound at once, and no e-mail is needed.
     Nextcloud's user ID is the login name; Google's is a number.
   - An **e-mail** binds the account to the user at its first sign-in,
     which is then found by the subject alone.
     The provider must send the e-mail and say it is verified (`email_verified`),
     else it does not bind.
     Only the account's one primary e-mail is sent;
     a second address never matches.
   - **Linked by the user**:
     a signed-in user links its own account in My profile → **Linked accounts**,
     **Link Nextcloud**, by signing in there once;
     it is bound at once, and replaces the user's other link at that provider.
     An account another user has is refused.
3. **Users created**:
   where the provider's **Create users who sign in** is on,
   an account with a verified e-mail of the domains named (any where none are)
   becomes a new user with the role chosen (`viewer` by default),
   which may hold no administrative capability.
   It is off by default.

Anyone else comes back to the sign-in card,
which says, under the providers' buttons, why:
the account is not known on this server,
the provider sent no e-mail,
or it did not mark the e-mail verified.
The log has a warning with the provider, the account and that reason.

Massimo reads the claims from the `id_token`,
and from the provider's userinfo endpoint where its discovery document names one,
for what the `id_token` lacks.
A disabled or expired user is refused as with a password.

## Nextcloud

Nextcloud is a provider with its app **OpenID Connect Provider** (`oidc`).
In Administration settings → Security → OpenID Connect clients,
add a client named `Massimo` with the redirect URI above,
of the type confidential;
Nextcloud shows its client ID and secret.
The issuer is Nextcloud's address, such as `https://cloud.example.com`,
without a trailing slash, as its discovery document writes it;
the web server must answer `/.well-known/openid-configuration`
as the app's documentation describes.
Since its version 2.0 the app puts the e-mail and the profile into the `id_token`
only where the client asks for them, or its admin setting says so;
Massimo reads them from its userinfo endpoint.
Where the e-mail still does not bind (the log says `unverified` or `noemail`),
assign the user's Nextcloud login name as the user ID instead.

## Google and Microsoft

- **Google**:
  in the Google Cloud console,
  an OAuth client of the type "Web application" with the redirect URI;
  the issuer is `https://accounts.google.com`.
- **Microsoft Entra ID**:
  an app registration with a web redirect URI and a client secret;
  the issuer is `https://login.microsoftonline.com/<tenant id>/v2.0`.

## What Massimo does not do

- **Sign out at the provider**:
  Sign out ends Massimo's session only.
- **The app**:
  the Android and iOS apps show no provider button yet,
  since the providers refuse a sign-in inside an app's page.
- **Check the `id_token`'s signature**:
  Massimo receives it straight from the provider's token endpoint over TLS,
  which OpenID Connect allows in place of the signature, and checks its issuer,
  audience, expiry and nonce.
