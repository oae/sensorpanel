//go:build linux

package sensors

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	Register(&MotherboardProvider{})
}

// MotherboardProvider provides motherboard sensor data (fans, voltages) on Linux.
type MotherboardProvider struct{}

// Meta returns the sensor metadata.
func (p *MotherboardProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "motherboard",
		Name:        "Motherboard",
		Description: "Motherboard sensors (fans, voltages, temperatures)",
		Category:    "system",
		Platforms:   []string{"linux"},
		Fields: []FieldDef{
			{Name: "CPUFan", JSONName: "cpu_fan", TSName: "cpuFan", Type: FieldTypeOptionalNumber, Unit: "RPM", Description: "CPU fan speed"},
			{Name: "ChipsetFan", JSONName: "chipset_fan", TSName: "chipsetFan", Type: FieldTypeOptionalNumber, Unit: "RPM", Description: "Chipset fan speed"},
			{Name: "SystemFan1", JSONName: "system_fan1", TSName: "systemFan1", Type: FieldTypeOptionalNumber, Unit: "RPM", Description: "System fan 1 speed"},
			{Name: "SystemFan2", JSONName: "system_fan2", TSName: "systemFan2", Type: FieldTypeOptionalNumber, Unit: "RPM", Description: "System fan 2 speed"},
			{Name: "SystemFan3", JSONName: "system_fan3", TSName: "systemFan3", Type: FieldTypeOptionalNumber, Unit: "RPM", Description: "System fan 3 speed"},
			{Name: "CPUVoltage", JSONName: "cpu_voltage", TSName: "cpuVoltage", Type: FieldTypeOptionalNumber, Unit: "V", Description: "CPU core voltage"},
			{Name: "Dimm1Temp", JSONName: "dimm1_temp", TSName: "dimm1Temp", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "DIMM 1 temperature"},
			{Name: "Dimm2Temp", JSONName: "dimm2_temp", TSName: "dimm2Temp", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "DIMM 2 temperature"},
			{Name: "Dimm3Temp", JSONName: "dimm3_temp", TSName: "dimm3Temp", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "DIMM 3 temperature"},
			{Name: "Dimm4Temp", JSONName: "dimm4_temp", TSName: "dimm4Temp", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "DIMM 4 temperature"},
		},
	}
}

// Available returns true if motherboard data can be collected.
func (p *MotherboardProvider) Available() bool {
	return p.findMotherboardHwmon() != ""
}

// Collect gathers motherboard sensor data.
func (p *MotherboardProvider) Collect(state *CollectorState) map[string]interface{} {
	hwmonPath := p.findMotherboardHwmon()
	if hwmonPath == "" {
		return nil
	}

	result := make(map[string]interface{})

	// Read fan speeds
	fanFiles, _ := filepath.Glob(filepath.Join(hwmonPath, "fan*_input"))
	fanIndex := 1
	for _, fanFile := range fanFiles {
		data, err := os.ReadFile(fanFile)
		if err != nil {
			continue
		}
		rpm, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil || rpm == 0 {
			continue
		}

		// Try to get label for this fan
		labelFile := strings.Replace(fanFile, "_input", "_label", 1)
		label := ""
		if labelData, err := os.ReadFile(labelFile); err == nil {
			label = strings.ToLower(strings.TrimSpace(string(labelData)))
		}

		// Map to our fields
		switch {
		case strings.Contains(label, "cpu"):
			result["cpu_fan"] = float64(rpm)
		case strings.Contains(label, "chipset"):
			result["chipset_fan"] = float64(rpm)
		default:
			// Generic system fans
			switch fanIndex {
			case 1:
				if _, exists := result["system_fan1"]; !exists {
					result["system_fan1"] = float64(rpm)
				}
			case 2:
				if _, exists := result["system_fan2"]; !exists {
					result["system_fan2"] = float64(rpm)
				}
			case 3:
				if _, exists := result["system_fan3"]; !exists {
					result["system_fan3"] = float64(rpm)
				}
			}
			fanIndex++
		}
	}

	// Read CPU voltage (look for VCore or similar labels)
	inLabels, _ := filepath.Glob(filepath.Join(hwmonPath, "in*_label"))
	for _, labelPath := range inLabels {
		labelBytes, err := os.ReadFile(labelPath)
		if err != nil {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(string(labelBytes)))

		if strings.Contains(label, "vcore") || strings.Contains(label, "cpu") {
			inputPath := strings.Replace(labelPath, "_label", "_input", 1)
			if data, err := os.ReadFile(inputPath); err == nil {
				if milliV, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
					result["cpu_voltage"] = float64(milliV) / 1000.0
				}
			}
			break
		}
	}

	// Read DIMM temperatures from spd5118 sensors (DDR5 SPD Hub)
	dimmTemps := p.readDimmTemperatures()
	for i, temp := range dimmTemps {
		if temp > 0 {
			result[dimmTempKey(i+1)] = temp
		}
	}

	return result
}

func dimmTempKey(index int) string {
	switch index {
	case 1:
		return "dimm1_temp"
	case 2:
		return "dimm2_temp"
	case 3:
		return "dimm3_temp"
	case 4:
		return "dimm4_temp"
	default:
		return ""
	}
}

func (p *MotherboardProvider) readDimmTemperatures() []float64 {
	var temps []float64

	// Find all spd5118 hwmon devices (DDR5 SPD Hub temperature sensors)
	hwmonPaths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	for _, namePath := range hwmonPaths {
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		if name == "spd5118" {
			hwmonDir := filepath.Dir(namePath)
			tempPath := filepath.Join(hwmonDir, "temp1_input")
			if data, err := os.ReadFile(tempPath); err == nil {
				if milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
					temps = append(temps, float64(milliC)/1000.0)
				}
			}
		}
	}

	return temps
}

func (p *MotherboardProvider) findMotherboardHwmon() string {
	// Look for common motherboard sensor chips
	hwmonNames := []string{
		"nct6687", "nct6683", "nct6775", "nct6776", "nct6779", "nct6791", "nct6792", "nct6793", "nct6795", "nct6796", "nct6797", "nct6798",
		"it8603", "it8620", "it8622", "it8625", "it8628", "it8655", "it8665", "it8686", "it8688", "it8689", "it8695", "it8705", "it8712", "it8716", "it8718", "it8720", "it8721", "it8726", "it8728", "it8732", "it8771", "it8772", "it8781", "it8782", "it8783", "it8786", "it8790", "it8792",
		"f71858fg", "f71862fg", "f71869", "f71869a", "f71882fg", "f71889ed", "f71889fg",
		"w83627dhg", "w83627ehf", "w83627hf", "w83627thf", "w83667hg", "w83687thf",
		"asus_wmi_sensors", "asus_ec_sensors",
	}

	hwmonPaths, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	for _, namePath := range hwmonPaths {
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		for _, hwmonName := range hwmonNames {
			if strings.Contains(name, hwmonName) {
				return filepath.Dir(namePath)
			}
		}
	}

	return ""
}
