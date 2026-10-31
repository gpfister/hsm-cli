# nrf52840-hsm-cli (`hsmutil`)

Cross-platform command-line utility and interactive terminal dashboard (TUI) for the **nRF52840 Hardware Security Module (HSM)** dongle (`PCA10059`), built in Go using Cobra and BubbleTea following Clean Architecture.

Supports direct AT-style ASCII protocol communication over dedicated CDC-ACM serial endpoints (`VID 0x1209`, `PID 0x0001`), PKCS#11 Cryptoki provider diagnostics, key generation and management, and dual-stage secure boot updates.

## Features

### Device & Session Management
- `hsmutil devices`: List connected nRF52840 HSM dongles with detected BLE addresses, AT data ports, console ports, and USB descriptors (Alias: `hsmutil list`, supports `--json`).
- `hsmutil info`: Query device build version, hardware model, unique device UID, active port, and bootloader stages (`AT+INFO?`).
- `hsmutil status`: Inspect active authentication state, remaining PIN retries, and stored key count (`AT+STATUS?`).
- `hsmutil auth --pin <pin>`: Authenticate User PIN, unlocking cryptographic operations (`AT+AUTH=<pin>`). (Alias: `unlock`).
- `hsmutil lock`: Invalidate active session and return HSM to locked state (`AT+LOCK`).
- `hsmutil reset`: Reset token session state and clear volatile context (`AT+RESET`).
- `hsmutil echo [message]`: Ping HSM dongle and verify roundtrip serial communication latency (`AT`).
- `hsmutil random --len <n>`: Generate hardware random bytes from CryptoCell CC310 TRNG (`AT+RANDOM=<len>`).

### Hardware Key Management
- `hsmutil key list`: Enumerate stored EC key slots (`AT+SLOTS?`).
- `hsmutil key generate --slot <id> --algo EC_P256`: Generate NIST P-256 (`secp256r1`) keypair using hardware TRNG (`AT+KEYGEN=<id>,EC_P256`).
- `hsmutil key pubkey --slot <id>`: Read uncompressed 65-byte public key point (`AT+PUBKEY=<id>`).
- `hsmutil key sign --slot <id> --hash <hex>`: Compute hardware ECDSA signature over 32-byte SHA-256 hash (`AT+SIGN=<id>,<hash>`).

### PKCS#11 Provider Diagnostics
- `hsmutil p11 info --module <path>`: Inspect Cryptoki version, manufacturer, and library description.
- `hsmutil p11 slots --module <path>`: Enumerate provider slots, token information, and supported mechanisms (`CKM_EC_KEY_PAIR_GEN`, `CKM_ECDSA`, `CKM_ECDSA_SHA256`).
- `hsmutil p11 test --module <path> --pin <pin>`: Automated end-to-end diagnostic suite verifying library initialization, slot discovery, session management, user login, random generation, and clean finalization.

### Firmware & Integration Testing
- `hsmutil firmware info`: Query active firmware version and MCUboot partition slot.
- `hsmutil firmware update --file <bin>`: Stream chunked signed firmware images into candidate flash slot (Note: DFU streaming via `AT+DFU_CHUNK` / `AT+DFU_APPLY` will be finalized in firmware follow-up).
- `hsmutil test`: Automated HSM integration self-test suite.
- `hsmutil tui`: Interactive terminal dashboard powered by BubbleTea.

## Serial Port Auto-Discovery & Targeting
The CLI automatically searches for nRF52840 HSM dongles matching Vendor ID `0x1209` and Product ID `0x0001` (pid.codes nRF52840 HSM Dongle) at 115200 8N1.

- **Dual CDC-ACM Port Prioritization**: Automatically prioritizes suffix `3` (`/dev/tty.usbmodem...3` / AT data interface) over suffix `1` (Zephyr console logger interface) on macOS and Linux CDC-ACM serial ports.
- **BLE Address Matching**: Discovers and matches dongles formatted as `/dev/tty.usbmodem<BLE_ADDR>3`.
- **Targeting by Serial / BLE Address**:
  - **CLI Flag**: `hsmutil --serial F4CE368F12AB <command>` (shorthand `-s`)
  - **Environment Variable**: `export NRF52840_HSM_SERIAL=F4CE368F12AB`
- **Port Path Override**:
  - **CLI Flag**: `hsmutil --port /dev/tty.usbmodem31424103 <command>` (shorthand `-p`)
  - **Environment Variable**: `export NRF52840_HSM_PORT=/tmp/mock-hsm-pty` (e.g. for virtual PTY testing)

## Supported Architectures
- macOS (`darwin/arm64`)
- Linux (`linux/arm64`, `linux/amd64`)
- Windows (`windows/arm64`, `windows/amd64`)

## Build & Test
```bash
make build       # Build host executable in bin/hsmutil
make build-all   # Cross-compile across all 5 target architectures
make test        # Run unit tests
```
