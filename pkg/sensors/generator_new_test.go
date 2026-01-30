package sensors

import (
	"testing"
)

func TestSensorSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    SensorSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			spec: SensorSpec{
				Name:     "Test",
				Category: "system",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid ID",
			spec: SensorSpec{
				ID:       "Invalid-ID",
				Name:     "Test",
				Category: "system",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			spec: SensorSpec{
				ID:       "test",
				Category: "system",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: true,
		},
		{
			name: "no fields",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				Fields:   []FieldDef{},
			},
			wantErr: true,
		},
		{
			name: "array without key",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				IsArray:  true,
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: true,
		},
		{
			name: "array with key",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				IsArray:  true,
				ArrayKey: "id",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid platform",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				Platform: "invalid",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: true,
		},
		{
			name: "valid linux platform",
			spec: SensorSpec{
				ID:       "test",
				Name:     "Test",
				Category: "system",
				Platform: "linux",
				Fields: []FieldDef{
					{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSensorSpec_StructName(t *testing.T) {
	spec := SensorSpec{ID: "test_sensor"}
	if got := spec.StructName(); got != "TestSensorProvider" {
		t.Errorf("StructName() = %q, want 'TestSensorProvider'", got)
	}
}

func TestSensorSpec_FileName(t *testing.T) {
	tests := []struct {
		id       string
		platform string
		want     string
	}{
		{"test", "", "test.go"},
		{"test", "linux", "test_linux.go"},
		{"test", "darwin", "test_darwin.go"},
		{"test", "windows", "test_windows.go"},
	}

	for _, tt := range tests {
		spec := SensorSpec{ID: tt.id, Platform: tt.platform}
		if got := spec.FileName(); got != tt.want {
			t.Errorf("FileName(%q, %q) = %q, want %q", tt.id, tt.platform, got, tt.want)
		}
	}
}

func TestSensorSpec_BuildTag(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"", ""},
		{"linux", "//go:build linux"},
		{"darwin", "//go:build darwin"},
		{"windows", "//go:build windows"},
	}

	for _, tt := range tests {
		spec := SensorSpec{Platform: tt.platform}
		if got := spec.BuildTag(); got != tt.want {
			t.Errorf("BuildTag(%q) = %q, want %q", tt.platform, got, tt.want)
		}
	}
}

func TestGenerateSensorProvider(t *testing.T) {
	spec := SensorSpec{
		ID:          "test",
		Name:        "Test Sensor",
		Description: "A test sensor",
		Category:    "test",
		Platform:    "linux",
		Fields: []FieldDef{
			{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber, Unit: "%", Description: "A value"},
			{Name: "Name", JSONName: "name", TSName: "name", Type: FieldTypeOptionalString, Description: "Optional name"},
		},
	}

	code, err := GenerateSensorProvider(spec)
	if err != nil {
		t.Fatalf("GenerateSensorProvider() error = %v", err)
	}

	// Check for required elements
	checks := []string{
		"//go:build linux",
		"package sensors",
		"func init()",
		"Register(&TestProvider{})",
		"type TestProvider struct{}",
		"func (p *TestProvider) Meta() SensorMeta",
		`ID:          "test"`,
		`Name:        "Test Sensor"`,
		"func (p *TestProvider) Available() bool",
		"func (p *TestProvider) Collect(state *CollectorState)",
	}

	for _, check := range checks {
		if !containsHelper(code, check) {
			t.Errorf("Generated code missing: %q", check)
		}
	}
}

func TestGenerateSensorProvider_Invalid(t *testing.T) {
	spec := SensorSpec{
		// Missing required fields
	}

	_, err := GenerateSensorProvider(spec)
	if err == nil {
		t.Error("GenerateSensorProvider() expected error for invalid spec")
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"test", true},
		{"test_sensor", true},
		{"test123", true},
		{"_test", true},
		{"", false},
		{"Test", false},        // uppercase
		{"test-sensor", false}, // hyphen
		{"123test", false},     // starts with number
		{"test sensor", false}, // space
	}

	for _, tt := range tests {
		if got := isValidIdentifier(tt.input); got != tt.want {
			t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSensorExists(t *testing.T) {
	// Should find sensors registered by init()
	if !SensorExists("cpu") {
		t.Error("SensorExists(cpu) = false, expected true")
	}

	if SensorExists("nonexistent") {
		t.Error("SensorExists(nonexistent) = true, expected false")
	}
}

func TestGetExistingSensorPlatforms(t *testing.T) {
	// CPU should exist
	platforms := GetExistingSensorPlatforms("cpu")
	if len(platforms) == 0 {
		t.Error("GetExistingSensorPlatforms(cpu) returned empty")
	}

	// Nonexistent should return nil
	platforms = GetExistingSensorPlatforms("nonexistent")
	if platforms != nil {
		t.Errorf("GetExistingSensorPlatforms(nonexistent) = %v, want nil", platforms)
	}
}
