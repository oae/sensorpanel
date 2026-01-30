#!/usr/bin/env python3
"""
AX206 Protocol Diagnostic Script - Part 2

Testing alternative protocols for QTKeJi USB-Display devices.
These devices often use simpler bulk transfer protocols.

Usage: sudo python diagnose2.py
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


def reset_device(dev):
    """Reset device to clear error state."""
    try:
        dev.reset()
        time.sleep(0.5)
        dev.set_configuration()
    except:
        pass


def main():
    print("=" * 60)
    print("AX206 Protocol Diagnostic - Part 2 (QTKeJi variant)")
    print("=" * 60)
    
    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("\nERROR: No AX206 device found")
        sys.exit(1)
    
    print(f"\nDevice: {dev.manufacturer} {dev.product}")
    
    # Detach kernel driver
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
    
    print(f"Endpoints: OUT={hex(ep_out.bEndpointAddress)}, IN={hex(ep_in.bEndpointAddress)}")
    
    # Many cheap USB displays use a simple protocol:
    # - First packet: header with resolution and format info
    # - Following packets: raw pixel data
    
    print("\n" + "=" * 60)
    print("TEST 1: Simple header + raw RGB565 data")
    print("=" * 60)
    
    # Common simple header formats
    width, height = 320, 240
    
    # Create a test pattern - top half red, bottom half blue
    red = struct.pack('<H', (31 << 11))      # RGB565 red
    blue = struct.pack('<H', 31)              # RGB565 blue
    
    # Small test: 320x10 strip at top
    test_height = 10
    test_data = red * (width * test_height)
    
    headers_to_try = [
        # Format: (header_bytes, description)
        (struct.pack('<HH', width, height), "Simple: width,height (LE)"),
        (struct.pack('>HH', width, height), "Simple: width,height (BE)"),
        (struct.pack('<HHHH', 0, 0, width-1, height-1), "Rect: x0,y0,x1,y1 (LE)"),
        (struct.pack('<BBHH', 0x00, 0x2C, width, height), "LCD cmd: 0x00,0x2C,w,h"),
        (struct.pack('<BBHHHH', 0x00, 0x2C, 0, 0, width-1, height-1), "LCD cmd: 0x00,0x2C,rect"),
        (bytes([0x2C]) + struct.pack('<HHHH', 0, 0, width-1, test_height-1), "0x2C + rect"),
    ]
    
    for header, desc in headers_to_try:
        print(f"\n[TEST] {desc}")
        print(f"  Header: {header.hex()}")
        
        reset_device(dev)
        time.sleep(0.1)
        
        try:
            # Send header
            written = ep_out.write(header, timeout=1000)
            print(f"  Header written: {written} bytes")
            
            # Send pixel data (just a strip)
            written = ep_out.write(test_data, timeout=2000)
            print(f"  Data written: {written} bytes - CHECK DISPLAY!")
            
            time.sleep(0.5)
            
        except usb.core.USBError as e:
            print(f"  ERROR: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 2: Control Transfer based protocol")
    print("=" * 60)
    
    # Some USB displays use control transfers for setup
    reset_device(dev)
    
    control_requests = [
        # (bmRequestType, bRequest, wValue, wIndex, data_or_length)
        (0x40, 0x01, 0, 0, None),        # Vendor OUT, init
        (0x40, 0x02, width, height, None),  # Set resolution
        (0xC0, 0x01, 0, 0, 64),          # Vendor IN, get info
        (0xC0, 0x81, 0, 0, 64),          # Alt get info
    ]
    
    for bmReq, bReq, wVal, wIdx, data in control_requests:
        desc = f"bmReq={hex(bmReq)}, bReq={hex(bReq)}, wVal={wVal}, wIdx={wIdx}"
        print(f"\n[TEST] Control: {desc}")
        
        try:
            if data is None:
                # OUT transfer
                ret = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, timeout=1000)
                print(f"  Result: {ret}")
            else:
                # IN transfer
                ret = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, data, timeout=1000)
                print(f"  Response: {bytes(ret).hex()}")
        except usb.core.USBError as e:
            print(f"  ERROR: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 3: Raw frame buffer write (no header)")
    print("=" * 60)
    
    # Some displays just accept raw RGB565 data for the full frame
    reset_device(dev)
    
    # Create full frame - gradient pattern
    print("\n[TEST] Sending full 320x240 gradient frame...")
    
    frame = bytearray(width * height * 2)
    for y in range(height):
        for x in range(width):
            # Create gradient: red increases left-to-right, green increases top-to-bottom
            r = (x * 31) // width
            g = (y * 63) // height
            rgb565 = (r << 11) | (g << 5)
            offset = (y * width + x) * 2
            frame[offset:offset+2] = struct.pack('<H', rgb565)
    
    try:
        # Try sending in chunks
        chunk_size = 16384
        total_sent = 0
        
        for i in range(0, len(frame), chunk_size):
            chunk = frame[i:i+chunk_size]
            written = ep_out.write(bytes(chunk), timeout=5000)
            total_sent += written
            
        print(f"  Total sent: {total_sent} bytes")
        print("  CHECK DISPLAY for gradient pattern!")
        
    except usb.core.USBError as e:
        print(f"  ERROR at byte {total_sent}: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 4: Investigate with USB descriptor info")  
    print("=" * 60)
    
    # Print full descriptor info
    print("\nFull device descriptor:")
    print(f"  bcdUSB: {hex(dev.bcdUSB)}")
    print(f"  bDeviceClass: {dev.bDeviceClass}")
    print(f"  bDeviceSubClass: {dev.bDeviceSubClass}")
    print(f"  bDeviceProtocol: {dev.bDeviceProtocol}")
    print(f"  bMaxPacketSize0: {dev.bMaxPacketSize0}")
    print(f"  bcdDevice: {hex(dev.bcdDevice)}")
    
    print("\nInterface descriptor:")
    print(f"  bInterfaceClass: {intf.bInterfaceClass}")
    print(f"  bInterfaceSubClass: {intf.bInterfaceSubClass}")
    print(f"  bInterfaceProtocol: {intf.bInterfaceProtocol}")
    
    # The interface class 220 (0xDC) is "Diagnostic Device"
    # SubClass 160 (0xA0) and Protocol 176 (0xB0) might give hints
    
    print("\n" + "=" * 60)
    print("DIAGNOSTIC COMPLETE")
    print("=" * 60)
    print("""
Please report:
1. Did any test show something on the display?
2. Which headers wrote without error?
3. The interface class info above may help identify the protocol.

The interface class 220 (0xDC) = Diagnostic Device is unusual.
SubClass 0xA0 and Protocol 0xB0 might be vendor-specific identifiers.
""")


if __name__ == "__main__":
    main()
