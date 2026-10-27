# File Formats Specification — Cyberspace Refactor

Cyberspace Refactor supports both the original 1995 INI-like text formats (`.ltg`, `.dek`, `.npc`) encoded in Windows-1252 / Latin-1 / UTF-8 and modern structured `.json` representations.

---

## 1. Local Telecommunications Grid (`.ltg`)

### `[Global Settings]` Section
```ini
[Global Settings]
Description=First Run
Start=0,2,none
Shutdown=0,0
ImagePath=IMAGES
```
- `Description`: Display name of the Matrix grid.
- `Start=<entryNode>,<startRange>,<alertState>`:
  - `<entryNode>`: Integer node ID (`0..100`).
  - `<startRange>`: `2` (Observation), `1` (Sensor), or `0` (Contact).
  - `<alertState>`: `none`, `passive`, or `active`.
- `Shutdown=<tics>,<emergencyPct>`: Countdown tics and emergency trigger percentage.
- `ImagePath`: Optional custom sprite directory (legacy absolute paths like `C:\CYBER\IMAGES` are safely normalized to their final folder name).

### `[Node <ID>]` Sections (`[Node 0]` .. `[Node 100]`)
```ini
[Node 2]
Type=DS, Green, 4
Pos=3, 2
Links=0, 1, 3
Mesg=Corporate Accounting Datastore
Files=Payroll_2054, 40, 2500, secretpw,
      Audit_Log, 20, 0
ICE1=White, Access, 4, ever, immobile
ICE2=Black, Killer[M], 5, active, mobile
```
- `Type=<Kind>, <Color>, <Rating>` (or `Type=LNK, <TargetFile.ltg>, <TargetEntryID>`):
  - `<Kind>`: `SAN`, `SPU`, `DS`, `SM`, `IOP`, `CPU`, `DLJ`, `LNK`, or `MAT`.
  - `<Color>`: `Blue`, `Green`, `Orange`, `Red`, `Purple`, or `Dark` (matched by first letter, case-insensitive).
  - `<Rating>`: Node security rating (`1..20`).
- `Pos=<X>, <Y>`: Grid coordinates used for layout and clockwise exit sorting (starting at 6 o'clock).
- `Links=<id1>, <id2>, ...`: Connected node IDs (up to 30 per node).
- `Mesg=<text>`: Optional node message (supports multiline continuation lines).
- `Files=<Name>, <SizeMp>, <ValueNuyen>[, <Password>]`: Repeated tuple list (4 tokens per file when password-protected, or 3 tokens; supports multiline continuation).
- `ICE1` .. `ICE101`: `<Class>, <Type>[<Stage>], <Rating>[, <Trigger>, <Mobility>]`.

---

## 2. Cyberdeck Configuration (`.dek`)

```ini
[Persona]
Model=Fuchi Cyber-4
MPCP=6+0-0-0
Bod=4+0-0-0
Evasion=4+0-0-0
Masking=5+0-0-0
Sensor=5+0-0-0

[Technical Parameters]
Hardening=3+0-0-0
Active Memory=200
Storage Memory=500
Load/Unload Speed=50
I/O Speed=30
Response Increase=1
Satellite Uplink=No
Pure Cyberterminal=No
Reality Filter=No
Display=2

[Utilities]
1=Analyze, 5, 75
2=Attack[M], 5, 75
3=Sleaze(Access), 5, 75
```
- Values formatted as `6+0-0-0` encode `<base>+<bonus>-<damage>-<penalty>`; the effective rating and MPCP damage are extracted while preserving the raw string for round-trip fidelity.

---

## 3. Decker Character Sheet (`.npc`)

```ini
[Natural Attributes]
Name=Heavy Hitter
Body=4
Quickness=6
Strength=3
Charisma=4
Intelligence=6
Willpower=6
Reaction=6+1D6,0,0

[Skills]
Computer=6
Etiquette (Matrix)=4

[Pools]
Karma=5
Hacking Pool=14
Teamwork Pool=0
```
- `.npc` files may also embed `[Persona]`, `[Technical Parameters]`, and `[Utilities]` sections, allowing them to be loaded both as a Decker and as a Cyberdeck.

---

## 4. Modern JSON Formats (`.json`)

All three domain types (`Matrix`, `Deck`, `Decker`) serialize to and from human-readable, schema-validated JSON via [`internal/format/json.go`](../internal/format/json.go) and can be batch-converted from the **Settings & About** tab into `assets/JSON/`.
