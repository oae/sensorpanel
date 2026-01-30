"""
AX206 USB Protocol Implementation

This module implements the USB protocol for AX206-based digital photo frames,
based on the dpf-ax project (https://github.com/dreamlayers/dpf-ax).

Protocol overview:
- Uses USB bulk transfers
- Commands are wrapped in SCSI Command Block Wrapper (CBW) format
- Vendor-specific commands use 0xCD prefix
- After data transfer, device sends CSW (Command Status Wrapper) response
- Pixel data is sent as raw RGB565 (16-bit, little-endian)
"""

import struct
from typing import Tuple
from dataclasses import dataclass

# =============================================================================
# USB Constants
# =============================================================================

VENDOR_ID = 0x1908
PRODUCT_ID = 0x0102
PRODUCT_ID_BOOTLOADER = 0x3318  # Device in bootloader mode

ENDPOINT_OUT = 0x01
ENDPOINT_IN = 0x81

# USB timeout in milliseconds
USB_TIMEOUT = 5000  # Increased for QTKeJi devices
USB_TIMEOUT_DATA = 10000  # Longer timeout for data transfers

# =============================================================================
# SCSI CBW (Command Block Wrapper) Constants
# =============================================================================

CBW_SIGNATURE = b"USBC"
CSW_SIGNATURE = b"USBS"
CBW_LENGTH = 31  # Standard CBW length
CSW_LENGTH = 13  # Standard CSW length

# Direction flags
DIR_OUT = 0x00  # Host to device (data out)
DIR_IN = 0x80  # Device to host (data in)

# =============================================================================
# AX206 Vendor Commands (via SCSI)
# Based on dpf-ax/include/usbuser.h
# =============================================================================

# Main vendor command prefix (first byte of SCSI CDB)
VENDOR_CMD_PREFIX = 0xCD

# =============================================================================
# QTKeJi/AIDA64 Protocol (discovered via USB capture)
# This device uses a different command format than standard dpf-ax
# =============================================================================

# cmd[5] = operation type
SUBCMD_GET_PARAM = 0x02  # Get LCD parameters (width, height)
SUBCMD_PROBE = 0x03  # Probe protocol version
SUBCMD_BLIT = 0x06  # Blit operation

# cmd[6] = subcommand (used with SUBCMD_BLIT)
BLIT_CMD_SETPROP = 0x01  # Set property (brightness, etc)
BLIT_CMD_WRITE = 0x12  # Write image data to screen

# Legacy dpf-ax subcommands (for reference, not used by this device)
# USBCMD_GETPROPERTY = 0x00
# USBCMD_SETPROPERTY = 0x01
# USBCMD_MEMREAD = 0x04
# USBCMD_APPLOAD = 0x05
# USBCMD_FILLRECT = 0x11
# USBCMD_BLIT = 0x12
# USBCMD_COPYRECT = 0x13
# USBCMD_FLASHLOCK = 0x20
# USBCMD_PROBE = 0xff

# Properties for GET/SET_PROPERTY
PROPERTY_BRIGHTNESS = 0x01
PROPERTY_FGCOLOR = 0x02
PROPERTY_BGCOLOR = 0x03
PROPERTY_ORIENTATION = 0x10


@dataclass
class DeviceInfo:
    """Information about the connected AX206 device."""

    width: int
    height: int
    bpp: int = 2  # Bytes per pixel (RGB565 = 2)
    protocol_version: int = 0
    can_lock: bool = False

    @property
    def pixel_count(self) -> int:
        return self.width * self.height

    @property
    def buffer_size(self) -> int:
        """Size of RGB565 buffer in bytes."""
        return self.pixel_count * self.bpp


def build_cbw(
    command: bytes,
    data_length: int = 0,
    direction: int = DIR_OUT,
    tag: int = 0xDEADBEEF,
    lun: int = 0,
) -> bytes:
    """
    Build a SCSI Command Block Wrapper (CBW).

    CBW Structure (31 bytes):
    - Bytes 0-3: Signature ('USBC')
    - Bytes 4-7: Tag (echoed in CSW response)
    - Bytes 8-11: Data Transfer Length (little-endian)
    - Byte 12: Flags (0x80 = data in, 0x00 = data out)
    - Byte 13: LUN (logical unit number)
    - Byte 14: CB Length (command block length, 1-16)
    - Bytes 15-30: Command Block (16 bytes, padded with zeros)

    Args:
        command: The SCSI command bytes (max 16 bytes)
        data_length: Number of bytes to transfer after CBW
        direction: DIR_OUT or DIR_IN
        tag: Transaction tag (for matching responses)
        lun: Logical unit number (usually 0)

    Returns:
        31-byte CBW packet
    """
    if len(command) > 16:
        raise ValueError("Command block cannot exceed 16 bytes")

    # Pad command to 16 bytes
    padded_cmd = command.ljust(16, b"\x00")

    cbw = struct.pack(
        "<4sIIBBB16s",
        CBW_SIGNATURE,  # Bytes 0-3: Signature
        tag,  # Bytes 4-7: Tag
        data_length,  # Bytes 8-11: Data transfer length
        direction,  # Byte 12: Flags
        lun,  # Byte 13: LUN
        len(command),  # Byte 14: CB Length
        padded_cmd,  # Bytes 15-30: Command Block
    )

    return cbw


def parse_csw(csw: bytes) -> Tuple[int, int]:
    """
    Parse Command Status Wrapper (CSW) response.

    CSW Structure (13 bytes):
    - Bytes 0-3: Signature ('USBS')
    - Bytes 4-7: Tag (matches CBW tag)
    - Bytes 8-11: Data Residue
    - Byte 12: Status (0 = success)

    Args:
        csw: 13-byte CSW response from device

    Returns:
        Tuple of (tag, status)

    Raises:
        ValueError: If CSW signature is invalid
    """
    if len(csw) < CSW_LENGTH:
        raise ValueError(f"CSW too short: {len(csw)} bytes, expected {CSW_LENGTH}")

    signature = csw[0:4]
    if signature != CSW_SIGNATURE:
        raise ValueError(
            f"Invalid CSW signature: {signature!r}, expected {CSW_SIGNATURE!r}"
        )

    tag = struct.unpack("<I", csw[4:8])[0]
    status = csw[12]

    return tag, status


def build_vendor_cmd(
    subcmd: int,
    param1: int = 0,
    param2: int = 0,
    param3: int = 0,
    param4: int = 0,
    param5: int = 0,
) -> bytes:
    """
    Build a vendor-specific command.

    Command format (from dpf-ax scsi.c g_excmd):
    Byte 0: 0xCD (vendor prefix)
    Byte 1-5: Reserved (zeros)
    Byte 6: Subcommand
    Byte 7-8: Parameter 1 (16-bit LE)
    Byte 9-10: Parameter 2 (16-bit LE)
    Byte 11-12: Parameter 3 (16-bit LE)
    Byte 13-14: Parameter 4 (16-bit LE)
    Byte 15: Parameter 5

    Args:
        subcmd: Subcommand code (USBCMD_*)
        param1-5: Command parameters

    Returns:
        16-byte command block
    """
    cmd = bytearray(16)
    cmd[0] = VENDOR_CMD_PREFIX  # 0xCD
    cmd[6] = subcmd
    cmd[7] = param1 & 0xFF
    cmd[8] = (param1 >> 8) & 0xFF
    cmd[9] = param2 & 0xFF
    cmd[10] = (param2 >> 8) & 0xFF
    cmd[11] = param3 & 0xFF
    cmd[12] = (param3 >> 8) & 0xFF
    cmd[13] = param4 & 0xFF
    cmd[14] = (param4 >> 8) & 0xFF
    cmd[15] = param5 & 0xFF
    return bytes(cmd)


def build_get_params_cmd() -> bytes:
    """
    Build command to query LCD parameters (width, height).

    From dpf-ax scsi.c probe():
    cmd[5] = 2; // get LCD parameters
    Response is 5 bytes: width_lo, width_hi, height_lo, height_hi, bpp

    Returns:
        CBW packet for GET_PARAM command
    """
    # Command format: 0xCD, 0, 0, 0, 0, 2 (subcmd in byte 5)
    cmd = bytes(
        [
            VENDOR_CMD_PREFIX,
            0x00,
            0x00,
            0x00,
            0x00,
            SUBCMD_GET_PARAM,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
        ]
    )
    return build_cbw(cmd, data_length=5, direction=DIR_IN)


def parse_params_response(data: bytes) -> Tuple[int, int]:
    """
    Parse response from GET_PARAM command.

    Response format (5 bytes) from dpf-ax probe():
    - Bytes 0-1: Width (little-endian)
    - Bytes 2-3: Height (little-endian)
    - Byte 4: BPP (bytes per pixel, usually 2)

    Args:
        data: 5-byte response from device

    Returns:
        Tuple of (width, height)
    """
    if len(data) < 5:
        raise ValueError(f"Response too short: {len(data)} bytes, expected 5")

    width = struct.unpack("<H", data[0:2])[0]
    height = struct.unpack("<H", data[2:4])[0]

    return width, height


def build_probe_cmd() -> bytes:
    """
    Build command to probe device protocol version.

    From dpf-ax scsi.c probe():
    cmd[5] = 3; // probe
    Return value indicates protocol:
    - 0: Original protocol (no flash lock)
    - 1: Improved hack (has flash lock)

    Returns:
        CBW packet for PROBE command
    """
    cmd = bytes(
        [
            VENDOR_CMD_PREFIX,
            0x00,
            0x00,
            0x00,
            0x00,
            SUBCMD_PROBE,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
            0x00,
        ]
    )
    return build_cbw(cmd, data_length=0, direction=DIR_IN)


def build_set_property_cmd(property_id: int, value: int) -> bytes:
    """
    Build command to set a device property (e.g., brightness).

    QTKeJi/AIDA64 Protocol (discovered via USB capture):
    cmd[0] = 0xCD (vendor prefix)
    cmd[5] = 0x06 (BLIT/display operation)
    cmd[6] = 0x01 (set property subcommand)
    cmd[7] = 0x01 (property enabled)
    cmd[8-10] = value bytes

    Captured: cd 00 00 00 00 06 01 01 00 07 00 00 00 00 00 00

    Args:
        property_id: Property token (PROPERTY_*)
        value: Value to set (16-bit)

    Returns:
        CBW packet for SET_PROPERTY command
    """
    cmd = bytearray(16)
    cmd[0] = VENDOR_CMD_PREFIX  # 0xCD
    cmd[5] = SUBCMD_BLIT  # 0x06
    cmd[6] = BLIT_CMD_SETPROP  # 0x01
    cmd[7] = 0x01  # Enable
    cmd[8] = 0x00
    cmd[9] = value & 0xFF
    cmd[10] = (value >> 8) & 0xFF
    return build_cbw(bytes(cmd), data_length=0, direction=DIR_OUT)


def build_set_backlight_cmd(level: int) -> bytes:
    """
    Build command to set backlight level.

    Args:
        level: Backlight level (0-7, where 7 is brightest)

    Returns:
        CBW packet for SET_PROPERTY (brightness) command
    """
    level = max(0, min(7, level))
    return build_set_property_cmd(PROPERTY_BRIGHTNESS, level)


def build_blit_cmd(x0: int, y0: int, x1: int, y1: int) -> bytes:
    """
    Build command to blit (send) pixel data to screen.

    QTKeJi/AIDA64 Protocol (discovered via USB capture):
    cmd[0] = 0xCD (vendor prefix)
    cmd[5] = 0x06 (BLIT operation type)
    cmd[6] = 0x12 (write image subcommand)
    cmd[7-10] = 0 (reserved/unused for full-screen blit)
    cmd[11-12] = width - 1 (little-endian)
    cmd[13-14] = height - 1 (little-endian)

    Captured command: cd 00 00 00 00 06 12 00 00 00 00 df 01 3f 01 00
    Where 0x01df = 479 (width-1), 0x013f = 319 (height-1)

    Args:
        x0, y0: Top-left corner (usually 0, 0 for full screen)
        x1, y1: Bottom-right corner (inclusive, e.g., 479, 319)

    Returns:
        CBW packet for BLIT command
    """
    width = x1 - x0 + 1
    height = y1 - y0 + 1
    pixel_count = width * height
    data_length = pixel_count * 2  # RGB565 = 2 bytes per pixel

    # Build command using QTKeJi/AIDA64 format
    cmd = bytearray(16)
    cmd[0] = VENDOR_CMD_PREFIX  # 0xCD
    cmd[5] = SUBCMD_BLIT  # 0x06 - BLIT operation type
    cmd[6] = BLIT_CMD_WRITE  # 0x12 - Write image subcommand
    # cmd[7-10] = 0 for full-screen blit (x0, y0 offset - not used)
    cmd[11] = x1 & 0xFF  # width - 1, low byte
    cmd[12] = (x1 >> 8) & 0xFF  # width - 1, high byte
    cmd[13] = y1 & 0xFF  # height - 1, low byte
    cmd[14] = (y1 >> 8) & 0xFF  # height - 1, high byte

    return build_cbw(bytes(cmd), data_length=data_length, direction=DIR_OUT)


def build_fillrect_cmd(x0: int, y0: int, x1: int, y1: int, color_rgb565: int) -> bytes:
    """
    Build command to fill a screen rectangle with a color.

    NOTE: This command may not be supported by QTKeJi devices.
    Prefer using build_blit_cmd with a solid color buffer instead.

    Args:
        x0, y0: Top-left corner
        x1, y1: Bottom-right corner (inclusive)
        color_rgb565: Fill color in RGB565 format

    Returns:
        CBW packet for FILLRECT command (may not work on QTKeJi devices)
    """
    # For QTKeJi devices, fill rect by blitting a solid color buffer
    # This is a placeholder - actual implementation should use blit
    width = x1 - x0 + 1
    height = y1 - y0 + 1

    cmd = bytearray(16)
    cmd[0] = VENDOR_CMD_PREFIX
    cmd[5] = SUBCMD_BLIT
    cmd[6] = 0x11  # FILLRECT subcommand (may not be supported)
    cmd[11] = x1 & 0xFF
    cmd[12] = (x1 >> 8) & 0xFF
    cmd[13] = y1 & 0xFF
    cmd[14] = (y1 >> 8) & 0xFF

    return build_cbw(bytes(cmd), data_length=0, direction=DIR_OUT)


def rgb_to_rgb565(r: int, g: int, b: int) -> int:
    """
    Convert 24-bit RGB to 16-bit RGB565.

    Standard RGB565 format:
    - Red: 5 bits (bits 15-11)
    - Green: 6 bits (bits 10-5)
    - Blue: 5 bits (bits 4-0)

    Args:
        r, g, b: 8-bit color components (0-255)

    Returns:
        16-bit RGB565 value
    """
    r5 = (r >> 3) & 0x1F
    g6 = (g >> 2) & 0x3F
    b5 = (b >> 3) & 0x1F
    return (r5 << 11) | (g6 << 5) | b5


def rgb565_to_bytes(rgb565: int) -> bytes:
    """
    Convert RGB565 value to bytes for wire transfer.

    QTKeJi/AIDA64 devices expect big-endian byte order
    (high byte first, low byte second).

    Args:
        rgb565: 16-bit RGB565 value

    Returns:
        2-byte big-endian representation
    """
    return struct.pack(">H", rgb565)  # Big-endian for QTKeJi devices


def image_to_rgb565_buffer(pixels, width: int, height: int) -> bytes:
    """
    Convert image pixel data to RGB565 buffer.

    Args:
        pixels: PIL Image pixel access object or list of (r, g, b) tuples
        width: Image width
        height: Image height

    Returns:
        Raw RGB565 bytes ready to send to device
    """
    buffer = bytearray(width * height * 2)
    idx = 0

    for y in range(height):
        for x in range(width):
            pixel = pixels[x, y]
            if isinstance(pixel, int):
                # Grayscale
                r = g = b = pixel
            else:
                r, g, b = pixel[:3]  # Handle RGB or RGBA
            rgb565 = rgb_to_rgb565(r, g, b)
            buffer[idx : idx + 2] = rgb565_to_bytes(rgb565)
            idx += 2

    return bytes(buffer)


def create_solid_color_buffer(width: int, height: int, r: int, g: int, b: int) -> bytes:
    """
    Create a solid color RGB565 buffer for testing.

    Args:
        width, height: Buffer dimensions
        r, g, b: Color components (0-255)

    Returns:
        RGB565 buffer filled with the specified color
    """
    rgb565 = rgb_to_rgb565(r, g, b)
    pixel_bytes = rgb565_to_bytes(rgb565)
    return pixel_bytes * (width * height)
