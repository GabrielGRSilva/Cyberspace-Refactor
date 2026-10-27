# Cyberspace Refactor

**Cyberspace Refactor** is a clean-room, modern **Golang** recreation of the classic *Cyberspace / Matrix Program Package* originally created by **Stefan Markgraf** (1995–1998) for *Shadowrun 2nd Edition (SR 2.01D) / Virtual Realities 1.0* (`CYBERSPC.EXE`, `MATED.EXE`, `CYBDECK.EXE`, and `DECKERED.EXE`).

Designed from the ground up for **Windows 10 and Windows 11**, it replaces the obsolete 16-bit Win16 binaries and legacy Python prototype with a single, self-contained, strictly offline native executable (`Cyberspace Refactor.exe`) featuring a modular architecture, full backward compatibility with all legacy `.ltg`, `.dek`, `.npc`, `.bmp`, and `.wav` files, native `.json` import/export, and hardened security aligned with **OWASP Top 10** and **NIST CSF 2.0**.

---

## ✨ Key Features

- **Unified Single-Executable Suite**: All four classic programs plus settings and batch tools live inside one native GUI window with instant tab switching:
  1. **Cyberspace Terminal (`CYBERSPC`)**: Real-time Matrix run simulator with Vidscreen (`.bmp` node backgrounds + white-keyed ICE sprites across `Standard`, `Advanced`, and `Sculpted` display modes), interactive fog-of-war Auto-Map, Net Navigator (`Forward`, `Backward`, clockwise exit cycling starting at 6 o'clock), System Operations pad (all 16 node-contextual SysOps), Active & Storage Memory management, Condition Monitor (`Deck MPCP`, `Stun`, `Physical` 10-box tracks), Hacking Pool & Karma Pool controls, manual/physical dice override, and `.txt` run log export.
  2. **Matrix Construction Set (`MATED`)**: Visual grid editor for `.ltg` and `.json` Local Telecommunications Grids. Create, move, and connect nodes (`SAN`, `SPU`, `DS`, `SM`, `IOP`, `CPU`, `DLJ`, `LNK`, `MAT`), configure security colors (`Blue`, `Green`, `Orange`, `Red`, `Purple`, `Dark`) and ratings, populate Datastore (`DS`) files with sizes, nuyen values, and passwords, install multi-IC defenses with triggers and mobility (`immobile`, `mobile`, `pmobile`, `amobile`), repair one-way dataline links, and validate against `NODELIM` / SR2 rules.
  3. **Cyberdeck Designer (`CYBDECK`)**: Build and validate `.dek` and `.json` cyberdecks. Edit Persona attributes (`MPCP`, `Bod`, `Evasion`, `Masking`, `Sensor`), hardware specs (`Hardening`, `Active/Storage Memory`, `Load Speed`, `I/O Speed`, `Response Increase`, `Satellite Uplink`, `Pure Cyberterminal`, `Reality Filter`), and Storage Utilities with automatic `Multiplier × Rating²` Megapulse calculation and `Compressor` support.
  4. **Decker Biomonitor (`DECKERED`)**: Create and edit `.npc` and `.json` decker characters, attributes, skills (`Computer`, `Etiquette`), pools (`Karma`, `Hacking Pool`, `Teamwork`), pre-existing wounds, and live Matrix Initiative & Hacking Pool previews.
  5. **Settings & About**: Toggle `.wav` sound effects (`winmm.dll`), toggle Auto-Karma, switch between the original `CYBERSPC.EXE` Hacking Pool calibration (`2 × Computer + 2 × Response`) and the *Shadowrun 2nd Ed* Core Rulebook formula (`(Intelligence + MPCP) / 3`), override Vidscreen display mode, and batch-convert the entire legacy corpus to `.json`.

- **100% Legacy Corpus Compatibility**:
  - Parses and round-trips legacy `.ltg` (Matrix grids), `.dek` (Cyberdecks), and `.npc` (Deckers), including Windows-1252 / Latin-1 encodings, multiline `Files=` and `Mesg=` continuations, legacy `Attack` / `Slow` utilities, and inter-matrix `LNK` node transitions.
  - Loads all original `.bmp` node and ICE artwork and plays all 53 `.wav` sound effects natively on Windows without `cgo`.

- **Strictly Offline & Hardened by Design**:
  - Zero network stack imports (`net`, `net/http`, etc. are completely absent from application packages and enforced via automated tests).
  - Confined filesystem (`internal/safefs`) preventing path traversal (`..`), symlink escape, UNC/device access, Alternate Data Streams (ADS), oversized files, and BMP decompression bombs, while using atomic writes with `.bak` backups and protecting legacy adventure `.txt` files from modification.

---

## 📂 Project Structure

```text
Cyberspace Refactor/
├── Cyberspace Refactor.exe     # Pre-built standalone Windows 10/11 executable
├── go.mod / go.sum             # Go module definition (Go 1.26+, Gio UI)
├── README.md                   # This file
├── assets/                     # Confined runtime workspace (copies of legacy assets)
│   ├── DATA/                   # 241 legacy .ltg, .dek, .npc, and adventure files
│   ├── IMAGES/                 # 257 node and ICE .bmp sprites
│   ├── SOUNDS/                 # 53 event and SysOp .wav audio files
│   ├── LOGS/                   # Exported session history logs (.txt / .log)
│   └── JSON/                   # Exported / batch-converted .json files
├── cmd/
│   └── cyberspace/             # Application entrypoint & --headless-check CLI
├── internal/
│   ├── app/                    # Workspace discovery, settings.json, confined loaders
│   ├── assets/                 # BMP loader (white-key transparency) & WinMM WAV player
│   ├── dice/                   # SR2 "Rule of Six" exploding d6 roller (CSPRNG + deterministic)
│   ├── engine/                 # Pure SR2 / VR 1.0 Matrix simulation engine
│   ├── format/                 # Tolerant INI, .ltg, .dek, .npc, and .json parsers/writers
│   ├── model/                  # Pure domain data structures, deep clone, clockwise exits
│   ├── safefs/                 # Security-hardened filesystem confinement & atomic writes
│   ├── ui/                     # Gio single-window tabbed GUI & custom canvas widgets
│   └── security_test.go        # Offline enforcement & security control tests
├── docs/
│   ├── ARCHITECTURE.md         # Architectural overview & module boundaries
│   ├── FORMATS.md              # Legacy (.ltg, .dek, .npc) & JSON format specifications
│   ├── SECURITY.md             # OWASP Top 10 & NIST CSF 2.0 compliance matrix
│   └── TEST_PLAN.md            # Complete automated & manual test routine (PT-BR / EN)
└── scripts/
    └── verify.ps1              # Automated verification, test, fuzz, and build script
```

---

## 🚀 Getting Started

### Running the Pre-Built Executable (Windows 10 / 11)

1. Open the `Cyberspace Refactor` folder.
2. Ensure the `assets/` folder (`assets/DATA`, `assets/IMAGES`, `assets/SOUNDS`) is located in the same directory as `Cyberspace Refactor.exe`.
3. Double-click **`Cyberspace Refactor.exe`** (or run `.\Cyberspace Refactor.exe` from PowerShell).
4. **Quick Start in the Terminal Tab**:
   - Click **"Quick start (First Run + Fuchi Cyber-4 + Heavy Hitter)"** to jump straight into a Matrix run, **or**
   - Click **"Choose Cyberdeck"**, **"Choose Decker"**, and **"Choose Matrix"**, then click **"Jack In"**.

### Keyboard Shortcuts (Cyberspace Terminal)

| Key | Action |
| :--- | :--- |
| `Up Arrow` / `W` | **Forward** (advance range `Observation -> Sensor -> Contact` or move through selected exit) |
| `Down Arrow` / `S` | **Backward** (retreat to previous node) |
| `Left Arrow` / `A` | **Exit -** (select previous exit counter-clockwise) |
| `Right Arrow` / `D` | **Exit +** (select next exit clockwise, starting from 6 o'clock) |
| `Space` | **Wait** (pass one action / advance Matrix turn) |

---

## 🛠️ Building from Source

### Prerequisites
- **Go 1.26+** (`windows/amd64`)
- **No C compiler (`gcc` / `cgo`) required** — the entire application and GUI compile as 100% pure Go.

### Build Commands (PowerShell)

```powershell
# Navigate to the project folder
Set-Location "Cyberspace Refactor"

# Build the standalone Windows GUI executable
go build -trimpath -ldflags "-s -w -H=windowsgui" -o "Cyberspace Refactor.exe" ./cmd/cyberspace

# Run headless self-verification
& ".\Cyberspace Refactor.exe" --headless-check
```

---

## 🧪 Running the Test Suite

To run the complete automated verification suite (static analysis, unit tests across all packages, legacy corpus round-trip tests, security & zero-network checks, short fuzzing, and binary self-check):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ".\Cyberspace Refactor\scripts\verify.ps1"
```

Or run `go test` directly:

```powershell
go -C "Cyberspace Refactor" test -v ./... -count=1
```

For the complete step-by-step manual testing guide, see [**docs/TEST_PLAN.md**](docs/TEST_PLAN.md).

---

## 🔒 Security & Compliance

Cyberspace Refactor is engineered and verified against the **OWASP Top 10 (2021)** and **NIST Cybersecurity Framework (CSF 2.0)**:
- **Strictly Offline**: Verified by `TestZeroNetworkImportsInApplicationPackages` in [`internal/security_test.go`](internal/security_test.go).
- **Confined File Operations**: Enforced by [`internal/safefs`](internal/safefs/safefs.go) (blocks path traversal, symlinks outside root, Windows device names, ADS, and unauthorized extensions).
- **Resource & Parser Hardening**: Bounded file sizes, node/ICE/file/utility count limits, BMP dimension checks (`<= 2048×2048`), and dice explosion caps (`MaxExplosions = 50`).

See [**docs/SECURITY.md**](docs/SECURITY.md) for the complete control matrix.

---

## 🙏 Credits & Acknowledgments

- **Original Author**: **Stefan Markgraf** — Creator and developer of the original *Cyberspace / Matrix Program Package* (1995–1998) for *Shadowrun 2nd Edition / Virtual Realities 1.0*, including the original `.ltg` / `.dek` / `.npc` data formats, rendered 3D node & ICE sprites, and sound design.
- *Shadowrun* and *Virtual Realities* are trademarks of FASA Corporation / Topps / Catalyst Game Labs. This project is a non-commercial preservation and modernization effort for tabletop RPG groups.
