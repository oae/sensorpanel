#!/usr/bin/env python3
"""
Benchmark maximum FPS for the AX206 display.
Tests how fast we can push frames to the panel.
"""

import time
import struct
import sys

try:
    import usb.core
    import usb.util
except ImportError:
    print("ERROR: pyusb not installed")
    sys.exit(1)

# USB constants
ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81
WIDTH = 480
HEIGHT = 320
FRAME_SIZE = WIDTH * HEIGHT * 2  # 307200 bytes


def build_cbw(data_length: int) -> bytes:
    """Build CBW for BLIT command."""
    cmd = bytearray(16)
    cmd[0] = 0xCD
    cmd[5] = 0x06
    cmd[6] = 0x12
    cmd[11] = (WIDTH - 1) & 0xFF
    cmd[12] = ((WIDTH - 1) >> 8) & 0xFF
    cmd[13] = (HEIGHT - 1) & 0xFF
    cmd[14] = ((HEIGHT - 1) >> 8) & 0xFF

    cbw = struct.pack(
        "<4sIIBBB", b"USBC", 0xDEADBEEF, data_length, 0x00, 0x00, 16
    ) + bytes(cmd)
    return cbw


def create_test_frame(frame_num: int) -> bytes:
    """Create a test frame with alternating colors."""
    # Alternate between red and blue frames for visual confirmation
    if frame_num % 2 == 0:
        color = struct.pack(">H", 0xF800)  # Red
    else:
        color = struct.pack(">H", 0x001F)  # Blue
    return color * (WIDTH * HEIGHT)


def main():
    print("=" * 60)
    print("AX206 Display FPS Benchmark")
    print("=" * 60)

    # Find device
    dev = usb.core.find(idVendor=0x1908, idProduct=0x0102)
    if dev is None:
        print("ERROR: Device not found")
        sys.exit(1)

    print(f"Device found: {dev.manufacturer} {dev.product}")

    # Setup
    try:
        if dev.is_kernel_driver_active(0):
            dev.detach_kernel_driver(0)
    except Exception:
        pass

    try:
        dev.set_configuration()
    except usb.core.USBError:
        pass

    usb.util.claim_interface(dev, 0)

    # Clear halts
    try:
        dev.clear_halt(ENDPOINT_OUT)
        dev.clear_halt(ENDPOINT_IN)
    except:
        pass

    # Turn on backlight
    print("Turning on backlight...")
    backlight_cmd = bytearray(16)
    backlight_cmd[0] = 0xCD
    backlight_cmd[5] = 0x06
    backlight_cmd[6] = 0x01
    backlight_cmd[7] = 0x01
    backlight_cmd[9] = 0x07  # Max brightness

    backlight_cbw = struct.pack(
        "<4sIIBBB",
        b"USBC",
        0xDEADBEEF,
        0,  # No data
        0x00,
        0x00,
        16,
    ) + bytes(backlight_cmd)

    try:
        dev.write(ENDPOINT_OUT, backlight_cbw, timeout=1000)
        dev.read(ENDPOINT_IN, 13, timeout=1000)
    except:
        pass

    print(f"\nFrame size: {FRAME_SIZE:,} bytes ({FRAME_SIZE / 1024:.1f} KB)")
    print(f"Resolution: {WIDTH}x{HEIGHT}")

    # Benchmark
    num_frames = 30
    print(f"\nSending {num_frames} frames...")
    print("Watch the display - it should alternate red/blue rapidly\n")

    cbw = build_cbw(FRAME_SIZE)

    # Pre-generate frames
    frames = [create_test_frame(i) for i in range(2)]

    errors = 0
    start_time = time.time()

    for i in range(num_frames):
        frame_start = time.time()
        frame = frames[i % 2]

        try:
            # Send CBW
            dev.write(ENDPOINT_OUT, cbw, timeout=5000)

            # Send frame data
            dev.write(ENDPOINT_OUT, frame, timeout=10000)

            # Read CSW
            csw = dev.read(ENDPOINT_IN, 13, timeout=5000)

            frame_time = time.time() - frame_start
            fps_instant = 1.0 / frame_time if frame_time > 0 else 0

            if (i + 1) % 5 == 0:
                print(
                    f"  Frame {i + 1}/{num_frames}: {frame_time * 1000:.1f}ms ({fps_instant:.2f} FPS)"
                )

        except usb.core.USBError as e:
            print(f"  Frame {i + 1}: ERROR - {e}")
            errors += 1
            # Try to recover
            try:
                dev.clear_halt(ENDPOINT_OUT)
                dev.clear_halt(ENDPOINT_IN)
            except:
                pass

    end_time = time.time()
    total_time = end_time - start_time
    successful_frames = num_frames - errors
    avg_fps = successful_frames / total_time if total_time > 0 else 0

    print("\n" + "=" * 60)
    print("RESULTS")
    print("=" * 60)
    print(f"Total frames: {num_frames}")
    print(f"Successful:   {successful_frames}")
    print(f"Errors:       {errors}")
    print(f"Total time:   {total_time:.2f}s")
    print(f"Average FPS:  {avg_fps:.2f}")
    print(
        f"Frame time:   {total_time / successful_frames * 1000:.1f}ms"
        if successful_frames > 0
        else "N/A"
    )
    print(
        f"\nData throughput: {(successful_frames * FRAME_SIZE) / total_time / 1024:.1f} KB/s"
    )
    print("=" * 60)

    # Cleanup
    usb.util.release_interface(dev, 0)
    dev.reset()


if __name__ == "__main__":
    main()
