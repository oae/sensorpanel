#!/usr/bin/env python3
"""
Minimal BLIT test - Skip probe/get_params, just send image data.

Based on exact USB capture from AIDA64:
- CBW: 55534243 efbeadde 00b00400 00 00 10 cd0000000006120000000000df013f0100
- Then 307200 bytes of RGB565 data
- Then CSW response
"""

import sys
import struct
import time

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    print("Install: pip install pyusb (or: sudo pacman -S python-pyusb)")
    sys.exit(1)

# USB constants
ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81

# Display dimensions (known from previous tests)
WIDTH = 480
HEIGHT = 320


def main():
    print("=" * 60)
    print("Minimal BLIT Test")
    print("=" * 60)

    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("ERROR: Device not found (1908:0102)")
        print("\nCheck: lsusb | grep 1908")
        sys.exit(1)

    print(f"\nDevice found: {dev.manufacturer} {dev.product}")

    # Detach kernel driver if needed
    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
            print("Detached kernel driver")
    except Exception as e:
        pass  # Not supported on Windows

    # Set configuration and claim interface
    try:
        dev.set_configuration()
    except usb.core.USBError:
        pass  # Already configured

    try:
        usb.util.claim_interface(dev, 0)
        print("Interface claimed")
    except usb.core.USBError as e:
        print(f"ERROR: Could not claim interface: {e}")
        print("\nMake sure no other program (like AIDA64) is using the device")
        sys.exit(1)

    # Build test image - solid colors in quadrants
    print(f"\nCreating test image ({WIDTH}x{HEIGHT})...")

    pixel_count = WIDTH * HEIGHT
    buffer = bytearray(pixel_count * 2)

    # RGB565 colors in BIG-ENDIAN format (this device requires big-endian)
    # RGB565: R=bits 15-11, G=bits 10-5, B=bits 4-0
    RED = struct.pack(">H", 0xF800)  # Pure red
    GREEN = struct.pack(">H", 0x07E0)  # Pure green
    BLUE = struct.pack(">H", 0x001F)  # Pure blue
    WHITE = struct.pack(">H", 0xFFFF)  # White

    for i in range(pixel_count):
        y = i // WIDTH
        x = i % WIDTH

        # Quadrant pattern
        if y < HEIGHT // 2:
            color = RED if x < WIDTH // 2 else BLUE
        else:
            color = GREEN if x < WIDTH // 2 else WHITE

        buffer[i * 2 : i * 2 + 2] = color

    # Build CBW exactly as captured from AIDA64
    # CBW structure:
    # - 'USBC' (4 bytes)
    # - Tag: 0xDEADBEEF (4 bytes)
    # - Data length: 307200 = 0x0004b000 (4 bytes, little-endian)
    # - Flags: 0x00 (OUT direction)
    # - LUN: 0x00
    # - CB Length: 0x10 (16)
    # - Command: cd 00 00 00 00 06 12 00 00 00 00 df 01 3f 01 00

    data_length = len(buffer)  # 307200

    # Build command block (16 bytes)
    cmd = bytearray(16)
    cmd[0] = 0xCD  # Vendor prefix
    cmd[5] = 0x06  # BLIT operation
    cmd[6] = 0x12  # Write image subcommand
    cmd[11] = (WIDTH - 1) & 0xFF  # 479 low byte = 0xDF
    cmd[12] = ((WIDTH - 1) >> 8) & 0xFF  # 479 high byte = 0x01
    cmd[13] = (HEIGHT - 1) & 0xFF  # 319 low byte = 0x3F
    cmd[14] = ((HEIGHT - 1) >> 8) & 0xFF  # 319 high byte = 0x01

    # Build full CBW (31 bytes)
    cbw = struct.pack(
        "<4sIIBBB",
        b"USBC",  # Signature
        0xDEADBEEF,  # Tag
        data_length,  # Data transfer length
        0x00,  # Flags (OUT)
        0x00,  # LUN
        16,  # CB Length
    ) + bytes(cmd)

    print(f"CBW ({len(cbw)} bytes): {cbw.hex()}")
    print(f"Command block: {bytes(cmd).hex()}")
    print(f"Data length: {data_length} bytes")

    # Send CBW
    print("\nSending CBW...")
    try:
        written = dev.write(ENDPOINT_OUT, cbw, timeout=1000)
        print(f"  CBW sent: {written} bytes")
    except usb.core.USBError as e:
        print(f"  ERROR sending CBW: {e}")
        usb.util.release_interface(dev, 0)
        sys.exit(1)

    # Send image data
    print("Sending image data...")
    try:
        written = dev.write(ENDPOINT_OUT, buffer, timeout=10000)
        print(f"  Image data sent: {written} bytes")
    except usb.core.USBError as e:
        print(f"  ERROR sending image data: {e}")
        usb.util.release_interface(dev, 0)
        sys.exit(1)

    # Read CSW
    print("Reading CSW...")
    try:
        csw = dev.read(ENDPOINT_IN, 13, timeout=5000)
        csw = bytes(csw)
        print(f"  CSW ({len(csw)} bytes): {csw.hex()}")

        if csw[:4] == b"USBS":
            status = csw[12]
            print(f"  Status: {status} {'(SUCCESS)' if status == 0 else '(FAILED)'}")
        else:
            print(f"  WARNING: Invalid CSW signature")
    except usb.core.USBError as e:
        print(f"  CSW read error (may still have worked): {e}")

    # Cleanup
    usb.util.release_interface(dev, 0)

    print("\n" + "=" * 60)
    print("CHECK THE DISPLAY!")
    print("Expected: Red (top-left), Blue (top-right)")
    print("          Green (bottom-left), White (bottom-right)")
    print("=" * 60)


if __name__ == "__main__":
    main()
