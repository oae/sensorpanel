"""
Dashboard Renderer

Renders system metrics as a visual dashboard image using Pillow.
Designed for 320x240 displays but adaptable to other resolutions.
"""

import logging
from typing import Optional, Tuple, List
from dataclasses import dataclass

try:
    from PIL import Image, ImageDraw, ImageFont
except ImportError:
    raise ImportError("Pillow is required. Install with: pip install Pillow")

from .config import Config
from .sensors import SensorData, format_bytes_per_sec
from .themes import get_theme

logger = logging.getLogger(__name__)


@dataclass
class Rect:
    """Simple rectangle helper."""
    x: int
    y: int
    width: int
    height: int
    
    @property
    def x2(self) -> int:
        return self.x + self.width
    
    @property
    def y2(self) -> int:
        return self.y + self.height
    
    @property
    def center_x(self) -> int:
        return self.x + self.width // 2
    
    @property
    def center_y(self) -> int:
        return self.y + self.height // 2
    
    def inset(self, amount: int) -> "Rect":
        return Rect(
            self.x + amount,
            self.y + amount,
            self.width - 2 * amount,
            self.height - 2 * amount
        )
    
    def tuple(self) -> Tuple[int, int, int, int]:
        return (self.x, self.y, self.x2, self.y2)


class DashboardRenderer:
    """
    Renders sensor data as a dashboard image.
    
    Usage:
        renderer = DashboardRenderer(config)
        image = renderer.render(sensor_data)
    """
    
    def __init__(self, config: Optional[Config] = None):
        self.config = config or Config()
        self.width = config.width if config else 320
        self.height = config.height if config else 240
        
        # Get theme
        self.theme = get_theme(self.config.theme)()
        
        # Load fonts
        self._load_fonts()
    
    def _load_fonts(self) -> None:
        """Load fonts for rendering."""
        # Use Any type to avoid strict type checking with PIL's complex font types
        self.font_large = None
        self.font_medium = None
        self.font_small = None
        
        # Try to load system fonts
        font_paths = [
            "/run/current-system/sw/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
            "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
            "/usr/share/fonts/TTF/DejaVuSansMono.ttf",
            "/nix/store/*/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
        ]
        
        font_path = None
        for path in font_paths:
            if "*" in path:
                import glob
                matches = glob.glob(path)
                if matches:
                    font_path = matches[0]
                    break
            else:
                import os
                if os.path.exists(path):
                    font_path = path
                    break
        
        if font_path:
            try:
                self.font_large = ImageFont.truetype(font_path, self.theme.font_large)
                self.font_medium = ImageFont.truetype(font_path, self.theme.font_medium)
                self.font_small = ImageFont.truetype(font_path, self.theme.font_small)
                logger.debug(f"Loaded font from {font_path}")
            except Exception as e:
                logger.warning(f"Failed to load font: {e}")
        
        # Fallback to default font
        if self.font_large is None:
            self.font_large = ImageFont.load_default()
            self.font_medium = self.font_large
            self.font_small = self.font_large
            logger.debug("Using default bitmap font")
    
    def render(self, data: SensorData) -> Image.Image:
        """
        Render sensor data as a dashboard image.
        
        Args:
            data: Collected sensor data
        
        Returns:
            PIL Image object
        """
        # Create image
        img = Image.new("RGB", (self.width, self.height), self.theme.background)
        draw = ImageDraw.Draw(img)
        
        # Calculate layout
        padding = self.theme.padding
        
        # Determine which sections to show
        sections = []
        if self.config.show_cpu:
            sections.append(("cpu", self._render_cpu))
        if self.config.show_gpu and data.gpu.available:
            sections.append(("gpu", self._render_gpu))
        if self.config.show_ram:
            sections.append(("ram", self._render_ram))
        if self.config.show_disk and data.disks:
            sections.append(("disk", self._render_disk))
        if self.config.show_network and data.networks:
            sections.append(("network", self._render_network))
        
        if not sections:
            # No data to show
            draw.text(
                (self.width // 2, self.height // 2),
                "No data",
                fill=self.theme.text_secondary,
                font=self.font_medium,
                anchor="mm"
            )
            return img
        
        # Calculate section heights
        available_height = self.height - padding * 2
        section_height = available_height // len(sections)
        
        # Render each section
        y = padding
        for name, render_func in sections:
            rect = Rect(padding, y, self.width - padding * 2, section_height - 2)
            render_func(draw, rect, data)
            y += section_height
        
        return img
    
    def _draw_section_bg(self, draw: ImageDraw.Draw, rect: Rect) -> None:
        """Draw section background with rounded corners."""
        # Simple rectangle (rounded corners require more complex drawing)
        draw.rectangle(rect.tuple(), fill=self.theme.section_bg, outline=self.theme.section_border)
    
    def _draw_bar(
        self, 
        draw: ImageDraw.Draw, 
        rect: Rect, 
        percent: float,
        color: Optional[Tuple[int, int, int]] = None
    ) -> None:
        """Draw a progress bar."""
        # Background
        draw.rectangle(rect.tuple(), fill=self.theme.bar_bg)
        
        # Fill
        if percent > 0:
            fill_width = int(rect.width * min(percent, 100) / 100)
            if fill_width > 0:
                fill_color = color or self.theme.get_bar_color(percent)
                fill_rect = Rect(rect.x, rect.y, fill_width, rect.height)
                draw.rectangle(fill_rect.tuple(), fill=fill_color)
    
    def _render_cpu(self, draw: ImageDraw.Draw, rect: Rect, data: SensorData) -> None:
        """Render CPU section."""
        self._draw_section_bg(draw, rect)
        inner = rect.inset(self.theme.section_padding)
        
        # Title row
        draw.text(
            (inner.x, inner.y),
            "CPU",
            fill=self.theme.cpu_color,
            font=self.font_medium
        )
        
        # Temperature (if available)
        if data.cpu.temperature is not None:
            temp_text = f"{data.cpu.temperature:.0f}C"
            temp_color = self.theme.get_temp_color(data.cpu.temperature)
            draw.text(
                (inner.x2 - 40, inner.y),
                temp_text,
                fill=temp_color,
                font=self.font_medium
            )
        
        # Load percentage
        load_text = f"{data.cpu.load_percent:.0f}%"
        text_y = inner.y + 16
        draw.text(
            (inner.x, text_y),
            load_text,
            fill=self.theme.text_primary,
            font=self.font_large
        )
        
        # Frequency (if available)
        if data.cpu.frequency_mhz is not None:
            freq_text = f"{data.cpu.frequency_mhz:.0f}MHz"
            draw.text(
                (inner.x + 60, text_y + 2),
                freq_text,
                fill=self.theme.text_secondary,
                font=self.font_small
            )
        
        # Progress bar
        bar_y = inner.y + inner.height - self.theme.bar_height - 2
        bar_rect = Rect(inner.x, bar_y, inner.width, self.theme.bar_height)
        self._draw_bar(draw, bar_rect, data.cpu.load_percent, self.theme.cpu_color)
    
    def _render_gpu(self, draw: ImageDraw.Draw, rect: Rect, data: SensorData) -> None:
        """Render GPU section."""
        self._draw_section_bg(draw, rect)
        inner = rect.inset(self.theme.section_padding)
        
        # Title row
        draw.text(
            (inner.x, inner.y),
            "GPU",
            fill=self.theme.gpu_color,
            font=self.font_medium
        )
        
        # Temperature (if available)
        if data.gpu.temperature is not None:
            temp_text = f"{data.gpu.temperature:.0f}C"
            temp_color = self.theme.get_temp_color(data.gpu.temperature)
            draw.text(
                (inner.x2 - 40, inner.y),
                temp_text,
                fill=temp_color,
                font=self.font_medium
            )
        
        # Load percentage
        load_percent = data.gpu.load_percent or 0
        load_text = f"{load_percent:.0f}%"
        text_y = inner.y + 16
        draw.text(
            (inner.x, text_y),
            load_text,
            fill=self.theme.text_primary,
            font=self.font_large
        )
        
        # Memory usage
        if data.gpu.memory_used_mb is not None and data.gpu.memory_total_mb is not None:
            mem_text = f"{data.gpu.memory_used_mb:.0f}/{data.gpu.memory_total_mb:.0f}MB"
            draw.text(
                (inner.x + 50, text_y + 2),
                mem_text,
                fill=self.theme.text_secondary,
                font=self.font_small
            )
        
        # Progress bar
        bar_y = inner.y + inner.height - self.theme.bar_height - 2
        bar_rect = Rect(inner.x, bar_y, inner.width, self.theme.bar_height)
        self._draw_bar(draw, bar_rect, load_percent, self.theme.gpu_color)
    
    def _render_ram(self, draw: ImageDraw.Draw, rect: Rect, data: SensorData) -> None:
        """Render RAM section."""
        self._draw_section_bg(draw, rect)
        inner = rect.inset(self.theme.section_padding)
        
        # Title
        draw.text(
            (inner.x, inner.y),
            "RAM",
            fill=self.theme.ram_color,
            font=self.font_medium
        )
        
        # Usage percentage
        draw.text(
            (inner.x2 - 45, inner.y),
            f"{data.memory.percent:.0f}%",
            fill=self.theme.text_primary,
            font=self.font_medium
        )
        
        # Used / Total
        text_y = inner.y + 16
        used_gb = data.memory.used_mb / 1024
        total_gb = data.memory.total_mb / 1024
        mem_text = f"{used_gb:.1f} / {total_gb:.1f} GB"
        draw.text(
            (inner.x, text_y),
            mem_text,
            fill=self.theme.text_primary,
            font=self.font_medium
        )
        
        # Progress bar
        bar_y = inner.y + inner.height - self.theme.bar_height - 2
        bar_rect = Rect(inner.x, bar_y, inner.width, self.theme.bar_height)
        self._draw_bar(draw, bar_rect, data.memory.percent, self.theme.ram_color)
    
    def _render_disk(self, draw: ImageDraw.Draw, rect: Rect, data: SensorData) -> None:
        """Render disk section."""
        self._draw_section_bg(draw, rect)
        inner = rect.inset(self.theme.section_padding)
        
        # Show first disk only (for space)
        disk = data.disks[0] if data.disks else None
        if not disk:
            return
        
        # Title with mount point
        label = disk.mount_point if len(disk.mount_point) <= 10 else disk.mount_point[:10]
        draw.text(
            (inner.x, inner.y),
            f"DISK {label}",
            fill=self.theme.disk_color,
            font=self.font_medium
        )
        
        # Usage percentage
        draw.text(
            (inner.x2 - 45, inner.y),
            f"{disk.percent:.0f}%",
            fill=self.theme.text_primary,
            font=self.font_medium
        )
        
        # Used / Total
        text_y = inner.y + 16
        disk_text = f"{disk.used_gb:.1f} / {disk.total_gb:.1f} GB"
        draw.text(
            (inner.x, text_y),
            disk_text,
            fill=self.theme.text_primary,
            font=self.font_medium
        )
        
        # Progress bar
        bar_y = inner.y + inner.height - self.theme.bar_height - 2
        bar_rect = Rect(inner.x, bar_y, inner.width, self.theme.bar_height)
        self._draw_bar(draw, bar_rect, disk.percent, self.theme.disk_color)
    
    def _render_network(self, draw: ImageDraw.Draw, rect: Rect, data: SensorData) -> None:
        """Render network section."""
        self._draw_section_bg(draw, rect)
        inner = rect.inset(self.theme.section_padding)
        
        # Show first network interface
        net = data.networks[0] if data.networks else None
        if not net:
            return
        
        # Title with interface name
        iface_label = net.interface[:8] if len(net.interface) > 8 else net.interface
        draw.text(
            (inner.x, inner.y),
            f"NET {iface_label}",
            fill=self.theme.network_rx_color,
            font=self.font_medium
        )
        
        # Download (RX)
        text_y = inner.y + 16
        rx_text = f"D: {format_bytes_per_sec(net.rx_bytes_per_sec)}"
        draw.text(
            (inner.x, text_y),
            rx_text,
            fill=self.theme.network_rx_color,
            font=self.font_small
        )
        
        # Upload (TX)
        tx_text = f"U: {format_bytes_per_sec(net.tx_bytes_per_sec)}"
        draw.text(
            (inner.x + inner.width // 2, text_y),
            tx_text,
            fill=self.theme.network_tx_color,
            font=self.font_small
        )
    
    def render_test_pattern(self) -> Image.Image:
        """Render a test pattern for display verification."""
        img = Image.new("RGB", (self.width, self.height), (0, 0, 0))
        draw = ImageDraw.Draw(img)
        
        # Color bars
        colors = [
            (255, 255, 255),  # White
            (255, 255, 0),    # Yellow
            (0, 255, 255),    # Cyan
            (0, 255, 0),      # Green
            (255, 0, 255),    # Magenta
            (255, 0, 0),      # Red
            (0, 0, 255),      # Blue
        ]
        
        bar_height = self.height // len(colors)
        for i, color in enumerate(colors):
            y0 = i * bar_height
            y1 = (i + 1) * bar_height
            draw.rectangle([0, y0, self.width, y1], fill=color)
        
        # Resolution text
        text = f"{self.width}x{self.height}"
        draw.text((10, 10), text, fill=(0, 0, 0), font=self.font_medium)
        draw.text((10, self.height - 25), "AX206 Test", fill=(255, 255, 255), font=self.font_medium)
        
        return img
