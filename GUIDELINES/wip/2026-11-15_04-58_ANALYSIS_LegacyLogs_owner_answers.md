# Your answers on D134–D147, weighed

2026-11-15, 04:58 UTC. This is an EVAL of your reply to the TL;DR gate for AP LegacyLogs.
It follows these files:
- `2026-11-14_21-13_ANALYSIS_LegacyLogs_protocol_findings.md`;
- `2026-11-15_01-41_ANALYSIS_LegacyLogs_week_programme_test.md` (T130);
- `2026-11-15_03-24_ANALYSIS_LegacyLogs_valid_bit_test.md` (T131).

The new figures were read from breno's store and backups, read-only.
**Nothing was sent to any Cube.**
Cubes are named by their IP suffix and devices by type and room id: no names, serials or addresses.
Times are in UTC.

## TL;DR

- **Settled:** D134-A, D135-B, D136-A, D138-A, D139-A, D141-A, D142-A, D143-A with a setting, and D147-A.
  - **Still open:** D140, which your reply did not name (A recommended).
- **D137: how long from flickering to steady.** No device has gone from one to the other yet.
  - Massimo has kept states for 3.3 days.
  - Both flickering radiator thermostats have flickered all that time and still do.
  - The steady ones were steady from the start, or went steady with no flicker before.
- **Your two levels fit the data.**
  - All three flickering radiator thermostats showed the flag in Saturday's decalcification, the week's longest valve move.
  - For the 3.3 days, the decalcification was the only valve move they made.
  - So it is the test that can clear the amber warning.
  - The rule is in section 2, and the notification question is the new D149.
- **D138: a test, T134,** after Block 2 (D150).
  - The question is whether one room-flagged link to one device reaches every radiator thermostat of the room.
  - The method works: a radiator thermostat shows a change made at its wall thermostat's display in the same poll (.22 room 4, 01:47 and 01:50).
- **D102** is the transmit-error topic of the session "Investigate the MAX! Cube's transmit error on hardware" (section 4).
- **The discovery hint:**
  - I had left it out because it was seen once, at setup, and the docs have it.
  - Weighed again, it belongs in the plan, since a first-time user is stuck exactly there (new D148).

## 1. What is settled

| Decision | Settled | What it means for the plan |
|---|---|---|
| D134 | A | 0 free slots read as 50 pending once the budget is known |
| D135 | B | build "not confirmed until each member shows it" here, after D110 (D102's session) is on main; see section 7 |
| D136 | A | `stale_after` becomes the radiator thermostats' threshold (default 50 h); a new `stale_after_wall` (24 h) is added; both through the API only, as `stale_after` is today; see section 7 |
| D137 | C, yours, to be weighed | section 2 |
| D138 | A | 0x23 first; one room-flagged link per radiator thermostat until T134 shows otherwise (section 3) |
| D139 | A | the original's second telegram (0x10, padded) |
| D140 | open | A recommended: show an out-of-date entry's last values, marked, with their age |
| D141 | A | the docs block in this plan, off D102's paragraphs (section 4) |
| D142 | A | comments and names in one commit; G12 tested on its own; the discovery hint now D148 (section 5) |
| D143 | A, with a setting | re-read in 10-minute steps, the step set in Maintenance (section 6) |
| D147 | A | a Warn in the log when a wall thermostat comes back on another mode |

## 2. D137: the battery flag, measured

### What breno holds

Every device that ever showed the flag, from 11-11 19:36 (the first state rows of the five Cubes, and the first backup with an L line) to 11-15 03:50:

| Device | The flag | Episodes | Length |
|---|---|---|---|
| .25 rad r4 | flickering since 11-11 21:35 at the latest | 10: 1 on 11-11, 2 on 11-12, 1 on 11-13, 6 on 11-14; none since 19:29 | 1–4 min |
| .21 rad r1, the first | flickering since 11-11 22:19 at the latest | 4, about one a day, the last 11-15 01:51 | 2–4 min |
| .21 rad r1, the second | once, 11-14 11:04 | 1 | 1 min |
| .24 rad r1 | steady since 11-11 19:36 at the latest | still on at its daily reports (11-12, 11-13) | ≥ 80 h |
| .21 wall r1 | clear for 2¼ days of 3-minute reports, then steady from 11-14 01:49 | no flicker before | ≥ 26 h |
| .21 wall r2 | in its last entry since 11-11 at the latest; the device has not been heard since | — | presumably flat |

**February** (the original software's logs, another host) showed the same thing.
- Five devices had the flag.
- All were steady while watched, for 4 to 7 days, until a battery change or a Cube restart.
- None flickered.

### Your question: how long from flickering to steady

- **No device was seen going from flickering to steady.**
  - Both flickering radiator thermostats have flickered for at least 3.3 days and still do.
  - .25 rad r4 flickered more often on 11-14 than before, and not at all since.
- **The steady ones never flickered first.**
  - .24 rad r1 was already steady at the start.
  - .21 wall r1 went steady with no flicker before, between two of its reports.
- The span is therefore unknown, and longer than 3.3 days.
- The second look at breno around 11-22, planned for the duty readings (T133), can measure it too.

### What the pattern shows

- **Each flicker is a report out of turn, and so is its end.**
  - These radiator thermostats are idle: manual at 5 °C with the valve at 0 %, or auto with the valve at 100 % in a room at 9–10 °C.
  - Such a device otherwise reports once a day at noon (K2).
  - Each episode starts with a report carrying its measured temperature and ends 1–4 minutes later with another message.
  - The device seems to transmit when its battery state changes, both ways (inferred).
- **The decalcification shows it.**
  - On Saturday 11-14, 11:03–11:05 (12:03–12:05 local), all three flickering radiator thermostats reported with the flag.
  - That is the decalcification hour of every device here.
  - They were the only radiator thermostats to report in that hour; on their decalcification day the others do not (K2).
  - The second of .21 room 1 has flickered only then.
  - The decalcification drives the valve over its whole travel, the week's heaviest load.
- **The decalcification is the only long move they make.**
  - Their valves did not move at all in the 3.3 days.
  - So for them the "long valve move" you name is the decalcification, once a week.
- **Wall thermostats have no motor, and they do not flicker.**

### Your option C, worked out

- **Amber, "battery weak":** the flag seen at all.
- **Red, "battery low":** the flag held for 15 minutes.
  - The longest flicker of the 15 was 4 minutes.
  - A wall thermostat reports every 3 minutes.
- **Amber clears** when a long valve move passes without the flag.
  - The device's next decalcification counts, its day and hour taken from its C line (`internal/protocol/cube/config.go:217-218`). It has passed 15 minutes after the hour.
  - So does a valve move of 50 points or more seen in L, with no flag within 10 minutes.
  - A wall thermostat, which has no valve, clears after 24 hours without the flag.
- **Red clears** when the flag has been off for an hour.
  - A steady flag does not go away by itself; new batteries make it go.
  - It clears straight to none, not to amber.
- **A restart keeps the levels.** serve rebuilds them from the log's state rows when it starts.
  - The log is kept for 30 days by default, so no migration is needed.
  - With `log.keep_days` below 8, a restart can lose an amber level.
- **API:** `battery: "ok" | "weak" | "low"` beside `battery_low`, which stays the bit as now; `docs/api-v1.md` and the OpenAPI file change with it.
- **UI:**
  - The tile's battery badge is amber (`tone-warn`, as it is for every flag today, `tiles.js:396`) for weak and red (`tone-bad`) for low.
  - One new key, `badge.batteryWeak`, in all six languages.
- This is a rule in Massimo, not a delay each user sets: the notification settings refuse `after_min` for `battery_low` today (`rules_test.go:173`).

### D149. Notifications for the two levels (new)

- **A (recommended).** One notification when a device turns amber and one when it turns red, each with its own text, under the existing kind `battery_low`.
  - Nothing is sent while it flickers.
  - Users keep one switch, as today, and get at most two notifications per set of batteries.
- **B.** A second kind, `battery_weak`, for amber, so that each user can turn it off.
  - It costs a row in the notification settings and its six translations.
- **C.** Red only: amber shows on the tile and in the log.

## 3. D138: the link test T134

**The question:** does one room-flagged 0x20 or 0x21 to one radiator thermostat (the "anchor") reach every member of the room?
The original software 1.4.6 sends it that way (`AssociateNewDeviceToRoom` and `DisassociateDeviceFromRoom`, inferred from the bytecode).
Massimo sends one to each radiator thermostat (`internal/cubeio/move.go:30-34`).
If the room flag reaches every member, a move needs one link radiogram fewer for each further radiator thermostat.

**How to see it.** The Cube cannot read links back, but their effect shows in serve's store.
Tonight, a change at .22 room 4's wall thermostat display showed on its radiator thermostat in the same poll:
- the mode at 01:47:15;
- the setpoint at 01:50:15.

A radiator thermostat follows a wall thermostat only while it has it as a partner.

**The room:** .25 room 5, with two radiator thermostats and one wall thermostat.
- It is the smallest room with two radiator thermostats.
- .25 room 1 has four: three members reached by the flag instead of one, for about four radiograms more (D150-B).
- .21 room 1 has two weak batteries.

**The steps.** The wall thermostat is moved out of its room and back in, by radio only. No `m:` is sent, so the Cube's room list stays as it is.

0. **Guards, as in T130:** every member valid and without the error flag; the duty cycle at most 60 %; at least 10 slots free.
1. **Baseline.** You turn the wall thermostat's wheel by half a degree. Both radiator thermostats should follow in the same poll, which proves the method.
2. **Out, in the original's form:**
   - `z:` to room 5;
   - 0x23 to the wall thermostat;
   - a 0x21 to it for each radiator thermostat;
   - one room-flagged 0x21 to the anchor, the first radiator thermostat, naming the wall thermostat.
3. **You turn the wheel.** Neither radiator thermostat should follow. If the second one does, the room flag did not reach it.
4. **In, in the original's form:**
   - `z:`;
   - one room-flagged 0x20 to the anchor;
   - a 0x20 to the wall thermostat for each radiator thermostat;
   - 0x22 (room 5) to the wall thermostat.
5. **You turn the wheel.** Both should follow, which shows that the room flag reaches every member. Then the wheel goes back.
6. **Repair, if a member missed a step:** Massimo's per-member form, one room-flagged 0x20 or 0x21 to that radiator thermostat, as `move.go` sends it today.

**Cost and risk:**
- **Radio:** 2 wakeups and 8 radiograms, plus up to 2 for a repair. That is about 5 to 10 % of .25's duty cycle, judging by T130, where 2 wakeups and 3 telegrams cost 3 %.
- **The wall thermostat's group:** between steps 2 and 4, a few minutes, room commands from the Cube do not reach it.
- **.25's history:** it is the Cube whose room 6 had every member flagged (D102), and the guards check for that.

**When:** after Block 2 (0x23 first) is built, as you said.
A script, `GUIDELINES/temp/legacy/t134_links.py`, is written then, on the pattern of `t130_week.py`.
Nothing goes to the Cube without your `EXEC t134` and your walk to the room.

### D150. T134 (new)

- **A (recommended).** In .25 room 5, after Block 2, under a gate of its own. It is the cheapest room that still answers the question.
- **B.** In .25 room 1, with four radiator thermostats: stronger evidence, about four radiograms more.
- **C.** No test: Massimo keeps one link per radiator thermostat, which is D138-A as it stands.

## 4. D141: who D102 is

- **D102** is a decision of the session "Investigate the MAX! Cube's transmit error on hardware". It took D100 to D117.
  - Its topic is the Cube's transmit-error flag on a device, byte 5's 0x08.
  - It also covers the `r:` that resets the flag, free of radio cost (tested on 2026-11-14), and room commands that are lost when every member of the room is flagged.
  - Its analyses are its own and not on main.
- **Its D110** is where it overlaps this plan: the queue resets the room's flagged members before a room command, as the original software does.
  - D135-B builds on it.
- That session has registered no plan yet, and it has not been told about the four overlaps (section 6 of the analysis of 2026-11-14).
  - The overlaps are its byte-4 statement, the 0x81 attribution, K1 as evidence for D110, and the naming of the flag.
  - `EXEC tell` sends them.
- **Under D141-A** its docs PR and ours touch different paragraphs of `docs/cube-protocol.md`.
  - Ours takes the H line, the valid bit, the reports, the week programme and the costs.
  - Its PR takes the error flag, `r:` and the FrameType list.

## 5. D142: why I left the discovery hint out

**What happens.** A Cube with DHCP on and no lease answers discovery from its fallback address, 192.168.0.222/16, and refuses TCP from another subnet.
It was seen on 2026-02-12, at the setup of a Cube on another host.

**Why I left it out:**
- the fallback is in the docs already (`docs/cube-protocol.md:639`);
- `massimo netconfig -sn <serial> -broadcast -dhcp` is the way back (`:695`);
- it was seen once;
- a hint needs a check against the host's networks, a text in six languages and a test.

**Weighed again,** that is the wrong way round.
- A first-time user gets stuck exactly there: the Cube is found, then "does not answer at its addresses" (`connect.unreachable`), with no reason given.
- Nothing else in Massimo tells them why.

### D148. A found Cube in another network (new)

- **A (recommended).** When discovery finds a Cube whose address lies in none of this host's networks, the found Cube says so.
  - It shows the Cube's address.
  - It says that a Cube without a DHCP lease uses 192.168.0.222.
  - It gives the `netconfig` line that sets it.
  - The check is generic, so a wrong static address is caught too.
  - It needs:
    - a Go check against the host's interface addresses;
    - one field on the found Cube in the API;
    - one text in six languages;
    - a test.
  - It is a block of its own.
  - In WSL 2's NAT mode discovery does not work in the first place, so nothing gets worse there.
- **B.** Leave it out, as I first advised: it happens once, at setup, and the docs have it.

## 6. D143: the hold, set in System settings

**Where:** in Maintenance, beside the three Cube intervals that CubePoll adds (AP CubePoll, Block 2).
It works like them:
- a choice of steps: 5, 10, 15, 30 or 60 minutes, 10 by default;
- applied at the next restart;
- read-only, with `fixed_by`, where a flag decides.

**The label:** "Re-read the radio budget after a hold every …", with a line on what it does.

**The threshold** of 95 % (`MaxDutyPct`) stays as it is.

**The order:** this needs CubePoll's Block 2 on main.
D134's half of the budget block can go first; the setting follows CubePoll.

## 7. D135-B and D136-A in the plan

- **D135-B** is a block of its own, built after D110 is on main.
  If D102's session does not take D110 up, the plan's gate asks you then.
  Until then, the queue's "acknowledged" stays what it is.
- **D136-A:**
  - `stale_after` exists only in the API today (`internal/server/settings_api.go`), with no UI field. The new `stale_after_wall` follows it, so it needs no UI and no translations.
  - The `-stale-after` flag, which is for tests, sets both.
  - CubePoll lists that flag in `fixed_by`, so the two plans meet in `settings_api.go`, and a merge settles it.

## 8. Open decisions

- **D140. Out-of-date entries**
  - **A (recommended).** Show their last values, marked "out of date", with their age.
    A silent wall thermostat's values are minutes old (T131).
    .21 wall r2's frozen entry still carries its battery flag, which Massimo now drops.
  - **B.** Leave it.
- **D148. A found Cube in another network:** section 5, A recommended.
- **D149. Notifications for the two battery levels:** section 2, A recommended.
- **D150. T134:** section 3, A recommended.

## 9. A correction to T131's timeline

T131's file says room 4 was set to manual 5.5 °C "through serve's queue (02:18:48)".
- **What serve's log shows at 02:18:48 on .22** is a pairing window of 60 s and its commands, not a setpoint command.
- **Where the change came from:** room 4's 5.5 °C (02:17:15) and manual mode (02:19:15) showed on its wall thermostat and its radiator thermostat in the same poll.
  That is what a change at the wall thermostat's display does (01:47:15, 01:50:15).
  So they were your changes at the display, as you said.

## 10. How it was read

All in `GUIDELINES/temp/legacy/`:
- `breno_batttl.py`, `breno_battbak.py`, `breno_battwall.py`: the battery episodes, the backups' L lines, and Saturday's decalcification hour;
- `breno_rooms.py`: the rooms with two or more radiator thermostats, as counts by type;
- `breno_follow.py`: .22 rooms 3 and 4, 01:40–02:25;
- `battfeb.py`: the battery bit in the February and November logs.

The breno scripts ran as `sudo -n python3 -I -` against `file:/var/lib/massimo/massimo.db?mode=ro`.
