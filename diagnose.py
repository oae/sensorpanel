#!/usr/bin/env python3
"""
AX206 Protocol Diagnostic Script

Run this with sudo to investigate the USB protocol your device uses.
Usage: sudo python diagnose.py
"""

import sys
import time
import struct

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed. Run: nix develop")
    sys.exit(1)


def hexdump(data, prefix=""):
    """Pretty print hex data."""
    if not data:
        return f"{prefix}(empty)"
    return f"{prefix}{bytes(data).hex()}"


def test_write_read(dev, ep_out, ep_in, data, description, timeout=1000):
    """Try to write data and read response."""
    print(f"\n[TEST] {description}")
    print(f"  Sending: {data.hex()}")
    
    try:
        written = ep_out.write(data, timeout=timeout)
        print(f"  Wrote {written} bytes")
    except usb.core.USBError as e:
        print(f"  Write ERROR: {e}")
        return None
    
    time.sleep(0.05)
    
    try:
        response = ep_in.read(64, timeout=timeout)
        print(f"  Response ({len(response)} bytes): {bytes(response).hex()}")
        return bytes(response)
    except usb.core.USBError as e:
        print(f"  Read: {e}")
        return None


def build_scsi_cbw(cmd_bytes, data_len=0, direction=0x80, tag=0xDEADBEEF):
    """Build SCSI Command Block Wrapper."""
    cmd_padded = cmd_bytes.ljust(16, b'\x00')
    return struct.pack('<4sIIBBB16s',
        b'USBC',           # Signature
        tag,               # Tag
        data_len,          # Data Transfer Length
        direction,         # Flags (0x80 = IN, 0x00 = OUT)
        0,                 # LUN
        len(cmd_bytes),    # CB Length
        cmd_padded         # Command Block
    )


def main():
    print("=" * 60)
    print("AX206 Protocol Diagnostic Tool")
    print("=" * 60)
    
    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("\nERROR: No AX206 device found (1908:0102)")
        print("Make sure the device is connected.")
        sys.exit(1)
    
    print(f"\nDevice found!")
    print(f"  Manufacturer: {dev.manufacturer}")
    print(f"  Product: {dev.product}")
    print(f"  Serial: {dev.serial_number}")
    print(f"  Bus {dev.bus}, Device {dev.address}")
    
    # Detach kernel driver
    try:
        if dev.is_kernel_driver_active(0):
            print("\nDetaching kernel driver...")
            dev.detach_kernel_driver(0)
    except Exception as e:
        print(f"Kernel driver note: {e}")
    
    # Set configuration
    try:
        dev.set_configuration()
        print("Configuration set.")
    except usb.core.USBError as e:
        print(f"Configuration note: {e}")
    
    # Get endpoints
    cfg = dev.get_active_configuration()
    intf = cfg[(0, 0)]
    
    ep_out = usb.util.find_descriptor(
        intf,
        custom_match=lambda e: usb.util.endpoint_direction(e.bEndpointAddress) == usb.util.ENDPOINT_OUT
    )
    ep_in = usb.util.find_descriptor(
        intf,
        custom_match=lambda e: usb.util.endpoint_direction(e.bEndpointAddress) == usb.util.ENDPOINT_IN
    )
    
    print(f"\nEndpoints:")
    print(f"  OUT: {hex(ep_out.bEndpointAddress)}, max packet: {ep_out.wMaxPacketSize}")
    print(f"  IN:  {hex(ep_in.bEndpointAddress)}, max packet: {ep_in.wMaxPacketSize}")
    
    print("\n" + "=" * 60)
    print("PROTOCOL TESTS")
    print("=" * 60)
    
    # Test 1: SCSI CBW with dpf-ax GET_PARAM command
    print("\n--- Test Group 1: SCSI CBW Format (dpf-ax style) ---")
    
    cmd = bytes([0xCD, 0x00, 0x02])  # GET_PARAM
    cbw = build_scsi_cbw(cmd, data_len=5, direction=0x80)
    test_write_read(dev, ep_out, ep_in, cbw, "SCSI CBW: GET_PARAM (0xCD 0x00 0x02)")
    
    # Clear any stall
    time.sleep(0.2)
    
    cmd = bytes([0xCD, 0x00, 0x03])  # PROBE
    cbw = build_scsi_cbw(cmd, data_len=1, direction=0x80)
    test_write_read(dev, ep_out, ep_in, cbw, "SCSI CBW: PROBE (0xCD 0x00 0x03)")
    
    # Test 2: Raw vendor commands (no SCSI wrapper)
    print("\n--- Test Group 2: Raw Commands (no SCSI wrapper) ---")
    
    raw_cmds = [
        (bytes([0x00]), "Null command"),
        (bytes([0x06]), "GetInfo variant 1"),
        (bytes([0x40, 0x00, 0x00, 0x00]), "Control transfer style"),
        (bytes([0xCD, 0x00, 0x02]), "Raw dpf-ax GET_PARAM"),
    ]
    
    for cmd, desc in raw_cmds:
        test_write_read(dev, ep_out, ep_in, cmd, f"Raw: {desc}")
        time.sleep(0.2)
    
    # Test 3: Try to identify the exact protocol by sending a small image
    print("\n--- Test Group 3: Image Blit Test ---")
    
    # Create a 16x16 red square in RGB565
    width, height = 16, 16
    red_rgb565 = struct.pack('<H', (31 << 11))  # Red in RGB565
    image_data = red_rgb565 * (width * height)
    
    # dpf-ax blit command format
    x0, y0, x1, y1 = 0, 0, width - 1, height - 1
    blit_cmd = bytes([
        0xCD, 0x00, 0x12,  # BLIT command
        x0 & 0xFF, (x0 >> 8) & 0xFF,
        y0 & 0xFF, (y0 >> 8) & 0xFF,
        x1 & 0xFF, (x1 >> 8) & 0xFF,
        y1 & 0xFF, (y1 >> 8) & 0xFF,
    ])
    
    cbw = build_scsi_cbw(blit_cmd, data_len=len(image_data), direction=0x00)
    
    print(f"\n[TEST] SCSI CBW: BLIT 16x16 red square")
    print(f"  CBW: {cbw.hex()}")
    print(f"  Image data: {len(image_data)} bytes")
    
    try:
        # Send CBW
        written = ep_out.write(cbw, timeout=1000)
        print(f"  CBW written: {written} bytes")
        
        # Send image data
        written = ep_out.write(image_data, timeout=1000)
        print(f"  Image written: {written} bytes")
        
        print("  SUCCESS: No error sending image data!")
        print("  Check your display - do you see a small red square in the top-left?")
        
    except usb.core.USBError as e:
        print(f"  ERROR: {e}")
    
    # Test 4: Alternative blit format (some devices)
    print("\n--- Test Group 4: Alternative Blit Format ---")
    
    # Some devices use a different header format
    alt_header = bytes([
        0x00,  # Command type
        0x22,  # Write frame buffer
    ]) + struct.pack('<HHHH', x0, y0, x1, y1)
    
    test_write_read(dev, ep_out, ep_in, alt_header + image_data[:64], 
                    "Alt format: 0x00 0x22 + coords + data")
    
    print("\n" + "=" * 60)
    print("DIAGNOSTIC COMPLETE")
    print("=" * 60)
    print("""
Please report the output above. Key questions:
1. Did any command get a response?
2. Did the red square appear on the display?
3. Were there any successful writes without pipe errors?
""")


if __name__ == "__main__":
    main()
