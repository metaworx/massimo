# User manual

How to use Massimo, for the people who live with it rather than run it.
This first chapter covers the app's login by fingerprint; more chapters follow.
The web UI's pages are described in [ui.md](ui.md),
and how the apps work inside in [app.md](app.md).

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

The sign-in form shows **Login by fingerprint** with your name.
Touch it and then the sensor.
On the list of servers, the fingerprint button beside a server
signs you in straight away, without the form.

Your password still works as before.

### When the fingerprint is asked

Each server's settings, on the app's list of servers (⚙ beside the server),
say when the app asks for your fingerprint:

- **each time the app comes back**: whenever you return to the app,
  it is covered until you touch the sensor;
  the page is as you left it.
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

### Not encrypted

Over a plain `http://` address outside your home network,
the app never sends the key: anyone on the way could read it there.
The sign-in form then says so and offers your password only.
At home, on the local network, plain `http://` works as usual.

### Turning it off

- **Turn off**, in the server's settings on the app's list of servers,
  asks for your fingerprint once, removes the key at the server
  and forgets it on the phone.
- **Remove** of a server forgets everything the app kept for it.
- **Forget all stored credentials**, under the list of servers,
  forgets the keys of every server at once, without asking for the fingerprint.

When only the phone forgot a key, the server still lists it.
To remove it there, open **My profile** → **Sessions and keys** in the web UI:
the key carries the phone's name and "login by fingerprint".
**Revoke** removes it, and the phone's session with it.
Do the same if you lose the phone.

### Moving between servers

**Select server**, at the foot of the sign-in form
and at the end of the Cube choice in the header, takes you back to the app's list
of servers. You stay signed in where you were.
