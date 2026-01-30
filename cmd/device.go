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
		}

		fmt.Println()
		fmt.Println("=== Other Settings ===")
		fmt.Printf("Brightness:   %d\n", cfg.Brightness)
		fmt.Printf("Interval:     %.1fs\n", cfg.UpdateInterval)
		fmt.Printf("Disk Mounts:  %v\n", cfg.DiskMounts)

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

func init() {
	rootCmd.AddCommand(deviceCmd)

	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceSelectCmd)
	deviceCmd.AddCommand(deviceInfoCmd)
	deviceCmd.AddCommand(deviceResetCmd)

	deviceListCmd.Flags().BoolP("all", "a", false, "Show all USB devices (not just known displays)")
}
