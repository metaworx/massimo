- `massimo install` hands the files it wrote to the service user with
  `chown -h`: a link the service put in their place no longer has root
  give it the link's target.
