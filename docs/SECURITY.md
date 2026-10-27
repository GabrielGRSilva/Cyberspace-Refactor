# Security Architecture & Compliance — Cyberspace Refactor

Cyberspace Refactor was designed, implemented, and tested following **Security-by-Design** principles aligned with the **OWASP Top 10 (2021)** and the **NIST Cybersecurity Framework (CSF 2.0)**.

---

## 1. Core Security Guarantees

1. **100% Offline Execution (Zero Network Surface)**:
   - None of the application packages (`cmd/...`, `internal/...`) import `net`, `net/http`, `net/url`, `net/rpc`, `net/smtp`, or `net/mail`.
   - Enforced automatically on every test run by `TestZeroNetworkImportsInApplicationPackages` in [`internal/security_test.go`](../internal/security_test.go).
2. **Strict Filesystem Confinement (`internal/safefs`)**:
   - All runtime reads and writes are confined to the `assets/` workspace via [`safefs.Root`](../internal/safefs/safefs.go).
   - Blocks directory traversal (`..`), absolute paths, drive letters (`C:`), UNC paths (`\\server\share`), NTFS Alternate Data Streams (`file.ltg:stream`), Windows reserved device names (`CON`, `PRN`, `AUX`, `NUL`, `COM1`..`COM9`, `LPT1`..`LPT9`), and symlinks resolving outside `assets/`.
3. **Protection of Non-Technical / Legacy Adventure Files**:
   - `Workspace.Save` strictly allowlists `.ltg`, `.dek`, `.npc`, and `.json`.
   - `Workspace.SaveText` restricts exported session history logs (`.txt` / `.log`) exclusively to the `assets/LOGS/` subdirectory, making it impossible to overwrite legacy `.txt` adventure descriptions in `assets/DATA/`.
   - Original files outside `Cyberspace Refactor/` are never accessed or modified.
4. **Atomic Writes & Automatic Backups**:
   - `safefs.WriteAtomic` writes to a temporary file in the target directory, flushes to disk, copies any existing file to `<filename>.bak`, and atomically renames the temporary file into place.

---

## 2. OWASP Top 10 (2021) Mapping

| OWASP Category | Risk in Desktop / File-Driven Apps | Mitigation in Cyberspace Refactor | Verification Test |
| :--- | :--- | :--- | :--- |
| **A01: Broken Access Control** | Path traversal via crafted `ImagePath=`, `LNK` node targets, or file picker inputs escaping the workspace. | `safefs.Root.Resolve` & `ResolveFold` validate lexical normalization and `filepath.EvalSymlinks` containment inside `assets/`. | `TestPathConfinement` (`internal/safefs/safefs_test.go`), `TestWorkspaceOpenLoadAndConfinement` (`internal/app/app_test.go`) |
| **A02: Cryptographic Failures** | Predictable PRNG in dice simulation. | `internal/dice` uses `crypto/rand` (`CSPRNG`) for all live rolls while allowing deterministic injection only in unit tests. | `TestExplodingD6AndPool` (`internal/dice/dice_test.go`) |
| **A03: Injection** | INI/LTG injection via multiline fields or control characters; shell command injection. | Zero shell/process execution in application runtime; INI parser sanitizes control characters and bounds line/section counts. | `FuzzParseLTG`, `FuzzParseDeck`, `FuzzParseNPC` (`internal/format/format_test.go`) |
| **A04: Insecure Design** | Resource exhaustion via decompression-bomb BMPs, infinite exploding dice, or cyclic `LNK` grids. | `MaxTextFileBytes` (2 MB), `MaxImageFileBytes` (16 MB), `maxDim` (`2048×2048` BMP header check), `MaxExplosions` (`50`), single-step `LNK` transition per action. | `TestDecodeWhiteKeyAndDimensionBombRejection` (`internal/assets/assets_test.go`), `TestCyclicLNKGridTransitionDoesNotInfiniteLoop` (`internal/security_test.go`) |
| **A05: Security Misconfiguration** | Unintended network listeners or external telemetry. | Zero network imports across all packages; pure offline desktop binary. | `TestZeroNetworkImportsInApplicationPackages` (`internal/security_test.go`) |
| **A06: Vulnerable Components** | C/C++ memory corruption via `cgo` GUI/image bindings. | 100% pure Go build (`CGO_ENABLED=0` compatible) with minimal dependency tree (`gioui.org`, `golang.org/x/image`, `golang.org/x/sys`). | `scripts/verify.ps1` (`go vet ./...`) |
| **A08: Software & Data Integrity Failures** | Corrupted saves on crash or power loss. | Atomic temp-file write + `.bak` backup before rename (`safefs.WriteAtomic`). | `TestWriteAtomicAndBackup` (`internal/safefs/safefs_test.go`) |

---

## 3. NIST Cybersecurity Framework (CSF 2.0) Mapping

- **GOVERN (GV)**: Explicit scope boundaries, zero external network dependencies, and immutable legacy corpus policy.
- **IDENTIFY (ID)**: Threat-modeled legacy parsers (`.ltg`, `.dek`, `.npc`, `.bmp`, `.wav`) and inter-file references (`ImagePath`, `LNK`).
- **PROTECT (PR)**:
  - **PR.AA / PR.DS**: `safefs` confinement, extension allowlists, `LOGS/` isolation for `.txt` exports, and atomic writes with `.bak`.
  - **PR.PS**: Pure memory-safe Go implementation without `cgo`.
- **DETECT (DE)**: Fuzz targets (`FuzzParseLTG`, `FuzzParseDeck`, `FuzzParseNPC`), corpus round-trip checks across 241 legacy files, and Matrix rule validator (`MATED`).
- **RESPOND & RECOVER (RS / RC)**: Graceful error handling in UI status bar (never panics on malformed user files) and automatic `.bak` recovery files on every save.
