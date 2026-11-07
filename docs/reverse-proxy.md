# Behind a reverse proxy

`massimo serve` runs on plain HTTP on the box by default,
and a reverse proxy in front of it is the usual way to add TLS where one exists already:
nginx or Caddy on the box or another one, Traefik,
the reverse proxy of a Synology or QNAP NAS.
The proxy terminates TLS with its own certificate and renews it with its own client,
and sets HSTS itself; the server then needs `-tls` not at all.
What the server must learn from the proxy, and what the proxy must do,
is below;
the server's own TLS (`-tls files`, `-tls self-signed`,
`-tls acme`) is in [README.md](../README.md), "Server".

## The client's address

The server locks an address out after repeated failed token checks or logins,
offers the first-owner form to the local network only,
and writes the client's address into the audit and the log.
Behind a proxy every request comes from the proxy's address,
so without more every client would share one lockout and the form would see the proxy.
`-trusted-proxies <addresses or CIDRs>` (`MASSIMO_TRUSTED_PROXIES`) names
the proxies whose headers the server believes:
for a request from one of them the client is the rightmost entry of
`X-Forwarded-For` that is not a trusted proxy itself (its header lines read as one list,
the leftmost entry when every one is a trusted proxy),
and `X-Real-IP` only when the request came without `X-Forwarded-For`,
else the proxy.
An entry that is not an address, such as `unknown` or `192.0.2.10:443`,
makes the client the proxy itself, whatever `X-Real-IP` says:
a client can set that header, and a proxy that appends to
`X-Forwarded-For` may leave it in place.
Without the option the headers are ignored, since anyone can set them,
except on an open server (no users, or no store and no token),
which refuses a request that carries them:
it answers its local network only, and behind a proxy it needs the option
to know whether the client is on it.

```bash
massimo serve -trusted-proxies 127.0.0.1          # a proxy on the box
massimo serve -listen 192.168.1.5:8099 -trusted-proxies 192.168.1.2   # a proxy on another host
```

A proxy on another host gets the server's `-listen` on a LAN address,
and the firewall limits that port to the proxy,
since the token and the sessions travel in the clear between the two.

## A path prefix

Every URL of the web UI is relative to the page,
so the proxy may serve it at `https://name/` or under a prefix such as
`https://name/massimo/`,
as long as it strips the prefix before passing the request on,
and the page is opened with the trailing slash (the samples redirect `/massimo`
to `/massimo/`).
The server never learns the prefix, and needs no flag for it.

## The event stream

The web UI's live updates are server-sent events on `/api/v1/cubes/{sn}/events`,
a response that never ends.
The server sends a keep-alive comment every 30 seconds,
which satisfies the read timeouts of nginx (60 s) and of the NAS proxies,
and `X-Accel-Buffering: no`, which tells nginx not to hold the stream back;
Caddy streams `text/event-stream` by itself.
The proxy must speak HTTP/1.1 to the server with the `Connection` header cleared,
as the samples do.

## The `Host` header

The server refuses a write that another site's page sent:
see `docs/api-v1.md`, "Authentication".
A browser says where a request comes from in `Sec-Fetch-Site`,
which every browser since 2023 sends and a proxy passes as it is;
for an older browser the server compares the `Origin` with the `Host` the request
arrived with,
so the proxy passes the `Host` the browser used.
Caddy does that by itself; nginx needs `proxy_set_header Host $host`,
as its sample has.

## nginx

```nginx
server {
    listen 443 ssl;
    server_name massimo.example.net;
    ssl_certificate     /etc/letsencrypt/live/massimo.example.net/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/massimo.example.net/privkey.pem;
    add_header Strict-Transport-Security "max-age=31536000" always;

    location = /massimo { return 301 /massimo/; }
    location /massimo/ {                 # or "location / {" on a name of its own
        proxy_pass http://127.0.0.1:8099/;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

The trailing slash of `proxy_pass` strips the prefix.
On the box: `massimo serve -trusted-proxies 127.0.0.1`.

## Caddy

```caddyfile
massimo.example.net {
    redir /massimo /massimo/
    handle_path /massimo/* {
        reverse_proxy 127.0.0.1:8099
    }
}
```

Caddy gets its certificate from Let's Encrypt by itself,
sets `X-Forwarded-For` and streams the events;
`handle_path` strips the prefix.
On the box: `massimo serve -trusted-proxies 127.0.0.1`.

## A NAS

The reverse proxy of a Synology NAS (Control Panel → Login Portal →
Advanced → Reverse Proxy) or a QNAP (Network & Virtual Switch → Access
Services → Reverse Proxy) takes a source such as
`https://massimo.example.net:443` and the destination `http://localhost:8099`
when the server runs on the NAS, else its LAN address,
with the NAS's own certificate.
It passes `X-Forwarded-For` and `X-Real-IP`;
on the box: `-trusted-proxies` with the NAS's address.
A prefix needs the NAS proxy to strip it, which not every one does;
a name of its own (a subdomain) avoids the question.

## What the server does not need

`X-Forwarded-Proto` is unused:
the server sets no cookies and builds no absolute URLs,
and HSTS comes from the proxy.
A second listener is not needed either: one `-listen` serves the proxy,
plain HTTP on the box.
