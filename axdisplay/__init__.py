"""
AX206 USB Sensor Display for NixOS

A pure Python implementation for driving AX206-based USB LCD displays
with real-time system monitoring dashboards.
"""

__version__ = "1.0.0"
__author__ = "Generated for NixOS"

from .device import AX206Device
from .sensors import SensorCollector
from .renderer import DashboardRenderer
from .config import Config

__all__ = ["AX206Device", "SensorCollector", "DashboardRenderer", "Config"]
