# nrf52840-hms-cli (`hmsutil`)

Cross-platform command-line utility and interactive terminal dashboard (TUI) for the **nRF52840 Hardware Security Module (HMS)** dongle, built in Go using Cobra and BubbleTea following Clean Architecture.

## Features
- **Firmware Management**:
  - `hmsutil firmware info`: Display active firmware version, active partition slot, and SHA-256 digest.
  - `hmsutil firmware update --file <file_path>`: Stream chunked signed firmware images into MCUboot candidate slot.
- **Device Management**:
  - `hmsutil info`: Retrieve device information, serial number, algorithms.
  - `hmsutil echo [message]`: Echo packet roundtrip latency test.
  - `hmsutil unlock` / `hmsutil auth`: Authenticate session, unlocking HSM (LED transitions to solid GREEN).
  - `hmsutil lock`: Terminate session and lock HSM (LED transitions to solid RED).
  - `hmsutil reset`: Reset state back to unconfigured (LED transitions to slowly pulsating BLUE).
  - `hmsutil test`: Automated end-to-end integration test suite.
  - `hmsutil tui`: Interactive terminal dashboard powered by BubbleTea.

## Supported Architectures
- macOS (`darwin/arm64`)
- Linux (`linux/arm64`, `linux/amd64`)
- Windows (`windows/arm64`, `windows/amd64`)

## Build & Test
```bash
make build       # Build host executable in bin/hmsutil
make build-all   # Cross-compile for all 5 platforms
make test        # Run unit tests
```
