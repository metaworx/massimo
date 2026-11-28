# User manual

How to use Massimo, for the people who live with it rather than run it.
The first chapters cover the app: its list of servers,
and its login by fingerprint; more chapters follow.
The web UI's pages are described in [ui.md](ui.md),
and how the apps work inside in [app.md](app.md).

## The list of servers in the app

The Massimo app opens on its list of servers,
or straight on the server you chose to open at start.
The servers are in the order of their names.
Each server in the list says whether the app reaches it
(on the LAN, remote, not reachable, or a changed certificate)
and, in green, who you are signed in as there.
Touch a server to open it; ⚙, or a long press, opens its settings.
A server's page that does not load brings you back to the list,
which says that the server does not answer.
Pull the list down to check every server again, as ↻ does.
**Back** closes an open menu or panel first; from the bare list it leaves the app.
Where the app keeps a login by fingerprint for a server,
touching it asks for your fingerprint first (below),
and the sign-in icon beside it opens its sign-in form straight away.

### The list's menu

**☰**, at the top right, beside Massimo's logo, holds:

- the app's version: touch it, or its copy icon, to copy it;
  the menu closes, and "Copied" shows at the foot of the screen.
- **Add server…**, which shows the card to add one:
  the servers the app finds on your network, and a field for an address.
  The card closes once a server is added, or with **×**.
  While the list is empty, the card is always there.
- **Sign out of all**, which signs the app out of every server.
- **Forget all stored credentials**, where a server keeps a login by
  fingerprint; it asks first, under the list,
  and turns every such login off (below).
- **App settings**, below.
- **Exit**, which closes the app as swiping it away from the recent apps does:
  the sessions it held only in memory end at the next start.

### A server's settings

Opening a server's settings does not open the keyboard;
touch a field to change it.
On a server's page, **☰** → **This server's settings** opens them too,
on the list, and **App settings** the app's own.
They have three tabs, **Connection** first; ✕ at the top closes them.

**Connection**:

- the name and the two addresses;
- **Open at start**: the app opens this server when it starts,
  in place of the list.
  One server at most has it, and the first server you add has it
  until you choose another.
  **Select server** in its ☰, or **Cancel** on its sign-in form,
  takes you back to the list.
  When the server does not answer, the app shows the list;
- when the app asks for your fingerprint (below);
- **Cancel** and **Save**, on once you change something;
  the tab shows a disk while a change is not saved,
  and **Cancel** puts back what is saved;
- **Remove server**, at the foot under **Cancel** and **Save**,
  takes the server off the list; below.

**Back** or ✕ with changes not saved asks first: **Discard** them, or **Stay**.

**Login**, where each button acts at once:

- **Signed in as …** and **Sign out**, where the app is signed in there:
  Sign out ends that session on the server and on the phone.
  While the server does not answer, the list's "signed in as …" is grey,
  and the line says that Sign out ends the session there
  the next time the app reaches the server.
- **Store login**, where you signed in without the login by fingerprint,
  keeps it now: the phone asks for your fingerprint once.
- **A session of this phone is still valid here**, where the app
  forgot a session while the server did not answer
  (Sign out, Turn off, Forget all, or a new sign-in in its place,
  or the app closed or ended by the phone while it was signed in):
  the next time the app reaches the server, it ends that session there
  by itself, and the line goes; a server older than the app's way of doing
  that ends it at the next sign-in there as the same user.
- **A key of this phone is still valid here**, where Turn off or Forget all
  could not remove the phone's key at the server,
  or the phone no longer opens its login, or a new certificate was trusted:
  **Show in My profile** checks the server, then opens it at My profile,
  the key marked, to revoke it there;
  it is off while the server does not answer.

The Login tab shows a dot while such a key or session is there.

**Certificate**, for a server reached over `https://`:
who it is for and who issued it,
its names, its validity, its SHA-256 fingerprint,
and since when the app trusts it:
since you compared its fingerprint when you added the server,
or because a certificate authority the phone knows signed it.
While the server does not answer, it shows the fingerprint the app trusts.

### App settings

**Appearance**: as the phone, light or dark.
It sets the list, and every server's page whose own appearance
(My settings) is "as the system".

## The login by fingerprint in the app

The Massimo app for Android can sign you in to a server with your fingerprint,
so that you type your password once.

### Turning it on

1. Open the app and choose the server.
2. On the sign-in form, switch on **Enable login by fingerprint**,
   then sign in with your name and password as usual.
3. The phone asks for your fingerprint. Touch the sensor.

The app now keeps a key for you on the phone, locked by your fingerprint.
If you cancel the fingerprint, you are still signed in,
and the form offers **Try again** or **Sign in without it**.

The switch shows only where the phone has a fingerprint enrolled.

### Signing in

The sign-in form's row of the ways to sign in, **Password** and **API key**,
starts with the fingerprint, **Login by fingerprint** for your name.
Touch it and then the sensor.

On the list of servers, touching a server with a login by fingerprint
asks for your fingerprint, or your PIN, and opens it signed in,
without the form.
If you cancel, the server opens at its sign-in form,
where the fingerprint, your password and an API key are all offered.
The sign-in icon beside the server, **Sign in with password or key**,
opens that form at once, without asking for the fingerprint.

Your password still works as before.

### When the fingerprint is asked

Each server's settings, on the app's list of servers (⚙ beside the server),
say when the app asks for your fingerprint:

- **each time the app comes back**: whenever you return to the app,
  it is covered until you touch the sensor;
  the page is as you left it.
  Opening the server from the list asks each time too,
  and going back to it from the list shows its sign-in form.
  The phone also keeps no picture of the page in its list of recent apps.
  On Android 12 and older, this also blocks screenshots of that page;
- **when the app starts** (the usual choice):
  each time the app is started anew;
- **only after a logout**: you stay signed in until you log out,
  and the fingerprint is asked on the sign-in form only.

Logging out ends the session on this phone only.
The key stays, so the next sign-in can again be by fingerprint.

### The PIN, and a new fingerprint

- On **Android 11 and later**, the phone also accepts your screen lock's PIN
  in place of the fingerprint.
  Because the PIN opens the key too, enrolling a new fingerprint keeps it.
- On **Android 8 to 10**, only the fingerprint opens the key.
  Enrolling a new fingerprint ends it,
  and the form asks for your password again.
- On every version, turning the screen lock off ends the key.

The app keeps one key for all its logins, behind your fingerprint,
so that one fingerprint opens what a step needs: keeping a login,
signing in, turning one off, or Forget all.
When that key ends, every login by fingerprint ends with it,
and the app removes each key at its server the next time it signs in there
as the same user.

### Not encrypted

Over a plain `http://` address outside your home network,
the app never sends the key: anyone on the way could read it there.
The sign-in form then says so and offers your password only.
At home, on the local network, plain `http://` works as usual.

### Turning it off

- **Turn off**, in the server's settings on the app's list of servers,
  removes the key at the server, signs you out there
  and forgets the key on the phone.
  Where the app is signed in to that server it needs nothing more;
  where it is not, it asks for your fingerprint once.
- **Forget all stored credentials**, in the list's ☰,
  does the same for every server with a login by fingerprint,
  one after the other:
  it asks for your fingerprint once,
  for all the servers the app is not signed in to.
  A login kept by an older version of the app asks once more for itself,
  until you turn it off or keep it again.
  If you cancel the fingerprint, it asks no more,
  and those keys stay valid at their servers (below).
  Its message names the servers where a key stays valid.
- **Remove server**, in a server's settings, turns its login by fingerprint off first,
  as Turn off does, or signs the app out there,
  then forgets everything the app kept for it.
  It says so before, and warns where a key or a session of the phone is
  still valid there;
  afterwards its message names any key or session that stays valid,
  to revoke under My profile.

When Turn off or Forget all cannot remove the key at the server (it does not answer,
or you dismiss the fingerprint), the phone forgets the key all the same,
and the server's settings say that a key of this phone is still valid there.
Where the server did not answer, the app ends the key the next time it
reaches the server, and says so in the message and on the key's line;
Remove does the same for up to 90 days after the server left the list,
wherever the app finds that server again;
so do they where the phone no longer opens its login,
or where you trusted a new certificate for the server.
Remove names such a key in its message instead,
since the server is gone from the list.
The next time the app signs in to that server as the same user,
it removes the key there by itself.
A login kept by an older version of the app does not know its key's number
until it is used once; where such a key cannot be removed,
the message says it is still listed at the server, under My profile.
To remove it at once, use **Show in My profile**,
or open **My profile** → **Sessions and keys** in the web UI:
the key carries the phone's name and "login by fingerprint",
and in the app the key the phone keeps says **this device's key**.
Every row there shows its id, and a server's settings in the app
name the id of the key its login by fingerprint keeps,
so that you can tell which row is which.
**Revoke** removes it, and the phone's session with it.
Do the same if you lose the phone.

### Moving between servers

**Select server**, last in the sign-in form's ☰,
at the end of the Cube choice in the header, and in the ☰ after
**App settings** and **This server's settings**, just before Sign out,
and **Cancel**, left of the sign-in form's **Sign in**,
take you back to the app's list of servers.
You stay signed in where you were; the list shows where.
**Sign out** on a server's page signs you out there and takes you back to the list too.

The sign-in form names the server above its title: its name, its address,
and ⓘ, which shows the server's instance id and its certificate's fingerprint;
touch either, or its copy icon, to copy it.
On a server without users, the form that creates its owner gives the command
that does it on the server's box, `massimo user add`, on a line of its own:
touch it, or its copy icon, to copy it.
A new password is typed twice; one eye shows and hides both,
and the second field turns red, with a line under it, while the two differ.
