package cmd

import (
	"fmt"

	"github.com/alperen/sensorpanel/pkg/service"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage sensorpanel autostart service",
	Long: `Manage the sensorpanel background service that starts automatically on login.

This command provides cross-platform service management:
  - Linux: systemd user service
  - macOS: launchd LaunchAgent
  - Windows: Startup folder shortcut`,
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install sensorpanel as an autostart service",
	Long: `Install sensorpanel to start automatically when you log in.

The service will run 'sensorpanel run' with the options specified.
Use --opt to pass sensor options to the service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, _ := cmd.Flags().GetStringArray("opt")

		if err := service.Install(opts); err != nil {
			return err
		}

		fmt.Println("Service installed successfully!")
		fmt.Println()
		fmt.Println("The service will start automatically on next login.")
		fmt.Println("To start it now, run: sensorpanel service start")
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove sensorpanel autostart service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Uninstall(); err != nil {
			return err
		}

		fmt.Println("Service uninstalled successfully!")
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the sensorpanel service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Start(); err != nil {
			return err
		}

		fmt.Println("Service started!")
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the sensorpanel service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Stop(); err != nil {
			return err
		}

		fmt.Println("Service stopped!")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sensorpanel service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := service.Status()
		if err != nil {
			return err
		}

		fmt.Printf("Service: %s\n", status.State)
		if status.Installed {
			fmt.Printf("Installed: yes\n")
			fmt.Printf("Path: %s\n", status.Path)
		} else {
			fmt.Printf("Installed: no\n")
		}
		if status.Running {
			fmt.Printf("Running: yes\n")
			if status.PID > 0 {
				fmt.Printf("PID: %d\n", status.PID)
			}
		} else {
			fmt.Printf("Running: no\n")
		}

		return nil
	},
}

var serviceLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show sensorpanel service logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")

		return service.Logs(follow, lines)
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)

	// Install flags
	serviceInstallCmd.Flags().StringArrayP("opt", "o", nil, "Sensor options to pass to the service (e.g., disk.mounts=/)")

	// Logs flags
	serviceLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	serviceLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
}
