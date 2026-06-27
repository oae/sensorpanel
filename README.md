# SensorPanel

[![CI](https://github.com/oae/sensorpanel/actions/workflows/ci.yml/badge.svg)](https://github.com/oae/sensorpanel/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/oae/sensorpanel)](https://goreportcard.com/report/github.com/oae/sensorpanel)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A cross-platform CLI tool for driving USB LCD displays as real-time system monitoring dashboards.

![Dashboard Example](docs/dashboard-preview.png)

## Features

- **Multi-device support** - Modular device profiles for different USB displays
- **Easy device contribution** - Interactive wizard to add support for new panels
- **Real-time monitoring** - CPU, GPU (NVIDIA/AMD), RAM, disk, and network stats
- **Media display modes** - Static images, animated GIFs, and a now-playing dashboard
- **Music dashboard** - Cover art, song metadata, progress waveform, and synchronized lyrics
- **Dynamic sensor config** - Enable/disable sensors and configure options at runtime
- **Web-based themes** - Create custom themes using React + TypeScript
- **TypeScript SDK** - React hooks for easy theme development with hot reload
- **Single-command dev** - One command starts everything for theme development
- **Headless rendering** - Auto-downloads Chrome for Testing to render themes
- **Cross-platform** - Works on Linux, macOS, and Windows
- **Autostart service** - Install as system service on all platforms
- **NixOS support** - Flake with module, udev rules, and systemd service

## Quick Start

### 1. Build

```bash
# Install Mage (recommended build tool)
go install github.com/magefile/mage@latest

# Build with Mage
mage build

# Or directly with Go
go build .

# Or with Nix
nix build
```

### 2. Select your display

```bash
./sensorpanel device list     # See available devices
./sensorpanel device select   # Interactive selection
```

### 3. Run the dashboard

```bash
# With built-in renderer
./sensorpanel run

# Play an animated GIF instead of sensor data
./sensorpanel run --gif /path/to/animation.gif

# URLs are also supported
./sensorpanel run --gif https://media.tenor.com/j8dwT9wdyc8AAAAi/evernight-anime.gif

# Display a static PNG, JPEG, or GIF from a file or URL
./sensorpanel run --image /path/to/wallpaper.png

# Display the active song with artwork, progress waveform, and synchronized lyrics
./sensorpanel run --music

# With sensor options
./sensorpanel run --opt disk.mounts=/,/home --opt network.interface=eth*

# Or create and use a custom theme
./sensorpanel theme create my-theme
./sensorpanel theme select my-theme
./sensorpanel run
```

### 4. (Optional) Install as autostart service

```bash
# Install to start on login
./sensorpanel service install --opt disk.mounts=/

# Start now
./sensorpanel service start

# Check status
./sensorpanel service status
```

## Supported Devices

SensorPanel uses a modular device profile system. Currently supported:

| Device | Resolution | Color Format | Notes |
|--------|------------|--------------|-------|
| QTKeJi/AIDA64 USB Display | 480x320 | RGB565 BE | VID 0x1908 |
| Generic AX206-based frames | Various | RGB565 | GEMBIRD, Pearl, Coby, etc. |

**Don't see your device?** Run `sensorpanel device create` to add support for it!

## Commands

### Run Dashboard

```bash
sensorpanel run [flags]

Flags:
  -i, --interval float    Update interval in seconds (default 1.0)
  -b, --brightness int    Backlight brightness 0-7 (default 7)
  -s, --sensors strings   Sensors to enable (e.g., cpu,memory,disk). Default: all
  -x, --exclude strings   Sensors to exclude (e.g., network,nvidia_gpu)
  -o, --opt strings       Sensor options (e.g., disk.mounts=/,/home)
      --gif string        Play an animated GIF file or URL instead of sensor data
      --image string      Display a PNG, JPEG, or GIF file or URL instead of sensor data
      --music             Show now-playing music dashboard instead of sensor data
```

The music dashboard currently supports Linux MPRIS players such as Spotify,
VLC, and compatible browser players. It requires `playerctl`. Synchronized
lyrics are loaded from LRCLIB when available.
Lyrics are cached locally for faster reuse and fewer network requests.

See [Media and Music Modes](docs/media-modes.md) for format support, layout
behavior, requirements, lyrics behavior, and service setup.

### Sensor Options

Configure sensor behavior with `--opt` flags or in config.json:

```bash
# Show available sensor options
sensorpanel sensor opts

# Examples
sensorpanel run --opt disk.mounts=/,/home --opt network.interface=eth*
```

| Option | Type | Description |
|--------|------|-------------|
| `disk.mounts` | `[]string` | Disk mount points to monitor |
| `network.interface` | `string` | Network interface filter (supports `*` wildcard) |
| `nvidia_gpu.smi_path` | `string` | Custom path to nvidia-smi binary |

### Device Management

```bash
sensorpanel device list      # List connected USB displays
sensorpanel device select    # Interactive device selection
sensorpanel device info      # Show current device and profile info
sensorpanel device create    # Generate code for new device support
sensorpanel device reset     # Reset to defaults
```

### Theme Management

```bash
sensorpanel theme list              # List installed themes
sensorpanel theme create <name>     # Create from React+TypeScript template
sensorpanel theme select <name>     # Set active theme
sensorpanel theme dev [name]        # Start dev server with hot reload
sensorpanel theme build [name]      # Build theme for production
sensorpanel theme preview [name]    # Open in browser
sensorpanel theme delete <name>     # Remove theme
sensorpanel theme path              # Show themes directory
sensorpanel theme sdk update [name] # Update SDK in existing theme
sensorpanel theme browser install   # Download Chrome for Testing
sensorpanel theme browser status    # Check browser availability
sensorpanel theme browser remove    # Remove cached browser
```

### Panel Control

```bash
sensorpanel panel status       # Check if panel is connected
sensorpanel panel test         # Display test pattern
sensorpanel panel on           # Turn backlight on
sensorpanel panel off          # Turn backlight off
sensorpanel panel brightness 5 # Set brightness (0-7)
```

### Sensor Management

```bash
sensorpanel sensor list              # List all registered sensors
sensorpanel sensor list -a           # List only available sensors on this system
sensorpanel sensor opts              # List available sensor options
sensorpanel sensor types             # Generate TypeScript types for all sensors
sensorpanel sensor types -o types.ts # Output to file
sensorpanel sensor create            # Interactive wizard to create a new sensor
```

### Service Management (Autostart)

```bash
sensorpanel service install          # Install as autostart service
sensorpanel service install --opt disk.mounts=/  # With sensor options
sensorpanel service install --music  # Start in now-playing mode
sensorpanel service install --gif https://example.com/animation.gif
sensorpanel service install --image /path/to/wallpaper.png
sensorpanel service uninstall        # Remove autostart service
sensorpanel service start            # Start the service now
sensorpanel service stop             # Stop the service
sensorpanel service status           # Show service status
sensorpanel service logs             # View service logs
sensorpanel service logs -f          # Follow logs in real-time
```

Cross-platform support:
- **Linux**: systemd user service (`~/.config/systemd/user/`)
- **macOS**: launchd LaunchAgent (`~/Library/LaunchAgents/`)
- **Windows**: Startup folder + Registry

Re-running `service install` updates the existing service definition. Stop and
start the service afterward to apply the new command line.

### Other Commands

```bash
sensorpanel benchmark          # Measure FPS performance
sensorpanel prune              # Remove config and cache (keeps themes)
sensorpanel prune --all        # Also remove themes
```

## Adding Device Support

Got a USB display that isn't supported yet? Adding support is easy:

```bash
# Run the interactive wizard
./sensorpanel device create
```

This prompts you for:
- Device name and ID
- USB Vendor ID and Product ID
- Display resolution
- Color format (RGB565/RGB888) and byte order
- Backlight levels

It generates a skeleton Go file in `pkg/device/` that you can customize.

See [docs/adding-devices.md](docs/adding-devices.md) for detailed protocol research tips.

## Adding Custom Sensors

SensorPanel uses a modular sensor provider system. Each sensor is a Go provider that implements the `sensors.Provider` interface.

### Built-in Sensors

| Sensor | Platforms | Description |
|--------|-----------|-------------|
| `cpu` | Linux | CPU load, temperature, frequency |
| `memory` | Linux | RAM usage |
| `disk` | Linux, macOS, Windows | Disk usage per mount point |
| `network` | Linux | Network interface statistics |
| `nvidia_gpu` | Linux | NVIDIA GPU via nvidia-smi |
| `amd_gpu` | Linux | AMD GPU via sysfs |

### Create a Custom Sensor

```bash
# Run the interactive wizard
./sensorpanel sensor create
```

This prompts you for:
- Sensor ID and name
- Target platform (Linux, macOS, Windows, or all)
- Category (system, gpu, storage, network, power)
- Field definitions with types and units

It generates a skeleton Go file in `pkg/sensors/` that you can customize.

### Adding Platform-Specific Implementations

If a sensor already exists but only for certain platforms, running `sensor create` with the same ID will prompt you to add an implementation for a different platform:

```bash
./sensorpanel sensor create
Sensor ID: cpu
Sensor 'cpu' already exists for platforms: linux
Which platform would you like to add?
  1. linux
  2. darwin (macOS)
  3. windows
```

### Update TypeScript Types

After adding or modifying sensors, regenerate the TypeScript types for themes:

```bash
./sensorpanel sensor types -o path/to/theme/lib/sensorpanel/types.ts
```

## Theme Development

Themes are React + TypeScript applications that receive sensor data via WebSocket. A bundled SDK provides React hooks for easy integration.

### Create a theme

```bash
sensorpanel theme create my-theme
```

### Development workflow (single command!)

```bash
# Start everything with one command:
sensorpanel theme dev my-theme

# With sensor options:
sensorpanel theme dev my-theme --opt disk.mounts=/ --opt network.interface=eth*

# This automatically:
# - Detects your package manager (npm/yarn/pnpm/bun)
# - Installs dependencies if needed
# - Starts WebSocket sensor server (port 19847)
# - Starts Vite dev server with HMR (port 15173)
# - Opens your browser
```

### Using the SDK

```tsx
import { useSensorData, useConnectionStatus, formatRate } from "../lib/sensorpanel";

function App() {
  const data = useSensorData();
  const status = useConnectionStatus();

  if (status !== "connected" || !data) {
    return <div>Connecting...</div>;
  }

  return (
    <div>
      <p>CPU: {data.cpu.load.toFixed(0)}%</p>
      <p>GPU: {data.gpu.temperature?.toFixed(0) ?? "--"}°C</p>
      <p>RAM: {data.memory.percent.toFixed(0)}%</p>
    </div>
  );
}
```

### Build and use

```bash
sensorpanel theme build my-theme
sensorpanel theme select my-theme
sensorpanel run
```

See [docs/creating-themes.md](docs/creating-themes.md) for the full guide.

## NixOS Installation

### Add to your flake.nix

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    sensorpanel.url = "github:alperen/sensorpanel";
  };

  outputs = { self, nixpkgs, sensorpanel, ... }: {
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ./configuration.nix
        sensorpanel.nixosModules.default
        {
          services.sensorpanel = {
            enable = true;
            interval = 1.0;
            brightness = 7;
            theme = "my-theme";  # or null for built-in renderer
          };
        }
      ];
    };
  };
}
```

### Module options

```nix
services.sensorpanel = {
  enable = true;
  interval = 1.0;        # Update interval in seconds
  brightness = 7;        # Backlight brightness (0-7)
  theme = null;          # Theme name or null for built-in
  sensorOptions = {      # Sensor-specific options
    "disk.mounts" = [ "/" "/home" ];
    "network.interface" = "eth*";
  };
  user = "sensorpanel";  # Service user
  group = "sensorpanel"; # Service group (for USB access)
};
```

## File Locations

| Type | Path |
|------|------|
| Config | `~/.config/sensorpanel/config.json` |
| Themes | `~/.local/share/sensorpanel/themes/` |
| Browser cache | `~/.cache/sensorpanel/browser/` |

## Architecture

### Device Profiles

Each USB display is supported via a device profile that implements:

```go
type DeviceProfile interface {
    ID() string                           // "qtkeji", "my-device"
    Name() string                         // Human-readable name
    Matches(vid, pid uint16) bool         // USB device matching
    Width() int                           // Display width
    Height() int                          // Display height
    ColorFormat() ColorFormat             // RGB565 or RGB888
    ByteOrder() ByteOrder                 // BigEndian or LittleEndian
    BlitCommand(x, y, w, h, len int) []byte  // Build display command
    BacklightCommand(level int) []byte    // Build backlight command
    ConvertImage(img image.Image) []byte  // Convert to device format
}
```

### Sensor Sources (Linux)

| Metric | Source |
|--------|--------|
| CPU Load | `/proc/stat` |
| CPU Temp | `/sys/class/hwmon/*/temp*_input` |
| CPU Freq | `/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq` |
| GPU (NVIDIA) | `nvidia-smi` |
| GPU (AMD) | `/sys/class/drm/card*/device/` |
| RAM | `/proc/meminfo` |
| Disk | `syscall.Statfs` |
| Network | `/proc/net/dev` |

## Troubleshooting

### Device not found

```bash
# List USB devices
lsusb

# Check if sensorpanel detects it
./sensorpanel device list
```

If your device shows in `lsusb` but not in sensorpanel, it may need a new device profile. Run `sensorpanel device create` to add support.

### Permission denied

Create a udev rule for your device:

```bash
# Replace XXXX and YYYY with your device's VID and PID
sudo tee /etc/udev/rules.d/99-sensorpanel.rules << EOF
SUBSYSTEM=="usb", ATTR{idVendor}=="XXXX", ATTR{idProduct}=="YYYY", MODE="0666"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger
```

On NixOS with the module, udev rules are set up automatically for known devices.

### Theme not rendering

```bash
# Check if browser is installed
sensorpanel theme browser status

# Install browser if needed
sensorpanel theme browser install

# Check theme is built
ls ~/.local/share/sensorpanel/themes/my-theme/dist/
```

### No GPU stats

```bash
# NVIDIA: Check nvidia-smi works
nvidia-smi

# AMD: Check sysfs
ls /sys/class/drm/card*/device/gpu_busy_percent
```

## Development with Mage

SensorPanel uses [Mage](https://magefile.org/) as its build tool. Install it with:

```bash
go install github.com/magefile/mage@latest
```

### Available Targets

```bash
mage -l              # List all targets

mage build           # Build for current platform (default)
mage install         # Build and install to GOPATH/bin
mage test            # Run all tests
mage vet             # Run go vet
mage lint            # Run golangci-lint
mage check           # Run all checks (vet, test, lint)
mage clean           # Remove build artifacts
mage release         # Cross-compile for all platforms (dist/)
mage dev             # Build and run the dashboard
mage devTheme        # Build and start theme dev mode
```

## Testing

SensorPanel has comprehensive unit tests with good coverage across all core packages.

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out    # Summary by function
go tool cover -html=coverage.out    # Interactive HTML report
```

### Coverage by Package

| Package | Coverage | Description |
|---------|----------|-------------|
| `pkg/device` | ~99% | Device profiles and registry |
| `pkg/server` | ~95% | WebSocket server for themes |
| `pkg/paths` | ~77% | XDG directory handling |
| `pkg/sensors` | ~57% | System sensor collection |
| `pkg/panel` | ~50% | USB panel protocol (hardware-dependent) |
| `pkg/config` | ~40% | Configuration and device discovery |
| `pkg/theme` | ~34% | Theme management and building |

Some packages have lower coverage because they interact with hardware (USB devices), external processes (Chromium), or the filesystem in ways that are difficult to test in isolation.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Ways to contribute

- **Add device support** - Run `sensorpanel device create` and submit a PR
- **Create themes** - Share your themes with the community
- **Improve docs** - Help others get started
- **Fix bugs** - Check the issue tracker

## License

MIT License - See LICENSE file for details.
