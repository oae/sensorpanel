package sensors

import (
	"bytes"
	"fmt"
	"text/template"
)

// SensorSpec holds the specification for generating a new sensor provider.
type SensorSpec struct {
	ID          string     // Unique identifier (e.g., "battery")
	Name        string     // Human-readable name (e.g., "Battery")
	Description string     // Description
	Category    string     // Category (e.g., "power", "system")
	Platform    string     // Target platform: "linux", "darwin", "windows", or "" for all
	Fields      []FieldDef // Field definitions
	IsArray     bool       // Whether this sensor returns multiple items
	ArrayKey    string     // For arrays, the key field
}

// Validate checks if the spec is valid.
func (s *SensorSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("sensor ID is required")
	}
	if !isValidIdentifier(s.ID) {
		return fmt.Errorf("sensor ID must be a valid identifier (lowercase letters, numbers, underscores)")
	}
	if s.Name == "" {
		return fmt.Errorf("sensor name is required")
	}
	if len(s.Fields) == 0 {
		return fmt.Errorf("at least one field is required")
	}
	if s.IsArray && s.ArrayKey == "" {
		return fmt.Errorf("array sensors require an array key field")
	}
	if s.Platform != "" && s.Platform != "linux" && s.Platform != "darwin" && s.Platform != "windows" {
		return fmt.Errorf("platform must be 'linux', 'darwin', 'windows', or empty for all")
	}
	return nil
}

// StructName returns the Go struct name for this sensor.
func (s *SensorSpec) StructName() string {
	return toPascalCase(s.ID) + "Provider"
}

// FileName returns the file name for this sensor.
func (s *SensorSpec) FileName() string {
	if s.Platform != "" {
		return s.ID + "_" + s.Platform + ".go"
	}
	return s.ID + ".go"
}

// BuildTag returns the build tag for this sensor.
func (s *SensorSpec) BuildTag() string {
	if s.Platform != "" {
		return "//go:build " + s.Platform
	}
	return ""
}

func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

// GenerateSensorProvider generates Go source code for a sensor provider.
func GenerateSensorProvider(spec SensorSpec) (string, error) {
	if err := spec.Validate(); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	tmpl, err := template.New("sensor").Parse(sensorTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, &spec); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

const sensorTemplate = `{{if .BuildTag}}{{.BuildTag}}

{{end}}package sensors

import (
	"runtime"
)

func init() {
	Register(&{{.StructName}}{})
}

// {{.StructName}} provides {{.Name}} sensor data.
type {{.StructName}} struct{}

// Meta returns the sensor metadata.
func (p *{{.StructName}}) Meta() SensorMeta {
	return SensorMeta{
		ID:          "{{.ID}}",
		Name:        "{{.Name}}",
		Description: "{{.Description}}",
		Category:    "{{.Category}}",
		Platforms:   []string{ {{- if .Platform}}"{{.Platform}}"{{else}}"linux", "darwin", "windows"{{end -}} },
		IsArray:     {{.IsArray}},
		ArrayKey:    "{{.ArrayKey}}",
		Fields: []FieldDef{
			{{- range .Fields}}
			{
				Name:        "{{.Name}}",
				JSONName:    "{{.JSONName}}",
				TSName:      "{{.TSName}}",
				Type:        {{printf "%#v" .Type}},
				Unit:        "{{.Unit}}",
				Description: "{{.Description}}",
			},
			{{- end}}
		},
	}
}

// Available returns true if this sensor can collect data on the current system.
func (p *{{.StructName}}) Available() bool {
	{{- if .Platform}}
	return runtime.GOOS == "{{.Platform}}"
	{{- else}}
	// TODO: Implement availability check
	return true
	{{- end}}
}

// Collect gathers sensor data.
func (p *{{.StructName}}) Collect(state *CollectorState) map[string]interface{} {
	// TODO: Implement data collection
	//
	// Return a map with keys matching the JSONName in Fields:
	{{- range .Fields}}
	// - "{{.JSONName}}": {{.Description}} ({{.Unit}})
	{{- end}}
	//
	// Use state.Get/Set to store values between collections for delta calculations.
	// Example:
	//   prevValue, _ := GetTyped[float64](state, "{{.ID}}_prev")
	//   state.Set("{{.ID}}_prev", currentValue)
	//   delta := currentValue - prevValue
	
	return map[string]interface{}{
		{{- range .Fields}}
		"{{.JSONName}}": nil, // TODO: Replace with actual value
		{{- end}}
	}
}
`

// GetExistingSensorPlatforms returns which platforms have implementations for a sensor ID.
func GetExistingSensorPlatforms(id string) []string {
	registry := GlobalRegistry()
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	if p, ok := registry.providers[id]; ok {
		return p.Meta().Platforms
	}
	return nil
}

// SensorExists checks if a sensor with the given ID exists.
func SensorExists(id string) bool {
	_, ok := GlobalRegistry().Get(id)
	return ok
}
