# SensorPanel Theme System Implementation Plan

## Project Overview
Cross-platform CLI tool in Go to control cheap USB displays (AX206-based) as sensor panels.

## Status: COMPLETE

All tasks have been implemented and tested successfully.

## Completed Work

### Core CLI Infrastructure
- Full Go CLI with commands: `panel`, `run`, `benchmark`, `device`
- USB protocol implementation (SCSI CBW/CSW, RGB565 big-endian)
- Sensor collection (CPU, GPU, RAM, Disk, Network)
- Removed all hardcoded USB device defaults - user must run `device select`
- Added permission checking on device selection with platform-specific fix instructions

### Theme System

#### 1. XDG-compliant Directory Structure (`pkg/paths/paths.go`)
- `~/.config/sensorpanel/` - Config
- `~/.local/share/sensorpanel/themes/` - User themes
- `~/.cache/sensorpanel/browser/` - Headless Chrome cache

#### 2. Theme Package (`pkg/theme/`)
- `theme.go` - Theme discovery, loading, creation, deletion
- `template.go` - React template files generator
  - Creates full React project with Vite
  - Pre-built `dist/index.html` that works immediately without npm build
  - WebSocket-based sensor data consumption
  - Modern dark theme with CSS gradients and progress bars

#### 3. Browser Package (`pkg/browser/`)
- `browser.go` - Chrome for Testing download/management
  - Downloads Chrome v131.0.6778.85 for Linux/macOS/Windows
  - Falls back to system Chrome/Chromium if available
  - XDG cache directory storage
- `renderer.go` - Headless Chrome rendering via chromedp
  - Starts local HTTP server for theme
  - Takes screenshots for display
  - Injects sensor data via postMessage

#### 4. Server Package (`pkg/server/server.go`)
- HTTP server for theme static files
- WebSocket endpoint (`/ws`) for real-time sensor data
- Broadcasts sensor data to all connected clients

#### 5. Theme CLI Commands (`cmd/theme.go`)
```
sensorpanel theme list              # List installed themes
sensorpanel theme create <name>     # Create from React template
sensorpanel theme select <name>     # Set active theme
sensorpanel theme preview [name]    # Open in browser
sensorpanel theme delete <name>     # Remove theme
sensorpanel theme path              # Show themes directory
sensorpanel theme browser install   # Download Chrome for Testing
sensorpanel theme browser status    # Check browser availability
sensorpanel theme browser remove    # Remove cached browser
```

#### 6. Config Updates (`pkg/config/config.go`)
- Added `Theme string` field
- Added `SetTheme()` and `GetTheme()` helpers

#### 7. Run Command Integration (`cmd/run.go`)
- Detects if a theme is selected in config
- If theme selected: uses headless browser rendering
- If no theme: uses existing built-in bitmap renderer
- Starts theme server with WebSocket for sensor data
- Captures screenshots and sends to USB display

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      sensorpanel run                         │
├─────────────────────────────────────────────────────────────┤
│  1. Load config → get active theme                          │
│  2. Start theme server (HTTP + WebSocket on random port)    │
│  3. Start headless Chrome → navigate to theme               │
│  4. Loop:                                                   │
│     a. Collect sensor data                                  │
│     b. Broadcast via WebSocket to theme                     │
│     c. Screenshot Chrome → convert to RGB565                │
│     d. Send to USB display                                  │
└─────────────────────────────────────────────────────────────┘
```

## Theme Template Structure
```
~/.local/share/sensorpanel/themes/my-theme/
├── package.json        # NPM metadata + theme dimensions
├── vite.config.js      # Vite build config
├── index.html          # Dev entry point
├── src/
│   ├── main.jsx        # React entry
│   ├── App.jsx         # Main dashboard component
│   ├── App.css         # Styles
│   └── hooks/
│       └── useSensorData.js  # WebSocket hook for sensor data
└── dist/
    └── index.html      # Pre-built standalone (works immediately)
```

## Sensor Data JSON Format
Sent via WebSocket to theme:
```json
{
  "cpu": {
    "load_percent": 45.2,
    "temperature": 52.0,
    "frequency_mhz": 3600,
    "core_count": 8
  },
  "gpu": {
    "available": true,
    "name": "NVIDIA GeForce RTX 3080",
    "load_percent": 30.0,
    "temperature": 48.0,
    "memory_used_mb": 2048,
    "memory_total_mb": 8192,
    "power_watts": 120.5
  },
  "memory": {
    "total_mb": 32768,
    "used_mb": 16384,
    "available_mb": 16384,
    "percent": 50.0
  },
  "disks": [
    {"mount_point": "/", "total_gb": 500, "used_gb": 250, "free_gb": 250, "percent": 50}
  ],
  "networks": [
    {"interface": "eth0", "rx_bytes_per_sec": 1024000, "tx_bytes_per_sec": 512000}
  ]
}
```

## Files Created/Modified

| File | Status | Purpose |
|------|--------|---------|
| `pkg/paths/paths.go` | NEW | XDG directory helpers |
| `pkg/theme/theme.go` | NEW | Theme management |
| `pkg/theme/template.go` | NEW | React template generator |
| `pkg/browser/browser.go` | NEW | Chrome download/management |
| `pkg/browser/renderer.go` | NEW | Headless Chrome rendering |
| `pkg/server/server.go` | NEW | Theme HTTP server + WebSocket |
| `cmd/theme.go` | NEW | Theme CLI commands |
| `pkg/config/config.go` | MODIFIED | Added theme field |
| `cmd/run.go` | MODIFIED | Added theme rendering support |

## Quick Start Commands
```bash
# Build
go build .

# Setup (first time)
./sensorpanel device select          # Select USB display
./sensorpanel theme browser install  # Get Chrome for Testing

# Create and use a theme
./sensorpanel theme create my-theme  # Create theme from template
./sensorpanel theme select my-theme  # Activate theme
./sensorpanel run                    # Run with theme rendering

# Verify
./sensorpanel theme list
./sensorpanel theme browser status

# To use built-in renderer instead
./sensorpanel theme select ""        # Clear theme selection
./sensorpanel run                    # Uses built-in bitmap renderer
```

## Test Results

Tested on 2026-01-30:
- Theme list command: Works
- Theme path command: Works  
- Browser install: Successfully downloaded Chrome v131.0.6778.85 (157 MB)
- Browser status: Correctly shows installed browser
- Theme preview: Opens theme in browser
- Run command with theme: Successfully rendered 5 frames at 1.08 FPS to USB display
