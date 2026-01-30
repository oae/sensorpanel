#!/usr/bin/env python3
"""
AX206 Command Discovery

The device responds to GET_PARAM but not BLIT.
Let's discover what commands it actually supports.
"""

import sys
import struct
import time

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    sys.exit(1)


ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81


def build_cbw(cmd: bytes, data_length: int = 0, direction: int = 0x00, tag: int = 0xDEADBEEF) -> bytes:
    """Build CBW packet."""
    if len(cmd) > 16:
        cmd = cmd[:16]
    padded_cmd = cmd.ljust(16, b'\x00')
    
    cbw = struct.pack(
        '<4sIIBBB',
        b'USBC',
        tag,
        data_length,
        direction,
        0,
        len(cmd)
    ) + padded_cmd
    
    return cbw


def try_command(dev, cmd: bytes, data_out: bytes = None, data_in_len: int = 0, 
                timeout: int = 500, desc: str = "") -> tuple:
    """Try a command and return (success, data, status)."""
    dir_flag = 0x00 if data_out else 0x80
    xfer_len = len(data_out) if data_out else data_in_len
    cbw = build_cbw(cmd, xfer_len, dir_flag)
    
    try:
        dev.write(ENDPOINT_OUT, cbw, timeout=timeout)
    except usb.core.USBError as e:
        return False, None, f"CBW write: {e}"
    
    # Data phase
    received = None
    if data_out:
        try:
            dev.write(ENDPOINT_OUT, data_out, timeout=timeout * 2)
        except usb.core.USBError as e:
            return False, None, f"Data OUT: {e}"
    elif data_in_len > 0:
        try:
            raw = dev.read(ENDPOINT_IN, data_in_len, timeout=timeout * 2)
            received = bytes(raw)
        except usb.core.USBError as e:
            pass  # Continue to CSW
    
    # CSW
    try:
        csw = bytes(dev.read(ENDPOINT_IN, 13, timeout=timeout * 2))
        if csw[:4] == b'USBS':
            status = csw[12]
            return True, received, status
        else:
            return False, received, f"Bad CSW: {csw.hex()}"
    except usb.core.USBError as e:
        return False, received, f"CSW: {e}"


def main():
    print("=" * 60)
    print("AX206 Command Discovery")
    print("=" * 60)
    
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("ERROR: Device not found")
        sys.exit(1)
    
    print(f"\nDevice: {dev.manufacturer} {dev.product}")
    
    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
    except:
        pass
    
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    # We know cmd[5]=2 works for params
    # Let's scan all possible subcommands at cmd[5] and cmd[6]
    
    print("\n" + "=" * 60)
    print("SCAN: Subcommands at cmd[5] (dpf-ax style probe/params)")
    print("=" * 60)
    
    # Try reading 5-64 bytes for each subcommand
    for subcmd in range(256):
        cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, subcmd, 0x00, 0x00,
                     0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
        
        success, data, status = try_command(dev, cmd, data_in_len=64, timeout=200)
        
        if success and status == 0:
            if data and any(b != 0 for b in data):
                print(f"  cmd[5]=0x{subcmd:02X}: status={status}, data={data[:16].hex()}")
            elif data is None or all(b == 0 for b in data):
                print(f"  cmd[5]=0x{subcmd:02X}: status={status} (no data)")
    
    time.sleep(0.2)
    dev.reset()
    time.sleep(0.5)
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    print("\n" + "=" * 60)
    print("SCAN: Subcommands at cmd[6] (dpf-ax style BLIT/etc)")
    print("=" * 60)
    
    for subcmd in range(256):
        cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, subcmd, 0x00,
                     0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
        
        success, data, status = try_command(dev, cmd, data_in_len=64, timeout=200)
        
        if success:
            if data and any(b != 0 for b in data):
                print(f"  cmd[6]=0x{subcmd:02X}: status={status}, data={data[:16].hex()}")
            else:
                print(f"  cmd[6]=0x{subcmd:02X}: status={status}")
    
    time.sleep(0.2)
    dev.reset()
    time.sleep(0.5)
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    print("\n" + "=" * 60)
    print("TEST: Different image transfer approaches")
    print("=" * 60)
    
    # Create small test image (just 100 pixels = 200 bytes)
    test_data = bytes([0xF8, 0x00] * 100)  # Red pixels in RGB565
    
    # Approach 1: dpf-ax style BLIT at cmd[6]
    print("\n[Test] dpf-ax BLIT (cmd[6]=0x12):")
    cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x00,
                 0x00, 0x00, 0x00, 0x09, 0x00, 0x09, 0x00, 0x00])  # 10x10 rect
    success, _, status = try_command(dev, cmd, data_out=test_data, timeout=1000)
    print(f"  Result: success={success}, status={status}")
    
    # Approach 2: BLIT params at cmd[2] onwards
    print("\n[Test] BLIT variant (cmd[2]=0x12):")
    cmd = bytes([0xCD, 0x00, 0x12, 0x00, 0x00, 0x00, 0x00, 0x00,
                 0x00, 0x09, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00])  # 10x10 rect
    success, _, status = try_command(dev, cmd, data_out=test_data, timeout=1000)
    print(f"  Result: success={success}, status={status}")
    
    # Approach 3: Write to memory address 0
    print("\n[Test] Memory write (cmd[6]=0x01):")
    cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
                 0x00, 0xC8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])  # addr=0, len=200
    success, _, status = try_command(dev, cmd, data_out=test_data, timeout=1000)
    print(f"  Result: success={success}, status={status}")
    
    # Approach 4: Raw data after just a simple header
    print("\n[Test] Raw frame with minimal header:")
    dev.reset()
    time.sleep(0.3)
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    # Try sending data without CBW
    try:
        written = dev.write(ENDPOINT_OUT, test_data, timeout=1000)
        print(f"  Raw write: {written} bytes")
        # Try to read response
        try:
            resp = bytes(dev.read(ENDPOINT_IN, 64, timeout=500))
            print(f"  Response: {resp.hex()}")
        except:
            print(f"  No response")
    except usb.core.USBError as e:
        print(f"  Error: {e}")
    
    # Approach 5: Different SCSI command prefixes
    print("\n" + "=" * 60)
    print("TEST: Alternative SCSI command prefixes")
    print("=" * 60)
    
    dev.reset()
    time.sleep(0.3)
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    prefixes = [0xCB, 0xCC, 0xCE, 0xCF, 0xFE, 0xFF, 0x00, 0x2A, 0x28]
    for prefix in prefixes:
        cmd = bytes([prefix, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
                     0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
        success, data, status = try_command(dev, cmd, data_in_len=5, timeout=300)
        if success:
            print(f"  prefix=0x{prefix:02X}: status={status}, data={data.hex() if data else 'None'}")
    
    print("\n" + "=" * 60)
    print("ANALYSIS: What the device accepts")
    print("=" * 60)
    
    dev.reset()
    time.sleep(0.3)
    dev.set_configuration()
    usb.util.claim_interface(dev, 0)
    
    # Re-confirm working commands
    print("\n[Confirm] GET_PARAM (cmd[5]=0x02):")
    cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
                 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
    success, data, status = try_command(dev, cmd, data_in_len=5, timeout=500)
    if success and data:
        w = struct.unpack('<H', data[0:2])[0]
        h = struct.unpack('<H', data[2:4])[0]
        print(f"  LCD: {w}x{h}")
    
    # Try write with params location matching BLIT expectation
    print("\n[Test] Full frame BLIT (correct size 480x320):")
    frame_size = 480 * 320 * 2
    # Just send 1KB to test
    test_chunk = bytes([0x00, 0xF8] * 512)  # 1KB of red
    
    cmd = bytearray(16)
    cmd[0] = 0xCD
    cmd[6] = 0x12  # BLIT
    # x0, y0 = 0, 0
    cmd[7] = 0
    cmd[8] = 0
    cmd[9] = 0
    cmd[10] = 0
    # x1, y1 = 31, 31 (32x32 = 2048 bytes, send 1024)
    cmd[11] = 15  # x1 = 15
    cmd[12] = 0
    cmd[13] = 31  # y1 = 31 (16 * 32 = 512 pixels = 1024 bytes)
    cmd[14] = 0
    
    success, _, status = try_command(dev, bytes(cmd), data_out=test_chunk, timeout=2000)
    print(f"  Result: success={success}, status={status}")
    
    # Cleanup
    try:
        usb.util.release_interface(dev, 0)
    except:
        pass
    
    print("\nDone!")


if __name__ == "__main__":
    main()
