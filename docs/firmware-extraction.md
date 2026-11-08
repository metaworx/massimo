# Reading the Cube's firmware over JTAG

Why: the portal reply format,
the firmware update encryption and the remaining unknown bytes of the LAN
protocol are all decided by code that only exists inside the Cube.
The update image the original software ships (`firmware.enc` in its jars) is encrypted,
1170 frames of 2+16+256 bytes, and the key lives in the Cube's bootloader.
The only way to that key is a dump of the Cube's flash.
Nothing public exists: no dump, no datasheet of the Cube,
no JTAG write-up (research 2026-11-04, see portal-protocol.md).

Everything below is read-only.
No step erases or writes the flash.
The Cube used for this is Cube 1 (KEQ…), the one with the unreliable thermostat,
so Cube 2 keeps running the house.

## Hardware facts

- MCU: Atmel **AT91SAM7X256**, ARM7TDMI, 256 KB internal flash at `0x00100000`,
  64 KB SRAM at `0x00200000`, boot ROM (SAM-BA) at `0x00300000`. 3.3 V I/O.
  Radio: TI CC1101.
- The two sides of the board are called here the **ST3 side** (JTAG pads ST3,
  the 4-pin headers ST1 and ST2, J1,
  the serial-number barcode label) and the **chip side** (the MCU IC2,
  the DataFlash IC1, the MAC and KEY labels, the crystal).
- JTAG header: **ST3**, the standard ARM 20-pin footprint,
  two rows of ten surface-mount pads at 2.54 mm pitch, unpopulated.
  Photo taken 2026-11-04 (pads next to ST1/ST2 and J1, at the board edge).
- Standard ARM 20-pin pinout: 1 VTref (3.3 V), 2 Vsupply, 3 nTRST,
  5 TDI, 7 TMS, 9 TCK, 11 RTCK, 13 TDO, 15 nSRST, 17 DBGRQ, 19 DBGACK;
  every even pin 4 to 20 is GND.
- **Pin 1 was found on 2026-11-08** on Cube 1, with "Finding pin 1" below:
  it is the pad next to ST1, pin 19 the one towards J1.
  The odd row read, Cube powered, against ground:
  3.3 V on 1, 3, 5, 7, 9, 11 and 15;
  1.0 to 1.4 V, varying, on 13 (TDO);
  nothing on 17 and 19.
  Pin 2 (Vsupply) is open: nothing against ground and nothing against pin 1,
  the usual case, and the adapter takes no power from the Cube anyway.
  So VTref is there, TDI, TMS and TCK sit at their pull-ups,
  TDO floats as a tri-stated output does while the TAP is idle,
  and nSRST is on the header and pulled up,
  so pin 15 can hold the MCU in reset for the DataFlash read.
  The SAM7X has no nTRST and no RTCK pin,
  so 3 and 11 are the board's pull-ups or ties;
  OpenOCD's wiring uses neither.
  DBGRQ and DBGACK are unconnected, as usual.
- Chip side (photo 2026-11-04): the MCU is IC2,
  the large 100-pin square package marked ATMEL **AT91SAM7X256-AU** (date code 1322),
  18.432 MHz crystal at Q1.
  **IC1 is an Atmel AT45DB041D**,
  a 4 Mbit (512 KB) serial DataFlash in SOIC-8 next to the MAC label,
  so the Cube has external storage besides the MCU's 256 KB.
  The firmware update image's 256-byte frames match DataFlash pages (264 bytes
  with the extra 8), so the update is probably staged there, and the metadata,
  device configurations and portal settings likely live there too.
  The DataFlash can be read in two ways:
  through JTAG by running a small program in SRAM that reads it over SPI,
  or directly with the FT232H's SPI mode and an SOIC-8 test clip while the MCU
  is held in reset (NRST low), which keeps the MCU off the bus.
  A sticker labelled KEY sits below the MCU (the portal key,
  kept out of photos), a data-matrix label MAC next to IC1.
- Not yet checked: whether the JTAG lines reach the chip without series
  resistors or unpopulated jumpers (the traces from ST3 are not visible on either photo),
  and whether the SAM7's security bit is set, which disables JTAG and SAM-BA.
  OpenOCD reports it, see below.
  If it is set, the only way in is a full erase, which we do not do;
  the DataFlash is readable with the clip regardless.

## Finding pin 1

ST3 is the standard ARM 20-pin footprint, two rows of ten pads.
The odd pins run along one row, called row 1 here,
the even pins along the other, row 2;
1 and 2 sit at one end, 19 and 20 at the other.
Bold: the five wired to the FT232H (the table under "Adapter").

| signal, row 1 | pin, row 1 | pin, row 2 | signal, row 2 |
|---------------|-----------:|:-----------|---------------|
| VTref, 3.3 V  |  1 | 2  | Vsupply |
| nTRST         |  3 | 4  | GND     |
| **TDI**       |  * 5 | 6  | **GND** |
| **TMS**       |  * 7 | 8  | **GND** |
| **TCK**       |  * 9 | 10 | **GND** |
| RTCK          | 11 | 12 | **GND** |
| **TDO**       | ** 13 | 14 | **GND** |
| nSRST         | 15 | 16 | **GND** |
| DBGRQ         | 17 | 18 | **GND** |
| DBGACK        | 19 | 20 | **GND** |

Nothing on the board says which end is pin 1, so it is measured,
with a multimeter, on Cube 1 only, one pad at a time,
the probe tip held still so that it cannot bridge two pads,
and the meter set before the probe touches anything.
A slipped probe between VTref and GND restarts the Cube;
between Vsupply and a signal pin it can kill that pin.
Done on Cube 1 on 2026-11-08; the result is under "Hardware facts".

1. **The ground reference**, Cube unpowered, meter in continuity (beep) mode.
   Take the ground contact of the power socket;
   on a USB socket its metal shell beeps to it.
   Any second ground-looking point that beeps to it confirms it.
   Clip the black lead there for everything that follows.
2. **The even row**, still unpowered, still continuity.
   Touch each of the twenty pads.
   Expected: on one row nine pads beep and the tenth, at one end, does not.
   That row is row 2, the silent pad is pin 2 (Vsupply),
   and the pad beside it in the other row is pin 1.
   Row 1 has no grounded pad, or one: RTCK (11) is tied to ground on boards
   that do not use it; it never has nine.
   If both rows or neither have nine grounded pads,
   the footprint is not what the standard says:
   stop and follow the traces instead.
3. **Pin 1**, Cube powered, meter in DC volts (the 20 V range),
   black lead on the ground reference.
   Never continuity mode on a powered board, it drives current into the pins.
   Measure the two end pads of row 1:
   pin 1 (VTref) reads a steady 3.3 V,
   the other end (DBGACK, 19) reads 0 or drifts.
   Then pin 2 beside it: Vsupply reads 3.3 V, 5 V or floats,
   any of which is right; it must not read as ground.
   Pin 1 is confirmed when the 3.3 V end of row 1 is the end where step 2
   found the silent pad.
   If the two steps disagree, nothing is connected until they do.
4. **The signals**, optional, powered, DC volts:
   nTRST (3) and nSRST (15), if the board wires them,
   read 3.3 V while the Cube runs, pulled up by its reset circuit;
   TDI, TMS and TCK read 3.3 V or 0 as the chip's pull resistors decide
   and prove nothing about the orientation.
   A pad that reads neither 0 nor 3.3 V nor 5 V points at a series resistor
   or a shared net, which "Not yet checked" above is waiting for.

Mark pin 1 on the board (a dot of paint, or a photo with a note),
and write the result into "Hardware facts" above with the date.

## Adapter

An FT232H board (FTDI UM232H from Distrelec, or the Adafruit FT232H breakout;
both are the same chip, 3.3 V I/O, one MPSSE channel).
On the Adafruit board with the I2C mode switch, the switch must be off.

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

Parts (Distrelec, checked 2026-11-04): FT232H module UM232H;
SOIC-8 test clip for the DataFlash: Pomona 5250, Distrelec 140-45-803;
a 2x10 straight pin header 2.54 mm, for example Würth 61302021121 (through-hole,
pins bent flat onto the pads;
search Distrelec for "Stiftleiste 2x10 2.54" or take any equivalent);
female-to-female jumper wires 2.54 mm.

## Software

OpenOCD from the distribution (`sudo apt install openocd` on Mint).
Its interface file `interface/ftdi/um232h.cfg` describes exactly the wiring
above and works for the Adafruit board as well (same FT232H, same pins,
USB id 0403:6014).
The target file `target/at91sam7x256.cfg` ships with OpenOCD.

If the kernel's serial driver grabs the board (`/dev/ttyUSB0` appears),
OpenOCD still detaches it through libusb; if not, `sudo rmmod ftdi_sio` first.
Run as root or add a udev rule for 0403:6014.

### Step 1: does the chip answer

```bash
sudo openocd -f interface/ftdi/um232h.cfg -c "adapter speed 500" -f target/at91sam7x256.cfg
```

Expected in the output:
`JTAG tap: at91sam7x256.cpu tap/device found: 0x3f0f0f0f` (ARM7TDMI id code)
and the target halting is not needed yet.
Leave it running and continue in a second terminal with `telnet localhost 4444`,
or stop it (Ctrl-C) and use the one-shot form below.

If no TAP is found: check pin 1 ("Finding pin 1" above),
swap TDI/TDO (the two most common mistakes),
lower the speed to 100, confirm the ground wire.
If the TAP is found but the target cannot be halted or examined,
the security bit is probably set; `flash info 0` in step 2 says so.

### Step 2: security bit, then the dump

One-shot, from the directory where the dumps should land (keep that directory out of git,
the firmware is eQ-3's):

```bash
sudo openocd -f interface/ftdi/um232h.cfg -c "adapter speed 500" -f target/at91sam7x256.cfg \
  -c init -c "reset halt" -c "flash probe 0" -c "flash info 0" \
  -c "dump_image cube1-flash.bin 0x00100000 0x40000" \
  -c "dump_image cube1-sram.bin 0x00200000 0x10000" \
  -c "dump_image cube1-rom.bin 0x00300000 0x2000" \
  -c "arm reg" -c shutdown
```

- `flash info 0` prints the bank with its lock bits and the security bit
  (`at91sam7` driver).
  With the security bit set, stop here.
- `cube1-flash.bin` is the whole firmware including the bootloader, 256 KB.
  `cube1-sram.bin` is the live RAM,
  worth having because keys are often expanded there.
  The ROM dump is Atmel's SAM-BA, of no interest beyond completeness;
  if the size is wrong the command just fails, the others are unaffected.
- `reset halt` restarts the chip and stops it at the reset vector, so
  the Cube is not running during the dump; power-cycle it afterwards.

A second dump after a power cycle should be byte-identical for the flash;
that is the check that the read was clean.

### Step 3: what to look for

1. `strings -n 6 cube1-flash.bin`:
   expect the portal URL, `ntp.homematic.com`, the LAN command letters,
   and the format strings of the H/S/A lines.
   Their addresses locate the LAN protocol handler.
2. The AES S-box (`63 7c 77 7b f2 6b 6f c5`) or its T-tables locate the AES code;
   the key for `firmware.enc` is a 16-byte constant near the update handler in
   the bootloader (first 8 to 16 KB of the flash,
   the reset vector at 0x00100000 jumps there).
   The frame layout of `firmware.enc` (2 + 16 + 256 bytes) suggests a 16-byte IV
   or MAC per 256-byte block;
   try AES-128-CBC with the candidate key and that IV on one frame,
   a decrypted frame is plainly ARM code or data.
3. The portal client: the code that builds the POST to `/cube` and the one that
   parses the reply decides which reply lines, if any, the Cube acts on.
   That answers the open question in portal-protocol.md.
4. Disassembly: Ghidra (free) with the ARM v4T little-endian loader,
   image base 0x00100000;
   `arm-none-eabi-objdump -D -b binary -m arm` for a quick look.

The dumps and anything derived from them stay out of the repository.
Findings that matter for interoperability (protocol facts, formats,
constants needed to talk to the Cube) go into portal-protocol.md as usual;
the firmware itself and its key do not.

## The DataFlash (IC1, AT45DB041D)

Read it after the MCU flash, or first if JTAG is locked.
With the FT232H in SPI mode (same MPSSE pins: D0 SCK, D1 MOSI, D2 MISO,
D3 CS) and an SOIC-8 clip on IC1,
hold the MCU in reset (ST3 pin 15 nSRST to ground,
or NRST on the board) so it releases the bus,
then read 2048 pages of 264 bytes with the DataFlash "continuous array read"
command (0xE8 or 0x0B with the page/byte addressing of the AT45DB041D,
9 bits per byte address, 11 bits per page).
`flashrom` knows the AT45DB041D with the FT232H programmer
(`flashrom -p ft2232_spi:type=232H -r dataflash.bin`),
which is the quickest route if its wiring matches.
The power for the clip comes from the Cube;
do not back-feed 3.3 V from the adapter.

## Fallbacks

- **Bootloader over UDP.**
  The Cube takes firmware updates over UDP port 23272 (opcodes seen: I, N, B, U,
  W, R, c, C, h, H; a Cube in bootloader mode answers with `eQ3MaxBl`).
  Nothing observed reads the flash back,
  and opcode B (enter bootloader) without an update ready is a risk.
  Only the read-only opcodes are worth a probe,
  and only after the JTAG route failed.
- **SAM-BA** through the boot ROM: same security-bit limit as JTAG,
  needs the TST pin sequence at power-up and a serial or USB connection.
  Last resort.
