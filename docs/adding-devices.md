# Adding Support for New Devices

This guide explains how to add support for a new USB display device.

## Quick Start

Run the device creation wizard:

```bash
./sensorpanel device create
```

Follow the prompts to generate a skeleton device profile.

## Understanding Device Profiles

A device profile implements the `DeviceProfile` interface defined in `pkg/device/profile.go`.

Key methods you need to implement:

| Method | Purpose |
|--------|---------|
| `ID()` | Unique identifier (e.g., "qtkeji") |
| `Matches(vid, pid)` | Return true if this profile supports the device |
| `Width()`, `Height()` | Display resolution |
| `ColorFormat()` | RGB565 or RGB888 |
| `ByteOrder()` | BigEndian or LittleEndian |
| `BlitCommand()` | Build command bytes to send image data |
| `BacklightCommand()` | Build command bytes for backlight control |
| `ConvertImage()` | Convert Go image to device pixel format |

## Protocol Research

Before implementing, you need to understand your device's USB protocol:

### 1. Identify Your Device

```bash
lsusb
# Find your device, note VID:PID
lsusb -d XXXX:YYYY -v
```

### 2. Capture USB Traffic

Use Wireshark with USBPcap (Windows) or usbmon (Linux):

1. Install Wireshark with USB capture support
2. Run the manufacturer's software that drives the display
3. Capture the USB traffic
4. Analyze the packets

### 3. Key Things to Look For

- **Command structure**: Most devices use SCSI CBW or custom bulk transfers
- **Pixel format**: RGB565 (16-bit) or RGB888 (24-bit)
- **Byte order**: Big-endian or little-endian
- **Image transfer**: How coordinates and dimensions are encoded
- **Backlight control**: Command bytes for brightness levels

## Example Implementation

See `pkg/device/qtkeji.go` for a complete example:

```go
type QTKeJiProfile struct{}

func (p *QTKeJiProfile) ID() string { return "qtkeji" }

func (p *QTKeJiProfile) Matches(vid, pid uint16) bool {
    return vid == 0x1908 && (pid == 0x0102 || pid == 0x0103)
}

func (p *QTKeJiProfile) BlitCommand(x, y, w, h int, dataLen int) []byte {
    // Build SCSI CBW with vendor command
    cmd := make([]byte, 16)
    cmd[0] = 0xCD  // Vendor prefix
    // ... fill in command bytes
    return BuildCBW(cmd, dataLen, DirOut)
}
```

## Testing

1. Build: `go build .`
2. List devices: `./sensorpanel device list`
3. Select your device: `./sensorpanel device select`
4. Test pattern: `./sensorpanel panel test`
5. Run dashboard: `./sensorpanel run`

## Common Protocol Patterns

### SCSI Vendor Commands

Many USB displays use SCSI mass storage protocol with vendor-specific commands:

```
CBW (Command Block Wrapper):
- Signature: 0x55534243 ("USBC")
- Tag: Unique ID for matching response
- Data length: Size of image data
- Flags: Direction (0x00 = out, 0x80 = in)
- LUN: Usually 0
- Command length: 16 bytes
- Command: Vendor-specific bytes
```

### Direct Bulk Transfers

Some devices use simpler bulk transfers:
- Send command header
- Send image data
- Wait for acknowledgment

## Submitting Your Profile

1. Fork the repository
2. Add your profile in `pkg/device/`
3. Register it in `pkg/device/registry.go`
4. Add yourself to CONTRIBUTORS.md
5. Submit a pull request with:
   - Device name and manufacturer
   - Link to purchase (if available)
   - Brief description of protocol

## Need Help?

- Open an issue with the "new-device" label
- Include USB descriptor output (`lsusb -v`)
- Share any USB captures if possible
