"""
Dark theme for sensor dashboard.
Hardware monitoring aesthetic with dark background and colorful accents.
"""

from dataclasses import dataclass


@dataclass(frozen=True)
class DarkTheme:
    """Dark theme color palette and settings."""
    
    # Main colors
    background: tuple = (15, 15, 20)         # Near black
    foreground: tuple = (220, 220, 220)      # Light gray
    
    # Section backgrounds
    section_bg: tuple = (30, 30, 40)         # Dark gray
    section_border: tuple = (60, 60, 80)     # Medium gray
    
    # Text colors
    text_primary: tuple = (255, 255, 255)    # White
    text_secondary: tuple = (160, 160, 170)  # Gray
    text_label: tuple = (120, 120, 140)      # Dim gray
    
    # Bar colors (gradient from cool to hot)
    bar_bg: tuple = (40, 40, 50)             # Dark
    bar_low: tuple = (50, 180, 100)          # Green
    bar_medium: tuple = (230, 180, 50)       # Yellow
    bar_high: tuple = (230, 80, 60)          # Red
    
    # Metric-specific colors
    cpu_color: tuple = (80, 160, 255)        # Blue
    gpu_color: tuple = (130, 230, 80)        # Green
    ram_color: tuple = (200, 120, 255)       # Purple
    disk_color: tuple = (255, 180, 80)       # Orange
    network_rx_color: tuple = (80, 200, 200) # Cyan
    network_tx_color: tuple = (255, 120, 160)# Pink
    
    # Temperature colors
    temp_cold: tuple = (80, 180, 255)        # Cold blue
    temp_warm: tuple = (255, 200, 80)        # Warm yellow
    temp_hot: tuple = (255, 80, 60)          # Hot red
    
    # Layout
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
            return DarkTheme.bar_low
        elif percent < 80:
            return DarkTheme.bar_medium
        else:
            return DarkTheme.bar_high
    
    @staticmethod
    def get_temp_color(temp: float) -> tuple:
        """Get temperature color based on value."""
        if temp < 50:
            return DarkTheme.temp_cold
        elif temp < 75:
            return DarkTheme.temp_warm
        else:
            return DarkTheme.temp_hot
