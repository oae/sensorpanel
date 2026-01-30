//go:build linux

package sensors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	Register(&AMDGPUProvider{})
}

// AMDGPUProvider provides AMD GPU sensor data on Linux.
type AMDGPUProvider struct{}

// Meta returns the sensor metadata.
func (p *AMDGPUProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "amd_gpu",
		Name:        "AMD GPU",
		Description: "AMD GPU statistics via sysfs",
		Category:    "gpu",
		Platforms:   []string{"linux"},
		Fields: []FieldDef{
			{Name: "Name", JSONName: "name", TSName: "name", Type: FieldTypeString, Unit: "", Description: "GPU name"},
			{Name: "Temperature", JSONName: "temperature", TSName: "temperature", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "GPU temperature"},
			{Name: "Load", JSONName: "load", TSName: "load", Type: FieldTypeOptionalNumber, Unit: "%", Description: "GPU utilization"},
			{Name: "MemoryUsed", JSONName: "memory_used", TSName: "memoryUsed", Type: FieldTypeOptionalNumber, Unit: "MB", Description: "VRAM used"},
			{Name: "MemoryTotal", JSONName: "memory_total", TSName: "memoryTotal", Type: FieldTypeOptionalNumber, Unit: "MB", Description: "VRAM total"},
			{Name: "Power", JSONName: "power", TSName: "power", Type: FieldTypeOptionalNumber, Unit: "W", Description: "Power draw"},
			{Name: "FanSpeed", JSONName: "fan_speed", TSName: "fanSpeed", Type: FieldTypeOptionalNumber, Unit: "%", Description: "Fan speed"},
		},
	}
}

// Available returns true if AMD GPU data can be collected.
func (p *AMDGPUProvider) Available() bool {
	return p.findAMDCard() != ""
}

// Collect gathers AMD GPU sensor data.
func (p *AMDGPUProvider) Collect(state *CollectorState) map[string]interface{} {
	cardPath := p.findAMDCard()
	if cardPath == "" {
		return nil
	}

	result := map[string]interface{}{
		"name": "AMD GPU",
	}

	// Read GPU load
	if data, err := os.ReadFile(filepath.Join(cardPath, "gpu_busy_percent")); err == nil {
		if load, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
			result["load"] = load
		}
	}

	// Read temperature from hwmon
	hwmonPaths, _ := filepath.Glob(filepath.Join(cardPath, "hwmon", "hwmon*"))
	for _, hwmon := range hwmonPaths {
		// Temperature
		if data, err := os.ReadFile(filepath.Join(hwmon, "temp1_input")); err == nil {
			if milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
				result["temperature"] = float64(milliC) / 1000.0
			}
		}

		// Power
		if data, err := os.ReadFile(filepath.Join(hwmon, "power1_average")); err == nil {
			if microW, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
				result["power"] = float64(microW) / 1000000.0
			}
		}

		// Fan speed (PWM to percent)
		pwm, pwmErr := os.ReadFile(filepath.Join(hwmon, "pwm1"))
		pwmMax, pwmMaxErr := os.ReadFile(filepath.Join(hwmon, "pwm1_max"))
		if pwmErr == nil && pwmMaxErr == nil {
			if pwmVal, err := strconv.ParseFloat(strings.TrimSpace(string(pwm)), 64); err == nil {
				if maxVal, err := strconv.ParseFloat(strings.TrimSpace(string(pwmMax)), 64); err == nil && maxVal > 0 {
					result["fan_speed"] = (pwmVal / maxVal) * 100.0
				}
			}
		}
	}

	// VRAM info
	if data, err := os.ReadFile(filepath.Join(cardPath, "mem_info_vram_used")); err == nil {
		if bytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			result["memory_used"] = float64(bytes) / (1024 * 1024)
		}
	}

	if data, err := os.ReadFile(filepath.Join(cardPath, "mem_info_vram_total")); err == nil {
		if bytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			result["memory_total"] = float64(bytes) / (1024 * 1024)
		}
	}

	return result
}

func (p *AMDGPUProvider) findAMDCard() string {
	cardPaths, _ := filepath.Glob("/sys/class/drm/card*/device")

	for _, cardPath := range cardPaths {
		// Check if it's an AMD GPU by looking for amdgpu driver
		driverLink, err := os.Readlink(filepath.Join(cardPath, "driver"))
		if err != nil {
			continue
		}
		if strings.Contains(driverLink, "amdgpu") {
			return cardPath
		}

		// Also check for vendor ID (AMD = 0x1002)
		if data, err := os.ReadFile(filepath.Join(cardPath, "vendor")); err == nil {
			vendor := strings.TrimSpace(string(data))
			if vendor == "0x1002" {
				return cardPath
			}
		}
	}

	return ""
}
