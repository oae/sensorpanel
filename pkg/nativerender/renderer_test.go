package nativerender

import (
	"testing"
	"time"
)

func TestLowPowerFrameIsFullyMonochrome(t *testing.T) {
	render := New(&Theme{
		Layout:     "trofeo_vertical_v1",
		Background: "#000000",
		Accent:     "#2de2ff",
		Accent2:    "#ff4df3",
		Accent3:    "#71ffa8",
		Text:       "#f8fbff",
		Muted:      "#8ea1bb",
		Panel:      "#06132688",
		PanelLine:  "#245cff99",
	}, 462, 1920)
	data := map[string]interface{}{
		"disk": map[string]interface{}{
			"disks": []map[string]interface{}{
				{"mount_point": "/", "percent": 73.0},
			},
		},
	}

	img := render.RenderLowPower(data, time.Date(2026, 7, 28, 22, 0, 0, 0, time.UTC))
	for offset := 0; offset < len(img.Pix); offset += 4 {
		red, green, blue := img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2]
		if red != green || green != blue {
			t.Fatalf("colored idle pixel at byte %d: rgb(%d,%d,%d)", offset, red, green, blue)
		}
	}
}
