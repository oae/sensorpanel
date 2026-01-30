package cmd

import (
	"fmt"
	"strconv"

	"github.com/alperen/sensorpanel/pkg/config"
	"github.com/alperen/sensorpanel/pkg/panel"
	"github.com/spf13/cobra"
)

// openConfiguredDevice creates and opens a device using the saved config.
// Returns an error if no device is configured.
func openConfiguredDevice() (*panel.Device, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Device.IsZero() {
		return nil, fmt.Errorf("no device configured - run 'sensorpanel device select' first")
	}

	var dev *panel.Device
	if cfg.Device.Serial != "" {
		dev = panel.NewDeviceWithSerial(cfg.Device.VendorID, cfg.Device.ProductID, cfg.Device.Serial)
	} else {
		dev = panel.NewDeviceWithID(cfg.Device.VendorID, cfg.Device.ProductID)
	}

	if err := dev.Open(); err != nil {
		return nil, err
	}

	return dev, nil
}

var panelCmd = &cobra.Command{
	Use:   "panel",
	Short: "Control the USB display panel",
	Long:  `Commands for controlling the AX206 USB display panel hardware.`,
}

var panelStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if the panel is connected",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if cfg.Device.IsZero() {
			fmt.Println("No device configured.")
			fmt.Println("Run 'sensorpanel device select' to select a device.")
			return nil
		}

		vid := cfg.Device.VendorID
		pid := cfg.Device.ProductID

		info, err := panel.GetDeviceInfo(vid, pid)
		if err != nil {
			fmt.Println("Panel not connected")
			fmt.Printf("Looking for device %04X:%04X\n", vid, pid)
			if cfg.Device.Serial != "" {
				fmt.Printf("Serial: %s\n", cfg.Device.Serial)
			}
			return nil
		}

		fmt.Println("=== Panel Status ===")
		fmt.Printf("Connected:    Yes\n")
		fmt.Printf("Manufacturer: %s\n", info.Manufacturer)
		fmt.Printf("Product:      %s\n", info.Product)
		fmt.Printf("Serial:       %s\n", info.Serial)
		fmt.Printf("VID:PID:      %04X:%04X\n", info.VendorID, info.ProductID)
		fmt.Printf("USB Speed:    %s\n", info.Speed)
		fmt.Printf("Max Packet:   %d bytes\n", info.MaxPacketSize)
		fmt.Printf("Resolution:   %dx%d\n", info.Width, info.Height)
		fmt.Printf("Buffer Size:  %d bytes\n", info.BufferSize)
		return nil
	},
}

var panelOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Turn the panel backlight on (max brightness)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		if err := dev.BacklightOn(); err != nil {
			return fmt.Errorf("failed to turn on backlight: %w", err)
		}

		fmt.Println("Backlight turned on (level 7)")
		return nil
	},
}

var panelOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Turn the panel backlight off",
	RunE: func(cmd *cobra.Command, args []string) error {
		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		if err := dev.BacklightOff(); err != nil {
			return fmt.Errorf("failed to turn off backlight: %w", err)
		}

		fmt.Println("Backlight turned off")
		return nil
	},
}

var panelBrightnessCmd = &cobra.Command{
	Use:   "brightness <level>",
	Short: "Set the panel backlight brightness (0-7)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		level, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid brightness level: %s", args[0])
		}
		if level < 0 || level > 7 {
			return fmt.Errorf("brightness level must be 0-7, got %d", level)
		}

		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		if err := dev.SetBacklight(level); err != nil {
			return fmt.Errorf("failed to set brightness: %w", err)
		}

		fmt.Printf("Backlight set to level %d\n", level)
		return nil
	},
}

var panelTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Display a test pattern on the panel",
	RunE: func(cmd *cobra.Command, args []string) error {
		dev, err := openConfiguredDevice()
		if err != nil {
			return fmt.Errorf("failed to open device: %w", err)
		}
		defer dev.Close()

		pattern, _ := cmd.Flags().GetString("pattern")

		switch pattern {
		case "quadrant":
			fmt.Println("Displaying 4-color quadrant test pattern...")
			if err := dev.DisplayTestPattern(); err != nil {
				return fmt.Errorf("failed to display pattern: %w", err)
			}
		case "bars":
			fmt.Println("Displaying 8-color bar test pattern...")
			if err := dev.DisplayColorBars(); err != nil {
				return fmt.Errorf("failed to display pattern: %w", err)
			}
		case "red":
			fmt.Println("Displaying solid red...")
			if err := dev.DisplaySolidColor(255, 0, 0); err != nil {
				return fmt.Errorf("failed to display color: %w", err)
			}
		case "green":
			fmt.Println("Displaying solid green...")
			if err := dev.DisplaySolidColor(0, 255, 0); err != nil {
				return fmt.Errorf("failed to display color: %w", err)
			}
		case "blue":
			fmt.Println("Displaying solid blue...")
			if err := dev.DisplaySolidColor(0, 0, 255); err != nil {
				return fmt.Errorf("failed to display color: %w", err)
			}
		case "white":
			fmt.Println("Displaying solid white...")
			if err := dev.DisplaySolidColor(255, 255, 255); err != nil {
				return fmt.Errorf("failed to display color: %w", err)
			}
		case "black":
			fmt.Println("Displaying solid black...")
			if err := dev.DisplaySolidColor(0, 0, 0); err != nil {
				return fmt.Errorf("failed to display color: %w", err)
			}
		default:
			return fmt.Errorf("unknown pattern: %s (use: quadrant, bars, red, green, blue, white, black)", pattern)
		}

		fmt.Println("Test pattern displayed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(panelCmd)

	panelCmd.AddCommand(panelStatusCmd)
	panelCmd.AddCommand(panelOnCmd)
	panelCmd.AddCommand(panelOffCmd)
	panelCmd.AddCommand(panelBrightnessCmd)
	panelCmd.AddCommand(panelTestCmd)

	panelTestCmd.Flags().StringP("pattern", "p", "quadrant",
		"Test pattern to display (quadrant, bars, red, green, blue, white, black)")
}
