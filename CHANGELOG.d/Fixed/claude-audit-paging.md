- `GET /api/v1/audit`, paged with `before`, no longer skips or repeats
  rows written after newer ones, as a slow request's or a minute's
  refused requests' are: a page is ordered by time, then by id.
