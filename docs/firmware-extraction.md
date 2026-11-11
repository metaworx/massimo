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
  **Correction (2026-11-10):** pin 15 is *not* nSRST on this board —
  it reaches no MCU leg and sits at about 1 kΩ to 3.3 V,
  so it cannot hold the MCU in reset.
  The MCU reset (NRST, leg 57) is on test point MP2 instead, and even that does
  not reset the running MCU (URSTEN = 0).
  See "Attempt of 2026-11-10".
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

## Attempt of 2026-11-10: JTAG dead, DataFlash not yet dumped

A first full attempt on Cube 1, with the UM232H and OpenOCD 0.12.0 on a Mint box.
Outcome: the MCU's JTAG never answers and could not be forced to;
the DataFlash could not be powered enough to read in circuit.
Both routes are left where the hardware stops them, documented here so the next
session does not repeat the dead ends.

### The MCU's JTAG does not answer

The TAP never responds, at 500, 100 and 4 kHz alike:
`JTAG scan chain interrogation failed: all ones`,
then `sam7x256.cpu: IR capture error; saw 0x0f not 0x01`.
Every attempt reads back all ones, i.e. the chip shifts nothing out.

This is not a wiring fault.
The four JTAG signals (wires 1–4) were traced end to end, FT232H pin to MCU leg,
on Cube 1 (the 100-pin package, legs confirmed against the datasheet);
wire 5 is ground, wire 6 the later reset experiment:

| wire | colour | J-pin | pad | FT232H | ST3 pin | signal | MCU leg |
|-----:|--------|-------|-----|--------|--------:|--------|--------:|
| 1 | black  | J2-7  | AD0 | ADBUS0 / D0 | 9  | TCK | 79 |
| 2 | white  | J2-8  | AD1 | ADBUS1 / D1 | 5  | TDI | 51 |
| 3 | grey   | J2-9  | AD2 | ADBUS2 / D2 | 13 | TDO | 76 |
| 4 | violet | J2-10 | AD3 | ADBUS3 / D3 | 7  | TMS | 78 |
| 5 | —      | J2-6  | GND | GND | even pin | GND | 2/16/32/52/68/83/96 |
| 6 | — (+1 kΩ) | J1-13 | AC1 | ACBUS1 | MP2 ¹ | nSRST | 57 ² |

The UM232H pad is named as printed on the board; the J-pin follows its physical
header (J2 top to bottom: SLD 1, VIO 2, 3V3 3, PU2 4, PU1 5, GND 6, AD0 7 … AD7 14;
J1 top to bottom: GND 1, 5V0 2, USB 3, RST# 4, AC9 5 … AC1 13, AC0 14).
Note this is *not* OpenOCD's `um232h.cfg` numbering, which labels AD0 as J2-6.

¹ Wire 6 did not go to ST3: it went to test point MP2 (the open-drain reset
experiment); ST3 pin 15 is not nSRST on this board (see below).
² MP2 ties MCU leg 57 (NRST) and IC1 leg 3; pulling it low still does not reset
the running MCU.

The FT232H drives its pins (TCK 0 V at rest, TMS 3.3 V, VIO 3.3 V, USB 5 V),
and swapping TDI/TDO changes nothing.

### Why, most likely: Force NTRST

The SAM7X Debug Unit can hold its own TAP in reset in software,
through the register DBGU_FNR, bit FNTRST
(datasheet §27.4.8, "ICE Access Prevention":
"Writing the bit FNTRST … prevents any activity on the TAP controller").
FNTRST resets to 0, so the firmware has to set it on every boot.
That fits everything seen: all ones with good wiring,
and the floating 1.0 to 1.4 V on TDO measured on 2026-11-08
(a tri-stated output that is never enabled).

This is not the security bit.
The security bit (§9.4.4) only forbids access to the flash through the ICE
interface; the TAP would still return its id code.
We never get the id code, so the TAP itself is held in reset — Force NTRST,
not the security bit.

### The reset line does not reset the MCU

- **ST3 pin 15 is not nSRST on this board.**
  It reaches no MCU leg and sits at about 1 kΩ to 3.3 V.
  Driving OpenOCD's "srst" onto it pulls some other board net, not the MCU reset.
- **NRST is on test point MP2**,
  which ties MCU leg 57 (NRST) and IC1 leg 3 (the DataFlash RESET#) together.
- **Pulling MP2 low does not reset the running MCU.**
  The SAM7X performs a User Reset on NRST only when RSTC_MR bit URSTEN = 1,
  and its reset value is 0 (datasheet §14.3.3.2); the firmware leaves it 0.
  Through a 1 kΩ, MP2 was driven to about 0.3 V with no effect on the MCU.
- Because MP2 is the shared net, holding it low does hold the **DataFlash** in
  reset; once this stalled the boot (D1 blinked well past the usual 6 s),
  though it was not reproducible afterwards.

### The power-on race also failed

To catch the TAP in the window after reset, before the firmware sets FNTRST,
a Tcl loop (`catchtap`) scanned the id code continuously while the Cube was
power-cycled by hand.
No answer: 47 949 scans at 500 kHz, and none at 4 kHz.
The power cycle does reset the chip fully —
with only the FT232H attached, the 3.3 V rail (ST3 pin 1) falls to 0.47 V,
below the roughly 1.5 V power-on-reset threshold (datasheet §14.3.3.1) —
so the window exists, but is set too early to be caught this way.

**Conclusion: JTAG into the MCU flash is treated as closed.**
Beating FNTRST would need the reset held and the halt issued before release,
which requires a working external reset into the MCU — not available here.
The DataFlash is now the main route.

### DataFlash over the clip: attempted, not yet reading

IC1 is an AT45DB041D in SOIC-8; pin 1 is on the MCU side of the package.
Pinout confirmed, and wired to the FT232H's MPSSE in SPI mode
(colours as soldered on 2026-11-10):

| IC1 pin | function | FT232H | J-pin | pad | colour |
|--------:|----------|--------|-------|-----|--------|
| 1 | SI (MOSI) | ADBUS1 / D1 | J2-8  | AD1 | grey |
| 2 | SO (MISO) | ADBUS2 / D2 | J2-9  | AD2 | red |
| 3 | RESET#    | 3V3         | J2-3  | 3V3 | orange |
| 4 | GND       | GND         | J2-6  | GND | yellow |
| 5 | CS#       | ADBUS3 / D3 | J2-10 | AD3 | violet |
| 6 | SCK       | ADBUS0 / D0 | J2-7  | AD0 | blue |
| 7 | WP#       | 3V3         | J2-3  | 3V3 | green |
| 8 | VCC       | 3V3         | J2-3  | 3V3 | white |

The one UM232H 3V3 pad (J2-3) feeds IC1 pins 3, 7 and 8 together, and is also
jumpered to the UM232H's own VIO pad (J2-2) to power the FT232H's I/O bank.
IC1 pin 3 (RESET#) is the MP2 net; held high here so the flash runs.

`flashrom` (`ft2232_spi:type=232H`) initialises the FT232H cleanly but finds no
device: every id byte reads `0xFF`, at the 30 MHz default and at ~0.9 MHz
(`divisor=64`).
The cause is power.
IC1 pin 8 (VCC) ties to the board's whole 3.3 V rail,
so powering the flash through the clip tries to power the whole board
(MCU, CC1101, Ethernet PHY), which loads the rail down:
VCC at IC1.8 reached only about 1.1 V from the clip's 3V3,
and only 2.4 V with an extra external 3.3 V source added —
still below the AT45DB041D's 2.7 V minimum.
A clip-contact fault was also found and fixed on the way
(the FT232H ground and the board ground differed by 1.1 V until the GND and VCC
contacts were re-seated).

Note this corrects the DataFlash section below:
the MCU cannot simply be "held in reset" to free the bus on this board,
because NRST does not reset it and ST3 pin 15 is not NRST.

### What is left to try, next session

1. **Isolate IC1 from the board rail.**
   Lift pin 8 (VCC) off its pad and power the flash only from the clip,
   so the board no longer loads the 3.3 V rail; last resort, desolder IC1 and
   read it on a breakout.
2. Keep the SPI clock low (≈1 MHz, `divisor=64`) for the clip and jumper wires.
3. Read twice and `cmp` the two files; identical means a clean dump.
4. If flashrom still refuses the AT45 in page mode, read the pages by hand with
   the "continuous array read" opcode (see the DataFlash section).

### The key question is still open

`firmware.enc` carries a 256-byte payload per frame, exactly one AT45 page,
so the update is probably staged in IC1, with the per-device configuration and
the portal settings likely there too.
The AES key is either a constant in the MCU bootloader (now unreachable) or
written into IC1 near the MAC/KEY labels at the factory.
Reading IC1 is the way to settle which:
a 16-byte blob near the MAC region is the first candidate to try as the
AES-128-CBC key against one frame, with that frame's 16-byte field as the IV.

### Commands used

OpenOCD 0.12.0 on Mint ships the stock `interface/ftdi/um232h.cfg` and
`target/at91sam7x256.cfg`; only the id-code line differs from current OpenOCD —
0.12 wants `ftdi vid_pid`, not `adapter usb vid_pid`.

Step 1, does the chip answer (also tried with `adapter speed 100` and `4`):

```bash
sudo openocd -f interface/ftdi/um232h.cfg -c "adapter speed 500" -f target/at91sam7x256.cfg
```

A second terminal drives it:

```bash
telnet localhost 4444
```

The reset experiment used a custom open-drain interface file,
`~/um232h-srst-od.cfg`, putting nSRST on the FT232H's ACBUS1 (UM232H J1-13):

```tcl
# UM232H for JTAG, nSRST on AC1 (J1-13) as open drain:
# driven low when asserted, released (high-Z) otherwise. File as used.
adapter driver ftdi
ftdi vid_pid 0x0403 0x6014
ftdi layout_init 0xfdf8 0xfdfb
ftdi layout_signal nSRST -data 0x0200 -oe 0x0200
```

`-oe 0x0200` alone did nothing (the pin never drove); `-data 0x0200 -oe 0x0200`
is what actually pulled the line. In telnet:

```
reset_config srst_only separate srst_nogate   ;# plain "srst_only" gave: BUG: can't assert only SRST
adapter assert srst
jtag arp_init
adapter deassert srst
mdw 0xfffff248    ;# DBGU_FNR, bit 0 = FNTRST (read right after reset, expect 0)
mdw 0xfffffd44    ;# WDT_MR, watchdog
```

DataFlash over the clip:

```bash
sudo rmmod ftdi_sio                           # if /dev/ttyUSB0 grabbed the board
sudo pkill openocd                            # free the FT232H from a running OpenOCD
sudo flashrom -p ft2232_spi:type=232H,divisor=64 -V
```

`divisor=64` is ~0.9 MHz; flashrom's default (`divisor=2`) is 30 MHz SPI,
too fast for a clip on jumper wires.

### The race script (`~/catchtap.tcl`)

Loaded with `-f ~/catchtap.tcl`, then run from telnet as `catchtap 500 30`
(or `catchtap 4 30`): it scans the id code in a tight loop while the Cube is
power-cycled by hand, to catch the TAP in the moment after reset before the
firmware can set FNTRST, and halts the instant the TAP answers.
`0xe` is the ARM7TDMI IDCODE instruction (4-bit IR).

```tcl
# After a power-on, scan for the SAM7X IDCODE until it answers, then halt it.
proc catchtap {khz {seconds 30}} {
    adapter speed $khz
    set end [expr {[clock milliseconds] + $seconds * 1000}]
    set n 0
    while {[clock milliseconds] < $end} {
        incr n
        irscan sam7x256.cpu 0xe
        set id [drscan sam7x256.cpu 32 0]
        if {$id ne "ffffffff" && $id ne "00000000"} {
            echo "scan $n: IDCODE $id"
            adapter speed 4
            halt
            return
        }
    }
    echo "no answer in $n scans"
}
```

It returned `no answer in 47949 scans` at 500 kHz, and likewise nothing at 4 kHz.

### Key outputs

JTAG, every run and every speed:

```
Error: JTAG scan chain interrogation failed: all ones
Error: sam7x256.cpu: IR capture error; saw 0x0f not 0x01
Info : Embedded ICE version 15
Error: unknown EmbeddedICE version (comms ctrl: 0xffffffff)
target halted in ThumbEE state due to watchpoint, current mode: System
cpsr: 0xffffffff pc: 0xfffffff9
```

(The "target halted … ThumbEE … pc 0xfffffff9" line is garbage that follows the
failed scan, not a real halt.)

`reset_config` echoes — the second form is the one that let SRST assert alone:

```
srst_only srst_pulls_trst srst_gates_jtag srst_open_drain connect_deassert_srst
srst_only separate srst_nogate srst_open_drain connect_deassert_srst
```

flashrom: `No EEPROM/flash device found`, every id byte `0xff`
(`RDID byte 0 parity violation. compare_id: id1 0xff, id2 0xffff`),
at both 30 MHz and ~0.9 MHz. The FT232H itself initialised fine
(`Using device type FTDI FT232H channel A`, MPSSE clock set).

### Measurements

| point | reading | note |
|-------|---------|------|
| ST3 pin 1 (VTref), Cube powered | 3.3 V | the 3.3 V rail |
| ST3 pin 1, only FT232H on USB, Cube unplugged | 0.47 V | below the ~1.5 V POR threshold → the chip does fully reset |
| ST3 pin 9 TCK / pin 7 TMS, at rest, OpenOCD up | 0 V / 3.3 V | FT232H drives its pins |
| UM232H VIO / USB | 3.3 V / 5 V | adapter powered |
| ST3 pin 15, no wire 6 | 3.3 V | not nSRST; reaches no MCU leg |
| ST3 pin 15, wire 6 + 1 kΩ, on `assert srst` | 1.7 V | a strong (~1 kΩ) pull-up on that unknown net; no reset |
| FT232H J1-13 (ACBUS1), on `assert srst` | 0.05 V | the FT232H drives low fine |
| MP2, at rest / on `assert srst` | ~1.8 V / 0.3 V | NRST net (MCU leg 57 + IC1.3); no effect on the running MCU |
| IC1.8 VCC, clip 3V3 only | ~1.1 V | board loads the rail down |
| IC1.8 VCC, extra external 3V3 added | 2.4 V | still below the AT45DB041D 2.7 V minimum |
| D1 LED on boot | ~10 blinks over ~5–6 s, then steady | D2 lights on network traffic |

## The DataFlash (IC1, AT45DB041D)

Read it after the MCU flash, or first if JTAG is locked.
With the FT232H in SPI mode (same MPSSE pins: D0 SCK, D1 MOSI, D2 MISO,
D3 CS) and an SOIC-8 clip on IC1,
keep the MCU off the SPI bus
(not as simple as it sounds here: NRST does not reset a running SAM7X and
ST3 pin 15 is not NRST, so the MCU cannot just be held in reset —
isolate IC1 by lifting its VCC pin instead; see "Attempt of 2026-11-10"),
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

## Reference: pin maps and registers

Collected here so the next session has the numbers without re-deriving them.
The MCU and register facts are from the AT91SAM7X256/128 datasheet (doc6120);
the UM232H pads are read off the board silkscreen (photo 2026-11-10).

### UM232H / FT232H pins

Pads are named as printed on the board; the J-pin is its position on the header.
This differs from OpenOCD's `um232h.cfg`, which numbers AD0 as J2-6, not J2-7 —
trust the board, not the cfg comment.

| UM232H pad | J-pin | FT232H | use here |
|------------|-------|--------|----------|
| AD0 | J2-7  | ADBUS0 | TCK (JTAG) / SCK (SPI), "D0" |
| AD1 | J2-8  | ADBUS1 | TDI / MOSI, "D1" |
| AD2 | J2-9  | ADBUS2 | TDO / MISO, "D2" |
| AD3 | J2-10 | ADBUS3 | TMS / CS#, "D3" |
| 3V3 | J2-3  | —      | clip VCC (IC1 3/7/8) and VIO; the 3.3 V source |
| VIO | J2-2  | —      | FT232H I/O supply, jumpered to 3V3 |
| GND | J2-6  | GND    | ground (also GND at J1-1, and the large GND) |
| AC1 | J1-13 | ACBUS1 | /SRST → wire 6, the open-drain reset experiment |
| AC0 | J1-14 | ACBUS0 | /TRST (unused) |

Full header order, top to bottom:
J2 — SLD, VIO, 3V3, PU2, PU1, GND, AD0, AD1, AD2, AD3, AD4, AD5, AD6, AD7;
J1 — GND, 5V0, USB, RST#, AC9, AC8, AC7, AC6, AC5, AC4, AC3, AC2, AC1, AC0.

### AT91SAM7X256 pins (100-lead LQFP, datasheet Table 5-1)

The pin-1 dot is at a package corner; legs count anticlockwise
(bottom 1–25, right 26–50, top 51–75, left 76–100),
so the JTAG legs are the top-right and top-left corners.

| leg | signal | leg | signal |
|----:|--------|----:|--------|
| 51 | TDI | 57 | NRST |
| 76 | TDO | 58 | TST |
| 77 | JTAGSEL | 92 | ERASE — **never drive high: a 200 ms high wipes the flash** |
| 78 | TMS | 73 | PA27 / DRXD (Debug Unit RX) |
| 79 | TCK | 74 | PA28 / DTXD (Debug Unit TX) |
|    |     | 81 / 82 | PA0 / PA1 (fast-programming entry, with TST high) |

Supplies — GND: 2, 16, 32, 52, 68, 83, 96.
VDDIO: 17, 33, 48, 61, 84. VDDCORE: 15, 37, 62, 87. VDDFLASH: 95.
VDDIN 8, VDDOUT 7 (the on-chip 1.8 V regulator).

### AT45DB041D DataFlash (SOIC-8)

1 SI, 2 SO, 3 RESET#, 4 GND, 5 CS#, 6 SCK, 7 WP#, 8 VCC (2.7–3.6 V).
On this board IC1.3 (RESET#) joins the MCU's NRST at test point MP2,
and IC1.8 (VCC) ties to the board's 3.3 V rail.

### SAM7X registers and memory touched above

| address | register | note |
|---------|----------|------|
| 0xFFFFF248 | DBGU_FNR | bit 0 FNTRST; 1 = TAP held in reset (resets to 0 each boot) |
| 0xFFFFFD08 | RSTC_MR | bit 0 URSTEN; 0 (default, key 0xA5) = NRST does not reset the core |
| 0xFFFFFD44 | WDT_MR | watchdog mode (reset-init writes 0x00008000 to disable it) |
| 0x00100000 | internal flash | 256 KB (0x40000) |
| 0x00200000 | SRAM | 64 KB (0x10000) |
| 0x00300000 | boot ROM | Atmel SAM-BA |
