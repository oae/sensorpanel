"""
System Sensor Collection Module

Collects system metrics from various sources:
- CPU: temperature, load, frequency
- GPU: temperature, load, memory (NVIDIA via nvidia-smi)
- RAM: usage statistics
- Disk: usage per mount point
- Network: throughput per interface
"""

import os
import re
import glob
import time
import shutil
import logging
import subprocess
from typing import Dict, List, Optional, Any, Tuple
from dataclasses import dataclass, field
from pathlib import Path

from .config import Config

logger = logging.getLogger(__name__)


@dataclass
class CPUStats:
    """CPU statistics."""
    temperature: Optional[float] = None  # Celsius
    load_percent: float = 0.0            # 0-100
    frequency_mhz: Optional[float] = None
    core_count: int = 1


@dataclass 
class GPUStats:
    """GPU statistics (NVIDIA or AMD)."""
    name: str = "Unknown"
    temperature: Optional[float] = None  # Celsius
    load_percent: Optional[float] = None # 0-100
    memory_used_mb: Optional[float] = None
    memory_total_mb: Optional[float] = None
    power_watts: Optional[float] = None
    available: bool = False


@dataclass
class MemoryStats:
    """Memory statistics."""
    total_mb: float = 0.0
    used_mb: float = 0.0
    available_mb: float = 0.0
    percent: float = 0.0


@dataclass
class DiskStats:
    """Disk usage for a single mount point."""
    mount_point: str = "/"
    total_gb: float = 0.0
    used_gb: float = 0.0
    free_gb: float = 0.0
    percent: float = 0.0


@dataclass
class NetworkStats:
    """Network throughput statistics."""
    interface: str = ""
    rx_bytes_per_sec: float = 0.0
    tx_bytes_per_sec: float = 0.0
    rx_total_bytes: int = 0
    tx_total_bytes: int = 0


@dataclass
class SensorData:
    """Container for all sensor readings."""
    cpu: CPUStats = field(default_factory=CPUStats)
    gpu: GPUStats = field(default_factory=GPUStats)
    memory: MemoryStats = field(default_factory=MemoryStats)
    disks: List[DiskStats] = field(default_factory=list)
    networks: List[NetworkStats] = field(default_factory=list)
    timestamp: float = 0.0


class SensorCollector:
    """
    Collects system sensor data from various sources.
    
    Usage:
        collector = SensorCollector(config)
        data = collector.collect()
        print(f"CPU: {data.cpu.load_percent}%")
    """
    
    def __init__(self, config: Optional[Config] = None):
        self.config = config or Config()
        
        # Cache for CPU load calculation (need two samples)
        self._prev_cpu_times: Optional[Tuple[int, int]] = None
        self._prev_cpu_time: float = 0.0
        
        # Cache for network throughput calculation
        self._prev_net_stats: Dict[str, Tuple[int, int, float]] = {}
        
        # nvidia-smi path
        self._nvidia_smi: Optional[str] = None
        if self.config.gpu_method in ("nvidia", "auto"):
            self._nvidia_smi = self._find_nvidia_smi()
        
        logger.debug(f"SensorCollector initialized, nvidia-smi: {self._nvidia_smi}")
    
    def _find_nvidia_smi(self) -> Optional[str]:
        """Find nvidia-smi executable."""
        if self.config.nvidia_smi_path:
            if os.path.isfile(self.config.nvidia_smi_path):
                return self.config.nvidia_smi_path
        
        # Try common locations
        paths = [
            "/run/current-system/sw/bin/nvidia-smi",  # NixOS
            "/usr/bin/nvidia-smi",
            "/usr/local/bin/nvidia-smi",
        ]
        
        for path in paths:
            if os.path.isfile(path):
                return path
        
        # Try PATH
        return shutil.which("nvidia-smi")
    
    def collect(self) -> SensorData:
        """Collect all sensor data."""
        data = SensorData(timestamp=time.time())
        
        if self.config.show_cpu:
            data.cpu = self._collect_cpu()
        
        if self.config.show_gpu:
            data.gpu = self._collect_gpu()
        
        if self.config.show_ram:
            data.memory = self._collect_memory()
        
        if self.config.show_disk:
            data.disks = self._collect_disks()
        
        if self.config.show_network:
            data.networks = self._collect_network()
        
        return data
    
    def _collect_cpu(self) -> CPUStats:
        """Collect CPU statistics."""
        stats = CPUStats()
        
        # Core count
        try:
            stats.core_count = os.cpu_count() or 1
        except Exception:
            stats.core_count = 1
        
        # CPU load from /proc/stat
        try:
            stats.load_percent = self._calculate_cpu_load()
        except Exception as e:
            logger.debug(f"Failed to get CPU load: {e}")
        
        # CPU temperature from hwmon
        try:
            stats.temperature = self._get_cpu_temperature()
        except Exception as e:
            logger.debug(f"Failed to get CPU temp: {e}")
        
        # CPU frequency
        try:
            stats.frequency_mhz = self._get_cpu_frequency()
        except Exception as e:
            logger.debug(f"Failed to get CPU freq: {e}")
        
        return stats
    
    def _calculate_cpu_load(self) -> float:
        """Calculate CPU load percentage from /proc/stat."""
        with open("/proc/stat", "r") as f:
            line = f.readline()
        
        # cpu  user nice system idle iowait irq softirq steal guest guest_nice
        parts = line.split()
        if parts[0] != "cpu":
            return 0.0
        
        values = [int(x) for x in parts[1:]]
        idle = values[3] + values[4]  # idle + iowait
        total = sum(values)
        
        now = time.time()
        
        if self._prev_cpu_times is None:
            # First sample, return 0
            self._prev_cpu_times = (idle, total)
            self._prev_cpu_time = now
            return 0.0
        
        prev_idle, prev_total = self._prev_cpu_times
        
        # Calculate deltas
        idle_delta = idle - prev_idle
        total_delta = total - prev_total
        
        # Update cache
        self._prev_cpu_times = (idle, total)
        self._prev_cpu_time = now
        
        if total_delta == 0:
            return 0.0
        
        # CPU usage = (total - idle) / total * 100
        load = (1.0 - idle_delta / total_delta) * 100.0
        return max(0.0, min(100.0, load))
    
    def _get_cpu_temperature(self) -> Optional[float]:
        """Get CPU temperature from hwmon sysfs."""
        # Try coretemp first (Intel)
        hwmon_paths = glob.glob("/sys/class/hwmon/hwmon*/name")
        
        for name_path in hwmon_paths:
            try:
                with open(name_path, "r") as f:
                    name = f.read().strip()
                
                hwmon_dir = os.path.dirname(name_path)
                
                # Look for CPU-related sensors
                if name in ("coretemp", "k10temp", "zenpower", "cpu_thermal", "acpitz"):
                    # Find temp1_input or similar
                    temp_files = glob.glob(os.path.join(hwmon_dir, "temp*_input"))
                    if temp_files:
                        with open(temp_files[0], "r") as f:
                            # Value is in millidegrees
                            temp = int(f.read().strip()) / 1000.0
                            return temp
            except Exception:
                continue
        
        # Fallback: try thermal zones
        thermal_paths = glob.glob("/sys/class/thermal/thermal_zone*/temp")
        for temp_path in thermal_paths:
            try:
                type_path = os.path.join(os.path.dirname(temp_path), "type")
                with open(type_path, "r") as f:
                    zone_type = f.read().strip().lower()
                
                if "cpu" in zone_type or "x86" in zone_type or "core" in zone_type:
                    with open(temp_path, "r") as f:
                        temp = int(f.read().strip()) / 1000.0
                        return temp
            except Exception:
                continue
        
        return None
    
    def _get_cpu_frequency(self) -> Optional[float]:
        """Get current CPU frequency in MHz."""
        try:
            # Try scaling_cur_freq first
            freq_files = glob.glob("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
            if freq_files:
                with open(freq_files[0], "r") as f:
                    # Value is in kHz
                    freq = int(f.read().strip()) / 1000.0
                    return freq
            
            # Fallback to /proc/cpuinfo
            with open("/proc/cpuinfo", "r") as f:
                for line in f:
                    if line.startswith("cpu MHz"):
                        return float(line.split(":")[1].strip())
        except Exception:
            pass
        
        return None
    
    def _collect_gpu(self) -> GPUStats:
        """Collect GPU statistics."""
        stats = GPUStats()
        
        if self._nvidia_smi:
            stats = self._collect_nvidia_gpu()
        elif self.config.gpu_method == "amd":
            stats = self._collect_amd_gpu()
        
        return stats
    
    def _collect_nvidia_gpu(self) -> GPUStats:
        """Collect NVIDIA GPU stats via nvidia-smi."""
        stats = GPUStats()
        
        if not self._nvidia_smi:
            return stats
        
        try:
            # Query multiple values at once
            cmd = [
                self._nvidia_smi,
                "--query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw",
                "--format=csv,noheader,nounits"
            ]
            
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=5
            )
            
            if result.returncode != 0:
                logger.debug(f"nvidia-smi failed: {result.stderr}")
                return stats
            
            # Parse CSV output
            line = result.stdout.strip().split("\n")[0]  # First GPU only
            parts = [p.strip() for p in line.split(",")]
            
            if len(parts) >= 5:
                stats.available = True
                stats.name = parts[0]
                
                try:
                    stats.temperature = float(parts[1])
                except (ValueError, IndexError):
                    pass
                
                try:
                    stats.load_percent = float(parts[2])
                except (ValueError, IndexError):
                    pass
                
                try:
                    stats.memory_used_mb = float(parts[3])
                except (ValueError, IndexError):
                    pass
                
                try:
                    stats.memory_total_mb = float(parts[4])
                except (ValueError, IndexError):
                    pass
                
                try:
                    if len(parts) > 5 and parts[5] != "[N/A]":
                        stats.power_watts = float(parts[5])
                except (ValueError, IndexError):
                    pass
        
        except subprocess.TimeoutExpired:
            logger.warning("nvidia-smi timed out")
        except Exception as e:
            logger.debug(f"Failed to collect NVIDIA stats: {e}")
        
        return stats
    
    def _collect_amd_gpu(self) -> GPUStats:
        """Collect AMD GPU stats from sysfs."""
        stats = GPUStats()
        
        # Look for AMD GPU in DRM
        drm_cards = glob.glob("/sys/class/drm/card*/device")
        
        for card_path in drm_cards:
            try:
                # Check if AMD
                vendor_path = os.path.join(card_path, "vendor")
                if os.path.exists(vendor_path):
                    with open(vendor_path, "r") as f:
                        vendor = f.read().strip()
                    if vendor != "0x1002":  # AMD vendor ID
                        continue
                
                stats.available = True
                stats.name = "AMD GPU"
                
                # Temperature
                hwmon_paths = glob.glob(os.path.join(card_path, "hwmon/hwmon*/temp1_input"))
                if hwmon_paths:
                    with open(hwmon_paths[0], "r") as f:
                        stats.temperature = int(f.read().strip()) / 1000.0
                
                # GPU busy percent
                busy_path = os.path.join(card_path, "gpu_busy_percent")
                if os.path.exists(busy_path):
                    with open(busy_path, "r") as f:
                        stats.load_percent = float(f.read().strip())
                
                # Memory info
                mem_used_path = os.path.join(card_path, "mem_info_vram_used")
                mem_total_path = os.path.join(card_path, "mem_info_vram_total")
                
                if os.path.exists(mem_used_path):
                    with open(mem_used_path, "r") as f:
                        stats.memory_used_mb = int(f.read().strip()) / (1024 * 1024)
                
                if os.path.exists(mem_total_path):
                    with open(mem_total_path, "r") as f:
                        stats.memory_total_mb = int(f.read().strip()) / (1024 * 1024)
                
                break  # Use first AMD GPU found
                
            except Exception as e:
                logger.debug(f"Failed to read AMD GPU info: {e}")
                continue
        
        return stats
    
    def _collect_memory(self) -> MemoryStats:
        """Collect memory statistics from /proc/meminfo."""
        stats = MemoryStats()
        
        try:
            meminfo = {}
            with open("/proc/meminfo", "r") as f:
                for line in f:
                    parts = line.split(":")
                    if len(parts) == 2:
                        key = parts[0].strip()
                        # Value is in kB
                        value = int(parts[1].strip().split()[0])
                        meminfo[key] = value
            
            total = meminfo.get("MemTotal", 0)
            available = meminfo.get("MemAvailable", 0)
            
            if available == 0:
                # Fallback for older kernels
                free = meminfo.get("MemFree", 0)
                buffers = meminfo.get("Buffers", 0)
                cached = meminfo.get("Cached", 0)
                available = free + buffers + cached
            
            stats.total_mb = total / 1024.0
            stats.available_mb = available / 1024.0
            stats.used_mb = stats.total_mb - stats.available_mb
            
            if stats.total_mb > 0:
                stats.percent = (stats.used_mb / stats.total_mb) * 100.0
            
        except Exception as e:
            logger.debug(f"Failed to collect memory stats: {e}")
        
        return stats
    
    def _collect_disks(self) -> List[DiskStats]:
        """Collect disk usage for configured mount points."""
        disks = []
        
        for mount_point in self.config.disk_mounts:
            try:
                stat = os.statvfs(mount_point)
                
                # Calculate sizes in GB
                block_size = stat.f_frsize
                total = (stat.f_blocks * block_size) / (1024 ** 3)
                free = (stat.f_bfree * block_size) / (1024 ** 3)
                available = (stat.f_bavail * block_size) / (1024 ** 3)
                used = total - free
                
                percent = (used / total * 100.0) if total > 0 else 0.0
                
                disks.append(DiskStats(
                    mount_point=mount_point,
                    total_gb=total,
                    used_gb=used,
                    free_gb=available,  # Use available (for non-root)
                    percent=percent
                ))
            except Exception as e:
                logger.debug(f"Failed to stat {mount_point}: {e}")
        
        return disks
    
    def _collect_network(self) -> List[NetworkStats]:
        """Collect network throughput statistics."""
        networks = []
        now = time.time()
        
        try:
            with open("/proc/net/dev", "r") as f:
                lines = f.readlines()[2:]  # Skip headers
            
            for line in lines:
                parts = line.split(":")
                if len(parts) != 2:
                    continue
                
                iface = parts[0].strip()
                
                # Skip loopback and virtual interfaces unless specifically requested
                if iface == "lo":
                    continue
                
                # Check if interface matches pattern
                pattern = self.config.network_interface
                if pattern != "*":
                    import fnmatch
                    if not fnmatch.fnmatch(iface, pattern):
                        continue
                
                # Parse values
                values = parts[1].split()
                rx_bytes = int(values[0])
                tx_bytes = int(values[8])
                
                # Calculate throughput
                rx_per_sec = 0.0
                tx_per_sec = 0.0
                
                if iface in self._prev_net_stats:
                    prev_rx, prev_tx, prev_time = self._prev_net_stats[iface]
                    dt = now - prev_time
                    if dt > 0:
                        rx_per_sec = (rx_bytes - prev_rx) / dt
                        tx_per_sec = (tx_bytes - prev_tx) / dt
                
                self._prev_net_stats[iface] = (rx_bytes, tx_bytes, now)
                
                networks.append(NetworkStats(
                    interface=iface,
                    rx_bytes_per_sec=max(0, rx_per_sec),
                    tx_bytes_per_sec=max(0, tx_per_sec),
                    rx_total_bytes=rx_bytes,
                    tx_total_bytes=tx_bytes
                ))
        
        except Exception as e:
            logger.debug(f"Failed to collect network stats: {e}")
        
        return networks


def format_bytes(bytes_val: float) -> str:
    """Format bytes value to human-readable string."""
    for unit in ["B", "KB", "MB", "GB", "TB"]:
        if bytes_val < 1024:
            return f"{bytes_val:.1f}{unit}"
        bytes_val /= 1024
    return f"{bytes_val:.1f}PB"


def format_bytes_per_sec(bps: float) -> str:
    """Format bytes per second to human-readable string."""
    return format_bytes(bps) + "/s"
