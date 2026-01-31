// Package sensors provides modular sensor data collection.
package sensors

import (
	"reflect"
	"sync"
)

// FieldType represents the type of a sensor field for TypeScript generation.
type FieldType string

const (
	FieldTypeNumber         FieldType = "number"
	FieldTypeOptionalNumber FieldType = "number | undefined"
	FieldTypeString         FieldType = "string"
	FieldTypeOptionalString FieldType = "string | undefined"
	FieldTypeBool           FieldType = "boolean"
)

// FieldDef describes a single field in a sensor's data structure.
type FieldDef struct {
	Name        string    // Go field name (PascalCase)
	JSONName    string    // JSON field name (snake_case)
	TSName      string    // TypeScript field name (camelCase)
	Type        FieldType // TypeScript type
	Unit        string    // Unit for documentation (e.g., "MB", "°C", "%")
	Description string    // Human-readable description
}

// SensorMeta contains metadata about a sensor for code generation.
type SensorMeta struct {
	ID          string     // Unique identifier (e.g., "cpu", "nvidia_gpu")
	Name        string     // Human-readable name
	Description string     // Description for documentation
	Category    string     // Category (e.g., "system", "gpu", "storage")
	Platforms   []string   // Supported platforms: "linux", "darwin", "windows"
	Fields      []FieldDef // Field definitions
	IsArray     bool       // If true, sensor returns array of items (like disks, networks)
	ArrayKey    string     // For arrays, the field to use as map key in TypeScript
}

// OptionDef describes a configuration option for a sensor provider.
type OptionDef struct {
	Key         string // Option key (e.g., "disk.mounts")
	Type        string // Type description: "string", "[]string"
	Default     string // Default value description
	Description string // Human-readable description
	Example     string // Example CLI usage
}

// Provider is the interface that all sensor providers must implement.
type Provider interface {
	// Meta returns the sensor's metadata for registration and type generation.
	Meta() SensorMeta

	// Collect gathers sensor data and returns it as a map.
	// The returned map keys should match the JSONName in FieldDefs.
	Collect(state *CollectorState) map[string]interface{}

	// Available returns true if this sensor can collect data on the current system.
	Available() bool
}

// Configurable is an optional interface for providers that need configuration.
type Configurable interface {
	// Configure applies the given config to the provider.
	Configure(config *Config)
}

// OptionProvider is an optional interface for providers that have configurable options.
type OptionProvider interface {
	// Options returns the list of configuration options this provider supports.
	Options() []OptionDef
}

// CollectorState holds shared state for sensor collection (e.g., previous readings for deltas).
type CollectorState struct {
	mu   sync.Mutex
	data map[string]interface{}
}

// NewCollectorState creates a new collector state.
func NewCollectorState() *CollectorState {
	return &CollectorState{
		data: make(map[string]interface{}),
	}
}

// Get retrieves a value from the state.
func (s *CollectorState) Get(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

// Set stores a value in the state.
func (s *CollectorState) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// GetTyped retrieves a typed value from the state.
func GetTyped[T any](s *CollectorState, key string) (T, bool) {
	v, ok := s.Get(key)
	if !ok {
		var zero T
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}

// Registry holds all registered sensor providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	order     []string // Preserve registration order
}

var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// GlobalRegistry returns the global sensor registry.
func GlobalRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// NewRegistry creates a new sensor registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a sensor provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := p.Meta()
	if _, exists := r.providers[meta.ID]; !exists {
		r.order = append(r.order, meta.ID)
	}
	r.providers[meta.ID] = p
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// All returns all registered providers in registration order.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, 0, len(r.order))
	for _, id := range r.order {
		if p, ok := r.providers[id]; ok {
			result = append(result, p)
		}
	}
	return result
}

// Available returns all providers that are available on the current system.
func (r *Registry) Available() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, 0)
	for _, id := range r.order {
		if p, ok := r.providers[id]; ok && p.Available() {
			result = append(result, p)
		}
	}
	return result
}

// IDs returns all registered sensor IDs.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.order...)
}

// Categories returns a map of category -> sensor IDs.
func (r *Registry) Categories() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)
	for _, id := range r.order {
		if p, ok := r.providers[id]; ok {
			cat := p.Meta().Category
			result[cat] = append(result[cat], id)
		}
	}
	return result
}

// AllOptions returns all configuration options from all registered providers.
func (r *Registry) AllOptions() []OptionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []OptionDef
	for _, id := range r.order {
		if p, ok := r.providers[id]; ok {
			if op, ok := p.(OptionProvider); ok {
				result = append(result, op.Options()...)
			}
		}
	}
	return result
}

// Register adds a provider to the global registry.
func Register(p Provider) {
	GlobalRegistry().Register(p)
}

// Collector collects data from all registered and enabled sensors.
type Collector struct {
	registry *Registry
	state    *CollectorState
	config   *Config
}

// NewCollector creates a new modular collector.
func NewCollector(config *Config) *Collector {
	if config == nil {
		config = DefaultConfig()
	}

	c := &Collector{
		registry: GlobalRegistry(),
		state:    NewCollectorState(),
		config:   config,
	}

	// Configure providers that implement Configurable
	for _, p := range c.registry.All() {
		if configurable, ok := p.(Configurable); ok {
			configurable.Configure(config)
		}
	}

	return c
}

// CollectAll gathers data from all available sensors.
func (c *Collector) CollectAll() map[string]interface{} {
	result := make(map[string]interface{})

	for _, p := range c.registry.Available() {
		meta := p.Meta()

		// Check if sensor is enabled in config
		if !c.isSensorEnabled(meta.ID) {
			continue
		}

		data := p.Collect(c.state)
		if data != nil {
			result[meta.ID] = data
		}
	}

	return result
}

// CollectByID gathers data from a specific sensor.
func (c *Collector) CollectByID(id string) (map[string]interface{}, bool) {
	p, ok := c.registry.Get(id)
	if !ok || !p.Available() {
		return nil, false
	}
	return p.Collect(c.state), true
}

// isSensorEnabled checks if a sensor is enabled in config.
func (c *Collector) isSensorEnabled(id string) bool {
	// Check if sensor is explicitly disabled
	for _, disabled := range c.config.DisabledSensors {
		if disabled == id {
			return false
		}
	}

	// If EnabledSensors is nil, all sensors are enabled by default
	if c.config.EnabledSensors == nil {
		return true
	}

	// Check if sensor is in the enabled map
	enabled, exists := c.config.EnabledSensors[id]
	if exists {
		return enabled
	}

	// Sensor not in map - disabled by default when map is provided
	return false
}

// GenerateTypeScriptTypes generates TypeScript interface definitions from registered sensors.
func (r *Registry) GenerateTypeScriptTypes() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result string

	// Generate interface for each sensor
	for _, id := range r.order {
		p := r.providers[id]
		meta := p.Meta()
		result += generateTSInterface(meta)
	}

	// Generate main SensorData interface
	result += "export interface SensorData {\n"
	for _, id := range r.order {
		p := r.providers[id]
		meta := p.Meta()
		tsKey := toTSKey(meta.ID)
		typeName := toPascalCase(meta.ID) + "Data"

		if meta.IsArray {
			result += "  " + tsKey + "?: Record<string, " + typeName + ">;\n"
		} else {
			result += "  " + tsKey + "?: " + typeName + ";\n"
		}
	}
	result += "  timestamp?: number;\n"
	result += "}\n"

	return result
}

func generateTSInterface(meta SensorMeta) string {
	name := toPascalCase(meta.ID) + "Data"
	result := "/** " + meta.Description + " */\n"
	result += "export interface " + name + " {\n"

	for _, f := range meta.Fields {
		if f.Description != "" {
			result += "  /** " + f.Description
			if f.Unit != "" {
				result += " (" + f.Unit + ")"
			}
			result += " */\n"
		}
		result += "  " + f.TSName + optionalMark(f.Type) + ": " + baseType(f.Type) + ";\n"
	}

	result += "}\n\n"
	return result
}

func optionalMark(t FieldType) string {
	if t == FieldTypeOptionalNumber || t == FieldTypeOptionalString {
		return "?"
	}
	return ""
}

func baseType(t FieldType) string {
	switch t {
	case FieldTypeOptionalNumber:
		return "number"
	case FieldTypeOptionalString:
		return "string"
	default:
		return string(t)
	}
}

func toTSKey(id string) string {
	// Convert sensor ID to TypeScript key (e.g., "nvidia_gpu" -> "nvidiaGpu")
	return toCamelCase(id)
}

func toCamelCase(s string) string {
	result := ""
	capitalizeNext := false

	for i, c := range s {
		if c == '_' || c == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext || (i > 0 && result != "" && capitalizeNext) {
			result += string(toUpper(c))
			capitalizeNext = false
		} else if i == 0 {
			result += string(toLower(c))
		} else {
			result += string(c)
		}
	}

	return result
}

func toUpper(c rune) rune {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

func toLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// StructToFieldDefs extracts FieldDefs from a struct type using reflection.
// This is a helper for creating providers from existing struct types.
func StructToFieldDefs(v interface{}) []FieldDef {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var fields []FieldDef
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}

		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		if jsonTag == "" {
			jsonTag = toSnakeCase(f.Name)
		}

		fields = append(fields, FieldDef{
			Name:     f.Name,
			JSONName: jsonTag,
			TSName:   toCamelCase(jsonTag),
			Type:     goTypeToFieldType(f.Type),
		})
	}
	return fields
}

func goTypeToFieldType(t reflect.Type) FieldType {
	switch t.Kind() {
	case reflect.Ptr:
		inner := goTypeToFieldType(t.Elem())
		if inner == FieldTypeNumber {
			return FieldTypeOptionalNumber
		}
		if inner == FieldTypeString {
			return FieldTypeOptionalString
		}
		return inner
	case reflect.Float32, reflect.Float64, reflect.Int, reflect.Int64, reflect.Uint64:
		return FieldTypeNumber
	case reflect.String:
		return FieldTypeString
	case reflect.Bool:
		return FieldTypeBool
	default:
		return FieldTypeString
	}
}

func toSnakeCase(s string) string {
	var result []rune
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, toLower(c))
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

func toPascalCase(s string) string {
	result := ""
	capitalizeNext := true

	for _, c := range s {
		if c == '_' || c == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result += string(toUpper(c))
			capitalizeNext = false
		} else {
			result += string(c)
		}
	}

	return result
}
