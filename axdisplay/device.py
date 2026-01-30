"""
AX206 USB Device Interface

High-level interface for communicating with AX206 displays using pyusb.
Based on the dpf-ax project (https://github.com/dreamlayers/dpf-ax).

Protocol sequence:
1. Send CBW (Command Block Wrapper) - 31 bytes
2. Send/receive data (if any)
3. Read CSW (Command Status Wrapper) - 13 bytes response
"""

import time
import logging
from typing import Optional, List, Tuple
from contextlib import contextmanager

try:
    import usb.core
    import usb.util
except ImportError:
    raise ImportError("pyusb is required. Install with: pip install pyusb")

from .protocol import (
    VENDOR_ID,
    PRODUCT_ID,
    PRODUCT_ID_BOOTLOADER,
    ENDPOINT_OUT,
    ENDPOINT_IN,
    USB_TIMEOUT,
    USB_TIMEOUT_DATA,
    CSW_LENGTH,
    DeviceInfo,
    build_get_params_cmd,
    parse_params_response,
    parse_csw,
    build_probe_cmd,
    build_blit_cmd,
    build_set_backlight_cmd,
    image_to_rgb565_buffer,
    create_solid_color_buffer,
    DIR_IN,
    DIR_OUT,
)
from .config import Config

logger = logging.getLogger(__name__)


class DeviceNotFoundError(Exception):
    """Raised when no AX206 device is found."""

    pass


class DeviceCommunicationError(Exception):
    """Raised when communication with the device fails."""

    pass


class AX206Device:
    """
    High-level interface for AX206 USB displays.

    This implements the dpf-ax protocol which uses SCSI-like command
    wrapping over USB bulk transfers.

    Usage:
        with AX206Device() as device:
            device.display_image(pil_image)

    Or manually:
        device = AX206Device()
        device.open()
        try:
            device.display_image(pil_image)
        finally:
            device.close()
    """

    def __init__(self, config: Optional[Config] = None):
        """
        Initialize device interface.

        Args:
            config: Configuration object. If None, uses defaults.
        """
        self.config = config or Config()
        self._device: Optional[usb.core.Device] = None
        self._info: Optional[DeviceInfo] = None
        self._was_kernel_driver_active = False

    @property
    def is_open(self) -> bool:
        """Check if device is currently open."""
        return self._device is not None

    @property
    def info(self) -> Optional[DeviceInfo]:
        """Get device info (available after open())."""
        return self._info

    @property
    def width(self) -> int:
        """Display width in pixels."""
        return self._info.width if self._info else self.config.width

    @property
    def height(self) -> int:
        """Display height in pixels."""
        return self._info.height if self._info else self.config.height

    @staticmethod
    def find_devices() -> List[usb.core.Device]:
        """
        Find all connected AX206 devices.

        Returns:
            List of USB device objects
        """
        devices = list(
            usb.core.find(find_all=True, idVendor=VENDOR_ID, idProduct=PRODUCT_ID)
        )
        return devices

    @staticmethod
    def is_device_available() -> bool:
        """Check if any AX206 device is connected."""
        return len(AX206Device.find_devices()) > 0

    def open(self, device_index: int = 0) -> None:
        """
        Open connection to the AX206 device.

        Args:
            device_index: Which device to open if multiple are connected

        Raises:
            DeviceNotFoundError: If no device is found
            DeviceCommunicationError: If device cannot be initialized
        """
        if self._device is not None:
            logger.warning("Device already open, closing first")
            self.close()

        # Find device
        devices = self.find_devices()
        if not devices:
            raise DeviceNotFoundError(
                f"No AX206 device found (looking for {VENDOR_ID:04x}:{PRODUCT_ID:04x})"
            )

        if device_index >= len(devices):
            raise DeviceNotFoundError(
                f"Device index {device_index} out of range (found {len(devices)} devices)"
            )

        self._device = devices[device_index]

        try:
            manufacturer = self._device.manufacturer or "Unknown"
            product = self._device.product or "Unknown"
            logger.info(f"Found device: {manufacturer} {product}")
        except Exception:
            logger.info("Found AX206 device")

        # Detach kernel driver if necessary
        try:
            if self._device.is_kernel_driver_active(0):
                logger.debug("Detaching kernel driver")
                self._device.detach_kernel_driver(0)
                self._was_kernel_driver_active = True
        except (usb.core.USBError, NotImplementedError) as e:
            logger.debug(f"Kernel driver check/detach: {e}")

        # Set configuration
        try:
            self._device.set_configuration()
        except usb.core.USBError as e:
            if "Resource busy" in str(e):
                logger.debug("Device already configured")
            else:
                raise DeviceCommunicationError(f"Failed to set configuration: {e}")

        # Claim interface
        try:
            usb.util.claim_interface(self._device, 0)
        except usb.core.USBError as e:
            logger.debug(f"Interface claim: {e}")

        # Clear any stalled endpoints (important after failed transfers)
        try:
            self._device.clear_halt(ENDPOINT_OUT)
        except usb.core.USBError:
            pass
        try:
            self._device.clear_halt(ENDPOINT_IN)
        except usb.core.USBError:
            pass

        # Flush any pending data from IN endpoint
        self._usb_flush()

        # QTKeJi/AIDA64-compatible devices don't support GET_PARAMS query,
        # so we skip device query and use configured resolution directly.
        # This matches the working test_blit_only.py approach.
        self._info = DeviceInfo(width=self.config.width, height=self.config.height)

        # Turn on the backlight at maximum brightness
        try:
            self.set_backlight(7)
        except Exception as e:
            logger.debug(f"Could not set backlight: {e}")

        logger.info(f"Device opened: {self._info.width}x{self._info.height}")

    def _usb_flush(self) -> None:
        """Flush any pending data from the IN endpoint."""
        try:
            self._device.read(ENDPOINT_IN, 64, timeout=100)
        except usb.core.USBError:
            pass  # Expected - no data to read

    def _scsi_command(
        self,
        cbw: bytes,
        data_out: Optional[bytes] = None,
        data_in_length: int = 0,
        timeout: int = USB_TIMEOUT,
    ) -> Tuple[Optional[bytes], int]:
        """
        Execute a SCSI command with proper CBW/CSW protocol.

        This follows the dpf-ax emulate_scsi() function:
        1. Send 31-byte CBW
        2. Send data (if data_out) or receive data (if data_in_length > 0)
        3. Read 13-byte CSW response

        Args:
            cbw: 31-byte Command Block Wrapper
            data_out: Data to send after CBW (for DIR_OUT commands)
            data_in_length: Expected response data length (for DIR_IN commands)
            timeout: USB timeout in milliseconds

        Returns:
            Tuple of (received_data or None, status_code)

        Raises:
            DeviceCommunicationError: On USB errors
        """
        received_data = None

        try:
            # Step 1: Send CBW
            written = self._device.write(ENDPOINT_OUT, cbw, timeout=timeout)
            if written != len(cbw):
                raise DeviceCommunicationError(
                    f"CBW write incomplete: {written}/{len(cbw)}"
                )

            # Step 2: Data phase
            if data_out is not None:
                # Send data to device
                written = self._device.write(
                    ENDPOINT_OUT, data_out, timeout=USB_TIMEOUT_DATA
                )
                if written != len(data_out):
                    raise DeviceCommunicationError(
                        f"Data write incomplete: {written}/{len(data_out)}"
                    )
            elif data_in_length > 0:
                # Receive data from device
                received = self._device.read(
                    ENDPOINT_IN, data_in_length, timeout=USB_TIMEOUT_DATA
                )
                received_data = bytes(received)

            # Step 3: Read CSW (Command Status Wrapper)
            csw_data = None
            retries = 5
            for retry in range(retries):
                try:
                    csw_raw = self._device.read(
                        ENDPOINT_IN, CSW_LENGTH, timeout=timeout * 2
                    )
                    csw_data = bytes(csw_raw)
                    break
                except usb.core.USBError as e:
                    if retry < retries - 1:
                        logger.debug(f"CSW read retry {retry + 1}: {e}")
                        time.sleep(0.1)
                    else:
                        raise

            if csw_data is None:
                raise DeviceCommunicationError("Failed to read CSW after retries")

            # Parse CSW
            try:
                _, status = parse_csw(csw_data)
            except ValueError as e:
                logger.warning(f"CSW parse error: {e}, raw: {csw_data.hex()}")
                status = -1

            return received_data, status

        except usb.core.USBError as e:
            raise DeviceCommunicationError(f"USB error: {e}")

    def _query_device_info(self) -> None:
        """Query device for resolution and other parameters."""
        # First try probe command
        try:
            probe_cbw = build_probe_cmd()
            _, probe_status = self._scsi_command(probe_cbw)
            can_lock = probe_status == 1
            logger.debug(f"Probe status: {probe_status}, can_lock: {can_lock}")
        except Exception as e:
            logger.debug(f"Probe failed: {e}")
            can_lock = False

        # Get LCD parameters
        params_cbw = build_get_params_cmd()
        response, status = self._scsi_command(params_cbw, data_in_length=5)

        if response is None:
            raise DeviceCommunicationError("No response to GET_PARAMS")

        width, height = parse_params_response(response)
        self._info = DeviceInfo(width=width, height=height, can_lock=can_lock)

    def close(self) -> None:
        """Close the device connection and turn off the display."""
        if self._device is None:
            return

        # Turn off the backlight before closing
        try:
            self.set_backlight(0)
        except Exception:
            pass

        try:
            usb.util.release_interface(self._device, 0)
        except Exception:
            pass

        try:
            # Reset the device so it returns to its default state
            self._device.reset()
        except Exception:
            pass

        try:
            usb.util.dispose_resources(self._device)

            # Reattach kernel driver if we detached it
            if self._was_kernel_driver_active:
                try:
                    self._device.attach_kernel_driver(0)
                except Exception:
                    pass
        except Exception as e:
            logger.warning(f"Error closing device: {e}")
        finally:
            self._device = None
            self._info = None
            self._was_kernel_driver_active = False

        logger.debug("Device closed")

    def __enter__(self) -> "AX206Device":
        """Context manager entry."""
        self.open()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        """Context manager exit."""
        self.close()

    def set_backlight(self, level: int) -> None:
        """
        Set backlight brightness level.

        Args:
            level: Brightness level (0-7, where 7 is brightest)
        """
        if not self.is_open:
            raise DeviceCommunicationError("Device not open")

        cbw = build_set_backlight_cmd(level)
        _, status = self._scsi_command(cbw)
        if status != 0:
            logger.warning(f"Set backlight returned status {status}")

    def display_buffer(self, rgb565_buffer: bytes) -> None:
        """
        Send raw RGB565 buffer to display.

        Args:
            rgb565_buffer: Raw pixel data in RGB565 format
        """
        if not self.is_open:
            raise DeviceCommunicationError("Device not open")

        expected_size = self.width * self.height * 2
        if len(rgb565_buffer) != expected_size:
            raise ValueError(
                f"Buffer size mismatch: got {len(rgb565_buffer)}, "
                f"expected {expected_size} for {self.width}x{self.height}"
            )

        # Build blit command for full screen
        cbw = build_blit_cmd(0, 0, self.width - 1, self.height - 1)

        # Send command with pixel data
        _, status = self._scsi_command(cbw, data_out=rgb565_buffer)
        if status != 0:
            logger.warning(f"Blit returned status {status}")

    def display_image(self, image) -> None:
        """
        Display a PIL Image on the screen.

        Args:
            image: PIL Image object (will be converted to RGB565)
        """
        # Ensure correct size
        if image.size != (self.width, self.height):
            image = image.resize((self.width, self.height))

        # Ensure RGB mode
        if image.mode != "RGB":
            image = image.convert("RGB")

        # Apply rotation if configured
        if self.config.rotation != 0:
            from PIL import Image as PILImage

            rotation_map = {
                90: PILImage.Transpose.ROTATE_90,
                180: PILImage.Transpose.ROTATE_180,
                270: PILImage.Transpose.ROTATE_270,
            }
            if self.config.rotation in rotation_map:
                image = image.transpose(rotation_map[self.config.rotation])

        # Convert to RGB565
        pixels = image.load()
        buffer = image_to_rgb565_buffer(pixels, self.width, self.height)

        self.display_buffer(buffer)

    def display_solid_color(self, r: int, g: int, b: int) -> None:
        """
        Fill screen with a solid color (useful for testing).

        Args:
            r, g, b: Color components (0-255)
        """
        buffer = create_solid_color_buffer(self.width, self.height, r, g, b)
        self.display_buffer(buffer)

    def test_pattern(self) -> None:
        """Display a test pattern to verify device is working."""
        from PIL import Image, ImageDraw

        img = Image.new("RGB", (self.width, self.height), (0, 0, 0))
        draw = ImageDraw.Draw(img)

        # Draw colored bars
        bar_height = self.height // 8
        colors = [
            (255, 255, 255),  # White
            (255, 255, 0),  # Yellow
            (0, 255, 255),  # Cyan
            (0, 255, 0),  # Green
            (255, 0, 255),  # Magenta
            (255, 0, 0),  # Red
            (0, 0, 255),  # Blue
            (0, 0, 0),  # Black
        ]

        for i, color in enumerate(colors):
            y0 = i * bar_height
            y1 = (i + 1) * bar_height
            draw.rectangle([0, y0, self.width, y1], fill=color)

        # Draw device info text
        text = f"{self.width}x{self.height}"
        draw.text((10, 10), text, fill=(0, 0, 0))
        draw.text((10, self.height - 30), "AX206 Test", fill=(255, 255, 255))

        self.display_image(img)


@contextmanager
def open_device(
    config: Optional[Config] = None, max_retries: int = 5, retry_delay: float = 2.0
):
    """
    Context manager for opening AX206 device with retry logic.

    Args:
        config: Configuration object
        max_retries: Maximum number of connection attempts
        retry_delay: Delay between retries in seconds

    Yields:
        AX206Device instance

    Raises:
        DeviceNotFoundError: If device cannot be found after all retries
    """
    device = AX206Device(config)

    for attempt in range(max_retries):
        try:
            device.open()
            break
        except DeviceNotFoundError:
            if attempt < max_retries - 1:
                logger.warning(
                    f"Device not found, retrying in {retry_delay}s (attempt {attempt + 1}/{max_retries})"
                )
                time.sleep(retry_delay)
            else:
                raise

    try:
        yield device
    finally:
        device.close()
