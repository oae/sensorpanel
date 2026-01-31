// Package renderer draws sensor data to images for the USB display.
//
// This package provides a simple text-based dashboard renderer that displays
// system metrics (CPU, GPU, RAM, Disk, Network) on a 480x320 display.
package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// Colors used in the dashboard
var (
	ColorBackground = color.RGBA{0x1a, 0x1a, 0x2e, 0xff} // Dark blue-gray
	ColorText       = color.RGBA{0xff, 0xff, 0xff, 0xff} // White
	ColorLabel      = color.RGBA{0x88, 0x88, 0x99, 0xff} // Gray
	ColorBarBg      = color.RGBA{0x33, 0x33, 0x44, 0xff} // Dark gray
	ColorCPU        = color.RGBA{0x00, 0xd4, 0xff, 0xff} // Cyan
	ColorGPU        = color.RGBA{0x00, 0xff, 0x88, 0xff} // Green
	ColorRAM        = color.RGBA{0xff, 0x88, 0x00, 0xff} // Orange
	ColorDisk       = color.RGBA{0xff, 0x00, 0x88, 0xff} // Pink
	ColorNetwork    = color.RGBA{0x88, 0x00, 0xff, 0xff} // Purple
	ColorTempNormal = color.RGBA{0x00, 0xff, 0x00, 0xff} // Green
	ColorTempWarn   = color.RGBA{0xff, 0xff, 0x00, 0xff} // Yellow
	ColorTempHot    = color.RGBA{0xff, 0x00, 0x00, 0xff} // Red
)

// Config configures the renderer.
type Config struct {
	Width  int
	Height int
}

// DefaultConfig returns a default renderer configuration (480x320).
func DefaultConfig() *Config {
	return &Config{
		Width:  480,
		Height: 320,
	}
}

// Renderer draws sensor data to images.
type Renderer struct {
	config *Config
}

// New creates a new renderer.
func New(config *Config) *Renderer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Renderer{config: config}
}

// Render draws sensor data to an image.
// The data parameter is a map from sensor IDs to their collected data.
func (r *Renderer) Render(data map[string]interface{}) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, r.config.Width, r.config.Height))

	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorBackground}, image.Point{}, draw.Src)

	y := 10

	// CPU Section
	if cpu, ok := data["cpu"].(map[string]interface{}); ok {
		y = r.drawCPU(img, 10, y, cpu)
		y += 15
	}

	// GPU Section (try nvidia first, then amd)
	if gpu, ok := data["nvidia_gpu"].(map[string]interface{}); ok {
		y = r.drawGPU(img, 10, y, gpu)
		y += 15
	} else if gpu, ok := data["amd_gpu"].(map[string]interface{}); ok {
		y = r.drawGPU(img, 10, y, gpu)
		y += 15
	}

	// RAM Section
	if mem, ok := data["memory"].(map[string]interface{}); ok {
		y = r.drawRAM(img, 10, y, mem)
		y += 15
	}

	// Disk Section
	if disk, ok := data["disk"].(map[string]interface{}); ok {
		y = r.drawDisks(img, 10, y, disk)
		y += 15
	}

	// Network Section
	if net, ok := data["network"].(map[string]interface{}); ok {
		r.drawNetwork(img, 10, y, net)
	}

	return img
}

// drawCPU draws the CPU section.
func (r *Renderer) drawCPU(img *image.RGBA, x, y int, cpu map[string]interface{}) int {
	// Label
	r.drawText(img, x, y, "CPU", ColorLabel)

	// Temperature
	tempStr := "--°C"
	tempColor := ColorText
	if temp, ok := getFloat(cpu, "temperature"); ok {
		tempStr = fmt.Sprintf("%.0f°C", temp)
		tempColor = r.tempColor(temp)
	}
	r.drawText(img, x+40, y, tempStr, tempColor)

	// Frequency
	if freq, ok := getFloat(cpu, "frequency_mhz"); ok {
		freqStr := fmt.Sprintf("%.0fMHz", freq)
		r.drawText(img, x+100, y, freqStr, ColorLabel)
	}

	y += 20

	// Load bar
	load := 0.0
	if l, ok := getFloat(cpu, "load_percent"); ok {
		load = l
	}
	r.drawProgressBar(img, x, y, r.config.Width-20, 25, load, ColorCPU)
	r.drawText(img, x+5, y+5, fmt.Sprintf("%.0f%%", load), ColorText)

	return y + 30
}

// drawGPU draws the GPU section.
func (r *Renderer) drawGPU(img *image.RGBA, x, y int, gpu map[string]interface{}) int {
	// Label
	r.drawText(img, x, y, "GPU", ColorLabel)

	available, _ := gpu["available"].(bool)
	if !available {
		r.drawText(img, x+40, y, "N/A", ColorLabel)
		return y + 20
	}

	// Temperature
	tempStr := "--°C"
	tempColor := ColorText
	if temp, ok := getFloat(gpu, "temperature"); ok {
		tempStr = fmt.Sprintf("%.0f°C", temp)
		tempColor = r.tempColor(temp)
	}
	r.drawText(img, x+40, y, tempStr, tempColor)

	// Memory
	memUsed, hasUsed := getFloat(gpu, "memory_used_mb")
	memTotal, hasTotal := getFloat(gpu, "memory_total_mb")
	if hasUsed && hasTotal {
		memStr := fmt.Sprintf("%.0f/%.0fMB", memUsed, memTotal)
		r.drawText(img, x+100, y, memStr, ColorLabel)
	}

	// Power
	if power, ok := getFloat(gpu, "power_watts"); ok {
		powerStr := fmt.Sprintf("%.0fW", power)
		r.drawText(img, x+220, y, powerStr, ColorLabel)
	}

	y += 20

	// Load bar
	load := 0.0
	if l, ok := getFloat(gpu, "load_percent"); ok {
		load = l
	}
	r.drawProgressBar(img, x, y, r.config.Width-20, 25, load, ColorGPU)
	r.drawText(img, x+5, y+5, fmt.Sprintf("%.0f%%", load), ColorText)

	return y + 30
}

// drawRAM draws the RAM section.
func (r *Renderer) drawRAM(img *image.RGBA, x, y int, mem map[string]interface{}) int {
	usedMB, _ := getFloat(mem, "used_mb")
	totalMB, _ := getFloat(mem, "total_mb")
	percent, _ := getFloat(mem, "percent")

	// Label with usage
	label := fmt.Sprintf("RAM  %.1f/%.1fGB", usedMB/1024, totalMB/1024)
	r.drawText(img, x, y, label, ColorLabel)

	y += 20

	// Usage bar
	r.drawProgressBar(img, x, y, r.config.Width-20, 25, percent, ColorRAM)
	r.drawText(img, x+5, y+5, fmt.Sprintf("%.0f%%", percent), ColorText)

	return y + 30
}

// drawDisks draws the disk section.
func (r *Renderer) drawDisks(img *image.RGBA, x, y int, diskData map[string]interface{}) int {
	disks, ok := diskData["disks"].([]interface{})
	if !ok || len(disks) == 0 {
		r.drawText(img, x, y, "DISK  N/A", ColorLabel)
		return y + 20
	}

	for _, d := range disks {
		disk, ok := d.(map[string]interface{})
		if !ok {
			continue
		}

		mountPoint, _ := disk["mount_point"].(string)
		usedGB, _ := getFloat(disk, "used_gb")
		totalGB, _ := getFloat(disk, "total_gb")
		percent, _ := getFloat(disk, "percent")

		label := fmt.Sprintf("%-6s %.0f/%.0fGB", mountPoint, usedGB, totalGB)
		r.drawText(img, x, y, label, ColorLabel)

		y += 20

		r.drawProgressBar(img, x, y, r.config.Width-20, 20, percent, ColorDisk)
		r.drawText(img, x+5, y+3, fmt.Sprintf("%.0f%%", percent), ColorText)

		y += 25
	}

	return y
}

// drawNetwork draws the network section.
func (r *Renderer) drawNetwork(img *image.RGBA, x, y int, netData map[string]interface{}) int {
	r.drawText(img, x, y, "NET", ColorLabel)
	y += 20

	interfaces, ok := netData["interfaces"].([]interface{})
	if !ok || len(interfaces) == 0 {
		r.drawText(img, x, y, "No active interfaces", ColorLabel)
		return y + 20
	}

	for _, n := range interfaces {
		net, ok := n.(map[string]interface{})
		if !ok {
			continue
		}

		iface, _ := net["interface"].(string)
		rxBPS, _ := getFloat(net, "rx_bytes_per_sec")
		txBPS, _ := getFloat(net, "tx_bytes_per_sec")
		rxTotal, hasRxTotal := getFloat(net, "rx_total_bytes")

		// Only show interfaces with traffic
		if rxBPS == 0 && txBPS == 0 && (!hasRxTotal || rxTotal == 0) {
			continue
		}

		rxStr := formatBytesPerSec(rxBPS)
		txStr := formatBytesPerSec(txBPS)
		line := fmt.Sprintf("%-8s ↓%s ↑%s", iface, rxStr, txStr)
		r.drawText(img, x, y, line, ColorNetwork)
		y += 18
	}

	return y
}

// drawProgressBar draws a progress bar.
func (r *Renderer) drawProgressBar(img *image.RGBA, x, y, width, height int, percent float64, fillColor color.RGBA) {
	// Background
	r.fillRect(img, x, y, width, height, ColorBarBg)

	// Fill
	fillWidth := int(float64(width) * percent / 100.0)
	if fillWidth > 0 {
		r.fillRect(img, x, y, fillWidth, height, fillColor)
	}

	// Border (1px darker)
	borderColor := color.RGBA{0x22, 0x22, 0x33, 0xff}
	r.drawRect(img, x, y, width, height, borderColor)
}

// drawText draws text using a simple 5x7 bitmap font.
func (r *Renderer) drawText(img *image.RGBA, x, y int, text string, c color.RGBA) {
	for _, ch := range text {
		if ch < 32 || ch > 126 {
			ch = '?'
		}
		charData := font5x7[ch-32]
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if charData[row]&(1<<(4-col)) != 0 {
					img.SetRGBA(x+col, y+row, c)
				}
			}
		}
		x += 6 // 5 pixels + 1 spacing
	}
}

// fillRect fills a rectangle with a solid color.
func (r *Renderer) fillRect(img *image.RGBA, x, y, width, height int, c color.RGBA) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			img.SetRGBA(x+dx, y+dy, c)
		}
	}
}

// drawRect draws a rectangle border.
func (r *Renderer) drawRect(img *image.RGBA, x, y, width, height int, c color.RGBA) {
	// Top and bottom
	for dx := 0; dx < width; dx++ {
		img.SetRGBA(x+dx, y, c)
		img.SetRGBA(x+dx, y+height-1, c)
	}
	// Left and right
	for dy := 0; dy < height; dy++ {
		img.SetRGBA(x, y+dy, c)
		img.SetRGBA(x+width-1, y+dy, c)
	}
}

// tempColor returns a color based on temperature value.
func (r *Renderer) tempColor(temp float64) color.RGBA {
	if temp < 60 {
		return ColorTempNormal
	} else if temp < 80 {
		return ColorTempWarn
	}
	return ColorTempHot
}

// getFloat extracts a float64 from a map, handling both float64 and *float64.
func getFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case *float64:
		if val != nil {
			return *val, true
		}
		return 0, false
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// formatBytesPerSec formats bytes per second as a human-readable string.
func formatBytesPerSec(bps float64) string {
	return formatBytes(bps) + "/s"
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for _, unit := range units {
		if bytes < 1024 {
			return fmt.Sprintf("%.1f%s", bytes, unit)
		}
		bytes /= 1024
	}
	return fmt.Sprintf("%.1fPB", bytes)
}

// font5x7 is a simple 5x7 bitmap font for ASCII characters 32-126.
// Each character is 7 rows of 5 bits (stored in a byte, high bit = leftmost pixel).
var font5x7 = [95][7]byte{
	// Space (32)
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	// ! (33)
	{0x04, 0x04, 0x04, 0x04, 0x04, 0x00, 0x04},
	// " (34)
	{0x0a, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00},
	// # (35)
	{0x0a, 0x0a, 0x1f, 0x0a, 0x1f, 0x0a, 0x0a},
	// $ (36)
	{0x04, 0x0f, 0x14, 0x0e, 0x05, 0x1e, 0x04},
	// % (37)
	{0x18, 0x19, 0x02, 0x04, 0x08, 0x13, 0x03},
	// & (38)
	{0x08, 0x14, 0x14, 0x08, 0x15, 0x12, 0x0d},
	// ' (39)
	{0x04, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00},
	// ( (40)
	{0x02, 0x04, 0x08, 0x08, 0x08, 0x04, 0x02},
	// ) (41)
	{0x08, 0x04, 0x02, 0x02, 0x02, 0x04, 0x08},
	// * (42)
	{0x00, 0x04, 0x15, 0x0e, 0x15, 0x04, 0x00},
	// + (43)
	{0x00, 0x04, 0x04, 0x1f, 0x04, 0x04, 0x00},
	// , (44)
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x08},
	// - (45)
	{0x00, 0x00, 0x00, 0x1f, 0x00, 0x00, 0x00},
	// . (46)
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04},
	// / (47)
	{0x00, 0x01, 0x02, 0x04, 0x08, 0x10, 0x00},
	// 0 (48)
	{0x0e, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0e},
	// 1 (49)
	{0x04, 0x0c, 0x04, 0x04, 0x04, 0x04, 0x0e},
	// 2 (50)
	{0x0e, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1f},
	// 3 (51)
	{0x0e, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0e},
	// 4 (52)
	{0x02, 0x06, 0x0a, 0x12, 0x1f, 0x02, 0x02},
	// 5 (53)
	{0x1f, 0x10, 0x1e, 0x01, 0x01, 0x11, 0x0e},
	// 6 (54)
	{0x06, 0x08, 0x10, 0x1e, 0x11, 0x11, 0x0e},
	// 7 (55)
	{0x1f, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	// 8 (56)
	{0x0e, 0x11, 0x11, 0x0e, 0x11, 0x11, 0x0e},
	// 9 (57)
	{0x0e, 0x11, 0x11, 0x0f, 0x01, 0x02, 0x0c},
	// : (58)
	{0x00, 0x00, 0x04, 0x00, 0x00, 0x04, 0x00},
	// ; (59)
	{0x00, 0x00, 0x04, 0x00, 0x00, 0x04, 0x08},
	// < (60)
	{0x02, 0x04, 0x08, 0x10, 0x08, 0x04, 0x02},
	// = (61)
	{0x00, 0x00, 0x1f, 0x00, 0x1f, 0x00, 0x00},
	// > (62)
	{0x08, 0x04, 0x02, 0x01, 0x02, 0x04, 0x08},
	// ? (63)
	{0x0e, 0x11, 0x01, 0x02, 0x04, 0x00, 0x04},
	// @ (64)
	{0x0e, 0x11, 0x17, 0x15, 0x17, 0x10, 0x0e},
	// A (65)
	{0x0e, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11},
	// B (66)
	{0x1e, 0x11, 0x11, 0x1e, 0x11, 0x11, 0x1e},
	// C (67)
	{0x0e, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0e},
	// D (68)
	{0x1e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1e},
	// E (69)
	{0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x1f},
	// F (70)
	{0x1f, 0x10, 0x10, 0x1e, 0x10, 0x10, 0x10},
	// G (71)
	{0x0e, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0f},
	// H (72)
	{0x11, 0x11, 0x11, 0x1f, 0x11, 0x11, 0x11},
	// I (73)
	{0x0e, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0e},
	// J (74)
	{0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0c},
	// K (75)
	{0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	// L (76)
	{0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1f},
	// M (77)
	{0x11, 0x1b, 0x15, 0x15, 0x11, 0x11, 0x11},
	// N (78)
	{0x11, 0x11, 0x19, 0x15, 0x13, 0x11, 0x11},
	// O (79)
	{0x0e, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0e},
	// P (80)
	{0x1e, 0x11, 0x11, 0x1e, 0x10, 0x10, 0x10},
	// Q (81)
	{0x0e, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0d},
	// R (82)
	{0x1e, 0x11, 0x11, 0x1e, 0x14, 0x12, 0x11},
	// S (83)
	{0x0e, 0x11, 0x10, 0x0e, 0x01, 0x11, 0x0e},
	// T (84)
	{0x1f, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	// U (85)
	{0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0e},
	// V (86)
	{0x11, 0x11, 0x11, 0x11, 0x11, 0x0a, 0x04},
	// W (87)
	{0x11, 0x11, 0x11, 0x15, 0x15, 0x1b, 0x11},
	// X (88)
	{0x11, 0x11, 0x0a, 0x04, 0x0a, 0x11, 0x11},
	// Y (89)
	{0x11, 0x11, 0x0a, 0x04, 0x04, 0x04, 0x04},
	// Z (90)
	{0x1f, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1f},
	// [ (91)
	{0x0e, 0x08, 0x08, 0x08, 0x08, 0x08, 0x0e},
	// \ (92)
	{0x00, 0x10, 0x08, 0x04, 0x02, 0x01, 0x00},
	// ] (93)
	{0x0e, 0x02, 0x02, 0x02, 0x02, 0x02, 0x0e},
	// ^ (94)
	{0x04, 0x0a, 0x11, 0x00, 0x00, 0x00, 0x00},
	// _ (95)
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1f},
	// ` (96)
	{0x08, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00},
	// a (97)
	{0x00, 0x00, 0x0e, 0x01, 0x0f, 0x11, 0x0f},
	// b (98)
	{0x10, 0x10, 0x1e, 0x11, 0x11, 0x11, 0x1e},
	// c (99)
	{0x00, 0x00, 0x0e, 0x11, 0x10, 0x11, 0x0e},
	// d (100)
	{0x01, 0x01, 0x0f, 0x11, 0x11, 0x11, 0x0f},
	// e (101)
	{0x00, 0x00, 0x0e, 0x11, 0x1f, 0x10, 0x0e},
	// f (102)
	{0x06, 0x08, 0x1c, 0x08, 0x08, 0x08, 0x08},
	// g (103)
	{0x00, 0x00, 0x0f, 0x11, 0x0f, 0x01, 0x0e},
	// h (104)
	{0x10, 0x10, 0x1e, 0x11, 0x11, 0x11, 0x11},
	// i (105)
	{0x04, 0x00, 0x0c, 0x04, 0x04, 0x04, 0x0e},
	// j (106)
	{0x02, 0x00, 0x06, 0x02, 0x02, 0x12, 0x0c},
	// k (107)
	{0x10, 0x10, 0x12, 0x14, 0x18, 0x14, 0x12},
	// l (108)
	{0x0c, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0e},
	// m (109)
	{0x00, 0x00, 0x1a, 0x15, 0x15, 0x15, 0x15},
	// n (110)
	{0x00, 0x00, 0x1e, 0x11, 0x11, 0x11, 0x11},
	// o (111)
	{0x00, 0x00, 0x0e, 0x11, 0x11, 0x11, 0x0e},
	// p (112)
	{0x00, 0x00, 0x1e, 0x11, 0x1e, 0x10, 0x10},
	// q (113)
	{0x00, 0x00, 0x0f, 0x11, 0x0f, 0x01, 0x01},
	// r (114)
	{0x00, 0x00, 0x16, 0x19, 0x10, 0x10, 0x10},
	// s (115)
	{0x00, 0x00, 0x0f, 0x10, 0x0e, 0x01, 0x1e},
	// t (116)
	{0x08, 0x08, 0x1c, 0x08, 0x08, 0x09, 0x06},
	// u (117)
	{0x00, 0x00, 0x11, 0x11, 0x11, 0x11, 0x0f},
	// v (118)
	{0x00, 0x00, 0x11, 0x11, 0x11, 0x0a, 0x04},
	// w (119)
	{0x00, 0x00, 0x11, 0x11, 0x15, 0x15, 0x0a},
	// x (120)
	{0x00, 0x00, 0x11, 0x0a, 0x04, 0x0a, 0x11},
	// y (121)
	{0x00, 0x00, 0x11, 0x11, 0x0f, 0x01, 0x0e},
	// z (122)
	{0x00, 0x00, 0x1f, 0x02, 0x04, 0x08, 0x1f},
	// { (123)
	{0x02, 0x04, 0x04, 0x08, 0x04, 0x04, 0x02},
	// | (124)
	{0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	// } (125)
	{0x08, 0x04, 0x04, 0x02, 0x04, 0x04, 0x08},
	// ~ (126)
	{0x00, 0x00, 0x08, 0x15, 0x02, 0x00, 0x00},
}
