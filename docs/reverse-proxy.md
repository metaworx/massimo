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
Without the option the headers are ignored, since anyone can set them.

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
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

The trailing slash of `proxy_pass` strips the prefix.
On the box: `massimo serve -trusted-proxies 127.0.0.1`.

### A Raspberry Pi with the Debian package and a DNS-validated certificate

Tried on a Raspberry Pi 5 with Raspberry Pi OS Lite 64-bit (Debian 13),
for a name that resolves only in the site's own DNS,
so Let's Encrypt validates through the DNS provider's API (DNS-01)
and the box needs no port open to the internet.

1. Massimo from the apt repository (packaging/README.md, "The apt repository"),
   nginx and acme.sh from Debian:
   `sudo apt install massimo nginx acme.sh`.
2. Massimo on localhost only, trusting nginx's headers:
   `sudo massimo install -listen 127.0.0.1:8099`
   and `MASSIMO_TRUSTED_PROXIES=127.0.0.1` in `/etc/massimo/env`.
   Create the first owner before nginx forwards the LAN to it,
   since an empty store leaves the API open:
   `cd / && sudo -u massimo massimo -env /etc/massimo/env -data /var/lib/massimo user add <name> -role owner`.
3. The certificate, here with Infomaniak (`dns_infomaniak`, token in
   `INFOMANIAK_API_TOKEN`; for Cloudflare `dns_cf` with `CF_Token`).
   acme.sh defaults to ZeroSSL, so name Let's Encrypt;
   `--server letsencrypt_test` first is cheap, a failed real order counts
   against the rate limits:

   ```bash
   sudo install -d -m 0700 /etc/acme.sh /etc/nginx/ssl
   A="--home /etc/acme.sh --config-home /etc/acme.sh --cert-home /etc/acme.sh/certs"
   sudo INFOMANIAK_API_TOKEN=… acme.sh $A --server letsencrypt --issue \
       --dns dns_infomaniak -d massimo.example.net --keylength ec-256 \
       --accountemail certificates@example.net
   sudo acme.sh $A --install-cert -d massimo.example.net --ecc \
       --fullchain-file /etc/nginx/ssl/massimo.example.net.fullchain.pem \
       --key-file /etc/nginx/ssl/massimo.example.net.key.pem \
       --reloadcmd "systemctl reload nginx"
   sudo chmod 600 /etc/acme.sh/account.conf
   ```

   acme.sh keeps the token in `account.conf` for the renewals,
   but writes that file with mode 644 (the 0700 directory protects it;
   the `chmod` makes it explicit).
   Debian's acme.sh brings no timer:
   a oneshot service running `acme.sh --cron` with the same three
   directory options, and a timer with `OnCalendar=daily`,
   `RandomizedDelaySec=1h` and `Persistent=true`, renew 30 days before the end
   and run the reload command.
   Debian 13's lego (4.9.1) fails with Infomaniak
   (`expected record, got: <id>`, the record is created but not removed),
   since Infomaniak's API now answers a new record with its id;
   acme.sh's hook only checks for success.
4. One nginx site with the name of its own, the server's own `location /`
   instead of the prefix above, port 80 redirecting to it,
   and Debian's `sites-enabled/default` removed so that the box's address
   no longer shows nginx's welcome page:

   ```nginx
   server {
       listen 80 default_server;
       listen [::]:80 default_server;
       server_name massimo.example.net;
       return 301 https://massimo.example.net$request_uri;
   }
   server {
       listen 443 ssl default_server;
       listen [::]:443 ssl default_server;
       http2 on;
       server_name massimo.example.net;
       ssl_certificate     /etc/nginx/ssl/massimo.example.net.fullchain.pem;
       ssl_certificate_key /etc/nginx/ssl/massimo.example.net.key.pem;
       ssl_protocols TLSv1.2 TLSv1.3;
       add_header Strict-Transport-Security "max-age=31536000" always;
       location / {
           proxy_pass http://127.0.0.1:8099;
           proxy_http_version 1.1;
           proxy_set_header Connection "";
           proxy_set_header Host $host;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Real-IP $remote_addr;
       }
   }
   ```

To check from another machine before the site's DNS has the name,
`curl --resolve massimo.example.net:443:<the box's address> https://massimo.example.net/healthz`;
one failed `POST /api/v1/login` must appear in the log with that machine's
address as `remote`, not 127.0.0.1.
The Cubes' LAN proxy (port 62910 and discovery) does not go through nginx:
`MASSIMO_CUBE_<SN>_PROXY` names an address of the box as before.

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
