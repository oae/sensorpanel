// cmd/theme.go - Theme management commands
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/alperen/sensorpanel/pkg/browser"
	"github.com/alperen/sensorpanel/pkg/config"
	"github.com/alperen/sensorpanel/pkg/paths"
	"github.com/alperen/sensorpanel/pkg/sensors"
	"github.com/alperen/sensorpanel/pkg/server"
	"github.com/alperen/sensorpanel/pkg/theme"
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage display themes",
	Long: `Commands for creating, listing, and selecting themes.

Themes are web-based (React/HTML/CSS) and rendered using a headless browser.
Theme files are stored in ~/.local/share/sensorpanel/themes/`,
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed themes",
	RunE: func(cmd *cobra.Command, args []string) error {
		themes, err := theme.List()
		if err != nil {
			return fmt.Errorf("failed to list themes: %w", err)
		}

		// Get current theme from config
		currentTheme, _ := config.GetTheme()

		if len(themes) == 0 {
			fmt.Println("No themes installed.")
			fmt.Println("Use 'sensorpanel theme create <name>' to create one.")
			return nil
		}

		fmt.Println("=== Installed Themes ===")
		for _, t := range themes {
			marker := "  "
			if t.Name == currentTheme {
				marker = "* "
			}
			status := ""
			if !t.HasDist {
				status = " (not built)"
			}
			fmt.Printf("%s%s - %s%s\n", marker, t.Name, t.Metadata.Description, status)
		}
		fmt.Println()
		fmt.Println("* = active theme")

		return nil
	},
}

var themeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new theme from template",
	Long: `Create a new theme with React template.

This creates a new theme directory with:
  - package.json  - Theme metadata and dependencies
  - src/          - React source files
  - dist/         - Pre-built output (works immediately)
  - vite.config.js - Build configuration

You can customize the theme by editing files in src/ and running 'npm run build'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		fmt.Printf("Creating theme '%s'...\n", name)
		t, err := theme.Create(name)
		if err != nil {
			if err == theme.ErrThemeExists {
				return fmt.Errorf("theme '%s' already exists", name)
			}
			return err
		}

		fmt.Printf("Theme created at: %s\n", t.Path)
		fmt.Println()
		fmt.Println("The theme is ready to use with the pre-built template.")
		fmt.Println()
		fmt.Println("To customize:")
		fmt.Printf("  cd %s\n", t.Path)
		fmt.Println("  npm install      # Install dependencies")
		fmt.Println("  npm run dev      # Start development server")
		fmt.Println("  npm run build    # Build for production")
		fmt.Println()
		fmt.Printf("To use this theme: sensorpanel theme select %s\n", name)

		return nil
	},
}

var themeSelectCmd = &cobra.Command{
	Use:   "select <name>",
	Short: "Select a theme to use",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Verify theme exists
		t, err := theme.Load(name)
		if err != nil {
			if err == theme.ErrThemeNotFound {
				return fmt.Errorf("theme '%s' not found (use 'theme list' to see available themes)", name)
			}
			return err
		}

		if !t.HasDist {
			fmt.Printf("Warning: Theme '%s' has not been built.\n", name)
			fmt.Printf("Run 'cd %s && npm run build' first.\n", t.Path)
		}

		if err := config.SetTheme(name); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("Selected theme: %s\n", name)
		return nil
	},
}

var themePreviewCmd = &cobra.Command{
	Use:   "preview [name]",
	Short: "Preview a theme in the browser",
	Long: `Open a theme in the default web browser for preview.

If no name is provided, previews the currently selected theme.
The preview shows mock sensor data that updates in real-time.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		} else {
			var err error
			themeName, err = config.GetTheme()
			if err != nil || themeName == "" {
				return fmt.Errorf("no theme specified and no theme selected in config")
			}
		}

		t, err := theme.Load(themeName)
		if err != nil {
			return fmt.Errorf("failed to load theme: %w", err)
		}

		if !t.HasDist {
			return fmt.Errorf("theme '%s' not built - run 'npm run build' in theme directory", themeName)
		}

		// Open in browser
		indexPath := t.IndexPath()
		url := "file://" + indexPath

		fmt.Printf("Opening theme preview: %s\n", url)
		return openBrowser(url)
	},
}

var themeDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a theme",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if it's the active theme
		currentTheme, _ := config.GetTheme()
		if currentTheme == name {
			fmt.Println("Warning: Deleting the currently active theme.")
			fmt.Println("Run 'sensorpanel theme select <other>' to select another theme.")
		}

		if err := theme.Delete(name); err != nil {
			return err
		}

		// Clear from config if it was active
		if currentTheme == name {
			config.SetTheme("")
		}

		fmt.Printf("Deleted theme: %s\n", name)
		return nil
	},
}

var themePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show theme directory path",
	RunE: func(cmd *cobra.Command, args []string) error {
		themesDir, err := paths.ThemesDir()
		if err != nil {
			return err
		}
		fmt.Println(themesDir)
		return nil
	},
}

var themeDevPort int

var themeDevCmd = &cobra.Command{
	Use:   "dev [name]",
	Short: "Start sensor data server for theme development",
	Long: `Start a WebSocket server that broadcasts live sensor data for theme development.

This allows you to develop themes with real sensor data:
  1. Run 'sensorpanel theme dev' to start the sensor server
  2. In another terminal, run 'npm run dev' in your theme directory
  3. Open http://localhost:3000?ws=PORT (use the port shown by this command)

The sensor data will be broadcast via WebSocket and your theme will update in real-time.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var themeName string
		if len(args) > 0 {
			themeName = args[0]
		}

		// If theme specified, verify it exists
		if themeName != "" {
			if _, err := theme.Load(themeName); err != nil {
				return fmt.Errorf("failed to load theme '%s': %w", themeName, err)
			}
		}

		// Create sensor collector
		sensorConfig := &sensors.Config{
			ShowCPU:          true,
			ShowGPU:          true,
			ShowRAM:          true,
			ShowDisk:         true,
			ShowNetwork:      true,
			DiskMounts:       []string{"/"},
			NetworkInterface: "*",
			GPUMethod:        "auto",
		}
		collector := sensors.NewCollector(sensorConfig)

		// Start sensor server (no theme files, just WebSocket)
		// We'll use a simple HTTP server with just the /ws endpoint
		srv := server.New("") // Empty dir - will only serve WebSocket
		if err := srv.Start(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}
		defer srv.Stop()

		port := srv.Port()
		fmt.Println("=== Theme Development Server ===")
		fmt.Printf("WebSocket server running on port %d\n", port)
		fmt.Println()
		fmt.Println("To connect your theme:")
		fmt.Printf("  1. Run 'npm run dev' in your theme directory\n")
		fmt.Printf("  2. Open: http://localhost:3000?ws=%d\n", port)
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop.")
		fmt.Println()

		// Setup signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Prime the CPU load calculation
		collector.Collect()
		time.Sleep(100 * time.Millisecond)

		// Main loop - collect and broadcast sensor data
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		frameCount := 0
		for {
			select {
			case <-sigChan:
				fmt.Println("\nShutting down...")
				return nil

			case <-ticker.C:
				data := collector.Collect()
				jsonData := sensorDataToJSON(data)
				if err := srv.BroadcastSensorData(jsonData); err == nil {
					frameCount++
					if frameCount%10 == 0 {
						clients := srv.ClientCount()
						if clients > 0 {
							fmt.Printf("Broadcasting to %d client(s)...\n", clients)
						}
					}
				}
			}
		}
	},
}

var themeBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Manage headless browser for theme rendering",
}

var themeBrowserInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Download headless Chrome for theme rendering",
	Long: `Download Chrome for Testing to render themes.

This downloads a portable Chrome binary to ~/.cache/sensorpanel/browser/
The browser is used to render web-based themes to images for the display.

If you have Chrome/Chromium already installed, this is optional.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if browser.IsInstalled() {
			path, _ := browser.BrowserPath()
			fmt.Printf("Chrome already installed at: %s\n", path)
			fmt.Println("Use 'theme browser remove' to remove it first.")
			return nil
		}

		fmt.Printf("Downloading Chrome for Testing v%s...\n", browser.Version())
		fmt.Println("This may take a few minutes.")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		lastPercent := -1
		err := browser.Download(ctx, func(downloaded, total int64) {
			if total > 0 {
				percent := int(downloaded * 100 / total)
				if percent != lastPercent && percent%10 == 0 {
					fmt.Printf("  %d%% (%d / %d MB)\n", percent, downloaded/1024/1024, total/1024/1024)
					lastPercent = percent
				}
			}
		})
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		path, _ := browser.BrowserPath()
		fmt.Printf("Chrome installed at: %s\n", path)
		return nil
	},
}

var themeBrowserStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check headless browser status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Browser Status ===")

		// Check cached browser
		if path, err := browser.BrowserPath(); err == nil {
			fmt.Printf("Cached:  %s (v%s)\n", path, browser.Version())
		} else {
			fmt.Println("Cached:  Not installed")
		}

		// Check system browser
		if path, err := browser.SystemChromePath(); err == nil {
			fmt.Printf("System:  %s\n", path)
		} else {
			fmt.Println("System:  Not found")
		}

		// Which will be used
		if path, err := browser.GetChromePath(); err == nil {
			fmt.Printf("\nWill use: %s\n", path)
		} else {
			fmt.Println("\nNo browser available!")
			fmt.Println("Run 'sensorpanel theme browser install' to download one.")
		}

		return nil
	},
}

var themeBrowserRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove cached headless browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := browser.Remove(); err != nil {
			return err
		}
		fmt.Println("Cached browser removed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(themeCmd)

	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeCreateCmd)
	themeCmd.AddCommand(themeSelectCmd)
	themeCmd.AddCommand(themePreviewCmd)
	themeCmd.AddCommand(themeDeleteCmd)
	themeCmd.AddCommand(themePathCmd)
	themeCmd.AddCommand(themeDevCmd)
	themeCmd.AddCommand(themeBrowserCmd)

	themeBrowserCmd.AddCommand(themeBrowserInstallCmd)
	themeBrowserCmd.AddCommand(themeBrowserStatusCmd)
	themeBrowserCmd.AddCommand(themeBrowserRemoveCmd)
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	// Don't wait for browser to close
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
