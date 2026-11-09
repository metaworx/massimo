> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# The apt sources

```bash
curl -fsSLo /tmp/massimo-archive-keyring.deb https://massimo.metaworx.ch/apt/massimo-archive-keyring.deb
sudo apt install /tmp/massimo-archive-keyring.deb
sudo tee /etc/apt/sources.list.d/massimo.sources <<'EOF'
Types: deb
URIs: https://massimo.metaworx.ch/apt
Suites: alpha
Components: main
Signed-By: /usr/share/keyrings/massimo-archive-keyring.gpg
EOF
sudo apt update && sudo apt install massimo
```
