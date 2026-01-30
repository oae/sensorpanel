#!/usr/bin/env python3
"""
QTKeJi USB-Display Protocol Discovery

This device only responds to GET_PARAM but rejects all dpf-ax commands.
Let's try completely different approaches:
1. Maybe it expects JPEG data
2. Maybe it uses a simpler bulk transfer without SCSI wrapping
3. Maybe it needs a special initialization sequence
4. Maybe data goes to a different endpoint or uses control transfers
"""

import sys
import struct
import time
import io

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    sys.exit(1)

try:
    from PIL import Image
    HAS_PIL = True
except ImportError:
    HAS_PIL = False
    print("WARNING: PIL not installed, JPEG tests will be skipped")


ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81


def reset_device(dev):
    """Reset and reconfigure device."""
    try:
        usb.util.release_interface(dev, 0)
    except:
        pass
    try:
        dev.reset()
        time.sleep(0.5)
        dev.set_configuration()
        usb.util.claim_interface(dev, 0)
    except Exception as e:
        print(f"  Reset error: {e}")


def build_cbw(cmd: bytes, data_length: int = 0, direction: int = 0x00) -> bytes:
    """Build CBW packet."""
    padded_cmd = cmd[:16].ljust(16, b'\x00')
    return struct.pack('<4sIIBBB', b'USBC', 0xDEADBEEF, data_length, direction, 0, len(cmd)) + padded_cmd


def send_cbw_and_data(dev, cmd: bytes, data: bytes, timeout: int = 1000) -> tuple:
    """Send CBW then data, read CSW."""
    cbw = build_cbw(cmd, len(data), 0x00)
    
    try:
        dev.write(ENDPOINT_OUT, cbw, timeout=timeout)
    except usb.core.USBError as e:
        return False, f"CBW: {e}"
    
    try:
        dev.write(ENDPOINT_OUT, data, timeout=timeout * 3)
    except usb.core.USBError as e:
        return False, f"Data: {e}"
    
    try:
        csw = bytes(dev.read(ENDPOINT_IN, 13, timeout=timeout * 2))
        if csw[:4] == b'USBS':
            return True, csw[12]
    except:
        pass
    
    return False, "No CSW"


def try_raw_write(dev, data: bytes, timeout: int = 1000) -> tuple:
    """Try writing data directly without CBW wrapper."""
    try:
        written = dev.write(ENDPOINT_OUT, data, timeout=timeout)
        return True, written
    except usb.core.USBError as e:
        return False, str(e)


def main():
    print("=" * 60)
    print("QTKeJi USB-Display Protocol Discovery")
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
    
    WIDTH, HEIGHT = 480, 320
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 1: Different CBW command structures for BLIT")
    print("=" * 60)
    
    # Small test data - 16x16 red square = 512 bytes
    test_pixels = bytes([0x00, 0xF8] * 256)  # RGB565 red, 512 bytes
    
    # Different command structures to try
    command_variants = [
        # (description, command_bytes)
        ("Standard dpf-ax BLIT", 
         bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x00, 0x00, 0x00, 0x00, 0x0F, 0x00, 0x0F, 0x00, 0x00])),
        
        ("BLIT with coords at start",
         bytes([0xCD, 0x12, 0x00, 0x00, 0x0F, 0x00, 0x0F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
        
        ("Simple 0xCD 0x12",
         bytes([0xCD, 0x12, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
        
        ("0xCD with size in cmd[1:4]",
         bytes([0xCD, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
        
        ("Write screen cmd 0xB0",
         bytes([0xB0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
         
        ("LCD write 0x2C",
         bytes([0x2C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])),
    ]
    
    for desc, cmd in command_variants:
        reset_device(dev)
        print(f"\n[{desc}]")
        print(f"  CMD: {cmd.hex()}")
        success, result = send_cbw_and_data(dev, cmd, test_pixels)
        print(f"  Result: {success}, {result}")
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 2: Raw bulk writes (no SCSI wrapper)")
    print("=" * 60)
    
    reset_device(dev)
    
    # Try various header formats followed by pixel data
    headers_to_try = [
        ("No header (raw pixels)", b""),
        ("0x00 0x2C header", bytes([0x00, 0x2C])),
        ("Width/Height header (LE)", struct.pack('<HH', 16, 16)),
        ("Width/Height header (BE)", struct.pack('>HH', 16, 16)),
        ("Magic 0x55 0xAA", bytes([0x55, 0xAA])),
        ("Magic 0xAA 0x55", bytes([0xAA, 0x55])),
        ("FRAME header", b"FRAME"),
        ("IMG header", b"IMG"),
    ]
    
    for desc, header in headers_to_try:
        reset_device(dev)
        data = header + test_pixels
        print(f"\n[{desc}] ({len(data)} bytes)")
        success, result = try_raw_write(dev, data, timeout=500)
        print(f"  Result: {success}, {result}")
        
        # Try to read response
        if success:
            try:
                resp = bytes(dev.read(ENDPOINT_IN, 64, timeout=300))
                print(f"  Response: {resp.hex()}")
            except usb.core.USBError:
                print(f"  No response")
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 3: JPEG/compressed image transfer")
    print("=" * 60)
    
    if HAS_PIL:
        reset_device(dev)
        
        # Create a small JPEG
        img = Image.new('RGB', (64, 64), color=(255, 0, 0))
        jpeg_buffer = io.BytesIO()
        img.save(jpeg_buffer, format='JPEG', quality=85)
        jpeg_data = jpeg_buffer.getvalue()
        
        print(f"\nJPEG size: {len(jpeg_data)} bytes")
        
        # Try sending JPEG with various wrappers
        jpeg_tests = [
            ("Raw JPEG", jpeg_data),
            ("JPEG with size header", struct.pack('<I', len(jpeg_data)) + jpeg_data),
            ("JPEG with CBW", None),  # Special case
        ]
        
        for desc, data in jpeg_tests:
            reset_device(dev)
            print(f"\n[{desc}]")
            
            if data is None:
                # CBW wrapper for JPEG
                cmd = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
                            0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
                success, result = send_cbw_and_data(dev, cmd, jpeg_data)
            else:
                success, result = try_raw_write(dev, data, timeout=1000)
            
            print(f"  Result: {success}, {result}")
    else:
        print("  Skipped (PIL not available)")
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 4: Control transfers for image data")
    print("=" * 60)
    
    reset_device(dev)
    
    # Some USB displays use control transfers
    ctrl_tests = [
        # (bmRequestType, bRequest, wValue, wIndex, data_or_len, desc)
        (0x40, 0x00, 0, 0, test_pixels[:64], "Vendor OUT req 0"),
        (0x40, 0x01, 0, 0, test_pixels[:64], "Vendor OUT req 1"),
        (0x40, 0x02, 0, 0, test_pixels[:64], "Vendor OUT req 2"),
        (0x40, 0x12, 0, 0, test_pixels[:64], "Vendor OUT req 0x12 (BLIT?)"),
        (0x40, 0xB0, 0, 0, test_pixels[:64], "Vendor OUT req 0xB0"),
        (0x40, 0x40, 0, 0, test_pixels[:64], "Vendor OUT req 0x40"),
        (0x21, 0x09, 0x0200, 0, test_pixels[:64], "HID SET_REPORT"),
    ]
    
    for bmReq, bReq, wVal, wIdx, data, desc in ctrl_tests:
        try:
            result = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, data, timeout=300)
            print(f"  {desc}: OK ({result})")
        except usb.core.USBError as e:
            if "pipe" not in str(e).lower() and "stall" not in str(e).lower():
                print(f"  {desc}: {e}")
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 5: Initialization sequences")
    print("=" * 60)
    
    reset_device(dev)
    
    # Maybe device needs init before accepting image data
    # Try sending GET_PARAM first, then BLIT
    
    print("\n[Sequence: GET_PARAM -> BLIT]")
    
    # GET_PARAM
    cmd_param = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
                       0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00])
    cbw = build_cbw(cmd_param, 5, 0x80)
    try:
        dev.write(ENDPOINT_OUT, cbw, timeout=500)
        params = bytes(dev.read(ENDPOINT_IN, 5, timeout=500))
        csw = bytes(dev.read(ENDPOINT_IN, 13, timeout=500))
        print(f"  GET_PARAM: {params.hex()}, CSW status: {csw[12]}")
    except usb.core.USBError as e:
        print(f"  GET_PARAM failed: {e}")
    
    # Now try BLIT
    cmd_blit = bytes([0xCD, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x00,
                      0x00, 0x00, 0x00, 0x0F, 0x00, 0x0F, 0x00, 0x00])
    success, result = send_cbw_and_data(dev, cmd_blit, test_pixels)
    print(f"  BLIT: {success}, {result}")
    
    # =================================================================
    print("\n" + "=" * 60)
    print("TEST 6: Scan for any working OUT command")  
    print("=" * 60)
    
    reset_device(dev)
    
    # Scan first byte of command to find anything that accepts data
    print("\nScanning cmd[0] with small data payload...")
    
    for first_byte in range(256):
        reset_device(dev)
        cmd = bytes([first_byte] + [0x00] * 15)
        cbw = build_cbw(cmd, 64, 0x00)  # 64 bytes OUT
        
        try:
            dev.write(ENDPOINT_OUT, cbw, timeout=200)
            # CBW accepted, try data
            try:
                dev.write(ENDPOINT_OUT, bytes(64), timeout=200)
                # Data accepted, try CSW
                try:
                    csw = bytes(dev.read(ENDPOINT_IN, 13, timeout=200))
                    if csw[:4] == b'USBS':
                        print(f"  cmd[0]=0x{first_byte:02X}: ACCEPTED! CSW status={csw[12]}")
                except:
                    print(f"  cmd[0]=0x{first_byte:02X}: Data sent, no CSW")
            except usb.core.USBError as e:
                if "pipe" not in str(e).lower():
                    print(f"  cmd[0]=0x{first_byte:02X}: CBW ok, data error: {e}")
        except:
            pass  # CBW rejected
    
    # Cleanup
    try:
        usb.util.release_interface(dev, 0)
    except:
        pass
    
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)
    print("""
The device accepts SCSI CBW but only responds to GET_PARAM.
No data OUT commands seem to work through the CBW interface.

This suggests the device might:
1. Use a completely proprietary protocol
2. Require Windows driver initialization
3. Use a different transport mechanism

RECOMMENDATION: Capture USB traffic from Windows with AIDA64
to see exactly what protocol it uses.
""")


if __name__ == "__main__":
    main()
