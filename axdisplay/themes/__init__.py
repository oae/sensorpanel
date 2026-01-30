"""
Theme definitions for dashboard rendering.
"""

from .dark import DarkTheme
from .light import LightTheme

THEMES = {
    "dark": DarkTheme,
    "light": LightTheme,
}

def get_theme(name: str):
    """Get theme class by name."""
    return THEMES.get(name, DarkTheme)
