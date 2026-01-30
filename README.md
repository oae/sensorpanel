# SensorPanel

A cross-platform CLI tool for driving AX206-based USB LCD displays as real-time system monitoring dashboards.

![Dashboard Example](docs/dashboard-preview.png)

## Features

- **Go implementation** - Single binary, no dependencies at runtime
- **Real-time monitoring** - CPU, GPU (NVIDIA/AMD), RAM, disk, and network stats
- **Web-based themes** - Create custom themes using React/HTML/CSS
- **Headless rendering** - Auto-downloads Chrome for Testing to render themes
- **NixOS support** - Flake with module, udev rules, and systemd service
- **XDG compliance** - Config in `~/.config/`, themes in `~/.local/share/`

## Quick Start

### 1. Build

```bash
# With Nix
nix build

# Or with Go
go build .
```

### 2. Select your display

```bash
./sensorpanel device select
```

### 3. Run the dashboard

```bash
# With built-in renderer
./sensorpanel run

# Or create and use a custom theme
./sensorpanel theme create my-theme
./sensorpanel theme select my-theme
./sensorpanel run
```

## Commands

### Run Dashboard

```bash
sensorpanel run [flags]

Flags:
  -i, --interval float   Update interval in seconds (default 1.0)
  -b, --brightness int   Backlight brightness 0-7 (default 7)
  -m, --mounts strings   Disk mount points to monitor (default [/])
      --cpu              Show CPU stats (default true)
      --gpu              Show GPU stats (default true)
      --ram              Show RAM stats (default true)
      --disk             Show disk stats (default true)
      --network          Show network stats (default true)
```

### Device Management

```bash
sensorpanel device list      # List available USB displays
sensorpanel device select    # Interactive device selection
sensorpanel device info      # Show current device config
sensorpanel device reset     # Reset to defaults
```

### Theme Management

```bash
sensorpanel theme list              # List installed themes
sensorpanel theme create <name>     # Create from React template
sensorpanel theme select <name>     # Set active theme
sensorpanel theme preview [name]    # Open in browser
sensorpanel theme delete <name>     # Remove theme
sensorpanel theme path              # Show themes directory
sensorpanel theme dev               # Start dev server with live sensor data
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

### Benchmark

```bash
sensorpanel benchmark          # Measure FPS performance
```

### Cleanup

```bash
sensorpanel prune              # Remove config and cache (keeps themes)
sensorpanel prune --dry-run    # Show what would be deleted
sensorpanel prune --all        # Also remove themes
```

## Theme Development

Themes are React applications that receive sensor data via WebSocket.

### Create a theme

```bash
sensorpanel theme create my-theme
cd ~/.local/share/sensorpanel/themes/my-theme
```

### Development workflow

```bash
# Terminal 1: Start sensor data server
sensorpanel theme dev
# Shows: WebSocket server running on port XXXXX

# Terminal 2: Start React dev server
cd ~/.local/share/sensorpanel/themes/my-theme
npm run dev
# Open: http://localhost:3000?ws=XXXXX
```

### Build and use

```bash
cd ~/.local/share/sensorpanel/themes/my-theme
npm run build
sensorpanel theme select my-theme
sensorpanel run
```

### Sensor data format

The WebSocket sends JSON with this structure:

```json
{
  "cpu": {
    "temperature": 65.0,
    "load": 45.2,
    "frequency": 3800
  },
  "gpu": {
    "temperature": 70.0,
    "load": 80.0,
    "memory_used": 4096,
    "memory_total": 8192,
    "power": 150.0
  },
  "ram": {
    "used": 16384,
    "total": 32768,
    "percent": 50.0
  },
  "disk": {
    "/": {"used": 100, "total": 500, "percent": 20.0},
    "/home": {"used": 200, "total": 1000, "percent": 20.0}
  },
  "network": {
    "eth0": {"rx_rate": 1024000, "tx_rate": 512000}
  }
}
```

## NixOS Installation

### Add to your flake.nix

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    sensorpanel.url = "github:yourusername/sensorpanel";
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
  diskMounts = [ "/" ];  # Mount points to monitor
  user = "sensorpanel";  # Service user
  group = "sensorpanel"; # Service group (for USB access)
};
```

### Manual setup

```bash
# Rebuild system
sudo nixos-rebuild switch

# Check service status
systemctl status sensorpanel

# View logs
journalctl -u sensorpanel -f
```

## File Locations

| Type | Path |
|------|------|
| Config | `~/.config/sensorpanel/config.json` |
| Themes | `~/.local/share/sensorpanel/themes/` |
| Browser cache | `~/.cache/sensorpanel/browser/` |

## Technical Details

### USB Protocol

- **Vendor ID:** 0x1908
- **Product ID:** 0x0102
- **Interface:** USB Bulk transfers with SCSI CBW wrapper
- **Color Format:** RGB565 (16-bit, big-endian)
- **Resolution:** 480x320 (landscape)

### Supported Devices

Any AX206-based USB digital photo frame, including:
- AIDA64-compatible USB displays
- GEMBIRD Digital Photo Frame
- Pearl brand frames
- Various Coby models
- Generic "USB-Display" devices

### Sensor Sources

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
# Check USB connection
lsusb | grep 1908

# Expected output:
# Bus 003 Device 010: ID 1908:0102 GEMBIRD Digital Photo Frame
```

### Permission denied

On NixOS with the module, the udev rules are set up automatically. Otherwise:

```bash
# Create udev rule
sudo tee /etc/udev/rules.d/99-sensorpanel.rules << EOF
SUBSYSTEM=="usb", ATTR{idVendor}=="1908", ATTR{idProduct}=="0102", MODE="0666"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger
```

### Theme not rendering

```bash
# Check if browser is installed
sensorpanel theme browser status

# Install browser
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

## License

MIT License - See LICENSE file for details.
