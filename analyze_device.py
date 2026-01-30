#!/usr/bin/env python3
"""
AX206 Deep Device Analysis

Investigate why the device is returning I/O errors.
This script will check endpoints, try resets, and probe the device more carefully.
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


def print_device_info(dev):
    """Print detailed device information."""
    print("\n" + "=" * 60)
    print("DEVICE INFORMATION")
    print("=" * 60)
    
    print(f"\nBasic info:")
    print(f"  Bus: {dev.bus}, Address: {dev.address}")
    print(f"  VID:PID = {dev.idVendor:04x}:{dev.idProduct:04x}")
    print(f"  bcdUSB: {dev.bcdUSB:04x}")
    print(f"  bDeviceClass: {dev.bDeviceClass}")
    print(f"  bDeviceSubClass: {dev.bDeviceSubClass}")
    print(f"  bDeviceProtocol: {dev.bDeviceProtocol}")
    print(f"  bMaxPacketSize0: {dev.bMaxPacketSize0}")
    print(f"  bNumConfigurations: {dev.bNumConfigurations}")
    
    # Try to get strings
    print(f"\nString descriptors:")
    try:
        print(f"  Manufacturer: {dev.manufacturer}")
    except Exception as e:
        print(f"  Manufacturer: (error: {e})")
    try:
        print(f"  Product: {dev.product}")
    except Exception as e:
        print(f"  Product: (error: {e})")
    try:
        print(f"  Serial: {dev.serial_number}")
    except Exception as e:
        print(f"  Serial: (error: {e})")
    
    # Configurations
    print(f"\nConfigurations:")
    for cfg in dev:
        print(f"\n  Configuration {cfg.bConfigurationValue}:")
        print(f"    bNumInterfaces: {cfg.bNumInterfaces}")
        print(f"    bmAttributes: 0x{cfg.bmAttributes:02x}")
        print(f"    bMaxPower: {cfg.bMaxPower * 2} mA")
        
        for intf in cfg:
            print(f"\n    Interface {intf.bInterfaceNumber}, Alt {intf.bAlternateSetting}:")
            print(f"      bInterfaceClass: {intf.bInterfaceClass} (0x{intf.bInterfaceClass:02x})")
            print(f"      bInterfaceSubClass: {intf.bInterfaceSubClass} (0x{intf.bInterfaceSubClass:02x})")
            print(f"      bInterfaceProtocol: {intf.bInterfaceProtocol} (0x{intf.bInterfaceProtocol:02x})")
            print(f"      bNumEndpoints: {intf.bNumEndpoints}")
            
            for ep in intf:
                direction = "IN" if usb.util.endpoint_direction(ep.bEndpointAddress) == usb.util.ENDPOINT_IN else "OUT"
                ep_type = {
                    usb.util.ENDPOINT_TYPE_CTRL: "Control",
                    usb.util.ENDPOINT_TYPE_ISO: "Isochronous",
                    usb.util.ENDPOINT_TYPE_BULK: "Bulk",
                    usb.util.ENDPOINT_TYPE_INTR: "Interrupt"
                }.get(usb.util.endpoint_type(ep.bmAttributes), "Unknown")
                
                print(f"\n      Endpoint 0x{ep.bEndpointAddress:02x} ({direction}):")
                print(f"        Type: {ep_type}")
                print(f"        Max Packet Size: {ep.wMaxPacketSize}")
                print(f"        Interval: {ep.bInterval}")


def try_reset(dev):
    """Try to reset the device."""
    print("\n" + "=" * 60)
    print("DEVICE RESET")
    print("=" * 60)
    
    try:
        dev.reset()
        print("  Device reset successful")
        time.sleep(1)  # Wait for device to recover
        return True
    except usb.core.USBError as e:
        print(f"  Reset failed: {e}")
        return False


def try_clear_halt(dev, ep):
    """Try to clear halt on an endpoint."""
    try:
        usb.util.clear_halt(dev, ep)
        print(f"  Cleared halt on endpoint 0x{ep:02x}")
        return True
    except usb.core.USBError as e:
        print(f"  Clear halt 0x{ep:02x} failed: {e}")
        return False


def test_simple_writes(dev):
    """Test basic write operations to find what works."""
    print("\n" + "=" * 60)
    print("SIMPLE WRITE TESTS")
    print("=" * 60)
    
    ep_out = 0x01
    
    # Test 1: Single byte
    print("\n[Test] Single byte write:")
    try:
        written = dev.write(ep_out, b'\x00', timeout=1000)
        print(f"  Written: {written} bytes - SUCCESS")
    except usb.core.USBError as e:
        print(f"  Error: {e}")
    
    # Test 2: 64 bytes (max packet)
    print("\n[Test] 64 byte write:")
    try:
        written = dev.write(ep_out, bytes(64), timeout=1000)
        print(f"  Written: {written} bytes - SUCCESS")
    except usb.core.USBError as e:
        print(f"  Error: {e}")
    
    # Test 3: Just USBC signature
    print("\n[Test] USBC signature only (4 bytes):")
    try:
        written = dev.write(ep_out, b'USBC', timeout=1000)
        print(f"  Written: {written} bytes - SUCCESS")
    except usb.core.USBError as e:
        print(f"  Error: {e}")


def test_control_transfers(dev):
    """Test various control transfers."""
    print("\n" + "=" * 60)
    print("CONTROL TRANSFER TESTS")
    print("=" * 60)
    
    # Standard requests
    tests = [
        # (bmRequestType, bRequest, wValue, wIndex, length/data, description)
        (0x80, 0x00, 0, 0, 2, "GET_STATUS (device)"),
        (0x80, 0x06, 0x0100, 0, 18, "GET_DESCRIPTOR (device)"),
        (0x80, 0x06, 0x0200, 0, 64, "GET_DESCRIPTOR (config)"),
        (0x80, 0x06, 0x0300, 0, 4, "GET_DESCRIPTOR (string lang)"),
        (0xC0, 0x00, 0, 0, 64, "Vendor IN req 0x00"),
        (0xC0, 0x01, 0, 0, 64, "Vendor IN req 0x01"),
        (0xC0, 0x02, 0, 0, 64, "Vendor IN req 0x02"),
        (0xC0, 0x06, 0x0100, 0, 64, "Vendor get device desc"),
    ]
    
    for bmReq, bReq, wVal, wIdx, length, desc in tests:
        try:
            result = dev.ctrl_transfer(bmReq, bReq, wVal, wIdx, length, timeout=500)
            data = bytes(result)
            if len(data) > 16:
                print(f"  {desc}: {len(data)} bytes: {data[:16].hex()}...")
            else:
                print(f"  {desc}: {len(data)} bytes: {data.hex()}")
        except usb.core.USBError as e:
            err = str(e)
            if "not supported" in err.lower() or "stall" in err.lower() or "pipe" in err.lower():
                pass  # Expected for unsupported requests
            else:
                print(f"  {desc}: {e}")


def check_current_configuration(dev):
    """Check what configuration the device is in."""
    print("\n" + "=" * 60)
    print("CONFIGURATION STATE")
    print("=" * 60)
    
    try:
        cfg = dev.get_active_configuration()
        print(f"  Active configuration: {cfg.bConfigurationValue}")
    except usb.core.USBError as e:
        print(f"  No active configuration: {e}")
        return None
    
    return cfg


def try_different_setup(dev):
    """Try different device setup sequences."""
    print("\n" + "=" * 60)
    print("ALTERNATIVE SETUP SEQUENCES")
    print("=" * 60)
    
    # Sequence 1: Reset, then set config
    print("\n[Sequence 1] Reset -> Set Config -> Claim Interface")
    try:
        dev.reset()
        time.sleep(0.5)
        dev.set_configuration(1)
        usb.util.claim_interface(dev, 0)
        print("  Success!")
        
        # Try a write
        try:
            written = dev.write(0x01, bytes(31), timeout=1000)
            print(f"  Test write: {written} bytes")
        except usb.core.USBError as e:
            print(f"  Test write failed: {e}")
            
        usb.util.release_interface(dev, 0)
    except usb.core.USBError as e:
        print(f"  Failed: {e}")
    
    time.sleep(0.5)
    
    # Sequence 2: Clear halts first
    print("\n[Sequence 2] Set Config -> Clear Halts -> Claim Interface")
    try:
        dev.set_configuration(1)
        try_clear_halt(dev, 0x01)
        try_clear_halt(dev, 0x81)
        usb.util.claim_interface(dev, 0)
        print("  Success!")
        
        # Try a write
        try:
            written = dev.write(0x01, bytes(31), timeout=1000)
            print(f"  Test write: {written} bytes")
        except usb.core.USBError as e:
            print(f"  Test write failed: {e}")
            
        usb.util.release_interface(dev, 0)
    except usb.core.USBError as e:
        print(f"  Failed: {e}")
    
    time.sleep(0.5)
    
    # Sequence 3: Set alt interface
    print("\n[Sequence 3] Set Config -> Claim -> Set Alt Interface 0")
    try:
        dev.set_configuration(1)
        usb.util.claim_interface(dev, 0)
        dev.set_interface_altsetting(0, 0)
        print("  Success!")
        
        # Try a write
        try:
            written = dev.write(0x01, bytes(31), timeout=1000)
            print(f"  Test write: {written} bytes")
        except usb.core.USBError as e:
            print(f"  Test write failed: {e}")
            
        usb.util.release_interface(dev, 0)
    except usb.core.USBError as e:
        print(f"  Failed: {e}")


def main():
    print("=" * 60)
    print("AX206 Deep Device Analysis")
    print("=" * 60)
    
    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("\nERROR: Device not found (1908:0102)")
        print("\nSearching for any 1908:* device...")
        
        for d in usb.core.find(find_all=True, idVendor=0x1908):
            print(f"  Found: {d.idVendor:04x}:{d.idProduct:04x}")
        
        sys.exit(1)
    
    # Print device info
    print_device_info(dev)
    
    # Check if kernel driver is active
    print("\n" + "=" * 60)
    print("KERNEL DRIVER CHECK")
    print("=" * 60)
    try:
        for i in range(dev.get_active_configuration().bNumInterfaces if dev.get_active_configuration() else 1):
            try:
                active = dev.is_kernel_driver_active(i)
                print(f"  Interface {i}: kernel driver {'active' if active else 'not active'}")
                if active:
                    dev.detach_kernel_driver(i)
                    print(f"  Interface {i}: detached kernel driver")
            except Exception as e:
                print(f"  Interface {i}: {e}")
    except Exception as e:
        print(f"  Error checking: {e}")
    
    # Check configuration
    check_current_configuration(dev)
    
    # Try reset
    try_reset(dev)
    
    # Try different setups
    try_different_setup(dev)
    
    # Setup for remaining tests
    print("\n" + "=" * 60)
    print("FINAL SETUP FOR TESTS")
    print("=" * 60)
    try:
        dev.set_configuration()
        print("  Configuration set")
    except usb.core.USBError as e:
        print(f"  Config: {e}")
    
    try:
        usb.util.claim_interface(dev, 0)
        print("  Interface claimed")
    except usb.core.USBError as e:
        print(f"  Interface: {e}")
    
    # Test control transfers
    test_control_transfers(dev)
    
    # Test simple writes
    test_simple_writes(dev)
    
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
