- A Cube's proxy listener that cannot listen, on an address the host
  does not carry yet or a port in use, is tried again every 30 s at
  most, instead of staying down until serve restarts.
