#!/usr/bin/env python3
"""
AX206 Sensor Display - Main Entry Point

This module provides the main entry point for the axdisplay application.
It can be run as: python -m axdisplay
"""

import sys
import time
import signal
import logging
import argparse
from typing import Optional

from .config import Config
from .device import AX206Device, DeviceNotFoundError, DeviceCommunicationError
from .sensors import SensorCollector
from .renderer import DashboardRenderer

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)
logger = logging.getLogger("axdisplay")


class DisplayDaemon:
    """
    Main daemon that collects sensors and updates the display.
    
    Runs as a long-lived service, updating the display at regular intervals.
    """
    
    def __init__(self, config: Config):
        self.config = config
        self.running = False
        self.device: Optional[AX206Device] = None
        self.collector = SensorCollector(config)
        self.renderer = DashboardRenderer(config)
    
    def start(self) -> None:
        """Start the display daemon."""
        logger.info("Starting AX206 sensor display daemon")
        logger.info(f"Update interval: {self.config.interval}s")
        logger.info(f"Theme: {self.config.theme}")
        
        # Validate config
        warnings = self.config.validate()
        for warning in warnings:
            logger.warning(warning)
        
        self.running = True
        
        # Main loop with reconnection logic
        while self.running:
            try:
                self._run_display_loop()
            except DeviceNotFoundError as e:
                logger.error(f"Device not found: {e}")
                logger.info(f"Retrying in {self.config.retry_delay}s...")
                time.sleep(self.config.retry_delay)
            except DeviceCommunicationError as e:
                logger.error(f"Communication error: {e}")
                self._close_device()
                logger.info(f"Reconnecting in {self.config.retry_delay}s...")
                time.sleep(self.config.retry_delay)
            except KeyboardInterrupt:
                logger.info("Interrupted by user")
                break
            except Exception as e:
                logger.exception(f"Unexpected error: {e}")
                self._close_device()
                time.sleep(self.config.retry_delay)
        
        self._close_device()
        logger.info("Daemon stopped")
    
    def stop(self) -> None:
        """Stop the display daemon."""
        logger.info("Stopping daemon...")
        self.running = False
    
    def _run_display_loop(self) -> None:
        """Main display update loop."""
        # Open device
        self.device = AX206Device(self.config)
        self.device.open()
        
        # Update renderer dimensions if device reports different size
        if self.device.info:
            self.renderer.width = self.device.width
            self.renderer.height = self.device.height
            logger.info(f"Display resolution: {self.device.width}x{self.device.height}")
        
        # Display loop
        update_count = 0
        while self.running:
            start_time = time.time()
            
            try:
                # Collect sensor data
                data = self.collector.collect()
                
                # Render dashboard
                image = self.renderer.render(data)
                
                # Send to display
                self.device.display_image(image)
                
                update_count += 1
                if update_count % 100 == 0:
                    logger.debug(f"Completed {update_count} updates")
                
            except DeviceCommunicationError:
                # Re-raise to trigger reconnection
                raise
            except Exception as e:
                logger.error(f"Error during update: {e}")
            
            # Sleep for remaining interval time
            elapsed = time.time() - start_time
            sleep_time = max(0, self.config.interval - elapsed)
            if sleep_time > 0:
                time.sleep(sleep_time)
    
    def _close_device(self) -> None:
        """Close the device connection."""
        if self.device:
            try:
                self.device.close()
            except Exception:
                pass
            self.device = None


def run_test(config: Config) -> int:
    """Run a test pattern on the display."""
    logger.info("Running display test...")
    
    try:
        device = AX206Device(config)
        device.open()
        
        logger.info(f"Device: {device.width}x{device.height}")
        
        # Show test pattern
        logger.info("Displaying test pattern...")
        device.test_pattern()
        
        logger.info("Test complete! Check your display.")
        logger.info("Press Ctrl+C to exit or wait 10 seconds...")
        
        time.sleep(10)
        device.close()
        return 0
        
    except DeviceNotFoundError as e:
        logger.error(f"Device not found: {e}")
        return 1
    except DeviceCommunicationError as e:
        logger.error(f"Communication error: {e}")
        return 1
    except KeyboardInterrupt:
        logger.info("Test interrupted")
        return 0
    except Exception as e:
        logger.exception(f"Test failed: {e}")
        return 1


def run_once(config: Config) -> int:
    """Run a single update cycle (for testing/debugging)."""
    logger.info("Running single update...")
    
    try:
        device = AX206Device(config)
        device.open()
        
        collector = SensorCollector(config)
        renderer = DashboardRenderer(config)
        renderer.width = device.width
        renderer.height = device.height
        
        # Collect and render
        data = collector.collect()
        image = renderer.render(data)
        
        # Display
        device.display_image(image)
        
        logger.info("Update complete!")
        device.close()
        return 0
        
    except Exception as e:
        logger.exception(f"Update failed: {e}")
        return 1


def list_devices() -> int:
    """List connected AX206 devices."""
    devices = AX206Device.find_devices()
    
    if not devices:
        print("No AX206 devices found.")
        print("\nMake sure:")
        print("  1. Device is connected via USB")
        print("  2. udev rules are installed (see README)")
        print("  3. You have permission to access USB devices")
        return 1
    
    print(f"Found {len(devices)} AX206 device(s):\n")
    
    for i, dev in enumerate(devices):
        try:
            print(f"  [{i}] Bus {dev.bus:03d} Device {dev.address:03d}")
            print(f"      Vendor:  {dev.idVendor:04x}")
            print(f"      Product: {dev.idProduct:04x}")
            if dev.manufacturer:
                print(f"      Manufacturer: {dev.manufacturer}")
            if dev.product:
                print(f"      Product: {dev.product}")
            if dev.serial_number:
                print(f"      Serial: {dev.serial_number}")
            print()
        except Exception as e:
            print(f"  [{i}] (error reading details: {e})")
    
    return 0


def render_preview(config: Config, output_path: str) -> int:
    """Render a preview image without sending to device."""
    logger.info(f"Rendering preview to {output_path}...")
    
    try:
        collector = SensorCollector(config)
        renderer = DashboardRenderer(config)
        
        # Collect sensor data
        data = collector.collect()
        
        # Wait a bit for CPU load calculation (needs two samples)
        time.sleep(0.5)
        data = collector.collect()
        
        # Render
        image = renderer.render(data)
        
        # Save
        image.save(output_path)
        logger.info(f"Preview saved to {output_path}")
        return 0
        
    except Exception as e:
        logger.exception(f"Preview failed: {e}")
        return 1


def main() -> int:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="AX206 USB Sensor Display for NixOS",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  axdisplay                    Run the display daemon
  axdisplay --test             Display test pattern
  axdisplay --once             Single update (for debugging)
  axdisplay --list             List connected devices
  axdisplay --preview out.png  Render preview without device

Environment variables:
  AXDISPLAY_INTERVAL    Update interval in seconds (default: 2)
  AXDISPLAY_THEME       Theme name: dark, light (default: dark)
  AXDISPLAY_GPU_METHOD  GPU monitoring: nvidia, amd, auto, none
        """
    )
    
    # Mode options
    mode_group = parser.add_mutually_exclusive_group()
    mode_group.add_argument("--test", action="store_true", help="Display test pattern")
    mode_group.add_argument("--once", action="store_true", help="Run single update then exit")
    mode_group.add_argument("--list", action="store_true", help="List connected devices")
    mode_group.add_argument("--preview", metavar="FILE", help="Render preview image to file")
    
    # Configuration options
    parser.add_argument("-i", "--interval", type=float, help="Update interval in seconds")
    parser.add_argument("-t", "--theme", choices=["dark", "light"], help="Dashboard theme")
    parser.add_argument("-r", "--rotation", type=int, choices=[0, 90, 180, 270], help="Display rotation")
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable debug logging")
    parser.add_argument("-q", "--quiet", action="store_true", help="Suppress info logging")
    
    args = parser.parse_args()
    
    # Configure logging level
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    elif args.quiet:
        logging.getLogger().setLevel(logging.WARNING)
    
    # Load config from environment and override with CLI args
    config = Config.from_env()
    
    if args.interval is not None:
        config.interval = args.interval
    if args.theme is not None:
        config.theme = args.theme
    if args.rotation is not None:
        config.rotation = args.rotation
    
    # Handle modes
    if args.list:
        return list_devices()
    
    if args.test:
        return run_test(config)
    
    if args.once:
        return run_once(config)
    
    if args.preview:
        return render_preview(config, args.preview)
    
    # Default: run daemon
    daemon = DisplayDaemon(config)
    
    # Setup signal handlers
    def signal_handler(signum, frame):
        daemon.stop()
    
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)
    
    daemon.start()
    return 0


if __name__ == "__main__":
    sys.exit(main())
