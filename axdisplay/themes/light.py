"""
Light theme for sensor dashboard.
Clean, bright aesthetic suitable for well-lit environments.
"""

from dataclasses import dataclass


@dataclass(frozen=True)
class LightTheme:
    """Light theme color palette and settings."""
    
    # Main colors
    background: tuple = (240, 242, 245)      # Light gray
    foreground: tuple = (30, 30, 35)         # Near black
    
    # Section backgrounds
    section_bg: tuple = (255, 255, 255)      # White
    section_border: tuple = (200, 200, 210)  # Light gray border
    
    # Text colors
    text_primary: tuple = (30, 30, 40)       # Dark
    text_secondary: tuple = (80, 80, 100)    # Medium gray
    text_label: tuple = (120, 120, 140)      # Light gray
    
    # Bar colors
    bar_bg: tuple = (220, 220, 230)          # Light gray
    bar_low: tuple = (40, 160, 80)           # Green
    bar_medium: tuple = (220, 160, 40)       # Yellow/Orange
    bar_high: tuple = (220, 60, 50)          # Red
    
    # Metric-specific colors
    cpu_color: tuple = (60, 140, 230)        # Blue
    gpu_color: tuple = (80, 180, 60)         # Green
    ram_color: tuple = (160, 90, 220)        # Purple
    disk_color: tuple = (230, 150, 50)       # Orange
    network_rx_color: tuple = (50, 170, 170) # Cyan
    network_tx_color: tuple = (220, 90, 140) # Pink
    
    # Temperature colors
    temp_cold: tuple = (60, 150, 230)        # Cold blue
    temp_warm: tuple = (230, 170, 50)        # Warm yellow
    temp_hot: tuple = (220, 60, 50)          # Hot red
    
    # Layout (same as dark theme)
    padding: int = 5
    section_padding: int = 4
    bar_height: int = 10
    bar_radius: int = 3
    section_radius: int = 5
    
    # Font sizes
    font_large: int = 14
    font_medium: int = 11
    font_small: int = 9
    
    @staticmethod
    def get_bar_color(percent: float) -> tuple:
        """Get bar fill color based on percentage."""
        if percent < 50:
            return LightTheme.bar_low
        elif percent < 80:
            return LightTheme.bar_medium
        else:
            return LightTheme.bar_high
    
    @staticmethod
    def get_temp_color(temp: float) -> tuple:
        """Get temperature color based on value."""
        if temp < 50:
            return LightTheme.temp_cold
        elif temp < 75:
            return LightTheme.temp_warm
        else:
            return LightTheme.temp_hot
