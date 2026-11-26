- `massimo install -acme-dns-hook` refuses a relative path and asks for
  the absolute one, where it checked the hook where install ran, and
  the service, which runs elsewhere, could not find it.
