package sensors

import (
	"testing"
)

func TestFieldType_String(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldTypeNumber, "number"},
		{FieldTypeOptionalNumber, "number | undefined"},
		{FieldTypeString, "string"},
		{FieldTypeOptionalString, "string | undefined"},
		{FieldTypeBool, "boolean"},
	}

	for _, tt := range tests {
		if string(tt.ft) != tt.want {
			t.Errorf("FieldType %v: got %q, want %q", tt.ft, string(tt.ft), tt.want)
		}
	}
}

func TestCollectorState(t *testing.T) {
	state := NewCollectorState()

	// Test Set and Get
	state.Set("key1", "value1")
	state.Set("key2", 42)

	v1, ok := state.Get("key1")
	if !ok || v1 != "value1" {
		t.Errorf("Get key1: got %v, ok=%v", v1, ok)
	}

	v2, ok := state.Get("key2")
	if !ok || v2 != 42 {
		t.Errorf("Get key2: got %v, ok=%v", v2, ok)
	}

	// Test missing key
	_, ok = state.Get("missing")
	if ok {
		t.Error("Get missing: expected ok=false")
	}
}

func TestGetTyped(t *testing.T) {
	state := NewCollectorState()
	state.Set("int", 42)
	state.Set("string", "hello")
	state.Set("float", 3.14)

	// Test typed get
	i, ok := GetTyped[int](state, "int")
	if !ok || i != 42 {
		t.Errorf("GetTyped int: got %v, ok=%v", i, ok)
	}

	s, ok := GetTyped[string](state, "string")
	if !ok || s != "hello" {
		t.Errorf("GetTyped string: got %v, ok=%v", s, ok)
	}

	f, ok := GetTyped[float64](state, "float")
	if !ok || f != 3.14 {
		t.Errorf("GetTyped float: got %v, ok=%v", f, ok)
	}

	// Test missing key
	_, ok = GetTyped[int](state, "missing")
	if ok {
		t.Error("GetTyped missing: expected ok=false")
	}

	// Test wrong type
	_, ok = GetTyped[int](state, "string")
	if ok {
		t.Error("GetTyped wrong type: expected ok=false")
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Create a mock provider
	mock := &mockProvider{
		meta: SensorMeta{
			ID:          "test_sensor",
			Name:        "Test Sensor",
			Description: "A test sensor",
			Category:    "test",
			Platforms:   []string{"linux"},
		},
		available: true,
	}

	// Register
	reg.Register(mock)

	// Get
	p, ok := reg.Get("test_sensor")
	if !ok {
		t.Fatal("Get: expected to find test_sensor")
	}
	if p.Meta().ID != "test_sensor" {
		t.Errorf("Get: got ID %q, want 'test_sensor'", p.Meta().ID)
	}

	// All
	all := reg.All()
	if len(all) != 1 {
		t.Errorf("All: got %d providers, want 1", len(all))
	}

	// Available
	available := reg.Available()
	if len(available) != 1 {
		t.Errorf("Available: got %d providers, want 1", len(available))
	}

	// IDs
	ids := reg.IDs()
	if len(ids) != 1 || ids[0] != "test_sensor" {
		t.Errorf("IDs: got %v, want ['test_sensor']", ids)
	}

	// Categories
	cats := reg.Categories()
	if len(cats) != 1 || len(cats["test"]) != 1 {
		t.Errorf("Categories: got %v", cats)
	}
}

func TestRegistry_Available(t *testing.T) {
	reg := NewRegistry()

	available := &mockProvider{
		meta:      SensorMeta{ID: "available", Category: "test"},
		available: true,
	}
	unavailable := &mockProvider{
		meta:      SensorMeta{ID: "unavailable", Category: "test"},
		available: false,
	}

	reg.Register(available)
	reg.Register(unavailable)

	avail := reg.Available()
	if len(avail) != 1 {
		t.Errorf("Available: got %d, want 1", len(avail))
	}
	if len(avail) > 0 && avail[0].Meta().ID != "available" {
		t.Errorf("Available: got wrong provider")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Global registry should be initialized
	reg := GlobalRegistry()
	if reg == nil {
		t.Fatal("GlobalRegistry returned nil")
	}

	// Should return same instance
	reg2 := GlobalRegistry()
	if reg != reg2 {
		t.Error("GlobalRegistry should return same instance")
	}

	// Should have providers registered (from init functions)
	all := reg.All()
	if len(all) == 0 {
		t.Error("GlobalRegistry should have providers from init()")
	}
}

func TestGenerateTypeScriptTypes(t *testing.T) {
	reg := NewRegistry()

	mock := &mockProvider{
		meta: SensorMeta{
			ID:          "test",
			Name:        "Test",
			Description: "Test sensor",
			Category:    "test",
			Fields: []FieldDef{
				{Name: "Value", JSONName: "value", TSName: "value", Type: FieldTypeNumber, Unit: "%", Description: "A value"},
				{Name: "Name", JSONName: "name", TSName: "name", Type: FieldTypeOptionalString, Description: "A name"},
			},
		},
	}

	reg.Register(mock)

	ts := reg.GenerateTypeScriptTypes()

	// Check for interface
	if !contains(ts, "export interface TestData") {
		t.Error("Missing TestData interface")
	}

	// Check for fields
	if !contains(ts, "value: number") {
		t.Error("Missing value field")
	}
	if !contains(ts, "name?: string") {
		t.Error("Missing optional name field")
	}

	// Check for SensorData
	if !contains(ts, "export interface SensorData") {
		t.Error("Missing SensorData interface")
	}
	if !contains(ts, "test?: TestData") {
		t.Error("Missing test field in SensorData")
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"hello_world", "HelloWorld"},
		{"nvidia_gpu", "NvidiaGpu"},
		{"amd-gpu", "AmdGpu"},
		{"CPU", "CPU"},
	}

	for _, tt := range tests {
		got := toPascalCase(tt.input)
		if got != tt.want {
			t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello_world", "helloWorld"},
		{"nvidia_gpu", "nvidiaGpu"},
		{"rx_rate", "rxRate"},
	}

	for _, tt := range tests {
		got := toCamelCase(tt.input)
		if got != tt.want {
			t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"HelloWorld", "hello_world"},
		{"NvidiaGPU", "nvidia_g_p_u"},
		{"CPULoad", "c_p_u_load"},
	}

	for _, tt := range tests {
		got := toSnakeCase(tt.input)
		if got != tt.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// mockProvider is a test implementation of Provider
type mockProvider struct {
	meta      SensorMeta
	available bool
	data      map[string]interface{}
}

func (m *mockProvider) Meta() SensorMeta {
	return m.meta
}

func (m *mockProvider) Available() bool {
	return m.available
}

func (m *mockProvider) Collect(state *CollectorState) map[string]interface{} {
	return m.data
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
