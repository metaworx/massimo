- `serve` rotates its `-log` file itself, on every platform: at 10M, at a
  change of its format and on a reload; logrotate's file is gone, and the
  Mac's plist passes `-log`, stderr to `serve.stderr.log`.
