# Keenetic Tray

System tray application for managing Keenetic routers. Shows the active network policy for your device and lets you switch policies with a click.

## Features

- Detects the active Keenetic router on the current network automatically
- Displays the current access policy in the tray icon
- Switch policies (Default, Blocked, or any named policy) from the tray menu
- Multiple routers supported — activates the one reachable on the current network
- Passwords stored securely in the OS keychain (Windows Credential Manager / macOS Keychain / Linux Secret Service)

## Screenshots

The tray icon changes color based on the active policy:
- **Blue** — Default policy
- **Red** — Blocked
- **Green** — Custom named policy

## Installation

Download the binary for your platform from the [Releases](../../releases) page.

### Windows
Run `keenetic-tray.exe`. No installation required.

### macOS
Run `keenetic-tray-darwin-arm64` (Apple Silicon) or `keenetic-tray-darwin-amd64` (Intel).
You may need to allow it in System Settings → Privacy & Security.

### Linux
```bash
chmod +x keenetic-tray-linux-amd64
./keenetic-tray-linux-amd64
```
Requires a running D-Bus Secret Service (GNOME Keyring or KWallet) for password storage.

## First Run

1. Click the tray icon → **Add Router...**
2. Enter the router name, address (e.g. `192.168.1.1`), login, and password
3. The app verifies the connection and saves the router
4. The icon updates automatically to reflect the current policy

## Building from Source

### Requirements
- Go 1.21+
- GCC (MinGW-w64 on Windows via [MSYS2](https://www.msys2.org/))
- Linux: `libgl1-mesa-dev xorg-dev libdbus-1-dev libsecret-1-dev`

### Build

```bash
# Windows (run in Git Bash or MSYS2)
make windows

# Linux
make linux

# macOS (Apple Silicon)
make mac-arm

# macOS (Intel)
make mac-amd

# Run locally without building a binary
make run
```

The binary is placed in `bin/`.

## CI / CD

- **Every commit** → builds for all three platforms, artifacts available for **14 days** in GitHub Actions
- **Tag `v*`** → builds and creates a GitHub Release with binaries attached permanently

```bash
# Create a release
git tag v1.0.0
git push origin v1.0.0
```

## Project Structure

```
app/
├── main.go       Entry point
├── config.go     Router config (JSON) and keyring integration
├── router.go     Keenetic HTTP API client
├── network.go    Local network interface detection
├── icon.go       Dynamic tray icon generation
├── tray.go       System tray logic and menu building
├── ui.go         Fyne windows and dialogs
├── Makefile
└── .github/
    └── workflows/
        ├── build.yml    CI — build on every commit
        └── release.yml  CD — release on tag push
```

## Tech Stack

| Layer | Library |
|---|---|
| UI & Tray | [Fyne v2](https://fyne.io/) |
| Tray icon drawing | `image/draw` + `golang.org/x/image` (Go Bold font) |
| HTTP client | `net/http` (stdlib) |
| Password store | [99designs/keyring](https://github.com/99designs/keyring) |
| Default gateway | [jackpal/gateway](https://github.com/jackpal/gateway) |

## License

MIT
