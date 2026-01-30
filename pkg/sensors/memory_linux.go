//go:build linux

package sensors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func init() {
	Register(&MemoryProvider{})
}

// MemoryProvider provides memory sensor data on Linux.
type MemoryProvider struct{}

// Meta returns the sensor metadata.
func (p *MemoryProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "memory",
		Name:        "Memory",
		Description: "System memory (RAM) usage",
		Category:    "system",
		Platforms:   []string{"linux"},
		Fields: []FieldDef{
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "MB", Description: "Total memory"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "MB", Description: "Used memory"},
			{Name: "Available", JSONName: "available", TSName: "available", Type: FieldTypeNumber, Unit: "MB", Description: "Available memory"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Memory usage percentage"},
		},
	}
}

// Available returns true if memory data can be collected.
func (p *MemoryProvider) Available() bool {
	_, err := os.Stat("/proc/meminfo")
	return err == nil
}

// Collect gathers memory sensor data.
func (p *MemoryProvider) Collect(state *CollectorState) map[string]interface{} {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil
	}
	defer file.Close()

	var totalKB, freeKB, availableKB, buffersKB, cachedKB uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, _ := strconv.ParseUint(fields[1], 10, 64)

		switch key {
		case "MemTotal":
			totalKB = value
		case "MemFree":
			freeKB = value
		case "MemAvailable":
			availableKB = value
		case "Buffers":
			buffersKB = value
		case "Cached":
			cachedKB = value
		}
	}

	// If MemAvailable not present (old kernels), estimate it
	if availableKB == 0 {
		availableKB = freeKB + buffersKB + cachedKB
	}

	totalMB := float64(totalKB) / 1024.0
	availableMB := float64(availableKB) / 1024.0
	usedMB := totalMB - availableMB

	var percent float64
	if totalMB > 0 {
		percent = (usedMB / totalMB) * 100.0
	}

	return map[string]interface{}{
		"total":     totalMB,
		"used":      usedMB,
		"available": availableMB,
		"percent":   percent,
	}
}
