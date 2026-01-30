# AX206 Sensor Panel for NixOS

A complete NixOS solution for driving AX206-based USB LCD displays as real-time system monitoring dashboards.

![Dashboard Example](docs/dashboard-preview.png)

## Features

- **Pure Python implementation** - No C compilation required, uses pyusb
- **Real-time monitoring** - CPU, GPU (NVIDIA), RAM, disk, and network stats
- **NixOS native** - Flake-based with proper module, udev rules, and systemd service
- **Themeable** - Dark and light themes included
- **Headless rendering** - Works without X11/Wayland (uses Pillow)
- **Auto-reconnect** - Handles device disconnection gracefully

## Quick Start

### 1. Add to your flake.nix

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    ax206-display.url = "path:/home/alperen/code/personal/sensorpanel";
    # Or from GitHub: ax206-display.url = "github:yourusername/sensorpanel";
  };

  outputs = { self, nixpkgs, ax206-display, ... }: {
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ./configuration.nix
        ax206-display.nixosModules.default
        {
          services.ax206Display = {
            enable = true;
            interval = "2";
            theme = "dark";
            gpu.method = "nvidia";
          };
        }
      ];
    };
  };
}
```

### 2. Rebuild and test

```bash
sudo nixos-rebuild switch

# Check service status
systemctl status ax206-display

# View logs
journalctl -u ax206-display -f
```

## Configuration Options

### Full Configuration Example

```nix
services.ax206Display = {
  enable = true;
  
  # Update interval in seconds
  interval = "2";
  
  # Theme: "dark" or "light"
  theme = "dark";
  
  # Display rotation: 0, 90, 180, 270
  rotation = 0;
  
  # GPU settings
  gpu = {
    enable = true;
    method = "nvidia";  # "nvidia", "amd", "auto", "none"
  };
  
  # Metrics to display
  metrics = {
    cpu = true;
    ram = true;
    disk = true;
    network = true;
  };
  
  # Disk mount points to monitor
  diskMounts = [ "/" "/home" ];
  
  # Network interface (supports glob patterns)
  networkInterface = "enp*";  # or "eth0", "*" for auto
  
  # Service user/group (usually leave as default)
  user = "axdisplay";
  group = "axdisplay";
};
```

## Manual Testing

### Development Shell

```bash
# Enter development environment
nix develop

# List connected devices
python -m axdisplay --list

# Display test pattern
python -m axdisplay --test

# Run single update (for debugging)
python -m axdisplay --once

# Render preview without device
python -m axdisplay --preview output.png

# Run daemon manually
python -m axdisplay -v
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AXDISPLAY_INTERVAL` | Update interval (seconds) | `2` |
| `AXDISPLAY_THEME` | Theme name | `dark` |
| `AXDISPLAY_ROTATION` | Rotation degrees | `0` |
| `AXDISPLAY_GPU_METHOD` | GPU monitoring method | `nvidia` |
| `AXDISPLAY_SHOW_CPU` | Show CPU metrics | `1` |
| `AXDISPLAY_SHOW_GPU` | Show GPU metrics | `1` |
| `AXDISPLAY_SHOW_RAM` | Show RAM metrics | `1` |
| `AXDISPLAY_SHOW_DISK` | Show disk metrics | `1` |
| `AXDISPLAY_SHOW_NETWORK` | Show network metrics | `1` |
| `AXDISPLAY_DISK_MOUNTS` | Comma-separated mounts | `/` |
| `AXDISPLAY_NETWORK_IF` | Interface pattern | `*` |

## Verification Checklist

### Step 1: Verify USB Device Detection

```bash
# Check if device is detected
lsusb | grep 1908:0102

# Expected output:
# Bus 003 Device 010: ID 1908:0102 GEMBIRD Digital Photo Frame
```

### Step 2: Verify udev Rules

After `nixos-rebuild switch`:

```bash
# Check udev rules are loaded
cat /etc/udev/rules.d/99-local.rules | grep 1908

# Trigger udev reload (if needed)
sudo udevadm control --reload-rules
sudo udevadm trigger

# Check device permissions
ls -la /dev/bus/usb/003/010  # Adjust bus/device numbers

# Expected: group should be 'axdisplay'
# crw-rw---- 1 root axdisplay ... /dev/bus/usb/003/010
```

### Step 3: Test USB Communication

```bash
# Enter dev shell
nix develop

# List devices (tests basic USB access)
python -m axdisplay --list

# Expected: Device details shown
```

### Step 4: Display Test Pattern

```bash
# Display test pattern
python -m axdisplay --test

# You should see colored bars on the display
```

### Step 5: Test Full Dashboard

```bash
# Single update
python -m axdisplay --once

# Continuous updates
python -m axdisplay -v

# Press Ctrl+C to stop
```

### Step 6: Enable Systemd Service

```bash
# Start service
sudo systemctl start ax206-display

# Check status
systemctl status ax206-display

# View logs
journalctl -u ax206-display -f

# Enable on boot
sudo systemctl enable ax206-display
```

## Troubleshooting

### Device Not Found

1. **Check USB connection:**
   ```bash
   lsusb | grep 1908
   ```

2. **Check dmesg for errors:**
   ```bash
   dmesg | tail -50
   ```

3. **Verify udev rules are applied:**
   ```bash
   sudo udevadm test /sys/bus/usb/devices/3-1  # Adjust path
   ```

4. **Ensure user is in axdisplay group:**
   ```bash
   groups
   # Should include 'axdisplay'
   ```

### Permission Denied

1. **Add yourself to the group:**
   ```nix
   users.users.yourusername.extraGroups = [ "axdisplay" ];
   ```
   Then logout/login or run `newgrp axdisplay`.

2. **Check device permissions:**
   ```bash
   ls -la /dev/bus/usb/003/010
   ```

### No GPU Stats

1. **Check nvidia-smi works:**
   ```bash
   nvidia-smi
   ```

2. **Ensure NVIDIA drivers are installed:**
   ```nix
   services.xserver.videoDrivers = [ "nvidia" ];
   ```

3. **Try disabling GPU monitoring:**
   ```nix
   services.ax206Display.gpu.enable = false;
   ```

### Display Shows Wrong Resolution

The device should auto-detect resolution, but you can override:

```bash
AXDISPLAY_WIDTH=320 AXDISPLAY_HEIGHT=240 python -m axdisplay
```

### Display Shows Garbage / Wrong Colors

Your device might use a different byte order. Try opening an issue with:
```bash
lsusb -d 1908:0102 -v
```

## File Structure

```
sensorpanel/
├── flake.nix              # Nix flake definition
├── pyproject.toml         # Python package config
├── README.md              # This file
└── axdisplay/             # Python package
    ├── __init__.py        # Package init
    ├── __main__.py        # CLI entry point
    ├── config.py          # Configuration
    ├── device.py          # USB device interface
    ├── protocol.py        # AX206 protocol implementation
    ├── sensors.py         # System metrics collection
    ├── renderer.py        # Dashboard rendering
    └── themes/            # Theme definitions
        ├── __init__.py
        ├── dark.py
        └── light.py
```

## Technical Details

### USB Protocol

- **Vendor ID:** 0x1908
- **Product ID:** 0x0102
- **Interface:** USB Bulk transfers with SCSI CBW wrapper
- **Color Format:** RGB565 (16-bit, little-endian)
- **Typical Resolution:** 320x240 (landscape)

### Supported Devices

Any AX206-based USB digital photo frame should work, including:
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
| GPU | `nvidia-smi` or `/sys/class/drm/card*/device/` |
| RAM | `/proc/meminfo` |
| Disk | `os.statvfs()` |
| Network | `/proc/net/dev` |

## License

MIT License - See LICENSE file for details.

## Credits

- Protocol implementation based on [dpf-ax](https://github.com/dreamlayers/dpf-ax)
- Inspired by AIDA64 sensor panel
