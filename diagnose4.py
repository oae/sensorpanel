#!/usr/bin/env python3
"""
AX206 Protocol Diagnostic - Part 4

Focus on SCSI-wrapped commands since those are the only ones that don't error.
Systematically probe different SCSI vendor command codes.
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


def build_scsi_cbw(cmd_bytes, data_len=0, direction=0x80, tag=0xDEADBEEF):
    """Build SCSI Command Block Wrapper."""
    cmd_padded = cmd_bytes.ljust(16, b'\x00')
    return struct.pack('<4sIIBBB16s',
        b'USBC',
        tag,
        data_len,
        direction,  # 0x80 = IN (read), 0x00 = OUT (write)
        0,          # LUN
        len(cmd_bytes),
        cmd_padded
    )


def main():
    print("=" * 60)
    print("AX206 Protocol Diagnostic - Part 4")
    print("SCSI Vendor Command Enumeration")
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
    
    def try_scsi_cmd(cmd_bytes, desc, read_len=64, expect_data=True):
        """Try a SCSI command and report results."""
        direction = 0x80 if expect_data else 0x00
        cbw = build_scsi_cbw(cmd_bytes, data_len=read_len if expect_data else 0, direction=direction)
        
        try:
            written = ep_out.write(cbw, timeout=500)
            
            if expect_data:
                time.sleep(0.05)
                try:
                    response = ep_in.read(read_len, timeout=500)
                    resp_bytes = bytes(response)
                    print(f"  [{cmd_bytes.hex()}] {desc}: GOT RESPONSE!")
                    print(f"      Data: {resp_bytes.hex()}")
                    return resp_bytes
                except usb.core.USBError:
                    pass
            return True
        except usb.core.USBError as e:
            return None
    
    print("\n" + "=" * 60)
    print("TEST 1: Enumerate SCSI vendor commands (0x00-0xFF)")
    print("=" * 60)
    
    # SCSI vendor-specific commands are typically in the range 0xC0-0xFF
    # but let's check a wider range
    
    print("\nTrying single-byte commands with IN direction...")
    successful_cmds = []
    
    for cmd_byte in range(256):
        reset()
        cmd = bytes([cmd_byte])
        cbw = build_scsi_cbw(cmd, data_len=64, direction=0x80)
        
        try:
            ep_out.write(cbw, timeout=300)
            time.sleep(0.02)
            try:
                response = ep_in.read(64, timeout=300)
                resp_bytes = bytes(response)
                if resp_bytes != bytes(64):  # Not all zeros
                    print(f"  0x{cmd_byte:02X}: Response: {resp_bytes[:16].hex()}...")
                    successful_cmds.append((cmd_byte, resp_bytes))
            except:
                pass
        except:
            pass
        
        if cmd_byte % 32 == 31:
            print(f"  Scanned 0x00-0x{cmd_byte:02X}...")
    
    print(f"\nFound {len(successful_cmds)} commands with responses")
    
    print("\n" + "=" * 60)
    print("TEST 2: Two-byte command variations on 0xCD prefix")
    print("=" * 60)
    
    # dpf-ax uses 0xCD as vendor prefix, try all second bytes
    print("\nTrying 0xCD 0xXX commands...")
    
    for second_byte in range(256):
        reset()
        cmd = bytes([0xCD, second_byte])
        cbw = build_scsi_cbw(cmd, data_len=64, direction=0x80)
        
        try:
            ep_out.write(cbw, timeout=300)
            time.sleep(0.02)
            try:
                response = ep_in.read(64, timeout=300)
                resp_bytes = bytes(response)
                if resp_bytes != bytes(64):
                    print(f"  0xCD 0x{second_byte:02X}: {resp_bytes[:16].hex()}...")
                    successful_cmds.append((0xCD00 | second_byte, resp_bytes))
            except:
                pass
        except:
            pass
    
    print("\n" + "=" * 60)
    print("TEST 3: Try common SCSI commands")
    print("=" * 60)
    
    scsi_commands = [
        (bytes([0x00]), "Test Unit Ready"),
        (bytes([0x03]), "Request Sense"),
        (bytes([0x12]), "Inquiry"),
        (bytes([0x1A]), "Mode Sense (6)"),
        (bytes([0x25]), "Read Capacity"),
        (bytes([0x5A]), "Mode Sense (10)"),
        (bytes([0xA0]), "Report LUNs"),
        # Vendor specific ranges
        (bytes([0xC0]), "Vendor C0"),
        (bytes([0xC1]), "Vendor C1"),
        (bytes([0xC2]), "Vendor C2"),
        (bytes([0xCB]), "Vendor CB"),
        (bytes([0xCC]), "Vendor CC"),
        (bytes([0xCD]), "Vendor CD"),
        (bytes([0xCE]), "Vendor CE"),
        (bytes([0xCF]), "Vendor CF"),
        (bytes([0xD0]), "Vendor D0"),
        (bytes([0xDF]), "Vendor DF"),
        (bytes([0xE0]), "Vendor E0"),
        (bytes([0xF0]), "Vendor F0"),
        (bytes([0xFF]), "Vendor FF"),
    ]
    
    for cmd, desc in scsi_commands:
        reset()
        print(f"\n[TEST] {desc} (0x{cmd.hex()})")
        
        cbw = build_scsi_cbw(cmd, data_len=64, direction=0x80)
        try:
            ep_out.write(cbw, timeout=500)
            time.sleep(0.05)
            try:
                response = ep_in.read(64, timeout=500)
                print(f"  Response: {bytes(response)[:32].hex()}")
            except usb.core.USBError as e:
                print(f"  Read: {e}")
        except usb.core.USBError as e:
            print(f"  Write: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 4: SCSI Inquiry - standard identification")
    print("=" * 60)
    
    reset()
    
    # Standard SCSI Inquiry command
    inquiry_cmd = bytes([0x12, 0x00, 0x00, 0x00, 0x24, 0x00])  # Request 36 bytes
    cbw = build_scsi_cbw(inquiry_cmd, data_len=36, direction=0x80)
    
    print("\nSCSI Inquiry command:")
    try:
        ep_out.write(cbw, timeout=1000)
        time.sleep(0.1)
        try:
            response = ep_in.read(36, timeout=1000)
            resp = bytes(response)
            print(f"  Response: {resp.hex()}")
            
            # Parse inquiry response
            if len(resp) >= 36:
                vendor = resp[8:16].decode('ascii', errors='replace').strip()
                product = resp[16:32].decode('ascii', errors='replace').strip()
                revision = resp[32:36].decode('ascii', errors='replace').strip()
                print(f"  Vendor: '{vendor}'")
                print(f"  Product: '{product}'")
                print(f"  Revision: '{revision}'")
        except usb.core.USBError as e:
            print(f"  Read: {e}")
    except usb.core.USBError as e:
        print(f"  Write: {e}")
    
    print("\n" + "=" * 60)
    print("TEST 5: Try sending image data wrapped in SCSI")
    print("=" * 60)
    
    # Create small test image
    width, height = 16, 16
    red = struct.pack('<H', (31 << 11))
    image_data = red * (width * height)
    
    # Various SCSI-wrapped write attempts
    write_commands = [
        bytes([0x2A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00]),  # SCSI Write(10)
        bytes([0x0A, 0x00, 0x00, 0x00, 0x02, 0x00]),  # SCSI Write(6)
        bytes([0xCD, 0x00, 0x12, 0x00, 0x00, 0x00, 0x00, 0x0F, 0x00, 0x0F, 0x00]),  # dpf-ax blit
        bytes([0xCD, 0x12]),  # Simplified blit
        bytes([0xCB, 0x00]),  # Alternative vendor
        bytes([0xFE, 0x00]),  # Another vendor range
    ]
    
    for cmd in write_commands:
        reset()
        print(f"\n[TEST] SCSI Write: {cmd.hex()}")
        
        cbw = build_scsi_cbw(cmd, data_len=len(image_data), direction=0x00)
        
        try:
            ep_out.write(cbw, timeout=1000)
            print("  CBW sent")
            time.sleep(0.05)
            
            # Try to send data
            try:
                written = ep_out.write(image_data, timeout=1000)
                print(f"  Data sent: {written} bytes - CHECK DISPLAY!")
                time.sleep(0.5)
            except usb.core.USBError as e:
                print(f"  Data error: {e}")
                
        except usb.core.USBError as e:
            print(f"  CBW error: {e}")
    
    print("\n" + "=" * 60)
    print("SUMMARY")
    print("=" * 60)
    
    if successful_cmds:
        print(f"\nCommands that returned data:")
        for cmd_id, data in successful_cmds[:10]:
            print(f"  0x{cmd_id:04X}: {data[:16].hex()}")
    else:
        print("\nNo commands returned data.")
    
    print("""
The device accepts SCSI CBW packets but:
- Does not respond to standard SCSI commands
- Does not respond to dpf-ax vendor commands

This might be:
1. A completely proprietary protocol that just happens to use CBW framing
2. Needs a specific initialization/handshake first
3. Might need Windows driver analysis to understand
""")


if __name__ == "__main__":
    main()
