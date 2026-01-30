"""
Configuration for AX206 Sensor Display

All user-configurable options are defined here.
Settings can be overridden via environment variables.
"""

import os
from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class Config:
    """Main configuration class for axdisplay."""

    # ==========================================================================
    # DISPLAY SETTINGS
    # ==========================================================================

    # Display resolution (QTKeJi USB-Display default)
    width: int = 480
    height: int = 320

    # Display rotation: 0, 90, 180, 270 degrees
    rotation: int = 0

    # Update interval in seconds
    interval: float = 2.0

    # ==========================================================================
    # USB DEVICE SETTINGS
    # ==========================================================================

    # USB Vendor and Product ID for AX206 frames
    vendor_id: int = 0x1908
    product_id: int = 0x0102

    # USB endpoints (standard for AX206)
    endpoint_out: int = 0x01
    endpoint_in: int = 0x81

    # USB timeout in milliseconds
    usb_timeout: int = 1000

    # Retry settings for device connection
    max_retries: int = 5
    retry_delay: float = 2.0

    # ==========================================================================
    # THEME SETTINGS
    # ==========================================================================

    # Theme: "dark" or "light"
    theme: str = "dark"

    # Font settings
    font_name: str = "DejaVuSansMono"
    font_size_large: int = 16
    font_size_medium: int = 12
    font_size_small: int = 10

    # ==========================================================================
    # METRICS SETTINGS
    # ==========================================================================

    # Enable/disable specific metrics
    show_cpu: bool = True
    show_gpu: bool = True
    show_ram: bool = True
    show_disk: bool = True
    show_network: bool = True

    # Disk mount points to monitor
    disk_mounts: List[str] = field(default_factory=lambda: ["/"])

    # Network interface pattern (glob-style, e.g., "enp*", "eth*", "*")
    network_interface: str = "*"

    # GPU monitoring method: "nvidia" (nvidia-smi), "amd" (rocm-smi), "auto", "none"
    gpu_method: str = "nvidia"

    # ==========================================================================
    # PATHS
    # ==========================================================================

    # Runtime directory for temporary files
    runtime_dir: str = "/tmp/axdisplay"

    # Path to nvidia-smi (auto-detected if None)
    nvidia_smi_path: Optional[str] = None

    @classmethod
    def from_env(cls) -> "Config":
        """Create config from environment variables."""
        config = cls()

        # Display settings
        if val := os.getenv("AXDISPLAY_WIDTH"):
            config.width = int(val)
        if val := os.getenv("AXDISPLAY_HEIGHT"):
            config.height = int(val)
        if val := os.getenv("AXDISPLAY_ROTATION"):
            config.rotation = int(val)
        if val := os.getenv("AXDISPLAY_INTERVAL"):
            config.interval = float(val)

        # Theme
        if val := os.getenv("AXDISPLAY_THEME"):
            config.theme = val.lower()

        # Metrics
        if val := os.getenv("AXDISPLAY_SHOW_CPU"):
            config.show_cpu = val.lower() in ("1", "true", "yes")
        if val := os.getenv("AXDISPLAY_SHOW_GPU"):
            config.show_gpu = val.lower() in ("1", "true", "yes")
        if val := os.getenv("AXDISPLAY_SHOW_RAM"):
            config.show_ram = val.lower() in ("1", "true", "yes")
        if val := os.getenv("AXDISPLAY_SHOW_DISK"):
            config.show_disk = val.lower() in ("1", "true", "yes")
        if val := os.getenv("AXDISPLAY_SHOW_NETWORK"):
            config.show_network = val.lower() in ("1", "true", "yes")

        # Disk mounts
        if val := os.getenv("AXDISPLAY_DISK_MOUNTS"):
            config.disk_mounts = [m.strip() for m in val.split(",")]

        # Network interface
        if val := os.getenv("AXDISPLAY_NETWORK_IF"):
            config.network_interface = val

        # GPU method
        if val := os.getenv("AXDISPLAY_GPU_METHOD"):
            config.gpu_method = val.lower()

        # Paths
        if val := os.getenv("AXDISPLAY_RUNTIME_DIR"):
            config.runtime_dir = val
        if val := os.getenv("AXDISPLAY_NVIDIA_SMI"):
            config.nvidia_smi_path = val

        return config

    def validate(self) -> List[str]:
        """Validate configuration and return list of warnings."""
        warnings = []

        if self.rotation not in (0, 90, 180, 270):
            warnings.append(
                f"Invalid rotation {self.rotation}, must be 0, 90, 180, or 270"
            )

        if self.interval < 0.5:
            warnings.append(
                f"Interval {self.interval}s is very low, may cause high CPU usage"
            )

        if self.theme not in ("dark", "light"):
            warnings.append(f"Unknown theme '{self.theme}', falling back to 'dark'")
            self.theme = "dark"

        if self.gpu_method not in ("nvidia", "amd", "auto", "none"):
            warnings.append(
                f"Unknown GPU method '{self.gpu_method}', falling back to 'auto'"
            )
            self.gpu_method = "auto"

        return warnings
