# HSM CLI Utility (hsmutil)

Cross-platform command-line administration utility for the Hardware Security Module (HSM) USB key ecosystem, built in Go using Cobra and Charm's Bubble Tea micro-components following Clean Architecture.

`hsmutil` compiles as a 100% hermetic static binary (`CGO_ENABLED=0`) with zero external dynamic runtime dependencies. It supports direct AT-style ASCII protocol communication over dedicated CDC-ACM serial endpoints (`VID 0x1209`, `PID 0x0001`), PKCS#11 Cryptoki provider diagnostics, key generation and management, dual-device cryptographic key migration ceremonies, and dual-stage secure boot updates.

---

## 1. Command Hierarchy & Subcommands

Commands are organized hierarchically by resource:

```text
hsmutil
├── device                   # Device lifecycle, status, and administration
│   ├── list                 # Enumerate connected keys (aliases: devices, ls)
│   ├── info                 # Query device metadata and bootloader status
│   ├── init                 # Initialize token or run custodian threshold ceremony
│   ├── lock                 # Invalidate session and lock device (LED: RED)
│   ├── reset                # Destructive factory reset and key zeroization
│   ├── status               # Query authentication state and key count
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
├── unlock                   # Authenticate session with PIN (alias: auth)
├── echo                     # Verify serial link responsiveness and latency
├── random                   # Generate hardware random bytes from CC310 TRNG
└── pkcs11                   # PKCS#11 provider diagnostics (alias: p11)
    ├── info                 # Display Cryptoki module information
    ├── slots                # Enumerate PKCS#11 slots and mechanisms
    ├── init-token           # Initialize token on specified slot
    ├── keygen               # Generate keypair via PKCS#11 interface
    ├── sign                 # Test digital signature via PKCS#11 interface
    └── test                 # Automated end-to-end diagnostic suite
```

### 1.1 Device Lifecycle Management (`hsmutil device`)

- **`hsmutil device list`** (Aliases: `devices`, `ls`): Enumerate all connected HSM USB keys with detected BLE addresses/serials, AT data ports, console ports, and USB VID:PID descriptors. Supports `--json`.
- **`hsmutil device info`**: Query device build version, hardware model, unique device UID, active port, and bootloader stage (`AT+INFO?`).
- **`hsmutil device init [--threshold]`**: Initialize device master secret or run a turn-by-turn 3-of-5 custodian threshold ceremony.
- **`hsmutil device lock`**: Invalidate active authenticated session and return device to locked state (solid RED LED; `AT+LOCK`).
- **`hsmutil device reset [-y/--yes]`**: Factory reset the HSM, irreversibly zeroizing all persistent private keys and Shamir secrets, restoring unconfigured state (pulsating BLUE LED; `AT+RESET`). Safeguarded by an interactive confirmation prompt or bypassed via `-y`.
- **`hsmutil device status`**: Query authentication state, remaining PIN retries, and stored key count (`AT+STATUS?`).
- **`hsmutil device migrate --target <serial> [--source <serial>] [--zeroize-source] [-y]`**: Full device migration discovering all occupied key slots on the source device, performing ephemeral ECDH wrapped migration for each slot, verifying target public key digests, and enforcing **fail-fast lockdown** (immediate mutual `AT+LOCK` to both source and target on any failure). Optionally zeroizes the source device upon completion.
- **`hsmutil device firmware info`**: Query active firmware version and MCUboot partition slot.
- **`hsmutil device firmware update --file <bin>`**: Stream chunked signed firmware images into the candidate flash slot.

### 1.2 Cryptographic Key Management (`hsmutil key`)

- **`hsmutil key list`**: Enumerate stored EC key slots and metadata (`AT+SLOTS?`).
- **`hsmutil key generate --slot <id> [--algo EC_P256]`**: Generate NIST P-256 (`secp256r1`) keypair using hardware TRNG (`AT+KEYGEN`).
- **`hsmutil key pubkey --slot <id>`**: Read uncompressed 65-byte public key point (`AT+PUBKEY`).
- **`hsmutil key sign --slot <id> --hash <hex>`**: Compute hardware ECDSA signature over 32-byte SHA-256 hash (`AT+SIGN`).
- **`hsmutil key migrate --slot <id> --target <serial> [--source <serial>]`**: Migrate a single specified key slot to another HSM using blind ECDH encryption.

### 1.3 Session & Diagnostic Commands

- **`hsmutil unlock [--pin <pin>] [--threshold]`** (Alias: `auth`): Authenticate User PIN, unlocking cryptographic operations (solid GREEN LED; `AT+AUTH`). Features masked PIN input in interactive terminals.
- **`hsmutil echo [message]`**: Ping connected HSM USB key and verify roundtrip serial communication latency (`AT`).
- **`hsmutil random [--len <n>]`**: Generate hardware random bytes from CryptoCell CC310 TRNG (`AT+RANDOM=<len>`).
- **`hsmutil pkcs11`** (Alias: `p11`): Diagnostic suite for inspecting and testing the `hsm-libpkcs11` shared library provider.

---

## 2. Backward Compatibility & Ergonomic Aliases

To preserve existing operator muscle memory, scripts, and CI/CD pipelines:
1. **Hidden Root Aliases**: Legacy root-level commands (`hsmutil info`, `hsmutil reset`, `hsmutil lock`, `hsmutil init`, `hsmutil firmware`, `hsmutil status`) remain registered on `rootCmd` as hidden aliases (`Hidden: true`). They execute identically to their `hsmutil device <subcommand>` counterparts without cluttering `--help`.
2. **Top-Level Convenience Aliases**:
   - `hsmutil devices`: Direct top-level alias for `hsmutil device list`.
   - `hsmutil auth`: Direct top-level alias for `hsmutil unlock`.
3. **Plural Binary Symlink**: Building `hsmutil` automatically generates a relative symbolic link:
   ```bash
   bin/hsmutils -> hsmutil
   ```
   Both `bin/hsmutil` and `bin/hsmutils` execute identically.

---

## 3. Terminal UX & Bubble Tea Micro-Components

`hsmutil` integrates targeted Bubble Tea micro-components with automatic TTY detection (`isatty.IsTerminal`):

| Operation | Interactive Terminal (TTY) | Headless / Pipe / CI Fallback |
| :--- | :--- | :--- |
| **`hsmutil unlock`** | Masked password prompt (`bubbles/textinput`) rendering bullets (`••••••`) without plaintext echo. | Reads single line from `stdin` or requires `--pin` flag without ANSI escape sequences. |
| **`hsmutil device firmware update`** | Animated gradient progress bar (`bubbles/progress`) with live transfer metrics and percentage. | Emits discrete periodic plain-text log lines (e.g. `[INFO] DFU Transfer: 50% complete (300/600 bytes)`). |

---

## 4. Single Key vs Full Device Migration

| Dimension | Single Key Migration (`key migrate`) | Full Device Migration (`device migrate`) |
| :--- | :--- | :--- |
| **Command** | `hsmutil key migrate --slot <id> --target <serial>` | `hsmutil device migrate --target <serial>` |
| **Target Scope** | Exactly one designated hardware slot | All occupied slots (0..31), PINs, and metadata |
| **Source Key Action** | Preserved on source device | **Zeroized** after end-to-end verification (if `--zeroize-source`) |
| **Failure Handling** | Target slot erased; source unmodified | **Fail-fast lockdown**: mutual `AT+LOCK` immediately sent to both devices |
| **Use Case** | Replicating a shared signing key across dongles | Hardware replacement, dongle decommissioning |

---

## 5. PKCS#11 Equivalence Matrix & GAP Analysis

The table below maps `hsmutil` commands against standard OpenSC `pkcs11-tool` invocations when using the `hsm-libpkcs11` Cryptoki provider library (`libnrf52840_pkcs11.dylib` / `.so` / `.dll`):

| Functional Area | `hsmutil` Command | `pkcs11-tool` Equivalent (`--module <lib>`) | Parity Status | Architecture & Scope Rationale |
| :--- | :--- | :--- | :--- | :--- |
| **Device Enumeration** | `hsmutil device list` | `pkcs11-tool -L` (`--list-slots`) | Equivalent | `hsmutil` displays OS device nodes and hardware BLE serials; PKCS#11 displays Cryptoki slot tokens. |
| **Device Metadata** | `hsmutil device info` | `pkcs11-tool -M` (`--list-mechanisms`) / `-T` | Equivalent | `hsmutil` includes MCUboot slot metadata and PSA digests; PKCS#11 exposes standard token info. |
| **Device Initialization** | `hsmutil device init` | `pkcs11-tool --init-token --init-pin` | Equivalent | Supported in both interfaces. |
| **Session Unlock** | `hsmutil unlock` (with PIN) | `pkcs11-tool --login --pin <PIN>` | Equivalent | Standard Cryptoki `C_Login`. |
| **Session Lock** | `hsmutil device lock` | `pkcs11-tool --logout` | Equivalent | Standard Cryptoki `C_Logout`. |
| **Factory Reset** | `hsmutil device reset` | *None* | `hsmutil` Exclusive | Out-of-band administrative hardware zeroization not defined in standard Cryptoki specification. |
| **Firmware Update (DFU)** | `hsmutil device firmware update` | *None* | `hsmutil` Exclusive | Dual-stage secure boot firmware lifecycle is a host administration function. |
| **Key Generation** | `hsmutil key generate --slot <N>` | `pkcs11-tool --keypairgen --key-type EC:prime256v1 --id <N>` | Equivalent | Standard Cryptoki `C_GenerateKeyPair`. |
| **Key Enumeration** | `hsmutil key list` | `pkcs11-tool -O` (`--list-objects`) | Equivalent | Standard Cryptoki `C_FindObjects`. |
| **Digital Signature** | `hsmutil key sign` | `pkcs11-tool --sign --id <N> -m ECDSA-SHA256` | Equivalent | Standard Cryptoki `C_Sign`. |
| **Key Migration** | `hsmutil key migrate` | `pkcs11-tool --wrap` / `--unwrap` | Partial Gap | Direct slot-to-slot wrapping supported in `hsmutil`; PKCS#11 requires `C_WrapKey` mechanism support. |
| **Device Migration** | `hsmutil device migrate` | *None* | `hsmutil` Exclusive | Multi-key atomic device cloning with source zeroization is a platform administrative capability. |
| **Random Data (TRNG)** | `hsmutil random` | `pkcs11-tool --generate-random <bytes>` | Equivalent | Standard Cryptoki `C_GenerateRandom`. |

---

## 6. Serial Port Auto-Discovery & Targeting

`hsmutil` communicates over dedicated CDC-ACM serial interfaces (`VID 0x1209`, `PID 0x0001` at 115200 8N1).

- **Dual CDC-ACM Port Prioritization**: Automatically prioritizes the AT data port (interface index 3, e.g. `/dev/tty.usbmodem...3`) over the kernel console logger (interface index 1).
- **Targeting by Serial / BLE Address**:
  - **CLI Flag**: `hsmutil --serial F4CE368F12AB <command>` (shorthand `-s`)
  - **Environment Variable**: `export HSM_SERIAL=F4CE368F12AB` (fallback: `NRF52840_HSM_SERIAL`)
- **Port Path Override**:
  - **CLI Flag**: `hsmutil --port /dev/tty.usbmodem31424103 <command>` (shorthand `-p`)
  - **Environment Variable**: `export HSM_PORT=/tmp/mock-hsm-pty` (fallback: `NRF52840_HSM_PORT`)
- **Multi-Device Disambiguation Safeguard**:
  - If multiple HSM keys are attached without `--serial` or `--port`, `hsmutil` halts with a structured error table listing attached keys and their serials to prevent accidental execution against the wrong device.

---

## 7. Build Automation & Static Compilation

All binaries are compiled with `CGO_ENABLED=0` to guarantee 100% static linkage with zero C runtime dependencies:

```bash
make build       # Build host static binary in bin/hsmutil (creates symlink bin/hsmutils)
make build-all   # Cross-compile static binaries for all 5 target architectures
make test        # Run Go unit and component tests
make clean       # Remove compiled binaries and symlinks
```

### Target Architectures
- macOS (`darwin/arm64`)
- Linux (`linux/arm64`, `linux/amd64`)
- Windows (`windows/arm64`, `windows/amd64`)
