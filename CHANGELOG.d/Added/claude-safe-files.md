- `internal/safefile` writes a file whole, synced and renamed into place,
  keeping the replaced file's link, owner, mode and attributes, or in
  place where only the file is writable (its package comment).
