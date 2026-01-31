// cmd/device.go - USB device management commands
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alperen/sensorpanel/pkg/config"
	"github.com/alperen/sensorpanel/pkg/device"
	"github.com/alperen/sensorpanel/pkg/panel"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Manage USB display device selection",
	Long: `Commands for discovering, selecting, and managing USB display devices.

The selected device is saved to the config file and used by all other commands.
Config location: ~/.config/sensorpanel/config.json (or $XDG_CONFIG_HOME/sensorpanel/)`,
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available USB display devices",
	Long: `Scan the USB bus for potential display devices.

By default, only shows devices known to be display panels.
Use --all to show all USB devices with bulk endpoints that might be displays.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")

		devices, err := config.DiscoverDevices(!showAll)
		if err != nil {
			return fmt.Errorf("failed to scan USB devices: %w", err)
		}

		if len(devices) == 0 {
			fmt.Println("No display devices found.")
			if !showAll {
				fmt.Println("Try 'sensorpanel device list --all' to see all potential devices.")
			}
			return nil
		}

		fmt.Println("=== USB Display Devices ===")
		for i, dev := range devices {
			marker := " "
			if dev.IsProbable {
				marker = "*"
			}
			fmt.Printf("%s [%d] %s\n", marker, i+1, dev.String())

			// Show profile info if known
			if profile := device.FindByVIDPID(dev.VendorID, dev.ProductID); profile != nil {
				fmt.Printf("       Profile: %s (%dx%d)\n", profile.Name(), profile.Width(), profile.Height())
			}

			if dev.Speed != "" {
				fmt.Printf("       Speed: %s, Bus:Addr: %s\n", dev.Speed, dev.BusAddr)
			}
		}

		fmt.Println()
		fmt.Println("* = Known/probable display device")
		fmt.Println("Use 'sensorpanel device select <number>' to select a device.")

		return nil
	},
}

var deviceSelectCmd = &cobra.Command{
	Use:   "select [number]",
	Short: "Select a USB display device",
	Long: `Select a USB display device from the list.

If a number is provided, selects that device from the list.
If no number is provided, shows an interactive menu.

The selection is saved to the config file and used by all other commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get list of devices
		devices, err := config.DiscoverDevices(false) // Known devices first
		if err != nil {
			return fmt.Errorf("failed to scan USB devices: %w", err)
		}

		// If no known devices, try all probable devices
		if len(devices) == 0 {
			devices, err = config.DiscoverDevices(false)
			if err != nil {
				return fmt.Errorf("failed to scan USB devices: %w", err)
			}
			// Filter to probable only
			var probable []config.DiscoveredDevice
			for _, d := range devices {
				if d.IsProbable {
					probable = append(probable, d)
				}
			}
			devices = probable
		}

		if len(devices) == 0 {
			fmt.Println("No display devices found.")
			fmt.Println("Make sure the USB display is connected.")
			return nil
		}

		var selectedIdx int

		if len(args) > 0 {
			// Number provided as argument
			idx, err := strconv.Atoi(args[0])
			if err != nil || idx < 1 || idx > len(devices) {
				return fmt.Errorf("invalid device number: %s (must be 1-%d)", args[0], len(devices))
			}
			selectedIdx = idx - 1
		} else {
			// Interactive selection
			fmt.Println("=== Select USB Display Device ===")
			for i, dev := range devices {
				marker := " "
				if dev.IsProbable {
					marker = "*"
				}
				fmt.Printf("%s [%d] %s\n", marker, i+1, dev.String())
			}
			fmt.Println()
			fmt.Print("Enter device number (or 'q' to cancel): ")

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			input = strings.TrimSpace(input)
			if input == "q" || input == "" {
				fmt.Println("Cancelled.")
				return nil
			}

			idx, err := strconv.Atoi(input)
			if err != nil || idx < 1 || idx > len(devices) {
				return fmt.Errorf("invalid device number: %s", input)
			}
			selectedIdx = idx - 1
		}

		// Save selection to config
		dev := devices[selectedIdx]
		if err := config.SetDevice(dev.VendorID, dev.ProductID, dev.Serial); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Selected device: %s\n", dev.String())

		configPath, _ := config.ConfigPath()
		fmt.Printf("Config saved to: %s\n", configPath)

		// Check if we have permission to access the device
		fmt.Print("Checking device access... ")
		if err := panel.CheckDeviceAccess(dev.VendorID, dev.ProductID); err != nil {
			fmt.Println("FAILED")
			fmt.Println()
			if errors.Is(err, panel.ErrPermissionDenied) {
				fmt.Println(panel.PermissionFixInstructions(dev.VendorID, dev.ProductID))
			} else if errors.Is(err, panel.ErrDeviceBusy) {
				fmt.Println(panel.DeviceBusyInstructions())
			} else {
				fmt.Printf("Error: %v\n", err)
			}
			return nil // Don't return error - config was saved successfully
		}
		fmt.Println("OK")

		return nil
	},
}

var deviceInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show current device configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		configPath, _ := config.ConfigPath()

		fmt.Println("=== Device Configuration ===")
		fmt.Printf("Config file:  %s\n", configPath)
		fmt.Println()

		if cfg.Device.IsZero() {
			fmt.Println("No device configured.")
			fmt.Println("Run 'sensorpanel device select' to choose a device.")
		} else {
			fmt.Printf("VID:PID:      %04x:%04x\n", cfg.Device.VendorID, cfg.Device.ProductID)
			if cfg.Device.Serial != "" {
				fmt.Printf("Serial:       %s\n", cfg.Device.Serial)
			}

			// Show profile info
			if profile := device.FindByVIDPID(cfg.Device.VendorID, cfg.Device.ProductID); profile != nil {
				fmt.Printf("Profile:      %s (%s)\n", profile.Name(), profile.ID())
				fmt.Printf("Resolution:   %dx%d\n", profile.Width(), profile.Height())
				fmt.Printf("Color Format: %s\n", profile.ColorFormat())
			} else {
				fmt.Println("Profile:      Generic (unknown device)")
			}
		}

		fmt.Println()
		fmt.Println("=== Other Settings ===")
		fmt.Printf("Brightness:   %d\n", cfg.Brightness)
		fmt.Printf("Interval:     %.1fs\n", cfg.UpdateInterval)
		fmt.Printf("Sensor Opts:  %v\n", cfg.SensorOptions)

		// Check if configured device is connected
		fmt.Println()
		fmt.Print("Device status: ")
		if dev, err := config.FindConfiguredDevice(); err == nil {
			fmt.Printf("Connected (%s %s)\n", dev.Manufacturer, dev.Product)
		} else {
			fmt.Println("Not connected")
		}

		return nil
	},
}

var deviceResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset device configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("Configuration reset to defaults.")
		fmt.Println("No device configured. Run 'sensorpanel device select' to choose a device.")

		return nil
	},
}

var deviceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new device profile (for developers)",
	Long: `Interactive wizard to generate a skeleton device profile.

This command helps you create a new device profile for an unsupported USB display.
It will prompt you for device information and generate a Go source file that you
can complete by implementing the protocol methods.

The generated file will be saved to pkg/device/<id>.go and you'll need to:
1. Implement BlitCommand() based on USB traffic analysis
2. Implement BacklightCommand() if the device supports backlight control
3. Register the profile in pkg/device/registry.go
4. Rebuild sensorpanel

See docs/adding-devices.md for detailed instructions on protocol research.`,
	RunE: runDeviceCreate,
}

func runDeviceCreate(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== New Device Profile Wizard ===")
	fmt.Println()
	fmt.Println("This wizard will help you create a skeleton device profile.")
	fmt.Println("You'll need to implement the protocol methods afterward.")
	fmt.Println()

	// Device ID
	fmt.Print("Device ID (lowercase, e.g., 'my-device'): ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("device ID is required")
	}

	// Device Name
	fmt.Print("Device Name (human-readable, e.g., 'My USB Display'): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}

	// Description
	fmt.Print("Description (brief, e.g., '480x320 LCD with RGB565'): ")
	desc, _ := reader.ReadString('\n')
	desc = strings.TrimSpace(desc)

	// Vendor ID
	fmt.Print("Vendor ID (hex, e.g., '1908' or '0x1908'): ")
	vidStr, _ := reader.ReadString('\n')
	vid, err := parseHex16(strings.TrimSpace(vidStr))
	if err != nil {
		return fmt.Errorf("invalid vendor ID: %w", err)
	}

	// Product ID
	fmt.Print("Product ID (hex, e.g., '0102' or '0x0102'): ")
	pidStr, _ := reader.ReadString('\n')
	pid, err := parseHex16(strings.TrimSpace(pidStr))
	if err != nil {
		return fmt.Errorf("invalid product ID: %w", err)
	}

	// Width
	fmt.Print("Display Width (pixels, e.g., '480'): ")
	widthStr, _ := reader.ReadString('\n')
	width, err := strconv.Atoi(strings.TrimSpace(widthStr))
	if err != nil || width <= 0 {
		return fmt.Errorf("invalid width: must be a positive number")
	}

	// Height
	fmt.Print("Display Height (pixels, e.g., '320'): ")
	heightStr, _ := reader.ReadString('\n')
	height, err := strconv.Atoi(strings.TrimSpace(heightStr))
	if err != nil || height <= 0 {
		return fmt.Errorf("invalid height: must be a positive number")
	}

	// Color Format
	fmt.Print("Color Format (1=RGB565 16-bit, 2=RGB888 24-bit) [1]: ")
	cfStr, _ := reader.ReadString('\n')
	cfStr = strings.TrimSpace(cfStr)
	colorFormat := device.RGB565
	if cfStr == "2" {
		colorFormat = device.RGB888
	}

	// Byte Order
	fmt.Print("Byte Order (1=Big-endian, 2=Little-endian) [1]: ")
	boStr, _ := reader.ReadString('\n')
	boStr = strings.TrimSpace(boStr)
	byteOrder := device.BigEndian
	if boStr == "2" {
		byteOrder = device.LittleEndian
	}

	// Max Brightness
	fmt.Print("Max Brightness Level (0=no backlight, 7=typical, 255=8-bit) [7]: ")
	brightStr, _ := reader.ReadString('\n')
	brightStr = strings.TrimSpace(brightStr)
	maxBrightness := 7
	if brightStr != "" {
		maxBrightness, _ = strconv.Atoi(brightStr)
	}

	// Protocol Type
	fmt.Print("Protocol Type (1=SCSI/CBW, 2=Raw Bulk) [1]: ")
	ptStr, _ := reader.ReadString('\n')
	ptStr = strings.TrimSpace(ptStr)
	protocolType := device.ProtocolSCSI
	if ptStr == "2" {
		protocolType = device.ProtocolBulk
	}

	// Create spec
	spec := device.DeviceSpec{
		ID:            id,
		Name:          name,
		Description:   desc,
		VendorID:      vid,
		ProductID:     pid,
		Width:         width,
		Height:        height,
		ColorFormat:   colorFormat,
		ByteOrder:     byteOrder,
		MaxBrightness: maxBrightness,
		ProtocolType:  protocolType,
	}

	// Generate profile
	code, err := device.GenerateProfile(spec)
	if err != nil {
		return fmt.Errorf("failed to generate profile: %w", err)
	}

	// Show summary
	fmt.Println()
	fmt.Println("=== Profile Summary ===")
	fmt.Printf("ID:           %s\n", id)
	fmt.Printf("Name:         %s\n", name)
	fmt.Printf("VID:PID:      %04X:%04X\n", vid, pid)
	fmt.Printf("Resolution:   %dx%d\n", width, height)
	fmt.Printf("Color Format: %s\n", colorFormat)
	fmt.Printf("Byte Order:   %s\n", byteOrder)
	fmt.Printf("Buffer Size:  %d bytes\n", spec.BufferSize())
	fmt.Println()

	// Output path
	outputPath := fmt.Sprintf("pkg/device/%s.go", id)
	fmt.Printf("Output file: %s\n", outputPath)
	fmt.Print("Generate file? (y/n) [y]: ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "y" && confirm != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Println()
	fmt.Printf("Generated: %s\n", outputPath)
	fmt.Println()
	fmt.Println("=== Next Steps ===")
	fmt.Println("1. Open the generated file and implement BlitCommand()")
	fmt.Println("   - Use Wireshark/USBPcap to capture USB traffic from manufacturer software")
	fmt.Println("   - See docs/adding-devices.md for detailed protocol research guide")
	fmt.Println()
	fmt.Println("2. Implement BacklightCommand() if your device has backlight control")
	fmt.Println()
	fmt.Println("3. Register your profile in pkg/device/registry.go:")
	fmt.Printf("   Register(&%sProfile{})\n", toPascalCase(id))
	fmt.Println()
	fmt.Println("4. Rebuild: go build .")
	fmt.Println()
	fmt.Println("5. Test: ./sensorpanel panel test")
	fmt.Println()
	fmt.Println("6. Submit a PR to share your profile with the community!")

	return nil
}

func parseHex16(s string) (uint16, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	val, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func toPascalCase(s string) string {
	var result strings.Builder
	capitalizeNext := true

	for _, r := range s {
		if r == '_' || r == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(rune(strings.ToUpper(string(r))[0]))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

func init() {
	rootCmd.AddCommand(deviceCmd)

	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceSelectCmd)
	deviceCmd.AddCommand(deviceInfoCmd)
	deviceCmd.AddCommand(deviceResetCmd)
	deviceCmd.AddCommand(deviceCreateCmd)

	deviceListCmd.Flags().BoolP("all", "a", false, "Show all USB devices (not just known displays)")
}
