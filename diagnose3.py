#!/usr/bin/env python3
"""
AX206 Protocol Diagnostic - Part 3

Based on findings:
- Device reacts to data (screen goes black)
- Interface class 0xDC, subclass 0xA0, protocol 0xB0
- This matches "USB Display" devices from Chinese manufacturers

These devices often use a JPEG-based protocol or specific initialization sequence.
"""

import sys
import time
import struct
import io

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    sys.exit(1)

# Try to import PIL for JPEG creation
try:
    from PIL import Image
    HAS_PIL = True
except ImportError:
    HAS_PIL = False
    print("WARNING: PIL not available, JPEG tests will be skipped")


def main():
    print("=" * 60)
    print("AX206 Protocol Diagnostic - Part 3")
    print("Testing JPEG and initialization sequences")
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
    
    print(f"Endpoints: OUT={hex(ep_out.bEndpointAddress)}, IN={hex(ep_in.bEndpointAddress)}")
    
    # Helper to safely write
    def safe_write(data, desc, timeout=1000):
        print(f"\n[TEST] {desc}")
        print(f"  Data ({len(data)} bytes): {data[:32].hex()}{'...' if len(data) > 32 else ''}")
        try:
            written = ep_out.write(data, timeout=timeout)
            print(f"  Written: {written} bytes")
            return True
        except usb.core.USBError as e:
            print(f"  ERROR: {e}")
            return False
    
    def reset():
        try:
            dev.reset()
            time.sleep(0.3)
            dev.set_configuration()
        except:
            pass
    
    print("\n" + "=" * 60)
    print("TEST 1: USB Display common initialization sequences")
    print("=" * 60)
    
    # Many USB displays need an init sequence before accepting image data
    init_sequences = [
        # (sequence_bytes, description)
        (bytes([0x40, 0xC0]), "Init: 0x40 0xC0"),
        (bytes([0xB0, 0x00]), "Init: 0xB0 0x00 (matches protocol ID)"),
        (bytes([0xA0, 0x00]), "Init: 0xA0 0x00 (matches subclass)"),
        (bytes([0xDC, 0x00]), "Init: 0xDC 0x00 (matches class)"),
        (bytes([0x00, 0x00, 0x00, 0x00]), "Init: 4x zero"),
        (bytes([0xFF, 0xFF, 0xFF, 0xFF]), "Init: 4x 0xFF"),
        (bytes([0x55, 0xAA]), "Init: 0x55 0xAA (common magic)"),
        (bytes([0xAA, 0x55]), "Init: 0xAA 0x55 (reverse magic)"),
    ]
    
    for seq, desc in init_sequences:
        reset()
        safe_write(seq, desc)
        time.sleep(0.2)
    
    print("\n" + "=" * 60)
    print("TEST 2: JPEG-based protocol")
    print("=" * 60)
    
    # Some USB displays accept JPEG images directly
    if HAS_PIL:
        # Create a simple test image
        img = Image.new('RGB', (320, 240), color=(255, 0, 0))  # Solid red
        
        # Convert to JPEG
        jpeg_buffer = io.BytesIO()
        img.save(jpeg_buffer, format='JPEG', quality=85)
        jpeg_data = jpeg_buffer.getvalue()
        
        print(f"\nJPEG size: {len(jpeg_data)} bytes")
        print(f"JPEG header: {jpeg_data[:16].hex()}")
        
        reset()
        
        # Try raw JPEG
        safe_write(jpeg_data, "Raw JPEG data", timeout=5000)
        time.sleep(1)
        print("  Check display - is it RED?")
        
        # Try with size header
        reset()
        header = struct.pack('<I', len(jpeg_data))  # 4-byte length prefix
        safe_write(header + jpeg_data, "Length prefix (LE) + JPEG", timeout=5000)
        time.sleep(1)
        
        reset()
        header = struct.pack('>I', len(jpeg_data))  # Big endian
        safe_write(header + jpeg_data, "Length prefix (BE) + JPEG", timeout=5000)
        time.sleep(1)
        
        # Try with resolution header
        reset()
        header = struct.pack('<HHI', 320, 240, len(jpeg_data))
        safe_write(header + jpeg_data, "W,H,Len + JPEG", timeout=5000)
        time.sleep(1)
    
    print("\n" + "=" * 60)
    print("TEST 3: Packet-based protocol with sync bytes")
    print("=" * 60)
    
    # Some devices expect fixed-size packets with headers
    width, height = 320, 240
    
    # Create simple RGB565 red line
    red_rgb565 = struct.pack('<H', (31 << 11)) * width  # One line
    
    packet_formats = [
        # Header format experiments
        (bytes([0x00]) + red_rgb565, "0x00 prefix + line"),
        (bytes([0x01]) + red_rgb565, "0x01 prefix + line"),
        (bytes([0x80]) + red_rgb565, "0x80 prefix + line"),
        (bytes([0xFF]) + red_rgb565, "0xFF prefix + line"),
        (struct.pack('<HH', 0, 0) + red_rgb565, "Coords(0,0) + line"),
        (struct.pack('<BHH', 0, 0, 0) + red_rgb565, "Cmd,X,Y + line"),
    ]
    
    for packet, desc in packet_formats:
        reset()
        safe_write(packet, desc)
        time.sleep(0.3)
    
    print("\n" + "=" * 60)
    print("TEST 4: Try sending just after SCSI-like init")
    print("=" * 60)
    
    # The first diagnostic showed SCSI writes succeed without error
    # Maybe we need the SCSI init but different data format
    
    def scsi_cbw(cmd, data_len=0, direction=0x00):
        return struct.pack('<4sIIBBB16s',
            b'USBC', 0xDEADBEEF, data_len, direction, 0, len(cmd),
            cmd.ljust(16, b'\x00'))
    
    reset()
    
    # Send SCSI init commands first
    init_cbw = scsi_cbw(bytes([0xCD, 0x00, 0x04, 0x00, 0x00]), 0, 0x00)  # Some init
    print("\n[TEST] SCSI init + raw RGB565 frame")
    try:
        ep_out.write(init_cbw, timeout=1000)
        print("  SCSI init sent")
        time.sleep(0.1)
        
        # Now try raw RGB565
        frame_size = width * height * 2
        blue = struct.pack('<H', 31)  # Blue
        frame = blue * (width * height)
        
        # Send in chunks
        chunk_size = 16384
        for i in range(0, len(frame), chunk_size):
            chunk = frame[i:i+chunk_size]
            written = ep_out.write(chunk, timeout=2000)
        print(f"  Frame sent: {frame_size} bytes")
        print("  Check display - is it BLUE?")
    except usb.core.USBError as e:
        print(f"  ERROR: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 5: LCD controller commands (ST7789/ILI9341 style)")
    print("=" * 60)
    
    # Many small LCDs use standard controller commands
    # These are usually sent via SPI but some USB bridges expose them
    
    lcd_commands = [
        (bytes([0x01]), "Software Reset"),
        (bytes([0x11]), "Sleep Out"),
        (bytes([0x29]), "Display On"),
        (bytes([0x2A, 0x00, 0x00, 0x01, 0x3F]), "Column Address Set (0-319)"),
        (bytes([0x2B, 0x00, 0x00, 0x00, 0xEF]), "Row Address Set (0-239)"),
        (bytes([0x2C]), "Memory Write"),
        (bytes([0x36, 0x00]), "Memory Access Control"),
        (bytes([0x3A, 0x55]), "Pixel Format (16-bit)"),
    ]
    
    reset()
    for cmd, desc in lcd_commands:
        safe_write(cmd, f"LCD: {desc}")
        time.sleep(0.05)
    
    # After LCD init, try sending pixel data
    print("\n[TEST] After LCD init, send green frame")
    green = struct.pack('<H', (63 << 5))  # Green
    line = green * width
    
    try:
        for y in range(height):
            ep_out.write(line, timeout=1000)
        print("  Frame sent - check for GREEN!")
    except usb.core.USBError as e:
        print(f"  ERROR at line: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 6: Read device - maybe it tells us something")
    print("=" * 60)
    
    reset()
    
    # Try reading from the device
    print("\n[TEST] Attempting to read from device...")
    try:
        data = ep_in.read(64, timeout=2000)
        print(f"  Got data: {bytes(data).hex()}")
    except usb.core.USBError as e:
        print(f"  Read: {e}")
    
    # Try after writing something
    safe_write(bytes([0x00]), "Probe byte before read")
    try:
        data = ep_in.read(64, timeout=1000)
        print(f"  Response: {bytes(data).hex()}")
    except usb.core.USBError as e:
        print(f"  Read: {e}")
    
    print("\n" + "=" * 60)
    print("DIAGNOSTIC COMPLETE")
    print("=" * 60)
    print("""
Key observations needed:
1. Did the screen go BLACK during any specific test?
2. Did you see RED, BLUE, or GREEN at any point?
3. Any other screen reactions?

The fact that the screen reacts (goes black) means data IS being received.
We just need to find the right format.
""")


if __name__ == "__main__":
    main()
