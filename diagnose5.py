#!/usr/bin/env python3
"""
AX206 Protocol Diagnostic - Part 5

Try combined CBW + data in single transfer, and other edge cases.
Also try to match the exact packet patterns these devices might use.
"""

import sys
import time
import struct

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    sys.exit(1)


def main():
    print("=" * 60)
    print("AX206 Protocol Diagnostic - Part 5")
    print("Combined transfers and special cases")
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
    
    def reset():
        try:
            dev.reset()
            time.sleep(0.3)
            dev.set_configuration()
        except:
            pass
    
    print("\n" + "=" * 60)
    print("TEST 1: CBW + Data combined in single transfer")
    print("=" * 60)
    
    # Some devices want CBW and data in one packet
    width, height = 320, 240
    
    # Small test data
    red = struct.pack('<H', (31 << 11))
    small_image = red * 16  # Just 32 bytes of red pixels
    
    # dpf-ax style CBW
    def make_cbw(cmd, data_len=0):
        return struct.pack('<4sIIBBB',
            b'USBC', 0xDEADBEEF, data_len, 0x00, 0, len(cmd)) + cmd.ljust(16, b'\x00')
    
    # Try various combined packets
    test_cmds = [
        bytes([0xCD, 0x00, 0x12, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x03, 0x00]),  # 4x4 blit
        bytes([0xCD, 0x12]),
        bytes([0x2A]),  # SCSI write
        bytes([0xFE, 0x01]),  # Random vendor
    ]
    
    for cmd in test_cmds:
        reset()
        cbw = make_cbw(cmd, len(small_image))
        combined = cbw + small_image
        
        print(f"\n[TEST] Combined CBW({cmd.hex()}) + {len(small_image)} bytes data")
        print(f"  Total packet: {len(combined)} bytes")
        
        try:
            written = ep_out.write(combined, timeout=1000)
            print(f"  Written: {written} bytes - CHECK DISPLAY!")
            time.sleep(0.3)
        except usb.core.USBError as e:
            print(f"  ERROR: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 2: Try without SCSI wrapper at all (bulk only)")
    print("=" * 60)
    
    reset()
    
    # Some USB displays use a simple proprietary header
    # Let's try formats common in Chinese USB displays
    
    # Format: Magic + Width + Height + Data
    magic_headers = [
        b'\x00\x2C',  # LCD write memory command
        b'\x55\xAA',  # Common sync
        b'\xAA\x55',  # Reverse sync
        b'\x00\x00',  # Null header
        b'\xFF\xD8',  # JPEG magic (maybe it only accepts JPEG?)
        b'\x89\x50',  # PNG magic
        b'BM',        # BMP magic
    ]
    
    # Try each with small data following
    for magic in magic_headers:
        reset()
        time.sleep(0.1)
        
        # Send magic first
        print(f"\n[TEST] Magic: {magic.hex()}")
        try:
            ep_out.write(magic, timeout=500)
            print("  Magic sent")
            
            # Then try to send data
            try:
                written = ep_out.write(small_image, timeout=500)
                print(f"  Data sent: {written} bytes")
            except usb.core.USBError as e:
                print(f"  Data: {e}")
        except usb.core.USBError as e:
            print(f"  Magic: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 3: Full frame in 64-byte chunks (raw)")  
    print("=" * 60)
    
    reset()
    
    # Create a simple pattern - half red, half blue
    frame_size = 320 * 240 * 2
    frame = bytearray(frame_size)
    
    red = struct.pack('<H', (31 << 11))
    blue = struct.pack('<H', 31)
    
    for i in range(0, frame_size, 2):
        y = (i // 2) // 320
        if y < 120:
            frame[i:i+2] = red
        else:
            frame[i:i+2] = blue
    
    print(f"\nSending {frame_size} bytes in 64-byte chunks...")
    print("This matches USB Full Speed max packet size.")
    
    try:
        # Send exactly 64 bytes at a time
        for i in range(0, len(frame), 64):
            chunk = bytes(frame[i:i+64])
            if len(chunk) < 64:
                chunk = chunk.ljust(64, b'\x00')
            ep_out.write(chunk, timeout=100)
        print("  All chunks sent - CHECK DISPLAY!")
    except usb.core.USBError as e:
        print(f"  Error at offset {i}: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 4: Try with specific timing")
    print("=" * 60)
    
    reset()
    time.sleep(1)  # Wait for device to fully reset
    
    print("\nSending frame with delays between chunks...")
    
    try:
        chunk_size = 512
        for i in range(0, min(len(frame), 4096), chunk_size):
            chunk = bytes(frame[i:i+chunk_size])
            ep_out.write(chunk, timeout=500)
            time.sleep(0.01)  # Small delay between chunks
        print("  Sent 4KB with timing - CHECK DISPLAY!")
    except usb.core.USBError as e:
        print(f"  Error: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 5: Windows-style initialization sequence")
    print("=" * 60)
    
    # Windows drivers often do control transfers first
    reset()
    time.sleep(0.5)
    
    print("\nTrying control transfers that Windows might use...")
    
    # Standard USB control requests
    control_tests = [
        (0x00, 0x05, 1, 0, 0, "Set Address"),
        (0x00, 0x09, 1, 0, 0, "Set Configuration"),
        (0x01, 0x0B, 0, 0, 0, "Set Interface"),
        (0x02, 0x01, 0, 0x81, 0, "Clear Halt EP_IN"),
        (0x02, 0x01, 0, 0x01, 0, "Clear Halt EP_OUT"),
        (0xC0, 0x00, 0, 0, 64, "Vendor Read 0"),
        (0xC0, 0x01, 0, 0, 64, "Vendor Read 1"),
        (0xC0, 0x04, 0, 0, 64, "Vendor Read 4"),
        (0xC0, 0x06, 0, 0, 64, "Vendor Read 6 (Get Descriptor?)"),
        (0x40, 0x00, 0, 0, None, "Vendor Write 0"),
        (0x40, 0x01, 0, 0, None, "Vendor Write 1"),
        (0x40, 0x01, 320, 240, None, "Vendor Write (resolution)"),
    ]
    
    for bmReq, bReq, wVal, wIdx, data, desc in control_tests:
        try:
            if data is None:
                result = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, timeout=200)
                print(f"  {desc}: OK ({result})")
            elif data == 0:
                result = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, timeout=200)
                print(f"  {desc}: OK ({result})")
            else:
                result = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, data, timeout=200)
                print(f"  {desc}: {bytes(result).hex()}")
        except usb.core.USBError as e:
            if "not supported" not in str(e).lower() and "stall" not in str(e).lower():
                print(f"  {desc}: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 6: Endpoint analysis")
    print("=" * 60)
    
    print(f"\nEndpoint OUT details:")
    print(f"  Address: {hex(ep_out.bEndpointAddress)}")
    print(f"  Attributes: {hex(ep_out.bmAttributes)}")
    print(f"  Max Packet Size: {ep_out.wMaxPacketSize}")
    print(f"  Interval: {ep_out.bInterval}")
    
    print(f"\nEndpoint IN details:")
    print(f"  Address: {hex(ep_in.bEndpointAddress)}")
    print(f"  Attributes: {hex(ep_in.bmAttributes)}")
    print(f"  Max Packet Size: {ep_in.wMaxPacketSize}")
    print(f"  Interval: {ep_in.bInterval}")
    
    # Check if there are other interfaces or alternate settings
    print(f"\nConfiguration details:")
    for intf in cfg:
        print(f"  Interface {intf.bInterfaceNumber}, Alt {intf.bAlternateSetting}")
        print(f"    Class: {intf.bInterfaceClass}, SubClass: {intf.bInterfaceSubClass}, Protocol: {intf.bInterfaceProtocol}")
        for ep in intf:
            print(f"    Endpoint: {hex(ep.bEndpointAddress)}, Type: {hex(ep.bmAttributes)}")
    
    print("\n" + "=" * 60)
    print("CONCLUSION")
    print("=" * 60)
    print("""
This device appears to use a proprietary protocol that we cannot
determine through probing alone.

RECOMMENDED NEXT STEPS:
1. Capture USB traffic on Windows using Wireshark + USBPcap
   - Install AIDA64 on Windows
   - Start USB capture
   - Send an image to the display
   - Save the capture and share it

2. Search for this specific device:
   - "QTKeJi USB-Display linux"
   - "1908:0102 USB-Display driver"
   - Check if there's a Windows .inf or .sys file that reveals protocol

3. Check if this is actually a different device type:
   - Some USB displays need a custom kernel module
   - The interface class 0xDC might be a clue
""")


if __name__ == "__main__":
    main()
