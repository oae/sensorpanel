#!/usr/bin/env python3
"""
AX206 Protocol Test - Based on dpf-ax implementation

This script implements the exact dpf-ax protocol from:
https://github.com/dreamlayers/dpf-ax/blob/master/dpflib/rawusb.c

The key insight from dpf-ax:
1. Send 31-byte CBW (USBC...)
2. Send/receive data if needed  
3. Read 13-byte CSW (USBS...) response

This is the standard USB Mass Storage Bulk-Only Transport protocol.
"""

import sys
import struct
import time

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    print("Run: nix develop")
    sys.exit(1)


# USB endpoints
ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81

# CBW/CSW constants
CBW_SIGNATURE = b'USBC'
CSW_SIGNATURE = b'USBS'


def build_cbw(cmd: bytes, data_length: int = 0, direction: int = 0x00, tag: int = 0xDEADBEEF) -> bytes:
    """
    Build Command Block Wrapper exactly as dpf-ax does.
    
    From rawusb.c g_buf:
    0x55, 0x53, 0x42, 0x43,  // dCBWSignature = 'USBC'
    0xde, 0xad, 0xbe, 0xef,  // dCBWTag
    0x00, 0x80, 0x00, 0x00,  // dCBWLength (little-endian)
    0x00,                     // bmCBWFlags (0x80 = IN, 0x00 = OUT)
    0x00,                     // bCBWLUN
    0x10,                     // bCBWCBLength (16)
    + 16 bytes command
    = 31 bytes total
    """
    if len(cmd) > 16:
        cmd = cmd[:16]
    padded_cmd = cmd.ljust(16, b'\x00')
    
    cbw = struct.pack(
        '<4sIIBBB',
        CBW_SIGNATURE,
        tag,
        data_length,
        direction,
        0,  # LUN
        len(cmd)
    ) + padded_cmd
    
    return cbw


def emulate_scsi(dev, cmd: bytes, direction_out: bool, data: bytes = None, data_length: int = 0, timeout: int = 1000) -> tuple:
    """
    Emulate SCSI command exactly as dpf-ax rawusb.c emulate_scsi() does.
    
    Returns: (received_data, status_code)
    """
    # Build CBW
    dir_flag = 0x00 if direction_out else 0x80
    xfer_len = len(data) if data else data_length
    cbw = build_cbw(cmd, xfer_len, dir_flag)
    
    print(f"  CBW ({len(cbw)} bytes): {cbw.hex()}")
    
    # Step 1: Send CBW
    try:
        written = dev.write(ENDPOINT_OUT, cbw, timeout=timeout)
        print(f"  CBW sent: {written} bytes")
    except usb.core.USBError as e:
        print(f"  CBW write error: {e}")
        return None, -1
    
    # Step 2: Data phase
    received_data = None
    if direction_out and data:
        try:
            written = dev.write(ENDPOINT_OUT, data, timeout=timeout * 3)
            print(f"  Data OUT sent: {written} bytes")
        except usb.core.USBError as e:
            print(f"  Data OUT error: {e}")
            return None, -1
    elif not direction_out and data_length > 0:
        try:
            received = dev.read(ENDPOINT_IN, data_length, timeout=timeout * 4)
            received_data = bytes(received)
            print(f"  Data IN received: {len(received_data)} bytes: {received_data.hex()}")
        except usb.core.USBError as e:
            print(f"  Data IN error: {e}")
            # Continue to try reading CSW anyway
    
    # Step 3: Read CSW (13 bytes)
    retries = 5
    csw = None
    for attempt in range(retries):
        try:
            raw_csw = dev.read(ENDPOINT_IN, 13, timeout=timeout * 5)
            csw = bytes(raw_csw)
            print(f"  CSW ({len(csw)} bytes): {csw.hex()}")
            break
        except usb.core.USBError as e:
            print(f"  CSW read attempt {attempt+1}: {e}")
            time.sleep(0.1)
    
    if csw is None:
        print("  CSW: Failed to read after retries")
        return received_data, -1
    
    # Validate CSW
    if csw[:4] != CSW_SIGNATURE:
        print(f"  CSW: Invalid signature: {csw[:4]!r}")
        return received_data, -1
    
    status = csw[12]
    print(f"  CSW status: {status}")
    
    return received_data, status


def main():
    print("=" * 60)
    print("AX206 Protocol Test - dpf-ax style")
    print("=" * 60)
    
    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("ERROR: Device not found (1908:0102)")
        sys.exit(1)
    
    print(f"\nDevice found:")
    try:
        print(f"  Manufacturer: {dev.manufacturer}")
        print(f"  Product: {dev.product}")
    except:
        print("  (Could not read strings)")
    
    # Detach kernel driver
    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
            print("  Detached kernel driver")
    except:
        pass
    
    # Set configuration
    try:
        dev.set_configuration()
        print("  Configuration set")
    except usb.core.USBError as e:
        print(f"  Configuration: {e}")
    
    # Claim interface
    try:
        usb.util.claim_interface(dev, 0)
        print("  Interface claimed")
    except usb.core.USBError as e:
        print(f"  Interface claim: {e}")
    
    # Test 1: Probe command
    print("\n" + "=" * 60)
    print("TEST 1: Probe command (cmd[5] = 3)")
    print("=" * 60)
    
    # From dpf-ax scsi.c probe():
    # cmd[5] = 3; // probe
    cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00,
                 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
    
    _, status = emulate_scsi(dev, cmd, direction_out=False, data_length=0)
    if status == 0:
        print("  -> Original protocol (no flash lock)")
    elif status == 1:
        print("  -> Improved protocol (has flash lock)")
    else:
        print(f"  -> Unknown protocol status: {status}")
    
    # Test 2: Get LCD parameters
    print("\n" + "=" * 60)
    print("TEST 2: Get LCD parameters (cmd[5] = 2)")
    print("=" * 60)
    
    # From dpf-ax scsi.c probe():
    # cmd[5] = 2; // get LCD parameters
    cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
                 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
    
    data, status = emulate_scsi(dev, cmd, direction_out=False, data_length=5)
    if data and len(data) >= 5:
        width = struct.unpack('<H', data[0:2])[0]
        height = struct.unpack('<H', data[2:4])[0]
        bpp = data[4]
        print(f"  -> LCD: {width}x{height}, {bpp} bytes per pixel")
    else:
        print(f"  -> Failed to get LCD params")
        width, height = 320, 240
        print(f"  -> Using defaults: {width}x{height}")
    
    # Test 3: Blit test pattern
    print("\n" + "=" * 60)
    print("TEST 3: Blit test pattern")
    print("=" * 60)
    
    # Create test pattern - red/blue split
    pixel_count = width * height
    buffer = bytearray(pixel_count * 2)
    
    # RGB565: Red = 0xF800, Blue = 0x001F, Green = 0x07E0
    red = struct.pack('<H', 0xF800)
    blue = struct.pack('<H', 0x001F)
    green = struct.pack('<H', 0x07E0)
    
    for i in range(pixel_count):
        y = i // width
        x = i % width
        
        # Create a pattern: red top-left, blue top-right, green bottom
        if y < height // 2:
            if x < width // 2:
                color = red
            else:
                color = blue
        else:
            color = green
        
        buffer[i*2:i*2+2] = color
    
    # Build blit command
    # QTKeJi/AIDA64 Protocol (discovered via USB capture):
    # cmd[0] = 0xCD (vendor prefix)
    # cmd[5] = 0x06 (BLIT operation type)
    # cmd[6] = 0x12 (write image subcommand)
    # cmd[11-12] = width - 1 (little-endian)
    # cmd[13-14] = height - 1 (little-endian)
    # Captured: cd 00 00 00 00 06 12 00 00 00 00 df 01 3f 01 00
    x0, y0 = 0, 0
    x1, y1 = width - 1, height - 1
    
    cmd = bytearray(16)
    cmd[0] = 0xCD
    cmd[5] = 0x06  # BLIT operation type
    cmd[6] = 0x12  # Write image subcommand
    # cmd[7-10] = 0 (reserved for offset, not used for full-screen)
    cmd[11] = x1 & 0xFF         # width - 1, low byte
    cmd[12] = (x1 >> 8) & 0xFF  # width - 1, high byte
    cmd[13] = y1 & 0xFF         # height - 1, low byte
    cmd[14] = (y1 >> 8) & 0xFF  # height - 1, high byte
    
    print(f"  Sending {len(buffer)} bytes of image data ({width}x{height})")
    print(f"  Blit rect: ({x0},{y0}) to ({x1},{y1})")
    
    _, status = emulate_scsi(dev, bytes(cmd), direction_out=True, data=bytes(buffer), timeout=5000)
    
    if status == 0:
        print("  -> SUCCESS! Check the display!")
    else:
        print(f"  -> Blit returned status: {status}")
    
    # Cleanup
    print("\n" + "=" * 60)
    print("CLEANUP")
    print("=" * 60)
    
    try:
        usb.util.release_interface(dev, 0)
        print("  Interface released")
    except:
        pass
    
    print("\nDone!")


if __name__ == "__main__":
    main()
