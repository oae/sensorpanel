// Package nativerender draws JSON-native themes without a headless browser.
package nativerender

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
)

// Theme is a lightweight native theme definition.
type Theme struct {
	Name               string              `json:"name,omitempty"`
	Layout             string              `json:"layout"`
	Width              int                 `json:"width,omitempty"`
	Height             int                 `json:"height,omitempty"`
	Background         string              `json:"background,omitempty"`
	BackgroundSequence *BackgroundSequence `json:"background_sequence,omitempty"`
	Accent             string              `json:"accent,omitempty"`
	Accent2            string              `json:"accent2,omitempty"`
	Accent3            string              `json:"accent3,omitempty"`
	Text               string              `json:"text,omitempty"`
	Muted              string              `json:"muted,omitempty"`
	Panel              string              `json:"panel,omitempty"`
	PanelLine          string              `json:"panel_line,omitempty"`
	Performance        *Performance        `json:"performance,omitempty"`
}

// Performance controls the native renderer's resource budget. Themes may
// override these values, while the command line can still supply a temporary
// target FPS override.
type Performance struct {
	Profile            string  `json:"profile,omitempty"` // power-saver, balanced, smooth
	TargetFPS          float64 `json:"target_fps,omitempty"`
	ActiveFPS          float64 `json:"active_fps,omitempty"`
	IdleFPS            float64 `json:"idle_fps,omitempty"`
	IdleTimeoutSeconds int     `json:"idle_timeout_seconds,omitempty"`
	JPEGQuality        int     `json:"jpeg_quality,omitempty"`
	PrefetchFrames     int     `json:"prefetch_frames,omitempty"`
	JPEGEncoder        string  `json:"jpeg_encoder,omitempty"` // auto, stdlib, turbo
}

// BackgroundSequence configures a pre-rendered animated background.
type BackgroundSequence struct {
	Path    string  `json:"path"`
	Pattern string  `json:"pattern,omitempty"`
	Frames  int     `json:"frames,omitempty"`
	FPS     float64 `json:"fps,omitempty"`
	Opacity float64 `json:"opacity,omitempty"`
	Cache   string  `json:"cache,omitempty"` // lru or memory
}

// Load reads a native.theme.json file.
func Load(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Theme
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse native theme: %w", err)
	}
	if t.Layout == "" {
		return nil, fmt.Errorf("native theme missing layout")
	}
	if t.Background == "" {
		t.Background = "#000000"
	}
	if t.Accent == "" {
		t.Accent = "#00d8ff"
	}
	if t.Accent2 == "" {
		t.Accent2 = "#ff4df3"
	}
	if t.Accent3 == "" {
		t.Accent3 = "#71ffa8"
	}
	if t.Text == "" {
		t.Text = "#f8fbff"
	}
	if t.Muted == "" {
		t.Muted = "#8ea1bb"
	}
	if t.Panel == "" {
		t.Panel = "#071427cc"
	}
	if t.PanelLine == "" {
		t.PanelLine = "#1d5cff88"
	}
	if t.BackgroundSequence != nil {
		if t.BackgroundSequence.Pattern == "" {
			t.BackgroundSequence.Pattern = "frame_%04d.jpg"
		}
		if t.BackgroundSequence.FPS <= 0 {
			t.BackgroundSequence.FPS = 24
		}
		if t.BackgroundSequence.Opacity <= 0 {
			t.BackgroundSequence.Opacity = 0.3
		}
		if t.BackgroundSequence.Cache == "" {
			t.BackgroundSequence.Cache = "lru"
		}
	}
	if t.Performance == nil {
		t.Performance = &Performance{}
	}
	if t.Performance.Profile == "" {
		t.Performance.Profile = "balanced"
	}
	switch t.Performance.Profile {
	case "power-saver":
		if t.Performance.TargetFPS <= 0 {
			t.Performance.TargetFPS = 6
		}
		if t.Performance.JPEGQuality <= 0 {
			t.Performance.JPEGQuality = 72
		}
		if t.Performance.PrefetchFrames <= 0 {
			t.Performance.PrefetchFrames = 6
		}
	case "smooth":
		if t.Performance.TargetFPS <= 0 {
			t.Performance.TargetFPS = 12
		}
		if t.Performance.JPEGQuality <= 0 {
			t.Performance.JPEGQuality = 80
		}
		if t.Performance.PrefetchFrames <= 0 {
			t.Performance.PrefetchFrames = 12
		}
	default:
		t.Performance.Profile = "balanced"
		if t.Performance.TargetFPS <= 0 {
			t.Performance.TargetFPS = 8
		}
		if t.Performance.JPEGQuality <= 0 {
			t.Performance.JPEGQuality = 78
		}
		if t.Performance.PrefetchFrames <= 0 {
			t.Performance.PrefetchFrames = 10
		}
	}
	if t.Performance.JPEGEncoder == "" {
		t.Performance.JPEGEncoder = "auto"
	}
	if t.Performance.IdleTimeoutSeconds <= 0 {
		t.Performance.IdleTimeoutSeconds = 20
	}
	return &t, nil
}

func parseColor(value string) color.RGBA {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	alpha := uint8(0xff)
	if len(value) == 8 {
		if n, err := strconv.ParseUint(value[6:8], 16, 8); err == nil {
			alpha = uint8(n)
		}
		value = value[:6]
	}
	if len(value) != 6 {
		return color.RGBA{}
	}
	n, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return color.RGBA{}
	}
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: alpha}
}
