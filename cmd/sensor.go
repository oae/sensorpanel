package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alperen/sensorpanel/pkg/sensors"
	"github.com/spf13/cobra"
)

var sensorCmd = &cobra.Command{
	Use:   "sensor",
	Short: "Manage sensor providers",
	Long:  `Manage modular sensor providers for system monitoring.`,
}

var sensorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered sensors",
	RunE:  runSensorList,
}

var sensorTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "Generate TypeScript types for all sensors",
	Long: `Generate TypeScript interface definitions from all registered sensors.
This updates the theme SDK types to match the current sensor definitions.`,
	RunE: runSensorTypes,
}

var sensorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sensor provider",
	Long: `Interactive wizard to create a new sensor provider.

This will generate a Go source file implementing the sensor Provider interface.
If a sensor with the same ID already exists, you'll be asked which platform
to add the implementation for.`,
	RunE: runSensorCreate,
}

func init() {
	rootCmd.AddCommand(sensorCmd)
	sensorCmd.AddCommand(sensorListCmd)
	sensorCmd.AddCommand(sensorTypesCmd)
	sensorCmd.AddCommand(sensorCreateCmd)

	sensorListCmd.Flags().BoolP("available", "a", false, "Only show available sensors on this system")
	sensorTypesCmd.Flags().StringP("output", "o", "", "Output file path (default: stdout)")
}

func runSensorList(cmd *cobra.Command, args []string) error {
	availableOnly, _ := cmd.Flags().GetBool("available")

	registry := sensors.GlobalRegistry()
	var providerList []sensors.Provider

	if availableOnly {
		providerList = registry.Available()
	} else {
		providerList = registry.All()
	}

	if len(providerList) == 0 {
		fmt.Println("No sensors registered.")
		return nil
	}

	// Group by category
	categories := make(map[string][]sensors.Provider)
	for _, p := range providerList {
		meta := p.Meta()
		categories[meta.Category] = append(categories[meta.Category], p)
	}

	// Sort category names
	categoryNames := make([]string, 0, len(categories))
	for cat := range categories {
		categoryNames = append(categoryNames, cat)
	}
	sort.Strings(categoryNames)

	for _, cat := range categoryNames {
		fmt.Printf("\n[%s]\n", strings.ToUpper(cat))
		for _, p := range categories[cat] {
			meta := p.Meta()
			available := p.Available()

			status := "✓"
			if !available {
				status = "✗"
			}

			platforms := strings.Join(meta.Platforms, ", ")
			fmt.Printf("  %s %-20s %s\n", status, meta.ID, platforms)
			fmt.Printf("    %s\n", meta.Description)
			fmt.Printf("    Fields: %d\n", len(meta.Fields))
		}
	}

	fmt.Println()
	return nil
}

func runSensorTypes(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")

	registry := sensors.GlobalRegistry()
	types := registry.GenerateTypeScriptTypes()

	if output == "" {
		fmt.Println(types)
		return nil
	}

	// Write to file
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(output, []byte(types), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("TypeScript types written to: %s\n", output)
	return nil
}

func runSensorCreate(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Create New Sensor Provider ===")
	fmt.Println()
	fmt.Println("This wizard will help you create a new sensor provider.")
	fmt.Println("The generated code will be placed in pkg/sensors/")
	fmt.Println()

	// Get sensor ID
	fmt.Print("Sensor ID (lowercase, e.g., 'battery', 'intel_gpu'): ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("sensor ID is required")
	}

	// Check if sensor exists
	existingPlatforms := sensors.GetExistingSensorPlatforms(id)
	var platform string

	if len(existingPlatforms) > 0 {
		fmt.Printf("\nSensor '%s' already exists for platforms: %s\n", id, strings.Join(existingPlatforms, ", "))
		fmt.Println("Which platform would you like to add?")
		fmt.Println("  1. linux")
		fmt.Println("  2. darwin (macOS)")
		fmt.Println("  3. windows")
		fmt.Print("Platform [1]: ")

		platChoice, _ := reader.ReadString('\n')
		platChoice = strings.TrimSpace(platChoice)

		switch platChoice {
		case "2":
			platform = "darwin"
		case "3":
			platform = "windows"
		default:
			platform = "linux"
		}

		// Check if this platform already exists
		for _, ep := range existingPlatforms {
			if ep == platform {
				return fmt.Errorf("sensor '%s' already has a %s implementation", id, platform)
			}
		}
	} else {
		// New sensor - ask for platform
		fmt.Println("\nTarget platform (leave empty for all platforms):")
		fmt.Println("  1. All platforms (cross-platform)")
		fmt.Println("  2. linux")
		fmt.Println("  3. darwin (macOS)")
		fmt.Println("  4. windows")
		fmt.Print("Platform [1]: ")

		platChoice, _ := reader.ReadString('\n')
		platChoice = strings.TrimSpace(platChoice)

		switch platChoice {
		case "2":
			platform = "linux"
		case "3":
			platform = "darwin"
		case "4":
			platform = "windows"
		default:
			platform = ""
		}
	}

	// Get name
	defaultName := sensorToPascalCase(id)
	fmt.Printf("Sensor name (human-readable) [%s]: ", defaultName)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultName
	}

	// Get description
	fmt.Print("Description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// Get category
	fmt.Println("\nCategory:")
	fmt.Println("  1. system (CPU, memory, etc.)")
	fmt.Println("  2. gpu")
	fmt.Println("  3. storage")
	fmt.Println("  4. network")
	fmt.Println("  5. power")
	fmt.Println("  6. other")
	fmt.Print("Category [1]: ")

	catChoice, _ := reader.ReadString('\n')
	catChoice = strings.TrimSpace(catChoice)

	var category string
	switch catChoice {
	case "2":
		category = "gpu"
	case "3":
		category = "storage"
	case "4":
		category = "network"
	case "5":
		category = "power"
	case "6":
		category = "other"
	default:
		category = "system"
	}

	// Is it an array sensor?
	fmt.Print("\nDoes this sensor return multiple items (like disks, networks)? [y/N]: ")
	isArrayStr, _ := reader.ReadString('\n')
	isArray := strings.ToLower(strings.TrimSpace(isArrayStr)) == "y"

	var arrayKey string
	if isArray {
		fmt.Print("Key field for array items (e.g., 'mount', 'interface'): ")
		arrayKey, _ = reader.ReadString('\n')
		arrayKey = strings.TrimSpace(arrayKey)
	}

	// Collect fields
	fmt.Println("\n=== Define Fields ===")
	fmt.Println("Enter fields for this sensor. Press Enter with empty name to finish.")
	fmt.Println()

	var fields []sensors.FieldDef
	fieldNum := 1

	for {
		fmt.Printf("Field %d name (PascalCase, e.g., 'Temperature'): ", fieldNum)
		fieldName, _ := reader.ReadString('\n')
		fieldName = strings.TrimSpace(fieldName)

		if fieldName == "" {
			if len(fields) == 0 {
				fmt.Println("At least one field is required.")
				continue
			}
			break
		}

		// JSON name (snake_case)
		defaultJSON := sensorToSnakeCase(fieldName)
		fmt.Printf("  JSON name [%s]: ", defaultJSON)
		jsonName, _ := reader.ReadString('\n')
		jsonName = strings.TrimSpace(jsonName)
		if jsonName == "" {
			jsonName = defaultJSON
		}

		// TypeScript name (camelCase)
		defaultTS := sensorToCamelCase(jsonName)
		fmt.Printf("  TypeScript name [%s]: ", defaultTS)
		tsName, _ := reader.ReadString('\n')
		tsName = strings.TrimSpace(tsName)
		if tsName == "" {
			tsName = defaultTS
		}

		// Type
		fmt.Println("  Type:")
		fmt.Println("    1. number (required)")
		fmt.Println("    2. number (optional)")
		fmt.Println("    3. string (required)")
		fmt.Println("    4. string (optional)")
		fmt.Println("    5. boolean")
		fmt.Print("  Type [1]: ")

		typeChoice, _ := reader.ReadString('\n')
		typeChoice = strings.TrimSpace(typeChoice)

		var fieldType sensors.FieldType
		switch typeChoice {
		case "2":
			fieldType = sensors.FieldTypeOptionalNumber
		case "3":
			fieldType = sensors.FieldTypeString
		case "4":
			fieldType = sensors.FieldTypeOptionalString
		case "5":
			fieldType = sensors.FieldTypeBool
		default:
			fieldType = sensors.FieldTypeNumber
		}

		// Unit
		fmt.Print("  Unit (e.g., '%', '°C', 'MB', or empty): ")
		unit, _ := reader.ReadString('\n')
		unit = strings.TrimSpace(unit)

		// Description
		fmt.Print("  Description: ")
		fieldDesc, _ := reader.ReadString('\n')
		fieldDesc = strings.TrimSpace(fieldDesc)

		fields = append(fields, sensors.FieldDef{
			Name:        fieldName,
			JSONName:    jsonName,
			TSName:      tsName,
			Type:        fieldType,
			Unit:        unit,
			Description: fieldDesc,
		})

		fieldNum++
		fmt.Println()
	}

	// Create spec
	spec := sensors.SensorSpec{
		ID:          id,
		Name:        name,
		Description: description,
		Category:    category,
		Platform:    platform,
		Fields:      fields,
		IsArray:     isArray,
		ArrayKey:    arrayKey,
	}

	// Generate code
	code, err := sensors.GenerateSensorProvider(spec)
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	// Show summary
	fmt.Println("\n=== Summary ===")
	fmt.Printf("  ID:          %s\n", spec.ID)
	fmt.Printf("  Name:        %s\n", spec.Name)
	fmt.Printf("  Description: %s\n", spec.Description)
	fmt.Printf("  Category:    %s\n", spec.Category)
	fmt.Printf("  Platform:    %s\n", platformDisplay(spec.Platform))
	fmt.Printf("  Fields:      %d\n", len(spec.Fields))
	fmt.Printf("  Is Array:    %v\n", spec.IsArray)
	if spec.IsArray {
		fmt.Printf("  Array Key:   %s\n", spec.ArrayKey)
	}
	fmt.Printf("  File:        pkg/sensors/%s\n", spec.FileName())
	fmt.Println()

	// Confirm
	fmt.Print("Generate this sensor provider? [Y/n]: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "n" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Write file
	outputPath := filepath.Join("pkg", "sensors", spec.FileName())
	if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("\n✓ Created: %s\n", outputPath)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Open the file and implement the Collect() method")
	fmt.Println("  2. Implement the Available() check if needed")
	fmt.Println("  3. Run 'go build ./...' to verify compilation")
	fmt.Println("  4. Run 'sensorpanel sensor list' to see your sensor")
	fmt.Println("  5. Run 'sensorpanel sensor types' to generate updated TypeScript types")

	return nil
}

func platformDisplay(platform string) string {
	if platform == "" {
		return "all platforms"
	}
	return platform
}

func sensorToPascalCase(s string) string {
	result := ""
	capitalizeNext := true

	for _, c := range s {
		if c == '_' || c == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32)
			} else {
				result += string(c)
			}
			capitalizeNext = false
		} else {
			result += string(c)
		}
	}

	return result
}

func sensorToSnakeCase(s string) string {
	var result []rune
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, c+32)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

func sensorToCamelCase(s string) string {
	result := ""
	capitalizeNext := false

	for i, c := range s {
		if c == '_' || c == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32)
			} else {
				result += string(c)
			}
			capitalizeNext = false
		} else if i == 0 {
			if c >= 'A' && c <= 'Z' {
				result += string(c + 32)
			} else {
				result += string(c)
			}
		} else {
			result += string(c)
		}
	}

	return result
}
