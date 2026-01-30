# AX206 USB Display Project - Session State (January 30, 2026)

## Project Location
- Windows: `C:\Users\alperen\code\sensorpanel\`
- Linux: `/home/alperen/code/sensorpanel/`

## Device Information
```
Manufacturer: QTKeJi.Ltd
Product: USB-Display
VID:PID: 1908:0102
Resolution: 480x320
Color Format: RGB565 (16-bit, BIG-ENDIAN byte order!)
Image Size: 307,200 bytes (480 × 320 × 2)
Endpoints: Bulk OUT 0x01, Bulk IN 0x81
```

## STATUS: FULLY WORKING ON ARCH LINUX! ✓

### Confirmed Working
- ✅ Device detection and claiming
- ✅ Full-screen BLIT command
- ✅ Dashboard with CPU, GPU, RAM, Disk, Network metrics
- ✅ Correct colors (RGB565 big-endian)
- ✅ No tearing (display has internal buffer)

## Protocol Summary (QTKeJi/AIDA64 Format)

### BLIT Command (write image to screen)
```
CBW (31 bytes):
  Bytes 0-3:   'USBC' signature
  Bytes 4-7:   Tag (0xDEADBEEF)
  Bytes 8-11:  Data length = 0x0004b000 (307,200 bytes)
  Byte 12:     Direction = 0x00 (OUT)
  Byte 13:     LUN = 0
  Byte 14:     CB Length = 16
  Bytes 15-30: Command block (16 bytes)

Command Block (16 bytes):
  Byte 0:      0xCD (vendor prefix)
  Bytes 1-4:   0x00 (reserved)
  Byte 5:      0x06 (BLIT operation type)
  Byte 6:      0x12 (write image subcommand)
  Bytes 7-10:  0x00 (reserved/offset - not used for full-screen)
  Bytes 11-12: width - 1 (little-endian) = 0x01DF (479)
  Bytes 13-14: height - 1 (little-endian) = 0x013F (319)
  Byte 15:     0x00

Full CBW hex:
55534243 efbeadde 00b00400 00 00 10 cd00000000061200000000df013f0100

Then send 307,200 bytes of RGB565 pixel data.
Then read 13-byte CSW response.
```

### CSW Response (13 bytes)
```
Bytes 0-3:   'USBS' signature
Bytes 4-7:   Tag (echoed from CBW)
Bytes 8-11:  Data residue (0 if all data transferred)
Byte 12:     Status (0 = success)
```

### Key Findings from USB Capture Analysis
- AIDA64 uses **full-screen updates only** (no partial updates)
- Refresh rate: ~1.3 seconds per frame
- Image data split across 2 USB transfers: 256KB + 44KB
- No tearing - display has internal framebuffer
- No special "vsync" command - display updates after CSW
- **IMPORTANT: Pixel data uses BIG-ENDIAN byte order** (not little-endian!)
- Device does NOT support GET_PARAMS or PROBE commands (skip them)

## Arch Linux Setup

### 1. Install dependencies
```bash
sudo pacman -S python python-pyusb python-pillow libusb
```

### 2. Set up udev rule (one-time, allows non-root access)
```bash
echo 'SUBSYSTEM=="usb", ATTR{idVendor}=="1908", ATTR{idProduct}=="0102", MODE="0666"' | sudo tee /etc/udev/rules.d/99-ax206.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
```

### 3. Run tests
```bash
cd /path/to/sensorpanel

# Quick BLIT test (4-color quadrants)
python test_blit_only.py

# Test pattern with color bars
python -m axdisplay --test

# Full dashboard with sensor data
python -m axdisplay
```

## Project Files

### Working Test Scripts
- `test_blit_only.py` - **USE THIS** - Minimal working BLIT test
- `test_dpfax.py` - Full protocol test (probe/get_params may timeout)

### Main Package (`axdisplay/`)
- `protocol.py` - USB protocol implementation
- `device.py` - Device interface with CBW/CSW handling  
- `sensors.py` - System metrics collection
- `renderer.py` - Dashboard rendering
- `config.py` - Configuration
- `__main__.py` - CLI entry point

### Configuration (flake.nix)
- NixOS module for systemd service
- Works on NixOS with `nix develop`

## Troubleshooting

### Device not found
```bash
lsusb | grep 1908
# Should show: Bus XXX Device XXX: ID 1908:0102 ...
```

### Permission denied
```bash
# Either use sudo:
sudo python test_blit_only.py

# Or set up udev rule and replug device
```

### Interface claim error
- Make sure AIDA64 or other software isn't using the device
- Unplug and replug the USB device

### CSW timeout on probe command
- This is normal for QTKeJi devices
- Use `test_blit_only.py` which skips probe and goes straight to BLIT

## Next Steps on Arch Linux

1. Run `python test_blit_only.py` - verify 4-color pattern appears
2. Run `python -m axdisplay --test` - verify test pattern
3. Run `python -m axdisplay` - run full dashboard
4. Create systemd service for auto-start (optional)

## RGB565 Color Reference
```python
# Standard RGB565 values
RED   = 0xF800  # 5 bits red, 0 green, 0 blue
GREEN = 0x07E0  # 0 red, 6 bits green, 0 blue  
BLUE  = 0x001F  # 0 red, 0 green, 5 bits blue
WHITE = 0xFFFF
BLACK = 0x0000

# Conversion from RGB888 to RGB565:
rgb565 = ((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3)

# IMPORTANT: Send as BIG-ENDIAN bytes!
# For 0xF800 (red): send bytes 0xF8, 0x00 (not 0x00, 0xF8)
import struct
pixel_bytes = struct.pack('>H', rgb565)  # Big-endian
```

## Files in This Project
```
sensorpanel/
├── flake.nix              # Nix flake (NixOS module)
├── pyproject.toml         # Python package config
├── SESSION_STATE.md       # This file
├── test_blit_only.py      # ✓ Working minimal test
├── test_dpfax.py          # Full protocol test
├── usb_capture.pcapng     # USB traffic capture from AIDA64
├── axdisplay/
│   ├── __init__.py
│   ├── __main__.py        # CLI entry point
│   ├── config.py
│   ├── protocol.py        # USB protocol
│   ├── device.py          # Device interface
│   ├── sensors.py         # System metrics
│   ├── renderer.py        # Dashboard rendering
│   └── themes/
│       ├── __init__.py
│       ├── dark.py
│       └── light.py
└── docs/
    └── dashboard-preview.png
```
