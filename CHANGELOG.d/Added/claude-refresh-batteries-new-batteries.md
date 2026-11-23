- `POST /api/v1/cubes/{sn}/devices/{rf}/battery` marks new batteries:
  a weak or low device whose flag is off is ok at once, the log says
  who, and a restart keeps it ok; for operators and guests too.
