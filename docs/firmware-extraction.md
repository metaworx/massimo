# Reading the Cube's firmware over JTAG

Why: the portal reply format, the firmware update encryption and the
remaining unknown bytes of the LAN protocol are all decided by code
that only exists inside the Cube. The update image the original
software ships (`firmware.enc` in its jars) is encrypted, 1170 frames
of 2+16+256 bytes, and the key lives in the Cube's bootloader. The only
way to that key is a dump of the Cube's flash. Nothing public exists:
no dump, no datasheet of the Cube, no JTAG write-up (research
2026-11-04, see portal-protocol.md).

Everything below is read-only. No step erases or writes the flash. The
Cube used for this is Cube 1 (KEQ…), the one with the unreliable
thermostat, so Cube 2 keeps running the house.

## Hardware facts

- MCU: Atmel **AT91SAM7X256**, ARM7TDMI, 256 KB internal flash at
  `0x00100000`, 64 KB SRAM at `0x00200000`, boot ROM (SAM-BA) at
  `0x00300000`. 3.3 V I/O. Radio: TI CC1101.
- JTAG header: **ST3**, the standard ARM 20-pin footprint, two rows of
  ten surface-mount pads at 2.54 mm pitch, unpopulated. Photo taken
  2026-11-04 (pads next to ST1/ST2 and J1, top edge of the board).
- Standard ARM 20-pin pinout: 1 VTref (3.3 V), 2 Vsupply, 3 nTRST,
  5 TDI, 7 TMS, 9 TCK, 11 RTCK, 13 TDO, 15 nSRST, 17 DBGRQ, 19 DBGACK;
  every even pin 4 to 20 is GND.
- Which end is pin 1 is **not yet known**. Find it with a multimeter,
  Cube powered: the row where every pad beeps to ground is the even
  row; on the other row the pad at 3.3 V at one end is pin 1.
- Not yet checked: whether the JTAG lines reach the chip without series
  resistors or unpopulated jumpers (photo of the component side around
  the MCU still to be taken), and whether the SAM7's security bit is
  set, which disables JTAG and SAM-BA. OpenOCD reports it, see below.
  If it is set, the only way in is a full erase, which we do not do.

## Adapter

An FT232H board (FTDI UM232H from Distrelec, or the Adafruit FT232H
breakout; both are the same chip, 3.3 V I/O, one MPSSE channel). On
the Adafruit board with the I2C mode switch, the switch must be off.

Wiring, five wires, nothing else; the Cube runs on its own power
supply and the USB goes only to the PC:

| FT232H pin (UM232H: AD0…, Adafruit: D0…) | ST3 pin | signal |
|---|---|---|
| AD0 / D0 | 9  | TCK |
| AD1 / D1 | 5  | TDI |
| AD2 / D2 | 13 | TDO |
| AD3 / D3 | 7  | TMS |
| GND      | any even pin | GND |

Solder a 2x10 pin header (2.54 mm, surface mount or a through-hole one
with the pins bent flat) or five wires straight onto the pads.

## Software

OpenOCD from the distribution (`sudo apt install openocd` on Mint).
Its interface file `interface/ftdi/um232h.cfg` describes exactly the
wiring above and works for the Adafruit board as well (same FT232H,
same pins, USB id 0403:6014). The target file
`target/at91sam7x256.cfg` ships with OpenOCD.

If the kernel's serial driver grabs the board (`/dev/ttyUSB0`
appears), OpenOCD still detaches it through libusb; if not,
`sudo rmmod ftdi_sio` first. Run as root or add a udev rule for
0403:6014.

### Step 1: does the chip answer

```bash
sudo openocd -f interface/ftdi/um232h.cfg -c "adapter speed 500" -f target/at91sam7x256.cfg
```

Expected in the output: `JTAG tap: at91sam7x256.cpu tap/device found: 0x3f0f0f0f`
(ARM7TDMI id code) and the target halting is not needed yet. Leave it
running and continue in a second terminal with `telnet localhost 4444`,
or stop it (Ctrl-C) and use the one-shot form below.

If no TAP is found: check pin 1, swap TDI/TDO (the two most common
mistakes), lower the speed to 100, confirm the ground wire. If the TAP
is found but the target cannot be halted or examined, the security bit
is probably set; `flash info 0` in step 2 says so.

### Step 2: security bit, then the dump

One-shot, from the directory where the dumps should land (keep that
directory out of git, the firmware is eQ-3's):

```bash
sudo openocd -f interface/ftdi/um232h.cfg -c "adapter speed 500" -f target/at91sam7x256.cfg \
  -c init -c "reset halt" -c "flash probe 0" -c "flash info 0" \
  -c "dump_image cube1-flash.bin 0x00100000 0x40000" \
  -c "dump_image cube1-sram.bin 0x00200000 0x10000" \
  -c "dump_image cube1-rom.bin 0x00300000 0x2000" \
  -c "arm reg" -c shutdown
```

- `flash info 0` prints the bank with its lock bits and the security
  bit (`at91sam7` driver). With the security bit set, stop here.
- `cube1-flash.bin` is the whole firmware including the bootloader,
  256 KB. `cube1-sram.bin` is the live RAM, worth having because keys
  are often expanded there. The ROM dump is Atmel's SAM-BA, of no
  interest beyond completeness; if the size is wrong the command just
  fails, the others are unaffected.
- `reset halt` restarts the chip and stops it at the reset vector, so
  the Cube is not running during the dump; power-cycle it afterwards.

A second dump after a power cycle should be byte-identical for the
flash; that is the check that the read was clean.

### Step 3: what to look for

1. `strings -n 6 cube1-flash.bin`: expect the portal URL, `ntp.homematic.com`,
   the LAN command letters, and the format strings of the H/S/A lines.
   Their addresses locate the LAN protocol handler.
2. The AES S-box (`63 7c 77 7b f2 6b 6f c5`) or its T-tables locate
   the AES code; the key for `firmware.enc` is a 16-byte constant near
   the update handler in the bootloader (first 8 to 16 KB of the flash,
   the reset vector at 0x00100000 jumps there). The frame layout of
   `firmware.enc` (2 + 16 + 256 bytes) suggests a 16-byte IV or MAC
   per 256-byte block; try AES-128-CBC with the candidate key and that
   IV on one frame, a decrypted frame is plainly ARM code or data.
3. The portal client: the code that builds the POST to `/cube` and the
   one that parses the reply decides which reply lines, if any, the
   Cube acts on. That answers the open question in portal-protocol.md.
4. Disassembly: Ghidra (free) with the ARM v4T little-endian loader,
   image base 0x00100000; `arm-none-eabi-objdump -D -b binary -m arm`
   for a quick look.

The dumps and anything derived from them stay out of the repository.
Findings that matter for interoperability (protocol facts, formats,
constants needed to talk to the Cube) go into portal-protocol.md as
usual; the firmware itself and its key do not.

## Fallbacks

- **Bootloader over UDP.** The Cube takes firmware updates over UDP
  port 23272 (opcodes seen: I, N, B, U, W, R, c, C, h, H; a Cube in
  bootloader mode answers with `eQ3MaxBl`). Nothing observed reads the
  flash back, and opcode B (enter bootloader) without an update ready
  is a risk. Only the read-only opcodes are worth a probe, and only
  after the JTAG route failed.
- **SAM-BA** through the boot ROM: same security-bit limit as JTAG,
  needs the TST pin sequence at power-up and a serial or USB
  connection. Last resort.
