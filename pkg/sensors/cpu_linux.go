//go:build linux

package sensors

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&CPUProvider{})
}

// CPUProvider provides CPU sensor data on Linux. Static metadata and sysfs
// paths are discovered once and refreshed only after a read failure.
type CPUProvider struct {
	name       string
	tempPath   string
	freqPath   string
	discovered bool
}

// Meta returns the sensor metadata.
func (p *CPUProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "cpu",
		Name:        "CPU",
		Description: "CPU usage, temperature, and frequency",
		Category:    "system",
		Platforms:   []string{"linux"},
		Fields: []FieldDef{
			{Name: "Name", JSONName: "name", TSName: "name", Type: FieldTypeString, Unit: "", Description: "CPU model name"},
			{Name: "Load", JSONName: "load", TSName: "load", Type: FieldTypeNumber, Unit: "%", Description: "CPU load percentage"},
			{Name: "Temperature", JSONName: "temperature", TSName: "temperature", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "CPU temperature"},
			{Name: "Frequency", JSONName: "frequency", TSName: "frequency", Type: FieldTypeOptionalNumber, Unit: "MHz", Description: "CPU frequency"},
			{Name: "Cores", JSONName: "cores", TSName: "cores", Type: FieldTypeNumber, Unit: "", Description: "Number of CPU cores"},
		},
	}
}

// Available returns true if CPU data can be collected.
func (p *CPUProvider) Available() bool {
	_, err := os.Stat("/proc/stat")
	return err == nil
}

// Collect gathers CPU sensor data.
func (p *CPUProvider) Collect(state *CollectorState) map[string]interface{} {
	p.discover()
	result := make(map[string]interface{})

	// Get CPU name
	result["name"] = p.collectName()

	// Get CPU load
	load := p.collectLoad(state)
	result["load"] = load

	// Get temperature
	if temp := p.collectTemperature(); temp != nil {
		result["temperature"] = *temp
	}

	// Get frequency
	if freq := p.collectFrequency(); freq != nil {
		result["frequency"] = *freq
	}

	// Get core count
	result["cores"] = runtime.NumCPU()

	return result
}

func (p *CPUProvider) collectLoad(state *CollectorState) float64 {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return 0
	}

	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0
	}

	var total, idle uint64
	for i, field := range fields[1:] {
		val, _ := strconv.ParseUint(field, 10, 64)
		total += val
		if i == 3 { // idle is the 4th field
			idle = val
		}
	}

	// Get previous values
	prevIdle, _ := GetTyped[uint64](state, "cpu_prev_idle")
	prevTotal, _ := GetTyped[uint64](state, "cpu_prev_total")
	prevTime, hasPrev := GetTyped[time.Time](state, "cpu_prev_time")

	// Store current values
	state.Set("cpu_prev_idle", idle)
	state.Set("cpu_prev_total", total)
	state.Set("cpu_prev_time", time.Now())

	if !hasPrev || time.Since(prevTime) > 2*time.Minute {
		return 0
	}

	idleDelta := idle - prevIdle
	totalDelta := total - prevTotal

	if totalDelta == 0 {
		return 0
	}

	return (1.0 - float64(idleDelta)/float64(totalDelta)) * 100.0
}

func (p *CPUProvider) collectTemperature() *float64 {
	if p.tempPath != "" {
		if temp := readMilliValue(p.tempPath, 1000); temp != nil {
			return temp
		}
		p.discovered = false
		p.discover()
		if p.tempPath != "" {
			return readMilliValue(p.tempPath, 1000)
		}
	}
	return nil
}

func (p *CPUProvider) discover() {
	if p.discovered {
		return
	}
	p.discovered = true
	p.name = p.readName()
	p.tempPath = p.findTemperaturePath()
	p.freqPath = ""
	if _, err := os.Stat("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); err == nil {
		p.freqPath = "/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"
	}
}

func (p *CPUProvider) findTemperaturePath() string {
	// Try hwmon first
	hwmonPaths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	for _, namePath := range hwmonPaths {
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		// Look for CPU-related hwmon
		if strings.Contains(name, "coretemp") || strings.Contains(name, "k10temp") ||
			strings.Contains(name, "cpu") || strings.Contains(name, "zenpower") {
			dir := filepath.Dir(namePath)
			tempFiles, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
			for _, tempFile := range tempFiles {
				if readMilliValue(tempFile, 1000) != nil {
					return tempFile
				}
			}
		}
	}

	// Try thermal zones
	thermalPaths, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, tempPath := range thermalPaths {
		typePath := filepath.Join(filepath.Dir(tempPath), "type")
		typeBytes, err := os.ReadFile(typePath)
		if err != nil {
			continue
		}
		zoneType := strings.TrimSpace(string(typeBytes))
		if strings.Contains(strings.ToLower(zoneType), "cpu") ||
			strings.Contains(strings.ToLower(zoneType), "x86_pkg") {
			if readMilliValue(tempPath, 1000) != nil {
				return tempPath
			}
		}
	}

	return ""
}

func (p *CPUProvider) collectFrequency() *float64 {
	if p.freqPath != "" {
		data, err := os.ReadFile(p.freqPath)
		if err != nil {
			p.discovered = false
			return nil
		}
		khz, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			freq := float64(khz) / 1000.0
			return &freq
		}
	}

	// Fallback to /proc/cpuinfo
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				mhz, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err == nil {
					return &mhz
				}
			}
		}
	}

	return nil
}

func (p *CPUProvider) collectName() string {
	if p.name == "" {
		p.discover()
	}
	return p.name
}

func (p *CPUProvider) readName() string {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "Unknown CPU"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return "Unknown CPU"
}

func readMilliValue(path string, divisor float64) *float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	raw, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return nil
	}
	value := float64(raw) / divisor
	return &value
}
