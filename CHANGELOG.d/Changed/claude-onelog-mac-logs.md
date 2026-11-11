- Homebrew's service passes `-log`, so `serve` writes and rotates
  `var/log/massimo/serve.log` in brew's prefix; `var/log/massimo.log`
  keeps only what it prints before its log.
