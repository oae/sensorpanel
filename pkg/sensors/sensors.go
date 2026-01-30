// Package sensors collects system metrics from various sources.
//
// Supported metrics:
//   - CPU: temperature, load, frequency
//   - GPU: temperature, load, memory (NVIDIA via nvidia-smi, AMD via sysfs)
//   - RAM: usage statistics
//   - Disk: usage per mount point
//   - Network: throughput per interface
package sensors

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CPUStats contains CPU statistics.
type CPUStats struct {
	Temperature  *float64 // Celsius (nil if unavailable)
	LoadPercent  float64  // 0-100
	FrequencyMHz *float64 // Current frequency (nil if unavailable)
	CoreCount    int
}

// GPUStats contains GPU statistics.
type GPUStats struct {
	Name          string
	Temperature   *float64 // Celsius
	LoadPercent   *float64 // 0-100
	MemoryUsedMB  *float64
	MemoryTotalMB *float64
	PowerWatts    *float64
	Available     bool
}

// MemoryStats contains memory statistics.
type MemoryStats struct {
	TotalMB     float64
	UsedMB      float64
	AvailableMB float64
	Percent     float64 // 0-100
}

// DiskStats contains disk usage for a single mount point.
type DiskStats struct {
	MountPoint string
	TotalGB    float64
	UsedGB     float64
	FreeGB     float64
	Percent    float64 // 0-100
}

// NetworkStats contains network throughput statistics.
type NetworkStats struct {
	Interface     string
	RxBytesPerSec float64
	TxBytesPerSec float64
	RxTotalBytes  uint64
	TxTotalBytes  uint64
}

// Data contains all sensor readings.
type Data struct {
	CPU       CPUStats
	GPU       GPUStats
	Memory    MemoryStats
	Disks     []DiskStats
	Networks  []NetworkStats
	Timestamp time.Time
}

// Config configures the sensor collector.
type Config struct {
	// Which sensors to collect
	ShowCPU     bool
	ShowGPU     bool
	ShowRAM     bool
	ShowDisk    bool
	ShowNetwork bool

	// Disk mount points to monitor
	DiskMounts []string

	// Network interface pattern (e.g., "eth*", "enp*", "*" for all)
	NetworkInterface string

	// GPU collection method: "auto", "nvidia", "amd"
	GPUMethod string

	// Path to nvidia-smi (auto-detected if empty)
	NvidiaSMIPath string
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		ShowCPU:          true,
		ShowGPU:          true,
		ShowRAM:          true,
		ShowDisk:         true,
		ShowNetwork:      true,
		DiskMounts:       []string{"/"},
		NetworkInterface: "*",
		GPUMethod:        "auto",
	}
}

// Collector collects system sensor data.
type Collector struct {
	config *Config

	mu sync.Mutex

	// CPU load calculation state
	prevCPUIdle  uint64
	prevCPUTotal uint64
	prevCPUTime  time.Time

	// Network throughput calculation state
	prevNetStats map[string]netSample

	// nvidia-smi path (cached)
	nvidiaSMI string
}

type netSample struct {
	rxBytes uint64
	txBytes uint64
	time    time.Time
}

// NewCollector creates a new sensor collector.
func NewCollector(config *Config) *Collector {
	if config == nil {
		config = DefaultConfig()
	}

	c := &Collector{
		config:       config,
		prevNetStats: make(map[string]netSample),
	}

	// Find nvidia-smi
	if config.GPUMethod == "nvidia" || config.GPUMethod == "auto" {
		c.nvidiaSMI = c.findNvidiaSMI()
	}

	return c
}

// Collect gathers all sensor data.
func (c *Collector) Collect() *Data {
	data := &Data{
		Timestamp: time.Now(),
	}

	if c.config.ShowCPU {
		data.CPU = c.collectCPU()
	}

	if c.config.ShowGPU {
		data.GPU = c.collectGPU()
	}

	if c.config.ShowRAM {
		data.Memory = c.collectMemory()
	}

	if c.config.ShowDisk {
		data.Disks = c.collectDisks()
	}

	if c.config.ShowNetwork {
		data.Networks = c.collectNetwork()
	}

	return data
}

func (c *Collector) findNvidiaSMI() string {
	if c.config.NvidiaSMIPath != "" {
		if _, err := os.Stat(c.config.NvidiaSMIPath); err == nil {
			return c.config.NvidiaSMIPath
		}
	}

	// Common paths
	paths := []string{
		"/run/current-system/sw/bin/nvidia-smi", // NixOS
		"/usr/bin/nvidia-smi",
		"/usr/local/bin/nvidia-smi",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try PATH
	if path, err := exec.LookPath("nvidia-smi"); err == nil {
		return path
	}

	return ""
}

func (c *Collector) collectCPU() CPUStats {
	stats := CPUStats{
		CoreCount: runtime.NumCPU(),
	}

	// CPU load from /proc/stat
	if load, ok := c.calculateCPULoad(); ok {
		stats.LoadPercent = load
	}

	// CPU temperature
	if temp := c.getCPUTemperature(); temp != nil {
		stats.Temperature = temp
	}

	// CPU frequency
	if freq := c.getCPUFrequency(); freq != nil {
		stats.FrequencyMHz = freq
	}

	return stats
}

func (c *Collector) calculateCPULoad() (float64, bool) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, false
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return 0, false
	}

	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0, false
	}

	var values []uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, false
		}
		values = append(values, v)
	}

	// idle = idle + iowait
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	var total uint64
	for _, v := range values {
		total += v
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if c.prevCPUTime.IsZero() {
		// First sample
		c.prevCPUIdle = idle
		c.prevCPUTotal = total
		c.prevCPUTime = now
		return 0, true
	}

	idleDelta := idle - c.prevCPUIdle
	totalDelta := total - c.prevCPUTotal

	c.prevCPUIdle = idle
	c.prevCPUTotal = total
	c.prevCPUTime = now

	if totalDelta == 0 {
		return 0, true
	}

	load := (1.0 - float64(idleDelta)/float64(totalDelta)) * 100.0
	if load < 0 {
		load = 0
	}
	if load > 100 {
		load = 100
	}

	return load, true
}

func (c *Collector) getCPUTemperature() *float64 {
	// Try hwmon sensors
	hwmonPaths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")

	for _, namePath := range hwmonPaths {
		name, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}

		sensorName := strings.TrimSpace(string(name))
		hwmonDir := filepath.Dir(namePath)

		// CPU-related sensors
		cpuSensors := []string{"coretemp", "k10temp", "zenpower", "cpu_thermal", "acpitz"}
		isCPU := false
		for _, s := range cpuSensors {
			if sensorName == s {
				isCPU = true
				break
			}
		}

		if !isCPU {
			continue
		}

		// Find temp input files
		tempFiles, _ := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
		if len(tempFiles) == 0 {
			continue
		}

		data, err := os.ReadFile(tempFiles[0])
		if err != nil {
			continue
		}

		// Value is in millidegrees
		milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			continue
		}

		temp := float64(milliC) / 1000.0
		return &temp
	}

	// Fallback: thermal zones
	thermalPaths, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, tempPath := range thermalPaths {
		typePath := filepath.Join(filepath.Dir(tempPath), "type")
		typeData, err := os.ReadFile(typePath)
		if err != nil {
			continue
		}

		zoneType := strings.ToLower(strings.TrimSpace(string(typeData)))
		if !strings.Contains(zoneType, "cpu") && !strings.Contains(zoneType, "x86") && !strings.Contains(zoneType, "core") {
			continue
		}

		data, err := os.ReadFile(tempPath)
		if err != nil {
			continue
		}

		milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			continue
		}

		temp := float64(milliC) / 1000.0
		return &temp
	}

	return nil
}

func (c *Collector) getCPUFrequency() *float64 {
	// Try scaling_cur_freq
	freqFiles, _ := filepath.Glob("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if len(freqFiles) > 0 {
		data, err := os.ReadFile(freqFiles[0])
		if err == nil {
			// Value is in kHz
			kHz, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			if err == nil {
				freq := float64(kHz) / 1000.0
				return &freq
			}
		}
	}

	// Fallback: /proc/cpuinfo
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
			if len(parts) == 2 {
				freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err == nil {
					return &freq
				}
			}
		}
	}

	return nil
}

func (c *Collector) collectGPU() GPUStats {
	if c.nvidiaSMI != "" {
		return c.collectNvidiaGPU()
	}

	if c.config.GPUMethod == "amd" || c.config.GPUMethod == "auto" {
		return c.collectAMDGPU()
	}

	return GPUStats{}
}

func (c *Collector) collectNvidiaGPU() GPUStats {
	stats := GPUStats{}

	if c.nvidiaSMI == "" {
		return stats
	}

	cmd := exec.Command(c.nvidiaSMI,
		"--query-gpu=name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw",
		"--format=csv,noheader,nounits",
	)

	output, err := cmd.Output()
	if err != nil {
		return stats
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return stats
	}

	// Parse first GPU
	parts := strings.Split(lines[0], ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	if len(parts) < 5 {
		return stats
	}

	stats.Available = true
	stats.Name = parts[0]

	if temp, err := strconv.ParseFloat(parts[1], 64); err == nil {
		stats.Temperature = &temp
	}

	if load, err := strconv.ParseFloat(parts[2], 64); err == nil {
		stats.LoadPercent = &load
	}

	if memUsed, err := strconv.ParseFloat(parts[3], 64); err == nil {
		stats.MemoryUsedMB = &memUsed
	}

	if memTotal, err := strconv.ParseFloat(parts[4], 64); err == nil {
		stats.MemoryTotalMB = &memTotal
	}

	if len(parts) > 5 && parts[5] != "[N/A]" {
		if power, err := strconv.ParseFloat(parts[5], 64); err == nil {
			stats.PowerWatts = &power
		}
	}

	return stats
}

func (c *Collector) collectAMDGPU() GPUStats {
	stats := GPUStats{}

	// Look for AMD GPU in DRM
	cardPaths, _ := filepath.Glob("/sys/class/drm/card*/device")

	for _, cardPath := range cardPaths {
		// Check if AMD
		vendorPath := filepath.Join(cardPath, "vendor")
		vendorData, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}

		vendor := strings.TrimSpace(string(vendorData))
		if vendor != "0x1002" { // AMD vendor ID
			continue
		}

		stats.Available = true
		stats.Name = "AMD GPU"

		// Temperature
		hwmonPaths, _ := filepath.Glob(filepath.Join(cardPath, "hwmon/hwmon*/temp1_input"))
		if len(hwmonPaths) > 0 {
			data, err := os.ReadFile(hwmonPaths[0])
			if err == nil {
				milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
				if err == nil {
					temp := float64(milliC) / 1000.0
					stats.Temperature = &temp
				}
			}
		}

		// GPU busy percent
		busyPath := filepath.Join(cardPath, "gpu_busy_percent")
		if data, err := os.ReadFile(busyPath); err == nil {
			load, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err == nil {
				stats.LoadPercent = &load
			}
		}

		// Memory info
		memUsedPath := filepath.Join(cardPath, "mem_info_vram_used")
		if data, err := os.ReadFile(memUsedPath); err == nil {
			bytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			if err == nil {
				mb := float64(bytes) / (1024 * 1024)
				stats.MemoryUsedMB = &mb
			}
		}

		memTotalPath := filepath.Join(cardPath, "mem_info_vram_total")
		if data, err := os.ReadFile(memTotalPath); err == nil {
			bytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
			if err == nil {
				mb := float64(bytes) / (1024 * 1024)
				stats.MemoryTotalMB = &mb
			}
		}

		break // Use first AMD GPU
	}

	return stats
}

func (c *Collector) collectMemory() MemoryStats {
	stats := MemoryStats{}

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return stats
	}
	defer file.Close()

	meminfo := make(map[string]uint64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimSpace(parts[1])
		valueParts := strings.Fields(valueStr)
		if len(valueParts) == 0 {
			continue
		}

		value, err := strconv.ParseUint(valueParts[0], 10, 64)
		if err != nil {
			continue
		}

		meminfo[key] = value // Value is in kB
	}

	total := meminfo["MemTotal"]
	available := meminfo["MemAvailable"]

	if available == 0 {
		// Fallback for older kernels
		available = meminfo["MemFree"] + meminfo["Buffers"] + meminfo["Cached"]
	}

	stats.TotalMB = float64(total) / 1024.0
	stats.AvailableMB = float64(available) / 1024.0
	stats.UsedMB = stats.TotalMB - stats.AvailableMB

	if stats.TotalMB > 0 {
		stats.Percent = (stats.UsedMB / stats.TotalMB) * 100.0
	}

	return stats
}

func (c *Collector) collectDisks() []DiskStats {
	var disks []DiskStats

	for _, mountPoint := range c.config.DiskMounts {
		var stat syscallStatfs
		if err := statfs(mountPoint, &stat); err != nil {
			continue
		}

		blockSize := uint64(stat.Bsize)
		totalBytes := stat.Blocks * blockSize
		freeBytes := stat.Bfree * blockSize
		availBytes := stat.Bavail * blockSize
		usedBytes := totalBytes - freeBytes

		totalGB := float64(totalBytes) / (1024 * 1024 * 1024)
		usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
		freeGB := float64(availBytes) / (1024 * 1024 * 1024)

		var percent float64
		if totalGB > 0 {
			percent = (usedGB / totalGB) * 100.0
		}

		disks = append(disks, DiskStats{
			MountPoint: mountPoint,
			TotalGB:    totalGB,
			UsedGB:     usedGB,
			FreeGB:     freeGB,
			Percent:    percent,
		})
	}

	return disks
}

func (c *Collector) collectNetwork() []NetworkStats {
	var networks []NetworkStats
	now := time.Now()

	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return networks
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Skip header lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])

		// Skip loopback
		if iface == "lo" {
			continue
		}

		// Check interface pattern
		if c.config.NetworkInterface != "*" {
			matched, _ := filepath.Match(c.config.NetworkInterface, iface)
			if !matched {
				continue
			}
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)

		var rxPerSec, txPerSec float64

		c.mu.Lock()
		if prev, ok := c.prevNetStats[iface]; ok {
			dt := now.Sub(prev.time).Seconds()
			if dt > 0 {
				rxPerSec = float64(rxBytes-prev.rxBytes) / dt
				txPerSec = float64(txBytes-prev.txBytes) / dt
			}
		}
		c.prevNetStats[iface] = netSample{
			rxBytes: rxBytes,
			txBytes: txBytes,
			time:    now,
		}
		c.mu.Unlock()

		if rxPerSec < 0 {
			rxPerSec = 0
		}
		if txPerSec < 0 {
			txPerSec = 0
		}

		networks = append(networks, NetworkStats{
			Interface:     iface,
			RxBytesPerSec: rxPerSec,
			TxBytesPerSec: txPerSec,
			RxTotalBytes:  rxBytes,
			TxTotalBytes:  txBytes,
		})
	}

	return networks
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for _, unit := range units {
		if bytes < 1024 {
			return fmt.Sprintf("%.1f%s", bytes, unit)
		}
		bytes /= 1024
	}
	return fmt.Sprintf("%.1fPB", bytes)
}

// FormatBytesPerSec formats bytes per second as a human-readable string.
func FormatBytesPerSec(bps float64) string {
	return FormatBytes(bps) + "/s"
}
