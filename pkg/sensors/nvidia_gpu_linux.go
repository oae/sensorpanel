//go:build linux

package sensors

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	Register(&NvidiaGPUProvider{})
}

// NvidiaGPUProvider provides NVIDIA GPU sensor data on Linux.
type NvidiaGPUProvider struct {
	nvidiaSMIPath string
}

// Meta returns the sensor metadata.
func (p *NvidiaGPUProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "nvidia_gpu",
		Name:        "NVIDIA GPU",
		Description: "NVIDIA GPU statistics via nvidia-smi",
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
			{Name: "Clock", JSONName: "clock", TSName: "clock", Type: FieldTypeOptionalNumber, Unit: "MHz", Description: "GPU clock speed"},
			{Name: "MemoryClock", JSONName: "memory_clock", TSName: "memoryClock", Type: FieldTypeOptionalNumber, Unit: "MHz", Description: "Memory clock speed"},
		},
	}
}

// Available returns true if NVIDIA GPU data can be collected.
func (p *NvidiaGPUProvider) Available() bool {
	return p.findNvidiaSMI() != ""
}

// Configure applies the given config to the provider.
func (p *NvidiaGPUProvider) Configure(config *Config) {
	if path, ok := config.GetStringOption("nvidia_gpu.smi_path"); ok {
		p.nvidiaSMIPath = path
	}
}

// Options returns the configuration options for this provider.
func (p *NvidiaGPUProvider) Options() []OptionDef {
	return []OptionDef{
		{
			Key:         "nvidia_gpu.smi_path",
			Type:        "string",
			Default:     "nvidia-smi (searched in PATH)",
			Description: "Custom path to nvidia-smi binary",
			Example:     "--opt nvidia_gpu.smi_path=/usr/local/bin/nvidia-smi",
		},
	}
}

// Collect gathers NVIDIA GPU sensor data.
func (p *NvidiaGPUProvider) Collect(state *CollectorState) map[string]interface{} {
	smiPath := p.findNvidiaSMI()
	if smiPath == "" {
		return nil
	}

	// Query GPU stats via nvidia-smi
	cmd := exec.Command(smiPath,
		"--query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw,fan.speed,clocks.current.graphics,clocks.current.memory",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	line := strings.TrimSpace(string(output))
	parts := strings.Split(line, ", ")
	if len(parts) < 7 {
		return nil
	}

	result := map[string]interface{}{
		"name": strings.TrimSpace(parts[0]),
	}

	if temp, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
		result["temperature"] = temp
	}

	if load, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
		result["load"] = load
	}

	if memUsed, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
		result["memory_used"] = memUsed
	}

	if memTotal, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err == nil {
		result["memory_total"] = memTotal
	}

	if power, err := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64); err == nil {
		result["power"] = power
	}

	if fanSpeed, err := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64); err == nil {
		result["fan_speed"] = fanSpeed
	}

	if len(parts) > 7 {
		if clock, err := strconv.ParseFloat(strings.TrimSpace(parts[7]), 64); err == nil {
			result["clock"] = clock
		}
	}

	if len(parts) > 8 {
		if memClock, err := strconv.ParseFloat(strings.TrimSpace(parts[8]), 64); err == nil {
			result["memory_clock"] = memClock
		}
	}

	return result
}

func (p *NvidiaGPUProvider) findNvidiaSMI() string {
	if p.nvidiaSMIPath != "" {
		return p.nvidiaSMIPath
	}

	// Check common paths
	paths := []string{
		"/usr/bin/nvidia-smi",
		"/usr/local/bin/nvidia-smi",
		"/opt/cuda/bin/nvidia-smi",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			p.nvidiaSMIPath = path
			return path
		}
	}

	// Check PATH
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		p.nvidiaSMIPath = path
		return path
	}

	return ""
}
