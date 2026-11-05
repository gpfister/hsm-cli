# HSM CLI Utility (`gphsm`)

Cross-platform command-line administration utility for the Hardware Security Module (HSM) USB key ecosystem, built in Go using Cobra and Charm's Bubble Tea micro-components following Clean Architecture.

`gphsm` compiles as a 100% hermetic static binary (`CGO_ENABLED=0`) with zero external dynamic runtime dependencies. It supports direct AT-style ASCII protocol communication over dedicated CDC-ACM serial endpoints (`VID 0x1209`, `PID 0x0001`), dual-mode authentication (PIN protected with Yocto `pam_pwquality` validation vs. 3-of-5 Shamir Secret Sharing), key generation and signing, dual-device homogeneous key migration ceremonies, and dual-stage secure boot updates.

---

## 1. Command Hierarchy & Subcommands

Commands are organized hierarchically by resource:

```text
gphsm
├── device                   # Device lifecycle, status, and administration
│   ├── list                 # Enumerate connected keys (aliases: devices, ls)
│   ├── info                 # Query device metadata and bootloader status
│   ├── init                 # Initialize in PIN or Shamir Secret Sharing mode
│   ├── lock                 # Invalidate session and lock device (LED: RED)
│   ├── reset                # Destructive factory reset and key zeroization
│   ├── status               # Query authentication state, method, and key count
│   ├── migrate              # Full device migration with fail-fast lockdown
│   └── firmware             # Firmware lifecycle management
│       ├── info             # Query active firmware slot and size
│       └── update           # Stream MCUboot firmware binary with DFU progress
├── key                      # Hardware cryptographic key management
│   ├── list                 # Enumerate stored key slots
│   ├── generate             # Generate EC keypair using hardware TRNG
│   ├── pubkey               # Display uncompressed 65B public key point
│   ├── sign                 # Sign 32B SHA-256 digest using hardware accelerator
│   └── migrate              # Migrate a single key slot between devices
├── unlock                   # Authenticate session (PIN or Shamir) (alias: auth)
├── echo                     # Verify serial link responsiveness and latency
└── random                   # Generate hardware random bytes from CC310 TRNG
```

### 1.1 Device Lifecycle Management (`gphsm device`)

- **`gphsm device list`** (Aliases: `devices`, `ls`): Enumerate all connected HSM USB keys with detected BLE addresses/serials, AT data ports, console ports, and USB VID:PID descriptors. Supports `--json`.
- **`gphsm device info`**: Query device build version, hardware model, unique device UID, active port, and bootloader stage.
- **`gphsm device init [--mode pin|shamir] [--pin <pin>] [--threshold]`**: Initialize device authentication mode:
  - **PIN Mode**: Sets up password protection. Enforces Yocto-compliant `pam_pwquality` complexity (min 10 characters, upper, lower, digit, special, max 1 consecutive identical character, max 2 sequential monotonic characters). Supports `--pin` and `GPHSM_PIN` env var.
  - **Shamir Mode**: Turn-by-turn 3-of-5 custodian initialization ceremony.
- **`gphsm device lock`**: Invalidate active authenticated session and return device to locked state (solid RED LED).
- **`gphsm device reset [-y/--yes]`**: Factory reset the HSM, irreversibly zeroizing all persistent private keys and secrets, restoring unconfigured state (pulsating BLUE LED). Safeguarded by an interactive confirmation prompt or bypassed via `-y`.
- **`gphsm device status`**: Query authentication state, configuration status, configured authentication method (`NONE`, `PIN`, `SHAMIR`), remaining PIN retries, and stored key count.
- **`gphsm device migrate --target <serial> [--source <serial>] [--zeroize-source] [-y]`**: Full device migration discovering all occupied key slots on the source device, performing ephemeral ECDH wrapped migration for each slot, verifying target public key digests, and enforcing **fail-fast lockdown** (immediate mutual lock to both source and target on any failure). Verifies that both devices share identical authentication methods (**homogeneous migration**) before transferring keys. Optionally zeroizes the source device upon completion.
- **`gphsm device firmware info`**: Query active firmware version and MCUboot partition slot.
- **`gphsm device firmware update --file <bin>`**: Stream chunked signed firmware images into the candidate flash slot.

### 1.2 Cryptographic Key Management (`gphsm key`)

- **`gphsm key list`**: Enumerate stored EC key slots and metadata.
- **`gphsm key generate --slot <id> [--algo EC_P256]`**: Generate NIST P-256 (`secp256r1`) keypair using hardware TRNG.
- **`gphsm key pubkey --slot <id>`**: Read uncompressed 65-byte public key point.
- **`gphsm key sign --slot <id> --hash <hex>`**: Compute hardware ECDSA signature over 32-byte SHA-256 hash.
- **`gphsm key migrate --slot <id> --target <serial> [--source <serial>]`**: Migrate a single specified key slot to another HSM using blind ECDH encryption. Strictly verifies homogeneous authentication methods between devices.

### 1.3 Session & Utility Commands

- **`gphsm unlock [--pin <pin>] [--threshold]`** (Alias: `auth`): Automatically detects device configuration mode:
  - In **PIN Mode**: Prompts for masked PIN via Bubble Tea or accepts `--pin` / `GPHSM_PIN` / stdin.
  - In **Shamir Mode**: Executes 3-of-5 custodian turn-by-turn threshold unlock ceremony.
  - If unconfigured, instructs operator to run `gphsm device init`.
  - If already unlocked, reports current authenticated status.
- **`gphsm echo [message]`**: Ping connected HSM USB key and verify roundtrip serial communication latency.
- **`gphsm random [--len <n>]`**: Generate hardware random bytes from CryptoCell CC310 TRNG.

---

## 2. Password Quality Policy (`pam_pwquality`)

For devices initialized in PIN mode, `gphsm` enforces security parity with the Yocto appliance PAM configuration (`/etc/security/pwquality.conf`):

| Constraint | Value | Rationale |
| :--- | :--- | :--- |
| **Minimum Length** (`minlen`) | $\ge 10$ characters | Prevents brute-force exhaustion |
| **Character Diversity** | $\ge 1$ upper, lower, digit, special | High entropy assurance |
| **Max Repeat** (`maxrepeat`) | $\le 1$ consecutive identical char | Rejects "aa", "11", "!!" patterns |
| **Max Sequence** (`maxsequence`) | $\le 2$ sequential monotonic chars | Rejects "123", "abc", "321", "cba" sequences |
| **Maximum Length** | $\le 64$ characters | Bounded flash keystore buffer limit |

---

## 3. Backward Compatibility & Ergonomic Aliases

To preserve existing operator muscle memory, scripts, and CI/CD pipelines:
1. **Backward-Compatible Symlinks**:
   Building `gphsm` automatically generates relative symbolic links:
   ```bash
   bin/gphsms  -> gphsm
   bin/hsmutil -> gphsm
   bin/hsmutils -> gphsm
   ```
2. **Hidden Root Aliases**: Legacy root-level commands (`gphsm info`, `gphsm reset`, `gphsm lock`, `gphsm init`, `gphsm firmware`, `gphsm status`) remain registered on `rootCmd` as hidden aliases.
3. **Top-Level Convenience Aliases**:
   - `gphsm devices`: Direct top-level alias for `gphsm device list`.
   - `gphsm auth`: Direct top-level alias for `gphsm unlock`.

---

## 4. Terminal UX & Bubble Tea Micro-Components

`gphsm` integrates targeted Bubble Tea micro-components with automatic TTY detection (`isatty.IsTerminal`):

| Operation | Interactive Terminal (TTY) | Headless / Pipe / CI Fallback |
| :--- | :--- | :--- |
| **`gphsm unlock`** | Masked password prompt (`bubbles/textinput`) rendering bullets (`••••••`) without plaintext echo. | Reads single line from `stdin` or requires `--pin` flag without ANSI escape sequences. |
| **`gphsm device init`** | Interactive mode selection ([1] PIN, [2] Shamir) and masked confirmation prompts. | Requires `--mode=pin --pin=<pin>` or `--mode=shamir` flags. |
| **`gphsm device firmware update`** | Animated gradient progress bar (`bubbles/progress`) with live transfer metrics and percentage. | Emits discrete periodic plain-text log lines (e.g. `[INFO] DFU Transfer: 50% complete (300/600 bytes)`). |

---

## 5. Homogeneous Migration Safeguards

To prevent security policy downgrades or operational lockout, migration operations (`key migrate` and `device migrate`) strictly enforce homogeneous authentication configuration:
- Both source and target devices must be initialized (`CONFIGURED=1`).
- Source and target devices must use identical authentication methods (`source.METHOD == target.METHOD`).
- If either device is unconfigured or methods mismatch (e.g. migrating from Shamir to PIN or vice versa), the command immediately aborts with an actionable error message before transferring any cryptographic material.

---

## 6. Serial Port Auto-Discovery & Targeting

`gphsm` communicates over dedicated CDC-ACM serial interfaces (`VID 0x1209`, `PID 0x0001` at 115200 8N1).

- **Dual CDC-ACM Port Prioritization**: Automatically prioritizes the AT data port (interface index 3, e.g. `/dev/tty.usbmodem...3`) over the kernel console logger (interface index 1).
- **Targeting by Serial / BLE Address**:
  - **CLI Flag**: `gphsm --serial F4CE368F12AB <command>` (shorthand `-s`)
  - **Environment Variable**: `export GPHSM_SERIAL=F4CE368F12AB` (fallback: `HSM_SERIAL`, `NRF52840_HSM_SERIAL`)
- **Port Path Override**:
  - **CLI Flag**: `gphsm --port /dev/tty.usbmodem31424103 <command>` (shorthand `-p`)
  - **Environment Variable**: `export GPHSM_PORT=/tmp/mock-hsm-pty` (fallback: `HSM_PORT`, `NRF52840_HSM_PORT`)
- **Multi-Device Disambiguation Safeguard**:
  - If multiple HSM keys are attached without `--serial` or `--port`, `gphsm` halts with a structured error table listing attached keys and their serials to prevent accidental execution against the wrong device.

---

## 7. Build Automation & Static Compilation

All binaries are compiled with `CGO_ENABLED=0` to guarantee 100% static linkage with zero C runtime dependencies:

```bash
make build       # Build host static binary in bin/gphsm (creates symlinks gphsms, hsmutil, hsmutils)
make build-all   # Cross-compile static binaries for all 5 target architectures
make test        # Run Go unit and component tests
make clean       # Remove compiled binaries and symlinks
```

### Target Architectures

- `darwin/arm64` (Apple Silicon macOS)
- `linux/arm64` (Raspberry Pi 64-bit Yocto appliance / ARM servers)
- `linux/amd64` (Standard x86_64 Linux servers & workstations)
- `windows/arm64` (Surface Pro / Windows on ARM)
- `windows/amd64` (Standard Windows x86_64)
