- `massimo store downgrade -to <N>` takes the store back for an older
  binary by the migrations' down parts; `massimo update`'s rollback does
  it where the store is too new for the binary it puts back.
