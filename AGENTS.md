# AGENTS.md — Developer Guide for AI Agents

This file provides context for AI coding assistants working on the Keenetic Tray project.

## Project Overview

A cross-platform system tray application (Windows / macOS / Linux) written in Go.
It connects to a Keenetic router on the local network and displays / controls the active
access policy for the current device.

## Tech Stack

| Concern | Choice | Notes |
|---|---|---|
| GUI + Tray | `fyne.io/fyne/v2` | Fyne v2, uses OpenGL/Metal/D3D |
| Tray icon | `image/draw` + `golang.org/x/image` | Drawn programmatically, no image files |
| Keenetic API | `net/http` | HTTP Basic→Challenge auth, JSON REST |
| Config | `encoding/json` + `os` | Stored in OS-appropriate directory |
| Passwords | `github.com/99designs/keyring` | WinCred / macOS Keychain / SecretService |
| Default gateway | `github.com/jackpal/gateway` | Platform-agnostic |

## File Map

| File | Responsibility |
|---|---|
| `main.go` | Entry point. Creates Fyne app, checks for desktop (tray) support, starts TrayApp |
| `config.go` | `RouterConfig` struct, `loadRouters` / `saveRouters`, keyring helpers (`getPassword`, `setPassword`, `deletePassword`) |
| `router.go` | `KeeneticRouter` — Keenetic HTTP API: login (MD5+SHA256 challenge), get clients, get policies, apply/block policy |
| `network.go` | `getLocalNetworks`, `isIPInNetworks`, `extractHost`, `getDefaultInterfaceName`, `listLocalInterfaces`, `InterfaceInfo` |
| `icon.go` | `GenerateIcon(label)` → `fyne.Resource`. Draws a colored circle with 3-char label using Go Bold font. Color codes: blue=Default, red=Blocked, green=custom |
| `tray.go` | `TrayApp` — owns the refresh loop, state collection, menu building. Key methods: `newTrayApp`, `collectState`, `applyState`, `buildStateMenu`, `applyPolicy` |
| `ui.go` | Fyne windows: `showSettingsWindow` (router list), `showRouterFormDialog` (add/edit), `showRouterFormDialogWithError` (retry with error) |

## Key Data Flows

### Startup
```
main() → newTrayApp() → scheduleRefresh() → loop() → collectState() → applyState()
```

### Policy change
```
tray menu click → applyPolicy(mac, policy, mode) → goroutine → router.SetClientBlock / ApplyPolicy → scheduleRefresh()
```

### Add router
```
"Add Router..." menu → openSettingsAdd() → showAddRouterDialog() → showRouterFormDialog()
→ goroutine: router.Login() + GetNetworkIP() + GetKeenDNSURLs()
→ onConfirm() → setPassword() → saveRouters() → scheduleRefresh()
```

## Config File Location

| OS | Path |
|---|---|
| Windows | `%APPDATA%\RouterManager\routers.json` |
| macOS | `~/Library/Application Support/RouterManager/routers.json` |
| Linux | `~/.router_manager/routers.json` |

Passwords are **not** stored in JSON. They go into the OS keychain under service name `keenetic-tray`.

## Keenetic Auth Protocol

```
GET /auth  →  401 with X-NDM-Realm + X-NDM-Challenge headers
password_hash = SHA256( challenge + MD5( login + ":" + realm + ":" + password ) )
POST /auth  { "login": ..., "password": password_hash }  →  200 OK
Session cookie is maintained automatically by http.CookieJar.
```

## Building

```bash
# Native (on the target OS)
make windows   # or linux / mac-arm / mac-amd

# Run without binary
make run
```

CGO is required (Fyne uses OpenGL). On Windows use MinGW-w64 (via MSYS2 ucrt64).

## CI / CD

- `.github/workflows/build.yml` — runs on every commit (not tags), 14-day artifact retention
- `.github/workflows/release.yml` — runs on `v*` tags, creates GitHub Release with binaries

## Common Pitfalls

- **Fyne UI calls must not be made from goroutines** — use channels or callbacks to return results to the main goroutine before updating the menu/icon.
- **`desk.SetSystemTrayMenu` replaces the whole menu** — always rebuild the full menu from current state.
- **keyring on Linux** requires a running Secret Service daemon (GNOME Keyring / KWallet). Without it, `globalRing` is nil and passwords fall back to being missing — the router won't be selected.
- **Cross-compilation with CGo is not supported** from Windows to Linux/macOS. Use CI (each platform builds natively).
- **macOS tray** may require the app to be code-signed for distribution. For development, allow it manually in System Settings.
