# nrf52840-hsm-cli (`hsmutil`)

Cross-platform command-line utility and interactive terminal dashboard (TUI) for the **nRF52840 Hardware Security Module (HSM)** dongle, built in Go using Cobra and BubbleTea following Clean Architecture.

## Features
- **Firmware Management**:
  - `hsmutil firmware info`: Display active firmware version, active partition slot, and SHA-256 digest.
  - `hsmutil firmware update --file <file_path>`: Stream chunked signed firmware images into MCUboot candidate slot.
- **Device Management**:
  - `hsmutil info`: Retrieve device information, serial number, algorithms.
  - `hsmutil echo [message]`: Echo packet roundtrip latency test.
  - `hsmutil unlock` / `hsmutil auth`: Authenticate session, unlocking HSM (LED transitions to solid GREEN).
  - `hsmutil lock`: Terminate session and lock HSM (LED transitions to solid RED).
  - `hsmutil reset`: Reset state back to unconfigured (LED transitions to slowly pulsating BLUE).
  - `hsmutil test`: Automated end-to-end integration test suite.
  - `hsmutil tui`: Interactive terminal dashboard powered by BubbleTea.

## Supported Architectures
- macOS (`darwin/arm64`)
- Linux (`linux/arm64`, `linux/amd64`)
- Windows (`windows/arm64`, `windows/amd64`)

## Build & Test
```bash
make build       # Build host executable in bin/hsmutil
make build-all   # Cross-compile for all 5 platforms
make test        # Run unit tests
```
