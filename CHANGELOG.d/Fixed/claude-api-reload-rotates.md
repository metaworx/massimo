- serve: **Reload now** and `POST /api/v1/reload` rotate the log file and
  checkpoint the store where the settings file does not parse or cannot
  be read too, as SIGHUP does; before, only a good read did.
