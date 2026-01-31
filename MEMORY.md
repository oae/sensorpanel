# SensorPanel - AI Context Memory

This document provides context for AI assistants working on this codebase.

## Project Overview

**SensorPanel** is a cross-platform Go CLI tool that drives USB LCD displays as real-time system monitoring dashboards. It renders React/TypeScript themes via headless Chrome and sends the framebuffer to USB displays.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         sensorpanel CLI                          │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────┤
│   cmd/      │  pkg/       │  pkg/       │  pkg/       │  pkg/   │
│  (cobra)    │  sensors/   │  theme/     │  panel/     │ service/│
│             │             │             │             │         │
│  run.go     │  cpu        │  devserver  │  device     │ linux   │
│  theme.go   │  memory     │  template   │  protocol   │ darwin  │
│  device.go  │  disk       │  build      │  usb i/o    │ windows │
│  sensor.go  │  network    │  sdk        │             │         │
│  service.go │  nvidia_gpu │             │             │         │
│  panel.go   │  amd_gpu    │             │             │         │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────┘
```

## Key Packages

| Package | Purpose |
|---------|---------|
| `cmd/` | Cobra CLI commands |
| `pkg/sensors/` | Sensor providers with platform-specific implementations |
| `pkg/theme/` | Theme management, SDK generation, dev server |
| `pkg/panel/` | USB device communication |
| `pkg/device/` | Device profile registry |
| `pkg/service/` | Cross-platform autostart service management |
| `pkg/config/` | Configuration and device discovery |
| `pkg/paths/` | XDG-compliant path resolution |
| `pkg/browser/` | Chrome for Testing download/management |
| `pkg/renderer/` | Headless browser rendering |

## Sensor System

### Config Structure

```go
type Config struct {
    EnabledSensors  map[string]bool        // nil = all enabled
    DisabledSensors []string               // Override to disable specific
    Options         map[string]interface{} // Provider-specific options
}
```

### Available Options

| Option | Type | Provider | Description |
|--------|------|----------|-------------|
| `disk.mounts` | `[]string` | disk | Mount points to monitor |
| `network.interface` | `string` | network | Interface filter pattern |
| `nvidia_gpu.smi_path` | `string` | nvidia_gpu | Custom nvidia-smi path |

### Adding a New Sensor

1. Create `pkg/sensors/<name>_<platform>.go`
2. Implement `sensors.Provider` interface
3. Register in `init()` with `sensors.DefaultRegistry.Register()`
4. Optionally implement `sensors.OptionProvider` for custom options

## Theme System

### SDK Location

Theme SDK is embedded in `pkg/theme/template.go` and written to:
- `lib/sensorpanel/client.ts` - WebSocket client + data transformation
- `lib/sensorpanel/hooks.ts` - React hooks
- `lib/sensorpanel/types.ts` - TypeScript interfaces
- `lib/sensorpanel/index.ts` - Exports

### Data Flow

1. Backend sends JSON via WebSocket:
   ```json
   {
     "cpu": {"load": 5.7, "temperature": 53, ...},
     "memory": {"used": 9830, "total": 95640, ...},
     "disk": {"_items": [{"mount": "/", "percent": 78, ...}]},
     "network": {"_items": [{"interface": "eth0", "rx_rate": 1234, ...}]},
     "nvidia_gpu": {"load": 38, "temperature": 49, ...}
   }
   ```

2. SDK `transformData()` normalizes to `SensorData` interface

3. React hooks provide data to theme components

### Updating SDK

```bash
sensorpanel theme sdk update <theme-name>
```

## Service Management

Platform-specific implementations in `pkg/service/`:

| Platform | File | Mechanism |
|----------|------|-----------|
| Linux | `service_linux.go` | systemd user service |
| macOS | `service_darwin.go` | launchd LaunchAgent |
| Windows | `service_windows.go` | Startup folder + Registry |

## Common Patterns

### Platform-Specific Code

Use build tags:
```go
//go:build linux

package sensors
```

### Testing with XDG

Tests use `t.Setenv("XDG_DATA_HOME", tmpDir)` for isolation. All platform `DataDir()` functions respect this env var for testing.

### Error Handling

- Use `fmt.Errorf("context: %w", err)` for wrapping
- Return `nil, error` tuples for functions that can fail
- Use `t.Fatalf` not `t.Errorf` when test can't continue

## File Locations

| Type | Linux | macOS | Windows |
|------|-------|-------|---------|
| Config | `~/.config/sensorpanel/` | `~/Library/Application Support/sensorpanel/` | `%APPDATA%\sensorpanel\` |
| Data/Themes | `~/.local/share/sensorpanel/` | `~/Library/Application Support/sensorpanel/` | `%LOCALAPPDATA%\sensorpanel\` |
| Cache | `~/.cache/sensorpanel/` | `~/Library/Caches/sensorpanel/` | `%LOCALAPPDATA%\sensorpanel\cache\` |
| Service | `~/.config/systemd/user/` | `~/Library/LaunchAgents/` | `%APPDATA%\...\Startup\` |

## CI/CD

- GitHub Actions workflow in `.github/workflows/ci.yml`
- Builds on Linux, macOS, Windows
- Runs `go test ./...` on all platforms
- Uses `golangci-lint` for linting

## Recent Changes (Jan 2026)

1. **Dynamic sensor config** - Replaced hardcoded `--cpu`, `--gpu`, etc. flags with `--sensors`, `--exclude`, `--opt`

2. **Sensor options system** - `OptionProvider` interface, `sensor opts` command

3. **Theme SDK update command** - `theme sdk update` to refresh SDK in existing themes

4. **SDK data format fix** - `transformData()` now correctly handles `nvidia_gpu`/`amd_gpu`, `disk._items`, `network._items`

5. **Service management** - Cross-platform `service install/uninstall/start/stop/status/logs`

## Useful Commands

```bash
# Development
go build -o sensorpanel .
go test ./...
./sensorpanel theme dev dark --opt disk.mounts=/

# Check sensor output
./sensorpanel sensor list
./sensorpanel sensor opts

# Service management
./sensorpanel service install --opt disk.mounts=/
./sensorpanel service status
./sensorpanel service logs -f
```

## Known Quirks

1. **gousb errors in LSP** - The gousb package has platform-specific constants that show as errors in editors when viewing from different platforms. Build succeeds on each platform.

2. **Theme SDK sync** - When modifying `pkg/theme/template.go`, remember to run `theme sdk update` on affected themes.

3. **Windows path separators** - Use `filepath.Join()` for cross-platform paths in tests.

4. **XDG on non-Linux** - macOS/Windows `DataDir()` check `XDG_DATA_HOME` for testing only; production uses native paths.
