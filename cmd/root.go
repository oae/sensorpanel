// Package cmd implements the CLI commands for sensorpanel.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sensorpanel",
	Short: "Control AX206 USB display panels",
	Long: `sensorpanel is a CLI tool for controlling AX206-based USB display panels.

These cheap USB displays (often sold as AIDA64-compatible) can be used
as sensor panels to show system metrics like CPU, GPU, RAM, and network stats.

Supported: AX206-based displays (480x320, RGB565)

Before first use, run 'sensorpanel device select' to choose your display.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
}
